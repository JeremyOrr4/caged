package app

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/testkit"
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
				{Name: "claude", Config: []string{"~/.claude"}, EnvSet: []string{"ANTHROPIC_API_KEY"}},
				{Name: "codex", Config: []string{"~/.codex"}, EnvSet: []string{"ANTHROPIC_API_KEY"}},
				{Name: "gemini"},
				{Name: "pi", Config: []string{"~/.config/pi", "~/.pi"}},
			},
		},
		{
			name: "a root split into several mounts is counted once",
			report: plan.ConfigReport{Mounts: []plan.ConfigMount{
				mount("gemini", home+"/.gemini"),
				mount("gemini", home+"/.gemini"),
			}},
			want: []AgentStatus{
				{Name: "claude"},
				{Name: "codex"},
				{Name: "gemini", Config: []string{"~/.gemini (2 files)"}},
				{Name: "pi"},
			},
		},
		{
			name: "an override outside the home directory is shown as is",
			report: plan.ConfigReport{Mounts: []plan.ConfigMount{
				mount("codex", "/data/codex-home"),
			}},
			want: []AgentStatus{
				{Name: "claude"},
				{Name: "codex", Config: []string{"/data/codex-home"}},
				{Name: "gemini"},
				{Name: "pi"},
			},
		},
		{
			name: "the shared gitconfig belongs to no agent",
			report: plan.ConfigReport{Mounts: []plan.ConfigMount{
				{Agent: plan.SharedOwner, Root: home + "/.gitconfig", HostPath: home + "/.gitconfig"},
			}},
			want: []AgentStatus{
				{Name: "claude"},
				{Name: "codex"},
				{Name: "gemini"},
				{Name: "pi"},
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
				{Name: "claude"},
				{Name: "codex", EnvSet: []string{"OPENAI_API_KEY"}},
				{Name: "gemini"},
				{Name: "pi"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			configs := &fakeConfigs{report: tt.report}
			got, err := NewLister(&testkit.Env{Vars: tt.vars, Home: home}, configs, listerAgents()).List()
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

	env := &testkit.Env{Vars: map[string]string{"ANTHROPIC_API_KEY": "secret-value", "OPENAI_API_KEY": "secret-value"}, Home: "/Users/me"}
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
		b.WriteString(strings.Join(append(append([]string{status.Name}, status.Config...), status.EnvSet...), " "))
		b.WriteString("\n")
	}
	return b.String()
}

func TestListerReturnsTheConfigResolverError(t *testing.T) {
	t.Parallel()

	resolveErr := errors.New("~/.config/caged/config.yaml: invalid YAML: line 1: did not find expected node content")
	_, err := NewLister(&testkit.Env{Home: "/Users/me"}, &fakeConfigs{err: resolveErr}, listerAgents()).List()
	if !errors.Is(err, resolveErr) {
		t.Fatalf("List() error = %v, want %v", err, resolveErr)
	}
}

func TestListerNeedsAHomeDirectory(t *testing.T) {
	t.Parallel()

	_, err := NewLister(&testkit.Env{HomeErr: errors.New("unset")}, &fakeConfigs{}, listerAgents()).List()
	if err == nil || err.Error() != "find home directory: unset" {
		t.Fatalf("List() error = %v", err)
	}
}

// TestListerReportsWhatARunWouldPass pins that `caged list` names exactly the
// credential variables a run without --env passes. The Lister asks the agent
// (ResolveEnv) as the Planner does, which equals a plain walk over
// Auth.Environment only when every shipped name is valid, trimmed and unique.
func TestListerReportsWhatARunWouldPass(t *testing.T) {
	t.Parallel()

	agents := agent.Builtin().All()
	vars := map[string]string{}
	for _, definition := range agents {
		if got := definition.EnvNames(nil); len(definition.Auth.Environment) > 0 && !reflect.DeepEqual(got, definition.Auth.Environment) {
			t.Errorf("%s: EnvNames(nil) = %q, want its Auth.Environment %q unchanged", definition.Name, got, definition.Auth.Environment)
		}
		for _, name := range definition.Auth.Environment {
			vars[name] = "secret"
		}
	}
	env := &testkit.Env{Home: "/home/me", Vars: vars}
	statuses, err := NewLister(env, &fakeConfigs{}, agents).List()
	if err != nil {
		t.Fatal(err)
	}
	for i, definition := range agents {
		if want := definition.ResolveEnv(nil, func(name string) bool { _, ok := env.LookupEnv(name); return ok }); !reflect.DeepEqual(statuses[i].EnvSet, want) {
			t.Errorf("%s: list shows %q, a run passes %q", definition.Name, statuses[i].EnvSet, want)
		}
	}
}
