package app

import (
	"fmt"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/plan"
)

// runInputs are the resolved inputs of one container run.
type runInputs struct {
	Image     string
	Agent     *agent.Agent
	Prompt    *string
	AgentArgs []string
	Mount     *plan.Mount
	Ports     []plan.Port
	Configs   []plan.ConfigMount
	Env       []string
}

// buildRunSpec combines resolved inputs into a complete container invocation.
func buildRunSpec(in runInputs) (plan.RunSpec, error) {
	var mountCopy *plan.Mount
	if in.Mount != nil {
		copied := *in.Mount
		mountCopy = &copied
	}
	configCopies := append([]plan.ConfigMount(nil), in.Configs...)
	for i := range configCopies {
		configCopies[i].MaskPaths = append([]string(nil), configCopies[i].MaskPaths...)
	}
	spec := plan.RunSpec{Image: in.Image, Command: []string{"bash"}, Mount: mountCopy, Ports: append([]plan.Port(nil), in.Ports...), ConfigMounts: configCopies, EnvVars: append([]string(nil), in.Env...)}
	if in.Agent == nil {
		if len(in.AgentArgs) > 0 {
			return plan.RunSpec{}, fmt.Errorf("agent arguments after -- need an agent")
		}
		if in.Prompt != nil {
			return plan.RunSpec{}, fmt.Errorf("a prompt requires an agent")
		}
		return spec, nil
	}
	command, err := in.Agent.LaunchCommand(in.Prompt, in.AgentArgs)
	if err != nil {
		return plan.RunSpec{}, err
	}
	spec.Command = command
	spec.ReturnToShell = true
	return spec, nil
}
