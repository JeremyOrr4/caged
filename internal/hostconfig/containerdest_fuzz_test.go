package hostconfig

import (
	"path"
	"slices"
	"strings"
	"testing"
)

// FuzzContainerLinkDest checks containerLinkDest against its contract, written
// here independently of the code. A relative link climbs with ".." only at
// its start and lands strictly below the container home, where walking its
// components from the link's directory leads. An absolute link is its own
// clean path, inside one of the host roots, and neither inside nor holding a
// reserved container path (the system directories, /workspace and the
// container home).
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
	} {
		f.Add(seed[0], seed[1], seed[2], seed[3])
	}
	f.Fuzz(func(t *testing.T, linkDest, text, root1, root2 string) {
		linkDest, text, root1, root2 = truncate(linkDest), truncate(text), truncate(root1), truncate(root2)
		if !strings.HasPrefix(linkDest, "/home/caged/") || linkDest != path.Clean(linkDest) {
			return // linkDest is always a clean container path below the container home
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
	if strings.HasPrefix(text, "/") {
		dest := walk(nil, text)
		for _, dir := range reserved {
			if hasPrefix(dest, dir) || hasPrefix(dir, dest) {
				return "", false // inside a system directory, or holding one
			}
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
	if len(dest) < 3 || !hasPrefix(dest, []string{"home", "caged"}) {
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
