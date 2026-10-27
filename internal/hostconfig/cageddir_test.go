package hostconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/pathpolicy"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
	"github.com/JeremyOrr4/caged/internal/testkit"
)

const cagedDest = "/home/caged/.config/caged"

type fakeDirs string

func (d fakeDirs) Dir() (string, error) { return string(d), nil }

// StateDir returns an approval state folder no test folder overlaps.
func (fakeDirs) StateDir() (string, error) { return "/nonexistent/caged-state", nil }

// failingDirs fails with err.
type failingDirs struct{ err error }

func (f failingDirs) Dir() (string, error)      { return "", f.err }
func (f failingDirs) StateDir() (string, error) { return "", f.err }

// failingStateDirs reports dir and fails to report the state folder with err.
type failingStateDirs struct {
	dir string
	err error
}

func (f failingStateDirs) Dir() (string, error)      { return f.dir, nil }
func (f failingStateDirs) StateDir() (string, error) { return "", f.err }

// resolveWithCagedDir resolves home's config with caged's config directory at
// dir. Like userconfig.Store, which supplies both, the link policy denies dir.
func resolveWithCagedDir(t *testing.T, home, dir string, links policies) plan.ConfigReport {
	t.Helper()
	links.own = append(append([]string(nil), links.own...), dir)
	report, err := NewResolver(Deps{FS: system.FS{}, Env: testkit.Env{}, Agents: agent.Builtin().All(), Shared: DefaultSharedFiles(), Policies: links, ConfigDirs: fakeDirs(dir)}).Resolve(home)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

// realHome returns a temp home with its symlinks resolved, so expected paths
// match what the resolver reports on every platform.
func realHome(t *testing.T) string {
	t.Helper()
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return home
}

func TestConfigDirIsMountedWritableAtTheContainerConfigPath(t *testing.T) {
	t.Parallel()

	home := realHome(t)
	dir := filepath.Join(home, ".config", "caged")
	writeFiles(t, home, ".config/caged/config.yaml")
	report := resolveWithCagedDir(t, home, dir, policies{})
	mount, ok := mountAt(report, cagedDest)
	if !ok {
		t.Fatalf("mounts = %+v; want one at %s", report.Mounts, cagedDest)
	}
	if mount.HostPath != dir || !mount.Writable || mount.Agent != plan.SharedOwner || mount.Link != "" {
		t.Errorf("mount = %+v; want %s read/write, shared, not via a link", mount, dir)
	}
	if len(mount.MaskPaths)+len(mount.MaskFiles) != 0 {
		t.Errorf("mount = %+v; want the whole directory with no masks", mount)
	}
	if len(report.Refused) != 0 {
		t.Errorf("refused = %+v; want none", report.Refused)
	}
}

func TestConfigDirOutsideHomeFromAbsoluteXDGIsMounted(t *testing.T) {
	t.Parallel()

	home := realHome(t)
	xdg := realHome(t)
	dir := filepath.Join(xdg, "caged")
	writeFiles(t, xdg, "caged/config.yaml")
	report := resolveWithCagedDir(t, home, dir, policies{})
	mount, ok := mountAt(report, cagedDest)
	if !ok || mount.HostPath != dir || !mount.Writable {
		t.Fatalf("mounts = %+v, refused = %+v; want %s read/write at %s", report.Mounts, report.Refused, dir, cagedDest)
	}
}

func TestMissingConfigDirIsReportedNotMounted(t *testing.T) {
	t.Parallel()

	home := realHome(t)
	dir := filepath.Join(home, ".config", "caged")
	report := resolveWithCagedDir(t, home, dir, policies{})
	if _, ok := mountAt(report, cagedDest); ok {
		t.Error("mounted a directory that does not exist")
	}
	found := false
	for _, missing := range report.Missing {
		found = found || missing == dir
	}
	if !found {
		t.Errorf("missing = %v; want %s", report.Missing, dir)
	}
}

func TestNoConfigDirOptionMeansNoCagedMount(t *testing.T) {
	t.Parallel()

	home := realHome(t)
	writeFiles(t, home, ".config/caged/config.yaml")
	report := resolveConfigs(t, home, nil)
	if _, ok := mountAt(report, cagedDest); ok {
		t.Errorf("mounts = %+v; want no caged mount without ConfigDirs", report.Mounts)
	}
}

func TestSymlinkedConfigDirNeedsLinksAllow(t *testing.T) {
	t.Parallel()

	home := realHome(t)
	writeFiles(t, home, "dotfiles/caged/config.yaml")
	symlink(t, home, ".config/caged", filepath.Join(home, "dotfiles", "caged"))
	dir := filepath.Join(home, ".config", "caged")

	report := resolveWithCagedDir(t, home, dir, policies{})
	if _, ok := mountAt(report, cagedDest); ok {
		t.Fatalf("mounts = %+v; want the symlinked directory refused without links.allow", report.Mounts)
	}
	// The hint names the target's folder: links.allow cannot name the target
	// itself, a form of the denied ~/.config/caged (bugs.md B-HC-3).
	refusal, ok := refusalAt(report, dir)
	want := plan.Refusal{Path: dir, Reason: "symlink target ~/dotfiles/caged is not under links.allow", Hint: `add "- path: ~/dotfiles" under links.allow in ~/.config/caged/config.yaml`}
	if !ok || refusal != want {
		t.Fatalf("refusal = %+v, %v; want %+v", refusal, ok, want)
	}

	report = resolveWithCagedDir(t, home, dir, policies{allow: []string{filepath.Join(home, "dotfiles")}})
	mount, ok := mountAt(report, cagedDest)
	if !ok || mount.HostPath != filepath.Join(home, "dotfiles", "caged") || mount.Link != dir || !mount.Writable {
		t.Fatalf("mounts = %+v, refused = %+v; want the link target mounted read/write via %s", report.Mounts, report.Refused, dir)
	}
}

func TestSymlinkedConfigDirHonorsReadOnlyAllowRoots(t *testing.T) {
	t.Parallel()

	home := realHome(t)
	writeFiles(t, home, "dotfiles/caged/config.yaml")
	symlink(t, home, ".config/caged", filepath.Join(home, "dotfiles", "caged"))
	report := resolveWithCagedDir(t, home, filepath.Join(home, ".config", "caged"), policies{allow: []string{filepath.Join(home, "dotfiles")}, readOnly: true})
	mount, ok := mountAt(report, cagedDest)
	if !ok || mount.Writable {
		t.Fatalf("mounts = %+v; want the mount read-only through a read_only root", report.Mounts)
	}
}

func TestSymlinkedConfigDirIntoADeniedPathIsStillRefused(t *testing.T) {
	t.Parallel()

	home := realHome(t)
	writeFiles(t, home, ".ssh/caged/config.yaml")
	symlink(t, home, ".config/caged", filepath.Join(home, ".ssh", "caged"))
	dir := filepath.Join(home, ".config", "caged")
	report := resolveWithCagedDir(t, home, dir, policies{})
	if _, ok := mountAt(report, cagedDest); ok {
		t.Fatalf("mounts = %+v; want a link into ~/.ssh refused", report.Mounts)
	}
	if refusal, ok := refusalAt(report, dir); !ok || !strings.Contains(refusal.Reason, "denied") {
		t.Errorf("refusal = %+v, %v; want a denied reason", refusal, ok)
	}
}

func TestAgentConfigStillCannotLinkIntoTheCagedConfigDir(t *testing.T) {
	t.Parallel()

	home := realHome(t)
	writeFiles(t, home, ".config/caged/config.yaml", ".claude/settings.json")
	symlink(t, home, ".claude/policy", filepath.Join(home, ".config", "caged"))
	report := resolveWithCagedDir(t, home, filepath.Join(home, ".config", "caged"), policies{allow: []string{filepath.Join(home, ".config")}})
	link := filepath.Join(home, ".claude", "policy")
	refusal, ok := refusalAt(report, link)
	if !ok || !strings.Contains(refusal.Reason, "denied") {
		t.Fatalf("refusal = %+v, %v; want the built-in deny to refuse the agent's link", refusal, ok)
	}
	if mount, ok := mountAt(report, "/home/caged/.claude/policy"); ok {
		t.Errorf("mount = %+v; want none for a link into the caged config dir", mount)
	}
}

func TestSymlinkedConfigFileIsFollowedOnlyWhenAllowed(t *testing.T) {
	t.Parallel()

	home := realHome(t)
	writeFiles(t, home, "dotfiles/caged.yaml")
	if err := os.MkdirAll(filepath.Join(home, ".config", "caged"), 0o700); err != nil {
		t.Fatal(err)
	}
	symlink(t, home, ".config/caged/config.yaml", filepath.Join(home, "dotfiles", "caged.yaml"))
	dir := filepath.Join(home, ".config", "caged")
	link := filepath.Join(dir, "config.yaml")

	report := resolveWithCagedDir(t, home, dir, policies{})
	if _, ok := mountAt(report, cagedDest+"/config.yaml"); ok {
		t.Fatalf("mounts = %+v; want the unallowed file link not followed", report.Mounts)
	}
	if refusal, ok := refusalAt(report, link); !ok || !strings.HasPrefix(refusal.Hint, wantHintPrefix) {
		t.Fatalf("refusal = %+v, %v; want a hint to allow the file's folder", refusal, ok)
	}

	report = resolveWithCagedDir(t, home, dir, policies{allow: []string{filepath.Join(home, "dotfiles")}})
	mount, ok := mountAt(report, cagedDest+"/config.yaml")
	if !ok || mount.HostPath != filepath.Join(home, "dotfiles", "caged.yaml") || !mount.Writable {
		t.Fatalf("mounts = %+v, refused = %+v; want the allowed file mounted over config.yaml", report.Mounts, report.Refused)
	}
	if _, ok := mountAt(report, cagedDest); ok {
		t.Error("the whole directory was mounted even though it holds a symlink")
	}
}

// TestCagedDirMovedByXDGIsOnTheBuiltinDenyList pins WB-T1 (bugs.md Q-F03):
// when an absolute XDG_CONFIG_HOME moves caged's folder, inside or outside
// home, a link from agent config into it is refused like one into
// ~/.config/caged (TestAgentConfigStillCannotLinkIntoTheCagedConfigDir), even
// when links.allow covers its parent, and the folder itself is still mounted.
func TestCagedDirMovedByXDGIsOnTheBuiltinDenyList(t *testing.T) {
	t.Parallel()

	for _, outside := range []bool{false, true} {
		t.Run(fmt.Sprintf("outside home=%v", outside), func(t *testing.T) {
			t.Parallel()
			home := realHome(t)
			xdg := filepath.Join(home, "xdg")
			if outside {
				xdg = realHome(t)
			}
			dir := filepath.Join(xdg, "caged")
			writeFiles(t, dir, "config.yaml")
			writeFiles(t, home, ".claude/settings.json")
			symlink(t, home, ".claude/policy.yaml", filepath.Join(dir, "config.yaml"))

			report := resolveWithCagedDir(t, home, dir, policies{allow: []string{xdg}})

			link := filepath.Join(home, ".claude", "policy.yaml")
			want := []string{fmt.Sprintf("%s (symlink target %s is denied); %s is on caged's built-in deny list and cannot be allowed", link, pathpolicy.Tilde(home, filepath.Join(dir, "config.yaml")), pathpolicy.Tilde(home, dir))}
			if got := refusedStrings(report.Refused); !reflect.DeepEqual(got, want) {
				t.Fatalf("refused = %q; want %q", got, want)
			}
			if mount, ok := mountAt(report, "/home/caged/.claude/policy.yaml"); ok {
				t.Fatalf("mount = %+v; want no mount of the caged folder through an agent link", mount)
			}
			if mount := mustMount(t, report, cagedDest); mount.HostPath != dir || !mount.Writable {
				t.Fatalf("mount = %+v; want the moved caged folder mounted read/write", mount)
			}
		})
	}
}

// TestCagedDirMovedByXDGOntoAStowLinkedDefaultIsMounted: when XDG_CONFIG_HOME
// moves caged's folder to a link into dotfiles and ~/.config/caged links to
// the same target, the default entry denies that target through its resolved
// form, so cagedDir sets aside both entries (D-26).
func TestCagedDirMovedByXDGOntoAStowLinkedDefaultIsMounted(t *testing.T) {
	t.Parallel()

	home := realHome(t)
	target := filepath.Join(home, "dotfiles", "caged")
	writeFiles(t, target, "config.yaml")
	symlink(t, home, ".config/caged", target)
	xdg := realHome(t)
	symlink(t, xdg, "caged", target)
	dir := filepath.Join(xdg, "caged")

	report := resolveWithCagedDir(t, home, dir, policies{allow: []string{filepath.Join(home, "dotfiles")}})
	mount, ok := mountAt(report, cagedDest)
	if !ok || mount.HostPath != target || mount.Link != dir || !mount.Writable {
		t.Fatalf("mounts = %+v, refused = %v; want %s mounted read/write via %s", report.Mounts, refusedStrings(report.Refused), target, dir)
	}
}

// TestLinksInsideCagedDirAreJudgedByItsOwnPolicy pins which policy judges a
// link inside caged's folder: with the folder reached through a link, its own
// deny entries are set aside, so a link to the folder's parent is refused as
// a directory that contains it; with a plain folder the full policy applies,
// and the same link is denied because the parent holds caged's folder.
func TestLinksInsideCagedDirAreJudgedByItsOwnPolicy(t *testing.T) {
	t.Parallel()

	t.Run("linked folder", func(t *testing.T) {
		t.Parallel()
		home := realHome(t)
		target := filepath.Join(home, "dotfiles", "caged")
		writeFiles(t, target, "config.yaml")
		symlink(t, home, "dotfiles/caged/up", filepath.Join(home, "dotfiles"))
		symlink(t, home, ".config/caged", target)
		report := resolveWithCagedDir(t, home, filepath.Join(home, ".config", "caged"), policies{allow: []string{filepath.Join(home, "dotfiles")}})
		want := plan.Refusal{Path: filepath.Join(target, "up"), Reason: "symlink points to a directory that contains it"}
		if refusal, ok := refusalAt(report, want.Path); !ok || refusal != want {
			t.Fatalf("refusal = %+v, %v; want %+v", refusal, ok, want)
		}
	})
	t.Run("plain folder", func(t *testing.T) {
		t.Parallel()
		home := realHome(t)
		xdg := realHome(t)
		dir := filepath.Join(xdg, "caged")
		writeFiles(t, dir, "config.yaml")
		symlink(t, xdg, "caged/up", xdg)
		report := resolveWithCagedDir(t, home, dir, policies{allow: []string{xdg}})
		want := plan.Refusal{Path: filepath.Join(dir, "up"), Reason: "symlink target " + xdg + " is denied", Hint: dir + " is on caged's built-in deny list and cannot be allowed"}
		if refusal, ok := refusalAt(report, want.Path); !ok || refusal != want {
			t.Fatalf("refusal = %+v, %v; want %+v", refusal, ok, want)
		}
	})
}

// TestCagedDirIsCheckedLikeAnyConfigRoot pins WB-T1 (bugs.md B-HC-4): caged's
// folder, even when it is not a symlink, is refused when it is the home
// directory, as any config root would be, or when it is inside or holds a
// built-in deny root. links.deny judges links only.
func TestCagedDirIsCheckedLikeAnyConfigRoot(t *testing.T) {
	t.Parallel()

	t.Run("the home directory", func(t *testing.T) {
		t.Parallel()
		xdg := realHome(t)
		home := filepath.Join(xdg, "caged") // XDG_CONFIG_HOME=/home with HOME=/home/caged
		writeFiles(t, home, ".claude/settings.json")
		report := resolveWithCagedDir(t, home, home, policies{})
		want := plan.Refusal{Path: home, Reason: "config path resolves to the home directory"}
		if refusal, ok := refusalAt(report, home); !ok || refusal != want {
			t.Fatalf("refusal = %+v, %v; want %+v", refusal, ok, want)
		}
		if mount, ok := mountAt(report, cagedDest); ok {
			t.Fatalf("mount = %+v; want the home directory not mounted", mount)
		}
	})
	t.Run("inside a denied path", func(t *testing.T) {
		t.Parallel()
		home := realHome(t)
		dir := filepath.Join(home, ".ssh", "caged") // XDG_CONFIG_HOME=~/.ssh
		writeFiles(t, dir, "config.yaml")
		report := resolveWithCagedDir(t, home, dir, policies{})
		want := plan.Refusal{Path: dir, Reason: "config path is denied", Hint: "~/.ssh is on caged's built-in deny list and cannot be allowed"}
		if refusal, ok := refusalAt(report, dir); !ok || refusal != want {
			t.Fatalf("refusal = %+v, %v; want %+v", refusal, ok, want)
		}
		if mount, ok := mountAt(report, cagedDest); ok {
			t.Fatalf("mount = %+v; want a folder inside ~/.ssh not mounted", mount)
		}
	})
	t.Run("links.deny does not refuse a plain folder", func(t *testing.T) {
		t.Parallel()
		home := realHome(t)
		dir := filepath.Join(home, ".config", "caged")
		writeFiles(t, dir, "config.yaml")
		report := resolveWithCagedDir(t, home, dir, policies{deny: []string{filepath.Join(home, ".config")}})
		if mount, ok := mountAt(report, cagedDest); !ok || mount.HostPath != dir {
			t.Fatalf("mounts = %+v, refused = %v; want %s mounted: links.deny judges links only", report.Mounts, refusedStrings(report.Refused), dir)
		}
	})
	t.Run("containing a denied path", func(t *testing.T) {
		t.Parallel()
		xdg := realHome(t)
		dir := filepath.Join(xdg, "caged")
		home := filepath.Join(dir, "me") // the home directory is inside caged's folder
		writeFiles(t, home, ".claude/settings.json")
		report := resolveWithCagedDir(t, home, dir, policies{})
		if refusal, ok := refusalAt(report, dir); !ok || refusal.Reason != "config path is denied" {
			t.Fatalf("refusal = %+v, %v; want a folder containing ~/.ssh refused", refusal, ok)
		}
		if mount, ok := mountAt(report, cagedDest); ok {
			t.Fatalf("mount = %+v; want a folder holding the home directory not mounted", mount)
		}
	})
}
