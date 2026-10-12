package container

import (
	"net"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/JeremyOrr4/caged/internal/config"
)

func TestBuildArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		options Options
		goos    string
		uid     int
		gid     int
		want    []string
	}{
		{
			name: "no mount on macOS",
			options: Options{
				Image:   "ubuntu:24.04",
				Command: []string{"bash"},
			},
			goos: "darwin",
			want: []string{"run", "-it", "--rm", "ubuntu:24.04", "bash"},
		},
		{
			name: "mount with spaces on macOS",
			options: Options{
				Image:   "ubuntu:24.04",
				Command: []string{"bash"},
				Mount: &Mount{
					HostPath:      "/tmp/project with spaces",
					ContainerPath: "/workspace",
				},
			},
			goos: "darwin",
			want: []string{
				"run", "-it", "--rm",
				"-v", "/tmp/project with spaces:/workspace",
				"-w", "/workspace",
				"ubuntu:24.04", "bash",
			},
		},
		{
			name: "mount on Linux maps host user",
			options: Options{
				Image: "ubuntu:24.04",
				Mount: &Mount{HostPath: "/work", ContainerPath: "/workspace"},
			},
			goos: "linux",
			uid:  1001,
			gid:  1002,
			want: []string{
				"run", "-it", "--rm",
				"-v", "/work:/workspace",
				"-w", "/workspace",
				"--user", "1001:1002",
				"ubuntu:24.04",
			},
		},
		{
			name: "agent command in runtime image",
			options: Options{
				Image:         config.RuntimeImage,
				Command:       []string{"claude"},
				Mount:         &Mount{HostPath: "/project", ContainerPath: "/workspace"},
				ReturnToShell: true,
			},
			goos: "linux",
			uid:  2001,
			gid:  2002,
			want: []string{
				"run", "-it", "--rm",
				"-v", "/project:/workspace",
				"-w", "/workspace",
				"--user", "2001:2002",
				"-e", "CAGED_RETURN_TO_SHELL=1",
				config.RuntimeImage,
				"claude",
			},
		},
		{
			name: "config read-only mount and env names",
			options: Options{
				Image:         config.RuntimeImage,
				Command:       []string{"claude"},
				EnvVars:       []string{"ANTHROPIC_API_KEY", "GEMINI_API_KEY"},
				ReturnToShell: true,
				ConfigMounts: []config.ResolvedMount{{
					Agent:    "claude",
					Name:     ".claude",
					HostPath: "/home/test/.claude",
					DestPath: "/home/caged/.claude",
				}},
			},
			goos: "linux",
			uid:  3001,
			gid:  3002,
			want: []string{
				"run", "-it", "--rm",
				"-v", "/home/test/.claude:/home/caged/.claude:ro",
				"--user", "3001:3002",
				"-e", "ANTHROPIC_API_KEY",
				"-e", "GEMINI_API_KEY",
				"-e", "CAGED_RETURN_TO_SHELL=1",
				config.RuntimeImage,
				"claude",
			},
		},
		{
			name: "Pi config mounts directly read-write",
			options: Options{
				Image: config.RuntimeImage,
				ConfigMounts: []config.ResolvedMount{{
					Agent:    "pi",
					Name:     ".pi",
					HostPath: "/home/test/.pi",
					DestPath: "/home/caged/.pi",
					Writable: true,
				}},
			},
			goos: "darwin",
			uid:  501,
			gid:  20,
			want: []string{
				"run", "-it", "--rm",
				"-v", "/home/test/.pi:/home/caged/.pi",
				"--user", "501:20",
				config.RuntimeImage,
			},
		},
		{
			name: "read-only config staging maps host user on macOS",
			options: Options{
				Image: config.RuntimeImage,
				ConfigMounts: []config.ResolvedMount{{
					HostPath: "/tmp/caged-config",
					DestPath: "/home/caged/.codex",
				}},
			},
			goos: "darwin",
			uid:  501,
			gid:  20,
			want: []string{
				"run", "-it", "--rm",
				"-v", "/tmp/caged-config:/home/caged/.codex:ro",
				"--user", "501:20",
				config.RuntimeImage,
			},
		},
		{
			name: "Pi config falls back to staging when write-through is disabled",
			options: Options{
				Image: config.RuntimeImage,
				ConfigMounts: []config.ResolvedMount{{
					Agent:    "pi",
					Name:     ".pi",
					HostPath: "/home/test/.pi",
					DestPath: "/home/caged/.pi",
					Excludes: []string{"auth.json"},
				}},
			},
			goos: "darwin",
			uid:  501,
			gid:  20,
			want: []string{
				"run", "-it", "--rm",
				"-v", "/home/test/.pi:/home/caged/.pi:ro",
				"--user", "501:20",
				config.RuntimeImage,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := buildArgs(tt.options, tt.goos, tt.uid, tt.gid)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildArgs() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStageConfigsCopiesFilesAndSkipsSpecialAndExcludedFiles(t *testing.T) {
	t.Parallel()

	source, err := os.MkdirTemp("/tmp", "caged-config-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(source)
	if err := os.WriteFile(filepath.Join(source, "config.toml"), []byte("config"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "auth.json"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("config.toml", filepath.Join(source, "config-link")); err != nil {
		t.Fatal(err)
	}
	socket, err := net.Listen("unix", filepath.Join(source, "ipc.sock"))
	if err != nil {
		t.Fatal(err)
	}
	defer socket.Close()

	mounts, cleanup, err := stageConfigs([]config.ResolvedMount{{
		HostPath: source,
		DestPath: "/home/caged/.codex",
		Excludes: []string{"auth.json"},
	}})
	if err != nil {
		t.Fatalf("stageConfigs() error = %v", err)
	}
	defer cleanup()

	staged := mounts[0].HostPath
	if got, err := os.ReadFile(filepath.Join(staged, "config.toml")); err != nil || string(got) != "config" {
		t.Fatalf("staged config = %q, error = %v", got, err)
	}
	for _, name := range []string{"auth.json", "ipc.sock"} {
		if _, err := os.Lstat(filepath.Join(staged, name)); !os.IsNotExist(err) {
			t.Errorf("%s was copied into staging: error = %v", name, err)
		}
	}
	if got, err := os.Readlink(filepath.Join(staged, "config-link")); err != nil || got != "config.toml" {
		t.Errorf("staged symlink = %q, error = %v", got, err)
	}
}
