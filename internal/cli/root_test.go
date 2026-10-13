package cli

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/app"
)

// fakeService records the requests the CLI produces.
type fakeService struct {
	runs   []app.Request
	builds []bool
	err    error
}

func (f *fakeService) Run(_ context.Context, request app.Request) error {
	f.runs = append(f.runs, request)
	return f.err
}

func (f *fakeService) Build(_ context.Context, noCache bool) error {
	f.builds = append(f.builds, noCache)
	return f.err
}

// execute runs the root command with args and returns its stdout.
func execute(t *testing.T, service *fakeService, args ...string) (string, error) {
	t.Helper()
	cmd := NewRootCmd("test", agent.Builtin(), service)
	var stdout bytes.Buffer
	cmd.SetArgs(args)
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	return stdout.String(), err
}

func TestRootCmdRejectsTooManyArguments(t *testing.T) {
	t.Parallel()

	service := &fakeService{}
	_, err := execute(t, service, "claude", "prompt one", "extra")
	if err == nil {
		t.Fatal("Execute() error = nil, want an error for too many arguments")
	}
	if !strings.Contains(err.Error(), "accepts at most 2 arg") {
		t.Errorf("Execute() error = %q, want a too-many-args error", err)
	}
	if len(service.runs) != 0 {
		t.Fatal("service ran despite invalid arguments")
	}
}

func TestBuildRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		flags      app.Request
		args       []string
		wantAgent  string
		wantPrompt string
		wantErr    string
	}{
		{name: "shell", args: nil},
		{name: "agent", args: []string{"claude"}, wantAgent: "claude"},
		{name: "agent and prompt", args: []string{"codex", "fix it"}, wantAgent: "codex", wantPrompt: "fix it"},
		{name: "unsupported", args: []string{"foo"}, wantErr: "unsupported agent \"foo\""},
		{name: "invalid env", flags: app.Request{EnvVars: []string{"bad-name"}}, args: []string{"claude"}, wantErr: `invalid env name "bad-name"`},
		{name: "unsupported before invalid env", flags: app.Request{EnvVars: []string{"bad-name"}}, args: []string{"foo"}, wantErr: "unsupported agent"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			request, err := buildRequest(agent.Builtin(), tt.flags, tt.args)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("buildRequest(%v) error = %v, want %q", tt.args, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("buildRequest(%v) error = %v", tt.args, err)
			}
			if (request.Agent == nil) != (tt.wantAgent == "") || (request.Agent != nil && request.Agent.Name != tt.wantAgent) {
				t.Fatalf("buildRequest(%v) agent = %+v, want %q", tt.args, request.Agent, tt.wantAgent)
			}
			if (request.Prompt == nil) != (tt.wantPrompt == "") || (request.Prompt != nil && *request.Prompt != tt.wantPrompt) {
				t.Fatalf("buildRequest(%v) prompt = %v, want %q", tt.args, request.Prompt, tt.wantPrompt)
			}
		})
	}
}

func TestRootCmdPassesFlagsToService(t *testing.T) {
	t.Parallel()

	service := &fakeService{}
	_, err := execute(t, service, "--no-config", "--verbose", "--allow-broad-mount", "-w", "/work", "--env", "A", "--env", "B", "pi", "hi")
	if err != nil {
		t.Fatal(err)
	}
	if len(service.runs) != 1 {
		t.Fatalf("runs = %+v, want one", service.runs)
	}
	got := service.runs[0]
	pi, err := agent.Builtin().Lookup("pi")
	if err != nil {
		t.Fatal(err)
	}
	prompt := "hi"
	want := app.Request{Agent: &pi, Prompt: &prompt, NoConfig: true, Verbose: true, AllowBroadMount: true, WorkDir: "/work", EnvVars: []string{"A", "B"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("request = %+v, want %+v", got, want)
	}
}

func TestAgentNamesDoNotCollideWithSubcommands(t *testing.T) {
	t.Parallel()

	subcommands := map[string]struct{}{"build": {}, "list": {}, "help": {}, "completion": {}}
	for _, name := range agent.Builtin().Names() {
		if _, ok := subcommands[name]; ok {
			t.Errorf("agent %q collides with a root subcommand", name)
		}
	}
}

func TestInvalidEnvFlagRejected(t *testing.T) {
	t.Parallel()

	service := &fakeService{}
	_, err := execute(t, service, "claude", "--env", "bad-name")
	if err == nil || !strings.Contains(err.Error(), "invalid env name") {
		t.Fatalf("Execute() error = %v, want invalid env name rejection", err)
	}
	if len(service.runs) != 0 {
		t.Fatal("service ran despite an invalid env name")
	}
}

func TestBuildCommand(t *testing.T) {
	t.Parallel()

	service := &fakeService{}
	stdout, err := execute(t, service, "build", "--no-cache")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(service.builds, []bool{true}) || stdout != "built runtime image caged-runtime:latest\n" {
		t.Fatalf("builds = %v, stdout = %q", service.builds, stdout)
	}

	failing := &fakeService{err: errors.New("build failed")}
	if stdout, err := execute(t, failing, "build"); err == nil || err.Error() != "build failed" || stdout != "" {
		t.Fatalf("failed build: stdout = %q, err = %v", stdout, err)
	}
}

func TestListCommandUsesInjectedAgents(t *testing.T) {
	t.Parallel()

	agents, err := agent.NewRegistry(
		agent.Agent{Name: "zeta", Command: []string{"zeta"}},
		agent.Agent{Name: "alpha", Command: []string{"alpha"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	cmd := NewRootCmd("test", agents, &fakeService{})
	var stdout bytes.Buffer
	cmd.SetArgs([]string{"list"})
	cmd.SetOut(&stdout)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "alpha\nzeta\n" {
		t.Fatalf("list output = %q", stdout.String())
	}
}
