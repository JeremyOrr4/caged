package plan

import (
	"path"
	"slices"
	"strings"
	"testing"
)

// segments splits a cleaned slash path into its names; "/" and "." have none.
func segments(p string) []string {
	p = strings.Trim(path.Clean(p), "/")
	if p == "" || p == "." {
		return nil
	}
	return strings.Split(p, "/")
}

// FuzzContainerRel checks ContainerRel against a rule written by whole names:
// p is inside dir when both are absolute or both relative and dir's names are
// a prefix of p's, and a relative p never climbs out with "..". The returned
// relative path is "." for dir itself and otherwise joins back to p.
func FuzzContainerRel(f *testing.F) {
	for _, seed := range [][2]string{
		{"/home/caged/.pi/agent", "/home/caged/.pi"}, {"/home/caged/.pi2", "/home/caged/.pi"}, {"/workspace", "/"},
		{"/", "/"}, {"/a/b/", "/a/"}, {"/a/../b", "/a"}, {"/a", "/a/b"}, {"/a/b", "//a"},
		{"sub/c.json", "sub"}, {"subx/c", "sub"}, {"sub/c", "."}, {"../x", "."}, {"rel", "/"}, {"/abs", "rel"}, {"", ""},
	} {
		f.Add(seed[0], seed[1])
	}
	f.Fuzz(func(t *testing.T, p, dir string) {
		pNames, dirNames := segments(p), segments(dir)
		want := path.IsAbs(p) == path.IsAbs(dir) && len(dirNames) <= len(pNames) &&
			slices.Equal(dirNames, pNames[:len(dirNames)]) && (path.IsAbs(p) || len(dirNames) > 0 || len(pNames) == 0 || pNames[0] != "..")
		rel, ok := ContainerRel(p, dir)
		if ok != want {
			t.Fatalf("ContainerRel(%q, %q) ok = %v, want %v", p, dir, ok, want)
		}
		switch equal := path.Clean(p) == path.Clean(dir); {
		case !ok && rel != "":
			t.Fatalf("ContainerRel(%q, %q) = %q, false; want \"\"", p, dir, rel)
		case ok && equal && rel != ".":
			t.Fatalf("ContainerRel(%q, itself) = %q, want \".\"", p, rel)
		case ok && !equal && (rel == "." || path.Join(path.Clean(dir), rel) != path.Clean(p)):
			t.Fatalf("ContainerRel(%q, %q) = %q, which does not join back to the path", p, dir, rel)
		}
	})
}
