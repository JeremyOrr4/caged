package hostconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/plan"
)

type ResolvedMount = plan.ConfigMount
type ConfigReport struct {
	Mounts  []ResolvedMount
	Missing []string
	Refused []string
}

func ResolveAllConfigsReport(home string, exists func(string) bool) ConfigReport {
	report := ConfigReport{}
	if exists == nil {
		exists = func(path string) bool {
			_, err := os.Stat(path)
			return err == nil
		}
	}
	home = filepath.Clean(home)
	resolvedHome, err := filepath.EvalSymlinks(home)
	if err != nil {
		report.Refused = append(report.Refused, home+" (home cannot be resolved)")
		return report
	}
	seen := make(map[string]struct{})
	for _, name := range agent.Names() {
		entry, _ := agent.Lookup(name)
		for _, path := range entry.ConfigPaths {
			hostPath := filepath.Join(home, filepath.Clean(path.HostPath))
			if path.EnvOverride != "" {
				if override := os.Getenv(path.EnvOverride); override != "" {
					if !filepath.IsAbs(override) {
						override = filepath.Join(home, override)
					}
					hostPath = filepath.Clean(override)
				}
			}
			if !exists(hostPath) {
				report.Missing = appendUnique(report.Missing, hostPath)
				continue
			}
			resolvedHost, err := filepath.EvalSymlinks(hostPath)
			if err != nil {
				report.Refused = appendUnique(report.Refused, hostPath+" (unresolvable symlink)")
				continue
			}
			if filepath.Clean(resolvedHost) == filepath.Clean(resolvedHome) {
				report.Refused = appendUnique(report.Refused, hostPath+" (config path resolves to the home directory)")
				continue
			}
			if !IsWithinHome(resolvedHost, resolvedHome) {
				report.Refused = appendUnique(report.Refused, hostPath+" (resolves outside home)")
				continue
			}
			if err := validateConfigTree(resolvedHost); err != nil {
				report.Refused = appendUnique(report.Refused, hostPath+" ("+err.Error()+")")
				continue
			}

			key := hostPath + "\x00" + filepath.Clean(path.ContainerPath)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}

			report.Mounts = append(report.Mounts, ResolvedMount{
				Agent:    entry.Name,
				Name:     filepath.Clean(path.HostPath),
				HostPath: resolvedHost,
				DestPath: filepath.ToSlash(filepath.Join("/home/caged", filepath.Clean(path.ContainerPath))),
				Writable: path.Writable,
			})
		}
	}
	// Git identity is shared across agents rather than duplicated in each entry.
	sharedGitConfig := filepath.Join(home, ".gitconfig")
	if exists(sharedGitConfig) {
		resolved, resolveErr := filepath.EvalSymlinks(sharedGitConfig)
		if resolveErr == nil && IsWithinHome(resolved, resolvedHome) {
			if validateErr := validateConfigTree(resolved); validateErr == nil {
				if info, statErr := os.Stat(resolved); statErr == nil && info.Mode().IsRegular() {
					report.Mounts = append(report.Mounts, ResolvedMount{Agent: "shared", Name: ".gitconfig", HostPath: resolved, DestPath: "/home/caged/.gitconfig", Writable: false})
				} else {
					report.Refused = appendUnique(report.Refused, sharedGitConfig+" (shared git config must be a regular file)")
				}
			}
		} else {
			report.Refused = appendUnique(report.Refused, sharedGitConfig+" (resolves outside home or cannot be resolved)")
		}
	} else {
		report.Missing = appendUnique(report.Missing, sharedGitConfig)
	}
	return report
}

func validateConfigTree(root string) error {
	info, err := os.Lstat(root)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		resolved, err := filepath.EvalSymlinks(root)
		if err != nil || filepath.Clean(resolved) != filepath.Clean(root) {
			return fmt.Errorf("config root symlink cannot be resolved safely")
		}
		info, err = os.Stat(resolved)
		if err != nil {
			return err
		}
	}
	if !info.IsDir() {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("special files are not mounted")
		}
		return nil
	}
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil || !IsWithinHome(resolved, root) {
				return fmt.Errorf("symlink %q resolves outside the config directory or cannot be resolved", path)
			}
			info, err = os.Stat(resolved)
			if err != nil {
				return err
			}
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("special file %q is not mounted", path)
		}
		return nil
	})
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func IsWithinHome(path, home string) bool {
	path = filepath.Clean(path)
	home = filepath.Clean(home)
	rel, err := filepath.Rel(home, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
