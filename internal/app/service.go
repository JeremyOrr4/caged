// Package app holds caged's use cases: run an agent or shell in a container,
// and build the runtime image.
package app

import (
	"context"
	"fmt"
	"io"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/runtime"
)

// Engine is the container runtime caged drives. docker.Client implements it.
type Engine interface {
	ImageExists(ctx context.Context, image string) bool
	Build(ctx context.Context, spec plan.BuildSpec) error
	Run(ctx context.Context, spec plan.RunSpec) error
}

// Request is a parsed command line: what to run and how to set it up.
type Request struct {
	Agent                                       *agent.Agent
	Prompt                                      *string
	NoMount, NoConfig, Verbose, AllowBroadMount bool
	WorkDir                                     string
	EnvVars                                     []string
}

// Service runs caged's use cases.
type Service struct {
	engine  Engine
	planner *Planner
	stderr  io.Writer
}

// New returns a Service that plans runs with planner, executes them on
// engine, and reports progress on stderr.
func New(engine Engine, planner *Planner, stderr io.Writer) *Service {
	return &Service{engine: engine, planner: planner, stderr: stderr}
}

// Build refreshes the runtime image.
func (s *Service) Build(ctx context.Context, noCache bool) error {
	return s.engine.Build(ctx, runtime.BuildSpec(noCache))
}

// Run builds the runtime image if it is missing, plans the container, and
// runs it.
func (s *Service) Run(ctx context.Context, request Request) error {
	if !s.engine.ImageExists(ctx, runtime.Image) {
		fmt.Fprintf(s.stderr, "Runtime image %s not found; building it first...\n", runtime.Image)
		if err := s.engine.Build(ctx, runtime.BuildSpec(false)); err != nil {
			return err
		}
	}
	spec, err := s.planner.Plan(request)
	if err != nil {
		return err
	}
	return s.engine.Run(ctx, spec)
}
