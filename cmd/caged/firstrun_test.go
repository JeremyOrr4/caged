package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/system"
)

// scenario is a fake home with a fake docker, driven through run() like the
// golden tests, but with state that persists from one invocation to the next.
type scenario struct {
	t        *testing.T
	root     string
	platform system.Platform
}

func newScenario(t *testing.T, platform system.Platform, env map[string]string) *scenario {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake docker binary is a POSIX shell script")
	}
	root := t.TempDir()
	writeGoldenFixtures(t, root)
	writeFakeDockerState(t, root, goldenCase{}, platform)
	enterFakeDockerEnv(t, root, env, false)
	return &scenario{t: t, root: root, platform: platform}
}

func (s *scenario) path(rel string) string { return filepath.Join(s.root, rel) }

// invoke runs caged with args and no input, and returns what it printed and
// the docker commands it ran. The docker log starts empty for each call.
func (s *scenario) invoke(args ...string) (stderr, dockerLog string, code int) {
	s.t.Helper()
	_ = os.Remove(s.path("docker.log"))
	_, stderr, code, _ = executeRun(s.t, s.root, args, s.platform, originalAgents(s.t))
	logBytes, err := os.ReadFile(s.path("docker.log"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		s.t.Fatal(err)
	}
	return stderr, string(logBytes), code
}

func (s *scenario) read(rel string) string {
	s.t.Helper()
	data, err := os.ReadFile(s.path(rel))
	if err != nil {
		s.t.Fatal(err)
	}
	return string(data)
}

func (s *scenario) write(rel, content string) {
	s.t.Helper()
	if err := os.MkdirAll(filepath.Dir(s.path(rel)), 0o755); err != nil {
		s.t.Fatal(err)
	}
	if err := os.WriteFile(s.path(rel), []byte(content), 0o644); err != nil {
		s.t.Fatal(err)
	}
}

func (s *scenario) exists(rel string) bool {
	_, err := os.Lstat(s.path(rel))
	return err == nil
}

const cagedMount = "home/.config/caged:/home/caged/.config/caged"

func TestFirstRunCreatesAPrivateStarterConfigAndMountsItWritable(t *testing.T) {
	forEachPlatform(t, func(t *testing.T, platform system.Platform) {
		s := newScenario(t, platform, nil)
		stderr, dockerLog, code := s.invoke("--no-mount")
		if code != 0 {
			t.Fatalf("exit code %d; stderr:\n%s", code, stderr)
		}
		if !strings.Contains(stderr, "created starter config ~/.config/caged/config.yaml") {
			t.Errorf("stderr lacks the creation notice:\n%s", stderr)
		}
		info, err := os.Stat(s.path("home/.config/caged/config.yaml"))
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("config.yaml = %v, %v; want a 0600 file", info, err)
		}
		if dir, _ := os.Stat(s.path("home/.config/caged")); dir.Mode().Perm() != 0o700 {
			t.Errorf("config dir mode = %o; want 700", dir.Mode().Perm())
		}
		if !strings.Contains(dockerLog, s.root+"/"+cagedMount+"\n") {
			t.Errorf("docker run lacks a read/write mount of the config dir:\n%s", dockerLog)
		}
		if strings.Contains(dockerLog, "/home/caged/.config/caged:ro") {
			t.Error("the config dir is mounted read-only")
		}
	})
}

func TestSecondRunLeavesTheFileAloneAndSaysNothing(t *testing.T) {
	forEachPlatform(t, func(t *testing.T, platform system.Platform) {
		s := newScenario(t, platform, nil)
		s.invoke("--no-mount")
		first := s.read("home/.config/caged/config.yaml")
		stderr, _, code := s.invoke("--no-mount")
		if code != 0 || strings.Contains(stderr, "created") || strings.Contains(stderr, "CHANGED") {
			t.Errorf("second run: code %d, stderr:\n%s; want quiet success", code, stderr)
		}
		if got := s.read("home/.config/caged/config.yaml"); got != first {
			t.Error("the config file changed between runs")
		}
	})
}

func TestAnExistingConfigIsNeverReplacedOrReprivileged(t *testing.T) {
	forEachPlatform(t, func(t *testing.T, platform system.Platform) {
		s := newScenario(t, platform, nil)
		s.write("home/.config/caged/config.yaml", "image:\n  apt: [jq]\n")
		if _, _, code := s.invoke("--no-mount"); code != 0 {
			t.Fatalf("exit code %d", code)
		}
		if got := s.read("home/.config/caged/config.yaml"); got != "image:\n  apt: [jq]\n" {
			t.Errorf("config = %q; want it untouched", got)
		}
		if info, _ := os.Stat(s.path("home/.config/caged/config.yaml")); info.Mode().Perm() != 0o644 {
			t.Errorf("mode = %o; want the 644 the user chose", info.Mode().Perm())
		}
	})
}

func TestAbsoluteXDGConfigHomeOutsideHomeIsUsedAndMounted(t *testing.T) {
	forEachPlatform(t, func(t *testing.T, platform system.Platform) {
		s := newScenario(t, platform, map[string]string{"XDG_CONFIG_HOME": "$ROOT/xdg"})
		stderr, dockerLog, code := s.invoke("--no-mount")
		if code != 0 {
			t.Fatalf("exit code %d; stderr:\n%s", code, stderr)
		}
		if !s.exists("xdg/caged/config.yaml") || s.exists("home/.config/caged") {
			t.Error("the starter config is not only under $XDG_CONFIG_HOME/caged")
		}
		if !strings.Contains(dockerLog, s.root+"/xdg/caged:/home/caged/.config/caged\n") {
			t.Errorf("docker run lacks the XDG config dir mount:\n%s", dockerLog)
		}
	})
}

func TestRelativeXDGConfigHomeIsIgnored(t *testing.T) {
	forEachPlatform(t, func(t *testing.T, platform system.Platform) {
		s := newScenario(t, platform, map[string]string{"XDG_CONFIG_HOME": "relative"})
		if _, _, code := s.invoke("--no-mount"); code != 0 {
			t.Fatalf("exit code %d", code)
		}
		if !s.exists("home/.config/caged/config.yaml") {
			t.Error("a relative XDG_CONFIG_HOME was not ignored")
		}
	})
}

func TestCommandsThatNeedNoConfigDoNotCreateIt(t *testing.T) {
	forEachPlatform(t, func(t *testing.T, platform system.Platform) {
		for _, args := range [][]string{{"--version"}, {"--help"}, {"help"}, {"clean", "--dry-run"}, {"clean"}, {"__complete", ""}} {
			t.Run(strings.Join(args, " "), func(t *testing.T) {
				s := newScenario(t, platform, nil)
				s.invoke(args...)
				if s.exists("home/.config/caged") || s.exists("home/.local/state/caged") {
					t.Errorf("caged %v created config or state", args)
				}
			})
		}
	})
}

func TestNoConfigSkipsCreationAndTheMount(t *testing.T) {
	forEachPlatform(t, func(t *testing.T, platform system.Platform) {
		s := newScenario(t, platform, nil)
		stderr, dockerLog, code := s.invoke("--no-mount", "--no-config")
		if code != 0 {
			t.Fatalf("exit code %d; stderr:\n%s", code, stderr)
		}
		if s.exists("home/.config/caged") {
			t.Error("--no-config created the config directory")
		}
		if strings.Contains(dockerLog, "/home/caged/.config/caged") {
			t.Errorf("--no-config mounted the config dir:\n%s", dockerLog)
		}
	})
}

func TestDoctorListAndBuildCreateTheConfigToo(t *testing.T) {
	forEachPlatform(t, func(t *testing.T, platform system.Platform) {
		for _, args := range [][]string{{"doctor"}, {"list"}, {"build"}} {
			t.Run(args[0], func(t *testing.T) {
				s := newScenario(t, platform, nil)
				s.invoke(args...)
				if !s.exists("home/.config/caged/config.yaml") {
					t.Errorf("caged %v did not create the config", args)
				}
			})
		}
	})
}

func TestCreationFailureIsClearAndStopsBeforeDocker(t *testing.T) {
	forEachPlatform(t, func(t *testing.T, platform system.Platform) {
		if os.Getuid() == 0 {
			t.Skip("root ignores directory permissions")
		}
		// The folder exists, so it would be mounted: a file the container wrote
		// there would be approved without a question on the next run (D-55).
		s := newScenario(t, platform, map[string]string{"XDG_CONFIG_HOME": "$ROOT/readonly"})
		if err := os.MkdirAll(s.path("readonly/caged"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(s.path("readonly/caged"), 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(s.path("readonly/caged"), 0o700) })
		stderr, dockerLog, code := s.invoke("--no-mount")
		if code != 1 {
			t.Errorf("exit code %d; want 1", code)
		}
		for _, want := range []string{"create " + s.path("readonly/caged"), "XDG_CONFIG_HOME", "permission denied"} {
			if !strings.Contains(stderr, want) {
				t.Errorf("stderr lacks %q:\n%s", want, stderr)
			}
		}
		if dockerLog != "" {
			t.Errorf("docker was called after the failure:\n%s", dockerLog)
		}
	})
}

// TestAConfigFolderThatCannotBeCreatedWarnsAndTheRunGoesOn pins WB-T4
// (bugs.md B-APP-2): with no folder there is nothing to review or mount, so a
// read-only parent is a warning and the run goes on without the caged mount.
func TestAConfigFolderThatCannotBeCreatedWarnsAndTheRunGoesOn(t *testing.T) {
	forEachPlatform(t, func(t *testing.T, platform system.Platform) {
		if os.Getuid() == 0 {
			t.Skip("root ignores directory permissions")
		}
		s := newScenario(t, platform, map[string]string{"XDG_CONFIG_HOME": "$ROOT/readonly"})
		if err := os.Mkdir(s.path("readonly"), 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(s.path("readonly"), 0o700) })
		stderr, dockerLog, code := s.invoke("--no-mount")
		if code != 0 {
			t.Errorf("exit code %d; want 0", code)
		}
		if want := "caged: warning: create " + s.path("readonly/caged") + ": "; !strings.HasPrefix(stderr, want) || !strings.Contains(stderr, "XDG_CONFIG_HOME") {
			t.Errorf("stderr does not start with %q:\n%s", want, stderr)
		}
		if !strings.Contains(dockerLog, "run\n") || strings.Contains(dockerLog, s.path("readonly")) {
			t.Errorf("want a run without the caged folder; docker log:\n%s", dockerLog)
		}

		// The WB-T4 reviewer's case: under rootless Docker a container can own a
		// folder this user cannot write, and create the config there. The file is
		// shown and asked about, never taken as the first copy.
		if err := os.Chmod(s.path("readonly"), 0o700); err != nil {
			t.Fatal(err)
		}
		s.write("readonly/caged/config.yaml", "image:\n  apt: [nmap]\n")
		stderr, dockerLog, code = s.invoke("--no-mount")
		if code != 1 || !strings.Contains(stderr, "CAGED CONFIG CHANGED") || !strings.Contains(stderr, "+  apt: [nmap]") {
			t.Errorf("exit code %d, stderr:\n%s\nwant the new file shown and the run stopped", code, stderr)
		}
		if strings.Contains(dockerLog, "run\n") {
			t.Errorf("a container ran with an unapproved config:\n%s", dockerLog)
		}
	})
}

// TestADanglingConfigFolderSymlinkStopsBeforeDocker: the WB-T4 reviewer's
// reproduction. A dangling ~/.config/caged symlink into a folder the
// container can write (here ~/.claude) is not a missing folder: once the
// container creates the target, caged would read the file through the link and
// approve it unasked. So it stays an error, as before WB-T4.
func TestADanglingConfigFolderSymlinkStopsBeforeDocker(t *testing.T) {
	forEachPlatform(t, func(t *testing.T, platform system.Platform) {
		s := newScenario(t, platform, nil)
		if err := os.MkdirAll(s.path("home/.config"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(s.path("home/.claude/caged"), s.path("home/.config/caged")); err != nil {
			t.Fatal(err)
		}
		stderr, dockerLog, code := s.invoke("--no-mount")
		if code != 1 || !strings.Contains(stderr, "create ~/.config/caged: ") || strings.Contains(stderr, "warning") {
			t.Errorf("exit code %d, stderr:\n%s\nwant 1 and the creation error", code, stderr)
		}
		if dockerLog != "" {
			t.Errorf("docker was called:\n%s", dockerLog)
		}
	})
}

// TestADanglingConfigSymlinkStopsBeforeDocker pins WB-T4 (bugs.md Q-APP-6).
func TestADanglingConfigSymlinkStopsBeforeDocker(t *testing.T) {
	forEachPlatform(t, func(t *testing.T, platform system.Platform) {
		s := newScenario(t, platform, nil)
		if err := os.MkdirAll(s.path("home/.config/caged"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(s.path("nowhere/config.yaml"), s.path("home/.config/caged/config.yaml")); err != nil {
			t.Fatal(err)
		}
		stderr, dockerLog, code := s.invoke("--no-mount")
		if code != 1 || !strings.Contains(stderr, "~/.config/caged/config.yaml exists but cannot be read") {
			t.Errorf("exit code %d, stderr:\n%s\nwant 1 and the unreadable file named", code, stderr)
		}
		if dockerLog != "" {
			t.Errorf("docker was called:\n%s", dockerLog)
		}
	})
}
