package container

import (
	"strings"
	"testing"
)

func TestEmbeddedDockerfileIsPresent(t *testing.T) {
	t.Parallel()

	if len(runtimeDockerfile) == 0 {
		t.Fatal("runtime Dockerfile is empty")
	}
	if len(runtimeDockerfile) < 200 {
		t.Fatalf("runtime Dockerfile unexpectedly short: %d bytes", len(runtimeDockerfile))
	}
}

func TestRuntimeStateDirectoriesAreWritableByMappedUsers(t *testing.T) {
	t.Parallel()

	dockerfile := string(runtimeDockerfile)
	statePaths := []string{
		"/home/caged/.config/pi",
		"/home/caged/.pi",
		"/home/caged/.claude",
		"/home/caged/.codex",
		"/home/caged/.gemini",
	}
	for _, statePath := range statePaths {
		if !strings.Contains(dockerfile, statePath) {
			t.Errorf("runtime Dockerfile does not create state path %q", statePath)
		}
	}
	chmodLine := ""
	for _, line := range strings.Split(dockerfile, "\n") {
		if strings.Contains(line, "chmod 0777 /home/caged/.config") {
			chmodLine = line
			break
		}
	}
	if chmodLine == "" {
		t.Fatal("runtime Dockerfile does not make mounted state paths writable")
	}
	for _, statePath := range statePaths {
		if !strings.Contains(chmodLine, statePath) {
			t.Errorf("runtime Dockerfile does not make state path %q writable", statePath)
		}
	}
}

func TestEmbeddedEntrypointPreparesAndDropsFromStateVolumes(t *testing.T) {
	t.Parallel()

	entrypoint := string(runtimeEntrypoint)
	for _, required := range []string{
		"prepare_state_directories",
		"chmod -R a+rwX",
		"setpriv --reuid",
		"/home/caged/.codex",
	} {
		if !strings.Contains(entrypoint, required) {
			t.Errorf("embedded entrypoint is missing %q", required)
		}
	}
}
