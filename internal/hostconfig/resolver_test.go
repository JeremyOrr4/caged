package hostconfig

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestResolveAllConfigsMountsExistingConfigReadWrite(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	for _, dir := range []string{".claude", ".codex", ".gemini", ".pi/agent", ".config/pi"} {
		if err := os.MkdirAll(filepath.Join(home, dir), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range []string{
		".claude/settings.json",
		".claude/.credentials.json",
		".claude.json",
		".codex/config.toml",
		".codex/auth.json",
		".gemini/settings.json",
		".gemini/oauth_creds.json",
		".pi/agent/settings.json",
		".pi/agent/auth.json",
		".gitconfig",
	} {
		if err := os.WriteFile(filepath.Join(home, file), []byte("test"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	exists := func(path string) bool {
		_, err := os.Stat(path)
		return err == nil
	}

	report := ResolveAllConfigsReport(home, exists)
	defaultConfigs := report.Mounts
	defaultNames := make(map[string]struct{}, len(defaultConfigs))
	for _, mount := range defaultConfigs {
		defaultNames[mount.Name] = struct{}{}
	}
	for _, want := range []string{".claude", ".codex", ".gemini", ".pi", ".gitconfig"} {
		if _, ok := defaultNames[want]; !ok {
			t.Errorf("default staged configs missing %q; got %v", want, defaultNames)
		}
	}
	for _, mount := range defaultConfigs {
		if mount.Name == ".gitconfig" && mount.Writable {
			t.Errorf("shared git config should mount read-only")
		}
		if mount.Name != ".gitconfig" && !mount.Writable {
			t.Errorf("config %q should mount read-write", mount.Name)
		}
	}

	missingHome := t.TempDir()
	if got := ResolveAllConfigsReport(missingHome, exists).Mounts; len(got) != 0 {
		t.Errorf("ResolveAllConfigsReport() with no local configs = %v, want none", got)
	}
}

func TestConfigOverrideCannotMountHomeDirectory(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_HOME", home)
	report := ResolveAllConfigsReport(home, nil)
	for _, mount := range report.Mounts {
		if filepath.Clean(mount.HostPath) == filepath.Clean(home) {
			t.Fatal("agent config override mounted the entire home directory")
		}
	}
	foundRefusal := false
	for _, refusal := range report.Refused {
		if strings.Contains(refusal, "home directory") {
			foundRefusal = true
		}
	}
	if !foundRefusal {
		t.Fatalf("home-directory override was not reported as refused: %+v", report)
	}
}

func TestConfigSymlinkOutsideHomeIsRefused(t *testing.T) {
	home, outside := t.TempDir(), t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CODEX_HOME", "")
	t.Setenv("COPILOT_HOME", "")
	if err := os.WriteFile(filepath.Join(outside, "secret"), []byte("no"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(home, ".codex")); err != nil {
		t.Fatal(err)
	}
	report := ResolveAllConfigsReport(home, nil)
	for _, mount := range report.Mounts {
		if filepath.Clean(mount.HostPath) == filepath.Clean(outside) {
			t.Fatalf("mounted config symlink target outside home: %+v", mount)
		}
	}
	if len(report.Refused) == 0 {
		t.Fatalf("outside config symlink was not refused: %+v", report)
	}
}

func TestConfigSymlinkOutsideConfigDirectoryIsRefused(t *testing.T) {
	home := t.TempDir()
	configDir := filepath.Join(home, ".codex")
	privateDir := filepath.Join(home, "private")
	if err := os.Mkdir(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(privateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(privateDir, "secret"), []byte("no"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(privateDir, filepath.Join(configDir, "private-link")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CODEX_HOME", "")
	t.Setenv("COPILOT_HOME", "")
	report := ResolveAllConfigsReport(home, nil)
	for _, mount := range report.Mounts {
		if mount.Name == ".codex" {
			t.Fatal("config tree with a symlink outside the config directory was mounted")
		}
	}
	if len(report.Refused) == 0 {
		t.Fatalf("config directory escape was not reported as refused: %+v", report)
	}
}

func TestSpecialFileInsideConfigTreeIsRefused(t *testing.T) {
	home := t.TempDir()
	configDir := filepath.Join(home, ".gemini")
	if err := os.Mkdir(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(configDir, "agent.pipe"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CODEX_HOME", "")
	t.Setenv("COPILOT_HOME", "")
	report := ResolveAllConfigsReport(home, nil)
	for _, mount := range report.Mounts {
		if mount.Name == ".gemini" {
			t.Fatal("config tree containing a socket was mounted")
		}
	}
	if len(report.Refused) == 0 {
		t.Fatalf("special file was not reported as refused: %+v", report)
	}
}

func TestSharedGitConfigMustBeRegularFile(t *testing.T) {
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, ".gitconfig"), 0o700); err != nil {
		t.Fatal(err)
	}
	report := ResolveAllConfigsReport(home, nil)
	for _, mount := range report.Mounts {
		if mount.Name == ".gitconfig" {
			t.Fatal("directory mounted as shared git config")
		}
	}
	if len(report.Refused) == 0 {
		t.Fatal("non-file gitconfig was not reported as refused")
	}
}
