package hostconfig

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/testkit"
)

// TestSymlinkedDirectoryBelowHomeNeedsLinksAllow is Stow's tree folding:
// ~/.config itself is a link into the dotfiles, so every config root below it
// is reached through a symlink and needs a links.allow root.
func TestSymlinkedDirectoryBelowHomeNeedsLinksAllow(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, "dotfiles/config/gh/hosts.yml")
	symlink(t, home, ".config", "dotfiles/config")
	gh := filepath.Join(home, ".config", "gh")

	report := resolveConfigs(t, home, nil)
	if _, ok := mountAt(report, "/home/caged/.config/gh"); ok {
		t.Fatalf("a config root below a symlinked directory was mounted without links.allow: %+v", report.Mounts)
	}
	if refusal := mustRefuse(t, report, gh); refusal.Hint != `add "- path: ~/dotfiles/config/gh" under links.allow in ~/.config/caged/config.yaml` {
		t.Fatalf("Hint = %q", refusal.Hint)
	}

	report = resolveConfigsWith(t, home, nil, policies{allow: []string{filepath.Join(home, "dotfiles")}})
	mount := mustMount(t, report, "/home/caged/.config/gh")
	if mount.HostPath != filepath.Join(resolved, "dotfiles/config/gh") || mount.Link != gh || !mount.Writable {
		t.Fatalf("mount = %+v", mount)
	}
}

func TestReadOnlyRootMakesALinkedConfigRootReadOnly(t *testing.T) {
	t.Parallel()
	home, _ := testHome(t, nil, nil)
	writeFiles(t, home, "dotfiles/claude/settings.json")
	symlink(t, home, ".claude", "dotfiles/claude")

	report := resolveConfigsWith(t, home, nil, policies{allow: []string{filepath.Join(home, "dotfiles")}, readOnly: true})

	if mount := mustMount(t, report, "/home/caged/.claude"); mount.Writable || mount.Link != filepath.Join(home, ".claude") {
		t.Fatalf("a config root linked into a read_only root = %+v, want read-only via the link", mount)
	}
}

func TestUnresolvableSharedFileAndConfigDirAreRefused(t *testing.T) {
	t.Parallel()
	home, _ := testHome(t, []string{".config/caged"}, []string{".gitconfig"})
	gitconfig, caged := filepath.Join(home, ".gitconfig"), filepath.Join(home, ".config", "caged")
	fsys := faultyFS{evalErr: map[string]bool{gitconfig: true, caged: true}}

	report, err := NewResolver(Deps{FS: fsys, Env: testkit.Env{}, Shared: DefaultSharedFiles(), Policies: policies{}, ConfigDirs: fakeDirs(caged)}).Resolve(home)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{gitconfig + " (unresolvable symlink)", caged + " (unresolvable symlink)"}
	if got := refusedStrings(report.Refused); !reflect.DeepEqual(got, want) || len(report.Mounts) != 0 {
		t.Fatalf("Refused = %v, Mounts = %+v; want %v and no mounts", got, report.Mounts, want)
	}
}

// TestSymlinkedHomeKeepsBothFormsOfHome resolves a home that is itself a
// symlink, as macOS temp dirs are (/var -> /private/var): a config root below
// it is not a link, and absolute links in a whole tree may name either form.
func TestSymlinkedHomeKeepsBothFormsOfHome(t *testing.T) {
	t.Parallel()
	dir := homeBesideTheTest(t, "symlinked-home-")
	actual, home := filepath.Join(dir, "real"), filepath.Join(dir, "home")
	if err := os.Mkdir(actual, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real", home); err != nil {
		t.Fatal(err)
	}
	writeFiles(t, home, ".claude/settings.json", ".codex/config.toml")
	symlink(t, home, ".codex/gone-real", filepath.Join(actual, ".codex/missing"))
	symlink(t, home, ".codex/gone-home", filepath.Join(home, ".codex/missing"))

	report := resolveConfigs(t, home, nil)

	if mount := mustMount(t, report, "/home/caged/.claude"); mount.HostPath != filepath.Join(actual, ".claude") || mount.Link != "" {
		t.Fatalf("a config root below a symlinked home = %+v, want it mounted as a direct path", mount)
	}
	// Both links are followed (and found dangling) because their text lies in
	// one form of home; a text outside both would not be followed at all.
	want := []string{
		filepath.Join(actual, ".codex/gone-home") + " (dangling or unresolvable symlink)",
		filepath.Join(actual, ".codex/gone-real") + " (dangling or unresolvable symlink)",
	}
	if got := refusedStrings(report.Refused); !reflect.DeepEqual(got, want) {
		t.Fatalf("Refused = %v, want %v", got, want)
	}
}

// TestPlainAgentRootTouchingABuiltinDenyRootIsRefused pins WB-B2 (bugs.md
// Q-WB-2): an agent config root that is not a symlink is judged by the
// built-in deny list too, as caged's own folder is (B-HC-4). An override
// variable can name ~/.ssh or a folder holding caged's approval state, and the
// approval state can be moved inside an agent's folder; none is mounted.
func TestPlainAgentRootTouchingABuiltinDenyRootIsRefused(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		vars   map[string]string
		root   string // the refused root, relative to home
		state  string // caged's approval state folder, relative to home
		denied string // the deny root the hint names, relative to home
		dest   string // the container path that must not be mounted
		files  []string
	}{
		{name: "CLAUDE_CONFIG_DIR=~/.ssh", vars: map[string]string{"CLAUDE_CONFIG_DIR": "~/.ssh"}, root: ".ssh", state: ".local/state/caged", denied: ".ssh", dest: "/home/caged/.claude", files: []string{".ssh/id_ed25519"}},
		{name: "an override holding the approval state", vars: map[string]string{"CODEX_HOME": "~/.local/state"}, root: ".local/state", state: ".local/state/caged", denied: ".local/state/caged", dest: "/home/caged/.codex", files: []string{".local/state/caged/config-0.approved.yaml"}},
		{name: "approval state moved inside an agent folder", root: ".claude", state: ".claude/caged", denied: ".claude/caged", dest: "/home/caged/.claude", files: []string{".claude/settings.json", ".claude/caged/config-0.approved.yaml"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			home := realHome(t)
			writeFiles(t, home, tc.files...)
			vars := map[string]string{}
			for key, value := range tc.vars {
				vars[key] = filepath.Join(home, value[2:])
			}
			report := resolveConfigsWith(t, home, vars, policies{own: []string{filepath.Join(home, ".config", "caged"), filepath.Join(home, tc.state)}})
			want := plan.Refusal{Path: filepath.Join(home, tc.root), Reason: "config path is denied", Hint: "~/" + tc.denied + " is on caged's built-in deny list and cannot be allowed"}
			if refusal, ok := refusalAt(report, want.Path); !ok || refusal != want {
				t.Fatalf("refusal = %+v, %v; want %+v", refusal, ok, want)
			}
			if mount, ok := mountAt(report, tc.dest); ok {
				t.Fatalf("mount = %+v; want nothing mounted at %s", mount, tc.dest)
			}
		})
	}
}

// TestPlainAgentRootIsJudgedByTheBuiltinDenyListOnly: links.deny is about
// links, so a default agent folder it names is still mounted, as caged's own
// folder is; the default folders touch no built-in deny root.
func TestPlainAgentRootIsJudgedByTheBuiltinDenyListOnly(t *testing.T) {
	t.Parallel()

	home := realHome(t)
	writeFiles(t, home, ".claude/settings.json", ".codex/config.toml", ".config/gh/hosts.yml", ".gitconfig")
	report := resolveConfigsWith(t, home, nil, policies{deny: []string{filepath.Join(home, ".claude")}, own: []string{filepath.Join(home, ".config", "caged"), filepath.Join(home, ".local", "state", "caged")}})
	for _, dest := range []string{"/home/caged/.claude", "/home/caged/.codex", "/home/caged/.config/gh", "/home/caged/.gitconfig"} {
		if _, ok := mountAt(report, dest); !ok {
			t.Errorf("nothing mounted at %s; refused = %v", dest, refusedStrings(report.Refused))
		}
	}
	if len(report.Refused) != 0 {
		t.Errorf("refused = %v; want none", refusedStrings(report.Refused))
	}
}
