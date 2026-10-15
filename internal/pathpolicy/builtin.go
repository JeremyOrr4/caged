package pathpolicy

import "path/filepath"

// BuiltinDeny returns the paths no config symlink may ever point into or
// contain: credentials, keyrings, caged's own config (so an agent cannot widen
// its own policy), and host system directories.
func BuiltinDeny(home string) []string {
	relative := []string{
		".ssh", ".gnupg", ".aws", ".azure", ".kube", ".docker", ".config/gcloud",
		".password-store", ".netrc", ".git-credentials", ".local/share/keyrings",
		"Library/Keychains", ".config/caged",
	}
	paths := make([]string, 0, len(relative)+3)
	for _, rel := range relative {
		paths = append(paths, filepath.Join(home, rel))
	}
	return append(paths, "/etc", "/run", "/var/run")
}
