package app

import (
	"fmt"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/pathpolicy"
	"github.com/JeremyOrr4/caged/internal/plan"
)

// AgentStatus is one row of `caged list`: an agent, the host config that a run
// would mount for it, and which of its credential variables are set.
type AgentStatus struct {
	Name   string
	Config []string // host paths shown with ~, empty when none was found
	EnvSet []string // names of the agent's auth variables that are set
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
		statuses = append(statuses, AgentStatus{
			Name:   definition.Name,
			Config: describeRoots(home, roots[definition.Name]),
			EnvSet: definition.ResolveEnv(nil, envSet(l.env)),
		})
	}
	return statuses, nil
}

// describeRoots turns the root of each mount into one display entry per
// distinct root, with the home directory shown as "~". A root's link targets
// are mounts of their own, so a root can appear more than once in roots.
func describeRoots(home string, roots []string) []string {
	var described []string
	seen := map[string]bool{}
	for _, root := range roots {
		if !seen[root] {
			seen[root] = true
			described = append(described, pathpolicy.Tilde(home, root))
		}
	}
	return described
}
