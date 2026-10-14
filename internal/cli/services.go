package cli

import (
	"context"

	"github.com/JeremyOrr4/caged/internal/app"
)

// RunService runs an agent or shell in a container. *app.Runner implements it.
type RunService interface {
	Run(ctx context.Context, request app.Request) error
}

// BuildService builds the runtime images. *app.Builder implements it.
type BuildService interface {
	Build(ctx context.Context, opts app.BuildOptions) ([]app.BuiltImage, error)
}

// DoctorService checks the machine and the images. *app.Doctor implements it.
type DoctorService interface {
	Diagnose(ctx context.Context, opts app.DoctorOptions) app.DoctorReport
}

// ListService describes the supported agents. *app.Lister implements it.
type ListService interface {
	List() ([]app.AgentStatus, error)
}

// CleanService removes caged's images. *app.Cleaner implements it.
type CleanService interface {
	Clean(ctx context.Context, opts app.CleanOptions) ([]app.CleanResult, error)
}

// Services are the use cases the commands call.
type Services struct {
	Run    RunService
	Build  BuildService
	Doctor DoctorService
	List   ListService
	Clean  CleanService
}
