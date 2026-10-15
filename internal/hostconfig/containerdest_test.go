package hostconfig

import "testing"

func TestContainerLinkDest(t *testing.T) {
	t.Parallel()

	const linkDest = "/home/caged/.codex/a"
	tests := []struct {
		name  string
		text  string
		roots []string
		want  string
		ok    bool
	}{
		{name: "relative link climbing out of the config dir", text: "../dotfiles/a", want: "/home/caged/dotfiles/a", ok: true},
		{name: "climbing in the middle", text: "a/../b"},
		{name: "leaving the container home", text: "../../../etc/x"},
		{name: "resolving to the container home itself", text: ".."},
		{name: "absolute system path", text: "/usr/bin/x", roots: []string{"/usr"}},
		{name: "absolute path inside a root", text: "/Users/me/dot/a", roots: []string{"/Users/me"}, want: "/Users/me/dot/a", ok: true},
		{name: "absolute path outside every root", text: "/other/a", roots: []string{"/Users/me"}},
		{name: "absolute path under the container home is reserved", text: "/home/caged/x", roots: []string{"/home"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := containerLinkDest(linkDest, tt.text, tt.roots)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("containerLinkDest(%q, %q, %q) = %q, %v; want %q, %v", linkDest, tt.text, tt.roots, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestWithinSlash(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		path, root string
		want       bool
	}{
		{"/home/caged/.claude", "/home/caged", true},
		{"/home/caged", "/home/caged", true},
		{"/home/cagedx", "/home/caged", false},
		{"/anything", "/", true},
		{"/home", "/home/caged", false},
	} {
		if got := withinSlash(tt.path, tt.root); got != tt.want {
			t.Errorf("withinSlash(%q, %q) = %v, want %v", tt.path, tt.root, got, tt.want)
		}
	}
}
