// Package app holds caged's use cases: run an agent or shell in a container,
// and build the runtime image.
package app

import (
	"context"
	"io"

	"github.com/JeremyOrr4/caged/internal/images"
)

// Runner is the run use case.
type Runner struct {
	exts    Extensions
	planner RunPlanner
	daemon  Daemon
	ensurer *ImageEnsurer
	run     ContainerRunner
	stderr  io.Writer
}

// NewRunner returns a Runner that reads the image extension from exts, plans
// runs with planner, checks the engine with daemon, makes sure the images
// exist through ensurer, starts containers with run, and reports progress on
// stderr.
func NewRunner(exts Extensions, planner RunPlanner, daemon Daemon, ensurer *ImageEnsurer, run ContainerRunner, stderr io.Writer) *Runner {
	return &Runner{exts: exts, planner: planner, daemon: daemon, ensurer: ensurer, run: run, stderr: stderr}
}

// Run reads the image extension, plans the container, checks the engine,
// builds the images if they are missing or out of date, and runs the
// container. The extension file and the plan come first, so a bad file or a
// bad request fails before any docker call.
func (r *Runner) Run(ctx context.Context, req Request) error {
	rep := NewReporter(r.stderr, req.Verbose)
	ext, err := r.exts.Load()
	if err != nil {
		return err
	}
	spec, err := r.planner.Plan(req, images.RefFor(ext), rep)
	if err != nil {
		return err
	}
	if _, err := r.daemon.Daemon(ctx); err != nil {
		return err
	}
	if ext == nil {
		if source, err := r.exts.Source(); err == nil {
			rep.Detail("image: no extension (%s not found)", source)
		}
	}
	if _, err := r.ensurer.Ensure(ctx, ext, rep); err != nil {
		return err
	}
	return r.run.RunContainer(ctx, spec)
}
