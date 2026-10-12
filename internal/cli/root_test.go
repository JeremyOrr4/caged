package cli

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/config"
)

func TestRootCmdRejectsTooManyArguments(t *testing.T) {
	t.Parallel()

	cmd := NewRootCmd("test")
	cmd.SetArgs([]string{"claude", "pi"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want an error for too many arguments")
	}
	if !strings.Contains(err.Error(), "accepts at most 1 arg") {
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
	for _, name := range config.Names() {
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

			mount, err := newMount(tt.cwd, tt.home, tt.allow)
			if tt.want != "" {
				if err == nil || !strings.Contains(err.Error(), tt.want) {
					t.Fatalf("newMount() error = %v, want %q", err, tt.want)
				}
				return
			}
			if err != nil {
				t.Fatalf("newMount() error = %v", err)
			}
			if mount.ContainerPath != "/workspace" {
				t.Errorf("container path = %q, want /workspace", mount.ContainerPath)
			}
		})
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

	agent, err := config.Lookup("codex")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := agentCommand(agent), []string{"codex"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("agentCommand() = %v, want %v", got, want)
	}

	claude, err := config.Lookup("claude")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := agentCommand(claude), []string{"claude"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("agentCommand() = %v, want %v", got, want)
	}
}
