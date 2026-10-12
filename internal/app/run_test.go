package app

import (
	"errors"
	"reflect"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/docker"
)

type fakeRunner struct {
	got docker.Options
	err error
}

func (f *fakeRunner) Run(options docker.Options) error { f.got = options; return f.err }

type fakeRuntime struct {
	exists   bool
	builds   int
	buildErr error
}

func (f *fakeRuntime) ImageExists(string) bool { return f.exists }
func (f *fakeRuntime) Build(bool) error        { f.builds++; return f.buildErr }

func TestServicePlansAgentPromptAndDelegates(t *testing.T) {
	runner := &fakeRunner{}
	runtime := &fakeRuntime{exists: true}
	service := New(runner, runtime)
	agent, err := agent.Lookup("codex")
	if err != nil {
		t.Fatal(err)
	}
	prompt := "fix $(this) verbatim"
	if err := service.Run(Request{Agent: &agent, Prompt: &prompt, NoMount: true, NoConfig: true}); err != nil {
		t.Fatal(err)
	}
	want := []string{"codex", "exec", prompt}
	if !reflect.DeepEqual(runner.got.Command, want) {
		t.Fatalf("command = %q, want %q", runner.got.Command, want)
	}
	if runtime.builds != 0 {
		t.Fatalf("Build called %d times, want 0", runtime.builds)
	}
}

func TestServiceBuildsMissingImageBeforeRun(t *testing.T) {
	runner := &fakeRunner{}
	runtime := &fakeRuntime{}
	service := New(runner, runtime)
	if err := service.Run(Request{NoMount: true, NoConfig: true}); err != nil {
		t.Fatal(err)
	}
	if runtime.builds != 1 {
		t.Fatalf("Build called %d times, want 1", runtime.builds)
	}
}

func TestServiceReturnsBuildFailureWithoutRunning(t *testing.T) {
	runner := &fakeRunner{}
	runtime := &fakeRuntime{buildErr: errors.New("build failed")}
	service := New(runner, runtime)
	if err := service.Run(Request{NoMount: true, NoConfig: true}); err == nil {
		t.Fatal("Run() error = nil, want build failure")
	}
	if runner.got.Image != "" {
		t.Fatal("runner called after image build failed")
	}
}
