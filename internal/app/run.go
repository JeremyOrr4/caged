// Package app coordinates agent selection, host configuration, and container execution.
package app

import (
	"fmt"
	"os"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/docker"
	"github.com/JeremyOrr4/caged/internal/hostconfig"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/project"
	"github.com/JeremyOrr4/caged/internal/runtime"
)

// Runner and Runtime keep application policy independent of Docker's process API.
type Runner interface{ Run(plan.RunSpec) error }
type Runtime interface {
	ImageExists(string) bool
	Build(bool) error
}

type Service struct {
	runner  Runner
	runtime Runtime
}

type dockerRunner struct{}

func (dockerRunner) Run(spec plan.RunSpec) error { return docker.Run(spec) }

type dockerRuntime struct{}

func (dockerRuntime) ImageExists(image string) bool { return docker.ImageExists(image) }
func (dockerRuntime) Build(noCache bool) error {
	return runtime.BuildRuntimeImage(noCache, docker.CheckDocker)
}

func New(runner Runner, runtime Runtime) *Service { return &Service{runner: runner, runtime: runtime} }

// DefaultService wires the production Docker adapters.
func DefaultService() *Service { return New(dockerRunner{}, dockerRuntime{}) }

// Build refreshes the runtime image through the configured runtime adapter.
func (s *Service) Build(noCache bool) error {
	if s.runtime == nil {
		return fmt.Errorf("application service requires a runtime")
	}
	return s.runtime.Build(noCache)
}

type Request struct {
	Agent                                       *agent.Agent
	Prompt                                      *string
	NoMount, NoConfig, Verbose, AllowBroadMount bool
	WorkDir                                     string
	EnvVars                                     []string
}

// Run validates the request, plans mounts and environment, then delegates execution.
func (s *Service) Run(request Request) error {
	if s.runner == nil || s.runtime == nil {
		return fmt.Errorf("application service requires a runner and runtime")
	}
	if !s.runtime.ImageExists(runtime.Image) {
		fmt.Fprintf(os.Stderr, "Runtime image %s not found; building it first...\n", runtime.Image)
		if err := s.runtime.Build(false); err != nil {
			return err
		}
	}
	for _, name := range request.EnvVars {
		if err := agent.ValidateEnvName(name); err != nil {
			return err
		}
	}
	resolved := plan.RunSpec{}
	if !request.NoConfig {
		fmt.Fprintln(os.Stderr, "WARNING: allowlisted agent credentials are available inside the container.")
	}
	if err := ConfigureMounts(&resolved, request); err != nil {
		return err
	}
	var selected agent.Agent
	if request.Agent != nil {
		selected = *request.Agent
	}
	envVars := agent.ResolveEnvVars(selected, request.EnvVars, func(name string) bool { _, ok := os.LookupEnv(name); return ok })
	if request.Verbose {
		logEnvResolution(selected, request.EnvVars, envVars)
	}
	spec, err := plan.BuildRunSpec(runtime.Image, request.Agent, request.Prompt, resolved.Mount, resolved.ConfigMounts, envVars)
	if err != nil {
		return err
	}
	return s.runner.Run(spec)
}

func ConfigureMounts(options *plan.RunSpec, request Request) error {
	if request.NoMount && request.WorkDir != "" {
		return fmt.Errorf("--workdir cannot be used with --no-mount")
	}
	if !request.NoMount {
		workDir := request.WorkDir
		if workDir == "" {
			var err error
			workDir, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("get current working directory: %w", err)
			}
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("find home directory: %w", err)
		}
		options.Mount, err = project.ResolveMount(workDir, home, request.AllowBroadMount)
		if err != nil {
			return err
		}
	}
	if request.NoConfig {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("find home directory: %w", err)
	}
	report := hostconfig.ResolveAllConfigsReport(home, nil)
	options.ConfigMounts = report.Mounts
	if request.Verbose {
		logConfigReport(report)
	}
	return nil
}

func logConfigReport(report hostconfig.ConfigReport) {
	for _, mount := range report.Mounts {
		fmt.Fprintf(os.Stderr, "config: pass %s -> %s\n", mount.HostPath, mount.DestPath)
	}
	for _, path := range report.Missing {
		fmt.Fprintf(os.Stderr, "config: missing %s\n", path)
	}
	for _, path := range report.Refused {
		fmt.Fprintf(os.Stderr, "config: refused %s\n", path)
	}
}

func logEnvResolution(selected agent.Agent, requested, resolved []string) {
	passed := make(map[string]struct{}, len(resolved))
	for _, name := range resolved {
		passed[name] = struct{}{}
	}
	for _, name := range agent.EnvVarNames(selected, requested) {
		if _, ok := passed[name]; ok {
			fmt.Fprintf(os.Stderr, "env: pass %s\n", name)
		} else {
			fmt.Fprintf(os.Stderr, "env: skipped %s (unset or disallowed)\n", name)
		}
	}
}
