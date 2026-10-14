package app

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/plan"
)

func listerAgents() []agent.Agent {
	return []agent.Agent{
		{Name: "claude", Description: "Claude Code", Auth: agent.AuthStrategy{Environment: []string{"ANTHROPIC_API_KEY"}}},
		{Name: "codex", Description: "Codex CLI", Auth: agent.AuthStrategy{Environment: []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY"}}},
		{Name: "gemini", Description: "Gemini CLI"},
		{Name: "pi", Description: "Pi coding agent"},
	}
}

func TestListerList(t *testing.T) {
	t.Parallel()

	const home = "/Users/me"
	mount := func(agentName, root string) plan.ConfigMount {
		return plan.ConfigMount{Agent: agentName, Root: root, HostPath: root + "/resolved"}
	}
	tests := []struct {
		name   string
		vars   map[string]string
		report plan.ConfigReport
		want   []AgentStatus
	}{
		{
			name: "config and env of every agent, in registry order",
			vars: map[string]string{"ANTHROPIC_API_KEY": "secret-value"},
			report: plan.ConfigReport{Mounts: []plan.ConfigMount{
				mount("claude", home+"/.claude"),
				mount("codex", home+"/.codex"),
				mount("pi", home+"/.config/pi"),
				mount("pi", home+"/.pi"),
			}},
			want: []AgentStatus{
				{Name: "claude", Description: "Claude Code", Config: []string{"~/.claude"}, EnvSet: []string{"ANTHROPIC_API_KEY"}},
				{Name: "codex", Description: "Codex CLI", Config: []string{"~/.codex"}, EnvSet: []string{"ANTHROPIC_API_KEY"}},
				{Name: "gemini", Description: "Gemini CLI"},
				{Name: "pi", Description: "Pi coding agent", Config: []string{"~/.config/pi", "~/.pi"}},
			},
		},
		{
			name: "a root split into several mounts is counted once",
			report: plan.ConfigReport{Mounts: []plan.ConfigMount{
				mount("gemini", home+"/.gemini"),
				mount("gemini", home+"/.gemini"),
			}},
			want: []AgentStatus{
				{Name: "claude", Description: "Claude Code"},
				{Name: "codex", Description: "Codex CLI"},
				{Name: "gemini", Description: "Gemini CLI", Config: []string{"~/.gemini (2 files)"}},
				{Name: "pi", Description: "Pi coding agent"},
			},
		},
		{
			name: "an override outside the home directory is shown as is",
			report: plan.ConfigReport{Mounts: []plan.ConfigMount{
				mount("codex", "/data/codex-home"),
			}},
			want: []AgentStatus{
				{Name: "claude", Description: "Claude Code"},
				{Name: "codex", Description: "Codex CLI", Config: []string{"/data/codex-home"}},
				{Name: "gemini", Description: "Gemini CLI"},
				{Name: "pi", Description: "Pi coding agent"},
			},
		},
		{
			name: "the shared gitconfig belongs to no agent",
			report: plan.ConfigReport{Mounts: []plan.ConfigMount{
				{Agent: plan.SharedOwner, Root: home + "/.gitconfig", HostPath: home + "/.gitconfig"},
			}},
			want: []AgentStatus{
				{Name: "claude", Description: "Claude Code"},
				{Name: "codex", Description: "Codex CLI"},
				{Name: "gemini", Description: "Gemini CLI"},
				{Name: "pi", Description: "Pi coding agent"},
			},
		},
		{
			name: "refusals and missing paths are not listed",
			vars: map[string]string{"OPENAI_API_KEY": ""},
			report: plan.ConfigReport{
				Missing: []string{home + "/.claude"},
				Refused: []plan.Refusal{{Path: home + "/.pi", Reason: "resolves outside home"}},
			},
			want: []AgentStatus{
				{Name: "claude", Description: "Claude Code"},
				{Name: "codex", Description: "Codex CLI", EnvSet: []string{"OPENAI_API_KEY"}},
				{Name: "gemini", Description: "Gemini CLI"},
				{Name: "pi", Description: "Pi coding agent"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			configs := &fakeConfigs{report: tt.report}
			got, err := NewLister(&fakeEnv{vars: tt.vars, home: home}, configs, listerAgents()).List()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("List() =\n%+v\nwant\n%+v", got, tt.want)
			}
			if configs.gotHome != home {
				t.Fatalf("resolver got home %q, want %q", configs.gotHome, home)
			}
		})
	}
}

func TestListerNeverReturnsEnvValues(t *testing.T) {
	t.Parallel()

	env := &fakeEnv{vars: map[string]string{"ANTHROPIC_API_KEY": "secret-value", "OPENAI_API_KEY": "secret-value"}, home: "/Users/me"}
	got, err := NewLister(env, &fakeConfigs{}, listerAgents()).List()
	if err != nil {
		t.Fatal(err)
	}
	if text := strings.ReplaceAll(strings.TrimSpace(formatForSearch(got)), "\n", " "); strings.Contains(text, "secret-value") {
		t.Fatalf("List() leaked an environment value: %s", text)
	}
}

func formatForSearch(statuses []AgentStatus) string {
	var b strings.Builder
	for _, status := range statuses {
		b.WriteString(strings.Join(append(append([]string{status.Name, status.Description}, status.Config...), status.EnvSet...), " "))
		b.WriteString("\n")
	}
	return b.String()
}

func TestListerNeedsAHomeDirectory(t *testing.T) {
	t.Parallel()

	_, err := NewLister(&fakeEnv{homeErr: errors.New("unset")}, &fakeConfigs{}, listerAgents()).List()
	if err == nil || err.Error() != "find home directory: unset" {
		t.Fatalf("List() error = %v", err)
	}
}
