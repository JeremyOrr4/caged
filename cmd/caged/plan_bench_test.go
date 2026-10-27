package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/app"
	"github.com/JeremyOrr4/caged/internal/hostconfig"
	"github.com/JeremyOrr4/caged/internal/project"
	"github.com/JeremyOrr4/caged/internal/system"
	"github.com/JeremyOrr4/caged/internal/userconfig"
)

// BenchmarkPlan plans `caged --verbose claude` with the production planner,
// resolvers and config store on a real home: a Claude config of 5,000 files in
// 50 directories with one symlink (the split tree of hostconfig's
// BenchmarkResolveWideTree), plus Codex and the shared .gitconfig. It measures
// everything before the first docker call. The wiring mirrors run in main.go.
func BenchmarkPlan(b *testing.B) {
	home, work := b.TempDir(), b.TempDir()
	b.Setenv("HOME", home)
	unsetEnv(b, "XDG_CONFIG_HOME")
	unsetEnv(b, "XDG_STATE_HOME")
	for d := range 50 {
		dir := filepath.Join(home, ".claude", fmt.Sprintf("d%02d", d))
		if err := os.MkdirAll(dir, 0o700); err != nil {
			b.Fatal(err)
		}
		for i := range 100 {
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%04d", i)), []byte("x"), 0o600); err != nil {
				b.Fatal(err)
			}
		}
	}
	if err := os.Symlink("f0000", filepath.Join(home, ".claude", "d00", "link")); err != nil {
		b.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o700); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[user]\n"), 0o600); err != nil {
		b.Fatal(err)
	}
	fsys, env, agents := system.FS{}, system.Env{}, agent.Builtin()
	store := userconfig.NewStore(fsys, env)
	configs := hostconfig.NewResolver(hostconfig.Deps{FS: fsys, Env: env, Agents: agents.All(), Shared: hostconfig.DefaultSharedFiles(), Policies: store, ConfigDirs: store})
	planner := app.NewPlanner(env, project.NewResolver(fsys, store, false), configs)
	claude, err := agents.Lookup("claude")
	if err != nil {
		b.Fatal(err)
	}
	request := app.Request{Agent: &claude, Verbose: true, WorkDir: work}
	for b.Loop() {
		if _, err := planner.Plan(request, "caged-runtime:latest", app.NewReporter(io.Discard, true)); err != nil {
			b.Fatal(err)
		}
	}
}
