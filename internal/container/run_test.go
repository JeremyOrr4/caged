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
				Image:   config.RuntimeImage,
				Command: []string{"claude"},
				Mount:   &Mount{HostPath: "/project", ContainerPath: "/workspace"},
			},
			goos: "linux",
			uid:  2001,
			gid:  2002,
			want: []string{
				"run", "-it", "--rm",
				"-v", "/project:/workspace",
				"-w", "/workspace",
				"--user", "2001:2002",
				config.RuntimeImage,
				"claude",
			},
		},
		{
			name: "env names and persisted state volumes",
			options: Options{
				Image:        config.RuntimeImage,
				Command:      []string{"claude"},
				EnvVars:      []string{"ANTHROPIC_API_KEY", "GEMINI_API_KEY"},
				VolumeMounts: []VolumeMount{{Name: "caged-claude-home", ContainerPath: "/home/caged/.claude"}},
			},
			goos: "linux",
			uid:  3001,
			gid:  3002,
			want: []string{
				"run", "-it", "--rm",
				"-v", "caged-claude-home:/home/caged/.claude",
				"--user", "3001:3002",
				"-e", "ANTHROPIC_API_KEY",
				"-e", "GEMINI_API_KEY",
				config.RuntimeImage,
				"claude",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := buildArgs(tt.options, tt.goos, tt.uid, tt.gid); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildArgs() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVolumeNameSplit(t *testing.T) {
	t.Parallel()

	got := splitVolumeNames("caged-claude-home\ncaged-pi-home\nnot-a-volume\n")
	want := []string{"caged-claude-home", "caged-pi-home"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitVolumeNames() = %v, want %v", got, want)
	}
}
