package runtime

import (
	"reflect"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/plan"
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

	dockerfile := strings.ReplaceAll(string(runtimeDockerfile), "\\\n", " ")
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
	chmodStart := strings.Index(dockerfile, "chmod 0777 /home/caged")
	if chmodStart == -1 {
		t.Fatal("runtime Dockerfile does not make agent home paths writable by mapped users")
	}
	chmodEnd := strings.Index(dockerfile[chmodStart:], "&& git config")
	chmodLine := dockerfile[chmodStart:]
	if chmodEnd >= 0 {
		chmodLine = chmodLine[:chmodEnd]
	}
	for _, homePath := range homePaths {
		if !strings.Contains(chmodLine, homePath) {
			t.Errorf("runtime Dockerfile does not make agent home path %q writable", homePath)
		}
	}
}

func TestEmbeddedEntrypointRunsAgentAndReturnsToShell(t *testing.T) {
	t.Parallel()

	entrypoint := string(runtimeEntrypoint)
	if strings.Contains(entrypoint, "python3") || strings.Contains(entrypoint, "CAGED_CONFIG_MAP") {
		t.Error("embedded entrypoint still contains config-copy logic")
	}
	if !strings.Contains(entrypoint, `exec "$@"`) {
		t.Error("embedded entrypoint does not directly run the requested command")
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

func TestClaudeUsesDirectoryConfigMount(t *testing.T) {
	t.Parallel()

	if !strings.Contains(string(runtimeDockerfile), "CLAUDE_CONFIG_DIR=/home/caged/.claude") {
		t.Fatal("runtime image does not keep Claude config writes inside the mounted directory")
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

func TestRuntimeIncludesGithubCLIAndPersistsItsConfigDirectory(t *testing.T) {
	t.Parallel()
	dockerfile := string(runtimeDockerfile)
	for _, expected := range []string{
		"https://cli.github.com/packages/githubcli-archive-keyring.gpg",
		"apt-get install -y --no-install-recommends gh",
		"/home/caged/.config/gh",
	} {
		if !strings.Contains(dockerfile, expected) {
			t.Errorf("runtime Dockerfile missing GitHub CLI setup %q", expected)
		}
	}
}

func TestBuildSpecDescribesEmbeddedBuildContext(t *testing.T) {
	t.Parallel()

	want := plan.BuildSpec{
		Image:      "caged-runtime:latest",
		Dockerfile: "Dockerfile",
		NoCache:    true,
		Files: []plan.File{
			{Name: "Dockerfile", Label: "Dockerfile", Data: []byte(runtimeDockerfile), Mode: 0o644},
			{Name: "caged-entrypoint.sh", Label: "entrypoint", Data: []byte(runtimeEntrypoint), Mode: 0o755},
		},
	}
	got := BuildSpec(true)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildSpec(true) = %+v, want %+v", got, want)
	}
	got.Files[0].Data[0] = 'X'
	if BuildSpec(false).Files[0].Data[0] == 'X' || BuildSpec(false).NoCache {
		t.Fatal("BuildSpec must return fresh data and honor noCache")
	}
}
