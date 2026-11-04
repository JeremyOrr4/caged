package hostconfig

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/pathpolicy"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
	"github.com/JeremyOrr4/caged/internal/testkit"
)

// policies is a fixed link policy for tests.
type policies struct {
	allow         []string
	readOnly      bool     // the allow roots are read-only
	readOnlyAllow []string // further allow roots, always read-only
	own           []string // caged's own folders, as userconfig.Store.LinkPolicy passes them
	deny          []string // links.deny roots
}

func (p policies) LinkPolicy(home string) (*pathpolicy.Policy, error) {
	resolved, err := filepath.EvalSymlinks(home)
	if err != nil {
		resolved = home
	}
	cfg := pathpolicy.Config{Home: home, ResolvedHome: resolved, Source: "~/.config/caged/config.yaml", Field: "links.allow"}
	allowRoot := func(path string, readOnly bool) {
		forms := []string{path}
		if r, err := filepath.EvalSymlinks(path); err == nil {
			forms = append(forms, r)
		}
		cfg.Allow = append(cfg.Allow, pathpolicy.Root{Path: path, Forms: forms, ReadOnly: readOnly})
	}
	for _, path := range p.allow {
		allowRoot(path, p.readOnly)
	}
	for _, path := range p.readOnlyAllow {
		allowRoot(path, true)
	}
	for _, path := range pathpolicy.BuiltinDeny(home, p.own...) {
		forms := []string{path}
		if r, err := filepath.EvalSymlinks(path); err == nil {
			forms = append(forms, r)
		}
		cfg.Deny = append(cfg.Deny, pathpolicy.Root{Path: path, Forms: forms, Builtin: true})
	}
	for _, path := range p.deny {
		cfg.Deny = append(cfg.Deny, pathpolicy.Root{Path: path, Forms: []string{path}})
	}
	return pathpolicy.New(cfg)
}

// resolveConfigs runs the production resolver on the real filesystem with the
// built-in agents, the environment variables vars, and no allowed link roots.
func resolveConfigs(t *testing.T, home string, vars map[string]string) plan.ConfigReport {
	t.Helper()
	return resolveConfigsWith(t, home, vars, policies{})
}

// resolveConfigsWith is resolveConfigs with the given link policy.
func resolveConfigsWith(t *testing.T, home string, vars map[string]string, links policies) plan.ConfigReport {
	t.Helper()
	report, err := NewResolver(Deps{FS: system.FS{}, Env: testkit.Env{Vars: vars}, Agents: agent.Builtin().All(), Shared: DefaultSharedFiles(), Policies: links}).Resolve(home)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func TestResolveOutsideConfigThroughAliasedParentIsNotTreatedAsLink(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	outside := t.TempDir()
	if err := os.Mkdir(filepath.Join(outside, "copilot"), 0o700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "outside")
	if err := os.Symlink(outside, alias); err != nil {
		t.Fatal(err)
	}

	report := resolveConfigs(t, home, map[string]string{"COPILOT_HOME": filepath.Join(alias, "copilot")})
	for _, refusal := range report.Refused {
		if refusal.Path == filepath.Join(alias, "copilot") {
			if refusal.Reason != "resolves outside home" {
				t.Fatalf("refusal = %+v; want direct outside-home refusal", refusal)
			}
			return
		}
	}
	t.Fatalf("missing refusal for %s: %+v", filepath.Join(alias, "copilot"), report.Refused)
}

func TestResolveAllConfigsMountsExistingConfigReadWrite(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	for _, dir := range []string{".claude", ".codex", ".gemini", ".pi/agent", ".config/pi", ".config/gh"} {
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
		".config/gh/hosts.yml",
		".gitconfig",
	} {
		if err := os.WriteFile(filepath.Join(home, file), []byte("test"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	report := resolveConfigs(t, home, nil)
	defaultConfigs := report.Mounts
	defaultNames := make(map[string]struct{}, len(defaultConfigs))
	for _, mount := range defaultConfigs {
		defaultNames[mount.Name] = struct{}{}
	}
	for _, want := range []string{".claude", ".codex", ".gemini", ".pi", ".config/gh", ".gitconfig"} {
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
	if got := resolveConfigs(t, missingHome, nil).Mounts; len(got) != 0 {
		t.Errorf("Resolve() with no local configs = %v, want none", got)
	}
}

func TestConfigOverrideCannotMountHomeDirectory(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o700); err != nil {
		t.Fatal(err)
	}
	report := resolveConfigs(t, home, map[string]string{"CODEX_HOME": home})
	for _, mount := range report.Mounts {
		if filepath.Clean(mount.HostPath) == filepath.Clean(home) {
			t.Fatal("agent config override mounted the entire home directory")
		}
	}
	foundRefusal := false
	for _, refusal := range report.Refused {
		if strings.Contains(refusal.String(), "home directory") {
			foundRefusal = true
		}
	}
	if !foundRefusal {
		t.Fatalf("home-directory override was not reported as refused: %+v", report)
	}
}

func TestConfigSymlinkOutsideHomeIsRefused(t *testing.T) {
	t.Parallel()
	home, outside := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret"), []byte("no"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(home, ".codex")); err != nil {
		t.Fatal(err)
	}
	report := resolveConfigs(t, home, nil)
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
	t.Parallel()
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
	if err := os.Symlink("../private", filepath.Join(configDir, "private-link")); err != nil {
		t.Fatal(err)
	}
	report := resolveConfigs(t, home, nil)
	resolvedConfigDir, err := filepath.EvalSymlinks(configDir)
	if err != nil {
		t.Fatal(err)
	}
	resolvedPrivateDir, err := filepath.EvalSymlinks(privateDir)
	if err != nil {
		t.Fatal(err)
	}
	mountedCodex := false
	for _, mount := range report.Mounts {
		if mount.Name == ".codex" && mount.HostPath == resolvedConfigDir {
			mountedCodex = true
		}
		if mount.HostPath == resolvedPrivateDir {
			t.Fatalf("a symlink target outside the config directory and outside links.allow was mounted: %+v", mount)
		}
	}
	if !mountedCodex {
		t.Fatalf("the config tree was not mounted whole next to the refused link: %+v", report)
	}
	link := filepath.Join(resolvedConfigDir, "private-link")
	if len(report.Refused) != 1 || report.Refused[0].Path != link || !strings.Contains(report.Refused[0].Hint, "under links.allow") {
		t.Fatalf("config directory escape was not refused with a hint: %+v", report.Refused)
	}
}

func TestSpecialFileInsideConfigTreeIsRefused(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	configDir := filepath.Join(home, ".gemini")
	if err := os.Mkdir(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(configDir, "agent.pipe"), 0o600); err != nil {
		t.Fatal(err)
	}
	report := resolveConfigs(t, home, nil)
	for _, mount := range report.Mounts {
		if mount.Name == ".gemini" {
			t.Fatal("config tree containing a socket was mounted")
		}
	}
	if len(report.Refused) == 0 {
		t.Fatalf("special file was not reported as refused: %+v", report)
	}
}

func TestCodexHomeMountPreservesDirectoryAndMasksRuntimeEntries(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	for _, dir := range []string{"app-server-control", "app-server-daemon", "ipc", "packages/app-server-daemon", "tmp/arg0"} {
		if err := os.MkdirAll(filepath.Join(codexHome, dir), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for name, contents := range map[string]string{
		"auth.json":   "credentials",
		"config.toml": "model = 'test'",
	} {
		if err := os.WriteFile(filepath.Join(codexHome, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := syscall.Mkfifo(filepath.Join(codexHome, "app-server-control", "daemon.sock"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/opt/homebrew/bin/codex", filepath.Join(codexHome, "packages/app-server-daemon/current")); err != nil {
		t.Fatal(err)
	}
	report := resolveConfigs(t, home, nil)
	var codexMount *plan.ConfigMount
	for _, mount := range report.Mounts {
		if mount.Agent == "codex" {
			codexMount = &mount
		}
	}
	if codexMount == nil {
		t.Fatalf("Codex home directory was not mounted: %+v", report)
	}
	resolvedCodexHome, err := filepath.EvalSymlinks(codexHome)
	if err != nil {
		t.Fatal(err)
	}
	if codexMount.HostPath != resolvedCodexHome || codexMount.DestPath != "/home/caged/.codex" {
		t.Fatalf("Codex config was not mounted as one directory: %+v", codexMount)
	}
	if !reflect.DeepEqual(codexMount.MaskPaths, []string{"app-server-control", "app-server-daemon", "ipc", "packages/app-server-daemon", "tmp/arg0"}) {
		t.Fatalf("Codex runtime mask paths = %v", codexMount.MaskPaths)
	}
	for _, refused := range report.Refused {
		if strings.Contains(refused.String(), codexHome) {
			t.Fatalf("known masked Codex runtime entries were refused: %+v", report.Refused)
		}
	}
}

func TestSharedGitConfigMustBeRegularFile(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, ".gitconfig"), 0o700); err != nil {
		t.Fatal(err)
	}
	report := resolveConfigs(t, home, nil)
	for _, mount := range report.Mounts {
		if mount.Name == ".gitconfig" {
			t.Fatal("directory mounted as shared git config")
		}
	}
	if len(report.Refused) == 0 {
		t.Fatal("non-file gitconfig was not reported as refused")
	}
}
