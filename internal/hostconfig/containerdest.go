package hostconfig

import (
	"path"
	"path/filepath"
	"strings"
)

// containerLinkDest returns where a symlink at linkDest, a container path,
// with link text text points inside the container, or false when caged will
// not mount there. Relative text may climb with ".." only at its start, so
// the lexical result is what the kernel resolves, and must stay below the
// container home. Absolute text must lie inside one of hostRoots (the host
// home and the link allow roots) and outside the system directories.
func containerLinkDest(linkDest, text string, hostRoots []string) (string, bool) {
	text = filepath.ToSlash(text)
	if path.IsAbs(text) {
		dest := path.Clean(text)
		for _, reserved := range reservedContainerPaths() {
			if withinSlash(dest, reserved) {
				return "", false
			}
		}
		for _, root := range hostRoots {
			if withinSlash(dest, filepath.ToSlash(root)) {
				return dest, true
			}
		}
		return "", false
	}
	climbing := true
	for _, part := range strings.Split(text, "/") {
		switch part {
		case "..":
			if !climbing {
				return "", false
			}
		case "", ".":
		default:
			climbing = false
		}
	}
	dest := path.Join(path.Dir(linkDest), text)
	if dest == containerHome || !withinSlash(dest, containerHome) {
		return "", false
	}
	return dest, true
}

// reservedContainerPaths are container directories no symlink target is ever
// mounted into.
func reservedContainerPaths() []string {
	return []string{"/bin", "/boot", "/dev", "/etc", "/lib", "/lib32", "/lib64", "/libx32", "/opt", "/proc",
		"/root", "/run", "/sbin", "/sys", "/tmp", "/usr", "/var", "/workspace", containerHome}
}

// withinSlash reports whether the slash-separated path p is root or inside it.
func withinSlash(p, root string) bool {
	p, root = path.Clean(p), path.Clean(root)
	return p == root || root == "/" || strings.HasPrefix(p, root+"/")
}
