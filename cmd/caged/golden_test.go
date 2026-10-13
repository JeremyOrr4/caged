package main

import (
	"context"
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
	"github.com/JeremyOrr4/caged/internal/system"
)

// fakeDocker stands in for the docker CLI. It appends every invocation to
// $FAKE_DOCKER_LOG (one argument per line, then "---") and fakes the replies
// caged depends on. FAKE_DOCKER_* variables select failure modes.
const fakeDocker = `#!/bin/sh
{ printf '%s\n' "$@"; echo '---'; } >> "$FAKE_DOCKER_LOG"
case "$1" in
info)
  if [ -n "$FAKE_DOCKER_DAEMON_DOWN" ]; then
    echo 'Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?' >&2
    exit 1
  fi
  echo '27.0.0'
  ;;
image)
  if [ -n "$FAKE_DOCKER_IMAGE_MISSING" ]; then exit 1; fi
  ;;
build)
  for arg in "$@"; do context="$arg"; done
  {
    echo "build context Dockerfile: $(cksum < "$context/Dockerfile")"
    echo "build context caged-entrypoint.sh: $(cksum < "$context/caged-entrypoint.sh")"
    if [ -x "$context/caged-entrypoint.sh" ]; then echo 'build context entrypoint is executable'; fi
    echo '---'
  } >> "$FAKE_DOCKER_LOG"
  echo 'fake docker build output'
  ;;
run)
  echo 'fake docker run output'
  if [ -n "$FAKE_DOCKER_RUN_STDERR" ]; then echo "$FAKE_DOCKER_RUN_STDERR" >&2; fi
  exit "${FAKE_DOCKER_RUN_EXIT:-0}"
  ;;
esac
exit 0
`

// goldenPrompt contains shell syntax to prove prompts are passed verbatim.
const goldenPrompt = `hello $(world) "quoted" 'single'`

type goldenCase struct {
	name string
	args []string
	// env is applied after the defaults; "$ROOT" is replaced with the scenario root.
	env map[string]string
	// noDocker hides the fake docker binary from PATH.
	noDocker bool
}

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
		{name: "image-missing-builds-first", args: []string{"gemini"}, env: map[string]string{"FAKE_DOCKER_IMAGE_MISSING": "1"}},
		{name: "run-exit-code", args: []string{"claude"}, env: map[string]string{"FAKE_DOCKER_RUN_EXIT": "7"}},
		{
			name: "run-mounts-denied",
			args: []string{"codex"},
			env: map[string]string{
				"FAKE_DOCKER_RUN_EXIT":   "125",
				"FAKE_DOCKER_RUN_STDERR": "docker: Error response from daemon: Mounts denied: the path is not shared from the host.",
			},
		},
		{name: "daemon-down", args: []string{"claude"}, env: map[string]string{"FAKE_DOCKER_DAEMON_DOWN": "1"}},
		{name: "build-daemon-down", args: []string{"build"}, env: map[string]string{"FAKE_DOCKER_DAEMON_DOWN": "1"}},
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
	logPath := filepath.Join(root, "docker.log")

	for _, name := range []string{
		"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY",
		"GH_TOKEN", "GITHUB_TOKEN", "COPILOT_GITHUB_TOKEN",
		"CLAUDE_CONFIG_DIR", "CODEX_HOME", "COPILOT_HOME",
		"CAGED_GOLDEN_SET", "CAGED_GOLDEN_UNSET",
		"FAKE_DOCKER_DAEMON_DOWN", "FAKE_DOCKER_IMAGE_MISSING", "FAKE_DOCKER_RUN_EXIT", "FAKE_DOCKER_RUN_STDERR",
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
	for name, value := range tc.env {
		t.Setenv(name, strings.ReplaceAll(value, "$ROOT", root))
	}
	chdir(t, filepath.Join(root, "work", "project"))

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
	defer stdoutFile.Close()
	stderrFile, err := os.Create(filepath.Join(root, "stderr.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer stderrFile.Close()

	oldStdout, oldStderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdoutFile, stderrFile
	stdio := system.Stdio{In: strings.NewReader(""), Out: stdoutFile, Err: stderrFile}
	code := run(context.Background(), "test", args, stdio, platform, agents)
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
	if err := os.WriteFile(filepath.Join(root, "bin", "docker"), []byte(fakeDocker), 0o755); err != nil {
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

var buildDirPattern = regexp.MustCompile(`caged-runtime-[0-9]+`)

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
	return buildDirPattern.ReplaceAllString(text, "caged-runtime-N")
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

func chdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Error(err)
		}
	})
}
