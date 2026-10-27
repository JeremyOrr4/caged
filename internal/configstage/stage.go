// Package configstage runs staged config mounts (agent.ConfigPath.Staged)
// from symlink-free copies, and writes the container's changes back.
//
// Docker resolves a nested mount destination through any symlink inside the
// parent bind mount, so mounting a staged tree's masks and link targets
// directly could escape the config directory. Stage copies every tree that
// needs masks or nested mounts into a private temporary directory, with empty
// mountpoints where the nested mounts go. Once the container has exited,
// Session.Sync writes its changes back to the host, keeping the host's
// version of anything both sides changed, and Session.Cleanup removes the
// copies.
package configstage

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"

	"github.com/JeremyOrr4/caged/internal/plan"
)

// FileSystem is the filesystem access staging needs. system.FS implements it.
type FileSystem interface {
	Lstat(name string) (fs.FileInfo, error)
	ReadDir(name string) ([]fs.DirEntry, error)
	WalkDir(root string, fn fs.WalkDirFunc) error
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm fs.FileMode) error
	// ReplaceFile atomically replaces name with data, replacing (not
	// following) a symlink at name.
	ReplaceFile(name string, data []byte, perm fs.FileMode) error
	Mkdir(name string, perm fs.FileMode) error
	MkdirAll(name string, perm fs.FileMode) error
	MkdirTemp(dir, pattern string) (string, error)
	Chmod(name string, perm fs.FileMode) error
	Remove(name string) error
	RemoveAll(name string) error
}

// Stager stages config mounts for one container run at a time.
type Stager struct{ fs FileSystem }

// New returns a Stager that works through fsys.
func New(fsys FileSystem) *Stager { return &Stager{fs: fsys} }

// Session is one staged run: the mounts to give the container runtime, and
// the copies to write back when the container exits.
type Session struct {
	fs     FileSystem
	dir    string // the staging directory; "" when nothing was staged
	mounts []plan.ConfigMount
	trees  []stagedTree
}

// stagedTree is one copied mount and what is needed to write it back.
type stagedTree struct {
	source, copy    string   // the host path and its staged copy
	masks, children []string // slash paths, relative to the tree, left out of the copy
	baseline        snapshot // the copy before the container started, or at the last Sync that fully succeeded
}

// Unstaged returns a Session that stages nothing: Mounts returns mounts, and
// Sync and Cleanup do nothing.
func Unstaged(mounts []plan.ConfigMount) *Session {
	return &Session{mounts: slices.Clone(mounts)}
}

// Stage copies the staged mounts among mounts that need it and returns the
// session that runs them. Other mounts are passed through unchanged, and
// mounts itself is never modified.
func (s *Stager) Stage(mounts []plan.ConfigMount) (*Session, error) {
	groups := planGroups(mounts)
	if len(groups) == 0 {
		return Unstaged(mounts), nil
	}
	dir, err := s.fs.MkdirTemp("", "caged-pi-config-")
	if err != nil {
		return nil, fmt.Errorf("create Pi config staging directory: %w", err)
	}
	session := &Session{fs: s.fs, dir: dir, mounts: slices.Clone(mounts)}
	for _, group := range groups {
		if err := session.stageGroup(mounts, group); err != nil {
			_ = session.Cleanup() // the staging error is the one to report
			return nil, err
		}
	}
	return session, nil
}

// Mounts returns the config mounts to run with: staged mounts point at their
// copies and need no masks.
func (s *Session) Mounts() []plan.ConfigMount { return slices.Clone(s.mounts) }

// Copied reports whether any mount runs from a copy, whose changes reach the
// host only through Sync.
func (s *Session) Copied() bool { return len(s.trees) > 0 }

// Sync writes every staged copy back to its host source and returns every
// failure, joined. The container must have stopped: a running one could
// swap entries of its copy for symlinks while Sync reads them. Sync can be
// called again, and then writes what changed since the last Sync that
// succeeded for each copy. A copy whose Sync failed keeps its baseline, so
// the next Sync also redoes the writes the failed one made: a host change
// that put such an entry back as it was is overwritten (bugs.md Q-CS-8).
func (s *Session) Sync() error {
	var errs []error
	for i := range s.trees {
		tree := &s.trees[i]
		w := writer{fs: s.fs, baseline: tree.baseline, synced: snapshot{}}
		failed := failures(w.sync(tree.copy, tree.source, tree.masks, tree.children, "."))
		for _, err := range failed {
			errs = append(errs, fmt.Errorf("persist Pi config %s: %w", tree.source, err))
		}
		if len(failed) == 0 {
			tree.baseline = w.synced // the host now matches the copy where the container changed it
		}
	}
	return errors.Join(errs...)
}

// Cleanup removes the copies without writing anything back: after the last
// Sync, or instead of it for a run that never started.
func (s *Session) Cleanup() error {
	if s.dir == "" {
		return nil
	}
	return s.fs.RemoveAll(s.dir)
}

// stageGroup copies every mount of group, shallowest first, and then creates
// the empty mountpoints for nested mounts inside each copy.
func (s *Session) stageGroup(original []plan.ConfigMount, g group) error {
	for _, i := range g.members {
		if err := s.stageMount(original, g, i); err != nil {
			return err
		}
	}
	for _, p := range g.mountpoints(original) {
		if err := s.createMountpoint(original, p); err != nil {
			return err
		}
	}
	return nil
}

// stageMount copies mount i to a new directory in the staging directory,
// leaving out its masks and the paths of the mounts nested inside it, and
// records the copy so it can be written back.
func (s *Session) stageMount(original []plan.ConfigMount, g group, i int) error {
	mount := original[i]
	copyPath := filepath.Join(s.dir, fmt.Sprintf("mount-%d", len(s.trees)+1))
	tree := stagedTree{
		source:   mount.HostPath,
		copy:     copyPath,
		masks:    append(slices.Clone(mount.MaskPaths), mount.MaskFiles...),
		children: g.nestedIn(original, i),
	}
	if err := copyTree(s.fs, tree.source, tree.copy, tree.excluded); err != nil {
		return fmt.Errorf("stage Pi config %s: %w", mount.DestPath, err)
	}
	s.mounts[i].HostPath, s.mounts[i].MaskPaths, s.mounts[i].MaskFiles = copyPath, nil, nil
	baseline, err := takeSnapshot(s.fs, tree.copy, tree.excluded)
	if err != nil {
		return fmt.Errorf("snapshot Pi config %s: %w", mount.HostPath, err)
	}
	tree.baseline = baseline
	s.trees = append(s.trees, tree)
	return nil
}

// createMountpoint creates the empty file or directory, inside the copy of the
// parent, where the copy of a nested mount is mounted. The nested mount gets
// its own copy, never a path inside the parent's host source.
func (s *Session) createMountpoint(original []plan.ConfigMount, p mountpoint) error {
	path := filepath.Join(s.mounts[p.parent].HostPath, filepath.FromSlash(p.rel))
	if err := s.fs.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create Pi config staging mountpoint: %w", err)
	}
	source := original[p.child].HostPath
	info, err := s.fs.Lstat(source)
	if err != nil {
		return fmt.Errorf("inspect Pi config mount source %s: %w", source, err)
	}
	if info.IsDir() {
		err = s.fs.MkdirAll(path, 0o700)
	} else {
		err = s.fs.WriteFile(path, nil, 0o600)
	}
	if err != nil {
		return fmt.Errorf("create Pi config staging mountpoint: %w", err)
	}
	return nil
}

// failures returns each failure joined into err, so each can be reported on
// its own line; nil has none.
func failures(err error) []error {
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		if err == nil {
			return nil
		}
		return []error{err}
	}
	var all []error
	for _, e := range joined.Unwrap() {
		all = append(all, failures(e)...)
	}
	return all
}

// excluded reports whether rel, a slash path inside the tree, is masked or
// belongs to a nested mount, so it is neither copied nor written back.
func (t stagedTree) excluded(rel string) bool {
	return withinAny(rel, t.masks) || withinAny(rel, t.children)
}
