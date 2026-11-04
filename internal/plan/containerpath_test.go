package plan

import "testing"

func TestContainerRel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path, dir string
		rel       string
		ok        bool
	}{
		{"/home/caged/.pi", "/home/caged/.pi", ".", true},
		{"/home/caged/.pi/agent/x.json", "/home/caged/.pi", "agent/x.json", true},
		{"/home/caged/.pi/agent", "/home/caged/.pi/", "agent", true},
		{"/home/caged/.pi2", "/home/caged/.pi", "", false},
		{"/home/caged", "/home/caged/.pi", "", false},
		{"/home/caged/.pi/../x", "/home/caged/.pi", "", false},
		{"/workspace", "/", "workspace", true},
		{"/", "/", ".", true},
		{"sub/c.json", "sub", "c.json", true}, // tree-relative names, as configstage uses them
		{"subx/c.json", "sub", "", false},
		{"a/b", "a/b", ".", true},
		{"sub/c.json", ".", "sub/c.json", true}, // the tree root holds every name in the tree
		{"../x", ".", "", false},
		{"/abs", ".", "", false},
		{"rel", "/", "", false},
	}
	for _, tc := range tests {
		if rel, ok := ContainerRel(tc.path, tc.dir); rel != tc.rel || ok != tc.ok {
			t.Errorf("ContainerRel(%q, %q) = %q, %v; want %q, %v", tc.path, tc.dir, rel, ok, tc.rel, tc.ok)
		}
	}
}
