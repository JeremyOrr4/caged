package docker

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/engine"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
)

// fakeExecutor records commands instead of running them.
type fakeExecutor struct {
	lookPathErr error
	// run answers a command; nil means success.
	run      func(system.Command) error
	commands []system.Command
}

func (f *fakeExecutor) LookPath(file string) (string, error) {
	if f.lookPathErr != nil {
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
	mkdirErr  error
	writeErr  map[string]error // by file name
	removeErr error
	readErr   error  // returned by ReadFile
	idFile    string // what ReadFile returns; "" means "sha256:fake-id\n"
	written   map[string]writtenFile
	removed   []string
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

func (f *fakeFS) ReadFile(name string) ([]byte, error) {
	if f.readErr != nil {
		return nil, f.readErr
	}
	if f.idFile != "" {
		return []byte(f.idFile), nil
	}
	return []byte("sha256:fake-id\n"), nil
}

func (f *fakeFS) RemoveAll(path string) error {
	f.removed = append(f.removed, path)
	return f.removeErr
}

var (
	linux  = system.Platform{OS: "linux", Arch: "amd64", UID: 1000, GID: 1001}
	darwin = system.Platform{OS: "darwin", Arch: "arm64", UID: 501, GID: 20}
)

func TestDaemonRunsDockerInfoAndParsesTheReply(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		reply string
		want  engine.DaemonInfo
	}{
		{name: "Docker Desktop on Intel", reply: "27.0.0|Docker Desktop|x86_64\n", want: engine.DaemonInfo{CLIPath: "/usr/local/bin/docker", ServerVersion: "27.0.0", OperatingSystem: "Docker Desktop", Arch: "amd64"}},
		{name: "Apple silicon reports aarch64", reply: "28.1.1|Docker Desktop|aarch64\n", want: engine.DaemonInfo{CLIPath: "/usr/local/bin/docker", ServerVersion: "28.1.1", OperatingSystem: "Docker Desktop", Arch: "arm64"}},
		{name: "already normalized", reply: "25.0.0|Ubuntu 24.04 LTS|arm64", want: engine.DaemonInfo{CLIPath: "/usr/local/bin/docker", ServerVersion: "25.0.0", OperatingSystem: "Ubuntu 24.04 LTS", Arch: "arm64"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			exec := &fakeExecutor{run: func(cmd system.Command) error {
				_, err := io.WriteString(cmd.Stdout, tt.reply)
				return err
			}}
			got, err := NewClient(exec, &fakeFS{}, darwin, system.Stdio{}).Daemon(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Daemon() = %+v, want %+v", got, tt.want)
			}
			if len(exec.commands) != 1 {
				t.Fatalf("commands = %+v, want one docker info", exec.commands)
			}
			info := exec.commands[0]
			wantArgs := []string{"info", "--format", "{{.ServerVersion}}|{{.OperatingSystem}}|{{.Architecture}}"}
			if info.Path != "/usr/local/bin/docker" || !reflect.DeepEqual(info.Args, wantArgs) {
				t.Fatalf("command = %+v", info)
			}
			if info.Stdout == nil || info.Stderr == nil || info.Stdout == info.Stderr || info.Stdin != nil {
				t.Fatalf("docker info must capture stdout and stderr separately and read no input: %+v", info)
			}
		})
	}
}

func TestDaemonErrors(t *testing.T) {
	t.Parallel()

	daemonDown := func(cmd system.Command) error {
		if _, err := io.WriteString(cmd.Stderr, "Cannot connect to the Docker daemon\n"); err != nil {
			return err
		}
		return errors.New("exit status 1")
	}
	tests := []struct {
		name     string
		platform system.Platform
		exec     *fakeExecutor
		kind     engine.Kind
		want     string
	}{
		{name: "unsupported arch", platform: system.Platform{OS: "linux", Arch: "386"}, exec: &fakeExecutor{}, kind: engine.KindUnsupportedHost, want: `unsupported host architecture "386"; caged supports amd64 and arm64 images`},
		{name: "missing CLI", platform: linux, exec: &fakeExecutor{lookPathErr: errors.New("x")}, kind: engine.KindNotInstalled, want: `Docker is not installed or not on PATH: exec: "docker": executable file not found in $PATH`},
		{name: "daemon down on Linux", platform: linux, exec: &fakeExecutor{run: daemonDown}, kind: engine.KindDaemonDown, want: "Docker is installed but the daemon is unavailable; start the Docker daemon: Cannot connect to the Docker daemon"},
		{name: "daemon down on macOS", platform: darwin, exec: &fakeExecutor{run: daemonDown}, kind: engine.KindDaemonDown, want: "Docker is installed but the daemon is unavailable; start Docker Desktop: Cannot connect to the Docker daemon"},
		{
			name: "daemon down without output keeps the exit error", platform: linux, kind: engine.KindDaemonDown,
			exec: &fakeExecutor{run: func(system.Command) error { return errors.New("exit status 1") }},
			want: "Docker is installed but the daemon is unavailable; start the Docker daemon: exit status 1",
		},
		{
			name: "unexpected reply", platform: linux, kind: engine.KindUnknown,
			exec: &fakeExecutor{run: func(cmd system.Command) error { _, err := io.WriteString(cmd.Stdout, "27.0.0\n"); return err }},
			want: `unexpected docker info output "27.0.0"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewClient(tt.exec, &fakeFS{}, tt.platform, system.Stdio{}).Daemon(context.Background())
			if err == nil || err.Error() != tt.want {
				t.Fatalf("Daemon() error = %v, want %q", err, tt.want)
			}
			if tt.kind != engine.KindUnknown && !engine.IsKind(err, tt.kind) {
				t.Fatalf("Daemon() error kind is not %v: %v", tt.kind, err)
			}
		})
	}
}

func TestInspectImage(t *testing.T) {
	t.Parallel()

	const reply = `[{"Id":"sha256:abc","Os":"linux","Architecture":"arm64","Config":{"Labels":{"a":"b"}}}]`
	t.Run("runs the resolved path and parses the reply", func(t *testing.T) {
		t.Parallel()
		exec := &fakeExecutor{run: func(cmd system.Command) error { _, err := io.WriteString(cmd.Stdout, reply); return err }}
		got, err := NewClient(exec, &fakeFS{}, linux, system.Stdio{}).InspectImage(context.Background(), "caged-runtime:latest")
		if err != nil {
			t.Fatal(err)
		}
		want := engine.ImageInfo{ID: "sha256:abc", OS: "linux", Arch: "arm64", Labels: map[string]string{"a": "b"}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("InspectImage() = %+v, want %+v", got, want)
		}
		if len(exec.commands) != 1 || exec.commands[0].Path != "/usr/local/bin/docker" || !reflect.DeepEqual(exec.commands[0].Args, []string{"image", "inspect", "caged-runtime:latest"}) {
			t.Fatalf("commands = %+v", exec.commands)
		}
	})
	t.Run("errors", func(t *testing.T) {
		t.Parallel()
		failing := func(stderr string) *fakeExecutor {
			return &fakeExecutor{run: func(cmd system.Command) error {
				if _, err := io.WriteString(cmd.Stderr, stderr); err != nil {
					return err
				}
				return errors.New("exit status 1")
			}}
		}
		tests := []struct {
			name string
			exec *fakeExecutor
			kind engine.Kind
			want string
		}{
			{name: "missing image", exec: failing("Error response from daemon: No such image: img\n"), kind: engine.KindImageNotFound, want: "image img not found: exit status 1"},
			{name: "newer docker wording", exec: failing("Error: No such image: img\n"), kind: engine.KindImageNotFound, want: "image img not found: exit status 1"},
			{name: "other failure", exec: failing("permission denied\n"), kind: engine.KindUnknown, want: "inspect image img: permission denied: exit status 1"},
			{name: "missing CLI", exec: &fakeExecutor{lookPathErr: errors.New("x")}, kind: engine.KindNotInstalled, want: `Docker is not installed or not on PATH: exec: "docker": executable file not found in $PATH`},
			{name: "bad JSON", exec: &fakeExecutor{run: func(cmd system.Command) error { _, err := io.WriteString(cmd.Stdout, "nope"); return err }}, kind: engine.KindUnknown, want: "parse docker image inspect output: invalid character 'o' in literal null (expecting 'u')"},
		}
		for _, tt := range tests {
			_, err := NewClient(tt.exec, &fakeFS{}, linux, system.Stdio{}).InspectImage(context.Background(), "img")
			if err == nil || err.Error() != tt.want {
				t.Errorf("%s: InspectImage() error = %v, want %q", tt.name, err, tt.want)
			}
			if tt.kind != engine.KindUnknown && !engine.IsKind(err, tt.kind) {
				t.Errorf("%s: error kind is not %v: %v", tt.name, tt.kind, err)
			}
			if tt.kind != engine.KindImageNotFound && engine.IsKind(err, engine.KindImageNotFound) {
				t.Errorf("%s: a failure other than a missing image reported KindImageNotFound", tt.name)
			}
		}
	})
}

func TestBuildImageNamesTheTempDirAfterTheContext(t *testing.T) {
	t.Parallel()

	spec := testBuildSpec(false)
	spec.ContextName = "caged-extension"
	fsys := &fakeFS{}
	if _, err := NewClient(&fakeExecutor{}, fsys, linux, system.Stdio{}).BuildImage(context.Background(), spec); err != nil {
		t.Fatal(err)
	}
	if want := []string{"/tmp/caged-extension-123"}; !reflect.DeepEqual(fsys.removed, want) {
		t.Fatalf("temporary directories = %v, want %v", fsys.removed, want)
	}
}

func TestRemoveImage(t *testing.T) {
	t.Parallel()

	failing := func(stderr string) *fakeExecutor {
		return &fakeExecutor{run: func(cmd system.Command) error {
			if _, err := io.WriteString(cmd.Stderr, stderr); err != nil {
				return err
			}
			return errors.New("exit status 1")
		}}
	}
	t.Run("runs docker image rm without force", func(t *testing.T) {
		t.Parallel()
		exec := &fakeExecutor{}
		if err := NewClient(exec, &fakeFS{}, linux, system.Stdio{}).RemoveImage(context.Background(), "sha256:abc"); err != nil {
			t.Fatal(err)
		}
		if len(exec.commands) != 1 || exec.commands[0].Path != "/usr/local/bin/docker" || !reflect.DeepEqual(exec.commands[0].Args, []string{"image", "rm", "sha256:abc"}) {
			t.Fatalf("commands = %+v", exec.commands)
		}
	})
	tests := []struct {
		name     string
		exec     *fakeExecutor
		kind     engine.Kind
		inUse    bool
		notFound bool
		wantIn   string
	}{
		{name: "conflict", exec: failing("Error response from daemon: conflict: unable to delete abc (cannot be forced) - image is being used by running container x\n"), inUse: true, wantIn: "conflict: unable to delete abc"},
		{name: "being used", exec: failing("image is being used by stopped container x"), inUse: true, wantIn: "being used"},
		{name: "another tag", exec: failing("image is referenced in multiple repositories; the image is using it"), inUse: true, wantIn: "is using"},
		{name: "other failure", exec: failing("permission denied"), inUse: false, wantIn: "permission denied"},
		{name: "already gone", exec: failing("Error response from daemon: No such image: sha256:abc"), inUse: false, notFound: true, wantIn: "No such image"},
		{name: "missing CLI", exec: &fakeExecutor{lookPathErr: errors.New("x")}, inUse: false, wantIn: "Docker is not installed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := NewClient(tt.exec, &fakeFS{}, linux, system.Stdio{}).RemoveImage(context.Background(), "sha256:abc")
			if err == nil || !strings.Contains(err.Error(), tt.wantIn) {
				t.Fatalf("RemoveImage() error = %v, want it to contain %q", err, tt.wantIn)
			}
			if engine.IsKind(err, engine.KindImageNotFound) != tt.notFound {
				t.Fatalf("RemoveImage() not-found kind = %v, want %v: %v", !tt.notFound, tt.notFound, err)
			}
			if engine.IsKind(err, engine.KindImageInUse) != tt.inUse {
				t.Fatalf("RemoveImage() in-use kind = %v, want %v: %v", !tt.inUse, tt.inUse, err)
			}
		})
	}
}

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
	if err := NewClient(exec, &fakeFS{}, linux, system.Stdio{In: stdin, Out: &stdout, Err: &stderr}).RunContainer(context.Background(), spec); err != nil {
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
		{Agent: "pi", Root: root, HostPath: root, DestPath: rootDest, Writable: true, MaskFiles: []string{"agent/settings.json"}},
		{Agent: "pi", Root: root, HostPath: target, DestPath: linkDest, Writable: true, Link: link},
	}}
	if err := NewClient(exec, &fakeFS{}, linux, system.Stdio{}).RunContainer(context.Background(), spec); err != nil {
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

func TestRunContainerRemovesStagedCopiesWhenSetupFails(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	root := filepath.Join(t.TempDir(), ".pi")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	spec := plan.RunSpec{Image: "img", ConfigMounts: []plan.ConfigMount{
		{Agent: "pi", Root: root, HostPath: root, DestPath: "/home/caged/.pi", Writable: true, MaskFiles: []string{"sock"}},
		{HostPath: "/h/.x", DestPath: "/home/caged/.x", MaskFiles: []string{"sock"}},
	}}
	client := NewClient(&fakeExecutor{}, &fakeFS{mkdirErr: errors.New("boom")}, linux, system.Stdio{})
	if err := client.RunContainer(context.Background(), spec); err == nil {
		t.Fatal("RunContainer() succeeded, want the mask error")
	}
	if entries, err := os.ReadDir(tmp); err != nil || len(entries) != 0 {
		t.Fatalf("temp dir after a failed setup = %v (err %v), want the staged copies removed", entries, err)
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
		err := NewClient(&fakeExecutor{run: mountsDenied}, &fakeFS{}, linux, system.Stdio{Err: io.Discard}).RunContainer(context.Background(), plan.RunSpec{})
		if err != exitErr {
			t.Fatalf("RunContainer() error = %v, want the original exit error", err)
		}
	})
	t.Run("macOS explains Docker Desktop file sharing", func(t *testing.T) {
		t.Parallel()
		err := NewClient(&fakeExecutor{run: mountsDenied}, &fakeFS{}, darwin, system.Stdio{Err: io.Discard}).RunContainer(context.Background(), plan.RunSpec{})
		want := "Docker could not mount the project path; in Docker Desktop, add its parent directory under Settings > Resources > File Sharing: exit status 125"
		if err == nil || err.Error() != want || !errors.Is(err, exitErr) {
			t.Fatalf("RunContainer() error = %v, want %q wrapping the exit error", err, want)
		}
	})
	t.Run("missing CLI stops before running", func(t *testing.T) {
		t.Parallel()
		exec := &fakeExecutor{lookPathErr: errors.New("x")}
		if err := NewClient(exec, &fakeFS{}, linux, system.Stdio{}).RunContainer(context.Background(), plan.RunSpec{}); !engine.IsKind(err, engine.KindNotInstalled) {
			t.Fatalf("RunContainer() error = %v", err)
		}
		if len(exec.commands) != 0 {
			t.Fatalf("commands = %+v, want none", exec.commands)
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
	if got := sharedPathHint(darwin, "some other failure", plain); got != nil {
		t.Errorf("sharedPathHint() = %v, want no hint", got)
	}
	if got := sharedPathHint(linux, "Mounts denied", plain); got != nil {
		t.Errorf("sharedPathHint(linux) = %v, want no hint: only Docker Desktop has File Sharing", got)
	}
}

func testBuildSpec(noCache bool) plan.BuildSpec {
	return plan.BuildSpec{
		Image:       "caged-runtime:latest",
		Dockerfile:  "Dockerfile",
		ContextName: "caged-runtime",
		Labels:      []plan.Label{{Key: "a.role", Value: "runtime"}, {Key: "a.version", Value: "0.1.0"}},
		NoCache:     noCache,
		Files: []plan.File{
			{Name: "Dockerfile", Label: "Dockerfile", Data: []byte("FROM scratch\n"), Mode: 0o644},
			{Name: "caged-entrypoint.sh", Label: "entrypoint", Data: []byte("#!/bin/sh\n"), Mode: 0o755},
		},
	}
}

func TestBuildImageWritesContextRunsDockerBuildAndCleansUp(t *testing.T) {
	t.Parallel()

	for _, noCache := range []bool{false, true} {
		exec, fsys := &fakeExecutor{}, &fakeFS{}
		stdio := system.Stdio{In: strings.NewReader(""), Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
		id, err := NewClient(exec, fsys, linux, stdio).BuildImage(context.Background(), testBuildSpec(noCache))
		if err != nil {
			t.Fatal(err)
		}
		if id != "sha256:fake-id" {
			t.Fatalf("BuildImage() id = %q, want the trimmed contents of the --iidfile", id)
		}
		wantFiles := map[string]writtenFile{
			"/tmp/caged-runtime-123/Dockerfile":          {data: "FROM scratch\n", mode: 0o644},
			"/tmp/caged-runtime-123/caged-entrypoint.sh": {data: "#!/bin/sh\n", mode: 0o755},
		}
		if !reflect.DeepEqual(fsys.written, wantFiles) {
			t.Fatalf("written = %+v, want %+v", fsys.written, wantFiles)
		}
		if len(exec.commands) != 1 || exec.commands[0].Args[0] != "build" {
			t.Fatalf("commands = %+v, want only docker build (the daemon check is the caller's job)", exec.commands)
		}
		build := exec.commands[0]
		wantArgs := []string{
			"build", "-t", "caged-runtime:latest",
			"--label", "a.role=runtime", "--label", "a.version=0.1.0",
			"--iidfile", "/tmp/caged-runtime-123/image-id",
			"-f", "/tmp/caged-runtime-123/Dockerfile", "/tmp/caged-runtime-123",
		}
		if noCache {
			wantArgs = []string{
				"build", "-t", "caged-runtime:latest",
				"--label", "a.role=runtime", "--label", "a.version=0.1.0",
				"--iidfile", "/tmp/caged-runtime-123/image-id",
				"--no-cache", "-f", "/tmp/caged-runtime-123/Dockerfile", "/tmp/caged-runtime-123",
			}
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

func TestBuildImageErrors(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")
	t.Run("temporary directory", func(t *testing.T) {
		t.Parallel()
		exec := &fakeExecutor{}
		_, err := NewClient(exec, &fakeFS{mkdirErr: boom}, linux, system.Stdio{}).BuildImage(context.Background(), testBuildSpec(false))
		if err == nil || err.Error() != "create temporary build directory: boom" || len(exec.commands) != 0 {
			t.Fatalf("BuildImage() error = %v, commands = %+v", err, exec.commands)
		}
	})
	t.Run("write uses the file label", func(t *testing.T) {
		t.Parallel()
		fsys := &fakeFS{writeErr: map[string]error{"/tmp/caged-runtime-123/caged-entrypoint.sh": boom}}
		_, err := NewClient(&fakeExecutor{}, fsys, linux, system.Stdio{}).BuildImage(context.Background(), testBuildSpec(false))
		if err == nil || err.Error() != "write build file entrypoint: boom" || len(fsys.removed) != 1 {
			t.Fatalf("BuildImage() error = %v, removed = %v", err, fsys.removed)
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
		if _, err := NewClient(exec, fsys, linux, system.Stdio{}).BuildImage(context.Background(), testBuildSpec(false)); err != boom || len(fsys.removed) != 1 {
			t.Fatalf("BuildImage() error = %v, removed = %v", err, fsys.removed)
		}
	})
	t.Run("a missing image ID file is an error", func(t *testing.T) {
		t.Parallel()
		id, err := NewClient(&fakeExecutor{}, &fakeFS{readErr: boom}, linux, system.Stdio{}).BuildImage(context.Background(), testBuildSpec(false))
		if err == nil || err.Error() != "read the new image ID: boom" || id != "" {
			t.Fatalf("BuildImage() = %q, %v", id, err)
		}
	})
	t.Run("cleanup failure is returned after a successful build", func(t *testing.T) {
		t.Parallel()
		fsys := &fakeFS{removeErr: boom}
		_, err := NewClient(&fakeExecutor{}, fsys, linux, system.Stdio{}).BuildImage(context.Background(), testBuildSpec(false))
		if err == nil || err.Error() != "remove temporary build directory: boom" || !errors.Is(err, boom) {
			t.Fatalf("BuildImage() error = %v, want the cleanup error", err)
		}
	})
	t.Run("cleanup failure does not hide a build failure", func(t *testing.T) {
		t.Parallel()
		buildErr := errors.New("build failed")
		exec := &fakeExecutor{run: func(cmd system.Command) error {
			if cmd.Args[0] == "build" {
				return buildErr
			}
			return nil
		}}
		_, err := NewClient(exec, &fakeFS{removeErr: boom}, linux, system.Stdio{}).BuildImage(context.Background(), testBuildSpec(false))
		if err != buildErr {
			t.Fatalf("BuildImage() error = %v, want the build error unchanged", err)
		}
	})
	t.Run("missing CLI creates nothing", func(t *testing.T) {
		t.Parallel()
		fsys := &fakeFS{}
		_, err := NewClient(&fakeExecutor{lookPathErr: boom}, fsys, linux, system.Stdio{}).BuildImage(context.Background(), testBuildSpec(false))
		if !engine.IsKind(err, engine.KindNotInstalled) || fsys.written != nil || fsys.removed != nil {
			t.Fatalf("BuildImage() error = %v, fs = %+v", err, fsys)
		}
	})
}

func TestReadImageFile(t *testing.T) {
	t.Parallel()

	t.Run("runs cat through an overridden entrypoint and returns stdout", func(t *testing.T) {
		t.Parallel()
		exec := &fakeExecutor{run: func(cmd system.Command) error {
			_, err := io.WriteString(cmd.Stdout, "claude=2.1.3\n")
			return err
		}}
		got, err := NewClient(exec, &fakeFS{}, linux, system.Stdio{}).ReadImageFile(context.Background(), "caged-runtime:latest", "/opt/caged/versions")
		if err != nil || string(got) != "claude=2.1.3\n" {
			t.Fatalf("ReadImageFile() = %q, %v", got, err)
		}
		want := []string{"run", "--rm", "--entrypoint", "cat", "caged-runtime:latest", "/opt/caged/versions"}
		if len(exec.commands) != 1 || exec.commands[0].Path != "/usr/local/bin/docker" || !reflect.DeepEqual(exec.commands[0].Args, want) {
			t.Fatalf("commands = %+v, want args %q", exec.commands, want)
		}
		if exec.commands[0].Stdin != nil {
			t.Fatal("ReadImageFile must not attach a terminal or stdin")
		}
	})
	t.Run("a failure names the file and the image", func(t *testing.T) {
		t.Parallel()
		exec := &fakeExecutor{run: func(cmd system.Command) error {
			if _, err := io.WriteString(cmd.Stderr, "cat: /opt/caged/versions: No such file or directory\n"); err != nil {
				return err
			}
			return errors.New("exit status 1")
		}}
		_, err := NewClient(exec, &fakeFS{}, linux, system.Stdio{}).ReadImageFile(context.Background(), "img", "/opt/caged/versions")
		want := "read /opt/caged/versions from image img: cat: /opt/caged/versions: No such file or directory: exit status 1"
		if err == nil || err.Error() != want {
			t.Fatalf("ReadImageFile() error = %v, want %q", err, want)
		}
	})
	t.Run("a missing CLI", func(t *testing.T) {
		t.Parallel()
		_, err := NewClient(&fakeExecutor{lookPathErr: errors.New("x")}, &fakeFS{}, linux, system.Stdio{}).ReadImageFile(context.Background(), "img", "/p")
		if !engine.IsKind(err, engine.KindNotInstalled) {
			t.Fatalf("ReadImageFile() error = %v", err)
		}
	})
}

func TestProbeMount(t *testing.T) {
	t.Parallel()

	mount := plan.Mount{HostPath: "/Users/me/code/app", ContainerPath: "/workspace/app"}
	t.Run("starts a do-nothing container with a read-only mount", func(t *testing.T) {
		t.Parallel()
		exec := &fakeExecutor{}
		if err := NewClient(exec, &fakeFS{}, darwin, system.Stdio{}).ProbeMount(context.Background(), "img", mount); err != nil {
			t.Fatal(err)
		}
		want := []string{"run", "--rm", "-v", "/Users/me/code/app:/workspace/app:ro", "--entrypoint", "true", "img"}
		if len(exec.commands) != 1 || !reflect.DeepEqual(exec.commands[0].Args, want) {
			t.Fatalf("commands = %+v, want args %q", exec.commands, want)
		}
		for _, arg := range exec.commands[0].Args {
			if arg == "-it" || arg == "--user" || arg == "--privileged" {
				t.Fatalf("probe must not use %q", arg)
			}
		}
	})
	failing := func(stderr string) *fakeExecutor {
		return &fakeExecutor{run: func(cmd system.Command) error {
			if _, err := io.WriteString(cmd.Stderr, stderr); err != nil {
				return err
			}
			return errors.New("exit status 125")
		}}
	}
	t.Run("macOS maps a shared-path error to the File Sharing hint", func(t *testing.T) {
		t.Parallel()
		err := NewClient(failing("docker: Error response from daemon: Mounts denied: path not shared\n"), &fakeFS{}, darwin, system.Stdio{}).ProbeMount(context.Background(), "img", mount)
		want := "Docker could not mount the project path; in Docker Desktop, add its parent directory under Settings > Resources > File Sharing: exit status 125"
		if err == nil || err.Error() != want {
			t.Fatalf("ProbeMount() error = %v, want %q", err, want)
		}
	})
	t.Run("Linux keeps docker's own message", func(t *testing.T) {
		t.Parallel()
		err := NewClient(failing("Mounts denied: path not shared\n"), &fakeFS{}, linux, system.Stdio{}).ProbeMount(context.Background(), "img", mount)
		if err == nil || err.Error() != "Mounts denied: path not shared: exit status 125" {
			t.Fatalf("ProbeMount() error = %v", err)
		}
	})
}

func TestListImages(t *testing.T) {
	t.Parallel()

	const rows = `{"ID":"sha256:aaa","Repository":"caged-runtime","Tag":"ext","Size":"412MB"}
{"ID":"sha256:bbb","Repository":"<none>","Tag":"<none>","Size":"3.0GB","Labels":"a=b,c=d"}

`
	tests := []struct {
		name  string
		query engine.ImageQuery
		want  []string
	}{
		{name: "by label", query: engine.ImageQuery{Label: "io.x.role=extension"}, want: []string{"image", "ls", "--filter", "label=io.x.role=extension", "--format", "{{json .}}"}},
		{name: "by repository", query: engine.ImageQuery{Reference: "caged-runtime"}, want: []string{"image", "ls", "caged-runtime", "--format", "{{json .}}"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			exec := &fakeExecutor{run: func(cmd system.Command) error {
				_, err := io.WriteString(cmd.Stdout, rows)
				return err
			}}
			got, err := NewClient(exec, &fakeFS{}, linux, system.Stdio{}).ListImages(context.Background(), tt.query)
			if err != nil {
				t.Fatal(err)
			}
			want := []engine.ImageSummary{
				{ID: "sha256:aaa", Repository: "caged-runtime", Tag: "ext", Size: "412MB"},
				{ID: "sha256:bbb", Repository: "<none>", Tag: "<none>", Size: "3.0GB", Labels: map[string]string{"a": "b", "c": "d"}},
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("ListImages() = %+v, want %+v", got, want)
			}
			if len(exec.commands) != 1 || !reflect.DeepEqual(exec.commands[0].Args, tt.want) {
				t.Fatalf("commands = %+v, want args %q", exec.commands, tt.want)
			}
		})
	}
	t.Run("errors", func(t *testing.T) {
		t.Parallel()
		failing := &fakeExecutor{run: func(cmd system.Command) error {
			if _, err := io.WriteString(cmd.Stderr, "boom\n"); err != nil {
				return err
			}
			return errors.New("exit status 1")
		}}
		if _, err := NewClient(failing, &fakeFS{}, linux, system.Stdio{}).ListImages(context.Background(), engine.ImageQuery{Reference: "x"}); err == nil || err.Error() != "list images: boom: exit status 1" {
			t.Fatalf("ListImages() error = %v", err)
		}
		garbage := &fakeExecutor{run: func(cmd system.Command) error {
			_, err := io.WriteString(cmd.Stdout, "not json\n")
			return err
		}}
		if _, err := NewClient(garbage, &fakeFS{}, linux, system.Stdio{}).ListImages(context.Background(), engine.ImageQuery{Reference: "x"}); err == nil || !strings.HasPrefix(err.Error(), "parse docker image ls output: ") {
			t.Fatalf("ListImages() error = %v", err)
		}
		empty := &fakeExecutor{}
		got, err := NewClient(empty, &fakeFS{}, linux, system.Stdio{}).ListImages(context.Background(), engine.ImageQuery{Reference: "x"})
		if err != nil || got != nil {
			t.Fatalf("ListImages() with no output = %+v, %v; want nil, nil", got, err)
		}
	})
}
