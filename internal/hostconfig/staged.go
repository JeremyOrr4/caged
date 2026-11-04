package hostconfig

import (
	"io/fs"
	"path/filepath"
)

// mountStagedTree mounts root, a resolved config directory, as one tree so the
// agent can create files as well as edit existing ones. Symlinks and special
// files are hidden with masks; allowed symlink targets are mounted back at
// their logical paths by followLink. This keeps the mount scoped to the
// config, not home. A root that is a regular file is mounted as it is; any
// other root that is not a directory is refused.
func (r *resolution) mountStagedTree(o owner, root, dest string, depth int, via string) {
	switch classifyEntry(r.fs.Lstat(root)) {
	case entryUninspectable:
		r.refuse(root, reasonUninspectableRoot)
		return
	case entryFile:
		r.addMount(o, root, dest, nil, nil, via)
		return
	case entryDir:
	default:
		r.refuse(root, reasonRootNotFileOrDir)
		return
	}

	var scan stagedScan
	err := r.fs.WalkDir(root, func(name string, _ fs.DirEntry, walkErr error) error {
		if name == root {
			// WalkDir calls back a second time, with the error, for a root it
			// cannot read; returning it fails the walk, so the tree is refused.
			return walkErr
		}
		rel, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		if reason := scan.hide(r.fs, name, filepath.ToSlash(rel), walkErr); reason != "" {
			r.refuse(name, reason)
		}
		return nil
	})
	if err != nil {
		r.refuse(root, "cannot inspect config tree safely")
		return
	}
	r.addMount(o, root, dest, scan.dirMasks, scan.fileMasks, via)
	for _, link := range scan.links {
		if linkDest, ok := r.below(root, link, dest); ok {
			r.followLink(o, root, link, linkDest, depth)
		}
	}
}

// stagedScan is what the walk of a staged tree finds: the entries hidden from
// its mount, as directory masks (tmpfs) and file masks (empty-file binds), and
// the symlinks to follow afterwards.
type stagedScan struct {
	dirMasks, fileMasks, links []string
}

// hide records how the entry name, at mask below the tree root, is hidden
// from the mount, and returns the reason to refuse it with, or "" when the
// entry is not refused. walkErr is the error WalkDir reported for the entry.
func (s *stagedScan) hide(fsys FileSystem, name, mask string, walkErr error) string {
	kind := classifyEntry(fsys.Lstat(name))
	if walkErr != nil {
		s.add(mask, kind == entryDir)
		return reasonUninspectableEntry
	}
	switch kind {
	case entryUninspectable:
		s.add(mask, false)
		return reasonUninspectableEntry
	case entrySymlink:
		s.links = append(s.links, name)
		s.add(mask, linksToDir(fsys, name))
	case entrySpecial:
		s.add(mask, false)
		return reasonSpecialFile
	}
	return ""
}

// add hides mask as a directory or as a file.
func (s *stagedScan) add(mask string, dir bool) {
	if dir {
		s.dirMasks = append(s.dirMasks, mask)
	} else {
		s.fileMasks = append(s.fileMasks, mask)
	}
}

// linksToDir reports whether the symlink name resolves to a directory. Any
// other link, including one that cannot be resolved, is hidden as a file;
// followLink then decides whether its target is mounted.
func linksToDir(fsys FileSystem, name string) bool {
	target, err := fsys.EvalSymlinks(name)
	if err != nil {
		return false
	}
	info, err := fsys.Stat(target)
	return err == nil && info.IsDir()
}
