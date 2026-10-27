package hostconfig

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/pathpolicy"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
	"github.com/JeremyOrr4/caged/internal/testkit"
)

// faultyFS is the real filesystem with selected calls failing.
type faultyFS struct {
	system.FS
	evalErr     map[string]bool
	lstatErr    map[string]bool
	readDirErr  map[string]bool
	readlinkErr map[string]bool
}

var errInjected = errors.New("injected failure")

func (f faultyFS) EvalSymlinks(path string) (string, error) {
	if f.evalErr[path] {
		return "", errInjected
	}
	return f.FS.EvalSymlinks(path)
}

func (f faultyFS) Readlink(name string) (string, error) {
	if f.readlinkErr[name] {
		return "", errInjected
	}
	return f.FS.Readlink(name)
}

func (f faultyFS) Lstat(name string) (fs.FileInfo, error) {
	if f.lstatErr[name] {
		return nil, errInjected
	}
	return f.FS.Lstat(name)
}

func (f faultyFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if f.readDirErr[name] {
		return nil, errInjected
	}
	return f.FS.ReadDir(name)
}

// WalkDir walks like filepath.WalkDir, which reads directories itself: a
// directory in readDirErr gets the second call, with the error, that
// filepath.WalkDir makes for a directory it cannot read, and is not entered.
func (f faultyFS) WalkDir(root string, fn fs.WalkDirFunc) error {
	return f.FS.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || !entry.IsDir() || !f.readDirErr[path] {
			return fn(path, entry, err)
		}
		if err := fn(path, entry, nil); err != nil {
			return err
		}
		if err := fn(path, entry, errInjected); err != nil {
			return err
		}
		return filepath.SkipDir
	})
}

// testHome creates a home directory and returns it with its resolved path
// (they differ on macOS, where temp dirs live behind the /var -> /private/var symlink).
func testHome(t *testing.T, dirs, files []string) (string, string) {
	t.Helper()
	home := t.TempDir()
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(home, dir), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range files {
		if err := os.WriteFile(filepath.Join(home, file), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	resolved, err := filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatal(err)
	}
	return home, resolved
}

func TestResolveRefusesUnresolvableHome(t *testing.T) {
	t.Parallel()

	fsys := faultyFS{evalErr: map[string]bool{"/no/such/home": true}}
	got, err := NewResolver(Deps{FS: fsys, Env: testkit.Env{}, Agents: agent.Builtin().All(), Shared: DefaultSharedFiles(), Policies: policies{}}).Resolve("/no/such/home/")
	if err != nil {
		t.Fatal(err)
	}
	want := plan.ConfigReport{Refused: []plan.Refusal{{Path: "/no/such/home", Reason: "home cannot be resolved"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Resolve() = %+v, want %+v", got, want)
	}
}

func TestResolveReportsMissingPathsInRegistryOrder(t *testing.T) {
	t.Parallel()

	// The six original agents, so that adding a built-in agent does not change this list.
	var agents []agent.Agent
	for _, name := range []string{"claude", "codex", "copilot", "gemini", "gh", "pi"} {
		definition, err := agent.Builtin().Lookup(name)
		if err != nil {
			t.Fatal(err)
		}
		agents = append(agents, definition)
	}
	home, _ := testHome(t, nil, nil)
	report, err := NewResolver(Deps{FS: system.FS{}, Env: testkit.Env{}, Agents: agents, Shared: DefaultSharedFiles(), Policies: policies{}}).Resolve(home)
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, rel := range []string{".claude", ".codex", ".copilot", ".gemini", ".config/gh", ".config/pi", ".pi", ".gitconfig"} {
		want = append(want, filepath.Join(home, rel))
	}
	if !reflect.DeepEqual(report.Missing, want) || report.Mounts != nil || report.Refused != nil {
		t.Fatalf("Resolve() = %+v, want only Missing %v", report, want)
	}
}

func TestResolveHonorsOverrides(t *testing.T) {
	t.Parallel()

	home, resolvedHome := testHome(t, []string{"alt-codex", "abs-copilot", ".claude"}, nil)
	report := resolveConfigs(t, home, map[string]string{
		"CODEX_HOME":        "alt-codex",                        // relative to home
		"COPILOT_HOME":      filepath.Join(home, "abs-copilot"), // absolute
		"CLAUDE_CONFIG_DIR": "",                                 // empty means "use the default"
	})
	got := map[string]string{}
	for _, mount := range report.Mounts {
		got[mount.HostPath] = mount.DestPath
	}
	want := map[string]string{
		filepath.Join(resolvedHome, "alt-codex"):   "/home/caged/.codex",
		filepath.Join(resolvedHome, "abs-copilot"): "/home/caged/.copilot",
		filepath.Join(resolvedHome, ".claude"):     "/home/caged/.claude",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mounts = %v, want %v", got, want)
	}
}

func TestResolveRefusesUnresolvableConfigPath(t *testing.T) {
	t.Parallel()

	home, _ := testHome(t, []string{".gemini"}, nil)
	gemini := filepath.Join(home, ".gemini")
	fsys := faultyFS{evalErr: map[string]bool{gemini: true}}
	report, err := NewResolver(Deps{FS: fsys, Env: testkit.Env{}, Agents: agent.Builtin().All(), Policies: policies{}}).Resolve(home)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{gemini + " (unresolvable symlink)"}; !reflect.DeepEqual(refusedStrings(report.Refused), want) {
		t.Fatalf("Refused = %v, want %v", report.Refused, want)
	}
}

func TestResolveReportsUninspectableEntries(t *testing.T) {
	t.Parallel()

	home, resolvedHome := testHome(t, []string{".gemini/sub", ".gh"}, []string{".gemini/sub/a.json", ".gemini/b.json"})
	if err := os.Symlink("b.json", filepath.Join(home, ".gemini", "link")); err != nil {
		t.Fatal(err)
	}
	gemini := filepath.Join(resolvedHome, ".gemini")
	fsys := faultyFS{
		readDirErr: map[string]bool{filepath.Join(gemini, "sub"): true},
		lstatErr:   map[string]bool{filepath.Join(gemini, "b.json"): true},
	}
	report, err := NewResolver(Deps{FS: fsys, Env: testkit.Env{}, Agents: agent.Builtin().All(), Policies: policies{}}).Resolve(home)
	if err != nil {
		t.Fatal(err)
	}
	wantRefused := []string{
		filepath.Join(gemini, "b.json") + " (cannot inspect config entry safely)",
		filepath.Join(gemini, "sub") + " (cannot inspect config entry safely)",
	}
	// The link points inside the config directory, so it is followed without a policy.
	wantMounts := []plan.ConfigMount{{
		Agent: "gemini", Name: ".gemini", HostPath: filepath.Join(gemini, "b.json"), DestPath: "/home/caged/.gemini/link",
		Root: filepath.Join(home, ".gemini"), Writable: true, Link: filepath.Join(gemini, "link"),
	}}
	if !reflect.DeepEqual(refusedStrings(report.Refused), wantRefused) || !reflect.DeepEqual(report.Mounts, wantMounts) {
		t.Fatalf("Resolve() = %+v, want refused %v and mounts %+v", report, wantRefused, wantMounts)
	}

	root := filepath.Join(resolvedHome, ".gh")
	if mounts, links, refused := collectSafeMounts(faultyFS{lstatErr: map[string]bool{root: true}}, root); mounts != nil || links != nil || !reflect.DeepEqual(refusedStrings(refused), []string{root + " (cannot inspect config root safely)"}) {
		t.Fatalf("collectSafeMounts() = %v, %v, %v", mounts, links, refused)
	}
}

func TestResolveMountsSingleFileConfigAndRejectsBadMasks(t *testing.T) {
	t.Parallel()

	home, resolvedHome := testHome(t, []string{"masked"}, []string{"tool.json", "masked-file"})
	agents := []agent.Agent{
		{Name: "file", Command: []string{"file"}, ConfigPaths: []agent.ConfigPath{{HostPath: "tool.json", ContainerPath: "tool.json"}}},
		{Name: "escape", Command: []string{"escape"}, ConfigPaths: []agent.ConfigPath{{HostPath: "masked", ContainerPath: "masked", MaskPaths: []string{"../outside"}}}},
		{Name: "notdir", Command: []string{"notdir"}, ConfigPaths: []agent.ConfigPath{{HostPath: "masked-file", ContainerPath: "masked-file", MaskPaths: []string{"tmp"}}}},
	}
	report, err := NewResolver(Deps{FS: system.FS{}, Env: testkit.Env{}, Agents: agents, Policies: policies{}}).Resolve(home)
	if err != nil {
		t.Fatal(err)
	}
	wantMounts := []plan.ConfigMount{{Agent: "file", Name: "tool.json", HostPath: filepath.Join(resolvedHome, "tool.json"), DestPath: "/home/caged/tool.json", Root: filepath.Join(home, "tool.json")}}
	if !reflect.DeepEqual(report.Mounts, wantMounts) {
		t.Fatalf("Mounts = %+v, want %+v", report.Mounts, wantMounts)
	}
	wantRefused := []string{
		filepath.Join(home, "masked") + ` (invalid config mask path "../outside")`,
		filepath.Join(home, "masked-file") + " (masked config root must be a directory)",
	}
	if !reflect.DeepEqual(refusedStrings(report.Refused), wantRefused) {
		t.Fatalf("Refused = %v, want %v", report.Refused, wantRefused)
	}
}

func TestResolveReportsUnreadableMaskedLink(t *testing.T) {
	t.Parallel()

	home, resolvedHome := testHome(t, []string{".codex"}, nil)
	if err := os.Symlink("../dotfiles/AGENTS.md", filepath.Join(home, ".codex", "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(resolvedHome, ".codex", "AGENTS.md")
	fsys := faultyFS{readlinkErr: map[string]bool{link: true}}
	report, err := NewResolver(Deps{FS: fsys, Env: testkit.Env{}, Agents: agent.Builtin().All(), Policies: policies{}}).Resolve(home)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{link + " (cannot read symlink)"}; !reflect.DeepEqual(refusedStrings(report.Refused), want) {
		t.Fatalf("Refused = %v, want %v", refusedStrings(report.Refused), want)
	}
	if len(report.Mounts) != 1 || report.Mounts[0].Name != ".codex" {
		t.Fatalf("Mounts = %+v, want only the whole Codex tree", report.Mounts)
	}
}

func TestResolveDeduplicatesSharedConfigPaths(t *testing.T) {
	t.Parallel()

	home, _ := testHome(t, []string{".shared"}, nil)
	path := agent.ConfigPath{HostPath: ".shared", ContainerPath: ".shared", Writable: true}
	agents := []agent.Agent{
		{Name: "one", Command: []string{"one"}, ConfigPaths: []agent.ConfigPath{path}},
		{Name: "two", Command: []string{"two"}, ConfigPaths: []agent.ConfigPath{path}},
	}
	report, err := NewResolver(Deps{FS: system.FS{}, Env: testkit.Env{}, Agents: agents, Policies: policies{}}).Resolve(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Mounts) != 1 || report.Mounts[0].Agent != "one" {
		t.Fatalf("Mounts = %+v, want one mount owned by the first agent", report.Mounts)
	}
}

func TestSharedFilesAreData(t *testing.T) {
	t.Parallel()

	if got, want := DefaultSharedFiles(), []SharedFile{{path: ".gitconfig", description: "shared git config"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("DefaultSharedFiles() = %v, want %v", got, want)
	}

	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	home, resolvedHome := testHome(t, []string{".npmrc-dir"}, []string{".npmrc"})
	if err := os.Symlink(filepath.Join(outside, "secret"), filepath.Join(home, ".escape")); err != nil {
		t.Fatal(err)
	}
	shared := []SharedFile{
		{path: ".npmrc", description: "shared npm config"},
		{path: ".npmrc-dir", description: "shared npm config"},
		{path: ".escape", description: "shared escape"},
		{path: ".absent", description: "shared absent"},
	}
	report, err := NewResolver(Deps{FS: system.FS{}, Env: testkit.Env{}, Shared: shared, Policies: policies{}}).Resolve(home)
	if err != nil {
		t.Fatal(err)
	}
	wantMounts := []plan.ConfigMount{{Agent: plan.SharedOwner, Name: ".npmrc", HostPath: filepath.Join(resolvedHome, ".npmrc"), DestPath: "/home/caged/.npmrc", Root: filepath.Join(home, ".npmrc")}}
	if !reflect.DeepEqual(report.Mounts, wantMounts) {
		t.Fatalf("Mounts = %+v, want %+v", report.Mounts, wantMounts)
	}
	resolvedOutside, err := filepath.EvalSymlinks(outside)
	if err != nil {
		t.Fatal(err)
	}
	wantRefused := []string{
		filepath.Join(home, ".npmrc-dir") + " (shared npm config must be a regular file)",
		filepath.Join(home, ".escape") + " (symlink target " + filepath.Join(resolvedOutside, "secret") + " is not under links.allow); " +
			`add "- path: ` + resolvedOutside + `" under links.allow in ~/.config/caged/config.yaml`,
	}
	if !reflect.DeepEqual(refusedStrings(report.Refused), wantRefused) {
		t.Fatalf("Refused = %v, want %v", report.Refused, wantRefused)
	}
	if want := []string{filepath.Join(home, ".absent")}; !reflect.DeepEqual(report.Missing, want) {
		t.Fatalf("Missing = %v, want %v", report.Missing, want)
	}
}

func TestContainerHomeUsesSlashes(t *testing.T) {
	t.Parallel()
	if strings.Contains(containerHome, "\\") {
		t.Fatal("container paths must use forward slashes")
	}
}

// refusedStrings formats refusals as "path (reason)" for easy comparison.
func refusedStrings(refused []plan.Refusal) []string {
	var out []string
	for _, refusal := range refused {
		out = append(out, refusal.String())
	}
	return out
}

func TestNoAgentUsesTheSharedOwnerName(t *testing.T) {
	t.Parallel()

	for _, name := range agent.Builtin().Names() {
		if name == plan.SharedOwner {
			t.Errorf("agent %q collides with the shared config owner %q", name, plan.SharedOwner)
		}
	}
}

// failingPolicies fails with err.
type failingPolicies struct{ err error }

func (f failingPolicies) LinkPolicy(string) (*pathpolicy.Policy, error) { return nil, f.err }

func TestResolveReturnsLinkPolicyAndConfigDirErrors(t *testing.T) {
	t.Parallel()
	home, _ := testHome(t, []string{".claude"}, nil)
	policyErr, dirErr := errors.New("bad config.yaml"), errors.New("no config dir")

	for _, tt := range []struct {
		name     string
		resolver *Resolver
		want     error
	}{
		{"link policy", NewResolver(Deps{FS: system.FS{}, Env: testkit.Env{}, Agents: agent.Builtin().All(), Policies: failingPolicies{policyErr}}), policyErr},
		{"config dir", NewResolver(Deps{FS: system.FS{}, Env: testkit.Env{}, Agents: agent.Builtin().All(), Policies: policies{}, ConfigDirs: failingDirs{dirErr}}), dirErr},
		{"state dir", NewResolver(Deps{FS: system.FS{}, Env: testkit.Env{}, Agents: agent.Builtin().All(), Policies: policies{}, ConfigDirs: failingStateDirs{filepath.Join(home, ".config", "caged"), dirErr}}), dirErr},
	} {
		report, err := tt.resolver.Resolve(home)
		if err != tt.want || !reflect.DeepEqual(report, plan.ConfigReport{}) { //nolint:errorlint // asserts the error is returned unchanged, not wrapped
			t.Errorf("%s: Resolve() = %+v, %v; want an empty report and %v", tt.name, report, err, tt.want)
		}
	}
}
