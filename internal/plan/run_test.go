package plan

import (
	"reflect"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
)

func TestBuildRunSpecUsesAgentPromptStyle(t *testing.T) {
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
			selected, err := agent.Lookup(tc.agent)
			if err != nil {
				t.Fatal(err)
			}
			prompt := "hello $(world)"
			spec, err := BuildRunSpec("image", &selected, &prompt, nil, nil, nil)
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

func TestBuildRunSpecCopiesInputs(t *testing.T) {
	selected, _ := agent.Lookup("claude")
	command := selected.Command[0]
	prompt := "prompt"
	mount := &Mount{HostPath: "/project", ContainerPath: "/workspace/project"}
	configMounts := []ConfigMount{{HostPath: "/home/user/.claude", DestPath: "/home/caged/.claude", Writable: true}}
	env := []string{"ANTHROPIC_API_KEY"}
	spec, err := BuildRunSpec("image", &selected, &prompt, mount, configMounts, env)
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
