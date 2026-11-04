package plan

import (
	"path"
	"strings"
)

// ContainerRel returns p relative to dir and true when p is dir itself (".")
// or inside it, and "" and false otherwise. Both are slash-separated: container
// paths, or names relative to one tree. It is the containment test for
// container paths, with its arguments in the order of pathpolicy.Within, the
// test for host paths.
func ContainerRel(p, dir string) (string, bool) {
	p, dir = path.Clean(p), path.Clean(dir)
	if p == dir {
		return ".", true
	}
	switch {
	case dir == "." && p != ".." && !strings.HasPrefix(p, "../") && !path.IsAbs(p):
		return p, true // every relative name that does not climb out of the tree
	case dir == "/" && path.IsAbs(p):
		return p[1:], true
	}
	if rel, ok := strings.CutPrefix(p, dir+"/"); ok && dir != "." {
		return rel, true
	}
	return "", false
}
