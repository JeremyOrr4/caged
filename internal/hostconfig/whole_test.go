package hostconfig

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
)

// TestAbsoluteLinkIntoItsOwnTreeCannotReachAMask: an absolute link into its
// own tree has its target mounted again at the host path, without the tree's
// masks, so a link planted in ~/.codex must not reach Codex's masked runtime
// directories, in them or above them. A link to an unmasked part of the tree
// is mounted as usual.
func TestAbsoluteLinkIntoItsOwnTreeCannotReachAMask(t *testing.T) {
	t.Parallel()
	home := homeBesideTheTest(t, "own-tree-link-home-")
	resolved, err := filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatal(err)
	}
	writeFiles(t, home, ".codex/config.toml", ".codex/ipc/state", ".codex/packages/app-server-daemon/d", ".codex/rules/a.md")
	symlink(t, home, ".codex/i", filepath.Join(home, ".codex/ipc"))
	symlink(t, home, ".codex/p", filepath.Join(home, ".codex/packages"))
	symlink(t, home, ".codex/r", filepath.Join(home, ".codex/rules"))

	report := resolveConfigs(t, home, nil)

	for _, link := range []string{"i", "p"} {
		refusal := mustRefuse(t, report, filepath.Join(resolved, ".codex", link))
		if !strings.Contains(refusal.Reason, "folder caged hides from agents") || refusal.Hint == "" {
			t.Errorf("refusal for %s = %+v", link, refusal)
		}
	}
	for _, mount := range report.Mounts {
		if strings.Contains(mount.HostPath, "ipc") || strings.Contains(mount.HostPath, "packages") {
			t.Errorf("a masked directory was mounted: %+v", mount)
		}
	}
	if mount := mustMount(t, report, filepath.ToSlash(filepath.Join(home, ".codex/rules"))); mount.HostPath != filepath.Join(resolved, ".codex/rules") {
		t.Fatalf("rules mount = %+v", mount)
	}
}

// TestMaskCheckIgnoresCaseWhereTheFileSystemDoes: on macOS a link can name a
// masked directory in other letters; with a case-folding policy the check
// still catches it, in, at and above the mask. Where names keep case they are
// other directories.
func TestMaskCheckIgnoresCaseWhereTheFileSystemDoes(t *testing.T) {
	t.Parallel()
	home := realHome(t)
	exact, err := policies{}.LinkPolicy(home)
	if err != nil {
		t.Fatal(err)
	}
	folded, err := exact.FoldCase()
	if err != nil {
		t.Fatal(err)
	}
	masked := filepath.Join(home, ".codex/packages/app-server-daemon")
	for _, target := range []string{
		filepath.Join(home, ".CODEX/packages/app-server-daemon"),
		filepath.Join(home, ".codex/Packages/App-Server-Daemon/bin"),
		filepath.Join(home, ".Codex/packages"),
	} {
		if !(&resolution{policy: folded, masked: []string{masked}}).touchesMask(target) {
			t.Errorf("folded: %s not caught", target)
		}
		if (&resolution{policy: exact, masked: []string{masked}}).touchesMask(target) {
			t.Errorf("exact: %s caught, but names keep case", target)
		}
	}
}

// TestLinkFromAnotherTreeIntoAMaskIsRefused: Codex's masked directories stay
// hidden from every config tree, even when links.allow covers them.
func TestLinkFromAnotherTreeIntoAMaskIsRefused(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, ".codex/config.toml", ".codex/ipc/state", ".claude/settings.json")
	symlink(t, home, ".claude/ipc", "../.codex/ipc")
	symlink(t, home, ".claude/codex", "../.codex")

	report := resolveConfigsWith(t, home, nil, policies{allow: []string{filepath.Join(home, ".codex")}})

	for _, link := range []string{"ipc", "codex"} {
		if refusal := mustRefuse(t, report, filepath.Join(resolved, ".claude", link)); !strings.Contains(refusal.Reason, "folder caged hides from agents") {
			t.Errorf("refusal for %s = %+v", link, refusal)
		}
	}
	for _, mount := range report.Mounts {
		if mount.Agent == "claude" && mount.Link != "" {
			t.Errorf("a link into Codex's masks was mounted: %+v", mount)
		}
	}
}

// TestEntryWithAColonIsHiddenLikeAnyOther: a special file named with a ":"
// is masked as any other (docker.mountSpec quotes it), and the tree, Pi's
// too, stays mounted live.
func TestEntryWithAColonIsHiddenLikeAnyOther(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, ".claude/settings.json", ".pi/agent/settings.json")
	for _, fifo := range []string{".claude/p:exec", ".pi/agent/s:x"} {
		if err := syscall.Mkfifo(filepath.Join(home, fifo), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	report := resolveConfigs(t, home, nil)

	if mount := mustMount(t, report, "/home/caged/.claude"); !slices.Equal(mount.MaskFiles, []string{"p:exec"}) {
		t.Fatalf("~/.claude mount = %+v", mount)
	}
	if mount := mustMount(t, report, "/home/caged/.pi"); mount.Staged || !slices.Equal(mount.MaskFiles, []string{"agent/s:x"}) {
		t.Fatalf("Pi mount = %+v, want it live with the entry hidden", mount)
	}
	mustRefuse(t, report, filepath.Join(resolved, ".claude", "p:exec"))
}

// TestLinkMountAtOrOverAnotherMountIsRefused: in a Stow root-linked tree a
// link can point at the destination of another mount (the shared .gitconfig)
// or above one (~/.config, which holds gh's config). Both are refused.
func TestLinkMountAtOrOverAnotherMountIsRefused(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, ".gitconfig", ".config/gh/hosts.yml", "dotfiles/claude/.claude/settings.json", "dotfiles/claude/.gitconfig", "dotfiles/claude/.config/x")
	symlink(t, home, ".claude", "dotfiles/claude/.claude")
	symlink(t, home, "dotfiles/claude/.claude/gc", "../.gitconfig")
	symlink(t, home, "dotfiles/claude/.claude/cfg", "../.config")

	report := resolveConfigsWith(t, home, nil, policies{allow: []string{filepath.Join(home, "dotfiles")}})

	if gitconfig := mustMount(t, report, "/home/caged/.gitconfig"); gitconfig.Agent != "shared" {
		t.Fatalf(".gitconfig mount = %+v, want the shared one", gitconfig)
	}
	if config, ok := mountAt(report, "/home/caged/.config"); ok {
		t.Fatalf("a link target was mounted over ~/.config: %+v", config)
	}
	mustMount(t, report, "/home/caged/.config/gh")
	for _, link := range []string{"gc", "cfg"} {
		if refusal := mustRefuse(t, report, filepath.Join(resolved, "dotfiles/claude/.claude", link)); !strings.Contains(refusal.Reason, "at or over another mount") {
			t.Errorf("refusal for %s = %+v", link, refusal)
		}
	}
}

// TestFileLinkInsideALinkedDirectoryKeepsTheDirectory: a file link whose
// target lies in a directory another link mounts is dropped, and the
// directory, which holds the file, stays mounted.
func TestFileLinkInsideALinkedDirectoryKeepsTheDirectory(t *testing.T) {
	t.Parallel()
	home, _ := testHome(t, nil, nil)
	writeFiles(t, home, ".claude/settings.json", "dotfiles/claude/CLAUDE.md", "dotfiles/claude/agents/a.md")
	symlink(t, home, ".claude/CLAUDE.md", "../dotfiles/claude/CLAUDE.md")
	symlink(t, home, ".claude/dot", "../dotfiles/claude")

	report := resolveConfigsWith(t, home, nil, policies{allow: []string{filepath.Join(home, "dotfiles")}})

	mustMount(t, report, "/home/caged/dotfiles/claude")
	if _, ok := mountAt(report, "/home/caged/dotfiles/claude/CLAUDE.md"); ok {
		t.Fatal("the file was mounted inside the directory's mount")
	}
}

// TestLinkClimbingOutOfARootLinkedTreeIsMounted: with ~/.claude a Stow link,
// a relative link that climbs out of the package points outside the
// container home; its target is mounted there.
func TestLinkClimbingOutOfARootLinkedTreeIsMounted(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, "dotfiles/claude/.claude/settings.json", "dotfiles/shared/AGENTS.md")
	symlink(t, home, ".claude", "dotfiles/claude/.claude")
	symlink(t, home, "dotfiles/claude/.claude/AGENTS.md", "../../shared/AGENTS.md")

	report := resolveConfigsWith(t, home, nil, policies{allow: []string{filepath.Join(home, "dotfiles")}})

	mount := mustMount(t, report, "/home/shared/AGENTS.md")
	if mount.HostPath != filepath.Join(resolved, "dotfiles/shared/AGENTS.md") || len(report.Refused) != 0 {
		t.Fatalf("mount = %+v, Refused = %v", mount, refusedStrings(report.Refused))
	}
}

// TestLinkClimbingIntoASystemDirectoryIsRefused: the climbing rule still
// keeps targets out of the container's system directories.
func TestLinkClimbingIntoASystemDirectoryIsRefused(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, "dotfiles/claude/.claude/settings.json", "dotfiles/etc/x")
	symlink(t, home, ".claude", "dotfiles/claude/.claude")
	symlink(t, home, "dotfiles/claude/.claude/x", "../../../etc/x")

	report := resolveConfigsWith(t, home, nil, policies{allow: []string{filepath.Join(home, "dotfiles")}})

	if refusal := mustRefuse(t, report, filepath.Join(resolved, "dotfiles/claude/.claude/x")); refusal.Hint != hintUnfollowable {
		t.Fatalf("refusal = %+v", refusal)
	}
}

// TestRelativeLinksInADirectoryReachedByAnAbsoluteLink: the directory is
// mounted at its host path, so a link inside it that stays in it needs
// nothing, and one that climbs out has its target mounted beside it.
func TestRelativeLinksInADirectoryReachedByAnAbsoluteLink(t *testing.T) {
	t.Parallel()
	home := homeBesideTheTest(t, "absolute-dir-link-home-")
	resolved, err := filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatal(err)
	}
	writeFiles(t, home, ".claude/settings.json", "dotfiles/agents/b.md", "dotfiles/shared/s.md")
	symlink(t, home, ".claude/agents", filepath.Join(home, "dotfiles/agents"))
	symlink(t, home, "dotfiles/agents/a.md", "b.md")
	symlink(t, home, "dotfiles/agents/s.md", "../shared/s.md")

	report := resolveConfigsWith(t, home, nil, policies{allow: []string{filepath.Join(home, "dotfiles")}})

	agents := filepath.ToSlash(filepath.Join(home, "dotfiles/agents"))
	mustMount(t, report, agents)
	if mount := mustMount(t, report, filepath.ToSlash(filepath.Join(home, "dotfiles/shared/s.md"))); mount.HostPath != filepath.Join(resolved, "dotfiles/shared/s.md") {
		t.Fatalf("mount = %+v", mount)
	}
	if len(report.Refused) != 0 {
		t.Fatalf("Refused = %v, want none", refusedStrings(report.Refused))
	}
}

// TestPiFallsBackForALinkDeepInALinkedDirectory: the link the container
// cannot follow sits in a directory another link leads to.
func TestPiFallsBackForALinkDeepInALinkedDirectory(t *testing.T) {
	t.Parallel()
	home, _ := testHome(t, nil, nil)
	writeFiles(t, home, ".pi/agent/settings.json", "dotfiles/ext/sub/x.js", "dotfiles/ext/y.js")
	symlink(t, home, ".pi/agent/extensions", "../../dotfiles/ext")
	symlink(t, home, "dotfiles/ext/odd", "sub/../y.js")

	report := resolveConfigsWith(t, home, nil, policies{allow: []string{filepath.Join(home, "dotfiles")}})

	if root := mustMount(t, report, "/home/caged/.pi"); !root.Staged {
		t.Fatalf("Pi tree = %+v, want a staged copy", root)
	}
	if len(report.Refused) != 0 {
		t.Fatalf("Refused = %v; the live attempt's refusals must be undone", refusedStrings(report.Refused))
	}
}

// TestUndoKeepsRefusalsRecordedBeforeTheAttempt: a refusal recorded before
// Pi's live attempt, then repeated by it, survives the undo.
func TestUndoKeepsRefusalsRecordedBeforeTheAttempt(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, ".claude/settings.json", ".pi/agent/settings.json", "private/x", "dotfiles/y.js")
	symlink(t, home, ".claude/x", "../private/x")
	symlink(t, home, ".pi/agent/x", "../../private/x")
	symlink(t, home, ".pi/agent/odd", "../../dotfiles/../dotfiles/y.js")

	report := resolveConfigsWith(t, home, nil, policies{allow: []string{filepath.Join(home, "dotfiles")}})

	mustRefuse(t, report, filepath.Join(resolved, ".claude/x"))
	mustRefuse(t, report, filepath.Join(resolved, ".pi/agent/x"))
	if root := mustMount(t, report, "/home/caged/.pi"); !root.Staged {
		t.Fatalf("Pi tree = %+v, want a staged copy", root)
	}
	if n := len(refusedStrings(report.Refused)); n != 2 {
		t.Fatalf("Refused = %v, want exactly the two policy refusals", refusedStrings(report.Refused))
	}
}

// TestUnsafeMaskNameHidesItsParent: docker's --mount trims white space from
// the end of a destination, so a mask for "sub/p " would land on "sub/p".
// The nearest parent docker reads correctly is hidden instead, links inside
// it are not followed, and the refusal is a warning.
func TestUnsafeMaskNameHidesItsParent(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, ".claude/settings.json", ".claude/sub/p", ".claude/sub/keep.json", "dotfiles/x.md")
	for _, fifo := range []string{".claude/sub/p ", ".claude/sub/q\r\nz"} {
		if err := syscall.Mkfifo(filepath.Join(home, fifo), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	symlink(t, home, ".claude/sub/l", "../../dotfiles/x.md")

	report := resolveConfigsWith(t, home, nil, policies{allow: []string{filepath.Join(home, "dotfiles")}})

	mount := mustMount(t, report, "/home/caged/.claude")
	if !slices.Equal(mount.MaskPaths, []string{"sub"}) || len(mount.MaskFiles) != 0 {
		t.Fatalf("mount = %+v, want only sub hidden", mount)
	}
	if _, ok := mountAt(report, "/home/caged/dotfiles/x.md"); ok {
		t.Fatal("a link inside the hidden folder was followed")
	}
	refusal := mustRefuse(t, report, filepath.Join(resolved, ".claude/sub/p "))
	if !refusal.Warn || !strings.Contains(refusal.Reason, "is hidden instead") || refusal.Hint == "" {
		t.Fatalf("refusal = %+v", refusal)
	}
}

// TestUnsafeMaskNameInTheRootRefusesTheTreeLoudly: with no parent below the
// root to hide, the tree is not mounted, and the refusal is a warning with a
// hint, so every run and doctor show it. Pi runs from a copy instead.
func TestUnsafeMaskNameInTheRootRefusesTheTreeLoudly(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root can read the unreadable directory")
	}
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, ".claude/settings.json", ".claude/projects/history.jsonl", ".pi/agent/settings.json")
	for _, dir := range []string{".claude/projects ", ".pi/agent "} {
		if err := os.Mkdir(filepath.Join(home, dir), 0); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_ = os.Chmod(filepath.Join(home, ".claude/projects "), 0o700)
		_ = os.Chmod(filepath.Join(home, ".pi/agent "), 0o700)
	})

	report := resolveConfigs(t, home, nil)

	if _, ok := mountAt(report, "/home/caged/.claude"); ok {
		t.Fatalf("~/.claude was mounted with a mask docker would move onto projects: %+v", report.Mounts)
	}
	refusal := mustRefuse(t, report, filepath.Join(resolved, ".claude/projects "))
	if !refusal.Warn || !strings.Contains(refusal.Reason, "is not mounted") || refusal.Hint == "" {
		t.Fatalf("refusal = %+v", refusal)
	}
	if pi := mustMount(t, report, "/home/caged/.pi"); !pi.Staged {
		t.Fatalf("Pi = %+v, want a staged copy", pi)
	}
}

// TestLinkTargetDestinationWithACarriageReturnIsRefused: docker's --mount
// reader turns "\r\n" into "\n", so masks inside such a mount would land
// elsewhere; the link is refused instead.
func TestLinkTargetDestinationWithACarriageReturnIsRefused(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, ".claude/settings.json", "dotfiles/d\r\nz/a.md")
	symlink(t, home, ".claude/x", "../dotfiles/d\r\nz")

	report := resolveConfigsWith(t, home, nil, policies{allow: []string{filepath.Join(home, "dotfiles")}})

	if refusal := mustRefuse(t, report, filepath.Join(resolved, ".claude/x")); !strings.Contains(refusal.Reason, "carriage return") {
		t.Fatalf("refusal = %+v", refusal)
	}
	for _, mount := range report.Mounts {
		if strings.ContainsRune(mount.DestPath, '\r') {
			t.Fatalf("mount with a carriage return: %+v", mount)
		}
	}
}
