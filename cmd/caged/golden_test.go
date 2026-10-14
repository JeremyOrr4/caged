package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/images"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
)

// fakeDockerPath is the fake docker CLI. Its header documents the state
// directory that drives it.
const fakeDockerPath = "testdata/fakedocker/docker"

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

// goldenVersion is the caged version the golden tests run as.
const goldenVersion = "test"

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

// goldenExtensionFile is the path of the user's image.json under $ROOT.
const goldenExtensionFile = "home/.config/caged/image.json"

// currentExtension is the extension image that the golden version would build
// from the packages in apt, on the base image with ID baseID.
func currentExtension(apt []string, baseID string) fakeImage {
	target := images.Extension(plan.Extension{Source: "~/.config/caged/image.json", Apt: apt}, baseID, goldenVersion)
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

// goldenPrompt contains shell syntax to prove prompts are passed verbatim.
const goldenPrompt = `hello $(world) "quoted" 'single'`

type goldenCase struct {
	name string
	args []string
	// env is applied after the defaults; "$ROOT" is replaced with the scenario root.
	env map[string]string
	// noDocker hides the fake docker binary from PATH.
	noDocker bool
	// files are extra fixture files under $ROOT, by relative path.
	files map[string]string
	// images are the local images; nil means defaultImages.
	images map[string]fakeImage
	// fail makes docker verbs fail, by verb name (see the fake docker header).
	fail map[string]fakeFailure
	// ls are the lines `docker image ls` prints, one JSON object each.
	ls []string
}

// daemonDown is what docker info prints when the daemon is not running.
var daemonDown = fakeFailure{Code: 1, Stderr: "Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?"}

func goldenCases() []goldenCase {
	cases := []goldenCase{
		{name: "shell", args: []string{}},
		{name: "shell-no-mount-no-config", args: []string{"--no-mount", "--no-config"}},
		{name: "list", args: []string{"list"}},
		{name: "version", args: []string{"--version"}},
		{name: "help", args: []string{"--help"}},
		{name: "help-list", args: []string{"help", "list"}},
		{name: "help-unknown-topic", args: []string{"help", "nope"}},
		{name: "build-help", args: []string{"build", "--help"}},
		{name: "unknown-flag", args: []string{"--bogus"}},
		{name: "build", args: []string{"build"}},
		{name: "build-no-cache", args: []string{"build", "--no-cache"}},
		{name: "build-rejects-args", args: []string{"build", "extra"}},
		{name: "list-rejects-args", args: []string{"list", "extra"}},
		{name: "complete-agents", args: []string{"__complete", ""}},
		{name: "complete-prompt", args: []string{"__complete", "claude", ""}},
		{name: "agent-uppercase", args: []string{"Claude"}},
		{name: "unsupported-agent", args: []string{"aider"}},
		{name: "too-many-args", args: []string{"claude", "one", "two"}},
		{name: "env-invalid", args: []string{"--env", "bad-name", "claude"}},
		{name: "env-padded-name", args: []string{"--env", " CAGED_GOLDEN_SET ", "--verbose", "gemini"}, env: map[string]string{"CAGED_GOLDEN_SET": "x"}},
		{
			name: "shell-env",
			args: []string{"--env", "CAGED_GOLDEN_SET", "--env", "CAGED_GOLDEN_UNSET", "--verbose", "--no-config"},
			env:  map[string]string{"CAGED_GOLDEN_SET": "x", "ANTHROPIC_API_KEY": "not-forwarded-to-a-plain-shell"},
		},
		{
			name: "env-extra-names",
			args: []string{"--env", "CAGED_GOLDEN_SET", "--env", "CAGED_GOLDEN_UNSET", "--env", "ANTHROPIC_API_KEY", "--verbose", "codex"},
			env:  map[string]string{"CAGED_GOLDEN_SET": "value-is-never-printed", "OPENAI_API_KEY": "secret"},
		},
		{name: "verbose-claude", args: []string{"--verbose", "claude"}, env: map[string]string{"ANTHROPIC_API_KEY": "secret"}},
		{name: "no-mount-with-workdir", args: []string{"--no-mount", "-w", "$ROOT/work/other"}},
		{name: "workdir-flag", args: []string{"-w", "$ROOT/work/other", "pi"}},
		{name: "workdir-relative", args: []string{"--workdir", "../other", "--no-config"}},
		{name: "workdir-missing", args: []string{"-w", "$ROOT/work/missing"}},
		{name: "workdir-is-home", args: []string{"-w", "$ROOT/home"}},
		{name: "workdir-home-allowed", args: []string{"--allow-broad-mount", "-w", "$ROOT/home", "--no-config"}},
		{name: "workdir-root-allowed", args: []string{"--allow-broad-mount", "-w", "/", "--no-config"}},
		{name: "workdir-root", args: []string{"-w", "/"}},
		{
			name: "config-overrides",
			args: []string{"--verbose", "--no-mount"},
			env: map[string]string{
				"CODEX_HOME":        "alt-codex",
				"CLAUDE_CONFIG_DIR": "$ROOT/home",
				"COPILOT_HOME":      "$ROOT/outside/copilot",
			},
		},
		{name: "home-unresolvable", args: []string{"--verbose", "--no-mount"}, env: map[string]string{"HOME": "$ROOT/missing-home"}},
		{name: "ports-publish", args: []string{"-p", "3000", "-p", "8080:3000", "--verbose", "claude"}, env: map[string]string{"ANTHROPIC_API_KEY": "secret"}},
		{name: "ports-invalid", args: []string{"-p", "70000", "claude"}},
		{
			name: "ports-in-use",
			args: []string{"-p", "8080:3000", "claude"},
			fail: map[string]fakeFailure{"run": {Code: 125, Stderr: "docker: Error response from daemon: failed to set up container networking: driver failed programming external connectivity on endpoint x: failed to bind host port 127.0.0.1:8080/tcp: address already in use"}},
		},
		{name: "image-stale-rebuilds", args: []string{"claude"}, images: map[string]fakeImage{images.BaseRef: staleBase()}},
		{name: "image-unlabeled-rebuilds", args: []string{"gemini"}, images: map[string]fakeImage{images.BaseRef: {ID: "sha256:fake-base"}}},
		{name: "image-current-verbose", args: []string{"--verbose", "--no-mount", "--no-config"}},
		{
			name:   "image-rebuild-fails",
			args:   []string{"claude"},
			images: map[string]fakeImage{images.BaseRef: staleBase()},
			fail:   map[string]fakeFailure{"build": {Code: 1, Stderr: "ERROR: failed to solve: process \"/bin/sh -c false\" did not complete successfully"}},
		},
		{
			name:  "extension-first-build",
			args:  []string{"claude"},
			files: map[string]string{goldenExtensionFile: `{"apt": ["tmux", "neovim", "tmux"]}`},
		},
		{
			name:   "extension-current",
			args:   []string{"--verbose", "--no-config", "claude"},
			files:  map[string]string{goldenExtensionFile: `{"apt": ["neovim", "tmux"]}`},
			images: map[string]fakeImage{images.BaseRef: currentBase(), images.ExtensionRef: currentExtension([]string{"neovim", "tmux"}, "sha256:fake-base")},
		},
		{
			name:  "extension-xdg-config-home",
			args:  []string{"--no-config", "claude"},
			env:   map[string]string{"XDG_CONFIG_HOME": "$ROOT/xdg"},
			files: map[string]string{"xdg/caged/image.json": `{"apt": ["tmux"]}`},
		},
		{
			name:   "extension-stale-after-edit",
			args:   []string{"--no-config", "claude"},
			files:  map[string]string{goldenExtensionFile: `{"apt": ["neovim", "tmux", "shellcheck"]}`},
			images: map[string]fakeImage{images.BaseRef: currentBase(), images.ExtensionRef: staleExtension("sha256:fake-base")},
		},
		{
			name:  "extension-invalid",
			args:  []string{"claude"},
			files: map[string]string{goldenExtensionFile: `{"apt": ["vim; curl x"]}`},
		},
		{
			name:  "build-with-extension",
			args:  []string{"build"},
			files: map[string]string{goldenExtensionFile: `{"apt": ["neovim", "tmux"]}`},
		},
		{name: "passthrough-prompt-claude", args: []string{"claude", goldenPrompt, "--", "--model", "opus", "--add-dir", "../lib", "$(id)", `"q"`}},
		{name: "passthrough-gh", args: []string{"gh", "--", "pr", "list"}},
		{name: "passthrough-no-agent", args: []string{"--", "--resume"}},
		{name: "passthrough-dash-build", args: []string{"--", "build"}},
		{name: "doctor-ok", args: []string{"doctor"}},
		{name: "doctor-daemon-down", args: []string{"doctor"}, fail: map[string]fakeFailure{"info": daemonDown}},
		{name: "doctor-arch-mismatch", args: []string{"doctor"}, images: map[string]fakeImage{images.BaseRef: withArch(currentBase(), "s390x")}},
		{name: "list-env-set", args: []string{"list"}, env: map[string]string{"ANTHROPIC_API_KEY": "secret-value-never-printed", "GH_TOKEN": "secret-value-never-printed"}},
		{name: "clean", args: []string{"clean"}, ls: cleanFixture()},
		{name: "clean-dry-run", args: []string{"clean", "--dry-run"}, ls: cleanFixture()},
		{
			name: "clean-in-use",
			args: []string{"clean"},
			ls:   cleanFixture(),
			fail: map[string]fakeFailure{"image-rm": {Code: 1, Stderr: "Error response from daemon: conflict: unable to delete sha256:aaa (cannot be forced) - image is being used by running container 0123456789ab"}},
		},
		{name: "image-missing-builds-first", args: []string{"gemini"}, images: map[string]fakeImage{}},
		{name: "run-exit-code", args: []string{"claude"}, fail: map[string]fakeFailure{"run": {Code: 7}}},
		{
			name: "run-mounts-denied",
			args: []string{"codex"},
			fail: map[string]fakeFailure{"run": {Code: 125, Stderr: "docker: Error response from daemon: Mounts denied: the path is not shared from the host."}},
		},
		{name: "daemon-down", args: []string{"claude"}, fail: map[string]fakeFailure{"info": daemonDown}},
		{name: "build-daemon-down", args: []string{"build"}, fail: map[string]fakeFailure{"info": daemonDown}},
		{name: "docker-missing", args: []string{"claude"}, noDocker: true},
	}
	for _, name := range []string{"claude", "codex", "copilot", "gemini", "gh", "pi"} {
		cases = append(cases,
			goldenCase{name: "agent-" + name, args: []string{name}},
			goldenCase{name: "prompt-" + name, args: []string{name, goldenPrompt}},
		)
	}
	return cases
}

// goldenPlatforms are the hosts the golden files describe. The linux files
// were captured from the original code; the darwin files differ from them
// only by the macOS rules in internal/docker/platform.go.
var goldenPlatforms = []system.Platform{
	{OS: "linux", Arch: "amd64", UID: 4242, GID: 4343},
	{OS: "darwin", Arch: "arm64", UID: 501, GID: 20},
}

// TestGoldenDockerInvocations runs complete command lines against a fake
// docker binary and compares everything observable (exit code, error, stdout,
// stderr, and every docker invocation) with files in testdata/golden/<os>.
func TestGoldenDockerInvocations(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake docker binary is a POSIX shell script")
	}
	for _, platform := range goldenPlatforms {
		goldenDir, err := filepath.Abs(filepath.Join("testdata", "golden", platform.OS))
		if err != nil {
			t.Fatal(err)
		}
		for _, tc := range goldenCases() {
			t.Run(platform.OS+"/"+tc.name, func(t *testing.T) {
				got := runGoldenCase(t, tc, platform, originalAgents(t))
				compareGolden(t, filepath.Join(goldenDir, tc.name+".golden"), got)
			})
		}
	}
}

// originalAgents is a registry of the six agents the golden files were
// captured with, so adding a built-in agent does not change the fixtures.
func originalAgents(t *testing.T) *agent.Registry {
	t.Helper()
	builtin := agent.Builtin()
	var agents []agent.Agent
	for _, name := range []string{"claude", "codex", "copilot", "gemini", "gh", "pi"} {
		definition, err := builtin.Lookup(name)
		if err != nil {
			t.Fatal(err)
		}
		agents = append(agents, definition)
	}
	registry, err := agent.NewRegistry(agents...)
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func runGoldenCase(t *testing.T, tc goldenCase, platform system.Platform, agents *agent.Registry) string {
	root := t.TempDir()
	writeGoldenFixtures(t, root)
	writeFakeDockerState(t, root, tc, platform)
	logPath := filepath.Join(root, "docker.log")

	for _, name := range []string{
		"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY",
		"GH_TOKEN", "GITHUB_TOKEN", "COPILOT_GITHUB_TOKEN",
		"CLAUDE_CONFIG_DIR", "CODEX_HOME", "COPILOT_HOME",
		"CAGED_GOLDEN_SET", "CAGED_GOLDEN_UNSET", "XDG_CONFIG_HOME",
	} {
		unsetEnv(t, name)
	}
	path := filepath.Join(root, "bin") + string(os.PathListSeparator) + os.Getenv("PATH")
	if tc.noDocker {
		path = filepath.Join(root, "emptybin")
	}
	t.Setenv("PATH", path)
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("TMPDIR", filepath.Join(root, "tmp"))
	t.Setenv("FAKE_DOCKER_LOG", logPath)
	t.Setenv("FAKE_DOCKER_STATE", filepath.Join(root, "docker-state"))
	for name, value := range tc.env {
		t.Setenv(name, strings.ReplaceAll(value, "$ROOT", root))
	}
	restore := chdir(t, filepath.Join(root, "work", "project"))
	defer restore()

	args := make([]string, 0, len(tc.args))
	for _, arg := range tc.args {
		args = append(args, strings.ReplaceAll(arg, "$ROOT", root))
	}
	stdout, stderr, code, err := executeRun(t, root, args, platform, agents)

	dockerLog, readErr := os.ReadFile(logPath)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		t.Fatal(readErr)
	}
	leftovers, readErr := os.ReadDir(filepath.Join(root, "tmp"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	errText := "<nil>"
	if err != nil {
		errText = err.Error()
	}
	var b strings.Builder
	fmt.Fprintf(&b, "args: %q\n", tc.args)
	fmt.Fprintf(&b, "exit code: %d\n", code)
	fmt.Fprintf(&b, "error: %s\n", errText)
	fmt.Fprintf(&b, "leftover temp entries: %d\n", len(leftovers))
	fmt.Fprintf(&b, "=== stdout\n%s", stdout)
	fmt.Fprintf(&b, "=== stderr\n%s", stderr)
	fmt.Fprintf(&b, "=== docker invocations\n%s", dockerLog)
	return normalizeGolden(t, b.String(), root, platform)
}

// executeRun calls run, the composition root, and captures the process-wide
// stdout and stderr: cobra writes there directly, and caged's own output is
// pointed at the same files through stdio.
func executeRun(t *testing.T, root string, args []string, platform system.Platform, agents *agent.Registry) (string, string, int, error) {
	stdoutFile, err := os.Create(filepath.Join(root, "stdout.txt"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := stdoutFile.Close(); err != nil {
			t.Error(err)
		}
	})
	stderrFile, err := os.Create(filepath.Join(root, "stderr.txt"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := stderrFile.Close(); err != nil {
			t.Error(err)
		}
	})

	oldStdout, oldStderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdoutFile, stderrFile
	stdio := system.Stdio{In: strings.NewReader(""), Out: stdoutFile, Err: stderrFile}
	code := run(context.Background(), goldenVersion, args, stdio, platform, agents)
	os.Stdout, os.Stderr = oldStdout, oldStderr

	stdout, err := os.ReadFile(stdoutFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := os.ReadFile(stderrFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(stdout), string(stderr), code, errorLine(string(stderr), code)
}

// errorLine recovers the error that run printed as the last stderr line.
func errorLine(stderr string, code int) error {
	if code == 0 {
		return nil
	}
	lines := strings.Split(strings.TrimSuffix(stderr, "\n"), "\n")
	return errors.New(lines[len(lines)-1])
}

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
	script, err := os.ReadFile(fakeDockerPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "docker"), script, 0o755); err != nil {
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
		write(filepath.Join(root, name), []byte(contents))
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

var (
	buildDirPattern = regexp.MustCompile(`caged-(runtime|extension)-[0-9]+`)
	cksumPattern    = regexp.MustCompile(`(build context Dockerfile: )[0-9]+ [0-9]+`)
	extHashPattern  = regexp.MustCompile(`(extension-hash=)[0-9a-f]{64}`)
)

// normalizeGolden replaces values that change between runs and machines.
func normalizeGolden(t *testing.T, text, root string, platform system.Platform) string {
	t.Helper()
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	text = strings.ReplaceAll(text, resolvedRoot, "$ROOT")
	text = strings.ReplaceAll(text, root, "$ROOT")
	text = strings.ReplaceAll(text, fmt.Sprintf("\n--user\n%d:%d\n", platform.UID, platform.GID), "\n--user\n$UID:$GID\n")
	// The runtime hash and the Dockerfile checksum change with every edit of the
	// embedded files; the goldens cover argv, output, and exit codes only.
	hash := images.ContextHash(images.Base(goldenVersion).Files)
	text = strings.ReplaceAll(text, hash, "$RUNTIME_HASH")
	text = strings.ReplaceAll(text, hash[:12], "$RUNTIME_HASH12")
	text = cksumPattern.ReplaceAllString(text, "${1}$$CKSUM")
	text = extHashPattern.ReplaceAllString(text, "${1}$$EXT_HASH")
	return buildDirPattern.ReplaceAllString(text, "caged-$1-N")
}

// compareGolden checks got against the golden file. With CAGED_UPDATE_GOLDEN=1
// a missing golden file is created; existing files are never overwritten.
func compareGolden(t *testing.T, path, got string) {
	t.Helper()
	want, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) && os.Getenv("CAGED_UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if err != nil {
		t.Fatalf("read golden file (run with CAGED_UPDATE_GOLDEN=1 to create it): %v", err)
	}
	if got != string(want) {
		t.Errorf("output differs from %s\n--- want\n%s\n--- got\n%s", path, want, got)
	}
}

// unsetEnv removes an environment variable for the duration of the test;
// t.Setenv restores the previous value afterwards.
func unsetEnv(t *testing.T, name string) {
	t.Helper()
	t.Setenv(name, "")
	if err := os.Unsetenv(name); err != nil {
		t.Fatal(err)
	}
}

// chdir changes into dir and returns a function that changes back. The change
// is also undone when the test ends, in case the caller never calls it.
func chdir(t *testing.T, dir string) (restore func()) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	restore = func() {
		if err := os.Chdir(old); err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(restore)
	return restore
}
