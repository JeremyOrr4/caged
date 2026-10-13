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
			selected, err := agent.Builtin().Lookup(tc.agent)
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
	selected, _ := agent.Builtin().Lookup("claude")
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

func TestBuildRunSpecShellAndErrors(t *testing.T) {
	t.Parallel()

	mount := &Mount{HostPath: "/project", ContainerPath: "/workspace/project"}
	spec, err := BuildRunSpec("image", nil, nil, mount, nil, nil)
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
	if _, err := BuildRunSpec("image", nil, &prompt, nil, nil, nil); err == nil || err.Error() != "a prompt requires an agent" {
		t.Fatalf("shell with prompt error = %v", err)
	}
	gh, err := agent.Builtin().Lookup("gh")
	if err != nil {
		t.Fatal(err)
	}
	if spec, err := BuildRunSpec("image", &gh, &prompt, nil, nil, nil); err == nil || err.Error() != "gh does not support prompt launch" || spec.Image != "" {
		t.Fatalf("gh with prompt = %+v, %v; want an error and an empty spec", spec, err)
	}
}
