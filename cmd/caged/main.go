// Command caged is the command-line entry point for caged.
//
// This file is the composition root: the only place that chooses concrete
// implementations and wires them together.
package main

import (
	"context"
	"errors"
	"os"
	"runtime/debug"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/app"
	"github.com/JeremyOrr4/caged/internal/cli"
	"github.com/JeremyOrr4/caged/internal/configstage"
	"github.com/JeremyOrr4/caged/internal/docker"
	"github.com/JeremyOrr4/caged/internal/hostconfig"
	"github.com/JeremyOrr4/caged/internal/project"
	"github.com/JeremyOrr4/caged/internal/system"
	"github.com/JeremyOrr4/caged/internal/userconfig"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	stdio, platform := system.CurrentStdio(), system.CurrentPlatform()
	os.Exit(run(context.Background(), resolveVersion(version, debug.ReadBuildInfo), os.Args[1:], stdio, platform, agent.Builtin()))
}

// run wires caged together, executes one command line, and returns the
// process exit code. When the container fails, its exit code is passed on.
func run(ctx context.Context, version string, args []string, stdio system.Stdio, platform system.Platform, agents *agent.Registry) int {
	fsys, env := system.FS{}, system.Env{}
	store := userconfig.NewStore(fsys, env)
	guard := app.NewConfigGuard(app.GuardDeps{
		Init: store, Content: store, Approvals: userconfig.NewApprovals(fsys, env, store),
		In: stdio.In, Out: stdio.Err, Interactive: system.IsTerminal(stdio.In),
	})
	configs := hostconfig.NewResolver(hostconfig.Deps{FS: fsys, Env: env, Agents: agents.All(), Shared: hostconfig.DefaultSharedFiles(), Policies: store, ConfigDirs: store, FoldCase: platform.FoldsCase()})
	projects := project.NewResolver(fsys, store, platform.FoldsCase())
	planner := app.NewPlanner(env, projects, configs)
	client := docker.NewClient(docker.Deps{Exec: system.Executor{}, FS: fsys, Stager: configstage.New(fsys), Platform: platform, Stdio: stdio})
	ensurer := app.NewImageEnsurer(client, version)
	services := cli.Services{
		Run: app.GuardedRunner{Guard: guard, Next: app.NewRunner(app.RunnerDeps{
			Extensions: store, Planner: planner, Daemon: client, Ensurer: ensurer, Containers: client, Stderr: stdio.Err,
		})},
		Build: app.GuardedBuilder{Guard: guard, Next: app.NewBuilder(store, client, ensurer, stdio.Err)},
		Doctor: app.GuardedDoctor{Guard: guard, Next: app.NewDoctor(app.DoctorDeps{
			Daemon: client, Store: client, Prober: client, Extensions: store, Config: store, Configs: configs,
			Project: projects, Env: env, Host: app.Host{OS: platform.OS, Arch: platform.Arch}, Version: version,
		})},
		List:  app.GuardedLister{Guard: guard, Next: app.NewLister(env, configs, agents.All())},
		Clean: app.NewCleaner(client, client, client),
	}

	root := cli.NewRootCmd(version, agents, services)
	if args == nil {
		args = []string{} // a nil slice would make cobra read os.Args instead
	}
	root.SetArgs(args)
	// Cobra keeps its default writers (os.Stdout, and os.Stderr for usage
	// text) as before; SetOut would also move its stderr-bound messages.
	err := root.ExecuteContext(ctx)
	if err == nil {
		return 0
	}
	app.NewReporter(stdio.Err, false).Notice("%v", err)
	var coder interface{ ExitCode() int }
	if errors.As(err, &coder) {
		return coder.ExitCode()
	}
	return 1
}
