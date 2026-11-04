package configstage

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/JeremyOrr4/caged/internal/system"
)

// faultText is the message of every error faultFS injects.
const faultText = "injected fault"

// faultFS is the real filesystem with a hook that runs before each call it
// overrides. An error from fail is returned instead of making the call. fail
// may also change the real filesystem first, to act as the host changing
// something while staging or write-back runs.
type faultFS struct {
	system.FS
	fail func(op, name string) error // nil injects nothing
}

func (f *faultFS) hook(op, name string) error {
	if f.fail == nil {
		return nil
	}
	return f.fail(op, name)
}

func (f *faultFS) Lstat(name string) (fs.FileInfo, error) {
	if err := f.hook("Lstat", name); err != nil {
		return nil, err
	}
	return f.FS.Lstat(name)
}

func (f *faultFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if err := f.hook("ReadDir", name); err != nil {
		return nil, err
	}
	return f.FS.ReadDir(name)
}

func (f *faultFS) ReadFile(name string) ([]byte, error) {
	if err := f.hook("ReadFile", name); err != nil {
		return nil, err
	}
	return f.FS.ReadFile(name)
}

func (f *faultFS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	if err := f.hook("WriteFile", name); err != nil {
		return err
	}
	return f.FS.WriteFile(name, data, perm)
}

func (f *faultFS) Mkdir(name string, perm fs.FileMode) error {
	if err := f.hook("Mkdir", name); err != nil {
		return err
	}
	return f.FS.Mkdir(name, perm)
}

func (f *faultFS) MkdirAll(name string, perm fs.FileMode) error {
	if err := f.hook("MkdirAll", name); err != nil {
		return err
	}
	return f.FS.MkdirAll(name, perm)
}

func (f *faultFS) MkdirTemp(dir, pattern string) (string, error) {
	if err := f.hook("MkdirTemp", filepath.Join(dir, pattern)); err != nil {
		return "", err
	}
	return f.FS.MkdirTemp(dir, pattern)
}

func (f *faultFS) Chmod(name string, perm fs.FileMode) error {
	if err := f.hook("Chmod", name); err != nil {
		return err
	}
	return f.FS.Chmod(name, perm)
}

// call matches one filesystem call by its operation and the end of its path:
// a whole path, or a suffix such as "/mount-1/a.json" for paths inside the
// random staging directory.
type call struct{ op, path string }

func (c call) matches(op, name string) bool { return op == c.op && strings.HasSuffix(name, c.path) }

// onNth runs do, with the path, at the nth (from 1) call that c matches, and
// returns its result instead of making the call when it is an error.
func onNth(c call, n int, do func(name string) error) func(op, name string) error {
	seen := 0
	return func(op, name string) error {
		if !c.matches(op, name) {
			return nil
		}
		if seen++; seen == n {
			return do(name)
		}
		return nil
	}
}

// failNth fails the nth (from 1) call that c matches.
func failNth(c call, n int) func(op, name string) error {
	return onNth(c, n, func(string) error { return errors.New(faultText) })
}

// failAfter fails every call that c matches once a call that armed matches
// has been made.
func failAfter(armed, c call) func(op, name string) error {
	on := false
	return func(op, name string) error {
		if on && c.matches(op, name) {
			return errors.New(faultText)
		}
		on = on || armed.matches(op, name)
		return nil
	}
}
