package configstage

import (
	"reflect"
	"testing"

	"github.com/JeremyOrr4/caged/internal/plan"
)

// These tests pin the path rules of the staging layout: which container
// paths lie inside which, and which tree paths are masked or nested.

func TestInsideIsStrictContainment(t *testing.T) {
	tests := []struct {
		child, parent string
		want          bool
	}{
		{"/home/caged/.pi/agent", "/home/caged/.pi", true},
		{"/home/caged/.pi/a/b", "/home/caged/.pi", true},
		{"/home/caged/.pi/", "/home/caged/.pi", false}, // the same path, cleaned
		{"/home/caged/.pi", "/home/caged/.pi", false},
		{"/home/caged/.pi-other", "/home/caged/.pi", false}, // a namesake, not a child
		{"/home/caged", "/home/caged/.pi", false},
		{"/home/caged/.pi/x/../agent", "/home/caged/.pi", true},
		{"/home/caged/.pi/..", "/home/caged/.pi", false},
		{"/home/caged/.pi", "/home/caged/.pi/", false},
		{"/home/caged/.pi/agent", "/home/caged/.pi/", true},
		{"/a", "/", true},
		{"/", "/", false},
	}
	for _, tt := range tests {
		if got := inside(tt.child, tt.parent); got != tt.want {
			t.Errorf("inside(%q, %q) = %v, want %v", tt.child, tt.parent, got, tt.want)
		}
	}
}

func TestWithinAnyMatchesARootAndWhatIsBelowIt(t *testing.T) {
	roots := []string{"masked.txt", "sub/c.json", "themes/"}
	tests := []struct {
		rel  string
		want bool
	}{
		{"masked.txt", true},
		{"sub/c.json", true},
		{"c.json", false}, // a top-level namesake of sub/c.json
		{"sub", false},    // a parent of a root
		{"themes", true},  // a root written with a trailing slash
		{"themes/dark.json", true},
		{"themes-dark", false},
		{"sub/../masked.txt", true},
		{".", false},
		{"masked.txt/x", true},
	}
	for _, tt := range tests {
		if got := withinAny(tt.rel, roots); got != tt.want {
			t.Errorf("withinAny(%q, %q) = %v, want %v", tt.rel, roots, got, tt.want)
		}
	}
	if withinAny("a", nil) {
		t.Error("withinAny with no roots = true")
	}
}

func TestUnderReturnsTheListBelowADirectory(t *testing.T) {
	list := []string{"agent", "agent/settings.json", "agent/themes/", "agentx/a", "other/agent", "agent/../top"}
	if got, want := under(list, "agent"), []string{".", "settings.json", "themes"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("under(%q, agent) = %q, want %q", list, got, want)
	}
	if got := under(list, "none"); got != nil {
		t.Fatalf("under(%q, none) = %q, want nil", list, got)
	}
}

func TestGroupLayoutOfNestedMounts(t *testing.T) {
	const root = "/host/.pi"
	mounts := []plan.ConfigMount{
		{Staged: true, Root: root, DestPath: "/home/caged/.pi", MaskPaths: []string{"a"}},
		{Staged: true, Root: root, DestPath: "/home/caged/.pi/a/b"},
		{Staged: true, Root: root, DestPath: "/home/caged/.pi/a"},
		{Staged: true, Root: "/host/.other", DestPath: "/home/caged/.pi/a/c"}, // another config root
		{Staged: false, Root: root, DestPath: "/home/caged/.pi/d"},            // not staged
	}
	groups := planGroups(mounts)
	if want := []group{{members: []int{0, 2, 1}}}; !reflect.DeepEqual(groups, want) {
		t.Fatalf("planGroups() = %+v, want %+v", groups, want)
	}
	g := groups[0]
	if got, want := g.nestedIn(mounts, 0), []string{"a", "a/b"}; !reflect.DeepEqual(got, want) {
		t.Errorf("nestedIn(root) = %q, want %q", got, want)
	}
	if got, want := g.nestedIn(mounts, 2), []string{"b"}; !reflect.DeepEqual(got, want) {
		t.Errorf("nestedIn(a) = %q, want %q", got, want)
	}
	if got := g.nestedIn(mounts, 1); got != nil {
		t.Errorf("nestedIn(a/b) = %q, want nil", got)
	}
	want := []mountpoint{{parent: 0, child: 2, rel: "a"}, {parent: 2, child: 1, rel: "b"}}
	if got := g.mountpoints(mounts); !reflect.DeepEqual(got, want) {
		t.Errorf("mountpoints() = %+v, want %+v", got, want)
	}
}

// Two staged mounts of one config root that do not lie inside each other are
// separate groups: neither copy holds the other.
func TestPlanGroupsKeepsUnrelatedDestinationsApart(t *testing.T) {
	const root = "/host/.pi"
	mounts := []plan.ConfigMount{
		{Staged: true, Root: root, DestPath: "/home/caged/.pi", MaskFiles: []string{"a"}},
		{Staged: true, Root: root, DestPath: "/home/caged/.pi2", MaskFiles: []string{"b"}}, // a namesake, not a child
	}
	if got, want := planGroups(mounts), []group{{members: []int{0}}, {members: []int{1}}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("planGroups() = %+v, want %+v", got, want)
	}
}
