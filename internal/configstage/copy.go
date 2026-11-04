package configstage

import (
	"crypto/sha256"
	"errors"
	"io/fs"
	"path/filepath"
)

// entryState is what a snapshot records about one file or directory.
type entryState struct {
	dir    bool
	mode   fs.FileMode // permission bits only
	digest [32]byte    // SHA-256 of a file's content; zero for a directory
}

// snapshot maps the slash paths of a tree, relative to its root ("." for the
// root itself), to their state.
type snapshot map[string]entryState

// copyTree copies source, a regular file or a directory, to target. Symlinks,
// special files, entries that cannot be inspected, the contents of
// directories that cannot be listed, and the entries excluded reports are
// left out. A regular file that cannot be read fails the copy: hostconfig
// masks directories it cannot list, but not unreadable files, and the run
// stops rather than hide one. Permission bits are kept.
func copyTree(fsys FileSystem, source, target string, excluded func(rel string) bool) error {
	info, err := fsys.Lstat(source)
	if err != nil {
		return err
	}
	if info.Mode().IsRegular() {
		return copyFile(fsys, source, target, info.Mode())
	}
	if !info.IsDir() {
		return errNotFileOrDir()
	}
	if err := makeDir(fsys, target, info.Mode()); err != nil {
		return err
	}
	return walkTree(fsys, source, excluded, func(rel string, info fs.FileInfo) error {
		out := filepath.Join(target, filepath.FromSlash(rel))
		if info.IsDir() {
			return makeDir(fsys, out, info.Mode())
		}
		if info.Mode().IsRegular() {
			return copyFile(fsys, filepath.Join(source, filepath.FromSlash(rel)), out, info.Mode())
		}
		return nil
	})
}

// takeSnapshot records the state of every file and directory in root that
// copyTree would copy.
func takeSnapshot(fsys FileSystem, root string, excluded func(rel string) bool) (snapshot, error) {
	info, err := fsys.Lstat(root)
	if err != nil {
		return nil, err
	}
	if info.Mode().IsRegular() {
		state, err := fileState(fsys, root, info)
		if err != nil {
			return nil, err
		}
		return snapshot{".": state}, nil
	}
	if !info.IsDir() {
		return nil, errNotFileOrDir()
	}
	states := snapshot{".": dirState(info)}
	err = walkTree(fsys, root, excluded, func(rel string, info fs.FileInfo) error {
		if info.IsDir() {
			states[rel] = dirState(info)
		} else if info.Mode().IsRegular() {
			state, err := fileState(fsys, filepath.Join(root, filepath.FromSlash(rel)), info)
			if err != nil {
				return err
			}
			states[rel] = state
		}
		return nil
	})
	return states, err
}

// walkTree calls visit for every entry below root, with its slash path
// relative to root and its Lstat info. It skips excluded entries, symlinks
// and entries that cannot be inspected, with everything below them, and the
// contents of a directory that cannot be listed (the directory itself is
// visited).
func walkTree(fsys FileSystem, root string, excluded func(rel string) bool, visit func(rel string, info fs.FileInfo) error) error {
	return fsys.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if excluded(rel) {
			return skip(entry)
		}
		if walkErr != nil {
			return nil
		}
		info, err := fsys.Lstat(path)
		if err != nil || info.Mode()&fs.ModeSymlink != 0 {
			return skip(entry)
		}
		return visit(rel, info)
	})
}

// skip leaves out entry, and everything below it when it is a directory.
func skip(entry fs.DirEntry) error {
	if entry.IsDir() {
		return filepath.SkipDir
	}
	return nil
}

// makeDir creates dir with mode's permission bits, and sets them on a
// directory that already exists.
func makeDir(fsys FileSystem, dir string, mode fs.FileMode) error {
	if err := fsys.MkdirAll(dir, mode.Perm()); err != nil {
		return err
	}
	return fsys.Chmod(dir, mode.Perm())
}

// copyFile copies the regular file source to target with mode's permission
// bits, creating target's directory when it is missing.
func copyFile(fsys FileSystem, source, target string, mode fs.FileMode) error {
	if err := fsys.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	data, err := fsys.ReadFile(source)
	if err != nil {
		return err
	}
	if err := fsys.WriteFile(target, data, mode.Perm()); err != nil {
		return err
	}
	return fsys.Chmod(target, mode.Perm())
}

// errNotFileOrDir is returned for a staged source that is neither a regular
// file nor a directory.
func errNotFileOrDir() error { return errors.New("source is not a regular file or directory") }

func dirState(info fs.FileInfo) entryState {
	return entryState{dir: true, mode: info.Mode().Perm()}
}

// fileState reads the regular file at path, whose Lstat info is info.
func fileState(fsys FileSystem, path string, info fs.FileInfo) (entryState, error) {
	digest, err := fileDigest(fsys, path)
	if err != nil {
		return entryState{}, err
	}
	return entryState{mode: info.Mode().Perm(), digest: digest}, nil
}

func fileDigest(fsys FileSystem, path string) ([32]byte, error) {
	data, err := fsys.ReadFile(path)
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(data), nil
}
