package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/system"
)

// TestCredentialValuesNeverReachOutputOrDockerArgs sets every credential
// variable an agent reads, and one --env name, to a unique value, and runs
// each agent, a shell, list and doctor: the names may appear (docker gets
// `-e NAME`), the values never, on stdout, on stderr, or in docker's argv.
func TestCredentialValuesNeverReachOutputOrDockerArgs(t *testing.T) {
	agents := originalAgents(t)
	env := map[string]string{"CAGED_GOLDEN_SET": "extra-env-value-0"}
	for _, definition := range agents.All() {
		for _, name := range definition.Auth.Environment {
			env[name] = fmt.Sprintf("credential-value-of-%s", name)
		}
	}
	commands := [][]string{{"--verbose", "--env", "CAGED_GOLDEN_SET"}, {"list"}, {"doctor"}}
	for _, name := range agents.Names() {
		commands = append(commands, []string{"--verbose", "--env", "CAGED_GOLDEN_SET", name})
	}
	forEachPlatform(t, func(t *testing.T, platform system.Platform) {
		for _, args := range commands {
			t.Run(strings.Join(args, " "), func(t *testing.T) {
				got := runGoldenCase(t, goldenCase{args: args, env: env}, platform, agents)
				for name, value := range env {
					if strings.Contains(got, value) {
						t.Errorf("the value of %s is in the output or the docker arguments:\n%s", name, got)
					}
				}
				if len(args) > 1 && args[0] == "--verbose" && !strings.Contains(got, "\n-e\nCAGED_GOLDEN_SET\n") {
					t.Errorf("docker run lacks -e CAGED_GOLDEN_SET, so the check above proves nothing:\n%s", got)
				}
			})
		}
	})
}
