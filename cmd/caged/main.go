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
	"github.com/JeremyOrr4/caged/internal/docker"
	"github.com/JeremyOrr4/caged/internal/extension"
	"github.com/JeremyOrr4/caged/internal/hostconfig"
	"github.com/JeremyOrr4/caged/internal/project"
	"github.com/JeremyOrr4/caged/internal/system"
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
	configs := hostconfig.NewResolver(fsys, env, agents.All(), hostconfig.DefaultSharedFiles())
	projects := project.NewResolver(fsys)
	planner := app.NewPlanner(env, projects, configs)
	client := docker.NewClient(system.Executor{}, fsys, platform, stdio)
	exts := extension.NewLoader(fsys, env)
	ensurer := app.NewImageEnsurer(client, version)
	services := cli.Services{
		Run:   app.NewRunner(exts, planner, client, ensurer, client, stdio.Err),
		Build: app.NewBuilder(exts, client, ensurer, stdio.Err),
		Doctor: app.NewDoctor(app.DoctorDeps{
			Daemon: client, Store: client, Prober: client, Extensions: exts, Project: projects,
			Env: env, Host: app.Host{OS: platform.OS, Arch: platform.Arch}, Version: version,
		}),
		List:  app.NewLister(env, configs, agents.All()),
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
