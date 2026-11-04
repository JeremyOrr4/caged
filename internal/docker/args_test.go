package docker

import (
	"reflect"
	"strings"
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
			goos: "linux",
			uid:  1001,
			gid:  1002,
			want: []string{
				"run", "-it", "--rm",
				"-v", "/home/test/.pi:/home/caged/.pi",
				"--user", "1001:1002",
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
			got := runArgs(tt.options, system.Platform{OS: tt.goos, UID: tt.uid, GID: tt.gid}, "")
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
			if got := runArgs(spec, system.Platform{OS: "darwin"}, ""); !reflect.DeepEqual(got, tt.want) {
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
	args := runArgs(options, system.Platform{OS: "darwin"}, "")
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

func TestRunArgsMaskSymlinksBeforeMountingTheirTargets(t *testing.T) {
	t.Parallel()

	options := plan.RunSpec{Image: images.BaseRef, ConfigMounts: []plan.ConfigMount{
		{HostPath: "/host/.pi", DestPath: "/home/caged/.pi", Writable: true, MaskPaths: []string{"agent/extensions-link"}, MaskFiles: []string{"agent/settings.json"}},
		{HostPath: "/host/target", DestPath: "/home/caged/.pi/agent/extensions-link", Writable: true},
	}}
	args := runArgs(options, system.Platform{OS: "linux"}, "/tmp/empty-mask")
	want := []string{
		"-v", "/host/.pi:/home/caged/.pi",
		"--tmpfs", "/home/caged/.pi/agent/extensions-link",
		"-v", "/tmp/empty-mask:/home/caged/.pi/agent/settings.json:ro",
		"-v", "/host/target:/home/caged/.pi/agent/extensions-link",
	}
	last := -1
	for i := 0; i < len(want); {
		found := -1
		for j := last + 1; j < len(args)-1; j++ {
			if args[j] == want[i] && args[j+1] == want[i+1] {
				found = j
				break
			}
		}
		if found < 0 {
			t.Fatalf("Docker args do not contain ordered pair %q %q: %q", want[i], want[i+1], args)
		}
		last = found + 1
		i += 2
	}
}

func TestRunArgsMaskCodexRuntimeDirectories(t *testing.T) {
	t.Parallel()

	options := plan.RunSpec{Image: images.BaseRef, ConfigMounts: []plan.ConfigMount{{
		HostPath: "/home/user/.codex", DestPath: "/home/caged/.codex", Writable: true,
		MaskPaths: []string{"app-server-control", "ipc"},
	}}}
	args := runArgs(options, system.Platform{OS: "darwin"}, "")
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

func TestRunArgsFullOrder(t *testing.T) {
	t.Parallel()

	spec := plan.RunSpec{
		Image:   "img",
		Command: []string{"claude", "-p", "x"},
		Ports:   []plan.Port{{HostIP: "127.0.0.1", HostPort: 8080, ContainerPort: 3000}, {HostIP: "127.0.0.1", HostPort: 5173, ContainerPort: 5173}},
		Mount:   &plan.Mount{HostPath: "/p", ContainerPath: "/workspace/p"},
		ConfigMounts: []plan.ConfigMount{
			{HostPath: "/h/.pi", DestPath: "/home/caged/.pi/", Writable: true, MaskPaths: []string{"cache", "/abs"}, MaskFiles: []string{"agent/s.json", "/sock"}},
			{HostPath: "/h/.gitconfig", DestPath: "/home/caged/.gitconfig"},
		},
		EnvVars:       []string{"A", "B"},
		ReturnToShell: true,
	}
	middle := []string{
		"-p", "127.0.0.1:8080:3000", "-p", "127.0.0.1:5173:5173",
		"-v", "/p:/workspace/p", "-w", "/workspace/p",
		"-v", "/h/.pi:/home/caged/.pi/",
		"--tmpfs", "/home/caged/.pi/cache", "--tmpfs", "/home/caged/.pi/abs",
		"-v", "MASK:/home/caged/.pi/agent/s.json:ro", "-v", "MASK:/home/caged/.pi/sock:ro",
		"-v", "/h/.gitconfig:/home/caged/.gitconfig:ro",
	}
	tail := []string{"-e", "A", "-e", "B", "-e", "CAGED_RETURN_TO_SHELL=1", "img", "claude", "-p", "x"}
	build := func(mask string, user ...string) []string {
		args := []string{"run", "-it", "--rm"}
		for _, arg := range middle {
			args = append(args, strings.ReplaceAll(arg, "MASK", mask))
		}
		return append(append(args, user...), tail...)
	}
	tests := []struct {
		name           string
		platform       system.Platform
		fileMaskSource string
		wantMask       string
		user           []string
	}{
		{"linux", system.Platform{OS: "linux", UID: 1000, GID: 1001}, "/tmp/m/empty-file", "/tmp/m/empty-file", []string{"--user", "1000:1001"}},
		{"macOS", system.Platform{OS: "darwin", UID: 501, GID: 20}, "/tmp/m/empty-file", "/tmp/m/empty-file", nil},
		{"no mask file means /dev/null", system.Platform{OS: "darwin"}, "", "/dev/null", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			want := build(tt.wantMask, tt.user...)
			if got := runArgs(spec, tt.platform, tt.fileMaskSource); !reflect.DeepEqual(got, want) {
				t.Fatalf("runArgs() =\n%q\nwant\n%q", got, want)
			}
		})
	}
}
