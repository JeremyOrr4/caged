package hostconfig

import (
	"fmt"

	"github.com/JeremyOrr4/caged/internal/pathpolicy"
	"github.com/JeremyOrr4/caged/internal/plan"
)

// followLink mounts the target of link, a symlink in the tree rooted at
// treeRoot, at dest. A target outside treeRoot must pass the link policy, and
// a directory target is mounted with mountTree, so links inside it are
// followed too.
func (r *resolution) followLink(o owner, treeRoot, link, dest string, depth int) {
	if r.linksFollowed >= maxLinks {
		r.refuse(link, fmt.Sprintf("more than %d symlinks in config; not followed", maxLinks))
		return
	}
	r.linksFollowed++
	target, err := r.fs.EvalSymlinks(link)
	if err != nil {
		r.refuse(link, "dangling or unresolvable symlink")
		return
	}
	info, err := r.fs.Stat(target)
	if err != nil {
		r.refuse(link, "cannot inspect symlink target")
		return
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		r.refuse(link, "symlink target is not a regular file or directory")
		return
	}
	if !pathpolicy.Within(target, treeRoot) {
		decision := o.policy.Check(target)
		if decision.Verdict != pathpolicy.Allowed {
			r.refuseLink(link, target, decision)
			return
		}
		o.writable = o.writable && !decision.Root.ReadOnly
	}
	if !info.IsDir() {
		r.addMount(o, target, dest, nil, nil, link)
		return
	}
	if pathpolicy.Within(link, target) {
		r.refuse(link, "symlink points to a directory that contains it")
		return
	}
	if depth >= maxLinkDepth {
		r.refuse(link, "too many nested symlinked directories")
		return
	}
	r.mountTree(o, target, dest, depth+1, link)
}

// followMaskedLink handles a symlink that leaves a config tree mounted whole.
// The link itself is visible in the container, so its target is mounted where
// the link points inside the container.
func (r *resolution) followMaskedLink(o owner, treeRoot, link, treeDest string) {
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
	dest, ok := containerLinkDest(linkDest, text, roots)
	if !ok {
		r.refuse(link, fmt.Sprintf("symlink to %s cannot be followed inside the container", text))
		return
	}
	r.followLink(o, treeRoot, link, dest, 0)
}

// refuseLink records a symlink whose target the policy refused, with the hint
// that tells the user how to allow it.
func (r *resolution) refuseLink(link, target string, decision pathpolicy.Decision) {
	isDir := false
	if info, err := r.fs.Stat(target); err == nil {
		isDir = info.IsDir()
	}
	shown := pathpolicy.Tilde(r.resolvedHome, target)
	reason := "symlink target " + shown + " is not under links.allow"
	if decision.Verdict == pathpolicy.Denied {
		reason = "symlink target " + shown + " is denied"
	}
	r.report.Refused = appendUnique(r.report.Refused, plan.Refusal{Path: link, Reason: reason, Hint: r.policy.Remedy(decision, target, isDir)})
}
