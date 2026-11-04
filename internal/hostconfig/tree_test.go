package hostconfig

import (
	"path/filepath"
	"reflect"
	"syscall"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/system"
	"github.com/JeremyOrr4/caged/internal/testkit"
)

// TestConfigTreeWithALinkIsMountedWhole is the fix for new files being lost:
// a config directory holding a symlink used to be split into one mount per
// entry, so a file the agent created next to them never reached the host.
// It is now mounted whole, and a link that resolves inside the tree needs no
// mount of its own.
func TestConfigTreeWithALinkIsMountedWhole(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, ".claude/settings.json", ".claude/whole/deep/a.json", ".claude/split/c.json", ".claude/split/deep/d.json")
	symlink(t, home, ".claude/split/deep/link", "../c.json")

	claude, err := agent.Builtin().Lookup("claude")
	if err != nil {
		t.Fatal(err)
	}
	report, err := NewResolver(Deps{FS: system.FS{}, Env: testkit.Env{}, Agents: []agent.Agent{claude}, Policies: policies{}}).Resolve(home)
	if err != nil {
		t.Fatal(err)
	}

	want := "mount claude ~/.claude -> /home/caged/.claude root=~/.claude writable=true staged=false masks=[] files=[] link=\n"
	if got := describeReport(report, home, resolved); got != want {
		t.Fatalf("report:\n%s\nwant:\n%s", got, want)
	}
}

func TestWholeTreeHidesSpecialFiles(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, ".gemini/settings.json")
	if err := syscall.Mkfifo(filepath.Join(home, ".gemini", "agent.pipe"), 0o600); err != nil {
		t.Fatal(err)
	}

	report := resolveConfigs(t, home, nil)

	want := []string{filepath.Join(resolved, ".gemini/agent.pipe") + " (special files are excluded from config mounts)"}
	if got := refusedStrings(report.Refused); !reflect.DeepEqual(got, want) {
		t.Fatalf("Refused = %v, want %v", got, want)
	}
	if mount := mustMount(t, report, "/home/caged/.gemini"); !reflect.DeepEqual(mount.MaskFiles, []string{"agent.pipe"}) {
		t.Fatalf("MaskFiles = %v, want [agent.pipe]", mount.MaskFiles)
	}
}

func TestMaskedTreeHidesAnUncoveredSpecialFile(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, ".codex/config.toml")
	if err := syscall.Mkfifo(filepath.Join(home, ".codex", "stray.sock"), 0o600); err != nil {
		t.Fatal(err)
	}

	report := resolveConfigs(t, home, nil)

	mount := mustMount(t, report, "/home/caged/.codex")
	if !reflect.DeepEqual(mount.MaskFiles, []string{"stray.sock"}) || len(mount.MaskPaths) == 0 {
		t.Fatalf("Codex mount = %+v, want its masks and stray.sock hidden", mount)
	}
	want := []string{filepath.Join(resolved, ".codex/stray.sock") + " (special files are excluded from config mounts)"}
	if got := refusedStrings(report.Refused); !reflect.DeepEqual(got, want) {
		t.Fatalf("Refused = %v, want %v", got, want)
	}
}

func TestConfigRootThatIsASpecialFileIsRefused(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	if err := syscall.Mkfifo(filepath.Join(home, ".gemini"), 0o600); err != nil {
		t.Fatal(err)
	}

	report := resolveConfigs(t, home, nil)

	want := []string{filepath.Join(resolved, ".gemini") + " (config root is not a regular file or directory)"}
	if got := refusedStrings(report.Refused); !reflect.DeepEqual(got, want) || len(report.Mounts) != 0 {
		t.Fatalf("Refused = %v, Mounts = %+v; want %v and no mounts", got, report.Mounts, want)
	}
}

// TestWholeTreeEntryThatCannotBeInspectedIsHidden: an entry Lstat fails on
// is hidden from the mount and refused; the rest of the tree is mounted.
func TestWholeTreeEntryThatCannotBeInspectedIsHidden(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, ".codex/config.toml", ".codex/auth.json")
	fsys := faultyFS{lstatErr: map[string]bool{filepath.Join(resolved, ".codex/config.toml"): true}}

	report, err := NewResolver(Deps{FS: fsys, Env: testkit.Env{}, Agents: agent.Builtin().All(), Policies: policies{}}).Resolve(home)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{filepath.Join(resolved, ".codex/config.toml") + " (cannot inspect config entry safely)"}
	if got := refusedStrings(report.Refused); !reflect.DeepEqual(got, want) {
		t.Fatalf("Refused = %v, want %v", got, want)
	}
	if mount := mustMount(t, report, "/home/caged/.codex"); !reflect.DeepEqual(mount.MaskFiles, []string{"config.toml"}) {
		t.Fatalf("MaskFiles = %v, want [config.toml]", mount.MaskFiles)
	}
}
