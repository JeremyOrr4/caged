package hostconfig

import (
	"cmp"
	"path"
	"path/filepath"
	"strings"

	"github.com/JeremyOrr4/caged/internal/plan"
)

// mountTree mounts root, a resolved file or directory, at dest: as a staged
// copy for an owner that runs from one, otherwise as one live tree.
func (r *resolution) mountTree(o owner, root, dest string, depth int, via string) {
	if o.staged {
		r.mountStagedTree(o, root, dest, depth, via)
		return
	}
	r.mountWholeTree(o, root, dest, depth, via, nil)
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
// refused instead (dockerCanMount). It reports whether the mount was
// recorded by this call.
func (r *resolution) addMount(o owner, hostPath, dest string, masks, maskFiles []string, link string) bool {
	if !r.dockerCanMount(hostPath, dest, link) {
		return false
	}
	key := hostPath + "\x00" + dest
	if _, ok := r.seen[key]; ok {
		return false
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
	return true
}

// dockerCanMount refuses a mount whose host or container path holds a colon:
// docker's -v splits its argument there, so the run would fail,
// or a destination such as "x:ro" would be mounted elsewhere. A colon only in
// dest comes from a link's name, so the link is named. Commas, quotes and
// newlines mount as they are. Masks need no check: a staged tree's masks
// never reach docker (configstage mounts a copy without them), and the others
// are passed with --mount, whose fields are quoted (docker.mountSpec).
func (r *resolution) dockerCanMount(hostPath, dest, link string) bool {
	const reason = `path contains ":", which docker cannot mount`
	switch {
	case strings.Contains(hostPath, ":"):
		r.refuse(hostPath, reason)
	case strings.Contains(dest, ":"):
		r.refuse(cmp.Or(link, dest), reason)
	case strings.ContainsRune(dest, '\r'):
		// The masks inside the mount reach docker through --mount, whose CSV
		// reader turns "\r\n" into "\n", so they would land elsewhere.
		r.refuse(cmp.Or(link, dest), "path contains a carriage return, which docker cannot mount reliably")
	default:
		return true
	}
	return false
}

// dropNestedLinkMounts refuses symlink mounts that would collide with another
// mount: first those whose destination lies inside another mount (Docker
// would create the mountpoint inside that host directory), then, among the
// rest, those whose destination is another mount's (Docker refuses duplicate
// mountpoints) or holds one (the other mount's mountpoint would be created in
// the link target, through any symlink there). Root mounts and staged mounts
// are kept: configstage nests staged mounts inside its copy.
func (r *resolution) dropNestedLinkMounts() {
	keep := func(mounts []plan.ConfigMount, collides func(plan.ConfigMount, []plan.ConfigMount) bool, reason string) []plan.ConfigMount {
		var kept []plan.ConfigMount
		for _, mount := range mounts {
			if isLinkMount(mount) && collides(mount, mounts) {
				r.refuse(mount.Link, reason+mount.DestPath)
				continue
			}
			kept = append(kept, mount)
		}
		return kept
	}
	mounts := keep(r.report.Mounts, insideOtherMount, "symlink target would be mounted inside another mount at ")
	r.report.Mounts = keep(mounts, atOrOverOtherMount, "symlink target would be mounted at or over another mount at ")
}

// isLinkMount reports whether mount is the target of a symlink inside a
// config tree, not a config root.
func isLinkMount(mount plan.ConfigMount) bool {
	return !mount.Staged && mount.Link != "" && mount.Link != mount.Root
}

func insideOtherMount(mount plan.ConfigMount, mounts []plan.ConfigMount) bool {
	for _, other := range mounts {
		if other.DestPath == mount.DestPath {
			continue
		}
		if _, inside := plan.ContainerRel(mount.DestPath, other.DestPath); inside {
			return true
		}
	}
	return false
}

// atOrOverOtherMount reports whether another mount, of a different host path,
// has mount's destination, or lies below it.
func atOrOverOtherMount(mount plan.ConfigMount, mounts []plan.ConfigMount) bool {
	for _, other := range mounts {
		if other.HostPath == mount.HostPath && other.DestPath == mount.DestPath {
			continue // the mount itself
		}
		if _, below := plan.ContainerRel(other.DestPath, mount.DestPath); below {
			return true
		}
	}
	return false
}
