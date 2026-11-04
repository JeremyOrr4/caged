package hostconfig

import (
	"fmt"
	"io/fs"
	"maps"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"github.com/JeremyOrr4/caged/internal/pathpolicy"
	"github.com/JeremyOrr4/caged/internal/plan"
)

// mountWholeTree mounts root, a resolved config file or directory, at dest as
// one live tree, so the agent can create files as well as edit them and
// every change lands on the host as it is made. masks are paths relative to
// root that are hidden under tmpfs mounts and not inspected (Codex's runtime
// directories). Special files are hidden under the empty mask file and
// directories that cannot be read under a tmpfs. The tree's symlinks stay
// visible in the container: a link that resolves there as it does on the host
// needs nothing, and the target of any other is mounted where the link points
// inside the container (wholeTreeLink).
func (r *resolution) mountWholeTree(o owner, root, dest string, depth int, via string, masks []string) {
	switch classifyEntry(r.fs.Lstat(root)) {
	case entryUninspectable:
		r.refuse(root, reasonUninspectableRoot)
		return
	case entryFile:
		r.addMount(o, root, dest, nil, nil, via)
		return
	case entryDir:
	default:
		r.refuse(root, reasonRootNotFileOrDir)
		return
	}
	scan, ok := r.scanWholeTree(root, masks)
	if !ok {
		return
	}
	if !r.addMount(o, root, dest, scan.dirMasks, scan.fileMasks, via) {
		return
	}
	for _, link := range scan.links {
		r.wholeTreeLink(o, root, link, dest, depth)
	}
}

// scanWholeTree walks root, a directory, without following symlinks or
// entering masks, and records what its mount must hide and which links it
// holds. It refuses the entries it hides, and reports false, after refusing
// root, when root cannot be walked.
func (r *resolution) scanWholeTree(root string, masks []string) (wholeScan, bool) {
	maskPaths, err := maskedPaths(root, masks)
	if err != nil {
		r.refuse(root, err.Error())
		return wholeScan{}, false
	}
	scan := wholeScan{dirMasks: append([]string(nil), masks...)}
	err = r.fs.WalkDir(root, func(name string, entry fs.DirEntry, walkErr error) error {
		if name == root {
			// WalkDir calls back a second time, with the error, for a root it
			// cannot read; returning it fails the walk, so the tree is refused.
			return walkErr
		}
		if insideAny(name, maskPaths) {
			if entry != nil && entry.IsDir() && walkErr == nil {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		// An entry docker cannot hide is refused by hideUnsafeMasks instead.
		if reason := scan.record(r.fs, name, filepath.ToSlash(rel), walkErr); reason != "" && dockerSafeMask(filepath.ToSlash(rel)) {
			r.refuse(name, reason)
		}
		return nil
	})
	if err != nil {
		r.refuse(root, "cannot inspect config tree safely")
		return wholeScan{}, false
	}
	if !r.hideUnsafeMasks(root, len(masks), &scan) {
		return wholeScan{}, false
	}
	return scan, true
}

// dockerSafeMask reports whether docker's --mount reads back unchanged a
// mount destination ending in mask, a slash path below the tree root:
// docker trims white space from the end of the value (or refuses it there,
// before a following field), and its CSV reader turns "\r\n" into "\n".
func dockerSafeMask(mask string) bool {
	return strings.TrimRightFunc(mask, unicode.IsSpace) == mask && !strings.ContainsRune(mask, '\r')
}

// hideUnsafeMasks replaces each mask scan found that docker would misread
// (dockerSafeMask) by its nearest parent directory that it reads correctly,
// hidden under a tmpfs with everything in it, so a planted name cannot move a
// mask onto another entry. When the entry has no such parent below root,
// the tree is not mounted, and false is returned. The first fixed directory
// masks are the agent's own, and are kept. Either way the refusal is shown on
// every run, and a staged copy, which leaves the entry out, would help.
func (r *resolution) hideUnsafeMasks(root string, fixed int, scan *wholeScan) bool {
	var hidden []string // host paths of the parents hidden instead
	for _, mask := range append(slices.Clone(scan.dirMasks[fixed:]), scan.fileMasks...) {
		entry := filepath.Join(root, filepath.FromSlash(mask))
		if dockerSafeMask(mask) || insideAny(entry, hidden) {
			continue
		}
		r.needsCopy++
		parent := path.Dir(mask)
		for parent != "." && !dockerSafeMask(parent) {
			parent = path.Dir(parent)
		}
		const reason = "its name ends in white space or holds a carriage return, which docker cannot hide; "
		if parent == "." {
			r.warn(entry, reason+"its config folder "+pathpolicy.Tilde(r.resolvedHome, root)+" is not mounted", "rename or remove it")
			return false
		}
		hidden = append(hidden, filepath.Join(root, filepath.FromSlash(parent)))
		r.warn(entry, reason+"its folder "+pathpolicy.Tilde(r.resolvedHome, hidden[len(hidden)-1])+" is hidden instead", "rename or remove it")
	}
	if len(hidden) == 0 {
		return true
	}
	outside := func(list []string, host func(string) string) []string {
		return slices.DeleteFunc(list, func(item string) bool { return insideAny(host(item), hidden) })
	}
	rel := func(mask string) string { return filepath.Join(root, filepath.FromSlash(mask)) }
	own := slices.Clone(scan.dirMasks[:fixed])
	scan.dirMasks = append(own, outside(scan.dirMasks[fixed:], rel)...)
	scan.fileMasks = outside(scan.fileMasks, rel)
	scan.links = outside(scan.links, func(link string) string { return link })
	for _, dir := range hidden {
		parent, err := filepath.Rel(root, dir)
		if err != nil {
			return false
		}
		scan.dirMasks = append(scan.dirMasks, filepath.ToSlash(parent))
	}
	return true
}

// warn records a refusal the user must act on (plan.Refusal.Warn).
func (r *resolution) warn(path, reason, hint string) {
	r.report.Refused = appendUnique(r.report.Refused, plan.Refusal{Path: path, Reason: reason, Hint: hint, Warn: true})
}

// wholeScan is what the walk of a whole tree finds: the entries hidden from
// its mount, as directory masks (tmpfs) and file masks (empty-file binds), and
// its symlinks, which stay visible.
type wholeScan struct {
	dirMasks, fileMasks, links []string
}

// record notes the entry name, at mask below the tree root, and returns the
// reason to refuse it with, or "" when the entry is mounted as it is.
// walkErr is the error WalkDir reported for the entry.
func (s *wholeScan) record(fsys FileSystem, name, mask string, walkErr error) string {
	kind := classifyEntry(fsys.Lstat(name))
	switch {
	case walkErr != nil:
		s.hide(mask, kind == entryDir)
		return reasonUninspectableEntry
	case kind == entryUninspectable:
		s.hide(mask, false)
		return reasonUninspectableEntry
	case kind == entrySymlink:
		s.links = append(s.links, name)
	case kind == entrySpecial:
		s.hide(mask, false)
		return reasonSpecialFile
	}
	return ""
}

// hide masks the entry at mask as a directory or as a file.
func (s *wholeScan) hide(mask string, dir bool) {
	if dir {
		s.dirMasks = append(s.dirMasks, mask)
	} else {
		s.fileMasks = append(s.fileMasks, mask)
	}
}

// wholeTreeLink handles link, a symlink in the whole tree rooted at treeRoot
// and mounted at treeDest. The link is visible in the container, so it is
// read there as the kernel reads it: containerLinkDest is where it points. A
// link that points into the tree mount at the entry it resolves to on the
// host already works and needs nothing. Any other link has its target
// mounted where it points, by followLink, which applies the link policy. A
// link that cannot be followed inside the container, or that points into the
// tree mount at a different entry, is refused; when followLink would have
// accepted its target it is also counted in needsCopy, so an owner that may
// run from a staged copy instead knows the copy would help.
func (r *resolution) wholeTreeLink(o owner, treeRoot, link, treeDest string, depth int) {
	text, err := r.fs.Readlink(link)
	if err != nil {
		r.refuse(link, "cannot read symlink")
		return
	}
	linkDest, ok := r.below(treeRoot, link, treeDest)
	if !ok {
		return
	}
	roots := append([]string{r.home, r.resolvedHome}, r.policy.AllowPaths()...)
	dest, mapped := containerLinkDest(linkDest, text, roots)
	if rel, inside := plan.ContainerRel(dest, treeDest); mapped && inside {
		target, err := r.fs.EvalSymlinks(link)
		if err != nil {
			r.refuse(link, "dangling or unresolvable symlink")
			return
		}
		if r.resolvesTo(filepath.Join(treeRoot, filepath.FromSlash(rel)), target) {
			return // the link resolves inside the mount as it does on the host
		}
		r.refuseUnfollowable(o, treeRoot, link, text, hintInTree)
		return
	}
	if !mapped {
		r.refuseUnfollowable(o, treeRoot, link, text, hintUnfollowable)
		return
	}
	// The target leaves the tree, or is part of it but named by a path the
	// container sees elsewhere (an absolute host path): it is mounted there,
	// rescanned, and refused by followLink if it touches a config mask.
	r.followLink(o, treeRoot, link, dest, depth)
}

// The hints for a link the container cannot follow.
const (
	hintInTree       = "make it a relative link within its folder"
	hintUnfollowable = `point it at a path under your home or a links.allow root, with any ".." at the start`
)

// refuseUnfollowable refuses link, a symlink with text text in the tree
// rooted at treeRoot that the container cannot follow, with hint. A policy
// refusal says more, and gives its own hint, so it comes first. Otherwise,
// when followLink would have accepted the target, the link is counted in
// needsCopy, so an owner that may run from a staged copy instead knows the
// copy would help.
func (r *resolution) refuseUnfollowable(o owner, treeRoot, link, text, hint string) {
	if target, err := r.fs.EvalSymlinks(link); err == nil && !pathpolicy.Within(target, treeRoot) {
		if decision := o.policy.Check(target); decision.Verdict != pathpolicy.Allowed {
			r.refuseLink(link, target, decision)
			return
		}
	}
	if r.acceptable(o, treeRoot, link) {
		r.needsCopy++
	}
	reason := fmt.Sprintf("symlink to %s cannot be followed inside the container", text)
	r.report.Refused = appendUnique(r.report.Refused, plan.Refusal{Path: link, Reason: reason, Hint: hint})
}

// resolvesTo reports whether the host path name resolves to target.
func (r *resolution) resolvesTo(name, target string) bool {
	resolved, err := r.fs.EvalSymlinks(name)
	return err == nil && filepath.Clean(resolved) == filepath.Clean(target)
}

// acceptable reports whether followLink would mount the target of link, a
// symlink in the tree rooted at treeRoot, without recording anything.
func (r *resolution) acceptable(o owner, treeRoot, link string) bool {
	target, err := r.fs.EvalSymlinks(link)
	if err != nil {
		return false
	}
	info, err := r.fs.Stat(target)
	if err != nil || !info.IsDir() && !info.Mode().IsRegular() {
		return false
	}
	if !pathpolicy.Within(target, treeRoot) && o.policy.Check(target).Verdict != pathpolicy.Allowed {
		return false
	}
	if r.touchesMask(target) {
		return false
	}
	return !info.IsDir() || !pathpolicy.Within(link, target)
}

// touchesMask reports whether target is inside a masked path of an agent
// config tree, or holds one: mounting it would expose what the mask hides.
// Names are compared as the link policy's file system does, so on macOS a
// link spelled in another case is caught too.
func (r *resolution) touchesMask(target string) bool {
	for _, masked := range r.masked {
		if r.policy.Overlap(target, masked) {
			return true
		}
	}
	return false
}

// tryWholeTree mounts root as a live whole tree and reports true, or, when a
// link in it (or in a tree a link leads to) cannot be followed inside the
// container but would be accepted from a staged copy (needsCopy), undoes
// everything the attempt recorded and reports false, so the caller can stage
// a copy instead.
func (r *resolution) tryWholeTree(o owner, root, dest, via string) bool {
	mark := r.mark()
	r.mountWholeTree(o, root, dest, 0, via, nil)
	if r.needsCopy == mark.needsCopy {
		return true
	}
	r.restore(mark)
	return false
}

// resolutionMark is the state of a resolution that restore returns to. The
// report's lists only grow, so their lengths are enough.
type resolutionMark struct {
	mounts, missing, refused int
	seen                     map[string]struct{}
	linksFollowed            int
	needsCopy                int
}

func (r *resolution) mark() resolutionMark {
	return resolutionMark{
		mounts:        len(r.report.Mounts),
		missing:       len(r.report.Missing),
		refused:       len(r.report.Refused),
		seen:          maps.Clone(r.seen),
		linksFollowed: r.linksFollowed,
		needsCopy:     r.needsCopy,
	}
}

func (r *resolution) restore(m resolutionMark) {
	r.report.Mounts = r.report.Mounts[:m.mounts]
	r.report.Missing = r.report.Missing[:m.missing]
	r.report.Refused = r.report.Refused[:m.refused]
	r.seen = m.seen
	r.linksFollowed = m.linksFollowed
	r.needsCopy = m.needsCopy
}
