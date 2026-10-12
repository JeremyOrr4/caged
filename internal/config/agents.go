package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type ConfigPath struct {
	HostPath      string
	ContainerPath string
	AuthExcludes  []string
	IsAuth        bool
}

type ResolvedMount struct {
	Agent    string
	Name     string
	HostPath string
	DestPath string
	Excludes []string
	Writable bool
}

type ConfigReport struct {
	Mounts   []ResolvedMount
	Missing  []string
	Excluded []string
	Refused  []string
}

type Agent struct {
	Name        string
	Command     []string
	Description string
	ConfigPaths []ConfigPath
	EnvVars     []string
}

var envNameRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

var registry = map[string]Agent{
	"pi": {
		Name:        "pi",
		Command:     []string{"pi"},
		Description: "Pi coding agent",
		ConfigPaths: []ConfigPath{
			{HostPath: ".config/pi", ContainerPath: ".config/pi"},
			{HostPath: ".pi", ContainerPath: ".pi", AuthExcludes: []string{"auth.json"}},
			{HostPath: ".gitconfig", ContainerPath: ".gitconfig"},
		},
		EnvVars: []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY"},
	},
	"claude": {
		Name:        "claude",
		Command:     []string{"claude"},
		Description: "Claude Code",
		ConfigPaths: []ConfigPath{
			{HostPath: ".claude", ContainerPath: ".claude", AuthExcludes: []string{".credentials.json"}},
			{HostPath: ".claude.json", ContainerPath: ".claude.json", IsAuth: true},
			{HostPath: ".gitconfig", ContainerPath: ".gitconfig"},
		},
		EnvVars: []string{"ANTHROPIC_API_KEY"},
	},
	"codex": {
		Name:        "codex",
		Command:     []string{"codex"},
		Description: "Codex CLI",
		ConfigPaths: []ConfigPath{
			{HostPath: ".codex", ContainerPath: ".codex", AuthExcludes: []string{"auth.json"}},
			{HostPath: ".gitconfig", ContainerPath: ".gitconfig"},
		},
		EnvVars: []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY"},
	},
	"gemini": {
		Name:        "gemini",
		Command:     []string{"gemini"},
		Description: "Gemini CLI",
		ConfigPaths: []ConfigPath{
			{HostPath: ".gemini", ContainerPath: ".gemini", AuthExcludes: []string{"oauth_creds.json"}},
			{HostPath: ".gitconfig", ContainerPath: ".gitconfig"},
		},
		EnvVars: []string{"GEMINI_API_KEY", "GOOGLE_API_KEY"},
	},
}

func ValidateEnvName(name string) error {
	trimmed := strings.TrimSpace(name)
	if !envNameRE.MatchString(trimmed) {
		return fmt.Errorf("invalid env name %q; must match ^[A-Za-z_][A-Za-z0-9_]*$", name)
	}
	return nil
}

func ResolveEnvVars(agent Agent, requested []string, lookup func(string) bool) []string {
	if lookup == nil {
		lookup = func(name string) bool {
			_, ok := os.LookupEnv(name)
			return ok
		}
	}
	allowed := make([]string, 0, len(agent.EnvVars)+len(requested))
	for _, name := range EnvVarNames(agent, requested) {
		if lookup(name) {
			allowed = append(allowed, name)
		}
	}
	if len(allowed) == 0 {
		return nil
	}
	return allowed
}

func EnvVarNames(agent Agent, requested []string) []string {
	seen := make(map[string]struct{}, len(agent.EnvVars)+len(requested))
	names := make([]string, 0, len(agent.EnvVars)+len(requested))
	for _, name := range append(append([]string{}, agent.EnvVars...), requested...) {
		name = strings.TrimSpace(name)
		if name == "" || !envNameRE.MatchString(name) {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return names
}

func Lookup(name string) (Agent, error) {
	key := strings.TrimSpace(strings.ToLower(name))
	if agent, ok := registry[key]; ok {
		return agent, nil
	}

	supported := Names()
	return Agent{}, fmt.Errorf("unsupported agent %q; supported agents: %s", name, strings.Join(supported, ", "))
}

func Names() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func ResolveAllConfigs(home string, exists func(string) bool, excludeAuth bool) []ResolvedMount {
	return ResolveAllConfigsReport(home, exists, excludeAuth).Mounts
}

func ResolveAllConfigsReport(home string, exists func(string) bool, excludeAuth bool) ConfigReport {
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
	for _, name := range Names() {
		agent := registry[name]
		for _, path := range agent.ConfigPaths {
			hostPath := filepath.Join(home, filepath.Clean(path.HostPath))
			if excludeAuth && path.IsAuth {
				report.Excluded = appendUnique(report.Excluded, hostPath)
				continue
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
			if !IsWithinHome(resolvedHost, resolvedHome) {
				report.Refused = appendUnique(report.Refused, hostPath+" (resolves outside home)")
				continue
			}

			key := hostPath + "\x00" + filepath.Clean(path.ContainerPath)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}

			var excludes []string
			if excludeAuth {
				excludes = append(excludes, path.AuthExcludes...)
				for _, pattern := range path.AuthExcludes {
					report.Excluded = appendUnique(report.Excluded, filepath.Join(hostPath, pattern))
				}
			}
			report.Mounts = append(report.Mounts, ResolvedMount{
				Agent:    agent.Name,
				Name:     filepath.Clean(path.HostPath),
				HostPath: resolvedHost,
				DestPath: filepath.ToSlash(filepath.Join("/home/caged", filepath.Clean(path.ContainerPath))),
				Excludes: excludes,
				Writable: agent.Name == "pi" && !excludeAuth,
			})
		}
	}
	return report
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
