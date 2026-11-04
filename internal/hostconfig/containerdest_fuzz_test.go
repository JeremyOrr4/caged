package hostconfig

import (
	"path"
	"slices"
	"strings"
	"testing"
)

// FuzzContainerLinkDest checks containerLinkDest against its contract, written
// here independently of the code. A relative link climbs with ".." only at
// its start and lands where walking its components from the link's directory
// leads: strictly below the container home, or anywhere neither inside nor
// holding a reserved container path (the system directories, /workspace and
// the container home). An absolute link is its own clean path, inside one of
// the host roots, and neither inside nor holding a reserved container path.
func FuzzContainerLinkDest(f *testing.F) {
	for _, seed := range [][4]string{
		{"/home/caged/.codex/a", "../dotfiles/a", "", ""},
		{"/home/caged/.codex/a", "a/../b", "", ""},
		{"/home/caged/.codex/a", "../../../etc/x", "", ""},
		{"/home/caged/.codex/a", "..", "", ""},
		{"/home/caged/.codex/a", "/usr/bin/x", "/usr", ""},
		{"/home/caged/.codex/a", "/Users/me/dot/a", "/Users/me", ""},
		{"/home/caged/.codex/a", "/Users/me/dot/a", "/other", "/Users/me"},
		{"/home/caged/.codex/a", "/other/a", "/Users/me", ""},
		{"/home/caged/.codex/a", "/home/caged/x", "/home", ""},
		{"/home/caged/.codex/a", "/Users/me/../../etc/x", "/Users/me", ""},
		{"/home/caged/.codex/a", "./../.././x", "", ""},
		{"/home/caged/.codex/a/b", "../../x//y/./z", "", ""},
		{"/home/caged/.codex/a", "/Users/me", "/Users/me/", ""},
		{"/home/caged/.codex/a", "/varx/a", "/", ""},
		{"/home/caged/.codex/a", "/home", "/home", ""},
		{"/home/caged/.codex/a", "/", "/", ""},
		{"/home/caged/.codex/a", "../../../shared/a", "", ""},
		{"/home/caged/.codex/a", "../../..", "", ""},
		{"/home/u/dotfiles/agents/a", "b.md", "/home/u", ""},
		{"/home/u/dotfiles/agents/a", "../../../../etc/x", "/home/u", ""},
	} {
		f.Add(seed[0], seed[1], seed[2], seed[3])
	}
	f.Fuzz(func(t *testing.T, linkDest, text, root1, root2 string) {
		linkDest, text, root1, root2 = truncate(linkDest), truncate(text), truncate(root1), truncate(root2)
		if !strings.HasPrefix(linkDest, "/") || linkDest == "/" || linkDest != path.Clean(linkDest) {
			return // linkDest is always a clean absolute container path below "/"
		}
		roots := []string{root1, root2}
		wantDest, wantOK := expectedLinkDest(linkDest, text, roots)
		dest, ok := containerLinkDest(linkDest, text, roots)
		if dest != wantDest || ok != wantOK {
			t.Fatalf("containerLinkDest(%q, %q, %q) = %q, %v; want %q, %v", linkDest, text, roots, dest, ok, wantDest, wantOK)
		}
	})
}

// expectedLinkDest is the contract of containerLinkDest, by components.
func expectedLinkDest(linkDest, text string, roots []string) (string, bool) {
	reserved := [][]string{{"bin"}, {"boot"}, {"dev"}, {"etc"}, {"lib"}, {"lib32"}, {"lib64"}, {"libx32"}, {"opt"}, {"proc"},
		{"root"}, {"run"}, {"sbin"}, {"sys"}, {"tmp"}, {"usr"}, {"var"}, {"workspace"}, {"home", "caged"}}
	overlaps := func(dest []string) bool {
		for _, dir := range reserved {
			if hasPrefix(dest, dir) || hasPrefix(dir, dest) {
				return true // inside a system directory, or holding one
			}
		}
		return false
	}
	if strings.HasPrefix(text, "/") {
		dest := walk(nil, text)
		if overlaps(dest) {
			return "", false
		}
		for _, root := range roots {
			if strings.HasPrefix(root, "/") && hasPrefix(dest, walk(nil, root)) {
				return "/" + strings.Join(dest, "/"), true
			}
		}
		return "", false
	}
	named := false
	for part := range strings.SplitSeq(text, "/") {
		if part == ".." && named {
			return "", false
		}
		named = named || (part != ".." && part != "" && part != ".")
	}
	dest := walk(walk(nil, linkDest[:strings.LastIndex(linkDest, "/")]), text)
	if len(dest) >= 3 && hasPrefix(dest, []string{"home", "caged"}) {
		return "/" + strings.Join(dest, "/"), true
	}
	if overlaps(dest) {
		return "", false
	}
	return "/" + strings.Join(dest, "/"), true
}

// walk applies the components of p to the directory dir, given as names
// below "/": ".." climbs (never above "/"), "" and "." stay.
func walk(dir []string, p string) []string {
	dir = slices.Clone(dir)
	for part := range strings.SplitSeq(p, "/") {
		switch part {
		case "", ".":
		case "..":
			if len(dir) > 0 {
				dir = dir[:len(dir)-1]
			}
		default:
			dir = append(dir, part)
		}
	}
	return dir
}

// hasPrefix reports whether the names of p start with those of dir.
func hasPrefix(p, dir []string) bool {
	return len(p) >= len(dir) && slices.Equal(p[:len(dir)], dir)
}

// truncate bounds a fuzz input to 1 KiB.
func truncate(s string) string {
	if len(s) > 1024 {
		return s[:1024]
	}
	return s
}
