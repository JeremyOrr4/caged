package configstage

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
)

// These tests pin how staged config is copied and written back.

const stageDest = "/home/caged/.pi"

// stageTree stages mounts, removes the staging directory when the test ends,
// and returns the staged mounts and a func that finishes the session.
func stageTree(t *testing.T, mounts []plan.ConfigMount) ([]plan.ConfigMount, func() error) {
	t.Helper()
	session, err := New(system.FS{}).Stage(mounts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Cleanup(); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	return session.Mounts(), func() error { return finish(session) }
}

// finish is what docker's RunContainer does once the container has exited:
// Sync, then Cleanup, joining their errors.
func finish(session *Session) error { return errors.Join(session.Sync(), session.Cleanup()) }

func writeTestFile(t *testing.T, path, data string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func assertMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("%s still exists (err=%v)", path, err)
	}
}

// piTree makes a Pi config root with a.json, b.json, sub/c.json and a masked
// file, and returns the root and its single masked mount.
func piTree(t *testing.T) (string, plan.ConfigMount) {
	t.Helper()
	root := filepath.Join(t.TempDir(), ".pi")
	writeTestFile(t, filepath.Join(root, "a.json"), "a", 0o600)
	writeTestFile(t, filepath.Join(root, "b.json"), "b", 0o600)
	writeTestFile(t, filepath.Join(root, "sub", "c.json"), "c", 0o600)
	writeTestFile(t, filepath.Join(root, "masked.txt"), "secret", 0o600)
	return root, plan.ConfigMount{Staged: true, Root: root, HostPath: root, DestPath: stageDest, MaskFiles: []string{"masked.txt"}}
}

// persistError is the write-back error for a failure in the tree staged from root.
func persistError(root, failure string) string { return "persist Pi config " + root + ": " + failure }

// fileToFIFO returns a change that replaces the file name, in the tree it is
// given, with a FIFO.
func fileToFIFO(name string) func(t *testing.T, tree string) {
	return func(t *testing.T, tree string) {
		if err := os.Remove(filepath.Join(tree, name)); err != nil {
			t.Fatal(err)
		}
		if err := syscall.Mkfifo(filepath.Join(tree, name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// fileToDir returns a change that replaces the file name, in the tree it is
// given, with an empty directory.
func fileToDir(name string) func(t *testing.T, tree string) {
	return func(t *testing.T, tree string) {
		if err := os.Remove(filepath.Join(tree, name)); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(tree, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
}

func TestStageWithoutMasksOrChildrenStagesNothing(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".pi")
	writeTestFile(t, filepath.Join(root, "a.json"), "a", 0o600)
	mounts := []plan.ConfigMount{{Staged: true, Root: root, HostPath: root, DestPath: stageDest}}
	staged, persist := stageTree(t, mounts)
	if len(staged) != 1 || staged[0].HostPath != root {
		t.Fatalf("staged = %+v, want the mount unchanged", staged)
	}
	if err := persist(); err != nil {
		t.Fatal(err)
	}
	if staged, persist := stageTree(t, nil); len(staged) != 0 || persist() != nil {
		t.Fatalf("staging no mounts = %+v", staged)
	}
}

func TestStageLeavesMaskedEntriesOutOfTheCopy(t *testing.T) {
	root, mount := piTree(t)
	staged, persist := stageTree(t, []plan.ConfigMount{mount})
	assertMissing(t, filepath.Join(staged[0].HostPath, "masked.txt"))
	if got := readTestFile(t, filepath.Join(staged[0].HostPath, "sub", "c.json")); got != "c" {
		t.Fatalf("staged sub/c.json = %q", got)
	}
	if err := persist(); err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, filepath.Join(root, "masked.txt")); got != "secret" {
		t.Fatalf("masked host file = %q, want it untouched", got)
	}
}

func TestStageLeavesHostSymlinksAlone(t *testing.T) {
	root, mount := piTree(t)
	link := filepath.Join(root, "link.json")
	if err := os.Symlink("a.json", link); err != nil {
		t.Fatal(err)
	}
	staged, persist := stageTree(t, []plan.ConfigMount{mount})
	assertMissing(t, filepath.Join(staged[0].HostPath, "link.json"))
	if err := persist(); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("host symlink changed: info=%v err=%v", info, err)
	}
}

func TestStageStagesADirectoryChildSeparately(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, ".pi")
	themes := filepath.Join(home, "dotfiles", "themes")
	writeTestFile(t, filepath.Join(root, "settings.json"), "s", 0o600)
	writeTestFile(t, filepath.Join(themes, "dark.json"), "dark", 0o600)
	link := filepath.Join(root, "themes")
	if err := os.Symlink(themes, link); err != nil {
		t.Fatal(err)
	}
	mounts := []plan.ConfigMount{
		{Staged: true, Root: root, HostPath: root, DestPath: stageDest, MaskPaths: []string{"themes"}},
		{Staged: true, Root: root, HostPath: themes, DestPath: stageDest + "/themes", Link: link},
	}
	staged, persist := stageTree(t, mounts)
	if staged[0].MaskPaths != nil {
		t.Fatalf("staged root still has tmpfs masks: %+v", staged[0])
	}
	placeholder, err := os.Lstat(filepath.Join(staged[0].HostPath, "themes"))
	if err != nil || !placeholder.IsDir() {
		t.Fatalf("root stage has no directory mountpoint for the child: info=%v err=%v", placeholder, err)
	}
	if entries, _ := os.ReadDir(filepath.Join(staged[0].HostPath, "themes")); len(entries) != 0 {
		t.Fatalf("child mountpoint in the root stage is not empty: %v", entries)
	}
	if got := readTestFile(t, filepath.Join(staged[1].HostPath, "dark.json")); got != "dark" {
		t.Fatalf("staged child dark.json = %q", got)
	}
	writeTestFile(t, filepath.Join(staged[1].HostPath, "light.json"), "light", 0o600)
	if err := persist(); err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, filepath.Join(themes, "light.json")); got != "light" {
		t.Fatalf("host themes/light.json = %q", got)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("host symlink changed: info=%v err=%v", info, err)
	}
}

func TestStageKeepsDockerDestinationsSymlinkFreeAndPersistsWrites(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, ".pi")
	target := filepath.Join(home, "dotfiles", "settings.json")
	if err := os.MkdirAll(filepath.Join(root, "agent"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("initial"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "agent", "settings.json")
	if err := os.Symlink("../../dotfiles/settings.json", link); err != nil {
		t.Fatal(err)
	}
	mounts := []plan.ConfigMount{
		{Staged: true, Root: root, HostPath: root, DestPath: "/home/caged/.pi", MaskFiles: []string{"agent/settings.json"}},
		{Staged: true, Root: root, HostPath: target, DestPath: "/home/caged/.pi/agent/settings.json", Link: link},
	}
	staged, persist := stageTree(t, mounts)
	if len(staged) != 2 || strings.Contains(staged[0].HostPath, root) || strings.Contains(staged[1].HostPath, target) || staged[0].HostPath == staged[1].HostPath {
		t.Fatalf("Pi mounts were not redirected to temporary staging sources: %+v", staged)
	}
	if staged[0].MaskFiles != nil || staged[1].MaskFiles != nil {
		t.Fatalf("staged mounts still request Docker mask mounts: %+v", staged)
	}
	placeholder, err := os.Lstat(filepath.Join(staged[0].HostPath, "agent", "settings.json"))
	if err != nil || !placeholder.Mode().IsRegular() || placeholder.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("root staging tree has no safe regular mountpoint: info=%v err=%v", placeholder, err)
	}
	if got, err := os.ReadFile(staged[1].HostPath); err != nil || string(got) != "initial" {
		t.Fatalf("staged linked file = %q, %v", got, err)
	}
	if err := os.WriteFile(filepath.Join(staged[0].HostPath, "agent", "keybindings.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(staged[0].HostPath, "agent", "themes"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staged[0].HostPath, "agent", "themes", "new.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(staged[1].HostPath, []byte("updated"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := persist(); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "agent", "keybindings.json")); err != nil || string(got) != "{}" {
		t.Fatalf("new Pi config file on host = %q, %v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "agent", "themes", "new.json")); err != nil || string(got) != "{}" {
		t.Fatalf("new Pi config directory/file on host = %q, %v", got, err)
	}
	if got, err := os.ReadFile(target); err != nil || string(got) != "updated" {
		t.Fatalf("linked Pi config file on host = %q, %v", got, err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("host symlink was altered: info=%v err=%v", info, err)
	}
}

func TestStagePassesUnstagedMountsThrough(t *testing.T) {
	root, mount := piTree(t)
	other := plan.ConfigMount{Root: root, HostPath: root, DestPath: stageDest + "/other", MaskFiles: []string{"x"}}
	staged, persist := stageTree(t, []plan.ConfigMount{other, mount})
	if staged[0].HostPath != root || len(staged[0].MaskFiles) != 1 {
		t.Fatalf("unstaged mount changed: %+v", staged[0])
	}
	if staged[1].HostPath == root {
		t.Fatalf("staged mount was not copied: %+v", staged[1])
	}
	if err := persist(); err != nil {
		t.Fatal(err)
	}
}

func TestStageDoesNotModifyItsInput(t *testing.T) {
	root, mount := piTree(t)
	mounts := []plan.ConfigMount{mount}
	stageTree(t, mounts)
	if mounts[0].HostPath != root || len(mounts[0].MaskFiles) != 1 {
		t.Fatalf("Stage modified its input: %+v", mounts[0])
	}
}

func TestStageStagesThreeLevelsOfNestedLinks(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, ".pi")
	outer, inner := filepath.Join(home, "dotfiles", "a"), filepath.Join(home, "dotfiles", "b")
	writeTestFile(t, filepath.Join(root, "settings.json"), "s", 0o600)
	writeTestFile(t, filepath.Join(outer, "a.json"), "a", 0o600)
	writeTestFile(t, filepath.Join(inner, "b.json"), "b", 0o600)
	if err := os.Symlink(outer, filepath.Join(root, "a")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(inner, filepath.Join(outer, "b")); err != nil {
		t.Fatal(err)
	}
	mounts := []plan.ConfigMount{
		{Staged: true, Root: root, HostPath: root, DestPath: stageDest, MaskPaths: []string{"a"}},
		{Staged: true, Root: root, HostPath: outer, DestPath: stageDest + "/a", Link: filepath.Join(root, "a"), MaskPaths: []string{"b"}},
		{Staged: true, Root: root, HostPath: inner, DestPath: stageDest + "/a/b", Link: filepath.Join(outer, "b")},
	}
	staged, persist := stageTree(t, mounts)
	// Each copy holds an empty mountpoint for the mount right inside it only:
	// the innermost mount gets none in the outermost copy.
	for _, dir := range []string{filepath.Join(staged[0].HostPath, "a"), filepath.Join(staged[1].HostPath, "b")} {
		if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
			t.Fatalf("mountpoint %s = %v (err %v), want an empty directory", dir, entries, err)
		}
	}
	if got := readTestFile(t, filepath.Join(staged[1].HostPath, "a.json")); got != "a" {
		t.Fatalf("staged a.json = %q", got)
	}
	writeTestFile(t, filepath.Join(staged[2].HostPath, "new.json"), "new", 0o600)
	if err := persist(); err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, filepath.Join(inner, "new.json")); got != "new" {
		t.Fatalf("innermost new.json on the host = %q", got)
	}
}

func TestStageRemovesItsCopiesWhenStagingFails(t *testing.T) {
	tests := []struct {
		name   string
		mounts func(t *testing.T, home string) []plan.ConfigMount
		want   func(home string) string
	}{
		{
			name: "a staged root that is a symlink",
			mounts: func(t *testing.T, home string) []plan.ConfigMount {
				root := filepath.Join(home, ".pi")
				writeTestFile(t, filepath.Join(home, "real", "a.json"), "a", 0o600)
				if err := os.Symlink(filepath.Join(home, "real"), root); err != nil {
					t.Fatal(err)
				}
				return []plan.ConfigMount{{Staged: true, Root: root, HostPath: root, DestPath: stageDest, MaskFiles: []string{"x"}}}
			},
			want: func(string) string {
				return "stage Pi config " + stageDest + ": source is not a regular file or directory"
			},
		},
		{
			name: "a nested mount whose source is missing",
			mounts: func(t *testing.T, home string) []plan.ConfigMount {
				root := filepath.Join(home, ".pi")
				writeTestFile(t, filepath.Join(root, "a.json"), "a", 0o600)
				return []plan.ConfigMount{
					{Staged: true, Root: root, HostPath: root, DestPath: stageDest, MaskPaths: []string{"themes"}},
					{Staged: true, Root: root, HostPath: filepath.Join(home, "missing"), DestPath: stageDest + "/themes"},
				}
			},
			want: func(home string) string {
				return "stage Pi config " + stageDest + "/themes: lstat " + filepath.Join(home, "missing") + ": no such file or directory"
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			t.Setenv("TMPDIR", tmp)
			home := t.TempDir()
			session, err := New(system.FS{}).Stage(tc.mounts(t, home))
			if err == nil || err.Error() != tc.want(home) || session != nil {
				t.Fatalf("Stage() = %v, %v; want nil, %q", session, err, tc.want(home))
			}
			if entries, err := os.ReadDir(tmp); err != nil || len(entries) != 0 {
				t.Fatalf("temp dir after a failed Stage = %v (err %v), want the copies removed", entries, err)
			}
		})
	}
}

// B-CS-2 (KEEP): an unreadable file fails staging rather than being left out
// of the copy.
func TestStageFailsOnAnUnreadableFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads files whatever their mode")
	}
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	root, mount := piTree(t)
	if err := os.Chmod(filepath.Join(root, "b.json"), 0); err != nil {
		t.Fatal(err)
	}
	session, err := New(system.FS{}).Stage([]plan.ConfigMount{mount})
	want := "stage Pi config " + stageDest + ": open " + filepath.Join(root, "b.json") + ": permission denied"
	if err == nil || err.Error() != want || !errors.Is(err, fs.ErrPermission) || session != nil {
		t.Fatalf("Stage() = %v, %v; want nil, %q", session, err, want)
	}
	if entries, err := os.ReadDir(tmp); err != nil || len(entries) != 0 {
		t.Fatalf("temp dir after a failed Stage = %v (err %v), want the copies removed", entries, err)
	}
}

func TestStageLeavesSpecialFilesAndMaskedDirectoriesOutOfTheCopy(t *testing.T) {
	root, mount := piTree(t)
	if err := syscall.Mkfifo(filepath.Join(root, "pipe"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "cache", "x.json"), "cached", 0o600)
	mount.MaskPaths = []string{"cache"}
	staged, persist := stageTree(t, []plan.ConfigMount{mount})
	assertMissing(t, filepath.Join(staged[0].HostPath, "pipe"))
	assertMissing(t, filepath.Join(staged[0].HostPath, "cache"))
	if err := persist(); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(filepath.Join(root, "pipe")); err != nil || info.Mode()&os.ModeNamedPipe == 0 {
		t.Fatalf("host pipe: info=%v err=%v, want it kept", info, err)
	}
	if got := readTestFile(t, filepath.Join(root, "cache", "x.json")); got != "cached" {
		t.Fatalf("masked cache/x.json = %q, want it kept", got)
	}
}
