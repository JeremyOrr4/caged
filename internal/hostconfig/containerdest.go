package hostconfig

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/JeremyOrr4/caged/internal/plan"
)

// containerLinkDest returns where a symlink at linkDest, a container path,
// with link text text points inside the container, or false when caged will
// not mount there. Relative text may climb with ".." only at its start, so
// the lexical result is what the kernel resolves. Absolute text must lie
// inside one of hostRoots (the host home and the link allow roots). Either
// result must be neither inside nor above a reserved container path (the
// system directories, /workspace and the container home), except that a
// relative link may point below the container home.
func containerLinkDest(linkDest, text string, hostRoots []string) (string, bool) {
	text = filepath.ToSlash(text)
	if path.IsAbs(text) {
		dest := path.Clean(text)
		if overlapsReserved(dest) {
			return "", false
		}
		for _, root := range hostRoots {
			if _, inside := plan.ContainerRel(dest, filepath.ToSlash(root)); inside {
				return dest, true
			}
		}
		return "", false
	}
	climbing := true
	for part := range strings.SplitSeq(text, "/") {
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
	if rel, inside := plan.ContainerRel(dest, containerHome); inside && rel != "." {
		return dest, true
	}
	if overlapsReserved(dest) {
		return "", false
	}
	return dest, true
}

// overlapsReserved reports whether dest is inside a reserved container path
// or holds one, so a mount there would cover it.
func overlapsReserved(dest string) bool {
	for _, reserved := range reservedContainerPaths() {
		_, inside := plan.ContainerRel(dest, reserved)
		_, holds := plan.ContainerRel(reserved, dest)
		if inside || holds {
			return true
		}
	}
	return false
}

// reservedContainerPaths are container directories no symlink target is ever
// mounted into.
func reservedContainerPaths() []string {
	return []string{"/bin", "/boot", "/dev", "/etc", "/lib", "/lib32", "/lib64", "/libx32", "/opt", "/proc",
		"/root", "/run", "/sbin", "/sys", "/tmp", "/usr", "/var", "/workspace", containerHome}
}
