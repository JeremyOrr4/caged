// Package plan contains transport-neutral data for a disposable agent run.
package plan

import (
	"fmt"

	"github.com/JeremyOrr4/caged/internal/agent"
)

// Mount describes a host path exposed at a container path.
type Mount struct{ HostPath, ContainerPath string }

// ConfigMount includes ownership and access information for host configuration.
type ConfigMount struct {
	Agent, Name, HostPath, DestPath string
	Writable                        bool
	MaskPaths                       []string
}

// RunSpec is an immutable-by-convention description consumed by a runtime adapter.
type RunSpec struct {
	Image         string
	Command       []string
	Mount         *Mount
	ConfigMounts  []ConfigMount
	EnvVars       []string
	ReturnToShell bool
}

// BuildRunSpec combines resolved inputs into a complete container invocation.
func BuildRunSpec(image string, selected *agent.Agent, prompt *string, projectMount *Mount, configMounts []ConfigMount, envVars []string) (RunSpec, error) {
	var mountCopy *Mount
	if projectMount != nil {
		copied := *projectMount
		mountCopy = &copied
	}
	configCopies := append([]ConfigMount(nil), configMounts...)
	for i := range configCopies {
		configCopies[i].MaskPaths = append([]string(nil), configCopies[i].MaskPaths...)
	}
	spec := RunSpec{Image: image, Command: []string{"bash"}, Mount: mountCopy, ConfigMounts: configCopies, EnvVars: append([]string(nil), envVars...)}
	if selected == nil {
		if prompt != nil {
			return RunSpec{}, fmt.Errorf("a prompt requires an agent")
		}
		return spec, nil
	}
	spec.Command = append([]string(nil), selected.Command...)
	spec.ReturnToShell = true
	if prompt == nil {
		return spec, nil
	}
	switch selected.Prompt {
	case agent.PromptPositional:
		spec.Command = append(spec.Command, *prompt)
	case agent.PromptFlag:
		if selected.PromptFlag == "" {
			return RunSpec{}, fmt.Errorf("%s prompt flag is not configured", selected.Name)
		}
		spec.Command = append(spec.Command, selected.PromptFlag, *prompt)
	default:
		return RunSpec{}, fmt.Errorf("%s does not support prompt launch", selected.Name)
	}
	return spec, nil
}
