package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/plan"
)

// AgentStatus is one row of `caged list`: an agent, the host config that a run
// would mount for it, and which of its credential variables are set.
type AgentStatus struct {
	Name, Description string
	Config            []string // host paths shown with ~, empty when none was found
	EnvSet            []string // names of the agent's auth variables that are set
}

// Lister is the list use case. It needs no Docker.
type Lister struct {
	env     Env
	configs ConfigResolver
	agents  []agent.Agent
}

// NewLister returns a Lister for agents, in the order given.
func NewLister(env Env, configs ConfigResolver, agents []agent.Agent) *Lister {
	return &Lister{env: env, configs: configs, agents: agents}
}

// List reports every agent with the config paths the resolver would mount for
// it, so overrides such as CODEX_HOME and the safety refusals are honored, and
// the names (never the values) of its credential variables that are set.
func (l *Lister) List() ([]AgentStatus, error) {
	home, err := l.env.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("find home directory: %w", err)
	}
	report, err := l.configs.Resolve(home)
	if err != nil {
		return nil, err
	}
	roots := map[string][]string{} // agent name -> mount roots, one entry per mount
	for _, mount := range report.Mounts {
		if mount.Agent == plan.SharedOwner {
			continue
		}
		roots[mount.Agent] = append(roots[mount.Agent], mount.Root)
	}
	statuses := make([]AgentStatus, 0, len(l.agents))
	for _, definition := range l.agents {
		status := AgentStatus{Name: definition.Name, Description: definition.Description, Config: describeRoots(home, roots[definition.Name])}
		for _, name := range definition.Auth.Environment {
			if _, ok := l.env.LookupEnv(name); ok {
				status.EnvSet = append(status.EnvSet, name)
			}
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

// describeRoots turns the root of each mount into one display entry per
// distinct root: the path with the home directory shown as "~", and the
// number of files when one root was split into several mounts.
func describeRoots(home string, roots []string) []string {
	var order []string
	counts := map[string]int{}
	for _, root := range roots {
		if counts[root] == 0 {
			order = append(order, root)
		}
		counts[root]++
	}
	var described []string
	for _, root := range order {
		entry := tildePath(home, root)
		if counts[root] > 1 {
			entry += fmt.Sprintf(" (%d files)", counts[root])
		}
		described = append(described, entry)
	}
	return described
}

// tildePath shows path with the home directory replaced by "~".
func tildePath(home, path string) string {
	home = filepath.Clean(home)
	if rest, ok := strings.CutPrefix(filepath.Clean(path), home+string(filepath.Separator)); ok {
		return "~/" + filepath.ToSlash(rest)
	}
	return path
}
