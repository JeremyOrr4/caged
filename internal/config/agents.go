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
	Excludes      []string
	IsAuth        bool
}

type ResolvedMount struct {
	Agent      string
	Name       string
	HostPath   string
	StagedPath string
	DestPath   string
	Excludes   []string
}

type Agent struct {
	Name        string
	Command     []string
	Description string
	ConfigPaths []ConfigPath
	EnvVars     []string
	StatePaths  []string
}

var envNameRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

var registry = map[string]Agent{
	"pi": {
		Name:        "pi",
		Command:     []string{"pi"},
		Description: "Pi coding agent",
		ConfigPaths: []ConfigPath{
			{HostPath: ".config/pi", ContainerPath: ".config/pi"},
			{HostPath: ".pi", ContainerPath: ".pi"},
			{HostPath: ".gitconfig", ContainerPath: ".gitconfig"},
		},
		EnvVars:    []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY"},
		StatePaths: []string{".config/pi", ".pi"},
	},
	"claude": {
		Name:        "claude",
		Command:     []string{"claude"},
		Description: "Claude Code",
		ConfigPaths: []ConfigPath{
			{HostPath: ".claude", ContainerPath: ".claude"},
			{HostPath: ".claude.json", ContainerPath: ".claude.json", IsAuth: true},
			{HostPath: ".gitconfig", ContainerPath: ".gitconfig"},
		},
		EnvVars:    []string{"ANTHROPIC_API_KEY"},
		StatePaths: []string{".claude"},
	},
	"codex": {
		Name:        "codex",
		Command:     []string{"codex"},
		Description: "Codex CLI",
		ConfigPaths: []ConfigPath{
			{HostPath: ".codex", ContainerPath: ".codex"},
			{HostPath: ".codex/auth.json", ContainerPath: ".codex/auth.json", IsAuth: true},
			{HostPath: ".gitconfig", ContainerPath: ".gitconfig"},
		},
		EnvVars:    []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY"},
		StatePaths: []string{".codex"},
	},
	"gemini": {
		Name:        "gemini",
		Command:     []string{"gemini"},
		Description: "Gemini CLI",
		ConfigPaths: []ConfigPath{
			{HostPath: ".gemini", ContainerPath: ".gemini"},
			{HostPath: ".gemini/oauth_creds.json", ContainerPath: ".gemini/oauth_creds.json", IsAuth: true},
			{HostPath: ".gitconfig", ContainerPath: ".gitconfig"},
		},
		EnvVars:    []string{"GEMINI_API_KEY", "GOOGLE_API_KEY"},
		StatePaths: []string{".gemini"},
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
	seen := make(map[string]struct{}, len(agent.EnvVars)+len(requested))
	allowed := make([]string, 0, len(agent.EnvVars)+len(requested))
	for _, name := range append(append([]string{}, agent.EnvVars...), requested...) {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" || !envNameRE.MatchString(trimmed) {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		if !lookup(trimmed) {
			continue
		}
		seen[trimmed] = struct{}{}
		allowed = append(allowed, trimmed)
	}
	if len(allowed) == 0 {
		return nil
	}
	return allowed
}

func StateVolumeName(agentName string) string {
	return "caged-" + strings.TrimSpace(strings.ToLower(agentName)) + "-home"
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

func ResolveConfigs(agent Agent, home string, exists func(string) bool, withAuth bool) []ResolvedMount {
	if exists == nil {
		exists = func(path string) bool {
			_, err := os.Stat(path)
			return err == nil
		}
	}

	home = filepath.Clean(home)
	resolvedHome, err := filepath.EvalSymlinks(home)
	if err != nil {
		return nil
	}
	resolved := make([]ResolvedMount, 0, len(agent.ConfigPaths))
	for _, path := range agent.ConfigPaths {
		if !withAuth && path.IsAuth {
			continue
		}

		hostPath := filepath.Join(home, filepath.Clean(path.HostPath))
		if !exists(hostPath) {
			continue
		}

		resolvedHost, err := filepath.EvalSymlinks(hostPath)
		if err != nil {
			continue
		}
		if !IsWithinHome(resolvedHost, resolvedHome) {
			continue
		}

		stagedPath := filepath.Join("/mnt/caged-host", agent.Name, filepath.Clean(path.HostPath))
		destPath := filepath.Join("/home/caged", filepath.Clean(path.ContainerPath))
		resolved = append(resolved, ResolvedMount{
			Agent:      agent.Name,
			Name:       filepath.Clean(path.HostPath),
			HostPath:   filepath.Clean(hostPath),
			StagedPath: filepath.ToSlash(stagedPath),
			DestPath:   filepath.ToSlash(destPath),
			Excludes:   append([]string(nil), path.Excludes...),
		})
	}
	return resolved
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
