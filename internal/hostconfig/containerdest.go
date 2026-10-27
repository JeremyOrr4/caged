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
// the lexical result is what the kernel resolves, and must stay below the
// container home. Absolute text must lie inside one of hostRoots (the host
// home and the link allow roots), and neither inside nor above a reserved
// container path (the system directories, /workspace and the container home).
func containerLinkDest(linkDest, text string, hostRoots []string) (string, bool) {
	text = filepath.ToSlash(text)
	if path.IsAbs(text) {
		dest := path.Clean(text)
		for _, reserved := range reservedContainerPaths() {
			_, inside := plan.ContainerRel(dest, reserved)
			_, holds := plan.ContainerRel(reserved, dest) // a mount there would cover it
			if inside || holds {
				return "", false
			}
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
	if _, inside := plan.ContainerRel(dest, containerHome); dest == containerHome || !inside {
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
