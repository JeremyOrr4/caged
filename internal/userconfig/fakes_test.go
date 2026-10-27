package userconfig

import (
	"io/fs"
	"sync"
)

// fakeFS is an in-memory filesystem: a map of path to content.
type fakeFS struct {
	mu        sync.Mutex
	files     map[string]string
	err       error // returned for every read when set
	reads     int
	read      []string
	dirs      []string
	mkdirErr  error                  // returned by MkdirAllPrivate when set
	createErr error                  // returned by CreateFile when set
	raceLost  bool                   // CreateFile finds the file another process just created
	creates   int                    // CreateFile calls
	writeErr  error                  // returned by ReplaceFile when set
	modes     map[string]fs.FileMode // the permissions ReplaceFile was asked for
	noHome    bool                   // /home/me does not exist
	links     map[string]string      // what EvalSymlinks resolves a listed path to
	evalErr   error                  // returned by EvalSymlinks for every path when set
}

func (f *fakeFS) MkdirAllPrivate(dir string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dirs = append(f.dirs, dir)
	return f.mkdirErr
}

func (f *fakeFS) CreateFile(name string, data []byte, _ fs.FileMode) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.creates++
	if f.raceLost {
		f.put(name, []byte("theirs"))
		return &fs.PathError{Op: "create", Path: name, Err: fs.ErrExist}
	}
	if f.createErr != nil {
		return f.createErr
	}
	if _, ok := f.files[name]; ok {
		return &fs.PathError{Op: "create", Path: name, Err: fs.ErrExist}
	}
	f.put(name, data)
	return nil
}

func (f *fakeFS) ReplaceFile(name string, data []byte, perm fs.FileMode) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.writeErr != nil {
		return f.writeErr
	}
	f.put(name, data)
	if f.modes == nil {
		f.modes = map[string]fs.FileMode{}
	}
	f.modes[name] = perm
	return nil
}

func (f *fakeFS) put(name string, data []byte) {
	if f.files == nil {
		f.files = map[string]string{}
	}
	f.files[name] = string(data)
}

func (f *fakeFS) ReadFile(name string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads++
	f.read = append(f.read, name)
	if f.err != nil {
		return nil, f.err
	}
	data, ok := f.files[name]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return []byte(data), nil
}

// ReadRegularFile is ReadFile: every fake file is regular and small.
func (f *fakeFS) ReadRegularFile(name string, _ int64) ([]byte, error) { return f.ReadFile(name) }

func (f *fakeFS) Stat(name string) (fs.FileInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if name == "/home/me" && !f.noHome {
		return nil, nil
	}
	if _, ok := f.files[name]; ok {
		return nil, nil
	}
	return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
}

func (f *fakeFS) EvalSymlinks(path string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.evalErr != nil {
		return "", f.evalErr
	}
	if resolved, ok := f.links[path]; ok {
		return resolved, nil
	}
	return path, nil
}
