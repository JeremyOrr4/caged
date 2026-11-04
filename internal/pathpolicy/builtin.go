package pathpolicy

import (
	"path/filepath"
	"slices"
)

// BuiltinDeny returns the paths no config symlink may ever point into or
// contain: credentials, keyrings, caged's own config (so an agent cannot widen
// its own policy), and host system directories. own are caged's own folders
// wherever the environment put them (its config and approval-state folders
// under XDG_CONFIG_HOME and XDG_STATE_HOME); they come last, so a path the
// fixed entries deny is still decided by the same entry.
func BuiltinDeny(home string, own ...string) []string {
	relative := []string{
		".ssh", ".gnupg", ".aws", ".azure", ".kube", ".docker", ".config/gcloud",
		".password-store", ".netrc", ".git-credentials", ".local/share/keyrings",
		"Library/Keychains", ".config/caged",
	}
	paths := make([]string, 0, len(relative)+3+len(own))
	for _, rel := range relative {
		paths = append(paths, filepath.Join(home, rel))
	}
	paths = append(paths, "/etc", "/run", "/var/run")
	for _, dir := range own {
		if dir = filepath.Clean(dir); !slices.Contains(paths, dir) {
			paths = append(paths, dir)
		}
	}
	return paths
}
