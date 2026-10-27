package docker

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/JeremyOrr4/caged/internal/configstage"
	"github.com/JeremyOrr4/caged/internal/engine"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
)

func TestRunContainerConnectsStdioAndPassesArgs(t *testing.T) {
	t.Parallel()

	stdin := strings.NewReader("")
	var stdout, stderr bytes.Buffer
	exec := &fakeExecutor{run: func(cmd system.Command) error {
		if cmd.Args[0] == "run" {
			_, err := io.WriteString(cmd.Stderr, "container stderr\n")
			return err
		}
		return nil
	}}
	spec := plan.RunSpec{Image: "img", Command: []string{"claude"}, ReturnToShell: true}
	if err := newClient(exec, &fakeFS{}, linux, system.Stdio{In: stdin, Out: &stdout, Err: &stderr}).RunContainer(context.Background(), spec); err != nil {
		t.Fatal(err)
	}
	if len(exec.commands) != 1 {
		t.Fatalf("commands = %+v, want only docker run (the daemon check is the caller's job)", exec.commands)
	}
	run := exec.commands[0]
	wantArgs := []string{"run", "-it", "--rm", "--user", "1000:1001", "-e", "CAGED_RETURN_TO_SHELL=1", "img", "claude"}
	if run.Path != "/usr/local/bin/docker" || !reflect.DeepEqual(run.Args, wantArgs) {
		t.Fatalf("run = %+v, want args %q", run, wantArgs)
	}
	if run.Stdin != stdin || run.Stdout != &stdout {
		t.Fatal("docker run is not connected to the client's stdin and stdout")
	}
	if stderr.String() != "container stderr\n" {
		t.Fatalf("stderr = %q, want the container's stderr passed through", stderr.String())
	}
}

func TestRunContainerStagesPiConfigSymlinksAndPersistsNewFiles(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".pi")
	target := filepath.Join(t.TempDir(), "dotfiles", "settings.json")
	if err := os.MkdirAll(filepath.Join(root, "agent"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("initial"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "agent", "settings.json")
	if err := os.Symlink("../../dotfiles/settings.json", link); err != nil {
		t.Fatal(err)
	}
	rootDest := "/home/caged/.pi"
	linkDest := rootDest + "/agent/settings.json"
	exec := &fakeExecutor{run: func(cmd system.Command) error {
		if cmd.Args[0] != "run" {
			return nil
		}
		var rootStage, targetStage string
		for i := 0; i+1 < len(cmd.Args); i++ {
			if cmd.Args[i] != "-v" {
				continue
			}
			source, destination, ok := strings.Cut(cmd.Args[i+1], ":")
			if !ok {
				continue
			}
			switch destination {
			case rootDest:
				rootStage = source
			case linkDest:
				targetStage = source
			}
		}
		if rootStage == root || targetStage == target || rootStage == "" || targetStage == "" {
			return errors.New("Pi mounts were not safely staged")
		}
		if err := os.WriteFile(filepath.Join(rootStage, "agent", "keybindings.json"), []byte("{}"), 0o600); err != nil {
			return err
		}
		return os.WriteFile(targetStage, []byte("updated"), 0o600)
	}}
	spec := plan.RunSpec{Image: "img", ConfigMounts: []plan.ConfigMount{
		{Agent: "pi", Root: root, HostPath: root, DestPath: rootDest, Writable: true, Staged: true, MaskFiles: []string{"agent/settings.json"}},
		{Agent: "pi", Root: root, HostPath: target, DestPath: linkDest, Writable: true, Staged: true, Link: link},
	}}
	if err := NewClient(Deps{Exec: exec, FS: &fakeFS{}, Stager: configstage.New(system.FS{}), Platform: linux}).RunContainer(context.Background(), spec); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "agent", "keybindings.json")); err != nil || string(got) != "{}" {
		t.Fatalf("new Pi config file on host = %q, %v", got, err)
	}
	if got, err := os.ReadFile(target); err != nil || string(got) != "updated" {
		t.Fatalf("linked Pi config file on host = %q, %v", got, err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("host symlink was altered: info=%v err=%v", info, err)
	}
}

func TestRunContainerErrors(t *testing.T) {
	t.Parallel()

	exitErr := errors.New("exit status 125")
	mountsDenied := func(cmd system.Command) error {
		if cmd.Args[0] != "run" {
			return nil
		}
		if _, err := io.WriteString(cmd.Stderr, "docker: Error response from daemon: Mounts denied: path not shared\n"); err != nil {
			return err
		}
		return exitErr
	}
	t.Run("linux returns the exit error unchanged", func(t *testing.T) {
		t.Parallel()
		err := newClient(&fakeExecutor{run: mountsDenied}, &fakeFS{}, linux, system.Stdio{Err: io.Discard}).RunContainer(context.Background(), plan.RunSpec{})
		if err != exitErr { //nolint:errorlint // asserts the error is returned unchanged, not wrapped
			t.Fatalf("RunContainer() error = %v, want the original exit error", err)
		}
	})
	t.Run("macOS explains Docker Desktop file sharing", func(t *testing.T) {
		t.Parallel()
		err := newClient(&fakeExecutor{run: mountsDenied}, &fakeFS{}, darwin, system.Stdio{Err: io.Discard}).RunContainer(context.Background(), plan.RunSpec{})
		want := "Docker could not mount the project path; in Docker Desktop, add its parent directory under Settings > Resources > File Sharing: exit status 125"
		if err == nil || err.Error() != want || !errors.Is(err, exitErr) {
			t.Fatalf("RunContainer() error = %v, want %q wrapping the exit error", err, want)
		}
	})
	t.Run("missing CLI stops before running", func(t *testing.T) {
		t.Parallel()
		exec := &fakeExecutor{lookPathErr: errors.New("x")}
		if err := newClient(exec, &fakeFS{}, linux, system.Stdio{}).RunContainer(context.Background(), plan.RunSpec{}); !engine.IsKind(err, engine.KindNotInstalled) {
			t.Fatalf("RunContainer() error = %v", err)
		}
		if len(exec.commands) != 0 {
			t.Fatalf("commands = %+v, want none", exec.commands)
		}
	})
}

func TestRunContainerSetupErrors(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")
	masked := plan.RunSpec{ConfigMounts: []plan.ConfigMount{{HostPath: "/h/.x", DestPath: "/home/caged/.x", MaskFiles: []string{"sock"}}}}
	t.Run("staging fails before docker runs", func(t *testing.T) {
		t.Parallel()
		exec := &fakeExecutor{}
		err := NewClient(Deps{Exec: exec, FS: &fakeFS{}, Stager: failingStager{boom}, Platform: linux}).RunContainer(context.Background(), masked)
		if !errors.Is(err, boom) || len(exec.commands) != 0 {
			t.Fatalf("RunContainer() error = %v, commands = %+v", err, exec.commands)
		}
	})
	t.Run("mask directory cannot be created", func(t *testing.T) {
		t.Parallel()
		exec := &fakeExecutor{}
		err := newClient(exec, &fakeFS{mkdirErr: boom}, linux, system.Stdio{}).RunContainer(context.Background(), masked)
		if err == nil || err.Error() != "create config mask source: boom" || len(exec.commands) != 0 {
			t.Fatalf("RunContainer() error = %v, commands = %+v", err, exec.commands)
		}
	})
	t.Run("mask file cannot be written", func(t *testing.T) {
		t.Parallel()
		exec := &fakeExecutor{}
		fsys := &fakeFS{writeErr: map[string]error{"/tmp/caged-run-masks-123/empty-file": boom}}
		err := newClient(exec, fsys, linux, system.Stdio{}).RunContainer(context.Background(), masked)
		if err == nil || err.Error() != "create config mask file: boom" || len(exec.commands) != 0 || len(fsys.removed) != 1 {
			t.Fatalf("RunContainer() error = %v, commands = %+v, removed = %v", err, exec.commands, fsys.removed)
		}
	})
	t.Run("a run without file masks creates and removes nothing", func(t *testing.T) {
		t.Parallel()
		exec := &fakeExecutor{}
		fsys := &fakeFS{}
		unmasked := plan.RunSpec{ConfigMounts: []plan.ConfigMount{{HostPath: "/h/.x", DestPath: "/home/caged/.x", MaskPaths: []string{"cache"}}}}
		if err := newClient(exec, fsys, linux, system.Stdio{}).RunContainer(context.Background(), unmasked); err != nil {
			t.Fatal(err)
		}
		if fsys.written != nil || fsys.removed != nil || len(exec.commands) != 1 {
			t.Fatalf("written = %v, removed = %v, commands = %+v; want only docker run", fsys.written, fsys.removed, exec.commands)
		}
	})
	t.Run("mask file is mounted and removed", func(t *testing.T) {
		t.Parallel()
		exec := &fakeExecutor{}
		fsys := &fakeFS{}
		if err := newClient(exec, fsys, linux, system.Stdio{}).RunContainer(context.Background(), masked); err != nil {
			t.Fatal(err)
		}
		if args := strings.Join(exec.commands[0].Args, " "); !strings.Contains(args, "-v /tmp/caged-run-masks-123/empty-file:/home/caged/.x/sock:ro") {
			t.Fatalf("docker run args = %s, want the mask file mounted", args)
		}
		if len(fsys.removed) != 1 || fsys.removed[0] != "/tmp/caged-run-masks-123" {
			t.Fatalf("removed = %v, want the mask directory", fsys.removed)
		}
	})
}

func TestRunContainerRemovesStagedCopiesWhenSetupFails(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	root := filepath.Join(t.TempDir(), ".pi")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	spec := plan.RunSpec{ConfigMounts: []plan.ConfigMount{{Staged: true, Root: root, HostPath: root, DestPath: "/home/caged/.pi", MaskFiles: []string{"sock"}}}}
	staged := &fakeFS{mkdirErr: errors.New("boom")}
	client := NewClient(Deps{Exec: &fakeExecutor{}, FS: staged, Stager: configstage.New(system.FS{}), Platform: linux})
	// The staged mount needs no file mask once copied, so add an unstaged one that does.
	spec.ConfigMounts = append(spec.ConfigMounts, plan.ConfigMount{HostPath: "/h/.x", DestPath: "/home/caged/.x", MaskFiles: []string{"sock"}})
	if err := client.RunContainer(context.Background(), spec); err == nil {
		t.Fatal("RunContainer() succeeded, want the mask error")
	}
	if entries, err := os.ReadDir(tmp); err != nil || len(entries) != 0 {
		t.Fatalf("temp dir after a failed setup = %v (err %v), want the staged copies removed", entries, err)
	}
}

// A write-back that fails still removes the staged copies, and the failure is
// returned.
func TestRunContainerRemovesStagedCopiesWhenWriteBackFails(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp) // the staging directory goes here
	root := filepath.Join(t.TempDir(), ".pi")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.json"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	const dest = "/home/caged/.pi"
	var stage string
	exec := &fakeExecutor{run: func(cmd system.Command) error {
		for i := 0; i+1 < len(cmd.Args); i++ {
			if source, destination, ok := strings.Cut(cmd.Args[i+1], ":"); ok && cmd.Args[i] == "-v" && destination == dest {
				stage = source
			}
		}
		if stage == "" {
			return errors.New("the staged copy was not mounted")
		}
		if err := os.WriteFile(filepath.Join(stage, "a.json"), []byte("container"), 0o600); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(root, "a.json"), []byte("host"), 0o600)
	}}
	spec := plan.RunSpec{Image: "img", ConfigMounts: []plan.ConfigMount{{Staged: true, Writable: true, Root: root, HostPath: root, DestPath: dest, MaskFiles: []string{"sock"}}}}
	err := NewClient(Deps{Exec: exec, FS: &fakeFS{}, Stager: configstage.New(system.FS{}), Platform: linux}).RunContainer(context.Background(), spec)
	if err == nil || !strings.Contains(err.Error(), "host and container both changed") {
		t.Fatalf("RunContainer() error = %v, want the write-back conflict", err)
	}
	if entries, err := os.ReadDir(tmp); err != nil || len(entries) != 0 {
		t.Fatalf("temp dir entries = %v, %v; want the staging directory removed", entries, err)
	}
}

func TestRunContainerJoinsStagingCleanupErrors(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")
	stagedSpec := func(t *testing.T) plan.RunSpec {
		root := filepath.Join(t.TempDir(), ".pi")
		if err := os.MkdirAll(root, 0o700); err != nil {
			t.Fatal(err)
		}
		return plan.RunSpec{ConfigMounts: []plan.ConfigMount{{Staged: true, Root: root, HostPath: root, DestPath: "/home/caged/.pi", MaskFiles: []string{"sock"}}}}
	}
	client := func(exec *fakeExecutor, fsys FileSystem) *Client {
		return NewClient(Deps{Exec: exec, FS: fsys, Stager: configstage.New(removeFailFS{err: boom}), Platform: linux, Stdio: system.Stdio{Err: io.Discard}})
	}
	t.Run("a failed run comes first, then the cleanup error", func(t *testing.T) {
		t.Parallel()
		exec := &fakeExecutor{run: func(system.Command) error { return exitError{code: 125} }}
		err := client(exec, &fakeFS{}).RunContainer(context.Background(), stagedSpec(t))
		var coder interface{ ExitCode() int }
		if err == nil || err.Error() != "exit status 125\nboom" || !errors.Is(err, boom) || !errors.As(err, &coder) || coder.ExitCode() != 125 {
			t.Fatalf("RunContainer() error = %v, want the exit error (code 125) joined with the cleanup error", err)
		}
	})
	t.Run("a successful run still reports the cleanup error", func(t *testing.T) {
		t.Parallel()
		err := client(&fakeExecutor{}, &fakeFS{}).RunContainer(context.Background(), stagedSpec(t))
		if err == nil || err.Error() != "boom" || !errors.Is(err, boom) {
			t.Fatalf("RunContainer() error = %v, want the cleanup error", err)
		}
	})
	t.Run("after a signal the signal comes first, then the cleanup error", func(t *testing.T) {
		t.Parallel()
		exec := &fakeExecutor{run: func(system.Command) error {
			return &system.KilledError{Signal: syscall.SIGKILL, Err: errors.New("signal: killed")}
		}}
		err := client(exec, &fakeFS{}).RunContainer(context.Background(), stagedSpec(t))
		var coder interface{ ExitCode() int }
		if err == nil || err.Error() != "signal: killed; Pi config changes from this run were not written back\nboom" || !errors.Is(err, boom) || !errors.As(err, &coder) || coder.ExitCode() != 137 {
			t.Fatalf("RunContainer() error = %v, want the signal (code 137) joined with the cleanup error", err)
		}
	})
	t.Run("a setup failure comes first, then the cleanup error", func(t *testing.T) {
		t.Parallel()
		spec := stagedSpec(t)
		// The staged mount needs no file mask once copied, so add an unstaged one that does.
		spec.ConfigMounts = append(spec.ConfigMounts, plan.ConfigMount{HostPath: "/h/.x", DestPath: "/home/caged/.x", MaskFiles: []string{"sock"}})
		exec := &fakeExecutor{}
		err := client(exec, &fakeFS{mkdirErr: errors.New("mk")}).RunContainer(context.Background(), spec)
		if err == nil || err.Error() != "create config mask source: mk\nboom" || !errors.Is(err, boom) || len(exec.commands) != 0 {
			t.Fatalf("RunContainer() error = %v, commands = %+v; want the mask error joined with the cleanup error, and no docker run", err, exec.commands)
		}
	})
}
