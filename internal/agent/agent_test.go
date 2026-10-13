package agent

import (
	"reflect"
	"strings"
	"testing"
)

func TestLookup(t *testing.T) {
	t.Parallel()

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

			agent, err := Lookup(tt.input)
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

	names := Names()
	want := []string{"claude", "codex", "copilot", "gemini", "gh", "pi"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("Names() = %v, want %v", names, want)
	}
}

func TestClaudeConfigUsesDirectoryInsteadOfSingleFileMount(t *testing.T) {
	t.Parallel()

	claude, err := Lookup("claude")
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
	codex, err := Lookup("codex")
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
	gh, err := Lookup("gh")
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
			got := ResolveEnvVars(tt.agent, tt.requested, lookup)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ResolveEnvVars() = %v, want %v", got, tt.want)
			}
		})
	}
}
