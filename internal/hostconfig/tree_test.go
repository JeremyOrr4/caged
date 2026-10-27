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

func TestSplitTreeMountsSymlinkFreeSubdirectoriesWhole(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, ".claude/settings.json", ".claude/whole/deep/a.json", ".claude/whole/b.json", ".claude/split/c.json", ".claude/split/deep/d.json")
	symlink(t, home, ".claude/split/deep/link", "../c.json")

	claude, err := agent.Builtin().Lookup("claude")
	if err != nil {
		t.Fatal(err)
	}
	report, err := NewResolver(Deps{FS: system.FS{}, Env: testkit.Env{}, Agents: []agent.Agent{claude}, Policies: policies{}}).Resolve(home)
	if err != nil {
		t.Fatal(err)
	}

	want := "" +
		"mount claude ~/.claude/settings.json -> /home/caged/.claude/settings.json root=~/.claude writable=true staged=false masks=[] files=[] link=\n" +
		"mount claude ~/.claude/split/c.json -> /home/caged/.claude/split/c.json root=~/.claude writable=true staged=false masks=[] files=[] link=\n" +
		"mount claude ~/.claude/split/deep/d.json -> /home/caged/.claude/split/deep/d.json root=~/.claude writable=true staged=false masks=[] files=[] link=\n" +
		"mount claude ~/.claude/whole -> /home/caged/.claude/whole root=~/.claude writable=true staged=false masks=[] files=[] link=\n" +
		"mount claude ~/.claude/split/c.json -> /home/caged/.claude/split/deep/link root=~/.claude writable=true staged=false masks=[] files=[] link=~/.claude/split/deep/link\n"
	if got := describeReport(report, home, resolved); got != want {
		t.Fatalf("report:\n%s\nwant:\n%s", got, want)
	}
}

func TestSplitTreeSpecialFileReason(t *testing.T) {
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
	mustMount(t, report, "/home/caged/.gemini/settings.json")
}

func TestMaskedTreeWithAnUncoveredSpecialFileIsRefused(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, ".codex/config.toml")
	if err := syscall.Mkfifo(filepath.Join(home, ".codex", "stray.sock"), 0o600); err != nil {
		t.Fatal(err)
	}

	report := resolveConfigs(t, home, nil)

	if _, ok := mountAt(report, "/home/caged/.codex"); ok {
		t.Fatalf("a whole tree holding an unmasked special file was mounted: %+v", report.Mounts)
	}
	want := []string{filepath.Join(home, ".codex") + ` (special file "` + filepath.Join(resolved, ".codex/stray.sock") + `" is not covered by a config mask)`}
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

// TestMaskedTreeEntryThatCannotBeInspected pins Q-HC-2 (KEEP): a whole tree
// is refused at its root, with the raw Lstat error as the reason.
func TestMaskedTreeEntryThatCannotBeInspected(t *testing.T) {
	t.Parallel()
	home, resolved := testHome(t, nil, nil)
	writeFiles(t, home, ".codex/config.toml")
	fsys := faultyFS{lstatErr: map[string]bool{filepath.Join(resolved, ".codex/config.toml"): true}}

	report, err := NewResolver(Deps{FS: fsys, Env: testkit.Env{}, Agents: agent.Builtin().All(), Policies: policies{}}).Resolve(home)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{filepath.Join(home, ".codex") + " (" + errInjected.Error() + ")"}
	if got := refusedStrings(report.Refused); !reflect.DeepEqual(got, want) || len(report.Mounts) != 0 {
		t.Fatalf("Refused = %v, Mounts = %+v; want %v and no mounts", got, report.Mounts, want)
	}
}

// TestSplitTreeInspectsEachEntryOnce pins the cost of a split tree: one Lstat
// per entry plus the root, and one ReadDir per directory, however deep the
// symlink that splits it (F25).
func TestSplitTreeInspectsEachEntryOnce(t *testing.T) {
	t.Parallel()
	_, resolved := testHome(t, nil, nil)
	writeFiles(t, resolved, ".claude/a.json", ".claude/d1/b.json", ".claude/d1/d2/c.json", ".claude/d1/d3/e.json")
	symlink(t, resolved, ".claude/d1/d2/link", "../b.json")
	root := filepath.Join(resolved, ".claude")
	fsys := &countingFS{}

	mounts, links, refused := collectSafeMounts(fsys, root)

	// 8 entries (a.json, d1, b.json, d2, c.json, link, d3, e.json) plus the root; 4 directories.
	if fsys.lstats != 9 || fsys.readDirs != 4 {
		t.Fatalf("Lstat %d times and ReadDir %d times; want 9 and 4", fsys.lstats, fsys.readDirs)
	}
	want := []string{filepath.Join(root, "a.json"), filepath.Join(root, "d1/b.json"), filepath.Join(root, "d1/d2/c.json"), filepath.Join(root, "d1/d3")}
	if !reflect.DeepEqual(mounts, want) || !reflect.DeepEqual(links, []string{filepath.Join(root, "d1/d2/link")}) || refused != nil {
		t.Fatalf("collectSafeMounts() = %v, %v, %v; want %v, the link, no refusals", mounts, links, refused, want)
	}
}
