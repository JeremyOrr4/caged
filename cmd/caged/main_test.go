package main

import (
	"bytes"
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/system"
)

// TestNewAgentWorksEndToEnd registers one extra agent and runs it through the
// real composition root: the config resolver, env forwarding, prompt style,
// and `caged list` all pick it up with no other code change.
func TestNewAgentWorksEndToEnd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake docker binary is a POSIX shell script")
	}
	aider := agent.Agent{
		Name:        "aider",
		Command:     []string{"aider"},
		Description: "Aider",
		ConfigPaths: []agent.ConfigPath{{HostPath: ".aider", ContainerPath: ".aider", Writable: true}},
		Prompt:      agent.PromptFlag,
		PromptFlag:  "--message",
		Auth:        agent.AuthStrategy{Environment: []string{"OPENAI_API_KEY"}},
	}
	agents, err := agent.NewRegistry(append(originalAgents(t).All(), aider)...)
	if err != nil {
		t.Fatal(err)
	}
	forEachPlatform(t, func(t *testing.T, platform system.Platform) {
		got := runGoldenCase(t, goldenCase{args: []string{"--verbose", "aider", "add tests"}, env: map[string]string{"OPENAI_API_KEY": "x"}}, platform, agents)
		for _, want := range []string{
			"exit code: 0\n",
			"config: missing $ROOT/home/.aider\n",
			"env: pass OPENAI_API_KEY\n",
			"\n-e\nOPENAI_API_KEY\n",
			"\ncaged-runtime:latest\naider\n--message\nadd tests\n---\n",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("output does not contain %q:\n%s", want, got)
			}
		}

		list := runGoldenCase(t, goldenCase{args: []string{"list"}}, platform, agents)
		if !strings.Contains(list, "=== stdout\nAGENT    CONFIG") || !strings.Contains(list, "\naider    not found  ") || strings.Index(list, "\naider ") > strings.Index(list, "\nclaude ") {
			t.Errorf("list does not show the new agent first, with no config found:\n%s", list)
		}
	})
}

// TestRunTreatsNilArgsAsNoArguments makes sure run never falls back to the
// test binary's own os.Args.
func TestRunTreatsNilArgsAsNoArguments(t *testing.T) {
	forEachPlatform(t, func(t *testing.T, platform system.Platform) {
		t.Setenv("PATH", t.TempDir()) // no docker: the shell run fails at the image check
		home := t.TempDir()           // a run creates its starter config; never in the real home
		t.Setenv("HOME", home)
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
		t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
		var stderr bytes.Buffer
		stdio := system.Stdio{In: strings.NewReader(""), Out: &bytes.Buffer{}, Err: &stderr}
		code := run(context.Background(), "test", nil, stdio, platform, originalAgents(t))
		if code != 1 || !strings.Contains(stderr.String(), "Docker is not installed or not on PATH") {
			t.Fatalf("run(nil) = %d, stderr %q; want the plain shell path", code, stderr.String())
		}
	})
}
