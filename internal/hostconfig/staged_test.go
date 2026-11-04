package hostconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
	"github.com/JeremyOrr4/caged/internal/testkit"
)

// describeReport prints a report with home, in either form, shown as ~, one
// entry per line, so a whole report can be compared at once.
func describeReport(report plan.ConfigReport, home, resolvedHome string) string {
	short := func(path string) string {
		path = strings.ReplaceAll(path, resolvedHome, "~")
		return strings.ReplaceAll(path, home, "~")
	}
	var b strings.Builder
	for _, m := range report.Mounts {
		fmt.Fprintf(&b, "mount %s %s -> %s root=%s writable=%v staged=%v masks=%v files=%v link=%s\n",
			m.Agent, short(m.HostPath), m.DestPath, short(m.Root), m.Writable, m.Staged, m.MaskPaths, m.MaskFiles, short(m.Link))
	}
	for _, path := range report.Missing {
		fmt.Fprintf(&b, "missing %s\n", short(path))
	}
	for _, refusal := range report.Refused {
		fmt.Fprintf(&b, "refused %s\n", short(refusal.String()))
	}
	return b.String()
}

func TestResolveStagedTrees(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read the unreadable directory")
	}
	home, resolvedHome := testHome(t, []string{".st/sub", ".st/locked"}, []string{".st/a.json", ".st/sub/b.json", ".stfile"})
	for link, target := range map[string]string{"dirlink": "sub", "filelink": "a.json", "dangling": "missing.json"} {
		if err := os.Symlink(target, filepath.Join(home, ".st", link)); err != nil {
			t.Fatal(err)
		}
	}
	if err := syscall.Mkfifo(filepath.Join(home, ".st", "pipe"), 0o600); err != nil {
		t.Fatal(err)
	}
	locked := filepath.Join(home, ".st", "locked")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })
	agents := []agent.Agent{
		{Name: "st", Command: []string{"st"}, ConfigPaths: []agent.ConfigPath{
			{HostPath: ".st", ContainerPath: ".st", Writable: true, Staged: true},
			{HostPath: ".stfile", ContainerPath: ".stfile", Writable: true, Staged: true},
		}},
	}
	report, err := NewResolver(Deps{FS: system.FS{}, Env: testkit.Env{}, Agents: agents, Policies: policies{}}).Resolve(home)
	if err != nil {
		t.Fatal(err)
	}
	want := "" +
		"mount st ~/.st -> /home/caged/.st root=~/.st writable=true staged=true masks=[dirlink locked] files=[dangling filelink pipe] link=\n" +
		"mount st ~/.st/sub -> /home/caged/.st/dirlink root=~/.st writable=true staged=true masks=[] files=[] link=~/.st/dirlink\n" +
		"mount st ~/.st/a.json -> /home/caged/.st/filelink root=~/.st writable=true staged=true masks=[] files=[] link=~/.st/filelink\n" +
		"mount st ~/.stfile -> /home/caged/.stfile root=~/.stfile writable=true staged=true masks=[] files=[] link=\n" +
		"refused ~/.st/locked (cannot inspect config entry safely)\n" +
		"refused ~/.st/pipe (special files are excluded from config mounts)\n" +
		"refused ~/.st/dangling (dangling or unresolvable symlink)\n"
	if got := describeReport(report, home, resolvedHome); got != want {
		t.Fatalf("report:\n%s\nwant:\n%s", got, want)
	}
}

func TestResolveStagedTreeTwiceKeepsMaskFilesOnTheRoot(t *testing.T) {
	home, _ := testHome(t, []string{".st"}, []string{".st/a.json"})
	if err := os.Symlink("a.json", filepath.Join(home, ".st", "filelink")); err != nil {
		t.Fatal(err)
	}
	path := agent.ConfigPath{HostPath: ".st", ContainerPath: ".st", Writable: true, Staged: true}
	agents := []agent.Agent{{Name: "st", Command: []string{"st"}, ConfigPaths: []agent.ConfigPath{path, path}}}
	report, err := NewResolver(Deps{FS: system.FS{}, Env: testkit.Env{}, Agents: agents, Policies: policies{}}).Resolve(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Mounts) != 2 || len(report.Mounts[0].MaskFiles) != 1 || len(report.Mounts[1].MaskFiles) != 0 {
		t.Fatalf("Mounts = %+v, want the root with one mask file and the link target with none", report.Mounts)
	}
}

// stagedAgents is one agent whose .st config path is a staged tree.
func stagedAgents() []agent.Agent {
	return []agent.Agent{{Name: "st", Command: []string{"st"}, ConfigPaths: []agent.ConfigPath{{HostPath: ".st", ContainerPath: ".st", Writable: true, Staged: true}}}}
}

func TestStagedTreeEntryThatCannotBeInspected(t *testing.T) {
	t.Parallel()
	home, resolvedHome := testHome(t, []string{".st"}, []string{".st/a.json", ".st/b.json"})
	fsys := faultyFS{lstatErr: map[string]bool{filepath.Join(resolvedHome, ".st", "b.json"): true}}

	report, err := NewResolver(Deps{FS: fsys, Env: testkit.Env{}, Agents: stagedAgents(), Policies: policies{}}).Resolve(home)
	if err != nil {
		t.Fatal(err)
	}

	want := "" +
		"mount st ~/.st -> /home/caged/.st root=~/.st writable=true staged=true masks=[] files=[b.json] link=\n" +
		"refused ~/.st/b.json (cannot inspect config entry safely)\n"
	if got := describeReport(report, home, resolvedHome); got != want {
		t.Fatalf("report:\n%s\nwant:\n%s", got, want)
	}
}

// TestStagedConfigRootThatIsASpecialFileIsRefused is bug B-HC-1: a FIFO at a
// staged root is refused, as at any other root, instead of being mounted.
func TestStagedConfigRootThatIsASpecialFileIsRefused(t *testing.T) {
	t.Parallel()
	home, resolvedHome := testHome(t, nil, nil)
	if err := syscall.Mkfifo(filepath.Join(home, ".st"), 0o600); err != nil {
		t.Fatal(err)
	}

	report, err := NewResolver(Deps{FS: system.FS{}, Env: testkit.Env{}, Agents: stagedAgents(), Policies: policies{}}).Resolve(home)
	if err != nil {
		t.Fatal(err)
	}

	want := "refused ~/.st (config root is not a regular file or directory)\n"
	if got := describeReport(report, home, resolvedHome); got != want {
		t.Fatalf("report:\n%s\nwant:\n%s", got, want)
	}
}

// TestLinkedStagedConfigRootThatIsASpecialFileIsRefused is the Stow form of
// B-HC-1: the staged root is an allowed link whose target is a FIFO.
func TestLinkedStagedConfigRootThatIsASpecialFileIsRefused(t *testing.T) {
	t.Parallel()
	home, resolvedHome := testHome(t, []string{"dotfiles"}, nil)
	if err := syscall.Mkfifo(filepath.Join(home, "dotfiles", "st"), 0o600); err != nil {
		t.Fatal(err)
	}
	symlink(t, home, ".st", "dotfiles/st")

	report, err := NewResolver(Deps{FS: system.FS{}, Env: testkit.Env{}, Agents: stagedAgents(), Policies: policies{allow: []string{filepath.Join(home, "dotfiles")}}}).Resolve(home)
	if err != nil {
		t.Fatal(err)
	}

	want := "refused ~/dotfiles/st (config root is not a regular file or directory)\n"
	if got := describeReport(report, home, resolvedHome); got != want {
		t.Fatalf("report:\n%s\nwant:\n%s", got, want)
	}
}

// TestUnreadableStagedRootIsRefused is bug B-HC-2: a staged root that cannot
// be read is refused, instead of being mounted whole with nothing masked.
func TestUnreadableStagedRootIsRefused(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root can read the unreadable directory")
	}
	home, resolvedHome := testHome(t, []string{".st"}, []string{".st/a.json"})
	symlink(t, home, ".st/link", "a.json")
	root := filepath.Join(home, ".st")
	if err := os.Chmod(root, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o700) })

	report, err := NewResolver(Deps{FS: system.FS{}, Env: testkit.Env{}, Agents: stagedAgents(), Policies: policies{}}).Resolve(home)
	if err != nil {
		t.Fatal(err)
	}

	want := "refused ~/.st (cannot inspect config tree safely)\n"
	if got := describeReport(report, home, resolvedHome); got != want {
		t.Fatalf("report:\n%s\nwant:\n%s", got, want)
	}
}

func TestStagedTreeRootThatCannotBeInspected(t *testing.T) {
	t.Parallel()
	home, resolvedHome := testHome(t, []string{".st"}, []string{".st/a.json"})
	fsys := faultyFS{lstatErr: map[string]bool{filepath.Join(resolvedHome, ".st"): true}}

	report, err := NewResolver(Deps{FS: fsys, Env: testkit.Env{}, Agents: stagedAgents(), Policies: policies{}}).Resolve(home)
	if err != nil {
		t.Fatal(err)
	}

	want := "refused ~/.st (cannot inspect config root safely)\n"
	if got := describeReport(report, home, resolvedHome); got != want {
		t.Fatalf("report:\n%s\nwant:\n%s", got, want)
	}
}

// TestStagedRootThatCannotBeListedIsRefused is B-HC-2 for any uid, plain and
// behind a Stow link, with the listing failure injected.
func TestStagedRootThatCannotBeListedIsRefused(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, link, root, want string
	}{
		{name: "plain", root: ".st", want: "refused ~/.st (cannot inspect config tree safely)\n"},
		{name: "linked", link: ".st", root: "dotfiles/st", want: "refused ~/dotfiles/st (cannot inspect config tree safely)\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			home, resolvedHome := testHome(t, []string{tt.root}, []string{tt.root + "/a.json"})
			if tt.link != "" {
				symlink(t, home, tt.link, tt.root)
			}
			fsys := faultyFS{readDirErr: map[string]bool{filepath.Join(resolvedHome, tt.root): true}}

			report, err := NewResolver(Deps{FS: fsys, Env: testkit.Env{}, Agents: stagedAgents(), Policies: policies{allow: []string{filepath.Join(home, "dotfiles")}}}).Resolve(home)
			if err != nil {
				t.Fatal(err)
			}

			if got := describeReport(report, home, resolvedHome); got != tt.want {
				t.Fatalf("report:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}
