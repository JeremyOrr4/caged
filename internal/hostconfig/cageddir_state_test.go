package hostconfig

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/pathpolicy"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
	"github.com/JeremyOrr4/caged/internal/testkit"
)

// TestApprovalStateDirIsOnTheBuiltinDenyList pins WB-T1 (bugs.md Q-UC-3): a
// link from agent config into the folder that keeps caged's approved copy is
// refused, even when links.allow covers it, and links.allow cannot name it.
func TestApprovalStateDirIsOnTheBuiltinDenyList(t *testing.T) {
	t.Parallel()

	home := realHome(t)
	state := filepath.Join(home, ".local", "state", "caged")
	writeFiles(t, home, ".local/state/caged/approved.yaml", ".claude/settings.json")
	symlink(t, home, ".claude/approved.yaml", filepath.Join(state, "approved.yaml"))

	report := resolveConfigsWith(t, home, nil, policies{allow: []string{filepath.Join(home, ".local")}, own: []string{state}})

	link := filepath.Join(home, ".claude", "approved.yaml")
	if refusal, ok := refusalAt(report, link); !ok || refusal.Reason != "symlink target ~/.local/state/caged/approved.yaml is denied" ||
		refusal.Hint != "~/.local/state/caged is on caged's built-in deny list and cannot be allowed" {
		t.Fatalf("refusal = %+v, %v; want the approval copy denied", refusal, ok)
	}
	if mount, ok := mountAt(report, "/home/caged/.claude/approved.yaml"); ok {
		t.Fatalf("mount = %+v; want no mount of the approval copy", mount)
	}
	if _, err := (policies{allow: []string{state}, own: []string{state}}).LinkPolicy(home); err == nil || !strings.Contains(err.Error(), "is inside the denied path") {
		t.Fatalf("LinkPolicy() error = %v; want links.allow naming the state folder refused", err)
	}
}

// stateDirs reports caged's config folder and its approval-state folder, as
// userconfig.Store does.
type stateDirs struct{ dir, state string }

func (d stateDirs) Dir() (string, error) { return d.dir, nil }

func (d stateDirs) StateDir() (string, error) { return d.state, nil }

// TestCagedDirSharingTheApprovalStateIsRefused pins WB-B1 (bugs.md Q-WB-1):
// the read/write caged mount must never hold the approved copies, or an agent
// could rewrite both copies to match. XDG_CONFIG_HOME equal to
// XDG_STATE_HOME makes the two folders one; XDG_STATE_HOME=~/.config does it
// with the default config folder; and either folder can sit inside the other.
func TestCagedDirSharingTheApprovalStateIsRefused(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		dir, state func(home, xdg string) string
	}{
		{"XDG_CONFIG_HOME equal to XDG_STATE_HOME", func(_, xdg string) string { return filepath.Join(xdg, "caged") }, func(_, xdg string) string { return filepath.Join(xdg, "caged") }},
		{"XDG_STATE_HOME=~/.config", func(home, _ string) string { return filepath.Join(home, ".config", "caged") }, func(home, _ string) string { return filepath.Join(home, ".config", "caged") }},
		{"state folder inside the config folder", func(_, xdg string) string { return filepath.Join(xdg, "caged") }, func(_, xdg string) string { return filepath.Join(xdg, "caged", "s", "caged") }},
		{"config folder inside the state folder", func(_, xdg string) string { return filepath.Join(xdg, "caged", "c", "caged") }, func(_, xdg string) string { return filepath.Join(xdg, "caged") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			home, xdg := realHome(t), realHome(t)
			dir, state := tc.dir(home, xdg), tc.state(home, xdg)
			writeFiles(t, dir, "config.yaml")
			writeFiles(t, state, "config-0.approved.yaml")
			links := policies{own: []string{dir, state}}
			report, err := NewResolver(Deps{FS: system.FS{}, Env: testkit.Env{}, Agents: agent.Builtin().All(), Shared: DefaultSharedFiles(), Policies: links, ConfigDirs: stateDirs{dir, state}}).Resolve(home)
			if err != nil {
				t.Fatal(err)
			}
			want := plan.Refusal{Path: dir, Reason: "config path overlaps caged's approval state folder " + pathpolicy.Tilde(home, state), Hint: "set XDG_CONFIG_HOME and XDG_STATE_HOME to different folders"}
			if refusal, ok := refusalAt(report, dir); !ok || refusal != want {
				t.Fatalf("refusal = %+v, %v; want %+v", refusal, ok, want)
			}
			if mount, ok := mountAt(report, cagedDest); ok {
				t.Fatalf("mount = %+v; want no mount holding the approved copies", mount)
			}
		})
	}
}

// TestCagedDirBesideTheApprovalStateIsMounted: folders that only share a
// parent, as the defaults ~/.config/caged and ~/.local/state/caged do, or a
// name prefix, are no overlap.
func TestCagedDirBesideTheApprovalStateIsMounted(t *testing.T) {
	t.Parallel()

	xdg := realHome(t)
	home := realHome(t)
	dir := filepath.Join(xdg, "caged")
	state := filepath.Join(xdg, "caged-state", "caged")
	writeFiles(t, dir, "config.yaml")
	report, err := NewResolver(Deps{FS: system.FS{}, Env: testkit.Env{}, Agents: agent.Builtin().All(), Shared: DefaultSharedFiles(), Policies: policies{own: []string{dir, state}}, ConfigDirs: stateDirs{dir, state}}).Resolve(home)
	if err != nil {
		t.Fatal(err)
	}
	if mount, ok := mountAt(report, cagedDest); !ok || mount.HostPath != dir || len(report.Refused) != 0 {
		t.Fatalf("mounts = %+v, refused = %v; want %s mounted", report.Mounts, refusedStrings(report.Refused), dir)
	}
}

// TestCagedDirReachingTheApprovalStateThroughALinkIsRefused: a Stow link at
// the caged folder that resolves into, or around, the state folder is
// refused by the same rule, judged on the resolved folder.
func TestCagedDirReachingTheApprovalStateThroughALinkIsRefused(t *testing.T) {
	t.Parallel()

	home := realHome(t)
	state := filepath.Join(home, "dotfiles", "state", "caged")
	writeFiles(t, state, "config.yaml")
	dir := filepath.Join(home, ".config", "caged")
	symlink(t, home, ".config/caged", state)
	report, err := NewResolver(Deps{FS: system.FS{}, Env: testkit.Env{}, Agents: agent.Builtin().All(), Shared: DefaultSharedFiles(), Policies: policies{allow: []string{filepath.Join(home, "dotfiles")}, own: []string{dir, state}}, ConfigDirs: stateDirs{dir, state}}).Resolve(home)
	if err != nil {
		t.Fatal(err)
	}
	if refusal, ok := refusalAt(report, dir); !ok || refusal.Reason != "config path overlaps caged's approval state folder ~/dotfiles/state/caged" {
		t.Fatalf("refusal = %+v, %v; want the linked folder refused", refusal, ok)
	}
	if mount, ok := mountAt(report, cagedDest); ok {
		t.Fatalf("mount = %+v; want no mount holding the approved copies", mount)
	}
}

// TestApprovalStateReachingTheCagedDirIsRefused: the state folder is
// compared as named and resolved too, so a state folder that is a link into
// the caged folder is caught, and so is one not created yet whose name is
// inside the linked caged folder.
func TestApprovalStateReachingTheCagedDirIsRefused(t *testing.T) {
	t.Parallel()

	t.Run("state folder linked into the caged folder", func(t *testing.T) {
		t.Parallel()
		home := realHome(t)
		dir := filepath.Join(home, ".config", "caged")
		writeFiles(t, dir, "config.yaml", "approved/config-0.approved.yaml")
		state := filepath.Join(home, ".local", "state", "caged")
		symlink(t, home, ".local/state/caged", filepath.Join(dir, "approved"))
		report, err := NewResolver(Deps{FS: system.FS{}, Env: testkit.Env{}, Agents: agent.Builtin().All(), Shared: DefaultSharedFiles(), Policies: policies{own: []string{dir, state}}, ConfigDirs: stateDirs{dir, state}}).Resolve(home)
		if err != nil {
			t.Fatal(err)
		}
		if refusal, ok := refusalAt(report, dir); !ok || refusal.Reason != "config path overlaps caged's approval state folder ~/.local/state/caged" {
			t.Fatalf("refusal = %+v, %v; want the caged folder refused", refusal, ok)
		}
	})
	t.Run("state folder not created yet inside the linked caged folder", func(t *testing.T) {
		t.Parallel()
		home := realHome(t)
		writeFiles(t, home, "dotfiles/caged/config.yaml")
		dir := filepath.Join(home, ".config", "caged")
		symlink(t, home, ".config/caged", filepath.Join(home, "dotfiles", "caged"))
		state := filepath.Join(dir, "s", "caged") // XDG_STATE_HOME=~/.config/caged/s
		report, err := NewResolver(Deps{FS: system.FS{}, Env: testkit.Env{}, Agents: agent.Builtin().All(), Shared: DefaultSharedFiles(), Policies: policies{allow: []string{filepath.Join(home, "dotfiles")}, own: []string{dir, state}}, ConfigDirs: stateDirs{dir, state}}).Resolve(home)
		if err != nil {
			t.Fatal(err)
		}
		if refusal, ok := refusalAt(report, dir); !ok || refusal.Reason != "config path overlaps caged's approval state folder ~/.config/caged/s/caged" {
			t.Fatalf("refusal = %+v, %v; want the caged folder refused", refusal, ok)
		}
	})
}
