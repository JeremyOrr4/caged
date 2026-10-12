package agent

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

type ConfigPath struct {
	HostPath      string
	ContainerPath string
	Writable      bool
	EnvOverride   string
}

type PromptStyle int

const (
	PromptNone PromptStyle = iota
	PromptPositional
	PromptFlag
)

type Agent struct {
	Name        string
	Command     []string
	Description string
	ConfigPaths []ConfigPath
	Prompt      PromptStyle
	PromptFlag  string
	Auth        AuthStrategy
}

// AuthStrategy describes how an agent can authenticate in the container.
// Keychain extraction is intentionally unsupported; host-provided tokens and
// mounted credential files are the v1 mechanisms.
type AuthStrategy struct {
	Environment         []string
	CredentialPaths     []string
	SupportsDeviceLogin bool
}

var envNameRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

var registry = map[string]Agent{
	"pi": {
		Name:        "pi",
		Command:     []string{"pi"},
		Description: "Pi coding agent",
		ConfigPaths: []ConfigPath{
			{HostPath: ".config/pi", ContainerPath: ".config/pi", Writable: true},
			{HostPath: ".pi", ContainerPath: ".pi", Writable: true},
		},
		Prompt: PromptPositional,
		Auth:   AuthStrategy{Environment: []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY"}},
	},
	"claude": {
		Name:        "claude",
		Command:     []string{"claude"},
		Description: "Claude Code",
		ConfigPaths: []ConfigPath{
			{HostPath: ".claude", ContainerPath: ".claude", Writable: true, EnvOverride: "CLAUDE_CONFIG_DIR"},
		},
		Prompt: PromptPositional,
		Auth:   AuthStrategy{Environment: []string{"ANTHROPIC_API_KEY"}, CredentialPaths: []string{".claude/.credentials.json"}},
	},
	"codex": {
		Name:        "codex",
		Command:     []string{"codex"},
		Description: "Codex CLI",
		ConfigPaths: []ConfigPath{
			{HostPath: ".codex", ContainerPath: ".codex", Writable: true, EnvOverride: "CODEX_HOME"},
		},
		Prompt: PromptFlag, PromptFlag: "exec",
		Auth: AuthStrategy{Environment: []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY"}, CredentialPaths: []string{".codex/auth.json"}, SupportsDeviceLogin: true},
	},
	"gemini": {
		Name:        "gemini",
		Command:     []string{"gemini"},
		Description: "Gemini CLI",
		ConfigPaths: []ConfigPath{
			{HostPath: ".gemini", ContainerPath: ".gemini", Writable: true},
		},
		Prompt: PromptFlag, PromptFlag: "-i",
		Auth: AuthStrategy{Environment: []string{"GEMINI_API_KEY", "GOOGLE_API_KEY"}, CredentialPaths: []string{".gemini/oauth_creds.json"}},
	},
	"copilot": {
		Name:        "copilot",
		Command:     []string{"copilot"},
		Description: "GitHub Copilot CLI",
		ConfigPaths: []ConfigPath{
			{HostPath: ".copilot", ContainerPath: ".copilot", Writable: true, EnvOverride: "COPILOT_HOME"},
		},
		Prompt: PromptFlag, PromptFlag: "-p",
		Auth: AuthStrategy{Environment: []string{"COPILOT_GITHUB_TOKEN", "GH_TOKEN", "GITHUB_TOKEN"}, CredentialPaths: []string{".copilot/"}, SupportsDeviceLogin: true},
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
	allowed := make([]string, 0, len(agent.Auth.Environment)+len(requested))
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
	seen := make(map[string]struct{}, len(agent.Auth.Environment)+len(requested))
	names := make([]string, 0, len(agent.Auth.Environment)+len(requested))
	for _, name := range append(append([]string{}, agent.Auth.Environment...), requested...) {
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
