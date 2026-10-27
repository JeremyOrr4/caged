package images

import (
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
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
	// Every built-in agent's config directories, so a new agent is checked
	// too, and Pi's install directory, which Pi writes at run time.
	homePaths := []string{"/home/caged/.pi/agent"}
	for _, definition := range agent.Builtin().All() {
		for _, configPath := range definition.ConfigPaths {
			homePaths = append(homePaths, "/home/caged/"+configPath.ContainerPath)
		}
	}
	mkdirStart := strings.LastIndex(dockerfile, "mkdir -p /workspace")
	if mkdirStart == -1 {
		t.Fatal("runtime Dockerfile does not create the agent home paths")
	}
	mkdirLine, _, _ := strings.Cut(dockerfile[mkdirStart:], "&&")
	for _, homePath := range homePaths {
		if !strings.Contains(mkdirLine+" ", " "+homePath+" ") {
			t.Errorf("runtime Dockerfile does not create agent home path %q", homePath)
		}
	}
	// The final stage's chmod, up to the next command: the build stage has
	// its own, and the mkdir before it names the same paths.
	chmodStart := strings.LastIndex(dockerfile, "chmod 0777 /home/caged")
	if chmodStart == -1 {
		t.Fatal("runtime Dockerfile does not make agent home paths writable by mapped users")
	}
	chmodLine, _, _ := strings.Cut(dockerfile[chmodStart:], "&&")
	for _, homePath := range homePaths {
		if !strings.Contains(chmodLine+" ", " "+homePath+" ") {
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

func TestRuntimeCreatesTheCagedConfigMountPoint(t *testing.T) {
	t.Parallel()

	dockerfile := string(runtimeDockerfile)
	if got := strings.Count(dockerfile, "/home/caged/.config/caged"); got != 2 {
		t.Errorf("runtime Dockerfile names /home/caged/.config/caged %d times; want 2 (mkdir and chmod 0777)", got)
	}
}

func TestDockerfileBasesArePinnedByTheSameDigest(t *testing.T) {
	t.Parallel()

	var digests []string
	for line := range strings.SplitSeq(runtimeDockerfile, "\n") {
		if !strings.HasPrefix(line, "FROM ") {
			continue
		}
		image := strings.Fields(line)[1]
		name, digest, found := strings.Cut(image, "@sha256:")
		if !found || name != "ubuntu:24.04" || len(digest) != 64 || strings.Trim(digest, "0123456789abcdef") != "" {
			t.Errorf("FROM line %q must be ubuntu:24.04@sha256:<64 hex characters>", line)
		}
		digests = append(digests, digest)
	}
	if len(digests) != 2 {
		t.Fatalf("found %d FROM lines, want 2", len(digests))
	}
	if digests[0] != digests[1] {
		t.Errorf("the two FROM lines use different digests: %q and %q", digests[0], digests[1])
	}
}

func TestBaseTargetUsesTheEmbeddedFiles(t *testing.T) {
	t.Parallel()

	target := Base("1.2.3")
	if target.role != RoleRuntime || target.Ref != BaseRef || target.version != "1.2.3" || target.contextName != "caged-runtime" || target.ext.Source != "" || target.ext.Apt != nil {
		t.Fatalf("Base() = %+v", target)
	}
	if len(target.Files) != 2 || target.Files[0].Name != "Dockerfile" || string(target.Files[0].Data) != runtimeDockerfile ||
		target.Files[1].Name != "caged-entrypoint.sh" || string(target.Files[1].Data) != runtimeEntrypoint || target.Files[1].Mode != 0o755 {
		t.Fatalf("Base() files = %+v, want the embedded Dockerfile and executable entrypoint", target.Files)
	}
}

func TestDockerfileRecordsEveryCLIVersion(t *testing.T) {
	t.Parallel()

	dockerfile := strings.ReplaceAll(runtimeDockerfile, "\\\n", " ")
	var loop string
	for line := range strings.SplitSeq(dockerfile, "\n") {
		if strings.Contains(line, "for cli in ") {
			loop = line
		}
	}
	if loop == "" {
		t.Fatal("the Dockerfile has no versions loop")
	}
	for _, definition := range agent.Builtin().All() {
		if !strings.Contains(loop, " "+definition.Command[0]+" ") && !strings.Contains(loop, " "+definition.Command[0]+";") {
			t.Errorf("the versions loop does not name %q", definition.Command[0])
		}
	}
	for _, want := range []string{"HOME=/tmp/caged-versions", "timeout 60", "> " + VersionsPath} {
		if !strings.Contains(loop, want) {
			t.Errorf("the versions step is missing %q", want)
		}
	}
}
