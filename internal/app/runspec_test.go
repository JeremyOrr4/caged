package app

import (
	"reflect"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/plan"
)

func TestRunSpecUsesAgentPromptStyle(t *testing.T) {
	for _, tc := range []struct {
		name  string
		agent string
		want  []string
	}{
		{"claude", "claude", []string{"claude", "hello $(world)"}},
		{"codex", "codex", []string{"codex", "exec", "hello $(world)"}},
		{"gemini", "gemini", []string{"gemini", "-i", "hello $(world)"}},
		{"pi", "pi", []string{"pi", "hello $(world)"}},
		{"copilot", "copilot", []string{"copilot", "-p", "hello $(world)"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			selected, err := agent.Builtin().Lookup(tc.agent)
			if err != nil {
				t.Fatal(err)
			}
			prompt := "hello $(world)"
			spec, err := buildRunSpec(runInputs{Image: "image", Agent: &selected, Prompt: &prompt})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(spec.Command, tc.want) {
				t.Fatalf("command = %q, want %q", spec.Command, tc.want)
			}
			if !spec.ReturnToShell {
				t.Fatal("agent runs must return to the container shell")
			}
		})
	}
}

func TestRunSpecCopiesInputs(t *testing.T) {
	selected, _ := agent.Builtin().Lookup("claude")
	command := selected.Command[0]
	prompt := "prompt"
	mount := &plan.Mount{HostPath: "/project", ContainerPath: "/workspace/project"}
	configMounts := []plan.ConfigMount{{HostPath: "/home/user/.claude", DestPath: "/home/caged/.claude", Writable: true}}
	env := []string{"ANTHROPIC_API_KEY"}
	spec, err := buildRunSpec(runInputs{Image: "image", Agent: &selected, Prompt: &prompt, Mount: mount, Configs: configMounts, Env: env})
	if err != nil {
		t.Fatal(err)
	}
	spec.Command[0] = "changed"
	spec.ConfigMounts[0].Name = "changed"
	spec.EnvVars[0] = "changed"
	if selected.Command[0] != command || configMounts[0].Name == "changed" || env[0] == "changed" {
		t.Fatal("run spec mutated source inputs")
	}
}

func TestRunSpecCopiesPorts(t *testing.T) {
	t.Parallel()

	ports := []plan.Port{{HostIP: "127.0.0.1", HostPort: 8080, ContainerPort: 3000}}
	spec, err := buildRunSpec(runInputs{Image: "image", Ports: ports})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(spec.Ports, ports) {
		t.Fatalf("Ports = %+v, want %+v", spec.Ports, ports)
	}
	spec.Ports[0].HostPort = 1
	if ports[0].HostPort != 8080 {
		t.Fatal("run spec shares the ports slice with its input")
	}
}

func TestRunSpecShellAndErrors(t *testing.T) {
	t.Parallel()

	mount := &plan.Mount{HostPath: "/project", ContainerPath: "/workspace/project"}
	spec, err := buildRunSpec(runInputs{Image: "image", Mount: mount})
	if err != nil {
		t.Fatal(err)
	}
	if spec.Image != "image" || !reflect.DeepEqual(spec.Command, []string{"bash"}) || spec.ReturnToShell {
		t.Fatalf("shell spec = %+v, want bash without return-to-shell", spec)
	}
	spec.Mount.HostPath = "changed"
	if mount.HostPath != "/project" {
		t.Fatal("run spec shares the project mount with its input")
	}

	prompt := "hello"
	if _, err := buildRunSpec(runInputs{Image: "image", Prompt: &prompt}); err == nil || err.Error() != "a prompt requires an agent" {
		t.Fatalf("shell with prompt error = %v", err)
	}
	gh, err := agent.Builtin().Lookup("gh")
	if err != nil {
		t.Fatal(err)
	}
	if spec, err := buildRunSpec(runInputs{Image: "image", Agent: &gh, Prompt: &prompt}); err == nil || err.Error() != "gh does not support prompt launch" || spec.Image != "" {
		t.Fatalf("gh with prompt = %+v, %v; want an error and an empty spec", spec, err)
	}
}

func TestRunSpecAgentArgs(t *testing.T) {
	t.Parallel()

	claude, err := agent.Builtin().Lookup("claude")
	if err != nil {
		t.Fatal(err)
	}
	gh, err := agent.Builtin().Lookup("gh")
	if err != nil {
		t.Fatal(err)
	}
	prompt := "fix tests"
	tests := []struct {
		name    string
		in      runInputs
		want    []string
		wantErr string
	}{
		{name: "after the prompt", in: runInputs{Agent: &claude, Prompt: &prompt, AgentArgs: []string{"--model", "opus"}}, want: []string{"claude", "fix tests", "--model", "opus"}},
		{name: "without a prompt", in: runInputs{Agent: &claude, AgentArgs: []string{"--resume"}}, want: []string{"claude", "--resume"}},
		{name: "an agent with no prompt style", in: runInputs{Agent: &gh, AgentArgs: []string{"pr", "list"}}, want: []string{"gh", "pr", "list"}},
		{name: "none", in: runInputs{Agent: &claude}, want: []string{"claude"}},
		{name: "without an agent", in: runInputs{AgentArgs: []string{"--resume"}}, wantErr: "agent arguments after -- need an agent"},
		{name: "without an agent but empty", in: runInputs{AgentArgs: []string{}}, want: []string{"bash"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			spec, err := buildRunSpec(tt.in)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("buildRunSpec() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(spec.Command, tt.want) {
				t.Fatalf("Command = %q, %v; want %q", spec.Command, err, tt.want)
			}
		})
	}
}
