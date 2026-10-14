# caged architecture

## Launch pipeline

```text
caged codex 'fix tests'
   |
 [1 parse]    internal/cli      argv + flags -> app.Request (agent looked up, env names validated,
   |                            -p values kept as strings in Request.Ports)
   |
 [2 plan]     internal/app      Runner.Run first loads the image extension (a bad image.json fails here),
   |                            then Planner.Plan: ports (plan.ParsePorts, first) + project mount +
   |                            config mounts + env names
   |                            -> plan.RunSpec (pure data, no Docker knowledge);
   |                            warnings and --verbose lines go through app.Reporter.
   |                            A bad request fails here, before any docker call
   |
 [3 prepare]  internal/app      Runner.Run: Daemon check (docker installed? daemon up?), then
   |                            ImageEnsurer.Ensure: build or rebuild the runtime image when it is
   |                            missing or out of date, and remove the image it replaced
   |
 [4 execute]  internal/docker   Client.RunContainer: RunSpec -> `docker run -it --rm ...`
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
internal/cli/         Cobra commands (one file each) and flags -> app requests; nothing else
internal/app/         use cases: Runner (run) and Builder (build), and Planner (Request -> RunSpec; runspec.go assembles the RunSpec)
internal/plan/        specs that flow down: RunSpec, Mount, ConfigMount, ConfigReport, BuildSpec
internal/extension/   locate, read, and validate the user's image.json -> plan.Extension
internal/engine/      facts and errors that flow up: Kind, Error, DaemonInfo, ImageInfo
internal/agent/       Agent definition, Registry, one file per built-in agent
internal/project/     project directory validation -> plan.Mount
internal/hostconfig/  host config discovery and safety policy -> plan.ConfigReport
internal/docker/      Docker CLI adapter: RunSpec/BuildSpec -> docker commands
internal/images/      the runtime image: tag + embedded Dockerfile and entrypoint
internal/system/      thin OS adapters: FS, Env, Executor, Platform, Stdio
```

## Dependency direction

| Package | May import (caged packages) |
|---|---|
| `cmd/caged` | `agent`, `app`, `cli`, `docker`, `extension`, `hostconfig`, `project`, `system` |
| `internal/cli` | `agent`, `app` |
| `internal/app` | `agent`, `engine`, `images`, `plan` |
| `internal/docker` | `engine`, `plan`, `system` |
| `internal/engine` | nothing |
| `internal/extension` | `plan` |
| `internal/hostconfig` | `agent`, `plan` |
| `internal/project` | `plan` |
| `internal/images` | `plan` |
| `internal/plan` | nothing |
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
| `cli` | `RunService` | `Run(ctx, app.Request) error` | `*app.Runner` |
| `cli` | `BuildService` | `Build(ctx, app.BuildOptions) ([]app.BuiltImage, error)` | `*app.Builder` |
| `cli` | `DoctorService` | `Diagnose(ctx, app.DoctorOptions) app.DoctorReport` | `*app.Doctor` |
| `cli` | `ListService` | `List() ([]app.AgentStatus, error)` | `*app.Lister` |
| `cli` | `CleanService` | `Clean(ctx, app.CleanOptions) ([]app.CleanResult, error)` | `*app.Cleaner` |
| `cli` | `Agents` | `Lookup(name) (agent.Agent, error)`, `Names() []string` | `*agent.Registry` |
| `app` | `Extensions` | `Load() (*plan.Extension, error)`, `Source() (string, error)` | `*extension.Loader` |
| `app` | `ImageInspector` | `InspectImage(ctx, ref) (engine.ImageInfo, error)` | `*docker.Client` |
| `app` | `ImageLister` | `ListImages(ctx, engine.ImageQuery) ([]engine.ImageSummary, error)` | `*docker.Client` |
| `app` | `Prober` | `ReadImageFile(ctx, ref, path) ([]byte, error)`, `ProbeMount(ctx, ref, plan.Mount) error` | `*docker.Client` |
| `app` | `Daemon` | `Daemon(ctx) (engine.DaemonInfo, error)` | `*docker.Client` |
| `app` | `ImageStore` | `InspectImage(ctx, ref) (engine.ImageInfo, error)`, `BuildImage(ctx, plan.BuildSpec) (id string, err error)`, `RemoveImage(ctx, id) error` | `*docker.Client` |
| `app` | `ContainerRunner` | `RunContainer(ctx, plan.RunSpec) error` | `*docker.Client` |
| `app` | `RunPlanner` | `Plan(Request, imageRef string, *Reporter) (plan.RunSpec, error)` | `*app.Planner` |
| `app` | `Env` | `LookupEnv(key) (string, bool)`, `UserHomeDir() (string, error)`, `Getwd() (string, error)` | `system.Env` |
| `app` | `ProjectResolver` | `Resolve(workDir, home string, allowBroadMount bool) (*plan.Mount, error)` | `*project.Resolver` |
| `app` | `ConfigResolver` | `Resolve(home string) plan.ConfigReport` | `*hostconfig.Resolver` |
| `extension` | `FileSystem` | `ReadFile` | `system.FS` |
| `extension` | `Env` | `LookupEnv(key) (string, bool)`, `UserHomeDir() (string, error)` | `system.Env` |
| `hostconfig` | `FileSystem` | `Stat`, `Lstat`, `ReadDir`, `EvalSymlinks`, `WalkDir` | `system.FS` |
| `hostconfig` | `Env` | `LookupEnv(key) (string, bool)` | `system.Env` |
| `project` | `FileSystem` | `Abs`, `Stat`, `EvalSymlinks` | `system.FS` |
| `docker` | `Executor` | `LookPath(file) (string, error)`, `Run(ctx, system.Command) error` | `system.Executor` |
| `docker` | `FileSystem` | `MkdirTemp`, `WriteFile`, `ReadFile`, `RemoveAll` | `system.FS` |

Failures from the container engine are `*engine.Error` values with a `Kind`
(not installed, daemon down, unsupported host, image not found, and so on).
Callers branch with `engine.IsKind`, never on message text. Only
`internal/docker` turns docker's stderr into a `Kind`. Sentinel `var` errors
are banned by the architecture test, so the errors are a type.

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
- Only amd64 and arm64 hosts are supported. `docker info` reports the daemon's architecture
  as `x86_64` or `aarch64`; it is normalized to `amd64` or `arm64`.
- `caged doctor` warns when caged's own architecture differs from the daemon's (an amd64
  caged on Apple silicon runs under Rosetta), and when the engine is older than the
  documented minimum.

Both branches are unit-tested by injecting `OS: "linux"` or `OS: "darwin"`.
The golden tests run every command line for both platforms. Paths are always
built with `path/filepath` on the host side and with forward slashes on the
container side. macOS `/tmp` -> `/private/tmp` style symlinks are resolved
with `EvalSymlinks` before any comparison.

## Doctor, list, and clean

These commands are use cases of their own, so none of them needs the `Runner`'s dependencies.

- `app.Doctor` runs a fixed list of checks (`doctor_checks.go`). Each check names the checks
  it needs; one whose needs failed or were skipped is skipped, so a stopped daemon is one
  FAIL and a column of SKIPs. A `facts` value caches each docker answer, so every question
  is asked once. The doctor's dependencies (`ImageInspector`, `Prober`) have no build or
  remove method: it cannot change anything. Its probes are `docker run --rm --entrypoint cat`
  (the versions file) and `docker run --rm -v <dir>:<dir>:ro --entrypoint true`, without a
  terminal, a user, or extra capabilities. `ChecksFailedError` makes the command exit 1.
- `app.Lister` needs no Docker. It asks the config resolver what it would mount, groups the
  mounts of each agent by `ConfigMount.Root` (the config path after overrides, before
  symlink resolution), and reports which of the agent's credential variables are set, by
  name only.
- `app.Cleaner` lists images by label (`role=extension`, then `role=runtime`) and then by the
  `caged-runtime` repository (older, unlabeled images), de-duplicates them by ID, and removes
  them in that order, never with `-f`.

## Releases

The version string reaches `main.version` at link time: `make build` passes `git describe`,
GoReleaser and the Homebrew formula pass the tag. A build with no linked version (a plain
`go install ...@v0.1.0`) reads the module version from the build information, and a leading
`v` is stripped, so every install channel prints the same text (`cmd/caged/version.go`). The
composition root passes that string to `app.NewImageEnsurer`, which writes it into the
`version` label of every image it builds, and to the doctor's header. Release steps are in
[releasing.md](releasing.md).

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
  that implements `app.ImageStore` and `app.ContainerRunner`, plus one changed line in `cmd/caged/main.go`.
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

Arguments after `--` are agent arguments. The CLI splits them off with
`cmd.ArgsLenAtDash()` (the two-argument limit counts only what comes before
`--`), and they travel as `Request.AgentArgs` to `agent.LaunchCommand`, which
builds the command in one order: the agent's command, the prompt part, then the
agent arguments. Each is one element of `RunSpec.Command` and one `docker run`
argument; the entrypoint runs `"$@"`, so no shell reads them. They go after the
prompt because flags such as `--resume` and `--add-dir` consume the arguments
that follow them. Agent arguments without an agent are an error (checked in the
CLI and again in `buildRunSpec`).

## Environment and credentials

Each agent's auth metadata declares the environment names it accepts. Repeatable `--env NAME` adds names; only set variables are passed to
Docker, and values are never logged. `--verbose` reports mounts and passed or
skipped environment names.

Because agent configs mount read-write, their settings and credentials are
available in the container and changes persist on the host. On macOS, Claude
Code's Keychain session is not forwarded. Subscription users log in once inside
the container, creating a separate plaintext credential under the mounted
config. Codex mounts `~/.codex` as a directory so atomic config writes persist;
container tmpfs mounts hide its host daemon and socket paths. GitHub CLI reads
and writes `~/.config/gh`, with `GH_TOKEN` and `GITHUB_TOKEN` forwarded when set.
Keychain extraction and non-Docker-Desktop runtimes are out of scope
for v1. Claude Code's `CLAUDE_CONFIG_DIR` points to
`/home/caged/.claude` so config writes stay inside the directory bind mount;
the host's `~/.claude.json` is not shared because Docker Desktop cannot replace
a mounted single file by rename.

## Runtime image

`internal/images` embeds the runtime Dockerfile and entrypoint and describes
them as an `images.Target`: the tag, the build context, and the labels that say
what the image was built from. `Target.Spec` turns it into a `plan.BuildSpec`;
`internal/docker` writes it to a temporary build context and runs
`docker build --label ... --iidfile ...`. The image installs Claude Code, Pi,
Codex, Gemini CLI, GitHub CLI (`gh`), and GitHub Copilot CLI. Both `FROM` lines
pin `ubuntu:24.04` by its multi-arch index digest; Dependabot bumps it.

Every image caged builds carries labels under `io.github.jeremyorr4.caged.`:
`role` (`runtime`), `runtime-hash` (SHA-256 over the names, modes, and bytes of
the build files), and `version` (the caged version that built it). Before a
run, `ImageEnsurer` inspects the image and calls `Target.Assess`:

| State | Meaning | What happens |
|---|---|---|
| current | the labels match this binary's files | nothing; `--verbose` prints `image: ... is current` |
| missing | no image with the tag | build, with `Runtime image ... not found; building it first...` |
| unlabeled | built before caged wrote labels | rebuild once, as "built by an older caged" |
| changed | the hash differs | rebuild, naming both caged versions when they differ |

The hash does not include the caged version or `--no-cache`, so an upgrade that
leaves the runtime files unchanged does not rebuild. A failed rebuild is an
error: caged never falls back to the stale image. After a successful rebuild,
caged removes the image the tag used to point to with `docker image rm <id>`,
never with `-f`. If a container or another tag still uses it, the image is kept
(`--verbose` says so) and the run goes on.

`caged build --no-cache` refreshes the latest installer versions. The
development-only Dockerfile is `dev/Dockerfile`.

### Image extension

If the user has an `image.json` (`internal/extension`), caged builds a second
image on top of the runtime image: the tag `caged-runtime:ext`, from a
generated Dockerfile (`FROM caged-runtime:latest`, then one `apt-get install`
of the sorted, de-duplicated package list). The package names are validated
byte by byte, so no shell syntax can reach the `RUN` line. `images.Extension`
describes it as a second `Target`, so the same freshness logic applies: its
labels are `role=extension`, `extension-hash` (a hash of the generated
Dockerfile, which depends only on the package set), `base-id` (the ID of the
runtime image it was built from), and `version`. Rebuilding the runtime image
therefore rebuilds the extension. The extension is built after the runtime
image; when both are replaced, the old extension image is removed before the
old runtime image, because a base cannot be removed while a child image
depends on it. A failed extension build names the file, because it blocks
every run until the file is fixed. With no file, runs use `caged-runtime:latest`
and a leftover `caged-runtime:ext` tag is ignored.
