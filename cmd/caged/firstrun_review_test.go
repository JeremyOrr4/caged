package main

import (
	"os"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/system"
)

func TestAChangeMadeAfterApprovalIsShownAndStopsTheRun(t *testing.T) {
	forEachPlatform(t, func(t *testing.T, platform system.Platform) {
		s := newScenario(t, platform, nil)
		if _, _, code := s.invoke("--no-mount"); code != 0 {
			t.Fatal("first run failed")
		}
		// What an agent could do from inside the container.
		s.write("home/.config/caged/config.yaml", s.read("home/.config/caged/config.yaml")+"links:\n  allow:\n    - path: ~/private\n")

		stderr, dockerLog, code := s.invoke("--no-mount")
		if code != 1 {
			t.Errorf("exit code %d; want 1 with no terminal to approve", code)
		}
		for _, want := range []string{"CAGED CONFIG CHANGED: ~/.config/caged/config.yaml", "+links:", "+    - path: ~/private", "--trust-config"} {
			if !strings.Contains(stderr, want) {
				t.Errorf("stderr lacks %q:\n%s", want, stderr)
			}
		}
		if strings.Contains(dockerLog, "run") {
			t.Errorf("a container ran with an unapproved config:\n%s", dockerLog)
		}

		stderr, _, code = s.invoke("--no-mount", "--trust-config")
		if code != 0 || !strings.Contains(stderr, "+    - path: ~/private") {
			t.Errorf("--trust-config: code %d, stderr:\n%s; want success that still shows the diff", code, stderr)
		}
		if stderr, _, code = s.invoke("--no-mount"); code != 0 || strings.Contains(stderr, "CHANGED") {
			t.Errorf("after approval: code %d, stderr:\n%s; want a quiet run", code, stderr)
		}
	})
}

func TestNoConfigStillReviewsAChangedFile(t *testing.T) {
	forEachPlatform(t, func(t *testing.T, platform system.Platform) {
		s := newScenario(t, platform, nil)
		s.invoke("--no-mount")
		s.write("home/.config/caged/config.yaml", "image:\n  apt: [nmap]\n")
		if _, _, code := s.invoke("--no-mount", "--no-config"); code != 1 {
			t.Errorf("exit code %d; want 1: --no-config still reads image.apt from the file", code)
		}
	})
}

func TestDoctorAndListShowAPendingChangeButStillWork(t *testing.T) {
	forEachPlatform(t, func(t *testing.T, platform system.Platform) {
		for _, command := range []string{"doctor", "list"} {
			t.Run(command, func(t *testing.T) {
				s := newScenario(t, platform, nil)
				s.invoke(command)
				s.write("home/.config/caged/config.yaml", "image:\n  apt: [nmap]\n")
				stderr, _, code := s.invoke(command)
				if code != 0 || !strings.Contains(stderr, "CAGED CONFIG CHANGED") || !strings.Contains(stderr, "Not approved yet") {
					t.Errorf("code %d, stderr:\n%s; want a pending-change report and success", code, stderr)
				}
				if stderr, _, code := s.invoke("--no-mount"); code != 1 || !strings.Contains(stderr, "CAGED CONFIG CHANGED") {
					t.Errorf("a run after %s: code %d; want it still to need approval (%s never approves)", command, code, command)
				}
			})
		}
	})
}

func TestApprovedCopyLivesOutsideTheMountedConfigDirectory(t *testing.T) {
	forEachPlatform(t, func(t *testing.T, platform system.Platform) {
		s := newScenario(t, platform, nil)
		s.invoke("--no-mount")
		entries, err := os.ReadDir(s.path("home/.local/state/caged"))
		if err != nil || len(entries) != 1 {
			t.Fatalf("state dir = %v, %v; want one approved copy", entries, err)
		}
		if entries, _ := os.ReadDir(s.path("home/.config/caged")); len(entries) != 1 {
			t.Errorf("config dir has %d entries; want only config.yaml", len(entries))
		}
	})
}

// TestTerminalControlCharactersInAChangedConfigAreEscaped pins WB-T5 (bugs.md
// Q-APP-5) end to end. YAML refuses escape characters, but a bare carriage
// return is a line break to it, and on a terminal it moves the cursor back so
// the rest of the line overwrites what came before: an added links.allow root
// could be shown as a harmless comment. caged prints it escaped.
func TestTerminalControlCharactersInAChangedConfigAreEscaped(t *testing.T) {
	forEachPlatform(t, func(t *testing.T, platform system.Platform) {
		s := newScenario(t, platform, nil)
		if _, _, code := s.invoke("--no-mount"); code != 0 {
			t.Fatal("first run failed")
		}
		s.write("home/.config/caged/config.yaml", s.read("home/.config/caged/config.yaml")+"links:\n  allow:\n    - path: ~/private\r# only a comment\n")
		stderr, _, code := s.invoke("--no-mount")
		if code != 1 || strings.Contains(stderr, "\r") || !strings.Contains(stderr, `+    - path: ~/private\r# only a comment`) {
			t.Fatalf("exit code %d; stderr holds a raw carriage return or lacks the escaped line:\n%q", code, stderr)
		}
	})
}
