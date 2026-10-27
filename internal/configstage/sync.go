package configstage

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
)

// writer writes one staged copy back to its host source. The baseline is the
// copy as it was before the container started (or at the last successful
// Sync), so a difference between the copy and the baseline is a container
// change, and a difference between the host and the baseline is a host
// change. When both sides changed the same
// entry differently, the host version is kept and the conflict is reported.
// A failed entry does not stop the others: every failure is returned,
// joined, and only a failed directory skips what is inside it.
type writer struct {
	fs       FileSystem
	baseline snapshot
	synced   snapshot // filled in by the walk: the copy's state, the next baseline
}

// sync writes the entry at copy, whose slash path in the tree is rel, back
// to source. masks and children are relative to copy and are not written.
func (w writer) sync(copyPath, source string, masks, children []string, rel string) error {
	info, err := w.fs.Lstat(copyPath)
	if err != nil {
		return err
	}
	if info.Mode().IsRegular() {
		return w.syncFile(copyPath, source, info, rel)
	}
	if !info.IsDir() || info.Mode()&fs.ModeSymlink != 0 {
		return nil
	}
	return w.syncDir(copyPath, source, info, masks, children, rel)
}

// syncFile writes a regular file back when the container changed it.
func (w writer) syncFile(copyPath, source string, info fs.FileInfo, rel string) error {
	staged, err := fileState(w.fs, copyPath, info)
	if err != nil {
		return err
	}
	w.synced[rel] = staged
	old, existed := w.baseline[rel]
	if existed && old.dir {
		return containerChangedType(source)
	}
	if existed && old == staged {
		return nil // the container did not change this file
	}
	host, err := w.fs.Lstat(source)
	switch {
	case err == nil:
		if !host.Mode().IsRegular() || host.Mode()&fs.ModeSymlink != 0 {
			return changedType(source) // never follow or replace a host-side link or type change
		}
		current, err := fileState(w.fs, source, host)
		if err != nil {
			return err
		}
		hostChanged := !existed || current != old
		if hostChanged && current != staged {
			return bothChanged(source)
		}
	case !errors.Is(err, fs.ErrNotExist):
		return err
	case existed:
		return fmt.Errorf("host config file disappeared during container run: %s", source)
	}
	return w.replaceFile(copyPath, source, info.Mode())
}

// syncDir writes a directory back: its permissions, the entries the
// container removed, and every entry in the copy.
func (w writer) syncDir(copyPath, source string, info fs.FileInfo, masks, children []string, rel string) error {
	w.synced[rel] = dirState(info)
	old, existed := w.baseline[rel]
	if existed && !old.dir {
		return containerChangedType(source)
	}
	host, err := w.hostDir(source, info.Mode(), existed)
	if err != nil {
		return err
	}
	if err := w.syncDirMode(source, old, existed, host.Mode().Perm(), info.Mode().Perm()); err != nil {
		return err
	}
	skipped := func(name string) bool { return withinAny(name, masks) || withinAny(name, children) }
	copied, err := w.fs.ReadDir(copyPath)
	if err != nil {
		return err
	}
	hostEntries, err := w.fs.ReadDir(source)
	if err != nil {
		return err
	}
	errs := []error{w.removeDeletedEntries(source, rel, copied, hostEntries, skipped)}
	for _, entry := range copied {
		if name := entry.Name(); !skipped(name) {
			errs = append(errs, w.syncEntry(copyPath, source, name, masks, children, rel))
		}
	}
	return errors.Join(errs...)
}

// removeDeletedEntries removes from the host directory source, whose slash
// path in the tree is rel and whose listing is hostEntries, the entries the
// container deleted: those missing from copied, the listing of its copy.
// Skipped names (masks and the mountpoints of nested mounts) are left alone
// on the host, whether or not the copy has them.
func (w writer) removeDeletedEntries(source, rel string, copied, hostEntries []fs.DirEntry, skipped func(name string) bool) error {
	inCopy := make(map[string]bool, len(copied))
	for _, entry := range copied {
		inCopy[entry.Name()] = true
	}
	var errs []error
	for _, entry := range hostEntries {
		if name := entry.Name(); !skipped(name) && !inCopy[name] {
			errs = append(errs, w.removeDeleted(filepath.Join(source, name), path.Join(rel, name)))
		}
	}
	return errors.Join(errs...)
}

// hostDir returns the Lstat info of the host directory source, creating it
// when the container created it.
func (w writer) hostDir(source string, mode fs.FileMode, existed bool) (fs.FileInfo, error) {
	host, err := w.fs.Lstat(source)
	switch {
	case err == nil:
		if !host.IsDir() || host.Mode()&fs.ModeSymlink != 0 {
			return nil, changedType(source)
		}
		return host, nil
	case !errors.Is(err, fs.ErrNotExist):
		return nil, err
	case existed:
		return nil, fmt.Errorf("host config directory disappeared during container run: %s", source)
	}
	if err := w.ensureDir(source, mode.Perm()); err != nil {
		return nil, err
	}
	return w.fs.Lstat(source)
}

// syncDirMode applies the container's permission change to the host
// directory, unless the host changed them differently.
func (w writer) syncDirMode(source string, old entryState, existed bool, hostMode, stagedMode fs.FileMode) error {
	if !existed || old.mode == stagedMode {
		return nil
	}
	if hostMode != old.mode && hostMode != stagedMode {
		return fmt.Errorf("host and container both changed permissions on %s; leaving the host version intact", source)
	}
	if hostMode == old.mode {
		return w.fs.Chmod(source, stagedMode)
	}
	return nil
}

// syncEntry writes back the entry name of the directory copyPath. A symlink
// or special file in the copy is never written back: a new one is ignored,
// and one that replaced an entry of the baseline is a container type change.
func (w writer) syncEntry(copyPath, source, name string, masks, children []string, rel string) error {
	from, to := filepath.Join(copyPath, name), filepath.Join(source, name)
	info, err := w.fs.Lstat(from)
	if err != nil {
		return nil // gone since the copy was listed
	}
	childRel := path.Join(rel, name)
	switch {
	case info.IsDir():
		return w.sync(from, to, under(masks, name), under(children, name), childRel)
	case info.Mode().IsRegular():
		return w.sync(from, to, nil, nil, childRel)
	}
	if _, existed := w.baseline[childRel]; existed {
		return containerChangedType(to)
	}
	return nil
}

// removeDeleted removes from the host an entry the container deleted, rel
// being its key in the baseline. An entry that is not in the baseline was
// added on the host during the run and is kept; one the host changed, in
// content or type, is a conflict. A directory is removed once everything in
// it is.
func (w writer) removeDeleted(hostPath, rel string) error {
	old, existed := w.baseline[rel]
	if !existed {
		return nil // added by the host while the container was running
	}
	info, err := w.fs.Lstat(hostPath)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil // already gone
	case err != nil:
		return err
	case info.Mode()&fs.ModeSymlink != 0:
		return changedType(hostPath) // nothing is deleted through a host symlink
	case info.IsDir():
		return w.removeDeletedDir(hostPath, rel, old, info)
	case !info.Mode().IsRegular() || old.dir:
		return changedType(hostPath)
	}
	current, err := fileState(w.fs, hostPath, info)
	if err != nil {
		return err
	}
	if current != old {
		return bothChanged(hostPath)
	}
	return w.fs.Remove(hostPath)
}

func (w writer) removeDeletedDir(hostPath, rel string, old entryState, info fs.FileInfo) error {
	if !old.dir {
		return changedType(hostPath)
	}
	if info.Mode().Perm() != old.mode {
		return fmt.Errorf("host config changed during container run at %s", hostPath)
	}
	entries, err := w.fs.ReadDir(hostPath)
	if err != nil {
		return err
	}
	var errs []error
	for _, entry := range entries {
		errs = append(errs, w.removeDeleted(filepath.Join(hostPath, entry.Name()), path.Join(rel, entry.Name())))
	}
	if err := errors.Join(errs...); err != nil {
		return err // what failed is still in the directory, so it stays
	}
	if entries, err = w.fs.ReadDir(hostPath); err != nil {
		return err
	}
	if len(entries) == 0 {
		return w.fs.Remove(hostPath)
	}
	return nil
}

// replaceFile atomically replaces the host file target with the staged file
// copyPath. A symlink or special file at target is left alone, as a type
// change.
func (w writer) replaceFile(copyPath, target string, mode fs.FileMode) error {
	if err := w.ensureDir(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	if info, err := w.fs.Lstat(target); err == nil && (!info.Mode().IsRegular() || info.Mode()&fs.ModeSymlink != 0) {
		return changedType(target)
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	data, err := w.fs.ReadFile(copyPath)
	if err != nil {
		return err
	}
	return w.fs.ReplaceFile(target, data, mode.Perm())
}

// ensureDir makes sure dir is a real directory, not a symlink, creating it
// with mode when it is missing.
func (w writer) ensureDir(dir string, mode fs.FileMode) error {
	info, err := w.fs.Lstat(dir)
	if err == nil {
		if !info.IsDir() || info.Mode()&fs.ModeSymlink != 0 {
			return errors.New("destination is no longer a directory")
		}
		return nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := w.fs.Mkdir(dir, mode); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	} else if err == nil {
		if err := w.fs.Chmod(dir, mode.Perm()); err != nil {
			return err
		}
	}
	return w.ensureDir(dir, mode)
}

// changedType is the conflict for a host entry, at path, whose type changed
// while the container ran.
func changedType(path string) error {
	return fmt.Errorf("host config changed type during container run at %s", path)
}

// containerChangedType is the conflict for an entry the container replaced
// with another type; path is its host path.
func containerChangedType(path string) error {
	return fmt.Errorf("container changed the type of %s; leaving the host version intact", path)
}

func bothChanged(path string) error {
	return fmt.Errorf("host and container both changed %s; leaving the host version intact", path)
}
