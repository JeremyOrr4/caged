package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/app"
	"github.com/JeremyOrr4/caged/internal/docker"
	"github.com/JeremyOrr4/caged/internal/project"
)

func TestRootCmdRejectsTooManyArguments(t *testing.T) {
	t.Parallel()

	cmd := NewRootCmd("test")
	cmd.SetArgs([]string{"claude", "prompt one", "extra"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want an error for too many arguments")
	}
	if !strings.Contains(err.Error(), "accepts at most 2 arg") {
		t.Errorf("Execute() error = %q, want a too-many-args error", err)
	}
}

func TestResolveTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		args      []string
		wantKind  string
		wantAgent string
		wantErr   string
	}{
		{name: "shell", args: nil, wantKind: "shell"},
		{name: "agent", args: []string{"claude"}, wantKind: "agent", wantAgent: "claude"},
		{name: "unsupported", args: []string{"foo"}, wantErr: "unsupported agent \"foo\""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			target, err := resolveTarget(tt.args)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("resolveTarget(%v) error = %v, want %q", tt.args, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveTarget(%v) error = %v", tt.args, err)
			}
			if target.kind != tt.wantKind {
				t.Fatalf("resolveTarget(%v) kind = %q, want %q", tt.args, target.kind, tt.wantKind)
			}
			if tt.wantAgent != "" && target.agent.Name != tt.wantAgent {
				t.Fatalf("resolveTarget(%v) agent = %q, want %q", tt.args, target.agent.Name, tt.wantAgent)
			}
		})
	}
}

func TestAgentNamesDoNotCollideWithSubcommands(t *testing.T) {
	t.Parallel()

	subcommands := map[string]struct{}{"build": {}, "list": {}}
	for _, name := range agent.Names() {
		if _, ok := subcommands[name]; ok {
			t.Errorf("agent %q collides with a root subcommand", name)
		}
	}
}

func TestNewMountGuardRails(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		cwd   string
		home  string
		allow bool
		want  string
	}{
		{name: "filesystem root", cwd: "/", home: "/Users/test", want: "filesystem root"},
		{name: "home directory", cwd: "/Users/test", home: "/Users/test", want: "entire home directory"},
		{name: "allowed home directory", cwd: "/Users/test", home: "/Users/test", allow: true},
		{name: "project directory", cwd: "/Users/test/code/caged", home: "/Users/test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mount, err := project.NewMount(tt.cwd, tt.home, tt.allow)
			if tt.want != "" {
				if err == nil || !strings.Contains(err.Error(), tt.want) {
					t.Fatalf("newMount() error = %v, want %q", err, tt.want)
				}
				return
			}
			if err != nil {
				t.Fatalf("newMount() error = %v", err)
			}
			wantPath := filepath.Join("/workspace", filepath.Base(filepath.Clean(tt.cwd)))
			if mount.ContainerPath != wantPath {
				t.Errorf("container path = %q, want %q", mount.ContainerPath, wantPath)
			}
		})
	}
}

func TestResolveProjectMount(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	projectPath := filepath.Join(t.TempDir(), "project")
	if err := os.Mkdir(projectPath, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(projectPath, "file.txt")
	if err := os.WriteFile(file, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}

	mount, err := project.ResolveMount(projectPath, home, false)
	if err != nil {
		t.Fatalf("resolveProjectMount() error = %v", err)
	}
	resolvedProject, err := filepath.EvalSymlinks(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if mount.HostPath != resolvedProject || mount.ContainerPath != "/workspace/project" {
		t.Fatalf("resolveProjectMount() = %+v, want host %q at /workspace/project", mount, resolvedProject)
	}

	if _, err := project.ResolveMount(file, home, false); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("resolveProjectMount(file) error = %v, want not-a-directory error", err)
	}

	if _, err := project.ResolveMount(home, home, false); err == nil || !strings.Contains(err.Error(), "entire home directory") {
		t.Fatalf("resolveProjectMount(home) error = %v, want broad-mount rejection", err)
	}
}

func TestNoMountAndWorkdirAreIncompatible(t *testing.T) {
	t.Parallel()

	request := app.Request{NoMount: true, WorkDir: "/tmp"}
	options := &docker.Options{}
	if err := app.ConfigureMounts(options, request); err == nil || !strings.Contains(err.Error(), "cannot be used with --no-mount") {
		t.Fatalf("ConfigureMounts() error = %v, want incompatible-flags error", err)
	}
}

func TestInvalidEnvFlagRejected(t *testing.T) {
	t.Parallel()

	cmd := NewRootCmd("test")
	cmd.SetArgs([]string{"claude", "--env", "bad-name"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "invalid env name") {
		t.Fatalf("Execute() error = %v, want invalid env name rejection", err)
	}
}

func TestAgentCommandUsesRegistryCommand(t *testing.T) {
	t.Parallel()

	codex, err := agent.Lookup("codex")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := agentCommand(codex), []string{"codex"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("agentCommand() = %v, want %v", got, want)
	}

	claude, err := agent.Lookup("claude")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := agentCommand(claude), []string{"claude"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("agentCommand() = %v, want %v", got, want)
	}
}
