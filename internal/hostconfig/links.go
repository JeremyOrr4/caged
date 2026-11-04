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
	target, isDir, ok := r.linkTarget(&o, treeRoot, link)
	if !ok {
		return
	}
	if !isDir {
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

// linkTarget resolves link and checks its target: a regular file or
// directory, allowed by the link policy when outside treeRoot (a read_only
// root makes o read-only), and clear of every config mask. It refuses link
// and reports false when the target fails a check.
func (r *resolution) linkTarget(o *owner, treeRoot, link string) (target string, isDir, ok bool) {
	target, err := r.fs.EvalSymlinks(link)
	if err != nil {
		r.refuse(link, "dangling or unresolvable symlink")
		return "", false, false
	}
	info, err := r.fs.Stat(target)
	if err != nil {
		r.refuse(link, "cannot inspect symlink target")
		return "", false, false
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		r.refuse(link, "symlink target is not a regular file or directory")
		return "", false, false
	}
	if !pathpolicy.Within(target, treeRoot) {
		decision := o.policy.Check(target)
		if decision.Verdict != pathpolicy.Allowed {
			r.refuseLink(link, target, decision)
			return "", false, false
		}
		o.writable = o.writable && !decision.Root.ReadOnly
	}
	if r.touchesMask(target) {
		r.report.Refused = appendUnique(r.report.Refused, plan.Refusal{Path: link, Reason: "symlink target is, holds or sits inside a folder caged hides from agents (" + pathpolicy.Tilde(r.resolvedHome, target) + ")", Hint: "point the link elsewhere; Codex's runtime folders are never mounted"})
		return "", false, false
	}
	return target, info.IsDir(), true
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
