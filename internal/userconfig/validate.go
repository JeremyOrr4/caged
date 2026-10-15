package userconfig

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// validAptName reports whether name is a Debian package name that caged
// accepts: at least two characters of lowercase letters, digits, "+", "-" and
// ".", starting with a letter or digit. This keeps shell syntax, version
// pins ("vim=2:9.1"), and architecture suffixes ("vim:arm64") out of the RUN
// line of the generated Dockerfile.
func validAptName(name string) bool {
	if len(name) < 2 {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case 'a' <= c && c <= 'z', '0' <= c && c <= '9':
		case i > 0 && (c == '+' || c == '-' || c == '.'):
		default:
			return false
		}
	}
	return true
}

// sortedUnique returns the names sorted, without duplicates, or nil when
// there are none.
func sortedUnique(names []string) []string {
	if len(names) == 0 {
		return nil
	}
	sorted := append([]string(nil), names...)
	sort.Strings(sorted)
	unique := sorted[:1]
	for _, name := range sorted[1:] {
		if name != unique[len(unique)-1] {
			unique = append(unique, name)
		}
	}
	return unique
}

// checkPath accepts "~", a path starting with "~/", or an absolute path, and
// nothing with "$" in it: caged does not expand variables.
func checkPath(source, field, path string) error {
	if path == "" {
		return fmt.Errorf("%s: %s entry has no path (a bare ~ is YAML null; quote it as \"~\")", source, field)
	}
	if (path == "~" || strings.HasPrefix(path, "~/") || filepath.IsAbs(path)) && !strings.Contains(path, "$") {
		return nil
	}
	return fmt.Errorf("%s: %s entry %q must be an absolute path or start with ~/", source, field, path)
}

// expand turns a checked path into a host path.
func expand(home, path string) string {
	if path == "~" {
		return filepath.Clean(home)
	}
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		return filepath.Join(home, rest)
	}
	return filepath.Clean(path)
}
