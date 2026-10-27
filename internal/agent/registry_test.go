package agent

import (
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestBuiltinIncludesShippedAgents(t *testing.T) {
	t.Parallel()

	registry := Builtin()
	for _, name := range []string{"claude", "codex", "copilot", "gemini", "gh", "pi"} {
		if _, err := registry.Lookup(name); err != nil {
			t.Errorf("Builtin() is missing %q: %v", name, err)
		}
	}
}

// TestBuiltinRegistryContract checks rules every agent must follow, so a new
// agent file is validated without editing any test.
func TestBuiltinRegistryContract(t *testing.T) {
	t.Parallel()

	registry := Builtin()
	names := registry.Names()
	if !slices.IsSorted(names) {
		t.Fatalf("Names() = %v, want sorted", names)
	}
	all := registry.All()
	if len(all) != len(names) {
		t.Fatalf("All() returned %d agents, Names() %d", len(all), len(names))
	}
	for i, definition := range all {
		if definition.Name != names[i] {
			t.Errorf("All()[%d] = %q, want %q", i, definition.Name, names[i])
		}
		if definition.Description == "" {
			t.Errorf("%s: missing description", definition.Name)
		}
		if definition.Prompt == PromptFlag && definition.PromptFlag == "" {
			t.Errorf("%s: PromptFlag style without a flag", definition.Name)
		}
		for _, name := range definition.Auth.Environment {
			if err := ValidateEnvName(name); err != nil {
				t.Errorf("%s: %v", definition.Name, err)
			}
		}
		for _, path := range definition.ConfigPaths {
			for _, rel := range []string{path.HostPath, path.ContainerPath} {
				if rel == "" || filepath.IsAbs(rel) || filepath.Clean(rel) != rel || strings.HasPrefix(rel, "..") {
					t.Errorf("%s: config path %q must be a clean path relative to the home directory", definition.Name, rel)
				}
			}
			if path.EnvOverride != "" {
				if err := ValidateEnvName(path.EnvOverride); err != nil {
					t.Errorf("%s: %v", definition.Name, err)
				}
			}
		}
	}
}

func TestNewRegistryRejectsInvalidDefinitions(t *testing.T) {
	t.Parallel()

	valid := Agent{Name: "valid", Command: []string{"valid"}}
	tests := []struct {
		name   string
		agents []Agent
		want   string
	}{
		{name: "empty name", agents: []Agent{{Command: []string{"x"}}}, want: `agent name "" must be lowercase`},
		{name: "uppercase name", agents: []Agent{{Name: "Aider", Command: []string{"aider"}}}, want: `agent name "Aider" must be lowercase`},
		{name: "padded name", agents: []Agent{{Name: " aider", Command: []string{"aider"}}}, want: `agent name " aider" must be lowercase`},
		{name: "no command", agents: []Agent{{Name: "aider"}}, want: `agent "aider" has no command`},
		{name: "duplicate", agents: []Agent{valid, valid}, want: `agent "valid" is registered more than once`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewRegistry(tt.agents...); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("NewRegistry() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestRegistryLookupNormalizesInput(t *testing.T) {
	t.Parallel()

	got, err := Builtin().Lookup("  Claude ")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "claude" {
		t.Fatalf("Lookup() = %q, want claude", got.Name)
	}
	if _, err := Builtin().Lookup("  Nope "); err == nil || !strings.HasPrefix(err.Error(), `unsupported agent "  Nope "; supported agents: `) {
		t.Fatalf("Lookup() error = %v, want the original input quoted", err)
	}
}

func TestRegistryReturnsCopies(t *testing.T) {
	t.Parallel()

	registry := Builtin()
	names := registry.Names()
	names[0] = "changed"
	first, err := registry.Lookup("codex")
	if err != nil {
		t.Fatal(err)
	}
	first.Command[0] = "changed"
	first.ConfigPaths[0].MaskPaths[0] = "changed"
	first.Auth.Environment[0] = "changed"
	all := registry.All()
	all[0].Command[0] = "changed"

	if registry.Names()[0] == "changed" {
		t.Error("Names() exposed the registry's slice")
	}
	second, err := registry.Lookup("codex")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(second, codex()) {
		t.Errorf("Lookup() exposed registry internals; got %+v", second)
	}
	if again := registry.All(); again[0].Command[0] == "changed" {
		t.Error("All() exposed registry internals")
	}
}

// TestNewAgentNeedsOnlyItsDefinition shows the open/closed property: a new
// agent is usable through the registry without changes anywhere else. The
// name is test-only so the test keeps passing when real agents are added.
func TestNewAgentNeedsOnlyItsDefinition(t *testing.T) {
	t.Parallel()

	extra := Agent{
		Name:        "test-only-agent",
		Command:     []string{"test-only-agent"},
		Description: "Test-only agent",
		ConfigPaths: []ConfigPath{{HostPath: ".test-only-agent", ContainerPath: ".test-only-agent", Writable: true}},
		Prompt:      PromptFlag,
		PromptFlag:  "--message",
		Auth:        AuthStrategy{Environment: []string{"OPENAI_API_KEY"}},
	}
	registry, err := NewRegistry(append(Builtin().All(), extra)...)
	if err != nil {
		t.Fatal(err)
	}
	got, err := registry.Lookup("test-only-agent")
	if err != nil {
		t.Fatal(err)
	}
	prompt := "add tests"
	command, err := got.LaunchCommand(&prompt, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"test-only-agent", "--message", "add tests"}; !reflect.DeepEqual(command, want) {
		t.Fatalf("LaunchCommand() = %q, want %q", command, want)
	}
	if names := registry.Names(); len(names) != len(Builtin().Names())+1 || !slices.IsSorted(names) {
		t.Fatalf("Names() = %v, want every built-in plus the new agent, sorted", names)
	}
}

// TestRegistrySortsNamesWhateverTheOrderGiven pins that Names, All and the
// lookup error list the agents sorted, not in registration order.
func TestRegistrySortsNamesWhateverTheOrderGiven(t *testing.T) {
	t.Parallel()

	registry, err := NewRegistry(Agent{Name: "mid", Command: []string{"mid"}}, Agent{Name: "zed", Command: []string{"zed"}}, Agent{Name: "abc", Command: []string{"abc"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := registry.Names(); !slices.Equal(got, []string{"abc", "mid", "zed"}) {
		t.Errorf("Names() = %q", got)
	}
	if all := registry.All(); len(all) != 3 || all[0].Name != "abc" || all[1].Name != "mid" || all[2].Name != "zed" {
		t.Errorf("All() = %+v", all)
	}
	if _, err := registry.Lookup("nope"); err == nil || !strings.HasSuffix(err.Error(), "supported agents: abc, mid, zed") {
		t.Errorf("Lookup error = %v", err)
	}
}
