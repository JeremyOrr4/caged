package container

import (
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
					Agent:      "claude",
					Name:       ".claude",
					HostPath:   "/home/test/.claude",
					StagedPath: "/mnt/caged-host/claude/.claude",
					DestPath:   "/home/caged/.claude",
				}},
			},
			goos: "linux",
			uid:  3001,
			gid:  3002,
			want: []string{
				"run", "-it", "--rm",
				"-v", "/home/test/.claude:/mnt/caged-host/claude/.claude:ro",
				"--user", "3001:3002",
				"-e", `CAGED_CONFIG_MAP=[{"agent":"claude","name":".claude","host":"/home/test/.claude","staged":"/mnt/caged-host/claude/.claude","dest":"/home/caged/.claude","excludes":null}]`,
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
					Agent:      "pi",
					Name:       ".pi",
					HostPath:   "/home/test/.pi",
					StagedPath: "/mnt/caged-host/pi/.pi",
					DestPath:   "/home/caged/.pi",
					Writable:   true,
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
			name: "Pi config falls back to staging when write-through is disabled",
			options: Options{
				Image: config.RuntimeImage,
				ConfigMounts: []config.ResolvedMount{{
					Agent:      "pi",
					Name:       ".pi",
					HostPath:   "/home/test/.pi",
					StagedPath: "/mnt/caged-host/pi/.pi",
					DestPath:   "/home/caged/.pi",
					Excludes:   []string{"auth.json"},
				}},
			},
			goos: "darwin",
			want: []string{
				"run", "-it", "--rm",
				"-v", "/home/test/.pi:/mnt/caged-host/pi/.pi:ro",
				"-e", `CAGED_CONFIG_MAP=[{"agent":"pi","name":".pi","host":"/home/test/.pi","staged":"/mnt/caged-host/pi/.pi","dest":"/home/caged/.pi","excludes":["auth.json"]}]`,
				config.RuntimeImage,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := buildArgs(tt.options, tt.goos, tt.uid, tt.gid)
			if err != nil {
				t.Fatalf("buildArgs() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildArgs() = %q, want %q", got, tt.want)
			}
		})
	}
}
