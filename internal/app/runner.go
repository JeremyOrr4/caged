// Package app holds caged's use cases: run an agent or shell in a container,
// build, diagnose and list, each behind ConfigGuard's review of config.yaml,
// and clean. It declares the interfaces it needs and imports no adapter.
package app

import (
	"context"
	"io"

	"github.com/JeremyOrr4/caged/internal/images"
)

// RunnerDeps are the collaborators of a Runner.
type RunnerDeps struct {
	Extensions Extensions      // reads the image extension
	Planner    RunPlanner      // plans the container
	Daemon     Daemon          // checks the engine
	Ensurer    *ImageEnsurer   // makes sure the images exist and are current
	Containers ContainerRunner // starts the container
	Stderr     io.Writer       // progress messages
}

// Runner is the run use case.
type Runner struct {
	deps RunnerDeps
}

// NewRunner returns a Runner that works through deps.
func NewRunner(deps RunnerDeps) *Runner {
	return &Runner{deps: deps}
}

// Run reads the image extension, plans the container, checks the engine,
// builds the images if they are missing or out of date, and runs the
// container. The config file and the plan come first, so a bad file or a
// bad request fails before any docker call.
func (r *Runner) Run(ctx context.Context, req Request) error {
	rep := NewReporter(r.deps.Stderr, req.Verbose)
	ext, err := r.deps.Extensions.Load()
	if err != nil {
		return err
	}
	spec, err := r.deps.Planner.Plan(req, images.RefFor(ext), rep)
	if err != nil {
		return err
	}
	if _, err := r.deps.Daemon.Daemon(ctx); err != nil {
		return err
	}
	if ext == nil {
		// best effort: a --verbose line that needs the path; Load already succeeded
		if source, err := r.deps.Extensions.Source(); err == nil {
			rep.detail("image: no image.apt packages in %s", source)
		}
	}
	if err := r.deps.Ensurer.ensureImages(ctx, ext, rep); err != nil {
		return err
	}
	return r.deps.Containers.RunContainer(ctx, spec)
}
