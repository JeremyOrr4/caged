package main

import (
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/JeremyOrr4/caged/internal/system"
)

// TestXDGConfigHomeEqualToXDGStateHomeIsNotMounted pins WB-B1 (bugs.md
// Q-WB-1): with both variables naming one folder, caged's config folder holds
// the approved copies, so it is refused instead of mounted read/write.
func TestXDGConfigHomeEqualToXDGStateHomeIsNotMounted(t *testing.T) {
	forEachPlatform(t, func(t *testing.T, platform system.Platform) {
		s := newScenario(t, platform, map[string]string{"XDG_CONFIG_HOME": "$ROOT/xdg", "XDG_STATE_HOME": "$ROOT/xdg"})
		s.invoke("--no-mount") // creates the starter config and records its approval
		stderr, dockerLog, code := s.invoke("--verbose", "--no-mount")
		if code != 0 {
			t.Fatalf("exit code %d; stderr:\n%s", code, stderr)
		}
		refusal := "config: refused " + s.root + "/xdg/caged (config path overlaps caged's approval state folder " + s.root + "/xdg/caged); set XDG_CONFIG_HOME and XDG_STATE_HOME to different folders"
		if !strings.Contains(stderr, refusal) {
			t.Errorf("stderr lacks %q:\n%s", refusal, stderr)
		}
		if strings.Contains(dockerLog, "/home/caged/.config/caged") {
			t.Errorf("docker run mounts the folder holding the approved copies:\n%s", dockerLog)
		}
	})
}

// TestAnOverrideNamingADeniedFolderIsNotMounted pins WB-B2 (bugs.md
// Q-WB-2): CLAUDE_CONFIG_DIR=~/.ssh used to mount ~/.ssh read/write as
// Claude's config.
func TestAnOverrideNamingADeniedFolderIsNotMounted(t *testing.T) {
	forEachPlatform(t, func(t *testing.T, platform system.Platform) {
		s := newScenario(t, platform, map[string]string{"CLAUDE_CONFIG_DIR": "$ROOT/home/.ssh"})
		s.write("home/.ssh/id_ed25519", "secret\n")
		stderr, dockerLog, code := s.invoke("--verbose", "--no-mount")
		if code != 0 {
			t.Fatalf("exit code %d; stderr:\n%s", code, stderr)
		}
		refusal := "config: refused " + s.root + "/home/.ssh (config path is denied); ~/.ssh is on caged's built-in deny list and cannot be allowed"
		if !strings.Contains(stderr, refusal) {
			t.Errorf("stderr lacks %q:\n%s", refusal, stderr)
		}
		if strings.Contains(dockerLog, ".ssh") {
			t.Errorf("docker run mounts ~/.ssh:\n%s", dockerLog)
		}
	})
}

// TestADockerCLIKilledByASignalExitsLikeIt pins WB-B3 (bugs.md Q-WB-3):
// when docker run is killed, caged exits with the status a shell gives a
// killed process, 128 plus the signal number, instead of 255.
func TestADockerCLIKilledByASignalExitsLikeIt(t *testing.T) {
	forEachPlatform(t, func(t *testing.T, platform system.Platform) {
		s := newScenario(t, platform, nil)
		if err := os.Rename(s.path("bin/docker"), s.path("bin/docker-real")); err != nil {
			t.Fatal(err)
		}
		s.write("bin/docker", "#!/bin/sh\nif [ \"$1\" = run ]; then kill -KILL $$; fi\nexec \""+s.path("bin/docker-real")+"\" \"$@\"\n")
		if err := os.Chmod(s.path("bin/docker"), 0o755); err != nil {
			t.Fatal(err)
		}
		stderr, _, code := s.invoke("--no-mount")
		if code != 137 || !strings.HasSuffix(stderr, "\nsignal: killed\n") {
			t.Fatalf("exit code %d, stderr:\n%s; want 137 and docker's end reported", code, stderr)
		}
	})
}

// TestAFIFOConfigFileStopsTheCommandAtOnce pins WB-B4 (bugs.md Q-WB-5): a
// container can mkfifo config.yaml in the folder it may write; the next caged
// command used to block reading it until killed.
func TestAFIFOConfigFileStopsTheCommandAtOnce(t *testing.T) {
	forEachPlatform(t, func(t *testing.T, platform system.Platform) {
		s := newScenario(t, platform, nil)
		if err := os.MkdirAll(s.path("home/.config/caged"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := syscall.Mkfifo(s.path("home/.config/caged/config.yaml"), 0o600); err != nil {
			t.Fatal(err)
		}
		type result struct {
			stderr, dockerLog string
			code              int
		}
		done := make(chan result, 1)
		go func() {
			stderr, dockerLog, code := s.invoke("--no-mount")
			done <- result{stderr, dockerLog, code}
		}()
		select {
		case got := <-done:
			if got.code != 1 || !strings.Contains(got.stderr, "read ~/.config/caged/config.yaml: not a regular file") || strings.Contains(got.dockerLog, "run") {
				t.Fatalf("exit code %d, stderr:\n%s\ndocker:\n%s\nwant the file refused before docker runs", got.code, got.stderr, got.dockerLog)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("caged blocked reading a FIFO config.yaml")
		}
	})
}

// TestDoctorEscapesALinkNameAContainerChose pins WB-B5 (bugs.md Q-WB-6): a
// container can name a link in Claude's folder with an escape sequence, and
// caged doctor reports the refused link on stdout.
func TestDoctorEscapesALinkNameAContainerChose(t *testing.T) {
	forEachPlatform(t, func(t *testing.T, platform system.Platform) {
		s := newScenario(t, platform, nil)
		if err := os.Symlink(s.path("outside/pi/settings.json"), s.path("home/.claude/x\x1b[2J\ry")); err != nil {
			t.Fatal(err)
		}
		stdout, stderr, code, _ := executeRun(t, s.root, []string{"doctor"}, platform, originalAgents(t))
		if code != 0 || strings.ContainsAny(stdout, "\x1b\r") || !strings.Contains(stdout, s.root+`/home/.claude/x\x1b[2J\ry (symlink target`) {
			t.Fatalf("exit code %d, stdout:\n%q\nstderr:\n%s\nwant the link name escaped", code, stdout, stderr)
		}
	})
}

// TestADenyRootThatDoesNotExistYetIsDeniedBehindASymlinkedHome pins the
// WB-B2 review (bugs.md Q-WB-2): with home a symlink, as on Fedora Silverblue
// (/home -> var/home), a built-in deny root that does not exist yet, such as
// ~/.local/share/keyrings, kept only its unresolved name, so a folder holding
// it was mounted read/write, as a plain root and as a link target, and the
// container could create the keyring there.
func TestADenyRootThatDoesNotExistYetIsDeniedBehindASymlinkedHome(t *testing.T) {
	forEachPlatform(t, func(t *testing.T, platform system.Platform) {
		const keyrings = "~/.local/share/keyrings is on caged's built-in deny list and cannot be allowed"
		for _, tc := range []struct {
			name   string
			env    map[string]string
			config string // config.yaml, which the link needs
			want   string
		}{
			{name: "plain root", env: map[string]string{"CODEX_HOME": "$ROOT/homelink/.local/share"}, want: keyrings},
			{name: "link target", config: "links:\n  allow:\n    - path: ~/.local\n", want: keyrings},
			{name: "links.deny entry", config: "links:\n  allow:\n    - path: ~/.local/share/applications\n    - path: ~/.local/share/other\n  deny: [~/.local/share/applications/private]\n", want: "~/.local/share/applications/private is denied by links.deny"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				s := newScenario(t, platform, tc.env)
				if err := os.Symlink(s.path("home"), s.path("homelink")); err != nil {
					t.Fatal(err)
				}
				t.Setenv("HOME", s.path("homelink"))
				s.write("home/.local/share/applications/a.desktop", "")
				args := []string{"--verbose", "--no-mount"}
				if tc.config != "" {
					target := "homelink/.local/share"
					if strings.Contains(tc.config, "deny:") {
						target = "homelink/.local/share/applications"
					}
					if err := os.Symlink(s.path(target), s.path("home/.claude/share")); err != nil {
						t.Fatal(err)
					}
					s.write("home/.config/caged/config.yaml", tc.config)
					args = append(args, "--trust-config")
				}
				stderr, dockerLog, code := s.invoke(args...)
				if code != 0 {
					t.Fatalf("exit code %d; stderr:\n%s", code, stderr)
				}
				if !strings.Contains(stderr, tc.want) {
					t.Errorf("stderr lacks %q:\n%s", tc.want, stderr)
				}
				if strings.Contains(dockerLog, ".local/share") {
					t.Errorf("docker run mounts a folder holding ~/.local/share/keyrings:\n%s", dockerLog)
				}
			})
		}
	})
}

// TestAHugeConfigFileStopsTheCommandAtOnce pins WB-B6 (bugs.md Q-WB-9): a
// container can truncate config.yaml to a sparse 200 GB; every command used to
// read it whole.
func TestAHugeConfigFileStopsTheCommandAtOnce(t *testing.T) {
	forEachPlatform(t, func(t *testing.T, platform system.Platform) {
		s := newScenario(t, platform, nil)
		s.write("home/.config/caged/config.yaml", "")
		if err := os.Truncate(s.path("home/.config/caged/config.yaml"), 200<<30); err != nil {
			t.Fatal(err)
		}
		stderr, dockerLog, code := s.invoke("--no-mount")
		if code != 1 || !strings.Contains(stderr, "read ~/.config/caged/config.yaml: larger than 1048576 bytes") || strings.Contains(dockerLog, "run") {
			t.Fatalf("exit code %d, stderr:\n%s\ndocker:\n%s\nwant the file refused before docker runs", code, stderr, dockerLog)
		}
	})
}
