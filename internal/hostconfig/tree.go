package hostconfig

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/JeremyOrr4/caged/internal/pathpolicy"
	"github.com/JeremyOrr4/caged/internal/plan"
)

// entryKind is what a config tree entry is, as Lstat sees it: no symlink is
// followed.
type entryKind int

const (
	entryUninspectable entryKind = iota // Lstat failed
	entryDir
	entryFile // a regular file
	entrySymlink
	entrySpecial // a socket, FIFO, device or anything else
)

// classifyEntry reports the kind of an entry from the result of Lstat, so a
// caller writes classifyEntry(fsys.Lstat(path)).
func classifyEntry(info fs.FileInfo, err error) entryKind {
	switch {
	case err != nil:
		return entryUninspectable
	case info.Mode()&fs.ModeSymlink != 0:
		return entrySymlink
	case info.IsDir():
		return entryDir
	case info.Mode().IsRegular():
		return entryFile
	default:
		return entrySpecial
	}
}

// The refusal reasons that more than one walker gives.
const (
	reasonUninspectableEntry = "cannot inspect config entry safely"
	reasonUninspectableRoot  = "cannot inspect config root safely"
	reasonSpecialFile        = "special files are excluded from config mounts"
	reasonRootNotFileOrDir   = "config root is not a regular file or directory"
)

// validateMaskedConfigTree accepts a directory that will be mounted whole and
// returns its symlinks that leave root or cannot be resolved. It fails on any
// special file (socket, FIFO, device) that is not inside one of the masked
// paths.
func validateMaskedConfigTree(fsys FileSystem, root string, masks []string) ([]string, error) {
	rootInfo, err := fsys.Stat(root)
	if err != nil {
		return nil, err
	}
	if !rootInfo.IsDir() {
		return nil, fmt.Errorf("masked config root must be a directory")
	}
	maskPaths, err := maskedPaths(root, masks)
	if err != nil {
		return nil, err
	}
	var outside []string
	if err := fsys.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path != root && insideAny(path, maskPaths) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := fsys.Lstat(path)
		switch classifyEntry(info, err) {
		case entryUninspectable:
			return err
		case entrySymlink:
			if resolved, err := fsys.EvalSymlinks(path); err != nil || !pathpolicy.Within(resolved, root) {
				outside = append(outside, path)
			}
		case entrySpecial:
			return fmt.Errorf("special file %q is not covered by a config mask", path)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return outside, nil
}

// maskedPaths returns the host paths of masks, which must be relative paths
// inside root, or an error naming the first one that is not.
func maskedPaths(root string, masks []string) ([]string, error) {
	paths := make([]string, 0, len(masks))
	for _, mask := range masks {
		clean := filepath.Clean(mask)
		if filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("invalid config mask path %q", mask)
		}
		paths = append(paths, filepath.Join(root, clean))
	}
	return paths, nil
}

// insideAny reports whether path is one of roots or inside one of them.
func insideAny(path string, roots []string) bool {
	return slices.ContainsFunc(roots, func(root string) bool { return pathpolicy.Within(path, root) })
}

// collectSafeMounts returns the largest subtrees of root that contain only
// regular files and directories, plus the symlinks it found and a reason for
// every other entry it skipped.
func collectSafeMounts(fsys FileSystem, root string) (mounts, links []string, refused []plan.Refusal) {
	switch classifyEntry(fsys.Lstat(root)) {
	case entryUninspectable:
		return nil, nil, []plan.Refusal{{Path: root, Reason: reasonUninspectableRoot}}
	case entryFile:
		return []string{root}, nil, nil
	case entryDir:
		scan := &treeScan{fs: fsys}
		scan.scan(root)
		return scan.mounts, scan.links, scan.refused
	default:
		return nil, nil, []plan.Refusal{{Path: root, Reason: reasonRootNotFileOrDir}}
	}
}

// treeScan collects the mounts, links and refusals of one split tree in a
// single pass: each entry is inspected once (a directory that cannot be read,
// twice), however deep the tree.
type treeScan struct {
	fs      FileSystem
	mounts  []string
	links   []string
	refused []plan.Refusal
}

// scan records dir and reports whether it holds only regular files and
// directories. Such a dir is mounted whole, in place of everything below it;
// any other dir is recorded through its entries.
func (s *treeScan) scan(dir string) bool {
	entries, err := s.fs.ReadDir(dir)
	if err != nil {
		if classifyEntry(s.fs.Lstat(dir)) == entryFile {
			s.mounts = append(s.mounts, dir)
		} else {
			s.refuse(dir, reasonUninspectableEntry)
		}
		return false
	}
	below := len(s.mounts)
	safe := true
	for _, entry := range entries {
		if !s.entry(filepath.Join(dir, entry.Name())) {
			safe = false // keep going: every entry of an unsafe dir is recorded
		}
	}
	if safe {
		// Only mounts grow below a safe dir: a link or refusal makes it unsafe.
		s.mounts = append(s.mounts[:below], dir)
	}
	return safe
}

// entry records one entry of a directory and reports whether it is safe to
// mount as part of that directory.
func (s *treeScan) entry(path string) bool {
	switch classifyEntry(s.fs.Lstat(path)) {
	case entryDir:
		return s.scan(path)
	case entryFile:
		s.mounts = append(s.mounts, path)
		return true
	case entrySymlink:
		s.links = append(s.links, path)
	case entrySpecial:
		s.refuse(path, reasonSpecialFile)
	default: // entryUninspectable, and fail safe for any kind added later
		s.refuse(path, reasonUninspectableEntry)
	}
	return false
}

func (s *treeScan) refuse(path, reason string) {
	s.refused = append(s.refused, plan.Refusal{Path: path, Reason: reason})
}

// appendUnique appends value to values unless values already holds it.
func appendUnique[T comparable](values []T, value T) []T {
	if slices.Contains(values, value) {
		return values
	}
	return append(values, value)
}
