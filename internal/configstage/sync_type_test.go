package configstage

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/JeremyOrr4/caged/internal/plan"
)

// These tests pin how write-back handles type changes on either side: the
// host version is kept, and the side that changed the type is reported.

func TestStageRefusesATypeChange(t *testing.T) {
	tests := []struct {
		name      string
		change    func(t *testing.T, stage string) // the container's change
		host      func(t *testing.T, root string)
		entry     string
		container bool // the container made the type change, not the host
		check     func(t *testing.T, root string)
	}{
		{
			name: "the container replaced a directory with a file",
			change: func(t *testing.T, stage string) {
				if err := os.RemoveAll(filepath.Join(stage, "sub")); err != nil {
					t.Fatal(err)
				}
				writeTestFile(t, filepath.Join(stage, "sub"), "x", 0o600)
			},
			host:      func(*testing.T, string) {},
			entry:     "sub",
			container: true,
			check: func(t *testing.T, root string) {
				if got := readTestFile(t, filepath.Join(root, "sub", "c.json")); got != "c" {
					t.Fatalf("sub/c.json = %q, want it kept", got)
				}
			},
		},
		{
			name:      "the container replaced a file with a directory",
			change:    fileToDir("a.json"),
			host:      func(*testing.T, string) {},
			entry:     "a.json",
			container: true,
			check: func(t *testing.T, root string) {
				if got := readTestFile(t, filepath.Join(root, "a.json")); got != "a" {
					t.Fatalf("a.json = %q, want it kept", got)
				}
			},
		},
		{
			name:      "both replaced a file with a directory",
			change:    fileToDir("a.json"),
			host:      fileToDir("a.json"),
			entry:     "a.json",
			container: true,
			check: func(t *testing.T, root string) {
				if info, err := os.Lstat(filepath.Join(root, "a.json")); err != nil || !info.IsDir() {
					t.Fatalf("a.json: info=%v err=%v, want the host's directory kept", info, err)
				}
			},
		},
		{
			// Nothing is ever deleted through a host symlink.
			name: "the host replaced a directory with a symlink",
			change: func(t *testing.T, stage string) {
				if err := os.Remove(filepath.Join(stage, "sub", "c.json")); err != nil {
					t.Fatal(err)
				}
			},
			host: func(t *testing.T, root string) {
				other := filepath.Join(filepath.Dir(root), "other")
				writeTestFile(t, filepath.Join(other, "c.json"), "c", 0o600)
				if err := os.RemoveAll(filepath.Join(root, "sub")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(other, filepath.Join(root, "sub")); err != nil {
					t.Fatal(err)
				}
			},
			entry: "sub",
			check: func(t *testing.T, root string) {
				if got := readTestFile(t, filepath.Join(filepath.Dir(root), "other", "c.json")); got != "c" {
					t.Fatalf("the symlink target's c.json = %q, want it kept", got)
				}
			},
		},
		{
			name:   "the host replaced a file the container changed with a directory",
			change: func(t *testing.T, stage string) { writeTestFile(t, filepath.Join(stage, "a.json"), "container", 0o600) },
			host:   fileToDir("a.json"),
			entry:  "a.json",
			check: func(t *testing.T, root string) {
				if info, err := os.Lstat(filepath.Join(root, "a.json")); err != nil || !info.IsDir() {
					t.Fatalf("a.json: info=%v err=%v, want the host's directory kept", info, err)
				}
			},
		},
		{
			name: "the host created a symlink where the container created a file",
			change: func(t *testing.T, stage string) {
				writeTestFile(t, filepath.Join(stage, "new.json"), "container", 0o600)
			},
			host: func(t *testing.T, root string) {
				writeTestFile(t, filepath.Join(filepath.Dir(root), "target.json"), "target", 0o600)
				if err := os.Symlink(filepath.Join(filepath.Dir(root), "target.json"), filepath.Join(root, "new.json")); err != nil {
					t.Fatal(err)
				}
			},
			entry: "new.json",
			check: func(t *testing.T, root string) {
				if got := readTestFile(t, filepath.Join(filepath.Dir(root), "target.json")); got != "target" {
					t.Fatalf("symlink target = %q, want it untouched", got)
				}
				if info, err := os.Lstat(filepath.Join(root, "new.json")); err != nil || info.Mode()&os.ModeSymlink == 0 {
					t.Fatalf("new.json: info=%v err=%v, want the host's symlink kept", info, err)
				}
			},
		},
		{
			name:   "the host replaced a file the container changed with a FIFO",
			change: func(t *testing.T, stage string) { writeTestFile(t, filepath.Join(stage, "a.json"), "container", 0o600) },
			host:   fileToFIFO("a.json"),
			entry:  "a.json",
			check: func(t *testing.T, root string) {
				if info, err := os.Lstat(filepath.Join(root, "a.json")); err != nil || info.Mode()&os.ModeNamedPipe == 0 {
					t.Fatalf("a.json: info=%v err=%v, want the host's FIFO kept", info, err)
				}
			},
		},
		{
			name:      "the container replaced a file with a FIFO",
			change:    fileToFIFO("a.json"),
			host:      func(*testing.T, string) {},
			entry:     "a.json",
			container: true,
			check: func(t *testing.T, root string) {
				if got := readTestFile(t, filepath.Join(root, "a.json")); got != "a" {
					t.Fatalf("a.json = %q, want the host's file kept", got)
				}
			},
		},
		{
			name: "the container replaced a file with a symlink",
			change: func(t *testing.T, stage string) {
				if err := os.Remove(filepath.Join(stage, "a.json")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("b.json", filepath.Join(stage, "a.json")); err != nil {
					t.Fatal(err)
				}
			},
			host:      func(*testing.T, string) {},
			entry:     "a.json",
			container: true,
			check: func(t *testing.T, root string) {
				if info, err := os.Lstat(filepath.Join(root, "a.json")); err != nil || !info.Mode().IsRegular() || readTestFile(t, filepath.Join(root, "a.json")) != "a" {
					t.Fatalf("a.json: info=%v err=%v, want the host's file kept", info, err)
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root, mount := piTree(t)
			staged, persist := stageTree(t, []plan.ConfigMount{mount})
			tc.change(t, staged[0].HostPath)
			tc.host(t, root)
			host := filepath.Join(root, tc.entry)
			want := persistError(root, "host config changed type during container run at "+host)
			if tc.container {
				want = persistError(root, "container changed the type of "+host+"; leaving the host version intact")
			}
			if err := persist(); err == nil || err.Error() != want {
				t.Fatalf("persist() error = %v, want %q", err, want)
			}
			tc.check(t, root)
		})
	}
}

// Q-CS-4: a host file replaced by a symlink is a conflict, like any other
// type change; the link and its target are left alone.
func TestStageRefusesAHostFileReplacedByASymlink(t *testing.T) {
	root, mount := piTree(t)
	staged, persist := stageTree(t, []plan.ConfigMount{mount})
	target := filepath.Join(filepath.Dir(root), "target.json")
	writeTestFile(t, target, "target", 0o600)
	if err := os.Remove(filepath.Join(root, "a.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "a.json")); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(staged[0].HostPath, "a.json"), "container", 0o600)
	want := persistError(root, "host config changed type during container run at "+filepath.Join(root, "a.json"))
	if err := persist(); err == nil || err.Error() != want {
		t.Fatalf("persist() error = %v, want %q", err, want)
	}
	if got := readTestFile(t, target); got != "target" {
		t.Fatalf("symlink target = %q, want it untouched", got)
	}
	if info, err := os.Lstat(filepath.Join(root, "a.json")); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("host symlink changed: info=%v err=%v", info, err)
	}
}

// Q-CS-7: a type change the host made to an entry the container deleted is a
// conflict; the host entry is kept.
func TestStageRefusesAHostTypeChangeTheContainerDeleted(t *testing.T) {
	tests := []struct {
		name  string
		entry string
		host  func(t *testing.T, path string)
		kept  os.FileMode // the type bits the host entry must still have
		check func(t *testing.T, path string)
	}{
		{
			name: "a file replaced by a symlink", entry: "a.json", kept: os.ModeSymlink,
			host: func(t *testing.T, path string) {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("b.json", path); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "a file replaced by a named pipe", entry: "a.json", kept: os.ModeNamedPipe,
			host: func(t *testing.T, path string) {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := syscall.Mkfifo(path, 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "a directory replaced by a file", entry: "sub", kept: 0,
			host: func(t *testing.T, path string) {
				if err := os.RemoveAll(path); err != nil {
					t.Fatal(err)
				}
				writeTestFile(t, path, "host", 0o600)
			},
		},
		{
			// Nothing is ever deleted through a host symlink.
			name: "a directory replaced by a symlink", entry: "sub", kept: os.ModeSymlink,
			host: func(t *testing.T, path string) {
				other := filepath.Join(filepath.Dir(filepath.Dir(path)), "other")
				writeTestFile(t, filepath.Join(other, "c.json"), "c", 0o600)
				if err := os.RemoveAll(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(other, path); err != nil {
					t.Fatal(err)
				}
			},
			check: func(t *testing.T, path string) {
				if got := readTestFile(t, filepath.Join(filepath.Dir(filepath.Dir(path)), "other", "c.json")); got != "c" {
					t.Fatalf("the symlink target's c.json = %q, want it kept", got)
				}
			},
		},
		{
			name: "a file replaced by a directory", entry: "a.json", kept: os.ModeDir,
			host: func(t *testing.T, path string) { fileToDir(filepath.Base(path))(t, filepath.Dir(path)) },
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root, mount := piTree(t)
			staged, persist := stageTree(t, []plan.ConfigMount{mount})
			if err := os.RemoveAll(filepath.Join(staged[0].HostPath, tc.entry)); err != nil {
				t.Fatal(err)
			}
			host := filepath.Join(root, tc.entry)
			tc.host(t, host)
			want := persistError(root, "host config changed type during container run at "+host)
			if err := persist(); err == nil || err.Error() != want {
				t.Fatalf("persist() error = %v, want %q", err, want)
			}
			if info, err := os.Lstat(host); err != nil || info.Mode().Type() != tc.kept {
				t.Fatalf("host %s: info=%v err=%v, want it kept with type %v", tc.entry, info, err, tc.kept)
			}
			if tc.check != nil {
				tc.check(t, host)
			}
		})
	}
}

func TestStageRefusesAParentDirectoryReplacedByASymlink(t *testing.T) {
	_, target, mounts := linkedFileTree(t)
	staged, persist := stageTree(t, mounts)
	writeTestFile(t, staged[1].HostPath, "updated", 0o600)
	dotfiles, other := filepath.Dir(target), filepath.Join(filepath.Dir(filepath.Dir(target)), "other")
	writeTestFile(t, filepath.Join(other, "settings.json"), "initial", 0o600)
	if err := os.RemoveAll(dotfiles); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, dotfiles); err != nil {
		t.Fatal(err)
	}
	want := persistError(target, "destination is no longer a directory")
	if err := persist(); err == nil || err.Error() != want {
		t.Fatalf("persist() error = %v, want %q", err, want)
	}
	if got := readTestFile(t, filepath.Join(other, "settings.json")); got != "initial" {
		t.Fatalf("file behind the symlinked directory = %q, want it untouched", got)
	}
}
