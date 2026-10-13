package app

import (
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/runtime"
)

// fakeEngine records what the service asks the container runtime to do.
type fakeEngine struct {
	exists   bool
	builds   []plan.BuildSpec
	buildErr error
	ran      *plan.RunSpec
	runErr   error
}

func (f *fakeEngine) ImageExists(context.Context, string) bool { return f.exists }

func (f *fakeEngine) Build(_ context.Context, spec plan.BuildSpec) error {
	f.builds = append(f.builds, spec)
	return f.buildErr
}

func (f *fakeEngine) Run(_ context.Context, spec plan.RunSpec) error {
	f.ran = &spec
	return f.runErr
}

// newTestService wires a Service whose planner sees no environment at all;
// requests must use NoMount and NoConfig.
func newTestService(engine *fakeEngine, stderr io.Writer) *Service {
	planner := NewPlanner(&fakeEnv{homeErr: errors.New("no home"), wdErr: errors.New("no cwd")}, &fakeProject{}, &fakeConfigs{}, stderr)
	return New(engine, planner, stderr)
}

func TestServicePlansAgentPromptAndDelegates(t *testing.T) {
	engine := &fakeEngine{exists: true}
	service := newTestService(engine, io.Discard)
	agent, err := agent.Builtin().Lookup("codex")
	if err != nil {
		t.Fatal(err)
	}
	prompt := "fix $(this) verbatim"
	if err := service.Run(context.Background(), Request{Agent: &agent, Prompt: &prompt, NoMount: true, NoConfig: true}); err != nil {
		t.Fatal(err)
	}
	want := []string{"codex", "exec", prompt}
	if engine.ran == nil || !reflect.DeepEqual(engine.ran.Command, want) {
		t.Fatalf("command = %+v, want %q", engine.ran, want)
	}
	if len(engine.builds) != 0 {
		t.Fatalf("Build called %d times, want 0", len(engine.builds))
	}
}

func TestServiceBuildsMissingImageBeforeRun(t *testing.T) {
	engine := &fakeEngine{}
	var stderr strings.Builder
	service := newTestService(engine, &stderr)
	if err := service.Run(context.Background(), Request{NoMount: true, NoConfig: true}); err != nil {
		t.Fatal(err)
	}
	if len(engine.builds) != 1 || !reflect.DeepEqual(engine.builds[0], runtime.BuildSpec(false)) {
		t.Fatalf("builds = %+v, want one cached runtime image build", engine.builds)
	}
	if want := "Runtime image caged-runtime:latest not found; building it first...\n"; stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
	if engine.ran == nil {
		t.Fatal("container did not run after the image was built")
	}
}

func TestServiceReturnsBuildFailureWithoutRunning(t *testing.T) {
	engine := &fakeEngine{buildErr: errors.New("build failed")}
	service := newTestService(engine, io.Discard)
	if err := service.Run(context.Background(), Request{NoMount: true, NoConfig: true}); err == nil {
		t.Fatal("Run() error = nil, want build failure")
	}
	if engine.ran != nil {
		t.Fatal("runner called after image build failed")
	}
}

func TestServiceReturnsPlanFailureWithoutRunning(t *testing.T) {
	engine := &fakeEngine{exists: true}
	prompt := "hello"
	err := newTestService(engine, io.Discard).Run(context.Background(), Request{Prompt: &prompt, NoMount: true, NoConfig: true})
	if err == nil || err.Error() != "a prompt requires an agent" || engine.ran != nil {
		t.Fatalf("Run() error = %v, ran = %+v", err, engine.ran)
	}
}

func TestServiceReturnsRunErrorUnchanged(t *testing.T) {
	runErr := errors.New("exit status 3")
	engine := &fakeEngine{exists: true, runErr: runErr}
	if err := newTestService(engine, io.Discard).Run(context.Background(), Request{NoMount: true, NoConfig: true}); err != runErr {
		t.Fatalf("Run() error = %v, want %v", err, runErr)
	}
}

func TestServiceBuildHonorsNoCache(t *testing.T) {
	for _, noCache := range []bool{false, true} {
		engine := &fakeEngine{}
		if err := newTestService(engine, io.Discard).Build(context.Background(), noCache); err != nil {
			t.Fatal(err)
		}
		if len(engine.builds) != 1 || !reflect.DeepEqual(engine.builds[0], runtime.BuildSpec(noCache)) {
			t.Fatalf("Build(%v) sent %+v", noCache, engine.builds)
		}
	}
}
