package config

import (
	"os"
	"path/filepath"
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

func TestResolveAllConfigsChecksHostPathsAndAuth(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	for _, dir := range []string{".claude", ".codex", ".gemini", ".pi/agent", ".config/pi"} {
		if err := os.MkdirAll(filepath.Join(home, dir), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range []string{
		".claude/settings.json",
		".claude/.credentials.json",
		".claude.json",
		".codex/config.toml",
		".codex/auth.json",
		".gemini/settings.json",
		".gemini/oauth_creds.json",
		".pi/agent/settings.json",
		".pi/agent/auth.json",
		".gitconfig",
	} {
		if err := os.WriteFile(filepath.Join(home, file), []byte("test"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	exists := func(path string) bool {
		_, err := os.Stat(path)
		return err == nil
	}

	report := ResolveAllConfigsReport(home, exists, false)
	defaultConfigs := report.Mounts
	defaultNames := make(map[string]struct{}, len(defaultConfigs))
	writableNames := make(map[string]string)
	for _, mount := range defaultConfigs {
		defaultNames[mount.Name] = struct{}{}
		if mount.Writable {
			writableNames[mount.Name] = mount.Agent
		}
	}
	for _, want := range []string{".claude", ".claude.json", ".codex", ".gemini", ".pi", ".gitconfig"} {
		if _, ok := defaultNames[want]; !ok {
			t.Errorf("default staged configs missing %q; got %v", want, defaultNames)
		}
	}
	if len(report.Excluded) != 0 {
		t.Errorf("default config report excludes credentials: %v", report.Excluded)
	}
	for _, name := range []string{".pi", ".config/pi"} {
		if got := writableNames[name]; got != "pi" {
			t.Errorf("config %q writable agent = %q, want pi", name, got)
		}
	}
	for _, mount := range defaultConfigs {
		if mount.Agent != "pi" && mount.Writable {
			t.Errorf("non-Pi config %q is writable", mount.Name)
		}
	}

	noAuthReport := ResolveAllConfigsReport(home, exists, true)
	for _, want := range []string{
		filepath.Join(home, ".claude", ".credentials.json"),
		filepath.Join(home, ".codex", "auth.json"),
		filepath.Join(home, ".gemini", "oauth_creds.json"),
		filepath.Join(home, ".pi", "auth.json"),
	} {
		if !containsString(noAuthReport.Excluded, want) {
			t.Errorf("--no-auth report does not list excluded credential path %q: %v", want, noAuthReport.Excluded)
		}
	}
	for _, mount := range noAuthReport.Mounts {
		if mount.Name == ".claude.json" {
			t.Error("--no-auth still stages .claude.json")
		}
		if mount.Agent == "pi" && mount.Writable {
			t.Errorf("--no-auth left Pi config %q writable", mount.Name)
		}
	}

	missingHome := t.TempDir()
	if got := ResolveAllConfigs(missingHome, exists, false); len(got) != 0 {
		t.Errorf("ResolveAllConfigs() with no local configs = %v, want none", got)
	}
}

func containsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}
