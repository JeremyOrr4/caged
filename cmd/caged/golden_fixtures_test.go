package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/JeremyOrr4/caged/internal/images"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
)

// fakeDockerScript is the fake docker CLI. Its header documents the state
// directory that drives it. It is embedded so that writing a fixture does not
// depend on the working directory, which enterFakeDockerEnv changes.
//
//go:embed testdata/fakedocker/docker
var fakeDockerScript []byte

// fakeImage is the image inspect reply of one local image.
type fakeImage struct {
	ID, OS, Arch string
	Labels       map[string]string
}

// fakeFailure makes one docker verb fail with an exit code and stderr text.
type fakeFailure struct {
	Code   int
	Stderr string
}

// withArch returns a copy of image built for another architecture.
func withArch(image fakeImage, arch string) fakeImage {
	image.Arch = arch
	return image
}

// cleanFixture is what `docker image ls` shows on a machine with an extension
// image, a runtime image, a replaced (untagged) image, and an older unlabeled one.
func cleanFixture() []string {
	return []string{
		`{"ID":"sha256:aaa111","Repository":"caged-runtime","Tag":"ext","Size":"412MB","Labels":"io.github.jeremyorr4.caged.role=extension"}`,
		`{"ID":"sha256:bbb222","Repository":"caged-runtime","Tag":"latest","Size":"3.1GB","Labels":"io.github.jeremyorr4.caged.role=runtime"}`,
		`{"ID":"sha256:1a2b3c4d5e6f7a8b","Repository":"<none>","Tag":"<none>","Size":"3.0GB","Labels":"io.github.jeremyorr4.caged.role=runtime"}`,
		`{"ID":"sha256:ccc333","Repository":"caged-runtime","Tag":"old","Size":"2.9GB"}`,
	}
}

// currentBase is the runtime image that the golden version would build.
func currentBase() fakeImage {
	labels := map[string]string{images.LabelVersion: goldenVersion}
	for _, label := range images.Base(goldenVersion).Identity {
		labels[label.Key] = label.Value
	}
	return fakeImage{ID: "sha256:fake-base", Labels: labels}
}

// staleBase is a runtime image built from other files by an older caged.
func staleBase() fakeImage {
	return fakeImage{ID: "sha256:fake-base", Labels: map[string]string{
		images.LabelRole: string(images.RoleRuntime), images.LabelRuntimeHash: "old", images.LabelVersion: "0.0.1",
	}}
}

// goldenExtensionFile is the path of the user's config.yaml under $ROOT.
const goldenExtensionFile = "home/.config/caged/config.yaml"

// currentExtension is the extension image that the golden version would build
// from the packages in apt, on the base image with ID baseID.
func currentExtension(apt []string, baseID string) fakeImage {
	target := images.Extension(plan.Extension{Source: "~/.config/caged/config.yaml", Apt: apt}, baseID, goldenVersion)
	labels := map[string]string{images.LabelVersion: goldenVersion}
	for _, label := range target.Identity {
		labels[label.Key] = label.Value
	}
	return fakeImage{ID: "sha256:fake-ext", Labels: labels}
}

// staleExtension is an extension image built from other packages.
func staleExtension(baseID string) fakeImage {
	return fakeImage{ID: "sha256:fake-ext", Labels: map[string]string{
		images.LabelRole: string(images.RoleExtension), images.LabelExtensionHash: "old", images.LabelBaseID: baseID, images.LabelVersion: goldenVersion,
	}}
}

// defaultImages is the state of a machine with an up-to-date runtime image.
func defaultImages() map[string]fakeImage {
	return map[string]fakeImage{images.BaseRef: currentBase()}
}

// inspectJSON renders the reply the fake docker gives for `image inspect`. An
// image with no architecture gets defaultArch, the architecture of the host.
func (i fakeImage) inspectJSON(defaultArch string) ([]byte, error) {
	if i.ID == "" {
		i.ID = "sha256:fake-base"
	}
	if i.OS == "" {
		i.OS = "linux"
	}
	if i.Arch == "" {
		i.Arch = defaultArch
	}
	type config struct{ Labels map[string]string }
	reply := []struct {
		ID           string `json:"Id"`
		OS           string `json:"Os"`
		Architecture string
		Config       config
	}{{ID: i.ID, OS: i.OS, Architecture: i.Arch, Config: config{Labels: i.Labels}}}
	return json.Marshal(reply)
}

// daemonDown is what docker info prints when the daemon is not running.
var daemonDown = fakeFailure{Code: 1, Stderr: "Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?"}

// writeGoldenFixtures creates a fake home with every supported config, a
// project directory, and the fake docker binary.
func writeGoldenFixtures(t *testing.T, root string) {
	t.Helper()
	for _, dir := range []string{
		"bin", "emptybin", "tmp", "work/project", "work/other", "outside/pi", "outside/copilot",
		"home/.claude", "home/.codex/app-server-control", "home/.codex/packages/app-server-daemon",
		"home/.codex/tmp/arg0", "home/.gemini", "home/.pi/agent", "home/.config/gh", "home/.copilot",
		"home/alt-codex",
	} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for file, contents := range map[string]string{
		"work/project/main.go":           "package main\n",
		"outside/pi/settings.json":       "{}\n",
		"home/.claude/settings.json":     "{}\n",
		"home/.claude/.credentials.json": "{}\n",
		"home/.claude.json":              "{}\n",
		"home/.codex/config.toml":        "model = 'test'\n",
		"home/.codex/auth.json":          "{}\n",
		"home/.gemini/settings.json":     "{}\n",
		"home/.gemini/oauth_creds.json":  "{}\n",
		"home/.pi/agent/settings.json":   "{}\n",
		"home/.config/gh/hosts.yml":      "github.com: {}\n",
		"home/.gitconfig":                "[user]\n\tname = Test\n",
		"home/alt-codex/config.toml":     "model = 'alt'\n",
	} {
		if err := os.WriteFile(filepath.Join(root, file), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "docker"), fakeDockerScript, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(root, "home/.codex/app-server-control/daemon.sock"), 0o600); err != nil {
		t.Fatal(err)
	}
	for link, target := range map[string]string{
		"home/.codex/packages/app-server-daemon/current": "/opt/homebrew/bin/codex",
		"home/.gemini/current-settings.json":             "settings.json",
		"home/.config/pi":                                filepath.Join(root, "outside", "pi"),
	} {
		if err := os.Symlink(target, filepath.Join(root, link)); err != nil {
			t.Fatal(err)
		}
	}
}

// writeFakeDockerState writes the case's extra fixture files and the state
// directory that drives the fake docker.
func writeFakeDockerState(t *testing.T, root string, tc goldenCase, platform system.Platform) {
	t.Helper()
	write := func(path string, data []byte) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for name, contents := range tc.files {
		write(filepath.Join(root, name), []byte(strings.ReplaceAll(contents, "$ROOT", root)))
	}
	for link, target := range tc.links {
		path := filepath.Join(root, link)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.RemoveAll(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(strings.ReplaceAll(target, "$ROOT", root), path); err != nil {
			t.Fatal(err)
		}
	}
	state := filepath.Join(root, "docker-state")
	images := tc.images
	if images == nil {
		images = defaultImages()
	}
	if err := os.MkdirAll(filepath.Join(state, "images"), 0o755); err != nil {
		t.Fatal(err)
	}
	for ref, image := range images {
		reply, err := image.inspectJSON(platform.Arch)
		if err != nil {
			t.Fatal(err)
		}
		write(filepath.Join(state, "images", ref+".json"), append(reply, '\n'))
	}
	// The fake daemon runs on the same architecture as the host, as Docker
	// Desktop does, so no case sees a mismatch it did not ask for.
	daemonArch := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[platform.Arch]
	write(filepath.Join(state, "info"), []byte("27.0.0|Docker Desktop|"+daemonArch+"\n"))
	write(filepath.Join(state, "versions"), []byte("claude=2.1.3\ncodex=0.46.0\ngemini=0.9.0\ncopilot=0.0.330\npi=0.12.0\ngh=2.81.0\n"))
	if len(tc.ls) > 0 {
		write(filepath.Join(state, "ls"), []byte(strings.Join(tc.ls, "\n")+"\n"))
	}
	for verb, failure := range tc.fail {
		contents := fmt.Sprintf("%d\n", failure.Code)
		if failure.Stderr != "" {
			contents += failure.Stderr + "\n"
		}
		write(filepath.Join(state, "fail", verb), []byte(contents))
	}
}

// enterFakeDockerEnv points the process at the fixture under root until the
// test ends: credentials and config overrides of the outer environment
// removed, the fake docker first on PATH (noDocker leaves no docker at all),
// HOME, TMPDIR and the fake docker's log and state under root, then env with
// "$ROOT" replaced, and root/work/project as the working directory.
//
// It uses os.Chdir, not t.Chdir: t.Chdir also sets PWD, which os.Getwd
// returns as is, so under a symlinked TMPDIR (macOS /var) the planner would
// see an unresolved project path (decisions.md D-69).
func enterFakeDockerEnv(t *testing.T, root string, env map[string]string, noDocker bool) {
	t.Helper()
	for _, name := range []string{
		"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY",
		"GH_TOKEN", "GITHUB_TOKEN", "COPILOT_GITHUB_TOKEN",
		"CLAUDE_CONFIG_DIR", "CODEX_HOME", "COPILOT_HOME",
		"CAGED_GOLDEN_SET", "CAGED_GOLDEN_UNSET", "XDG_CONFIG_HOME", "XDG_STATE_HOME",
	} {
		unsetEnv(t, name)
	}
	path := filepath.Join(root, "bin") + string(os.PathListSeparator) + os.Getenv("PATH")
	if noDocker {
		path = filepath.Join(root, "emptybin")
	}
	t.Setenv("PATH", path)
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("TMPDIR", filepath.Join(root, "tmp"))
	t.Setenv("FAKE_DOCKER_LOG", filepath.Join(root, "docker.log"))
	t.Setenv("FAKE_DOCKER_STATE", filepath.Join(root, "docker-state"))
	for name, value := range env {
		t.Setenv(name, strings.ReplaceAll(value, "$ROOT", root))
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(filepath.Join(root, "work", "project")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Error(err)
		}
	})
}

// unsetEnv removes an environment variable for the duration of the test;
// t.Setenv restores the previous value afterwards.
func unsetEnv(t testing.TB, name string) {
	t.Helper()
	t.Setenv(name, "")
	if err := os.Unsetenv(name); err != nil {
		t.Fatal(err)
	}
}
