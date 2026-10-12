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

func TestRuntimeAgentHomeDirectoriesAreWritableByMappedUsers(t *testing.T) {
	t.Parallel()

	dockerfile := string(runtimeDockerfile)
	homePaths := []string{
		"/home/caged/.config/pi",
		"/home/caged/.pi",
		"/home/caged/.claude",
		"/home/caged/.codex",
		"/home/caged/.gemini",
	}
	for _, homePath := range homePaths {
		if !strings.Contains(dockerfile, homePath) {
			t.Errorf("runtime Dockerfile does not create agent home path %q", homePath)
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
		t.Fatal("runtime Dockerfile does not make agent home paths writable by mapped users")
	}
	for _, homePath := range homePaths {
		if !strings.Contains(chmodLine, homePath) {
			t.Errorf("runtime Dockerfile does not make agent home path %q writable", homePath)
		}
	}
}

func TestEmbeddedEntrypointCopiesConfigAndReturnsToShell(t *testing.T) {
	t.Parallel()

	entrypoint := string(runtimeEntrypoint)
	if !strings.Contains(entrypoint, "copy_host_config") {
		t.Error("embedded entrypoint does not copy staged configs")
	}
	if !strings.Contains(entrypoint, "os.path.isfile(src) or os.path.islink(src)") {
		t.Error("embedded entrypoint does not skip sockets and other special config files")
	}
	if !strings.Contains(entrypoint, `CAGED_RETURN_TO_SHELL`) || !strings.Contains(entrypoint, "bash -i") {
		t.Error("embedded entrypoint does not open an interactive shell after agent exit")
	}
}

func TestRuntimeCodexCommandDisablesDaemon(t *testing.T) {
	t.Parallel()

	if !strings.Contains(string(runtimeDockerfile), `codex-real --no-daemon "$@"`) {
		t.Fatal("runtime image does not wrap Codex with --no-daemon for shell launches")
	}
}

func TestRuntimeImageLeavesClaudeConfigDestinationWritable(t *testing.T) {
	t.Parallel()

	if !strings.Contains(string(runtimeDockerfile), "rm -f /home/caged/.claude.json") {
		t.Fatal("runtime image does not remove the installer-created Claude config file")
	}
}

func TestRuntimePiInstallDoesNotOverlapHostConfigMount(t *testing.T) {
	t.Parallel()

	dockerfile := string(runtimeDockerfile)
	for _, expected := range []string{
		"HOME=/opt/caged/pi-home",
		"ln -sf /opt/caged/pi-home/.pi/agent/bin/pi /usr/local/bin/pi",
	} {
		if !strings.Contains(dockerfile, expected) {
			t.Errorf("runtime Dockerfile missing Pi install isolation %q", expected)
		}
	}
}
