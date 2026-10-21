package hostconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
)

const wantHintPrefix = `add "- path: `

// writeFiles creates files under home, with their directories.
func writeFiles(t *testing.T, home string, files ...string) {
	t.Helper()
	for _, file := range files {
		path := filepath.Join(home, file)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// symlink creates the symlink link under home, pointing at text.
func symlink(t *testing.T, home, link, text string) {
	t.Helper()
	path := filepath.Join(home, link)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(text, path); err != nil {
		t.Fatal(err)
	}
}

// mountAt returns the mount whose container path is dest.
func mountAt(report plan.ConfigReport, dest string) (plan.ConfigMount, bool) {
	for _, mount := range report.Mounts {
		if mount.DestPath == dest {
			return mount, true
		}
	}
	return plan.ConfigMount{}, false
}

// refusalAt returns the refusal for path.
func refusalAt(report plan.ConfigReport, path string) (plan.Refusal, bool) {
	for _, refusal := range report.Refused {
		if refusal.Path == path {
			return refusal, true
		}
	}
	return plan.Refusal{}, false
}

func mustMount(t *testing.T, report plan.ConfigReport, dest string) plan.ConfigMount {
	t.Helper()
	mount, ok := mountAt(report, dest)
	if !ok {
		t.Fatalf("no mount at %s; got mounts %+v and refusals %v", dest, report.Mounts, refusedStrings(report.Refused))
	}
	return mount
}

func mustRefuse(t *testing.T, report plan.ConfigReport, path string) plan.Refusal {
	t.Helper()
	refusal, ok := refusalAt(report, path)
	if !ok {
		t.Fatalf("%s was not refused; got mounts %+v and refusals %v", path, report.Mounts, refusedStrings(report.Refused))
	}
	return refusal
}

func TestLinkedFileInSplitTreeIsMountedWhereTheLinkIs(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, "dotfiles/claude/CLAUDE.md")
	symlink(t, home, ".claude/CLAUDE.md", "../dotfiles/claude/CLAUDE.md")

	report := resolveConfigsWith(t, home, nil, policies{allow: []string{filepath.Join(home, "dotfiles")}})

	want := plan.ConfigMount{
		Agent: "claude", Name: ".claude",
		HostPath: filepath.Join(resolved, "dotfiles/claude/CLAUDE.md"), DestPath: "/home/caged/.claude/CLAUDE.md",
		Root: filepath.Join(home, ".claude"), Writable: true, Link: filepath.Join(resolved, ".claude/CLAUDE.md"),
	}
	if got := mustMount(t, report, want.DestPath); fmt.Sprintf("%+v", got) != fmt.Sprintf("%+v", want) {
		t.Fatalf("mount = %+v, want %+v", got, want)
	}
	if len(report.Refused) != 0 {
		t.Fatalf("Refused = %v, want none", refusedStrings(report.Refused))
	}
}

func TestLinkedFileWithoutAllowIsRefusedWithAHint(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, "dotfiles/claude/CLAUDE.md")
	symlink(t, home, ".claude/CLAUDE.md", "../dotfiles/claude/CLAUDE.md")

	report := resolveConfigs(t, home, nil)

	if len(report.Mounts) != 0 {
		t.Fatalf("Mounts = %+v, want none", report.Mounts)
	}
	if len(report.Refused) != 1 {
		t.Fatalf("Refused = %v, want one refusal", refusedStrings(report.Refused))
	}
	refusal := mustRefuse(t, report, filepath.Join(resolved, ".claude/CLAUDE.md"))
	if want := `add "- path: ~/dotfiles/claude" under links.allow in ~/.config/caged/config.yaml`; refusal.Hint != want {
		t.Fatalf("Hint = %q, want %q", refusal.Hint, want)
	}
	if !strings.Contains(refusal.Reason, "is not under links.allow") {
		t.Fatalf("Reason = %q", refusal.Reason)
	}
}

func TestLinkedDirectoryIsMountedWhereTheLinkIs(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, "dotfiles/claude/agents/a.md")
	symlink(t, home, ".claude/agents", "../dotfiles/claude/agents")

	report := resolveConfigsWith(t, home, nil, policies{allow: []string{filepath.Join(home, "dotfiles")}})

	mount := mustMount(t, report, "/home/caged/.claude/agents")
	if mount.HostPath != filepath.Join(resolved, "dotfiles/claude/agents") || mount.Link != filepath.Join(resolved, ".claude/agents") || !mount.Writable {
		t.Fatalf("mount = %+v", mount)
	}
}

func TestSymlinkedConfigRootIsFollowedWhenAllowed(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, "dotfiles/pi/.pi/agent/settings.json", "dotfiles/pi/.pi/agent/settings-target.json", "dotfiles/pi/.pi/agent/extensions-target/example.js")
	symlink(t, home, ".pi", "dotfiles/pi/.pi")
	symlink(t, home, "dotfiles/pi/.pi/agent/settings-link.json", "settings-target.json")
	symlink(t, home, "dotfiles/pi/.pi/agent/extensions-link", "extensions-target")

	report := resolveConfigsWith(t, home, nil, policies{allow: []string{filepath.Join(home, "dotfiles")}})
	mount := mustMount(t, report, "/home/caged/.pi")
	if mount.HostPath != filepath.Join(resolved, "dotfiles/pi/.pi") || mount.Link != filepath.Join(home, ".pi") || !mount.Writable {
		t.Fatalf("mount = %+v", mount)
	}
	if len(mount.MaskPaths) != 1 || mount.MaskPaths[0] != "agent/extensions-link" {
		t.Fatalf("Pi tree directory masks = %v, want the nested symlink masked", mount.MaskPaths)
	}
	if len(mount.MaskFiles) != 1 || mount.MaskFiles[0] != "agent/settings-link.json" {
		t.Fatalf("Pi tree file masks = %v, want the file symlink masked", mount.MaskFiles)
	}
	linkMount := mustMount(t, report, "/home/caged/.pi/agent/extensions-link")
	if linkMount.HostPath != filepath.Join(resolved, "dotfiles/pi/.pi/agent/extensions-target") {
		t.Fatalf("Pi link target mount = %+v", linkMount)
	}

	report = resolveConfigs(t, home, nil)
	if _, ok := mountAt(report, "/home/caged/.pi"); ok {
		t.Fatalf("a symlinked config root outside links.allow was mounted: %+v", report.Mounts)
	}
	refusal := mustRefuse(t, report, filepath.Join(home, ".pi"))
	if !strings.HasPrefix(refusal.Hint, wantHintPrefix) || !strings.Contains(refusal.Hint, "~/dotfiles/pi/.pi") {
		t.Fatalf("Hint = %q", refusal.Hint)
	}
}

func TestPiTreeFollowsLinksAddedAfterAnEarlierResolve(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, ".pi/agent/settings.json", ".ssh/id_e2e")
	allow := []string{filepath.Join(home, "dotfiles")}

	first := resolveConfigsWith(t, home, nil, policies{allow: allow})
	if _, ok := mountAt(first, "/home/caged/.pi"); !ok {
		t.Fatalf("initial Pi tree was not mounted: %+v", first)
	}

	writeFiles(t, home,
		"dotfiles/pi/agent/future-settings.json",
		"dotfiles/pi/agent/future-extensions/index.js",
	)
	symlink(t, home, ".pi/agent/future-settings.json", "../../dotfiles/pi/agent/future-settings.json")
	symlink(t, home, ".pi/agent/future-extensions", "../../dotfiles/pi/agent/future-extensions")
	symlink(t, home, ".pi/agent/future-secret", "../../.ssh/id_e2e")

	second := resolveConfigsWith(t, home, nil, policies{allow: allow})
	root := mustMount(t, second, "/home/caged/.pi")
	if !slices.Contains(root.MaskFiles, "agent/future-settings.json") || !slices.Contains(root.MaskFiles, "agent/future-secret") {
		t.Fatalf("new file symlinks were not masked from the parent tree: %v", root.MaskFiles)
	}
	if !slices.Contains(root.MaskPaths, "agent/future-extensions") {
		t.Fatalf("new directory symlink was not masked from the parent tree: %v", root.MaskPaths)
	}
	if mount := mustMount(t, second, "/home/caged/.pi/agent/future-settings.json"); mount.HostPath != filepath.Join(resolved, "dotfiles/pi/agent/future-settings.json") {
		t.Fatalf("new file link mount = %+v", mount)
	}
	if mount := mustMount(t, second, "/home/caged/.pi/agent/future-extensions"); mount.HostPath != filepath.Join(resolved, "dotfiles/pi/agent/future-extensions") {
		t.Fatalf("new directory link mount = %+v", mount)
	}
	if _, ok := mountAt(second, "/home/caged/.pi/agent/future-secret"); ok {
		t.Fatal("a newly added link to a denied target was mounted")
	}
	if refusal := mustRefuse(t, second, filepath.Join(resolved, ".pi/agent/future-secret")); !strings.Contains(refusal.Hint, "built-in deny list") {
		t.Fatalf("new denied link refusal = %+v", refusal)
	}
}

func TestLinkInsideLinkedDirectoryIsFollowed(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, "dotfiles/claude/agents/a.md", "dotfiles2/shared.md")
	symlink(t, home, "dotfiles/claude/agents/shared.md", "../../../dotfiles2/shared.md")
	symlink(t, home, ".claude/agents", "../dotfiles/claude/agents")

	report := resolveConfigsWith(t, home, nil, policies{allow: []string{filepath.Join(home, "dotfiles"), filepath.Join(home, "dotfiles2")}})

	if mount := mustMount(t, report, "/home/caged/.claude/agents/a.md"); mount.HostPath != filepath.Join(resolved, "dotfiles/claude/agents/a.md") {
		t.Fatalf("mount = %+v", mount)
	}
	mount := mustMount(t, report, "/home/caged/.claude/agents/shared.md")
	if mount.HostPath != filepath.Join(resolved, "dotfiles2/shared.md") {
		t.Fatalf("mount = %+v", mount)
	}
	if len(report.Refused) != 0 {
		t.Fatalf("Refused = %v, want none", refusedStrings(report.Refused))
	}
}

func TestLinkInsideItsOwnConfigFolderNeedsNoPolicy(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, ".gemini/settings.json")
	symlink(t, home, ".gemini/current-settings.json", "settings.json")

	report := resolveConfigs(t, home, nil)

	mount := mustMount(t, report, "/home/caged/.gemini/current-settings.json")
	if mount.HostPath != filepath.Join(resolved, ".gemini/settings.json") || mount.Link != filepath.Join(resolved, ".gemini/current-settings.json") {
		t.Fatalf("mount = %+v", mount)
	}
	mustMount(t, report, "/home/caged/.gemini/settings.json")
	if len(report.Refused) != 0 {
		t.Fatalf("Refused = %v, want none", refusedStrings(report.Refused))
	}
}

func TestDanglingLinkIsRefused(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, ".claude/settings.json")
	symlink(t, home, ".claude/gone", "../dotfiles/missing")

	report := resolveConfigsWith(t, home, nil, policies{allow: []string{filepath.Join(home, "dotfiles")}})

	if refusal := mustRefuse(t, report, filepath.Join(resolved, ".claude/gone")); refusal.Reason != "dangling or unresolvable symlink" {
		t.Fatalf("Reason = %q", refusal.Reason)
	}
}

func TestLinkToItsOwnDirectoryIsRefused(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, ".claude/settings.json")
	symlink(t, home, ".claude/loop", ".")

	report := resolveConfigs(t, home, nil)

	if refusal := mustRefuse(t, report, filepath.Join(resolved, ".claude/loop")); refusal.Reason != "symlink points to a directory that contains it" {
		t.Fatalf("Reason = %q", refusal.Reason)
	}
}

func TestBuiltinDenyListBeatsAnAllowedRoot(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, ".ssh/id")
	symlink(t, home, ".claude/key", "../.ssh/id")

	report := resolveConfigsWith(t, home, nil, policies{allow: []string{filepath.Join(home, "dotfiles")}})

	refusal := mustRefuse(t, report, filepath.Join(resolved, ".claude/key"))
	if !strings.Contains(refusal.Hint, "built-in deny list") {
		t.Fatalf("Hint = %q", refusal.Hint)
	}
	if _, ok := mountAt(report, "/home/caged/.claude/key"); ok {
		t.Fatal("a link to ~/.ssh was mounted")
	}
}

func TestLinkToADirectoryThatContainsADeniedPathIsRefused(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, "dotfiles/ssh/.ssh/id")
	symlink(t, home, ".ssh", "dotfiles/ssh/.ssh")
	symlink(t, home, ".claude/all", "../dotfiles")

	report := resolveConfigsWith(t, home, nil, policies{allow: []string{filepath.Join(home, "dotfiles")}})

	refusal := mustRefuse(t, report, filepath.Join(resolved, ".claude/all"))
	if !strings.Contains(refusal.Hint, "built-in deny list") || !strings.Contains(refusal.Reason, "is denied") {
		t.Fatalf("refusal = %+v", refusal)
	}
	if _, ok := mountAt(report, "/home/caged/.claude/all"); ok {
		t.Fatal("a link to a directory containing ~/.ssh was mounted")
	}
}

func TestReadOnlyRootMountsLinksReadOnly(t *testing.T) {
	t.Parallel()
	home, _ := testHome(t, nil, nil)
	writeFiles(t, home, "dotfiles/claude/CLAUDE.md")
	symlink(t, home, ".claude/CLAUDE.md", "../dotfiles/claude/CLAUDE.md")

	report := resolveConfigsWith(t, home, nil, policies{allow: []string{filepath.Join(home, "dotfiles")}, readOnly: true})

	if mount := mustMount(t, report, "/home/caged/.claude/CLAUDE.md"); mount.Writable {
		t.Fatalf("mount through a read_only root is writable: %+v", mount)
	}
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
	// The container refuses absolute link targets under /tmp, where t.TempDir
	// lives, so this home is created next to the test. A checkout that itself
	// lives under a reserved container path (/tmp, /var, /opt, ...) cannot
	// run it; containerLinkDest covers absolute links on their own.
	base, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, reserved := range reservedContainerPaths() {
		if withinSlash(filepath.ToSlash(base), reserved) {
			t.Skipf("the checkout is under %s, which absolute link targets may not use", reserved)
		}
	}
	home, err := os.MkdirTemp(base, "absolute-link-home-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(home) })
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

func TestMoreThanSixtyFourLinksAreNotFollowed(t *testing.T) {
	t.Parallel()
	home, _ := testHome(t, nil, nil)
	for i := range 65 {
		writeFiles(t, home, fmt.Sprintf("dotfiles/claude/f%d", i))
		symlink(t, home, fmt.Sprintf(".claude/l%d", i), fmt.Sprintf("../dotfiles/claude/f%d", i))
	}

	report := resolveConfigsWith(t, home, nil, policies{allow: []string{filepath.Join(home, "dotfiles")}})

	if len(report.Mounts) != 64 {
		t.Fatalf("got %d mounts, want 64", len(report.Mounts))
	}
	var over []plan.Refusal
	for _, refusal := range report.Refused {
		if strings.Contains(refusal.Reason, "more than 64 symlinks") {
			over = append(over, refusal)
		}
	}
	if len(over) != 1 || len(report.Refused) != 1 {
		t.Fatalf("Refused = %v, want exactly one over-budget refusal", refusedStrings(report.Refused))
	}
}

func TestResolvePiTreeListedTwiceKeepsMaskFilesOnTheRoot(t *testing.T) {
	home, _ := testHome(t, []string{".st"}, []string{".st/a.json"})
	if err := os.Symlink("a.json", filepath.Join(home, ".st", "filelink")); err != nil {
		t.Fatal(err)
	}
	path := agent.ConfigPath{HostPath: ".st", ContainerPath: ".st", Writable: true}
	agents := []agent.Agent{{Name: "pi", Command: []string{"pi"}, ConfigPaths: []agent.ConfigPath{path, path}}}
	report, err := NewResolver(system.FS{}, fakeEnv{}, agents, nil, policies{}).Resolve(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Mounts) != 2 || len(report.Mounts[0].MaskFiles) != 1 || len(report.Mounts[1].MaskFiles) != 0 {
		t.Fatalf("Mounts = %+v, want the root with one mask file and the link target with none", report.Mounts)
	}
}
