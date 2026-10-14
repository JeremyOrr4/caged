// Package plan contains transport-neutral data for a disposable agent run.
package plan

import "io/fs"

// Mount describes a host path exposed at a container path.
type Mount struct{ HostPath, ContainerPath string }

// SharedOwner is the ConfigMount.Agent value of mounts shared by all agents.
// No agent may use this name.
const SharedOwner = "shared"

// ConfigMount includes ownership and access information for host configuration.
type ConfigMount struct {
	Agent, Name, HostPath, DestPath string
	// Root is the config root on the host after any override, before symlink
	// resolution. HostPath is where the mount actually comes from.
	Root      string
	Writable  bool
	MaskPaths []string
}

// ConfigReport is the outcome of host config discovery: the mounts to make,
// the paths that do not exist, and the paths refused with their reasons.
type ConfigReport struct {
	Mounts  []ConfigMount
	Missing []string
	Refused []Refusal
}

// Refusal is a host config path that is not mounted, and why.
type Refusal struct{ Path, Reason string }

// String formats the refusal as "path (reason)".
func (r Refusal) String() string { return r.Path + " (" + r.Reason + ")" }

// RunSpec is an immutable-by-convention description consumed by a runtime adapter.
type RunSpec struct {
	Image         string
	Command       []string
	Mount         *Mount
	Ports         []Port
	ConfigMounts  []ConfigMount
	EnvVars       []string
	ReturnToShell bool
}

// BuildSpec describes an image build from an in-memory build context.
type BuildSpec struct {
	Image       string  // tag of the image to build
	Dockerfile  string  // name of the Dockerfile inside Files
	ContextName string  // names the temporary build directory
	Files       []File  // the build context, written in order
	Labels      []Label // image labels, in the order given to docker build
	NoCache     bool
}

// BuildOptions are the per-build settings that do not depend on the image.
type BuildOptions struct{ NoCache bool }

// Extension is the user's request for extra packages in the image, read from
// image.json. Apt is sorted and has no duplicates.
type Extension struct {
	Source string // the file it came from, shown with ~ for the home directory
	Apt    []string
}

// Label is one image label.
type Label struct{ Key, Value string }

// File is one file of an image build context.
type File struct {
	Name  string // file name inside the build context
	Label string // short name used in error messages
	Data  []byte
	Mode  fs.FileMode
}
