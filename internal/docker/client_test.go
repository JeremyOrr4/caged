package docker

import (
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/engine"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
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
			got, err := newClient(exec, &fakeFS{}, darwin, system.Stdio{}).Daemon(context.Background())
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

	daemonDown := func() *fakeExecutor {
		return failingExec("Cannot connect to the Docker daemon\n", errors.New("exit status 1"))
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
		{name: "daemon down on Linux", platform: linux, exec: daemonDown(), kind: engine.KindDaemonDown, want: "Docker is installed but the daemon is unavailable; start the Docker daemon: Cannot connect to the Docker daemon"},
		{name: "daemon down on macOS", platform: darwin, exec: daemonDown(), kind: engine.KindDaemonDown, want: "Docker is installed but the daemon is unavailable; start Docker Desktop: Cannot connect to the Docker daemon"},
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
			_, err := newClient(tt.exec, &fakeFS{}, tt.platform, system.Stdio{}).Daemon(context.Background())
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
		got, err := newClient(exec, &fakeFS{}, linux, system.Stdio{}).InspectImage(context.Background(), "caged-runtime:latest")
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
		failing := func(stderr string) *fakeExecutor { return failingExec(stderr, errors.New("exit status 1")) }
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
			_, err := newClient(tt.exec, &fakeFS{}, linux, system.Stdio{}).InspectImage(context.Background(), "img")
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

func TestRemoveImage(t *testing.T) {
	t.Parallel()

	failing := func(stderr string) *fakeExecutor { return failingExec(stderr, errors.New("exit status 1")) }
	t.Run("runs docker image rm without force", func(t *testing.T) {
		t.Parallel()
		exec := &fakeExecutor{}
		if err := newClient(exec, &fakeFS{}, linux, system.Stdio{}).RemoveImage(context.Background(), "sha256:abc"); err != nil {
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
			err := newClient(tt.exec, &fakeFS{}, linux, system.Stdio{}).RemoveImage(context.Background(), "sha256:abc")
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

func TestReadImageFile(t *testing.T) {
	t.Parallel()

	t.Run("runs cat through an overridden entrypoint and returns stdout", func(t *testing.T) {
		t.Parallel()
		exec := &fakeExecutor{run: func(cmd system.Command) error {
			_, err := io.WriteString(cmd.Stdout, "claude=2.1.3\n")
			return err
		}}
		got, err := newClient(exec, &fakeFS{}, linux, system.Stdio{}).ReadImageFile(context.Background(), "caged-runtime:latest", "/opt/caged/versions")
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
		exec := failingExec("cat: /opt/caged/versions: No such file or directory\n", errors.New("exit status 1"))
		_, err := newClient(exec, &fakeFS{}, linux, system.Stdio{}).ReadImageFile(context.Background(), "img", "/opt/caged/versions")
		want := "read /opt/caged/versions from image img: cat: /opt/caged/versions: No such file or directory: exit status 1"
		if err == nil || err.Error() != want {
			t.Fatalf("ReadImageFile() error = %v, want %q", err, want)
		}
	})
	t.Run("a missing CLI", func(t *testing.T) {
		t.Parallel()
		_, err := newClient(&fakeExecutor{lookPathErr: errors.New("x")}, &fakeFS{}, linux, system.Stdio{}).ReadImageFile(context.Background(), "img", "/p")
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
		if err := newClient(exec, &fakeFS{}, darwin, system.Stdio{}).ProbeMount(context.Background(), "img", mount); err != nil {
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
	failing := func(stderr string) *fakeExecutor { return failingExec(stderr, errors.New("exit status 125")) }
	t.Run("macOS maps a shared-path error to the File Sharing hint", func(t *testing.T) {
		t.Parallel()
		err := newClient(failing("docker: Error response from daemon: Mounts denied: path not shared\n"), &fakeFS{}, darwin, system.Stdio{}).ProbeMount(context.Background(), "img", mount)
		want := "Docker could not mount the project path; in Docker Desktop, add its parent directory under Settings > Resources > File Sharing: exit status 125"
		if err == nil || err.Error() != want {
			t.Fatalf("ProbeMount() error = %v, want %q", err, want)
		}
	})
	t.Run("Linux keeps docker's own message", func(t *testing.T) {
		t.Parallel()
		err := newClient(failing("Mounts denied: path not shared\n"), &fakeFS{}, linux, system.Stdio{}).ProbeMount(context.Background(), "img", mount)
		if err == nil || err.Error() != "Mounts denied: path not shared: exit status 125" {
			t.Fatalf("ProbeMount() error = %v", err)
		}
	})
	t.Run("a missing CLI", func(t *testing.T) {
		t.Parallel()
		exec := &fakeExecutor{lookPathErr: errors.New("x")}
		err := newClient(exec, &fakeFS{}, linux, system.Stdio{}).ProbeMount(context.Background(), "img", mount)
		if !engine.IsKind(err, engine.KindNotInstalled) || len(exec.commands) != 0 {
			t.Fatalf("ProbeMount() error = %v, commands = %+v", err, exec.commands)
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
			got, err := newClient(exec, &fakeFS{}, linux, system.Stdio{}).ListImages(context.Background(), tt.query)
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
		failing := failingExec("boom\n", errors.New("exit status 1"))
		if _, err := newClient(failing, &fakeFS{}, linux, system.Stdio{}).ListImages(context.Background(), engine.ImageQuery{Reference: "x"}); err == nil || err.Error() != "list images: boom: exit status 1" {
			t.Fatalf("ListImages() error = %v", err)
		}
		garbage := &fakeExecutor{run: func(cmd system.Command) error {
			_, err := io.WriteString(cmd.Stdout, "not json\n")
			return err
		}}
		if _, err := newClient(garbage, &fakeFS{}, linux, system.Stdio{}).ListImages(context.Background(), engine.ImageQuery{Reference: "x"}); err == nil || !strings.HasPrefix(err.Error(), "parse docker image ls output: ") {
			t.Fatalf("ListImages() error = %v", err)
		}
		tooLong := &fakeExecutor{run: func(cmd system.Command) error {
			_, err := io.WriteString(cmd.Stdout, strings.Repeat("x", 70*1024)+"\n")
			return err
		}}
		if _, err := newClient(tooLong, &fakeFS{}, linux, system.Stdio{}).ListImages(context.Background(), engine.ImageQuery{Reference: "x"}); err == nil || err.Error() != "parse docker image ls output: bufio.Scanner: token too long" {
			t.Fatalf("ListImages() with a line over 64 KiB: error = %v", err)
		}
		missing := &fakeExecutor{lookPathErr: errors.New("x")}
		if _, err := newClient(missing, &fakeFS{}, linux, system.Stdio{}).ListImages(context.Background(), engine.ImageQuery{Reference: "x"}); !engine.IsKind(err, engine.KindNotInstalled) || len(missing.commands) != 0 {
			t.Fatalf("ListImages() without the CLI: error = %v, commands = %+v", err, missing.commands)
		}
		empty := &fakeExecutor{}
		got, err := newClient(empty, &fakeFS{}, linux, system.Stdio{}).ListImages(context.Background(), engine.ImageQuery{Reference: "x"})
		if err != nil || got != nil {
			t.Fatalf("ListImages() with no output = %+v, %v; want nil, nil", got, err)
		}
	})
}
