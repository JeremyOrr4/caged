// Command caged is the command-line entry point for caged.
//
// This file is the composition root: the only place that chooses concrete
// implementations and wires them together.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/app"
	"github.com/JeremyOrr4/caged/internal/cli"
	"github.com/JeremyOrr4/caged/internal/docker"
	"github.com/JeremyOrr4/caged/internal/hostconfig"
	"github.com/JeremyOrr4/caged/internal/project"
	"github.com/JeremyOrr4/caged/internal/system"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	stdio, platform := system.CurrentStdio(), system.CurrentPlatform()
	os.Exit(run(context.Background(), version, os.Args[1:], stdio, platform, agent.Builtin()))
}

// run wires caged together, executes one command line, and returns the
// process exit code. When the container fails, its exit code is passed on.
func run(ctx context.Context, version string, args []string, stdio system.Stdio, platform system.Platform, agents *agent.Registry) int {
	fsys, env := system.FS{}, system.Env{}
	configs := hostconfig.NewResolver(fsys, env, agents.All(), hostconfig.DefaultSharedFiles())
	planner := app.NewPlanner(env, project.NewResolver(fsys), configs, stdio.Err)
	engine := docker.NewClient(system.Executor{}, fsys, platform, stdio)
	service := app.New(engine, planner, stdio.Err)

	root := cli.NewRootCmd(version, agents, service)
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
	fmt.Fprintln(stdio.Err, err)
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return 1
}
