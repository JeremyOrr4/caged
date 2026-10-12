package config

import (
	"reflect"
	"strings"
	"testing"
)

func TestLookup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantName  string
		wantErr   bool
		wantNames []string
	}{
		{name: "known claude", input: "claude", wantName: "claude"},
		{name: "known pi", input: "pi", wantName: "pi"},
		{name: "known gemini", input: "gemini", wantName: "gemini"},
		{name: "known codex", input: "codex", wantName: "codex"},
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
				if !strings.Contains(err.Error(), "supported agents: claude, codex, gemini, pi") {
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
	want := []string{"claude", "codex", "gemini", "pi"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("Names() = %v, want %v", names, want)
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
			agent:     Agent{EnvVars: []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY"}},
			requested: []string{"GEMINI_API_KEY", "INVALID-NAME"},
			lookup: map[string]bool{
				"ANTHROPIC_API_KEY": true,
				"GEMINI_API_KEY":    true,
			},
			want: []string{"ANTHROPIC_API_KEY", "GEMINI_API_KEY"},
		},
		{
			name:   "skip unset vars",
			agent:  Agent{EnvVars: []string{"OPENAI_API_KEY"}},
			lookup: map[string]bool{},
			want:   nil,
		},
		{
			name:      "dedupe and ignore invalid",
			agent:     Agent{EnvVars: []string{"ANTHROPIC_API_KEY"}},
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
