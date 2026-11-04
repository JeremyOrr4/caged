package hostconfig

import (
	"path/filepath"
	"syscall"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/system"
	"github.com/JeremyOrr4/caged/internal/testkit"
)

// stowHome builds a home managed with GNU Stow that reaches every walker:
// split trees with symlink-free subdirectories, file and directory links into
// allowed, read-only and denied roots, a link deep inside a split tree, a
// special file, a Codex whole tree with relative, absolute and refused links,
// a staged Pi tree whose root is a link, caged's own folder and .gitconfig.
func stowHome(t *testing.T) (home, resolved string) {
	t.Helper()
	home, resolved = testHome(t, nil, nil)
	writeFiles(t, home,
		".claude/settings.json", ".claude/sub/y.json", ".claude/sub/deep/x.json", ".claude/run/z.json", ".claude/nested/deep/n.json",
		"dotfiles/claude/CLAUDE.md", "dotfiles/claude/agents/a.md", "dotfiles2/shared.md", ".ssh/id_x",
		".codex/config.toml", ".codex/app-server-control/state", "dotfiles/codex/AGENTS.md", "private/secret",
		"dotfiles/pi/.pi/agent/settings-target.json", "dotfiles/pi/.pi/agent/extensions-target/ext.js", "dotfiles/pi/.pi/agent/plain.json",
		".gemini/settings.json", ".config/gh/hosts.yml", ".config/caged/config.yaml", ".gitconfig",
	)
	symlink(t, home, ".claude/CLAUDE.md", "../dotfiles/claude/CLAUDE.md")
	symlink(t, home, ".claude/agents", "../dotfiles/claude/agents")
	symlink(t, home, "dotfiles/claude/agents/shared.md", "../../../dotfiles2/shared.md")
	symlink(t, home, ".claude/key", "../.ssh/id_x")
	symlink(t, home, ".claude/dangling", "../outside/nowhere")
	symlink(t, home, ".claude/nested/deep/link", "../../settings.json")
	symlink(t, home, ".codex/AGENTS.md", "../dotfiles/codex/AGENTS.md")
	symlink(t, home, ".codex/abs-link", filepath.Join(home, "dotfiles/codex"))
	symlink(t, home, ".codex/private", "../private")
	symlink(t, home, "dotfiles/pi/.pi/agent/settings.json", "settings-target.json")
	symlink(t, home, "dotfiles/pi/.pi/agent/extensions", "extensions-target")
	symlink(t, home, ".pi", "dotfiles/pi/.pi")
	for _, fifo := range []string{".claude/run/pipe", ".codex/app-server-control/daemon.sock"} {
		if err := syscall.Mkfifo(filepath.Join(home, fifo), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return home, resolved
}

// TestResolveSnapshotOfAStowManagedHome pins the whole report for stowHome,
// mount by mount and refusal by refusal, in order. Every walker refactor must
// leave it identical.
func TestResolveSnapshotOfAStowManagedHome(t *testing.T) {
	t.Parallel()
	home, resolvedHome := stowHome(t)
	links := policies{allow: []string{filepath.Join(home, "dotfiles")}, readOnlyAllow: []string{filepath.Join(home, "dotfiles2")}}
	resolver := NewResolver(Deps{FS: system.FS{}, Env: testkit.Env{}, Agents: agent.Builtin().All(), Shared: DefaultSharedFiles(), Policies: links, ConfigDirs: fakeDirs(filepath.Join(home, ".config", "caged"))})
	report, err := resolver.Resolve(home)
	if err != nil {
		t.Fatal(err)
	}
	want := "" +
		"mount claude ~/.claude/nested/deep/n.json -> /home/caged/.claude/nested/deep/n.json root=~/.claude writable=true staged=false masks=[] files=[] link=\n" +
		"mount claude ~/.claude/run/z.json -> /home/caged/.claude/run/z.json root=~/.claude writable=true staged=false masks=[] files=[] link=\n" +
		"mount claude ~/.claude/settings.json -> /home/caged/.claude/settings.json root=~/.claude writable=true staged=false masks=[] files=[] link=\n" +
		"mount claude ~/.claude/sub -> /home/caged/.claude/sub root=~/.claude writable=true staged=false masks=[] files=[] link=\n" +
		"mount claude ~/dotfiles/claude/CLAUDE.md -> /home/caged/.claude/CLAUDE.md root=~/.claude writable=true staged=false masks=[] files=[] link=~/.claude/CLAUDE.md\n" +
		"mount claude ~/dotfiles/claude/agents/a.md -> /home/caged/.claude/agents/a.md root=~/.claude writable=true staged=false masks=[] files=[] link=~/.claude/agents\n" +
		"mount claude ~/dotfiles2/shared.md -> /home/caged/.claude/agents/shared.md root=~/.claude writable=false staged=false masks=[] files=[] link=~/dotfiles/claude/agents/shared.md\n" +
		"mount claude ~/.claude/settings.json -> /home/caged/.claude/nested/deep/link root=~/.claude writable=true staged=false masks=[] files=[] link=~/.claude/nested/deep/link\n" +
		"mount codex ~/.codex -> /home/caged/.codex root=~/.codex writable=true staged=false masks=[app-server-control app-server-daemon ipc packages/app-server-daemon tmp/arg0] files=[] link=\n" +
		"mount codex ~/dotfiles/codex/AGENTS.md -> /home/caged/dotfiles/codex/AGENTS.md root=~/.codex writable=true staged=false masks=[] files=[] link=~/.codex/AGENTS.md\n" +
		"mount gemini ~/.gemini -> /home/caged/.gemini root=~/.gemini writable=true staged=false masks=[] files=[] link=\n" +
		"mount gh ~/.config/gh -> /home/caged/.config/gh root=~/.config/gh writable=true staged=false masks=[] files=[] link=\n" +
		"mount pi ~/dotfiles/pi/.pi -> /home/caged/.pi root=~/.pi writable=true staged=true masks=[agent/extensions] files=[agent/settings.json] link=~/.pi\n" +
		"mount pi ~/dotfiles/pi/.pi/agent/extensions-target -> /home/caged/.pi/agent/extensions root=~/.pi writable=true staged=true masks=[] files=[] link=~/dotfiles/pi/.pi/agent/extensions\n" +
		"mount pi ~/dotfiles/pi/.pi/agent/settings-target.json -> /home/caged/.pi/agent/settings.json root=~/.pi writable=true staged=true masks=[] files=[] link=~/dotfiles/pi/.pi/agent/settings.json\n" +
		"mount shared ~/.gitconfig -> /home/caged/.gitconfig root=~/.gitconfig writable=false staged=false masks=[] files=[] link=\n" +
		"mount shared ~/.config/caged -> /home/caged/.config/caged root=~/.config/caged writable=true staged=false masks=[] files=[] link=\n" +
		"missing ~/.copilot\n" +
		"missing ~/.config/pi\n" +
		"refused ~/.claude/run/pipe (special files are excluded from config mounts)\n" +
		"refused ~/.claude/dangling (dangling or unresolvable symlink)\n" +
		"refused ~/.claude/key (symlink target ~/.ssh/id_x is denied); ~/.ssh is on caged's built-in deny list and cannot be allowed\n" +
		"refused ~/.codex/abs-link (symlink to ~/dotfiles/codex cannot be followed inside the container)\n" +
		"refused ~/.codex/private (symlink target ~/private is not under links.allow); add \"- path: ~/private\" under links.allow in ~/.config/caged/config.yaml\n"
	if got := describeReport(report, home, resolvedHome); got != want {
		t.Fatalf("report:\n%s\nwant:\n%s", got, want)
	}
}
