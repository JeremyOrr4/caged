package app

import (
	"fmt"
	"io"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/runtime"
)

// Env is the process environment the planner reads. system.Env implements it.
type Env interface {
	LookupEnv(key string) (string, bool)
	UserHomeDir() (string, error)
	Getwd() (string, error)
}

// ProjectResolver validates the project directory and returns its bind
// mount. project.Resolver implements it.
type ProjectResolver interface {
	Resolve(workDir, home string, allowBroadMount bool) (*plan.Mount, error)
}

// ConfigResolver finds the host config that can be mounted.
// hostconfig.Resolver implements it.
type ConfigResolver interface {
	Resolve(home string) plan.ConfigReport
}

// Planner turns a Request into a complete plan.RunSpec.
type Planner struct {
	env     Env
	project ProjectResolver
	configs ConfigResolver
	stderr  io.Writer
}

// NewPlanner returns a Planner that writes warnings and --verbose details to stderr.
func NewPlanner(env Env, project ProjectResolver, configs ConfigResolver, stderr io.Writer) *Planner {
	return &Planner{env: env, project: project, configs: configs, stderr: stderr}
}

// Plan resolves the project mount, the config mounts, and the environment
// names for request, and combines them into a RunSpec.
func (p *Planner) Plan(request Request) (plan.RunSpec, error) {
	if !request.NoConfig {
		fmt.Fprintln(p.stderr, "WARNING: allowlisted agent credentials are available inside the container.")
	}
	mount, err := p.projectMount(request)
	if err != nil {
		return plan.RunSpec{}, err
	}
	configMounts, err := p.configMounts(request)
	if err != nil {
		return plan.RunSpec{}, err
	}
	var selected agent.Agent
	if request.Agent != nil {
		selected = *request.Agent
	}
	envVars := selected.ResolveEnv(request.EnvVars, p.isSet)
	if request.Verbose {
		p.logEnvResolution(selected, request.EnvVars, envVars)
	}
	return plan.BuildRunSpec(runtime.Image, request.Agent, request.Prompt, mount, configMounts, envVars)
}

// projectMount returns the project bind mount, or nil with --no-mount.
func (p *Planner) projectMount(request Request) (*plan.Mount, error) {
	if request.NoMount && request.WorkDir != "" {
		return nil, fmt.Errorf("--workdir cannot be used with --no-mount")
	}
	if request.NoMount {
		return nil, nil
	}
	workDir := request.WorkDir
	if workDir == "" {
		var err error
		workDir, err = p.env.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get current working directory: %w", err)
		}
	}
	home, err := p.env.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("find home directory: %w", err)
	}
	return p.project.Resolve(workDir, home, request.AllowBroadMount)
}

// configMounts returns the host config mounts, or none with --no-config.
func (p *Planner) configMounts(request Request) ([]plan.ConfigMount, error) {
	if request.NoConfig {
		return nil, nil
	}
	home, err := p.env.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("find home directory: %w", err)
	}
	report := p.configs.Resolve(home)
	if request.Verbose {
		p.logConfigReport(report)
	}
	return report.Mounts, nil
}

func (p *Planner) isSet(name string) bool {
	_, ok := p.env.LookupEnv(name)
	return ok
}

func (p *Planner) logConfigReport(report plan.ConfigReport) {
	for _, mount := range report.Mounts {
		fmt.Fprintf(p.stderr, "config: pass %s -> %s\n", mount.HostPath, mount.DestPath)
	}
	for _, path := range report.Missing {
		fmt.Fprintf(p.stderr, "config: missing %s\n", path)
	}
	for _, path := range report.Refused {
		fmt.Fprintf(p.stderr, "config: refused %s\n", path)
	}
}

func (p *Planner) logEnvResolution(selected agent.Agent, requested, resolved []string) {
	passed := make(map[string]struct{}, len(resolved))
	for _, name := range resolved {
		passed[name] = struct{}{}
	}
	for _, name := range selected.EnvNames(requested) {
		if _, ok := passed[name]; ok {
			fmt.Fprintf(p.stderr, "env: pass %s\n", name)
		} else {
			fmt.Fprintf(p.stderr, "env: skipped %s (unset or disallowed)\n", name)
		}
	}
}
