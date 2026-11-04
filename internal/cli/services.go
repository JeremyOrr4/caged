package cli

import (
	"context"

	"github.com/JeremyOrr4/caged/internal/app"
)

// RunService runs an agent or shell in a container. cmd/caged wires
// app.GuardedRunner around *app.Runner, so config.yaml is prepared first.
type RunService interface {
	Run(ctx context.Context, request app.Request) error
}

// BuildService builds the runtime images. cmd/caged wires app.GuardedBuilder
// around *app.Builder, so config.yaml is prepared first.
type BuildService interface {
	Build(ctx context.Context, opts app.BuildOptions) ([]app.BuiltImage, error)
}

// DoctorService checks the machine and the images. cmd/caged wires
// app.GuardedDoctor around *app.Doctor, so config.yaml is created if missing
// and a pending change is shown first, never blocking.
type DoctorService interface {
	Diagnose(ctx context.Context, opts app.DoctorOptions) app.DoctorReport
}

// ListService describes the supported agents. cmd/caged wires app.GuardedLister
// around *app.Lister, so config.yaml is created if missing and a pending
// change is shown first, never blocking.
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
