package hostconfig

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/JeremyOrr4/caged/internal/plan"
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
	maskPaths := make([]string, 0, len(masks))
	for _, mask := range masks {
		clean := filepath.Clean(mask)
		if filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("invalid config mask path %q", mask)
		}
		maskPaths = append(maskPaths, filepath.Join(root, clean))
	}
	var outside []string
	if err := fsys.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path != root {
			for _, mask := range maskPaths {
				if path == mask || isWithin(path, mask) {
					if entry.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}
		}
		info, err := fsys.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			if resolved, err := fsys.EvalSymlinks(path); err != nil || !isWithin(resolved, root) {
				outside = append(outside, path)
			}
			return nil
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("special file %q is not covered by a config mask", path)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return outside, nil
}

// collectSafeMounts returns the largest subtrees of root that contain only
// regular files and directories, plus the symlinks it found and a reason for
// every other entry it skipped.
func collectSafeMounts(fsys FileSystem, root string) (mounts, links []string, refused []plan.Refusal) {
	var collect func(string)
	collect = func(path string) {
		entries, err := fsys.ReadDir(path)
		if err != nil {
			info, statErr := fsys.Lstat(path)
			if statErr == nil && info.Mode().IsRegular() {
				mounts = append(mounts, path)
			} else {
				refused = append(refused, plan.Refusal{Path: path, Reason: "cannot inspect config entry safely"})
			}
			return
		}
		if subtreeIsSafe(fsys, path) {
			mounts = append(mounts, path)
			return
		}
		for _, entry := range entries {
			child := filepath.Join(path, entry.Name())
			info, err := fsys.Lstat(child)
			if err != nil {
				refused = append(refused, plan.Refusal{Path: child, Reason: "cannot inspect config entry safely"})
				continue
			}
			if info.IsDir() {
				collect(child)
			} else if info.Mode().IsRegular() {
				mounts = append(mounts, child)
			} else if info.Mode()&fs.ModeSymlink != 0 {
				links = append(links, child)
			} else {
				refused = append(refused, plan.Refusal{Path: child, Reason: "special files are excluded from config mounts"})
			}
		}
	}
	info, err := fsys.Lstat(root)
	if err != nil {
		return nil, nil, []plan.Refusal{{Path: root, Reason: "cannot inspect config root safely"}}
	}
	if info.Mode().IsRegular() {
		return []string{root}, nil, nil
	}
	if !info.IsDir() {
		return nil, nil, []plan.Refusal{{Path: root, Reason: "config root is not a regular file or directory"}}
	}
	collect(root)
	return mounts, links, refused
}

// subtreeIsSafe reports whether root contains only regular files and directories.
func subtreeIsSafe(fsys FileSystem, root string) bool {
	entries, err := fsys.ReadDir(root)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		info, err := fsys.Lstat(path)
		if err != nil {
			return false
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return false
		}
		if info.IsDir() {
			if !subtreeIsSafe(fsys, path) {
				return false
			}
		} else if !info.Mode().IsRegular() {
			return false
		}
	}
	return true
}

func appendUnique[T comparable](values []T, value T) []T {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

// isWithin reports whether path is root itself or inside it.
func isWithin(path, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
