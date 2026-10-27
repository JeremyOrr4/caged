package configstage

import (
	"slices"
	"strings"

	"github.com/JeremyOrr4/caged/internal/plan"
)

// group is a staged config root and the staged mounts nested inside it, such
// as the targets of symlinks in the root. Indexes refer to the mounts given
// to planGroups.
type group struct {
	members []int // the root first, then the nested mounts, shallowest first
}

// planGroups returns the groups that need staging: a root with masks, or with
// mounts nested inside it. A root is a staged mount that no other staged
// mount of the same config root encloses.
func planGroups(mounts []plan.ConfigMount) []group {
	var groups []group
	for i, mount := range mounts {
		if !mount.Staged || enclosed(mounts, i) {
			continue
		}
		members := []int{i}
		for j, other := range mounts {
			if j != i && sameTree(mount, other) && inside(other.DestPath, mount.DestPath) {
				members = append(members, j)
			}
		}
		if len(members) == 1 && len(mount.MaskPaths) == 0 && len(mount.MaskFiles) == 0 {
			continue // nothing in it needs hiding: mounted as it is
		}
		slices.SortStableFunc(members, func(a, b int) int {
			return depth(mounts[a].DestPath) - depth(mounts[b].DestPath)
		})
		groups = append(groups, group{members: members})
	}
	return groups
}

// enclosed reports whether another staged mount of the same config root
// encloses mount i.
func enclosed(mounts []plan.ConfigMount, i int) bool {
	for j, other := range mounts {
		if j != i && sameTree(mounts[i], other) && inside(mounts[i].DestPath, other.DestPath) {
			return true
		}
	}
	return false
}

// sameTree reports whether a and b are staged mounts made for the same
// config root.
func sameTree(a, b plan.ConfigMount) bool {
	return a.Staged && b.Staged && a.Root == b.Root
}

// nestedIn returns the destinations of the group's mounts inside mount i,
// relative to its destination.
func (g group) nestedIn(mounts []plan.ConfigMount, i int) []string {
	var paths []string
	for _, j := range g.members {
		if j == i {
			continue
		}
		if rel, ok := plan.ContainerRel(mounts[j].DestPath, mounts[i].DestPath); ok && rel != "." {
			paths = append(paths, rel)
		}
	}
	return paths
}

// mountpoint is where, inside the copy of parent, the copy of child is
// mounted.
type mountpoint struct {
	parent, child int
	rel           string // the child's destination relative to the parent's
}

// mountpoints returns the mountpoint of every mount in the group inside its
// nearest enclosing mount. A mount is not given a mountpoint in an ancestor
// when another mount lies in between.
func (g group) mountpoints(mounts []plan.ConfigMount) []mountpoint {
	var points []mountpoint
	for _, parent := range g.members {
		for _, child := range g.members {
			rel, ok := plan.ContainerRel(mounts[child].DestPath, mounts[parent].DestPath)
			if !ok || rel == "." || g.between(mounts, parent, child) { // "." is the mount itself
				continue
			}
			points = append(points, mountpoint{parent: parent, child: child, rel: rel})
		}
	}
	return points
}

// between reports whether a third mount of the group lies between parent and
// child.
func (g group) between(mounts []plan.ConfigMount, parent, child int) bool {
	for _, middle := range g.members {
		if middle != parent && middle != child &&
			inside(mounts[middle].DestPath, mounts[parent].DestPath) &&
			inside(mounts[child].DestPath, mounts[middle].DestPath) {
			return true
		}
	}
	return false
}

// inside reports whether the container path child lies strictly inside parent.
func inside(child, parent string) bool {
	rel, ok := plan.ContainerRel(child, parent)
	return ok && rel != "."
}

// depth counts the separators in a container path.
func depth(p string) int { return strings.Count(p, "/") }

// withinAny reports whether rel, a slash path in the tree, is one of roots or
// inside one. roots are masks and nested mounts, never the tree itself.
func withinAny(rel string, roots []string) bool {
	return slices.ContainsFunc(roots, func(root string) bool {
		_, ok := plan.ContainerRel(rel, root)
		return ok
	})
}

// under returns the paths of list that lie in the directory name, an entry of
// the tree, relative to it; name itself becomes ".". syncDir never descends
// into a name that is itself in list, so a "." never reaches withinAny, where
// it would hold every entry.
func under(list []string, name string) []string {
	var out []string
	for _, p := range list {
		if rest, ok := plan.ContainerRel(p, name); ok {
			out = append(out, rest)
		}
	}
	return out
}
