package docker

import (
	"reflect"
	"testing"

	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/runtime"
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
				Image:         runtime.Image,
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
				runtime.Image,
				"claude",
			},
		},
		{
			name: "config read-write mount and env names",
			options: Options{
				Image:         runtime.Image,
				Command:       []string{"claude"},
				EnvVars:       []string{"ANTHROPIC_API_KEY", "GEMINI_API_KEY"},
				ReturnToShell: true,
				ConfigMounts: []plan.ConfigMount{{
					Agent:    "claude",
					Name:     ".claude",
					HostPath: "/home/test/.claude",
					DestPath: "/home/caged/.claude",
					Writable: true,
				}},
			},
			goos: "linux",
			uid:  3001,
			gid:  3002,
			want: []string{
				"run", "-it", "--rm",
				"-v", "/home/test/.claude:/home/caged/.claude",
				"--user", "3001:3002",
				"-e", "ANTHROPIC_API_KEY",
				"-e", "GEMINI_API_KEY",
				"-e", "CAGED_RETURN_TO_SHELL=1",
				runtime.Image,
				"claude",
			},
		},
		{
			name: "Pi config mounts directly read-write",
			options: Options{
				Image: runtime.Image,
				ConfigMounts: []plan.ConfigMount{{
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
				runtime.Image,
			},
		},
		{
			name: "config mount uses image user on macOS",
			options: Options{
				Image: runtime.Image,
				ConfigMounts: []plan.ConfigMount{{
					HostPath: "/tmp/caged-config",
					DestPath: "/home/caged/.codex",
					Writable: true,
				}},
			},
			goos: "darwin",
			uid:  501,
			gid:  20,
			want: []string{
				"run", "-it", "--rm",
				"-v", "/tmp/caged-config:/home/caged/.codex",
				runtime.Image,
			},
		},
		{
			name: "Pi config is writable",
			options: Options{
				Image: runtime.Image,
				ConfigMounts: []plan.ConfigMount{{
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
				runtime.Image,
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

func TestBuildArgsExposeOnlyPlannedMounts(t *testing.T) {
	t.Parallel()

	options := Options{
		Image: runtime.Image,
		Mount: &Mount{HostPath: "/projects/current", ContainerPath: "/workspace/current"},
		ConfigMounts: []plan.ConfigMount{
			{HostPath: "/home/user/.claude", DestPath: "/home/caged/.claude", Writable: true},
			{HostPath: "/home/user/.gitconfig", DestPath: "/home/caged/.gitconfig", Writable: false},
		},
		Command: []string{"bash"},
	}
	args := buildArgs(options, "darwin", 0, 0)
	volumeCount := 0
	for _, arg := range args {
		if arg == "-v" {
			volumeCount++
		}
		if arg == "--privileged" || arg == "/var/run/docker.sock" {
			t.Fatalf("unexpected privileged host access in Docker args: %q", arg)
		}
	}
	if want := 1 + len(options.ConfigMounts); volumeCount != want {
		t.Fatalf("Docker args contain %d volume mounts, want only the %d planned mounts: %q", volumeCount, want, args)
	}
}
