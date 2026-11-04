package userconfig

import (
	"errors"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/pathpolicy"
	"github.com/JeremyOrr4/caged/internal/testkit"
)

func TestLinkPolicy(t *testing.T) {
	t.Parallel()

	t.Run("without a file only the built-in deny list applies", func(t *testing.T) {
		t.Parallel()
		policy, err := NewStore(&fakeFS{}, testkit.Env{Home: "/home/me"}).LinkPolicy("/home/me")
		if err != nil || policy == nil {
			t.Fatalf("LinkPolicy() = %v, %v; want a policy", policy, err)
		}
		if got := policy.Check("/home/me/dot/a").Verdict; got != pathpolicy.NotAllowed {
			t.Fatalf("Check(/home/me/dot/a) = %d, want NotAllowed", got)
		}
		if got := policy.Check("/home/me/.ssh/id").Verdict; got != pathpolicy.Denied {
			t.Fatalf("Check(/home/me/.ssh/id) = %d, want Denied", got)
		}
		const want = `add "- path: ~/dot" under links.allow in ~/.config/caged/config.yaml`
		if got := policy.Remedy(policy.Check("/home/me/dot/a"), "/home/me/dot/a", false); got != want {
			t.Fatalf("Remedy() = %q, want %q: the hint names the file to create", got, want)
		}
	})
	t.Run("links.allow and links.deny shape the policy", func(t *testing.T) {
		t.Parallel()
		store, _ := storeWith("links:\n  allow:\n    - path: ~/dot\n      read_only: true\n  deny: [~/dot/private]\n")
		policy, err := store.LinkPolicy("/home/me")
		if err != nil {
			t.Fatal(err)
		}
		if decision := policy.Check("/home/me/dot/pi"); decision.Verdict != pathpolicy.Allowed || !decision.Root.ReadOnly {
			t.Fatalf("Check(/home/me/dot/pi) = %+v, want Allowed and read-only", decision)
		}
		if got := policy.Check("/home/me/dot/private/x").Verdict; got != pathpolicy.Denied {
			t.Fatalf("Check(/home/me/dot/private/x) = %d, want Denied", got)
		}
	})
	t.Run("an allow root of ~ is too broad", func(t *testing.T) {
		t.Parallel()
		store, _ := storeWith("links:\n  allow:\n    - path: \"~\"\n")
		_, err := store.LinkPolicy("/home/me")
		if err == nil || !strings.Contains(err.Error(), "too broad") {
			t.Fatalf("LinkPolicy() error = %v, want too broad", err)
		}
	})
	t.Run("an invalid file is an error", func(t *testing.T) {
		t.Parallel()
		store, _ := storeWith("image: [\n")
		if _, err := store.LinkPolicy("/home/me"); err == nil {
			t.Fatal("LinkPolicy() succeeded on an invalid file")
		}
	})

	const body = "links:\n  allow:\n    - path: ~/dot\n      read_only: true\n  deny: [~/secret]\n"
	t.Run("every root also matches its symlink-resolved form", func(t *testing.T) {
		t.Parallel()
		store, fsys := storeWith(body)
		fsys.links = map[string]string{"/home/me/dot": "/srv/dot", "/home/me/secret": "/vault", "/home/me/.ssh": "/data/ssh"}
		policy, err := store.LinkPolicy("/home/me")
		if err != nil {
			t.Fatal(err)
		}
		if decision := policy.Check("/srv/dot/pi"); decision.Verdict != pathpolicy.Allowed || !decision.Root.ReadOnly {
			t.Errorf("Check(/srv/dot/pi) = %+v, want Allowed and read-only through the resolved allow root", decision)
		}
		if got := policy.Check("/vault/x").Verdict; got != pathpolicy.Denied {
			t.Errorf("Check(/vault/x) = %d, want Denied through the resolved links.deny root", got)
		}
		if decision := policy.Check("/data/ssh/id"); decision.Verdict != pathpolicy.Denied || !decision.Root.Builtin {
			t.Errorf("Check(/data/ssh/id) = %+v, want Denied by the resolved built-in root", decision)
		}
	})
	t.Run("a root that cannot be resolved is matched as written", func(t *testing.T) {
		t.Parallel()
		store, fsys := storeWith(body)
		fsys.evalErr = errors.New("permission denied")
		policy, err := store.LinkPolicy("/home/me")
		if err != nil {
			t.Fatalf("LinkPolicy() error = %v; want the unresolved roots kept as written", err)
		}
		if got := policy.Check("/home/me/dot/pi").Verdict; got != pathpolicy.Allowed {
			t.Errorf("Check(/home/me/dot/pi) = %d, want Allowed", got)
		}
		if got := policy.Check("/home/me/.ssh/id").Verdict; got != pathpolicy.Denied {
			t.Errorf("Check(/home/me/.ssh/id) = %d, want Denied", got)
		}
		if got := policy.Check("/home/me/secret/x").Verdict; got != pathpolicy.Denied {
			t.Errorf("Check(/home/me/secret/x) = %d, want Denied", got)
		}
	})
	t.Run("an allow root that contains the resolved home is too broad", func(t *testing.T) {
		t.Parallel()
		store, fsys := storeWith("links:\n  allow:\n    - path: /data\n")
		fsys.links = map[string]string{"/home/me": "/data/me"}
		_, err := store.LinkPolicy("/home/me")
		if err == nil || err.Error() != `~/.config/caged/config.yaml: links.allow entry "/data" is too broad: it contains the home directory` {
			t.Fatalf("LinkPolicy() error = %v, want /data refused as too broad", err)
		}
	})
}

// TestLinkPolicyKeepsDenyRootsWhenNothingResolves: the walk up to a
// resolvable parent ends at the root, and the entry is still denied by name.
func TestLinkPolicyKeepsDenyRootsWhenNothingResolves(t *testing.T) {
	t.Parallel()

	policy, err := NewStore(&fakeFS{evalErr: errors.New("eval failed")}, testkit.Env{Home: "/home/me"}).LinkPolicy("/home/me")
	if err != nil {
		t.Fatal(err)
	}
	if decision := policy.Check("/home/me/.local/share"); decision.Verdict != pathpolicy.Denied || decision.Root.Path != "/home/me/.local/share/keyrings" {
		t.Fatalf("Check(~/.local/share) = %+v; want it denied by ~/.local/share/keyrings", decision)
	}
}

// TestLinkPolicyDeniesCagedsOwnFolders pins WB-T1 (bugs.md Q-F03, Q-UC-3):
// the config folder Dir mounts and the folder that keeps the approved copy are
// on the built-in deny list wherever XDG_CONFIG_HOME and XDG_STATE_HOME put
// them, and links.allow cannot name them. ~/.config/caged stays denied too.
func TestLinkPolicyDeniesCagedsOwnFolders(t *testing.T) {
	t.Parallel()

	moved := testkit.Env{Home: "/home/me", Vars: map[string]string{"XDG_CONFIG_HOME": "/xdg", "XDG_STATE_HOME": "/state"}}
	for _, tc := range []struct {
		name   string
		env    testkit.Env
		denied []string
	}{
		{name: "default folders", env: testkit.Env{Home: "/home/me"}, denied: []string{"/home/me/.config/caged/config.yaml", "/home/me/.local/state/caged/config-0.approved.yaml"}},
		{name: "folders moved by XDG", env: moved, denied: []string{"/xdg/caged/config.yaml", "/state/caged/config-0.approved.yaml", "/home/me/.config/caged/config.yaml"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			policy, err := NewStore(&fakeFS{}, tc.env).LinkPolicy("/home/me")
			if err != nil {
				t.Fatal(err)
			}
			for _, path := range tc.denied {
				if decision := policy.Check(path); decision.Verdict != pathpolicy.Denied || !decision.Root.Builtin {
					t.Errorf("Check(%s) = %+v, want Denied by a built-in root", path, decision)
				}
			}
			if got := policy.Check("/home/me/.local/state/other").Verdict; got != pathpolicy.NotAllowed {
				t.Errorf("Check(~/.local/state/other) = %d, want NotAllowed: only caged's own state folder is denied", got)
			}
		})
	}
	for _, root := range []string{"/xdg/caged", "/state/caged/sub"} {
		t.Run("links.allow may not name "+root, func(t *testing.T) {
			t.Parallel()
			fsys := &fakeFS{files: map[string]string{"/xdg/caged/config.yaml": "links:\n  allow:\n    - path: " + root + "\n"}}
			_, err := NewStore(fsys, moved).LinkPolicy("/home/me")
			if err == nil || !strings.Contains(err.Error(), "is inside the denied path") {
				t.Fatalf("LinkPolicy() error = %v, want %s refused as inside a denied path", err, root)
			}
		})
	}
	t.Run("a home that cannot be found is an error", func(t *testing.T) {
		t.Parallel()
		_, err := NewStore(&fakeFS{}, testkit.Env{Vars: map[string]string{"XDG_CONFIG_HOME": "/xdg"}, HomeErr: errors.New("no home")}).LinkPolicy("/home/me")
		if err == nil || !strings.Contains(err.Error(), "no home") {
			t.Fatalf("LinkPolicy() error = %v, want the home lookup failure", err)
		}
	})
}

func TestWorkdirPolicy(t *testing.T) {
	t.Parallel()

	t.Run("nil without a workdirs key", func(t *testing.T) {
		t.Parallel()
		store, _ := storeWith("image:\n  apt: [tmux]\n")
		policy, err := store.WorkdirPolicy("/home/me")
		if policy != nil || err != nil {
			t.Fatalf("WorkdirPolicy() = %v, %v; want nil, nil", policy, err)
		}
	})
	t.Run("nil without a file", func(t *testing.T) {
		t.Parallel()
		policy, err := NewStore(&fakeFS{}, testkit.Env{Home: "/home/me"}).WorkdirPolicy("/home/me")
		if policy != nil || err != nil {
			t.Fatalf("WorkdirPolicy() = %v, %v; want nil, nil", policy, err)
		}
	})
	t.Run("an empty allow list refuses everything", func(t *testing.T) {
		t.Parallel()
		store, _ := storeWith("workdirs:\n  allow: []\n")
		policy, err := store.WorkdirPolicy("/home/me")
		if err != nil || policy == nil {
			t.Fatalf("WorkdirPolicy() = %v, %v; want a policy", policy, err)
		}
		if got := policy.Check("/home/me/git/app").Verdict; got != pathpolicy.NotAllowed {
			t.Fatalf("Check() = %d, want NotAllowed", got)
		}
	})
	t.Run("allows the listed roots", func(t *testing.T) {
		t.Parallel()
		store, _ := storeWith("workdirs:\n  allow:\n    - path: ~/git\n")
		policy, err := store.WorkdirPolicy("/home/me")
		if err != nil {
			t.Fatal(err)
		}
		if got := policy.Check("/home/me/git/app").Verdict; got != pathpolicy.Allowed {
			t.Fatalf("Check(/home/me/git/app) = %d, want Allowed", got)
		}
		if got := policy.Check("/home/me/other").Verdict; got != pathpolicy.NotAllowed {
			t.Fatalf("Check(/home/me/other) = %d, want NotAllowed", got)
		}
		const want = `add "- path: ~/other" under workdirs.allow in ~/.config/caged/config.yaml`
		if got := policy.Remedy(policy.Check("/home/me/other"), "/home/me/other", true); got != want {
			t.Fatalf("Remedy() = %q, want %q", got, want)
		}
	})
	t.Run("a bare workdirs key refuses everything", func(t *testing.T) {
		t.Parallel()
		store, _ := storeWith("workdirs:\n")
		policy, err := store.WorkdirPolicy("/home/me")
		if err != nil || policy == nil {
			t.Fatalf("WorkdirPolicy() = %v, %v; want a policy", policy, err)
		}
		if got := policy.Check("/home/me/git/app").Verdict; got != pathpolicy.NotAllowed {
			t.Fatalf("Check() = %d, want NotAllowed", got)
		}
		if status, err := store.Status(); err != nil || !status.Workdirs || status.WorkdirAllow != 0 {
			t.Fatalf("Status() = %+v, %v; want workdirs set with no roots", status, err)
		}
	})
	t.Run("every root also matches its symlink-resolved form", func(t *testing.T) {
		t.Parallel()
		store, fsys := storeWith("workdirs:\n  allow:\n    - path: ~/git\n")
		fsys.links = map[string]string{"/home/me/git": "/srv/git"}
		policy, err := store.WorkdirPolicy("/home/me")
		if err != nil {
			t.Fatal(err)
		}
		if got := policy.Check("/srv/git/app").Verdict; got != pathpolicy.Allowed {
			t.Errorf("Check(/srv/git/app) = %d, want Allowed through the resolved workdirs root", got)
		}
	})
}
