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
	builds []app.BuildOptions
	built  []app.BuiltImage // what Build returns
	err    error

	doctorOpts []app.DoctorOptions
	report     app.DoctorReport // what Diagnose returns
	statuses   []app.AgentStatus
	cleanOpts  []app.CleanOptions
	cleaned    []app.CleanResult
}

func (f *fakeService) Run(_ context.Context, request app.Request) error {
	f.runs = append(f.runs, request)
	return f.err
}

func (f *fakeService) Build(_ context.Context, opts app.BuildOptions) ([]app.BuiltImage, error) {
	f.builds = append(f.builds, opts)
	if f.err != nil {
		return nil, f.err
	}
	return f.built, nil
}

func (f *fakeService) Diagnose(_ context.Context, opts app.DoctorOptions) app.DoctorReport {
	f.doctorOpts = append(f.doctorOpts, opts)
	return f.report
}

func (f *fakeService) List() ([]app.AgentStatus, error) { return f.statuses, f.err }

func (f *fakeService) Clean(_ context.Context, opts app.CleanOptions) ([]app.CleanResult, error) {
	f.cleanOpts = append(f.cleanOpts, opts)
	return f.cleaned, f.err
}

func (f *fakeService) services() Services {
	return Services{Run: f, Build: f, Doctor: f, List: f, Clean: f}
}

// execute runs the root command with args and returns its stdout.
func execute(t *testing.T, service *fakeService, args ...string) (string, error) {
	t.Helper()
	cmd := NewRootCmd("test", agent.Builtin(), service.services())
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
	_, err := execute(t, service, "--no-config", "--verbose", "--allow-broad-mount", "-w", "/work", "--env", "A", "--env", "B", "-p", "3000", "--publish", "8080:4000", "pi", "hi")
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
	want := app.Request{Agent: &pi, Prompt: &prompt, NoConfig: true, Verbose: true, AllowBroadMount: true, WorkDir: "/work", EnvVars: []string{"A", "B"}, Ports: []string{"3000", "8080:4000"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("request = %+v, want %+v", got, want)
	}
}

func TestAgentNamesDoNotCollideWithSubcommands(t *testing.T) {
	t.Parallel()

	// "shared" is not a subcommand: it is the owner name of shared config
	// mounts (hostconfig.SharedOwner), so no agent may use it either.
	subcommands := map[string]struct{}{"build": {}, "list": {}, "doctor": {}, "clean": {}, "help": {}, "completion": {}, "shared": {}}
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

	service := &fakeService{built: []app.BuiltImage{{Role: "runtime", Ref: "caged-runtime:latest"}}}
	stdout, err := execute(t, service, "build", "--no-cache")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(service.builds, []app.BuildOptions{{NoCache: true}}) || stdout != "built runtime image caged-runtime:latest\n" {
		t.Fatalf("builds = %v, stdout = %q", service.builds, stdout)
	}

	both := &fakeService{built: []app.BuiltImage{
		{Role: "runtime", Ref: "caged-runtime:latest"},
		{Role: "extension", Ref: "caged-runtime:ext", Summary: "apt: neovim, tmux"},
	}}
	wantBoth := "built runtime image caged-runtime:latest\nbuilt extension image caged-runtime:ext (apt: neovim, tmux)\n"
	if stdout, err := execute(t, both, "build"); err != nil || stdout != wantBoth {
		t.Fatalf("build with an extension: stdout = %q, err = %v; want %q", stdout, err, wantBoth)
	}

	failing := &fakeService{err: errors.New("build failed")}
	if stdout, err := execute(t, failing, "build"); err == nil || err.Error() != "build failed" || stdout != "" {
		t.Fatalf("failed build: stdout = %q, err = %v", stdout, err)
	}
}

func TestPassthroughArguments(t *testing.T) {
	t.Parallel()

	const noAgent = "arguments after -- are passed to an agent; name one first, e.g. caged claude -- --resume"
	tests := []struct {
		name       string
		args       []string
		wantAgent  string
		wantPrompt string
		wantArgs   []string
		wantErr    string
		wantEnv    []string
		wantNoMnt  bool
	}{
		{name: "flags after an agent", args: []string{"claude", "--", "--resume"}, wantAgent: "claude", wantArgs: []string{"--resume"}},
		{name: "prompt and flags", args: []string{"claude", "fix tests", "--", "--model", "opus", "--add-dir", "../lib"}, wantAgent: "claude", wantPrompt: "fix tests", wantArgs: []string{"--model", "opus", "--add-dir", "../lib"}},
		{name: "an agent with no prompt style", args: []string{"gh", "--", "pr", "list"}, wantAgent: "gh", wantArgs: []string{"pr", "list"}},
		{name: "nothing after the dash and no agent opens the shell", args: []string{"--"}},
		{name: "nothing after the dash keeps the agent", args: []string{"claude", "--"}, wantAgent: "claude"},
		{name: "nothing after the dash keeps the prompt", args: []string{"claude", "hi", "--"}, wantAgent: "claude", wantPrompt: "hi"},
		{name: "no agent is an error", args: []string{"--", "--resume"}, wantErr: noAgent},
		{name: "a subcommand name after the dash is an error", args: []string{"--", "build"}, wantErr: noAgent},
		{name: "too many arguments before the dash", args: []string{"claude", "a", "b", "--", "x"}, wantErr: "accepts at most 2 arg(s), received 3"},
		{name: "too many arguments with no dash", args: []string{"claude", "a", "b"}, wantErr: "accepts at most 2 arg(s), received 3"},
		{name: "caged flags after the dash belong to the agent", args: []string{"claude", "--", "--verbose", "-p", "x", "--no-mount"}, wantAgent: "claude", wantArgs: []string{"--verbose", "-p", "x", "--no-mount"}},
		{name: "caged flags before the agent are still caged's", args: []string{"--no-mount", "--env", "A", "claude", "--", "--verbose"}, wantAgent: "claude", wantArgs: []string{"--verbose"}, wantEnv: []string{"A"}, wantNoMnt: true},
		{name: "a second dash is just an argument", args: []string{"claude", "--", "a", "--", "b"}, wantAgent: "claude", wantArgs: []string{"a", "--", "b"}},
		{name: "shell syntax is kept as is", args: []string{"claude", "--", "$(id)", `"q"`}, wantAgent: "claude", wantArgs: []string{"$(id)", `"q"`}},
		{name: "an unsupported agent is still reported", args: []string{"foo", "--", "x"}, wantErr: `unsupported agent "foo"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeService{}
			_, err := execute(t, service, tt.args...)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Execute(%q) error = %v, want %q", tt.args, err, tt.wantErr)
				}
				if len(service.runs) != 0 {
					t.Fatalf("service ran despite the error: %+v", service.runs)
				}
				return
			}
			if err != nil {
				t.Fatalf("Execute(%q) error = %v", tt.args, err)
			}
			if len(service.runs) != 1 {
				t.Fatalf("runs = %+v, want one", service.runs)
			}
			got := service.runs[0]
			if (got.Agent == nil) != (tt.wantAgent == "") || (got.Agent != nil && got.Agent.Name != tt.wantAgent) {
				t.Errorf("agent = %+v, want %q", got.Agent, tt.wantAgent)
			}
			if (got.Prompt == nil) != (tt.wantPrompt == "") || (got.Prompt != nil && *got.Prompt != tt.wantPrompt) {
				t.Errorf("prompt = %v, want %q", got.Prompt, tt.wantPrompt)
			}
			if !reflect.DeepEqual(got.AgentArgs, tt.wantArgs) {
				t.Errorf("AgentArgs = %q, want %q", got.AgentArgs, tt.wantArgs)
			}
			if !reflect.DeepEqual(got.EnvVars, tt.wantEnv) || got.NoMount != tt.wantNoMnt {
				t.Errorf("flags = env %q, no-mount %v; want env %q, no-mount %v", got.EnvVars, got.NoMount, tt.wantEnv, tt.wantNoMnt)
			}
		})
	}
}

func TestUnknownFlagHintPointsToTheDash(t *testing.T) {
	t.Parallel()

	const hint = " (caged's flags go before the agent name; pass agent flags after --, e.g. caged claude -- --resume)"
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "unknown long flag", args: []string{"claude", "--resume"}, want: "unknown flag: --resume" + hint},
		{name: "unknown short flag", args: []string{"-x", "claude"}, want: "unknown shorthand flag: 'x' in -x" + hint},
		{name: "a subcommand's unknown flag gets the hint too", args: []string{"build", "--bogus"}, want: "unknown flag: --bogus" + hint},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			service := &fakeService{}
			_, err := execute(t, service, tt.args...)
			if err == nil || err.Error() != tt.want {
				t.Fatalf("Execute(%q) error = %v, want %q", tt.args, err, tt.want)
			}
		})
	}
	t.Run("other flag errors are untouched", func(t *testing.T) {
		t.Parallel()
		_, err := execute(t, &fakeService{}, "--workdir")
		if err == nil || strings.Contains(err.Error(), "agent flags") {
			t.Fatalf("Execute() error = %v, want cobra's own message", err)
		}
	})
}

func TestCompletionStopsAfterTheDash(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "nothing after an agent", args: []string{"__complete", "claude", ""}, want: ":4\n"},
		{name: "nothing after the dash", args: []string{"__complete", "claude", "--", ""}, want: ":4\n"},
		{name: "nothing after the prompt and the dash", args: []string{"__complete", "claude", "hi", "--", ""}, want: ":4\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			stdout, err := execute(t, &fakeService{}, tt.args...)
			if err != nil {
				t.Fatal(err)
			}
			if stdout != tt.want {
				t.Fatalf("completion output = %q, want %q", stdout, tt.want)
			}
		})
	}
	t.Run("agents are still suggested before the dash", func(t *testing.T) {
		t.Parallel()
		stdout, err := execute(t, &fakeService{}, "__complete", "")
		if err != nil || !strings.Contains(stdout, "claude") {
			t.Fatalf("completion output = %q, %v; want agent names", stdout, err)
		}
	})
}
