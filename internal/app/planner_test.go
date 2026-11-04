package app

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/testkit"
)

// fakeProject records the arguments it was asked to resolve.
type fakeProject struct {
	gotWorkDir, gotHome string
	gotAllow            bool
	err                 error
}

func (f *fakeProject) Resolve(workDir, home string, allowBroadMount bool) (*plan.Mount, error) {
	f.gotWorkDir, f.gotHome, f.gotAllow = workDir, home, allowBroadMount
	if f.err != nil {
		return nil, f.err
	}
	return &plan.Mount{HostPath: workDir, ContainerPath: "/workspace/app"}, nil
}

// fakeConfigs returns a fixed report.
type fakeConfigs struct {
	report  plan.ConfigReport
	gotHome string
	err     error
}

func (f *fakeConfigs) Resolve(home string) (plan.ConfigReport, error) {
	f.gotHome = home
	return f.report, f.err
}

func lookup(t *testing.T, name string) *agent.Agent {
	t.Helper()
	definition, err := agent.Builtin().Lookup(name)
	if err != nil {
		t.Fatal(err)
	}
	return &definition
}

func TestPlannerCombinesMountsConfigsAndEnv(t *testing.T) {
	t.Parallel()

	env := &testkit.Env{Vars: map[string]string{"ANTHROPIC_API_KEY": "secret", "EXTRA": ""}, Home: "/Users/dev", WorkDir: "/Users/dev/app"}
	project := &fakeProject{}
	configs := &fakeConfigs{report: plan.ConfigReport{
		Mounts:  []plan.ConfigMount{{Agent: "claude", Name: ".claude", HostPath: "/Users/dev/.claude", DestPath: "/home/caged/.claude", Writable: true}},
		Missing: []string{"/Users/dev/.codex"},
		Refused: []plan.Refusal{{Path: "/Users/dev/.pi", Reason: "resolves outside home"}},
	}}
	var stderr strings.Builder
	prompt := "hello"
	spec, err := NewPlanner(env, project, configs).Plan(Request{
		Agent: lookup(t, "claude"), Prompt: &prompt, Verbose: true, AllowBroadMount: true, EnvVars: []string{"EXTRA", "MISSING"},
	}, "caged-runtime:latest", NewReporter(&stderr, true))
	if err != nil {
		t.Fatal(err)
	}
	want := plan.RunSpec{
		Image:         "caged-runtime:latest",
		Command:       []string{"claude", "hello"},
		Mount:         &plan.Mount{HostPath: "/Users/dev/app", ContainerPath: "/workspace/app"},
		ConfigMounts:  configs.report.Mounts,
		EnvVars:       []string{"ANTHROPIC_API_KEY", "EXTRA"},
		ReturnToShell: true,
	}
	if !reflect.DeepEqual(spec, want) {
		t.Fatalf("Plan() = %+v, want %+v", spec, want)
	}
	if project.gotWorkDir != "/Users/dev/app" || project.gotHome != "/Users/dev" || !project.gotAllow || configs.gotHome != "/Users/dev" {
		t.Fatalf("resolvers got project %+v, configs home %q", project, configs.gotHome)
	}
	wantLog := `WARNING: allowlisted agent credentials are available inside the container.
config: pass /Users/dev/.claude -> /home/caged/.claude
config: missing /Users/dev/.codex
config: refused /Users/dev/.pi (resolves outside home)
env: pass ANTHROPIC_API_KEY
env: pass EXTRA
env: skipped MISSING (unset or disallowed)
command: ["claude" "hello"]
`
	if stderr.String() != wantLog {
		t.Fatalf("stderr = %q, want %q", stderr.String(), wantLog)
	}
}

func TestPlannerReturnsTheConfigResolverError(t *testing.T) {
	t.Parallel()

	resolveErr := errors.New("~/.config/caged/config.yaml: line 1: unknown field \"imgae\"")
	env := &testkit.Env{Home: "/Users/dev", WorkDir: "/Users/dev/app"}
	_, err := NewPlanner(env, &fakeProject{}, &fakeConfigs{err: resolveErr}).Plan(Request{}, "img", NewReporter(io.Discard, false))
	if !errors.Is(err, resolveErr) {
		t.Fatalf("Plan() error = %v, want %v", err, resolveErr)
	}
}

func TestPlannerLogsFollowedLinksAndRefusalHints(t *testing.T) {
	t.Parallel()

	env := &testkit.Env{Home: "/Users/dev", WorkDir: "/Users/dev/app"}
	configs := &fakeConfigs{report: plan.ConfigReport{
		Mounts: []plan.ConfigMount{{Agent: "claude", Name: ".claude", HostPath: "/Users/dev/dot/CLAUDE.md", DestPath: "/home/caged/.claude/CLAUDE.md", Link: "/Users/dev/.claude/CLAUDE.md"}},
		Refused: []plan.Refusal{{
			Path: "/Users/dev/.claude/agents", Reason: "symlink target ~/dot2/agents is not under links.allow",
			Hint: `add "- path: ~/dot2/agents" under links.allow in ~/.config/caged/config.yaml`,
		}},
	}}
	var stderr strings.Builder
	if _, err := NewPlanner(env, &fakeProject{}, configs).Plan(Request{NoMount: true}, "img", NewReporter(&stderr, true)); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"config: pass /Users/dev/dot/CLAUDE.md -> /home/caged/.claude/CLAUDE.md (via link /Users/dev/.claude/CLAUDE.md)\n",
		"config: refused /Users/dev/.claude/agents (symlink target ~/dot2/agents is not under links.allow); add \"- path: ~/dot2/agents\" under links.allow in ~/.config/caged/config.yaml\n",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr = %q, want it to contain %q", stderr.String(), want)
		}
	}
}

func TestPlannerUsesWorkDirFlagWithoutReadingCwd(t *testing.T) {
	t.Parallel()

	project := &fakeProject{}
	env := &testkit.Env{Home: "/home/dev", WorkDirErr: errors.New("cwd must not be read")}
	if _, err := NewPlanner(env, project, &fakeConfigs{}).Plan(Request{WorkDir: "../other", NoConfig: true}, "img", NewReporter(io.Discard, false)); err != nil {
		t.Fatal(err)
	}
	if project.gotWorkDir != "../other" {
		t.Fatalf("project resolver got %q, want the --workdir value", project.gotWorkDir)
	}
}

func TestNoMountAndWorkdirAreIncompatible(t *testing.T) {
	t.Parallel()

	request := Request{NoMount: true, WorkDir: "/tmp"}
	_, err := NewPlanner(&testkit.Env{}, &fakeProject{}, &fakeConfigs{}).Plan(request, "img", NewReporter(io.Discard, false))
	if err == nil || !strings.Contains(err.Error(), "cannot be used with --no-mount") {
		t.Fatalf("Plan() error = %v, want incompatible-flags error", err)
	}
}

func TestPlannerNoMountNoConfigNeedsNoHomeOrCwd(t *testing.T) {
	t.Parallel()

	env := &testkit.Env{HomeErr: errors.New("no home"), WorkDirErr: errors.New("no cwd")}
	var stderr strings.Builder
	spec, err := NewPlanner(env, &fakeProject{}, &fakeConfigs{}).Plan(Request{NoMount: true, NoConfig: true}, "img", NewReporter(&stderr, false))
	if err != nil {
		t.Fatal(err)
	}
	if spec.Mount != nil || spec.ConfigMounts != nil || !reflect.DeepEqual(spec.Command, []string{"bash"}) || spec.ReturnToShell {
		t.Fatalf("Plan() = %+v, want a bare shell", spec)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want no credentials warning with --no-config", stderr.String())
	}
}

func TestPlannerErrors(t *testing.T) {
	t.Parallel()

	prompt := "hello"
	tests := []struct {
		name    string
		env     *testkit.Env
		project *fakeProject
		request Request
		want    string
	}{
		{name: "cwd", env: &testkit.Env{WorkDirErr: errors.New("gone")}, request: Request{NoConfig: true}, want: "get current working directory: gone"},
		{name: "home for project", env: &testkit.Env{WorkDir: "/w", HomeErr: errors.New("unset")}, request: Request{NoConfig: true}, want: "find home directory: unset"},
		{name: "home for config", env: &testkit.Env{HomeErr: errors.New("unset")}, request: Request{NoMount: true}, want: "find home directory: unset"},
		{name: "project", env: &testkit.Env{WorkDir: "/w"}, project: &fakeProject{err: errors.New("refusing")}, request: Request{NoConfig: true}, want: "refusing"},
		{name: "prompt without agent", env: &testkit.Env{}, request: Request{Prompt: &prompt, NoMount: true, NoConfig: true}, want: "a prompt requires an agent"},
		{name: "agent without prompt support", env: &testkit.Env{}, request: Request{Agent: lookup(t, "gh"), Prompt: &prompt, NoMount: true, NoConfig: true}, want: "gh does not support prompt launch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			project := tt.project
			if project == nil {
				project = &fakeProject{}
			}
			if _, err := NewPlanner(tt.env, project, &fakeConfigs{}).Plan(tt.request, "img", NewReporter(io.Discard, false)); err == nil || err.Error() != tt.want {
				t.Fatalf("Plan() error = %v, want %q", err, tt.want)
			}
		})
	}
}

// countingEnv counts home directory lookups.
type countingEnv struct {
	testkit.Env
	homeCalls int
}

func (e *countingEnv) UserHomeDir() (string, error) {
	e.homeCalls++
	return e.Env.UserHomeDir()
}

func TestPlannerResolvesHomeOnce(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		request Request
		want    int
	}{
		{name: "mount and config", request: Request{}, want: 1},
		{name: "mount only", request: Request{NoConfig: true}, want: 1},
		{name: "config only", request: Request{NoMount: true}, want: 1},
		{name: "neither", request: Request{NoMount: true, NoConfig: true}, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			env := &countingEnv{Home: "/home/dev", WorkDir: "/home/dev/app"}
			if _, err := NewPlanner(env, &fakeProject{}, &fakeConfigs{}).Plan(tt.request, "img", NewReporter(io.Discard, false)); err != nil {
				t.Fatal(err)
			}
			if env.homeCalls != tt.want {
				t.Fatalf("UserHomeDir called %d times, want %d", env.homeCalls, tt.want)
			}
		})
	}
}

func TestPlannerPublishedPorts(t *testing.T) {
	t.Parallel()

	t.Run("parsed ports reach the spec and are logged with --verbose", func(t *testing.T) {
		t.Parallel()
		var stderr strings.Builder
		request := Request{NoMount: true, NoConfig: true, Verbose: true, Ports: []string{"3000", "8080:4000"}}
		spec, err := NewPlanner(&testkit.Env{}, &fakeProject{}, &fakeConfigs{}).Plan(request, "img", NewReporter(&stderr, true))
		if err != nil {
			t.Fatal(err)
		}
		want := []plan.Port{{HostIP: "127.0.0.1", HostPort: 3000, ContainerPort: 3000}, {HostIP: "127.0.0.1", HostPort: 8080, ContainerPort: 4000}}
		if !reflect.DeepEqual(spec.Ports, want) {
			t.Fatalf("Ports = %+v, want %+v", spec.Ports, want)
		}
		wantLog := "port: publish 127.0.0.1:3000 -> container 3000\nport: publish 127.0.0.1:8080 -> container 4000\ncommand: [\"bash\"]\n"
		if stderr.String() != wantLog {
			t.Fatalf("stderr = %q, want %q", stderr.String(), wantLog)
		}
	})
	t.Run("port lines need --verbose", func(t *testing.T) {
		t.Parallel()
		var stderr strings.Builder
		request := Request{NoMount: true, NoConfig: true, Ports: []string{"3000"}}
		if _, err := NewPlanner(&testkit.Env{}, &fakeProject{}, &fakeConfigs{}).Plan(request, "img", NewReporter(&stderr, false)); err != nil {
			t.Fatal(err)
		}
		if stderr.Len() != 0 {
			t.Fatalf("stderr = %q, want nothing without --verbose", stderr.String())
		}
	})
	t.Run("a bad port fails before the warning and before any resolver runs", func(t *testing.T) {
		t.Parallel()
		var stderr strings.Builder
		env := &testkit.Env{Home: "/home/dev", WorkDir: "/home/dev/app"}
		project, configs := &fakeProject{}, &fakeConfigs{}
		request := Request{Ports: []string{"70000"}}
		_, err := NewPlanner(env, project, configs).Plan(request, "img", NewReporter(&stderr, true))
		if err == nil || err.Error() != `invalid port "70000": 70000 is out of range 1-65535` {
			t.Fatalf("Plan() error = %v", err)
		}
		if stderr.Len() != 0 {
			t.Fatalf("stderr = %q, want no warning before a port error", stderr.String())
		}
		if project.gotWorkDir != "" || configs.gotHome != "" {
			t.Fatalf("resolvers ran before the port error: project %+v, configs home %q", project, configs.gotHome)
		}
	})
}

func TestPlannerPassesAgentArgsAndLogsTheCommand(t *testing.T) {
	t.Parallel()

	prompt := "fix tests"
	tests := []struct {
		name    string
		request Request
		want    []string
		wantLog string
		wantErr string
	}{
		{name: "agent args after the prompt", request: Request{Agent: lookup(t, "claude"), Prompt: &prompt, AgentArgs: []string{"--model", "opus"}}, want: []string{"claude", "fix tests", "--model", "opus"}, wantLog: "env: skipped ANTHROPIC_API_KEY (unset or disallowed)\n" + `command: ["claude" "fix tests" "--model" "opus"]` + "\n"},
		{name: "the shell is logged too", request: Request{}, want: []string{"bash"}, wantLog: `command: ["bash"]` + "\n"},
		{name: "agent args without an agent", request: Request{AgentArgs: []string{"--resume"}}, wantErr: "agent arguments after -- need an agent"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var stderr strings.Builder
			request := tt.request
			request.NoMount, request.NoConfig, request.Verbose = true, true, true
			spec, err := NewPlanner(&testkit.Env{}, &fakeProject{}, &fakeConfigs{}).Plan(request, "img", NewReporter(&stderr, true))
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("Plan() error = %v, want %q", err, tt.wantErr)
				}
				if strings.Contains(stderr.String(), "command:") {
					t.Fatalf("stderr = %q, want no command line for a failed plan", stderr.String())
				}
				return
			}
			if err != nil || !reflect.DeepEqual(spec.Command, tt.want) {
				t.Fatalf("Command = %q, %v; want %q", spec.Command, err, tt.want)
			}
			if stderr.String() != tt.wantLog {
				t.Fatalf("stderr = %q, want %q", stderr.String(), tt.wantLog)
			}
		})
	}
}
