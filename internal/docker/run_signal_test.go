package docker

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/JeremyOrr4/caged/internal/configstage"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
)

// TestRunContainerSkipsWriteBackAfterASignal pins WB-T3 (bugs.md Q-CS-6,
// D-25): when caged was signalled during the run, the container may still be
// running, so its staged copies are removed without being written back, and
// the signal's exit code and a note about the lost changes are returned.
func TestRunContainerSkipsWriteBackAfterASignal(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp) // the staging directory goes here
	root := filepath.Join(t.TempDir(), ".pi")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	const dest = "/home/caged/.pi"
	exec := &fakeExecutor{run: func(cmd system.Command) error {
		for i := 0; i+1 < len(cmd.Args); i++ {
			if source, destination, ok := strings.Cut(cmd.Args[i+1], ":"); ok && cmd.Args[i] == "-v" && destination == dest {
				if source == root {
					return errors.New("the Pi tree was not staged")
				}
				if err := os.WriteFile(filepath.Join(source, "new.json"), []byte("{}"), 0o600); err != nil {
					return err
				}
			}
		}
		return &system.SignalError{Signal: syscall.SIGTERM}
	}}
	spec := plan.RunSpec{Image: "img", ConfigMounts: []plan.ConfigMount{{Agent: "pi", Staged: true, Writable: true, Root: root, HostPath: root, DestPath: dest, MaskFiles: []string{"sock"}}}}
	err := NewClient(Deps{Exec: exec, FS: &fakeFS{}, Stager: configstage.New(system.FS{}), Platform: linux}).RunContainer(context.Background(), spec)

	var coder interface{ ExitCode() int }
	if !errors.As(err, &coder) || coder.ExitCode() != 143 || err.Error() != "received signal: terminated; Pi config changes from this run were not written back" {
		t.Fatalf("RunContainer() error = %v; want the signal with exit code 143 and the note", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "new.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("new.json on the host: %v; want nothing written back after a signal", statErr)
	}
	if entries, err := os.ReadDir(tmp); err != nil || len(entries) != 0 {
		t.Fatalf("temp dir entries = %v, %v; want the staged copies removed", entries, err)
	}
	if len(exec.commands) != 1 || !exec.commands[0].CatchSignals {
		t.Fatalf("commands = %+v; want one docker run that catches signals", exec.commands)
	}
}

// TestRunContainerSkipsWriteBackWhenDockerIsKilled pins WB-B3 (bugs.md
// Q-WB-3): when the docker CLI itself ends by a signal the container may still
// be running and writing to the copies, so they are removed, not written back,
// as after a caught signal, and the exit code is 128 plus the signal number.
func TestRunContainerSkipsWriteBackWhenDockerIsKilled(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp) // the staging directory goes here
	root := filepath.Join(t.TempDir(), ".pi")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	const dest = "/home/caged/.pi"
	exec := &fakeExecutor{run: func(cmd system.Command) error {
		for i := 0; i+1 < len(cmd.Args); i++ {
			if source, destination, ok := strings.Cut(cmd.Args[i+1], ":"); ok && cmd.Args[i] == "-v" && destination == dest && source != root {
				if err := os.WriteFile(filepath.Join(source, "new.json"), []byte("{}"), 0o600); err != nil {
					return err
				}
			}
		}
		return &system.KilledError{Signal: syscall.SIGKILL, Err: errors.New("signal: killed")}
	}}
	spec := plan.RunSpec{Image: "img", ConfigMounts: []plan.ConfigMount{{Agent: "pi", Staged: true, Writable: true, Root: root, HostPath: root, DestPath: dest, MaskFiles: []string{"sock"}}}}
	err := NewClient(Deps{Exec: exec, FS: &fakeFS{}, Stager: configstage.New(system.FS{}), Platform: linux}).RunContainer(context.Background(), spec)

	var coder interface{ ExitCode() int }
	if !errors.As(err, &coder) || coder.ExitCode() != 137 || err.Error() != "signal: killed; Pi config changes from this run were not written back" {
		t.Fatalf("RunContainer() error = %v; want docker's end with exit code 137 and the note", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "new.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("new.json on the host: %v; want nothing written back after docker was killed", statErr)
	}
	if entries, err := os.ReadDir(tmp); err != nil || len(entries) != 0 {
		t.Fatalf("temp dir entries = %v, %v; want the staged copies removed", entries, err)
	}
}

// TestRunContainerWithoutStagedConfigReportsOnlyTheSignal: with nothing
// staged nothing is lost, so the note is left out.
func TestRunContainerWithoutStagedConfigReportsOnlyTheSignal(t *testing.T) {
	t.Parallel()
	exec := &fakeExecutor{run: func(system.Command) error { return &system.SignalError{Signal: syscall.SIGHUP} }}
	err := newClient(exec, &fakeFS{}, linux, system.Stdio{}).RunContainer(context.Background(), plan.RunSpec{Image: "img"})
	if err == nil || err.Error() != "received signal: hangup" {
		t.Fatalf("RunContainer() error = %v; want only the signal", err)
	}
}

// TestOnlyTheContainerRunCatchesSignals: builds, probes and every other
// docker call keep Go's default signal handling (D-25).
func TestOnlyTheContainerRunCatchesSignals(t *testing.T) {
	t.Parallel()
	exec := &fakeExecutor{}
	client := newClient(exec, &fakeFS{}, linux, system.Stdio{})
	if _, err := client.BuildImage(context.Background(), testBuildSpec(false)); err != nil {
		t.Fatal(err)
	}
	if err := client.ProbeMount(context.Background(), "img", plan.Mount{HostPath: "/p", ContainerPath: "/workspace/p"}); err != nil {
		t.Fatal(err)
	}
	if err := client.RunContainer(context.Background(), plan.RunSpec{Image: "img"}); err != nil {
		t.Fatal(err)
	}
	for i, cmd := range exec.commands {
		if want := i == len(exec.commands)-1; cmd.CatchSignals != want {
			t.Errorf("command %v: CatchSignals = %v, want %v", cmd.Args, cmd.CatchSignals, want)
		}
	}
}

// TestRunContainerNotesLostChangesOnlyForCopies: a staged mount with nothing
// to hide runs from the host tree itself, so its changes are already on the
// host and a signal loses nothing.
func TestRunContainerNotesLostChangesOnlyForCopies(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".pi")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	exec := &fakeExecutor{run: func(system.Command) error { return &system.SignalError{Signal: syscall.SIGINT} }}
	spec := plan.RunSpec{Image: "img", ConfigMounts: []plan.ConfigMount{{Agent: "pi", Staged: true, Writable: true, Root: root, HostPath: root, DestPath: "/home/caged/.pi"}}}
	err := NewClient(Deps{Exec: exec, FS: &fakeFS{}, Stager: configstage.New(system.FS{}), Platform: linux}).RunContainer(context.Background(), spec)
	if err == nil || err.Error() != "received signal: interrupt" {
		t.Fatalf("RunContainer() error = %v; want only the signal", err)
	}
}
