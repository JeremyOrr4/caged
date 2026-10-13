package docker

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
)

// fakeExecutor records commands instead of running them.
type fakeExecutor struct {
	lookPathErr error
	// lookPathFailAfter makes LookPath fail once it has succeeded this many times (0 = never).
	lookPathFailAfter int
	lookPathCalls     int
	// run answers a command; nil means success.
	run      func(system.Command) error
	commands []system.Command
}

func (f *fakeExecutor) LookPath(file string) (string, error) {
	f.lookPathCalls++
	if f.lookPathErr != nil || (f.lookPathFailAfter > 0 && f.lookPathCalls > f.lookPathFailAfter) {
		return "", errors.New(`exec: "` + file + `": executable file not found in $PATH`)
	}
	return "/usr/local/bin/" + file, nil
}

func (f *fakeExecutor) Run(_ context.Context, cmd system.Command) error {
	f.commands = append(f.commands, cmd)
	if f.run != nil {
		return f.run(cmd)
	}
	return nil
}

// fakeFS records the temporary build context instead of touching the disk.
type fakeFS struct {
	mkdirErr error
	writeErr map[string]error // by file name
	written  map[string]writtenFile
	removed  []string
}

type writtenFile struct {
	data string
	mode fs.FileMode
}

func (f *fakeFS) MkdirTemp(dir, pattern string) (string, error) {
	if f.mkdirErr != nil {
		return "", f.mkdirErr
	}
	return filepath.Join("/tmp", pattern+"123"), nil
}

func (f *fakeFS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	if err := f.writeErr[name]; err != nil {
		return err
	}
	if f.written == nil {
		f.written = map[string]writtenFile{}
	}
	f.written[name] = writtenFile{data: string(data), mode: perm}
	return nil
}

func (f *fakeFS) RemoveAll(path string) error {
	f.removed = append(f.removed, path)
	return nil
}

var (
	linux  = system.Platform{OS: "linux", Arch: "amd64", UID: 1000, GID: 1001}
	darwin = system.Platform{OS: "darwin", Arch: "arm64", UID: 501, GID: 20}
)

func TestCheckRunsDockerInfo(t *testing.T) {
	t.Parallel()

	exec := &fakeExecutor{}
	if err := NewClient(exec, &fakeFS{}, darwin, system.Stdio{}).Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(exec.commands) != 1 {
		t.Fatalf("commands = %+v, want one docker info", exec.commands)
	}
	info := exec.commands[0]
	if info.Path != "/usr/local/bin/docker" || !reflect.DeepEqual(info.Args, []string{"info", "--format", "{{.ServerVersion}}"}) {
		t.Fatalf("command = %+v", info)
	}
	// Combined output: stdout and stderr share one buffer, like exec.Cmd.CombinedOutput.
	if info.Stdout == nil || info.Stdout != info.Stderr || info.Stdin != nil {
		t.Fatalf("docker info must capture combined output and read no input: %+v", info)
	}
}

func TestCheckErrors(t *testing.T) {
	t.Parallel()

	daemonDown := func(cmd system.Command) error {
		io.WriteString(cmd.Stdout, "Cannot connect to the Docker daemon\n")
		return errors.New("exit status 1")
	}
	tests := []struct {
		name     string
		platform system.Platform
		exec     *fakeExecutor
		want     string
	}{
		{name: "unsupported arch", platform: system.Platform{OS: "linux", Arch: "386"}, exec: &fakeExecutor{}, want: `unsupported host architecture "386"; caged supports amd64 and arm64 images`},
		{name: "missing CLI", platform: linux, exec: &fakeExecutor{lookPathErr: errors.New("x")}, want: `Docker is not installed or not on PATH: exec: "docker": executable file not found in $PATH`},
		{name: "daemon down on Linux", platform: linux, exec: &fakeExecutor{run: daemonDown}, want: "Docker is installed but the daemon is unavailable; start the Docker daemon: Cannot connect to the Docker daemon"},
		{name: "daemon down on macOS", platform: darwin, exec: &fakeExecutor{run: daemonDown}, want: "Docker is installed but the daemon is unavailable; start Docker Desktop: Cannot connect to the Docker daemon"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := NewClient(tt.exec, &fakeFS{}, tt.platform, system.Stdio{}).Check(context.Background())
			if err == nil || err.Error() != tt.want {
				t.Fatalf("Check() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestImageExists(t *testing.T) {
	t.Parallel()

	present := &fakeExecutor{}
	if !NewClient(present, &fakeFS{}, linux, system.Stdio{}).ImageExists(context.Background(), "caged-runtime:latest") {
		t.Fatal("ImageExists() = false, want true")
	}
	want := system.Command{Path: "docker", Args: []string{"image", "inspect", "caged-runtime:latest"}}
	if !reflect.DeepEqual(present.commands, []system.Command{want}) {
		t.Fatalf("commands = %+v, want %+v", present.commands, want)
	}
	missing := &fakeExecutor{run: func(system.Command) error { return errors.New("exit status 1") }}
	if NewClient(missing, &fakeFS{}, linux, system.Stdio{}).ImageExists(context.Background(), "caged-runtime:latest") {
		t.Fatal("ImageExists() = true, want false")
	}
}

func TestRunConnectsStdioAndPassesArgs(t *testing.T) {
	t.Parallel()

	stdin := strings.NewReader("")
	var stdout, stderr bytes.Buffer
	exec := &fakeExecutor{run: func(cmd system.Command) error {
		if cmd.Args[0] == "run" {
			io.WriteString(cmd.Stderr, "container stderr\n")
		}
		return nil
	}}
	spec := plan.RunSpec{Image: "img", Command: []string{"claude"}, ReturnToShell: true}
	if err := NewClient(exec, &fakeFS{}, linux, system.Stdio{In: stdin, Out: &stdout, Err: &stderr}).Run(context.Background(), spec); err != nil {
		t.Fatal(err)
	}
	if len(exec.commands) != 2 {
		t.Fatalf("commands = %+v, want docker info then docker run", exec.commands)
	}
	run := exec.commands[1]
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

func TestRunErrors(t *testing.T) {
	t.Parallel()

	exitErr := errors.New("exit status 125")
	mountsDenied := func(cmd system.Command) error {
		if cmd.Args[0] != "run" {
			return nil
		}
		io.WriteString(cmd.Stderr, "docker: Error response from daemon: Mounts denied: path not shared\n")
		return exitErr
	}
	t.Run("linux returns the exit error unchanged", func(t *testing.T) {
		t.Parallel()
		err := NewClient(&fakeExecutor{run: mountsDenied}, &fakeFS{}, linux, system.Stdio{Err: io.Discard}).Run(context.Background(), plan.RunSpec{})
		if err != exitErr {
			t.Fatalf("Run() error = %v, want the original exit error", err)
		}
	})
	t.Run("macOS explains Docker Desktop file sharing", func(t *testing.T) {
		t.Parallel()
		err := NewClient(&fakeExecutor{run: mountsDenied}, &fakeFS{}, darwin, system.Stdio{Err: io.Discard}).Run(context.Background(), plan.RunSpec{})
		want := "Docker could not mount the project path; in Docker Desktop, add its parent directory under Settings > Resources > File Sharing: exit status 125"
		if err == nil || err.Error() != want || !errors.Is(err, exitErr) {
			t.Fatalf("Run() error = %v, want %q wrapping the exit error", err, want)
		}
	})
	t.Run("check failure stops before running", func(t *testing.T) {
		t.Parallel()
		exec := &fakeExecutor{lookPathErr: errors.New("x")}
		if err := NewClient(exec, &fakeFS{}, linux, system.Stdio{}).Run(context.Background(), plan.RunSpec{}); err == nil || !strings.HasPrefix(err.Error(), "Docker is not installed") {
			t.Fatalf("Run() error = %v", err)
		}
		if len(exec.commands) != 0 {
			t.Fatalf("commands = %+v, want none", exec.commands)
		}
	})
	t.Run("docker disappears after the check", func(t *testing.T) {
		t.Parallel()
		exec := &fakeExecutor{lookPathFailAfter: 1}
		err := NewClient(exec, &fakeFS{}, linux, system.Stdio{}).Run(context.Background(), plan.RunSpec{})
		if err == nil || err.Error() != `docker not found, is Docker installed and running?: exec: "docker": executable file not found in $PATH` {
			t.Fatalf("Run() error = %v", err)
		}
	})
}

func TestPlatformRules(t *testing.T) {
	t.Parallel()

	if got := userArgs(linux); !reflect.DeepEqual(got, []string{"--user", "1000:1001"}) {
		t.Errorf("userArgs(linux) = %q", got)
	}
	if got := userArgs(darwin); got != nil {
		t.Errorf("userArgs(darwin) = %q, want none: Docker Desktop maps ownership", got)
	}
	if got := daemonLauncher(darwin); got != "start Docker Desktop" {
		t.Errorf("daemonLauncher(darwin) = %q", got)
	}
	if got := daemonLauncher(linux); got != "start the Docker daemon" {
		t.Errorf("daemonLauncher(linux) = %q", got)
	}
	for _, arch := range []string{"amd64", "arm64"} {
		if err := checkArch(system.Platform{Arch: arch}); err != nil {
			t.Errorf("checkArch(%s) = %v", arch, err)
		}
	}
	for _, message := range []string{"Mounts denied", "path is NOT SHARED from the host", "mount denied"} {
		if !looksLikeSharedPathError(message) {
			t.Errorf("looksLikeSharedPathError(%q) = false", message)
		}
	}
	plain := errors.New("exit status 1")
	if got := explainRunError(darwin, "some other failure", plain); got != plain {
		t.Errorf("explainRunError() = %v, want the error unchanged", got)
	}
}

func testBuildSpec(noCache bool) plan.BuildSpec {
	return plan.BuildSpec{
		Image:      "caged-runtime:latest",
		Dockerfile: "Dockerfile",
		NoCache:    noCache,
		Files: []plan.File{
			{Name: "Dockerfile", Label: "Dockerfile", Data: []byte("FROM scratch\n"), Mode: 0o644},
			{Name: "caged-entrypoint.sh", Label: "entrypoint", Data: []byte("#!/bin/sh\n"), Mode: 0o755},
		},
	}
}

func TestBuildWritesContextRunsDockerBuildAndCleansUp(t *testing.T) {
	t.Parallel()

	for _, noCache := range []bool{false, true} {
		exec, fsys := &fakeExecutor{}, &fakeFS{}
		stdio := system.Stdio{In: strings.NewReader(""), Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
		if err := NewClient(exec, fsys, linux, stdio).Build(context.Background(), testBuildSpec(noCache)); err != nil {
			t.Fatal(err)
		}
		wantFiles := map[string]writtenFile{
			"/tmp/caged-runtime-123/Dockerfile":          {data: "FROM scratch\n", mode: 0o644},
			"/tmp/caged-runtime-123/caged-entrypoint.sh": {data: "#!/bin/sh\n", mode: 0o755},
		}
		if !reflect.DeepEqual(fsys.written, wantFiles) {
			t.Fatalf("written = %+v, want %+v", fsys.written, wantFiles)
		}
		if len(exec.commands) != 2 || exec.commands[0].Args[0] != "info" {
			t.Fatalf("commands = %+v, want docker info then docker build", exec.commands)
		}
		build := exec.commands[1]
		wantArgs := []string{"build", "-t", "caged-runtime:latest", "-f", "/tmp/caged-runtime-123/Dockerfile", "/tmp/caged-runtime-123"}
		if noCache {
			wantArgs = []string{"build", "-t", "caged-runtime:latest", "--no-cache", "-f", "/tmp/caged-runtime-123/Dockerfile", "/tmp/caged-runtime-123"}
		}
		if build.Path != "/usr/local/bin/docker" || !reflect.DeepEqual(build.Args, wantArgs) {
			t.Fatalf("build = %+v, want args %q", build, wantArgs)
		}
		if build.Stdin != stdio.In || build.Stdout != stdio.Out || build.Stderr != stdio.Err {
			t.Fatal("docker build is not connected to the client's stdio")
		}
		if !reflect.DeepEqual(fsys.removed, []string{"/tmp/caged-runtime-123"}) {
			t.Fatalf("removed = %v, want the build directory", fsys.removed)
		}
	}
}

func TestBuildErrors(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")
	t.Run("temporary directory", func(t *testing.T) {
		t.Parallel()
		exec := &fakeExecutor{}
		err := NewClient(exec, &fakeFS{mkdirErr: boom}, linux, system.Stdio{}).Build(context.Background(), testBuildSpec(false))
		if err == nil || err.Error() != "create temporary build directory: boom" || len(exec.commands) != 1 {
			t.Fatalf("Build() error = %v, commands = %+v", err, exec.commands)
		}
	})
	t.Run("write uses the file label", func(t *testing.T) {
		t.Parallel()
		fsys := &fakeFS{writeErr: map[string]error{"/tmp/caged-runtime-123/caged-entrypoint.sh": boom}}
		err := NewClient(&fakeExecutor{}, fsys, linux, system.Stdio{}).Build(context.Background(), testBuildSpec(false))
		if err == nil || err.Error() != "write embedded entrypoint: boom" || len(fsys.removed) != 1 {
			t.Fatalf("Build() error = %v, removed = %v", err, fsys.removed)
		}
	})
	t.Run("docker build failure is returned unchanged", func(t *testing.T) {
		t.Parallel()
		fsys := &fakeFS{}
		exec := &fakeExecutor{run: func(cmd system.Command) error {
			if cmd.Args[0] == "build" {
				return boom
			}
			return nil
		}}
		if err := NewClient(exec, fsys, linux, system.Stdio{}).Build(context.Background(), testBuildSpec(false)); err != boom || len(fsys.removed) != 1 {
			t.Fatalf("Build() error = %v, removed = %v", err, fsys.removed)
		}
	})
	t.Run("check failure creates nothing", func(t *testing.T) {
		t.Parallel()
		fsys := &fakeFS{}
		err := NewClient(&fakeExecutor{lookPathErr: boom}, fsys, linux, system.Stdio{}).Build(context.Background(), testBuildSpec(false))
		if err == nil || !strings.HasPrefix(err.Error(), "Docker is not installed or not on PATH") || fsys.written != nil || fsys.removed != nil {
			t.Fatalf("Build() error = %v, fs = %+v", err, fsys)
		}
	})
	t.Run("docker disappears after the check", func(t *testing.T) {
		t.Parallel()
		err := NewClient(&fakeExecutor{lookPathFailAfter: 1}, &fakeFS{}, linux, system.Stdio{}).Build(context.Background(), testBuildSpec(false))
		if err == nil || err.Error() != `Docker is not installed or not on PATH: exec: "docker": executable file not found in $PATH` {
			t.Fatalf("Build() error = %v", err)
		}
	})
}
