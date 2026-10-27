package main

import (
	"os"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/system"
)

// TestNamesDifferingOnlyInCaseAreOneFolderOnMacOS pins W7-T8 (bugs.md
// Q-WB-7): macOS's default file system ignores case, so ~/.SSH is ~/.ssh and
// $XDG_STATE_HOME may name $XDG_CONFIG_HOME in other letters. The deny list
// and the approval-state overlap compared names exactly, so on macOS each
// case below mounted what they exist to keep out. On Linux the names are
// different folders, and they are mounted as before.
func TestNamesDifferingOnlyInCaseAreOneFolderOnMacOS(t *testing.T) {
	forEachPlatform(t, func(t *testing.T, platform system.Platform) {
		for _, tc := range []struct {
			name    string
			env     map[string]string
			setup   func(s *scenario)
			refusal string // on macOS, with $ROOT for the scenario root
			mount   string // in the docker log on Linux only
			fails   bool   // on macOS the refusal is a config error (exit 1, no docker run)
		}{
			{
				name:    "CLAUDE_CONFIG_DIR=~/.SSH",
				env:     map[string]string{"CLAUDE_CONFIG_DIR": "$ROOT/home/.SSH"},
				setup:   func(s *scenario) { s.write("home/.SSH/id_ed25519", "secret\n") },
				refusal: "config: refused $ROOT/home/.SSH (config path is denied); ~/.ssh is on caged's built-in deny list and cannot be allowed",
				mount:   "home/.SSH:/home/caged/.claude",
			},
			{
				name: "a link into ~/.SSH allowed by links.allow",
				setup: func(s *scenario) {
					s.write("home/.SSH/id_ed25519", "secret\n")
					if err := os.Symlink("../.SSH/id_ed25519", s.path("home/.claude/key")); err != nil {
						t.Fatal(err)
					}
					s.write("home/.config/caged/config.yaml", "links:\n  allow:\n    - path: ~/.SSH\n")
				},
				refusal: `~/.config/caged/config.yaml: links.allow entry "~/.SSH" is inside the denied path "~/.ssh"`,
				mount:   "home/.SSH/id_ed25519:/home/caged/.claude/key",
				fails:   true,
			},
			{
				name:    "XDG_STATE_HOME naming XDG_CONFIG_HOME in capitals",
				env:     map[string]string{"XDG_CONFIG_HOME": "$ROOT/xdg", "XDG_STATE_HOME": "$ROOT/XDG"},
				refusal: "config: refused $ROOT/xdg/caged (config path overlaps caged's approval state folder $ROOT/XDG/caged); set XDG_CONFIG_HOME and XDG_STATE_HOME to different folders",
				mount:   "xdg/caged:/home/caged/.config/caged",
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				s := newScenario(t, platform, tc.env)
				if tc.setup != nil {
					tc.setup(s)
				}
				s.invoke("--no-mount", "--trust-config") // creates or approves the config
				stderr, dockerLog, code := s.invoke("--verbose", "--no-mount")
				refusal := strings.ReplaceAll(tc.refusal, "$ROOT", s.root)
				if want := map[bool]int{false: 0, true: 1}[tc.fails && platform.OS == "darwin"]; code != want {
					t.Fatalf("exit code %d, want %d; stderr:\n%s", code, want, stderr)
				}
				if platform.OS == "darwin" {
					if !strings.Contains(stderr, refusal) {
						t.Errorf("stderr lacks %q:\n%s", refusal, stderr)
					}
					if strings.Contains(dockerLog, ".SSH") || strings.Contains(dockerLog, tc.mount) {
						t.Errorf("docker run mounts what the refusal keeps out:\n%s", dockerLog)
					}
					return
				}
				if strings.Contains(stderr, refusal) || !strings.Contains(dockerLog, tc.mount) {
					t.Errorf("on Linux %s is its own folder and must be mounted (%s); stderr:\n%s\ndocker log:\n%s", tc.name, tc.mount, stderr, dockerLog)
				}
			})
		}
	})
}

// TestHomeInOtherLettersIsABroadMountOnMacOS pins the W7-T8 review: -w (or
// a cd) naming the home folder in other letters mounted all of it, ~/.ssh
// included, without --allow-broad-mount on macOS.
func TestHomeInOtherLettersIsABroadMountOnMacOS(t *testing.T) {
	forEachPlatform(t, func(t *testing.T, platform system.Platform) {
		s := newScenario(t, platform, nil)
		s.write("HOME/notes.txt", "") // the home folder in capitals: one folder on macOS, another on Linux
		s.invoke("--no-config", "--no-mount")
		stderr, dockerLog, code := s.invoke("--no-config", "-w", s.path("HOME"))
		if platform.OS == "darwin" {
			if code != 1 || !strings.Contains(stderr, "refusing to mount your entire home directory") || strings.Contains(dockerLog, " run ") {
				t.Errorf("exit %d; stderr:\n%s\ndocker log:\n%s\nwant the home directory refused before docker run", code, stderr, dockerLog)
			}
			return
		}
		if code != 0 || !strings.Contains(dockerLog, s.path("HOME")+":/workspace/HOME") {
			t.Errorf("exit %d; stderr:\n%s\ndocker log:\n%s\nwant $ROOT/HOME mounted on Linux", code, stderr, dockerLog)
		}
	})
}
