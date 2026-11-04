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
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/images"
	"github.com/JeremyOrr4/caged/internal/system"
)

// goldenVersion is the caged version the golden tests run as.
const goldenVersion = "test"

// goldenPlatforms are the hosts the golden files describe. The linux files
// were captured from the original code; the darwin files differ from them
// only by the macOS rules in internal/docker/platform.go.
var goldenPlatforms = []system.Platform{
	{OS: "linux", Arch: "amd64", UID: 4242, GID: 4343},
	{OS: "darwin", Arch: "arm64", UID: 501, GID: 20},
}

// forEachPlatform runs test once under each of goldenPlatforms, as a subtest
// named after its OS, so the first-run and end-to-end tests are also checked
// under the macOS rules.
func forEachPlatform(t *testing.T, test func(t *testing.T, platform system.Platform)) {
	for _, platform := range goldenPlatforms {
		t.Run(platform.OS, func(t *testing.T) { test(t, platform) })
	}
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

// TestGoldenFilesMatchCases fails on a golden file that no case reads (left
// behind when a case was renamed or removed) and on a case defined twice. A
// case without a file already fails in TestGoldenDockerInvocations.
func TestGoldenFilesMatchCases(t *testing.T) {
	cases := map[string]bool{}
	for _, tc := range goldenCases() {
		if cases[tc.name+".golden"] {
			t.Errorf("golden case %q is defined twice", tc.name)
		}
		cases[tc.name+".golden"] = true
	}
	for _, platform := range goldenPlatforms {
		entries, err := os.ReadDir(filepath.Join("testdata", "golden", platform.OS))
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if !cases[entry.Name()] {
				t.Errorf("testdata/golden/%s/%s belongs to no golden case", platform.OS, entry.Name())
			}
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

	enterFakeDockerEnv(t, root, tc.env, tc.noDocker)

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
