package hostconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/JeremyOrr4/caged/internal/plan"
)

const wantHintPrefix = `add "- path: `

// writeFiles creates files under home, with their directories.
func writeFiles(t testing.TB, home string, files ...string) {
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
func symlink(t testing.TB, home, link, text string) {
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

// TestLinkedFileIsMountedWhereTheLinkPoints: the tree is mounted whole, so the
// link is visible in the container, and its target is mounted where the
// link's text points there.
func TestLinkedFileIsMountedWhereTheLinkPoints(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, "dotfiles/claude/CLAUDE.md")
	symlink(t, home, ".claude/CLAUDE.md", "../dotfiles/claude/CLAUDE.md")

	report := resolveConfigsWith(t, home, nil, policies{allow: []string{filepath.Join(home, "dotfiles")}})

	want := plan.ConfigMount{
		Agent: "claude", Name: ".claude",
		HostPath: filepath.Join(resolved, "dotfiles/claude/CLAUDE.md"), DestPath: "/home/caged/dotfiles/claude/CLAUDE.md",
		Root: filepath.Join(home, ".claude"), Writable: true, Link: filepath.Join(resolved, ".claude/CLAUDE.md"),
	}
	if got := mustMount(t, report, want.DestPath); fmt.Sprintf("%+v", got) != fmt.Sprintf("%+v", want) {
		t.Fatalf("mount = %+v, want %+v", got, want)
	}
	mustMount(t, report, "/home/caged/.claude")
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

	if len(report.Mounts) != 1 || report.Mounts[0].DestPath != "/home/caged/.claude" {
		t.Fatalf("Mounts = %+v, want only the ~/.claude tree", report.Mounts)
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

func TestLinkedDirectoryIsMountedWhereTheLinkPoints(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, "dotfiles/claude/agents/a.md")
	symlink(t, home, ".claude/agents", "../dotfiles/claude/agents")

	report := resolveConfigsWith(t, home, nil, policies{allow: []string{filepath.Join(home, "dotfiles")}})

	mount := mustMount(t, report, "/home/caged/dotfiles/claude/agents")
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
	// Both links resolve inside the tree, in the container as on the host, so
	// the tree is mounted live: no copy, no masks and no mounts of their own.
	if mount.Staged || len(mount.MaskPaths) != 0 || len(mount.MaskFiles) != 0 {
		t.Fatalf("Pi tree = %+v, want a live mount without masks", mount)
	}
	for _, m := range report.Mounts {
		if m.Agent == "pi" && m.DestPath != "/home/caged/.pi" {
			t.Fatalf("unexpected Pi mount %+v", m)
		}
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

// TestPiTreeFallsBackToACopyForALinkTheContainerCannotFollow: a relative link
// with a ".." after a name resolves on the host, but caged does not follow it
// in the container, so Pi runs from a staged copy, with the link hidden and
// its target mounted at the link's path.
func TestPiTreeFallsBackToACopyForALinkTheContainerCannotFollow(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, "dotfiles/pi/.pi/agent/settings.json", "dotfiles/ext/index.js")
	symlink(t, home, ".pi", "dotfiles/pi/.pi")
	symlink(t, home, "dotfiles/pi/.pi/agent/extensions", "../../../ext/../ext")

	report := resolveConfigsWith(t, home, nil, policies{allow: []string{filepath.Join(home, "dotfiles")}})

	root := mustMount(t, report, "/home/caged/.pi")
	if !root.Staged || !slices.Equal(root.MaskPaths, []string{"agent/extensions"}) {
		t.Fatalf("Pi tree = %+v, want a staged copy with the link masked", root)
	}
	ext := mustMount(t, report, "/home/caged/.pi/agent/extensions")
	if !ext.Staged || ext.HostPath != filepath.Join(resolved, "dotfiles/ext") {
		t.Fatalf("link target mount = %+v", ext)
	}
	if len(report.Refused) != 0 {
		t.Fatalf("Refused = %v; the live attempt's refusal must be undone", refusedStrings(report.Refused))
	}
}

// TestPiTreeStaysLiveWhenTheUnfollowableLinkIsRefusedAnyway: a copy would not
// help a link whose target the policy refuses, so the tree stays live.
func TestPiTreeStaysLiveWhenTheUnfollowableLinkIsRefusedAnyway(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, "dotfiles/pi/.pi/agent/settings.json", "private/ext/index.js")
	symlink(t, home, ".pi", "dotfiles/pi/.pi")
	symlink(t, home, "dotfiles/pi/.pi/agent/extensions", "../../../../private/ext")

	report := resolveConfigsWith(t, home, nil, policies{allow: []string{filepath.Join(home, "dotfiles")}})

	if root := mustMount(t, report, "/home/caged/.pi"); root.Staged {
		t.Fatalf("Pi tree = %+v, want it live", root)
	}
	mustRefuse(t, report, filepath.Join(resolved, "dotfiles/pi/.pi/agent/extensions"))
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
	if root.Staged || len(root.MaskPaths) != 0 || len(root.MaskFiles) != 0 {
		t.Fatalf("Pi tree = %+v, want it live with its links visible", root)
	}
	if mount := mustMount(t, second, "/home/caged/dotfiles/pi/agent/future-settings.json"); mount.HostPath != filepath.Join(resolved, "dotfiles/pi/agent/future-settings.json") {
		t.Fatalf("new file link mount = %+v", mount)
	}
	if mount := mustMount(t, second, "/home/caged/dotfiles/pi/agent/future-extensions"); mount.HostPath != filepath.Join(resolved, "dotfiles/pi/agent/future-extensions") {
		t.Fatalf("new directory link mount = %+v", mount)
	}
	if _, ok := mountAt(second, "/home/caged/.ssh/id_e2e"); ok {
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

	if mount := mustMount(t, report, "/home/caged/dotfiles/claude/agents"); mount.HostPath != filepath.Join(resolved, "dotfiles/claude/agents") {
		t.Fatalf("mount = %+v", mount)
	}
	mount := mustMount(t, report, "/home/caged/dotfiles2/shared.md")
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

	// The link stays visible in the whole tree and resolves there as it does
	// on the host, so it needs no mount and no policy.
	if len(report.Mounts) != 1 || report.Mounts[0].HostPath != filepath.Join(resolved, ".gemini") || len(report.Refused) != 0 {
		t.Fatalf("Mounts = %+v, Refused = %v; want only the ~/.gemini tree", report.Mounts, refusedStrings(report.Refused))
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

// TestLinkToItsOwnDirectoryIsLeftAsItIs: a link inside a whole tree that
// points back at the tree resolves in the container as on the host and is
// never followed, so it needs no mount and no refusal.
func TestLinkToItsOwnDirectoryIsLeftAsItIs(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, ".claude/settings.json")
	symlink(t, home, ".claude/loop", ".")

	report := resolveConfigs(t, home, nil)

	if len(report.Mounts) != 1 || report.Mounts[0].HostPath != filepath.Join(resolved, ".claude") || len(report.Refused) != 0 {
		t.Fatalf("Mounts = %+v, Refused = %v; want only the ~/.claude tree", report.Mounts, refusedStrings(report.Refused))
	}
}

// TestLinkFromALinkedDirectoryToItsParentIsRefused: a followed directory link
// whose target holds the link would be mounted inside itself.
func TestLinkFromALinkedDirectoryToItsParentIsRefused(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, ".claude/settings.json", "dotfiles/claude/a.md")
	symlink(t, home, ".claude/dot", "../dotfiles/claude")
	symlink(t, home, "dotfiles/claude/up", "..")

	report := resolveConfigsWith(t, home, nil, policies{allow: []string{filepath.Join(home, "dotfiles")}})

	mustMount(t, report, "/home/caged/dotfiles/claude")
	if refusal := mustRefuse(t, report, filepath.Join(resolved, "dotfiles/claude/up")); refusal.Reason != "symlink points to a directory that contains it" {
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
	if _, ok := mountAt(report, "/home/caged/.ssh/id"); ok {
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
	if _, ok := mountAt(report, "/home/caged/dotfiles"); ok {
		t.Fatal("a link to a directory containing ~/.ssh was mounted")
	}
}

func TestReadOnlyRootMountsLinksReadOnly(t *testing.T) {
	t.Parallel()
	home, _ := testHome(t, nil, nil)
	writeFiles(t, home, "dotfiles/claude/CLAUDE.md")
	symlink(t, home, ".claude/CLAUDE.md", "../dotfiles/claude/CLAUDE.md")

	report := resolveConfigsWith(t, home, nil, policies{allow: []string{filepath.Join(home, "dotfiles")}, readOnly: true})

	if mount := mustMount(t, report, "/home/caged/dotfiles/claude/CLAUDE.md"); mount.Writable {
		t.Fatalf("mount through a read_only root is writable: %+v", mount)
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

	if len(report.Mounts) != 1+64 {
		t.Fatalf("got %d mounts, want the tree and 64 links", len(report.Mounts))
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

func TestNestedSymlinkedDirectoriesStopAtDepthEight(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, ".claude/settings.json")
	symlink(t, home, ".claude/next", "../dotfiles/d1")
	for i := 1; i <= 10; i++ {
		writeFiles(t, home, fmt.Sprintf("dotfiles/d%d/f", i))
		symlink(t, home, fmt.Sprintf("dotfiles/d%d/next", i), fmt.Sprintf("../d%d", i+1))
	}

	report := resolveConfigsWith(t, home, nil, policies{allow: []string{filepath.Join(home, "dotfiles")}})

	want := []string{filepath.Join(resolved, "dotfiles/d8/next") + " (too many nested symlinked directories)"}
	if got := refusedStrings(report.Refused); !slices.Equal(got, want) {
		t.Fatalf("Refused = %v, want %v", got, want)
	}
	mustMount(t, report, "/home/caged/dotfiles/d8")
	if _, ok := mountAt(report, "/home/caged/dotfiles/d9"); ok {
		t.Fatal("a ninth nested symlinked directory was mounted")
	}
}

func TestLinkToASpecialFileIsRefused(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, ".claude/settings.json")
	if err := os.MkdirAll(filepath.Join(home, "dotfiles"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(home, "dotfiles", "pipe"), 0o600); err != nil {
		t.Fatal(err)
	}
	symlink(t, home, ".claude/pipe", "../dotfiles/pipe")

	report := resolveConfigsWith(t, home, nil, policies{allow: []string{filepath.Join(home, "dotfiles")}})

	if refusal := mustRefuse(t, report, filepath.Join(resolved, ".claude/pipe")); refusal.Reason != "symlink target is not a regular file or directory" {
		t.Fatalf("Reason = %q", refusal.Reason)
	}
	if _, ok := mountAt(report, "/home/caged/dotfiles/pipe"); ok {
		t.Fatal("a link to a FIFO was mounted")
	}
}
