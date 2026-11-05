package hostconfig

import (
	"path/filepath"
	"testing"

	"github.com/JeremyOrr4/caged/internal/plan"
)

// TestStowLinkedPiRootIsRefusedLoudlyWithTheEntryToAllow is the owner's
// layout: ~/.pi is one Stow link into the dotfiles and links.allow is empty.
// Pi's whole config is left out, so the refusal is shown on every run, names
// Pi, and carries the links.allow entry that would mount it.
func TestStowLinkedPiRootIsRefusedLoudlyWithTheEntryToAllow(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, "dotfiles/pi/.pi/agent/settings.json")
	symlink(t, home, ".pi", "dotfiles/pi/.pi")
	root := filepath.Join(home, ".pi")

	report := resolveConfigs(t, home, nil)
	if _, ok := mountAt(report, "/home/caged/.pi"); ok {
		t.Fatalf("~/.pi was mounted without links.allow: %+v", report.Mounts)
	}
	refusal := mustRefuse(t, report, root)
	want := plan.Refusal{
		Path:   root,
		Reason: "symlink target ~/dotfiles/pi/.pi is not under links.allow",
		Hint:   `add "- path: ~/dotfiles/pi/.pi" under links.allow in ~/.config/caged/config.yaml`,
		Warn:   true, Agent: "pi", Allow: filepath.Join(resolved, "dotfiles/pi/.pi"),
	}
	if refusal != want {
		t.Fatalf("refusal = %+v\nwant      %+v", refusal, want)
	}

	report = resolveConfigsWith(t, home, nil, policies{allow: []string{refusal.Allow}})
	if mount := mustMount(t, report, "/home/caged/.pi"); !mount.Writable || mount.Staged || mount.Link != root {
		t.Fatalf("mount after allowing %s = %+v; want a live, writable mount via the link", refusal.Allow, mount)
	}
}

// TestLinkInsideAMountedTreeIsNeverOfferedForLinksAllow covers a link an
// agent could plant in its writable config: it is refused quietly as before,
// and never carries an entry caged would offer to add.
func TestLinkInsideAMountedTreeIsNeverOfferedForLinksAllow(t *testing.T) {
	t.Parallel()
	home, _ := testHome(t, nil, nil)
	writeFiles(t, home, ".claude/settings.json", "Documents/private.txt")
	symlink(t, home, ".claude/planted", "../Documents")
	link := filepath.Join(home, ".claude", "planted")

	refusal := mustRefuse(t, resolveConfigs(t, home, nil), link)
	if refusal.Allow != "" || refusal.Warn || refusal.Agent != "" {
		t.Fatalf("refusal of a link inside a mounted tree = %+v; want no Allow, Warn or Agent", refusal)
	}
}

// TestDeniedRootIsLoudButNeverOffered: a config root linked into a built-in
// deny root is shown every run, but no links.allow entry can fix it.
func TestDeniedRootIsLoudButNeverOffered(t *testing.T) {
	t.Parallel()
	home, _ := testHome(t, nil, nil)
	writeFiles(t, home, ".ssh/pi/settings.json")
	symlink(t, home, ".pi", ".ssh/pi")

	refusal := mustRefuse(t, resolveConfigs(t, home, nil), filepath.Join(home, ".pi"))
	if !refusal.Warn || refusal.Agent != "pi" || refusal.Allow != "" {
		t.Fatalf("refusal = %+v; want Warn and Agent pi without Allow", refusal)
	}
}

// TestLinkedSharedFileIsOfferedReadOnly: .gitconfig is mounted read-only, so
// the folder offered for it is offered read_only.
func TestLinkedSharedFileIsOfferedReadOnly(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, "dotfiles/git/.gitconfig")
	symlink(t, home, ".gitconfig", "dotfiles/git/.gitconfig")

	refusal := mustRefuse(t, resolveConfigs(t, home, nil), filepath.Join(home, ".gitconfig"))
	if refusal.Agent != plan.SharedOwner || refusal.Allow != filepath.Join(resolved, "dotfiles/git") || !refusal.AllowReadOnly {
		t.Fatalf("refusal = %+v; want shared, Allow ~/dotfiles/git, read-only", refusal)
	}
}
