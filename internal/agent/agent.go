// Package agent defines the supported agent CLIs as data and keeps them in a Registry.
package agent

import (
	"fmt"
	"strings"
)

// ConfigPath is a host config path, relative to the home directory, that is
// mounted at the same relative path under the container home.
type ConfigPath struct {
	HostPath      string
	ContainerPath string
	Writable      bool
	EnvOverride   string
	MaskPaths     []string
	// Staged mounts the path as one tree, so the agent can create files as
	// well as edit them. Docker reads it from a symlink-free copy made for the
	// run: symlinks and special files are hidden, allowed symlink targets are
	// mounted back at their link paths, and the container's changes are
	// written back to the host when it exits.
	Staged bool
}

// PromptStyle says how an agent accepts an initial prompt on its command line.
type PromptStyle int

// The prompt styles. The values are part of the agent golden file.
const (
	PromptNone       PromptStyle = iota // takes no initial prompt
	PromptPositional                    // the prompt is one argument right after Command
	PromptFlag                          // the prompt follows Agent.PromptFlag
)

// Agent is the complete, data-only definition of a supported agent CLI.
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
// mounted config directories are the v1 mechanisms.
type AuthStrategy struct {
	Environment []string
}

// LaunchCommand returns the container command that starts the agent: the
// command, then the prompt part, then a copy of extra. A non-nil prompt is
// passed as exactly one argument in the agent's prompt style. extra is added
// unchanged for every prompt style, including agents that take no prompt.
func (a Agent) LaunchCommand(prompt *string, extra []string) ([]string, error) {
	command := append([]string(nil), a.Command...)
	if prompt != nil {
		switch a.Prompt {
		case PromptPositional:
			command = append(command, *prompt)
		case PromptFlag:
			if a.PromptFlag == "" {
				return nil, fmt.Errorf("%s prompt flag is not configured", a.Name)
			}
			command = append(command, a.PromptFlag, *prompt)
		default:
			return nil, fmt.Errorf("%s does not support prompt launch", a.Name)
		}
	}
	return append(command, extra...), nil
}

// ValidateEnvName rejects names that are not valid environment variable names.
func ValidateEnvName(name string) error {
	if !validEnvName(strings.TrimSpace(name)) {
		return fmt.Errorf("invalid env name %q; must match ^[A-Za-z_][A-Za-z0-9_]*$", name)
	}
	return nil
}

// validEnvName reports whether name matches ^[A-Za-z_][A-Za-z0-9_]*$.
func validEnvName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case r == '_', 'A' <= r && r <= 'Z', 'a' <= r && r <= 'z':
		case i > 0 && '0' <= r && r <= '9':
		default:
			return false
		}
	}
	return true
}

// ResolveEnv returns the names from EnvNames that isSet reports as present in
// the host environment, or nil when there are none.
func (a Agent) ResolveEnv(requested []string, isSet func(string) bool) []string {
	allowed := make([]string, 0, len(a.Auth.Environment)+len(requested))
	for _, name := range a.EnvNames(requested) {
		if isSet(name) {
			allowed = append(allowed, name)
		}
	}
	if len(allowed) == 0 {
		return nil
	}
	return allowed
}

// EnvNames returns the agent's default environment names followed by the
// requested ones, trimmed, de-duplicated, and with invalid names dropped.
func (a Agent) EnvNames(requested []string) []string {
	seen := make(map[string]struct{}, len(a.Auth.Environment)+len(requested))
	names := make([]string, 0, len(a.Auth.Environment)+len(requested))
	for _, name := range append(append([]string{}, a.Auth.Environment...), requested...) {
		name = strings.TrimSpace(name)
		if !validEnvName(name) {
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

// clone returns a deep copy so callers can never modify a registered definition.
func (a Agent) clone() Agent {
	a.Command = append([]string(nil), a.Command...)
	a.ConfigPaths = append([]ConfigPath(nil), a.ConfigPaths...)
	for i := range a.ConfigPaths {
		a.ConfigPaths[i].MaskPaths = append([]string(nil), a.ConfigPaths[i].MaskPaths...)
	}
	a.Auth.Environment = append([]string(nil), a.Auth.Environment...)
	return a
}
