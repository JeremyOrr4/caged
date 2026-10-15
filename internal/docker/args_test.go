package docker

import (
	"reflect"
	"testing"

	"github.com/JeremyOrr4/caged/internal/images"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
)

func TestRunArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		options plan.RunSpec
		goos    string
		uid     int
		gid     int
		want    []string
	}{
		{
			name: "no mount on macOS",
			options: plan.RunSpec{
				Image:   "ubuntu:24.04",
				Command: []string{"bash"},
			},
			goos: "darwin",
			want: []string{"run", "-it", "--rm", "ubuntu:24.04", "bash"},
		},
		{
			name: "mount with spaces on macOS",
			options: plan.RunSpec{
				Image:   "ubuntu:24.04",
				Command: []string{"bash"},
				Mount: &plan.Mount{
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
			options: plan.RunSpec{
				Image: "ubuntu:24.04",
				Mount: &plan.Mount{HostPath: "/work", ContainerPath: "/workspace"},
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
			options: plan.RunSpec{
				Image:         images.BaseRef,
				Command:       []string{"claude"},
				Mount:         &plan.Mount{HostPath: "/project", ContainerPath: "/workspace"},
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
				images.BaseRef,
				"claude",
			},
		},
		{
			name: "config read-write mount and env names",
			options: plan.RunSpec{
				Image:         images.BaseRef,
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
				images.BaseRef,
				"claude",
			},
		},
		{
			name: "Pi config mounts directly read-write",
			options: plan.RunSpec{
				Image: images.BaseRef,
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
				images.BaseRef,
			},
		},
		{
			name: "config mount uses image user on macOS",
			options: plan.RunSpec{
				Image: images.BaseRef,
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
				images.BaseRef,
			},
		},
		{
			name: "Pi config is writable",
			options: plan.RunSpec{
				Image: images.BaseRef,
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
				images.BaseRef,
			},
		},
		{
			name: "a followed symlink that is not writable mounts read-only",
			options: plan.RunSpec{
				Image: images.BaseRef,
				ConfigMounts: []plan.ConfigMount{{
					Agent:    "claude",
					Name:     ".claude",
					HostPath: "/home/test/dotfiles/claude/CLAUDE.md",
					DestPath: "/home/caged/.claude/CLAUDE.md",
					Link:     "/home/test/.claude/CLAUDE.md",
					Writable: false,
				}},
			},
			goos: "linux",
			uid:  3001,
			gid:  3002,
			want: []string{
				"run", "-it", "--rm",
				"-v", "/home/test/dotfiles/claude/CLAUDE.md:/home/caged/.claude/CLAUDE.md:ro",
				"--user", "3001:3002",
				images.BaseRef,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := runArgs(tt.options, system.Platform{OS: tt.goos, UID: tt.uid, GID: tt.gid})
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("runArgs() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRunArgsPublishPortsRightAfterRm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		ports []plan.Port
		want  []string
	}{
		{name: "no ports leaves the arguments as before", want: []string{"run", "-it", "--rm", "-v", "/p:/workspace/p", "-w", "/workspace/p", "img", "bash"}},
		{
			name:  "one port",
			ports: []plan.Port{{HostIP: "127.0.0.1", HostPort: 3000, ContainerPort: 3000}},
			want:  []string{"run", "-it", "--rm", "-p", "127.0.0.1:3000:3000", "-v", "/p:/workspace/p", "-w", "/workspace/p", "img", "bash"},
		},
		{
			name:  "several ports keep their order",
			ports: []plan.Port{{HostIP: "127.0.0.1", HostPort: 8080, ContainerPort: 3000}, {HostIP: "127.0.0.1", HostPort: 5173, ContainerPort: 5173}},
			want:  []string{"run", "-it", "--rm", "-p", "127.0.0.1:8080:3000", "-p", "127.0.0.1:5173:5173", "-v", "/p:/workspace/p", "-w", "/workspace/p", "img", "bash"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			spec := plan.RunSpec{Image: "img", Command: []string{"bash"}, Ports: tt.ports, Mount: &plan.Mount{HostPath: "/p", ContainerPath: "/workspace/p"}}
			if got := runArgs(spec, system.Platform{OS: "darwin"}); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("runArgs() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRunArgsExposeOnlyPlannedMounts(t *testing.T) {
	t.Parallel()

	options := plan.RunSpec{
		Image: images.BaseRef,
		Mount: &plan.Mount{HostPath: "/projects/current", ContainerPath: "/workspace/current"},
		ConfigMounts: []plan.ConfigMount{
			{HostPath: "/home/user/.claude", DestPath: "/home/caged/.claude", Writable: true},
			{HostPath: "/home/user/.gitconfig", DestPath: "/home/caged/.gitconfig", Writable: false},
		},
		Command: []string{"bash"},
	}
	args := runArgs(options, system.Platform{OS: "darwin"})
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

func TestRunArgsMaskCodexRuntimeDirectories(t *testing.T) {
	t.Parallel()

	options := plan.RunSpec{Image: images.BaseRef, ConfigMounts: []plan.ConfigMount{{
		HostPath: "/home/user/.codex", DestPath: "/home/caged/.codex", Writable: true,
		MaskPaths: []string{"app-server-control", "ipc"},
	}}}
	args := runArgs(options, system.Platform{OS: "darwin"})
	want := []string{"--tmpfs", "/home/caged/.codex/app-server-control", "--tmpfs", "/home/caged/.codex/ipc"}
	for i := 0; i < len(want); i += 2 {
		found := false
		for j := 0; j+1 < len(args); j++ {
			if args[j] == want[i] && args[j+1] == want[i+1] {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Docker args do not contain %q %q: %q", want[i], want[i+1], args)
		}
	}
}
