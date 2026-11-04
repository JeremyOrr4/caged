package hostconfig

import (
	"cmp"
	"path"
	"path/filepath"
	"strings"

	"github.com/JeremyOrr4/caged/internal/plan"
)

// mountTree mounts root, a resolved file or directory, at dest: its largest
// symlink-free subtrees directly, and each symlink inside it through followLink.
func (r *resolution) mountTree(o owner, root, dest string, depth int, via string) {
	if o.staged {
		r.mountStagedTree(o, root, dest, depth, via)
		return
	}
	mounts, links, refused := collectSafeMounts(r.fs, root)
	for _, refusal := range refused {
		r.report.Refused = appendUnique(r.report.Refused, refusal)
	}
	for _, mountPath := range mounts {
		if mountDest, ok := r.below(root, mountPath, dest); ok {
			r.addMount(o, mountPath, mountDest, nil, nil, via)
		}
	}
	for _, link := range links {
		if linkDest, ok := r.below(root, link, dest); ok {
			r.followLink(o, root, link, linkDest, depth)
		}
	}
}

// below maps hostPath, inside root, to the same relative path under dest.
func (r *resolution) below(root, hostPath, dest string) (string, bool) {
	rel, err := filepath.Rel(root, hostPath)
	if err != nil {
		r.refuse(hostPath, "cannot map safe config path")
		return "", false
	}
	if rel == "." {
		return dest, true
	}
	return path.Join(dest, filepath.ToSlash(rel)), true
}

// addMount records a mount of hostPath at dest, a container path, with its
// masked directories and files, unless the same host path is already mounted
// there. link is the symlink it follows. A path docker cannot mount is
// refused instead (dockerCanMount).
func (r *resolution) addMount(o owner, hostPath, dest string, masks, maskFiles []string, link string) {
	if !r.dockerCanMount(hostPath, dest, link) {
		return
	}
	key := hostPath + "\x00" + dest
	if _, ok := r.seen[key]; ok {
		return
	}
	r.seen[key] = struct{}{}
	r.report.Mounts = append(r.report.Mounts, plan.ConfigMount{
		Agent:     o.agent,
		Name:      o.name,
		HostPath:  hostPath,
		DestPath:  dest,
		Root:      o.root,
		Writable:  o.writable,
		MaskPaths: masks,
		MaskFiles: maskFiles,
		Link:      link,
		Staged:    o.staged,
	})
}

// dockerCanMount refuses a mount whose host or container path holds a colon:
// docker's -v and --tmpfs split their argument there, so the run would fail,
// or a destination such as "x:ro" would be mounted elsewhere. A colon only in
// dest comes from a link's name, so the link is named. Commas, quotes and
// newlines mount as they are. Masks need no check: a staged tree's masks
// never reach docker (configstage mounts a copy without them), and the others
// come from agent definitions.
func (r *resolution) dockerCanMount(hostPath, dest, link string) bool {
	const reason = `path contains ":", which docker cannot mount`
	switch {
	case strings.Contains(hostPath, ":"):
		r.refuse(hostPath, reason)
	case strings.Contains(dest, ":"):
		r.refuse(cmp.Or(link, dest), reason)
	default:
		return true
	}
	return false
}

// dropNestedLinkMounts refuses symlink mounts whose destination lies inside
// another mount: Docker would create the mountpoint inside that host directory.
func (r *resolution) dropNestedLinkMounts() {
	var kept []plan.ConfigMount
	for _, mount := range r.report.Mounts {
		if !mount.Staged && mount.Link != "" && mount.Link != mount.Root && r.insideOtherMount(mount) {
			r.refuse(mount.Link, "symlink target would be mounted inside another mount at "+mount.DestPath)
			continue
		}
		kept = append(kept, mount)
	}
	r.report.Mounts = kept
}

func (r *resolution) insideOtherMount(mount plan.ConfigMount) bool {
	for _, other := range r.report.Mounts {
		if other.DestPath == mount.DestPath {
			continue
		}
		if _, inside := plan.ContainerRel(mount.DestPath, other.DestPath); inside {
			return true
		}
	}
	return false
}
