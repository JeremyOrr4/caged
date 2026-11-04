package system

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// MkdirAllPrivate creates dir and any missing parents readable only by the
// owner (0700). Directories that already exist keep their mode. A read-only
// filesystem is reported as a permission error (errors.Is(err,
// fs.ErrPermission)), with the same text: either way the directory cannot be
// made by this user.
func (FS) MkdirAllPrivate(dir string) error { return permissionLike(os.MkdirAll(dir, 0o700)) }

// permissionLike marks EROFS as a permission error.
func permissionLike(err error) error {
	if errors.Is(err, syscall.EROFS) {
		return readOnlyError{err}
	}
	return err
}

// readOnlyError is an EROFS error that also matches fs.ErrPermission.
type readOnlyError struct{ err error }

func (e readOnlyError) Error() string      { return e.err.Error() }
func (e readOnlyError) Unwrap() error      { return e.err }
func (readOnlyError) Is(target error) bool { return target == fs.ErrPermission }

// CreateFile creates name with data and perm, but only if nothing is there.
// The file appears complete or not at all, so a concurrent reader never sees
// a partial file, and exactly one of several concurrent creators succeeds; the
// others get an error that wraps fs.ErrExist. An existing file or symlink,
// even a dangling one, is never written through or replaced.
func (FS) CreateFile(name string, data []byte, perm fs.FileMode) error {
	tmp, err := writeTemp(filepath.Dir(name), data, perm)
	if err != nil {
		return err
	}
	// Best effort: the outcome is decided by Link or createExclusive; a
	// failed removal leaves only a hidden .caged-* copy next to name.
	defer func() { _ = os.Remove(tmp) }()
	err = os.Link(tmp, name)
	if err == nil || errors.Is(err, fs.ErrExist) {
		return err
	}
	// Some filesystems have no hard links; an exclusive create is the next best
	// thing. The window in which a reader sees a partial file is tiny.
	return createExclusive(name, data, perm)
}

// createExclusive creates name with data and perm in place, failing with an
// error that wraps fs.ErrExist when anything, even a dangling symlink, is there.
func createExclusive(name string, data []byte, perm fs.FileMode) error {
	file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	_, err = file.Write(data)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(name) // best effort: the write or close error is the one to report
	}
	return err
}

// ReplaceFile atomically replaces name with data and perm, creating it when it
// is missing. A symlink at name is replaced, not followed.
func (FS) ReplaceFile(name string, data []byte, perm fs.FileMode) error {
	tmp, err := writeTemp(filepath.Dir(name), data, perm)
	if err != nil {
		return err
	}
	if err := os.Rename(tmp, name); err != nil {
		_ = os.Remove(tmp) // best effort: the rename error is the one to report
		return err
	}
	return nil
}

// writeTemp writes data to a new temporary file in dir with mode perm and
// returns its path.
func writeTemp(dir string, data []byte, perm fs.FileMode) (string, error) {
	file, err := os.CreateTemp(dir, ".caged-*")
	if err != nil {
		return "", err
	}
	name := file.Name()
	fail := func(err error) (string, error) {
		// Best effort: the original error is the one to report.
		_ = file.Close()
		_ = os.Remove(name)
		return "", fmt.Errorf("write %s: %w", name, err)
	}
	if err := file.Chmod(perm); err != nil {
		return fail(err)
	}
	if _, err := file.Write(data); err != nil {
		return fail(err)
	}
	if err := file.Sync(); err != nil {
		return fail(err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(name) // best effort: the close error is the one to report
		return "", err
	}
	return name, nil
}
