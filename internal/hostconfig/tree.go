package hostconfig

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/JeremyOrr4/caged/internal/pathpolicy"
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

// appendUnique appends value to values unless values already holds it.
func appendUnique[T comparable](values []T, value T) []T {
	if slices.Contains(values, value) {
		return values
	}
	return append(values, value)
}
