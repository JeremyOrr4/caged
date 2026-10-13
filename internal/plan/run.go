// Package plan contains transport-neutral data for a disposable agent run.
package plan

import (
	"fmt"
	"io/fs"

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

// ConfigReport is the outcome of host config discovery: the mounts to make,
// the paths that do not exist, and the paths refused with their reasons.
type ConfigReport struct {
	Mounts  []ConfigMount
	Missing []string
	Refused []string
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

// BuildSpec describes an image build from an in-memory build context.
type BuildSpec struct {
	Image      string // tag of the image to build
	Dockerfile string // name of the Dockerfile inside Files
	Files      []File // the build context, written in order
	NoCache    bool
}

// File is one file of an image build context.
type File struct {
	Name  string // file name inside the build context
	Label string // short name used in error messages
	Data  []byte
	Mode  fs.FileMode
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
	command, err := selected.LaunchCommand(prompt)
	if err != nil {
		return RunSpec{}, err
	}
	spec.Command = command
	spec.ReturnToShell = true
	return spec, nil
}
