package app

import (
	"context"

	"github.com/JeremyOrr4/caged/internal/engine"
	"github.com/JeremyOrr4/caged/internal/plan"
)

// Daemon checks that the container engine is installed and running.
// *docker.Client implements it.
type Daemon interface {
	Daemon(ctx context.Context) (engine.DaemonInfo, error)
}

// ImageStore finds and builds images in the container engine's local store.
// *docker.Client implements it.
type ImageStore interface {
	InspectImage(ctx context.Context, ref string) (engine.ImageInfo, error) // KindImageNotFound when absent
	BuildImage(ctx context.Context, spec plan.BuildSpec) (id string, err error)
	RemoveImage(ctx context.Context, idOrRef string) error // an image ID or repository:tag; KindImageInUse when kept
}

// ContainerRunner runs one disposable container to completion.
// *docker.Client implements it.
type ContainerRunner interface {
	RunContainer(ctx context.Context, spec plan.RunSpec) error
}

// RunPlanner turns a Request into the container to run. *Planner implements it.
type RunPlanner interface {
	Plan(request Request, imageRef string, rep *Reporter) (plan.RunSpec, error)
}

// Extensions finds the user's image extension. *userconfig.Store implements it.
type Extensions interface {
	// Load returns the validated extension, or nil when there is none.
	Load() (*plan.Extension, error)
	// Source returns the path of the config file, shown with ~ for the
	// home directory, whether or not the file exists.
	Source() (string, error)
}

// ConfigFile summarizes the user's config file. *userconfig.Store implements it.
type ConfigFile interface {
	Status() (plan.ConfigStatus, error)
}

// ImageInspector looks at one local image. *docker.Client implements it.
type ImageInspector interface {
	InspectImage(ctx context.Context, ref string) (engine.ImageInfo, error) // KindImageNotFound when absent
}

// ImageLister lists local images. *docker.Client implements it.
type ImageLister interface {
	ListImages(ctx context.Context, q engine.ImageQuery) ([]engine.ImageSummary, error)
}

// Prober runs throwaway containers that check what a real run would need.
// *docker.Client implements it.
type Prober interface {
	ReadImageFile(ctx context.Context, ref, path string) ([]byte, error)
	ProbeMount(ctx context.Context, ref string, m plan.Mount) error
}
