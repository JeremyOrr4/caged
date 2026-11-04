package system

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestMkdirAllPrivateCreatesOwnerOnlyDirectories(t *testing.T) {
	t.Parallel()

	target := filepath.Join(t.TempDir(), "a", "b")
	if err := (FS{}).MkdirAllPrivate(target); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{target, filepath.Dir(target)} {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			t.Fatalf("Stat(%s) = %v, %v; want a directory", dir, info, err)
		}
		if got := info.Mode().Perm(); got != 0o700 {
			t.Errorf("%s mode = %o; want 700", dir, got)
		}
	}
}

func TestMkdirAllPrivateLeavesAnExistingDirectoryAlone(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "shared")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := (FS{}).MkdirAllPrivate(dir); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(dir); info.Mode().Perm() != 0o755 {
		t.Errorf("mode = %o; want the existing 755 kept", info.Mode().Perm())
	}
}

func TestCreateFileWritesCompleteContentWithTheGivenMode(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	name := filepath.Join(dir, "config.yaml")
	if err := (FS{}).CreateFile(name, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(name); string(got) != "hello" {
		t.Errorf("content = %q; want hello", got)
	}
	if info, _ := os.Stat(name); info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %o; want 600", info.Mode().Perm())
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("directory has %d entries; want only the new file (no temp leftovers)", len(entries))
	}
}

func TestCreateFileNeverReplacesAnExistingFile(t *testing.T) {
	t.Parallel()

	name := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(name, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := (FS{}).CreateFile(name, []byte("template"), 0o600)
	if !errors.Is(err, fs.ErrExist) {
		t.Fatalf("CreateFile() error = %v; want fs.ErrExist", err)
	}
	if got, _ := os.ReadFile(name); string(got) != "mine" {
		t.Errorf("content = %q; want the existing file untouched", got)
	}
	if info, _ := os.Stat(name); info.Mode().Perm() != 0o644 {
		t.Errorf("mode = %o; want 644 untouched", info.Mode().Perm())
	}
}

func TestCreateFileDoesNotWriteThroughASymlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "elsewhere")
	name := filepath.Join(dir, "config.yaml")
	if err := os.Symlink(target, name); err != nil {
		t.Fatal(err)
	}
	if err := (FS{}).CreateFile(name, []byte("template"), 0o600); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("CreateFile() error = %v; want fs.ErrExist for a dangling link", err)
	}
	if _, err := os.Lstat(target); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("link target was created (%v); want it untouched", err)
	}
}

func TestCreateFileRaceHasExactlyOneWinner(t *testing.T) {
	t.Parallel()

	name := filepath.Join(t.TempDir(), "config.yaml")
	const workers = 16
	results := make(chan error, workers)
	for range workers {
		go func() { results <- (FS{}).CreateFile(name, []byte("template"), 0o600) }()
	}
	won := 0
	for range workers {
		switch err := <-results; {
		case err == nil:
			won++
		case !errors.Is(err, fs.ErrExist):
			t.Errorf("CreateFile() error = %v; want nil or fs.ErrExist", err)
		}
	}
	if won != 1 {
		t.Errorf("%d creators won; want exactly 1", won)
	}
	if got, _ := os.ReadFile(name); string(got) != "template" {
		t.Errorf("content = %q; want the full template", got)
	}
}

func TestCreateExclusiveIsTheFallbackWithoutHardLinks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	name := filepath.Join(dir, "config.yaml")
	if err := createExclusive(name, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(name); string(got) != "hello" {
		t.Errorf("content = %q; want hello", got)
	}
	if info, _ := os.Stat(name); info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %o; want 600", info.Mode().Perm())
	}
	if err := createExclusive(name, []byte("again"), 0o600); !errors.Is(err, fs.ErrExist) {
		t.Errorf("createExclusive(existing) error = %v; want fs.ErrExist", err)
	}
	if got, _ := os.ReadFile(name); string(got) != "hello" {
		t.Errorf("content = %q; want the existing file untouched", got)
	}
	link, target := filepath.Join(dir, "link"), filepath.Join(dir, "elsewhere")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := createExclusive(link, []byte("template"), 0o600); !errors.Is(err, fs.ErrExist) {
		t.Errorf("createExclusive(dangling link) error = %v; want fs.ErrExist", err)
	}
	if _, err := os.Lstat(target); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("link target was created (%v); want it untouched", err)
	}
}

func TestReplaceFileSwapsContentAtomically(t *testing.T) {
	t.Parallel()

	name := filepath.Join(t.TempDir(), "approved.yaml")
	if err := os.WriteFile(name, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := (FS{}).ReplaceFile(name, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(name); string(got) != "new" {
		t.Errorf("content = %q; want new", got)
	}
	if entries, _ := os.ReadDir(filepath.Dir(name)); len(entries) != 1 {
		t.Errorf("directory has %d entries; want no temp leftovers", len(entries))
	}
}

func TestReplaceFileReplacesASymlinkWithoutFollowingIt(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target, name := filepath.Join(dir, "target.yaml"), filepath.Join(dir, "approved.yaml")
	if err := os.WriteFile(target, []byte("target"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, name); err != nil {
		t.Fatal(err)
	}
	if err := (FS{}).ReplaceFile(name, []byte("new"), 0o640); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(name)
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("%s: info=%v err=%v; want a regular file in place of the link", name, info, err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Errorf("mode = %o; want 640", info.Mode().Perm())
	}
	if got, _ := os.ReadFile(name); string(got) != "new" {
		t.Errorf("content = %q; want new", got)
	}
	if got, _ := os.ReadFile(target); string(got) != "target" {
		t.Errorf("link target = %q; want it untouched", got)
	}
}

func TestReplaceFileFailsInAMissingDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	err := (FS{}).ReplaceFile(filepath.Join(dir, "missing", "approved.yaml"), []byte("new"), 0o600)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("ReplaceFile() error = %v; want fs.ErrNotExist", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("parent directory has %d entries; want nothing created", len(entries))
	}
}

func TestReplaceFileOntoADirectoryLeavesNoTempFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	name := filepath.Join(dir, "approved.yaml")
	if err := os.MkdirAll(filepath.Join(name, "inside"), 0o700); err != nil {
		t.Fatal(err)
	}
	var linkErr *os.LinkError
	if err := (FS{}).ReplaceFile(name, []byte("new"), 0o600); !errors.As(err, &linkErr) || linkErr.Op != "rename" {
		t.Fatalf("ReplaceFile() onto a directory error = %v; want the rename error", err)
	}
	if info, err := os.Lstat(filepath.Join(name, "inside")); err != nil || !info.IsDir() {
		t.Errorf("directory content: info=%v err=%v; want it untouched", info, err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("parent directory has %d entries; want no temp leftovers", len(entries))
	}
}

func TestCreateFileFailsInAMissingDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	err := (FS{}).CreateFile(filepath.Join(dir, "missing", "config.yaml"), []byte("template"), 0o600)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("CreateFile() error = %v; want fs.ErrNotExist", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("parent directory has %d entries; want nothing created", len(entries))
	}
}

// TestMkdirAllPrivateReportsAReadOnlyFilesystemAsAPermissionError: userconfig
// warns and goes on only for a config folder this user cannot create
// (permission denied or a read-only filesystem, WB-T4), and cannot import
// syscall to ask about EROFS itself.
func TestMkdirAllPrivateReportsAReadOnlyFilesystemAsAPermissionError(t *testing.T) {
	t.Parallel()
	rofs := &fs.PathError{Op: "mkdir", Path: "/ro/caged", Err: syscall.EROFS}
	err := permissionLike(rofs)
	if !errors.Is(err, fs.ErrPermission) || !errors.Is(err, syscall.EROFS) || err.Error() != rofs.Error() {
		t.Fatalf("permissionLike(EROFS) = %v; want the same text, matching fs.ErrPermission and EROFS", err)
	}
	exists := &fs.PathError{Op: "mkdir", Path: "/h/link", Err: syscall.EEXIST}
	if err := permissionLike(exists); err != exists || errors.Is(err, fs.ErrPermission) { //nolint:errorlint // asserts the error is returned unchanged, not wrapped
		t.Fatalf("permissionLike(EEXIST) = %v; want it unchanged", err)
	}
}
