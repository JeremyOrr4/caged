package extension

import "sort"

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
