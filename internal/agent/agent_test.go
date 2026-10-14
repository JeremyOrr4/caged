package agent

import (
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestLookup(t *testing.T) {
	t.Parallel()

	// An explicit registry keeps the exact-list assertions below independent
	// of agents added to Builtin later.
	registry, err := NewRegistry(claude(), codex(), copilot(), gemini(), gh(), pi())
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		input    string
		wantName string
		wantErr  bool
	}{
		{name: "known claude", input: "claude", wantName: "claude"},
		{name: "known pi", input: "pi", wantName: "pi"},
		{name: "known gemini", input: "gemini", wantName: "gemini"},
		{name: "known codex", input: "codex", wantName: "codex"},
		{name: "known copilot", input: "copilot", wantName: "copilot"},
		{name: "unknown", input: "foo", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			agent, err := registry.Lookup(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("Lookup() error = nil, want an error")
				}
				if !strings.Contains(err.Error(), "supported agents: claude, codex, copilot, gemini, gh, pi") {
					t.Fatalf("Lookup() error = %q, want supported agents list", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Lookup(%q) error = %v", tt.input, err)
			}
			if agent.Name != tt.wantName {
				t.Fatalf("Lookup(%q) name = %q, want %q", tt.input, agent.Name, tt.wantName)
			}
		})
	}

	names := registry.Names()
	want := []string{"claude", "codex", "copilot", "gemini", "gh", "pi"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("Names() = %v, want %v", names, want)
	}
}

func TestClaudeConfigUsesDirectoryInsteadOfSingleFileMount(t *testing.T) {
	t.Parallel()

	claude, err := Builtin().Lookup("claude")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range claude.ConfigPaths {
		if path.HostPath == ".claude.json" {
			t.Fatal("Claude config still uses a single-file bind mount")
		}
	}
}

func TestCodexConfigUsesDirectoryMountAndMasksRuntimePaths(t *testing.T) {
	t.Parallel()
	codex, err := Builtin().Lookup("codex")
	if err != nil {
		t.Fatal(err)
	}
	if len(codex.ConfigPaths) != 1 || codex.ConfigPaths[0].HostPath != ".codex" || codex.ConfigPaths[0].ContainerPath != ".codex" {
		t.Fatalf("Codex config paths = %+v, want one .codex directory mount", codex.ConfigPaths)
	}
	wantMasks := []string{"app-server-control", "app-server-daemon", "ipc", "packages/app-server-daemon", "tmp/arg0"}
	if !reflect.DeepEqual(codex.ConfigPaths[0].MaskPaths, wantMasks) {
		t.Fatalf("Codex runtime masks = %v, want %v", codex.ConfigPaths[0].MaskPaths, wantMasks)
	}
}

func TestGitHubCLIUsesHostConfigAndTokens(t *testing.T) {
	t.Parallel()
	gh, err := Builtin().Lookup("gh")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gh.Command, []string{"gh"}) {
		t.Fatalf("GitHub CLI command = %v, want [gh]", gh.Command)
	}
	if len(gh.ConfigPaths) != 1 || gh.ConfigPaths[0].HostPath != ".config/gh" || gh.ConfigPaths[0].ContainerPath != ".config/gh" || !gh.ConfigPaths[0].Writable {
		t.Fatalf("GitHub CLI config paths = %+v", gh.ConfigPaths)
	}
	if !reflect.DeepEqual(gh.Auth.Environment, []string{"GH_TOKEN", "GITHUB_TOKEN"}) {
		t.Fatalf("GitHub CLI token variables = %v", gh.Auth.Environment)
	}
}

func TestResolveEnvVars(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		agent     Agent
		requested []string
		lookup    map[string]bool
		want      []string
	}{
		{
			name:      "set allowlisted vars and added names",
			agent:     Agent{Auth: AuthStrategy{Environment: []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY"}}},
			requested: []string{"GEMINI_API_KEY", "INVALID-NAME"},
			lookup: map[string]bool{
				"ANTHROPIC_API_KEY": true,
				"GEMINI_API_KEY":    true,
			},
			want: []string{"ANTHROPIC_API_KEY", "GEMINI_API_KEY"},
		},
		{
			name:   "skip unset vars",
			agent:  Agent{Auth: AuthStrategy{Environment: []string{"OPENAI_API_KEY"}}},
			lookup: map[string]bool{},
			want:   nil,
		},
		{
			name:      "dedupe and ignore invalid",
			agent:     Agent{Auth: AuthStrategy{Environment: []string{"ANTHROPIC_API_KEY"}}},
			requested: []string{"ANTHROPIC_API_KEY", "MY_VAR", "bad-name"},
			lookup: map[string]bool{
				"ANTHROPIC_API_KEY": true,
				"MY_VAR":            true,
			},
			want: []string{"ANTHROPIC_API_KEY", "MY_VAR"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			lookup := func(name string) bool { return tt.lookup[name] }
			got := tt.agent.ResolveEnv(tt.requested, lookup)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ResolveEnv() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLaunchCommandWithoutPromptUsesRegistryCommand(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"codex", "claude"} {
		definition, err := Builtin().Lookup(name)
		if err != nil {
			t.Fatal(err)
		}
		got, err := definition.LaunchCommand(nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{name}; !reflect.DeepEqual(got, want) {
			t.Fatalf("LaunchCommand(nil) = %v, want %v", got, want)
		}
		got[0] = "changed"
		if definition.Command[0] != name {
			t.Fatal("LaunchCommand() returned the registry's own slice")
		}
	}
}

func TestLaunchCommandPromptStyles(t *testing.T) {
	t.Parallel()

	prompt := "fix $(this) verbatim"
	tests := []struct {
		name    string
		agent   Agent
		want    []string
		wantErr string
	}{
		{name: "positional", agent: Agent{Name: "a", Command: []string{"a"}, Prompt: PromptPositional}, want: []string{"a", prompt}},
		{name: "flag", agent: Agent{Name: "b", Command: []string{"b", "run"}, Prompt: PromptFlag, PromptFlag: "-p"}, want: []string{"b", "run", "-p", prompt}},
		{name: "flag missing", agent: Agent{Name: "c", Command: []string{"c"}, Prompt: PromptFlag}, wantErr: "c prompt flag is not configured"},
		{name: "unsupported", agent: Agent{Name: "d", Command: []string{"d"}}, wantErr: "d does not support prompt launch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := tt.agent.LaunchCommand(&prompt, nil)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("LaunchCommand() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("LaunchCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidEnvNameMatchesDocumentedPattern(t *testing.T) {
	t.Parallel()

	pattern := regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	for _, name := range []string{
		"", "A", "_", "a1", "1a", "ANTHROPIC_API_KEY", "bad-name", "has space", "trailing\n",
		"é", "A\x00", "Z9_z", "9", "__", "a.b", "\xff",
	} {
		if got, want := validEnvName(name), pattern.MatchString(name); got != want {
			t.Errorf("validEnvName(%q) = %v, want %v", name, got, want)
		}
	}
	if err := ValidateEnvName("  PADDED  "); err != nil {
		t.Errorf("ValidateEnvName() rejected a name with surrounding spaces: %v", err)
	}
	if err := ValidateEnvName("bad-name"); err == nil || err.Error() != `invalid env name "bad-name"; must match ^[A-Za-z_][A-Za-z0-9_]*$` {
		t.Errorf("ValidateEnvName(bad-name) error = %v", err)
	}
}

func TestLaunchCommandAppendsExtraArgsAfterThePrompt(t *testing.T) {
	t.Parallel()

	prompt := "fix tests"
	extra := []string{"--model", "opus", "--add-dir", "../lib"}
	tests := []struct {
		name   string
		agent  Agent
		prompt *string
		want   []string
	}{
		{name: "positional with prompt", agent: Agent{Name: "a", Command: []string{"a"}, Prompt: PromptPositional}, prompt: &prompt, want: []string{"a", "fix tests", "--model", "opus", "--add-dir", "../lib"}},
		{name: "positional without prompt", agent: Agent{Name: "a", Command: []string{"a"}, Prompt: PromptPositional}, want: []string{"a", "--model", "opus", "--add-dir", "../lib"}},
		{name: "flag style with prompt", agent: Agent{Name: "b", Command: []string{"b", "run"}, Prompt: PromptFlag, PromptFlag: "-p"}, prompt: &prompt, want: []string{"b", "run", "-p", "fix tests", "--model", "opus", "--add-dir", "../lib"}},
		{name: "flag style without prompt", agent: Agent{Name: "b", Command: []string{"b", "run"}, Prompt: PromptFlag, PromptFlag: "-p"}, want: []string{"b", "run", "--model", "opus", "--add-dir", "../lib"}},
		{name: "an agent without a prompt style still takes extra args", agent: Agent{Name: "gh", Command: []string{"gh"}}, want: []string{"gh", "--model", "opus", "--add-dir", "../lib"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			args := append([]string(nil), extra...)
			got, err := tt.agent.LaunchCommand(tt.prompt, args)
			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("LaunchCommand() = %q, %v; want %q", got, err, tt.want)
			}
			got[len(got)-1] = "changed"
			if args[len(args)-1] != "../lib" {
				t.Fatal("LaunchCommand() shares its result with the extra args")
			}
		})
	}
	t.Run("no prompt style still rejects a prompt, with or without extra args", func(t *testing.T) {
		t.Parallel()
		gh := Agent{Name: "gh", Command: []string{"gh"}}
		if _, err := gh.LaunchCommand(&prompt, extra); err == nil || err.Error() != "gh does not support prompt launch" {
			t.Fatalf("LaunchCommand() error = %v", err)
		}
	})
	t.Run("extra args are kept verbatim", func(t *testing.T) {
		t.Parallel()
		odd := []string{"$(id)", `"q"`, "", "--", "-p"}
		got, err := Agent{Name: "a", Command: []string{"a"}}.LaunchCommand(nil, odd)
		if want := append([]string{"a"}, odd...); err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("LaunchCommand() = %q, %v; want %q", got, err, want)
		}
	})
}
