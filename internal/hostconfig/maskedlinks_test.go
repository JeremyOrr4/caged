package hostconfig

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/plan"
)

// homeBesideTheTest creates a directory next to the test, because the
// container refuses absolute link targets under /tmp, where t.TempDir lives. A
// checkout that itself lives under a reserved container path (/tmp, /var,
// /opt, ...) cannot run such a test; containerLinkDest covers absolute links
// on their own.
func homeBesideTheTest(t *testing.T, prefix string) string {
	t.Helper()
	base, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, reserved := range reservedContainerPaths() {
		if _, inside := plan.ContainerRel(filepath.ToSlash(base), reserved); inside {
			t.Skipf("the checkout is under %s, which absolute link targets may not use", reserved)
		}
	}
	dir, err := os.MkdirTemp(base, prefix)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func TestCodexTreeWithARelativeLinkKeepsTheWholeMount(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, ".codex/config.toml", "dotfiles/codex/AGENTS.md")
	symlink(t, home, ".codex/AGENTS.md", "../dotfiles/codex/AGENTS.md")

	report := resolveConfigsWith(t, home, nil, policies{allow: []string{filepath.Join(home, "dotfiles")}})

	tree := mustMount(t, report, "/home/caged/.codex")
	if tree.HostPath != filepath.Join(resolved, ".codex") || len(tree.MaskPaths) == 0 || tree.Link != "" {
		t.Fatalf("Codex tree mount = %+v", tree)
	}
	link := mustMount(t, report, "/home/caged/dotfiles/codex/AGENTS.md")
	if link.HostPath != filepath.Join(resolved, "dotfiles/codex/AGENTS.md") || link.Link != filepath.Join(resolved, ".codex/AGENTS.md") {
		t.Fatalf("link mount = %+v", link)
	}

	// Without an allowed root the tree stays mounted and only the link is refused.
	report = resolveConfigs(t, home, nil)
	mustMount(t, report, "/home/caged/.codex")
	if _, ok := mountAt(report, "/home/caged/dotfiles/codex/AGENTS.md"); ok {
		t.Fatal("the link target was mounted without links.allow")
	}
	if refusal := mustRefuse(t, report, filepath.Join(resolved, ".codex/AGENTS.md")); !strings.HasPrefix(refusal.Hint, wantHintPrefix) {
		t.Fatalf("Hint = %q", refusal.Hint)
	}
}

func TestCodexTreeWithAnAbsoluteLinkMountsAtTheSamePath(t *testing.T) {
	t.Parallel()
	home := homeBesideTheTest(t, "absolute-link-home-")
	resolved, err := filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatal(err)
	}
	writeFiles(t, home, ".codex/config.toml", "dotfiles/codex/AGENTS.md")
	symlink(t, home, ".codex/AGENTS.md", filepath.Join(home, "dotfiles/codex/AGENTS.md"))

	report := resolveConfigsWith(t, home, nil, policies{allow: []string{filepath.Join(home, "dotfiles")}})

	dest := filepath.ToSlash(filepath.Join(home, "dotfiles/codex/AGENTS.md"))
	mount := mustMount(t, report, dest)
	if mount.HostPath != filepath.Join(resolved, "dotfiles/codex/AGENTS.md") {
		t.Fatalf("mount = %+v", mount)
	}
	mustMount(t, report, "/home/caged/.codex")
}

func TestMaskedLinkLandingInsideAnotherMountIsRefused(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, ".claude/settings.json", ".codex/config.toml")
	symlink(t, home, ".codex/y", "../.claude/settings.json")

	report := resolveConfigsWith(t, home, nil, policies{allow: []string{filepath.Join(home, ".claude")}})

	mustMount(t, report, "/home/caged/.claude")
	if mount, ok := mountAt(report, "/home/caged/.claude/settings.json"); ok {
		t.Fatalf("a mount landed inside another mount: %+v", mount)
	}
	if refusal := mustRefuse(t, report, filepath.Join(resolved, ".codex/y")); !strings.Contains(refusal.Reason, "inside another mount") {
		t.Fatalf("Reason = %q", refusal.Reason)
	}
}

func TestMaskedLinkThatCannotBeFollowedInTheContainerIsRefused(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, ".codex/config.toml")
	symlink(t, home, ".codex/x", "/etc/x")

	report := resolveConfigs(t, home, nil)

	want := []string{filepath.Join(resolved, ".codex/x") + " (symlink to /etc/x cannot be followed inside the container)"}
	if got := refusedStrings(report.Refused); !slices.Equal(got, want) {
		t.Fatalf("Refused = %v, want %v", got, want)
	}
	mustMount(t, report, "/home/caged/.codex")
}

// TestMaskedLinkToADirectoryHoldingTheContainerHomeIsRefused is bug B-HC-5: a
// link to /home, even under an allowed root, would be mounted over the parent
// of the container home. It is refused before its target is read.
func TestMaskedLinkToADirectoryHoldingTheContainerHomeIsRefused(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, ".codex/config.toml")
	symlink(t, home, ".codex/h", "/home")

	report := resolveConfigsWith(t, home, nil, policies{allow: []string{"/home"}})

	want := []string{filepath.Join(resolved, ".codex/h") + " (symlink to /home cannot be followed inside the container)"}
	if got := refusedStrings(report.Refused); !slices.Equal(got, want) {
		t.Fatalf("Refused = %v, want %v", got, want)
	}
	if len(report.Mounts) != 1 || report.Mounts[0].DestPath != "/home/caged/.codex" {
		t.Fatalf("Mounts = %+v, want only the Codex tree", report.Mounts)
	}
}

// TestCodexTreeLinkInsideTheTreeIsLeftAsItIs: a whole tree is mounted with
// its links, so a link that stays inside the tree needs no mount of its own
// and no refusal.
func TestCodexTreeLinkInsideTheTreeIsLeftAsItIs(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, ".codex/config.toml", ".codex/rules/a.md")
	symlink(t, home, ".codex/current.toml", "config.toml")
	symlink(t, home, ".codex/all-rules", "rules")

	report := resolveConfigs(t, home, nil)

	if len(report.Mounts) != 1 || report.Mounts[0].DestPath != "/home/caged/.codex" || report.Mounts[0].HostPath != filepath.Join(resolved, ".codex") || len(report.Refused) != 0 {
		t.Fatalf("Mounts = %+v, Refused = %v; want only the whole Codex tree", report.Mounts, refusedStrings(report.Refused))
	}
}
