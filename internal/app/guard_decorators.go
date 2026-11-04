package app

import "context"

// The Guarded* types wrap a use case so the config file is prepared first.
// Each one is the same use case as far as the CLI can tell.

// GuardedRunner prepares the config before a run. --no-config skips creating
// the file, because that run uses no host config; the file is still reviewed
// because it still sets image.apt and workdirs.allow.
type GuardedRunner struct {
	Guard *ConfigGuard
	Next  interface {
		Run(ctx context.Context, request Request) error
	}
}

// Run prepares the config, then runs the wrapped use case.
func (g GuardedRunner) Run(ctx context.Context, request Request) error {
	if err := g.Guard.prepare(guardOptions{Create: !request.NoConfig, Trust: request.TrustConfig}); err != nil {
		return err
	}
	return g.Next.Run(ctx, request)
}

// GuardedBuilder prepares the config before a build.
type GuardedBuilder struct {
	Guard *ConfigGuard
	Next  interface {
		Build(ctx context.Context, opts BuildOptions) ([]BuiltImage, error)
	}
}

// Build prepares the config, then builds.
func (g GuardedBuilder) Build(ctx context.Context, opts BuildOptions) ([]BuiltImage, error) {
	if err := g.Guard.prepare(guardOptions{Create: true, Trust: opts.TrustConfig}); err != nil {
		return nil, err
	}
	return g.Next.Build(ctx, opts)
}

// GuardedDoctor shows config changes before the report; it never blocks.
type GuardedDoctor struct {
	Guard *ConfigGuard
	Next  interface {
		Diagnose(ctx context.Context, opts DoctorOptions) DoctorReport
	}
}

// Diagnose reports a pending config change, then diagnoses.
func (g GuardedDoctor) Diagnose(ctx context.Context, opts DoctorOptions) DoctorReport {
	g.Guard.notify()
	return g.Next.Diagnose(ctx, opts)
}

// GuardedLister shows config changes before the list; it never blocks.
type GuardedLister struct {
	Guard *ConfigGuard
	Next  interface{ List() ([]AgentStatus, error) }
}

// List reports a pending config change, then lists.
func (g GuardedLister) List() ([]AgentStatus, error) {
	g.Guard.notify()
	return g.Next.List()
}
