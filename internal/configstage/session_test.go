package configstage

import (
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
)

// These tests pin how a session ends: Sync writes back, repeatably, and
// Cleanup removes the copies.

// Sync can run again: each call writes what the container changed since the
// last one, without reporting its own earlier writes as host changes.
func TestSyncTwiceWritesLaterChangesWithoutFalseConflicts(t *testing.T) {
	root, mount := piTree(t)
	session, err := New(system.FS{}).Stage([]plan.ConfigMount{mount})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Cleanup() }) // best effort: the test checks Sync only
	stage := session.Mounts()[0].HostPath
	writeTestFile(t, filepath.Join(stage, "a.json"), "first", 0o600)
	writeTestFile(t, filepath.Join(stage, "new", "n.json"), "new", 0o600)
	if err := session.Sync(); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(stage, "a.json"), "second", 0o600)
	writeTestFile(t, filepath.Join(stage, "new", "n.json"), "newer", 0o600)
	if err := os.Remove(filepath.Join(stage, "b.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(stage, "sub"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := session.Sync(); err != nil {
		t.Fatalf("second Sync() error = %v, want none", err)
	}
	for name, content := range map[string]string{"a.json": "second", "new/n.json": "newer", "sub/c.json": "c"} {
		if got := readTestFile(t, filepath.Join(root, filepath.FromSlash(name))); got != content {
			t.Errorf("%s = %q, want %q", name, got, content)
		}
	}
	assertMissing(t, filepath.Join(root, "b.json"))
	if info, err := os.Lstat(filepath.Join(root, "sub")); err != nil || info.Mode().Perm() != 0o750 {
		t.Fatalf("sub: info=%v err=%v, want mode 0750", info, err)
	}
}

// After a Sync, the baseline is the copy as Sync wrote it: the same as a new
// snapshot of the copy.
func TestSyncRecordsTheSameBaselineAsASnapshot(t *testing.T) {
	_, mount := piTree(t)
	session, err := New(system.FS{}).Stage([]plan.ConfigMount{mount})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Cleanup() }) // best effort: the test checks Sync only
	stage := session.Mounts()[0].HostPath
	writeTestFile(t, filepath.Join(stage, "a.json"), "edited", 0o640)
	writeTestFile(t, filepath.Join(stage, "new", "deep", "n.json"), "new", 0o600)
	writeTestFile(t, filepath.Join(stage, "masked.txt"), "container", 0o600)
	if err := os.Remove(filepath.Join(stage, "b.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("a.json", filepath.Join(stage, "link")); err != nil {
		t.Fatal(err)
	}
	if err := session.Sync(); err != nil {
		t.Fatal(err)
	}
	tree := session.trees[0]
	want, err := takeSnapshot(system.FS{}, tree.copy, tree.excluded)
	if err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(tree.baseline, want) {
		t.Fatalf("baseline after Sync = %v, want the copy's snapshot %v", tree.baseline, want)
	}
}

// A Sync that fails keeps the tree's baseline, so the next Sync reports the
// same conflict instead of taking the container's version.
func TestAFailedSyncKeepsTheBaseline(t *testing.T) {
	root, mount := piTree(t)
	session, err := New(system.FS{}).Stage([]plan.ConfigMount{mount})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Cleanup() }) // best effort: the test checks Sync only
	stage := session.Mounts()[0].HostPath
	writeTestFile(t, filepath.Join(stage, "a.json"), "container", 0o600)
	writeTestFile(t, filepath.Join(stage, "b.json"), "container", 0o600)
	writeTestFile(t, filepath.Join(root, "a.json"), "host", 0o600)
	want := persistError(root, "host and container both changed "+filepath.Join(root, "a.json")+"; leaving the host version intact")
	for i := range 2 {
		if err := session.Sync(); err == nil || err.Error() != want {
			t.Fatalf("Sync() %d error = %v, want %q", i+1, err, want)
		}
	}
	for name, content := range map[string]string{"a.json": "host", "b.json": "container"} {
		if got := readTestFile(t, filepath.Join(root, name)); got != content {
			t.Errorf("%s = %q, want %q", name, got, content)
		}
	}
}

// A host change made between two Syncs is still a host change: the second
// Sync keeps it and reports the conflict.
func TestSyncKeepsAHostChangeMadeBetweenSyncs(t *testing.T) {
	root, mount := piTree(t)
	session, err := New(system.FS{}).Stage([]plan.ConfigMount{mount})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Cleanup() }) // best effort: the test checks Sync only
	stage := session.Mounts()[0].HostPath
	writeTestFile(t, filepath.Join(stage, "a.json"), "first", 0o600)
	if err := session.Sync(); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "a.json"), "host", 0o600)
	writeTestFile(t, filepath.Join(stage, "a.json"), "second", 0o600)
	want := persistError(root, "host and container both changed "+filepath.Join(root, "a.json")+"; leaving the host version intact")
	if err := session.Sync(); err == nil || err.Error() != want {
		t.Fatalf("second Sync() error = %v, want %q", err, want)
	}
	if got := readTestFile(t, filepath.Join(root, "a.json")); got != "host" {
		t.Fatalf("a.json = %q, want the host version", got)
	}
}

// Each copy advances on its own: a failed tree keeps its baseline while the
// other, here a single staged file, takes later changes without a false
// conflict.
func TestSyncAdvancesEachCopyOnItsOwn(t *testing.T) {
	root, target, mounts := linkedFileTree(t)
	session, err := New(system.FS{}).Stage(mounts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Cleanup() }) // best effort: the test checks Sync only
	staged := session.Mounts()
	writeTestFile(t, filepath.Join(staged[0].HostPath, "agent", "keys.json"), "container", 0o600)
	writeTestFile(t, filepath.Join(root, "agent", "keys.json"), "host", 0o600)
	want := persistError(root, "host and container both changed "+filepath.Join(root, "agent", "keys.json")+"; leaving the host version intact")
	for i, content := range []string{"first", "second"} {
		writeTestFile(t, staged[1].HostPath, content, 0o600)
		if err := session.Sync(); err == nil || err.Error() != want {
			t.Fatalf("Sync() %d error = %v, want %q", i+1, err, want)
		}
		if got := readTestFile(t, target); got != content {
			t.Fatalf("after Sync %d the linked file = %q, want %q", i+1, got, content)
		}
	}
}

func TestCleanupRemovesTheStagingDirectory(t *testing.T) {
	_, mount := piTree(t)
	session, err := New(system.FS{}).Stage([]plan.ConfigMount{mount})
	if err != nil {
		t.Fatal(err)
	}
	staged := session.Mounts()
	if err := session.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := session.Cleanup(); err != nil {
		t.Fatal(err)
	}
	assertMissing(t, filepath.Dir(staged[0].HostPath))
}

func TestCleanupRemovesCopiesWithoutWritingBack(t *testing.T) {
	root, mount := piTree(t)
	session, err := New(system.FS{}).Stage([]plan.ConfigMount{mount})
	if err != nil {
		t.Fatal(err)
	}
	staged := session.Mounts()
	writeTestFile(t, filepath.Join(staged[0].HostPath, "a.json"), "container", 0o600)
	if err := session.Cleanup(); err != nil {
		t.Fatal(err)
	}
	assertMissing(t, filepath.Dir(staged[0].HostPath))
	if got := readTestFile(t, filepath.Join(root, "a.json")); got != "a" {
		t.Fatalf("a.json = %q, want the host version", got)
	}
}

func TestCleanupRemovesCopiesAfterAFailedSync(t *testing.T) {
	root, mount := piTree(t)
	staged, persist := stageTree(t, []plan.ConfigMount{mount})
	writeTestFile(t, filepath.Join(staged[0].HostPath, "a.json"), "container", 0o600)
	writeTestFile(t, filepath.Join(root, "a.json"), "host", 0o600)
	want := persistError(root, "host and container both changed "+filepath.Join(root, "a.json")+"; leaving the host version intact")
	if err := persist(); err == nil || err.Error() != want {
		t.Fatalf("finish() error = %v, want %q", err, want)
	}
	assertMissing(t, filepath.Dir(staged[0].HostPath))
}

// TestSessionCopiedSaysWhetherAnythingRunsFromACopy: docker tells the user
// that a signal lost the container's config changes only when they live in a
// copy (WB-T3).
func TestSessionCopiedSaysWhetherAnythingRunsFromACopy(t *testing.T) {
	_, mount := piTree(t)
	session, err := New(system.FS{}).Stage([]plan.ConfigMount{mount})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Cleanup() }) // best effort: the test's temp dir goes anyway
	if !session.Copied() {
		t.Error("Copied() = false for a staged tree that was copied")
	}
	plain := plan.ConfigMount{HostPath: "/h/.x", DestPath: stageDest + "/x"}
	unstaged, err := New(system.FS{}).Stage([]plan.ConfigMount{plain})
	if err != nil {
		t.Fatal(err)
	}
	if unstaged.Copied() || Unstaged(nil).Copied() {
		t.Error("Copied() = true with nothing staged")
	}
}
