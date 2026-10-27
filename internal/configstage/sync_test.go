package configstage

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/JeremyOrr4/caged/internal/plan"
)

// These tests pin how the container's changes are written back to the host.

func TestStageWritesContainerDeletionsBack(t *testing.T) {
	root, mount := piTree(t)
	staged, persist := stageTree(t, []plan.ConfigMount{mount})
	if err := os.Remove(filepath.Join(staged[0].HostPath, "a.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(staged[0].HostPath, "sub")); err != nil {
		t.Fatal(err)
	}
	if err := persist(); err != nil {
		t.Fatal(err)
	}
	assertMissing(t, filepath.Join(root, "a.json"))
	assertMissing(t, filepath.Join(root, "sub"))
	if got := readTestFile(t, filepath.Join(root, "b.json")); got != "b" {
		t.Fatalf("b.json = %q, want it kept", got)
	}
}

func TestStageKeepsHostChangesTheContainerDidNotTouch(t *testing.T) {
	root, mount := piTree(t)
	_, persist := stageTree(t, []plan.ConfigMount{mount})
	writeTestFile(t, filepath.Join(root, "b.json"), "host edit", 0o600)
	writeTestFile(t, filepath.Join(root, "host-new.json"), "new", 0o600)
	if err := persist(); err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, filepath.Join(root, "b.json")); got != "host edit" {
		t.Fatalf("b.json = %q, want the host edit", got)
	}
	if got := readTestFile(t, filepath.Join(root, "host-new.json")); got != "new" {
		t.Fatalf("host-new.json = %q, want it kept", got)
	}
}

func TestStageWritesModeChangesBack(t *testing.T) {
	root, mount := piTree(t)
	staged, persist := stageTree(t, []plan.ConfigMount{mount})
	if err := os.Chmod(filepath.Join(staged[0].HostPath, "a.json"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(staged[0].HostPath, "sub"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := persist(); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]os.FileMode{"a.json": 0o640, "sub": 0o750} {
		info, err := os.Lstat(filepath.Join(root, path))
		if err != nil || info.Mode().Perm() != want {
			t.Fatalf("%s mode = %v (err=%v), want %v", path, info.Mode().Perm(), err, want)
		}
	}
}

func TestStageWritesNestedDeletionsBack(t *testing.T) {
	root, mount := piTree(t)
	writeTestFile(t, filepath.Join(root, "sub", "keep.json"), "keep", 0o600)
	writeTestFile(t, filepath.Join(root, "c.json"), "top-level namesake", 0o600)
	staged, persist := stageTree(t, []plan.ConfigMount{mount})
	if err := os.Remove(filepath.Join(staged[0].HostPath, "sub", "c.json")); err != nil {
		t.Fatal(err)
	}
	if err := persist(); err != nil {
		t.Fatal(err)
	}
	assertMissing(t, filepath.Join(root, "sub", "c.json"))
	if got := readTestFile(t, filepath.Join(root, "sub", "keep.json")); got != "keep" {
		t.Fatalf("sub/keep.json = %q, want it kept", got)
	}
	if got := readTestFile(t, filepath.Join(root, "c.json")); got != "top-level namesake" {
		t.Fatalf("c.json = %q, want it untouched", got)
	}
}

func TestStageIgnoresContainerFilesAtMaskedPaths(t *testing.T) {
	root, mount := piTree(t)
	staged, persist := stageTree(t, []plan.ConfigMount{mount})
	writeTestFile(t, filepath.Join(staged[0].HostPath, "masked.txt"), "container", 0o600)
	if err := persist(); err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, filepath.Join(root, "masked.txt")); got != "secret" {
		t.Fatalf("masked host file = %q, want it untouched", got)
	}
}

func TestStageIgnoresContainerSymlinksAndSpecialFiles(t *testing.T) {
	root, mount := piTree(t)
	staged, persist := stageTree(t, []plan.ConfigMount{mount})
	if err := os.Symlink("a.json", filepath.Join(staged[0].HostPath, "new-link")); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(staged[0].HostPath, "new-fifo"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := persist(); err != nil {
		t.Fatal(err)
	}
	assertMissing(t, filepath.Join(root, "new-link"))
	assertMissing(t, filepath.Join(root, "new-fifo"))
}

func TestStageAcceptsTheSameModeChangeOnBothSides(t *testing.T) {
	root, mount := piTree(t)
	staged, persist := stageTree(t, []plan.ConfigMount{mount})
	for _, dir := range []string{staged[0].HostPath, root} {
		if err := os.Chmod(filepath.Join(dir, "sub"), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	if err := persist(); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(filepath.Join(root, "sub")); err != nil || info.Mode().Perm() != 0o750 {
		t.Fatalf("sub: info=%v err=%v, want mode 0750", info, err)
	}
}
