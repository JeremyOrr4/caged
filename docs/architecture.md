# caged architecture

## Launch pipeline

```text
caged codex 'fix tests'
   |
 [1 parse]    internal/cli      argv + flags -> app.Request (agent looked up, env names validated)
   |
 [2 prepare]  internal/app      Service.Run: build the runtime image first if it is missing
   |
 [3 plan]     internal/app      Planner.Plan: project mount + config mounts + env names
   |                            -> plan.RunSpec (pure data, no Docker knowledge)
   |
 [4 execute]  internal/docker   Client.Run: RunSpec -> `docker run -it --rm ...`
```

Every stage talks to the next one through a small interface or a plain data
type, so each stage can be unit-tested with fakes and none of them needs a
Docker daemon. Only `internal/system` touches the operating system.

Existing agent configs mount read-write so settings and login changes persist.
The shared `.gitconfig` mount is read-only. Missing config paths are skipped;
paths and symlinks resolving outside the home directory, and trees containing
special files, are refused. `--no-config` disables these mounts.

## Package layout

```text
cmd/caged/            composition root: the only place that picks concrete types
internal/cli/         Cobra commands and flags -> app.Request; nothing else
internal/app/         use cases: Service (Run, Build) and Planner (Request -> RunSpec)
internal/plan/        neutral data: RunSpec, Mount, ConfigMount, ConfigReport, BuildSpec
internal/agent/       Agent definition, Registry, one file per built-in agent
internal/project/     project directory validation -> plan.Mount
internal/hostconfig/  host config discovery and safety policy -> plan.ConfigReport
internal/docker/      Docker CLI adapter: RunSpec/BuildSpec -> docker commands
internal/runtime/     the runtime image: tag + embedded Dockerfile and entrypoint
internal/system/      thin OS adapters: FS, Env, Executor, Platform, Stdio
```

## Dependency direction

| Package | May import (caged packages) |
|---|---|
| `cmd/caged` | `agent`, `app`, `cli`, `docker`, `hostconfig`, `project`, `system` |
| `internal/cli` | `agent`, `app`, `runtime` |
| `internal/app` | `agent`, `plan`, `runtime` |
| `internal/docker` | `plan`, `system` |
| `internal/hostconfig` | `agent`, `plan` |
| `internal/project` | `plan` |
| `internal/runtime` | `plan` |
| `internal/plan` | `agent` |
| `internal/agent` | nothing |
| `internal/system` | nothing |

Dependencies point from policy to the things policy needs. `app` (the use
cases) imports no adapter package: it does not know about Docker, the
filesystem, or the environment. Those reach it through interfaces that `app`
itself declares. `agent` and `system` import only the standard library. There
are no import cycles. `cmd/caged/architecture_test.go` enforces this table
exactly. It also fails if any package other than `internal/system` and
`cmd/caged` imports `os`, `os/exec`, `os/signal`, `os/user`, `syscall`, or the
standard `runtime` package. It fails if any package other than
`internal/system` calls `filepath.Abs`, `EvalSymlinks`, `Glob`, `Walk`, or
`WalkDir`. It also fails on any package-level `var`, except `main.version`
(set by `-ldflags`) and the `//go:embed` files.

## Key interfaces

Interfaces are declared by the package that consumes them and contain only
the methods that consumer calls. The production implementation is listed on
the right.

| Consumer | Interface | Methods | Implemented by |
|---|---|---|---|
| `cli` | `Service` | `Run(ctx, app.Request) error`, `Build(ctx, noCache bool) error` | `*app.Service` |
| `cli` | `Agents` | `Lookup(name) (agent.Agent, error)`, `Names() []string` | `*agent.Registry` |
| `app` | `Engine` | `ImageExists(ctx, image) bool`, `Build(ctx, plan.BuildSpec) error`, `Run(ctx, plan.RunSpec) error` | `*docker.Client` |
| `app` | `Env` | `LookupEnv(key) (string, bool)`, `UserHomeDir() (string, error)`, `Getwd() (string, error)` | `system.Env` |
| `app` | `ProjectResolver` | `Resolve(workDir, home string, allowBroadMount bool) (*plan.Mount, error)` | `*project.Resolver` |
| `app` | `ConfigResolver` | `Resolve(home string) plan.ConfigReport` | `*hostconfig.Resolver` |
| `hostconfig` | `FileSystem` | `Stat`, `Lstat`, `ReadDir`, `EvalSymlinks`, `WalkDir` | `system.FS` |
| `hostconfig` | `Env` | `LookupEnv(key) (string, bool)` | `system.Env` |
| `project` | `FileSystem` | `Abs`, `Stat`, `EvalSymlinks` | `system.FS` |
| `docker` | `Executor` | `LookPath(file) (string, error)`, `Run(ctx, system.Command) error` | `system.Executor` |
| `docker` | `FileSystem` | `MkdirTemp`, `WriteFile`, `RemoveAll` | `system.FS` |

Agents are data, not an interface. Every agent has the same shape (command,
prompt style, config paths, environment names), and behaviour that depends on
that data is a method on `agent.Agent` (`LaunchCommand`, `EnvNames`,
`ResolveEnv`). A Go interface with one type per agent would add six types and
no flexibility. Because there is only one type, every agent is
substitutable and no generic code checks an agent's name.

## Composition root

`cmd/caged/main.go` is the composition root. `main` collects the real
process inputs (`system.CurrentStdio()`, `system.CurrentPlatform()`,
`agent.Builtin()`) and passes them to `run`. `run` builds every component,
wires them together, executes the command line, and returns the exit code.
Tests call `run` with a fake `docker` on `PATH`, captured stdio, and an
injected platform. This lets the full Linux and macOS behaviour be checked
on any machine. Cobra keeps its default output writers on purpose: calling
`SetOut` would also move Cobra's stderr-bound usage text to stdout.

## Platform-specific behaviour

The host platform (`system.Platform{OS, Arch, UID, GID}`) is a value that is
injected into `docker.Client`. Nothing reads `runtime.GOOS` directly.
`internal/docker/platform.go` holds all of the OS-specific policy:

- Linux runs the container as the host UID:GID (`--user`). macOS keeps the
  image user, because Docker Desktop translates bind-mount ownership.
- On macOS, a failed run whose stderr says a path is not shared gets the
  Docker Desktop "File Sharing" hint.
- The daemon-down message says "start Docker Desktop" on macOS and "start
  the Docker daemon" elsewhere.
- Only amd64 and arm64 hosts are supported.

Both branches are unit-tested by injecting `OS: "linux"` or `OS: "darwin"`.
The golden tests run every command line for both platforms. Paths are always
built with `path/filepath` on the host side and with forward slashes on the
container side. macOS `/tmp` -> `/private/tmp` style symlinks are resolved
with `EvalSymlinks` before any comparison.

## Adding a new agent

Before this refactor:

1. Add an entry to the global `registry` map literal in `internal/agent/agent.go`.
2. Edit the two hard-coded agent lists in `internal/agent/agent_test.go`.
3. If the agent needed a new prompt style, edit the `switch` in `internal/plan/run.go`.

After this refactor:

1. Create `internal/agent/<name>.go` containing one function that returns the
   `Agent` value.
2. Add one line, `<name>(),`, to the list in `internal/agent/builtin.go`.

Nothing else changes. `TestBuiltinRegistryContract` and
`TestAgentNamesDoNotCollideWithSubcommands` check the new entry automatically.
The config resolver, planner, CLI completion, and `caged list` pick it up from
the registry.

## Adding a container runtime or a host-config source

- A new container runtime (for example Podman) is a new package with a type
  that implements `app.Engine`, plus one changed line in `cmd/caged/main.go`.
- A new agent config directory is one more `ConfigPath` in that agent's file.
- A new shared file (like `.gitconfig`) is one more entry in
  `hostconfig.DefaultSharedFiles()`.

## Project and prompt behavior

The selected host directory (the current directory by default) is mounted at
`/workspace/<folder-name>`, which is also the container working directory.
Mounting `/` or the complete home directory requires `--allow-broad-mount`.
`--no-mount` disables the project bind mount.

`caged <agent> '<prompt>'` passes one prompt argument according to the agent's
registered style: positional for Claude and Pi, `exec <prompt>` for Codex,
`-i <prompt>` for Gemini, and `-p <prompt>` for GitHub Copilot CLI. Agent
launches return to an interactive shell in the same container after the agent
exits.

## Environment and credentials

Each agent's auth metadata declares environment names and known credential
paths. Repeatable `--env NAME` adds names; only set variables are passed to
Docker, and values are never logged. `--verbose` reports mounts and passed or
skipped environment names.

Because agent configs mount read-write, their settings and credentials are
available in the container and changes persist on the host. On macOS, Claude
Code's Keychain session is not forwarded. Subscription users log in once inside
the container, creating a separate plaintext credential under the mounted
config. Codex mounts `~/.codex` as a directory so atomic config writes persist;
container tmpfs mounts hide its host daemon and socket paths. GitHub CLI reads
and writes `~/.config/gh`, with `GH_TOKEN` and `GITHUB_TOKEN` forwarded when set.
Keychain extraction, ports, and non-Docker-Desktop runtimes are out of scope
for v1. Claude Code's `CLAUDE_CONFIG_DIR` points to
`/home/caged/.claude` so config writes stay inside the directory bind mount;
the host's `~/.claude.json` is not shared because Docker Desktop cannot replace
a mounted single file by rename.

## Runtime image

`internal/runtime` embeds the runtime Dockerfile and entrypoint and describes
them as a `plan.BuildSpec`; `internal/docker` writes them to a temporary build
context and runs `docker build`. The image installs Claude Code, Pi, Codex,
Gemini CLI, GitHub CLI (`gh`), and GitHub Copilot CLI. It is built on first use
when absent. `caged build --no-cache` refreshes the latest installer versions.
The development-only Dockerfile is `dev/Dockerfile`.
