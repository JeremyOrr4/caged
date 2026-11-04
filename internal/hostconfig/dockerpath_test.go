package hostconfig

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/plan"
)

// TestMountPathsWithAColonAreRefused pins WB-T2 (bugs.md Q-DK-1): docker's -v
// and --tmpfs split their argument at ":", so a host path or container path
// holding one is refused instead of being passed on. On the isolated daemon a
// destination ending in ":ro" was mounted read-only one level up, and any
// other ":" failed the whole run. A link the container plants in its own
// config folder must not be able to do either. A comma, a double quote or a
// newline needs no refusal: docker mounts those paths as they are.
func TestMountPathsWithAColonAreRefused(t *testing.T) {
	t.Parallel()

	home := realHome(t)
	writeFiles(t, home, ".claude/settings.json", "dotfiles/f.md", "dotfiles/a:b/x.md", ".claude/sub:x/y.json", "dotfiles/c,d\"e\nf/g.md")
	symlink(t, home, ".claude/x:ro", filepath.Join(home, "dotfiles", "f.md"))     // container path holds ":"
	symlink(t, home, ".claude/ab", filepath.Join(home, "dotfiles", "a:b"))        // host path holds ":"
	symlink(t, home, ".claude/odd", filepath.Join(home, "dotfiles", "c,d\"e\nf")) // mounted as it is

	report := resolveConfigsWith(t, home, nil, policies{allow: []string{filepath.Join(home, "dotfiles")}})

	const reason = `path contains ":", which docker cannot mount`
	for _, path := range []string{
		filepath.Join(home, ".claude", "x:ro"),
		filepath.Join(home, "dotfiles", "a:b"),
		filepath.Join(home, ".claude", "sub:x"),
	} {
		if refusal, ok := refusalAt(report, path); !ok || refusal != (plan.Refusal{Path: path, Reason: reason}) {
			t.Errorf("refusal for %s = %+v, %v; want %q", path, refusal, ok, reason)
		}
	}
	for _, mount := range report.Mounts {
		if strings.Contains(mount.HostPath, ":") || strings.Contains(mount.DestPath, ":") {
			t.Errorf("mount %+v holds a colon", mount)
		}
	}
	if mount, ok := mountAt(report, "/home/caged/.claude/odd"); !ok || mount.HostPath != filepath.Join(home, "dotfiles", "c,d\"e\nf") {
		t.Errorf("mount = %+v, %v; want the comma, quote and newline path mounted", mount, ok)
	}
}

// TestColonLinksInWholeAndStagedTreesAreRefused: in a Codex tree (mounted
// whole) a link stays visible as itself, so only a target with a colon, which
// is mounted where the link points, is refused; in a Pi tree (staged) the link
// is mounted at its own place, so a colon in its name is refused. Either tree
// is still mounted. A staged tree's masks never reach docker (configstage
// mounts a symlink-free copy and drops them), so they need no check.
func TestColonLinksInWholeAndStagedTreesAreRefused(t *testing.T) {
	t.Parallel()

	home := realHome(t)
	writeFiles(t, home, ".codex/config.toml", ".pi/agent/settings.json", "dotfiles/f.md", "dotfiles/a:b.md")
	symlink(t, home, ".codex/x:ro", "../dotfiles/f.md") // a whole tree needs a link the container can follow
	symlink(t, home, ".codex/ab", "../dotfiles/a:b.md")
	symlink(t, home, ".pi/agent/y:ro", filepath.Join(home, "dotfiles", "f.md"))

	report := resolveConfigsWith(t, home, nil, policies{allow: []string{filepath.Join(home, "dotfiles")}})

	const reason = `path contains ":", which docker cannot mount`
	want := []string{
		filepath.Join(home, "dotfiles", "a:b.md") + " (" + reason + ")",
		filepath.Join(home, ".pi", "agent", "y:ro") + " (" + reason + ")",
	}
	if got := refusedStrings(report.Refused); !slices.Equal(got, want) {
		t.Errorf("refused = %q; want %q", got, want)
	}
	for _, dest := range []string{"/home/caged/.codex", "/home/caged/.pi", "/home/caged/dotfiles/f.md"} {
		if _, ok := mountAt(report, dest); !ok {
			t.Errorf("mounts = %+v; want a mount at %s", report.Mounts, dest)
		}
	}
	for _, mount := range report.Mounts {
		if strings.Contains(mount.HostPath, ":") || strings.Contains(mount.DestPath, ":") {
			t.Errorf("mount %+v holds a colon", mount)
		}
	}
}
