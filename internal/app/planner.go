package app

import (
	"fmt"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/plan"
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
}

// NewPlanner returns a Planner that reads env and resolves the project and
// config mounts through project and configs.
func NewPlanner(env Env, project ProjectResolver, configs ConfigResolver) *Planner {
	return &Planner{env: env, project: project, configs: configs}
}

// homeLookup resolves the home directory at most once per Plan call, and only
// when a step needs it.
type homeLookup struct {
	env  Env
	dir  string
	err  error
	done bool
}

func (h *homeLookup) get() (string, error) {
	if !h.done {
		h.done = true
		var err error
		if h.dir, err = h.env.UserHomeDir(); err != nil {
			h.err = fmt.Errorf("find home directory: %w", err)
		}
	}
	return h.dir, h.err
}

// Plan resolves the project mount, the config mounts, and the environment
// names for request, and combines them into a RunSpec that runs imageRef.
// Warnings and --verbose details go to rep.
func (p *Planner) Plan(request Request, imageRef string, rep *Reporter) (plan.RunSpec, error) {
	ports, err := plan.ParsePorts(request.Ports)
	if err != nil {
		return plan.RunSpec{}, err
	}
	if !request.NoConfig {
		rep.Notice("WARNING: allowlisted agent credentials are available inside the container.")
	}
	for _, port := range ports {
		rep.Detail("port: publish %s:%d -> container %d", port.HostIP, port.HostPort, port.ContainerPort)
	}
	home := &homeLookup{env: p.env}
	mount, err := p.projectMount(request, home)
	if err != nil {
		return plan.RunSpec{}, err
	}
	configMounts, err := p.configMounts(request, home, rep)
	if err != nil {
		return plan.RunSpec{}, err
	}
	var selected agent.Agent
	if request.Agent != nil {
		selected = *request.Agent
	}
	envVars := selected.ResolveEnv(request.EnvVars, p.isSet)
	logEnvResolution(rep, selected, request.EnvVars, envVars)
	spec, err := buildRunSpec(runInputs{Image: imageRef, Agent: request.Agent, Prompt: request.Prompt, AgentArgs: request.AgentArgs, Mount: mount, Ports: ports, Configs: configMounts, Env: envVars})
	if err != nil {
		return plan.RunSpec{}, err
	}
	rep.Detail("command: %q", spec.Command)
	return spec, nil
}

// projectMount returns the project bind mount, or nil with --no-mount.
func (p *Planner) projectMount(request Request, home *homeLookup) (*plan.Mount, error) {
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
	homeDir, err := home.get()
	if err != nil {
		return nil, err
	}
	return p.project.Resolve(workDir, homeDir, request.AllowBroadMount)
}

// configMounts returns the host config mounts, or none with --no-config.
func (p *Planner) configMounts(request Request, home *homeLookup, rep *Reporter) ([]plan.ConfigMount, error) {
	if request.NoConfig {
		return nil, nil
	}
	homeDir, err := home.get()
	if err != nil {
		return nil, err
	}
	report := p.configs.Resolve(homeDir)
	logConfigReport(rep, report)
	return report.Mounts, nil
}

func (p *Planner) isSet(name string) bool {
	_, ok := p.env.LookupEnv(name)
	return ok
}

func logConfigReport(rep *Reporter, report plan.ConfigReport) {
	for _, mount := range report.Mounts {
		rep.Detail("config: pass %s -> %s", mount.HostPath, mount.DestPath)
	}
	for _, path := range report.Missing {
		rep.Detail("config: missing %s", path)
	}
	for _, refusal := range report.Refused {
		rep.Detail("config: refused %s", refusal)
	}
}

func logEnvResolution(rep *Reporter, selected agent.Agent, requested, resolved []string) {
	passed := make(map[string]struct{}, len(resolved))
	for _, name := range resolved {
		passed[name] = struct{}{}
	}
	for _, name := range selected.EnvNames(requested) {
		if _, ok := passed[name]; ok {
			rep.Detail("env: pass %s", name)
		} else {
			rep.Detail("env: skipped %s (unset or disallowed)", name)
		}
	}
}
