package userconfig

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// validate checks the decoded file and turns it into a config. source names
// the file in error messages.
func (f fileV1) validate(source string) (config, error) {
	if f.Version != nil && *f.Version != 1 {
		return config{}, fmt.Errorf("%s: unsupported version %d; this caged reads version 1", source, *f.Version)
	}
	apt := orEmpty(f.Image.Apt)
	for _, name := range apt {
		if !validAptName(name) {
			return config{}, fmt.Errorf("%s: invalid apt package name %q in image.apt; names must match ^[a-z0-9][a-z0-9+.-]+$", source, name)
		}
	}
	cfg := config{apt: sortedUnique(apt)}
	for _, root := range f.Links.Allow {
		if root == nil {
			return config{}, nullRoot(source, "links.allow")
		}
		if err := checkPath(source, "links.allow", root.Path); err != nil {
			return config{}, err
		}
		cfg.linkAllow = append(cfg.linkAllow, allowRoot{path: root.Path, readOnly: root.ReadOnly})
	}
	for _, path := range orEmpty(f.Links.Deny) {
		if err := checkPath(source, "links.deny", path); err != nil {
			return config{}, err
		}
		cfg.linkDeny = append(cfg.linkDeny, path)
	}
	if f.Workdirs != nil {
		cfg.workdirsSet = true
		for _, root := range f.Workdirs.Allow {
			if root == nil {
				return config{}, nullRoot(source, "workdirs.allow")
			}
			if err := checkPath(source, "workdirs.allow", root.Path); err != nil {
				return config{}, err
			}
			cfg.workdirs = append(cfg.workdirs, root.Path)
		}
	}
	return cfg, nil
}

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
	slices.Sort(sorted)
	return slices.Compact(sorted)
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

// nullRoot refuses a root that is null as a whole (a bare ~ or an empty "-").
// A root is a mapping, so the "quote it" advice of checkPath would not help.
func nullRoot(source, field string) error {
	return fmt.Errorf(`%s: %s entry has no path (write each entry as "- path: ~/dir")`, source, field)
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
