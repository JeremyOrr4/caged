package agent

import (
	"fmt"
	"sort"
	"strings"
)

// Registry is an immutable set of agents that can be looked up by name.
type Registry struct {
	byName map[string]Agent
	names  []string
}

// NewRegistry builds a registry. Every agent needs a command and a unique,
// lowercase name without surrounding spaces, because Lookup normalizes user
// input the same way.
func NewRegistry(agents ...Agent) (*Registry, error) {
	registry := &Registry{byName: make(map[string]Agent, len(agents))}
	for _, definition := range agents {
		if definition.Name == "" || definition.Name != strings.TrimSpace(strings.ToLower(definition.Name)) {
			return nil, fmt.Errorf("agent name %q must be lowercase, non-empty, and without surrounding spaces", definition.Name)
		}
		if len(definition.Command) == 0 {
			return nil, fmt.Errorf("agent %q has no command", definition.Name)
		}
		if _, exists := registry.byName[definition.Name]; exists {
			return nil, fmt.Errorf("agent %q is registered more than once", definition.Name)
		}
		registry.byName[definition.Name] = definition.clone()
		registry.names = append(registry.names, definition.Name)
	}
	sort.Strings(registry.names)
	return registry, nil
}

// Lookup finds an agent by name, ignoring case and surrounding spaces.
func (r *Registry) Lookup(name string) (Agent, error) {
	key := strings.TrimSpace(strings.ToLower(name))
	if definition, ok := r.byName[key]; ok {
		return definition.clone(), nil
	}
	return Agent{}, fmt.Errorf("unsupported agent %q; supported agents: %s", name, strings.Join(r.names, ", "))
}

// Names returns the sorted agent names.
func (r *Registry) Names() []string {
	return append([]string(nil), r.names...)
}

// All returns every agent, sorted by name.
func (r *Registry) All() []Agent {
	agents := make([]Agent, 0, len(r.names))
	for _, name := range r.names {
		agents = append(agents, r.byName[name].clone())
	}
	return agents
}
