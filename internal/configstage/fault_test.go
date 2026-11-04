package configstage

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/JeremyOrr4/caged/internal/plan"
)

// These tests pin what staging and write-back do when a filesystem call
// fails or the host changes something while they run. faultFS injects both.

// linkedFileTree makes a Pi config root whose agent/settings.json is a
// symlink to a dotfiles file, and returns the root, the link target and the
// two staged mounts for them.
func linkedFileTree(t *testing.T) (string, string, []plan.ConfigMount) {
	t.Helper()
	home := t.TempDir()
	root, target := filepath.Join(home, ".pi"), filepath.Join(home, "dotfiles", "settings.json")
	writeTestFile(t, target, "initial", 0o600)
	writeTestFile(t, filepath.Join(root, "agent", "keys.json"), "k", 0o600)
	if err := os.Symlink(target, filepath.Join(root, "agent", "settings.json")); err != nil {
		t.Fatal(err)
	}
	return root, target, []plan.ConfigMount{
		{Staged: true, Root: root, HostPath: root, DestPath: stageDest, MaskFiles: []string{"agent/settings.json"}},
		{Staged: true, Root: root, HostPath: target, DestPath: stageDest + "/agent/settings.json", Link: filepath.Join(root, "agent", "settings.json")},
	}
}

// linkedDirTree makes a Pi config root whose themes directory is a symlink
// to a dotfiles directory, and returns the root, the link target and the two
// staged mounts for them.
func linkedDirTree(t *testing.T) (string, string, []plan.ConfigMount) {
	t.Helper()
	home := t.TempDir()
	root, themes := filepath.Join(home, ".pi"), filepath.Join(home, "dotfiles", "themes")
	writeTestFile(t, filepath.Join(root, "settings.json"), "s", 0o600)
	writeTestFile(t, filepath.Join(themes, "dark.json"), "dark", 0o600)
	if err := os.Symlink(themes, filepath.Join(root, "themes")); err != nil {
		t.Fatal(err)
	}
	return root, themes, []plan.ConfigMount{
		{Staged: true, Root: root, HostPath: root, DestPath: stageDest, MaskPaths: []string{"themes"}},
		{Staged: true, Root: root, HostPath: themes, DestPath: stageDest + "/themes", Link: filepath.Join(root, "themes")},
	}
}

func TestStageReportsFilesystemFaults(t *testing.T) {
	piMounts := func(t *testing.T) (string, string, []plan.ConfigMount) {
		root, mount := piTree(t)
		return root, "", []plan.ConfigMount{mount}
	}
	mountpointErr := func(string, string) string { return "create Pi config staging mountpoint: " + faultText }
	tests := []struct {
		name string
		tree func(t *testing.T) (root, linked string, mounts []plan.ConfigMount)
		fail func(root, linked string) func(op, name string) error
		want func(root, linked string) string
	}{
		{
			name: "the staging directory cannot be created",
			tree: piMounts,
			fail: func(string, string) func(string, string) error {
				return failNth(call{"MkdirTemp", "caged-pi-config-"}, 1) // dir "" is the default temp dir
			},
			want: func(string, string) string { return "create Pi config staging directory: " + faultText },
		},
		{
			name: "a copied directory cannot be created",
			tree: piMounts,
			fail: func(string, string) func(string, string) error { return failNth(call{"MkdirAll", "/mount-1"}, 1) },
			want: func(string, string) string { return "stage Pi config " + stageDest + ": " + faultText },
		},
		{
			name: "a copied file cannot be written",
			tree: piMounts,
			fail: func(string, string) func(string, string) error {
				return failNth(call{"WriteFile", "/mount-1/a.json"}, 1)
			},
			want: func(string, string) string { return "stage Pi config " + stageDest + ": " + faultText },
		},
		{
			name: "the directory of a copied file cannot be created",
			tree: linkedFileTree,
			fail: func(string, string) func(string, string) error {
				return func(op, name string) error {
					if op == "MkdirAll" && strings.HasPrefix(filepath.Base(name), "caged-pi-config-") {
						return errors.New(faultText)
					}
					return nil
				}
			},
			want: func(string, string) string {
				return "stage Pi config " + stageDest + "/agent/settings.json: " + faultText
			},
		},
		{
			name: "a copy cannot be inspected for its snapshot",
			tree: piMounts,
			fail: func(string, string) func(string, string) error { return failNth(call{"Lstat", "/mount-1"}, 1) },
			want: func(root, _ string) string { return "snapshot Pi config " + root + ": " + faultText },
		},
		{
			name: "a copied file cannot be read for the snapshot",
			tree: piMounts,
			fail: func(string, string) func(string, string) error {
				return failNth(call{"ReadFile", "/mount-1/a.json"}, 1)
			},
			want: func(root, _ string) string { return "snapshot Pi config " + root + ": " + faultText },
		},
		{
			name: "a copied single file cannot be read for the snapshot",
			tree: linkedFileTree,
			fail: func(string, string) func(string, string) error { return failNth(call{"ReadFile", "/mount-2"}, 1) },
			want: func(_, target string) string { return "snapshot Pi config " + target + ": " + faultText },
		},
		{
			name: "a copy that turned into a special file",
			tree: piMounts,
			fail: func(string, string) func(string, string) error {
				return onNth(call{"Lstat", "/mount-1"}, 1, func(name string) error {
					if err := os.RemoveAll(name); err != nil {
						return err
					}
					return syscall.Mkfifo(name, 0o600)
				})
			},
			want: func(root, _ string) string {
				return "snapshot Pi config " + root + ": source is not a regular file or directory"
			},
		},
		{
			name: "the directory of a mountpoint cannot be created",
			tree: linkedDirTree,
			fail: func(string, string) func(string, string) error {
				return failAfter(call{"Lstat", "/mount-2"}, call{"MkdirAll", "/mount-1"})
			},
			want: mountpointErr,
		},
		{
			name: "the source of a nested mount cannot be inspected for its mountpoint",
			tree: linkedDirTree,
			fail: func(_, themes string) func(string, string) error {
				return failAfter(call{"Lstat", "/mount-2"}, call{"Lstat", themes})
			},
			want: func(_, themes string) string { return "inspect Pi config mount source " + themes + ": " + faultText },
		},
		{
			name: "a directory mountpoint cannot be created",
			tree: linkedDirTree,
			fail: func(string, string) func(string, string) error {
				return failNth(call{"MkdirAll", "/mount-1/themes"}, 1)
			},
			want: mountpointErr,
		},
		{
			name: "a file mountpoint cannot be created",
			tree: linkedFileTree,
			fail: func(string, string) func(string, string) error {
				return failNth(call{"WriteFile", "/mount-1/agent/settings.json"}, 1)
			},
			want: mountpointErr,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			t.Setenv("TMPDIR", tmp)
			root, linked, mounts := tc.tree(t)
			session, err := New(&faultFS{fail: tc.fail(root, linked)}).Stage(mounts)
			if want := tc.want(root, linked); err == nil || err.Error() != want || session != nil {
				t.Fatalf("Stage() = %v, %v; want nil, %q", session, err, want)
			}
			if entries, err := os.ReadDir(tmp); err != nil || len(entries) != 0 {
				t.Fatalf("temp dir after a failed Stage = %v (err %v), want the copies removed", entries, err)
			}
		})
	}
}

func TestWriteBackReportsFilesystemFaults(t *testing.T) {
	edit := func(name string) func(t *testing.T, stage string) {
		return func(t *testing.T, stage string) { writeTestFile(t, filepath.Join(stage, name), "container", 0o600) }
	}
	remove := func(name string) func(t *testing.T, stage string) {
		return func(t *testing.T, stage string) {
			if err := os.RemoveAll(filepath.Join(stage, name)); err != nil {
				t.Fatal(err)
			}
		}
	}
	none := func(*testing.T, string) {}
	tests := []struct {
		name      string
		container func(t *testing.T, stage string)
		fail      func(root, stage string) func(op, name string) error
	}{
		{"the copy cannot be inspected", none, func(_, stage string) func(string, string) error { return failNth(call{"Lstat", stage}, 1) }},
		{"a copied file cannot be read", none, func(_, stage string) func(string, string) error {
			return failNth(call{"ReadFile", filepath.Join(stage, "a.json")}, 1)
		}},
		{"a host file the container changed cannot be read", edit("a.json"), func(root, _ string) func(string, string) error {
			return failNth(call{"ReadFile", filepath.Join(root, "a.json")}, 1)
		}},
		{"a host file the container changed cannot be inspected", edit("a.json"), func(root, _ string) func(string, string) error {
			return failNth(call{"Lstat", filepath.Join(root, "a.json")}, 1)
		}},
		{"the copied directory cannot be listed", none, func(_, stage string) func(string, string) error { return failNth(call{"ReadDir", stage}, 1) }},
		{"the host directory cannot be listed", none, func(root, _ string) func(string, string) error { return failNth(call{"ReadDir", root}, 1) }},
		{"the host directory cannot be inspected", none, func(root, _ string) func(string, string) error { return failNth(call{"Lstat", root}, 1) }},
		{"a directory the container created cannot be created on the host", edit("newdir/x.json"), func(root, _ string) func(string, string) error {
			return failNth(call{"Mkdir", filepath.Join(root, "newdir")}, 1)
		}},
		{"a directory the container created cannot get its mode on the host", edit("newdir/x.json"), func(root, _ string) func(string, string) error {
			return failNth(call{"Chmod", filepath.Join(root, "newdir")}, 1)
		}},
		{"a host file the container deleted cannot be read", remove("a.json"), func(root, _ string) func(string, string) error {
			return failNth(call{"ReadFile", filepath.Join(root, "a.json")}, 1)
		}},
		{"a host file the container deleted cannot be inspected", remove("a.json"), func(root, _ string) func(string, string) error {
			return failNth(call{"Lstat", filepath.Join(root, "a.json")}, 1)
		}},
		{"a host directory the container deleted cannot be listed", remove("sub"), func(root, _ string) func(string, string) error {
			return failNth(call{"ReadDir", filepath.Join(root, "sub")}, 1)
		}},
		{"a host directory the container deleted cannot be listed again once emptied", remove("sub"), func(root, _ string) func(string, string) error {
			return failNth(call{"ReadDir", filepath.Join(root, "sub")}, 2)
		}},
		{"the directory of a new host file cannot be inspected", edit("new.json"), func(root, _ string) func(string, string) error {
			return failNth(call{"Lstat", root}, 2) // the first is the directory's own write-back
		}},
		{"a new host file cannot be inspected before it is written", edit("new.json"), func(root, _ string) func(string, string) error {
			return failNth(call{"Lstat", filepath.Join(root, "new.json")}, 2) // the first finds it missing
		}},
		{"a new copied file cannot be read when it is written", edit("new.json"), func(_, stage string) func(string, string) error {
			return failNth(call{"ReadFile", filepath.Join(stage, "new.json")}, 2) // the first takes its state
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root, mount := piTree(t)
			fsys := &faultFS{}
			session, err := New(fsys).Stage([]plan.ConfigMount{mount})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = session.Cleanup() }) // best effort: finish normally removes the copies
			stage := session.Mounts()[0].HostPath
			tc.container(t, stage)
			fsys.fail = tc.fail(root, stage)
			want := persistError(root, faultText)
			if err := finish(session); err == nil || err.Error() != want {
				t.Fatalf("finish() error = %v, want %q", err, want)
			}
			assertMissing(t, filepath.Dir(stage))
		})
	}
}

// The host can create a symlink after write-back found the path missing and
// before it writes the file: the symlink is left alone.
func TestWriteBackNeverReplacesASymlinkTheHostCreatesMeanwhile(t *testing.T) {
	root, mount := piTree(t)
	target := filepath.Join(filepath.Dir(root), "target.json")
	writeTestFile(t, target, "target", 0o600)
	fsys := &faultFS{}
	session, err := New(fsys).Stage([]plan.ConfigMount{mount})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Cleanup() }) // best effort: finish normally removes the copies
	writeTestFile(t, filepath.Join(session.Mounts()[0].HostPath, "new.json"), "container", 0o600)
	link := filepath.Join(root, "new.json")
	// The first Lstat finds new.json missing; the second checks it before writing.
	fsys.fail = onNth(call{"Lstat", link}, 2, func(string) error { return os.Symlink(target, link) })
	want := persistError(root, "host config changed type during container run at "+link)
	if err := finish(session); err == nil || err.Error() != want {
		t.Fatalf("finish() error = %v, want %q", err, want)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("host symlink changed: info=%v err=%v", info, err)
	}
	if got := readTestFile(t, target); got != "target" {
		t.Fatalf("symlink target = %q, want it untouched", got)
	}
}

// B-CS-1: a host directory that cannot be listed is a failed directory, so
// nothing inside it is written back.
func TestWriteBackSkipsAHostDirectoryThatCannotBeListed(t *testing.T) {
	root, mount := piTree(t)
	fsys := &faultFS{}
	session, err := New(fsys).Stage([]plan.ConfigMount{mount})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Cleanup() }) // best effort: finish normally removes the copies
	stage := session.Mounts()[0].HostPath
	writeTestFile(t, filepath.Join(stage, "sub", "c.json"), "container", 0o600)
	writeTestFile(t, filepath.Join(stage, "sub", "d.json"), "new", 0o600)
	fsys.fail = failNth(call{"ReadDir", filepath.Join(root, "sub")}, 1)
	if err := finish(session); err == nil || err.Error() != persistError(root, faultText) {
		t.Fatalf("finish() error = %v, want %q", err, persistError(root, faultText))
	}
	if got := readTestFile(t, filepath.Join(root, "sub", "c.json")); got != "c" {
		t.Errorf("sub/c.json = %q, want the host version", got)
	}
	assertMissing(t, filepath.Join(root, "sub", "d.json"))
}

// A copied entry that is gone by the time write-back inspects it is skipped:
// the host keeps its version.
func TestWriteBackSkipsACopiedEntryThatVanishes(t *testing.T) {
	root, mount := piTree(t)
	fsys := &faultFS{}
	session, err := New(fsys).Stage([]plan.ConfigMount{mount})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Cleanup() }) // best effort: finish normally removes the copies
	copied := filepath.Join(session.Mounts()[0].HostPath, "b.json")
	writeTestFile(t, copied, "container", 0o600)
	fsys.fail = onNth(call{"Lstat", copied}, 1, func(string) error { return os.Remove(copied) })
	if err := finish(session); err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, filepath.Join(root, "b.json")); got != "b" {
		t.Fatalf("b.json = %q, want the host version", got)
	}
}

// The host can delete an entry the container deleted while write-back runs:
// it is already gone, so there is nothing to report.
func TestWriteBackAcceptsADeletionTheHostMakesMeanwhile(t *testing.T) {
	root, mount := piTree(t)
	fsys := &faultFS{}
	session, err := New(fsys).Stage([]plan.ConfigMount{mount})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Cleanup() }) // best effort: finish normally removes the copies
	if err := os.Remove(filepath.Join(session.Mounts()[0].HostPath, "a.json")); err != nil {
		t.Fatal(err)
	}
	host := filepath.Join(root, "a.json")
	fsys.fail = onNth(call{"Lstat", host}, 1, func(string) error { return os.Remove(host) })
	if err := finish(session); err != nil {
		t.Fatal(err)
	}
	assertMissing(t, host)
}

// The host can create a directory the container created while write-back
// runs: write-back uses it.
func TestWriteBackUsesADirectoryTheHostCreatesMeanwhile(t *testing.T) {
	root, mount := piTree(t)
	fsys := &faultFS{}
	session, err := New(fsys).Stage([]plan.ConfigMount{mount})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Cleanup() }) // best effort: finish normally removes the copies
	writeTestFile(t, filepath.Join(session.Mounts()[0].HostPath, "newdir", "x.json"), "container", 0o600)
	dir := filepath.Join(root, "newdir")
	created := false
	fsys.fail = onNth(call{"Mkdir", dir}, 1, func(string) error {
		created = true
		return os.Mkdir(dir, 0o700)
	})
	if err := finish(session); err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("write-back never created the directory, so the host never raced it")
	}
	if got := readTestFile(t, filepath.Join(dir, "x.json")); got != "container" {
		t.Fatalf("newdir/x.json = %q, want the container's file", got)
	}
}

// A directory's write-back lists the copy, then the host, then removes what
// the container deleted, and only then writes back the copy's entries.
func TestWriteBackListsBothSidesThenDeletesThenWrites(t *testing.T) {
	root, mount := piTree(t)
	fsys := &faultFS{}
	session, err := New(fsys).Stage([]plan.ConfigMount{mount})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Cleanup() }) // best effort: finish normally removes the copies
	stage := session.Mounts()[0].HostPath
	// b.json sorts after a.json, so name order alone cannot put its deletion first.
	if err := os.Remove(filepath.Join(stage, "b.json")); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(stage, "a.json"), "container", 0o600)
	steps := map[call]string{
		{"ReadDir", stage}:                        "list the copy",
		{"ReadDir", root}:                         "list the host",
		{"Lstat", filepath.Join(root, "b.json")}:  "delete b.json",
		{"Lstat", filepath.Join(stage, "a.json")}: "write a.json",
		{"ReadDir", filepath.Join(stage, "sub")}:  "list the copy of sub",
	}
	var order []string
	fsys.fail = func(op, name string) error {
		if step, ok := steps[call{op, name}]; ok && !slices.Contains(order, step) {
			order = append(order, step)
		}
		return nil
	}
	if err := finish(session); err != nil {
		t.Fatal(err)
	}
	want := []string{"list the copy", "list the host", "delete b.json", "write a.json", "list the copy of sub"}
	if !slices.Equal(order, want) {
		t.Fatalf("write-back order = %q, want %q", order, want)
	}
	assertMissing(t, filepath.Join(root, "b.json"))
	if got := readTestFile(t, filepath.Join(root, "a.json")); got != "container" {
		t.Fatalf("a.json = %q, want the container's edit", got)
	}
}
