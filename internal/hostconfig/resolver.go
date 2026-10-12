package hostconfig

import (
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
			mounts, refused := collectSafeMounts(resolvedHost)
			for _, reason := range refused {
				report.Refused = appendUnique(report.Refused, reason)
			}
			for _, mountPath := range mounts {
				relative, err := filepath.Rel(resolvedHost, mountPath)
				if err != nil {
					report.Refused = appendUnique(report.Refused, hostPath+" (cannot map safe config path)")
					continue
				}
				destPath := filepath.Clean(path.ContainerPath)
				if relative != "." {
					destPath = filepath.Join(destPath, relative)
				}
				key := mountPath + "\x00" + destPath
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				report.Mounts = append(report.Mounts, ResolvedMount{
					Agent:    entry.Name,
					Name:     filepath.Clean(path.HostPath),
					HostPath: mountPath,
					DestPath: filepath.ToSlash(filepath.Join("/home/caged", destPath)),
					Writable: path.Writable,
				})
			}
		}
	}
	// Git identity is shared across agents rather than duplicated in each entry.
	sharedGitConfig := filepath.Join(home, ".gitconfig")
	if exists(sharedGitConfig) {
		resolved, resolveErr := filepath.EvalSymlinks(sharedGitConfig)
		if resolveErr == nil && IsWithinHome(resolved, resolvedHome) {
			if info, statErr := os.Stat(resolved); statErr == nil && info.Mode().IsRegular() {
				report.Mounts = append(report.Mounts, ResolvedMount{Agent: "shared", Name: ".gitconfig", HostPath: resolved, DestPath: "/home/caged/.gitconfig", Writable: false})
			} else {
				report.Refused = appendUnique(report.Refused, sharedGitConfig+" (shared git config must be a regular file)")
			}
		} else {
			report.Refused = appendUnique(report.Refused, sharedGitConfig+" (resolves outside home or cannot be resolved)")
		}
	} else {
		report.Missing = appendUnique(report.Missing, sharedGitConfig)
	}
	return report
}

func collectSafeMounts(root string) ([]string, []string) {
	var mounts, refused []string
	var collect func(string)
	collect = func(path string) {
		entries, err := os.ReadDir(path)
		if err != nil {
			info, statErr := os.Lstat(path)
			if statErr == nil && info.Mode().IsRegular() {
				mounts = append(mounts, path)
			} else {
				refused = append(refused, path+" (cannot inspect config entry safely)")
			}
			return
		}
		if subtreeIsSafe(path) {
			mounts = append(mounts, path)
			return
		}
		for _, entry := range entries {
			child := filepath.Join(path, entry.Name())
			info, err := os.Lstat(child)
			if err != nil {
				refused = append(refused, child+" (cannot inspect config entry safely)")
				continue
			}
			if info.IsDir() {
				collect(child)
			} else if info.Mode().IsRegular() {
				mounts = append(mounts, child)
			} else {
				refused = append(refused, child+" (symlinks and special files are excluded from config mounts)")
			}
		}
	}
	info, err := os.Lstat(root)
	if err != nil {
		return nil, []string{root + " (cannot inspect config root safely)"}
	}
	if info.Mode().IsRegular() {
		return []string{root}, nil
	}
	if !info.IsDir() {
		return nil, []string{root + " (config root is not a regular file or directory)"}
	}
	collect(root)
	return mounts, refused
}

func subtreeIsSafe(root string) bool {
	entries, err := os.ReadDir(root)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		info, err := os.Lstat(path)
		if err != nil {
			return false
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return false
		}
		if info.IsDir() {
			if !subtreeIsSafe(path) {
				return false
			}
		} else if !info.Mode().IsRegular() {
			return false
		}
	}
	return true
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
