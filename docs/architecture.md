# caged architecture

## Launch pipeline

```text
caged codex 'fix tests'
   |
 [1 parse]    internal/cli      argv + flags -> app.Request (agent looked up, env names validated,
   |                            -p values kept as strings in Request.Ports); the doctor, list,
   |                            build and clean lines it prints go through app.PrintableLine
   |                            (control characters, newlines and tabs escaped)
   |
 [2 plan]     internal/app      Runner.Run first loads the image extension (a bad config.yaml fails here),
   |                            then Planner.Plan: ports (plan.ParsePorts, first) + project mount +
   |                            config mounts (agent configs, .gitconfig, and caged's own config folder) + env names
   |                            -> plan.RunSpec (pure data, no Docker knowledge);
   |                            warnings and --verbose lines go through app.Reporter
   |                            (control characters escaped, as in ConfigGuard's review).
   |                            A bad request fails here, before any docker call
   |
 [3 prepare]  internal/app      Runner.Run: Daemon check (docker installed? daemon up?), then
   |                            ImageEnsurer.ensureImages: build or rebuild the runtime image when it is
   |                            missing or out of date, and remove the image it replaced
   |
 [4 execute]  internal/docker   Client.RunContainer: RunSpec -> `docker run -it --rm ...`;
                                staged (Pi) config runs from configstage.Stager.Stage copies:
                                once the container exits, Session.Sync writes its changes back
                                and Session.Cleanup removes the copies; after SIGINT, SIGHUP or
                                SIGTERM (system.Command.CatchSignals), or when a signal ended the
                                docker CLI itself, only Cleanup runs, and *system.SignalError or
                                *system.KilledError carries exit code 128+signal
```

Every stage talks to the next one through a small interface or a plain data
type, so each stage can be unit-tested with fakes and none of them needs a
Docker daemon. Only `internal/system` touches the operating system.

## Life of `caged codex 'fix tests'`

Each step names the function that does it, so the walk can be followed in the
code from `cmd/caged/main.go` to `docker run`.

1. **Process inputs.** `main` collects the real stdio and platform
   (`system.CurrentStdio`, `system.CurrentPlatform`), the version
   (`resolveVersion`) and the agents (`agent.Builtin`), and calls `run`.
2. **Wiring.** `run` builds every component once: `userconfig.NewStore`,
   `app.NewConfigGuard`, `hostconfig.NewResolver`, `project.NewResolver`,
   `app.NewPlanner`, `docker.NewClient` (with `configstage.New` as its stager),
   `app.NewImageEnsurer`, and the use cases behind `cli.Services`. It then
   executes `cli.NewRootCmd` with the arguments `codex` and `fix tests`.
3. **Parsing.** Cobra checks the positional arguments with `cli.positionalArgs`
   (at most an agent and a prompt before `--`). The root command's `RunE` calls
   `splitArgs` (before and after `--`), then `buildRequest`, which looks up
   `codex` in the registry and returns an `app.Request` with the agent, the
   prompt `fix tests` and the flags.
4. **Config review.** `RunE` calls `cli.Services.Run`, which is
   `app.GuardedRunner.Run`. `ConfigGuard.prepare` creates the starter
   `config.yaml` when there is none (`Store.Ensure`), reads the exact bytes
   (`Store.Content`), compares them with the approved copy
   (`Approvals.Approved`), and on a change shows the diff and asks; the first
   copy, or an approved change, is recorded with `Approvals.Approve`.
5. **Image choice.** `app.Runner.Run` loads `image.apt` (`Store.Load`, through
   `Extensions`); `images.RefFor` names the image to run: `caged-runtime:latest`,
   or `caged-runtime:ext` when there are packages.
6. **Planning (pure, no Docker).** `Planner.Plan` parses `-p` with
   `plan.ParsePorts`, prints the credentials warning, and asks two resolvers:
   - `Planner.projectMount` -> `project.Resolver.Resolve(workDir, home,
     allowBroadMount)`: the current directory becomes `/workspace/<folder>`,
     after the `/`, home and `workdirs.allow` checks.
   - `Planner.configMounts` -> `hostconfig.Resolver.Resolve(home)`: for every
     agent's `ConfigPath`, `resolution.agentPath` resolves and checks the root
     (`resolveRoot`, `checkRoot`). Codex's `~/.codex` has `MaskPaths`, so it is
     checked by `validateMaskedConfigTree`, mounted whole with tmpfs masks
     (`addMount`), and each link out of it is followed by `followMaskedLink`.
     Other trees go through `mountTree` (split trees, `followLink`), and Pi's
     through `mountStagedTree`. Then `sharedFile` adds `~/.gitconfig`
     read-only, `cagedDir` adds caged's own config folder, and
     `dropNestedLinkMounts` refuses link mounts inside other mounts (staged
     mounts are left to configstage, which nests them inside its copy).
   `Agent.ResolveEnv` keeps the credential names that are set, and
   `buildRunSpec` calls `Agent.LaunchCommand`, which gives
   `codex exec 'fix tests'` (Codex's prompt style). The result is a
   `plan.RunSpec`.
7. **Docker ready.** `Runner.Run` checks the daemon (`docker.Client.Daemon`,
   `docker info`), then `ImageEnsurer.ensureImages` -> `refresh` ->
   `ensureTarget` for each image (`InspectImage`, `images.Target.Assess`, and
   a build when it is missing or stale, `BuildImage`); then `refresh` calls
   `ImageEnsurer.cleanup`, which removes the images the builds replaced
   (`RemoveImage`, the extension's first).
8. **Run.** `docker.Client.RunContainer` stages the config mounts that need
   it (`configstage.Stager.Stage`; with a Pi config that has masks or nested
   mounts, see "Pi staging" below), creates the empty file bound over file
   masks (`createMaskFile`; only an unstaged mount can still have file masks,
   so a normal run creates none), and `Client.run` executes `docker run -it --rm ...`
   built by `runArgs` through `system.Executor.Run` with `CatchSignals`. A
   failure is explained by `classifyRunError` (port in use, path not shared).
9. **After the container.** `finish` writes the staged copies back
   (`Session.Sync`) and removes them (`Session.Cleanup`); after a signal it
   only removes them. Back in `run`, an error is printed with
   `Reporter.Notice`, and the exit code is the error's `ExitCode()` (the
   container's own code, or 128+signal), else 1.

## Pi staging

Pi's config folders (`~/.pi`, `~/.config/pi`) are `Staged`. Stow-managed
folders hold symlinks, and their targets have to be mounted inside the
folder's own mount. Docker resolves a nested mount destination through any
symlink inside the parent bind mount, so mounting those targets, or masks,
directly into the real folder could escape it. A staged tree that needs masks
or nested mounts therefore runs from a symlink-free copy; one that needs
neither is mounted as it is. `hostconfig` describes the staged mount
(`mountStagedTree`: its directory and file masks and its nested link mounts);
`configstage` makes and writes back the copy:

- `Stager.Stage` copies each staged tree that has masks or nested mounts
  into one temporary directory (`caged-pi-config-*`) and returns a `Session`;
  `Session.Mounts` points those mounts at the copies, and passes the other
  mounts through unchanged. When nothing needs a copy, the session is
  `Unstaged`: Sync and Cleanup do nothing.
- `Session.Sync`, after the container stops, writes back what the container
  changed against the baseline taken before it started: new and modified
  files, permission changes, and deletions, also inside subdirectories (a
  file deleted in `sub/` is removed there; a same-named top-level file stays).
- When the host changed the same entry differently during the run, the host
  version is kept and the conflict is reported ("both changed"). A host
  entry added during the run is kept. A host symlink is never replaced, and
  a symlink or special file the container created is never written back.
- `Session.Cleanup` removes the copies after a run, and also when staging or
  the mask file fails before the run. After a signal, Sync is skipped and
  the error says the changes were not written back.
- Each staged tree's masks are recorded on its own mount
  (`TestResolveStagedTreeTwiceKeepsMaskFilesOnTheRoot`), so the right copy
  leaves them out; a copied mount needs no file mask at run time.

Sync can be called again (it advances each copy's baseline), which is the
seam a future long-lived session would use.

Existing agent configs mount read-write so settings and login changes persist.
The shared `.gitconfig` mount is read-only. Missing config paths are skipped.
Special files (sockets, FIFOs, devices) are never mounted: a split tree leaves
them out and refuses only them, a staged (Pi) tree masks them, and a Codex home
with one outside its masks is refused whole. `--no-config` disables these mounts.

Caged's own config folder (`userconfig.Store.Dir`) is mounted read-write at
`/home/caged/.config/caged` by `hostconfig.Resolver` (field `Deps.ConfigDirs`), so
it is part of the same `ConfigReport` as the agent mounts. It may live outside
the home directory (absolute `XDG_CONFIG_HOME`). When it is reached through a
symlink it is judged by `links.allow` like any link target, using
`pathpolicy.Policy.Without` to set aside only its own built-in deny entries
(`~/.config/caged` and the real folder) for that one tree (each mount owner
carries the policy that judges its links); links from agent config into it are
still refused. Linked or not, it is refused when it is the home directory,
touches another deny root, or is, holds or sits inside the approval state
folder (`ConfigDirs.StateDir`), which setting its own entries aside would
otherwise mount. `userconfig.Store.LinkPolicy` passes the real config
folder and the approval state folder to `pathpolicy.BuiltinDeny`, so both are
denied wherever the XDG variables put them. A deny entry that does not exist
yet also gets its deepest existing parent resolved (`Store.denyRoot`), so a
symlinked home or a Stow-linked `~/.local` does not hide it.

### First use and change review

The config file is created and reviewed by `app.ConfigGuard`, applied as a
decorator around each use case (`GuardedRunner`, `GuardedBuilder`,
`GuardedDoctor`, `GuardedLister`) in `cmd/caged`, so the use cases themselves
stay unchanged and `help`, `--version` and `clean` never reach it.
`ConfigGuard.prepare` does three things: `Store.Ensure` creates the folder and starter file
(skipped with `--no-config`; exclusive create through `system.FS.CreateFile`, a
temp file hard-linked into place so the file appears complete or not at all);
`Store.Content` returns the exact bytes the store parsed, so what the user
reviews is what caged acts on; and `ConfigGuard` compares them with the last
approved copy, which `userconfig.Approvals` keeps in `$XDG_STATE_HOME/caged`,
outside any mount. The diff
is computed by `internal/textdiff` (pure). Under `prepare` (run, build) a change
needs a `y` at a terminal or `--trust-config`; `doctor` and `list` call
`notify`, which only reports it and never stops the command. Every command
that reviews the file records the first copy it sees without asking; when that
record fails, `notify` warns and goes on, while `prepare` stops.

Symlinks in agent config (Stow-managed dotfiles, for example) are followed on
the host, never by Docker. Each target is resolved and checked by
`pathpolicy` against the user's `links.allow` roots and the built-in deny list
(`pathpolicy.BuiltinDeny`); a target inside the link's own config folder needs
no policy. A config root that is not a symlink must be inside home and must
not be inside or hold a built-in deny root (an override such as
`CLAUDE_CONFIG_DIR=~/.ssh`); `links.deny` judges links only. Where a mount goes
depends on how the config is mounted:

- **Split trees.** A directory that holds symlinks is not mounted whole. Its
  symlink-free subtrees are mounted directly, and each symlink's target is
  mounted at the logical path of the link (`~/.claude/CLAUDE.md` becomes a
  mount of the target at `/home/caged/.claude/CLAUDE.md`). A config root that
  is itself a symlink is mounted at the root's own container path.
- **Whole trees** (Codex, whose runtime directories are masked with tmpfs). The
  directory is mounted whole, so its symlinks are visible in the container and
  resolve there. Each link that leaves the tree is followed by mounting its
  target where the link points inside the container, computed by
  `containerLinkDest`: a relative link must stay below `/home/caged` and climb
  with `..` only at its start; an absolute link must lie in the host home or a
  `links.allow` root and outside the system directories.

A symlink mount whose container path lies inside another mount is refused
(Docker would create the mount point inside that host directory). At most 64
symlinks are followed, and symlinked directories nest at most 8 deep.

Known limits: the deny list does not apply to files directly inside an agent
config directory (those are the agent's own files, mounted as before), and
paths are checked when the run is planned, not when Docker starts the
container (a time-of-check to time-of-use gap).

## Package layout

```text
cmd/caged/            composition root: the only place that picks concrete types
internal/cli/         Cobra commands (one file each) and flags -> app requests; nothing else
internal/app/         use cases: Runner (run), Builder (build), Doctor, Lister, Cleaner; ConfigGuard and its Guarded* decorators (config review);
                      Planner (Request -> RunSpec; runspec.go assembles it); ImageEnsurer (images current); Reporter (stderr lines)
internal/plan/        specs that flow down: RunSpec, Mount, ConfigMount, ConfigReport, BuildSpec
internal/userconfig/  locate, read, and validate the user's config.yaml -> plan.Extension, plan.ConfigStatus, and the link and workdir policies
internal/pathpolicy/  allow and deny lists for host paths (no IO) -> pathpolicy.Policy
internal/textdiff/    unified line diff of two texts (no IO), used to show config changes
internal/engine/      facts and errors that flow up: Kind, Error, DaemonInfo, ImageInfo
internal/agent/       Agent definition, Registry, one file per built-in agent
internal/project/     project directory validation -> plan.Mount
internal/hostconfig/  host config discovery and safety policy -> plan.ConfigReport
internal/configstage/ runs staged config mounts (Pi) from symlink-free copies, writes changes back
internal/docker/      Docker CLI adapter: RunSpec/BuildSpec -> docker commands
internal/images/      the runtime image: tag + embedded Dockerfile and entrypoint
internal/system/      thin OS adapters: FS, Env, Executor, Platform, Stdio
internal/testkit/     test fakes shared by several packages (Env); imported only by _test.go files
```

## Dependency direction

| Package | May import (caged packages) |
|---|---|
| `cmd/caged` | `agent`, `app`, `cli`, `configstage`, `docker`, `hostconfig`, `project`, `system`, `userconfig` |
| `internal/cli` | `agent`, `app` |
| `internal/app` | `agent`, `engine`, `images`, `pathpolicy`, `plan`, `textdiff` |
| `internal/configstage` | `plan` |
| `internal/docker` | `configstage`, `engine`, `plan`, `system` |
| `internal/engine` | nothing |
| `internal/hostconfig` | `agent`, `pathpolicy`, `plan` |
| `internal/pathpolicy` | nothing |
| `internal/project` | `pathpolicy`, `plan` |
| `internal/userconfig` | `pathpolicy`, `plan` |
| `internal/images` | `plan` |
| `internal/plan` | nothing |
| `internal/agent` | nothing |
| `internal/system` | nothing |
| `internal/textdiff` | nothing |
| `internal/testkit` | nothing (and only `_test.go` files may import it) |

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

`internal/testkit` holds the `Env` fake shared by the app, userconfig and
hostconfig tests. Only `_test.go` files import it: no row above lists it, so a
production import fails (`TestTestkitIsOnlyForTests`). Test imports are
otherwise not checked.

## Key interfaces

Interfaces are declared by the package that consumes them and contain only
the methods that consumer calls. The production implementation is listed on
the right.

| Consumer | Interface | Methods | Implemented by |
|---|---|---|---|
| `cli` | `RunService` | `Run(ctx, app.Request) error` | `app.GuardedRunner` (wraps `*app.Runner`) |
| `cli` | `BuildService` | `Build(ctx, app.BuildOptions) ([]app.BuiltImage, error)` | `app.GuardedBuilder` (wraps `*app.Builder`) |
| `cli` | `DoctorService` | `Diagnose(ctx, app.DoctorOptions) app.DoctorReport` | `app.GuardedDoctor` (wraps `*app.Doctor`) |
| `cli` | `ListService` | `List() ([]app.AgentStatus, error)` | `app.GuardedLister` (wraps `*app.Lister`) |
| `cli` | `CleanService` | `Clean(ctx, app.CleanOptions) ([]app.CleanResult, error)` | `*app.Cleaner` |
| `app` | `Extensions` | `Load() (*plan.Extension, error)`, `Source() (string, error)` | `*userconfig.Store` |
| `app` | `ConfigFile` | `Status() (plan.ConfigStatus, error)` | `*userconfig.Store` |
| `app` | `ImageInspector` | `InspectImage(ctx, ref) (engine.ImageInfo, error)` | `*docker.Client` |
| `app` | `ImageLister` | `ListImages(ctx, engine.ImageQuery) ([]engine.ImageSummary, error)` | `*docker.Client` |
| `app` | `Prober` | `ReadImageFile(ctx, ref, path) ([]byte, error)`, `ProbeMount(ctx, ref, plan.Mount) error` | `*docker.Client` |
| `app` | `Daemon` | `Daemon(ctx) (engine.DaemonInfo, error)` | `*docker.Client` |
| `app` | `ImageStore` | `InspectImage(ctx, ref) (engine.ImageInfo, error)`, `BuildImage(ctx, plan.BuildSpec) (id string, err error)`, `RemoveImage(ctx, idOrRef) error` | `*docker.Client` |
| `app` | `ContainerRunner` | `RunContainer(ctx, plan.RunSpec) error` | `*docker.Client` |
| `app` | `RunPlanner` | `Plan(Request, imageRef string, *Reporter) (plan.RunSpec, error)` | `*app.Planner` |
| `app` | `Env` | `LookupEnv(key) (string, bool)`, `UserHomeDir() (string, error)`, `Getwd() (string, error)` | `system.Env` |
| `app` | `ProjectResolver` | `Resolve(workDir, home string, allowBroadMount bool) (*plan.Mount, error)` | `*project.Resolver` |
| `app` | `ConfigResolver` | `Resolve(home string) (plan.ConfigReport, error)` | `*hostconfig.Resolver` |
| `app` | `ConfigInitializer` | `Ensure() (created bool, err error)` | `*userconfig.Store` |
| `app` | `ConfigContent` | `Content() ([]byte, bool, error)`, `Source() (string, error)` | `*userconfig.Store` |
| `app` | `ConfigApprovals` | `Approved() ([]byte, bool, error)`, `Approve([]byte) error` | `*userconfig.Approvals` |
| `userconfig` | `FileSystem` | `ReadRegularFile`, `Stat`, `EvalSymlinks`, `MkdirAllPrivate`, `CreateFile` | `system.FS` |
| `userconfig` | `StateFS` | `ReadFile`, `MkdirAllPrivate`, `ReplaceFile` | `system.FS` |
| `userconfig` | `ConfigPath` | `Path() (string, error)` | `*userconfig.Store` |
| `userconfig` | `Env` | `LookupEnv(key) (string, bool)`, `UserHomeDir() (string, error)` | `system.Env` |
| `hostconfig` | `FileSystem` | `Stat`, `Lstat`, `ReadDir`, `EvalSymlinks`, `WalkDir`, `Readlink` | `system.FS` |
| `hostconfig` | `ConfigDirs` | `Dir() (string, error)`, `StateDir() (string, error)` | `*userconfig.Store` |
| `hostconfig` | `LinkPolicies` | `LinkPolicy(home string) (*pathpolicy.Policy, error)` | `*userconfig.Store` |
| `project` | `WorkdirPolicies` | `WorkdirPolicy(home string) (*pathpolicy.Policy, error)` | `*userconfig.Store` |
| `hostconfig` | `Env` | `LookupEnv(key) (string, bool)` | `system.Env` |
| `project` | `FileSystem` | `Abs`, `Stat`, `EvalSymlinks` | `system.FS` |
| `docker` | `Executor` | `LookPath(file) (string, error)`, `Run(ctx, system.Command) error` | `system.Executor` |
| `docker` | `FileSystem` | `MkdirTemp`, `WriteFile`, `ReadFile`, `RemoveAll` | `system.FS` |
| `docker` | `ConfigStager` | `Stage([]plan.ConfigMount) (*configstage.Session, error)` | `*configstage.Stager` |
| `configstage` | `FileSystem` | `Lstat`, `ReadDir`, `WalkDir`, `ReadFile`, `WriteFile`, `ReplaceFile`, `Mkdir`, `MkdirAll`, `MkdirTemp`, `Chmod`, `Remove`, `RemoveAll` | `system.FS` |

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
injected into `docker.Client` and, as `app.Host`, into the doctor. Nothing
reads `runtime.GOOS` except `system.CurrentPlatform`, which builds that value. `internal/docker/platform.go` holds the
OS-specific rules for docker, and `internal/app/doctor_checks.go` the doctor's
(the Docker Desktop, Rosetta and file-sharing messages):

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
- macOS's file systems ignore case in names by default, so `~/.SSH` is `~/.ssh`.
  `system.Platform.FoldsCase` sets `hostconfig.Deps.FoldCase`, and the resolver then
  uses `pathpolicy.Policy.FoldCase`: deny roots, the home directory and the overlap of
  caged's config and approval-state folders (`Policy.Overlap`) match in any case.
  Allow roots still match exactly, so a name in other letters can only be refused.
  The folding is `strings.ToLower`: Unicode normalization (NFC against NFD) is not
  folded.

Both branches are unit-tested by injecting `OS: "linux"` or `OS: "darwin"`.
The golden tests run every command line for both platforms. The end-to-end
suite in `dev/e2e` runs on Linux and on macOS (`make e2e-macos`, with a
macOS-only section M); `dev/e2e/README.md` describes it. Paths are always
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
`version` label of every image it builds, and to the doctor's header. Releases are automated
(`.github/workflows/release.yml`, the branch rules in [CONTRIBUTING.md](../CONTRIBUTING.md));
`make release-check` validates the packaging locally without publishing.

## Adding a new agent

1. Create `internal/agent/<name>.go` containing one function that returns the
   `Agent` value: its command, prompt style, config paths (with `Writable`,
   and `MaskPaths`, `EnvOverride` or `Staged` when it needs them) and the
   credential variables it reads.
2. Add one line, `<name>(),`, to the list in `internal/agent/builtin.go`.
3. Install it in `internal/images/Dockerfile`: a `RUN` step in the build
   stage (and a `COPY --from=caged-build` of what it installed), its config
   directories under `/home/caged` in the final stage's `mkdir -p` and
   `chmod 0777` lists (Linux runs as the host user, who must be able to write
   them), a link onto `PATH` in the final stage when it is installed with npm
   (as Gemini and Copilot are), and its name in the `for cli in ...` loop that
   records versions for `caged doctor`.
4. Update `internal/agent/testdata/builtin_agents.golden`, which
   `TestBuiltinAgentDefinitionsAreUnchanged` compares with every built-in
   definition. It is edited by hand: `CAGED_UPDATE_GOLDEN=1` only creates a
   missing golden file and never overwrites one. The command goldens in
   `cmd/caged/testdata` do not change: they run with the six original agents
   (`originalAgents`).

No Go code outside `internal/agent` changes. `TestBuiltinRegistryContract` and
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

If the user's `config.yaml` lists `image.apt` packages (`internal/userconfig`), caged builds a second
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
every run until the file is fixed. With no packages, runs use `caged-runtime:latest`
and a leftover `caged-runtime:ext` tag is ignored.
