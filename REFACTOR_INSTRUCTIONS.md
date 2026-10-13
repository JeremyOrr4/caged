# caged architecture refactor: step-by-step instructions

This file turns the original `caged` repository into the refactored one, one small step at a time.
If you follow it exactly, your result is byte-for-byte identical to the refactored repository.
This was checked by a literal replay (see section 7.5).

- Starting point: the original repository. Its newest commit is `69a9068` ("update makefile").
  Its fingerprint is in section 1.5.
- End point: 156 files (the original had 31),
  88 top-level tests (the original had 36), and identical command-line behaviour.
- Steps: 14. The repository builds, passes `go vet`, and passes every test after each step.

# 0. How to use this file

Read this section first. It explains the words and the layout used everywhere else.

## 0.1 The one rule

Do the steps in order, from Step 1 to Step 14. Never skip a step. Never skip the **Verify** part at the end of a step.
If a Verify check fails, stop and fix it before you go on. Each step's **If it fails** list and section 8 tell you how.

## 0.2 Words used in this file

- **Repository root**: the folder that contains the file `go.mod`. Every command in this file is run from the repository root.
- **Path**: a file location written relative to the repository root, with forward slashes, for example `internal/agent/agent.go`.
- **NEW FILE**: a file that does not exist yet. Create it with exactly the text shown.
- **Edit**: a change to a file that already exists. Every edit shows the exact old text and the exact new text.
- **Fingerprint**: a single SHA-256 value calculated from every file in the repository. If your fingerprint matches the
  expected one, every file is byte-for-byte correct. It is the strongest check in this file. The fingerprint command
  skips `.git/` and any file named `REFACTOR_INSTRUCTIONS.md`, so it does not matter whether you keep this file inside
  the repository root or somewhere else.

## 0.3 How code blocks work

- Text you must type or paste is shown in code blocks. The fence lines (the lines made only of backticks, like
  ```` ```` ```` or ```` ````` ````) are **not** part of the text.
- Copy everything between the fences exactly, including blank lines and the indentation. Some blocks **start or end
  with an empty line**. That empty line is part of the text and must be copied too.
- A "Find this exact text" block is just a unique piece of the file. It often starts or stops in the middle of a function.
  Replace only that piece; the rest of the function stays as it is.
- Every file ends with one newline character after its last line. Most editors add it automatically.
- Go files are indented with TAB characters, not spaces. If your editor turns tabs into spaces, run `gofmt -w .` from the
  repository root. That restores the tabs in every Go file. Two non-Go files also contain tabs: both copies of
  `complete-agents.golden` (see the note in Step 1).
- Code blocks that are marked `diff` are for reference only. They show the change as `diff -u` prints it. Lines starting
  with `-` are removed, lines starting with `+` are added, and lines starting with a space stay the same.
  You do not paste a diff anywhere.

## 0.4 How an edit is written

Every edit looks like this:

1. A heading with the file name: `#### File 2 of 5: internal/agent/agent.go`.
2. A sentence about imports, which tells you which `import` lines this edit adds or removes.
3. One or more numbered edits. Each one has:
   - the line numbers, **as they are in the file at the START of the step**;
   - "Find this exact text": the old text, which appears exactly once in the file;
   - "Replace it with this exact text": the new text.
4. When almost the whole file changes, there is a single edit called "replace the whole file". It shows the complete old
   text, so you can check that you have the right file, and the complete new text.
5. A reference diff.

Edits inside one file are listed **from the bottom of the file to the top**. Do them in the order shown. Each edit only
changes lines below the next one, so the line numbers of the next edit are still correct when you get to it.

## 0.5 Things you must not do

- Do not run any `git` command. This refactor was produced and checked without git, and you do not need it.
- Do not run `make build` or `make install` while following the steps. They create a `bin/` folder, which changes the
  fingerprint. If you did, run `rm -rf bin` before checking the fingerprint.
- Do not run `go mod tidy` or `go get`. No dependency changes in this refactor.
- Do not edit the golden files (`*.golden`) to make a test pass. They are the proof that behaviour did not change.

## 0.6 Decisions made where the request was ambiguous

These are the conservative choices made while producing this refactor:

1. **Order of side effects is preserved exactly.** For example, `caged --no-mount --workdir X` still checks for the Docker
   image (and builds it if it is missing) before it reports that the two flags cannot be combined. Reporting the flag
   error first would be nicer, but it would change behaviour, so it is listed as a follow-up instead.
2. **Error messages are unchanged.** Some old messages start with a capital letter or do not wrap their cause with `%w`.
   They were kept as they are, because the messages are part of the observable behaviour.
3. **Environment-variable names are validated only in the CLI layer.** The old code validated them twice. The second check
   (in `app`) could never fail, so it was removed. The CLI check still runs at the same moment as before.
4. **Cobra's output writers are not redirected.** Calling `SetOut` would also move Cobra's stderr-bound usage text to
   stdout, so the composition root leaves Cobra's defaults alone, exactly as before.
5. **`docs/spec.md` was not changed.** It is a historical requirements document that already described an older layout
   before this refactor.
6. **Agents are data, not an interface.** Every agent has the same shape, so one `Agent` type with methods is simpler and
   fully substitutable. See `docs/architecture.md` ("Key interfaces").

# 1. Prerequisites

## 1.1 Operating system

This refactor was produced and replayed on **Ubuntu 22.04 LTS (Linux 6.8, x86-64)**. Every step also works on macOS
(Apple silicon or Intel); section 6 lists the few differences. Windows is not supported by caged itself.

## 1.2 Go, exactly version 1.23.12

`go.mod` says `go 1.23`, and the development image (`dev/Dockerfile`) uses Go 1.23. Use **go1.23.12**, so that
`go vet` and the test output match this file exactly.

Check your version:

````bash
go version
````

Expected output on Linux (on a Mac it ends in `darwin/arm64` or `darwin/amd64`):

````text
go version go1.23.12 linux/amd64
````

If you do not have that version, install it into your home folder. This does not touch any system Go.

On Linux (x86-64):

````bash
mkdir -p "$HOME/sdk"
cd "$HOME/sdk"
curl -fsSLO https://go.dev/dl/go1.23.12.linux-amd64.tar.gz
echo "d3847fef834e9db11bf64e3fb34db9c04db14e068eeb064f49af747010454f90  go1.23.12.linux-amd64.tar.gz" | sha256sum -c -
tar -xzf go1.23.12.linux-amd64.tar.gz
mv go go1.23.12
````

On a Mac with Apple silicon (for an Intel Mac use `darwin-amd64` and checksum
`0f6efdc3ffc6f03b230016acca0aef43c229de022d0ff401e7aa4ad4862eca8e`):

````bash
mkdir -p "$HOME/sdk"
cd "$HOME/sdk"
curl -fsSLO https://go.dev/dl/go1.23.12.darwin-arm64.tar.gz
echo "5bfa117e401ae64e7ffb960243c448b535fe007e682a13ff6c7371f4a6f0ccaa  go1.23.12.darwin-arm64.tar.gz" | shasum -a 256 -c -
tar -xzf go1.23.12.darwin-arm64.tar.gz
mv go go1.23.12
````

The checksum line must print `OK`. If it prints `FAILED`, delete the download and try again.

## 1.3 Environment for every terminal

Run these three lines in every new terminal before you work on the steps:

````bash
export PATH="$HOME/sdk/go1.23.12/bin:$PATH"
export GOTOOLCHAIN=local
export GOFLAGS=-buildvcs=false
````

- `GOTOOLCHAIN=local` stops Go from downloading a different Go version on its own.
- `GOFLAGS=-buildvcs=false` stops `go build` from calling `git` to stamp version information into binaries.

## 1.4 Other tools

All of these come with Linux and macOS. Nothing else needs to be installed:

- `bash` and `sh`: the shell. The tests use `/bin/sh` for a fake `docker` program.
- `mkdir`, `mv`, `rm`, `cp`, `find`, `sort`, `xargs`, `diff`: file commands.
- `sha256sum` on Linux, or `shasum` on macOS: used for fingerprints.
- `cksum`: used by the fake `docker` in the golden tests.

You do **not** need Docker. Every test replaces Docker with a fake. The first `go build` downloads the Cobra library
(`github.com/spf13/cobra v1.10.2`) once, so you need internet access for that one time.

## 1.5 Confirm a clean baseline

1. Make a fresh copy of the original repository and go into it. Here the copy is called `caged`:

````bash
cp -r /path/to/original/caged ./caged
cd caged
````

2. Build and test it:

````bash
go build ./... && go test -count=1 ./...
````

Expected output. The seconds at the end of each `ok` line will differ, and the lines may come in a different order:

````text
?   	github.com/JeremyOrr4/caged/cmd/caged	[no test files]
ok  	github.com/JeremyOrr4/caged/internal/agent	0.006s
ok  	github.com/JeremyOrr4/caged/internal/app	0.003s
ok  	github.com/JeremyOrr4/caged/internal/cli	0.006s
ok  	github.com/JeremyOrr4/caged/internal/docker	0.004s
ok  	github.com/JeremyOrr4/caged/internal/hostconfig	0.010s
?   	github.com/JeremyOrr4/caged/internal/project	[no test files]
ok  	github.com/JeremyOrr4/caged/internal/plan	0.003s
ok  	github.com/JeremyOrr4/caged/internal/runtime	0.003s
````

(`go test ./...` without `-count=1` prints the same lines, but may show `(cached)` instead of a time.)

3. Check that you start from exactly the right files:

````bash
find . -type f -not -path './.git/*' -not -name REFACTOR_INSTRUCTIONS.md -print0 | LC_ALL=C sort -z | xargs -0 sha256sum | sha256sum
````

On macOS use:

````bash
find . -type f -not -path './.git/*' -not -name REFACTOR_INSTRUCTIONS.md -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256 | shasum -a 256
````

Expected output:

````text
b745d84f849cf751d76ba506dc25b49df290026723905239789c53dc0901d3c9  -
````

If the fingerprint is different, your copy is not the original. It may contain a `bin/` folder or edited files.
Get a clean copy before you start.

# 2. Before and after overview

## 2.1 Directory trees side by side

````text
BEFORE (original)          AFTER (refactored)

.gitignore                 .gitignore
Makefile                   Makefile
README.md                  README.md
cmd/                       cmd/
  caged/                     caged/
    main.go                    architecture_test.go
dev/                           golden_test.go
  Dockerfile                   main.go
docs/                          main_test.go
  architecture.md              testdata/
  implementation.md              golden/
  spec.md                          darwin/  (51 *.golden files)
go.mod                             linux/  (51 *.golden files)
go.sum                     dev/
internal/                    Dockerfile
  agent/                   docs/
    agent.go                 architecture.md
    agent_test.go            implementation.md
  app/                       spec.md
    run.go                 go.mod
    run_test.go            go.sum
  cli/                     internal/
    doc.go                   agent/
    root.go                    agent.go
    root_test.go               agent_test.go
  docker/                      builtin.go
    doc.go                     claude.go
    run.go                     codex.go
    run_test.go                copilot.go
    runtime.go                 definitions_test.go
  hostconfig/                  gemini.go
    resolver.go                gh.go
    resolver_test.go           pi.go
  plan/                        registry.go
    run.go                     registry_test.go
    run_test.go                testdata/
  project/                       builtin_agents.golden
    mount.go                 app/
  runtime/                     planner.go
    Dockerfile                 planner_test.go
    build.go                   service.go
    caged-entrypoint.sh        service_test.go
    image.go                 cli/
    runtime_test.go            doc.go
                               root.go
                               root_test.go
                             docker/
                               args.go
                               args_test.go
                               client.go
                               client_test.go
                               doc.go
                               platform.go
                             hostconfig/
                               resolver.go
                               resolver_edges_test.go
                               resolver_test.go
                               tree.go
                             plan/
                               run.go
                               run_test.go
                             project/
                               mount.go
                               mount_test.go
                             runtime/
                               Dockerfile
                               assets.go
                               caged-entrypoint.sh
                               image.go
                               runtime_test.go
                             system/
                               system.go
                               system_test.go
````

## 2.2 Where every piece went

| Old location | New location |
|---|---|
| `cmd/caged/main.go`: `main()` calls `cli.NewRootCmd(version).Execute()` and maps exit codes | `cmd/caged/main.go`: `main()` plus `run(...)`, the composition root (same exit-code mapping) |
| `internal/agent/agent.go`: global `registry` map with six entries | `internal/agent/claude.go`, `codex.go`, `copilot.go`, `gemini.go`, `gh.go`, `pi.go` (one function each) and `internal/agent/builtin.go` (`Builtin()`, the registration list) |
| `agent.Lookup`, `agent.Names` (package functions over the global map) | `(*agent.Registry).Lookup`, `.Names`, `.All` in `internal/agent/registry.go` |
| `agent.ResolveEnvVars(agent, requested, lookup)` (fell back to `os.LookupEnv`) | method `(agent.Agent).ResolveEnv(requested, isSet)` (no fallback) |
| `agent.EnvVarNames(agent, requested)` | method `(agent.Agent).EnvNames(requested)` |
| `agent.envNameRE` (package-level regexp) | function `agent.validEnvName` |
| prompt-style `switch` inside `plan.BuildRunSpec` | method `(agent.Agent).LaunchCommand(prompt)` |
| `cli.agentCommand` (only used by its test) | removed; its test now tests `LaunchCommand(nil)` |
| `cli.runTarget`, `cli.resolveTarget` | `cli.buildRequest` |
| `cli.NewRootCmd(version)` (built its own service) | `cli.NewRootCmd(version, agents, service)` with interfaces `cli.Agents` and `cli.Service` |
| `app.Runner`, `app.Runtime`, `dockerRunner`, `dockerRuntime`, `app.DefaultService` | interface `app.Engine`; the wiring lives in `cmd/caged/main.go` (`run`) |
| `app.Service.Run` in `internal/app/run.go` (all stages in one function) | `app.Service.Run` in `internal/app/service.go` (image, then plan, then run) and `app.Planner.Plan` in `internal/app/planner.go` |
| `app.ConfigureMounts` (exported, filled a scratch `RunSpec`) | `(*app.Planner).projectMount` and `(*app.Planner).configMounts` |
| `app.logConfigReport`, `app.logEnvResolution` (wrote to `os.Stderr`) | methods of `*app.Planner` that write to the injected stderr |
| `hostconfig.ResolveAllConfigsReport(home, exists)` (106 lines) | `hostconfig.NewResolver(fs, env, agents, shared).Resolve(home)` with small methods |
| `hostconfig.ConfigReport`, alias `hostconfig.ResolvedMount` | `plan.ConfigReport`, `plan.ConfigMount` |
| hard-coded `.gitconfig` block | data: `hostconfig.SharedFile` and `hostconfig.DefaultSharedFiles()` |
| `validateMaskedConfigTree`, `collectSafeMounts`, `subtreeIsSafe`, `appendUnique`, `IsWithinHome` in `resolver.go` | `internal/hostconfig/tree.go`, same logic through an injected `FileSystem`; `IsWithinHome` is now `isWithin` |
| `project.ResolveMount` | `(*project.Resolver).Resolve` (injected `FileSystem`) |
| `docker.Run`, `docker.ImageExists`, `docker.CheckDocker` | `(*docker.Client).Run`, `.ImageExists`, `.Check` in `internal/docker/client.go` |
| `docker.buildArgs(options, goos, uid, gid)` in `run.go` | `docker.runArgs(spec, platform)` in `internal/docker/args.go` |
| `goos == "linux"` (`--user`), the darwin File Sharing hint, the "start Docker Desktop" text, the CPU check | `internal/docker/platform.go` |
| aliases `docker.Options`, `docker.Mount` | removed; code uses `plan.RunSpec` and `plan.Mount` |
| `runtime.BuildRuntimeImage` in `internal/runtime/build.go` (ran `docker build` itself) | `(*docker.Client).Build` and `docker.buildArgs`, fed by `runtime.BuildSpec` in `internal/runtime/assets.go` |
| embedded files as `[]byte` vars | embedded files as `string` vars (immutable) |
| direct `os`, `os/exec`, `runtime.GOOS` calls in 7 packages | `internal/system`: `FS`, `Env`, `Executor`, `Command`, `Platform`, `Stdio` |
| project and app tests inside `internal/cli/root_test.go` | `internal/project/mount_test.go`, `internal/app/planner_test.go` |

## 2.3 Every file, original compared with final

Files **moved** (renamed with `mv`, then edited):

- `internal/app/run.go` -> `internal/app/service.go`
- `internal/app/run_test.go` -> `internal/app/service_test.go`
- `internal/docker/run.go` -> `internal/docker/args.go`
- `internal/docker/run_test.go` -> `internal/docker/args_test.go`
- `internal/docker/runtime.go` -> `internal/docker/client.go`
- `internal/runtime/build.go` -> `internal/runtime/assets.go`

Files **created** (125):

- `cmd/caged/architecture_test.go`
- `cmd/caged/golden_test.go`
- `cmd/caged/main_test.go`
- `internal/agent/builtin.go`
- `internal/agent/claude.go`
- `internal/agent/codex.go`
- `internal/agent/copilot.go`
- `internal/agent/definitions_test.go`
- `internal/agent/gemini.go`
- `internal/agent/gh.go`
- `internal/agent/pi.go`
- `internal/agent/registry.go`
- `internal/agent/registry_test.go`
- `internal/agent/testdata/builtin_agents.golden`
- `internal/app/planner.go`
- `internal/app/planner_test.go`
- `internal/docker/client_test.go`
- `internal/docker/platform.go`
- `internal/hostconfig/resolver_edges_test.go`
- `internal/hostconfig/tree.go`
- `internal/project/mount_test.go`
- `internal/system/system.go`
- `internal/system/system_test.go`
- `cmd/caged/testdata/golden/linux/`: 51 files: `agent-claude.golden`, `agent-codex.golden`, `agent-copilot.golden`, `agent-gemini.golden`, `agent-gh.golden`, `agent-pi.golden`, `agent-uppercase.golden`, `build-daemon-down.golden`, `build-help.golden`, `build-no-cache.golden`, `build-rejects-args.golden`, `build.golden`, `complete-agents.golden`, `complete-prompt.golden`, `config-overrides.golden`, `daemon-down.golden`, `docker-missing.golden`, `env-extra-names.golden`, `env-invalid.golden`, `env-padded-name.golden`, `help-list.golden`, `help-unknown-topic.golden`, `help.golden`, `home-unresolvable.golden`, `image-missing-builds-first.golden`, `list-rejects-args.golden`, `list.golden`, `no-mount-with-workdir.golden`, `prompt-claude.golden`, `prompt-codex.golden`, `prompt-copilot.golden`, `prompt-gemini.golden`, `prompt-gh.golden`, `prompt-pi.golden`, `run-exit-code.golden`, `run-mounts-denied.golden`, `shell-env.golden`, `shell-no-mount-no-config.golden`, `shell.golden`, `too-many-args.golden`, `unknown-flag.golden`, `unsupported-agent.golden`, `verbose-claude.golden`, `version.golden`, `workdir-flag.golden`, `workdir-home-allowed.golden`, `workdir-is-home.golden`, `workdir-missing.golden`, `workdir-relative.golden`, `workdir-root-allowed.golden`, `workdir-root.golden`
- `cmd/caged/testdata/golden/darwin/`: 51 files: `agent-claude.golden`, `agent-codex.golden`, `agent-copilot.golden`, `agent-gemini.golden`, `agent-gh.golden`, `agent-pi.golden`, `agent-uppercase.golden`, `build-daemon-down.golden`, `build-help.golden`, `build-no-cache.golden`, `build-rejects-args.golden`, `build.golden`, `complete-agents.golden`, `complete-prompt.golden`, `config-overrides.golden`, `daemon-down.golden`, `docker-missing.golden`, `env-extra-names.golden`, `env-invalid.golden`, `env-padded-name.golden`, `help-list.golden`, `help-unknown-topic.golden`, `help.golden`, `home-unresolvable.golden`, `image-missing-builds-first.golden`, `list-rejects-args.golden`, `list.golden`, `no-mount-with-workdir.golden`, `prompt-claude.golden`, `prompt-codex.golden`, `prompt-copilot.golden`, `prompt-gemini.golden`, `prompt-gh.golden`, `prompt-pi.golden`, `run-exit-code.golden`, `run-mounts-denied.golden`, `shell-env.golden`, `shell-no-mount-no-config.golden`, `shell.golden`, `too-many-args.golden`, `unknown-flag.golden`, `unsupported-agent.golden`, `verbose-claude.golden`, `version.golden`, `workdir-flag.golden`, `workdir-home-allowed.golden`, `workdir-is-home.golden`, `workdir-missing.golden`, `workdir-relative.golden`, `workdir-root-allowed.golden`, `workdir-root.golden`

Files **modified** (17):

- `README.md`
- `cmd/caged/main.go`
- `docs/architecture.md`
- `docs/implementation.md`
- `internal/agent/agent.go`
- `internal/agent/agent_test.go`
- `internal/cli/doc.go`
- `internal/cli/root.go`
- `internal/cli/root_test.go`
- `internal/docker/doc.go`
- `internal/hostconfig/resolver.go`
- `internal/hostconfig/resolver_test.go`
- `internal/plan/run.go`
- `internal/plan/run_test.go`
- `internal/project/mount.go`
- `internal/runtime/image.go`
- `internal/runtime/runtime_test.go`

Files **deleted**: none. Every file that disappears from its old place was moved, see the list above.

Files **unchanged** (8): `.gitignore`, `Makefile`, `dev/Dockerfile`, `docs/spec.md`, `go.mod`, `go.sum`, `internal/runtime/Dockerfile`, `internal/runtime/caged-entrypoint.sh`.

## 2.4 The steps at a glance

| Step | What it does |
|---|---|
| 1 | Add golden tests that capture today's behaviour |
| 2 | Write the target architecture into docs/architecture.md |
| 3 | Add internal/system, the only package that touches the operating system |
| 4 | Move agent behaviour onto agent.Agent |
| 5 | Replace the global agent map with a Registry and one file per agent |
| 6 | Put project-directory validation behind a FileSystem interface |
| 7 | Turn host-config discovery into hostconfig.Resolver (risky change 2 of 3) |
| 8 | Turn docker functions into docker.Client (risky change 1 of 3, part 1) |
| 9 | Move docker build into docker.Client; make runtime pure data (risky change 1 of 3, part 2) |
| 10 | Split app into Service and Planner with injected dependencies |
| 11 | Clean up the CLI and move all wiring into cmd/caged/main.go |
| 12 | Add an architecture test that enforces the structure |
| 13 | Close the remaining test gap in plan |
| 14 | Bring the docs up to date |

## 2.5 Why: summary of the audit

Before the refactor, the package graph was `cmd/caged -> cli -> app -> {agent, docker, hostconfig, plan, project, runtime}`.
`app` built its own Docker adapters, and `cli` called that hidden wiring from inside each command. The main problems were:

- package-level mutable state (the agent map and a regexp);
- 58 direct `os`, `exec`, `runtime.GOOS` and filesystem calls spread over 7 of the 9 packages;
- `app.Run` doing eight jobs at once;
- a 106-line config resolver;
- two packages that ran Docker;
- type aliases leaking between packages;
- duplicated validation;
- platform checks scattered around, two of which could not be tested on Linux at all.

The three riskiest changes, and how each was checked:

1. **Docker argument building and execution** (Steps 8 and 9). Golden tests compare every docker argument in order. They
   also check exit-code passthrough, the stderr tee, and the daemon-down, docker-missing and missing-image flows. A unit
   test checks that the real `*os.File` stdin still reaches `docker run -it`.
2. **Host-config resolution** (Step 7). The `--verbose` golden output fixes every `config: pass/missing/refused` line in
   order. The fixture home includes a FIFO, masked folders, symlinks in and out of home, overrides, and a home that
   cannot be resolved. The old tests are kept, and new fault-injection tests were added.
3. **macOS-specific paths** (Steps 8 and 11). The platform is injected, and the full golden suite runs for Linux and for
   macOS. A script checked that the 51 macOS recordings differ from the Linux ones only by the three documented macOS
   rules.

# 3. The refactor steps

Each step uses the same template: **Goal**, **Why**, **Files touched**, **Commands to run**, **File edits** (with
imports, exact old and new text, and a reference diff), **Verify** (with the expected output, the number of passing
tests, and the fingerprint), **If it fails**, and **Checkpoint** (the complete file list).

## Step 1: Add golden tests that capture today's behaviour

> **Note for Step 1:** `cmd/caged/testdata/golden/linux/complete-agents.golden` contains four TAB characters, one between each subcommand name and its description (`build`, `completion`, `help`, `list`). Keep them as tabs. All other lines of all golden files contain no tabs.

**Goal:** Record everything caged prints and every docker command it runs, for 51 command lines, before changing any code.

**Why:** These recordings ("golden files") are the proof that the refactor changes nothing a user can see. Every later step must keep them passing without editing them.

### Files touched

- `cmd/caged/golden_test.go`: create this new file.
- `cmd/caged/testdata/golden/linux/agent-claude.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/agent-codex.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/agent-copilot.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/agent-gemini.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/agent-gh.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/agent-pi.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/agent-uppercase.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/build-daemon-down.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/build-help.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/build-no-cache.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/build-rejects-args.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/build.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/complete-agents.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/complete-prompt.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/config-overrides.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/daemon-down.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/docker-missing.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/env-extra-names.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/env-invalid.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/env-padded-name.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/help-list.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/help-unknown-topic.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/help.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/home-unresolvable.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/image-missing-builds-first.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/list-rejects-args.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/list.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/no-mount-with-workdir.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/prompt-claude.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/prompt-codex.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/prompt-copilot.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/prompt-gemini.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/prompt-gh.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/prompt-pi.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/run-exit-code.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/run-mounts-denied.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/shell-env.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/shell-no-mount-no-config.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/shell.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/too-many-args.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/unknown-flag.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/unsupported-agent.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/verbose-claude.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/version.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/workdir-flag.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/workdir-home-allowed.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/workdir-is-home.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/workdir-missing.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/workdir-relative.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/workdir-root-allowed.golden`: create this new file.
- `cmd/caged/testdata/golden/linux/workdir-root.golden`: create this new file.
- `internal/agent/definitions_test.go`: create this new file.
- `internal/agent/testdata/builtin_agents.golden`: create this new file.

### Commands to run

Run these commands from the repository root:

````bash
mkdir -p cmd/caged/testdata/golden/linux
mkdir -p internal/agent/testdata
````

Each command prints nothing when it works.

### File edits

#### File 1 of 54: `cmd/caged/golden_test.go`

This is a NEW FILE (366 lines).

Imports: this new file has these import lines: `"errors"`, `"fmt"`, `"os"`, `"os/exec"`, `"path/filepath"`, `"regexp"`, `"runtime"`, `"strings"`, `"syscall"`, `"testing"`, `"github.com/JeremyOrr4/caged/internal/cli"`. (The file content below shows how they are grouped.)

Create the file `cmd/caged/golden_test.go` with exactly this content:

````go
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/JeremyOrr4/caged/internal/cli"
)

// fakeDocker stands in for the docker CLI. It appends every invocation to
// $FAKE_DOCKER_LOG (one argument per line, then "---") and fakes the replies
// caged depends on. FAKE_DOCKER_* variables select failure modes.
const fakeDocker = `#!/bin/sh
{ printf '%s\n' "$@"; echo '---'; } >> "$FAKE_DOCKER_LOG"
case "$1" in
info)
  if [ -n "$FAKE_DOCKER_DAEMON_DOWN" ]; then
    echo 'Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?' >&2
    exit 1
  fi
  echo '27.0.0'
  ;;
image)
  if [ -n "$FAKE_DOCKER_IMAGE_MISSING" ]; then exit 1; fi
  ;;
build)
  for arg in "$@"; do context="$arg"; done
  {
    echo "build context Dockerfile: $(cksum < "$context/Dockerfile")"
    echo "build context caged-entrypoint.sh: $(cksum < "$context/caged-entrypoint.sh")"
    if [ -x "$context/caged-entrypoint.sh" ]; then echo 'build context entrypoint is executable'; fi
    echo '---'
  } >> "$FAKE_DOCKER_LOG"
  echo 'fake docker build output'
  ;;
run)
  echo 'fake docker run output'
  if [ -n "$FAKE_DOCKER_RUN_STDERR" ]; then echo "$FAKE_DOCKER_RUN_STDERR" >&2; fi
  exit "${FAKE_DOCKER_RUN_EXIT:-0}"
  ;;
esac
exit 0
`

// goldenPrompt contains shell syntax to prove prompts are passed verbatim.
const goldenPrompt = `hello $(world) "quoted" 'single'`

type goldenCase struct {
	name string
	args []string
	// env is applied after the defaults; "$ROOT" is replaced with the scenario root.
	env map[string]string
	// noDocker hides the fake docker binary from PATH.
	noDocker bool
}

func goldenCases() []goldenCase {
	cases := []goldenCase{
		{name: "shell", args: []string{}},
		{name: "shell-no-mount-no-config", args: []string{"--no-mount", "--no-config"}},
		{name: "list", args: []string{"list"}},
		{name: "version", args: []string{"--version"}},
		{name: "help", args: []string{"--help"}},
		{name: "help-list", args: []string{"help", "list"}},
		{name: "help-unknown-topic", args: []string{"help", "nope"}},
		{name: "build-help", args: []string{"build", "--help"}},
		{name: "unknown-flag", args: []string{"--bogus"}},
		{name: "build", args: []string{"build"}},
		{name: "build-no-cache", args: []string{"build", "--no-cache"}},
		{name: "build-rejects-args", args: []string{"build", "extra"}},
		{name: "list-rejects-args", args: []string{"list", "extra"}},
		{name: "complete-agents", args: []string{"__complete", ""}},
		{name: "complete-prompt", args: []string{"__complete", "claude", ""}},
		{name: "agent-uppercase", args: []string{"Claude"}},
		{name: "unsupported-agent", args: []string{"aider"}},
		{name: "too-many-args", args: []string{"claude", "one", "two"}},
		{name: "env-invalid", args: []string{"--env", "bad-name", "claude"}},
		{name: "env-padded-name", args: []string{"--env", " CAGED_GOLDEN_SET ", "--verbose", "gemini"}, env: map[string]string{"CAGED_GOLDEN_SET": "x"}},
		{
			name: "shell-env",
			args: []string{"--env", "CAGED_GOLDEN_SET", "--env", "CAGED_GOLDEN_UNSET", "--verbose", "--no-config"},
			env:  map[string]string{"CAGED_GOLDEN_SET": "x", "ANTHROPIC_API_KEY": "not-forwarded-to-a-plain-shell"},
		},
		{
			name: "env-extra-names",
			args: []string{"--env", "CAGED_GOLDEN_SET", "--env", "CAGED_GOLDEN_UNSET", "--env", "ANTHROPIC_API_KEY", "--verbose", "codex"},
			env:  map[string]string{"CAGED_GOLDEN_SET": "value-is-never-printed", "OPENAI_API_KEY": "secret"},
		},
		{name: "verbose-claude", args: []string{"--verbose", "claude"}, env: map[string]string{"ANTHROPIC_API_KEY": "secret"}},
		{name: "no-mount-with-workdir", args: []string{"--no-mount", "-w", "$ROOT/work/other"}},
		{name: "workdir-flag", args: []string{"-w", "$ROOT/work/other", "pi"}},
		{name: "workdir-relative", args: []string{"--workdir", "../other", "--no-config"}},
		{name: "workdir-missing", args: []string{"-w", "$ROOT/work/missing"}},
		{name: "workdir-is-home", args: []string{"-w", "$ROOT/home"}},
		{name: "workdir-home-allowed", args: []string{"--allow-broad-mount", "-w", "$ROOT/home", "--no-config"}},
		{name: "workdir-root-allowed", args: []string{"--allow-broad-mount", "-w", "/", "--no-config"}},
		{name: "workdir-root", args: []string{"-w", "/"}},
		{
			name: "config-overrides",
			args: []string{"--verbose", "--no-mount"},
			env: map[string]string{
				"CODEX_HOME":        "alt-codex",
				"CLAUDE_CONFIG_DIR": "$ROOT/home",
				"COPILOT_HOME":      "$ROOT/outside/copilot",
			},
		},
		{name: "home-unresolvable", args: []string{"--verbose", "--no-mount"}, env: map[string]string{"HOME": "$ROOT/missing-home"}},
		{name: "image-missing-builds-first", args: []string{"gemini"}, env: map[string]string{"FAKE_DOCKER_IMAGE_MISSING": "1"}},
		{name: "run-exit-code", args: []string{"claude"}, env: map[string]string{"FAKE_DOCKER_RUN_EXIT": "7"}},
		{
			name: "run-mounts-denied",
			args: []string{"codex"},
			env: map[string]string{
				"FAKE_DOCKER_RUN_EXIT":   "125",
				"FAKE_DOCKER_RUN_STDERR": "docker: Error response from daemon: Mounts denied: the path is not shared from the host.",
			},
		},
		{name: "daemon-down", args: []string{"claude"}, env: map[string]string{"FAKE_DOCKER_DAEMON_DOWN": "1"}},
		{name: "build-daemon-down", args: []string{"build"}, env: map[string]string{"FAKE_DOCKER_DAEMON_DOWN": "1"}},
		{name: "docker-missing", args: []string{"claude"}, noDocker: true},
	}
	for _, name := range []string{"claude", "codex", "copilot", "gemini", "gh", "pi"} {
		cases = append(cases,
			goldenCase{name: "agent-" + name, args: []string{name}},
			goldenCase{name: "prompt-" + name, args: []string{name, goldenPrompt}},
		)
	}
	return cases
}

// TestGoldenDockerInvocations runs complete command lines against a fake
// docker binary and compares everything observable (exit code, error, stdout,
// stderr, and every docker invocation) with files in testdata/golden.
func TestGoldenDockerInvocations(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("golden fixtures were captured on linux and caged reads the host platform directly")
	}
	goldenDir, err := filepath.Abs(filepath.Join("testdata", "golden", "linux"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range goldenCases() {
		t.Run(tc.name, func(t *testing.T) {
			got := runGoldenCase(t, tc)
			compareGolden(t, filepath.Join(goldenDir, tc.name+".golden"), got)
		})
	}
}

func runGoldenCase(t *testing.T, tc goldenCase) string {
	root := t.TempDir()
	writeGoldenFixtures(t, root)
	logPath := filepath.Join(root, "docker.log")

	for _, name := range []string{
		"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY",
		"GH_TOKEN", "GITHUB_TOKEN", "COPILOT_GITHUB_TOKEN",
		"CLAUDE_CONFIG_DIR", "CODEX_HOME", "COPILOT_HOME",
		"CAGED_GOLDEN_SET", "CAGED_GOLDEN_UNSET",
		"FAKE_DOCKER_DAEMON_DOWN", "FAKE_DOCKER_IMAGE_MISSING", "FAKE_DOCKER_RUN_EXIT", "FAKE_DOCKER_RUN_STDERR",
	} {
		unsetEnv(t, name)
	}
	path := filepath.Join(root, "bin") + string(os.PathListSeparator) + os.Getenv("PATH")
	if tc.noDocker {
		path = filepath.Join(root, "emptybin")
	}
	t.Setenv("PATH", path)
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("TMPDIR", filepath.Join(root, "tmp"))
	t.Setenv("FAKE_DOCKER_LOG", logPath)
	for name, value := range tc.env {
		t.Setenv(name, strings.ReplaceAll(value, "$ROOT", root))
	}
	chdir(t, filepath.Join(root, "work", "project"))

	args := make([]string, 0, len(tc.args))
	for _, arg := range tc.args {
		args = append(args, strings.ReplaceAll(arg, "$ROOT", root))
	}
	stdout, stderr, code, err := executeOriginal(t, root, args)

	dockerLog, readErr := os.ReadFile(logPath)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		t.Fatal(readErr)
	}
	leftovers, readErr := os.ReadDir(filepath.Join(root, "tmp"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	errText := "<nil>"
	if err != nil {
		errText = err.Error()
	}
	var b strings.Builder
	fmt.Fprintf(&b, "args: %q\n", tc.args)
	fmt.Fprintf(&b, "exit code: %d\n", code)
	fmt.Fprintf(&b, "error: %s\n", errText)
	fmt.Fprintf(&b, "leftover temp entries: %d\n", len(leftovers))
	fmt.Fprintf(&b, "=== stdout\n%s", stdout)
	fmt.Fprintf(&b, "=== stderr\n%s", stderr)
	fmt.Fprintf(&b, "=== docker invocations\n%s", dockerLog)
	return normalizeGolden(t, b.String(), root)
}

// executeOriginal runs the command the way main does and captures the
// process-wide stdout and stderr that caged writes to.
func executeOriginal(t *testing.T, root string, args []string) (string, string, int, error) {
	stdoutFile, err := os.Create(filepath.Join(root, "stdout.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer stdoutFile.Close()
	stderrFile, err := os.Create(filepath.Join(root, "stderr.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer stderrFile.Close()

	oldStdout, oldStderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdoutFile, stderrFile
	cmd := cli.NewRootCmd("test")
	cmd.SetArgs(args)
	runErr := cmd.Execute()
	code := 0
	if runErr != nil {
		fmt.Fprintln(os.Stderr, runErr)
		code = 1
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			code = exitErr.ExitCode()
		}
	}
	os.Stdout, os.Stderr = oldStdout, oldStderr

	stdout, err := os.ReadFile(stdoutFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := os.ReadFile(stderrFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(stdout), string(stderr), code, runErr
}

// writeGoldenFixtures creates a fake home with every supported config, a
// project directory, and the fake docker binary.
func writeGoldenFixtures(t *testing.T, root string) {
	t.Helper()
	for _, dir := range []string{
		"bin", "emptybin", "tmp", "work/project", "work/other", "outside/pi", "outside/copilot",
		"home/.claude", "home/.codex/app-server-control", "home/.codex/packages/app-server-daemon",
		"home/.codex/tmp/arg0", "home/.gemini", "home/.pi/agent", "home/.config/gh", "home/.copilot",
		"home/alt-codex",
	} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for file, contents := range map[string]string{
		"work/project/main.go":           "package main\n",
		"outside/pi/settings.json":       "{}\n",
		"home/.claude/settings.json":     "{}\n",
		"home/.claude/.credentials.json": "{}\n",
		"home/.claude.json":              "{}\n",
		"home/.codex/config.toml":        "model = 'test'\n",
		"home/.codex/auth.json":          "{}\n",
		"home/.gemini/settings.json":     "{}\n",
		"home/.gemini/oauth_creds.json":  "{}\n",
		"home/.pi/agent/settings.json":   "{}\n",
		"home/.config/gh/hosts.yml":      "github.com: {}\n",
		"home/.gitconfig":                "[user]\n\tname = Test\n",
		"home/alt-codex/config.toml":     "model = 'alt'\n",
	} {
		if err := os.WriteFile(filepath.Join(root, file), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "docker"), []byte(fakeDocker), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(root, "home/.codex/app-server-control/daemon.sock"), 0o600); err != nil {
		t.Fatal(err)
	}
	for link, target := range map[string]string{
		"home/.codex/packages/app-server-daemon/current": "/opt/homebrew/bin/codex",
		"home/.gemini/current-settings.json":             "settings.json",
		"home/.config/pi":                                filepath.Join(root, "outside", "pi"),
	} {
		if err := os.Symlink(target, filepath.Join(root, link)); err != nil {
			t.Fatal(err)
		}
	}
}

var buildDirPattern = regexp.MustCompile(`caged-runtime-[0-9]+`)

// normalizeGolden replaces values that change between runs and machines.
func normalizeGolden(t *testing.T, text, root string) string {
	t.Helper()
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	text = strings.ReplaceAll(text, resolvedRoot, "$ROOT")
	text = strings.ReplaceAll(text, root, "$ROOT")
	text = strings.ReplaceAll(text, fmt.Sprintf("\n--user\n%d:%d\n", os.Getuid(), os.Getgid()), "\n--user\n$UID:$GID\n")
	return buildDirPattern.ReplaceAllString(text, "caged-runtime-N")
}

// compareGolden checks got against the golden file. With CAGED_UPDATE_GOLDEN=1
// a missing golden file is created; existing files are never overwritten.
func compareGolden(t *testing.T, path, got string) {
	t.Helper()
	want, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) && os.Getenv("CAGED_UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if err != nil {
		t.Fatalf("read golden file (run with CAGED_UPDATE_GOLDEN=1 to create it): %v", err)
	}
	if got != string(want) {
		t.Errorf("output differs from %s\n--- want\n%s\n--- got\n%s", path, want, got)
	}
}

// unsetEnv removes an environment variable for the duration of the test;
// t.Setenv restores the previous value afterwards.
func unsetEnv(t *testing.T, name string) {
	t.Helper()
	t.Setenv(name, "")
	if err := os.Unsetenv(name); err != nil {
		t.Fatal(err)
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Error(err)
		}
	})
}
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 2 of 54: `cmd/caged/testdata/golden/linux/agent-claude.golden`

This is a NEW FILE (57 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/agent-claude.golden` with exactly this content:

````text
args: ["claude"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
--user
$UID:$GID
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
claude
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 3 of 54: `cmd/caged/testdata/golden/linux/agent-codex.golden`

This is a NEW FILE (57 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/agent-codex.golden` with exactly this content:

````text
args: ["codex"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
--user
$UID:$GID
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
codex
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 4 of 54: `cmd/caged/testdata/golden/linux/agent-copilot.golden`

This is a NEW FILE (57 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/agent-copilot.golden` with exactly this content:

````text
args: ["copilot"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
--user
$UID:$GID
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
copilot
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 5 of 54: `cmd/caged/testdata/golden/linux/agent-gemini.golden`

This is a NEW FILE (57 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/agent-gemini.golden` with exactly this content:

````text
args: ["gemini"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
--user
$UID:$GID
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
gemini
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 6 of 54: `cmd/caged/testdata/golden/linux/agent-gh.golden`

This is a NEW FILE (57 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/agent-gh.golden` with exactly this content:

````text
args: ["gh"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
--user
$UID:$GID
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
gh
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 7 of 54: `cmd/caged/testdata/golden/linux/agent-pi.golden`

This is a NEW FILE (57 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/agent-pi.golden` with exactly this content:

````text
args: ["pi"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
--user
$UID:$GID
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
pi
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 8 of 54: `cmd/caged/testdata/golden/linux/agent-uppercase.golden`

This is a NEW FILE (57 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/agent-uppercase.golden` with exactly this content:

````text
args: ["Claude"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
--user
$UID:$GID
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
claude
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 9 of 54: `cmd/caged/testdata/golden/linux/build-daemon-down.golden`

This is a NEW FILE (12 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/build-daemon-down.golden` with exactly this content:

````text
args: ["build"]
exit code: 1
error: Docker is installed but the daemon is unavailable; start the Docker daemon: Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?
leftover temp entries: 0
=== stdout
=== stderr
Docker is installed but the daemon is unavailable; start the Docker daemon: Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?
=== docker invocations
info
--format
{{.ServerVersion}}
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 10 of 54: `cmd/caged/testdata/golden/linux/build-help.golden`

This is a NEW FILE (15 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/build-help.golden` with exactly this content:

````text
args: ["build" "--help"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
Build the runtime image

Usage:
  caged build [flags]

Flags:
  -h, --help       help for build
      --no-cache   build without using the Docker cache
=== stderr
=== docker invocations
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 11 of 54: `cmd/caged/testdata/golden/linux/build-no-cache.golden`

This is a NEW FILE (25 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/build-no-cache.golden` with exactly this content:

````text
args: ["build" "--no-cache"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker build output
built runtime image caged-runtime:latest
=== stderr
=== docker invocations
info
--format
{{.ServerVersion}}
---
build
-t
caged-runtime:latest
--no-cache
-f
$ROOT/tmp/caged-runtime-N/Dockerfile
$ROOT/tmp/caged-runtime-N
---
build context Dockerfile: 2970434799 5025
build context caged-entrypoint.sh: 848039441 211
build context entrypoint is executable
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 12 of 54: `cmd/caged/testdata/golden/linux/build-rejects-args.golden`

This is a NEW FILE (8 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/build-rejects-args.golden` with exactly this content:

````text
args: ["build" "extra"]
exit code: 1
error: unknown command "extra" for "caged build"
leftover temp entries: 0
=== stdout
=== stderr
unknown command "extra" for "caged build"
=== docker invocations
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 13 of 54: `cmd/caged/testdata/golden/linux/build.golden`

This is a NEW FILE (24 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/build.golden` with exactly this content:

````text
args: ["build"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker build output
built runtime image caged-runtime:latest
=== stderr
=== docker invocations
info
--format
{{.ServerVersion}}
---
build
-t
caged-runtime:latest
-f
$ROOT/tmp/caged-runtime-N/Dockerfile
$ROOT/tmp/caged-runtime-N
---
build context Dockerfile: 2970434799 5025
build context caged-entrypoint.sh: 848039441 211
build context entrypoint is executable
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 14 of 54: `cmd/caged/testdata/golden/linux/complete-agents.golden`

This is a NEW FILE (19 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/complete-agents.golden` with exactly this content:

````text
args: ["__complete" ""]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
build	Build the runtime image
completion	Generate the autocompletion script for the specified shell
help	Help about any command
list	List supported AI agents
claude
codex
copilot
gemini
gh
pi
:4
=== stderr
Completion ended with directive: ShellCompDirectiveNoFileComp
=== docker invocations
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 15 of 54: `cmd/caged/testdata/golden/linux/complete-prompt.golden`

This is a NEW FILE (9 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/complete-prompt.golden` with exactly this content:

````text
args: ["__complete" "claude" ""]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
:4
=== stderr
Completion ended with directive: ShellCompDirectiveNoFileComp
=== docker invocations
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 16 of 54: `cmd/caged/testdata/golden/linux/config-overrides.golden`

This is a NEW FILE (57 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/config-overrides.golden` with exactly this content:

````text
args: ["--verbose" "--no-mount"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
config: pass $ROOT/home/alt-codex -> /home/caged/.codex
config: pass $ROOT/home/.gemini/oauth_creds.json -> /home/caged/.gemini/oauth_creds.json
config: pass $ROOT/home/.gemini/settings.json -> /home/caged/.gemini/settings.json
config: pass $ROOT/home/.config/gh -> /home/caged/.config/gh
config: pass $ROOT/home/.pi -> /home/caged/.pi
config: pass $ROOT/home/.gitconfig -> /home/caged/.gitconfig
config: refused $ROOT/home (config path resolves to the home directory)
config: refused $ROOT/outside/copilot (resolves outside home)
config: refused $ROOT/home/.gemini/current-settings.json (symlinks and special files are excluded from config mounts)
config: refused $ROOT/home/.config/pi (resolves outside home)
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/home/alt-codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
--user
$UID:$GID
caged-runtime:latest
bash
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 17 of 54: `cmd/caged/testdata/golden/linux/daemon-down.golden`

This is a NEW FILE (17 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/daemon-down.golden` with exactly this content:

````text
args: ["claude"]
exit code: 1
error: Docker is installed but the daemon is unavailable; start the Docker daemon: Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?
leftover temp entries: 0
=== stdout
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
Docker is installed but the daemon is unavailable; start the Docker daemon: Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 18 of 54: `cmd/caged/testdata/golden/linux/docker-missing.golden`

This is a NEW FILE (9 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/docker-missing.golden` with exactly this content:

````text
args: ["claude"]
exit code: 1
error: Docker is not installed or not on PATH: exec: "docker": executable file not found in $PATH
leftover temp entries: 0
=== stdout
=== stderr
Runtime image caged-runtime:latest not found; building it first...
Docker is not installed or not on PATH: exec: "docker": executable file not found in $PATH
=== docker invocations
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 19 of 54: `cmd/caged/testdata/golden/linux/env-extra-names.golden`

This is a NEW FILE (77 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/env-extra-names.golden` with exactly this content:

````text
args: ["--env" "CAGED_GOLDEN_SET" "--env" "CAGED_GOLDEN_UNSET" "--env" "ANTHROPIC_API_KEY" "--verbose" "codex"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
config: pass $ROOT/home/.claude -> /home/caged/.claude
config: pass $ROOT/home/.codex -> /home/caged/.codex
config: pass $ROOT/home/.copilot -> /home/caged/.copilot
config: pass $ROOT/home/.gemini/oauth_creds.json -> /home/caged/.gemini/oauth_creds.json
config: pass $ROOT/home/.gemini/settings.json -> /home/caged/.gemini/settings.json
config: pass $ROOT/home/.config/gh -> /home/caged/.config/gh
config: pass $ROOT/home/.pi -> /home/caged/.pi
config: pass $ROOT/home/.gitconfig -> /home/caged/.gitconfig
config: refused $ROOT/home/.gemini/current-settings.json (symlinks and special files are excluded from config mounts)
config: refused $ROOT/home/.config/pi (resolves outside home)
env: pass OPENAI_API_KEY
env: skipped ANTHROPIC_API_KEY (unset or disallowed)
env: skipped GEMINI_API_KEY (unset or disallowed)
env: skipped GOOGLE_API_KEY (unset or disallowed)
env: pass CAGED_GOLDEN_SET
env: skipped CAGED_GOLDEN_UNSET (unset or disallowed)
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
--user
$UID:$GID
-e
OPENAI_API_KEY
-e
CAGED_GOLDEN_SET
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
codex
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 20 of 54: `cmd/caged/testdata/golden/linux/env-invalid.golden`

This is a NEW FILE (8 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/env-invalid.golden` with exactly this content:

````text
args: ["--env" "bad-name" "claude"]
exit code: 1
error: invalid env name "bad-name"; must match ^[A-Za-z_][A-Za-z0-9_]*$
leftover temp entries: 0
=== stdout
=== stderr
invalid env name "bad-name"; must match ^[A-Za-z_][A-Za-z0-9_]*$
=== docker invocations
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 21 of 54: `cmd/caged/testdata/golden/linux/env-padded-name.golden`

This is a NEW FILE (72 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/env-padded-name.golden` with exactly this content:

````text
args: ["--env" " CAGED_GOLDEN_SET " "--verbose" "gemini"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
config: pass $ROOT/home/.claude -> /home/caged/.claude
config: pass $ROOT/home/.codex -> /home/caged/.codex
config: pass $ROOT/home/.copilot -> /home/caged/.copilot
config: pass $ROOT/home/.gemini/oauth_creds.json -> /home/caged/.gemini/oauth_creds.json
config: pass $ROOT/home/.gemini/settings.json -> /home/caged/.gemini/settings.json
config: pass $ROOT/home/.config/gh -> /home/caged/.config/gh
config: pass $ROOT/home/.pi -> /home/caged/.pi
config: pass $ROOT/home/.gitconfig -> /home/caged/.gitconfig
config: refused $ROOT/home/.gemini/current-settings.json (symlinks and special files are excluded from config mounts)
config: refused $ROOT/home/.config/pi (resolves outside home)
env: skipped GEMINI_API_KEY (unset or disallowed)
env: skipped GOOGLE_API_KEY (unset or disallowed)
env: pass CAGED_GOLDEN_SET
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
--user
$UID:$GID
-e
CAGED_GOLDEN_SET
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
gemini
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 22 of 54: `cmd/caged/testdata/golden/linux/help-list.golden`

This is a NEW FILE (14 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/help-list.golden` with exactly this content:

````text
args: ["help" "list"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
List supported AI agents

Usage:
  caged list [flags]

Flags:
  -h, --help   help for list
=== stderr
=== docker invocations
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 23 of 54: `cmd/caged/testdata/golden/linux/help-unknown-topic.golden`

This is a NEW FILE (30 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/help-unknown-topic.golden` with exactly this content:

````text
args: ["help" "nope"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
Run a bash shell or a supported AI coding agent in a disposable Docker container

Usage:
  caged [agent [prompt]] [flags]
  caged [command]

Available Commands:
  build       Build the runtime image
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  list        List supported AI agents

Flags:
      --allow-broad-mount   allow mounting the filesystem root or home directory
      --env stringArray     pass a host environment variable into the container by NAME only
  -h, --help                help for caged
      --no-config           do not mount any host AI config into the container
      --no-mount            do not mount the current working directory
      --verbose             print which config files and environment names are passed
  -v, --version             version for caged
  -w, --workdir string      mount this host directory at /workspace/<folder-name>

Use "caged [command] --help" for more information about a command.
=== stderr
=== docker invocations
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 24 of 54: `cmd/caged/testdata/golden/linux/help.golden`

This is a NEW FILE (30 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/help.golden` with exactly this content:

````text
args: ["--help"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
Run a bash shell or a supported AI coding agent in a disposable Docker container

Usage:
  caged [agent [prompt]] [flags]
  caged [command]

Available Commands:
  build       Build the runtime image
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  list        List supported AI agents

Flags:
      --allow-broad-mount   allow mounting the filesystem root or home directory
      --env stringArray     pass a host environment variable into the container by NAME only
  -h, --help                help for caged
      --no-config           do not mount any host AI config into the container
      --no-mount            do not mount the current working directory
      --verbose             print which config files and environment names are passed
  -v, --version             version for caged
  -w, --workdir string      mount this host directory at /workspace/<folder-name>

Use "caged [command] --help" for more information about a command.
=== stderr
=== docker invocations
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 25 of 54: `cmd/caged/testdata/golden/linux/home-unresolvable.golden`

This is a NEW FILE (26 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/home-unresolvable.golden` with exactly this content:

````text
args: ["--verbose" "--no-mount"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
config: refused $ROOT/missing-home (home cannot be resolved)
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
--user
$UID:$GID
caged-runtime:latest
bash
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 26 of 54: `cmd/caged/testdata/golden/linux/image-missing-builds-first.golden`

This is a NEW FILE (74 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/image-missing-builds-first.golden` with exactly this content:

````text
args: ["gemini"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker build output
fake docker run output
=== stderr
Runtime image caged-runtime:latest not found; building it first...
WARNING: allowlisted agent credentials are available inside the container.
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
build
-t
caged-runtime:latest
-f
$ROOT/tmp/caged-runtime-N/Dockerfile
$ROOT/tmp/caged-runtime-N
---
build context Dockerfile: 2970434799 5025
build context caged-entrypoint.sh: 848039441 211
build context entrypoint is executable
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
--user
$UID:$GID
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
gemini
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 27 of 54: `cmd/caged/testdata/golden/linux/list-rejects-args.golden`

This is a NEW FILE (8 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/list-rejects-args.golden` with exactly this content:

````text
args: ["list" "extra"]
exit code: 1
error: unknown command "extra" for "caged list"
leftover temp entries: 0
=== stdout
=== stderr
unknown command "extra" for "caged list"
=== docker invocations
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 28 of 54: `cmd/caged/testdata/golden/linux/list.golden`

This is a NEW FILE (13 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/list.golden` with exactly this content:

````text
args: ["list"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
claude
codex
copilot
gemini
gh
pi
=== stderr
=== docker invocations
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 29 of 54: `cmd/caged/testdata/golden/linux/no-mount-with-workdir.golden`

This is a NEW FILE (13 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/no-mount-with-workdir.golden` with exactly this content:

````text
args: ["--no-mount" "-w" "$ROOT/work/other"]
exit code: 1
error: --workdir cannot be used with --no-mount
leftover temp entries: 0
=== stdout
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
--workdir cannot be used with --no-mount
=== docker invocations
image
inspect
caged-runtime:latest
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 30 of 54: `cmd/caged/testdata/golden/linux/prompt-claude.golden`

This is a NEW FILE (58 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/prompt-claude.golden` with exactly this content:

````text
args: ["claude" "hello $(world) \"quoted\" 'single'"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
--user
$UID:$GID
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
claude
hello $(world) "quoted" 'single'
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 31 of 54: `cmd/caged/testdata/golden/linux/prompt-codex.golden`

This is a NEW FILE (59 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/prompt-codex.golden` with exactly this content:

````text
args: ["codex" "hello $(world) \"quoted\" 'single'"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
--user
$UID:$GID
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
codex
exec
hello $(world) "quoted" 'single'
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 32 of 54: `cmd/caged/testdata/golden/linux/prompt-copilot.golden`

This is a NEW FILE (59 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/prompt-copilot.golden` with exactly this content:

````text
args: ["copilot" "hello $(world) \"quoted\" 'single'"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
--user
$UID:$GID
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
copilot
-p
hello $(world) "quoted" 'single'
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 33 of 54: `cmd/caged/testdata/golden/linux/prompt-gemini.golden`

This is a NEW FILE (59 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/prompt-gemini.golden` with exactly this content:

````text
args: ["gemini" "hello $(world) \"quoted\" 'single'"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
--user
$UID:$GID
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
gemini
-i
hello $(world) "quoted" 'single'
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 34 of 54: `cmd/caged/testdata/golden/linux/prompt-gh.golden`

This is a NEW FILE (13 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/prompt-gh.golden` with exactly this content:

````text
args: ["gh" "hello $(world) \"quoted\" 'single'"]
exit code: 1
error: gh does not support prompt launch
leftover temp entries: 0
=== stdout
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
gh does not support prompt launch
=== docker invocations
image
inspect
caged-runtime:latest
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 35 of 54: `cmd/caged/testdata/golden/linux/prompt-pi.golden`

This is a NEW FILE (58 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/prompt-pi.golden` with exactly this content:

````text
args: ["pi" "hello $(world) \"quoted\" 'single'"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
--user
$UID:$GID
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
pi
hello $(world) "quoted" 'single'
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 36 of 54: `cmd/caged/testdata/golden/linux/run-exit-code.golden`

This is a NEW FILE (58 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/run-exit-code.golden` with exactly this content:

````text
args: ["claude"]
exit code: 7
error: exit status 7
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
exit status 7
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
--user
$UID:$GID
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
claude
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 37 of 54: `cmd/caged/testdata/golden/linux/run-mounts-denied.golden`

This is a NEW FILE (59 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/run-mounts-denied.golden` with exactly this content:

````text
args: ["codex"]
exit code: 125
error: exit status 125
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
docker: Error response from daemon: Mounts denied: the path is not shared from the host.
exit status 125
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
--user
$UID:$GID
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
codex
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 38 of 54: `cmd/caged/testdata/golden/linux/shell-env.golden`

This is a NEW FILE (32 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/shell-env.golden` with exactly this content:

````text
args: ["--env" "CAGED_GOLDEN_SET" "--env" "CAGED_GOLDEN_UNSET" "--verbose" "--no-config"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
env: pass CAGED_GOLDEN_SET
env: skipped CAGED_GOLDEN_UNSET (unset or disallowed)
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
--user
$UID:$GID
-e
CAGED_GOLDEN_SET
caged-runtime:latest
bash
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 39 of 54: `cmd/caged/testdata/golden/linux/shell-no-mount-no-config.golden`

This is a NEW FILE (24 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/shell-no-mount-no-config.golden` with exactly this content:

````text
args: ["--no-mount" "--no-config"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
--user
$UID:$GID
caged-runtime:latest
bash
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 40 of 54: `cmd/caged/testdata/golden/linux/shell.golden`

This is a NEW FILE (55 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/shell.golden` with exactly this content:

````text
args: []
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
--user
$UID:$GID
caged-runtime:latest
bash
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 41 of 54: `cmd/caged/testdata/golden/linux/too-many-args.golden`

This is a NEW FILE (8 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/too-many-args.golden` with exactly this content:

````text
args: ["claude" "one" "two"]
exit code: 1
error: accepts at most 2 arg(s), received 3
leftover temp entries: 0
=== stdout
=== stderr
accepts at most 2 arg(s), received 3
=== docker invocations
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 42 of 54: `cmd/caged/testdata/golden/linux/unknown-flag.golden`

This is a NEW FILE (8 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/unknown-flag.golden` with exactly this content:

````text
args: ["--bogus"]
exit code: 1
error: unknown flag: --bogus
leftover temp entries: 0
=== stdout
=== stderr
unknown flag: --bogus
=== docker invocations
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 43 of 54: `cmd/caged/testdata/golden/linux/unsupported-agent.golden`

This is a NEW FILE (8 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/unsupported-agent.golden` with exactly this content:

````text
args: ["aider"]
exit code: 1
error: unsupported agent "aider"; supported agents: claude, codex, copilot, gemini, gh, pi
leftover temp entries: 0
=== stdout
=== stderr
unsupported agent "aider"; supported agents: claude, codex, copilot, gemini, gh, pi
=== docker invocations
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 44 of 54: `cmd/caged/testdata/golden/linux/verbose-claude.golden`

This is a NEW FILE (70 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/verbose-claude.golden` with exactly this content:

````text
args: ["--verbose" "claude"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
config: pass $ROOT/home/.claude -> /home/caged/.claude
config: pass $ROOT/home/.codex -> /home/caged/.codex
config: pass $ROOT/home/.copilot -> /home/caged/.copilot
config: pass $ROOT/home/.gemini/oauth_creds.json -> /home/caged/.gemini/oauth_creds.json
config: pass $ROOT/home/.gemini/settings.json -> /home/caged/.gemini/settings.json
config: pass $ROOT/home/.config/gh -> /home/caged/.config/gh
config: pass $ROOT/home/.pi -> /home/caged/.pi
config: pass $ROOT/home/.gitconfig -> /home/caged/.gitconfig
config: refused $ROOT/home/.gemini/current-settings.json (symlinks and special files are excluded from config mounts)
config: refused $ROOT/home/.config/pi (resolves outside home)
env: pass ANTHROPIC_API_KEY
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
--user
$UID:$GID
-e
ANTHROPIC_API_KEY
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
claude
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 45 of 54: `cmd/caged/testdata/golden/linux/version.golden`

This is a NEW FILE (8 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/version.golden` with exactly this content:

````text
args: ["--version"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
caged version test
=== stderr
=== docker invocations
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 46 of 54: `cmd/caged/testdata/golden/linux/workdir-flag.golden`

This is a NEW FILE (57 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/workdir-flag.golden` with exactly this content:

````text
args: ["-w" "$ROOT/work/other" "pi"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/other:/workspace/other
-w
/workspace/other
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
--user
$UID:$GID
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
pi
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 47 of 54: `cmd/caged/testdata/golden/linux/workdir-home-allowed.golden`

This is a NEW FILE (28 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/workdir-home-allowed.golden` with exactly this content:

````text
args: ["--allow-broad-mount" "-w" "$ROOT/home" "--no-config"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/home:/workspace/home
-w
/workspace/home
--user
$UID:$GID
caged-runtime:latest
bash
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 48 of 54: `cmd/caged/testdata/golden/linux/workdir-is-home.golden`

This is a NEW FILE (13 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/workdir-is-home.golden` with exactly this content:

````text
args: ["-w" "$ROOT/home"]
exit code: 1
error: refusing to mount your entire home directory; use --allow-broad-mount to override
leftover temp entries: 0
=== stdout
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
refusing to mount your entire home directory; use --allow-broad-mount to override
=== docker invocations
image
inspect
caged-runtime:latest
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 49 of 54: `cmd/caged/testdata/golden/linux/workdir-missing.golden`

This is a NEW FILE (13 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/workdir-missing.golden` with exactly this content:

````text
args: ["-w" "$ROOT/work/missing"]
exit code: 1
error: inspect working directory "$ROOT/work/missing": stat $ROOT/work/missing: no such file or directory
leftover temp entries: 0
=== stdout
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
inspect working directory "$ROOT/work/missing": stat $ROOT/work/missing: no such file or directory
=== docker invocations
image
inspect
caged-runtime:latest
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 50 of 54: `cmd/caged/testdata/golden/linux/workdir-relative.golden`

This is a NEW FILE (28 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/workdir-relative.golden` with exactly this content:

````text
args: ["--workdir" "../other" "--no-config"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/other:/workspace/other
-w
/workspace/other
--user
$UID:$GID
caged-runtime:latest
bash
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 51 of 54: `cmd/caged/testdata/golden/linux/workdir-root-allowed.golden`

This is a NEW FILE (28 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/workdir-root-allowed.golden` with exactly this content:

````text
args: ["--allow-broad-mount" "-w" "/" "--no-config"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
/:/workspace/project
-w
/workspace/project
--user
$UID:$GID
caged-runtime:latest
bash
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 52 of 54: `cmd/caged/testdata/golden/linux/workdir-root.golden`

This is a NEW FILE (13 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/linux/workdir-root.golden` with exactly this content:

````text
args: ["-w" "/"]
exit code: 1
error: refusing to mount the filesystem root; use --allow-broad-mount to override
leftover temp entries: 0
=== stdout
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
refusing to mount the filesystem root; use --allow-broad-mount to override
=== docker invocations
image
inspect
caged-runtime:latest
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 53 of 54: `internal/agent/definitions_test.go`

This is a NEW FILE (43 lines).

Imports: this new file has these import lines: `"errors"`, `"fmt"`, `"os"`, `"path/filepath"`, `"strings"`, `"testing"`. (The file content below shows how they are grouped.)

Create the file `internal/agent/definitions_test.go` with exactly this content:

````go
package agent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBuiltinAgentDefinitionsAreUnchanged pins every field of the built-in
// agents so that moving the definitions around cannot silently change them.
func TestBuiltinAgentDefinitionsAreUnchanged(t *testing.T) {
	t.Parallel()

	var b strings.Builder
	for _, name := range []string{"claude", "codex", "copilot", "gemini", "gh", "pi"} {
		definition, err := Lookup(name)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&b, "%#v\n", definition)
	}
	path := filepath.Join("testdata", "builtin_agents.golden")
	want, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) && os.Getenv("CAGED_UPDATE_GOLDEN") == "1" {
		// Create a missing golden file; existing files are never overwritten.
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if err != nil {
		t.Fatalf("read golden file (run with CAGED_UPDATE_GOLDEN=1 to create it): %v", err)
	}
	if got := b.String(); got != string(want) {
		t.Errorf("built-in agent definitions changed\n--- want\n%s\n--- got\n%s", want, got)
	}
}
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 54 of 54: `internal/agent/testdata/builtin_agents.golden`

This is a NEW FILE (6 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `internal/agent/testdata/builtin_agents.golden` with exactly this content:

````text
agent.Agent{Name:"claude", Command:[]string{"claude"}, Description:"Claude Code", ConfigPaths:[]agent.ConfigPath{agent.ConfigPath{HostPath:".claude", ContainerPath:".claude", Writable:true, EnvOverride:"CLAUDE_CONFIG_DIR", MaskPaths:[]string(nil)}}, Prompt:1, PromptFlag:"", Auth:agent.AuthStrategy{Environment:[]string{"ANTHROPIC_API_KEY"}, CredentialPaths:[]string{".claude/.credentials.json"}, SupportsDeviceLogin:false}}
agent.Agent{Name:"codex", Command:[]string{"codex"}, Description:"Codex CLI", ConfigPaths:[]agent.ConfigPath{agent.ConfigPath{HostPath:".codex", ContainerPath:".codex", Writable:true, EnvOverride:"CODEX_HOME", MaskPaths:[]string{"app-server-control", "app-server-daemon", "ipc", "packages/app-server-daemon", "tmp/arg0"}}}, Prompt:2, PromptFlag:"exec", Auth:agent.AuthStrategy{Environment:[]string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY"}, CredentialPaths:[]string{".codex/auth.json"}, SupportsDeviceLogin:true}}
agent.Agent{Name:"copilot", Command:[]string{"copilot"}, Description:"GitHub Copilot CLI", ConfigPaths:[]agent.ConfigPath{agent.ConfigPath{HostPath:".copilot", ContainerPath:".copilot", Writable:true, EnvOverride:"COPILOT_HOME", MaskPaths:[]string(nil)}}, Prompt:2, PromptFlag:"-p", Auth:agent.AuthStrategy{Environment:[]string{"COPILOT_GITHUB_TOKEN", "GH_TOKEN", "GITHUB_TOKEN"}, CredentialPaths:[]string{".copilot/"}, SupportsDeviceLogin:true}}
agent.Agent{Name:"gemini", Command:[]string{"gemini"}, Description:"Gemini CLI", ConfigPaths:[]agent.ConfigPath{agent.ConfigPath{HostPath:".gemini", ContainerPath:".gemini", Writable:true, EnvOverride:"", MaskPaths:[]string(nil)}}, Prompt:2, PromptFlag:"-i", Auth:agent.AuthStrategy{Environment:[]string{"GEMINI_API_KEY", "GOOGLE_API_KEY"}, CredentialPaths:[]string{".gemini/oauth_creds.json"}, SupportsDeviceLogin:false}}
agent.Agent{Name:"gh", Command:[]string{"gh"}, Description:"GitHub CLI", ConfigPaths:[]agent.ConfigPath{agent.ConfigPath{HostPath:".config/gh", ContainerPath:".config/gh", Writable:true, EnvOverride:"", MaskPaths:[]string(nil)}}, Prompt:0, PromptFlag:"", Auth:agent.AuthStrategy{Environment:[]string{"GH_TOKEN", "GITHUB_TOKEN"}, CredentialPaths:[]string{".config/gh/hosts.yml"}, SupportsDeviceLogin:false}}
agent.Agent{Name:"pi", Command:[]string{"pi"}, Description:"Pi coding agent", ConfigPaths:[]agent.ConfigPath{agent.ConfigPath{HostPath:".config/pi", ContainerPath:".config/pi", Writable:true, EnvOverride:"", MaskPaths:[]string(nil)}, agent.ConfigPath{HostPath:".pi", ContainerPath:".pi", Writable:true, EnvOverride:"", MaskPaths:[]string(nil)}}, Prompt:1, PromptFlag:"", Auth:agent.AuthStrategy{Environment:[]string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY"}, CredentialPaths:[]string(nil), SupportsDeviceLogin:false}}
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

### Verify

Run these verification commands from the repository root:

````bash
gofmt -l .
go build ./...
go vet ./...
go test -count=1 ./...
````

Expected result:

- `gofmt -l .` prints nothing.
- `go build ./...` prints nothing.
- `go vet ./...` prints nothing.
- `go test -count=1 ./...` prints the lines below. The numbers of seconds at the end of the `ok` lines will be different on your machine, and lines may come in a different order. Every package must say `ok` or `[no test files]`:

````text
?   	github.com/JeremyOrr4/caged/internal/project	[no test files]
ok  	github.com/JeremyOrr4/caged/cmd/caged	0.491s
ok  	github.com/JeremyOrr4/caged/internal/agent	0.004s
ok  	github.com/JeremyOrr4/caged/internal/app	0.005s
ok  	github.com/JeremyOrr4/caged/internal/cli	0.007s
ok  	github.com/JeremyOrr4/caged/internal/docker	0.007s
ok  	github.com/JeremyOrr4/caged/internal/hostconfig	0.008s
ok  	github.com/JeremyOrr4/caged/internal/plan	0.004s
ok  	github.com/JeremyOrr4/caged/internal/runtime	0.003s
````

Count the passing tests and subtests with `go test -count=1 -v ./... 2>&1 | grep -c -- "--- PASS"`. On Linux the expected number is **118** (on macOS the Linux golden test is skipped, so the count there is lower by 52 (51 golden subtests plus their parent test)).

Run this command to compute the fingerprint of the whole repository (on macOS use the second command):

````bash
find . -type f -not -path './.git/*' -not -name REFACTOR_INSTRUCTIONS.md -print0 | LC_ALL=C sort -z | xargs -0 sha256sum | sha256sum
````

````bash
find . -type f -not -path './.git/*' -not -name REFACTOR_INSTRUCTIONS.md -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256 | shasum -a 256
````

Expected output (exactly this line):

````text
f5a3be7e50fdef9723f422871e1d901db58d8d3ecd64016942a72150af40b65a  -
````

**If it fails:**

- Problem: `TestGoldenDockerInvocations` fails with `read golden file ... no such file or directory`.
  Fix: You missed one of the 51 `.golden` files, or saved it in the wrong folder. Every file must be in `cmd/caged/testdata/golden/linux/`.
- Problem: `TestGoldenDockerInvocations` reports `output differs` for `complete-agents.golden`.
  Fix: That file contains TAB characters between a command name and its description (for example `build<TAB>Build the runtime image`). Your editor turned them into spaces. Re-create the file and keep the tabs.
- Problem: On macOS the golden test prints `SKIP`.
  Fix: That is expected in this step: the original code reads the real operating system, so the Linux recordings can only be checked on Linux. Step 11 makes them run everywhere.
- Problem: the fingerprint is different but every test passes.
  Fix: some file differs from these instructions by a character, often a tab turned into spaces or a missing final newline. Run `gofmt -w .` (it fixes Go indentation), then check the non-Go files of this step again.

### Checkpoint

At this point, the repo should have these 85 files (not counting `.git/` and this instructions file):

- `./` (5 files): `.gitignore`, `Makefile`, `README.md`, `go.mod`, `go.sum`
- `cmd/caged/` (2 files): `golden_test.go`, `main.go`
- `cmd/caged/testdata/golden/linux/` (51 files): `agent-claude.golden`, `agent-codex.golden`, `agent-copilot.golden`, `agent-gemini.golden`, `agent-gh.golden`, `agent-pi.golden`, `agent-uppercase.golden`, `build-daemon-down.golden`, `build-help.golden`, `build-no-cache.golden`, `build-rejects-args.golden`, `build.golden`, `complete-agents.golden`, `complete-prompt.golden`, `config-overrides.golden`, `daemon-down.golden`, `docker-missing.golden`, `env-extra-names.golden`, `env-invalid.golden`, `env-padded-name.golden`, `help-list.golden`, `help-unknown-topic.golden`, `help.golden`, `home-unresolvable.golden`, `image-missing-builds-first.golden`, `list-rejects-args.golden`, `list.golden`, `no-mount-with-workdir.golden`, `prompt-claude.golden`, `prompt-codex.golden`, `prompt-copilot.golden`, `prompt-gemini.golden`, `prompt-gh.golden`, `prompt-pi.golden`, `run-exit-code.golden`, `run-mounts-denied.golden`, `shell-env.golden`, `shell-no-mount-no-config.golden`, `shell.golden`, `too-many-args.golden`, `unknown-flag.golden`, `unsupported-agent.golden`, `verbose-claude.golden`, `version.golden`, `workdir-flag.golden`, `workdir-home-allowed.golden`, `workdir-is-home.golden`, `workdir-missing.golden`, `workdir-relative.golden`, `workdir-root-allowed.golden`, `workdir-root.golden`
- `dev/` (1 files): `Dockerfile`
- `docs/` (3 files): `architecture.md`, `implementation.md`, `spec.md`
- `internal/agent/` (3 files): `agent.go`, `agent_test.go`, `definitions_test.go`
- `internal/agent/testdata/` (1 files): `builtin_agents.golden`
- `internal/app/` (2 files): `run.go`, `run_test.go`
- `internal/cli/` (3 files): `doc.go`, `root.go`, `root_test.go`
- `internal/docker/` (4 files): `doc.go`, `run.go`, `run_test.go`, `runtime.go`
- `internal/hostconfig/` (2 files): `resolver.go`, `resolver_test.go`
- `internal/plan/` (2 files): `run.go`, `run_test.go`
- `internal/project/` (1 files): `mount.go`
- `internal/runtime/` (5 files): `Dockerfile`, `build.go`, `caged-entrypoint.sh`, `image.go`, `runtime_test.go`


## Step 2: Write the target architecture into docs/architecture.md

**Goal:** Replace the first part of `docs/architecture.md` with the design that the next steps build.

**Why:** The plan is written down before any code moves, so every later step has a clear target.

### Files touched

- `docs/architecture.md`: modify.

### Commands to run

No shell commands are needed in this step. You only edit and create files.

### File edits

#### File 1 of 1: `docs/architecture.md`

Imports: this is not a Go file, so there are no imports.

**Edit 1 of 1: replace the whole file.** At the start of this step the file has 62 lines (lines 1-62).

The file currently contains exactly this text:

````markdown
# caged architecture

## Launch pipeline

```text
Cobra CLI -> application service -> agent registry + config/project resolvers
          -> transport-neutral RunSpec -> Docker runner -> runtime image
```

`internal/cli` parses arguments and flags. `internal/app.Service` coordinates
the request through narrow `Runner` and `Runtime` interfaces, allowing its
behavior to be tested without a Docker daemon. `internal/agent` contains the
registry as data: command, prompt style, config paths, environment names, and
authentication metadata. `internal/hostconfig` validates and resolves host
config mounts. `internal/project` validates the project mount. `internal/plan`
defines the neutral `RunSpec`, and `internal/docker` translates it into Docker
arguments. `internal/runtime` embeds and builds the runtime image.

Existing agent configs mount read-write so settings and login changes persist.
The shared `.gitconfig` mount is read-only. Missing config paths are skipped;
paths and symlinks resolving outside the home directory, and trees containing
special files, are refused. `--no-config` disables these mounts.

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

`internal/runtime` embeds the runtime Dockerfile and entrypoint. The image
installs Claude Code, Pi, Codex, Gemini CLI, GitHub CLI (`gh`), and GitHub Copilot CLI. It is built
on first use when absent. `caged build --no-cache` refreshes the latest
installer versions. The development-only Dockerfile is `dev/Dockerfile`.
````

Replace the entire contents of the file with exactly this text:

````markdown
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

```text
cmd/caged ──> cli ──> app ──> plan ──> agent
   │           │       │
   │           │       └────> runtime ──> plan
   │           └────────────> agent, runtime
   ├──> hostconfig ──> agent, plan
   ├──> project ──────> plan
   ├──> docker ───────> plan, system
   └──> system (stdlib only)
```

Arrows point from policy to the things policy needs. `app` (the use cases)
imports no adapter package: it does not know about Docker, the filesystem, or
the environment. Those reach it through interfaces that `app` itself declares.
`agent` and `system` import only the standard library. There are no import
cycles. `cmd/caged/architecture_test.go` enforces these rules: it fails if a
package imports something outside its allowed list, or if any package other
than `internal/system` and `cmd/caged` imports `os`, `os/exec`, `syscall`, or
the standard `runtime` package.

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
Tests call `run` with a fake `docker` on `PATH`, buffers for stdio, and an
injected platform. This lets the full Linux and macOS behaviour be checked
on any machine.

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
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/docs/architecture.md
+++ b/docs/architecture.md
@@ -3,24 +3,148 @@
 ## Launch pipeline
 
 ```text
-Cobra CLI -> application service -> agent registry + config/project resolvers
-          -> transport-neutral RunSpec -> Docker runner -> runtime image
+caged codex 'fix tests'
+   |
+ [1 parse]    internal/cli      argv + flags -> app.Request (agent looked up, env names validated)
+   |
+ [2 prepare]  internal/app      Service.Run: build the runtime image first if it is missing
+   |
+ [3 plan]     internal/app      Planner.Plan: project mount + config mounts + env names
+   |                            -> plan.RunSpec (pure data, no Docker knowledge)
+   |
+ [4 execute]  internal/docker   Client.Run: RunSpec -> `docker run -it --rm ...`
 ```
 
-`internal/cli` parses arguments and flags. `internal/app.Service` coordinates
-the request through narrow `Runner` and `Runtime` interfaces, allowing its
-behavior to be tested without a Docker daemon. `internal/agent` contains the
-registry as data: command, prompt style, config paths, environment names, and
-authentication metadata. `internal/hostconfig` validates and resolves host
-config mounts. `internal/project` validates the project mount. `internal/plan`
-defines the neutral `RunSpec`, and `internal/docker` translates it into Docker
-arguments. `internal/runtime` embeds and builds the runtime image.
+Every stage talks to the next one through a small interface or a plain data
+type, so each stage can be unit-tested with fakes and none of them needs a
+Docker daemon. Only `internal/system` touches the operating system.
 
 Existing agent configs mount read-write so settings and login changes persist.
 The shared `.gitconfig` mount is read-only. Missing config paths are skipped;
 paths and symlinks resolving outside the home directory, and trees containing
 special files, are refused. `--no-config` disables these mounts.
 
+## Package layout
+
+```text
+cmd/caged/            composition root: the only place that picks concrete types
+internal/cli/         Cobra commands and flags -> app.Request; nothing else
+internal/app/         use cases: Service (Run, Build) and Planner (Request -> RunSpec)
+internal/plan/        neutral data: RunSpec, Mount, ConfigMount, ConfigReport, BuildSpec
+internal/agent/       Agent definition, Registry, one file per built-in agent
+internal/project/     project directory validation -> plan.Mount
+internal/hostconfig/  host config discovery and safety policy -> plan.ConfigReport
+internal/docker/      Docker CLI adapter: RunSpec/BuildSpec -> docker commands
+internal/runtime/     the runtime image: tag + embedded Dockerfile and entrypoint
+internal/system/      thin OS adapters: FS, Env, Executor, Platform, Stdio
+```
+
+## Dependency direction
+
+```text
+cmd/caged ──> cli ──> app ──> plan ──> agent
+   │           │       │
+   │           │       └────> runtime ──> plan
+   │           └────────────> agent, runtime
+   ├──> hostconfig ──> agent, plan
+   ├──> project ──────> plan
+   ├──> docker ───────> plan, system
+   └──> system (stdlib only)
+```
+
+Arrows point from policy to the things policy needs. `app` (the use cases)
+imports no adapter package: it does not know about Docker, the filesystem, or
+the environment. Those reach it through interfaces that `app` itself declares.
+`agent` and `system` import only the standard library. There are no import
+cycles. `cmd/caged/architecture_test.go` enforces these rules: it fails if a
+package imports something outside its allowed list, or if any package other
+than `internal/system` and `cmd/caged` imports `os`, `os/exec`, `syscall`, or
+the standard `runtime` package.
+
+## Key interfaces
+
+Interfaces are declared by the package that consumes them and contain only
+the methods that consumer calls. The production implementation is listed on
+the right.
+
+| Consumer | Interface | Methods | Implemented by |
+|---|---|---|---|
+| `cli` | `Service` | `Run(ctx, app.Request) error`, `Build(ctx, noCache bool) error` | `*app.Service` |
+| `cli` | `Agents` | `Lookup(name) (agent.Agent, error)`, `Names() []string` | `*agent.Registry` |
+| `app` | `Engine` | `ImageExists(ctx, image) bool`, `Build(ctx, plan.BuildSpec) error`, `Run(ctx, plan.RunSpec) error` | `*docker.Client` |
+| `app` | `Env` | `LookupEnv(key) (string, bool)`, `UserHomeDir() (string, error)`, `Getwd() (string, error)` | `system.Env` |
+| `app` | `ProjectResolver` | `Resolve(workDir, home string, allowBroadMount bool) (*plan.Mount, error)` | `*project.Resolver` |
+| `app` | `ConfigResolver` | `Resolve(home string) plan.ConfigReport` | `*hostconfig.Resolver` |
+| `hostconfig` | `FileSystem` | `Stat`, `Lstat`, `ReadDir`, `EvalSymlinks`, `WalkDir` | `system.FS` |
+| `hostconfig` | `Env` | `LookupEnv(key) (string, bool)` | `system.Env` |
+| `project` | `FileSystem` | `Abs`, `Stat`, `EvalSymlinks` | `system.FS` |
+| `docker` | `Executor` | `LookPath(file) (string, error)`, `Run(ctx, system.Command) error` | `system.Executor` |
+| `docker` | `FileSystem` | `MkdirTemp`, `WriteFile`, `RemoveAll` | `system.FS` |
+
+Agents are data, not an interface. Every agent has the same shape (command,
+prompt style, config paths, environment names), and behaviour that depends on
+that data is a method on `agent.Agent` (`LaunchCommand`, `EnvNames`,
+`ResolveEnv`). A Go interface with one type per agent would add six types and
+no flexibility. Because there is only one type, every agent is
+substitutable and no generic code checks an agent's name.
+
+## Composition root
+
+`cmd/caged/main.go` is the composition root. `main` collects the real
+process inputs (`system.CurrentStdio()`, `system.CurrentPlatform()`,
+`agent.Builtin()`) and passes them to `run`. `run` builds every component,
+wires them together, executes the command line, and returns the exit code.
+Tests call `run` with a fake `docker` on `PATH`, buffers for stdio, and an
+injected platform. This lets the full Linux and macOS behaviour be checked
+on any machine.
+
+## Platform-specific behaviour
+
+The host platform (`system.Platform{OS, Arch, UID, GID}`) is a value that is
+injected into `docker.Client`. Nothing reads `runtime.GOOS` directly.
+`internal/docker/platform.go` holds all of the OS-specific policy:
+
+- Linux runs the container as the host UID:GID (`--user`). macOS keeps the
+  image user, because Docker Desktop translates bind-mount ownership.
+- On macOS, a failed run whose stderr says a path is not shared gets the
+  Docker Desktop "File Sharing" hint.
+- The daemon-down message says "start Docker Desktop" on macOS and "start
+  the Docker daemon" elsewhere.
+- Only amd64 and arm64 hosts are supported.
+
+Both branches are unit-tested by injecting `OS: "linux"` or `OS: "darwin"`.
+The golden tests run every command line for both platforms. Paths are always
+built with `path/filepath` on the host side and with forward slashes on the
+container side. macOS `/tmp` -> `/private/tmp` style symlinks are resolved
+with `EvalSymlinks` before any comparison.
+
+## Adding a new agent
+
+Before this refactor:
+
+1. Add an entry to the global `registry` map literal in `internal/agent/agent.go`.
+2. Edit the two hard-coded agent lists in `internal/agent/agent_test.go`.
+3. If the agent needed a new prompt style, edit the `switch` in `internal/plan/run.go`.
+
+After this refactor:
+
+1. Create `internal/agent/<name>.go` containing one function that returns the
+   `Agent` value.
+2. Add one line, `<name>(),`, to the list in `internal/agent/builtin.go`.
+
+Nothing else changes. `TestBuiltinRegistryContract` and
+`TestAgentNamesDoNotCollideWithSubcommands` check the new entry automatically.
+The config resolver, planner, CLI completion, and `caged list` pick it up from
+the registry.
+
+## Adding a container runtime or a host-config source
+
+- A new container runtime (for example Podman) is a new package with a type
+  that implements `app.Engine`, plus one changed line in `cmd/caged/main.go`.
+- A new agent config directory is one more `ConfigPath` in that agent's file.
+- A new shared file (like `.gitconfig`) is one more entry in
+  `hostconfig.DefaultSharedFiles()`.
+
 ## Project and prompt behavior
 
 The selected host directory (the current directory by default) is mounted at
@@ -56,7 +180,9 @@
 
 ## Runtime image
 
-`internal/runtime` embeds the runtime Dockerfile and entrypoint. The image
-installs Claude Code, Pi, Codex, Gemini CLI, GitHub CLI (`gh`), and GitHub Copilot CLI. It is built
-on first use when absent. `caged build --no-cache` refreshes the latest
-installer versions. The development-only Dockerfile is `dev/Dockerfile`.
+`internal/runtime` embeds the runtime Dockerfile and entrypoint and describes
+them as a `plan.BuildSpec`; `internal/docker` writes them to a temporary build
+context and runs `docker build`. The image installs Claude Code, Pi, Codex,
+Gemini CLI, GitHub CLI (`gh`), and GitHub Copilot CLI. It is built on first use
+when absent. `caged build --no-cache` refreshes the latest installer versions.
+The development-only Dockerfile is `dev/Dockerfile`.
````

### Verify

Run these verification commands from the repository root:

````bash
gofmt -l .
go build ./...
go vet ./...
go test -count=1 ./...
````

Expected result:

- `gofmt -l .` prints nothing.
- `go build ./...` prints nothing.
- `go vet ./...` prints nothing.
- `go test -count=1 ./...` prints the lines below. The numbers of seconds at the end of the `ok` lines will be different on your machine, and lines may come in a different order. Every package must say `ok` or `[no test files]`:

````text
?   	github.com/JeremyOrr4/caged/internal/project	[no test files]
ok  	github.com/JeremyOrr4/caged/cmd/caged	0.486s
ok  	github.com/JeremyOrr4/caged/internal/agent	0.007s
ok  	github.com/JeremyOrr4/caged/internal/app	0.004s
ok  	github.com/JeremyOrr4/caged/internal/cli	0.006s
ok  	github.com/JeremyOrr4/caged/internal/docker	0.005s
ok  	github.com/JeremyOrr4/caged/internal/hostconfig	0.009s
ok  	github.com/JeremyOrr4/caged/internal/plan	0.003s
ok  	github.com/JeremyOrr4/caged/internal/runtime	0.002s
````

Count the passing tests and subtests with `go test -count=1 -v ./... 2>&1 | grep -c -- "--- PASS"`. On Linux the expected number is **118** (on macOS the Linux golden test is skipped, so the count there is lower by 52 (51 golden subtests plus their parent test)).

Run this command to compute the fingerprint of the whole repository (on macOS use the second command):

````bash
find . -type f -not -path './.git/*' -not -name REFACTOR_INSTRUCTIONS.md -print0 | LC_ALL=C sort -z | xargs -0 sha256sum | sha256sum
````

````bash
find . -type f -not -path './.git/*' -not -name REFACTOR_INSTRUCTIONS.md -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256 | shasum -a 256
````

Expected output (exactly this line):

````text
e13a944a4be7d28a087ed02f7168086c258e2bc92bcf52f7f3475143eb5b6ce6  -
````

**If it fails:**

- Problem: Nothing to build fails in this step, because only a Markdown file changes.
  Fix: If the fingerprint does not match, compare your `docs/architecture.md` with the new text character by character. Look closely at blank lines and at the `|` characters in the tables.
- Problem: the fingerprint is different but every test passes.
  Fix: some file differs from these instructions by a character, often a tab turned into spaces or a missing final newline. Run `gofmt -w .` (it fixes Go indentation), then check the non-Go files of this step again.

### Checkpoint

At this point, the repo should have these 85 files (not counting `.git/` and this instructions file):

- `./` (5 files): `.gitignore`, `Makefile`, `README.md`, `go.mod`, `go.sum`
- `cmd/caged/` (2 files): `golden_test.go`, `main.go`
- `cmd/caged/testdata/golden/linux/` (51 files): `agent-claude.golden`, `agent-codex.golden`, `agent-copilot.golden`, `agent-gemini.golden`, `agent-gh.golden`, `agent-pi.golden`, `agent-uppercase.golden`, `build-daemon-down.golden`, `build-help.golden`, `build-no-cache.golden`, `build-rejects-args.golden`, `build.golden`, `complete-agents.golden`, `complete-prompt.golden`, `config-overrides.golden`, `daemon-down.golden`, `docker-missing.golden`, `env-extra-names.golden`, `env-invalid.golden`, `env-padded-name.golden`, `help-list.golden`, `help-unknown-topic.golden`, `help.golden`, `home-unresolvable.golden`, `image-missing-builds-first.golden`, `list-rejects-args.golden`, `list.golden`, `no-mount-with-workdir.golden`, `prompt-claude.golden`, `prompt-codex.golden`, `prompt-copilot.golden`, `prompt-gemini.golden`, `prompt-gh.golden`, `prompt-pi.golden`, `run-exit-code.golden`, `run-mounts-denied.golden`, `shell-env.golden`, `shell-no-mount-no-config.golden`, `shell.golden`, `too-many-args.golden`, `unknown-flag.golden`, `unsupported-agent.golden`, `verbose-claude.golden`, `version.golden`, `workdir-flag.golden`, `workdir-home-allowed.golden`, `workdir-is-home.golden`, `workdir-missing.golden`, `workdir-relative.golden`, `workdir-root-allowed.golden`, `workdir-root.golden`
- `dev/` (1 files): `Dockerfile`
- `docs/` (3 files): `architecture.md`, `implementation.md`, `spec.md`
- `internal/agent/` (3 files): `agent.go`, `agent_test.go`, `definitions_test.go`
- `internal/agent/testdata/` (1 files): `builtin_agents.golden`
- `internal/app/` (2 files): `run.go`, `run_test.go`
- `internal/cli/` (3 files): `doc.go`, `root.go`, `root_test.go`
- `internal/docker/` (4 files): `doc.go`, `run.go`, `run_test.go`, `runtime.go`
- `internal/hostconfig/` (2 files): `resolver.go`, `resolver_test.go`
- `internal/plan/` (2 files): `run.go`, `run_test.go`
- `internal/project/` (1 files): `mount.go`
- `internal/runtime/` (5 files): `Dockerfile`, `build.go`, `caged-entrypoint.sh`, `image.go`, `runtime_test.go`


## Step 3: Add internal/system, the only package that touches the operating system

**Goal:** Create small types (`FS`, `Env`, `Executor`, `Platform`, `Stdio`) that wrap the real file system, environment, processes, and platform.

**Why:** Later steps hand these to the business logic through interfaces. Then that logic never calls `os` or `exec` directly, and tests can swap in fakes.

### Files touched

- `internal/system/system.go`: create this new file.
- `internal/system/system_test.go`: create this new file.

### Commands to run

Run these commands from the repository root:

````bash
mkdir -p internal/system
````

Each command prints nothing when it works.

### File edits

#### File 1 of 2: `internal/system/system.go`

This is a NEW FILE (87 lines).

Imports: this new file has these import lines: `"context"`, `"io"`, `"io/fs"`, `"os"`, `"os/exec"`, `"path/filepath"`, `"runtime"`. (The file content below shows how they are grouped.)

Create the file `internal/system/system.go` with exactly this content:

````go
// Package system holds the thin adapters between caged and the host operating
// system. It is the only internal package that imports os, os/exec, or the
// standard runtime package. Other packages declare small interfaces that these
// types satisfy, so they can be tested with fakes.
package system

import (
	"context"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// FS is the real filesystem.
type FS struct{}

func (FS) Abs(path string) (string, error)              { return filepath.Abs(path) }
func (FS) Stat(name string) (fs.FileInfo, error)        { return os.Stat(name) }
func (FS) Lstat(name string) (fs.FileInfo, error)       { return os.Lstat(name) }
func (FS) ReadDir(name string) ([]fs.DirEntry, error)   { return os.ReadDir(name) }
func (FS) EvalSymlinks(path string) (string, error)     { return filepath.EvalSymlinks(path) }
func (FS) WalkDir(root string, fn fs.WalkDirFunc) error { return filepath.WalkDir(root, fn) }
func (FS) MkdirTemp(dir, pattern string) (string, error) {
	return os.MkdirTemp(dir, pattern)
}
func (FS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(name, data, perm)
}
func (FS) RemoveAll(path string) error { return os.RemoveAll(path) }

// Env is the real process environment.
type Env struct{}

func (Env) LookupEnv(key string) (string, bool) { return os.LookupEnv(key) }
func (Env) UserHomeDir() (string, error)        { return os.UserHomeDir() }
func (Env) Getwd() (string, error)              { return os.Getwd() }

// Command describes one process to run. Nil streams are connected to the null device.
type Command struct {
	Path   string
	Args   []string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Executor runs real processes.
type Executor struct{}

func (Executor) LookPath(file string) (string, error) { return exec.LookPath(file) }

// Run starts the command and waits for it. A non-zero exit status is returned
// as an *exec.ExitError so callers can pass the exit code through.
func (Executor) Run(ctx context.Context, c Command) error {
	cmd := exec.CommandContext(ctx, c.Path, c.Args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = c.Stdin, c.Stdout, c.Stderr
	return cmd.Run()
}

// Platform describes the host operating system, CPU architecture, and user.
type Platform struct {
	OS   string // a GOOS value such as "linux" or "darwin"
	Arch string // a GOARCH value such as "amd64" or "arm64"
	UID  int
	GID  int
}

// CurrentPlatform reports the platform of the running process.
func CurrentPlatform() Platform {
	return Platform{OS: runtime.GOOS, Arch: runtime.GOARCH, UID: os.Getuid(), GID: os.Getgid()}
}

// Stdio holds the standard streams of the process.
type Stdio struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
}

// CurrentStdio returns the real standard streams. They stay *os.File values so
// child processes inherit the terminal directly, which `docker run -it` needs.
func CurrentStdio() Stdio {
	return Stdio{In: os.Stdin, Out: os.Stdout, Err: os.Stderr}
}
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 2 of 2: `internal/system/system_test.go`

This is a NEW FILE (137 lines).

Imports: this new file has these import lines: `"bytes"`, `"context"`, `"errors"`, `"io/fs"`, `"os"`, `"os/exec"`, `"path/filepath"`, `"runtime"`, `"testing"`. (The file content below shows how they are grouped.)

Create the file `internal/system/system_test.go` with exactly this content:

````go
package system

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFSUsesTheRealFilesystem(t *testing.T) {
	t.Parallel()

	var fsys FS
	root, err := fsys.MkdirTemp(t.TempDir(), "system-test-")
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "file.txt")
	if err := fsys.WriteFile(file, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(file, link); err != nil {
		t.Fatal(err)
	}
	if info, err := fsys.Stat(link); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("Stat(link) = %v, %v; want the regular target file", info, err)
	}
	if info, err := fsys.Lstat(link); err != nil || info.Mode()&fs.ModeSymlink == 0 {
		t.Fatalf("Lstat(link) = %v, %v; want the symlink itself", info, err)
	}
	if entries, err := fsys.ReadDir(root); err != nil || len(entries) != 2 {
		t.Fatalf("ReadDir() = %v, %v; want two entries", entries, err)
	}
	resolvedFile, err := filepath.EvalSymlinks(file)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := fsys.EvalSymlinks(link); err != nil || got != resolvedFile {
		t.Fatalf("EvalSymlinks(link) = %q, %v; want %q", got, err, resolvedFile)
	}
	if got, err := fsys.Abs(file); err != nil || got != file {
		t.Fatalf("Abs(%q) = %q, %v", file, got, err)
	}
	visited := 0
	if err := fsys.WalkDir(root, func(string, fs.DirEntry, error) error { visited++; return nil }); err != nil || visited != 3 {
		t.Fatalf("WalkDir() visited %d entries, err %v; want 3", visited, err)
	}
	if err := fsys.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("RemoveAll() left %q behind: %v", root, err)
	}
}

func TestEnvReadsTheProcessEnvironment(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CAGED_SYSTEM_TEST", "value")

	var env Env
	if got, ok := env.LookupEnv("CAGED_SYSTEM_TEST"); !ok || got != "value" {
		t.Fatalf("LookupEnv() = %q, %v", got, ok)
	}
	if runtime.GOOS != "windows" {
		if got, err := env.UserHomeDir(); err != nil || got != home {
			t.Fatalf("UserHomeDir() = %q, %v; want %q", got, err, home)
		}
	}
	want, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if got, err := env.Getwd(); err != nil || got != want {
		t.Fatalf("Getwd() = %q, %v; want %q", got, err, want)
	}
}

func TestExecutorRunsProcessesAndReportsExitCodes(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("needs a POSIX shell")
	}

	var executor Executor
	sh, err := executor.LookPath("sh")
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	err = executor.Run(context.Background(), Command{
		Path:   sh,
		Args:   []string{"-c", "read line; echo \"out:$line\"; echo err >&2; exit 3"},
		Stdin:  bytes.NewBufferString("input\n"),
		Stdout: &stdout,
		Stderr: &stderr,
	})
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 3 {
		t.Fatalf("Run() error = %v, want exit status 3", err)
	}
	if stdout.String() != "out:input\n" || stderr.String() != "err\n" {
		t.Fatalf("stdout = %q, stderr = %q", stdout.String(), stderr.String())
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := executor.Run(ctx, Command{Path: sh, Args: []string{"-c", "exit 0"}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() with a cancelled context = %v, want context.Canceled", err)
	}
}

func TestCurrentPlatformAndStdioDescribeThisProcess(t *testing.T) {
	t.Parallel()

	want := Platform{OS: runtime.GOOS, Arch: runtime.GOARCH, UID: os.Getuid(), GID: os.Getgid()}
	if got := CurrentPlatform(); got != want {
		t.Fatalf("CurrentPlatform() = %+v, want %+v", got, want)
	}
	stdio := CurrentStdio()
	// docker run -it needs the real terminal file, not a copy through a pipe.
	if stdin, ok := stdio.In.(*os.File); !ok || stdin != os.Stdin {
		t.Fatal("CurrentStdio().In is not os.Stdin")
	}
	if stdout, ok := stdio.Out.(*os.File); !ok || stdout != os.Stdout {
		t.Fatal("CurrentStdio().Out is not os.Stdout")
	}
	if stderr, ok := stdio.Err.(*os.File); !ok || stderr != os.Stderr {
		t.Fatal("CurrentStdio().Err is not os.Stderr")
	}
}
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

### Verify

Run these verification commands from the repository root:

````bash
gofmt -l .
go build ./...
go vet ./...
go test -count=1 ./...
````

Expected result:

- `gofmt -l .` prints nothing.
- `go build ./...` prints nothing.
- `go vet ./...` prints nothing.
- `go test -count=1 ./...` prints the lines below. The numbers of seconds at the end of the `ok` lines will be different on your machine, and lines may come in a different order. Every package must say `ok` or `[no test files]`:

````text
?   	github.com/JeremyOrr4/caged/internal/project	[no test files]
ok  	github.com/JeremyOrr4/caged/cmd/caged	0.490s
ok  	github.com/JeremyOrr4/caged/internal/agent	0.003s
ok  	github.com/JeremyOrr4/caged/internal/app	0.003s
ok  	github.com/JeremyOrr4/caged/internal/cli	0.006s
ok  	github.com/JeremyOrr4/caged/internal/docker	0.009s
ok  	github.com/JeremyOrr4/caged/internal/hostconfig	0.008s
ok  	github.com/JeremyOrr4/caged/internal/plan	0.003s
ok  	github.com/JeremyOrr4/caged/internal/runtime	0.004s
ok  	github.com/JeremyOrr4/caged/internal/system	0.005s
````

Count the passing tests and subtests with `go test -count=1 -v ./... 2>&1 | grep -c -- "--- PASS"`. On Linux the expected number is **122** (on macOS the Linux golden test is skipped, so the count there is lower by 52 (51 golden subtests plus their parent test)).

Run this command to compute the fingerprint of the whole repository (on macOS use the second command):

````bash
find . -type f -not -path './.git/*' -not -name REFACTOR_INSTRUCTIONS.md -print0 | LC_ALL=C sort -z | xargs -0 sha256sum | sha256sum
````

````bash
find . -type f -not -path './.git/*' -not -name REFACTOR_INSTRUCTIONS.md -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256 | shasum -a 256
````

Expected output (exactly this line):

````text
d31848457b0c047481e905e29c51649474758db9476b02bed86fc01627c64063  -
````

**If it fails:**

- Problem: `go vet` reports `declared and not used` or a missing import in `internal/system`.
  Fix: Compare the `import (...)` block at the top of `internal/system/system.go` with the text below. It must list exactly `context`, `io`, `io/fs`, `os`, `os/exec`, `path/filepath`, and `runtime`.
- Problem: `TestExecutorRunsProcessesAndReportsExitCodes` fails with `exec: "sh": executable file not found`.
  Fix: The test needs a POSIX shell called `sh` on your `PATH`. Every Linux and macOS system has `/bin/sh`. Check that your `PATH` contains `/bin`.
- Problem: the fingerprint is different but every test passes.
  Fix: some file differs from these instructions by a character, often a tab turned into spaces or a missing final newline. Run `gofmt -w .` (it fixes Go indentation), then check the non-Go files of this step again.

### Checkpoint

At this point, the repo should have these 87 files (not counting `.git/` and this instructions file):

- `./` (5 files): `.gitignore`, `Makefile`, `README.md`, `go.mod`, `go.sum`
- `cmd/caged/` (2 files): `golden_test.go`, `main.go`
- `cmd/caged/testdata/golden/linux/` (51 files): `agent-claude.golden`, `agent-codex.golden`, `agent-copilot.golden`, `agent-gemini.golden`, `agent-gh.golden`, `agent-pi.golden`, `agent-uppercase.golden`, `build-daemon-down.golden`, `build-help.golden`, `build-no-cache.golden`, `build-rejects-args.golden`, `build.golden`, `complete-agents.golden`, `complete-prompt.golden`, `config-overrides.golden`, `daemon-down.golden`, `docker-missing.golden`, `env-extra-names.golden`, `env-invalid.golden`, `env-padded-name.golden`, `help-list.golden`, `help-unknown-topic.golden`, `help.golden`, `home-unresolvable.golden`, `image-missing-builds-first.golden`, `list-rejects-args.golden`, `list.golden`, `no-mount-with-workdir.golden`, `prompt-claude.golden`, `prompt-codex.golden`, `prompt-copilot.golden`, `prompt-gemini.golden`, `prompt-gh.golden`, `prompt-pi.golden`, `run-exit-code.golden`, `run-mounts-denied.golden`, `shell-env.golden`, `shell-no-mount-no-config.golden`, `shell.golden`, `too-many-args.golden`, `unknown-flag.golden`, `unsupported-agent.golden`, `verbose-claude.golden`, `version.golden`, `workdir-flag.golden`, `workdir-home-allowed.golden`, `workdir-is-home.golden`, `workdir-missing.golden`, `workdir-relative.golden`, `workdir-root-allowed.golden`, `workdir-root.golden`
- `dev/` (1 files): `Dockerfile`
- `docs/` (3 files): `architecture.md`, `implementation.md`, `spec.md`
- `internal/agent/` (3 files): `agent.go`, `agent_test.go`, `definitions_test.go`
- `internal/agent/testdata/` (1 files): `builtin_agents.golden`
- `internal/app/` (2 files): `run.go`, `run_test.go`
- `internal/cli/` (3 files): `doc.go`, `root.go`, `root_test.go`
- `internal/docker/` (4 files): `doc.go`, `run.go`, `run_test.go`, `runtime.go`
- `internal/hostconfig/` (2 files): `resolver.go`, `resolver_test.go`
- `internal/plan/` (2 files): `run.go`, `run_test.go`
- `internal/project/` (1 files): `mount.go`
- `internal/runtime/` (5 files): `Dockerfile`, `build.go`, `caged-entrypoint.sh`, `image.go`, `runtime_test.go`
- `internal/system/` (2 files): `system.go`, `system_test.go`


## Step 4: Move agent behaviour onto agent.Agent

**Goal:** Give `agent.Agent` the methods `LaunchCommand`, `EnvNames` and `ResolveEnv`, and delete the prompt-style `switch` from `plan`.

**Why:** How an agent takes a prompt is a fact about the agent, so it belongs next to the agent's data. This step also removes a hidden `os.LookupEnv` call and a package-level regular expression.

### Files touched

- `internal/agent/agent.go`: modify.
- `internal/agent/agent_test.go`: modify.
- `internal/app/run.go`: modify.
- `internal/cli/root.go`: modify.
- `internal/cli/root_test.go`: modify.
- `internal/plan/run.go`: modify.

### Commands to run

No shell commands are needed in this step. You only edit and create files.

### File edits

#### File 1 of 6: `internal/agent/agent.go`

Imports: removed `"os"`, `"regexp"`. (The edits below already contain these import changes.)

There are 6 edits in this file. They are listed from the bottom of the file to the top. Do them in the order shown: each edit only changes lines below the next one, so the line numbers stay correct. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 6: lines 141-152.**

Find this exact text (it appears exactly once in the file):

````go
	return allowed
}

func EnvVarNames(agent Agent, requested []string) []string {
	seen := make(map[string]struct{}, len(agent.Auth.Environment)+len(requested))
	names := make([]string, 0, len(agent.Auth.Environment)+len(requested))
	for _, name := range append(append([]string{}, agent.Auth.Environment...), requested...) {
		name = strings.TrimSpace(name)
		if name == "" || !envNameRE.MatchString(name) {
			continue
		}
		if _, exists := seen[name]; exists {
````

Replace it with this exact text:

````go
	return allowed
}

// EnvNames returns the agent's default environment names followed by the
// requested ones, trimmed, de-duplicated, and with invalid names dropped.
func (a Agent) EnvNames(requested []string) []string {
	seen := make(map[string]struct{}, len(a.Auth.Environment)+len(requested))
	names := make([]string, 0, len(a.Auth.Environment)+len(requested))
	for _, name := range append(append([]string{}, a.Auth.Environment...), requested...) {
		name = strings.TrimSpace(name)
		if !validEnvName(name) {
			continue
		}
		if _, exists := seen[name]; exists {
````

**Edit 2 of 6: lines 114-137.**

Find this exact text (it appears exactly once in the file):

````go
	},
}

func ValidateEnvName(name string) error {
	trimmed := strings.TrimSpace(name)
	if !envNameRE.MatchString(trimmed) {
		return fmt.Errorf("invalid env name %q; must match ^[A-Za-z_][A-Za-z0-9_]*$", name)
	}
	return nil
}

func ResolveEnvVars(agent Agent, requested []string, lookup func(string) bool) []string {
	if lookup == nil {
		lookup = func(name string) bool {
			_, ok := os.LookupEnv(name)
			return ok
		}
	}
	allowed := make([]string, 0, len(agent.Auth.Environment)+len(requested))
	for _, name := range EnvVarNames(agent, requested) {
		if lookup(name) {
			allowed = append(allowed, name)
		}
	}
````

Replace it with this exact text:

````go
	},
}

// LaunchCommand returns the container command that starts the agent. A
// non-nil prompt is passed as exactly one argument in the agent's prompt style.
func (a Agent) LaunchCommand(prompt *string) ([]string, error) {
	command := append([]string(nil), a.Command...)
	if prompt == nil {
		return command, nil
	}
	switch a.Prompt {
	case PromptPositional:
		return append(command, *prompt), nil
	case PromptFlag:
		if a.PromptFlag == "" {
			return nil, fmt.Errorf("%s prompt flag is not configured", a.Name)
		}
		return append(command, a.PromptFlag, *prompt), nil
	default:
		return nil, fmt.Errorf("%s does not support prompt launch", a.Name)
	}
}

// ValidateEnvName rejects names that are not valid environment variable names.
func ValidateEnvName(name string) error {
	if !validEnvName(strings.TrimSpace(name)) {
		return fmt.Errorf("invalid env name %q; must match ^[A-Za-z_][A-Za-z0-9_]*$", name)
	}
	return nil
}

// validEnvName reports whether name matches ^[A-Za-z_][A-Za-z0-9_]*$.
func validEnvName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case r == '_', 'A' <= r && r <= 'Z', 'a' <= r && r <= 'z':
		case i > 0 && '0' <= r && r <= '9':
		default:
			return false
		}
	}
	return true
}

// ResolveEnv returns the names from EnvNames that isSet reports as present in
// the host environment, or nil when there are none.
func (a Agent) ResolveEnv(requested []string, isSet func(string) bool) []string {
	allowed := make([]string, 0, len(a.Auth.Environment)+len(requested))
	for _, name := range a.EnvNames(requested) {
		if isSet(name) {
			allowed = append(allowed, name)
		}
	}
````

**Edit 3 of 6: lines 43-50.**

Find this exact text (it appears exactly once in the file):

````go
	SupportsDeviceLogin bool
}

var envNameRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

var registry = map[string]Agent{
	"pi": {
		Name:        "pi",
````

Replace it with this exact text:

````go
	SupportsDeviceLogin bool
}

var registry = map[string]Agent{
	"pi": {
		Name:        "pi",
````

**Edit 4 of 6: lines 24-29.**

Find this exact text (it appears exactly once in the file):

````go
	PromptFlag
)

type Agent struct {
	Name        string
	Command     []string
````

Replace it with this exact text:

````go
	PromptFlag
)

// Agent is the complete, data-only definition of a supported agent CLI.
type Agent struct {
	Name        string
	Command     []string
````

**Edit 5 of 6: lines 16-21.**

Find this exact text (it appears exactly once in the file):

````go
	MaskPaths     []string
}

type PromptStyle int

const (
````

Replace it with this exact text:

````go
	MaskPaths     []string
}

// PromptStyle says how an agent accepts an initial prompt on its command line.
type PromptStyle int

const (
````

**Edit 6 of 6: lines 2-13.**

Find this exact text (it appears exactly once in the file):

````go

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

type ConfigPath struct {
	HostPath      string
	ContainerPath string
````

Replace it with this exact text:

````go

import (
	"fmt"
	"sort"
	"strings"
)

// ConfigPath is a host config path, relative to the home directory, that is
// mounted at the same relative path under the container home.
type ConfigPath struct {
	HostPath      string
	ContainerPath string
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/agent/agent.go
+++ b/internal/agent/agent.go
@@ -2,12 +2,12 @@
 
 import (
 	"fmt"
-	"os"
-	"regexp"
 	"sort"
 	"strings"
 )
 
+// ConfigPath is a host config path, relative to the home directory, that is
+// mounted at the same relative path under the container home.
 type ConfigPath struct {
 	HostPath      string
 	ContainerPath string
@@ -16,6 +16,7 @@
 	MaskPaths     []string
 }
 
+// PromptStyle says how an agent accepts an initial prompt on its command line.
 type PromptStyle int
 
 const (
@@ -24,6 +25,7 @@
 	PromptFlag
 )
 
+// Agent is the complete, data-only definition of a supported agent CLI.
 type Agent struct {
 	Name        string
 	Command     []string
@@ -43,8 +45,6 @@
 	SupportsDeviceLogin bool
 }
 
-var envNameRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
-
 var registry = map[string]Agent{
 	"pi": {
 		Name:        "pi",
@@ -114,24 +114,56 @@
 	},
 }
 
+// LaunchCommand returns the container command that starts the agent. A
+// non-nil prompt is passed as exactly one argument in the agent's prompt style.
+func (a Agent) LaunchCommand(prompt *string) ([]string, error) {
+	command := append([]string(nil), a.Command...)
+	if prompt == nil {
+		return command, nil
+	}
+	switch a.Prompt {
+	case PromptPositional:
+		return append(command, *prompt), nil
+	case PromptFlag:
+		if a.PromptFlag == "" {
+			return nil, fmt.Errorf("%s prompt flag is not configured", a.Name)
+		}
+		return append(command, a.PromptFlag, *prompt), nil
+	default:
+		return nil, fmt.Errorf("%s does not support prompt launch", a.Name)
+	}
+}
+
+// ValidateEnvName rejects names that are not valid environment variable names.
 func ValidateEnvName(name string) error {
-	trimmed := strings.TrimSpace(name)
-	if !envNameRE.MatchString(trimmed) {
+	if !validEnvName(strings.TrimSpace(name)) {
 		return fmt.Errorf("invalid env name %q; must match ^[A-Za-z_][A-Za-z0-9_]*$", name)
 	}
 	return nil
 }
 
-func ResolveEnvVars(agent Agent, requested []string, lookup func(string) bool) []string {
-	if lookup == nil {
-		lookup = func(name string) bool {
-			_, ok := os.LookupEnv(name)
-			return ok
+// validEnvName reports whether name matches ^[A-Za-z_][A-Za-z0-9_]*$.
+func validEnvName(name string) bool {
+	if name == "" {
+		return false
+	}
+	for i, r := range name {
+		switch {
+		case r == '_', 'A' <= r && r <= 'Z', 'a' <= r && r <= 'z':
+		case i > 0 && '0' <= r && r <= '9':
+		default:
+			return false
 		}
 	}
-	allowed := make([]string, 0, len(agent.Auth.Environment)+len(requested))
-	for _, name := range EnvVarNames(agent, requested) {
-		if lookup(name) {
+	return true
+}
+
+// ResolveEnv returns the names from EnvNames that isSet reports as present in
+// the host environment, or nil when there are none.
+func (a Agent) ResolveEnv(requested []string, isSet func(string) bool) []string {
+	allowed := make([]string, 0, len(a.Auth.Environment)+len(requested))
+	for _, name := range a.EnvNames(requested) {
+		if isSet(name) {
 			allowed = append(allowed, name)
 		}
 	}
@@ -141,12 +173,14 @@
 	return allowed
 }
 
-func EnvVarNames(agent Agent, requested []string) []string {
-	seen := make(map[string]struct{}, len(agent.Auth.Environment)+len(requested))
-	names := make([]string, 0, len(agent.Auth.Environment)+len(requested))
-	for _, name := range append(append([]string{}, agent.Auth.Environment...), requested...) {
+// EnvNames returns the agent's default environment names followed by the
+// requested ones, trimmed, de-duplicated, and with invalid names dropped.
+func (a Agent) EnvNames(requested []string) []string {
+	seen := make(map[string]struct{}, len(a.Auth.Environment)+len(requested))
+	names := make([]string, 0, len(a.Auth.Environment)+len(requested))
+	for _, name := range append(append([]string{}, a.Auth.Environment...), requested...) {
 		name = strings.TrimSpace(name)
-		if name == "" || !envNameRE.MatchString(name) {
+		if !validEnvName(name) {
 			continue
 		}
 		if _, exists := seen[name]; exists {
````

#### File 2 of 6: `internal/agent/agent_test.go`

Imports: added `"regexp"`. (The edits below already contain these import changes.)

There are 2 edits in this file. They are listed from the bottom of the file to the top. Do them in the order shown: each edit only changes lines below the next one, so the line numbers stay correct. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 2: lines 141-150.**

Find this exact text (it appears exactly once in the file):

````go
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			lookup := func(name string) bool { return tt.lookup[name] }
			got := ResolveEnvVars(tt.agent, tt.requested, lookup)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ResolveEnvVars() = %v, want %v", got, tt.want)
			}
		})
	}
}
````

Replace it with this exact text:

````go
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			lookup := func(name string) bool { return tt.lookup[name] }
			got := tt.agent.ResolveEnv(tt.requested, lookup)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ResolveEnv() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLaunchCommandWithoutPromptUsesRegistryCommand(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"codex", "claude"} {
		definition, err := Lookup(name)
		if err != nil {
			t.Fatal(err)
		}
		got, err := definition.LaunchCommand(nil)
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{name}; !reflect.DeepEqual(got, want) {
			t.Fatalf("LaunchCommand(nil) = %v, want %v", got, want)
		}
		got[0] = "changed"
		if definition.Command[0] != name {
			t.Fatal("LaunchCommand() returned the registry's own slice")
		}
	}
}

func TestLaunchCommandPromptStyles(t *testing.T) {
	t.Parallel()

	prompt := "fix $(this) verbatim"
	tests := []struct {
		name    string
		agent   Agent
		want    []string
		wantErr string
	}{
		{name: "positional", agent: Agent{Name: "a", Command: []string{"a"}, Prompt: PromptPositional}, want: []string{"a", prompt}},
		{name: "flag", agent: Agent{Name: "b", Command: []string{"b", "run"}, Prompt: PromptFlag, PromptFlag: "-p"}, want: []string{"b", "run", "-p", prompt}},
		{name: "flag missing", agent: Agent{Name: "c", Command: []string{"c"}, Prompt: PromptFlag}, wantErr: "c prompt flag is not configured"},
		{name: "unsupported", agent: Agent{Name: "d", Command: []string{"d"}}, wantErr: "d does not support prompt launch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := tt.agent.LaunchCommand(&prompt)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("LaunchCommand() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("LaunchCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidEnvNameMatchesDocumentedPattern(t *testing.T) {
	t.Parallel()

	pattern := regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	for _, name := range []string{
		"", "A", "_", "a1", "1a", "ANTHROPIC_API_KEY", "bad-name", "has space", "trailing\n",
		"é", "A\x00", "Z9_z", "9", "__", "a.b", "\xff",
	} {
		if got, want := validEnvName(name), pattern.MatchString(name); got != want {
			t.Errorf("validEnvName(%q) = %v, want %v", name, got, want)
		}
	}
	if err := ValidateEnvName("  PADDED  "); err != nil {
		t.Errorf("ValidateEnvName() rejected a name with surrounding spaces: %v", err)
	}
	if err := ValidateEnvName("bad-name"); err == nil || err.Error() != `invalid env name "bad-name"; must match ^[A-Za-z_][A-Za-z0-9_]*$` {
		t.Errorf("ValidateEnvName(bad-name) error = %v", err)
	}
}
````

**Edit 2 of 2: lines 2-7.**

Find this exact text (it appears exactly once in the file):

````go

import (
	"reflect"
	"strings"
	"testing"
)
````

Replace it with this exact text:

````go

import (
	"reflect"
	"regexp"
	"strings"
	"testing"
)
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/agent/agent_test.go
+++ b/internal/agent/agent_test.go
@@ -2,6 +2,7 @@
 
 import (
 	"reflect"
+	"regexp"
 	"strings"
 	"testing"
 )
@@ -141,10 +142,87 @@
 		t.Run(tt.name, func(t *testing.T) {
 			t.Parallel()
 			lookup := func(name string) bool { return tt.lookup[name] }
-			got := ResolveEnvVars(tt.agent, tt.requested, lookup)
+			got := tt.agent.ResolveEnv(tt.requested, lookup)
 			if !reflect.DeepEqual(got, tt.want) {
-				t.Fatalf("ResolveEnvVars() = %v, want %v", got, tt.want)
+				t.Fatalf("ResolveEnv() = %v, want %v", got, tt.want)
 			}
 		})
 	}
 }
+
+func TestLaunchCommandWithoutPromptUsesRegistryCommand(t *testing.T) {
+	t.Parallel()
+
+	for _, name := range []string{"codex", "claude"} {
+		definition, err := Lookup(name)
+		if err != nil {
+			t.Fatal(err)
+		}
+		got, err := definition.LaunchCommand(nil)
+		if err != nil {
+			t.Fatal(err)
+		}
+		if want := []string{name}; !reflect.DeepEqual(got, want) {
+			t.Fatalf("LaunchCommand(nil) = %v, want %v", got, want)
+		}
+		got[0] = "changed"
+		if definition.Command[0] != name {
+			t.Fatal("LaunchCommand() returned the registry's own slice")
+		}
+	}
+}
+
+func TestLaunchCommandPromptStyles(t *testing.T) {
+	t.Parallel()
+
+	prompt := "fix $(this) verbatim"
+	tests := []struct {
+		name    string
+		agent   Agent
+		want    []string
+		wantErr string
+	}{
+		{name: "positional", agent: Agent{Name: "a", Command: []string{"a"}, Prompt: PromptPositional}, want: []string{"a", prompt}},
+		{name: "flag", agent: Agent{Name: "b", Command: []string{"b", "run"}, Prompt: PromptFlag, PromptFlag: "-p"}, want: []string{"b", "run", "-p", prompt}},
+		{name: "flag missing", agent: Agent{Name: "c", Command: []string{"c"}, Prompt: PromptFlag}, wantErr: "c prompt flag is not configured"},
+		{name: "unsupported", agent: Agent{Name: "d", Command: []string{"d"}}, wantErr: "d does not support prompt launch"},
+	}
+	for _, tt := range tests {
+		t.Run(tt.name, func(t *testing.T) {
+			t.Parallel()
+			got, err := tt.agent.LaunchCommand(&prompt)
+			if tt.wantErr != "" {
+				if err == nil || err.Error() != tt.wantErr {
+					t.Fatalf("LaunchCommand() error = %v, want %q", err, tt.wantErr)
+				}
+				return
+			}
+			if err != nil {
+				t.Fatal(err)
+			}
+			if !reflect.DeepEqual(got, tt.want) {
+				t.Fatalf("LaunchCommand() = %q, want %q", got, tt.want)
+			}
+		})
+	}
+}
+
+func TestValidEnvNameMatchesDocumentedPattern(t *testing.T) {
+	t.Parallel()
+
+	pattern := regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
+	for _, name := range []string{
+		"", "A", "_", "a1", "1a", "ANTHROPIC_API_KEY", "bad-name", "has space", "trailing\n",
+		"é", "A\x00", "Z9_z", "9", "__", "a.b", "\xff",
+	} {
+		if got, want := validEnvName(name), pattern.MatchString(name); got != want {
+			t.Errorf("validEnvName(%q) = %v, want %v", name, got, want)
+		}
+	}
+	if err := ValidateEnvName("  PADDED  "); err != nil {
+		t.Errorf("ValidateEnvName() rejected a name with surrounding spaces: %v", err)
+	}
+	if err := ValidateEnvName("bad-name"); err == nil || err.Error() != `invalid env name "bad-name"; must match ^[A-Za-z_][A-Za-z0-9_]*$` {
+		t.Errorf("ValidateEnvName(bad-name) error = %v", err)
+	}
+}
````

#### File 3 of 6: `internal/app/run.go`

Imports: no import lines are added or removed in this file.

There are 2 edits in this file. They are listed from the bottom of the file to the top. Do them in the order shown: each edit only changes lines below the next one, so the line numbers stay correct. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 2: lines 149-155.**

Find this exact text (it appears exactly once in the file):

````go
	for _, name := range resolved {
		passed[name] = struct{}{}
	}
	for _, name := range agent.EnvVarNames(selected, requested) {
		if _, ok := passed[name]; ok {
			fmt.Fprintf(os.Stderr, "env: pass %s\n", name)
		} else {
````

Replace it with this exact text:

````go
	for _, name := range resolved {
		passed[name] = struct{}{}
	}
	for _, name := range selected.EnvNames(requested) {
		if _, ok := passed[name]; ok {
			fmt.Fprintf(os.Stderr, "env: pass %s\n", name)
		} else {
````

**Edit 2 of 2: lines 84-90.**

Find this exact text (it appears exactly once in the file):

````go
	if request.Agent != nil {
		selected = *request.Agent
	}
	envVars := agent.ResolveEnvVars(selected, request.EnvVars, func(name string) bool { _, ok := os.LookupEnv(name); return ok })
	if request.Verbose {
		logEnvResolution(selected, request.EnvVars, envVars)
	}
````

Replace it with this exact text:

````go
	if request.Agent != nil {
		selected = *request.Agent
	}
	envVars := selected.ResolveEnv(request.EnvVars, func(name string) bool { _, ok := os.LookupEnv(name); return ok })
	if request.Verbose {
		logEnvResolution(selected, request.EnvVars, envVars)
	}
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/app/run.go
+++ b/internal/app/run.go
@@ -84,7 +84,7 @@
 	if request.Agent != nil {
 		selected = *request.Agent
 	}
-	envVars := agent.ResolveEnvVars(selected, request.EnvVars, func(name string) bool { _, ok := os.LookupEnv(name); return ok })
+	envVars := selected.ResolveEnv(request.EnvVars, func(name string) bool { _, ok := os.LookupEnv(name); return ok })
 	if request.Verbose {
 		logEnvResolution(selected, request.EnvVars, envVars)
 	}
@@ -149,7 +149,7 @@
 	for _, name := range resolved {
 		passed[name] = struct{}{}
 	}
-	for _, name := range agent.EnvVarNames(selected, requested) {
+	for _, name := range selected.EnvNames(requested) {
 		if _, ok := passed[name]; ok {
 			fmt.Fprintf(os.Stderr, "env: pass %s\n", name)
 		} else {
````

#### File 4 of 6: `internal/cli/root.go`

Imports: no import lines are added or removed in this file.

There is 1 edit in this file. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 1: lines 83-92.**

Find this exact text (it appears exactly once in the file):

````go
	return runTarget{kind: "agent", agent: agent}, nil
}

func agentCommand(agent agent.Agent) []string {
	return append([]string(nil), agent.Command...)
}

func newBuildCmd() *cobra.Command {
	var noCache bool
	cmd := &cobra.Command{
````

Replace it with this exact text:

````go
	return runTarget{kind: "agent", agent: agent}, nil
}

func newBuildCmd() *cobra.Command {
	var noCache bool
	cmd := &cobra.Command{
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/cli/root.go
+++ b/internal/cli/root.go
@@ -83,10 +83,6 @@
 	return runTarget{kind: "agent", agent: agent}, nil
 }
 
-func agentCommand(agent agent.Agent) []string {
-	return append([]string(nil), agent.Command...)
-}
-
 func newBuildCmd() *cobra.Command {
 	var noCache bool
 	cmd := &cobra.Command{
````

#### File 5 of 6: `internal/cli/root_test.go`

Imports: removed `"reflect"`. (The edits below already contain these import changes.)

There are 2 edits in this file. They are listed from the bottom of the file to the top. Do them in the order shown: each edit only changes lines below the next one, so the line numbers stay correct. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 2: lines 176-198.**

Find this exact text (it appears exactly once in the file):

````go
		t.Fatalf("Execute() error = %v, want invalid env name rejection", err)
	}
}

func TestAgentCommandUsesRegistryCommand(t *testing.T) {
	t.Parallel()

	codex, err := agent.Lookup("codex")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := agentCommand(codex), []string{"codex"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("agentCommand() = %v, want %v", got, want)
	}

	claude, err := agent.Lookup("claude")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := agentCommand(claude), []string{"claude"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("agentCommand() = %v, want %v", got, want)
	}
}
````

Replace it with this exact text:

````go
		t.Fatalf("Execute() error = %v, want invalid env name rejection", err)
	}
}
````

**Edit 2 of 2: lines 4-10.**

Find this exact text (it appears exactly once in the file):

````go
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

````

Replace it with this exact text:

````go
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/cli/root_test.go
+++ b/internal/cli/root_test.go
@@ -4,7 +4,6 @@
 	"bytes"
 	"os"
 	"path/filepath"
-	"reflect"
 	"strings"
 	"testing"
 
@@ -176,23 +175,3 @@
 		t.Fatalf("Execute() error = %v, want invalid env name rejection", err)
 	}
 }
-
-func TestAgentCommandUsesRegistryCommand(t *testing.T) {
-	t.Parallel()
-
-	codex, err := agent.Lookup("codex")
-	if err != nil {
-		t.Fatal(err)
-	}
-	if got, want := agentCommand(codex), []string{"codex"}; !reflect.DeepEqual(got, want) {
-		t.Fatalf("agentCommand() = %v, want %v", got, want)
-	}
-
-	claude, err := agent.Lookup("claude")
-	if err != nil {
-		t.Fatal(err)
-	}
-	if got, want := agentCommand(claude), []string{"claude"}; !reflect.DeepEqual(got, want) {
-		t.Fatalf("agentCommand() = %v, want %v", got, want)
-	}
-}
````

#### File 6 of 6: `internal/plan/run.go`

Imports: no import lines are added or removed in this file.

There is 1 edit in this file. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 1: lines 45-65.**

Find this exact text (it appears exactly once in the file):

````go
		}
		return spec, nil
	}
	spec.Command = append([]string(nil), selected.Command...)
	spec.ReturnToShell = true
	if prompt == nil {
		return spec, nil
	}
	switch selected.Prompt {
	case agent.PromptPositional:
		spec.Command = append(spec.Command, *prompt)
	case agent.PromptFlag:
		if selected.PromptFlag == "" {
			return RunSpec{}, fmt.Errorf("%s prompt flag is not configured", selected.Name)
		}
		spec.Command = append(spec.Command, selected.PromptFlag, *prompt)
	default:
		return RunSpec{}, fmt.Errorf("%s does not support prompt launch", selected.Name)
	}
	return spec, nil
}
````

Replace it with this exact text:

````go
		}
		return spec, nil
	}
	command, err := selected.LaunchCommand(prompt)
	if err != nil {
		return RunSpec{}, err
	}
	spec.Command = command
	spec.ReturnToShell = true
	return spec, nil
}
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/plan/run.go
+++ b/internal/plan/run.go
@@ -45,21 +45,11 @@
 		}
 		return spec, nil
 	}
-	spec.Command = append([]string(nil), selected.Command...)
-	spec.ReturnToShell = true
-	if prompt == nil {
-		return spec, nil
-	}
-	switch selected.Prompt {
-	case agent.PromptPositional:
-		spec.Command = append(spec.Command, *prompt)
-	case agent.PromptFlag:
-		if selected.PromptFlag == "" {
-			return RunSpec{}, fmt.Errorf("%s prompt flag is not configured", selected.Name)
-		}
-		spec.Command = append(spec.Command, selected.PromptFlag, *prompt)
-	default:
-		return RunSpec{}, fmt.Errorf("%s does not support prompt launch", selected.Name)
+	command, err := selected.LaunchCommand(prompt)
+	if err != nil {
+		return RunSpec{}, err
 	}
+	spec.Command = command
+	spec.ReturnToShell = true
 	return spec, nil
 }
````

### Verify

Run these verification commands from the repository root:

````bash
gofmt -l .
go build ./...
go vet ./...
go test -count=1 ./...
````

Expected result:

- `gofmt -l .` prints nothing.
- `go build ./...` prints nothing.
- `go vet ./...` prints nothing.
- `go test -count=1 ./...` prints the lines below. The numbers of seconds at the end of the `ok` lines will be different on your machine, and lines may come in a different order. Every package must say `ok` or `[no test files]`:

````text
?   	github.com/JeremyOrr4/caged/internal/project	[no test files]
ok  	github.com/JeremyOrr4/caged/cmd/caged	0.497s
ok  	github.com/JeremyOrr4/caged/internal/agent	0.004s
ok  	github.com/JeremyOrr4/caged/internal/app	0.004s
ok  	github.com/JeremyOrr4/caged/internal/cli	0.011s
ok  	github.com/JeremyOrr4/caged/internal/docker	0.009s
ok  	github.com/JeremyOrr4/caged/internal/hostconfig	0.009s
ok  	github.com/JeremyOrr4/caged/internal/plan	0.003s
ok  	github.com/JeremyOrr4/caged/internal/runtime	0.003s
ok  	github.com/JeremyOrr4/caged/internal/system	0.005s
````

Count the passing tests and subtests with `go test -count=1 -v ./... 2>&1 | grep -c -- "--- PASS"`. On Linux the expected number is **128** (on macOS the Linux golden test is skipped, so the count there is lower by 52 (51 golden subtests plus their parent test)).

Run this command to compute the fingerprint of the whole repository (on macOS use the second command):

````bash
find . -type f -not -path './.git/*' -not -name REFACTOR_INSTRUCTIONS.md -print0 | LC_ALL=C sort -z | xargs -0 sha256sum | sha256sum
````

````bash
find . -type f -not -path './.git/*' -not -name REFACTOR_INSTRUCTIONS.md -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256 | shasum -a 256
````

Expected output (exactly this line):

````text
0cdb5a91af2307bf953fe4da1fe4efd02a5c680199c70be9660b7d755c3e24d7  -
````

**If it fails:**

- Problem: `go build` says `undefined: agent.ResolveEnvVars` or `undefined: agent.EnvVarNames`.
  Fix: One of the two calls in `internal/app/run.go` still uses the old function. Both must call methods on `selected`, exactly as in the edits for `internal/app/run.go` in this step.
- Problem: `go vet` says `"regexp" imported and not used` in `internal/agent/agent.go`.
  Fix: Remove `"regexp"` and `"os"` from the import block of `internal/agent/agent.go`. The test file `agent_test.go` is the one that now imports `regexp`.
- Problem: the fingerprint is different but every test passes.
  Fix: some file differs from these instructions by a character, often a tab turned into spaces or a missing final newline. Run `gofmt -w .` (it fixes Go indentation), then check the non-Go files of this step again.

### Checkpoint

At this point, the repo should have these 87 files (not counting `.git/` and this instructions file):

- `./` (5 files): `.gitignore`, `Makefile`, `README.md`, `go.mod`, `go.sum`
- `cmd/caged/` (2 files): `golden_test.go`, `main.go`
- `cmd/caged/testdata/golden/linux/` (51 files): `agent-claude.golden`, `agent-codex.golden`, `agent-copilot.golden`, `agent-gemini.golden`, `agent-gh.golden`, `agent-pi.golden`, `agent-uppercase.golden`, `build-daemon-down.golden`, `build-help.golden`, `build-no-cache.golden`, `build-rejects-args.golden`, `build.golden`, `complete-agents.golden`, `complete-prompt.golden`, `config-overrides.golden`, `daemon-down.golden`, `docker-missing.golden`, `env-extra-names.golden`, `env-invalid.golden`, `env-padded-name.golden`, `help-list.golden`, `help-unknown-topic.golden`, `help.golden`, `home-unresolvable.golden`, `image-missing-builds-first.golden`, `list-rejects-args.golden`, `list.golden`, `no-mount-with-workdir.golden`, `prompt-claude.golden`, `prompt-codex.golden`, `prompt-copilot.golden`, `prompt-gemini.golden`, `prompt-gh.golden`, `prompt-pi.golden`, `run-exit-code.golden`, `run-mounts-denied.golden`, `shell-env.golden`, `shell-no-mount-no-config.golden`, `shell.golden`, `too-many-args.golden`, `unknown-flag.golden`, `unsupported-agent.golden`, `verbose-claude.golden`, `version.golden`, `workdir-flag.golden`, `workdir-home-allowed.golden`, `workdir-is-home.golden`, `workdir-missing.golden`, `workdir-relative.golden`, `workdir-root-allowed.golden`, `workdir-root.golden`
- `dev/` (1 files): `Dockerfile`
- `docs/` (3 files): `architecture.md`, `implementation.md`, `spec.md`
- `internal/agent/` (3 files): `agent.go`, `agent_test.go`, `definitions_test.go`
- `internal/agent/testdata/` (1 files): `builtin_agents.golden`
- `internal/app/` (2 files): `run.go`, `run_test.go`
- `internal/cli/` (3 files): `doc.go`, `root.go`, `root_test.go`
- `internal/docker/` (4 files): `doc.go`, `run.go`, `run_test.go`, `runtime.go`
- `internal/hostconfig/` (2 files): `resolver.go`, `resolver_test.go`
- `internal/plan/` (2 files): `run.go`, `run_test.go`
- `internal/project/` (1 files): `mount.go`
- `internal/runtime/` (5 files): `Dockerfile`, `build.go`, `caged-entrypoint.sh`, `image.go`, `runtime_test.go`
- `internal/system/` (2 files): `system.go`, `system_test.go`


## Step 5: Replace the global agent map with a Registry and one file per agent

**Goal:** Put each built-in agent in its own file, register all of them in one list in `builtin.go`, and look them up through an immutable `agent.Registry`.

**Why:** This removes the package-level mutable map. Adding an agent now means one new file plus one line in `builtin.go`.

### Files touched

- `internal/agent/agent.go`: modify.
- `internal/agent/agent_test.go`: modify.
- `internal/agent/builtin.go`: create this new file.
- `internal/agent/claude.go`: create this new file.
- `internal/agent/codex.go`: create this new file.
- `internal/agent/copilot.go`: create this new file.
- `internal/agent/definitions_test.go`: modify.
- `internal/agent/gemini.go`: create this new file.
- `internal/agent/gh.go`: create this new file.
- `internal/agent/pi.go`: create this new file.
- `internal/agent/registry.go`: create this new file.
- `internal/agent/registry_test.go`: create this new file.
- `internal/app/run_test.go`: modify.
- `internal/cli/root.go`: modify.
- `internal/cli/root_test.go`: modify.
- `internal/hostconfig/resolver.go`: modify.
- `internal/plan/run_test.go`: modify.

### Commands to run

No shell commands are needed in this step. You only edit and create files.

### File edits

#### File 1 of 17: `internal/agent/agent.go`

Imports: removed `"sort"`. (The edits below already contain these import changes.)

There are 3 edits in this file. They are listed from the bottom of the file to the top. Do them in the order shown: each edit only changes lines below the next one, so the line numbers stay correct. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 3: lines 192-212.**

Find this exact text (it appears exactly once in the file):

````go
	return names
}

func Lookup(name string) (Agent, error) {
	key := strings.TrimSpace(strings.ToLower(name))
	if agent, ok := registry[key]; ok {
		return agent, nil
	}

	supported := Names()
	return Agent{}, fmt.Errorf("unsupported agent %q; supported agents: %s", name, strings.Join(supported, ", "))
}

func Names() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
````

Replace it with this exact text:

````go
	return names
}

// clone returns a deep copy so callers can never modify a registered definition.
func (a Agent) clone() Agent {
	a.Command = append([]string(nil), a.Command...)
	a.ConfigPaths = append([]ConfigPath(nil), a.ConfigPaths...)
	for i := range a.ConfigPaths {
		a.ConfigPaths[i].MaskPaths = append([]string(nil), a.ConfigPaths[i].MaskPaths...)
	}
	a.Auth.Environment = append([]string(nil), a.Auth.Environment...)
	a.Auth.CredentialPaths = append([]string(nil), a.Auth.CredentialPaths...)
	return a
}
````

**Edit 2 of 3: lines 45-119.**

Find this exact text (it appears exactly once in the file):

````go
	SupportsDeviceLogin bool
}

var registry = map[string]Agent{
	"pi": {
		Name:        "pi",
		Command:     []string{"pi"},
		Description: "Pi coding agent",
		ConfigPaths: []ConfigPath{
			{HostPath: ".config/pi", ContainerPath: ".config/pi", Writable: true},
			{HostPath: ".pi", ContainerPath: ".pi", Writable: true},
		},
		Prompt: PromptPositional,
		Auth:   AuthStrategy{Environment: []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY"}},
	},
	"claude": {
		Name:        "claude",
		Command:     []string{"claude"},
		Description: "Claude Code",
		ConfigPaths: []ConfigPath{
			{HostPath: ".claude", ContainerPath: ".claude", Writable: true, EnvOverride: "CLAUDE_CONFIG_DIR"},
		},
		Prompt: PromptPositional,
		Auth:   AuthStrategy{Environment: []string{"ANTHROPIC_API_KEY"}, CredentialPaths: []string{".claude/.credentials.json"}},
	},
	"codex": {
		Name:        "codex",
		Command:     []string{"codex"},
		Description: "Codex CLI",
		ConfigPaths: []ConfigPath{
			{
				HostPath:      ".codex",
				ContainerPath: ".codex",
				Writable:      true,
				EnvOverride:   "CODEX_HOME",
				MaskPaths:     []string{"app-server-control", "app-server-daemon", "ipc", "packages/app-server-daemon", "tmp/arg0"},
			},
		},
		Prompt: PromptFlag, PromptFlag: "exec",
		Auth: AuthStrategy{Environment: []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY"}, CredentialPaths: []string{".codex/auth.json"}, SupportsDeviceLogin: true},
	},
	"gemini": {
		Name:        "gemini",
		Command:     []string{"gemini"},
		Description: "Gemini CLI",
		ConfigPaths: []ConfigPath{
			{HostPath: ".gemini", ContainerPath: ".gemini", Writable: true},
		},
		Prompt: PromptFlag, PromptFlag: "-i",
		Auth: AuthStrategy{Environment: []string{"GEMINI_API_KEY", "GOOGLE_API_KEY"}, CredentialPaths: []string{".gemini/oauth_creds.json"}},
	},
	"gh": {
		Name:        "gh",
		Command:     []string{"gh"},
		Description: "GitHub CLI",
		ConfigPaths: []ConfigPath{
			{HostPath: ".config/gh", ContainerPath: ".config/gh", Writable: true},
		},
		Auth: AuthStrategy{Environment: []string{"GH_TOKEN", "GITHUB_TOKEN"}, CredentialPaths: []string{".config/gh/hosts.yml"}},
	},
	"copilot": {
		Name:        "copilot",
		Command:     []string{"copilot"},
		Description: "GitHub Copilot CLI",
		ConfigPaths: []ConfigPath{
			{HostPath: ".copilot", ContainerPath: ".copilot", Writable: true, EnvOverride: "COPILOT_HOME"},
		},
		Prompt: PromptFlag, PromptFlag: "-p",
		Auth: AuthStrategy{Environment: []string{"COPILOT_GITHUB_TOKEN", "GH_TOKEN", "GITHUB_TOKEN"}, CredentialPaths: []string{".copilot/"}, SupportsDeviceLogin: true},
	},
}

// LaunchCommand returns the container command that starts the agent. A
// non-nil prompt is passed as exactly one argument in the agent's prompt style.
func (a Agent) LaunchCommand(prompt *string) ([]string, error) {
````

Replace it with this exact text:

````go
	SupportsDeviceLogin bool
}

// LaunchCommand returns the container command that starts the agent. A
// non-nil prompt is passed as exactly one argument in the agent's prompt style.
func (a Agent) LaunchCommand(prompt *string) ([]string, error) {
````

**Edit 3 of 3: lines 1-8.**

Find this exact text (it appears exactly once in the file):

````go
package agent

import (
	"fmt"
	"sort"
	"strings"
)

````

Replace it with this exact text:

````go
// Package agent defines the supported agent CLIs as data and keeps them in a Registry.
package agent

import (
	"fmt"
	"strings"
)

````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/agent/agent.go
+++ b/internal/agent/agent.go
@@ -1,8 +1,8 @@
+// Package agent defines the supported agent CLIs as data and keeps them in a Registry.
 package agent
 
 import (
 	"fmt"
-	"sort"
 	"strings"
 )
 
@@ -45,75 +45,6 @@
 	SupportsDeviceLogin bool
 }
 
-var registry = map[string]Agent{
-	"pi": {
-		Name:        "pi",
-		Command:     []string{"pi"},
-		Description: "Pi coding agent",
-		ConfigPaths: []ConfigPath{
-			{HostPath: ".config/pi", ContainerPath: ".config/pi", Writable: true},
-			{HostPath: ".pi", ContainerPath: ".pi", Writable: true},
-		},
-		Prompt: PromptPositional,
-		Auth:   AuthStrategy{Environment: []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY"}},
-	},
-	"claude": {
-		Name:        "claude",
-		Command:     []string{"claude"},
-		Description: "Claude Code",
-		ConfigPaths: []ConfigPath{
-			{HostPath: ".claude", ContainerPath: ".claude", Writable: true, EnvOverride: "CLAUDE_CONFIG_DIR"},
-		},
-		Prompt: PromptPositional,
-		Auth:   AuthStrategy{Environment: []string{"ANTHROPIC_API_KEY"}, CredentialPaths: []string{".claude/.credentials.json"}},
-	},
-	"codex": {
-		Name:        "codex",
-		Command:     []string{"codex"},
-		Description: "Codex CLI",
-		ConfigPaths: []ConfigPath{
-			{
-				HostPath:      ".codex",
-				ContainerPath: ".codex",
-				Writable:      true,
-				EnvOverride:   "CODEX_HOME",
-				MaskPaths:     []string{"app-server-control", "app-server-daemon", "ipc", "packages/app-server-daemon", "tmp/arg0"},
-			},
-		},
-		Prompt: PromptFlag, PromptFlag: "exec",
-		Auth: AuthStrategy{Environment: []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY"}, CredentialPaths: []string{".codex/auth.json"}, SupportsDeviceLogin: true},
-	},
-	"gemini": {
-		Name:        "gemini",
-		Command:     []string{"gemini"},
-		Description: "Gemini CLI",
-		ConfigPaths: []ConfigPath{
-			{HostPath: ".gemini", ContainerPath: ".gemini", Writable: true},
-		},
-		Prompt: PromptFlag, PromptFlag: "-i",
-		Auth: AuthStrategy{Environment: []string{"GEMINI_API_KEY", "GOOGLE_API_KEY"}, CredentialPaths: []string{".gemini/oauth_creds.json"}},
-	},
-	"gh": {
-		Name:        "gh",
-		Command:     []string{"gh"},
-		Description: "GitHub CLI",
-		ConfigPaths: []ConfigPath{
-			{HostPath: ".config/gh", ContainerPath: ".config/gh", Writable: true},
-		},
-		Auth: AuthStrategy{Environment: []string{"GH_TOKEN", "GITHUB_TOKEN"}, CredentialPaths: []string{".config/gh/hosts.yml"}},
-	},
-	"copilot": {
-		Name:        "copilot",
-		Command:     []string{"copilot"},
-		Description: "GitHub Copilot CLI",
-		ConfigPaths: []ConfigPath{
-			{HostPath: ".copilot", ContainerPath: ".copilot", Writable: true, EnvOverride: "COPILOT_HOME"},
-		},
-		Prompt: PromptFlag, PromptFlag: "-p",
-		Auth: AuthStrategy{Environment: []string{"COPILOT_GITHUB_TOKEN", "GH_TOKEN", "GITHUB_TOKEN"}, CredentialPaths: []string{".copilot/"}, SupportsDeviceLogin: true},
-	},
-}
-
 // LaunchCommand returns the container command that starts the agent. A
 // non-nil prompt is passed as exactly one argument in the agent's prompt style.
 func (a Agent) LaunchCommand(prompt *string) ([]string, error) {
@@ -192,21 +123,14 @@
 	return names
 }
 
-func Lookup(name string) (Agent, error) {
-	key := strings.TrimSpace(strings.ToLower(name))
-	if agent, ok := registry[key]; ok {
-		return agent, nil
-	}
-
-	supported := Names()
-	return Agent{}, fmt.Errorf("unsupported agent %q; supported agents: %s", name, strings.Join(supported, ", "))
-}
-
-func Names() []string {
-	names := make([]string, 0, len(registry))
-	for name := range registry {
-		names = append(names, name)
-	}
-	sort.Strings(names)
-	return names
+// clone returns a deep copy so callers can never modify a registered definition.
+func (a Agent) clone() Agent {
+	a.Command = append([]string(nil), a.Command...)
+	a.ConfigPaths = append([]ConfigPath(nil), a.ConfigPaths...)
+	for i := range a.ConfigPaths {
+		a.ConfigPaths[i].MaskPaths = append([]string(nil), a.ConfigPaths[i].MaskPaths...)
+	}
+	a.Auth.Environment = append([]string(nil), a.Auth.Environment...)
+	a.Auth.CredentialPaths = append([]string(nil), a.Auth.CredentialPaths...)
+	return a
 }
````

#### File 2 of 17: `internal/agent/agent_test.go`

Imports: no import lines are added or removed in this file.

There are 7 edits in this file. They are listed from the bottom of the file to the top. Do them in the order shown: each edit only changes lines below the next one, so the line numbers stay correct. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 7: lines 154-160.**

Find this exact text (it appears exactly once in the file):

````go
	t.Parallel()

	for _, name := range []string{"codex", "claude"} {
		definition, err := Lookup(name)
		if err != nil {
			t.Fatal(err)
		}
````

Replace it with this exact text:

````go
	t.Parallel()

	for _, name := range []string{"codex", "claude"} {
		definition, err := Builtin().Lookup(name)
		if err != nil {
			t.Fatal(err)
		}
````

**Edit 2 of 7: lines 85-91.**

Find this exact text (it appears exactly once in the file):

````go

func TestGitHubCLIUsesHostConfigAndTokens(t *testing.T) {
	t.Parallel()
	gh, err := Lookup("gh")
	if err != nil {
		t.Fatal(err)
	}
````

Replace it with this exact text:

````go

func TestGitHubCLIUsesHostConfigAndTokens(t *testing.T) {
	t.Parallel()
	gh, err := Builtin().Lookup("gh")
	if err != nil {
		t.Fatal(err)
	}
````

**Edit 3 of 7: lines 70-76.**

Find this exact text (it appears exactly once in the file):

````go

func TestCodexConfigUsesDirectoryMountAndMasksRuntimePaths(t *testing.T) {
	t.Parallel()
	codex, err := Lookup("codex")
	if err != nil {
		t.Fatal(err)
	}
````

Replace it with this exact text:

````go

func TestCodexConfigUsesDirectoryMountAndMasksRuntimePaths(t *testing.T) {
	t.Parallel()
	codex, err := Builtin().Lookup("codex")
	if err != nil {
		t.Fatal(err)
	}
````

**Edit 4 of 7: lines 57-63.**

Find this exact text (it appears exactly once in the file):

````go
func TestClaudeConfigUsesDirectoryInsteadOfSingleFileMount(t *testing.T) {
	t.Parallel()

	claude, err := Lookup("claude")
	if err != nil {
		t.Fatal(err)
	}
````

Replace it with this exact text:

````go
func TestClaudeConfigUsesDirectoryInsteadOfSingleFileMount(t *testing.T) {
	t.Parallel()

	claude, err := Builtin().Lookup("claude")
	if err != nil {
		t.Fatal(err)
	}
````

**Edit 5 of 7: lines 47-53.**

Find this exact text (it appears exactly once in the file):

````go
		})
	}

	names := Names()
	want := []string{"claude", "codex", "copilot", "gemini", "gh", "pi"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("Names() = %v, want %v", names, want)
````

Replace it with this exact text:

````go
		})
	}

	names := registry.Names()
	want := []string{"claude", "codex", "copilot", "gemini", "gh", "pi"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("Names() = %v, want %v", names, want)
````

**Edit 6 of 7: lines 28-34.**

Find this exact text (it appears exactly once in the file):

````go
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			agent, err := Lookup(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("Lookup() error = nil, want an error")
````

Replace it with this exact text:

````go
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			agent, err := registry.Lookup(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("Lookup() error = nil, want an error")
````

**Edit 7 of 7: lines 10-15.**

Find this exact text (it appears exactly once in the file):

````go
func TestLookup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
````

Replace it with this exact text:

````go
func TestLookup(t *testing.T) {
	t.Parallel()

	// An explicit registry keeps the exact-list assertions below independent
	// of agents added to Builtin later.
	registry, err := NewRegistry(claude(), codex(), copilot(), gemini(), gh(), pi())
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		input    string
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/agent/agent_test.go
+++ b/internal/agent/agent_test.go
@@ -10,6 +10,13 @@
 func TestLookup(t *testing.T) {
 	t.Parallel()
 
+	// An explicit registry keeps the exact-list assertions below independent
+	// of agents added to Builtin later.
+	registry, err := NewRegistry(claude(), codex(), copilot(), gemini(), gh(), pi())
+	if err != nil {
+		t.Fatal(err)
+	}
+
 	tests := []struct {
 		name     string
 		input    string
@@ -28,7 +35,7 @@
 		t.Run(tt.name, func(t *testing.T) {
 			t.Parallel()
 
-			agent, err := Lookup(tt.input)
+			agent, err := registry.Lookup(tt.input)
 			if tt.wantErr {
 				if err == nil {
 					t.Fatal("Lookup() error = nil, want an error")
@@ -47,7 +54,7 @@
 		})
 	}
 
-	names := Names()
+	names := registry.Names()
 	want := []string{"claude", "codex", "copilot", "gemini", "gh", "pi"}
 	if !reflect.DeepEqual(names, want) {
 		t.Fatalf("Names() = %v, want %v", names, want)
@@ -57,7 +64,7 @@
 func TestClaudeConfigUsesDirectoryInsteadOfSingleFileMount(t *testing.T) {
 	t.Parallel()
 
-	claude, err := Lookup("claude")
+	claude, err := Builtin().Lookup("claude")
 	if err != nil {
 		t.Fatal(err)
 	}
@@ -70,7 +77,7 @@
 
 func TestCodexConfigUsesDirectoryMountAndMasksRuntimePaths(t *testing.T) {
 	t.Parallel()
-	codex, err := Lookup("codex")
+	codex, err := Builtin().Lookup("codex")
 	if err != nil {
 		t.Fatal(err)
 	}
@@ -85,7 +92,7 @@
 
 func TestGitHubCLIUsesHostConfigAndTokens(t *testing.T) {
 	t.Parallel()
-	gh, err := Lookup("gh")
+	gh, err := Builtin().Lookup("gh")
 	if err != nil {
 		t.Fatal(err)
 	}
@@ -154,7 +161,7 @@
 	t.Parallel()
 
 	for _, name := range []string{"codex", "claude"} {
-		definition, err := Lookup(name)
+		definition, err := Builtin().Lookup(name)
 		if err != nil {
 			t.Fatal(err)
 		}
````

#### File 3 of 17: `internal/agent/builtin.go`

This is a NEW FILE (21 lines).

Imports: this new file has no import lines.

Create the file `internal/agent/builtin.go` with exactly this content:

````go
package agent

// Builtin returns the agents that ship with caged. To add an agent, create a
// file in this package with a function that returns its Agent value, then add
// one line calling that function to the list below.
func Builtin() *Registry {
	registry, err := NewRegistry(
		claude(),
		codex(),
		copilot(),
		gemini(),
		gh(),
		pi(),
	)
	if err != nil {
		// The built-in definitions are fixed at compile time, and
		// TestBuiltinRegistryContract fails before a broken one can ship.
		panic(err)
	}
	return registry
}
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 4 of 17: `internal/agent/claude.go`

This is a NEW FILE (15 lines).

Imports: this new file has no import lines.

Create the file `internal/agent/claude.go` with exactly this content:

````go
package agent

// claude is Anthropic's Claude Code CLI.
func claude() Agent {
	return Agent{
		Name:        "claude",
		Command:     []string{"claude"},
		Description: "Claude Code",
		ConfigPaths: []ConfigPath{
			{HostPath: ".claude", ContainerPath: ".claude", Writable: true, EnvOverride: "CLAUDE_CONFIG_DIR"},
		},
		Prompt: PromptPositional,
		Auth:   AuthStrategy{Environment: []string{"ANTHROPIC_API_KEY"}, CredentialPaths: []string{".claude/.credentials.json"}},
	}
}
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 5 of 17: `internal/agent/codex.go`

This is a NEW FILE (24 lines).

Imports: this new file has no import lines.

Create the file `internal/agent/codex.go` with exactly this content:

````go
package agent

// codex is OpenAI's Codex CLI. Its home is mounted as one directory so atomic
// config writes work; the masked paths hide the host's daemon sockets and
// runtime binaries from the container.
func codex() Agent {
	return Agent{
		Name:        "codex",
		Command:     []string{"codex"},
		Description: "Codex CLI",
		ConfigPaths: []ConfigPath{
			{
				HostPath:      ".codex",
				ContainerPath: ".codex",
				Writable:      true,
				EnvOverride:   "CODEX_HOME",
				MaskPaths:     []string{"app-server-control", "app-server-daemon", "ipc", "packages/app-server-daemon", "tmp/arg0"},
			},
		},
		Prompt:     PromptFlag,
		PromptFlag: "exec",
		Auth:       AuthStrategy{Environment: []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY"}, CredentialPaths: []string{".codex/auth.json"}, SupportsDeviceLogin: true},
	}
}
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 6 of 17: `internal/agent/copilot.go`

This is a NEW FILE (16 lines).

Imports: this new file has no import lines.

Create the file `internal/agent/copilot.go` with exactly this content:

````go
package agent

// copilot is the GitHub Copilot CLI.
func copilot() Agent {
	return Agent{
		Name:        "copilot",
		Command:     []string{"copilot"},
		Description: "GitHub Copilot CLI",
		ConfigPaths: []ConfigPath{
			{HostPath: ".copilot", ContainerPath: ".copilot", Writable: true, EnvOverride: "COPILOT_HOME"},
		},
		Prompt:     PromptFlag,
		PromptFlag: "-p",
		Auth:       AuthStrategy{Environment: []string{"COPILOT_GITHUB_TOKEN", "GH_TOKEN", "GITHUB_TOKEN"}, CredentialPaths: []string{".copilot/"}, SupportsDeviceLogin: true},
	}
}
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 7 of 17: `internal/agent/definitions_test.go`

Imports: no import lines are added or removed in this file.

There is 1 edit in this file. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 1: lines 16-22.**

Find this exact text (it appears exactly once in the file):

````go

	var b strings.Builder
	for _, name := range []string{"claude", "codex", "copilot", "gemini", "gh", "pi"} {
		definition, err := Lookup(name)
		if err != nil {
			t.Fatal(err)
		}
````

Replace it with this exact text:

````go

	var b strings.Builder
	for _, name := range []string{"claude", "codex", "copilot", "gemini", "gh", "pi"} {
		definition, err := Builtin().Lookup(name)
		if err != nil {
			t.Fatal(err)
		}
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/agent/definitions_test.go
+++ b/internal/agent/definitions_test.go
@@ -16,7 +16,7 @@
 
 	var b strings.Builder
 	for _, name := range []string{"claude", "codex", "copilot", "gemini", "gh", "pi"} {
-		definition, err := Lookup(name)
+		definition, err := Builtin().Lookup(name)
 		if err != nil {
 			t.Fatal(err)
 		}
````

#### File 8 of 17: `internal/agent/gemini.go`

This is a NEW FILE (16 lines).

Imports: this new file has no import lines.

Create the file `internal/agent/gemini.go` with exactly this content:

````go
package agent

// gemini is Google's Gemini CLI.
func gemini() Agent {
	return Agent{
		Name:        "gemini",
		Command:     []string{"gemini"},
		Description: "Gemini CLI",
		ConfigPaths: []ConfigPath{
			{HostPath: ".gemini", ContainerPath: ".gemini", Writable: true},
		},
		Prompt:     PromptFlag,
		PromptFlag: "-i",
		Auth:       AuthStrategy{Environment: []string{"GEMINI_API_KEY", "GOOGLE_API_KEY"}, CredentialPaths: []string{".gemini/oauth_creds.json"}},
	}
}
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 9 of 17: `internal/agent/gh.go`

This is a NEW FILE (14 lines).

Imports: this new file has no import lines.

Create the file `internal/agent/gh.go` with exactly this content:

````go
package agent

// gh is the GitHub CLI. It takes no initial prompt.
func gh() Agent {
	return Agent{
		Name:        "gh",
		Command:     []string{"gh"},
		Description: "GitHub CLI",
		ConfigPaths: []ConfigPath{
			{HostPath: ".config/gh", ContainerPath: ".config/gh", Writable: true},
		},
		Auth: AuthStrategy{Environment: []string{"GH_TOKEN", "GITHUB_TOKEN"}, CredentialPaths: []string{".config/gh/hosts.yml"}},
	}
}
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 10 of 17: `internal/agent/pi.go`

This is a NEW FILE (16 lines).

Imports: this new file has no import lines.

Create the file `internal/agent/pi.go` with exactly this content:

````go
package agent

// pi is the Pi coding agent.
func pi() Agent {
	return Agent{
		Name:        "pi",
		Command:     []string{"pi"},
		Description: "Pi coding agent",
		ConfigPaths: []ConfigPath{
			{HostPath: ".config/pi", ContainerPath: ".config/pi", Writable: true},
			{HostPath: ".pi", ContainerPath: ".pi", Writable: true},
		},
		Prompt: PromptPositional,
		Auth:   AuthStrategy{Environment: []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY"}},
	}
}
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 11 of 17: `internal/agent/registry.go`

This is a NEW FILE (58 lines).

Imports: this new file has these import lines: `"fmt"`, `"sort"`, `"strings"`. (The file content below shows how they are grouped.)

Create the file `internal/agent/registry.go` with exactly this content:

````go
package agent

import (
	"fmt"
	"sort"
	"strings"
)

// Registry is an immutable set of agents that can be looked up by name.
type Registry struct {
	byName map[string]Agent
	names  []string
}

// NewRegistry builds a registry. Every agent needs a command and a unique,
// lowercase name without surrounding spaces, because Lookup normalizes user
// input the same way.
func NewRegistry(agents ...Agent) (*Registry, error) {
	registry := &Registry{byName: make(map[string]Agent, len(agents))}
	for _, definition := range agents {
		if definition.Name == "" || definition.Name != strings.TrimSpace(strings.ToLower(definition.Name)) {
			return nil, fmt.Errorf("agent name %q must be lowercase, non-empty, and without surrounding spaces", definition.Name)
		}
		if len(definition.Command) == 0 {
			return nil, fmt.Errorf("agent %q has no command", definition.Name)
		}
		if _, exists := registry.byName[definition.Name]; exists {
			return nil, fmt.Errorf("agent %q is registered more than once", definition.Name)
		}
		registry.byName[definition.Name] = definition.clone()
		registry.names = append(registry.names, definition.Name)
	}
	sort.Strings(registry.names)
	return registry, nil
}

// Lookup finds an agent by name, ignoring case and surrounding spaces.
func (r *Registry) Lookup(name string) (Agent, error) {
	key := strings.TrimSpace(strings.ToLower(name))
	if definition, ok := r.byName[key]; ok {
		return definition.clone(), nil
	}
	return Agent{}, fmt.Errorf("unsupported agent %q; supported agents: %s", name, strings.Join(r.names, ", "))
}

// Names returns the sorted agent names.
func (r *Registry) Names() []string {
	return append([]string(nil), r.names...)
}

// All returns every agent, sorted by name.
func (r *Registry) All() []Agent {
	agents := make([]Agent, 0, len(r.names))
	for _, name := range r.names {
		agents = append(agents, r.byName[name].clone())
	}
	return agents
}
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 12 of 17: `internal/agent/registry_test.go`

This is a NEW FILE (171 lines).

Imports: this new file has these import lines: `"path/filepath"`, `"reflect"`, `"sort"`, `"strings"`, `"testing"`. (The file content below shows how they are grouped.)

Create the file `internal/agent/registry_test.go` with exactly this content:

````go
package agent

import (
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestBuiltinIncludesShippedAgents(t *testing.T) {
	t.Parallel()

	registry := Builtin()
	for _, name := range []string{"claude", "codex", "copilot", "gemini", "gh", "pi"} {
		if _, err := registry.Lookup(name); err != nil {
			t.Errorf("Builtin() is missing %q: %v", name, err)
		}
	}
}

// TestBuiltinRegistryContract checks rules every agent must follow, so a new
// agent file is validated without editing any test.
func TestBuiltinRegistryContract(t *testing.T) {
	t.Parallel()

	registry := Builtin()
	names := registry.Names()
	if !sort.StringsAreSorted(names) {
		t.Fatalf("Names() = %v, want sorted", names)
	}
	all := registry.All()
	if len(all) != len(names) {
		t.Fatalf("All() returned %d agents, Names() %d", len(all), len(names))
	}
	for i, definition := range all {
		if definition.Name != names[i] {
			t.Errorf("All()[%d] = %q, want %q", i, definition.Name, names[i])
		}
		if definition.Description == "" {
			t.Errorf("%s: missing description", definition.Name)
		}
		if definition.Prompt == PromptFlag && definition.PromptFlag == "" {
			t.Errorf("%s: PromptFlag style without a flag", definition.Name)
		}
		for _, name := range definition.Auth.Environment {
			if err := ValidateEnvName(name); err != nil {
				t.Errorf("%s: %v", definition.Name, err)
			}
		}
		for _, path := range definition.ConfigPaths {
			for _, rel := range []string{path.HostPath, path.ContainerPath} {
				if rel == "" || filepath.IsAbs(rel) || filepath.Clean(rel) != rel || strings.HasPrefix(rel, "..") {
					t.Errorf("%s: config path %q must be a clean path relative to the home directory", definition.Name, rel)
				}
			}
			if path.EnvOverride != "" {
				if err := ValidateEnvName(path.EnvOverride); err != nil {
					t.Errorf("%s: %v", definition.Name, err)
				}
			}
		}
	}
}

func TestNewRegistryRejectsInvalidDefinitions(t *testing.T) {
	t.Parallel()

	valid := Agent{Name: "valid", Command: []string{"valid"}}
	tests := []struct {
		name   string
		agents []Agent
		want   string
	}{
		{name: "empty name", agents: []Agent{{Command: []string{"x"}}}, want: `agent name "" must be lowercase`},
		{name: "uppercase name", agents: []Agent{{Name: "Aider", Command: []string{"aider"}}}, want: `agent name "Aider" must be lowercase`},
		{name: "padded name", agents: []Agent{{Name: " aider", Command: []string{"aider"}}}, want: `agent name " aider" must be lowercase`},
		{name: "no command", agents: []Agent{{Name: "aider"}}, want: `agent "aider" has no command`},
		{name: "duplicate", agents: []Agent{valid, valid}, want: `agent "valid" is registered more than once`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewRegistry(tt.agents...); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("NewRegistry() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestRegistryLookupNormalizesInput(t *testing.T) {
	t.Parallel()

	got, err := Builtin().Lookup("  Claude ")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "claude" {
		t.Fatalf("Lookup() = %q, want claude", got.Name)
	}
	if _, err := Builtin().Lookup("  Nope "); err == nil || !strings.HasPrefix(err.Error(), `unsupported agent "  Nope "; supported agents: `) {
		t.Fatalf("Lookup() error = %v, want the original input quoted", err)
	}
}

func TestRegistryReturnsCopies(t *testing.T) {
	t.Parallel()

	registry := Builtin()
	names := registry.Names()
	names[0] = "changed"
	first, err := registry.Lookup("codex")
	if err != nil {
		t.Fatal(err)
	}
	first.Command[0] = "changed"
	first.ConfigPaths[0].MaskPaths[0] = "changed"
	first.Auth.Environment[0] = "changed"
	all := registry.All()
	all[0].Command[0] = "changed"

	if registry.Names()[0] == "changed" {
		t.Error("Names() exposed the registry's slice")
	}
	second, err := registry.Lookup("codex")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(second, codex()) {
		t.Errorf("Lookup() exposed registry internals; got %+v", second)
	}
	if again := registry.All(); again[0].Command[0] == "changed" {
		t.Error("All() exposed registry internals")
	}
}

// TestNewAgentNeedsOnlyItsDefinition shows the open/closed property: a new
// agent is usable through the registry without changes anywhere else. The
// name is test-only so the test keeps passing when real agents are added.
func TestNewAgentNeedsOnlyItsDefinition(t *testing.T) {
	t.Parallel()

	extra := Agent{
		Name:        "test-only-agent",
		Command:     []string{"test-only-agent"},
		Description: "Test-only agent",
		ConfigPaths: []ConfigPath{{HostPath: ".test-only-agent", ContainerPath: ".test-only-agent", Writable: true}},
		Prompt:      PromptFlag,
		PromptFlag:  "--message",
		Auth:        AuthStrategy{Environment: []string{"OPENAI_API_KEY"}},
	}
	registry, err := NewRegistry(append(Builtin().All(), extra)...)
	if err != nil {
		t.Fatal(err)
	}
	got, err := registry.Lookup("test-only-agent")
	if err != nil {
		t.Fatal(err)
	}
	prompt := "add tests"
	command, err := got.LaunchCommand(&prompt)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"test-only-agent", "--message", "add tests"}; !reflect.DeepEqual(command, want) {
		t.Fatalf("LaunchCommand() = %q, want %q", command, want)
	}
	if names := registry.Names(); len(names) != len(Builtin().Names())+1 || !sort.StringsAreSorted(names) {
		t.Fatalf("Names() = %v, want every built-in plus the new agent, sorted", names)
	}
}
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 13 of 17: `internal/app/run_test.go`

Imports: no import lines are added or removed in this file.

There is 1 edit in this file. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 1: lines 29-35.**

Find this exact text (it appears exactly once in the file):

````go
	runner := &fakeRunner{}
	runtime := &fakeRuntime{exists: true}
	service := New(runner, runtime)
	agent, err := agent.Lookup("codex")
	if err != nil {
		t.Fatal(err)
	}
````

Replace it with this exact text:

````go
	runner := &fakeRunner{}
	runtime := &fakeRuntime{exists: true}
	service := New(runner, runtime)
	agent, err := agent.Builtin().Lookup("codex")
	if err != nil {
		t.Fatal(err)
	}
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/app/run_test.go
+++ b/internal/app/run_test.go
@@ -29,7 +29,7 @@
 	runner := &fakeRunner{}
 	runtime := &fakeRuntime{exists: true}
 	service := New(runner, runtime)
-	agent, err := agent.Lookup("codex")
+	agent, err := agent.Builtin().Lookup("codex")
 	if err != nil {
 		t.Fatal(err)
 	}
````

#### File 14 of 17: `internal/cli/root.go`

Imports: no import lines are added or removed in this file.

There are 5 edits in this file. They are listed from the bottom of the file to the top. Do them in the order shown: each edit only changes lines below the next one, so the line numbers stay correct. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 5: lines 102-115.**

Find this exact text (it appears exactly once in the file):

````go
	return cmd
}

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List supported AI agents",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, name := range agent.Names() {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), name); err != nil {
					return err
				}
````

Replace it with this exact text:

````go
	return cmd
}

func newListCmd(agents *agent.Registry) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List supported AI agents",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, name := range agents.Names() {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), name); err != nil {
					return err
				}
````

**Edit 2 of 5: lines 71-82.**

Find this exact text (it appears exactly once in the file):

````go
	return cmd
}

func resolveTarget(args []string) (runTarget, error) {
	if len(args) == 0 {
		return runTarget{kind: "shell"}, nil
	}

	agent, err := agent.Lookup(args[0])
	if err != nil {
		return runTarget{}, err
	}
````

Replace it with this exact text:

````go
	return cmd
}

func resolveTarget(agents *agent.Registry, args []string) (runTarget, error) {
	if len(args) == 0 {
		return runTarget{kind: "shell"}, nil
	}

	agent, err := agents.Lookup(args[0])
	if err != nil {
		return runTarget{}, err
	}
````

**Edit 3 of 5: lines 60-66.**

Find this exact text (it appears exactly once in the file):

````go
	}

	cmd.AddCommand(newBuildCmd())
	cmd.AddCommand(newListCmd())
	cmd.Flags().BoolVar(&noMount, "no-mount", false, "do not mount the current working directory")
	cmd.Flags().StringVarP(&workDir, "workdir", "w", "", "mount this host directory at /workspace/<folder-name>")
	cmd.Flags().BoolVar(&allowBroadMount, "allow-broad-mount", false, "allow mounting the filesystem root or home directory")
````

Replace it with this exact text:

````go
	}

	cmd.AddCommand(newBuildCmd())
	cmd.AddCommand(newListCmd(agents))
	cmd.Flags().BoolVar(&noMount, "no-mount", false, "do not mount the current working directory")
	cmd.Flags().StringVarP(&workDir, "workdir", "w", "", "mount this host directory at /workspace/<folder-name>")
	cmd.Flags().BoolVar(&allowBroadMount, "allow-broad-mount", false, "allow mounting the filesystem root or home directory")
````

**Edit 4 of 5: lines 34-43.**

Find this exact text (it appears exactly once in the file):

````go
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return agent.Names(), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := resolveTarget(args)
			if err != nil {
				return err
			}
````

Replace it with this exact text:

````go
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return agents.Names(), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := resolveTarget(agents, args)
			if err != nil {
				return err
			}
````

**Edit 5 of 5: lines 16-21.**

Find this exact text (it appears exactly once in the file):

````go

// NewRootCmd creates the root caged command.
func NewRootCmd(version string) *cobra.Command {
	var noMount bool
	var workDir string
	var noConfig bool
````

Replace it with this exact text:

````go

// NewRootCmd creates the root caged command.
func NewRootCmd(version string) *cobra.Command {
	agents := agent.Builtin()
	var noMount bool
	var workDir string
	var noConfig bool
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/cli/root.go
+++ b/internal/cli/root.go
@@ -16,6 +16,7 @@
 
 // NewRootCmd creates the root caged command.
 func NewRootCmd(version string) *cobra.Command {
+	agents := agent.Builtin()
 	var noMount bool
 	var workDir string
 	var noConfig bool
@@ -34,10 +35,10 @@
 			if len(args) > 0 {
 				return nil, cobra.ShellCompDirectiveNoFileComp
 			}
-			return agent.Names(), cobra.ShellCompDirectiveNoFileComp
+			return agents.Names(), cobra.ShellCompDirectiveNoFileComp
 		},
 		RunE: func(cmd *cobra.Command, args []string) error {
-			target, err := resolveTarget(args)
+			target, err := resolveTarget(agents, args)
 			if err != nil {
 				return err
 			}
@@ -60,7 +61,7 @@
 	}
 
 	cmd.AddCommand(newBuildCmd())
-	cmd.AddCommand(newListCmd())
+	cmd.AddCommand(newListCmd(agents))
 	cmd.Flags().BoolVar(&noMount, "no-mount", false, "do not mount the current working directory")
 	cmd.Flags().StringVarP(&workDir, "workdir", "w", "", "mount this host directory at /workspace/<folder-name>")
 	cmd.Flags().BoolVar(&allowBroadMount, "allow-broad-mount", false, "allow mounting the filesystem root or home directory")
@@ -71,12 +72,12 @@
 	return cmd
 }
 
-func resolveTarget(args []string) (runTarget, error) {
+func resolveTarget(agents *agent.Registry, args []string) (runTarget, error) {
 	if len(args) == 0 {
 		return runTarget{kind: "shell"}, nil
 	}
 
-	agent, err := agent.Lookup(args[0])
+	agent, err := agents.Lookup(args[0])
 	if err != nil {
 		return runTarget{}, err
 	}
@@ -102,14 +103,14 @@
 	return cmd
 }
 
-func newListCmd() *cobra.Command {
+func newListCmd(agents *agent.Registry) *cobra.Command {
 	cmd := &cobra.Command{
 		Use:          "list",
 		Short:        "List supported AI agents",
 		Args:         cobra.NoArgs,
 		SilenceUsage: true,
 		RunE: func(cmd *cobra.Command, args []string) error {
-			for _, name := range agent.Names() {
+			for _, name := range agents.Names() {
 				if _, err := fmt.Fprintln(cmd.OutOrStdout(), name); err != nil {
 					return err
 				}
````

#### File 15 of 17: `internal/cli/root_test.go`

Imports: no import lines are added or removed in this file.

There are 2 edits in this file. They are listed from the bottom of the file to the top. Do them in the order shown: each edit only changes lines below the next one, so the line numbers stay correct. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 2: lines 73-79.**

Find this exact text (it appears exactly once in the file):

````go
	t.Parallel()

	subcommands := map[string]struct{}{"build": {}, "list": {}}
	for _, name := range agent.Names() {
		if _, ok := subcommands[name]; ok {
			t.Errorf("agent %q collides with a root subcommand", name)
		}
````

Replace it with this exact text:

````go
	t.Parallel()

	subcommands := map[string]struct{}{"build": {}, "list": {}}
	for _, name := range agent.Builtin().Names() {
		if _, ok := subcommands[name]; ok {
			t.Errorf("agent %q collides with a root subcommand", name)
		}
````

**Edit 2 of 2: lines 49-55.**

Find this exact text (it appears exactly once in the file):

````go
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			target, err := resolveTarget(tt.args)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("resolveTarget(%v) error = %v, want %q", tt.args, err, tt.wantErr)
````

Replace it with this exact text:

````go
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			target, err := resolveTarget(agent.Builtin(), tt.args)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("resolveTarget(%v) error = %v, want %q", tt.args, err, tt.wantErr)
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/cli/root_test.go
+++ b/internal/cli/root_test.go
@@ -49,7 +49,7 @@
 		t.Run(tt.name, func(t *testing.T) {
 			t.Parallel()
 
-			target, err := resolveTarget(tt.args)
+			target, err := resolveTarget(agent.Builtin(), tt.args)
 			if tt.wantErr != "" {
 				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
 					t.Fatalf("resolveTarget(%v) error = %v, want %q", tt.args, err, tt.wantErr)
@@ -73,7 +73,7 @@
 	t.Parallel()
 
 	subcommands := map[string]struct{}{"build": {}, "list": {}}
-	for _, name := range agent.Names() {
+	for _, name := range agent.Builtin().Names() {
 		if _, ok := subcommands[name]; ok {
 			t.Errorf("agent %q collides with a root subcommand", name)
 		}
````

#### File 16 of 17: `internal/hostconfig/resolver.go`

Imports: no import lines are added or removed in this file.

There is 1 edit in this file. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 1: lines 32-39.**

Find this exact text (it appears exactly once in the file):

````go
		return report
	}
	seen := make(map[string]struct{})
	for _, name := range agent.Names() {
		entry, _ := agent.Lookup(name)
		for _, path := range entry.ConfigPaths {
			hostPath := filepath.Join(home, filepath.Clean(path.HostPath))
			if path.EnvOverride != "" {
````

Replace it with this exact text:

````go
		return report
	}
	seen := make(map[string]struct{})
	for _, entry := range agent.Builtin().All() {
		for _, path := range entry.ConfigPaths {
			hostPath := filepath.Join(home, filepath.Clean(path.HostPath))
			if path.EnvOverride != "" {
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/hostconfig/resolver.go
+++ b/internal/hostconfig/resolver.go
@@ -32,8 +32,7 @@
 		return report
 	}
 	seen := make(map[string]struct{})
-	for _, name := range agent.Names() {
-		entry, _ := agent.Lookup(name)
+	for _, entry := range agent.Builtin().All() {
 		for _, path := range entry.ConfigPaths {
 			hostPath := filepath.Join(home, filepath.Clean(path.HostPath))
 			if path.EnvOverride != "" {
````

#### File 17 of 17: `internal/plan/run_test.go`

Imports: no import lines are added or removed in this file.

There are 2 edits in this file. They are listed from the bottom of the file to the top. Do them in the order shown: each edit only changes lines below the next one, so the line numbers stay correct. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 2: lines 40-46.**

Find this exact text (it appears exactly once in the file):

````go
}

func TestBuildRunSpecCopiesInputs(t *testing.T) {
	selected, _ := agent.Lookup("claude")
	command := selected.Command[0]
	prompt := "prompt"
	mount := &Mount{HostPath: "/project", ContainerPath: "/workspace/project"}
````

Replace it with this exact text:

````go
}

func TestBuildRunSpecCopiesInputs(t *testing.T) {
	selected, _ := agent.Builtin().Lookup("claude")
	command := selected.Command[0]
	prompt := "prompt"
	mount := &Mount{HostPath: "/project", ContainerPath: "/workspace/project"}
````

**Edit 2 of 2: lines 20-26.**

Find this exact text (it appears exactly once in the file):

````go
		{"copilot", "copilot", []string{"copilot", "-p", "hello $(world)"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			selected, err := agent.Lookup(tc.agent)
			if err != nil {
				t.Fatal(err)
			}
````

Replace it with this exact text:

````go
		{"copilot", "copilot", []string{"copilot", "-p", "hello $(world)"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			selected, err := agent.Builtin().Lookup(tc.agent)
			if err != nil {
				t.Fatal(err)
			}
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/plan/run_test.go
+++ b/internal/plan/run_test.go
@@ -20,7 +20,7 @@
 		{"copilot", "copilot", []string{"copilot", "-p", "hello $(world)"}},
 	} {
 		t.Run(tc.name, func(t *testing.T) {
-			selected, err := agent.Lookup(tc.agent)
+			selected, err := agent.Builtin().Lookup(tc.agent)
 			if err != nil {
 				t.Fatal(err)
 			}
@@ -40,7 +40,7 @@
 }
 
 func TestBuildRunSpecCopiesInputs(t *testing.T) {
-	selected, _ := agent.Lookup("claude")
+	selected, _ := agent.Builtin().Lookup("claude")
 	command := selected.Command[0]
 	prompt := "prompt"
 	mount := &Mount{HostPath: "/project", ContainerPath: "/workspace/project"}
````

### Verify

Run these verification commands from the repository root:

````bash
gofmt -l .
go build ./...
go vet ./...
go test -count=1 ./...
````

Expected result:

- `gofmt -l .` prints nothing.
- `go build ./...` prints nothing.
- `go vet ./...` prints nothing.
- `go test -count=1 ./...` prints the lines below. The numbers of seconds at the end of the `ok` lines will be different on your machine, and lines may come in a different order. Every package must say `ok` or `[no test files]`:

````text
?   	github.com/JeremyOrr4/caged/internal/project	[no test files]
ok  	github.com/JeremyOrr4/caged/cmd/caged	0.495s
ok  	github.com/JeremyOrr4/caged/internal/agent	0.006s
ok  	github.com/JeremyOrr4/caged/internal/app	0.004s
ok  	github.com/JeremyOrr4/caged/internal/cli	0.007s
ok  	github.com/JeremyOrr4/caged/internal/docker	0.005s
ok  	github.com/JeremyOrr4/caged/internal/hostconfig	0.010s
ok  	github.com/JeremyOrr4/caged/internal/plan	0.004s
ok  	github.com/JeremyOrr4/caged/internal/runtime	0.003s
ok  	github.com/JeremyOrr4/caged/internal/system	0.005s
````

Count the passing tests and subtests with `go test -count=1 -v ./... 2>&1 | grep -c -- "--- PASS"`. On Linux the expected number is **139** (on macOS the Linux golden test is skipped, so the count there is lower by 52 (51 golden subtests plus their parent test)).

Run this command to compute the fingerprint of the whole repository (on macOS use the second command):

````bash
find . -type f -not -path './.git/*' -not -name REFACTOR_INSTRUCTIONS.md -print0 | LC_ALL=C sort -z | xargs -0 sha256sum | sha256sum
````

````bash
find . -type f -not -path './.git/*' -not -name REFACTOR_INSTRUCTIONS.md -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256 | shasum -a 256
````

Expected output (exactly this line):

````text
bd154c2e269390830b325ff263d94d577855c13a7fb52685113e2b5cde0cf63a  -
````

**If it fails:**

- Problem: `go build` says `undefined: agent.Lookup` or `undefined: agent.Names`.
  Fix: A caller still uses the old package functions. Every caller must now say `agent.Builtin().Lookup(...)` or `agent.Builtin().Names()`, or use a `*agent.Registry` that it was given. Search for `agent.Lookup(` and `agent.Names(`.
- Problem: `TestBuiltinAgentDefinitionsAreUnchanged` fails.
  Fix: One of the six agent files has a typo in its data. The test prints the wanted and the actual value. Compare them field by field.
- Problem: the fingerprint is different but every test passes.
  Fix: some file differs from these instructions by a character, often a tab turned into spaces or a missing final newline. Run `gofmt -w .` (it fixes Go indentation), then check the non-Go files of this step again.

### Checkpoint

At this point, the repo should have these 96 files (not counting `.git/` and this instructions file):

- `./` (5 files): `.gitignore`, `Makefile`, `README.md`, `go.mod`, `go.sum`
- `cmd/caged/` (2 files): `golden_test.go`, `main.go`
- `cmd/caged/testdata/golden/linux/` (51 files): `agent-claude.golden`, `agent-codex.golden`, `agent-copilot.golden`, `agent-gemini.golden`, `agent-gh.golden`, `agent-pi.golden`, `agent-uppercase.golden`, `build-daemon-down.golden`, `build-help.golden`, `build-no-cache.golden`, `build-rejects-args.golden`, `build.golden`, `complete-agents.golden`, `complete-prompt.golden`, `config-overrides.golden`, `daemon-down.golden`, `docker-missing.golden`, `env-extra-names.golden`, `env-invalid.golden`, `env-padded-name.golden`, `help-list.golden`, `help-unknown-topic.golden`, `help.golden`, `home-unresolvable.golden`, `image-missing-builds-first.golden`, `list-rejects-args.golden`, `list.golden`, `no-mount-with-workdir.golden`, `prompt-claude.golden`, `prompt-codex.golden`, `prompt-copilot.golden`, `prompt-gemini.golden`, `prompt-gh.golden`, `prompt-pi.golden`, `run-exit-code.golden`, `run-mounts-denied.golden`, `shell-env.golden`, `shell-no-mount-no-config.golden`, `shell.golden`, `too-many-args.golden`, `unknown-flag.golden`, `unsupported-agent.golden`, `verbose-claude.golden`, `version.golden`, `workdir-flag.golden`, `workdir-home-allowed.golden`, `workdir-is-home.golden`, `workdir-missing.golden`, `workdir-relative.golden`, `workdir-root-allowed.golden`, `workdir-root.golden`
- `dev/` (1 files): `Dockerfile`
- `docs/` (3 files): `architecture.md`, `implementation.md`, `spec.md`
- `internal/agent/` (12 files): `agent.go`, `agent_test.go`, `builtin.go`, `claude.go`, `codex.go`, `copilot.go`, `definitions_test.go`, `gemini.go`, `gh.go`, `pi.go`, `registry.go`, `registry_test.go`
- `internal/agent/testdata/` (1 files): `builtin_agents.golden`
- `internal/app/` (2 files): `run.go`, `run_test.go`
- `internal/cli/` (3 files): `doc.go`, `root.go`, `root_test.go`
- `internal/docker/` (4 files): `doc.go`, `run.go`, `run_test.go`, `runtime.go`
- `internal/hostconfig/` (2 files): `resolver.go`, `resolver_test.go`
- `internal/plan/` (2 files): `run.go`, `run_test.go`
- `internal/project/` (1 files): `mount.go`
- `internal/runtime/` (5 files): `Dockerfile`, `build.go`, `caged-entrypoint.sh`, `image.go`, `runtime_test.go`
- `internal/system/` (2 files): `system.go`, `system_test.go`


## Step 6: Put project-directory validation behind a FileSystem interface

**Goal:** Turn `project.ResolveMount` into `(*project.Resolver).Resolve`, which uses an injected file system. Move the project tests out of `cli` into `project`.

**Why:** The project package now does no hidden I/O. Its tests live next to its code (before this step it had 0% coverage of its own), and fakes can test every error path.

### Files touched

- `internal/app/run.go`: modify.
- `internal/cli/root_test.go`: modify.
- `internal/project/mount.go`: modify.
- `internal/project/mount_test.go`: create this new file.

### Commands to run

No shell commands are needed in this step. You only edit and create files.

### File edits

#### File 1 of 4: `internal/app/run.go`

Imports: added `"github.com/JeremyOrr4/caged/internal/system"`. (The edits below already contain these import changes.)

There are 2 edits in this file. They are listed from the bottom of the file to the top. Do them in the order shown: each edit only changes lines below the next one, so the line numbers stay correct. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 2: lines 112-118.**

Find this exact text (it appears exactly once in the file):

````go
		if err != nil {
			return fmt.Errorf("find home directory: %w", err)
		}
		options.Mount, err = project.ResolveMount(workDir, home, request.AllowBroadMount)
		if err != nil {
			return err
		}
````

Replace it with this exact text:

````go
		if err != nil {
			return fmt.Errorf("find home directory: %w", err)
		}
		options.Mount, err = project.NewResolver(system.FS{}).Resolve(workDir, home, request.AllowBroadMount)
		if err != nil {
			return err
		}
````

**Edit 2 of 2: lines 11-16.**

Find this exact text (it appears exactly once in the file):

````go
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/project"
	"github.com/JeremyOrr4/caged/internal/runtime"
)

// Runner and Runtime keep application policy independent of Docker's process API.
````

Replace it with this exact text:

````go
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/project"
	"github.com/JeremyOrr4/caged/internal/runtime"
	"github.com/JeremyOrr4/caged/internal/system"
)

// Runner and Runtime keep application policy independent of Docker's process API.
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/app/run.go
+++ b/internal/app/run.go
@@ -11,6 +11,7 @@
 	"github.com/JeremyOrr4/caged/internal/plan"
 	"github.com/JeremyOrr4/caged/internal/project"
 	"github.com/JeremyOrr4/caged/internal/runtime"
+	"github.com/JeremyOrr4/caged/internal/system"
 )
 
 // Runner and Runtime keep application policy independent of Docker's process API.
@@ -112,7 +113,7 @@
 		if err != nil {
 			return fmt.Errorf("find home directory: %w", err)
 		}
-		options.Mount, err = project.ResolveMount(workDir, home, request.AllowBroadMount)
+		options.Mount, err = project.NewResolver(system.FS{}).Resolve(workDir, home, request.AllowBroadMount)
 		if err != nil {
 			return err
 		}
````

#### File 2 of 4: `internal/cli/root_test.go`

Imports: removed `"os"`, `"path/filepath"`, `"github.com/JeremyOrr4/caged/internal/project"`. (The edits below already contain these import changes.)

There are 2 edits in this file. They are listed from the bottom of the file to the top. Do them in the order shown: each edit only changes lines below the next one, so the line numbers stay correct. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 2: lines 80-157.**

Find this exact text (it appears exactly once in the file):

````go
	}
}

func TestNewMountGuardRails(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		cwd   string
		home  string
		allow bool
		want  string
	}{
		{name: "filesystem root", cwd: "/", home: "/Users/test", want: "filesystem root"},
		{name: "home directory", cwd: "/Users/test", home: "/Users/test", want: "entire home directory"},
		{name: "allowed home directory", cwd: "/Users/test", home: "/Users/test", allow: true},
		{name: "project directory", cwd: "/Users/test/code/caged", home: "/Users/test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mount, err := project.NewMount(tt.cwd, tt.home, tt.allow)
			if tt.want != "" {
				if err == nil || !strings.Contains(err.Error(), tt.want) {
					t.Fatalf("newMount() error = %v, want %q", err, tt.want)
				}
				return
			}
			if err != nil {
				t.Fatalf("newMount() error = %v", err)
			}
			wantPath := filepath.Join("/workspace", filepath.Base(filepath.Clean(tt.cwd)))
			if mount.ContainerPath != wantPath {
				t.Errorf("container path = %q, want %q", mount.ContainerPath, wantPath)
			}
		})
	}
}

func TestResolveProjectMount(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	projectPath := filepath.Join(t.TempDir(), "project")
	if err := os.Mkdir(projectPath, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(projectPath, "file.txt")
	if err := os.WriteFile(file, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}

	mount, err := project.ResolveMount(projectPath, home, false)
	if err != nil {
		t.Fatalf("resolveProjectMount() error = %v", err)
	}
	resolvedProject, err := filepath.EvalSymlinks(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if mount.HostPath != resolvedProject || mount.ContainerPath != "/workspace/project" {
		t.Fatalf("resolveProjectMount() = %+v, want host %q at /workspace/project", mount, resolvedProject)
	}

	if _, err := project.ResolveMount(file, home, false); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("resolveProjectMount(file) error = %v, want not-a-directory error", err)
	}

	if _, err := project.ResolveMount(home, home, false); err == nil || !strings.Contains(err.Error(), "entire home directory") {
		t.Fatalf("resolveProjectMount(home) error = %v, want broad-mount rejection", err)
	}
}

func TestNoMountAndWorkdirAreIncompatible(t *testing.T) {
	t.Parallel()

````

Replace it with this exact text:

````go
	}
}

func TestNoMountAndWorkdirAreIncompatible(t *testing.T) {
	t.Parallel()

````

**Edit 2 of 2: lines 2-16.**

Find this exact text (it appears exactly once in the file):

````go

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/app"
	"github.com/JeremyOrr4/caged/internal/docker"
	"github.com/JeremyOrr4/caged/internal/project"
)

func TestRootCmdRejectsTooManyArguments(t *testing.T) {
````

Replace it with this exact text:

````go

import (
	"bytes"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/app"
	"github.com/JeremyOrr4/caged/internal/docker"
)

func TestRootCmdRejectsTooManyArguments(t *testing.T) {
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/cli/root_test.go
+++ b/internal/cli/root_test.go
@@ -2,15 +2,12 @@
 
 import (
 	"bytes"
-	"os"
-	"path/filepath"
 	"strings"
 	"testing"
 
 	"github.com/JeremyOrr4/caged/internal/agent"
 	"github.com/JeremyOrr4/caged/internal/app"
 	"github.com/JeremyOrr4/caged/internal/docker"
-	"github.com/JeremyOrr4/caged/internal/project"
 )
 
 func TestRootCmdRejectsTooManyArguments(t *testing.T) {
@@ -80,78 +77,6 @@
 	}
 }
 
-func TestNewMountGuardRails(t *testing.T) {
-	t.Parallel()
-
-	tests := []struct {
-		name  string
-		cwd   string
-		home  string
-		allow bool
-		want  string
-	}{
-		{name: "filesystem root", cwd: "/", home: "/Users/test", want: "filesystem root"},
-		{name: "home directory", cwd: "/Users/test", home: "/Users/test", want: "entire home directory"},
-		{name: "allowed home directory", cwd: "/Users/test", home: "/Users/test", allow: true},
-		{name: "project directory", cwd: "/Users/test/code/caged", home: "/Users/test"},
-	}
-
-	for _, tt := range tests {
-		t.Run(tt.name, func(t *testing.T) {
-			t.Parallel()
-
-			mount, err := project.NewMount(tt.cwd, tt.home, tt.allow)
-			if tt.want != "" {
-				if err == nil || !strings.Contains(err.Error(), tt.want) {
-					t.Fatalf("newMount() error = %v, want %q", err, tt.want)
-				}
-				return
-			}
-			if err != nil {
-				t.Fatalf("newMount() error = %v", err)
-			}
-			wantPath := filepath.Join("/workspace", filepath.Base(filepath.Clean(tt.cwd)))
-			if mount.ContainerPath != wantPath {
-				t.Errorf("container path = %q, want %q", mount.ContainerPath, wantPath)
-			}
-		})
-	}
-}
-
-func TestResolveProjectMount(t *testing.T) {
-	t.Parallel()
-
-	home := t.TempDir()
-	projectPath := filepath.Join(t.TempDir(), "project")
-	if err := os.Mkdir(projectPath, 0o755); err != nil {
-		t.Fatal(err)
-	}
-	file := filepath.Join(projectPath, "file.txt")
-	if err := os.WriteFile(file, []byte("test"), 0o600); err != nil {
-		t.Fatal(err)
-	}
-
-	mount, err := project.ResolveMount(projectPath, home, false)
-	if err != nil {
-		t.Fatalf("resolveProjectMount() error = %v", err)
-	}
-	resolvedProject, err := filepath.EvalSymlinks(projectPath)
-	if err != nil {
-		t.Fatal(err)
-	}
-	if mount.HostPath != resolvedProject || mount.ContainerPath != "/workspace/project" {
-		t.Fatalf("resolveProjectMount() = %+v, want host %q at /workspace/project", mount, resolvedProject)
-	}
-
-	if _, err := project.ResolveMount(file, home, false); err == nil || !strings.Contains(err.Error(), "not a directory") {
-		t.Fatalf("resolveProjectMount(file) error = %v, want not-a-directory error", err)
-	}
-
-	if _, err := project.ResolveMount(home, home, false); err == nil || !strings.Contains(err.Error(), "entire home directory") {
-		t.Fatalf("resolveProjectMount(home) error = %v, want broad-mount rejection", err)
-	}
-}
-
 func TestNoMountAndWorkdirAreIncompatible(t *testing.T) {
 	t.Parallel()
 
````

#### File 3 of 4: `internal/project/mount.go`

Imports: added `"io/fs"`; removed `"os"`. (The edits below already contain these import changes.)

There is 1 edit in this file. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 1: lines 3-37.**

Find this exact text (it appears exactly once in the file):

````go

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/JeremyOrr4/caged/internal/plan"
)

func ResolveMount(workDir, home string, allowBroadMount bool) (*plan.Mount, error) {
	workDir, err := filepath.Abs(workDir)
	if err != nil {
		return nil, fmt.Errorf("resolve working directory: %w", err)
	}
	info, err := os.Stat(workDir)
	if err != nil {
		return nil, fmt.Errorf("inspect working directory %q: %w", workDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("working directory %q is not a directory", workDir)
	}
	workDir, err = filepath.EvalSymlinks(workDir)
	if err != nil {
		return nil, fmt.Errorf("resolve working directory symlinks: %w", err)
	}
	home, err = filepath.EvalSymlinks(home)
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}
	return NewMount(workDir, home, allowBroadMount)
}

func NewMount(cwd, home string, allowBroadMount bool) (*plan.Mount, error) {
	cwd, home = filepath.Clean(cwd), filepath.Clean(home)
	if !allowBroadMount && cwd == string(filepath.Separator) {
````

Replace it with this exact text:

````go

import (
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/JeremyOrr4/caged/internal/plan"
)

// FileSystem is the filesystem access the resolver needs.
type FileSystem interface {
	Abs(path string) (string, error)
	Stat(name string) (fs.FileInfo, error)
	EvalSymlinks(path string) (string, error)
}

// Resolver validates the host project directory and builds its bind mount.
type Resolver struct {
	fs FileSystem
}

// NewResolver returns a Resolver that inspects paths through fsys.
func NewResolver(fsys FileSystem) *Resolver {
	return &Resolver{fs: fsys}
}

// Resolve makes workDir absolute, checks that it is a directory, resolves
// symlinks in it and in home, and then applies the NewMount guard rails.
func (r *Resolver) Resolve(workDir, home string, allowBroadMount bool) (*plan.Mount, error) {
	workDir, err := r.fs.Abs(workDir)
	if err != nil {
		return nil, fmt.Errorf("resolve working directory: %w", err)
	}
	info, err := r.fs.Stat(workDir)
	if err != nil {
		return nil, fmt.Errorf("inspect working directory %q: %w", workDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("working directory %q is not a directory", workDir)
	}
	workDir, err = r.fs.EvalSymlinks(workDir)
	if err != nil {
		return nil, fmt.Errorf("resolve working directory symlinks: %w", err)
	}
	home, err = r.fs.EvalSymlinks(home)
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}
	return NewMount(workDir, home, allowBroadMount)
}

// NewMount refuses the filesystem root and the home directory unless
// allowBroadMount is set, and mounts cwd at /workspace/<folder-name>.
func NewMount(cwd, home string, allowBroadMount bool) (*plan.Mount, error) {
	cwd, home = filepath.Clean(cwd), filepath.Clean(home)
	if !allowBroadMount && cwd == string(filepath.Separator) {
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/project/mount.go
+++ b/internal/project/mount.go
@@ -3,35 +3,56 @@
 
 import (
 	"fmt"
-	"os"
+	"io/fs"
 	"path/filepath"
 
 	"github.com/JeremyOrr4/caged/internal/plan"
 )
 
-func ResolveMount(workDir, home string, allowBroadMount bool) (*plan.Mount, error) {
-	workDir, err := filepath.Abs(workDir)
+// FileSystem is the filesystem access the resolver needs.
+type FileSystem interface {
+	Abs(path string) (string, error)
+	Stat(name string) (fs.FileInfo, error)
+	EvalSymlinks(path string) (string, error)
+}
+
+// Resolver validates the host project directory and builds its bind mount.
+type Resolver struct {
+	fs FileSystem
+}
+
+// NewResolver returns a Resolver that inspects paths through fsys.
+func NewResolver(fsys FileSystem) *Resolver {
+	return &Resolver{fs: fsys}
+}
+
+// Resolve makes workDir absolute, checks that it is a directory, resolves
+// symlinks in it and in home, and then applies the NewMount guard rails.
+func (r *Resolver) Resolve(workDir, home string, allowBroadMount bool) (*plan.Mount, error) {
+	workDir, err := r.fs.Abs(workDir)
 	if err != nil {
 		return nil, fmt.Errorf("resolve working directory: %w", err)
 	}
-	info, err := os.Stat(workDir)
+	info, err := r.fs.Stat(workDir)
 	if err != nil {
 		return nil, fmt.Errorf("inspect working directory %q: %w", workDir, err)
 	}
 	if !info.IsDir() {
 		return nil, fmt.Errorf("working directory %q is not a directory", workDir)
 	}
-	workDir, err = filepath.EvalSymlinks(workDir)
+	workDir, err = r.fs.EvalSymlinks(workDir)
 	if err != nil {
 		return nil, fmt.Errorf("resolve working directory symlinks: %w", err)
 	}
-	home, err = filepath.EvalSymlinks(home)
+	home, err = r.fs.EvalSymlinks(home)
 	if err != nil {
 		return nil, fmt.Errorf("resolve home directory: %w", err)
 	}
 	return NewMount(workDir, home, allowBroadMount)
 }
 
+// NewMount refuses the filesystem root and the home directory unless
+// allowBroadMount is set, and mounts cwd at /workspace/<folder-name>.
 func NewMount(cwd, home string, allowBroadMount bool) (*plan.Mount, error) {
 	cwd, home = filepath.Clean(cwd), filepath.Clean(home)
 	if !allowBroadMount && cwd == string(filepath.Separator) {
````

#### File 4 of 4: `internal/project/mount_test.go`

This is a NEW FILE (195 lines).

Imports: this new file has these import lines: `"errors"`, `"io/fs"`, `"os"`, `"path/filepath"`, `"strings"`, `"testing"`, `"time"`, `"github.com/JeremyOrr4/caged/internal/system"`. (The file content below shows how they are grouped.)

Create the file `internal/project/mount_test.go` with exactly this content:

````go
package project

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JeremyOrr4/caged/internal/system"
)

func TestNewMountGuardRails(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		cwd   string
		home  string
		allow bool
		want  string
	}{
		{name: "filesystem root", cwd: "/", home: "/Users/test", want: "filesystem root"},
		{name: "home directory", cwd: "/Users/test", home: "/Users/test", want: "entire home directory"},
		{name: "allowed home directory", cwd: "/Users/test", home: "/Users/test", allow: true},
		{name: "project directory", cwd: "/Users/test/code/caged", home: "/Users/test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mount, err := NewMount(tt.cwd, tt.home, tt.allow)
			if tt.want != "" {
				if err == nil || !strings.Contains(err.Error(), tt.want) {
					t.Fatalf("newMount() error = %v, want %q", err, tt.want)
				}
				return
			}
			if err != nil {
				t.Fatalf("newMount() error = %v", err)
			}
			wantPath := filepath.Join("/workspace", filepath.Base(filepath.Clean(tt.cwd)))
			if mount.ContainerPath != wantPath {
				t.Errorf("container path = %q, want %q", mount.ContainerPath, wantPath)
			}
		})
	}
}

func TestResolveProjectMount(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	projectPath := filepath.Join(t.TempDir(), "project")
	if err := os.Mkdir(projectPath, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(projectPath, "file.txt")
	if err := os.WriteFile(file, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	resolver := NewResolver(system.FS{})

	mount, err := resolver.Resolve(projectPath, home, false)
	if err != nil {
		t.Fatalf("resolveProjectMount() error = %v", err)
	}
	resolvedProject, err := filepath.EvalSymlinks(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if mount.HostPath != resolvedProject || mount.ContainerPath != "/workspace/project" {
		t.Fatalf("resolveProjectMount() = %+v, want host %q at /workspace/project", mount, resolvedProject)
	}

	if _, err := resolver.Resolve(file, home, false); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("resolveProjectMount(file) error = %v, want not-a-directory error", err)
	}

	if _, err := resolver.Resolve(home, home, false); err == nil || !strings.Contains(err.Error(), "entire home directory") {
		t.Fatalf("resolveProjectMount(home) error = %v, want broad-mount rejection", err)
	}
}

// fakeFS answers from maps. Paths missing from entries do not exist, and
// paths missing from resolved cannot be resolved.
type fakeFS struct {
	cwd      string
	entries  map[string]bool   // path -> is a directory
	resolved map[string]string // EvalSymlinks results
	absErr   error
}

func (f fakeFS) Abs(path string) (string, error) {
	if f.absErr != nil {
		return "", f.absErr
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	return filepath.Join(f.cwd, path), nil
}

func (f fakeFS) Stat(name string) (fs.FileInfo, error) {
	isDir, ok := f.entries[name]
	if !ok {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
	}
	return fakeInfo{name: filepath.Base(name), dir: isDir}, nil
}

func (f fakeFS) EvalSymlinks(path string) (string, error) {
	if resolved, ok := f.resolved[path]; ok {
		return resolved, nil
	}
	return "", &fs.PathError{Op: "lstat", Path: path, Err: fs.ErrNotExist}
}

type fakeInfo struct {
	name string
	dir  bool
}

func (i fakeInfo) Name() string       { return i.name }
func (i fakeInfo) Size() int64        { return 0 }
func (i fakeInfo) ModTime() time.Time { return time.Time{} }
func (i fakeInfo) IsDir() bool        { return i.dir }
func (i fakeInfo) Sys() any           { return nil }
func (i fakeInfo) Mode() fs.FileMode {
	if i.dir {
		return fs.ModeDir | 0o755
	}
	return 0o644
}

func TestResolverResolvesRelativePathsAndMacOSSymlinks(t *testing.T) {
	t.Parallel()

	// On macOS /tmp is a symlink to /private/tmp; the mount must use the real path.
	fsys := fakeFS{
		cwd:      "/tmp",
		entries:  map[string]bool{"/tmp/app": true},
		resolved: map[string]string{"/tmp/app": "/private/tmp/app", "/Users/dev": "/Users/dev"},
	}
	mount, err := NewResolver(fsys).Resolve("app", "/Users/dev", false)
	if err != nil {
		t.Fatal(err)
	}
	if mount.HostPath != "/private/tmp/app" || mount.ContainerPath != "/workspace/app" {
		t.Fatalf("Resolve() = %+v", mount)
	}
}

func TestResolverComparesHomeAfterResolvingSymlinks(t *testing.T) {
	t.Parallel()

	fsys := fakeFS{
		entries:  map[string]bool{"/home/dev": true},
		resolved: map[string]string{"/home/dev": "/data/home/dev"},
	}
	if _, err := NewResolver(fsys).Resolve("/home/dev", "/home/dev", false); err == nil || !strings.Contains(err.Error(), "entire home directory") {
		t.Fatalf("Resolve(home) error = %v, want broad-mount rejection", err)
	}
}

func TestResolverErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		fsys fakeFS
		want string
	}{
		{name: "abs fails", fsys: fakeFS{absErr: errors.New("boom")}, want: "resolve working directory: boom"},
		{name: "missing directory", fsys: fakeFS{}, want: `inspect working directory "/work/app": stat /work/app: file does not exist`},
		{name: "regular file", fsys: fakeFS{entries: map[string]bool{"/work/app": false}}, want: `working directory "/work/app" is not a directory`},
		{name: "unresolvable directory", fsys: fakeFS{entries: map[string]bool{"/work/app": true}}, want: "resolve working directory symlinks: lstat /work/app: file does not exist"},
		{
			name: "unresolvable home",
			fsys: fakeFS{entries: map[string]bool{"/work/app": true}, resolved: map[string]string{"/work/app": "/work/app"}},
			want: "resolve home directory: lstat /Users/dev: file does not exist",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewResolver(tt.fsys).Resolve("/work/app", "/Users/dev", false); err == nil || err.Error() != tt.want {
				t.Fatalf("Resolve() error = %v, want %q", err, tt.want)
			}
		})
	}
}
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

### Verify

Run these verification commands from the repository root:

````bash
gofmt -l .
go build ./...
go vet ./...
go test -count=1 ./...
````

Expected result:

- `gofmt -l .` prints nothing.
- `go build ./...` prints nothing.
- `go vet ./...` prints nothing.
- `go test -count=1 ./...` prints the lines below. The numbers of seconds at the end of the `ok` lines will be different on your machine, and lines may come in a different order. Every package must say `ok` or `[no test files]`:

````text
ok  	github.com/JeremyOrr4/caged/cmd/caged	0.503s
ok  	github.com/JeremyOrr4/caged/internal/agent	0.005s
ok  	github.com/JeremyOrr4/caged/internal/app	0.004s
ok  	github.com/JeremyOrr4/caged/internal/cli	0.006s
ok  	github.com/JeremyOrr4/caged/internal/docker	0.006s
ok  	github.com/JeremyOrr4/caged/internal/hostconfig	0.009s
ok  	github.com/JeremyOrr4/caged/internal/plan	0.005s
ok  	github.com/JeremyOrr4/caged/internal/project	0.003s
ok  	github.com/JeremyOrr4/caged/internal/runtime	0.004s
ok  	github.com/JeremyOrr4/caged/internal/system	0.006s
````

Count the passing tests and subtests with `go test -count=1 -v ./... 2>&1 | grep -c -- "--- PASS"`. On Linux the expected number is **147** (on macOS the Linux golden test is skipped, so the count there is lower by 52 (51 golden subtests plus their parent test)).

Run this command to compute the fingerprint of the whole repository (on macOS use the second command):

````bash
find . -type f -not -path './.git/*' -not -name REFACTOR_INSTRUCTIONS.md -print0 | LC_ALL=C sort -z | xargs -0 sha256sum | sha256sum
````

````bash
find . -type f -not -path './.git/*' -not -name REFACTOR_INSTRUCTIONS.md -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256 | shasum -a 256
````

Expected output (exactly this line):

````text
f23d088f17adb8cee407d73d59377c735983b11f937219b0aa0a3976dc6b6c53  -
````

**If it fails:**

- Problem: `go vet` says `"os" imported and not used` in `internal/cli/root_test.go`.
  Fix: You removed the two moved tests but not their imports. In this step `root_test.go` must import only `bytes`, `strings`, `testing`, `agent`, `app` and `docker`.
- Problem: the fingerprint is different but every test passes.
  Fix: some file differs from these instructions by a character, often a tab turned into spaces or a missing final newline. Run `gofmt -w .` (it fixes Go indentation), then check the non-Go files of this step again.

### Checkpoint

At this point, the repo should have these 97 files (not counting `.git/` and this instructions file):

- `./` (5 files): `.gitignore`, `Makefile`, `README.md`, `go.mod`, `go.sum`
- `cmd/caged/` (2 files): `golden_test.go`, `main.go`
- `cmd/caged/testdata/golden/linux/` (51 files): `agent-claude.golden`, `agent-codex.golden`, `agent-copilot.golden`, `agent-gemini.golden`, `agent-gh.golden`, `agent-pi.golden`, `agent-uppercase.golden`, `build-daemon-down.golden`, `build-help.golden`, `build-no-cache.golden`, `build-rejects-args.golden`, `build.golden`, `complete-agents.golden`, `complete-prompt.golden`, `config-overrides.golden`, `daemon-down.golden`, `docker-missing.golden`, `env-extra-names.golden`, `env-invalid.golden`, `env-padded-name.golden`, `help-list.golden`, `help-unknown-topic.golden`, `help.golden`, `home-unresolvable.golden`, `image-missing-builds-first.golden`, `list-rejects-args.golden`, `list.golden`, `no-mount-with-workdir.golden`, `prompt-claude.golden`, `prompt-codex.golden`, `prompt-copilot.golden`, `prompt-gemini.golden`, `prompt-gh.golden`, `prompt-pi.golden`, `run-exit-code.golden`, `run-mounts-denied.golden`, `shell-env.golden`, `shell-no-mount-no-config.golden`, `shell.golden`, `too-many-args.golden`, `unknown-flag.golden`, `unsupported-agent.golden`, `verbose-claude.golden`, `version.golden`, `workdir-flag.golden`, `workdir-home-allowed.golden`, `workdir-is-home.golden`, `workdir-missing.golden`, `workdir-relative.golden`, `workdir-root-allowed.golden`, `workdir-root.golden`
- `dev/` (1 files): `Dockerfile`
- `docs/` (3 files): `architecture.md`, `implementation.md`, `spec.md`
- `internal/agent/` (12 files): `agent.go`, `agent_test.go`, `builtin.go`, `claude.go`, `codex.go`, `copilot.go`, `definitions_test.go`, `gemini.go`, `gh.go`, `pi.go`, `registry.go`, `registry_test.go`
- `internal/agent/testdata/` (1 files): `builtin_agents.golden`
- `internal/app/` (2 files): `run.go`, `run_test.go`
- `internal/cli/` (3 files): `doc.go`, `root.go`, `root_test.go`
- `internal/docker/` (4 files): `doc.go`, `run.go`, `run_test.go`, `runtime.go`
- `internal/hostconfig/` (2 files): `resolver.go`, `resolver_test.go`
- `internal/plan/` (2 files): `run.go`, `run_test.go`
- `internal/project/` (2 files): `mount.go`, `mount_test.go`
- `internal/runtime/` (5 files): `Dockerfile`, `build.go`, `caged-entrypoint.sh`, `image.go`, `runtime_test.go`
- `internal/system/` (2 files): `system.go`, `system_test.go`


## Step 7: Turn host-config discovery into hostconfig.Resolver (risky change 2 of 3)

**Goal:** Split the 106-line `ResolveAllConfigsReport` function into a `Resolver` type with small methods. Inject its file system, environment, agent list and shared files, and make `.gitconfig` a data entry.

**Why:** The old function mixed six jobs and read the real environment and disk directly. Now each rule is a small method, every error path can be tested with fakes, and a new shared file is one line of data.

### Files touched

- `internal/app/run.go`: modify.
- `internal/hostconfig/resolver.go`: modify.
- `internal/hostconfig/resolver_edges_test.go`: create this new file.
- `internal/hostconfig/resolver_test.go`: modify.
- `internal/hostconfig/tree.go`: create this new file.
- `internal/plan/run.go`: modify.

### Commands to run

No shell commands are needed in this step. You only edit and create files.

### File edits

#### File 1 of 6: `internal/app/run.go`

Imports: no import lines are added or removed in this file.

There are 2 edits in this file. They are listed from the bottom of the file to the top. Do them in the order shown: each edit only changes lines below the next one, so the line numbers stay correct. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 2: lines 133-139.**

Find this exact text (it appears exactly once in the file):

````go
	return nil
}

func logConfigReport(report hostconfig.ConfigReport) {
	for _, mount := range report.Mounts {
		fmt.Fprintf(os.Stderr, "config: pass %s -> %s\n", mount.HostPath, mount.DestPath)
	}
````

Replace it with this exact text:

````go
	return nil
}

func logConfigReport(report plan.ConfigReport) {
	for _, mount := range report.Mounts {
		fmt.Fprintf(os.Stderr, "config: pass %s -> %s\n", mount.HostPath, mount.DestPath)
	}
````

**Edit 2 of 2: lines 125-131.**

Find this exact text (it appears exactly once in the file):

````go
	if err != nil {
		return fmt.Errorf("find home directory: %w", err)
	}
	report := hostconfig.ResolveAllConfigsReport(home, nil)
	options.ConfigMounts = report.Mounts
	if request.Verbose {
		logConfigReport(report)
````

Replace it with this exact text:

````go
	if err != nil {
		return fmt.Errorf("find home directory: %w", err)
	}
	report := hostconfig.NewResolver(system.FS{}, system.Env{}, agent.Builtin().All(), hostconfig.DefaultSharedFiles()).Resolve(home)
	options.ConfigMounts = report.Mounts
	if request.Verbose {
		logConfigReport(report)
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/app/run.go
+++ b/internal/app/run.go
@@ -125,7 +125,7 @@
 	if err != nil {
 		return fmt.Errorf("find home directory: %w", err)
 	}
-	report := hostconfig.ResolveAllConfigsReport(home, nil)
+	report := hostconfig.NewResolver(system.FS{}, system.Env{}, agent.Builtin().All(), hostconfig.DefaultSharedFiles()).Resolve(home)
 	options.ConfigMounts = report.Mounts
 	if request.Verbose {
 		logConfigReport(report)
@@ -133,7 +133,7 @@
 	return nil
 }
 
-func logConfigReport(report hostconfig.ConfigReport) {
+func logConfigReport(report plan.ConfigReport) {
 	for _, mount := range report.Mounts {
 		fmt.Fprintf(os.Stderr, "config: pass %s -> %s\n", mount.HostPath, mount.DestPath)
 	}
````

#### File 2 of 6: `internal/hostconfig/resolver.go`

Imports: added `"io/fs"`; removed `"fmt"`, `"os"`, `"strings"`. (The edits below already contain these import changes.)

**Edit 1 of 1: replace the whole file.** At the start of this step the file has 265 lines (lines 1-265).

The file currently contains exactly this text:

````go
package hostconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/plan"
)

type ResolvedMount = plan.ConfigMount
type ConfigReport struct {
	Mounts  []ResolvedMount
	Missing []string
	Refused []string
}

func ResolveAllConfigsReport(home string, exists func(string) bool) ConfigReport {
	report := ConfigReport{}
	if exists == nil {
		exists = func(path string) bool {
			_, err := os.Stat(path)
			return err == nil
		}
	}
	home = filepath.Clean(home)
	resolvedHome, err := filepath.EvalSymlinks(home)
	if err != nil {
		report.Refused = append(report.Refused, home+" (home cannot be resolved)")
		return report
	}
	seen := make(map[string]struct{})
	for _, entry := range agent.Builtin().All() {
		for _, path := range entry.ConfigPaths {
			hostPath := filepath.Join(home, filepath.Clean(path.HostPath))
			if path.EnvOverride != "" {
				if override := os.Getenv(path.EnvOverride); override != "" {
					if !filepath.IsAbs(override) {
						override = filepath.Join(home, override)
					}
					hostPath = filepath.Clean(override)
				}
			}
			if !exists(hostPath) {
				report.Missing = appendUnique(report.Missing, hostPath)
				continue
			}
			resolvedHost, err := filepath.EvalSymlinks(hostPath)
			if err != nil {
				report.Refused = appendUnique(report.Refused, hostPath+" (unresolvable symlink)")
				continue
			}
			if filepath.Clean(resolvedHost) == filepath.Clean(resolvedHome) {
				report.Refused = appendUnique(report.Refused, hostPath+" (config path resolves to the home directory)")
				continue
			}
			if !IsWithinHome(resolvedHost, resolvedHome) {
				report.Refused = appendUnique(report.Refused, hostPath+" (resolves outside home)")
				continue
			}
			if len(path.MaskPaths) > 0 {
				if err := validateMaskedConfigTree(resolvedHost, path.MaskPaths); err != nil {
					report.Refused = appendUnique(report.Refused, hostPath+" ("+err.Error()+")")
					continue
				}
				key := resolvedHost + "\x00" + filepath.Clean(path.ContainerPath)
				if _, ok := seen[key]; !ok {
					seen[key] = struct{}{}
					report.Mounts = append(report.Mounts, ResolvedMount{
						Agent: entry.Name, Name: filepath.Clean(path.HostPath), HostPath: resolvedHost,
						DestPath: filepath.ToSlash(filepath.Join("/home/caged", filepath.Clean(path.ContainerPath))),
						Writable: path.Writable, MaskPaths: append([]string(nil), path.MaskPaths...),
					})
				}
				continue
			}
			mounts, refused := collectSafeMounts(resolvedHost)
			for _, reason := range refused {
				report.Refused = appendUnique(report.Refused, reason)
			}
			for _, mountPath := range mounts {
				relative, err := filepath.Rel(resolvedHost, mountPath)
				if err != nil {
					report.Refused = appendUnique(report.Refused, hostPath+" (cannot map safe config path)")
					continue
				}
				destPath := filepath.Clean(path.ContainerPath)
				if relative != "." {
					destPath = filepath.Join(destPath, relative)
				}
				key := mountPath + "\x00" + destPath
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				report.Mounts = append(report.Mounts, ResolvedMount{
					Agent:    entry.Name,
					Name:     filepath.Clean(path.HostPath),
					HostPath: mountPath,
					DestPath: filepath.ToSlash(filepath.Join("/home/caged", destPath)),
					Writable: path.Writable,
				})
			}
		}
	}
	// Git identity is shared across agents rather than duplicated in each entry.
	sharedGitConfig := filepath.Join(home, ".gitconfig")
	if exists(sharedGitConfig) {
		resolved, resolveErr := filepath.EvalSymlinks(sharedGitConfig)
		if resolveErr == nil && IsWithinHome(resolved, resolvedHome) {
			if info, statErr := os.Stat(resolved); statErr == nil && info.Mode().IsRegular() {
				report.Mounts = append(report.Mounts, ResolvedMount{Agent: "shared", Name: ".gitconfig", HostPath: resolved, DestPath: "/home/caged/.gitconfig", Writable: false})
			} else {
				report.Refused = appendUnique(report.Refused, sharedGitConfig+" (shared git config must be a regular file)")
			}
		} else {
			report.Refused = appendUnique(report.Refused, sharedGitConfig+" (resolves outside home or cannot be resolved)")
		}
	} else {
		report.Missing = appendUnique(report.Missing, sharedGitConfig)
	}
	return report
}

func validateMaskedConfigTree(root string, masks []string) error {
	rootInfo, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !rootInfo.IsDir() {
		return fmt.Errorf("masked config root must be a directory")
	}
	maskPaths := make([]string, 0, len(masks))
	for _, mask := range masks {
		clean := filepath.Clean(mask)
		if filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("invalid config mask path %q", mask)
		}
		maskPaths = append(maskPaths, filepath.Join(root, clean))
	}
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path != root {
			for _, mask := range maskPaths {
				if path == mask || IsWithinHome(path, mask) {
					if entry.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil || !IsWithinHome(resolved, root) {
				return fmt.Errorf("symlink %q resolves outside the config directory or cannot be resolved", path)
			}
			return nil
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("special file %q is not covered by a config mask", path)
		}
		return nil
	})
}

func collectSafeMounts(root string) ([]string, []string) {
	var mounts, refused []string
	var collect func(string)
	collect = func(path string) {
		entries, err := os.ReadDir(path)
		if err != nil {
			info, statErr := os.Lstat(path)
			if statErr == nil && info.Mode().IsRegular() {
				mounts = append(mounts, path)
			} else {
				refused = append(refused, path+" (cannot inspect config entry safely)")
			}
			return
		}
		if subtreeIsSafe(path) {
			mounts = append(mounts, path)
			return
		}
		for _, entry := range entries {
			child := filepath.Join(path, entry.Name())
			info, err := os.Lstat(child)
			if err != nil {
				refused = append(refused, child+" (cannot inspect config entry safely)")
				continue
			}
			if info.IsDir() {
				collect(child)
			} else if info.Mode().IsRegular() {
				mounts = append(mounts, child)
			} else {
				refused = append(refused, child+" (symlinks and special files are excluded from config mounts)")
			}
		}
	}
	info, err := os.Lstat(root)
	if err != nil {
		return nil, []string{root + " (cannot inspect config root safely)"}
	}
	if info.Mode().IsRegular() {
		return []string{root}, nil
	}
	if !info.IsDir() {
		return nil, []string{root + " (config root is not a regular file or directory)"}
	}
	collect(root)
	return mounts, refused
}

func subtreeIsSafe(root string) bool {
	entries, err := os.ReadDir(root)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		info, err := os.Lstat(path)
		if err != nil {
			return false
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return false
		}
		if info.IsDir() {
			if !subtreeIsSafe(path) {
				return false
			}
		} else if !info.Mode().IsRegular() {
			return false
		}
	}
	return true
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func IsWithinHome(path, home string) bool {
	path = filepath.Clean(path)
	home = filepath.Clean(home)
	rel, err := filepath.Rel(home, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
````

Replace the entire contents of the file with exactly this text:

````go
// Package hostconfig decides which host config paths are mounted into the
// container and refuses anything that is unsafe to expose.
package hostconfig

import (
	"io/fs"
	"path/filepath"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/plan"
)

// containerHome is the home directory of the user inside the runtime image.
const containerHome = "/home/caged"

// FileSystem is the read-only filesystem access the resolver needs.
type FileSystem interface {
	Stat(name string) (fs.FileInfo, error)
	Lstat(name string) (fs.FileInfo, error)
	ReadDir(name string) ([]fs.DirEntry, error)
	EvalSymlinks(path string) (string, error)
	WalkDir(root string, fn fs.WalkDirFunc) error
}

// Env looks up config-directory overrides such as CODEX_HOME.
type Env interface {
	LookupEnv(key string) (string, bool)
}

// SharedFile is a regular file in the home directory, such as .gitconfig,
// that all agents share. It is mounted read-only at the same relative path
// under the container home.
type SharedFile struct {
	Path        string // relative to the home directory
	Description string // used in refusal messages
}

// DefaultSharedFiles returns the shared files caged mounts.
func DefaultSharedFiles() []SharedFile {
	return []SharedFile{{Path: ".gitconfig", Description: "shared git config"}}
}

// Resolver finds the agent config paths and shared files that can be mounted.
type Resolver struct {
	fs     FileSystem
	env    Env
	agents []agent.Agent
	shared []SharedFile
}

// NewResolver returns a Resolver for the config paths of agents, in order,
// followed by the shared files.
func NewResolver(fsys FileSystem, env Env, agents []agent.Agent, shared []SharedFile) *Resolver {
	return &Resolver{fs: fsys, env: env, agents: agents, shared: shared}
}

// Resolve inspects every config path under home and reports what will be
// mounted, what is missing, and what is refused.
func (r *Resolver) Resolve(home string) plan.ConfigReport {
	home = filepath.Clean(home)
	resolvedHome, err := r.fs.EvalSymlinks(home)
	if err != nil {
		return plan.ConfigReport{Refused: []string{home + " (home cannot be resolved)"}}
	}
	pass := &resolution{fs: r.fs, home: home, resolvedHome: resolvedHome, seen: make(map[string]struct{})}
	for _, definition := range r.agents {
		for _, path := range definition.ConfigPaths {
			pass.agentPath(definition.Name, path, r.hostPath(home, path))
		}
	}
	for _, file := range r.shared {
		pass.sharedFile(file)
	}
	return pass.report
}

// hostPath is where a config path lives on the host. A set override variable
// (absolute, or relative to home) replaces the default location.
func (r *Resolver) hostPath(home string, path agent.ConfigPath) string {
	hostPath := filepath.Join(home, filepath.Clean(path.HostPath))
	if path.EnvOverride == "" {
		return hostPath
	}
	override, _ := r.env.LookupEnv(path.EnvOverride)
	if override == "" {
		return hostPath
	}
	if !filepath.IsAbs(override) {
		override = filepath.Join(home, override)
	}
	return filepath.Clean(override)
}

// resolution accumulates the report of one Resolve call.
type resolution struct {
	fs           FileSystem
	home         string
	resolvedHome string
	seen         map[string]struct{} // host path + "\x00" + container path already mounted
	report       plan.ConfigReport
}

// agentPath checks one agent config path and records its mounts, or why it
// is missing or refused.
func (r *resolution) agentPath(agentName string, path agent.ConfigPath, hostPath string) {
	if !r.exists(hostPath) {
		r.report.Missing = appendUnique(r.report.Missing, hostPath)
		return
	}
	resolvedHost, err := r.fs.EvalSymlinks(hostPath)
	if err != nil {
		r.refuse(hostPath + " (unresolvable symlink)")
		return
	}
	if filepath.Clean(resolvedHost) == filepath.Clean(r.resolvedHome) {
		r.refuse(hostPath + " (config path resolves to the home directory)")
		return
	}
	if !isWithin(resolvedHost, r.resolvedHome) {
		r.refuse(hostPath + " (resolves outside home)")
		return
	}
	name, dest := filepath.Clean(path.HostPath), filepath.Clean(path.ContainerPath)
	if len(path.MaskPaths) > 0 {
		// A masked tree is mounted whole; tmpfs mounts hide the masked entries.
		if err := validateMaskedConfigTree(r.fs, resolvedHost, path.MaskPaths); err != nil {
			r.refuse(hostPath + " (" + err.Error() + ")")
			return
		}
		r.addMount(agentName, name, resolvedHost, dest, path.Writable, append([]string(nil), path.MaskPaths...))
		return
	}
	mounts, refused := collectSafeMounts(r.fs, resolvedHost)
	for _, reason := range refused {
		r.refuse(reason)
	}
	for _, mountPath := range mounts {
		relative, err := filepath.Rel(resolvedHost, mountPath)
		if err != nil {
			r.refuse(hostPath + " (cannot map safe config path)")
			continue
		}
		mountDest := dest
		if relative != "." {
			mountDest = filepath.Join(dest, relative)
		}
		r.addMount(agentName, name, mountPath, mountDest, path.Writable, nil)
	}
}

// sharedFile checks one shared file. It must resolve inside home and be a
// regular file, and it is always mounted read-only.
func (r *resolution) sharedFile(file SharedFile) {
	name := filepath.Clean(file.Path)
	hostPath := filepath.Join(r.home, name)
	if !r.exists(hostPath) {
		r.report.Missing = appendUnique(r.report.Missing, hostPath)
		return
	}
	resolved, err := r.fs.EvalSymlinks(hostPath)
	if err != nil || !isWithin(resolved, r.resolvedHome) {
		r.refuse(hostPath + " (resolves outside home or cannot be resolved)")
		return
	}
	if info, err := r.fs.Stat(resolved); err != nil || !info.Mode().IsRegular() {
		r.refuse(hostPath + " (" + file.Description + " must be a regular file)")
		return
	}
	r.report.Mounts = append(r.report.Mounts, plan.ConfigMount{
		Agent:    "shared",
		Name:     name,
		HostPath: resolved,
		DestPath: filepath.ToSlash(filepath.Join(containerHome, name)),
		Writable: false,
	})
}

// addMount records a mount unless the same host path is already mounted at
// the same container path.
func (r *resolution) addMount(agentName, name, hostPath, dest string, writable bool, masks []string) {
	key := hostPath + "\x00" + dest
	if _, ok := r.seen[key]; ok {
		return
	}
	r.seen[key] = struct{}{}
	r.report.Mounts = append(r.report.Mounts, plan.ConfigMount{
		Agent:     agentName,
		Name:      name,
		HostPath:  hostPath,
		DestPath:  filepath.ToSlash(filepath.Join(containerHome, dest)),
		Writable:  writable,
		MaskPaths: masks,
	})
}

func (r *resolution) exists(path string) bool {
	_, err := r.fs.Stat(path)
	return err == nil
}

func (r *resolution) refuse(reason string) {
	r.report.Refused = appendUnique(r.report.Refused, reason)
}
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/hostconfig/resolver.go
+++ b/internal/hostconfig/resolver.go
@@ -1,265 +1,203 @@
+// Package hostconfig decides which host config paths are mounted into the
+// container and refuses anything that is unsafe to expose.
 package hostconfig
 
 import (
-	"fmt"
-	"os"
+	"io/fs"
 	"path/filepath"
-	"strings"
 
 	"github.com/JeremyOrr4/caged/internal/agent"
 	"github.com/JeremyOrr4/caged/internal/plan"
 )
 
-type ResolvedMount = plan.ConfigMount
-type ConfigReport struct {
-	Mounts  []ResolvedMount
-	Missing []string
-	Refused []string
+// containerHome is the home directory of the user inside the runtime image.
+const containerHome = "/home/caged"
+
+// FileSystem is the read-only filesystem access the resolver needs.
+type FileSystem interface {
+	Stat(name string) (fs.FileInfo, error)
+	Lstat(name string) (fs.FileInfo, error)
+	ReadDir(name string) ([]fs.DirEntry, error)
+	EvalSymlinks(path string) (string, error)
+	WalkDir(root string, fn fs.WalkDirFunc) error
 }
 
-func ResolveAllConfigsReport(home string, exists func(string) bool) ConfigReport {
-	report := ConfigReport{}
-	if exists == nil {
-		exists = func(path string) bool {
-			_, err := os.Stat(path)
-			return err == nil
-		}
-	}
+// Env looks up config-directory overrides such as CODEX_HOME.
+type Env interface {
+	LookupEnv(key string) (string, bool)
+}
+
+// SharedFile is a regular file in the home directory, such as .gitconfig,
+// that all agents share. It is mounted read-only at the same relative path
+// under the container home.
+type SharedFile struct {
+	Path        string // relative to the home directory
+	Description string // used in refusal messages
+}
+
+// DefaultSharedFiles returns the shared files caged mounts.
+func DefaultSharedFiles() []SharedFile {
+	return []SharedFile{{Path: ".gitconfig", Description: "shared git config"}}
+}
+
+// Resolver finds the agent config paths and shared files that can be mounted.
+type Resolver struct {
+	fs     FileSystem
+	env    Env
+	agents []agent.Agent
+	shared []SharedFile
+}
+
+// NewResolver returns a Resolver for the config paths of agents, in order,
+// followed by the shared files.
+func NewResolver(fsys FileSystem, env Env, agents []agent.Agent, shared []SharedFile) *Resolver {
+	return &Resolver{fs: fsys, env: env, agents: agents, shared: shared}
+}
+
+// Resolve inspects every config path under home and reports what will be
+// mounted, what is missing, and what is refused.
+func (r *Resolver) Resolve(home string) plan.ConfigReport {
 	home = filepath.Clean(home)
-	resolvedHome, err := filepath.EvalSymlinks(home)
+	resolvedHome, err := r.fs.EvalSymlinks(home)
 	if err != nil {
-		report.Refused = append(report.Refused, home+" (home cannot be resolved)")
-		return report
+		return plan.ConfigReport{Refused: []string{home + " (home cannot be resolved)"}}
 	}
-	seen := make(map[string]struct{})
-	for _, entry := range agent.Builtin().All() {
-		for _, path := range entry.ConfigPaths {
-			hostPath := filepath.Join(home, filepath.Clean(path.HostPath))
-			if path.EnvOverride != "" {
-				if override := os.Getenv(path.EnvOverride); override != "" {
-					if !filepath.IsAbs(override) {
-						override = filepath.Join(home, override)
-					}
-					hostPath = filepath.Clean(override)
-				}
-			}
-			if !exists(hostPath) {
-				report.Missing = appendUnique(report.Missing, hostPath)
-				continue
-			}
-			resolvedHost, err := filepath.EvalSymlinks(hostPath)
-			if err != nil {
-				report.Refused = appendUnique(report.Refused, hostPath+" (unresolvable symlink)")
-				continue
-			}
-			if filepath.Clean(resolvedHost) == filepath.Clean(resolvedHome) {
-				report.Refused = appendUnique(report.Refused, hostPath+" (config path resolves to the home directory)")
-				continue
-			}
-			if !IsWithinHome(resolvedHost, resolvedHome) {
-				report.Refused = appendUnique(report.Refused, hostPath+" (resolves outside home)")
-				continue
-			}
-			if len(path.MaskPaths) > 0 {
-				if err := validateMaskedConfigTree(resolvedHost, path.MaskPaths); err != nil {
-					report.Refused = appendUnique(report.Refused, hostPath+" ("+err.Error()+")")
-					continue
-				}
-				key := resolvedHost + "\x00" + filepath.Clean(path.ContainerPath)
-				if _, ok := seen[key]; !ok {
-					seen[key] = struct{}{}
-					report.Mounts = append(report.Mounts, ResolvedMount{
-						Agent: entry.Name, Name: filepath.Clean(path.HostPath), HostPath: resolvedHost,
-						DestPath: filepath.ToSlash(filepath.Join("/home/caged", filepath.Clean(path.ContainerPath))),
-						Writable: path.Writable, MaskPaths: append([]string(nil), path.MaskPaths...),
-					})
-				}
-				continue
-			}
-			mounts, refused := collectSafeMounts(resolvedHost)
-			for _, reason := range refused {
-				report.Refused = appendUnique(report.Refused, reason)
-			}
-			for _, mountPath := range mounts {
-				relative, err := filepath.Rel(resolvedHost, mountPath)
-				if err != nil {
-					report.Refused = appendUnique(report.Refused, hostPath+" (cannot map safe config path)")
-					continue
-				}
-				destPath := filepath.Clean(path.ContainerPath)
-				if relative != "." {
-					destPath = filepath.Join(destPath, relative)
-				}
-				key := mountPath + "\x00" + destPath
-				if _, ok := seen[key]; ok {
-					continue
-				}
-				seen[key] = struct{}{}
-				report.Mounts = append(report.Mounts, ResolvedMount{
-					Agent:    entry.Name,
-					Name:     filepath.Clean(path.HostPath),
-					HostPath: mountPath,
-					DestPath: filepath.ToSlash(filepath.Join("/home/caged", destPath)),
-					Writable: path.Writable,
-				})
-			}
+	pass := &resolution{fs: r.fs, home: home, resolvedHome: resolvedHome, seen: make(map[string]struct{})}
+	for _, definition := range r.agents {
+		for _, path := range definition.ConfigPaths {
+			pass.agentPath(definition.Name, path, r.hostPath(home, path))
 		}
 	}
-	// Git identity is shared across agents rather than duplicated in each entry.
-	sharedGitConfig := filepath.Join(home, ".gitconfig")
-	if exists(sharedGitConfig) {
-		resolved, resolveErr := filepath.EvalSymlinks(sharedGitConfig)
-		if resolveErr == nil && IsWithinHome(resolved, resolvedHome) {
-			if info, statErr := os.Stat(resolved); statErr == nil && info.Mode().IsRegular() {
-				report.Mounts = append(report.Mounts, ResolvedMount{Agent: "shared", Name: ".gitconfig", HostPath: resolved, DestPath: "/home/caged/.gitconfig", Writable: false})
-			} else {
-				report.Refused = appendUnique(report.Refused, sharedGitConfig+" (shared git config must be a regular file)")
-			}
-		} else {
-			report.Refused = appendUnique(report.Refused, sharedGitConfig+" (resolves outside home or cannot be resolved)")
-		}
-	} else {
-		report.Missing = appendUnique(report.Missing, sharedGitConfig)
+	for _, file := range r.shared {
+		pass.sharedFile(file)
 	}
-	return report
+	return pass.report
 }
 
-func validateMaskedConfigTree(root string, masks []string) error {
-	rootInfo, err := os.Stat(root)
-	if err != nil {
-		return err
+// hostPath is where a config path lives on the host. A set override variable
+// (absolute, or relative to home) replaces the default location.
+func (r *Resolver) hostPath(home string, path agent.ConfigPath) string {
+	hostPath := filepath.Join(home, filepath.Clean(path.HostPath))
+	if path.EnvOverride == "" {
+		return hostPath
 	}
-	if !rootInfo.IsDir() {
-		return fmt.Errorf("masked config root must be a directory")
+	override, _ := r.env.LookupEnv(path.EnvOverride)
+	if override == "" {
+		return hostPath
 	}
-	maskPaths := make([]string, 0, len(masks))
-	for _, mask := range masks {
-		clean := filepath.Clean(mask)
-		if filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
-			return fmt.Errorf("invalid config mask path %q", mask)
-		}
-		maskPaths = append(maskPaths, filepath.Join(root, clean))
+	if !filepath.IsAbs(override) {
+		override = filepath.Join(home, override)
 	}
-	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
-		if walkErr != nil {
-			return walkErr
-		}
-		if path != root {
-			for _, mask := range maskPaths {
-				if path == mask || IsWithinHome(path, mask) {
-					if entry.IsDir() {
-						return filepath.SkipDir
-					}
-					return nil
-				}
-			}
-		}
-		info, err := os.Lstat(path)
-		if err != nil {
-			return err
-		}
-		if info.Mode()&os.ModeSymlink != 0 {
-			resolved, err := filepath.EvalSymlinks(path)
-			if err != nil || !IsWithinHome(resolved, root) {
-				return fmt.Errorf("symlink %q resolves outside the config directory or cannot be resolved", path)
-			}
-			return nil
-		}
-		if !info.IsDir() && !info.Mode().IsRegular() {
-			return fmt.Errorf("special file %q is not covered by a config mask", path)
-		}
-		return nil
-	})
+	return filepath.Clean(override)
 }
 
-func collectSafeMounts(root string) ([]string, []string) {
-	var mounts, refused []string
-	var collect func(string)
-	collect = func(path string) {
-		entries, err := os.ReadDir(path)
-		if err != nil {
-			info, statErr := os.Lstat(path)
-			if statErr == nil && info.Mode().IsRegular() {
-				mounts = append(mounts, path)
-			} else {
-				refused = append(refused, path+" (cannot inspect config entry safely)")
-			}
-			return
-		}
-		if subtreeIsSafe(path) {
-			mounts = append(mounts, path)
-			return
-		}
-		for _, entry := range entries {
-			child := filepath.Join(path, entry.Name())
-			info, err := os.Lstat(child)
-			if err != nil {
-				refused = append(refused, child+" (cannot inspect config entry safely)")
-				continue
-			}
-			if info.IsDir() {
-				collect(child)
-			} else if info.Mode().IsRegular() {
-				mounts = append(mounts, child)
-			} else {
-				refused = append(refused, child+" (symlinks and special files are excluded from config mounts)")
-			}
-		}
+// resolution accumulates the report of one Resolve call.
+type resolution struct {
+	fs           FileSystem
+	home         string
+	resolvedHome string
+	seen         map[string]struct{} // host path + "\x00" + container path already mounted
+	report       plan.ConfigReport
+}
+
+// agentPath checks one agent config path and records its mounts, or why it
+// is missing or refused.
+func (r *resolution) agentPath(agentName string, path agent.ConfigPath, hostPath string) {
+	if !r.exists(hostPath) {
+		r.report.Missing = appendUnique(r.report.Missing, hostPath)
+		return
 	}
-	info, err := os.Lstat(root)
+	resolvedHost, err := r.fs.EvalSymlinks(hostPath)
 	if err != nil {
-		return nil, []string{root + " (cannot inspect config root safely)"}
-	}
-	if info.Mode().IsRegular() {
-		return []string{root}, nil
+		r.refuse(hostPath + " (unresolvable symlink)")
+		return
 	}
-	if !info.IsDir() {
-		return nil, []string{root + " (config root is not a regular file or directory)"}
+	if filepath.Clean(resolvedHost) == filepath.Clean(r.resolvedHome) {
+		r.refuse(hostPath + " (config path resolves to the home directory)")
+		return
+	}
+	if !isWithin(resolvedHost, r.resolvedHome) {
+		r.refuse(hostPath + " (resolves outside home)")
+		return
+	}
+	name, dest := filepath.Clean(path.HostPath), filepath.Clean(path.ContainerPath)
+	if len(path.MaskPaths) > 0 {
+		// A masked tree is mounted whole; tmpfs mounts hide the masked entries.
+		if err := validateMaskedConfigTree(r.fs, resolvedHost, path.MaskPaths); err != nil {
+			r.refuse(hostPath + " (" + err.Error() + ")")
+			return
+		}
+		r.addMount(agentName, name, resolvedHost, dest, path.Writable, append([]string(nil), path.MaskPaths...))
+		return
 	}
-	collect(root)
-	return mounts, refused
-}
-
-func subtreeIsSafe(root string) bool {
-	entries, err := os.ReadDir(root)
-	if err != nil {
-		return false
+	mounts, refused := collectSafeMounts(r.fs, resolvedHost)
+	for _, reason := range refused {
+		r.refuse(reason)
 	}
-	for _, entry := range entries {
-		path := filepath.Join(root, entry.Name())
-		info, err := os.Lstat(path)
+	for _, mountPath := range mounts {
+		relative, err := filepath.Rel(resolvedHost, mountPath)
 		if err != nil {
-			return false
-		}
-		if info.Mode()&os.ModeSymlink != 0 {
-			return false
-		}
-		if info.IsDir() {
-			if !subtreeIsSafe(path) {
-				return false
-			}
-		} else if !info.Mode().IsRegular() {
-			return false
+			r.refuse(hostPath + " (cannot map safe config path)")
+			continue
 		}
+		mountDest := dest
+		if relative != "." {
+			mountDest = filepath.Join(dest, relative)
+		}
+		r.addMount(agentName, name, mountPath, mountDest, path.Writable, nil)
 	}
-	return true
 }
 
-func appendUnique(values []string, value string) []string {
-	for _, existing := range values {
-		if existing == value {
-			return values
-		}
-	}
-	return append(values, value)
+// sharedFile checks one shared file. It must resolve inside home and be a
+// regular file, and it is always mounted read-only.
+func (r *resolution) sharedFile(file SharedFile) {
+	name := filepath.Clean(file.Path)
+	hostPath := filepath.Join(r.home, name)
+	if !r.exists(hostPath) {
+		r.report.Missing = appendUnique(r.report.Missing, hostPath)
+		return
+	}
+	resolved, err := r.fs.EvalSymlinks(hostPath)
+	if err != nil || !isWithin(resolved, r.resolvedHome) {
+		r.refuse(hostPath + " (resolves outside home or cannot be resolved)")
+		return
+	}
+	if info, err := r.fs.Stat(resolved); err != nil || !info.Mode().IsRegular() {
+		r.refuse(hostPath + " (" + file.Description + " must be a regular file)")
+		return
+	}
+	r.report.Mounts = append(r.report.Mounts, plan.ConfigMount{
+		Agent:    "shared",
+		Name:     name,
+		HostPath: resolved,
+		DestPath: filepath.ToSlash(filepath.Join(containerHome, name)),
+		Writable: false,
+	})
 }
 
-func IsWithinHome(path, home string) bool {
-	path = filepath.Clean(path)
-	home = filepath.Clean(home)
-	rel, err := filepath.Rel(home, path)
-	if err != nil {
-		return false
-	}
-	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
+// addMount records a mount unless the same host path is already mounted at
+// the same container path.
+func (r *resolution) addMount(agentName, name, hostPath, dest string, writable bool, masks []string) {
+	key := hostPath + "\x00" + dest
+	if _, ok := r.seen[key]; ok {
+		return
+	}
+	r.seen[key] = struct{}{}
+	r.report.Mounts = append(r.report.Mounts, plan.ConfigMount{
+		Agent:     agentName,
+		Name:      name,
+		HostPath:  hostPath,
+		DestPath:  filepath.ToSlash(filepath.Join(containerHome, dest)),
+		Writable:  writable,
+		MaskPaths: masks,
+	})
+}
+
+func (r *resolution) exists(path string) bool {
+	_, err := r.fs.Stat(path)
+	return err == nil
+}
+
+func (r *resolution) refuse(reason string) {
+	r.report.Refused = appendUnique(r.report.Refused, reason)
 }
````

#### File 3 of 6: `internal/hostconfig/resolver_edges_test.go`

This is a NEW FILE (264 lines).

Imports: this new file has these import lines: `"errors"`, `"io/fs"`, `"os"`, `"path/filepath"`, `"reflect"`, `"strings"`, `"testing"`, `"github.com/JeremyOrr4/caged/internal/agent"`, `"github.com/JeremyOrr4/caged/internal/plan"`, `"github.com/JeremyOrr4/caged/internal/system"`. (The file content below shows how they are grouped.)

Create the file `internal/hostconfig/resolver_edges_test.go` with exactly this content:

````go
package hostconfig

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
)

// faultyFS is the real filesystem with selected calls failing.
type faultyFS struct {
	system.FS
	evalErr    map[string]bool
	lstatErr   map[string]bool
	readDirErr map[string]bool
}

var errInjected = errors.New("injected failure")

func (f faultyFS) EvalSymlinks(path string) (string, error) {
	if f.evalErr[path] {
		return "", errInjected
	}
	return f.FS.EvalSymlinks(path)
}

func (f faultyFS) Lstat(name string) (fs.FileInfo, error) {
	if f.lstatErr[name] {
		return nil, errInjected
	}
	return f.FS.Lstat(name)
}

func (f faultyFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if f.readDirErr[name] {
		return nil, errInjected
	}
	return f.FS.ReadDir(name)
}

// testHome creates a home directory and returns it with its resolved path
// (they differ on macOS, where temp dirs live behind the /var -> /private/var symlink).
func testHome(t *testing.T, dirs, files []string) (string, string) {
	t.Helper()
	home := t.TempDir()
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(home, dir), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range files {
		if err := os.WriteFile(filepath.Join(home, file), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	resolved, err := filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatal(err)
	}
	return home, resolved
}

func TestResolveRefusesUnresolvableHome(t *testing.T) {
	t.Parallel()

	fsys := faultyFS{evalErr: map[string]bool{"/no/such/home": true}}
	got := NewResolver(fsys, fakeEnv{}, agent.Builtin().All(), DefaultSharedFiles()).Resolve("/no/such/home/")
	want := plan.ConfigReport{Refused: []string{"/no/such/home (home cannot be resolved)"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Resolve() = %+v, want %+v", got, want)
	}
}

func TestResolveReportsMissingPathsInRegistryOrder(t *testing.T) {
	t.Parallel()

	// The six original agents, so that adding a built-in agent does not change this list.
	var agents []agent.Agent
	for _, name := range []string{"claude", "codex", "copilot", "gemini", "gh", "pi"} {
		definition, err := agent.Builtin().Lookup(name)
		if err != nil {
			t.Fatal(err)
		}
		agents = append(agents, definition)
	}
	home, _ := testHome(t, nil, nil)
	report := NewResolver(system.FS{}, fakeEnv{}, agents, DefaultSharedFiles()).Resolve(home)
	var want []string
	for _, rel := range []string{".claude", ".codex", ".copilot", ".gemini", ".config/gh", ".config/pi", ".pi", ".gitconfig"} {
		want = append(want, filepath.Join(home, rel))
	}
	if !reflect.DeepEqual(report.Missing, want) || report.Mounts != nil || report.Refused != nil {
		t.Fatalf("Resolve() = %+v, want only Missing %v", report, want)
	}
}

func TestResolveHonorsOverrides(t *testing.T) {
	t.Parallel()

	home, resolvedHome := testHome(t, []string{"alt-codex", "abs-copilot", ".claude"}, nil)
	report := resolveConfigs(home, fakeEnv{
		"CODEX_HOME":        "alt-codex",                        // relative to home
		"COPILOT_HOME":      filepath.Join(home, "abs-copilot"), // absolute
		"CLAUDE_CONFIG_DIR": "",                                 // empty means "use the default"
	})
	got := map[string]string{}
	for _, mount := range report.Mounts {
		got[mount.HostPath] = mount.DestPath
	}
	want := map[string]string{
		filepath.Join(resolvedHome, "alt-codex"):   "/home/caged/.codex",
		filepath.Join(resolvedHome, "abs-copilot"): "/home/caged/.copilot",
		filepath.Join(resolvedHome, ".claude"):     "/home/caged/.claude",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mounts = %v, want %v", got, want)
	}
}

func TestResolveRefusesUnresolvableConfigPath(t *testing.T) {
	t.Parallel()

	home, _ := testHome(t, []string{".gemini"}, nil)
	gemini := filepath.Join(home, ".gemini")
	fsys := faultyFS{evalErr: map[string]bool{gemini: true}}
	report := NewResolver(fsys, fakeEnv{}, agent.Builtin().All(), nil).Resolve(home)
	if want := []string{gemini + " (unresolvable symlink)"}; !reflect.DeepEqual(report.Refused, want) {
		t.Fatalf("Refused = %v, want %v", report.Refused, want)
	}
}

func TestResolveReportsUninspectableEntries(t *testing.T) {
	t.Parallel()

	home, resolvedHome := testHome(t, []string{".gemini/sub", ".gh"}, []string{".gemini/sub/a.json", ".gemini/b.json"})
	if err := os.Symlink("b.json", filepath.Join(home, ".gemini", "link")); err != nil {
		t.Fatal(err)
	}
	gemini := filepath.Join(resolvedHome, ".gemini")
	fsys := faultyFS{
		readDirErr: map[string]bool{filepath.Join(gemini, "sub"): true},
		lstatErr:   map[string]bool{filepath.Join(gemini, "b.json"): true},
	}
	report := NewResolver(fsys, fakeEnv{}, agent.Builtin().All(), nil).Resolve(home)
	wantRefused := []string{
		filepath.Join(gemini, "b.json") + " (cannot inspect config entry safely)",
		filepath.Join(gemini, "link") + " (symlinks and special files are excluded from config mounts)",
		filepath.Join(gemini, "sub") + " (cannot inspect config entry safely)",
	}
	if !reflect.DeepEqual(report.Refused, wantRefused) || len(report.Mounts) != 0 {
		t.Fatalf("Resolve() = %+v, want refused %v and no mounts", report, wantRefused)
	}

	root := filepath.Join(resolvedHome, ".gh")
	if mounts, refused := collectSafeMounts(faultyFS{lstatErr: map[string]bool{root: true}}, root); mounts != nil || !reflect.DeepEqual(refused, []string{root + " (cannot inspect config root safely)"}) {
		t.Fatalf("collectSafeMounts() = %v, %v", mounts, refused)
	}
}

func TestResolveMountsSingleFileConfigAndRejectsBadMasks(t *testing.T) {
	t.Parallel()

	home, resolvedHome := testHome(t, []string{"masked"}, []string{"tool.json", "masked-file"})
	agents := []agent.Agent{
		{Name: "file", Command: []string{"file"}, ConfigPaths: []agent.ConfigPath{{HostPath: "tool.json", ContainerPath: "tool.json"}}},
		{Name: "escape", Command: []string{"escape"}, ConfigPaths: []agent.ConfigPath{{HostPath: "masked", ContainerPath: "masked", MaskPaths: []string{"../outside"}}}},
		{Name: "notdir", Command: []string{"notdir"}, ConfigPaths: []agent.ConfigPath{{HostPath: "masked-file", ContainerPath: "masked-file", MaskPaths: []string{"tmp"}}}},
	}
	report := NewResolver(system.FS{}, fakeEnv{}, agents, nil).Resolve(home)
	wantMounts := []plan.ConfigMount{{Agent: "file", Name: "tool.json", HostPath: filepath.Join(resolvedHome, "tool.json"), DestPath: "/home/caged/tool.json"}}
	if !reflect.DeepEqual(report.Mounts, wantMounts) {
		t.Fatalf("Mounts = %+v, want %+v", report.Mounts, wantMounts)
	}
	wantRefused := []string{
		filepath.Join(home, "masked") + ` (invalid config mask path "../outside")`,
		filepath.Join(home, "masked-file") + " (masked config root must be a directory)",
	}
	if !reflect.DeepEqual(report.Refused, wantRefused) {
		t.Fatalf("Refused = %v, want %v", report.Refused, wantRefused)
	}
}

func TestResolveDeduplicatesSharedConfigPaths(t *testing.T) {
	t.Parallel()

	home, _ := testHome(t, []string{".shared"}, nil)
	path := agent.ConfigPath{HostPath: ".shared", ContainerPath: ".shared", Writable: true}
	agents := []agent.Agent{
		{Name: "one", Command: []string{"one"}, ConfigPaths: []agent.ConfigPath{path}},
		{Name: "two", Command: []string{"two"}, ConfigPaths: []agent.ConfigPath{path}},
	}
	report := NewResolver(system.FS{}, fakeEnv{}, agents, nil).Resolve(home)
	if len(report.Mounts) != 1 || report.Mounts[0].Agent != "one" {
		t.Fatalf("Mounts = %+v, want one mount owned by the first agent", report.Mounts)
	}
}

func TestSharedFilesAreData(t *testing.T) {
	t.Parallel()

	if got, want := DefaultSharedFiles(), []SharedFile{{Path: ".gitconfig", Description: "shared git config"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("DefaultSharedFiles() = %v, want %v", got, want)
	}

	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	home, resolvedHome := testHome(t, []string{".npmrc-dir"}, []string{".npmrc"})
	if err := os.Symlink(filepath.Join(outside, "secret"), filepath.Join(home, ".escape")); err != nil {
		t.Fatal(err)
	}
	shared := []SharedFile{
		{Path: ".npmrc", Description: "shared npm config"},
		{Path: ".npmrc-dir", Description: "shared npm config"},
		{Path: ".escape", Description: "shared escape"},
		{Path: ".absent", Description: "shared absent"},
	}
	report := NewResolver(system.FS{}, fakeEnv{}, nil, shared).Resolve(home)
	wantMounts := []plan.ConfigMount{{Agent: "shared", Name: ".npmrc", HostPath: filepath.Join(resolvedHome, ".npmrc"), DestPath: "/home/caged/.npmrc"}}
	if !reflect.DeepEqual(report.Mounts, wantMounts) {
		t.Fatalf("Mounts = %+v, want %+v", report.Mounts, wantMounts)
	}
	wantRefused := []string{
		filepath.Join(home, ".npmrc-dir") + " (shared npm config must be a regular file)",
		filepath.Join(home, ".escape") + " (resolves outside home or cannot be resolved)",
	}
	if !reflect.DeepEqual(report.Refused, wantRefused) {
		t.Fatalf("Refused = %v, want %v", report.Refused, wantRefused)
	}
	if want := []string{filepath.Join(home, ".absent")}; !reflect.DeepEqual(report.Missing, want) {
		t.Fatalf("Missing = %v, want %v", report.Missing, want)
	}
}

func TestIsWithin(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		path, root string
		want       bool
	}{
		{"/Users/me", "/Users/me", true},
		{"/Users/me/.codex", "/Users/me", true},
		{"/Users/me/../other", "/Users/me", false},
		{"/Users/meow", "/Users/me", false},
		{"/Users", "/Users/me", false},
		{"/Users/me/..hidden", "/Users/me", true},
	} {
		if got := isWithin(tt.path, tt.root); got != tt.want {
			t.Errorf("isWithin(%q, %q) = %v, want %v", tt.path, tt.root, got, tt.want)
		}
	}
	if strings.Contains(containerHome, "\\") {
		t.Fatal("container paths must use forward slashes")
	}
}
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 4 of 6: `internal/hostconfig/resolver_test.go`

Imports: added `"github.com/JeremyOrr4/caged/internal/agent"`, `"github.com/JeremyOrr4/caged/internal/plan"`, `"github.com/JeremyOrr4/caged/internal/system"`. (The edits below already contain these import changes.)

There are 11 edits in this file. They are listed from the bottom of the file to the top. Do them in the order shown: each edit only changes lines below the next one, so the line numbers stay correct. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 11: lines 218-228.**

Find this exact text (it appears exactly once in the file):

````go
}

func TestSharedGitConfigMustBeRegularFile(t *testing.T) {
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, ".gitconfig"), 0o700); err != nil {
		t.Fatal(err)
	}
	report := ResolveAllConfigsReport(home, nil)
	for _, mount := range report.Mounts {
		if mount.Name == ".gitconfig" {
			t.Fatal("directory mounted as shared git config")
````

Replace it with this exact text:

````go
}

func TestSharedGitConfigMustBeRegularFile(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, ".gitconfig"), 0o700); err != nil {
		t.Fatal(err)
	}
	report := resolveConfigs(home, nil)
	for _, mount := range report.Mounts {
		if mount.Name == ".gitconfig" {
			t.Fatal("directory mounted as shared git config")
````

**Edit 2 of 11: lines 187-197.**

Find this exact text (it appears exactly once in the file):

````go
	if err := os.Symlink("/opt/homebrew/bin/codex", filepath.Join(codexHome, "packages/app-server-daemon/current")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CODEX_HOME", "")
	t.Setenv("COPILOT_HOME", "")
	report := ResolveAllConfigsReport(home, nil)
	var codexMount *ResolvedMount
	for _, mount := range report.Mounts {
		if mount.Agent == "codex" {
			codexMount = &mount
````

Replace it with this exact text:

````go
	if err := os.Symlink("/opt/homebrew/bin/codex", filepath.Join(codexHome, "packages/app-server-daemon/current")); err != nil {
		t.Fatal(err)
	}
	report := resolveConfigs(home, nil)
	var codexMount *plan.ConfigMount
	for _, mount := range report.Mounts {
		if mount.Agent == "codex" {
			codexMount = &mount
````

**Edit 3 of 11: lines 166-171.**

Find this exact text (it appears exactly once in the file):

````go
}

func TestCodexHomeMountPreservesDirectoryAndMasksRuntimeEntries(t *testing.T) {
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	for _, dir := range []string{"app-server-control", "app-server-daemon", "ipc", "packages/app-server-daemon", "tmp/arg0"} {
````

Replace it with this exact text:

````go
}

func TestCodexHomeMountPreservesDirectoryAndMasksRuntimeEntries(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	for _, dir := range []string{"app-server-control", "app-server-daemon", "ipc", "packages/app-server-daemon", "tmp/arg0"} {
````

**Edit 4 of 11: lines 151-160.**

Find this exact text (it appears exactly once in the file):

````go
	if err := syscall.Mkfifo(filepath.Join(configDir, "agent.pipe"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CODEX_HOME", "")
	t.Setenv("COPILOT_HOME", "")
	report := ResolveAllConfigsReport(home, nil)
	for _, mount := range report.Mounts {
		if mount.Name == ".gemini" {
			t.Fatal("config tree containing a socket was mounted")
````

Replace it with this exact text:

````go
	if err := syscall.Mkfifo(filepath.Join(configDir, "agent.pipe"), 0o600); err != nil {
		t.Fatal(err)
	}
	report := resolveConfigs(home, nil)
	for _, mount := range report.Mounts {
		if mount.Name == ".gemini" {
			t.Fatal("config tree containing a socket was mounted")
````

**Edit 5 of 11: lines 143-148.**

Find this exact text (it appears exactly once in the file):

````go
}

func TestSpecialFileInsideConfigTreeIsRefused(t *testing.T) {
	home := t.TempDir()
	configDir := filepath.Join(home, ".gemini")
	if err := os.Mkdir(configDir, 0o700); err != nil {
````

Replace it with this exact text:

````go
}

func TestSpecialFileInsideConfigTreeIsRefused(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	configDir := filepath.Join(home, ".gemini")
	if err := os.Mkdir(configDir, 0o700); err != nil {
````

**Edit 6 of 11: lines 128-137.**

Find this exact text (it appears exactly once in the file):

````go
	if err := os.Symlink(privateDir, filepath.Join(configDir, "private-link")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CODEX_HOME", "")
	t.Setenv("COPILOT_HOME", "")
	report := ResolveAllConfigsReport(home, nil)
	for _, mount := range report.Mounts {
		if mount.Name == ".codex" {
			t.Fatal("config tree with a symlink outside the config directory was mounted")
````

Replace it with this exact text:

````go
	if err := os.Symlink(privateDir, filepath.Join(configDir, "private-link")); err != nil {
		t.Fatal(err)
	}
	report := resolveConfigs(home, nil)
	for _, mount := range report.Mounts {
		if mount.Name == ".codex" {
			t.Fatal("config tree with a symlink outside the config directory was mounted")
````

**Edit 7 of 11: lines 113-118.**

Find this exact text (it appears exactly once in the file):

````go
}

func TestConfigSymlinkOutsideConfigDirectoryIsRefused(t *testing.T) {
	home := t.TempDir()
	configDir := filepath.Join(home, ".codex")
	privateDir := filepath.Join(home, "private")
````

Replace it with this exact text:

````go
}

func TestConfigSymlinkOutsideConfigDirectoryIsRefused(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	configDir := filepath.Join(home, ".codex")
	privateDir := filepath.Join(home, "private")
````

**Edit 8 of 11: lines 91-107.**

Find this exact text (it appears exactly once in the file):

````go
}

func TestConfigSymlinkOutsideHomeIsRefused(t *testing.T) {
	home, outside := t.TempDir(), t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CODEX_HOME", "")
	t.Setenv("COPILOT_HOME", "")
	if err := os.WriteFile(filepath.Join(outside, "secret"), []byte("no"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(home, ".codex")); err != nil {
		t.Fatal(err)
	}
	report := ResolveAllConfigsReport(home, nil)
	for _, mount := range report.Mounts {
		if filepath.Clean(mount.HostPath) == filepath.Clean(outside) {
			t.Fatalf("mounted config symlink target outside home: %+v", mount)
````

Replace it with this exact text:

````go
}

func TestConfigSymlinkOutsideHomeIsRefused(t *testing.T) {
	t.Parallel()
	home, outside := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret"), []byte("no"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(home, ".codex")); err != nil {
		t.Fatal(err)
	}
	report := resolveConfigs(home, nil)
	for _, mount := range report.Mounts {
		if filepath.Clean(mount.HostPath) == filepath.Clean(outside) {
			t.Fatalf("mounted config symlink target outside home: %+v", mount)
````

**Edit 9 of 11: lines 62-79.**

Find this exact text (it appears exactly once in the file):

````go
	}

	missingHome := t.TempDir()
	if got := ResolveAllConfigsReport(missingHome, exists).Mounts; len(got) != 0 {
		t.Errorf("ResolveAllConfigsReport() with no local configs = %v, want none", got)
	}
}

func TestConfigOverrideCannotMountHomeDirectory(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_HOME", home)
	report := ResolveAllConfigsReport(home, nil)
	for _, mount := range report.Mounts {
		if filepath.Clean(mount.HostPath) == filepath.Clean(home) {
			t.Fatal("agent config override mounted the entire home directory")
````

Replace it with this exact text:

````go
	}

	missingHome := t.TempDir()
	if got := resolveConfigs(missingHome, nil).Mounts; len(got) != 0 {
		t.Errorf("Resolve() with no local configs = %v, want none", got)
	}
}

func TestConfigOverrideCannotMountHomeDirectory(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o700); err != nil {
		t.Fatal(err)
	}
	report := resolveConfigs(home, fakeEnv{"CODEX_HOME": home})
	for _, mount := range report.Mounts {
		if filepath.Clean(mount.HostPath) == filepath.Clean(home) {
			t.Fatal("agent config override mounted the entire home directory")
````

**Edit 10 of 11: lines 36-47.**

Find this exact text (it appears exactly once in the file):

````go
		}
	}

	exists := func(path string) bool {
		_, err := os.Stat(path)
		return err == nil
	}

	report := ResolveAllConfigsReport(home, exists)
	defaultConfigs := report.Mounts
	defaultNames := make(map[string]struct{}, len(defaultConfigs))
	for _, mount := range defaultConfigs {
````

Replace it with this exact text:

````go
		}
	}

	report := resolveConfigs(home, nil)
	defaultConfigs := report.Mounts
	defaultNames := make(map[string]struct{}, len(defaultConfigs))
	for _, mount := range defaultConfigs {
````

**Edit 11 of 11: lines 7-14.**

Find this exact text (it appears exactly once in the file):

````go
	"strings"
	"syscall"
	"testing"
)

func TestResolveAllConfigsMountsExistingConfigReadWrite(t *testing.T) {
	t.Parallel()

````

Replace it with this exact text:

````go
	"strings"
	"syscall"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
)

// fakeEnv is a fixed environment for config-directory overrides.
type fakeEnv map[string]string

func (e fakeEnv) LookupEnv(key string) (string, bool) {
	value, ok := e[key]
	return value, ok
}

// resolveConfigs runs the production resolver on the real filesystem with the
// built-in agents and a controlled environment.
func resolveConfigs(home string, env fakeEnv) plan.ConfigReport {
	return NewResolver(system.FS{}, env, agent.Builtin().All(), DefaultSharedFiles()).Resolve(home)
}

func TestResolveAllConfigsMountsExistingConfigReadWrite(t *testing.T) {
	t.Parallel()

````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/hostconfig/resolver_test.go
+++ b/internal/hostconfig/resolver_test.go
@@ -7,8 +7,26 @@
 	"strings"
 	"syscall"
 	"testing"
+
+	"github.com/JeremyOrr4/caged/internal/agent"
+	"github.com/JeremyOrr4/caged/internal/plan"
+	"github.com/JeremyOrr4/caged/internal/system"
 )
 
+// fakeEnv is a fixed environment for config-directory overrides.
+type fakeEnv map[string]string
+
+func (e fakeEnv) LookupEnv(key string) (string, bool) {
+	value, ok := e[key]
+	return value, ok
+}
+
+// resolveConfigs runs the production resolver on the real filesystem with the
+// built-in agents and a controlled environment.
+func resolveConfigs(home string, env fakeEnv) plan.ConfigReport {
+	return NewResolver(system.FS{}, env, agent.Builtin().All(), DefaultSharedFiles()).Resolve(home)
+}
+
 func TestResolveAllConfigsMountsExistingConfigReadWrite(t *testing.T) {
 	t.Parallel()
 
@@ -36,12 +54,7 @@
 		}
 	}
 
-	exists := func(path string) bool {
-		_, err := os.Stat(path)
-		return err == nil
-	}
-
-	report := ResolveAllConfigsReport(home, exists)
+	report := resolveConfigs(home, nil)
 	defaultConfigs := report.Mounts
 	defaultNames := make(map[string]struct{}, len(defaultConfigs))
 	for _, mount := range defaultConfigs {
@@ -62,18 +75,18 @@
 	}
 
 	missingHome := t.TempDir()
-	if got := ResolveAllConfigsReport(missingHome, exists).Mounts; len(got) != 0 {
-		t.Errorf("ResolveAllConfigsReport() with no local configs = %v, want none", got)
+	if got := resolveConfigs(missingHome, nil).Mounts; len(got) != 0 {
+		t.Errorf("Resolve() with no local configs = %v, want none", got)
 	}
 }
 
 func TestConfigOverrideCannotMountHomeDirectory(t *testing.T) {
+	t.Parallel()
 	home := t.TempDir()
 	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o700); err != nil {
 		t.Fatal(err)
 	}
-	t.Setenv("CODEX_HOME", home)
-	report := ResolveAllConfigsReport(home, nil)
+	report := resolveConfigs(home, fakeEnv{"CODEX_HOME": home})
 	for _, mount := range report.Mounts {
 		if filepath.Clean(mount.HostPath) == filepath.Clean(home) {
 			t.Fatal("agent config override mounted the entire home directory")
@@ -91,17 +104,15 @@
 }
 
 func TestConfigSymlinkOutsideHomeIsRefused(t *testing.T) {
+	t.Parallel()
 	home, outside := t.TempDir(), t.TempDir()
-	t.Setenv("CLAUDE_CONFIG_DIR", "")
-	t.Setenv("CODEX_HOME", "")
-	t.Setenv("COPILOT_HOME", "")
 	if err := os.WriteFile(filepath.Join(outside, "secret"), []byte("no"), 0o600); err != nil {
 		t.Fatal(err)
 	}
 	if err := os.Symlink(outside, filepath.Join(home, ".codex")); err != nil {
 		t.Fatal(err)
 	}
-	report := ResolveAllConfigsReport(home, nil)
+	report := resolveConfigs(home, nil)
 	for _, mount := range report.Mounts {
 		if filepath.Clean(mount.HostPath) == filepath.Clean(outside) {
 			t.Fatalf("mounted config symlink target outside home: %+v", mount)
@@ -113,6 +124,7 @@
 }
 
 func TestConfigSymlinkOutsideConfigDirectoryIsRefused(t *testing.T) {
+	t.Parallel()
 	home := t.TempDir()
 	configDir := filepath.Join(home, ".codex")
 	privateDir := filepath.Join(home, "private")
@@ -128,10 +140,7 @@
 	if err := os.Symlink(privateDir, filepath.Join(configDir, "private-link")); err != nil {
 		t.Fatal(err)
 	}
-	t.Setenv("CLAUDE_CONFIG_DIR", "")
-	t.Setenv("CODEX_HOME", "")
-	t.Setenv("COPILOT_HOME", "")
-	report := ResolveAllConfigsReport(home, nil)
+	report := resolveConfigs(home, nil)
 	for _, mount := range report.Mounts {
 		if mount.Name == ".codex" {
 			t.Fatal("config tree with a symlink outside the config directory was mounted")
@@ -143,6 +152,7 @@
 }
 
 func TestSpecialFileInsideConfigTreeIsRefused(t *testing.T) {
+	t.Parallel()
 	home := t.TempDir()
 	configDir := filepath.Join(home, ".gemini")
 	if err := os.Mkdir(configDir, 0o700); err != nil {
@@ -151,10 +161,7 @@
 	if err := syscall.Mkfifo(filepath.Join(configDir, "agent.pipe"), 0o600); err != nil {
 		t.Fatal(err)
 	}
-	t.Setenv("CLAUDE_CONFIG_DIR", "")
-	t.Setenv("CODEX_HOME", "")
-	t.Setenv("COPILOT_HOME", "")
-	report := ResolveAllConfigsReport(home, nil)
+	report := resolveConfigs(home, nil)
 	for _, mount := range report.Mounts {
 		if mount.Name == ".gemini" {
 			t.Fatal("config tree containing a socket was mounted")
@@ -166,6 +173,7 @@
 }
 
 func TestCodexHomeMountPreservesDirectoryAndMasksRuntimeEntries(t *testing.T) {
+	t.Parallel()
 	home := t.TempDir()
 	codexHome := filepath.Join(home, ".codex")
 	for _, dir := range []string{"app-server-control", "app-server-daemon", "ipc", "packages/app-server-daemon", "tmp/arg0"} {
@@ -187,11 +195,8 @@
 	if err := os.Symlink("/opt/homebrew/bin/codex", filepath.Join(codexHome, "packages/app-server-daemon/current")); err != nil {
 		t.Fatal(err)
 	}
-	t.Setenv("CLAUDE_CONFIG_DIR", "")
-	t.Setenv("CODEX_HOME", "")
-	t.Setenv("COPILOT_HOME", "")
-	report := ResolveAllConfigsReport(home, nil)
-	var codexMount *ResolvedMount
+	report := resolveConfigs(home, nil)
+	var codexMount *plan.ConfigMount
 	for _, mount := range report.Mounts {
 		if mount.Agent == "codex" {
 			codexMount = &mount
@@ -218,11 +223,12 @@
 }
 
 func TestSharedGitConfigMustBeRegularFile(t *testing.T) {
+	t.Parallel()
 	home := t.TempDir()
 	if err := os.Mkdir(filepath.Join(home, ".gitconfig"), 0o700); err != nil {
 		t.Fatal(err)
 	}
-	report := ResolveAllConfigsReport(home, nil)
+	report := resolveConfigs(home, nil)
 	for _, mount := range report.Mounts {
 		if mount.Name == ".gitconfig" {
 			t.Fatal("directory mounted as shared git config")
````

#### File 5 of 6: `internal/hostconfig/tree.go`

This is a NEW FILE (155 lines).

Imports: this new file has these import lines: `"fmt"`, `"io/fs"`, `"path/filepath"`, `"strings"`. (The file content below shows how they are grouped.)

Create the file `internal/hostconfig/tree.go` with exactly this content:

````go
package hostconfig

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// validateMaskedConfigTree accepts a directory that will be mounted whole. It
// fails on any symlink that leaves root and on any special file (socket, FIFO,
// device) that is not inside one of the masked paths.
func validateMaskedConfigTree(fsys FileSystem, root string, masks []string) error {
	rootInfo, err := fsys.Stat(root)
	if err != nil {
		return err
	}
	if !rootInfo.IsDir() {
		return fmt.Errorf("masked config root must be a directory")
	}
	maskPaths := make([]string, 0, len(masks))
	for _, mask := range masks {
		clean := filepath.Clean(mask)
		if filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("invalid config mask path %q", mask)
		}
		maskPaths = append(maskPaths, filepath.Join(root, clean))
	}
	return fsys.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path != root {
			for _, mask := range maskPaths {
				if path == mask || isWithin(path, mask) {
					if entry.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}
		}
		info, err := fsys.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			resolved, err := fsys.EvalSymlinks(path)
			if err != nil || !isWithin(resolved, root) {
				return fmt.Errorf("symlink %q resolves outside the config directory or cannot be resolved", path)
			}
			return nil
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("special file %q is not covered by a config mask", path)
		}
		return nil
	})
}

// collectSafeMounts returns the largest subtrees of root that contain only
// regular files and directories, plus a reason for every entry it skipped.
func collectSafeMounts(fsys FileSystem, root string) ([]string, []string) {
	var mounts, refused []string
	var collect func(string)
	collect = func(path string) {
		entries, err := fsys.ReadDir(path)
		if err != nil {
			info, statErr := fsys.Lstat(path)
			if statErr == nil && info.Mode().IsRegular() {
				mounts = append(mounts, path)
			} else {
				refused = append(refused, path+" (cannot inspect config entry safely)")
			}
			return
		}
		if subtreeIsSafe(fsys, path) {
			mounts = append(mounts, path)
			return
		}
		for _, entry := range entries {
			child := filepath.Join(path, entry.Name())
			info, err := fsys.Lstat(child)
			if err != nil {
				refused = append(refused, child+" (cannot inspect config entry safely)")
				continue
			}
			if info.IsDir() {
				collect(child)
			} else if info.Mode().IsRegular() {
				mounts = append(mounts, child)
			} else {
				refused = append(refused, child+" (symlinks and special files are excluded from config mounts)")
			}
		}
	}
	info, err := fsys.Lstat(root)
	if err != nil {
		return nil, []string{root + " (cannot inspect config root safely)"}
	}
	if info.Mode().IsRegular() {
		return []string{root}, nil
	}
	if !info.IsDir() {
		return nil, []string{root + " (config root is not a regular file or directory)"}
	}
	collect(root)
	return mounts, refused
}

// subtreeIsSafe reports whether root contains only regular files and directories.
func subtreeIsSafe(fsys FileSystem, root string) bool {
	entries, err := fsys.ReadDir(root)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		info, err := fsys.Lstat(path)
		if err != nil {
			return false
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return false
		}
		if info.IsDir() {
			if !subtreeIsSafe(fsys, path) {
				return false
			}
		} else if !info.Mode().IsRegular() {
			return false
		}
	}
	return true
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

// isWithin reports whether path is root itself or inside it.
func isWithin(path, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 6 of 6: `internal/plan/run.go`

Imports: no import lines are added or removed in this file.

There is 1 edit in this file. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 1: lines 17-22.**

Find this exact text (it appears exactly once in the file):

````go
	MaskPaths                       []string
}

// RunSpec is an immutable-by-convention description consumed by a runtime adapter.
type RunSpec struct {
	Image         string
````

Replace it with this exact text:

````go
	MaskPaths                       []string
}

// ConfigReport is the outcome of host config discovery: the mounts to make,
// the paths that do not exist, and the paths refused with their reasons.
type ConfigReport struct {
	Mounts  []ConfigMount
	Missing []string
	Refused []string
}

// RunSpec is an immutable-by-convention description consumed by a runtime adapter.
type RunSpec struct {
	Image         string
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/plan/run.go
+++ b/internal/plan/run.go
@@ -17,6 +17,14 @@
 	MaskPaths                       []string
 }
 
+// ConfigReport is the outcome of host config discovery: the mounts to make,
+// the paths that do not exist, and the paths refused with their reasons.
+type ConfigReport struct {
+	Mounts  []ConfigMount
+	Missing []string
+	Refused []string
+}
+
 // RunSpec is an immutable-by-convention description consumed by a runtime adapter.
 type RunSpec struct {
 	Image         string
````

### Verify

Run these verification commands from the repository root:

````bash
gofmt -l .
go build ./...
go vet ./...
go test -count=1 ./...
````

Expected result:

- `gofmt -l .` prints nothing.
- `go build ./...` prints nothing.
- `go vet ./...` prints nothing.
- `go test -count=1 ./...` prints the lines below. The numbers of seconds at the end of the `ok` lines will be different on your machine, and lines may come in a different order. Every package must say `ok` or `[no test files]`:

````text
ok  	github.com/JeremyOrr4/caged/cmd/caged	0.534s
ok  	github.com/JeremyOrr4/caged/internal/agent	0.011s
ok  	github.com/JeremyOrr4/caged/internal/app	0.005s
ok  	github.com/JeremyOrr4/caged/internal/cli	0.006s
ok  	github.com/JeremyOrr4/caged/internal/docker	0.004s
ok  	github.com/JeremyOrr4/caged/internal/hostconfig	0.017s
ok  	github.com/JeremyOrr4/caged/internal/plan	0.003s
ok  	github.com/JeremyOrr4/caged/internal/project	0.005s
ok  	github.com/JeremyOrr4/caged/internal/runtime	0.003s
ok  	github.com/JeremyOrr4/caged/internal/system	0.006s
````

Count the passing tests and subtests with `go test -count=1 -v ./... 2>&1 | grep -c -- "--- PASS"`. On Linux the expected number is **156** (on macOS the Linux golden test is skipped, so the count there is lower by 52 (51 golden subtests plus their parent test)).

Run this command to compute the fingerprint of the whole repository (on macOS use the second command):

````bash
find . -type f -not -path './.git/*' -not -name REFACTOR_INSTRUCTIONS.md -print0 | LC_ALL=C sort -z | xargs -0 sha256sum | sha256sum
````

````bash
find . -type f -not -path './.git/*' -not -name REFACTOR_INSTRUCTIONS.md -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256 | shasum -a 256
````

Expected output (exactly this line):

````text
e34b2ac40f48f3511332ac77d368f414786571dce916d895166f0a3284b76000  -
````

**If it fails:**

- Problem: A golden test fails and the diff shows `config: ...` lines in a different order.
  Fix: The order of agents, of config paths, and of refusals matters. `Resolve` must loop over agents first and over shared files second. Copy `resolver.go` again exactly.
- Problem: `go build` says `undefined: hostconfig.ConfigReport`.
  Fix: The report type moved to `plan.ConfigReport`. Update the parameter type of `logConfigReport` in `internal/app/run.go`.
- Problem: the fingerprint is different but every test passes.
  Fix: some file differs from these instructions by a character, often a tab turned into spaces or a missing final newline. Run `gofmt -w .` (it fixes Go indentation), then check the non-Go files of this step again.

### Checkpoint

At this point, the repo should have these 99 files (not counting `.git/` and this instructions file):

- `./` (5 files): `.gitignore`, `Makefile`, `README.md`, `go.mod`, `go.sum`
- `cmd/caged/` (2 files): `golden_test.go`, `main.go`
- `cmd/caged/testdata/golden/linux/` (51 files): `agent-claude.golden`, `agent-codex.golden`, `agent-copilot.golden`, `agent-gemini.golden`, `agent-gh.golden`, `agent-pi.golden`, `agent-uppercase.golden`, `build-daemon-down.golden`, `build-help.golden`, `build-no-cache.golden`, `build-rejects-args.golden`, `build.golden`, `complete-agents.golden`, `complete-prompt.golden`, `config-overrides.golden`, `daemon-down.golden`, `docker-missing.golden`, `env-extra-names.golden`, `env-invalid.golden`, `env-padded-name.golden`, `help-list.golden`, `help-unknown-topic.golden`, `help.golden`, `home-unresolvable.golden`, `image-missing-builds-first.golden`, `list-rejects-args.golden`, `list.golden`, `no-mount-with-workdir.golden`, `prompt-claude.golden`, `prompt-codex.golden`, `prompt-copilot.golden`, `prompt-gemini.golden`, `prompt-gh.golden`, `prompt-pi.golden`, `run-exit-code.golden`, `run-mounts-denied.golden`, `shell-env.golden`, `shell-no-mount-no-config.golden`, `shell.golden`, `too-many-args.golden`, `unknown-flag.golden`, `unsupported-agent.golden`, `verbose-claude.golden`, `version.golden`, `workdir-flag.golden`, `workdir-home-allowed.golden`, `workdir-is-home.golden`, `workdir-missing.golden`, `workdir-relative.golden`, `workdir-root-allowed.golden`, `workdir-root.golden`
- `dev/` (1 files): `Dockerfile`
- `docs/` (3 files): `architecture.md`, `implementation.md`, `spec.md`
- `internal/agent/` (12 files): `agent.go`, `agent_test.go`, `builtin.go`, `claude.go`, `codex.go`, `copilot.go`, `definitions_test.go`, `gemini.go`, `gh.go`, `pi.go`, `registry.go`, `registry_test.go`
- `internal/agent/testdata/` (1 files): `builtin_agents.golden`
- `internal/app/` (2 files): `run.go`, `run_test.go`
- `internal/cli/` (3 files): `doc.go`, `root.go`, `root_test.go`
- `internal/docker/` (4 files): `doc.go`, `run.go`, `run_test.go`, `runtime.go`
- `internal/hostconfig/` (4 files): `resolver.go`, `resolver_edges_test.go`, `resolver_test.go`, `tree.go`
- `internal/plan/` (2 files): `run.go`, `run_test.go`
- `internal/project/` (2 files): `mount.go`, `mount_test.go`
- `internal/runtime/` (5 files): `Dockerfile`, `build.go`, `caged-entrypoint.sh`, `image.go`, `runtime_test.go`
- `internal/system/` (2 files): `system.go`, `system_test.go`


## Step 8: Turn docker functions into docker.Client (risky change 1 of 3, part 1)

**Goal:** Make `CheckDocker`, `ImageExists` and `Run` methods of a `docker.Client`. The client gets an injected executor, platform and standard streams, and every operating-system rule moves into `platform.go`.

**Why:** The Docker code no longer reads `runtime.GOOS`, `os.Getuid` or `os.Stdin` itself. So the Linux and the macOS behaviour can both be tested on any machine.

### Files touched

- `internal/app/run.go`: modify.
- `internal/app/run_test.go`: modify.
- `internal/cli/root_test.go`: modify.
- `internal/docker/args.go`: moved here from `internal/docker/run.go`, then modified.
- `internal/docker/args_test.go`: moved here from `internal/docker/run_test.go`, then modified.
- `internal/docker/client.go`: moved here from `internal/docker/runtime.go`, then modified.
- `internal/docker/client_test.go`: create this new file.
- `internal/docker/platform.go`: create this new file.

### Commands to run

Run these commands from the repository root:

````bash
mv internal/docker/run.go internal/docker/args.go
mv internal/docker/runtime.go internal/docker/client.go
mv internal/docker/run_test.go internal/docker/args_test.go
````

Each command prints nothing when it works.

### File edits

#### File 1 of 8: `internal/app/run.go`

Imports: added `"context"`. (The edits below already contain these import changes.)

There are 2 edits in this file. They are listed from the bottom of the file to the top. Do them in the order shown: each edit only changes lines below the next one, so the line numbers stay correct. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 2: lines 26-46.**

Find this exact text (it appears exactly once in the file):

````go
	runtime Runtime
}

type dockerRunner struct{}

func (dockerRunner) Run(spec plan.RunSpec) error { return docker.Run(spec) }

type dockerRuntime struct{}

func (dockerRuntime) ImageExists(image string) bool { return docker.ImageExists(image) }
func (dockerRuntime) Build(noCache bool) error {
	return runtime.BuildRuntimeImage(noCache, docker.CheckDocker)
}

func New(runner Runner, runtime Runtime) *Service { return &Service{runner: runner, runtime: runtime} }

// DefaultService wires the production Docker adapters.
func DefaultService() *Service { return New(dockerRunner{}, dockerRuntime{}) }

// Build refreshes the runtime image through the configured runtime adapter.
func (s *Service) Build(noCache bool) error {
````

Replace it with this exact text:

````go
	runtime Runtime
}

type dockerRunner struct{ client *docker.Client }

func (r dockerRunner) Run(spec plan.RunSpec) error { return r.client.Run(context.Background(), spec) }

type dockerRuntime struct{ client *docker.Client }

func (r dockerRuntime) ImageExists(image string) bool {
	return r.client.ImageExists(context.Background(), image)
}
func (r dockerRuntime) Build(noCache bool) error {
	return runtime.BuildRuntimeImage(noCache, func() error { return r.client.Check(context.Background()) })
}

func New(runner Runner, runtime Runtime) *Service { return &Service{runner: runner, runtime: runtime} }

// DefaultService wires the production Docker adapters.
func DefaultService() *Service {
	client := docker.NewClient(system.Executor{}, system.CurrentPlatform(), system.CurrentStdio())
	return New(dockerRunner{client}, dockerRuntime{client})
}

// Build refreshes the runtime image through the configured runtime adapter.
func (s *Service) Build(noCache bool) error {
````

**Edit 2 of 2: lines 2-7.**

Find this exact text (it appears exactly once in the file):

````go
package app

import (
	"fmt"
	"os"

````

Replace it with this exact text:

````go
package app

import (
	"context"
	"fmt"
	"os"

````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/app/run.go
+++ b/internal/app/run.go
@@ -2,6 +2,7 @@
 package app
 
 import (
+	"context"
 	"fmt"
 	"os"
 
@@ -26,21 +27,26 @@
 	runtime Runtime
 }
 
-type dockerRunner struct{}
+type dockerRunner struct{ client *docker.Client }
 
-func (dockerRunner) Run(spec plan.RunSpec) error { return docker.Run(spec) }
+func (r dockerRunner) Run(spec plan.RunSpec) error { return r.client.Run(context.Background(), spec) }
 
-type dockerRuntime struct{}
+type dockerRuntime struct{ client *docker.Client }
 
-func (dockerRuntime) ImageExists(image string) bool { return docker.ImageExists(image) }
-func (dockerRuntime) Build(noCache bool) error {
-	return runtime.BuildRuntimeImage(noCache, docker.CheckDocker)
+func (r dockerRuntime) ImageExists(image string) bool {
+	return r.client.ImageExists(context.Background(), image)
+}
+func (r dockerRuntime) Build(noCache bool) error {
+	return runtime.BuildRuntimeImage(noCache, func() error { return r.client.Check(context.Background()) })
 }
 
 func New(runner Runner, runtime Runtime) *Service { return &Service{runner: runner, runtime: runtime} }
 
 // DefaultService wires the production Docker adapters.
-func DefaultService() *Service { return New(dockerRunner{}, dockerRuntime{}) }
+func DefaultService() *Service {
+	client := docker.NewClient(system.Executor{}, system.CurrentPlatform(), system.CurrentStdio())
+	return New(dockerRunner{client}, dockerRuntime{client})
+}
 
 // Build refreshes the runtime image through the configured runtime adapter.
 func (s *Service) Build(noCache bool) error {
````

#### File 2 of 8: `internal/app/run_test.go`

Imports: added `"github.com/JeremyOrr4/caged/internal/plan"`; removed `"github.com/JeremyOrr4/caged/internal/docker"`. (The edits below already contain these import changes.)

There is 1 edit in this file. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 1: lines 6-20.**

Find this exact text (it appears exactly once in the file):

````go
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/docker"
)

type fakeRunner struct {
	got docker.Options
	err error
}

func (f *fakeRunner) Run(options docker.Options) error { f.got = options; return f.err }

type fakeRuntime struct {
	exists   bool
````

Replace it with this exact text:

````go
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/plan"
)

type fakeRunner struct {
	got plan.RunSpec
	err error
}

func (f *fakeRunner) Run(options plan.RunSpec) error { f.got = options; return f.err }

type fakeRuntime struct {
	exists   bool
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/app/run_test.go
+++ b/internal/app/run_test.go
@@ -6,15 +6,15 @@
 	"testing"
 
 	"github.com/JeremyOrr4/caged/internal/agent"
-	"github.com/JeremyOrr4/caged/internal/docker"
+	"github.com/JeremyOrr4/caged/internal/plan"
 )
 
 type fakeRunner struct {
-	got docker.Options
+	got plan.RunSpec
 	err error
 }
 
-func (f *fakeRunner) Run(options docker.Options) error { f.got = options; return f.err }
+func (f *fakeRunner) Run(options plan.RunSpec) error { f.got = options; return f.err }
 
 type fakeRuntime struct {
 	exists   bool
````

#### File 3 of 8: `internal/cli/root_test.go`

Imports: added `"github.com/JeremyOrr4/caged/internal/plan"`; removed `"github.com/JeremyOrr4/caged/internal/docker"`. (The edits below already contain these import changes.)

There are 2 edits in this file. They are listed from the bottom of the file to the top. Do them in the order shown: each edit only changes lines below the next one, so the line numbers stay correct. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 2: lines 81-87.**

Find this exact text (it appears exactly once in the file):

````go
	t.Parallel()

	request := app.Request{NoMount: true, WorkDir: "/tmp"}
	options := &docker.Options{}
	if err := app.ConfigureMounts(options, request); err == nil || !strings.Contains(err.Error(), "cannot be used with --no-mount") {
		t.Fatalf("ConfigureMounts() error = %v, want incompatible-flags error", err)
	}
````

Replace it with this exact text:

````go
	t.Parallel()

	request := app.Request{NoMount: true, WorkDir: "/tmp"}
	options := &plan.RunSpec{}
	if err := app.ConfigureMounts(options, request); err == nil || !strings.Contains(err.Error(), "cannot be used with --no-mount") {
		t.Fatalf("ConfigureMounts() error = %v, want incompatible-flags error", err)
	}
````

**Edit 2 of 2: lines 7-13.**

Find this exact text (it appears exactly once in the file):

````go

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/app"
	"github.com/JeremyOrr4/caged/internal/docker"
)

func TestRootCmdRejectsTooManyArguments(t *testing.T) {
````

Replace it with this exact text:

````go

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/app"
	"github.com/JeremyOrr4/caged/internal/plan"
)

func TestRootCmdRejectsTooManyArguments(t *testing.T) {
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/cli/root_test.go
+++ b/internal/cli/root_test.go
@@ -7,7 +7,7 @@
 
 	"github.com/JeremyOrr4/caged/internal/agent"
 	"github.com/JeremyOrr4/caged/internal/app"
-	"github.com/JeremyOrr4/caged/internal/docker"
+	"github.com/JeremyOrr4/caged/internal/plan"
 )
 
 func TestRootCmdRejectsTooManyArguments(t *testing.T) {
@@ -81,7 +81,7 @@
 	t.Parallel()
 
 	request := app.Request{NoMount: true, WorkDir: "/tmp"}
-	options := &docker.Options{}
+	options := &plan.RunSpec{}
 	if err := app.ConfigureMounts(options, request); err == nil || !strings.Contains(err.Error(), "cannot be used with --no-mount") {
 		t.Fatalf("ConfigureMounts() error = %v, want incompatible-flags error", err)
 	}
````

#### File 4 of 8: `internal/docker/args.go`

This file was moved from `internal/docker/run.go` by the commands above. The edits below change the moved file.

Imports: added `"github.com/JeremyOrr4/caged/internal/system"`; removed `"bytes"`, `"fmt"`, `"io"`, `"os"`, `"os/exec"`, `"runtime"`. (The edits below already contain these import changes.)

There are 2 edits in this file. They are listed from the bottom of the file to the top. Do them in the order shown: each edit only changes lines below the next one, so the line numbers stay correct. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 2: lines 69-84.**

Find this exact text (it appears exactly once in the file):

````go
			args = append(args, "--tmpfs", maskedPath)
		}
	}
	if goos == "linux" {
		args = append(args, "--user", fmt.Sprintf("%d:%d", uid, gid))
	}
	for _, env := range options.EnvVars {
		args = append(args, "-e", env)
	}
	if options.ReturnToShell {
		args = append(args, "-e", "CAGED_RETURN_TO_SHELL=1")
	}

	args = append(args, options.Image)
	return append(args, options.Command...)
}
````

Replace it with this exact text:

````go
			args = append(args, "--tmpfs", maskedPath)
		}
	}
	args = append(args, userArgs(platform)...)
	for _, env := range spec.EnvVars {
		args = append(args, "-e", env)
	}
	if spec.ReturnToShell {
		args = append(args, "-e", "CAGED_RETURN_TO_SHELL=1")
	}

	args = append(args, spec.Image)
	return append(args, spec.Command...)
}
````

**Edit 2 of 2: lines 1-64.**

Find this exact text (it appears exactly once in the file):

````go
package docker

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/JeremyOrr4/caged/internal/plan"
)

// Options configures a disposable Docker container.
type Options = plan.RunSpec

// Mount maps a host directory into a container directory.
type Mount = plan.Mount

// Run starts an interactive, disposable Docker container.
func Run(options Options) error {
	if err := CheckDocker(); err != nil {
		return err
	}
	docker, err := exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("docker not found, is Docker installed and running?: %w", err)
	}

	args := buildArgs(options, runtime.GOOS, os.Getuid(), os.Getgid())
	cmd := exec.Command(docker, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	var stderr bytes.Buffer
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)
	if err := cmd.Run(); err != nil {
		if runtime.GOOS == "darwin" && looksLikeSharedPathError(stderr.String()) {
			return fmt.Errorf("Docker could not mount the project path; in Docker Desktop, add its parent directory under Settings > Resources > File Sharing: %w", err)
		}
		return err
	}
	return nil
}

func looksLikeSharedPathError(message string) bool {
	message = strings.ToLower(message)
	return strings.Contains(message, "mounts denied") || strings.Contains(message, "not shared") || strings.Contains(message, "mount denied")
}

func buildArgs(options Options, goos string, uid, gid int) []string {
	args := []string{"run", "-it", "--rm"}

	if options.Mount != nil {
		args = append(args,
			"-v", options.Mount.HostPath+":"+options.Mount.ContainerPath,
			"-w", options.Mount.ContainerPath,
		)
	}
	for _, mount := range options.ConfigMounts {
		volume := mount.HostPath + ":" + mount.DestPath
		if !mount.Writable {
			volume += ":ro"
````

Replace it with this exact text:

````go
package docker

import (
	"path/filepath"
	"strings"

	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
)

// runArgs translates a run plan into `docker run` arguments.
func runArgs(spec plan.RunSpec, platform system.Platform) []string {
	args := []string{"run", "-it", "--rm"}

	if spec.Mount != nil {
		args = append(args,
			"-v", spec.Mount.HostPath+":"+spec.Mount.ContainerPath,
			"-w", spec.Mount.ContainerPath,
		)
	}
	for _, mount := range spec.ConfigMounts {
		volume := mount.HostPath + ":" + mount.DestPath
		if !mount.Writable {
			volume += ":ro"
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/docker/run.go
+++ b/internal/docker/args.go
@@ -1,64 +1,24 @@
 package docker
 
 import (
-	"bytes"
-	"fmt"
-	"io"
-	"os"
-	"os/exec"
 	"path/filepath"
-	"runtime"
 	"strings"
 
 	"github.com/JeremyOrr4/caged/internal/plan"
+	"github.com/JeremyOrr4/caged/internal/system"
 )
 
-// Options configures a disposable Docker container.
-type Options = plan.RunSpec
-
-// Mount maps a host directory into a container directory.
-type Mount = plan.Mount
-
-// Run starts an interactive, disposable Docker container.
-func Run(options Options) error {
-	if err := CheckDocker(); err != nil {
-		return err
-	}
-	docker, err := exec.LookPath("docker")
-	if err != nil {
-		return fmt.Errorf("docker not found, is Docker installed and running?: %w", err)
-	}
-
-	args := buildArgs(options, runtime.GOOS, os.Getuid(), os.Getgid())
-	cmd := exec.Command(docker, args...)
-	cmd.Stdin = os.Stdin
-	cmd.Stdout = os.Stdout
-	var stderr bytes.Buffer
-	cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)
-	if err := cmd.Run(); err != nil {
-		if runtime.GOOS == "darwin" && looksLikeSharedPathError(stderr.String()) {
-			return fmt.Errorf("Docker could not mount the project path; in Docker Desktop, add its parent directory under Settings > Resources > File Sharing: %w", err)
-		}
-		return err
-	}
-	return nil
-}
-
-func looksLikeSharedPathError(message string) bool {
-	message = strings.ToLower(message)
-	return strings.Contains(message, "mounts denied") || strings.Contains(message, "not shared") || strings.Contains(message, "mount denied")
-}
-
-func buildArgs(options Options, goos string, uid, gid int) []string {
+// runArgs translates a run plan into `docker run` arguments.
+func runArgs(spec plan.RunSpec, platform system.Platform) []string {
 	args := []string{"run", "-it", "--rm"}
 
-	if options.Mount != nil {
+	if spec.Mount != nil {
 		args = append(args,
-			"-v", options.Mount.HostPath+":"+options.Mount.ContainerPath,
-			"-w", options.Mount.ContainerPath,
+			"-v", spec.Mount.HostPath+":"+spec.Mount.ContainerPath,
+			"-w", spec.Mount.ContainerPath,
 		)
 	}
-	for _, mount := range options.ConfigMounts {
+	for _, mount := range spec.ConfigMounts {
 		volume := mount.HostPath + ":" + mount.DestPath
 		if !mount.Writable {
 			volume += ":ro"
@@ -69,16 +29,14 @@
 			args = append(args, "--tmpfs", maskedPath)
 		}
 	}
-	if goos == "linux" {
-		args = append(args, "--user", fmt.Sprintf("%d:%d", uid, gid))
-	}
-	for _, env := range options.EnvVars {
+	args = append(args, userArgs(platform)...)
+	for _, env := range spec.EnvVars {
 		args = append(args, "-e", env)
 	}
-	if options.ReturnToShell {
+	if spec.ReturnToShell {
 		args = append(args, "-e", "CAGED_RETURN_TO_SHELL=1")
 	}
 
-	args = append(args, options.Image)
-	return append(args, options.Command...)
+	args = append(args, spec.Image)
+	return append(args, spec.Command...)
 }
````

#### File 5 of 8: `internal/docker/args_test.go`

This file was moved from `internal/docker/run_test.go` by the commands above. The edits below change the moved file.

Imports: added `"github.com/JeremyOrr4/caged/internal/system"`. (The edits below already contain these import changes.)

There are 11 edits in this file. They are listed from the bottom of the file to the top. Do them in the order shown: each edit only changes lines below the next one, so the line numbers stay correct. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 11: lines 214-227.**

Find this exact text (it appears exactly once in the file):

````go
	}
}

func TestBuildArgsMaskCodexRuntimeDirectories(t *testing.T) {
	t.Parallel()

	options := Options{Image: runtime.Image, ConfigMounts: []plan.ConfigMount{{
		HostPath: "/home/user/.codex", DestPath: "/home/caged/.codex", Writable: true,
		MaskPaths: []string{"app-server-control", "ipc"},
	}}}
	args := buildArgs(options, "darwin", 0, 0)
	want := []string{"--tmpfs", "/home/caged/.codex/app-server-control", "--tmpfs", "/home/caged/.codex/ipc"}
	for i := 0; i < len(want); i += 2 {
		found := false
````

Replace it with this exact text:

````go
	}
}

func TestRunArgsMaskCodexRuntimeDirectories(t *testing.T) {
	t.Parallel()

	options := plan.RunSpec{Image: runtime.Image, ConfigMounts: []plan.ConfigMount{{
		HostPath: "/home/user/.codex", DestPath: "/home/caged/.codex", Writable: true,
		MaskPaths: []string{"app-server-control", "ipc"},
	}}}
	args := runArgs(options, system.Platform{OS: "darwin"})
	want := []string{"--tmpfs", "/home/caged/.codex/app-server-control", "--tmpfs", "/home/caged/.codex/ipc"}
	for i := 0; i < len(want); i += 2 {
		found := false
````

**Edit 2 of 11: lines 179-205.**

Find this exact text (it appears exactly once in the file):

````go
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := buildArgs(tt.options, tt.goos, tt.uid, tt.gid)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildArgs() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildArgsExposeOnlyPlannedMounts(t *testing.T) {
	t.Parallel()

	options := Options{
		Image: runtime.Image,
		Mount: &Mount{HostPath: "/projects/current", ContainerPath: "/workspace/current"},
		ConfigMounts: []plan.ConfigMount{
			{HostPath: "/home/user/.claude", DestPath: "/home/caged/.claude", Writable: true},
			{HostPath: "/home/user/.gitconfig", DestPath: "/home/caged/.gitconfig", Writable: false},
		},
		Command: []string{"bash"},
	}
	args := buildArgs(options, "darwin", 0, 0)
	volumeCount := 0
	for _, arg := range args {
		if arg == "-v" {
````

Replace it with this exact text:

````go
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := runArgs(tt.options, system.Platform{OS: tt.goos, UID: tt.uid, GID: tt.gid})
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("runArgs() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRunArgsExposeOnlyPlannedMounts(t *testing.T) {
	t.Parallel()

	options := plan.RunSpec{
		Image: runtime.Image,
		Mount: &plan.Mount{HostPath: "/projects/current", ContainerPath: "/workspace/current"},
		ConfigMounts: []plan.ConfigMount{
			{HostPath: "/home/user/.claude", DestPath: "/home/caged/.claude", Writable: true},
			{HostPath: "/home/user/.gitconfig", DestPath: "/home/caged/.gitconfig", Writable: false},
		},
		Command: []string{"bash"},
	}
	args := runArgs(options, system.Platform{OS: "darwin"})
	volumeCount := 0
	for _, arg := range args {
		if arg == "-v" {
````

**Edit 3 of 11: lines 155-161.**

Find this exact text (it appears exactly once in the file):

````go
		},
		{
			name: "Pi config is writable",
			options: Options{
				Image: runtime.Image,
				ConfigMounts: []plan.ConfigMount{{
					Agent:    "pi",
````

Replace it with this exact text:

````go
		},
		{
			name: "Pi config is writable",
			options: plan.RunSpec{
				Image: runtime.Image,
				ConfigMounts: []plan.ConfigMount{{
					Agent:    "pi",
````

**Edit 4 of 11: lines 136-142.**

Find this exact text (it appears exactly once in the file):

````go
		},
		{
			name: "config mount uses image user on macOS",
			options: Options{
				Image: runtime.Image,
				ConfigMounts: []plan.ConfigMount{{
					HostPath: "/tmp/caged-config",
````

Replace it with this exact text:

````go
		},
		{
			name: "config mount uses image user on macOS",
			options: plan.RunSpec{
				Image: runtime.Image,
				ConfigMounts: []plan.ConfigMount{{
					HostPath: "/tmp/caged-config",
````

**Edit 5 of 11: lines 115-121.**

Find this exact text (it appears exactly once in the file):

````go
		},
		{
			name: "Pi config mounts directly read-write",
			options: Options{
				Image: runtime.Image,
				ConfigMounts: []plan.ConfigMount{{
					Agent:    "pi",
````

Replace it with this exact text:

````go
		},
		{
			name: "Pi config mounts directly read-write",
			options: plan.RunSpec{
				Image: runtime.Image,
				ConfigMounts: []plan.ConfigMount{{
					Agent:    "pi",
````

**Edit 6 of 11: lines 86-92.**

Find this exact text (it appears exactly once in the file):

````go
		},
		{
			name: "config read-write mount and env names",
			options: Options{
				Image:         runtime.Image,
				Command:       []string{"claude"},
				EnvVars:       []string{"ANTHROPIC_API_KEY", "GEMINI_API_KEY"},
````

Replace it with this exact text:

````go
		},
		{
			name: "config read-write mount and env names",
			options: plan.RunSpec{
				Image:         runtime.Image,
				Command:       []string{"claude"},
				EnvVars:       []string{"ANTHROPIC_API_KEY", "GEMINI_API_KEY"},
````

**Edit 7 of 11: lines 65-74.**

Find this exact text (it appears exactly once in the file):

````go
		},
		{
			name: "agent command in runtime image",
			options: Options{
				Image:         runtime.Image,
				Command:       []string{"claude"},
				Mount:         &Mount{HostPath: "/project", ContainerPath: "/workspace"},
				ReturnToShell: true,
			},
			goos: "linux",
````

Replace it with this exact text:

````go
		},
		{
			name: "agent command in runtime image",
			options: plan.RunSpec{
				Image:         runtime.Image,
				Command:       []string{"claude"},
				Mount:         &plan.Mount{HostPath: "/project", ContainerPath: "/workspace"},
				ReturnToShell: true,
			},
			goos: "linux",
````

**Edit 8 of 11: lines 48-56.**

Find this exact text (it appears exactly once in the file):

````go
		},
		{
			name: "mount on Linux maps host user",
			options: Options{
				Image: "ubuntu:24.04",
				Mount: &Mount{HostPath: "/work", ContainerPath: "/workspace"},
			},
			goos: "linux",
			uid:  1001,
````

Replace it with this exact text:

````go
		},
		{
			name: "mount on Linux maps host user",
			options: plan.RunSpec{
				Image: "ubuntu:24.04",
				Mount: &plan.Mount{HostPath: "/work", ContainerPath: "/workspace"},
			},
			goos: "linux",
			uid:  1001,
````

**Edit 9 of 11: lines 30-39.**

Find this exact text (it appears exactly once in the file):

````go
		},
		{
			name: "mount with spaces on macOS",
			options: Options{
				Image:   "ubuntu:24.04",
				Command: []string{"bash"},
				Mount: &Mount{
					HostPath:      "/tmp/project with spaces",
					ContainerPath: "/workspace",
				},
````

Replace it with this exact text:

````go
		},
		{
			name: "mount with spaces on macOS",
			options: plan.RunSpec{
				Image:   "ubuntu:24.04",
				Command: []string{"bash"},
				Mount: &plan.Mount{
					HostPath:      "/tmp/project with spaces",
					ContainerPath: "/workspace",
				},
````

**Edit 10 of 11: lines 21-27.**

Find this exact text (it appears exactly once in the file):

````go
	}{
		{
			name: "no mount on macOS",
			options: Options{
				Image:   "ubuntu:24.04",
				Command: []string{"bash"},
			},
````

Replace it with this exact text:

````go
	}{
		{
			name: "no mount on macOS",
			options: plan.RunSpec{
				Image:   "ubuntu:24.04",
				Command: []string{"bash"},
			},
````

**Edit 11 of 11: lines 6-19.**

Find this exact text (it appears exactly once in the file):

````go

	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/runtime"
)

func TestBuildArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		options Options
		goos    string
		uid     int
		gid     int
````

Replace it with this exact text:

````go

	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/runtime"
	"github.com/JeremyOrr4/caged/internal/system"
)

func TestRunArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		options plan.RunSpec
		goos    string
		uid     int
		gid     int
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/docker/run_test.go
+++ b/internal/docker/args_test.go
@@ -6,14 +6,15 @@
 
 	"github.com/JeremyOrr4/caged/internal/plan"
 	"github.com/JeremyOrr4/caged/internal/runtime"
+	"github.com/JeremyOrr4/caged/internal/system"
 )
 
-func TestBuildArgs(t *testing.T) {
+func TestRunArgs(t *testing.T) {
 	t.Parallel()
 
 	tests := []struct {
 		name    string
-		options Options
+		options plan.RunSpec
 		goos    string
 		uid     int
 		gid     int
@@ -21,7 +22,7 @@
 	}{
 		{
 			name: "no mount on macOS",
-			options: Options{
+			options: plan.RunSpec{
 				Image:   "ubuntu:24.04",
 				Command: []string{"bash"},
 			},
@@ -30,10 +31,10 @@
 		},
 		{
 			name: "mount with spaces on macOS",
-			options: Options{
+			options: plan.RunSpec{
 				Image:   "ubuntu:24.04",
 				Command: []string{"bash"},
-				Mount: &Mount{
+				Mount: &plan.Mount{
 					HostPath:      "/tmp/project with spaces",
 					ContainerPath: "/workspace",
 				},
@@ -48,9 +49,9 @@
 		},
 		{
 			name: "mount on Linux maps host user",
-			options: Options{
+			options: plan.RunSpec{
 				Image: "ubuntu:24.04",
-				Mount: &Mount{HostPath: "/work", ContainerPath: "/workspace"},
+				Mount: &plan.Mount{HostPath: "/work", ContainerPath: "/workspace"},
 			},
 			goos: "linux",
 			uid:  1001,
@@ -65,10 +66,10 @@
 		},
 		{
 			name: "agent command in runtime image",
-			options: Options{
+			options: plan.RunSpec{
 				Image:         runtime.Image,
 				Command:       []string{"claude"},
-				Mount:         &Mount{HostPath: "/project", ContainerPath: "/workspace"},
+				Mount:         &plan.Mount{HostPath: "/project", ContainerPath: "/workspace"},
 				ReturnToShell: true,
 			},
 			goos: "linux",
@@ -86,7 +87,7 @@
 		},
 		{
 			name: "config read-write mount and env names",
-			options: Options{
+			options: plan.RunSpec{
 				Image:         runtime.Image,
 				Command:       []string{"claude"},
 				EnvVars:       []string{"ANTHROPIC_API_KEY", "GEMINI_API_KEY"},
@@ -115,7 +116,7 @@
 		},
 		{
 			name: "Pi config mounts directly read-write",
-			options: Options{
+			options: plan.RunSpec{
 				Image: runtime.Image,
 				ConfigMounts: []plan.ConfigMount{{
 					Agent:    "pi",
@@ -136,7 +137,7 @@
 		},
 		{
 			name: "config mount uses image user on macOS",
-			options: Options{
+			options: plan.RunSpec{
 				Image: runtime.Image,
 				ConfigMounts: []plan.ConfigMount{{
 					HostPath: "/tmp/caged-config",
@@ -155,7 +156,7 @@
 		},
 		{
 			name: "Pi config is writable",
-			options: Options{
+			options: plan.RunSpec{
 				Image: runtime.Image,
 				ConfigMounts: []plan.ConfigMount{{
 					Agent:    "pi",
@@ -179,27 +180,27 @@
 	for _, tt := range tests {
 		t.Run(tt.name, func(t *testing.T) {
 			t.Parallel()
-			got := buildArgs(tt.options, tt.goos, tt.uid, tt.gid)
+			got := runArgs(tt.options, system.Platform{OS: tt.goos, UID: tt.uid, GID: tt.gid})
 			if !reflect.DeepEqual(got, tt.want) {
-				t.Errorf("buildArgs() = %q, want %q", got, tt.want)
+				t.Errorf("runArgs() = %q, want %q", got, tt.want)
 			}
 		})
 	}
 }
 
-func TestBuildArgsExposeOnlyPlannedMounts(t *testing.T) {
+func TestRunArgsExposeOnlyPlannedMounts(t *testing.T) {
 	t.Parallel()
 
-	options := Options{
+	options := plan.RunSpec{
 		Image: runtime.Image,
-		Mount: &Mount{HostPath: "/projects/current", ContainerPath: "/workspace/current"},
+		Mount: &plan.Mount{HostPath: "/projects/current", ContainerPath: "/workspace/current"},
 		ConfigMounts: []plan.ConfigMount{
 			{HostPath: "/home/user/.claude", DestPath: "/home/caged/.claude", Writable: true},
 			{HostPath: "/home/user/.gitconfig", DestPath: "/home/caged/.gitconfig", Writable: false},
 		},
 		Command: []string{"bash"},
 	}
-	args := buildArgs(options, "darwin", 0, 0)
+	args := runArgs(options, system.Platform{OS: "darwin"})
 	volumeCount := 0
 	for _, arg := range args {
 		if arg == "-v" {
@@ -214,14 +215,14 @@
 	}
 }
 
-func TestBuildArgsMaskCodexRuntimeDirectories(t *testing.T) {
+func TestRunArgsMaskCodexRuntimeDirectories(t *testing.T) {
 	t.Parallel()
 
-	options := Options{Image: runtime.Image, ConfigMounts: []plan.ConfigMount{{
+	options := plan.RunSpec{Image: runtime.Image, ConfigMounts: []plan.ConfigMount{{
 		HostPath: "/home/user/.codex", DestPath: "/home/caged/.codex", Writable: true,
 		MaskPaths: []string{"app-server-control", "ipc"},
 	}}}
-	args := buildArgs(options, "darwin", 0, 0)
+	args := runArgs(options, system.Platform{OS: "darwin"})
 	want := []string{"--tmpfs", "/home/caged/.codex/app-server-control", "--tmpfs", "/home/caged/.codex/ipc"}
 	for i := 0; i < len(want); i += 2 {
 		found := false
````

#### File 6 of 8: `internal/docker/client.go`

This file was moved from `internal/docker/runtime.go` by the commands above. The edits below change the moved file.

Imports: added `"bytes"`, `"context"`, `"io"`, `"github.com/JeremyOrr4/caged/internal/plan"`, `"github.com/JeremyOrr4/caged/internal/system"`; removed `"os/exec"`, `"runtime"`. (The edits below already contain these import changes.)

**Edit 1 of 1: replace the whole file.** At the start of this step the file has 33 lines (lines 1-33).

The file currently contains exactly this text:

````go
package docker

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

func ImageExists(image string) bool {
	cmd := exec.Command("docker", "image", "inspect", image)
	return cmd.Run() == nil
}

// CheckDocker distinguishes a missing CLI from a stopped Docker daemon.
func CheckDocker() error {
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		return fmt.Errorf("unsupported host architecture %q; caged supports amd64 and arm64 images", runtime.GOARCH)
	}
	docker, err := exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("Docker is not installed or not on PATH: %w", err)
	}
	cmd := exec.Command(docker, "info", "--format", "{{.ServerVersion}}")
	if output, err := cmd.CombinedOutput(); err != nil {
		launcher := "start the Docker daemon"
		if runtime.GOOS == "darwin" {
			launcher = "start Docker Desktop"
		}
		return fmt.Errorf("Docker is installed but the daemon is unavailable; %s: %s", launcher, strings.TrimSpace(string(output)))
	}
	return nil
}
````

Replace the entire contents of the file with exactly this text:

````go
package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
)

// Executor runs host processes. system.Executor is the production implementation.
type Executor interface {
	LookPath(file string) (string, error)
	Run(ctx context.Context, cmd system.Command) error
}

// Client drives the docker CLI.
type Client struct {
	exec     Executor
	platform system.Platform
	stdio    system.Stdio
}

// NewClient returns a Client that runs docker through exec, applies the
// rules for platform, and connects containers to stdio.
func NewClient(exec Executor, platform system.Platform, stdio system.Stdio) *Client {
	return &Client{exec: exec, platform: platform, stdio: stdio}
}

// Check distinguishes a missing docker CLI from a stopped Docker daemon.
func (c *Client) Check(ctx context.Context) error {
	if err := checkArch(c.platform); err != nil {
		return err
	}
	docker, err := c.exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("Docker is not installed or not on PATH: %w", err)
	}
	var output bytes.Buffer
	info := system.Command{Path: docker, Args: []string{"info", "--format", "{{.ServerVersion}}"}, Stdout: &output, Stderr: &output}
	if err := c.exec.Run(ctx, info); err != nil {
		return fmt.Errorf("Docker is installed but the daemon is unavailable; %s: %s", daemonLauncher(c.platform), strings.TrimSpace(output.String()))
	}
	return nil
}

// ImageExists reports whether image is in the local image store.
func (c *Client) ImageExists(ctx context.Context, image string) bool {
	return c.exec.Run(ctx, system.Command{Path: "docker", Args: []string{"image", "inspect", image}}) == nil
}

// Run starts an interactive, disposable container and waits for it to exit.
// A non-zero container exit is returned unchanged so its code can be passed on.
func (c *Client) Run(ctx context.Context, spec plan.RunSpec) error {
	if err := c.Check(ctx); err != nil {
		return err
	}
	docker, err := c.exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("docker not found, is Docker installed and running?: %w", err)
	}
	var stderr bytes.Buffer
	err = c.exec.Run(ctx, system.Command{
		Path:   docker,
		Args:   runArgs(spec, c.platform),
		Stdin:  c.stdio.In,
		Stdout: c.stdio.Out,
		Stderr: io.MultiWriter(c.stdio.Err, &stderr),
	})
	if err != nil {
		return explainRunError(c.platform, stderr.String(), err)
	}
	return nil
}
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/docker/runtime.go
+++ b/internal/docker/client.go
@@ -1,33 +1,77 @@
 package docker
 
 import (
+	"bytes"
+	"context"
 	"fmt"
-	"os/exec"
-	"runtime"
+	"io"
 	"strings"
+
+	"github.com/JeremyOrr4/caged/internal/plan"
+	"github.com/JeremyOrr4/caged/internal/system"
 )
 
-func ImageExists(image string) bool {
-	cmd := exec.Command("docker", "image", "inspect", image)
-	return cmd.Run() == nil
+// Executor runs host processes. system.Executor is the production implementation.
+type Executor interface {
+	LookPath(file string) (string, error)
+	Run(ctx context.Context, cmd system.Command) error
+}
+
+// Client drives the docker CLI.
+type Client struct {
+	exec     Executor
+	platform system.Platform
+	stdio    system.Stdio
+}
+
+// NewClient returns a Client that runs docker through exec, applies the
+// rules for platform, and connects containers to stdio.
+func NewClient(exec Executor, platform system.Platform, stdio system.Stdio) *Client {
+	return &Client{exec: exec, platform: platform, stdio: stdio}
 }
 
-// CheckDocker distinguishes a missing CLI from a stopped Docker daemon.
-func CheckDocker() error {
-	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
-		return fmt.Errorf("unsupported host architecture %q; caged supports amd64 and arm64 images", runtime.GOARCH)
+// Check distinguishes a missing docker CLI from a stopped Docker daemon.
+func (c *Client) Check(ctx context.Context) error {
+	if err := checkArch(c.platform); err != nil {
+		return err
 	}
-	docker, err := exec.LookPath("docker")
+	docker, err := c.exec.LookPath("docker")
 	if err != nil {
 		return fmt.Errorf("Docker is not installed or not on PATH: %w", err)
 	}
-	cmd := exec.Command(docker, "info", "--format", "{{.ServerVersion}}")
-	if output, err := cmd.CombinedOutput(); err != nil {
-		launcher := "start the Docker daemon"
-		if runtime.GOOS == "darwin" {
-			launcher = "start Docker Desktop"
-		}
-		return fmt.Errorf("Docker is installed but the daemon is unavailable; %s: %s", launcher, strings.TrimSpace(string(output)))
+	var output bytes.Buffer
+	info := system.Command{Path: docker, Args: []string{"info", "--format", "{{.ServerVersion}}"}, Stdout: &output, Stderr: &output}
+	if err := c.exec.Run(ctx, info); err != nil {
+		return fmt.Errorf("Docker is installed but the daemon is unavailable; %s: %s", daemonLauncher(c.platform), strings.TrimSpace(output.String()))
+	}
+	return nil
+}
+
+// ImageExists reports whether image is in the local image store.
+func (c *Client) ImageExists(ctx context.Context, image string) bool {
+	return c.exec.Run(ctx, system.Command{Path: "docker", Args: []string{"image", "inspect", image}}) == nil
+}
+
+// Run starts an interactive, disposable container and waits for it to exit.
+// A non-zero container exit is returned unchanged so its code can be passed on.
+func (c *Client) Run(ctx context.Context, spec plan.RunSpec) error {
+	if err := c.Check(ctx); err != nil {
+		return err
+	}
+	docker, err := c.exec.LookPath("docker")
+	if err != nil {
+		return fmt.Errorf("docker not found, is Docker installed and running?: %w", err)
+	}
+	var stderr bytes.Buffer
+	err = c.exec.Run(ctx, system.Command{
+		Path:   docker,
+		Args:   runArgs(spec, c.platform),
+		Stdin:  c.stdio.In,
+		Stdout: c.stdio.Out,
+		Stderr: io.MultiWriter(c.stdio.Err, &stderr),
+	})
+	if err != nil {
+		return explainRunError(c.platform, stderr.String(), err)
 	}
 	return nil
 }
````

#### File 7 of 8: `internal/docker/client_test.go`

This is a NEW FILE (220 lines).

Imports: this new file has these import lines: `"bytes"`, `"context"`, `"errors"`, `"io"`, `"reflect"`, `"strings"`, `"testing"`, `"github.com/JeremyOrr4/caged/internal/plan"`, `"github.com/JeremyOrr4/caged/internal/system"`. (The file content below shows how they are grouped.)

Create the file `internal/docker/client_test.go` with exactly this content:

````go
package docker

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
)

// fakeExecutor records commands instead of running them.
type fakeExecutor struct {
	lookPathErr error
	// lookPathFailAfter makes LookPath fail once it has succeeded this many times (0 = never).
	lookPathFailAfter int
	lookPathCalls     int
	// run answers a command; nil means success.
	run      func(system.Command) error
	commands []system.Command
}

func (f *fakeExecutor) LookPath(file string) (string, error) {
	f.lookPathCalls++
	if f.lookPathErr != nil || (f.lookPathFailAfter > 0 && f.lookPathCalls > f.lookPathFailAfter) {
		return "", errors.New(`exec: "` + file + `": executable file not found in $PATH`)
	}
	return "/usr/local/bin/" + file, nil
}

func (f *fakeExecutor) Run(_ context.Context, cmd system.Command) error {
	f.commands = append(f.commands, cmd)
	if f.run != nil {
		return f.run(cmd)
	}
	return nil
}

var (
	linux  = system.Platform{OS: "linux", Arch: "amd64", UID: 1000, GID: 1001}
	darwin = system.Platform{OS: "darwin", Arch: "arm64", UID: 501, GID: 20}
)

func TestCheckRunsDockerInfo(t *testing.T) {
	t.Parallel()

	exec := &fakeExecutor{}
	if err := NewClient(exec, darwin, system.Stdio{}).Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(exec.commands) != 1 {
		t.Fatalf("commands = %+v, want one docker info", exec.commands)
	}
	info := exec.commands[0]
	if info.Path != "/usr/local/bin/docker" || !reflect.DeepEqual(info.Args, []string{"info", "--format", "{{.ServerVersion}}"}) {
		t.Fatalf("command = %+v", info)
	}
	// Combined output: stdout and stderr share one buffer, like exec.Cmd.CombinedOutput.
	if info.Stdout == nil || info.Stdout != info.Stderr || info.Stdin != nil {
		t.Fatalf("docker info must capture combined output and read no input: %+v", info)
	}
}

func TestCheckErrors(t *testing.T) {
	t.Parallel()

	daemonDown := func(cmd system.Command) error {
		io.WriteString(cmd.Stdout, "Cannot connect to the Docker daemon\n")
		return errors.New("exit status 1")
	}
	tests := []struct {
		name     string
		platform system.Platform
		exec     *fakeExecutor
		want     string
	}{
		{name: "unsupported arch", platform: system.Platform{OS: "linux", Arch: "386"}, exec: &fakeExecutor{}, want: `unsupported host architecture "386"; caged supports amd64 and arm64 images`},
		{name: "missing CLI", platform: linux, exec: &fakeExecutor{lookPathErr: errors.New("x")}, want: `Docker is not installed or not on PATH: exec: "docker": executable file not found in $PATH`},
		{name: "daemon down on Linux", platform: linux, exec: &fakeExecutor{run: daemonDown}, want: "Docker is installed but the daemon is unavailable; start the Docker daemon: Cannot connect to the Docker daemon"},
		{name: "daemon down on macOS", platform: darwin, exec: &fakeExecutor{run: daemonDown}, want: "Docker is installed but the daemon is unavailable; start Docker Desktop: Cannot connect to the Docker daemon"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := NewClient(tt.exec, tt.platform, system.Stdio{}).Check(context.Background())
			if err == nil || err.Error() != tt.want {
				t.Fatalf("Check() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestImageExists(t *testing.T) {
	t.Parallel()

	present := &fakeExecutor{}
	if !NewClient(present, linux, system.Stdio{}).ImageExists(context.Background(), "caged-runtime:latest") {
		t.Fatal("ImageExists() = false, want true")
	}
	want := system.Command{Path: "docker", Args: []string{"image", "inspect", "caged-runtime:latest"}}
	if !reflect.DeepEqual(present.commands, []system.Command{want}) {
		t.Fatalf("commands = %+v, want %+v", present.commands, want)
	}
	missing := &fakeExecutor{run: func(system.Command) error { return errors.New("exit status 1") }}
	if NewClient(missing, linux, system.Stdio{}).ImageExists(context.Background(), "caged-runtime:latest") {
		t.Fatal("ImageExists() = true, want false")
	}
}

func TestRunConnectsStdioAndPassesArgs(t *testing.T) {
	t.Parallel()

	stdin := strings.NewReader("")
	var stdout, stderr bytes.Buffer
	exec := &fakeExecutor{run: func(cmd system.Command) error {
		if cmd.Args[0] == "run" {
			io.WriteString(cmd.Stderr, "container stderr\n")
		}
		return nil
	}}
	spec := plan.RunSpec{Image: "img", Command: []string{"claude"}, ReturnToShell: true}
	if err := NewClient(exec, linux, system.Stdio{In: stdin, Out: &stdout, Err: &stderr}).Run(context.Background(), spec); err != nil {
		t.Fatal(err)
	}
	if len(exec.commands) != 2 {
		t.Fatalf("commands = %+v, want docker info then docker run", exec.commands)
	}
	run := exec.commands[1]
	wantArgs := []string{"run", "-it", "--rm", "--user", "1000:1001", "-e", "CAGED_RETURN_TO_SHELL=1", "img", "claude"}
	if run.Path != "/usr/local/bin/docker" || !reflect.DeepEqual(run.Args, wantArgs) {
		t.Fatalf("run = %+v, want args %q", run, wantArgs)
	}
	if run.Stdin != stdin || run.Stdout != &stdout {
		t.Fatal("docker run is not connected to the client's stdin and stdout")
	}
	if stderr.String() != "container stderr\n" {
		t.Fatalf("stderr = %q, want the container's stderr passed through", stderr.String())
	}
}

func TestRunErrors(t *testing.T) {
	t.Parallel()

	exitErr := errors.New("exit status 125")
	mountsDenied := func(cmd system.Command) error {
		if cmd.Args[0] != "run" {
			return nil
		}
		io.WriteString(cmd.Stderr, "docker: Error response from daemon: Mounts denied: path not shared\n")
		return exitErr
	}
	t.Run("linux returns the exit error unchanged", func(t *testing.T) {
		t.Parallel()
		err := NewClient(&fakeExecutor{run: mountsDenied}, linux, system.Stdio{Err: io.Discard}).Run(context.Background(), plan.RunSpec{})
		if err != exitErr {
			t.Fatalf("Run() error = %v, want the original exit error", err)
		}
	})
	t.Run("macOS explains Docker Desktop file sharing", func(t *testing.T) {
		t.Parallel()
		err := NewClient(&fakeExecutor{run: mountsDenied}, darwin, system.Stdio{Err: io.Discard}).Run(context.Background(), plan.RunSpec{})
		want := "Docker could not mount the project path; in Docker Desktop, add its parent directory under Settings > Resources > File Sharing: exit status 125"
		if err == nil || err.Error() != want || !errors.Is(err, exitErr) {
			t.Fatalf("Run() error = %v, want %q wrapping the exit error", err, want)
		}
	})
	t.Run("check failure stops before running", func(t *testing.T) {
		t.Parallel()
		exec := &fakeExecutor{lookPathErr: errors.New("x")}
		if err := NewClient(exec, linux, system.Stdio{}).Run(context.Background(), plan.RunSpec{}); err == nil || !strings.HasPrefix(err.Error(), "Docker is not installed") {
			t.Fatalf("Run() error = %v", err)
		}
		if len(exec.commands) != 0 {
			t.Fatalf("commands = %+v, want none", exec.commands)
		}
	})
	t.Run("docker disappears after the check", func(t *testing.T) {
		t.Parallel()
		exec := &fakeExecutor{lookPathFailAfter: 1}
		err := NewClient(exec, linux, system.Stdio{}).Run(context.Background(), plan.RunSpec{})
		if err == nil || err.Error() != `docker not found, is Docker installed and running?: exec: "docker": executable file not found in $PATH` {
			t.Fatalf("Run() error = %v", err)
		}
	})
}

func TestPlatformRules(t *testing.T) {
	t.Parallel()

	if got := userArgs(linux); !reflect.DeepEqual(got, []string{"--user", "1000:1001"}) {
		t.Errorf("userArgs(linux) = %q", got)
	}
	if got := userArgs(darwin); got != nil {
		t.Errorf("userArgs(darwin) = %q, want none: Docker Desktop maps ownership", got)
	}
	if got := daemonLauncher(darwin); got != "start Docker Desktop" {
		t.Errorf("daemonLauncher(darwin) = %q", got)
	}
	if got := daemonLauncher(linux); got != "start the Docker daemon" {
		t.Errorf("daemonLauncher(linux) = %q", got)
	}
	for _, arch := range []string{"amd64", "arm64"} {
		if err := checkArch(system.Platform{Arch: arch}); err != nil {
			t.Errorf("checkArch(%s) = %v", arch, err)
		}
	}
	for _, message := range []string{"Mounts denied", "path is NOT SHARED from the host", "mount denied"} {
		if !looksLikeSharedPathError(message) {
			t.Errorf("looksLikeSharedPathError(%q) = false", message)
		}
	}
	plain := errors.New("exit status 1")
	if got := explainRunError(darwin, "some other failure", plain); got != plain {
		t.Errorf("explainRunError() = %v, want the error unchanged", got)
	}
}
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 8 of 8: `internal/docker/platform.go`

This is a NEW FILE (51 lines).

Imports: this new file has these import lines: `"fmt"`, `"strings"`, `"github.com/JeremyOrr4/caged/internal/system"`. (The file content below shows how they are grouped.)

Create the file `internal/docker/platform.go` with exactly this content:

````go
package docker

import (
	"fmt"
	"strings"

	"github.com/JeremyOrr4/caged/internal/system"
)

// This file holds every platform-specific Docker rule. The platform is
// injected, so the Linux and macOS branches are both tested on any host.

// checkArch rejects hosts that have no matching runtime image architecture.
func checkArch(platform system.Platform) error {
	if platform.Arch != "amd64" && platform.Arch != "arm64" {
		return fmt.Errorf("unsupported host architecture %q; caged supports amd64 and arm64 images", platform.Arch)
	}
	return nil
}

// userArgs runs the container as the host user on Linux, so files created in
// bind mounts belong to that user. Docker Desktop on macOS translates
// bind-mount ownership itself, so the image's own user is kept there.
func userArgs(platform system.Platform) []string {
	if platform.OS == "linux" {
		return []string{"--user", fmt.Sprintf("%d:%d", platform.UID, platform.GID)}
	}
	return nil
}

// daemonLauncher names what the user has to start when the daemon is down.
func daemonLauncher(platform system.Platform) string {
	if platform.OS == "darwin" {
		return "start Docker Desktop"
	}
	return "start the Docker daemon"
}

// explainRunError adds the Docker Desktop File Sharing hint on macOS when the
// project path is not shared with Docker Desktop's VM.
func explainRunError(platform system.Platform, stderr string, err error) error {
	if platform.OS == "darwin" && looksLikeSharedPathError(stderr) {
		return fmt.Errorf("Docker could not mount the project path; in Docker Desktop, add its parent directory under Settings > Resources > File Sharing: %w", err)
	}
	return err
}

func looksLikeSharedPathError(message string) bool {
	message = strings.ToLower(message)
	return strings.Contains(message, "mounts denied") || strings.Contains(message, "not shared") || strings.Contains(message, "mount denied")
}
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

### Verify

Run these verification commands from the repository root:

````bash
gofmt -l .
go build ./...
go vet ./...
go test -count=1 ./...
````

Expected result:

- `gofmt -l .` prints nothing.
- `go build ./...` prints nothing.
- `go vet ./...` prints nothing.
- `go test -count=1 ./...` prints the lines below. The numbers of seconds at the end of the `ok` lines will be different on your machine, and lines may come in a different order. Every package must say `ok` or `[no test files]`:

````text
ok  	github.com/JeremyOrr4/caged/cmd/caged	0.577s
ok  	github.com/JeremyOrr4/caged/internal/agent	0.006s
ok  	github.com/JeremyOrr4/caged/internal/app	0.003s
ok  	github.com/JeremyOrr4/caged/internal/cli	0.006s
ok  	github.com/JeremyOrr4/caged/internal/docker	0.009s
ok  	github.com/JeremyOrr4/caged/internal/hostconfig	0.010s
ok  	github.com/JeremyOrr4/caged/internal/plan	0.005s
ok  	github.com/JeremyOrr4/caged/internal/project	0.007s
ok  	github.com/JeremyOrr4/caged/internal/runtime	0.003s
ok  	github.com/JeremyOrr4/caged/internal/system	0.005s
````

Count the passing tests and subtests with `go test -count=1 -v ./... 2>&1 | grep -c -- "--- PASS"`. On Linux the expected number is **170** (on macOS the Linux golden test is skipped, so the count there is lower by 52 (51 golden subtests plus their parent test)).

Run this command to compute the fingerprint of the whole repository (on macOS use the second command):

````bash
find . -type f -not -path './.git/*' -not -name REFACTOR_INSTRUCTIONS.md -print0 | LC_ALL=C sort -z | xargs -0 sha256sum | sha256sum
````

````bash
find . -type f -not -path './.git/*' -not -name REFACTOR_INSTRUCTIONS.md -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256 | shasum -a 256
````

Expected output (exactly this line):

````text
9f0a631b2bc78c14d56b0b65ffa5c63d4ca4c954a7680ea97751e0c536a7a434  -
````

**If it fails:**

- Problem: `mv: cannot stat 'internal/docker/run.go'`.
  Fix: You already ran the `mv` commands once. Check with `ls internal/docker`. If `args.go` and `client.go` exist, skip the commands and go on with the edits.
- Problem: Golden test `docker-missing` or `daemon-down` fails.
  Fix: `ImageExists` must run the command `docker` by name, exactly `system.Command{Path: "docker", ...}`, and must not call `LookPath` first. `Check` must use one shared buffer for both Stdout and Stderr.
- Problem: the fingerprint is different but every test passes.
  Fix: some file differs from these instructions by a character, often a tab turned into spaces or a missing final newline. Run `gofmt -w .` (it fixes Go indentation), then check the non-Go files of this step again.

### Checkpoint

At this point, the repo should have these 101 files (not counting `.git/` and this instructions file):

- `./` (5 files): `.gitignore`, `Makefile`, `README.md`, `go.mod`, `go.sum`
- `cmd/caged/` (2 files): `golden_test.go`, `main.go`
- `cmd/caged/testdata/golden/linux/` (51 files): `agent-claude.golden`, `agent-codex.golden`, `agent-copilot.golden`, `agent-gemini.golden`, `agent-gh.golden`, `agent-pi.golden`, `agent-uppercase.golden`, `build-daemon-down.golden`, `build-help.golden`, `build-no-cache.golden`, `build-rejects-args.golden`, `build.golden`, `complete-agents.golden`, `complete-prompt.golden`, `config-overrides.golden`, `daemon-down.golden`, `docker-missing.golden`, `env-extra-names.golden`, `env-invalid.golden`, `env-padded-name.golden`, `help-list.golden`, `help-unknown-topic.golden`, `help.golden`, `home-unresolvable.golden`, `image-missing-builds-first.golden`, `list-rejects-args.golden`, `list.golden`, `no-mount-with-workdir.golden`, `prompt-claude.golden`, `prompt-codex.golden`, `prompt-copilot.golden`, `prompt-gemini.golden`, `prompt-gh.golden`, `prompt-pi.golden`, `run-exit-code.golden`, `run-mounts-denied.golden`, `shell-env.golden`, `shell-no-mount-no-config.golden`, `shell.golden`, `too-many-args.golden`, `unknown-flag.golden`, `unsupported-agent.golden`, `verbose-claude.golden`, `version.golden`, `workdir-flag.golden`, `workdir-home-allowed.golden`, `workdir-is-home.golden`, `workdir-missing.golden`, `workdir-relative.golden`, `workdir-root-allowed.golden`, `workdir-root.golden`
- `dev/` (1 files): `Dockerfile`
- `docs/` (3 files): `architecture.md`, `implementation.md`, `spec.md`
- `internal/agent/` (12 files): `agent.go`, `agent_test.go`, `builtin.go`, `claude.go`, `codex.go`, `copilot.go`, `definitions_test.go`, `gemini.go`, `gh.go`, `pi.go`, `registry.go`, `registry_test.go`
- `internal/agent/testdata/` (1 files): `builtin_agents.golden`
- `internal/app/` (2 files): `run.go`, `run_test.go`
- `internal/cli/` (3 files): `doc.go`, `root.go`, `root_test.go`
- `internal/docker/` (6 files): `args.go`, `args_test.go`, `client.go`, `client_test.go`, `doc.go`, `platform.go`
- `internal/hostconfig/` (4 files): `resolver.go`, `resolver_edges_test.go`, `resolver_test.go`, `tree.go`
- `internal/plan/` (2 files): `run.go`, `run_test.go`
- `internal/project/` (2 files): `mount.go`, `mount_test.go`
- `internal/runtime/` (5 files): `Dockerfile`, `build.go`, `caged-entrypoint.sh`, `image.go`, `runtime_test.go`
- `internal/system/` (2 files): `system.go`, `system_test.go`


## Step 9: Move docker build into docker.Client; make runtime pure data (risky change 1 of 3, part 2)

**Goal:** `runtime` now only describes the image (`Image` and `BuildSpec`). `docker.Client.Build` writes the build context and runs `docker build`.

**Why:** Before this step, two packages knew how to run Docker. Now only `docker` does, and its temporary-directory handling goes through an injected file system that tests can fake.

### Files touched

- `internal/app/run.go`: modify.
- `internal/docker/args.go`: modify.
- `internal/docker/client.go`: modify.
- `internal/docker/client_test.go`: modify.
- `internal/plan/run.go`: modify.
- `internal/runtime/assets.go`: moved here from `internal/runtime/build.go`, then modified.
- `internal/runtime/image.go`: modify.
- `internal/runtime/runtime_test.go`: modify.

### Commands to run

Run these commands from the repository root:

````bash
mv internal/runtime/build.go internal/runtime/assets.go
````

Each command prints nothing when it works.

### File edits

#### File 1 of 8: `internal/app/run.go`

Imports: no import lines are added or removed in this file.

There is 1 edit in this file. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 1: lines 37-50.**

Find this exact text (it appears exactly once in the file):

````go
	return r.client.ImageExists(context.Background(), image)
}
func (r dockerRuntime) Build(noCache bool) error {
	return runtime.BuildRuntimeImage(noCache, func() error { return r.client.Check(context.Background()) })
}

func New(runner Runner, runtime Runtime) *Service { return &Service{runner: runner, runtime: runtime} }

// DefaultService wires the production Docker adapters.
func DefaultService() *Service {
	client := docker.NewClient(system.Executor{}, system.CurrentPlatform(), system.CurrentStdio())
	return New(dockerRunner{client}, dockerRuntime{client})
}

````

Replace it with this exact text:

````go
	return r.client.ImageExists(context.Background(), image)
}
func (r dockerRuntime) Build(noCache bool) error {
	return r.client.Build(context.Background(), runtime.BuildSpec(noCache))
}

func New(runner Runner, runtime Runtime) *Service { return &Service{runner: runner, runtime: runtime} }

// DefaultService wires the production Docker adapters.
func DefaultService() *Service {
	client := docker.NewClient(system.Executor{}, system.FS{}, system.CurrentPlatform(), system.CurrentStdio())
	return New(dockerRunner{client}, dockerRuntime{client})
}

````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/app/run.go
+++ b/internal/app/run.go
@@ -37,14 +37,14 @@
 	return r.client.ImageExists(context.Background(), image)
 }
 func (r dockerRuntime) Build(noCache bool) error {
-	return runtime.BuildRuntimeImage(noCache, func() error { return r.client.Check(context.Background()) })
+	return r.client.Build(context.Background(), runtime.BuildSpec(noCache))
 }
 
 func New(runner Runner, runtime Runtime) *Service { return &Service{runner: runner, runtime: runtime} }
 
 // DefaultService wires the production Docker adapters.
 func DefaultService() *Service {
-	client := docker.NewClient(system.Executor{}, system.CurrentPlatform(), system.CurrentStdio())
+	client := docker.NewClient(system.Executor{}, system.FS{}, system.CurrentPlatform(), system.CurrentStdio())
 	return New(dockerRunner{client}, dockerRuntime{client})
 }
 
````

#### File 2 of 8: `internal/docker/args.go`

Imports: no import lines are added or removed in this file.

There is 1 edit in this file. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 1: lines 40-42.**

Find this exact text (it appears exactly once in the file):

````go
	args = append(args, spec.Image)
	return append(args, spec.Command...)
}
````

Replace it with this exact text:

````go
	args = append(args, spec.Image)
	return append(args, spec.Command...)
}

// buildArgs translates a build plan into `docker build` arguments for the
// build context written to dir.
func buildArgs(spec plan.BuildSpec, dir string) []string {
	args := []string{"build", "-t", spec.Image}
	if spec.NoCache {
		args = append(args, "--no-cache")
	}
	return append(args, "-f", filepath.Join(dir, spec.Dockerfile), dir)
}
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/docker/args.go
+++ b/internal/docker/args.go
@@ -40,3 +40,13 @@
 	args = append(args, spec.Image)
 	return append(args, spec.Command...)
 }
+
+// buildArgs translates a build plan into `docker build` arguments for the
+// build context written to dir.
+func buildArgs(spec plan.BuildSpec, dir string) []string {
+	args := []string{"build", "-t", spec.Image}
+	if spec.NoCache {
+		args = append(args, "--no-cache")
+	}
+	return append(args, "-f", filepath.Join(dir, spec.Dockerfile), dir)
+}
````

#### File 3 of 8: `internal/docker/client.go`

Imports: added `"io/fs"`, `"path/filepath"`. (The edits below already contain these import changes.)

There are 3 edits in this file. They are listed from the bottom of the file to the top. Do them in the order shown: each edit only changes lines below the next one, so the line numbers stay correct. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 3: lines 52-57.**

Find this exact text (it appears exactly once in the file):

````go
	return c.exec.Run(ctx, system.Command{Path: "docker", Args: []string{"image", "inspect", image}}) == nil
}

// Run starts an interactive, disposable container and waits for it to exit.
// A non-zero container exit is returned unchanged so its code can be passed on.
func (c *Client) Run(ctx context.Context, spec plan.RunSpec) error {
````

Replace it with this exact text:

````go
	return c.exec.Run(ctx, system.Command{Path: "docker", Args: []string{"image", "inspect", image}}) == nil
}

// Build writes the build context to a temporary directory, runs docker build,
// and removes the directory again.
func (c *Client) Build(ctx context.Context, spec plan.BuildSpec) error {
	if err := c.Check(ctx); err != nil {
		return err
	}
	docker, err := c.exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("Docker is not installed or not on PATH: %w", err)
	}
	dir, err := c.fs.MkdirTemp("", "caged-runtime-")
	if err != nil {
		return fmt.Errorf("create temporary build directory: %w", err)
	}
	defer c.fs.RemoveAll(dir)
	for _, file := range spec.Files {
		if err := c.fs.WriteFile(filepath.Join(dir, file.Name), file.Data, file.Mode); err != nil {
			return fmt.Errorf("write embedded %s: %w", file.Label, err)
		}
	}
	return c.exec.Run(ctx, system.Command{
		Path:   docker,
		Args:   buildArgs(spec, dir),
		Stdin:  c.stdio.In,
		Stdout: c.stdio.Out,
		Stderr: c.stdio.Err,
	})
}

// Run starts an interactive, disposable container and waits for it to exit.
// A non-zero container exit is returned unchanged so its code can be passed on.
func (c *Client) Run(ctx context.Context, spec plan.RunSpec) error {
````

**Edit 2 of 3: lines 17-33.**

Find this exact text (it appears exactly once in the file):

````go
	Run(ctx context.Context, cmd system.Command) error
}

// Client drives the docker CLI.
type Client struct {
	exec     Executor
	platform system.Platform
	stdio    system.Stdio
}

// NewClient returns a Client that runs docker through exec, applies the
// rules for platform, and connects containers to stdio.
func NewClient(exec Executor, platform system.Platform, stdio system.Stdio) *Client {
	return &Client{exec: exec, platform: platform, stdio: stdio}
}

// Check distinguishes a missing docker CLI from a stopped Docker daemon.
````

Replace it with this exact text:

````go
	Run(ctx context.Context, cmd system.Command) error
}

// FileSystem holds the temporary build context. system.FS is the production implementation.
type FileSystem interface {
	MkdirTemp(dir, pattern string) (string, error)
	WriteFile(name string, data []byte, perm fs.FileMode) error
	RemoveAll(path string) error
}

// Client drives the docker CLI.
type Client struct {
	exec     Executor
	fs       FileSystem
	platform system.Platform
	stdio    system.Stdio
}

// NewClient returns a Client that runs docker through exec, writes build
// contexts through fsys, applies the rules for platform, and connects
// containers to stdio.
func NewClient(exec Executor, fsys FileSystem, platform system.Platform, stdio system.Stdio) *Client {
	return &Client{exec: exec, fs: fsys, platform: platform, stdio: stdio}
}

// Check distinguishes a missing docker CLI from a stopped Docker daemon.
````

**Edit 3 of 3: lines 5-10.**

Find this exact text (it appears exactly once in the file):

````go
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/JeremyOrr4/caged/internal/plan"
````

Replace it with this exact text:

````go
	"context"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/JeremyOrr4/caged/internal/plan"
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/docker/client.go
+++ b/internal/docker/client.go
@@ -5,6 +5,8 @@
 	"context"
 	"fmt"
 	"io"
+	"io/fs"
+	"path/filepath"
 	"strings"
 
 	"github.com/JeremyOrr4/caged/internal/plan"
@@ -17,17 +19,26 @@
 	Run(ctx context.Context, cmd system.Command) error
 }
 
+// FileSystem holds the temporary build context. system.FS is the production implementation.
+type FileSystem interface {
+	MkdirTemp(dir, pattern string) (string, error)
+	WriteFile(name string, data []byte, perm fs.FileMode) error
+	RemoveAll(path string) error
+}
+
 // Client drives the docker CLI.
 type Client struct {
 	exec     Executor
+	fs       FileSystem
 	platform system.Platform
 	stdio    system.Stdio
 }
 
-// NewClient returns a Client that runs docker through exec, applies the
-// rules for platform, and connects containers to stdio.
-func NewClient(exec Executor, platform system.Platform, stdio system.Stdio) *Client {
-	return &Client{exec: exec, platform: platform, stdio: stdio}
+// NewClient returns a Client that runs docker through exec, writes build
+// contexts through fsys, applies the rules for platform, and connects
+// containers to stdio.
+func NewClient(exec Executor, fsys FileSystem, platform system.Platform, stdio system.Stdio) *Client {
+	return &Client{exec: exec, fs: fsys, platform: platform, stdio: stdio}
 }
 
 // Check distinguishes a missing docker CLI from a stopped Docker daemon.
@@ -52,6 +63,35 @@
 	return c.exec.Run(ctx, system.Command{Path: "docker", Args: []string{"image", "inspect", image}}) == nil
 }
 
+// Build writes the build context to a temporary directory, runs docker build,
+// and removes the directory again.
+func (c *Client) Build(ctx context.Context, spec plan.BuildSpec) error {
+	if err := c.Check(ctx); err != nil {
+		return err
+	}
+	docker, err := c.exec.LookPath("docker")
+	if err != nil {
+		return fmt.Errorf("Docker is not installed or not on PATH: %w", err)
+	}
+	dir, err := c.fs.MkdirTemp("", "caged-runtime-")
+	if err != nil {
+		return fmt.Errorf("create temporary build directory: %w", err)
+	}
+	defer c.fs.RemoveAll(dir)
+	for _, file := range spec.Files {
+		if err := c.fs.WriteFile(filepath.Join(dir, file.Name), file.Data, file.Mode); err != nil {
+			return fmt.Errorf("write embedded %s: %w", file.Label, err)
+		}
+	}
+	return c.exec.Run(ctx, system.Command{
+		Path:   docker,
+		Args:   buildArgs(spec, dir),
+		Stdin:  c.stdio.In,
+		Stdout: c.stdio.Out,
+		Stderr: c.stdio.Err,
+	})
+}
+
 // Run starts an interactive, disposable container and waits for it to exit.
 // A non-zero container exit is returned unchanged so its code can be passed on.
 func (c *Client) Run(ctx context.Context, spec plan.RunSpec) error {
````

#### File 4 of 8: `internal/docker/client_test.go`

Imports: added `"io/fs"`, `"path/filepath"`. (The edits below already contain these import changes.)

There are 11 edits in this file. They are listed from the bottom of the file to the top. Do them in the order shown: each edit only changes lines below the next one, so the line numbers stay correct. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 11: lines 218-220.**

Find this exact text (it appears exactly once in the file):

````go
		t.Errorf("explainRunError() = %v, want the error unchanged", got)
	}
}
````

Replace it with this exact text:

````go
		t.Errorf("explainRunError() = %v, want the error unchanged", got)
	}
}

func testBuildSpec(noCache bool) plan.BuildSpec {
	return plan.BuildSpec{
		Image:      "caged-runtime:latest",
		Dockerfile: "Dockerfile",
		NoCache:    noCache,
		Files: []plan.File{
			{Name: "Dockerfile", Label: "Dockerfile", Data: []byte("FROM scratch\n"), Mode: 0o644},
			{Name: "caged-entrypoint.sh", Label: "entrypoint", Data: []byte("#!/bin/sh\n"), Mode: 0o755},
		},
	}
}

func TestBuildWritesContextRunsDockerBuildAndCleansUp(t *testing.T) {
	t.Parallel()

	for _, noCache := range []bool{false, true} {
		exec, fsys := &fakeExecutor{}, &fakeFS{}
		stdio := system.Stdio{In: strings.NewReader(""), Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
		if err := NewClient(exec, fsys, linux, stdio).Build(context.Background(), testBuildSpec(noCache)); err != nil {
			t.Fatal(err)
		}
		wantFiles := map[string]writtenFile{
			"/tmp/caged-runtime-123/Dockerfile":          {data: "FROM scratch\n", mode: 0o644},
			"/tmp/caged-runtime-123/caged-entrypoint.sh": {data: "#!/bin/sh\n", mode: 0o755},
		}
		if !reflect.DeepEqual(fsys.written, wantFiles) {
			t.Fatalf("written = %+v, want %+v", fsys.written, wantFiles)
		}
		if len(exec.commands) != 2 || exec.commands[0].Args[0] != "info" {
			t.Fatalf("commands = %+v, want docker info then docker build", exec.commands)
		}
		build := exec.commands[1]
		wantArgs := []string{"build", "-t", "caged-runtime:latest", "-f", "/tmp/caged-runtime-123/Dockerfile", "/tmp/caged-runtime-123"}
		if noCache {
			wantArgs = []string{"build", "-t", "caged-runtime:latest", "--no-cache", "-f", "/tmp/caged-runtime-123/Dockerfile", "/tmp/caged-runtime-123"}
		}
		if build.Path != "/usr/local/bin/docker" || !reflect.DeepEqual(build.Args, wantArgs) {
			t.Fatalf("build = %+v, want args %q", build, wantArgs)
		}
		if build.Stdin != stdio.In || build.Stdout != stdio.Out || build.Stderr != stdio.Err {
			t.Fatal("docker build is not connected to the client's stdio")
		}
		if !reflect.DeepEqual(fsys.removed, []string{"/tmp/caged-runtime-123"}) {
			t.Fatalf("removed = %v, want the build directory", fsys.removed)
		}
	}
}

func TestBuildErrors(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")
	t.Run("temporary directory", func(t *testing.T) {
		t.Parallel()
		exec := &fakeExecutor{}
		err := NewClient(exec, &fakeFS{mkdirErr: boom}, linux, system.Stdio{}).Build(context.Background(), testBuildSpec(false))
		if err == nil || err.Error() != "create temporary build directory: boom" || len(exec.commands) != 1 {
			t.Fatalf("Build() error = %v, commands = %+v", err, exec.commands)
		}
	})
	t.Run("write uses the file label", func(t *testing.T) {
		t.Parallel()
		fsys := &fakeFS{writeErr: map[string]error{"/tmp/caged-runtime-123/caged-entrypoint.sh": boom}}
		err := NewClient(&fakeExecutor{}, fsys, linux, system.Stdio{}).Build(context.Background(), testBuildSpec(false))
		if err == nil || err.Error() != "write embedded entrypoint: boom" || len(fsys.removed) != 1 {
			t.Fatalf("Build() error = %v, removed = %v", err, fsys.removed)
		}
	})
	t.Run("docker build failure is returned unchanged", func(t *testing.T) {
		t.Parallel()
		fsys := &fakeFS{}
		exec := &fakeExecutor{run: func(cmd system.Command) error {
			if cmd.Args[0] == "build" {
				return boom
			}
			return nil
		}}
		if err := NewClient(exec, fsys, linux, system.Stdio{}).Build(context.Background(), testBuildSpec(false)); err != boom || len(fsys.removed) != 1 {
			t.Fatalf("Build() error = %v, removed = %v", err, fsys.removed)
		}
	})
	t.Run("check failure creates nothing", func(t *testing.T) {
		t.Parallel()
		fsys := &fakeFS{}
		err := NewClient(&fakeExecutor{lookPathErr: boom}, fsys, linux, system.Stdio{}).Build(context.Background(), testBuildSpec(false))
		if err == nil || !strings.HasPrefix(err.Error(), "Docker is not installed or not on PATH") || fsys.written != nil || fsys.removed != nil {
			t.Fatalf("Build() error = %v, fs = %+v", err, fsys)
		}
	})
	t.Run("docker disappears after the check", func(t *testing.T) {
		t.Parallel()
		err := NewClient(&fakeExecutor{lookPathFailAfter: 1}, &fakeFS{}, linux, system.Stdio{}).Build(context.Background(), testBuildSpec(false))
		if err == nil || err.Error() != `Docker is not installed or not on PATH: exec: "docker": executable file not found in $PATH` {
			t.Fatalf("Build() error = %v", err)
		}
	})
}
````

**Edit 2 of 11: lines 181-187.**

Find this exact text (it appears exactly once in the file):

````go
	t.Run("docker disappears after the check", func(t *testing.T) {
		t.Parallel()
		exec := &fakeExecutor{lookPathFailAfter: 1}
		err := NewClient(exec, linux, system.Stdio{}).Run(context.Background(), plan.RunSpec{})
		if err == nil || err.Error() != `docker not found, is Docker installed and running?: exec: "docker": executable file not found in $PATH` {
			t.Fatalf("Run() error = %v", err)
		}
````

Replace it with this exact text:

````go
	t.Run("docker disappears after the check", func(t *testing.T) {
		t.Parallel()
		exec := &fakeExecutor{lookPathFailAfter: 1}
		err := NewClient(exec, &fakeFS{}, linux, system.Stdio{}).Run(context.Background(), plan.RunSpec{})
		if err == nil || err.Error() != `docker not found, is Docker installed and running?: exec: "docker": executable file not found in $PATH` {
			t.Fatalf("Run() error = %v", err)
		}
````

**Edit 3 of 11: lines 171-177.**

Find this exact text (it appears exactly once in the file):

````go
	t.Run("check failure stops before running", func(t *testing.T) {
		t.Parallel()
		exec := &fakeExecutor{lookPathErr: errors.New("x")}
		if err := NewClient(exec, linux, system.Stdio{}).Run(context.Background(), plan.RunSpec{}); err == nil || !strings.HasPrefix(err.Error(), "Docker is not installed") {
			t.Fatalf("Run() error = %v", err)
		}
		if len(exec.commands) != 0 {
````

Replace it with this exact text:

````go
	t.Run("check failure stops before running", func(t *testing.T) {
		t.Parallel()
		exec := &fakeExecutor{lookPathErr: errors.New("x")}
		if err := NewClient(exec, &fakeFS{}, linux, system.Stdio{}).Run(context.Background(), plan.RunSpec{}); err == nil || !strings.HasPrefix(err.Error(), "Docker is not installed") {
			t.Fatalf("Run() error = %v", err)
		}
		if len(exec.commands) != 0 {
````

**Edit 4 of 11: lines 155-168.**

Find this exact text (it appears exactly once in the file):

````go
	}
	t.Run("linux returns the exit error unchanged", func(t *testing.T) {
		t.Parallel()
		err := NewClient(&fakeExecutor{run: mountsDenied}, linux, system.Stdio{Err: io.Discard}).Run(context.Background(), plan.RunSpec{})
		if err != exitErr {
			t.Fatalf("Run() error = %v, want the original exit error", err)
		}
	})
	t.Run("macOS explains Docker Desktop file sharing", func(t *testing.T) {
		t.Parallel()
		err := NewClient(&fakeExecutor{run: mountsDenied}, darwin, system.Stdio{Err: io.Discard}).Run(context.Background(), plan.RunSpec{})
		want := "Docker could not mount the project path; in Docker Desktop, add its parent directory under Settings > Resources > File Sharing: exit status 125"
		if err == nil || err.Error() != want || !errors.Is(err, exitErr) {
			t.Fatalf("Run() error = %v, want %q wrapping the exit error", err, want)
````

Replace it with this exact text:

````go
	}
	t.Run("linux returns the exit error unchanged", func(t *testing.T) {
		t.Parallel()
		err := NewClient(&fakeExecutor{run: mountsDenied}, &fakeFS{}, linux, system.Stdio{Err: io.Discard}).Run(context.Background(), plan.RunSpec{})
		if err != exitErr {
			t.Fatalf("Run() error = %v, want the original exit error", err)
		}
	})
	t.Run("macOS explains Docker Desktop file sharing", func(t *testing.T) {
		t.Parallel()
		err := NewClient(&fakeExecutor{run: mountsDenied}, &fakeFS{}, darwin, system.Stdio{Err: io.Discard}).Run(context.Background(), plan.RunSpec{})
		want := "Docker could not mount the project path; in Docker Desktop, add its parent directory under Settings > Resources > File Sharing: exit status 125"
		if err == nil || err.Error() != want || !errors.Is(err, exitErr) {
			t.Fatalf("Run() error = %v, want %q wrapping the exit error", err, want)
````

**Edit 5 of 11: lines 123-129.**

Find this exact text (it appears exactly once in the file):

````go
		return nil
	}}
	spec := plan.RunSpec{Image: "img", Command: []string{"claude"}, ReturnToShell: true}
	if err := NewClient(exec, linux, system.Stdio{In: stdin, Out: &stdout, Err: &stderr}).Run(context.Background(), spec); err != nil {
		t.Fatal(err)
	}
	if len(exec.commands) != 2 {
````

Replace it with this exact text:

````go
		return nil
	}}
	spec := plan.RunSpec{Image: "img", Command: []string{"claude"}, ReturnToShell: true}
	if err := NewClient(exec, &fakeFS{}, linux, system.Stdio{In: stdin, Out: &stdout, Err: &stderr}).Run(context.Background(), spec); err != nil {
		t.Fatal(err)
	}
	if len(exec.commands) != 2 {
````

**Edit 6 of 11: lines 106-112.**

Find this exact text (it appears exactly once in the file):

````go
		t.Fatalf("commands = %+v, want %+v", present.commands, want)
	}
	missing := &fakeExecutor{run: func(system.Command) error { return errors.New("exit status 1") }}
	if NewClient(missing, linux, system.Stdio{}).ImageExists(context.Background(), "caged-runtime:latest") {
		t.Fatal("ImageExists() = true, want false")
	}
}
````

Replace it with this exact text:

````go
		t.Fatalf("commands = %+v, want %+v", present.commands, want)
	}
	missing := &fakeExecutor{run: func(system.Command) error { return errors.New("exit status 1") }}
	if NewClient(missing, &fakeFS{}, linux, system.Stdio{}).ImageExists(context.Background(), "caged-runtime:latest") {
		t.Fatal("ImageExists() = true, want false")
	}
}
````

**Edit 7 of 11: lines 98-104.**

Find this exact text (it appears exactly once in the file):

````go
	t.Parallel()

	present := &fakeExecutor{}
	if !NewClient(present, linux, system.Stdio{}).ImageExists(context.Background(), "caged-runtime:latest") {
		t.Fatal("ImageExists() = false, want true")
	}
	want := system.Command{Path: "docker", Args: []string{"image", "inspect", "caged-runtime:latest"}}
````

Replace it with this exact text:

````go
	t.Parallel()

	present := &fakeExecutor{}
	if !NewClient(present, &fakeFS{}, linux, system.Stdio{}).ImageExists(context.Background(), "caged-runtime:latest") {
		t.Fatal("ImageExists() = false, want true")
	}
	want := system.Command{Path: "docker", Args: []string{"image", "inspect", "caged-runtime:latest"}}
````

**Edit 8 of 11: lines 86-92.**

Find this exact text (it appears exactly once in the file):

````go
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := NewClient(tt.exec, tt.platform, system.Stdio{}).Check(context.Background())
			if err == nil || err.Error() != tt.want {
				t.Fatalf("Check() error = %v, want %q", err, tt.want)
			}
````

Replace it with this exact text:

````go
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := NewClient(tt.exec, &fakeFS{}, tt.platform, system.Stdio{}).Check(context.Background())
			if err == nil || err.Error() != tt.want {
				t.Fatalf("Check() error = %v, want %q", err, tt.want)
			}
````

**Edit 9 of 11: lines 49-55.**

Find this exact text (it appears exactly once in the file):

````go
	t.Parallel()

	exec := &fakeExecutor{}
	if err := NewClient(exec, darwin, system.Stdio{}).Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(exec.commands) != 1 {
````

Replace it with this exact text:

````go
	t.Parallel()

	exec := &fakeExecutor{}
	if err := NewClient(exec, &fakeFS{}, darwin, system.Stdio{}).Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(exec.commands) != 1 {
````

**Edit 10 of 11: lines 40-45.**

Find this exact text (it appears exactly once in the file):

````go
	return nil
}

var (
	linux  = system.Platform{OS: "linux", Arch: "amd64", UID: 1000, GID: 1001}
	darwin = system.Platform{OS: "darwin", Arch: "arm64", UID: 501, GID: 20}
````

Replace it with this exact text:

````go
	return nil
}

// fakeFS records the temporary build context instead of touching the disk.
type fakeFS struct {
	mkdirErr error
	writeErr map[string]error // by file name
	written  map[string]writtenFile
	removed  []string
}

type writtenFile struct {
	data string
	mode fs.FileMode
}

func (f *fakeFS) MkdirTemp(dir, pattern string) (string, error) {
	if f.mkdirErr != nil {
		return "", f.mkdirErr
	}
	return filepath.Join("/tmp", pattern+"123"), nil
}

func (f *fakeFS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	if err := f.writeErr[name]; err != nil {
		return err
	}
	if f.written == nil {
		f.written = map[string]writtenFile{}
	}
	f.written[name] = writtenFile{data: string(data), mode: perm}
	return nil
}

func (f *fakeFS) RemoveAll(path string) error {
	f.removed = append(f.removed, path)
	return nil
}

var (
	linux  = system.Platform{OS: "linux", Arch: "amd64", UID: 1000, GID: 1001}
	darwin = system.Platform{OS: "darwin", Arch: "arm64", UID: 501, GID: 20}
````

**Edit 11 of 11: lines 5-10.**

Find this exact text (it appears exactly once in the file):

````go
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
````

Replace it with this exact text:

````go
	"context"
	"errors"
	"io"
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/docker/client_test.go
+++ b/internal/docker/client_test.go
@@ -5,6 +5,8 @@
 	"context"
 	"errors"
 	"io"
+	"io/fs"
+	"path/filepath"
 	"reflect"
 	"strings"
 	"testing"
@@ -40,6 +42,42 @@
 	return nil
 }
 
+// fakeFS records the temporary build context instead of touching the disk.
+type fakeFS struct {
+	mkdirErr error
+	writeErr map[string]error // by file name
+	written  map[string]writtenFile
+	removed  []string
+}
+
+type writtenFile struct {
+	data string
+	mode fs.FileMode
+}
+
+func (f *fakeFS) MkdirTemp(dir, pattern string) (string, error) {
+	if f.mkdirErr != nil {
+		return "", f.mkdirErr
+	}
+	return filepath.Join("/tmp", pattern+"123"), nil
+}
+
+func (f *fakeFS) WriteFile(name string, data []byte, perm fs.FileMode) error {
+	if err := f.writeErr[name]; err != nil {
+		return err
+	}
+	if f.written == nil {
+		f.written = map[string]writtenFile{}
+	}
+	f.written[name] = writtenFile{data: string(data), mode: perm}
+	return nil
+}
+
+func (f *fakeFS) RemoveAll(path string) error {
+	f.removed = append(f.removed, path)
+	return nil
+}
+
 var (
 	linux  = system.Platform{OS: "linux", Arch: "amd64", UID: 1000, GID: 1001}
 	darwin = system.Platform{OS: "darwin", Arch: "arm64", UID: 501, GID: 20}
@@ -49,7 +87,7 @@
 	t.Parallel()
 
 	exec := &fakeExecutor{}
-	if err := NewClient(exec, darwin, system.Stdio{}).Check(context.Background()); err != nil {
+	if err := NewClient(exec, &fakeFS{}, darwin, system.Stdio{}).Check(context.Background()); err != nil {
 		t.Fatal(err)
 	}
 	if len(exec.commands) != 1 {
@@ -86,7 +124,7 @@
 	for _, tt := range tests {
 		t.Run(tt.name, func(t *testing.T) {
 			t.Parallel()
-			err := NewClient(tt.exec, tt.platform, system.Stdio{}).Check(context.Background())
+			err := NewClient(tt.exec, &fakeFS{}, tt.platform, system.Stdio{}).Check(context.Background())
 			if err == nil || err.Error() != tt.want {
 				t.Fatalf("Check() error = %v, want %q", err, tt.want)
 			}
@@ -98,7 +136,7 @@
 	t.Parallel()
 
 	present := &fakeExecutor{}
-	if !NewClient(present, linux, system.Stdio{}).ImageExists(context.Background(), "caged-runtime:latest") {
+	if !NewClient(present, &fakeFS{}, linux, system.Stdio{}).ImageExists(context.Background(), "caged-runtime:latest") {
 		t.Fatal("ImageExists() = false, want true")
 	}
 	want := system.Command{Path: "docker", Args: []string{"image", "inspect", "caged-runtime:latest"}}
@@ -106,7 +144,7 @@
 		t.Fatalf("commands = %+v, want %+v", present.commands, want)
 	}
 	missing := &fakeExecutor{run: func(system.Command) error { return errors.New("exit status 1") }}
-	if NewClient(missing, linux, system.Stdio{}).ImageExists(context.Background(), "caged-runtime:latest") {
+	if NewClient(missing, &fakeFS{}, linux, system.Stdio{}).ImageExists(context.Background(), "caged-runtime:latest") {
 		t.Fatal("ImageExists() = true, want false")
 	}
 }
@@ -123,7 +161,7 @@
 		return nil
 	}}
 	spec := plan.RunSpec{Image: "img", Command: []string{"claude"}, ReturnToShell: true}
-	if err := NewClient(exec, linux, system.Stdio{In: stdin, Out: &stdout, Err: &stderr}).Run(context.Background(), spec); err != nil {
+	if err := NewClient(exec, &fakeFS{}, linux, system.Stdio{In: stdin, Out: &stdout, Err: &stderr}).Run(context.Background(), spec); err != nil {
 		t.Fatal(err)
 	}
 	if len(exec.commands) != 2 {
@@ -155,14 +193,14 @@
 	}
 	t.Run("linux returns the exit error unchanged", func(t *testing.T) {
 		t.Parallel()
-		err := NewClient(&fakeExecutor{run: mountsDenied}, linux, system.Stdio{Err: io.Discard}).Run(context.Background(), plan.RunSpec{})
+		err := NewClient(&fakeExecutor{run: mountsDenied}, &fakeFS{}, linux, system.Stdio{Err: io.Discard}).Run(context.Background(), plan.RunSpec{})
 		if err != exitErr {
 			t.Fatalf("Run() error = %v, want the original exit error", err)
 		}
 	})
 	t.Run("macOS explains Docker Desktop file sharing", func(t *testing.T) {
 		t.Parallel()
-		err := NewClient(&fakeExecutor{run: mountsDenied}, darwin, system.Stdio{Err: io.Discard}).Run(context.Background(), plan.RunSpec{})
+		err := NewClient(&fakeExecutor{run: mountsDenied}, &fakeFS{}, darwin, system.Stdio{Err: io.Discard}).Run(context.Background(), plan.RunSpec{})
 		want := "Docker could not mount the project path; in Docker Desktop, add its parent directory under Settings > Resources > File Sharing: exit status 125"
 		if err == nil || err.Error() != want || !errors.Is(err, exitErr) {
 			t.Fatalf("Run() error = %v, want %q wrapping the exit error", err, want)
@@ -171,7 +209,7 @@
 	t.Run("check failure stops before running", func(t *testing.T) {
 		t.Parallel()
 		exec := &fakeExecutor{lookPathErr: errors.New("x")}
-		if err := NewClient(exec, linux, system.Stdio{}).Run(context.Background(), plan.RunSpec{}); err == nil || !strings.HasPrefix(err.Error(), "Docker is not installed") {
+		if err := NewClient(exec, &fakeFS{}, linux, system.Stdio{}).Run(context.Background(), plan.RunSpec{}); err == nil || !strings.HasPrefix(err.Error(), "Docker is not installed") {
 			t.Fatalf("Run() error = %v", err)
 		}
 		if len(exec.commands) != 0 {
@@ -181,7 +219,7 @@
 	t.Run("docker disappears after the check", func(t *testing.T) {
 		t.Parallel()
 		exec := &fakeExecutor{lookPathFailAfter: 1}
-		err := NewClient(exec, linux, system.Stdio{}).Run(context.Background(), plan.RunSpec{})
+		err := NewClient(exec, &fakeFS{}, linux, system.Stdio{}).Run(context.Background(), plan.RunSpec{})
 		if err == nil || err.Error() != `docker not found, is Docker installed and running?: exec: "docker": executable file not found in $PATH` {
 			t.Fatalf("Run() error = %v", err)
 		}
@@ -218,3 +256,101 @@
 		t.Errorf("explainRunError() = %v, want the error unchanged", got)
 	}
 }
+
+func testBuildSpec(noCache bool) plan.BuildSpec {
+	return plan.BuildSpec{
+		Image:      "caged-runtime:latest",
+		Dockerfile: "Dockerfile",
+		NoCache:    noCache,
+		Files: []plan.File{
+			{Name: "Dockerfile", Label: "Dockerfile", Data: []byte("FROM scratch\n"), Mode: 0o644},
+			{Name: "caged-entrypoint.sh", Label: "entrypoint", Data: []byte("#!/bin/sh\n"), Mode: 0o755},
+		},
+	}
+}
+
+func TestBuildWritesContextRunsDockerBuildAndCleansUp(t *testing.T) {
+	t.Parallel()
+
+	for _, noCache := range []bool{false, true} {
+		exec, fsys := &fakeExecutor{}, &fakeFS{}
+		stdio := system.Stdio{In: strings.NewReader(""), Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
+		if err := NewClient(exec, fsys, linux, stdio).Build(context.Background(), testBuildSpec(noCache)); err != nil {
+			t.Fatal(err)
+		}
+		wantFiles := map[string]writtenFile{
+			"/tmp/caged-runtime-123/Dockerfile":          {data: "FROM scratch\n", mode: 0o644},
+			"/tmp/caged-runtime-123/caged-entrypoint.sh": {data: "#!/bin/sh\n", mode: 0o755},
+		}
+		if !reflect.DeepEqual(fsys.written, wantFiles) {
+			t.Fatalf("written = %+v, want %+v", fsys.written, wantFiles)
+		}
+		if len(exec.commands) != 2 || exec.commands[0].Args[0] != "info" {
+			t.Fatalf("commands = %+v, want docker info then docker build", exec.commands)
+		}
+		build := exec.commands[1]
+		wantArgs := []string{"build", "-t", "caged-runtime:latest", "-f", "/tmp/caged-runtime-123/Dockerfile", "/tmp/caged-runtime-123"}
+		if noCache {
+			wantArgs = []string{"build", "-t", "caged-runtime:latest", "--no-cache", "-f", "/tmp/caged-runtime-123/Dockerfile", "/tmp/caged-runtime-123"}
+		}
+		if build.Path != "/usr/local/bin/docker" || !reflect.DeepEqual(build.Args, wantArgs) {
+			t.Fatalf("build = %+v, want args %q", build, wantArgs)
+		}
+		if build.Stdin != stdio.In || build.Stdout != stdio.Out || build.Stderr != stdio.Err {
+			t.Fatal("docker build is not connected to the client's stdio")
+		}
+		if !reflect.DeepEqual(fsys.removed, []string{"/tmp/caged-runtime-123"}) {
+			t.Fatalf("removed = %v, want the build directory", fsys.removed)
+		}
+	}
+}
+
+func TestBuildErrors(t *testing.T) {
+	t.Parallel()
+
+	boom := errors.New("boom")
+	t.Run("temporary directory", func(t *testing.T) {
+		t.Parallel()
+		exec := &fakeExecutor{}
+		err := NewClient(exec, &fakeFS{mkdirErr: boom}, linux, system.Stdio{}).Build(context.Background(), testBuildSpec(false))
+		if err == nil || err.Error() != "create temporary build directory: boom" || len(exec.commands) != 1 {
+			t.Fatalf("Build() error = %v, commands = %+v", err, exec.commands)
+		}
+	})
+	t.Run("write uses the file label", func(t *testing.T) {
+		t.Parallel()
+		fsys := &fakeFS{writeErr: map[string]error{"/tmp/caged-runtime-123/caged-entrypoint.sh": boom}}
+		err := NewClient(&fakeExecutor{}, fsys, linux, system.Stdio{}).Build(context.Background(), testBuildSpec(false))
+		if err == nil || err.Error() != "write embedded entrypoint: boom" || len(fsys.removed) != 1 {
+			t.Fatalf("Build() error = %v, removed = %v", err, fsys.removed)
+		}
+	})
+	t.Run("docker build failure is returned unchanged", func(t *testing.T) {
+		t.Parallel()
+		fsys := &fakeFS{}
+		exec := &fakeExecutor{run: func(cmd system.Command) error {
+			if cmd.Args[0] == "build" {
+				return boom
+			}
+			return nil
+		}}
+		if err := NewClient(exec, fsys, linux, system.Stdio{}).Build(context.Background(), testBuildSpec(false)); err != boom || len(fsys.removed) != 1 {
+			t.Fatalf("Build() error = %v, removed = %v", err, fsys.removed)
+		}
+	})
+	t.Run("check failure creates nothing", func(t *testing.T) {
+		t.Parallel()
+		fsys := &fakeFS{}
+		err := NewClient(&fakeExecutor{lookPathErr: boom}, fsys, linux, system.Stdio{}).Build(context.Background(), testBuildSpec(false))
+		if err == nil || !strings.HasPrefix(err.Error(), "Docker is not installed or not on PATH") || fsys.written != nil || fsys.removed != nil {
+			t.Fatalf("Build() error = %v, fs = %+v", err, fsys)
+		}
+	})
+	t.Run("docker disappears after the check", func(t *testing.T) {
+		t.Parallel()
+		err := NewClient(&fakeExecutor{lookPathFailAfter: 1}, &fakeFS{}, linux, system.Stdio{}).Build(context.Background(), testBuildSpec(false))
+		if err == nil || err.Error() != `Docker is not installed or not on PATH: exec: "docker": executable file not found in $PATH` {
+			t.Fatalf("Build() error = %v", err)
+		}
+	})
+}
````

#### File 5 of 8: `internal/plan/run.go`

Imports: added `"io/fs"`. (The edits below already contain these import changes.)

There are 2 edits in this file. They are listed from the bottom of the file to the top. Do them in the order shown: each edit only changes lines below the next one, so the line numbers stay correct. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 2: lines 35-40.**

Find this exact text (it appears exactly once in the file):

````go
	ReturnToShell bool
}

// BuildRunSpec combines resolved inputs into a complete container invocation.
func BuildRunSpec(image string, selected *agent.Agent, prompt *string, projectMount *Mount, configMounts []ConfigMount, envVars []string) (RunSpec, error) {
	var mountCopy *Mount
````

Replace it with this exact text:

````go
	ReturnToShell bool
}

// BuildSpec describes an image build from an in-memory build context.
type BuildSpec struct {
	Image      string // tag of the image to build
	Dockerfile string // name of the Dockerfile inside Files
	Files      []File // the build context, written in order
	NoCache    bool
}

// File is one file of an image build context.
type File struct {
	Name  string // file name inside the build context
	Label string // short name used in error messages
	Data  []byte
	Mode  fs.FileMode
}

// BuildRunSpec combines resolved inputs into a complete container invocation.
func BuildRunSpec(image string, selected *agent.Agent, prompt *string, projectMount *Mount, configMounts []ConfigMount, envVars []string) (RunSpec, error) {
	var mountCopy *Mount
````

**Edit 2 of 2: lines 3-8.**

Find this exact text (it appears exactly once in the file):

````go

import (
	"fmt"

	"github.com/JeremyOrr4/caged/internal/agent"
)
````

Replace it with this exact text:

````go

import (
	"fmt"
	"io/fs"

	"github.com/JeremyOrr4/caged/internal/agent"
)
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/plan/run.go
+++ b/internal/plan/run.go
@@ -3,6 +3,7 @@
 
 import (
 	"fmt"
+	"io/fs"
 
 	"github.com/JeremyOrr4/caged/internal/agent"
 )
@@ -35,6 +36,22 @@
 	ReturnToShell bool
 }
 
+// BuildSpec describes an image build from an in-memory build context.
+type BuildSpec struct {
+	Image      string // tag of the image to build
+	Dockerfile string // name of the Dockerfile inside Files
+	Files      []File // the build context, written in order
+	NoCache    bool
+}
+
+// File is one file of an image build context.
+type File struct {
+	Name  string // file name inside the build context
+	Label string // short name used in error messages
+	Data  []byte
+	Mode  fs.FileMode
+}
+
 // BuildRunSpec combines resolved inputs into a complete container invocation.
 func BuildRunSpec(image string, selected *agent.Agent, prompt *string, projectMount *Mount, configMounts []ConfigMount, envVars []string) (RunSpec, error) {
 	var mountCopy *Mount
````

#### File 6 of 8: `internal/runtime/assets.go`

This file was moved from `internal/runtime/build.go` by the commands above. The edits below change the moved file.

Imports: added `"github.com/JeremyOrr4/caged/internal/plan"`; removed `"fmt"`, `"os"`, `"os/exec"`, `"path/filepath"`. (The edits below already contain these import changes.)

**Edit 1 of 1: replace the whole file.** At the start of this step the file has 50 lines (lines 1-50).

The file currently contains exactly this text:

````go
package runtime

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

//go:embed Dockerfile
var runtimeDockerfile []byte

//go:embed caged-entrypoint.sh
var runtimeEntrypoint []byte

// BuildRuntimeImage creates the local image from its embedded runtime recipe.
func BuildRuntimeImage(noCache bool, checkDocker func() error) error {
	if checkDocker == nil {
		return fmt.Errorf("Docker availability check is required")
	}
	if err := checkDocker(); err != nil {
		return err
	}
	docker, err := exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("Docker is not installed or not on PATH: %w", err)
	}
	dir, err := os.MkdirTemp("", "caged-runtime-")
	if err != nil {
		return fmt.Errorf("create temporary build directory: %w", err)
	}
	defer os.RemoveAll(dir)
	dockerfilePath := filepath.Join(dir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, runtimeDockerfile, 0o644); err != nil {
		return fmt.Errorf("write embedded Dockerfile: %w", err)
	}
	entrypointPath := filepath.Join(dir, "caged-entrypoint.sh")
	if err := os.WriteFile(entrypointPath, runtimeEntrypoint, 0o755); err != nil {
		return fmt.Errorf("write embedded entrypoint: %w", err)
	}
	args := []string{"build", "-t", Image}
	if noCache {
		args = append(args, "--no-cache")
	}
	args = append(args, "-f", dockerfilePath, dir)
	cmd := exec.Command(docker, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}
````

Replace the entire contents of the file with exactly this text:

````go
package runtime

import (
	_ "embed"

	"github.com/JeremyOrr4/caged/internal/plan"
)

//go:embed Dockerfile
var runtimeDockerfile string

//go:embed caged-entrypoint.sh
var runtimeEntrypoint string

// BuildSpec describes how to build the runtime image from the embedded
// Dockerfile and entrypoint.
func BuildSpec(noCache bool) plan.BuildSpec {
	return plan.BuildSpec{
		Image:      Image,
		Dockerfile: "Dockerfile",
		NoCache:    noCache,
		Files: []plan.File{
			{Name: "Dockerfile", Label: "Dockerfile", Data: []byte(runtimeDockerfile), Mode: 0o644},
			{Name: "caged-entrypoint.sh", Label: "entrypoint", Data: []byte(runtimeEntrypoint), Mode: 0o755},
		},
	}
}
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/runtime/build.go
+++ b/internal/runtime/assets.go
@@ -2,49 +2,26 @@
 
 import (
 	_ "embed"
-	"fmt"
-	"os"
-	"os/exec"
-	"path/filepath"
+
+	"github.com/JeremyOrr4/caged/internal/plan"
 )
 
 //go:embed Dockerfile
-var runtimeDockerfile []byte
+var runtimeDockerfile string
 
 //go:embed caged-entrypoint.sh
-var runtimeEntrypoint []byte
+var runtimeEntrypoint string
 
-// BuildRuntimeImage creates the local image from its embedded runtime recipe.
-func BuildRuntimeImage(noCache bool, checkDocker func() error) error {
-	if checkDocker == nil {
-		return fmt.Errorf("Docker availability check is required")
-	}
-	if err := checkDocker(); err != nil {
-		return err
-	}
-	docker, err := exec.LookPath("docker")
-	if err != nil {
-		return fmt.Errorf("Docker is not installed or not on PATH: %w", err)
-	}
-	dir, err := os.MkdirTemp("", "caged-runtime-")
-	if err != nil {
-		return fmt.Errorf("create temporary build directory: %w", err)
-	}
-	defer os.RemoveAll(dir)
-	dockerfilePath := filepath.Join(dir, "Dockerfile")
-	if err := os.WriteFile(dockerfilePath, runtimeDockerfile, 0o644); err != nil {
-		return fmt.Errorf("write embedded Dockerfile: %w", err)
-	}
-	entrypointPath := filepath.Join(dir, "caged-entrypoint.sh")
-	if err := os.WriteFile(entrypointPath, runtimeEntrypoint, 0o755); err != nil {
-		return fmt.Errorf("write embedded entrypoint: %w", err)
-	}
-	args := []string{"build", "-t", Image}
-	if noCache {
-		args = append(args, "--no-cache")
+// BuildSpec describes how to build the runtime image from the embedded
+// Dockerfile and entrypoint.
+func BuildSpec(noCache bool) plan.BuildSpec {
+	return plan.BuildSpec{
+		Image:      Image,
+		Dockerfile: "Dockerfile",
+		NoCache:    noCache,
+		Files: []plan.File{
+			{Name: "Dockerfile", Label: "Dockerfile", Data: []byte(runtimeDockerfile), Mode: 0o644},
+			{Name: "caged-entrypoint.sh", Label: "entrypoint", Data: []byte(runtimeEntrypoint), Mode: 0o755},
+		},
 	}
-	args = append(args, "-f", dockerfilePath, dir)
-	cmd := exec.Command(docker, args...)
-	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
-	return cmd.Run()
 }
````

#### File 7 of 8: `internal/runtime/image.go`

Imports: no import lines are added or removed in this file.

There is 1 edit in this file. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 1: lines 1-4.**

Find this exact text (it appears exactly once in the file):

````go
// Package runtime identifies the locally built caged runtime image.
package runtime

const Image = "caged-runtime:latest"
````

Replace it with this exact text:

````go
// Package runtime describes the caged runtime image: its tag and the embedded
// files it is built from. It does not run Docker itself.
package runtime

// Image is the tag of the locally built runtime image.
const Image = "caged-runtime:latest"
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/runtime/image.go
+++ b/internal/runtime/image.go
@@ -1,4 +1,6 @@
-// Package runtime identifies the locally built caged runtime image.
+// Package runtime describes the caged runtime image: its tag and the embedded
+// files it is built from. It does not run Docker itself.
 package runtime
 
+// Image is the tag of the locally built runtime image.
 const Image = "caged-runtime:latest"
````

#### File 8 of 8: `internal/runtime/runtime_test.go`

Imports: added `"reflect"`, `"github.com/JeremyOrr4/caged/internal/plan"`. (The edits below already contain these import changes.)

There are 2 edits in this file. They are listed from the bottom of the file to the top. Do them in the order shown: each edit only changes lines below the next one, so the line numbers stay correct. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 2: lines 112-116.**

Find this exact text (it appears exactly once in the file):

````go
		if !strings.Contains(dockerfile, expected) {
			t.Errorf("runtime Dockerfile missing GitHub CLI setup %q", expected)
		}
	}
}
````

Replace it with this exact text:

````go
		if !strings.Contains(dockerfile, expected) {
			t.Errorf("runtime Dockerfile missing GitHub CLI setup %q", expected)
		}
	}
}

func TestBuildSpecDescribesEmbeddedBuildContext(t *testing.T) {
	t.Parallel()

	want := plan.BuildSpec{
		Image:      "caged-runtime:latest",
		Dockerfile: "Dockerfile",
		NoCache:    true,
		Files: []plan.File{
			{Name: "Dockerfile", Label: "Dockerfile", Data: []byte(runtimeDockerfile), Mode: 0o644},
			{Name: "caged-entrypoint.sh", Label: "entrypoint", Data: []byte(runtimeEntrypoint), Mode: 0o755},
		},
	}
	got := BuildSpec(true)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildSpec(true) = %+v, want %+v", got, want)
	}
	got.Files[0].Data[0] = 'X'
	if BuildSpec(false).Files[0].Data[0] == 'X' || BuildSpec(false).NoCache {
		t.Fatal("BuildSpec must return fresh data and honor noCache")
	}
}
````

**Edit 2 of 2: lines 1-10.**

Find this exact text (it appears exactly once in the file):

````go
package runtime

import (
	"strings"
	"testing"
)

func TestEmbeddedDockerfileIsPresent(t *testing.T) {
	t.Parallel()

````

Replace it with this exact text:

````go
package runtime

import (
	"reflect"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/plan"
)

func TestEmbeddedDockerfileIsPresent(t *testing.T) {
	t.Parallel()

````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/runtime/runtime_test.go
+++ b/internal/runtime/runtime_test.go
@@ -1,8 +1,11 @@
 package runtime
 
 import (
+	"reflect"
 	"strings"
 	"testing"
+
+	"github.com/JeremyOrr4/caged/internal/plan"
 )
 
 func TestEmbeddedDockerfileIsPresent(t *testing.T) {
@@ -114,3 +117,25 @@
 		}
 	}
 }
+
+func TestBuildSpecDescribesEmbeddedBuildContext(t *testing.T) {
+	t.Parallel()
+
+	want := plan.BuildSpec{
+		Image:      "caged-runtime:latest",
+		Dockerfile: "Dockerfile",
+		NoCache:    true,
+		Files: []plan.File{
+			{Name: "Dockerfile", Label: "Dockerfile", Data: []byte(runtimeDockerfile), Mode: 0o644},
+			{Name: "caged-entrypoint.sh", Label: "entrypoint", Data: []byte(runtimeEntrypoint), Mode: 0o755},
+		},
+	}
+	got := BuildSpec(true)
+	if !reflect.DeepEqual(got, want) {
+		t.Fatalf("BuildSpec(true) = %+v, want %+v", got, want)
+	}
+	got.Files[0].Data[0] = 'X'
+	if BuildSpec(false).Files[0].Data[0] == 'X' || BuildSpec(false).NoCache {
+		t.Fatal("BuildSpec must return fresh data and honor noCache")
+	}
+}
````

### Verify

Run these verification commands from the repository root:

````bash
gofmt -l .
go build ./...
go vet ./...
go test -count=1 ./...
````

Expected result:

- `gofmt -l .` prints nothing.
- `go build ./...` prints nothing.
- `go vet ./...` prints nothing.
- `go test -count=1 ./...` prints the lines below. The numbers of seconds at the end of the `ok` lines will be different on your machine, and lines may come in a different order. Every package must say `ok` or `[no test files]`:

````text
ok  	github.com/JeremyOrr4/caged/cmd/caged	0.535s
ok  	github.com/JeremyOrr4/caged/internal/agent	0.004s
ok  	github.com/JeremyOrr4/caged/internal/app	0.004s
ok  	github.com/JeremyOrr4/caged/internal/cli	0.006s
ok  	github.com/JeremyOrr4/caged/internal/docker	0.008s
ok  	github.com/JeremyOrr4/caged/internal/hostconfig	0.012s
ok  	github.com/JeremyOrr4/caged/internal/plan	0.007s
ok  	github.com/JeremyOrr4/caged/internal/project	0.006s
ok  	github.com/JeremyOrr4/caged/internal/runtime	0.003s
ok  	github.com/JeremyOrr4/caged/internal/system	0.005s
````

Count the passing tests and subtests with `go test -count=1 -v ./... 2>&1 | grep -c -- "--- PASS"`. On Linux the expected number is **178** (on macOS the Linux golden test is skipped, so the count there is lower by 52 (51 golden subtests plus their parent test)).

Run this command to compute the fingerprint of the whole repository (on macOS use the second command):

````bash
find . -type f -not -path './.git/*' -not -name REFACTOR_INSTRUCTIONS.md -print0 | LC_ALL=C sort -z | xargs -0 sha256sum | sha256sum
````

````bash
find . -type f -not -path './.git/*' -not -name REFACTOR_INSTRUCTIONS.md -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256 | shasum -a 256
````

Expected output (exactly this line):

````text
8e3c0dbdce309762c99b36c351c7bd725169badd732bfba19f8a4aca85f1b2c1  -
````

**If it fails:**

- Problem: Golden test `build` fails on the `build context Dockerfile:` checksum line.
  Fix: The embedded files must be written byte for byte. `assets.go` must keep the two `//go:embed` lines directly above their `var` lines, with no blank line in between.
- Problem: `go build` says `not enough arguments in call to docker.NewClient`.
  Fix: `NewClient` now takes four arguments. In `internal/app/run.go` the call must be `docker.NewClient(system.Executor{}, system.FS{}, system.CurrentPlatform(), system.CurrentStdio())`.
- Problem: the fingerprint is different but every test passes.
  Fix: some file differs from these instructions by a character, often a tab turned into spaces or a missing final newline. Run `gofmt -w .` (it fixes Go indentation), then check the non-Go files of this step again.

### Checkpoint

At this point, the repo should have these 101 files (not counting `.git/` and this instructions file):

- `./` (5 files): `.gitignore`, `Makefile`, `README.md`, `go.mod`, `go.sum`
- `cmd/caged/` (2 files): `golden_test.go`, `main.go`
- `cmd/caged/testdata/golden/linux/` (51 files): `agent-claude.golden`, `agent-codex.golden`, `agent-copilot.golden`, `agent-gemini.golden`, `agent-gh.golden`, `agent-pi.golden`, `agent-uppercase.golden`, `build-daemon-down.golden`, `build-help.golden`, `build-no-cache.golden`, `build-rejects-args.golden`, `build.golden`, `complete-agents.golden`, `complete-prompt.golden`, `config-overrides.golden`, `daemon-down.golden`, `docker-missing.golden`, `env-extra-names.golden`, `env-invalid.golden`, `env-padded-name.golden`, `help-list.golden`, `help-unknown-topic.golden`, `help.golden`, `home-unresolvable.golden`, `image-missing-builds-first.golden`, `list-rejects-args.golden`, `list.golden`, `no-mount-with-workdir.golden`, `prompt-claude.golden`, `prompt-codex.golden`, `prompt-copilot.golden`, `prompt-gemini.golden`, `prompt-gh.golden`, `prompt-pi.golden`, `run-exit-code.golden`, `run-mounts-denied.golden`, `shell-env.golden`, `shell-no-mount-no-config.golden`, `shell.golden`, `too-many-args.golden`, `unknown-flag.golden`, `unsupported-agent.golden`, `verbose-claude.golden`, `version.golden`, `workdir-flag.golden`, `workdir-home-allowed.golden`, `workdir-is-home.golden`, `workdir-missing.golden`, `workdir-relative.golden`, `workdir-root-allowed.golden`, `workdir-root.golden`
- `dev/` (1 files): `Dockerfile`
- `docs/` (3 files): `architecture.md`, `implementation.md`, `spec.md`
- `internal/agent/` (12 files): `agent.go`, `agent_test.go`, `builtin.go`, `claude.go`, `codex.go`, `copilot.go`, `definitions_test.go`, `gemini.go`, `gh.go`, `pi.go`, `registry.go`, `registry_test.go`
- `internal/agent/testdata/` (1 files): `builtin_agents.golden`
- `internal/app/` (2 files): `run.go`, `run_test.go`
- `internal/cli/` (3 files): `doc.go`, `root.go`, `root_test.go`
- `internal/docker/` (6 files): `args.go`, `args_test.go`, `client.go`, `client_test.go`, `doc.go`, `platform.go`
- `internal/hostconfig/` (4 files): `resolver.go`, `resolver_edges_test.go`, `resolver_test.go`, `tree.go`
- `internal/plan/` (2 files): `run.go`, `run_test.go`
- `internal/project/` (2 files): `mount.go`, `mount_test.go`
- `internal/runtime/` (5 files): `Dockerfile`, `assets.go`, `caged-entrypoint.sh`, `image.go`, `runtime_test.go`
- `internal/system/` (2 files): `system.go`, `system_test.go`


## Step 10: Split app into Service and Planner with injected dependencies

**Goal:** `app.Service` (Run and Build) and `app.Planner` (Request to RunSpec) get every collaborator through small interfaces, and `context.Context` flows through.

**Why:** This separates the stages: prepare the image, plan the container, run it. The planner and the service can be tested completely with fakes, in the same order of messages as before.

### Files touched

- `internal/app/planner.go`: create this new file.
- `internal/app/planner_test.go`: create this new file.
- `internal/app/service.go`: moved here from `internal/app/run.go`, then modified.
- `internal/app/service_test.go`: moved here from `internal/app/run_test.go`, then modified.
- `internal/cli/root.go`: modify.
- `internal/cli/root_test.go`: modify.

### Commands to run

Run these commands from the repository root:

````bash
mv internal/app/run.go internal/app/service.go
mv internal/app/run_test.go internal/app/service_test.go
````

Each command prints nothing when it works.

### File edits

#### File 1 of 6: `internal/app/planner.go`

This is a NEW FILE (137 lines).

Imports: this new file has these import lines: `"fmt"`, `"io"`, `"github.com/JeremyOrr4/caged/internal/agent"`, `"github.com/JeremyOrr4/caged/internal/plan"`, `"github.com/JeremyOrr4/caged/internal/runtime"`. (The file content below shows how they are grouped.)

Create the file `internal/app/planner.go` with exactly this content:

````go
package app

import (
	"fmt"
	"io"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/runtime"
)

// Env is the process environment the planner reads. system.Env implements it.
type Env interface {
	LookupEnv(key string) (string, bool)
	UserHomeDir() (string, error)
	Getwd() (string, error)
}

// ProjectResolver validates the project directory and returns its bind
// mount. project.Resolver implements it.
type ProjectResolver interface {
	Resolve(workDir, home string, allowBroadMount bool) (*plan.Mount, error)
}

// ConfigResolver finds the host config that can be mounted.
// hostconfig.Resolver implements it.
type ConfigResolver interface {
	Resolve(home string) plan.ConfigReport
}

// Planner turns a Request into a complete plan.RunSpec.
type Planner struct {
	env     Env
	project ProjectResolver
	configs ConfigResolver
	stderr  io.Writer
}

// NewPlanner returns a Planner that writes warnings and --verbose details to stderr.
func NewPlanner(env Env, project ProjectResolver, configs ConfigResolver, stderr io.Writer) *Planner {
	return &Planner{env: env, project: project, configs: configs, stderr: stderr}
}

// Plan resolves the project mount, the config mounts, and the environment
// names for request, and combines them into a RunSpec.
func (p *Planner) Plan(request Request) (plan.RunSpec, error) {
	if !request.NoConfig {
		fmt.Fprintln(p.stderr, "WARNING: allowlisted agent credentials are available inside the container.")
	}
	mount, err := p.projectMount(request)
	if err != nil {
		return plan.RunSpec{}, err
	}
	configMounts, err := p.configMounts(request)
	if err != nil {
		return plan.RunSpec{}, err
	}
	var selected agent.Agent
	if request.Agent != nil {
		selected = *request.Agent
	}
	envVars := selected.ResolveEnv(request.EnvVars, p.isSet)
	if request.Verbose {
		p.logEnvResolution(selected, request.EnvVars, envVars)
	}
	return plan.BuildRunSpec(runtime.Image, request.Agent, request.Prompt, mount, configMounts, envVars)
}

// projectMount returns the project bind mount, or nil with --no-mount.
func (p *Planner) projectMount(request Request) (*plan.Mount, error) {
	if request.NoMount && request.WorkDir != "" {
		return nil, fmt.Errorf("--workdir cannot be used with --no-mount")
	}
	if request.NoMount {
		return nil, nil
	}
	workDir := request.WorkDir
	if workDir == "" {
		var err error
		workDir, err = p.env.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get current working directory: %w", err)
		}
	}
	home, err := p.env.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("find home directory: %w", err)
	}
	return p.project.Resolve(workDir, home, request.AllowBroadMount)
}

// configMounts returns the host config mounts, or none with --no-config.
func (p *Planner) configMounts(request Request) ([]plan.ConfigMount, error) {
	if request.NoConfig {
		return nil, nil
	}
	home, err := p.env.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("find home directory: %w", err)
	}
	report := p.configs.Resolve(home)
	if request.Verbose {
		p.logConfigReport(report)
	}
	return report.Mounts, nil
}

func (p *Planner) isSet(name string) bool {
	_, ok := p.env.LookupEnv(name)
	return ok
}

func (p *Planner) logConfigReport(report plan.ConfigReport) {
	for _, mount := range report.Mounts {
		fmt.Fprintf(p.stderr, "config: pass %s -> %s\n", mount.HostPath, mount.DestPath)
	}
	for _, path := range report.Missing {
		fmt.Fprintf(p.stderr, "config: missing %s\n", path)
	}
	for _, path := range report.Refused {
		fmt.Fprintf(p.stderr, "config: refused %s\n", path)
	}
}

func (p *Planner) logEnvResolution(selected agent.Agent, requested, resolved []string) {
	passed := make(map[string]struct{}, len(resolved))
	for _, name := range resolved {
		passed[name] = struct{}{}
	}
	for _, name := range selected.EnvNames(requested) {
		if _, ok := passed[name]; ok {
			fmt.Fprintf(p.stderr, "env: pass %s\n", name)
		} else {
			fmt.Fprintf(p.stderr, "env: skipped %s (unset or disallowed)\n", name)
		}
	}
}
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 2 of 6: `internal/app/planner_test.go`

This is a NEW FILE (179 lines).

Imports: this new file has these import lines: `"errors"`, `"reflect"`, `"strings"`, `"testing"`, `"github.com/JeremyOrr4/caged/internal/agent"`, `"github.com/JeremyOrr4/caged/internal/plan"`. (The file content below shows how they are grouped.)

Create the file `internal/app/planner_test.go` with exactly this content:

````go
package app

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/plan"
)

// fakeEnv is a fixed process environment.
type fakeEnv struct {
	vars    map[string]string
	home    string
	homeErr error
	wd      string
	wdErr   error
}

func (e *fakeEnv) LookupEnv(key string) (string, bool) {
	value, ok := e.vars[key]
	return value, ok
}
func (e *fakeEnv) UserHomeDir() (string, error) { return e.home, e.homeErr }
func (e *fakeEnv) Getwd() (string, error)       { return e.wd, e.wdErr }

// fakeProject records the arguments it was asked to resolve.
type fakeProject struct {
	gotWorkDir, gotHome string
	gotAllow            bool
	err                 error
}

func (f *fakeProject) Resolve(workDir, home string, allowBroadMount bool) (*plan.Mount, error) {
	f.gotWorkDir, f.gotHome, f.gotAllow = workDir, home, allowBroadMount
	if f.err != nil {
		return nil, f.err
	}
	return &plan.Mount{HostPath: workDir, ContainerPath: "/workspace/app"}, nil
}

// fakeConfigs returns a fixed report.
type fakeConfigs struct {
	report  plan.ConfigReport
	gotHome string
}

func (f *fakeConfigs) Resolve(home string) plan.ConfigReport {
	f.gotHome = home
	return f.report
}

func lookup(t *testing.T, name string) *agent.Agent {
	t.Helper()
	definition, err := agent.Builtin().Lookup(name)
	if err != nil {
		t.Fatal(err)
	}
	return &definition
}

func TestPlannerCombinesMountsConfigsAndEnv(t *testing.T) {
	t.Parallel()

	env := &fakeEnv{vars: map[string]string{"ANTHROPIC_API_KEY": "secret", "EXTRA": ""}, home: "/Users/dev", wd: "/Users/dev/app"}
	project := &fakeProject{}
	configs := &fakeConfigs{report: plan.ConfigReport{
		Mounts:  []plan.ConfigMount{{Agent: "claude", Name: ".claude", HostPath: "/Users/dev/.claude", DestPath: "/home/caged/.claude", Writable: true}},
		Missing: []string{"/Users/dev/.codex"},
		Refused: []string{"/Users/dev/.pi (resolves outside home)"},
	}}
	var stderr strings.Builder
	prompt := "hello"
	spec, err := NewPlanner(env, project, configs, &stderr).Plan(Request{
		Agent: lookup(t, "claude"), Prompt: &prompt, Verbose: true, AllowBroadMount: true, EnvVars: []string{"EXTRA", "MISSING"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := plan.RunSpec{
		Image:         "caged-runtime:latest",
		Command:       []string{"claude", "hello"},
		Mount:         &plan.Mount{HostPath: "/Users/dev/app", ContainerPath: "/workspace/app"},
		ConfigMounts:  configs.report.Mounts,
		EnvVars:       []string{"ANTHROPIC_API_KEY", "EXTRA"},
		ReturnToShell: true,
	}
	if !reflect.DeepEqual(spec, want) {
		t.Fatalf("Plan() = %+v, want %+v", spec, want)
	}
	if project.gotWorkDir != "/Users/dev/app" || project.gotHome != "/Users/dev" || !project.gotAllow || configs.gotHome != "/Users/dev" {
		t.Fatalf("resolvers got project %+v, configs home %q", project, configs.gotHome)
	}
	wantLog := `WARNING: allowlisted agent credentials are available inside the container.
config: pass /Users/dev/.claude -> /home/caged/.claude
config: missing /Users/dev/.codex
config: refused /Users/dev/.pi (resolves outside home)
env: pass ANTHROPIC_API_KEY
env: pass EXTRA
env: skipped MISSING (unset or disallowed)
`
	if stderr.String() != wantLog {
		t.Fatalf("stderr = %q, want %q", stderr.String(), wantLog)
	}
}

func TestPlannerUsesWorkDirFlagWithoutReadingCwd(t *testing.T) {
	t.Parallel()

	project := &fakeProject{}
	env := &fakeEnv{home: "/home/dev", wdErr: errors.New("cwd must not be read")}
	if _, err := NewPlanner(env, project, &fakeConfigs{}, &strings.Builder{}).Plan(Request{WorkDir: "../other", NoConfig: true}); err != nil {
		t.Fatal(err)
	}
	if project.gotWorkDir != "../other" {
		t.Fatalf("project resolver got %q, want the --workdir value", project.gotWorkDir)
	}
}

func TestNoMountAndWorkdirAreIncompatible(t *testing.T) {
	t.Parallel()

	request := Request{NoMount: true, WorkDir: "/tmp"}
	_, err := NewPlanner(&fakeEnv{}, &fakeProject{}, &fakeConfigs{}, &strings.Builder{}).Plan(request)
	if err == nil || !strings.Contains(err.Error(), "cannot be used with --no-mount") {
		t.Fatalf("Plan() error = %v, want incompatible-flags error", err)
	}
}

func TestPlannerNoMountNoConfigNeedsNoHomeOrCwd(t *testing.T) {
	t.Parallel()

	env := &fakeEnv{homeErr: errors.New("no home"), wdErr: errors.New("no cwd")}
	var stderr strings.Builder
	spec, err := NewPlanner(env, &fakeProject{}, &fakeConfigs{}, &stderr).Plan(Request{NoMount: true, NoConfig: true})
	if err != nil {
		t.Fatal(err)
	}
	if spec.Mount != nil || spec.ConfigMounts != nil || !reflect.DeepEqual(spec.Command, []string{"bash"}) || spec.ReturnToShell {
		t.Fatalf("Plan() = %+v, want a bare shell", spec)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want no credentials warning with --no-config", stderr.String())
	}
}

func TestPlannerErrors(t *testing.T) {
	t.Parallel()

	prompt := "hello"
	tests := []struct {
		name    string
		env     *fakeEnv
		project *fakeProject
		request Request
		want    string
	}{
		{name: "cwd", env: &fakeEnv{wdErr: errors.New("gone")}, request: Request{NoConfig: true}, want: "get current working directory: gone"},
		{name: "home for project", env: &fakeEnv{wd: "/w", homeErr: errors.New("unset")}, request: Request{NoConfig: true}, want: "find home directory: unset"},
		{name: "home for config", env: &fakeEnv{homeErr: errors.New("unset")}, request: Request{NoMount: true}, want: "find home directory: unset"},
		{name: "project", env: &fakeEnv{wd: "/w"}, project: &fakeProject{err: errors.New("refusing")}, request: Request{NoConfig: true}, want: "refusing"},
		{name: "prompt without agent", env: &fakeEnv{}, request: Request{Prompt: &prompt, NoMount: true, NoConfig: true}, want: "a prompt requires an agent"},
		{name: "agent without prompt support", env: &fakeEnv{}, request: Request{Agent: lookup(t, "gh"), Prompt: &prompt, NoMount: true, NoConfig: true}, want: "gh does not support prompt launch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			project := tt.project
			if project == nil {
				project = &fakeProject{}
			}
			if _, err := NewPlanner(tt.env, project, &fakeConfigs{}, &strings.Builder{}).Plan(tt.request); err == nil || err.Error() != tt.want {
				t.Fatalf("Plan() error = %v, want %q", err, tt.want)
			}
		})
	}
}
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 3 of 6: `internal/app/service.go`

This file was moved from `internal/app/run.go` by the commands above. The edits below change the moved file.

Imports: added `"io"`; removed `"os"`. (The edits below already contain these import changes.)

**Edit 1 of 1: replace the whole file.** At the start of this step the file has 166 lines (lines 1-166).

The file currently contains exactly this text:

````go
// Package app coordinates agent selection, host configuration, and container execution.
package app

import (
	"context"
	"fmt"
	"os"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/docker"
	"github.com/JeremyOrr4/caged/internal/hostconfig"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/project"
	"github.com/JeremyOrr4/caged/internal/runtime"
	"github.com/JeremyOrr4/caged/internal/system"
)

// Runner and Runtime keep application policy independent of Docker's process API.
type Runner interface{ Run(plan.RunSpec) error }
type Runtime interface {
	ImageExists(string) bool
	Build(bool) error
}

type Service struct {
	runner  Runner
	runtime Runtime
}

type dockerRunner struct{ client *docker.Client }

func (r dockerRunner) Run(spec plan.RunSpec) error { return r.client.Run(context.Background(), spec) }

type dockerRuntime struct{ client *docker.Client }

func (r dockerRuntime) ImageExists(image string) bool {
	return r.client.ImageExists(context.Background(), image)
}
func (r dockerRuntime) Build(noCache bool) error {
	return r.client.Build(context.Background(), runtime.BuildSpec(noCache))
}

func New(runner Runner, runtime Runtime) *Service { return &Service{runner: runner, runtime: runtime} }

// DefaultService wires the production Docker adapters.
func DefaultService() *Service {
	client := docker.NewClient(system.Executor{}, system.FS{}, system.CurrentPlatform(), system.CurrentStdio())
	return New(dockerRunner{client}, dockerRuntime{client})
}

// Build refreshes the runtime image through the configured runtime adapter.
func (s *Service) Build(noCache bool) error {
	if s.runtime == nil {
		return fmt.Errorf("application service requires a runtime")
	}
	return s.runtime.Build(noCache)
}

type Request struct {
	Agent                                       *agent.Agent
	Prompt                                      *string
	NoMount, NoConfig, Verbose, AllowBroadMount bool
	WorkDir                                     string
	EnvVars                                     []string
}

// Run validates the request, plans mounts and environment, then delegates execution.
func (s *Service) Run(request Request) error {
	if s.runner == nil || s.runtime == nil {
		return fmt.Errorf("application service requires a runner and runtime")
	}
	if !s.runtime.ImageExists(runtime.Image) {
		fmt.Fprintf(os.Stderr, "Runtime image %s not found; building it first...\n", runtime.Image)
		if err := s.runtime.Build(false); err != nil {
			return err
		}
	}
	for _, name := range request.EnvVars {
		if err := agent.ValidateEnvName(name); err != nil {
			return err
		}
	}
	resolved := plan.RunSpec{}
	if !request.NoConfig {
		fmt.Fprintln(os.Stderr, "WARNING: allowlisted agent credentials are available inside the container.")
	}
	if err := ConfigureMounts(&resolved, request); err != nil {
		return err
	}
	var selected agent.Agent
	if request.Agent != nil {
		selected = *request.Agent
	}
	envVars := selected.ResolveEnv(request.EnvVars, func(name string) bool { _, ok := os.LookupEnv(name); return ok })
	if request.Verbose {
		logEnvResolution(selected, request.EnvVars, envVars)
	}
	spec, err := plan.BuildRunSpec(runtime.Image, request.Agent, request.Prompt, resolved.Mount, resolved.ConfigMounts, envVars)
	if err != nil {
		return err
	}
	return s.runner.Run(spec)
}

func ConfigureMounts(options *plan.RunSpec, request Request) error {
	if request.NoMount && request.WorkDir != "" {
		return fmt.Errorf("--workdir cannot be used with --no-mount")
	}
	if !request.NoMount {
		workDir := request.WorkDir
		if workDir == "" {
			var err error
			workDir, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("get current working directory: %w", err)
			}
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("find home directory: %w", err)
		}
		options.Mount, err = project.NewResolver(system.FS{}).Resolve(workDir, home, request.AllowBroadMount)
		if err != nil {
			return err
		}
	}
	if request.NoConfig {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("find home directory: %w", err)
	}
	report := hostconfig.NewResolver(system.FS{}, system.Env{}, agent.Builtin().All(), hostconfig.DefaultSharedFiles()).Resolve(home)
	options.ConfigMounts = report.Mounts
	if request.Verbose {
		logConfigReport(report)
	}
	return nil
}

func logConfigReport(report plan.ConfigReport) {
	for _, mount := range report.Mounts {
		fmt.Fprintf(os.Stderr, "config: pass %s -> %s\n", mount.HostPath, mount.DestPath)
	}
	for _, path := range report.Missing {
		fmt.Fprintf(os.Stderr, "config: missing %s\n", path)
	}
	for _, path := range report.Refused {
		fmt.Fprintf(os.Stderr, "config: refused %s\n", path)
	}
}

func logEnvResolution(selected agent.Agent, requested, resolved []string) {
	passed := make(map[string]struct{}, len(resolved))
	for _, name := range resolved {
		passed[name] = struct{}{}
	}
	for _, name := range selected.EnvNames(requested) {
		if _, ok := passed[name]; ok {
			fmt.Fprintf(os.Stderr, "env: pass %s\n", name)
		} else {
			fmt.Fprintf(os.Stderr, "env: skipped %s (unset or disallowed)\n", name)
		}
	}
}
````

Replace the entire contents of the file with exactly this text:

````go
// Package app holds caged's use cases: run an agent or shell in a container,
// and build the runtime image.
package app

import (
	"context"
	"fmt"
	"io"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/docker"
	"github.com/JeremyOrr4/caged/internal/hostconfig"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/project"
	"github.com/JeremyOrr4/caged/internal/runtime"
	"github.com/JeremyOrr4/caged/internal/system"
)

// Engine is the container runtime caged drives. docker.Client implements it.
type Engine interface {
	ImageExists(ctx context.Context, image string) bool
	Build(ctx context.Context, spec plan.BuildSpec) error
	Run(ctx context.Context, spec plan.RunSpec) error
}

// Request is a parsed command line: what to run and how to set it up.
type Request struct {
	Agent                                       *agent.Agent
	Prompt                                      *string
	NoMount, NoConfig, Verbose, AllowBroadMount bool
	WorkDir                                     string
	EnvVars                                     []string
}

// Service runs caged's use cases.
type Service struct {
	engine  Engine
	planner *Planner
	stderr  io.Writer
}

// New returns a Service that plans runs with planner, executes them on
// engine, and reports progress on stderr.
func New(engine Engine, planner *Planner, stderr io.Writer) *Service {
	return &Service{engine: engine, planner: planner, stderr: stderr}
}

// DefaultService wires the production adapters. It moves to cmd/caged next.
func DefaultService() *Service {
	fsys, env, stdio := system.FS{}, system.Env{}, system.CurrentStdio()
	configs := hostconfig.NewResolver(fsys, env, agent.Builtin().All(), hostconfig.DefaultSharedFiles())
	planner := NewPlanner(env, project.NewResolver(fsys), configs, stdio.Err)
	engine := docker.NewClient(system.Executor{}, fsys, system.CurrentPlatform(), stdio)
	return New(engine, planner, stdio.Err)
}

// Build refreshes the runtime image.
func (s *Service) Build(ctx context.Context, noCache bool) error {
	return s.engine.Build(ctx, runtime.BuildSpec(noCache))
}

// Run builds the runtime image if it is missing, plans the container, and
// runs it.
func (s *Service) Run(ctx context.Context, request Request) error {
	if !s.engine.ImageExists(ctx, runtime.Image) {
		fmt.Fprintf(s.stderr, "Runtime image %s not found; building it first...\n", runtime.Image)
		if err := s.engine.Build(ctx, runtime.BuildSpec(false)); err != nil {
			return err
		}
	}
	spec, err := s.planner.Plan(request)
	if err != nil {
		return err
	}
	return s.engine.Run(ctx, spec)
}
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/app/run.go
+++ b/internal/app/service.go
@@ -1,10 +1,11 @@
-// Package app coordinates agent selection, host configuration, and container execution.
+// Package app holds caged's use cases: run an agent or shell in a container,
+// and build the runtime image.
 package app
 
 import (
 	"context"
 	"fmt"
-	"os"
+	"io"
 
 	"github.com/JeremyOrr4/caged/internal/agent"
 	"github.com/JeremyOrr4/caged/internal/docker"
@@ -15,47 +16,14 @@
 	"github.com/JeremyOrr4/caged/internal/system"
 )
 
-// Runner and Runtime keep application policy independent of Docker's process API.
-type Runner interface{ Run(plan.RunSpec) error }
-type Runtime interface {
-	ImageExists(string) bool
-	Build(bool) error
-}
-
-type Service struct {
-	runner  Runner
-	runtime Runtime
-}
-
-type dockerRunner struct{ client *docker.Client }
-
-func (r dockerRunner) Run(spec plan.RunSpec) error { return r.client.Run(context.Background(), spec) }
-
-type dockerRuntime struct{ client *docker.Client }
-
-func (r dockerRuntime) ImageExists(image string) bool {
-	return r.client.ImageExists(context.Background(), image)
-}
-func (r dockerRuntime) Build(noCache bool) error {
-	return r.client.Build(context.Background(), runtime.BuildSpec(noCache))
-}
-
-func New(runner Runner, runtime Runtime) *Service { return &Service{runner: runner, runtime: runtime} }
-
-// DefaultService wires the production Docker adapters.
-func DefaultService() *Service {
-	client := docker.NewClient(system.Executor{}, system.FS{}, system.CurrentPlatform(), system.CurrentStdio())
-	return New(dockerRunner{client}, dockerRuntime{client})
-}
-
-// Build refreshes the runtime image through the configured runtime adapter.
-func (s *Service) Build(noCache bool) error {
-	if s.runtime == nil {
-		return fmt.Errorf("application service requires a runtime")
-	}
-	return s.runtime.Build(noCache)
+// Engine is the container runtime caged drives. docker.Client implements it.
+type Engine interface {
+	ImageExists(ctx context.Context, image string) bool
+	Build(ctx context.Context, spec plan.BuildSpec) error
+	Run(ctx context.Context, spec plan.RunSpec) error
 }
 
+// Request is a parsed command line: what to run and how to set it up.
 type Request struct {
 	Agent                                       *agent.Agent
 	Prompt                                      *string
@@ -64,103 +32,45 @@
 	EnvVars                                     []string
 }
 
-// Run validates the request, plans mounts and environment, then delegates execution.
-func (s *Service) Run(request Request) error {
-	if s.runner == nil || s.runtime == nil {
-		return fmt.Errorf("application service requires a runner and runtime")
-	}
-	if !s.runtime.ImageExists(runtime.Image) {
-		fmt.Fprintf(os.Stderr, "Runtime image %s not found; building it first...\n", runtime.Image)
-		if err := s.runtime.Build(false); err != nil {
-			return err
-		}
-	}
-	for _, name := range request.EnvVars {
-		if err := agent.ValidateEnvName(name); err != nil {
-			return err
-		}
-	}
-	resolved := plan.RunSpec{}
-	if !request.NoConfig {
-		fmt.Fprintln(os.Stderr, "WARNING: allowlisted agent credentials are available inside the container.")
-	}
-	if err := ConfigureMounts(&resolved, request); err != nil {
-		return err
-	}
-	var selected agent.Agent
-	if request.Agent != nil {
-		selected = *request.Agent
-	}
-	envVars := selected.ResolveEnv(request.EnvVars, func(name string) bool { _, ok := os.LookupEnv(name); return ok })
-	if request.Verbose {
-		logEnvResolution(selected, request.EnvVars, envVars)
-	}
-	spec, err := plan.BuildRunSpec(runtime.Image, request.Agent, request.Prompt, resolved.Mount, resolved.ConfigMounts, envVars)
-	if err != nil {
-		return err
-	}
-	return s.runner.Run(spec)
+// Service runs caged's use cases.
+type Service struct {
+	engine  Engine
+	planner *Planner
+	stderr  io.Writer
 }
 
-func ConfigureMounts(options *plan.RunSpec, request Request) error {
-	if request.NoMount && request.WorkDir != "" {
-		return fmt.Errorf("--workdir cannot be used with --no-mount")
-	}
-	if !request.NoMount {
-		workDir := request.WorkDir
-		if workDir == "" {
-			var err error
-			workDir, err = os.Getwd()
-			if err != nil {
-				return fmt.Errorf("get current working directory: %w", err)
-			}
-		}
-		home, err := os.UserHomeDir()
-		if err != nil {
-			return fmt.Errorf("find home directory: %w", err)
-		}
-		options.Mount, err = project.NewResolver(system.FS{}).Resolve(workDir, home, request.AllowBroadMount)
-		if err != nil {
+// New returns a Service that plans runs with planner, executes them on
+// engine, and reports progress on stderr.
+func New(engine Engine, planner *Planner, stderr io.Writer) *Service {
+	return &Service{engine: engine, planner: planner, stderr: stderr}
+}
+
+// DefaultService wires the production adapters. It moves to cmd/caged next.
+func DefaultService() *Service {
+	fsys, env, stdio := system.FS{}, system.Env{}, system.CurrentStdio()
+	configs := hostconfig.NewResolver(fsys, env, agent.Builtin().All(), hostconfig.DefaultSharedFiles())
+	planner := NewPlanner(env, project.NewResolver(fsys), configs, stdio.Err)
+	engine := docker.NewClient(system.Executor{}, fsys, system.CurrentPlatform(), stdio)
+	return New(engine, planner, stdio.Err)
+}
+
+// Build refreshes the runtime image.
+func (s *Service) Build(ctx context.Context, noCache bool) error {
+	return s.engine.Build(ctx, runtime.BuildSpec(noCache))
+}
+
+// Run builds the runtime image if it is missing, plans the container, and
+// runs it.
+func (s *Service) Run(ctx context.Context, request Request) error {
+	if !s.engine.ImageExists(ctx, runtime.Image) {
+		fmt.Fprintf(s.stderr, "Runtime image %s not found; building it first...\n", runtime.Image)
+		if err := s.engine.Build(ctx, runtime.BuildSpec(false)); err != nil {
 			return err
 		}
 	}
-	if request.NoConfig {
-		return nil
-	}
-	home, err := os.UserHomeDir()
+	spec, err := s.planner.Plan(request)
 	if err != nil {
-		return fmt.Errorf("find home directory: %w", err)
-	}
-	report := hostconfig.NewResolver(system.FS{}, system.Env{}, agent.Builtin().All(), hostconfig.DefaultSharedFiles()).Resolve(home)
-	options.ConfigMounts = report.Mounts
-	if request.Verbose {
-		logConfigReport(report)
-	}
-	return nil
-}
-
-func logConfigReport(report plan.ConfigReport) {
-	for _, mount := range report.Mounts {
-		fmt.Fprintf(os.Stderr, "config: pass %s -> %s\n", mount.HostPath, mount.DestPath)
-	}
-	for _, path := range report.Missing {
-		fmt.Fprintf(os.Stderr, "config: missing %s\n", path)
-	}
-	for _, path := range report.Refused {
-		fmt.Fprintf(os.Stderr, "config: refused %s\n", path)
-	}
-}
-
-func logEnvResolution(selected agent.Agent, requested, resolved []string) {
-	passed := make(map[string]struct{}, len(resolved))
-	for _, name := range resolved {
-		passed[name] = struct{}{}
-	}
-	for _, name := range selected.EnvNames(requested) {
-		if _, ok := passed[name]; ok {
-			fmt.Fprintf(os.Stderr, "env: pass %s\n", name)
-		} else {
-			fmt.Fprintf(os.Stderr, "env: skipped %s (unset or disallowed)\n", name)
-		}
+		return err
 	}
+	return s.engine.Run(ctx, spec)
 }
````

#### File 4 of 6: `internal/app/service_test.go`

This file was moved from `internal/app/run_test.go` by the commands above. The edits below change the moved file.

Imports: added `"context"`, `"io"`, `"strings"`, `"github.com/JeremyOrr4/caged/internal/runtime"`. (The edits below already contain these import changes.)

There is 1 edit in this file. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 1: lines 1-71.**

Find this exact text (it appears exactly once in the file):

````go
package app

import (
	"errors"
	"reflect"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/plan"
)

type fakeRunner struct {
	got plan.RunSpec
	err error
}

func (f *fakeRunner) Run(options plan.RunSpec) error { f.got = options; return f.err }

type fakeRuntime struct {
	exists   bool
	builds   int
	buildErr error
}

func (f *fakeRuntime) ImageExists(string) bool { return f.exists }
func (f *fakeRuntime) Build(bool) error        { f.builds++; return f.buildErr }

func TestServicePlansAgentPromptAndDelegates(t *testing.T) {
	runner := &fakeRunner{}
	runtime := &fakeRuntime{exists: true}
	service := New(runner, runtime)
	agent, err := agent.Builtin().Lookup("codex")
	if err != nil {
		t.Fatal(err)
	}
	prompt := "fix $(this) verbatim"
	if err := service.Run(Request{Agent: &agent, Prompt: &prompt, NoMount: true, NoConfig: true}); err != nil {
		t.Fatal(err)
	}
	want := []string{"codex", "exec", prompt}
	if !reflect.DeepEqual(runner.got.Command, want) {
		t.Fatalf("command = %q, want %q", runner.got.Command, want)
	}
	if runtime.builds != 0 {
		t.Fatalf("Build called %d times, want 0", runtime.builds)
	}
}

func TestServiceBuildsMissingImageBeforeRun(t *testing.T) {
	runner := &fakeRunner{}
	runtime := &fakeRuntime{}
	service := New(runner, runtime)
	if err := service.Run(Request{NoMount: true, NoConfig: true}); err != nil {
		t.Fatal(err)
	}
	if runtime.builds != 1 {
		t.Fatalf("Build called %d times, want 1", runtime.builds)
	}
}

func TestServiceReturnsBuildFailureWithoutRunning(t *testing.T) {
	runner := &fakeRunner{}
	runtime := &fakeRuntime{buildErr: errors.New("build failed")}
	service := New(runner, runtime)
	if err := service.Run(Request{NoMount: true, NoConfig: true}); err == nil {
		t.Fatal("Run() error = nil, want build failure")
	}
	if runner.got.Image != "" {
		t.Fatal("runner called after image build failed")
	}
}
````

Replace it with this exact text:

````go
package app

import (
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/runtime"
)

// fakeEngine records what the service asks the container runtime to do.
type fakeEngine struct {
	exists   bool
	builds   []plan.BuildSpec
	buildErr error
	ran      *plan.RunSpec
	runErr   error
}

func (f *fakeEngine) ImageExists(context.Context, string) bool { return f.exists }

func (f *fakeEngine) Build(_ context.Context, spec plan.BuildSpec) error {
	f.builds = append(f.builds, spec)
	return f.buildErr
}

func (f *fakeEngine) Run(_ context.Context, spec plan.RunSpec) error {
	f.ran = &spec
	return f.runErr
}

// newTestService wires a Service whose planner sees no environment at all;
// requests must use NoMount and NoConfig.
func newTestService(engine *fakeEngine, stderr io.Writer) *Service {
	planner := NewPlanner(&fakeEnv{homeErr: errors.New("no home"), wdErr: errors.New("no cwd")}, &fakeProject{}, &fakeConfigs{}, stderr)
	return New(engine, planner, stderr)
}

func TestServicePlansAgentPromptAndDelegates(t *testing.T) {
	engine := &fakeEngine{exists: true}
	service := newTestService(engine, io.Discard)
	agent, err := agent.Builtin().Lookup("codex")
	if err != nil {
		t.Fatal(err)
	}
	prompt := "fix $(this) verbatim"
	if err := service.Run(context.Background(), Request{Agent: &agent, Prompt: &prompt, NoMount: true, NoConfig: true}); err != nil {
		t.Fatal(err)
	}
	want := []string{"codex", "exec", prompt}
	if engine.ran == nil || !reflect.DeepEqual(engine.ran.Command, want) {
		t.Fatalf("command = %+v, want %q", engine.ran, want)
	}
	if len(engine.builds) != 0 {
		t.Fatalf("Build called %d times, want 0", len(engine.builds))
	}
}

func TestServiceBuildsMissingImageBeforeRun(t *testing.T) {
	engine := &fakeEngine{}
	var stderr strings.Builder
	service := newTestService(engine, &stderr)
	if err := service.Run(context.Background(), Request{NoMount: true, NoConfig: true}); err != nil {
		t.Fatal(err)
	}
	if len(engine.builds) != 1 || !reflect.DeepEqual(engine.builds[0], runtime.BuildSpec(false)) {
		t.Fatalf("builds = %+v, want one cached runtime image build", engine.builds)
	}
	if want := "Runtime image caged-runtime:latest not found; building it first...\n"; stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
	if engine.ran == nil {
		t.Fatal("container did not run after the image was built")
	}
}

func TestServiceReturnsBuildFailureWithoutRunning(t *testing.T) {
	engine := &fakeEngine{buildErr: errors.New("build failed")}
	service := newTestService(engine, io.Discard)
	if err := service.Run(context.Background(), Request{NoMount: true, NoConfig: true}); err == nil {
		t.Fatal("Run() error = nil, want build failure")
	}
	if engine.ran != nil {
		t.Fatal("runner called after image build failed")
	}
}

func TestServiceReturnsPlanFailureWithoutRunning(t *testing.T) {
	engine := &fakeEngine{exists: true}
	prompt := "hello"
	err := newTestService(engine, io.Discard).Run(context.Background(), Request{Prompt: &prompt, NoMount: true, NoConfig: true})
	if err == nil || err.Error() != "a prompt requires an agent" || engine.ran != nil {
		t.Fatalf("Run() error = %v, ran = %+v", err, engine.ran)
	}
}

func TestServiceReturnsRunErrorUnchanged(t *testing.T) {
	runErr := errors.New("exit status 3")
	engine := &fakeEngine{exists: true, runErr: runErr}
	if err := newTestService(engine, io.Discard).Run(context.Background(), Request{NoMount: true, NoConfig: true}); err != runErr {
		t.Fatalf("Run() error = %v, want %v", err, runErr)
	}
}

func TestServiceBuildHonorsNoCache(t *testing.T) {
	for _, noCache := range []bool{false, true} {
		engine := &fakeEngine{}
		if err := newTestService(engine, io.Discard).Build(context.Background(), noCache); err != nil {
			t.Fatal(err)
		}
		if len(engine.builds) != 1 || !reflect.DeepEqual(engine.builds[0], runtime.BuildSpec(noCache)) {
			t.Fatalf("Build(%v) sent %+v", noCache, engine.builds)
		}
	}
}
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/app/run_test.go
+++ b/internal/app/service_test.go
@@ -1,71 +1,120 @@
 package app
 
 import (
+	"context"
 	"errors"
+	"io"
 	"reflect"
+	"strings"
 	"testing"
 
 	"github.com/JeremyOrr4/caged/internal/agent"
 	"github.com/JeremyOrr4/caged/internal/plan"
+	"github.com/JeremyOrr4/caged/internal/runtime"
 )
 
-type fakeRunner struct {
-	got plan.RunSpec
-	err error
+// fakeEngine records what the service asks the container runtime to do.
+type fakeEngine struct {
+	exists   bool
+	builds   []plan.BuildSpec
+	buildErr error
+	ran      *plan.RunSpec
+	runErr   error
 }
 
-func (f *fakeRunner) Run(options plan.RunSpec) error { f.got = options; return f.err }
+func (f *fakeEngine) ImageExists(context.Context, string) bool { return f.exists }
 
-type fakeRuntime struct {
-	exists   bool
-	builds   int
-	buildErr error
+func (f *fakeEngine) Build(_ context.Context, spec plan.BuildSpec) error {
+	f.builds = append(f.builds, spec)
+	return f.buildErr
 }
 
-func (f *fakeRuntime) ImageExists(string) bool { return f.exists }
-func (f *fakeRuntime) Build(bool) error        { f.builds++; return f.buildErr }
+func (f *fakeEngine) Run(_ context.Context, spec plan.RunSpec) error {
+	f.ran = &spec
+	return f.runErr
+}
+
+// newTestService wires a Service whose planner sees no environment at all;
+// requests must use NoMount and NoConfig.
+func newTestService(engine *fakeEngine, stderr io.Writer) *Service {
+	planner := NewPlanner(&fakeEnv{homeErr: errors.New("no home"), wdErr: errors.New("no cwd")}, &fakeProject{}, &fakeConfigs{}, stderr)
+	return New(engine, planner, stderr)
+}
 
 func TestServicePlansAgentPromptAndDelegates(t *testing.T) {
-	runner := &fakeRunner{}
-	runtime := &fakeRuntime{exists: true}
-	service := New(runner, runtime)
+	engine := &fakeEngine{exists: true}
+	service := newTestService(engine, io.Discard)
 	agent, err := agent.Builtin().Lookup("codex")
 	if err != nil {
 		t.Fatal(err)
 	}
 	prompt := "fix $(this) verbatim"
-	if err := service.Run(Request{Agent: &agent, Prompt: &prompt, NoMount: true, NoConfig: true}); err != nil {
+	if err := service.Run(context.Background(), Request{Agent: &agent, Prompt: &prompt, NoMount: true, NoConfig: true}); err != nil {
 		t.Fatal(err)
 	}
 	want := []string{"codex", "exec", prompt}
-	if !reflect.DeepEqual(runner.got.Command, want) {
-		t.Fatalf("command = %q, want %q", runner.got.Command, want)
+	if engine.ran == nil || !reflect.DeepEqual(engine.ran.Command, want) {
+		t.Fatalf("command = %+v, want %q", engine.ran, want)
 	}
-	if runtime.builds != 0 {
-		t.Fatalf("Build called %d times, want 0", runtime.builds)
+	if len(engine.builds) != 0 {
+		t.Fatalf("Build called %d times, want 0", len(engine.builds))
 	}
 }
 
 func TestServiceBuildsMissingImageBeforeRun(t *testing.T) {
-	runner := &fakeRunner{}
-	runtime := &fakeRuntime{}
-	service := New(runner, runtime)
-	if err := service.Run(Request{NoMount: true, NoConfig: true}); err != nil {
+	engine := &fakeEngine{}
+	var stderr strings.Builder
+	service := newTestService(engine, &stderr)
+	if err := service.Run(context.Background(), Request{NoMount: true, NoConfig: true}); err != nil {
 		t.Fatal(err)
 	}
-	if runtime.builds != 1 {
-		t.Fatalf("Build called %d times, want 1", runtime.builds)
+	if len(engine.builds) != 1 || !reflect.DeepEqual(engine.builds[0], runtime.BuildSpec(false)) {
+		t.Fatalf("builds = %+v, want one cached runtime image build", engine.builds)
+	}
+	if want := "Runtime image caged-runtime:latest not found; building it first...\n"; stderr.String() != want {
+		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
+	}
+	if engine.ran == nil {
+		t.Fatal("container did not run after the image was built")
 	}
 }
 
 func TestServiceReturnsBuildFailureWithoutRunning(t *testing.T) {
-	runner := &fakeRunner{}
-	runtime := &fakeRuntime{buildErr: errors.New("build failed")}
-	service := New(runner, runtime)
-	if err := service.Run(Request{NoMount: true, NoConfig: true}); err == nil {
+	engine := &fakeEngine{buildErr: errors.New("build failed")}
+	service := newTestService(engine, io.Discard)
+	if err := service.Run(context.Background(), Request{NoMount: true, NoConfig: true}); err == nil {
 		t.Fatal("Run() error = nil, want build failure")
 	}
-	if runner.got.Image != "" {
+	if engine.ran != nil {
 		t.Fatal("runner called after image build failed")
 	}
 }
+
+func TestServiceReturnsPlanFailureWithoutRunning(t *testing.T) {
+	engine := &fakeEngine{exists: true}
+	prompt := "hello"
+	err := newTestService(engine, io.Discard).Run(context.Background(), Request{Prompt: &prompt, NoMount: true, NoConfig: true})
+	if err == nil || err.Error() != "a prompt requires an agent" || engine.ran != nil {
+		t.Fatalf("Run() error = %v, ran = %+v", err, engine.ran)
+	}
+}
+
+func TestServiceReturnsRunErrorUnchanged(t *testing.T) {
+	runErr := errors.New("exit status 3")
+	engine := &fakeEngine{exists: true, runErr: runErr}
+	if err := newTestService(engine, io.Discard).Run(context.Background(), Request{NoMount: true, NoConfig: true}); err != runErr {
+		t.Fatalf("Run() error = %v, want %v", err, runErr)
+	}
+}
+
+func TestServiceBuildHonorsNoCache(t *testing.T) {
+	for _, noCache := range []bool{false, true} {
+		engine := &fakeEngine{}
+		if err := newTestService(engine, io.Discard).Build(context.Background(), noCache); err != nil {
+			t.Fatal(err)
+		}
+		if len(engine.builds) != 1 || !reflect.DeepEqual(engine.builds[0], runtime.BuildSpec(noCache)) {
+			t.Fatalf("Build(%v) sent %+v", noCache, engine.builds)
+		}
+	}
+}
````

#### File 5 of 6: `internal/cli/root.go`

Imports: no import lines are added or removed in this file.

There are 2 edits in this file. They are listed from the bottom of the file to the top. Do them in the order shown: each edit only changes lines below the next one, so the line numbers stay correct. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 2: lines 92-98.**

Find this exact text (it appears exactly once in the file):

````go
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.DefaultService().Build(noCache); err != nil {
				return err
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "built runtime image", runtime.Image)
````

Replace it with this exact text:

````go
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.DefaultService().Build(cmd.Context(), noCache); err != nil {
				return err
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "built runtime image", runtime.Image)
````

**Edit 2 of 2: lines 56-62.**

Find this exact text (it appears exactly once in the file):

````go
			if target.kind == "agent" {
				request.Agent = &target.agent
			}
			return app.DefaultService().Run(request)
		},
	}

````

Replace it with this exact text:

````go
			if target.kind == "agent" {
				request.Agent = &target.agent
			}
			return app.DefaultService().Run(cmd.Context(), request)
		},
	}

````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/cli/root.go
+++ b/internal/cli/root.go
@@ -56,7 +56,7 @@
 			if target.kind == "agent" {
 				request.Agent = &target.agent
 			}
-			return app.DefaultService().Run(request)
+			return app.DefaultService().Run(cmd.Context(), request)
 		},
 	}
 
@@ -92,7 +92,7 @@
 		Args:         cobra.NoArgs,
 		SilenceUsage: true,
 		RunE: func(cmd *cobra.Command, args []string) error {
-			if err := app.DefaultService().Build(noCache); err != nil {
+			if err := app.DefaultService().Build(cmd.Context(), noCache); err != nil {
 				return err
 			}
 			_, err := fmt.Fprintln(cmd.OutOrStdout(), "built runtime image", runtime.Image)
````

#### File 6 of 6: `internal/cli/root_test.go`

Imports: removed `"github.com/JeremyOrr4/caged/internal/app"`, `"github.com/JeremyOrr4/caged/internal/plan"`. (The edits below already contain these import changes.)

There are 2 edits in this file. They are listed from the bottom of the file to the top. Do them in the order shown: each edit only changes lines below the next one, so the line numbers stay correct. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 2: lines 77-92.**

Find this exact text (it appears exactly once in the file):

````go
	}
}

func TestNoMountAndWorkdirAreIncompatible(t *testing.T) {
	t.Parallel()

	request := app.Request{NoMount: true, WorkDir: "/tmp"}
	options := &plan.RunSpec{}
	if err := app.ConfigureMounts(options, request); err == nil || !strings.Contains(err.Error(), "cannot be used with --no-mount") {
		t.Fatalf("ConfigureMounts() error = %v, want incompatible-flags error", err)
	}
}

func TestInvalidEnvFlagRejected(t *testing.T) {
	t.Parallel()

````

Replace it with this exact text:

````go
	}
}

func TestInvalidEnvFlagRejected(t *testing.T) {
	t.Parallel()

````

**Edit 2 of 2: lines 6-13.**

Find this exact text (it appears exactly once in the file):

````go
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/app"
	"github.com/JeremyOrr4/caged/internal/plan"
)

func TestRootCmdRejectsTooManyArguments(t *testing.T) {
````

Replace it with this exact text:

````go
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
)

func TestRootCmdRejectsTooManyArguments(t *testing.T) {
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/cli/root_test.go
+++ b/internal/cli/root_test.go
@@ -6,8 +6,6 @@
 	"testing"
 
 	"github.com/JeremyOrr4/caged/internal/agent"
-	"github.com/JeremyOrr4/caged/internal/app"
-	"github.com/JeremyOrr4/caged/internal/plan"
 )
 
 func TestRootCmdRejectsTooManyArguments(t *testing.T) {
@@ -77,16 +75,6 @@
 	}
 }
 
-func TestNoMountAndWorkdirAreIncompatible(t *testing.T) {
-	t.Parallel()
-
-	request := app.Request{NoMount: true, WorkDir: "/tmp"}
-	options := &plan.RunSpec{}
-	if err := app.ConfigureMounts(options, request); err == nil || !strings.Contains(err.Error(), "cannot be used with --no-mount") {
-		t.Fatalf("ConfigureMounts() error = %v, want incompatible-flags error", err)
-	}
-}
-
 func TestInvalidEnvFlagRejected(t *testing.T) {
 	t.Parallel()
 
````

### Verify

Run these verification commands from the repository root:

````bash
gofmt -l .
go build ./...
go vet ./...
go test -count=1 ./...
````

Expected result:

- `gofmt -l .` prints nothing.
- `go build ./...` prints nothing.
- `go vet ./...` prints nothing.
- `go test -count=1 ./...` prints the lines below. The numbers of seconds at the end of the `ok` lines will be different on your machine, and lines may come in a different order. Every package must say `ok` or `[no test files]`:

````text
ok  	github.com/JeremyOrr4/caged/cmd/caged	0.538s
ok  	github.com/JeremyOrr4/caged/internal/agent	0.008s
ok  	github.com/JeremyOrr4/caged/internal/app	0.008s
ok  	github.com/JeremyOrr4/caged/internal/cli	0.007s
ok  	github.com/JeremyOrr4/caged/internal/docker	0.004s
ok  	github.com/JeremyOrr4/caged/internal/hostconfig	0.013s
ok  	github.com/JeremyOrr4/caged/internal/plan	0.007s
ok  	github.com/JeremyOrr4/caged/internal/project	0.006s
ok  	github.com/JeremyOrr4/caged/internal/runtime	0.002s
ok  	github.com/JeremyOrr4/caged/internal/system	0.005s
````

Count the passing tests and subtests with `go test -count=1 -v ./... 2>&1 | grep -c -- "--- PASS"`. On Linux the expected number is **191** (on macOS the Linux golden test is skipped, so the count there is lower by 52 (51 golden subtests plus their parent test)).

Run this command to compute the fingerprint of the whole repository (on macOS use the second command):

````bash
find . -type f -not -path './.git/*' -not -name REFACTOR_INSTRUCTIONS.md -print0 | LC_ALL=C sort -z | xargs -0 sha256sum | sha256sum
````

````bash
find . -type f -not -path './.git/*' -not -name REFACTOR_INSTRUCTIONS.md -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256 | shasum -a 256
````

Expected output (exactly this line):

````text
99844e275abc877d74186d617046c00990d356c18764c5b26738c3e509f1a8c8  -
````

**If it fails:**

- Problem: A golden test fails because the `WARNING:` line is in a different place.
  Fix: The order must stay: image check (and build), then `WARNING`, then the project mount, then the config mounts, then the env names. `Plan` prints the warning first, and `Service.Run` checks the image before calling `Plan`.
- Problem: `go build` says `cmd.Context undefined`.
  Fix: Inside `RunE`, the Cobra command variable is called `cmd`. The calls are `app.DefaultService().Run(cmd.Context(), request)` and `app.DefaultService().Build(cmd.Context(), noCache)`.
- Problem: the fingerprint is different but every test passes.
  Fix: some file differs from these instructions by a character, often a tab turned into spaces or a missing final newline. Run `gofmt -w .` (it fixes Go indentation), then check the non-Go files of this step again.

### Checkpoint

At this point, the repo should have these 103 files (not counting `.git/` and this instructions file):

- `./` (5 files): `.gitignore`, `Makefile`, `README.md`, `go.mod`, `go.sum`
- `cmd/caged/` (2 files): `golden_test.go`, `main.go`
- `cmd/caged/testdata/golden/linux/` (51 files): `agent-claude.golden`, `agent-codex.golden`, `agent-copilot.golden`, `agent-gemini.golden`, `agent-gh.golden`, `agent-pi.golden`, `agent-uppercase.golden`, `build-daemon-down.golden`, `build-help.golden`, `build-no-cache.golden`, `build-rejects-args.golden`, `build.golden`, `complete-agents.golden`, `complete-prompt.golden`, `config-overrides.golden`, `daemon-down.golden`, `docker-missing.golden`, `env-extra-names.golden`, `env-invalid.golden`, `env-padded-name.golden`, `help-list.golden`, `help-unknown-topic.golden`, `help.golden`, `home-unresolvable.golden`, `image-missing-builds-first.golden`, `list-rejects-args.golden`, `list.golden`, `no-mount-with-workdir.golden`, `prompt-claude.golden`, `prompt-codex.golden`, `prompt-copilot.golden`, `prompt-gemini.golden`, `prompt-gh.golden`, `prompt-pi.golden`, `run-exit-code.golden`, `run-mounts-denied.golden`, `shell-env.golden`, `shell-no-mount-no-config.golden`, `shell.golden`, `too-many-args.golden`, `unknown-flag.golden`, `unsupported-agent.golden`, `verbose-claude.golden`, `version.golden`, `workdir-flag.golden`, `workdir-home-allowed.golden`, `workdir-is-home.golden`, `workdir-missing.golden`, `workdir-relative.golden`, `workdir-root-allowed.golden`, `workdir-root.golden`
- `dev/` (1 files): `Dockerfile`
- `docs/` (3 files): `architecture.md`, `implementation.md`, `spec.md`
- `internal/agent/` (12 files): `agent.go`, `agent_test.go`, `builtin.go`, `claude.go`, `codex.go`, `copilot.go`, `definitions_test.go`, `gemini.go`, `gh.go`, `pi.go`, `registry.go`, `registry_test.go`
- `internal/agent/testdata/` (1 files): `builtin_agents.golden`
- `internal/app/` (4 files): `planner.go`, `planner_test.go`, `service.go`, `service_test.go`
- `internal/cli/` (3 files): `doc.go`, `root.go`, `root_test.go`
- `internal/docker/` (6 files): `args.go`, `args_test.go`, `client.go`, `client_test.go`, `doc.go`, `platform.go`
- `internal/hostconfig/` (4 files): `resolver.go`, `resolver_edges_test.go`, `resolver_test.go`, `tree.go`
- `internal/plan/` (2 files): `run.go`, `run_test.go`
- `internal/project/` (2 files): `mount.go`, `mount_test.go`
- `internal/runtime/` (5 files): `Dockerfile`, `assets.go`, `caged-entrypoint.sh`, `image.go`, `runtime_test.go`
- `internal/system/` (2 files): `system.go`, `system_test.go`


## Step 11: Clean up the CLI and move all wiring into cmd/caged/main.go

**Goal:** The `cli` package only turns arguments into an `app.Request`. `main.go` becomes the composition root that builds and connects every component. The golden test now runs for both Linux and macOS.

**Why:** `app` no longer imports any adapter, so the dependencies point inward. Injecting the platform lets the macOS behaviour be checked on any machine.

### Files touched

- `cmd/caged/golden_test.go`: modify.
- `cmd/caged/main.go`: modify.
- `cmd/caged/main_test.go`: create this new file.
- `cmd/caged/testdata/golden/darwin/agent-claude.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/agent-codex.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/agent-copilot.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/agent-gemini.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/agent-gh.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/agent-pi.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/agent-uppercase.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/build-daemon-down.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/build-help.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/build-no-cache.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/build-rejects-args.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/build.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/complete-agents.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/complete-prompt.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/config-overrides.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/daemon-down.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/docker-missing.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/env-extra-names.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/env-invalid.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/env-padded-name.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/help-list.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/help-unknown-topic.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/help.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/home-unresolvable.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/image-missing-builds-first.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/list-rejects-args.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/list.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/no-mount-with-workdir.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/prompt-claude.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/prompt-codex.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/prompt-copilot.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/prompt-gemini.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/prompt-gh.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/prompt-pi.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/run-exit-code.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/run-mounts-denied.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/shell-env.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/shell-no-mount-no-config.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/shell.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/too-many-args.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/unknown-flag.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/unsupported-agent.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/verbose-claude.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/version.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/workdir-flag.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/workdir-home-allowed.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/workdir-is-home.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/workdir-missing.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/workdir-relative.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/workdir-root-allowed.golden`: create this new file.
- `cmd/caged/testdata/golden/darwin/workdir-root.golden`: create this new file.
- `internal/app/service.go`: modify.
- `internal/cli/doc.go`: modify.
- `internal/cli/root.go`: modify.
- `internal/cli/root_test.go`: modify.

### Commands to run

Run these commands from the repository root:

````bash
mkdir -p cmd/caged/testdata/golden/darwin
````

Each command prints nothing when it works.

### File edits

#### File 1 of 58: `cmd/caged/golden_test.go`

Imports: added `"context"`, `"github.com/JeremyOrr4/caged/internal/agent"`, `"github.com/JeremyOrr4/caged/internal/system"`; removed `"os/exec"`, `"github.com/JeremyOrr4/caged/internal/cli"`. (The edits below already contain these import changes.)

There are 9 edits in this file. They are listed from the bottom of the file to the top. Do them in the order shown: each edit only changes lines below the next one, so the line numbers stay correct. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 9: lines 313-319.**

Find this exact text (it appears exactly once in the file):

````go
	}
	text = strings.ReplaceAll(text, resolvedRoot, "$ROOT")
	text = strings.ReplaceAll(text, root, "$ROOT")
	text = strings.ReplaceAll(text, fmt.Sprintf("\n--user\n%d:%d\n", os.Getuid(), os.Getgid()), "\n--user\n$UID:$GID\n")
	return buildDirPattern.ReplaceAllString(text, "caged-runtime-N")
}

````

Replace it with this exact text:

````go
	}
	text = strings.ReplaceAll(text, resolvedRoot, "$ROOT")
	text = strings.ReplaceAll(text, root, "$ROOT")
	text = strings.ReplaceAll(text, fmt.Sprintf("\n--user\n%d:%d\n", platform.UID, platform.GID), "\n--user\n$UID:$GID\n")
	return buildDirPattern.ReplaceAllString(text, "caged-runtime-N")
}

````

**Edit 2 of 9: lines 305-311.**

Find this exact text (it appears exactly once in the file):

````go
var buildDirPattern = regexp.MustCompile(`caged-runtime-[0-9]+`)

// normalizeGolden replaces values that change between runs and machines.
func normalizeGolden(t *testing.T, text, root string) string {
	t.Helper()
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
````

Replace it with this exact text:

````go
var buildDirPattern = regexp.MustCompile(`caged-runtime-[0-9]+`)

// normalizeGolden replaces values that change between runs and machines.
func normalizeGolden(t *testing.T, text, root string, platform system.Platform) string {
	t.Helper()
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
````

**Edit 3 of 9: lines 249-255.**

Find this exact text (it appears exactly once in the file):

````go
	if err != nil {
		t.Fatal(err)
	}
	return string(stdout), string(stderr), code, runErr
}

// writeGoldenFixtures creates a fake home with every supported config, a
````

Replace it with this exact text:

````go
	if err != nil {
		t.Fatal(err)
	}
	return string(stdout), string(stderr), code, errorLine(string(stderr), code)
}

// errorLine recovers the error that run printed as the last stderr line.
func errorLine(stderr string, code int) error {
	if code == 0 {
		return nil
	}
	lines := strings.Split(strings.TrimSuffix(stderr, "\n"), "\n")
	return errors.New(lines[len(lines)-1])
}

// writeGoldenFixtures creates a fake home with every supported config, a
````

**Edit 4 of 9: lines 227-244.**

Find this exact text (it appears exactly once in the file):

````go

	oldStdout, oldStderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdoutFile, stderrFile
	cmd := cli.NewRootCmd("test")
	cmd.SetArgs(args)
	runErr := cmd.Execute()
	code := 0
	if runErr != nil {
		fmt.Fprintln(os.Stderr, runErr)
		code = 1
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			code = exitErr.ExitCode()
		}
	}
	os.Stdout, os.Stderr = oldStdout, oldStderr

	stdout, err := os.ReadFile(stdoutFile.Name())
````

Replace it with this exact text:

````go

	oldStdout, oldStderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdoutFile, stderrFile
	stdio := system.Stdio{In: strings.NewReader(""), Out: stdoutFile, Err: stderrFile}
	code := run(context.Background(), "test", args, stdio, platform, agents)
	os.Stdout, os.Stderr = oldStdout, oldStderr

	stdout, err := os.ReadFile(stdoutFile.Name())
````

**Edit 5 of 9: lines 208-219.**

Find this exact text (it appears exactly once in the file):

````go
	fmt.Fprintf(&b, "=== stdout\n%s", stdout)
	fmt.Fprintf(&b, "=== stderr\n%s", stderr)
	fmt.Fprintf(&b, "=== docker invocations\n%s", dockerLog)
	return normalizeGolden(t, b.String(), root)
}

// executeOriginal runs the command the way main does and captures the
// process-wide stdout and stderr that caged writes to.
func executeOriginal(t *testing.T, root string, args []string) (string, string, int, error) {
	stdoutFile, err := os.Create(filepath.Join(root, "stdout.txt"))
	if err != nil {
		t.Fatal(err)
````

Replace it with this exact text:

````go
	fmt.Fprintf(&b, "=== stdout\n%s", stdout)
	fmt.Fprintf(&b, "=== stderr\n%s", stderr)
	fmt.Fprintf(&b, "=== docker invocations\n%s", dockerLog)
	return normalizeGolden(t, b.String(), root, platform)
}

// executeRun calls run, the composition root, and captures the process-wide
// stdout and stderr: cobra writes there directly, and caged's own output is
// pointed at the same files through stdio.
func executeRun(t *testing.T, root string, args []string, platform system.Platform, agents *agent.Registry) (string, string, int, error) {
	stdoutFile, err := os.Create(filepath.Join(root, "stdout.txt"))
	if err != nil {
		t.Fatal(err)
````

**Edit 6 of 9: lines 186-192.**

Find this exact text (it appears exactly once in the file):

````go
	for _, arg := range tc.args {
		args = append(args, strings.ReplaceAll(arg, "$ROOT", root))
	}
	stdout, stderr, code, err := executeOriginal(t, root, args)

	dockerLog, readErr := os.ReadFile(logPath)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
````

Replace it with this exact text:

````go
	for _, arg := range tc.args {
		args = append(args, strings.ReplaceAll(arg, "$ROOT", root))
	}
	stdout, stderr, code, err := executeRun(t, root, args, platform, agents)

	dockerLog, readErr := os.ReadFile(logPath)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
````

**Edit 7 of 9: lines 136-161.**

Find this exact text (it appears exactly once in the file):

````go
	return cases
}

// TestGoldenDockerInvocations runs complete command lines against a fake
// docker binary and compares everything observable (exit code, error, stdout,
// stderr, and every docker invocation) with files in testdata/golden.
func TestGoldenDockerInvocations(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("golden fixtures were captured on linux and caged reads the host platform directly")
	}
	goldenDir, err := filepath.Abs(filepath.Join("testdata", "golden", "linux"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range goldenCases() {
		t.Run(tc.name, func(t *testing.T) {
			got := runGoldenCase(t, tc)
			compareGolden(t, filepath.Join(goldenDir, tc.name+".golden"), got)
		})
	}
}

func runGoldenCase(t *testing.T, tc goldenCase) string {
	root := t.TempDir()
	writeGoldenFixtures(t, root)
	logPath := filepath.Join(root, "docker.log")
````

Replace it with this exact text:

````go
	return cases
}

// goldenPlatforms are the hosts the golden files describe. The linux files
// were captured from the original code; the darwin files differ from them
// only by the macOS rules in internal/docker/platform.go.
var goldenPlatforms = []system.Platform{
	{OS: "linux", Arch: "amd64", UID: 4242, GID: 4343},
	{OS: "darwin", Arch: "arm64", UID: 501, GID: 20},
}

// TestGoldenDockerInvocations runs complete command lines against a fake
// docker binary and compares everything observable (exit code, error, stdout,
// stderr, and every docker invocation) with files in testdata/golden/<os>.
func TestGoldenDockerInvocations(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake docker binary is a POSIX shell script")
	}
	for _, platform := range goldenPlatforms {
		goldenDir, err := filepath.Abs(filepath.Join("testdata", "golden", platform.OS))
		if err != nil {
			t.Fatal(err)
		}
		for _, tc := range goldenCases() {
			t.Run(platform.OS+"/"+tc.name, func(t *testing.T) {
				got := runGoldenCase(t, tc, platform, originalAgents(t))
				compareGolden(t, filepath.Join(goldenDir, tc.name+".golden"), got)
			})
		}
	}
}

// originalAgents is a registry of the six agents the golden files were
// captured with, so adding a built-in agent does not change the fixtures.
func originalAgents(t *testing.T) *agent.Registry {
	t.Helper()
	builtin := agent.Builtin()
	var agents []agent.Agent
	for _, name := range []string{"claude", "codex", "copilot", "gemini", "gh", "pi"} {
		definition, err := builtin.Lookup(name)
		if err != nil {
			t.Fatal(err)
		}
		agents = append(agents, definition)
	}
	registry, err := agent.NewRegistry(agents...)
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func runGoldenCase(t *testing.T, tc goldenCase, platform system.Platform, agents *agent.Registry) string {
	root := t.TempDir()
	writeGoldenFixtures(t, root)
	logPath := filepath.Join(root, "docker.log")
````

**Edit 8 of 9: lines 12-18.**

Find this exact text (it appears exactly once in the file):

````go
	"syscall"
	"testing"

	"github.com/JeremyOrr4/caged/internal/cli"
)

// fakeDocker stands in for the docker CLI. It appends every invocation to
````

Replace it with this exact text:

````go
	"syscall"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/system"
)

// fakeDocker stands in for the docker CLI. It appends every invocation to
````

**Edit 9 of 9: lines 1-10.**

Find this exact text (it appears exactly once in the file):

````go
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
````

Replace it with this exact text:

````go
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/cmd/caged/golden_test.go
+++ b/cmd/caged/golden_test.go
@@ -1,10 +1,10 @@
 package main
 
 import (
+	"context"
 	"errors"
 	"fmt"
 	"os"
-	"os/exec"
 	"path/filepath"
 	"regexp"
 	"runtime"
@@ -12,7 +12,8 @@
 	"syscall"
 	"testing"
 
-	"github.com/JeremyOrr4/caged/internal/cli"
+	"github.com/JeremyOrr4/caged/internal/agent"
+	"github.com/JeremyOrr4/caged/internal/system"
 )
 
 // fakeDocker stands in for the docker CLI. It appends every invocation to
@@ -136,26 +137,56 @@
 	return cases
 }
 
+// goldenPlatforms are the hosts the golden files describe. The linux files
+// were captured from the original code; the darwin files differ from them
+// only by the macOS rules in internal/docker/platform.go.
+var goldenPlatforms = []system.Platform{
+	{OS: "linux", Arch: "amd64", UID: 4242, GID: 4343},
+	{OS: "darwin", Arch: "arm64", UID: 501, GID: 20},
+}
+
 // TestGoldenDockerInvocations runs complete command lines against a fake
 // docker binary and compares everything observable (exit code, error, stdout,
-// stderr, and every docker invocation) with files in testdata/golden.
+// stderr, and every docker invocation) with files in testdata/golden/<os>.
 func TestGoldenDockerInvocations(t *testing.T) {
-	if runtime.GOOS != "linux" {
-		t.Skip("golden fixtures were captured on linux and caged reads the host platform directly")
+	if runtime.GOOS == "windows" {
+		t.Skip("the fake docker binary is a POSIX shell script")
+	}
+	for _, platform := range goldenPlatforms {
+		goldenDir, err := filepath.Abs(filepath.Join("testdata", "golden", platform.OS))
+		if err != nil {
+			t.Fatal(err)
+		}
+		for _, tc := range goldenCases() {
+			t.Run(platform.OS+"/"+tc.name, func(t *testing.T) {
+				got := runGoldenCase(t, tc, platform, originalAgents(t))
+				compareGolden(t, filepath.Join(goldenDir, tc.name+".golden"), got)
+			})
+		}
 	}
-	goldenDir, err := filepath.Abs(filepath.Join("testdata", "golden", "linux"))
+}
+
+// originalAgents is a registry of the six agents the golden files were
+// captured with, so adding a built-in agent does not change the fixtures.
+func originalAgents(t *testing.T) *agent.Registry {
+	t.Helper()
+	builtin := agent.Builtin()
+	var agents []agent.Agent
+	for _, name := range []string{"claude", "codex", "copilot", "gemini", "gh", "pi"} {
+		definition, err := builtin.Lookup(name)
+		if err != nil {
+			t.Fatal(err)
+		}
+		agents = append(agents, definition)
+	}
+	registry, err := agent.NewRegistry(agents...)
 	if err != nil {
 		t.Fatal(err)
 	}
-	for _, tc := range goldenCases() {
-		t.Run(tc.name, func(t *testing.T) {
-			got := runGoldenCase(t, tc)
-			compareGolden(t, filepath.Join(goldenDir, tc.name+".golden"), got)
-		})
-	}
+	return registry
 }
 
-func runGoldenCase(t *testing.T, tc goldenCase) string {
+func runGoldenCase(t *testing.T, tc goldenCase, platform system.Platform, agents *agent.Registry) string {
 	root := t.TempDir()
 	writeGoldenFixtures(t, root)
 	logPath := filepath.Join(root, "docker.log")
@@ -186,7 +217,7 @@
 	for _, arg := range tc.args {
 		args = append(args, strings.ReplaceAll(arg, "$ROOT", root))
 	}
-	stdout, stderr, code, err := executeOriginal(t, root, args)
+	stdout, stderr, code, err := executeRun(t, root, args, platform, agents)
 
 	dockerLog, readErr := os.ReadFile(logPath)
 	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
@@ -208,12 +239,13 @@
 	fmt.Fprintf(&b, "=== stdout\n%s", stdout)
 	fmt.Fprintf(&b, "=== stderr\n%s", stderr)
 	fmt.Fprintf(&b, "=== docker invocations\n%s", dockerLog)
-	return normalizeGolden(t, b.String(), root)
+	return normalizeGolden(t, b.String(), root, platform)
 }
 
-// executeOriginal runs the command the way main does and captures the
-// process-wide stdout and stderr that caged writes to.
-func executeOriginal(t *testing.T, root string, args []string) (string, string, int, error) {
+// executeRun calls run, the composition root, and captures the process-wide
+// stdout and stderr: cobra writes there directly, and caged's own output is
+// pointed at the same files through stdio.
+func executeRun(t *testing.T, root string, args []string, platform system.Platform, agents *agent.Registry) (string, string, int, error) {
 	stdoutFile, err := os.Create(filepath.Join(root, "stdout.txt"))
 	if err != nil {
 		t.Fatal(err)
@@ -227,18 +259,8 @@
 
 	oldStdout, oldStderr := os.Stdout, os.Stderr
 	os.Stdout, os.Stderr = stdoutFile, stderrFile
-	cmd := cli.NewRootCmd("test")
-	cmd.SetArgs(args)
-	runErr := cmd.Execute()
-	code := 0
-	if runErr != nil {
-		fmt.Fprintln(os.Stderr, runErr)
-		code = 1
-		var exitErr *exec.ExitError
-		if errors.As(runErr, &exitErr) {
-			code = exitErr.ExitCode()
-		}
-	}
+	stdio := system.Stdio{In: strings.NewReader(""), Out: stdoutFile, Err: stderrFile}
+	code := run(context.Background(), "test", args, stdio, platform, agents)
 	os.Stdout, os.Stderr = oldStdout, oldStderr
 
 	stdout, err := os.ReadFile(stdoutFile.Name())
@@ -249,7 +271,16 @@
 	if err != nil {
 		t.Fatal(err)
 	}
-	return string(stdout), string(stderr), code, runErr
+	return string(stdout), string(stderr), code, errorLine(string(stderr), code)
+}
+
+// errorLine recovers the error that run printed as the last stderr line.
+func errorLine(stderr string, code int) error {
+	if code == 0 {
+		return nil
+	}
+	lines := strings.Split(strings.TrimSuffix(stderr, "\n"), "\n")
+	return errors.New(lines[len(lines)-1])
 }
 
 // writeGoldenFixtures creates a fake home with every supported config, a
@@ -305,7 +336,7 @@
 var buildDirPattern = regexp.MustCompile(`caged-runtime-[0-9]+`)
 
 // normalizeGolden replaces values that change between runs and machines.
-func normalizeGolden(t *testing.T, text, root string) string {
+func normalizeGolden(t *testing.T, text, root string, platform system.Platform) string {
 	t.Helper()
 	resolvedRoot, err := filepath.EvalSymlinks(root)
 	if err != nil {
@@ -313,7 +344,7 @@
 	}
 	text = strings.ReplaceAll(text, resolvedRoot, "$ROOT")
 	text = strings.ReplaceAll(text, root, "$ROOT")
-	text = strings.ReplaceAll(text, fmt.Sprintf("\n--user\n%d:%d\n", os.Getuid(), os.Getgid()), "\n--user\n$UID:$GID\n")
+	text = strings.ReplaceAll(text, fmt.Sprintf("\n--user\n%d:%d\n", platform.UID, platform.GID), "\n--user\n$UID:$GID\n")
 	return buildDirPattern.ReplaceAllString(text, "caged-runtime-N")
 }
 
````

#### File 2 of 58: `cmd/caged/main.go`

Imports: added `"context"`, `"github.com/JeremyOrr4/caged/internal/agent"`, `"github.com/JeremyOrr4/caged/internal/app"`, `"github.com/JeremyOrr4/caged/internal/docker"`, `"github.com/JeremyOrr4/caged/internal/hostconfig"`, `"github.com/JeremyOrr4/caged/internal/project"`, `"github.com/JeremyOrr4/caged/internal/system"`. (The edits below already contain these import changes.)

There is 1 edit in this file. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 1: lines 1-26.**

Find this exact text (it appears exactly once in the file):

````go
// Command caged is the command-line entry point for caged.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/JeremyOrr4/caged/internal/cli"
)

var version = "dev"

func main() {
	if err := cli.NewRootCmd(version).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)

		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}

		os.Exit(1)
	}
}
````

Replace it with this exact text:

````go
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
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/cmd/caged/main.go
+++ b/cmd/caged/main.go
@@ -1,26 +1,57 @@
 // Command caged is the command-line entry point for caged.
+//
+// This file is the composition root: the only place that chooses concrete
+// implementations and wires them together.
 package main
 
 import (
+	"context"
 	"errors"
 	"fmt"
 	"os"
 	"os/exec"
 
+	"github.com/JeremyOrr4/caged/internal/agent"
+	"github.com/JeremyOrr4/caged/internal/app"
 	"github.com/JeremyOrr4/caged/internal/cli"
+	"github.com/JeremyOrr4/caged/internal/docker"
+	"github.com/JeremyOrr4/caged/internal/hostconfig"
+	"github.com/JeremyOrr4/caged/internal/project"
+	"github.com/JeremyOrr4/caged/internal/system"
 )
 
+// version is set at build time with -ldflags "-X main.version=...".
 var version = "dev"
 
 func main() {
-	if err := cli.NewRootCmd(version).Execute(); err != nil {
-		fmt.Fprintln(os.Stderr, err)
-
-		var exitErr *exec.ExitError
-		if errors.As(err, &exitErr) {
-			os.Exit(exitErr.ExitCode())
-		}
+	stdio, platform := system.CurrentStdio(), system.CurrentPlatform()
+	os.Exit(run(context.Background(), version, os.Args[1:], stdio, platform, agent.Builtin()))
+}
 
-		os.Exit(1)
+// run wires caged together, executes one command line, and returns the
+// process exit code. When the container fails, its exit code is passed on.
+func run(ctx context.Context, version string, args []string, stdio system.Stdio, platform system.Platform, agents *agent.Registry) int {
+	fsys, env := system.FS{}, system.Env{}
+	configs := hostconfig.NewResolver(fsys, env, agents.All(), hostconfig.DefaultSharedFiles())
+	planner := app.NewPlanner(env, project.NewResolver(fsys), configs, stdio.Err)
+	engine := docker.NewClient(system.Executor{}, fsys, platform, stdio)
+	service := app.New(engine, planner, stdio.Err)
+
+	root := cli.NewRootCmd(version, agents, service)
+	if args == nil {
+		args = []string{} // a nil slice would make cobra read os.Args instead
+	}
+	root.SetArgs(args)
+	// Cobra keeps its default writers (os.Stdout, and os.Stderr for usage
+	// text) as before; SetOut would also move its stderr-bound messages.
+	err := root.ExecuteContext(ctx)
+	if err == nil {
+		return 0
+	}
+	fmt.Fprintln(stdio.Err, err)
+	var exitErr *exec.ExitError
+	if errors.As(err, &exitErr) {
+		return exitErr.ExitCode()
 	}
+	return 1
 }
````

#### File 3 of 58: `cmd/caged/main_test.go`

This is a NEW FILE (65 lines).

Imports: this new file has these import lines: `"bytes"`, `"context"`, `"runtime"`, `"strings"`, `"testing"`, `"github.com/JeremyOrr4/caged/internal/agent"`, `"github.com/JeremyOrr4/caged/internal/system"`. (The file content below shows how they are grouped.)

Create the file `cmd/caged/main_test.go` with exactly this content:

````go
package main

import (
	"bytes"
	"context"
	"runtime"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/system"
)

// TestNewAgentWorksEndToEnd registers one extra agent and runs it through the
// real composition root: the config resolver, env forwarding, prompt style,
// and `caged list` all pick it up with no other code change.
func TestNewAgentWorksEndToEnd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake docker binary is a POSIX shell script")
	}
	aider := agent.Agent{
		Name:        "aider",
		Command:     []string{"aider"},
		Description: "Aider",
		ConfigPaths: []agent.ConfigPath{{HostPath: ".aider", ContainerPath: ".aider", Writable: true}},
		Prompt:      agent.PromptFlag,
		PromptFlag:  "--message",
		Auth:        agent.AuthStrategy{Environment: []string{"OPENAI_API_KEY"}},
	}
	agents, err := agent.NewRegistry(append(originalAgents(t).All(), aider)...)
	if err != nil {
		t.Fatal(err)
	}
	linux := goldenPlatforms[0]

	got := runGoldenCase(t, goldenCase{args: []string{"--verbose", "aider", "add tests"}, env: map[string]string{"OPENAI_API_KEY": "x"}}, linux, agents)
	for _, want := range []string{
		"exit code: 0\n",
		"config: missing $ROOT/home/.aider\n",
		"env: pass OPENAI_API_KEY\n",
		"\n-e\nOPENAI_API_KEY\n",
		"\ncaged-runtime:latest\naider\n--message\nadd tests\n---\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output does not contain %q:\n%s", want, got)
		}
	}

	list := runGoldenCase(t, goldenCase{args: []string{"list"}}, linux, agents)
	if !strings.Contains(list, "=== stdout\naider\nclaude\ncodex\n") {
		t.Errorf("list does not show the new agent first:\n%s", list)
	}
}

// TestRunTreatsNilArgsAsNoArguments makes sure run never falls back to the
// test binary's own os.Args.
func TestRunTreatsNilArgsAsNoArguments(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // no docker: the shell run fails at the image check
	var stderr bytes.Buffer
	stdio := system.Stdio{In: strings.NewReader(""), Out: &bytes.Buffer{}, Err: &stderr}
	code := run(context.Background(), "test", nil, stdio, goldenPlatforms[0], originalAgents(t))
	if code != 1 || !strings.Contains(stderr.String(), "Docker is not installed or not on PATH") {
		t.Fatalf("run(nil) = %d, stderr %q; want the plain shell path", code, stderr.String())
	}
}
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 4 of 58: `cmd/caged/testdata/golden/darwin/agent-claude.golden`

This is a NEW FILE (55 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/agent-claude.golden` with exactly this content:

````text
args: ["claude"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
claude
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 5 of 58: `cmd/caged/testdata/golden/darwin/agent-codex.golden`

This is a NEW FILE (55 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/agent-codex.golden` with exactly this content:

````text
args: ["codex"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
codex
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 6 of 58: `cmd/caged/testdata/golden/darwin/agent-copilot.golden`

This is a NEW FILE (55 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/agent-copilot.golden` with exactly this content:

````text
args: ["copilot"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
copilot
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 7 of 58: `cmd/caged/testdata/golden/darwin/agent-gemini.golden`

This is a NEW FILE (55 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/agent-gemini.golden` with exactly this content:

````text
args: ["gemini"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
gemini
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 8 of 58: `cmd/caged/testdata/golden/darwin/agent-gh.golden`

This is a NEW FILE (55 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/agent-gh.golden` with exactly this content:

````text
args: ["gh"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
gh
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 9 of 58: `cmd/caged/testdata/golden/darwin/agent-pi.golden`

This is a NEW FILE (55 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/agent-pi.golden` with exactly this content:

````text
args: ["pi"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
pi
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 10 of 58: `cmd/caged/testdata/golden/darwin/agent-uppercase.golden`

This is a NEW FILE (55 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/agent-uppercase.golden` with exactly this content:

````text
args: ["Claude"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
claude
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 11 of 58: `cmd/caged/testdata/golden/darwin/build-daemon-down.golden`

This is a NEW FILE (12 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/build-daemon-down.golden` with exactly this content:

````text
args: ["build"]
exit code: 1
error: Docker is installed but the daemon is unavailable; start Docker Desktop: Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?
leftover temp entries: 0
=== stdout
=== stderr
Docker is installed but the daemon is unavailable; start Docker Desktop: Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?
=== docker invocations
info
--format
{{.ServerVersion}}
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 12 of 58: `cmd/caged/testdata/golden/darwin/build-help.golden`

This is a NEW FILE (15 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/build-help.golden` with exactly this content:

````text
args: ["build" "--help"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
Build the runtime image

Usage:
  caged build [flags]

Flags:
  -h, --help       help for build
      --no-cache   build without using the Docker cache
=== stderr
=== docker invocations
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 13 of 58: `cmd/caged/testdata/golden/darwin/build-no-cache.golden`

This is a NEW FILE (25 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/build-no-cache.golden` with exactly this content:

````text
args: ["build" "--no-cache"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker build output
built runtime image caged-runtime:latest
=== stderr
=== docker invocations
info
--format
{{.ServerVersion}}
---
build
-t
caged-runtime:latest
--no-cache
-f
$ROOT/tmp/caged-runtime-N/Dockerfile
$ROOT/tmp/caged-runtime-N
---
build context Dockerfile: 2970434799 5025
build context caged-entrypoint.sh: 848039441 211
build context entrypoint is executable
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 14 of 58: `cmd/caged/testdata/golden/darwin/build-rejects-args.golden`

This is a NEW FILE (8 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/build-rejects-args.golden` with exactly this content:

````text
args: ["build" "extra"]
exit code: 1
error: unknown command "extra" for "caged build"
leftover temp entries: 0
=== stdout
=== stderr
unknown command "extra" for "caged build"
=== docker invocations
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 15 of 58: `cmd/caged/testdata/golden/darwin/build.golden`

This is a NEW FILE (24 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/build.golden` with exactly this content:

````text
args: ["build"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker build output
built runtime image caged-runtime:latest
=== stderr
=== docker invocations
info
--format
{{.ServerVersion}}
---
build
-t
caged-runtime:latest
-f
$ROOT/tmp/caged-runtime-N/Dockerfile
$ROOT/tmp/caged-runtime-N
---
build context Dockerfile: 2970434799 5025
build context caged-entrypoint.sh: 848039441 211
build context entrypoint is executable
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 16 of 58: `cmd/caged/testdata/golden/darwin/complete-agents.golden`

This is a NEW FILE (19 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/complete-agents.golden` with exactly this content:

````text
args: ["__complete" ""]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
build	Build the runtime image
completion	Generate the autocompletion script for the specified shell
help	Help about any command
list	List supported AI agents
claude
codex
copilot
gemini
gh
pi
:4
=== stderr
Completion ended with directive: ShellCompDirectiveNoFileComp
=== docker invocations
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 17 of 58: `cmd/caged/testdata/golden/darwin/complete-prompt.golden`

This is a NEW FILE (9 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/complete-prompt.golden` with exactly this content:

````text
args: ["__complete" "claude" ""]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
:4
=== stderr
Completion ended with directive: ShellCompDirectiveNoFileComp
=== docker invocations
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 18 of 58: `cmd/caged/testdata/golden/darwin/config-overrides.golden`

This is a NEW FILE (55 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/config-overrides.golden` with exactly this content:

````text
args: ["--verbose" "--no-mount"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
config: pass $ROOT/home/alt-codex -> /home/caged/.codex
config: pass $ROOT/home/.gemini/oauth_creds.json -> /home/caged/.gemini/oauth_creds.json
config: pass $ROOT/home/.gemini/settings.json -> /home/caged/.gemini/settings.json
config: pass $ROOT/home/.config/gh -> /home/caged/.config/gh
config: pass $ROOT/home/.pi -> /home/caged/.pi
config: pass $ROOT/home/.gitconfig -> /home/caged/.gitconfig
config: refused $ROOT/home (config path resolves to the home directory)
config: refused $ROOT/outside/copilot (resolves outside home)
config: refused $ROOT/home/.gemini/current-settings.json (symlinks and special files are excluded from config mounts)
config: refused $ROOT/home/.config/pi (resolves outside home)
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/home/alt-codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
caged-runtime:latest
bash
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 19 of 58: `cmd/caged/testdata/golden/darwin/daemon-down.golden`

This is a NEW FILE (17 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/daemon-down.golden` with exactly this content:

````text
args: ["claude"]
exit code: 1
error: Docker is installed but the daemon is unavailable; start Docker Desktop: Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?
leftover temp entries: 0
=== stdout
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
Docker is installed but the daemon is unavailable; start Docker Desktop: Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 20 of 58: `cmd/caged/testdata/golden/darwin/docker-missing.golden`

This is a NEW FILE (9 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/docker-missing.golden` with exactly this content:

````text
args: ["claude"]
exit code: 1
error: Docker is not installed or not on PATH: exec: "docker": executable file not found in $PATH
leftover temp entries: 0
=== stdout
=== stderr
Runtime image caged-runtime:latest not found; building it first...
Docker is not installed or not on PATH: exec: "docker": executable file not found in $PATH
=== docker invocations
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 21 of 58: `cmd/caged/testdata/golden/darwin/env-extra-names.golden`

This is a NEW FILE (75 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/env-extra-names.golden` with exactly this content:

````text
args: ["--env" "CAGED_GOLDEN_SET" "--env" "CAGED_GOLDEN_UNSET" "--env" "ANTHROPIC_API_KEY" "--verbose" "codex"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
config: pass $ROOT/home/.claude -> /home/caged/.claude
config: pass $ROOT/home/.codex -> /home/caged/.codex
config: pass $ROOT/home/.copilot -> /home/caged/.copilot
config: pass $ROOT/home/.gemini/oauth_creds.json -> /home/caged/.gemini/oauth_creds.json
config: pass $ROOT/home/.gemini/settings.json -> /home/caged/.gemini/settings.json
config: pass $ROOT/home/.config/gh -> /home/caged/.config/gh
config: pass $ROOT/home/.pi -> /home/caged/.pi
config: pass $ROOT/home/.gitconfig -> /home/caged/.gitconfig
config: refused $ROOT/home/.gemini/current-settings.json (symlinks and special files are excluded from config mounts)
config: refused $ROOT/home/.config/pi (resolves outside home)
env: pass OPENAI_API_KEY
env: skipped ANTHROPIC_API_KEY (unset or disallowed)
env: skipped GEMINI_API_KEY (unset or disallowed)
env: skipped GOOGLE_API_KEY (unset or disallowed)
env: pass CAGED_GOLDEN_SET
env: skipped CAGED_GOLDEN_UNSET (unset or disallowed)
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
-e
OPENAI_API_KEY
-e
CAGED_GOLDEN_SET
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
codex
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 22 of 58: `cmd/caged/testdata/golden/darwin/env-invalid.golden`

This is a NEW FILE (8 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/env-invalid.golden` with exactly this content:

````text
args: ["--env" "bad-name" "claude"]
exit code: 1
error: invalid env name "bad-name"; must match ^[A-Za-z_][A-Za-z0-9_]*$
leftover temp entries: 0
=== stdout
=== stderr
invalid env name "bad-name"; must match ^[A-Za-z_][A-Za-z0-9_]*$
=== docker invocations
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 23 of 58: `cmd/caged/testdata/golden/darwin/env-padded-name.golden`

This is a NEW FILE (70 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/env-padded-name.golden` with exactly this content:

````text
args: ["--env" " CAGED_GOLDEN_SET " "--verbose" "gemini"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
config: pass $ROOT/home/.claude -> /home/caged/.claude
config: pass $ROOT/home/.codex -> /home/caged/.codex
config: pass $ROOT/home/.copilot -> /home/caged/.copilot
config: pass $ROOT/home/.gemini/oauth_creds.json -> /home/caged/.gemini/oauth_creds.json
config: pass $ROOT/home/.gemini/settings.json -> /home/caged/.gemini/settings.json
config: pass $ROOT/home/.config/gh -> /home/caged/.config/gh
config: pass $ROOT/home/.pi -> /home/caged/.pi
config: pass $ROOT/home/.gitconfig -> /home/caged/.gitconfig
config: refused $ROOT/home/.gemini/current-settings.json (symlinks and special files are excluded from config mounts)
config: refused $ROOT/home/.config/pi (resolves outside home)
env: skipped GEMINI_API_KEY (unset or disallowed)
env: skipped GOOGLE_API_KEY (unset or disallowed)
env: pass CAGED_GOLDEN_SET
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
-e
CAGED_GOLDEN_SET
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
gemini
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 24 of 58: `cmd/caged/testdata/golden/darwin/help-list.golden`

This is a NEW FILE (14 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/help-list.golden` with exactly this content:

````text
args: ["help" "list"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
List supported AI agents

Usage:
  caged list [flags]

Flags:
  -h, --help   help for list
=== stderr
=== docker invocations
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 25 of 58: `cmd/caged/testdata/golden/darwin/help-unknown-topic.golden`

This is a NEW FILE (30 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/help-unknown-topic.golden` with exactly this content:

````text
args: ["help" "nope"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
Run a bash shell or a supported AI coding agent in a disposable Docker container

Usage:
  caged [agent [prompt]] [flags]
  caged [command]

Available Commands:
  build       Build the runtime image
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  list        List supported AI agents

Flags:
      --allow-broad-mount   allow mounting the filesystem root or home directory
      --env stringArray     pass a host environment variable into the container by NAME only
  -h, --help                help for caged
      --no-config           do not mount any host AI config into the container
      --no-mount            do not mount the current working directory
      --verbose             print which config files and environment names are passed
  -v, --version             version for caged
  -w, --workdir string      mount this host directory at /workspace/<folder-name>

Use "caged [command] --help" for more information about a command.
=== stderr
=== docker invocations
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 26 of 58: `cmd/caged/testdata/golden/darwin/help.golden`

This is a NEW FILE (30 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/help.golden` with exactly this content:

````text
args: ["--help"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
Run a bash shell or a supported AI coding agent in a disposable Docker container

Usage:
  caged [agent [prompt]] [flags]
  caged [command]

Available Commands:
  build       Build the runtime image
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  list        List supported AI agents

Flags:
      --allow-broad-mount   allow mounting the filesystem root or home directory
      --env stringArray     pass a host environment variable into the container by NAME only
  -h, --help                help for caged
      --no-config           do not mount any host AI config into the container
      --no-mount            do not mount the current working directory
      --verbose             print which config files and environment names are passed
  -v, --version             version for caged
  -w, --workdir string      mount this host directory at /workspace/<folder-name>

Use "caged [command] --help" for more information about a command.
=== stderr
=== docker invocations
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 27 of 58: `cmd/caged/testdata/golden/darwin/home-unresolvable.golden`

This is a NEW FILE (24 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/home-unresolvable.golden` with exactly this content:

````text
args: ["--verbose" "--no-mount"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
config: refused $ROOT/missing-home (home cannot be resolved)
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
caged-runtime:latest
bash
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 28 of 58: `cmd/caged/testdata/golden/darwin/image-missing-builds-first.golden`

This is a NEW FILE (72 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/image-missing-builds-first.golden` with exactly this content:

````text
args: ["gemini"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker build output
fake docker run output
=== stderr
Runtime image caged-runtime:latest not found; building it first...
WARNING: allowlisted agent credentials are available inside the container.
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
build
-t
caged-runtime:latest
-f
$ROOT/tmp/caged-runtime-N/Dockerfile
$ROOT/tmp/caged-runtime-N
---
build context Dockerfile: 2970434799 5025
build context caged-entrypoint.sh: 848039441 211
build context entrypoint is executable
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
gemini
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 29 of 58: `cmd/caged/testdata/golden/darwin/list-rejects-args.golden`

This is a NEW FILE (8 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/list-rejects-args.golden` with exactly this content:

````text
args: ["list" "extra"]
exit code: 1
error: unknown command "extra" for "caged list"
leftover temp entries: 0
=== stdout
=== stderr
unknown command "extra" for "caged list"
=== docker invocations
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 30 of 58: `cmd/caged/testdata/golden/darwin/list.golden`

This is a NEW FILE (13 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/list.golden` with exactly this content:

````text
args: ["list"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
claude
codex
copilot
gemini
gh
pi
=== stderr
=== docker invocations
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 31 of 58: `cmd/caged/testdata/golden/darwin/no-mount-with-workdir.golden`

This is a NEW FILE (13 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/no-mount-with-workdir.golden` with exactly this content:

````text
args: ["--no-mount" "-w" "$ROOT/work/other"]
exit code: 1
error: --workdir cannot be used with --no-mount
leftover temp entries: 0
=== stdout
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
--workdir cannot be used with --no-mount
=== docker invocations
image
inspect
caged-runtime:latest
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 32 of 58: `cmd/caged/testdata/golden/darwin/prompt-claude.golden`

This is a NEW FILE (56 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/prompt-claude.golden` with exactly this content:

````text
args: ["claude" "hello $(world) \"quoted\" 'single'"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
claude
hello $(world) "quoted" 'single'
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 33 of 58: `cmd/caged/testdata/golden/darwin/prompt-codex.golden`

This is a NEW FILE (57 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/prompt-codex.golden` with exactly this content:

````text
args: ["codex" "hello $(world) \"quoted\" 'single'"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
codex
exec
hello $(world) "quoted" 'single'
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 34 of 58: `cmd/caged/testdata/golden/darwin/prompt-copilot.golden`

This is a NEW FILE (57 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/prompt-copilot.golden` with exactly this content:

````text
args: ["copilot" "hello $(world) \"quoted\" 'single'"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
copilot
-p
hello $(world) "quoted" 'single'
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 35 of 58: `cmd/caged/testdata/golden/darwin/prompt-gemini.golden`

This is a NEW FILE (57 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/prompt-gemini.golden` with exactly this content:

````text
args: ["gemini" "hello $(world) \"quoted\" 'single'"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
gemini
-i
hello $(world) "quoted" 'single'
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 36 of 58: `cmd/caged/testdata/golden/darwin/prompt-gh.golden`

This is a NEW FILE (13 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/prompt-gh.golden` with exactly this content:

````text
args: ["gh" "hello $(world) \"quoted\" 'single'"]
exit code: 1
error: gh does not support prompt launch
leftover temp entries: 0
=== stdout
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
gh does not support prompt launch
=== docker invocations
image
inspect
caged-runtime:latest
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 37 of 58: `cmd/caged/testdata/golden/darwin/prompt-pi.golden`

This is a NEW FILE (56 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/prompt-pi.golden` with exactly this content:

````text
args: ["pi" "hello $(world) \"quoted\" 'single'"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
pi
hello $(world) "quoted" 'single'
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 38 of 58: `cmd/caged/testdata/golden/darwin/run-exit-code.golden`

This is a NEW FILE (56 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/run-exit-code.golden` with exactly this content:

````text
args: ["claude"]
exit code: 7
error: exit status 7
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
exit status 7
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
claude
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 39 of 58: `cmd/caged/testdata/golden/darwin/run-mounts-denied.golden`

This is a NEW FILE (57 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/run-mounts-denied.golden` with exactly this content:

````text
args: ["codex"]
exit code: 125
error: Docker could not mount the project path; in Docker Desktop, add its parent directory under Settings > Resources > File Sharing: exit status 125
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
docker: Error response from daemon: Mounts denied: the path is not shared from the host.
Docker could not mount the project path; in Docker Desktop, add its parent directory under Settings > Resources > File Sharing: exit status 125
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
codex
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 40 of 58: `cmd/caged/testdata/golden/darwin/shell-env.golden`

This is a NEW FILE (30 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/shell-env.golden` with exactly this content:

````text
args: ["--env" "CAGED_GOLDEN_SET" "--env" "CAGED_GOLDEN_UNSET" "--verbose" "--no-config"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
env: pass CAGED_GOLDEN_SET
env: skipped CAGED_GOLDEN_UNSET (unset or disallowed)
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-e
CAGED_GOLDEN_SET
caged-runtime:latest
bash
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 41 of 58: `cmd/caged/testdata/golden/darwin/shell-no-mount-no-config.golden`

This is a NEW FILE (22 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/shell-no-mount-no-config.golden` with exactly this content:

````text
args: ["--no-mount" "--no-config"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
caged-runtime:latest
bash
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 42 of 58: `cmd/caged/testdata/golden/darwin/shell.golden`

This is a NEW FILE (53 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/shell.golden` with exactly this content:

````text
args: []
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
caged-runtime:latest
bash
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 43 of 58: `cmd/caged/testdata/golden/darwin/too-many-args.golden`

This is a NEW FILE (8 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/too-many-args.golden` with exactly this content:

````text
args: ["claude" "one" "two"]
exit code: 1
error: accepts at most 2 arg(s), received 3
leftover temp entries: 0
=== stdout
=== stderr
accepts at most 2 arg(s), received 3
=== docker invocations
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 44 of 58: `cmd/caged/testdata/golden/darwin/unknown-flag.golden`

This is a NEW FILE (8 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/unknown-flag.golden` with exactly this content:

````text
args: ["--bogus"]
exit code: 1
error: unknown flag: --bogus
leftover temp entries: 0
=== stdout
=== stderr
unknown flag: --bogus
=== docker invocations
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 45 of 58: `cmd/caged/testdata/golden/darwin/unsupported-agent.golden`

This is a NEW FILE (8 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/unsupported-agent.golden` with exactly this content:

````text
args: ["aider"]
exit code: 1
error: unsupported agent "aider"; supported agents: claude, codex, copilot, gemini, gh, pi
leftover temp entries: 0
=== stdout
=== stderr
unsupported agent "aider"; supported agents: claude, codex, copilot, gemini, gh, pi
=== docker invocations
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 46 of 58: `cmd/caged/testdata/golden/darwin/verbose-claude.golden`

This is a NEW FILE (68 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/verbose-claude.golden` with exactly this content:

````text
args: ["--verbose" "claude"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
config: pass $ROOT/home/.claude -> /home/caged/.claude
config: pass $ROOT/home/.codex -> /home/caged/.codex
config: pass $ROOT/home/.copilot -> /home/caged/.copilot
config: pass $ROOT/home/.gemini/oauth_creds.json -> /home/caged/.gemini/oauth_creds.json
config: pass $ROOT/home/.gemini/settings.json -> /home/caged/.gemini/settings.json
config: pass $ROOT/home/.config/gh -> /home/caged/.config/gh
config: pass $ROOT/home/.pi -> /home/caged/.pi
config: pass $ROOT/home/.gitconfig -> /home/caged/.gitconfig
config: refused $ROOT/home/.gemini/current-settings.json (symlinks and special files are excluded from config mounts)
config: refused $ROOT/home/.config/pi (resolves outside home)
env: pass ANTHROPIC_API_KEY
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/project:/workspace/project
-w
/workspace/project
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
-e
ANTHROPIC_API_KEY
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
claude
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 47 of 58: `cmd/caged/testdata/golden/darwin/version.golden`

This is a NEW FILE (8 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/version.golden` with exactly this content:

````text
args: ["--version"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
caged version test
=== stderr
=== docker invocations
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 48 of 58: `cmd/caged/testdata/golden/darwin/workdir-flag.golden`

This is a NEW FILE (55 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/workdir-flag.golden` with exactly this content:

````text
args: ["-w" "$ROOT/work/other" "pi"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/other:/workspace/other
-w
/workspace/other
-v
$ROOT/home/.claude:/home/caged/.claude
-v
$ROOT/home/.codex:/home/caged/.codex
--tmpfs
/home/caged/.codex/app-server-control
--tmpfs
/home/caged/.codex/app-server-daemon
--tmpfs
/home/caged/.codex/ipc
--tmpfs
/home/caged/.codex/packages/app-server-daemon
--tmpfs
/home/caged/.codex/tmp/arg0
-v
$ROOT/home/.copilot:/home/caged/.copilot
-v
$ROOT/home/.gemini/oauth_creds.json:/home/caged/.gemini/oauth_creds.json
-v
$ROOT/home/.gemini/settings.json:/home/caged/.gemini/settings.json
-v
$ROOT/home/.config/gh:/home/caged/.config/gh
-v
$ROOT/home/.pi:/home/caged/.pi
-v
$ROOT/home/.gitconfig:/home/caged/.gitconfig:ro
-e
CAGED_RETURN_TO_SHELL=1
caged-runtime:latest
pi
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 49 of 58: `cmd/caged/testdata/golden/darwin/workdir-home-allowed.golden`

This is a NEW FILE (26 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/workdir-home-allowed.golden` with exactly this content:

````text
args: ["--allow-broad-mount" "-w" "$ROOT/home" "--no-config"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/home:/workspace/home
-w
/workspace/home
caged-runtime:latest
bash
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 50 of 58: `cmd/caged/testdata/golden/darwin/workdir-is-home.golden`

This is a NEW FILE (13 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/workdir-is-home.golden` with exactly this content:

````text
args: ["-w" "$ROOT/home"]
exit code: 1
error: refusing to mount your entire home directory; use --allow-broad-mount to override
leftover temp entries: 0
=== stdout
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
refusing to mount your entire home directory; use --allow-broad-mount to override
=== docker invocations
image
inspect
caged-runtime:latest
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 51 of 58: `cmd/caged/testdata/golden/darwin/workdir-missing.golden`

This is a NEW FILE (13 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/workdir-missing.golden` with exactly this content:

````text
args: ["-w" "$ROOT/work/missing"]
exit code: 1
error: inspect working directory "$ROOT/work/missing": stat $ROOT/work/missing: no such file or directory
leftover temp entries: 0
=== stdout
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
inspect working directory "$ROOT/work/missing": stat $ROOT/work/missing: no such file or directory
=== docker invocations
image
inspect
caged-runtime:latest
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 52 of 58: `cmd/caged/testdata/golden/darwin/workdir-relative.golden`

This is a NEW FILE (26 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/workdir-relative.golden` with exactly this content:

````text
args: ["--workdir" "../other" "--no-config"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
$ROOT/work/other:/workspace/other
-w
/workspace/other
caged-runtime:latest
bash
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 53 of 58: `cmd/caged/testdata/golden/darwin/workdir-root-allowed.golden`

This is a NEW FILE (26 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/workdir-root-allowed.golden` with exactly this content:

````text
args: ["--allow-broad-mount" "-w" "/" "--no-config"]
exit code: 0
error: <nil>
leftover temp entries: 0
=== stdout
fake docker run output
=== stderr
=== docker invocations
image
inspect
caged-runtime:latest
---
info
--format
{{.ServerVersion}}
---
run
-it
--rm
-v
/:/workspace/project
-w
/workspace/project
caged-runtime:latest
bash
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 54 of 58: `cmd/caged/testdata/golden/darwin/workdir-root.golden`

This is a NEW FILE (13 lines).

Imports: this is not a Go file, so there are no imports.

Create the file `cmd/caged/testdata/golden/darwin/workdir-root.golden` with exactly this content:

````text
args: ["-w" "/"]
exit code: 1
error: refusing to mount the filesystem root; use --allow-broad-mount to override
leftover temp entries: 0
=== stdout
=== stderr
WARNING: allowlisted agent credentials are available inside the container.
refusing to mount the filesystem root; use --allow-broad-mount to override
=== docker invocations
image
inspect
caged-runtime:latest
---
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

#### File 55 of 58: `internal/app/service.go`

Imports: removed `"github.com/JeremyOrr4/caged/internal/docker"`, `"github.com/JeremyOrr4/caged/internal/hostconfig"`, `"github.com/JeremyOrr4/caged/internal/project"`, `"github.com/JeremyOrr4/caged/internal/system"`. (The edits below already contain these import changes.)

There are 2 edits in this file. They are listed from the bottom of the file to the top. Do them in the order shown: each edit only changes lines below the next one, so the line numbers stay correct. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 2: lines 45-59.**

Find this exact text (it appears exactly once in the file):

````go
	return &Service{engine: engine, planner: planner, stderr: stderr}
}

// DefaultService wires the production adapters. It moves to cmd/caged next.
func DefaultService() *Service {
	fsys, env, stdio := system.FS{}, system.Env{}, system.CurrentStdio()
	configs := hostconfig.NewResolver(fsys, env, agent.Builtin().All(), hostconfig.DefaultSharedFiles())
	planner := NewPlanner(env, project.NewResolver(fsys), configs, stdio.Err)
	engine := docker.NewClient(system.Executor{}, fsys, system.CurrentPlatform(), stdio)
	return New(engine, planner, stdio.Err)
}

// Build refreshes the runtime image.
func (s *Service) Build(ctx context.Context, noCache bool) error {
	return s.engine.Build(ctx, runtime.BuildSpec(noCache))
````

Replace it with this exact text:

````go
	return &Service{engine: engine, planner: planner, stderr: stderr}
}

// Build refreshes the runtime image.
func (s *Service) Build(ctx context.Context, noCache bool) error {
	return s.engine.Build(ctx, runtime.BuildSpec(noCache))
````

**Edit 2 of 2: lines 8-19.**

Find this exact text (it appears exactly once in the file):

````go
	"io"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/docker"
	"github.com/JeremyOrr4/caged/internal/hostconfig"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/project"
	"github.com/JeremyOrr4/caged/internal/runtime"
	"github.com/JeremyOrr4/caged/internal/system"
)

// Engine is the container runtime caged drives. docker.Client implements it.
````

Replace it with this exact text:

````go
	"io"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/runtime"
)

// Engine is the container runtime caged drives. docker.Client implements it.
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/app/service.go
+++ b/internal/app/service.go
@@ -8,12 +8,8 @@
 	"io"
 
 	"github.com/JeremyOrr4/caged/internal/agent"
-	"github.com/JeremyOrr4/caged/internal/docker"
-	"github.com/JeremyOrr4/caged/internal/hostconfig"
 	"github.com/JeremyOrr4/caged/internal/plan"
-	"github.com/JeremyOrr4/caged/internal/project"
 	"github.com/JeremyOrr4/caged/internal/runtime"
-	"github.com/JeremyOrr4/caged/internal/system"
 )
 
 // Engine is the container runtime caged drives. docker.Client implements it.
@@ -45,15 +41,6 @@
 	return &Service{engine: engine, planner: planner, stderr: stderr}
 }
 
-// DefaultService wires the production adapters. It moves to cmd/caged next.
-func DefaultService() *Service {
-	fsys, env, stdio := system.FS{}, system.Env{}, system.CurrentStdio()
-	configs := hostconfig.NewResolver(fsys, env, agent.Builtin().All(), hostconfig.DefaultSharedFiles())
-	planner := NewPlanner(env, project.NewResolver(fsys), configs, stdio.Err)
-	engine := docker.NewClient(system.Executor{}, fsys, system.CurrentPlatform(), stdio)
-	return New(engine, planner, stdio.Err)
-}
-
 // Build refreshes the runtime image.
 func (s *Service) Build(ctx context.Context, noCache bool) error {
 	return s.engine.Build(ctx, runtime.BuildSpec(noCache))
````

#### File 56 of 58: `internal/cli/doc.go`

Imports: no import lines are added or removed in this file.

There is 1 edit in this file. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 1: lines 1-2.**

Find this exact text (it appears exactly once in the file):

````go
// Package cli wires Cobra commands and flags to caged's runtime operations.
package cli
````

Replace it with this exact text:

````go
// Package cli turns caged's command line into an app.Request. It parses
// arguments and flags with Cobra and leaves all work to injected services.
package cli
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/cli/doc.go
+++ b/internal/cli/doc.go
@@ -1,2 +1,3 @@
-// Package cli wires Cobra commands and flags to caged's runtime operations.
+// Package cli turns caged's command line into an app.Request. It parses
+// arguments and flags with Cobra and leaves all work to injected services.
 package cli
````

#### File 57 of 58: `internal/cli/root.go`

Imports: added `"context"`. (The edits below already contain these import changes.)

There are 5 edits in this file. They are listed from the bottom of the file to the top. Do them in the order shown: each edit only changes lines below the next one, so the line numbers stay correct. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 5: lines 103-109.**

Find this exact text (it appears exactly once in the file):

````go
	return cmd
}

func newListCmd(agents *agent.Registry) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List supported AI agents",
````

Replace it with this exact text:

````go
	return cmd
}

func newListCmd(agents Agents) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List supported AI agents",
````

**Edit 2 of 5: lines 92-98.**

Find this exact text (it appears exactly once in the file):

````go
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.DefaultService().Build(cmd.Context(), noCache); err != nil {
				return err
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "built runtime image", runtime.Image)
````

Replace it with this exact text:

````go
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := service.Build(cmd.Context(), noCache); err != nil {
				return err
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "built runtime image", runtime.Image)
````

**Edit 3 of 5: lines 38-90.**

Find this exact text (it appears exactly once in the file):

````go
			return agents.Names(), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := resolveTarget(agents, args)
			if err != nil {
				return err
			}

			for _, envName := range envVars {
				if err := agent.ValidateEnvName(envName); err != nil {
					return err
				}
			}
			request := app.Request{NoMount: noMount, WorkDir: workDir, NoConfig: noConfig, Verbose: verbose, AllowBroadMount: allowBroadMount, EnvVars: envVars}
			if len(args) == 2 {
				prompt := args[1]
				request.Prompt = &prompt
			}
			if target.kind == "agent" {
				request.Agent = &target.agent
			}
			return app.DefaultService().Run(cmd.Context(), request)
		},
	}

	cmd.AddCommand(newBuildCmd())
	cmd.AddCommand(newListCmd(agents))
	cmd.Flags().BoolVar(&noMount, "no-mount", false, "do not mount the current working directory")
	cmd.Flags().StringVarP(&workDir, "workdir", "w", "", "mount this host directory at /workspace/<folder-name>")
	cmd.Flags().BoolVar(&allowBroadMount, "allow-broad-mount", false, "allow mounting the filesystem root or home directory")
	cmd.Flags().BoolVar(&noConfig, "no-config", false, "do not mount any host AI config into the container")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "print which config files and environment names are passed")
	cmd.Flags().StringArrayVar(&envVars, "env", nil, "pass a host environment variable into the container by NAME only")
	cmd.Version = version
	return cmd
}

func resolveTarget(agents *agent.Registry, args []string) (runTarget, error) {
	if len(args) == 0 {
		return runTarget{kind: "shell"}, nil
	}

	agent, err := agents.Lookup(args[0])
	if err != nil {
		return runTarget{}, err
	}
	return runTarget{kind: "agent", agent: agent}, nil
}

func newBuildCmd() *cobra.Command {
	var noCache bool
	cmd := &cobra.Command{
		Use:          "build",
````

Replace it with this exact text:

````go
			return agents.Names(), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			request, err := buildRequest(agents, flags, args)
			if err != nil {
				return err
			}
			return service.Run(cmd.Context(), request)
		},
	}

	cmd.AddCommand(newBuildCmd(service))
	cmd.AddCommand(newListCmd(agents))
	cmd.Flags().BoolVar(&flags.NoMount, "no-mount", false, "do not mount the current working directory")
	cmd.Flags().StringVarP(&flags.WorkDir, "workdir", "w", "", "mount this host directory at /workspace/<folder-name>")
	cmd.Flags().BoolVar(&flags.AllowBroadMount, "allow-broad-mount", false, "allow mounting the filesystem root or home directory")
	cmd.Flags().BoolVar(&flags.NoConfig, "no-config", false, "do not mount any host AI config into the container")
	cmd.Flags().BoolVar(&flags.Verbose, "verbose", false, "print which config files and environment names are passed")
	cmd.Flags().StringArrayVar(&flags.EnvVars, "env", nil, "pass a host environment variable into the container by NAME only")
	cmd.Version = version
	return cmd
}

// buildRequest adds the positional arguments to the parsed flags. No
// argument means a shell, the first names an agent, and the second is its
// prompt. Environment names are validated here, before anything runs.
func buildRequest(agents Agents, flags app.Request, args []string) (app.Request, error) {
	request := flags
	if len(args) > 0 {
		selected, err := agents.Lookup(args[0])
		if err != nil {
			return app.Request{}, err
		}
		request.Agent = &selected
	}
	for _, name := range request.EnvVars {
		if err := agent.ValidateEnvName(name); err != nil {
			return app.Request{}, err
		}
	}
	if len(args) == 2 {
		prompt := args[1]
		request.Prompt = &prompt
	}
	return request, nil
}

func newBuildCmd(service Service) *cobra.Command {
	var noCache bool
	cmd := &cobra.Command{
		Use:          "build",
````

**Edit 4 of 5: lines 9-28.**

Find this exact text (it appears exactly once in the file):

````go
	"github.com/spf13/cobra"
)

type runTarget struct {
	kind  string
	agent agent.Agent
}

// NewRootCmd creates the root caged command.
func NewRootCmd(version string) *cobra.Command {
	agents := agent.Builtin()
	var noMount bool
	var workDir string
	var noConfig bool
	var verbose bool
	var allowBroadMount bool
	var envVars []string

	cmd := &cobra.Command{
		Use:           "caged [agent [prompt]]",
````

Replace it with this exact text:

````go
	"github.com/spf13/cobra"
)

// Service runs and builds caged containers. app.Service implements it.
type Service interface {
	Run(ctx context.Context, request app.Request) error
	Build(ctx context.Context, noCache bool) error
}

// Agents finds supported agents. agent.Registry implements it.
type Agents interface {
	Lookup(name string) (agent.Agent, error)
	Names() []string
}

// NewRootCmd creates the root caged command. It only turns arguments and
// flags into an app.Request; agents and service do the work.
func NewRootCmd(version string, agents Agents, service Service) *cobra.Command {
	var flags app.Request

	cmd := &cobra.Command{
		Use:           "caged [agent [prompt]]",
````

**Edit 5 of 5: lines 1-6.**

Find this exact text (it appears exactly once in the file):

````go
package cli

import (
	"fmt"

	"github.com/JeremyOrr4/caged/internal/agent"
````

Replace it with this exact text:

````go
package cli

import (
	"context"
	"fmt"

	"github.com/JeremyOrr4/caged/internal/agent"
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/cli/root.go
+++ b/internal/cli/root.go
@@ -1,6 +1,7 @@
 package cli
 
 import (
+	"context"
 	"fmt"
 
 	"github.com/JeremyOrr4/caged/internal/agent"
@@ -9,20 +10,22 @@
 	"github.com/spf13/cobra"
 )
 
-type runTarget struct {
-	kind  string
-	agent agent.Agent
+// Service runs and builds caged containers. app.Service implements it.
+type Service interface {
+	Run(ctx context.Context, request app.Request) error
+	Build(ctx context.Context, noCache bool) error
 }
 
-// NewRootCmd creates the root caged command.
-func NewRootCmd(version string) *cobra.Command {
-	agents := agent.Builtin()
-	var noMount bool
-	var workDir string
-	var noConfig bool
-	var verbose bool
-	var allowBroadMount bool
-	var envVars []string
+// Agents finds supported agents. agent.Registry implements it.
+type Agents interface {
+	Lookup(name string) (agent.Agent, error)
+	Names() []string
+}
+
+// NewRootCmd creates the root caged command. It only turns arguments and
+// flags into an app.Request; agents and service do the work.
+func NewRootCmd(version string, agents Agents, service Service) *cobra.Command {
+	var flags app.Request
 
 	cmd := &cobra.Command{
 		Use:           "caged [agent [prompt]]",
@@ -38,53 +41,51 @@
 			return agents.Names(), cobra.ShellCompDirectiveNoFileComp
 		},
 		RunE: func(cmd *cobra.Command, args []string) error {
-			target, err := resolveTarget(agents, args)
+			request, err := buildRequest(agents, flags, args)
 			if err != nil {
 				return err
 			}
-
-			for _, envName := range envVars {
-				if err := agent.ValidateEnvName(envName); err != nil {
-					return err
-				}
-			}
-			request := app.Request{NoMount: noMount, WorkDir: workDir, NoConfig: noConfig, Verbose: verbose, AllowBroadMount: allowBroadMount, EnvVars: envVars}
-			if len(args) == 2 {
-				prompt := args[1]
-				request.Prompt = &prompt
-			}
-			if target.kind == "agent" {
-				request.Agent = &target.agent
-			}
-			return app.DefaultService().Run(cmd.Context(), request)
+			return service.Run(cmd.Context(), request)
 		},
 	}
 
-	cmd.AddCommand(newBuildCmd())
+	cmd.AddCommand(newBuildCmd(service))
 	cmd.AddCommand(newListCmd(agents))
-	cmd.Flags().BoolVar(&noMount, "no-mount", false, "do not mount the current working directory")
-	cmd.Flags().StringVarP(&workDir, "workdir", "w", "", "mount this host directory at /workspace/<folder-name>")
-	cmd.Flags().BoolVar(&allowBroadMount, "allow-broad-mount", false, "allow mounting the filesystem root or home directory")
-	cmd.Flags().BoolVar(&noConfig, "no-config", false, "do not mount any host AI config into the container")
-	cmd.Flags().BoolVar(&verbose, "verbose", false, "print which config files and environment names are passed")
-	cmd.Flags().StringArrayVar(&envVars, "env", nil, "pass a host environment variable into the container by NAME only")
+	cmd.Flags().BoolVar(&flags.NoMount, "no-mount", false, "do not mount the current working directory")
+	cmd.Flags().StringVarP(&flags.WorkDir, "workdir", "w", "", "mount this host directory at /workspace/<folder-name>")
+	cmd.Flags().BoolVar(&flags.AllowBroadMount, "allow-broad-mount", false, "allow mounting the filesystem root or home directory")
+	cmd.Flags().BoolVar(&flags.NoConfig, "no-config", false, "do not mount any host AI config into the container")
+	cmd.Flags().BoolVar(&flags.Verbose, "verbose", false, "print which config files and environment names are passed")
+	cmd.Flags().StringArrayVar(&flags.EnvVars, "env", nil, "pass a host environment variable into the container by NAME only")
 	cmd.Version = version
 	return cmd
 }
 
-func resolveTarget(agents *agent.Registry, args []string) (runTarget, error) {
-	if len(args) == 0 {
-		return runTarget{kind: "shell"}, nil
+// buildRequest adds the positional arguments to the parsed flags. No
+// argument means a shell, the first names an agent, and the second is its
+// prompt. Environment names are validated here, before anything runs.
+func buildRequest(agents Agents, flags app.Request, args []string) (app.Request, error) {
+	request := flags
+	if len(args) > 0 {
+		selected, err := agents.Lookup(args[0])
+		if err != nil {
+			return app.Request{}, err
+		}
+		request.Agent = &selected
 	}
-
-	agent, err := agents.Lookup(args[0])
-	if err != nil {
-		return runTarget{}, err
+	for _, name := range request.EnvVars {
+		if err := agent.ValidateEnvName(name); err != nil {
+			return app.Request{}, err
+		}
+	}
+	if len(args) == 2 {
+		prompt := args[1]
+		request.Prompt = &prompt
 	}
-	return runTarget{kind: "agent", agent: agent}, nil
+	return request, nil
 }
 
-func newBuildCmd() *cobra.Command {
+func newBuildCmd(service Service) *cobra.Command {
 	var noCache bool
 	cmd := &cobra.Command{
 		Use:          "build",
@@ -92,7 +93,7 @@
 		Args:         cobra.NoArgs,
 		SilenceUsage: true,
 		RunE: func(cmd *cobra.Command, args []string) error {
-			if err := app.DefaultService().Build(cmd.Context(), noCache); err != nil {
+			if err := service.Build(cmd.Context(), noCache); err != nil {
 				return err
 			}
 			_, err := fmt.Fprintln(cmd.OutOrStdout(), "built runtime image", runtime.Image)
@@ -103,7 +104,7 @@
 	return cmd
 }
 
-func newListCmd(agents *agent.Registry) *cobra.Command {
+func newListCmd(agents Agents) *cobra.Command {
 	cmd := &cobra.Command{
 		Use:          "list",
 		Short:        "List supported AI agents",
````

#### File 58 of 58: `internal/cli/root_test.go`

Imports: added `"context"`, `"errors"`, `"reflect"`, `"github.com/JeremyOrr4/caged/internal/app"`. (The edits below already contain these import changes.)

There are 2 edits in this file. They are listed from the bottom of the file to the top. Do them in the order shown: each edit only changes lines below the next one, so the line numbers stay correct. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 2: lines 78-90.**

Find this exact text (it appears exactly once in the file):

````go
func TestInvalidEnvFlagRejected(t *testing.T) {
	t.Parallel()

	cmd := NewRootCmd("test")
	cmd.SetArgs([]string{"claude", "--env", "bad-name"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "invalid env name") {
		t.Fatalf("Execute() error = %v, want invalid env name rejection", err)
	}
}
````

Replace it with this exact text:

````go
func TestInvalidEnvFlagRejected(t *testing.T) {
	t.Parallel()

	service := &fakeService{}
	_, err := execute(t, service, "claude", "--env", "bad-name")
	if err == nil || !strings.Contains(err.Error(), "invalid env name") {
		t.Fatalf("Execute() error = %v, want invalid env name rejection", err)
	}
	if len(service.runs) != 0 {
		t.Fatal("service ran despite an invalid env name")
	}
}

func TestBuildCommand(t *testing.T) {
	t.Parallel()

	service := &fakeService{}
	stdout, err := execute(t, service, "build", "--no-cache")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(service.builds, []bool{true}) || stdout != "built runtime image caged-runtime:latest\n" {
		t.Fatalf("builds = %v, stdout = %q", service.builds, stdout)
	}

	failing := &fakeService{err: errors.New("build failed")}
	if stdout, err := execute(t, failing, "build"); err == nil || err.Error() != "build failed" || stdout != "" {
		t.Fatalf("failed build: stdout = %q, err = %v", stdout, err)
	}
}

func TestListCommandUsesInjectedAgents(t *testing.T) {
	t.Parallel()

	agents, err := agent.NewRegistry(
		agent.Agent{Name: "zeta", Command: []string{"zeta"}},
		agent.Agent{Name: "alpha", Command: []string{"alpha"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	cmd := NewRootCmd("test", agents, &fakeService{})
	var stdout bytes.Buffer
	cmd.SetArgs([]string{"list"})
	cmd.SetOut(&stdout)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "alpha\nzeta\n" {
		t.Fatalf("list output = %q", stdout.String())
	}
}
````

**Edit 2 of 2: lines 2-73.**

Find this exact text (it appears exactly once in the file):

````go

import (
	"bytes"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
)

func TestRootCmdRejectsTooManyArguments(t *testing.T) {
	t.Parallel()

	cmd := NewRootCmd("test")
	cmd.SetArgs([]string{"claude", "prompt one", "extra"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want an error for too many arguments")
	}
	if !strings.Contains(err.Error(), "accepts at most 2 arg") {
		t.Errorf("Execute() error = %q, want a too-many-args error", err)
	}
}

func TestResolveTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		args      []string
		wantKind  string
		wantAgent string
		wantErr   string
	}{
		{name: "shell", args: nil, wantKind: "shell"},
		{name: "agent", args: []string{"claude"}, wantKind: "agent", wantAgent: "claude"},
		{name: "unsupported", args: []string{"foo"}, wantErr: "unsupported agent \"foo\""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			target, err := resolveTarget(agent.Builtin(), tt.args)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("resolveTarget(%v) error = %v, want %q", tt.args, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveTarget(%v) error = %v", tt.args, err)
			}
			if target.kind != tt.wantKind {
				t.Fatalf("resolveTarget(%v) kind = %q, want %q", tt.args, target.kind, tt.wantKind)
			}
			if tt.wantAgent != "" && target.agent.Name != tt.wantAgent {
				t.Fatalf("resolveTarget(%v) agent = %q, want %q", tt.args, target.agent.Name, tt.wantAgent)
			}
		})
	}
}

func TestAgentNamesDoNotCollideWithSubcommands(t *testing.T) {
	t.Parallel()

	subcommands := map[string]struct{}{"build": {}, "list": {}}
	for _, name := range agent.Builtin().Names() {
		if _, ok := subcommands[name]; ok {
			t.Errorf("agent %q collides with a root subcommand", name)
````

Replace it with this exact text:

````go

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/app"
)

// fakeService records the requests the CLI produces.
type fakeService struct {
	runs   []app.Request
	builds []bool
	err    error
}

func (f *fakeService) Run(_ context.Context, request app.Request) error {
	f.runs = append(f.runs, request)
	return f.err
}

func (f *fakeService) Build(_ context.Context, noCache bool) error {
	f.builds = append(f.builds, noCache)
	return f.err
}

// execute runs the root command with args and returns its stdout.
func execute(t *testing.T, service *fakeService, args ...string) (string, error) {
	t.Helper()
	cmd := NewRootCmd("test", agent.Builtin(), service)
	var stdout bytes.Buffer
	cmd.SetArgs(args)
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	return stdout.String(), err
}

func TestRootCmdRejectsTooManyArguments(t *testing.T) {
	t.Parallel()

	service := &fakeService{}
	_, err := execute(t, service, "claude", "prompt one", "extra")
	if err == nil {
		t.Fatal("Execute() error = nil, want an error for too many arguments")
	}
	if !strings.Contains(err.Error(), "accepts at most 2 arg") {
		t.Errorf("Execute() error = %q, want a too-many-args error", err)
	}
	if len(service.runs) != 0 {
		t.Fatal("service ran despite invalid arguments")
	}
}

func TestBuildRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		flags      app.Request
		args       []string
		wantAgent  string
		wantPrompt string
		wantErr    string
	}{
		{name: "shell", args: nil},
		{name: "agent", args: []string{"claude"}, wantAgent: "claude"},
		{name: "agent and prompt", args: []string{"codex", "fix it"}, wantAgent: "codex", wantPrompt: "fix it"},
		{name: "unsupported", args: []string{"foo"}, wantErr: "unsupported agent \"foo\""},
		{name: "invalid env", flags: app.Request{EnvVars: []string{"bad-name"}}, args: []string{"claude"}, wantErr: `invalid env name "bad-name"`},
		{name: "unsupported before invalid env", flags: app.Request{EnvVars: []string{"bad-name"}}, args: []string{"foo"}, wantErr: "unsupported agent"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			request, err := buildRequest(agent.Builtin(), tt.flags, tt.args)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("buildRequest(%v) error = %v, want %q", tt.args, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("buildRequest(%v) error = %v", tt.args, err)
			}
			if (request.Agent == nil) != (tt.wantAgent == "") || (request.Agent != nil && request.Agent.Name != tt.wantAgent) {
				t.Fatalf("buildRequest(%v) agent = %+v, want %q", tt.args, request.Agent, tt.wantAgent)
			}
			if (request.Prompt == nil) != (tt.wantPrompt == "") || (request.Prompt != nil && *request.Prompt != tt.wantPrompt) {
				t.Fatalf("buildRequest(%v) prompt = %v, want %q", tt.args, request.Prompt, tt.wantPrompt)
			}
		})
	}
}

func TestRootCmdPassesFlagsToService(t *testing.T) {
	t.Parallel()

	service := &fakeService{}
	_, err := execute(t, service, "--no-config", "--verbose", "--allow-broad-mount", "-w", "/work", "--env", "A", "--env", "B", "pi", "hi")
	if err != nil {
		t.Fatal(err)
	}
	if len(service.runs) != 1 {
		t.Fatalf("runs = %+v, want one", service.runs)
	}
	got := service.runs[0]
	pi, err := agent.Builtin().Lookup("pi")
	if err != nil {
		t.Fatal(err)
	}
	prompt := "hi"
	want := app.Request{Agent: &pi, Prompt: &prompt, NoConfig: true, Verbose: true, AllowBroadMount: true, WorkDir: "/work", EnvVars: []string{"A", "B"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("request = %+v, want %+v", got, want)
	}
}

func TestAgentNamesDoNotCollideWithSubcommands(t *testing.T) {
	t.Parallel()

	subcommands := map[string]struct{}{"build": {}, "list": {}, "help": {}, "completion": {}}
	for _, name := range agent.Builtin().Names() {
		if _, ok := subcommands[name]; ok {
			t.Errorf("agent %q collides with a root subcommand", name)
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/cli/root_test.go
+++ b/internal/cli/root_test.go
@@ -2,72 +2,131 @@
 
 import (
 	"bytes"
+	"context"
+	"errors"
+	"reflect"
 	"strings"
 	"testing"
 
 	"github.com/JeremyOrr4/caged/internal/agent"
+	"github.com/JeremyOrr4/caged/internal/app"
 )
 
-func TestRootCmdRejectsTooManyArguments(t *testing.T) {
-	t.Parallel()
+// fakeService records the requests the CLI produces.
+type fakeService struct {
+	runs   []app.Request
+	builds []bool
+	err    error
+}
 
-	cmd := NewRootCmd("test")
-	cmd.SetArgs([]string{"claude", "prompt one", "extra"})
-	cmd.SetOut(&bytes.Buffer{})
-	cmd.SetErr(&bytes.Buffer{})
+func (f *fakeService) Run(_ context.Context, request app.Request) error {
+	f.runs = append(f.runs, request)
+	return f.err
+}
 
+func (f *fakeService) Build(_ context.Context, noCache bool) error {
+	f.builds = append(f.builds, noCache)
+	return f.err
+}
+
+// execute runs the root command with args and returns its stdout.
+func execute(t *testing.T, service *fakeService, args ...string) (string, error) {
+	t.Helper()
+	cmd := NewRootCmd("test", agent.Builtin(), service)
+	var stdout bytes.Buffer
+	cmd.SetArgs(args)
+	cmd.SetOut(&stdout)
+	cmd.SetErr(&bytes.Buffer{})
 	err := cmd.Execute()
+	return stdout.String(), err
+}
+
+func TestRootCmdRejectsTooManyArguments(t *testing.T) {
+	t.Parallel()
+
+	service := &fakeService{}
+	_, err := execute(t, service, "claude", "prompt one", "extra")
 	if err == nil {
 		t.Fatal("Execute() error = nil, want an error for too many arguments")
 	}
 	if !strings.Contains(err.Error(), "accepts at most 2 arg") {
 		t.Errorf("Execute() error = %q, want a too-many-args error", err)
 	}
+	if len(service.runs) != 0 {
+		t.Fatal("service ran despite invalid arguments")
+	}
 }
 
-func TestResolveTarget(t *testing.T) {
+func TestBuildRequest(t *testing.T) {
 	t.Parallel()
 
 	tests := []struct {
-		name      string
-		args      []string
-		wantKind  string
-		wantAgent string
-		wantErr   string
+		name       string
+		flags      app.Request
+		args       []string
+		wantAgent  string
+		wantPrompt string
+		wantErr    string
 	}{
-		{name: "shell", args: nil, wantKind: "shell"},
-		{name: "agent", args: []string{"claude"}, wantKind: "agent", wantAgent: "claude"},
+		{name: "shell", args: nil},
+		{name: "agent", args: []string{"claude"}, wantAgent: "claude"},
+		{name: "agent and prompt", args: []string{"codex", "fix it"}, wantAgent: "codex", wantPrompt: "fix it"},
 		{name: "unsupported", args: []string{"foo"}, wantErr: "unsupported agent \"foo\""},
+		{name: "invalid env", flags: app.Request{EnvVars: []string{"bad-name"}}, args: []string{"claude"}, wantErr: `invalid env name "bad-name"`},
+		{name: "unsupported before invalid env", flags: app.Request{EnvVars: []string{"bad-name"}}, args: []string{"foo"}, wantErr: "unsupported agent"},
 	}
 
 	for _, tt := range tests {
 		t.Run(tt.name, func(t *testing.T) {
 			t.Parallel()
 
-			target, err := resolveTarget(agent.Builtin(), tt.args)
+			request, err := buildRequest(agent.Builtin(), tt.flags, tt.args)
 			if tt.wantErr != "" {
 				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
-					t.Fatalf("resolveTarget(%v) error = %v, want %q", tt.args, err, tt.wantErr)
+					t.Fatalf("buildRequest(%v) error = %v, want %q", tt.args, err, tt.wantErr)
 				}
 				return
 			}
 			if err != nil {
-				t.Fatalf("resolveTarget(%v) error = %v", tt.args, err)
+				t.Fatalf("buildRequest(%v) error = %v", tt.args, err)
 			}
-			if target.kind != tt.wantKind {
-				t.Fatalf("resolveTarget(%v) kind = %q, want %q", tt.args, target.kind, tt.wantKind)
+			if (request.Agent == nil) != (tt.wantAgent == "") || (request.Agent != nil && request.Agent.Name != tt.wantAgent) {
+				t.Fatalf("buildRequest(%v) agent = %+v, want %q", tt.args, request.Agent, tt.wantAgent)
 			}
-			if tt.wantAgent != "" && target.agent.Name != tt.wantAgent {
-				t.Fatalf("resolveTarget(%v) agent = %q, want %q", tt.args, target.agent.Name, tt.wantAgent)
+			if (request.Prompt == nil) != (tt.wantPrompt == "") || (request.Prompt != nil && *request.Prompt != tt.wantPrompt) {
+				t.Fatalf("buildRequest(%v) prompt = %v, want %q", tt.args, request.Prompt, tt.wantPrompt)
 			}
 		})
 	}
 }
 
+func TestRootCmdPassesFlagsToService(t *testing.T) {
+	t.Parallel()
+
+	service := &fakeService{}
+	_, err := execute(t, service, "--no-config", "--verbose", "--allow-broad-mount", "-w", "/work", "--env", "A", "--env", "B", "pi", "hi")
+	if err != nil {
+		t.Fatal(err)
+	}
+	if len(service.runs) != 1 {
+		t.Fatalf("runs = %+v, want one", service.runs)
+	}
+	got := service.runs[0]
+	pi, err := agent.Builtin().Lookup("pi")
+	if err != nil {
+		t.Fatal(err)
+	}
+	prompt := "hi"
+	want := app.Request{Agent: &pi, Prompt: &prompt, NoConfig: true, Verbose: true, AllowBroadMount: true, WorkDir: "/work", EnvVars: []string{"A", "B"}}
+	if !reflect.DeepEqual(got, want) {
+		t.Fatalf("request = %+v, want %+v", got, want)
+	}
+}
+
 func TestAgentNamesDoNotCollideWithSubcommands(t *testing.T) {
 	t.Parallel()
 
-	subcommands := map[string]struct{}{"build": {}, "list": {}}
+	subcommands := map[string]struct{}{"build": {}, "list": {}, "help": {}, "completion": {}}
 	for _, name := range agent.Builtin().Names() {
 		if _, ok := subcommands[name]; ok {
 			t.Errorf("agent %q collides with a root subcommand", name)
@@ -78,13 +137,52 @@
 func TestInvalidEnvFlagRejected(t *testing.T) {
 	t.Parallel()
 
-	cmd := NewRootCmd("test")
-	cmd.SetArgs([]string{"claude", "--env", "bad-name"})
-	cmd.SetOut(&bytes.Buffer{})
-	cmd.SetErr(&bytes.Buffer{})
-
-	err := cmd.Execute()
+	service := &fakeService{}
+	_, err := execute(t, service, "claude", "--env", "bad-name")
 	if err == nil || !strings.Contains(err.Error(), "invalid env name") {
 		t.Fatalf("Execute() error = %v, want invalid env name rejection", err)
 	}
+	if len(service.runs) != 0 {
+		t.Fatal("service ran despite an invalid env name")
+	}
+}
+
+func TestBuildCommand(t *testing.T) {
+	t.Parallel()
+
+	service := &fakeService{}
+	stdout, err := execute(t, service, "build", "--no-cache")
+	if err != nil {
+		t.Fatal(err)
+	}
+	if !reflect.DeepEqual(service.builds, []bool{true}) || stdout != "built runtime image caged-runtime:latest\n" {
+		t.Fatalf("builds = %v, stdout = %q", service.builds, stdout)
+	}
+
+	failing := &fakeService{err: errors.New("build failed")}
+	if stdout, err := execute(t, failing, "build"); err == nil || err.Error() != "build failed" || stdout != "" {
+		t.Fatalf("failed build: stdout = %q, err = %v", stdout, err)
+	}
+}
+
+func TestListCommandUsesInjectedAgents(t *testing.T) {
+	t.Parallel()
+
+	agents, err := agent.NewRegistry(
+		agent.Agent{Name: "zeta", Command: []string{"zeta"}},
+		agent.Agent{Name: "alpha", Command: []string{"alpha"}},
+	)
+	if err != nil {
+		t.Fatal(err)
+	}
+	cmd := NewRootCmd("test", agents, &fakeService{})
+	var stdout bytes.Buffer
+	cmd.SetArgs([]string{"list"})
+	cmd.SetOut(&stdout)
+	if err := cmd.Execute(); err != nil {
+		t.Fatal(err)
+	}
+	if stdout.String() != "alpha\nzeta\n" {
+		t.Fatalf("list output = %q", stdout.String())
+	}
 }
````

### Verify

Run these verification commands from the repository root:

````bash
gofmt -l .
go build ./...
go vet ./...
go test -count=1 ./...
````

Expected result:

- `gofmt -l .` prints nothing.
- `go build ./...` prints nothing.
- `go vet ./...` prints nothing.
- `go test -count=1 ./...` prints the lines below. The numbers of seconds at the end of the `ok` lines will be different on your machine, and lines may come in a different order. Every package must say `ok` or `[no test files]`:

````text
ok  	github.com/JeremyOrr4/caged/cmd/caged	0.945s
ok  	github.com/JeremyOrr4/caged/internal/agent	0.005s
ok  	github.com/JeremyOrr4/caged/internal/app	0.005s
ok  	github.com/JeremyOrr4/caged/internal/cli	0.006s
ok  	github.com/JeremyOrr4/caged/internal/docker	0.004s
ok  	github.com/JeremyOrr4/caged/internal/hostconfig	0.009s
ok  	github.com/JeremyOrr4/caged/internal/plan	0.009s
ok  	github.com/JeremyOrr4/caged/internal/project	0.004s
ok  	github.com/JeremyOrr4/caged/internal/runtime	0.003s
ok  	github.com/JeremyOrr4/caged/internal/system	0.006s
````

Count the passing tests and subtests with `go test -count=1 -v ./... 2>&1 | grep -c -- "--- PASS"`. On Linux the expected number is **250** (all 102 of them run on every OS).

Run this command to compute the fingerprint of the whole repository (on macOS use the second command):

````bash
find . -type f -not -path './.git/*' -not -name REFACTOR_INSTRUCTIONS.md -print0 | LC_ALL=C sort -z | xargs -0 sha256sum | sha256sum
````

````bash
find . -type f -not -path './.git/*' -not -name REFACTOR_INSTRUCTIONS.md -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256 | shasum -a 256
````

Expected output (exactly this line):

````text
075a00033e2e4eb088f865413e51e06e03113fcbdba587da5831aac69046a930  -
````

**If it fails:**

- Problem: `TestGoldenDockerInvocations/darwin/...` fails with `no such file or directory`.
  Fix: You missed one of the 51 files in `cmd/caged/testdata/golden/darwin/`.
- Problem: `TestRunTreatsNilArgsAsNoArguments` fails with `unknown shorthand flag`.
  Fix: `run` must replace a nil `args` with `[]string{}` before `root.SetArgs(args)`. Otherwise Cobra reads the test binary's own flags.
- Problem: `go vet` says `"github.com/JeremyOrr4/caged/internal/system" imported and not used` in `internal/app/service.go`.
  Fix: Remove the `docker`, `hostconfig`, `project` and `system` imports from `internal/app/service.go`. After this step it imports only `agent`, `plan` and `runtime` from caged.
- Problem: the fingerprint is different but every test passes.
  Fix: some file differs from these instructions by a character, often a tab turned into spaces or a missing final newline. Run `gofmt -w .` (it fixes Go indentation), then check the non-Go files of this step again.

### Checkpoint

At this point, the repo should have these 155 files (not counting `.git/` and this instructions file):

- `./` (5 files): `.gitignore`, `Makefile`, `README.md`, `go.mod`, `go.sum`
- `cmd/caged/` (3 files): `golden_test.go`, `main.go`, `main_test.go`
- `cmd/caged/testdata/golden/darwin/` (51 files): `agent-claude.golden`, `agent-codex.golden`, `agent-copilot.golden`, `agent-gemini.golden`, `agent-gh.golden`, `agent-pi.golden`, `agent-uppercase.golden`, `build-daemon-down.golden`, `build-help.golden`, `build-no-cache.golden`, `build-rejects-args.golden`, `build.golden`, `complete-agents.golden`, `complete-prompt.golden`, `config-overrides.golden`, `daemon-down.golden`, `docker-missing.golden`, `env-extra-names.golden`, `env-invalid.golden`, `env-padded-name.golden`, `help-list.golden`, `help-unknown-topic.golden`, `help.golden`, `home-unresolvable.golden`, `image-missing-builds-first.golden`, `list-rejects-args.golden`, `list.golden`, `no-mount-with-workdir.golden`, `prompt-claude.golden`, `prompt-codex.golden`, `prompt-copilot.golden`, `prompt-gemini.golden`, `prompt-gh.golden`, `prompt-pi.golden`, `run-exit-code.golden`, `run-mounts-denied.golden`, `shell-env.golden`, `shell-no-mount-no-config.golden`, `shell.golden`, `too-many-args.golden`, `unknown-flag.golden`, `unsupported-agent.golden`, `verbose-claude.golden`, `version.golden`, `workdir-flag.golden`, `workdir-home-allowed.golden`, `workdir-is-home.golden`, `workdir-missing.golden`, `workdir-relative.golden`, `workdir-root-allowed.golden`, `workdir-root.golden`
- `cmd/caged/testdata/golden/linux/` (51 files): `agent-claude.golden`, `agent-codex.golden`, `agent-copilot.golden`, `agent-gemini.golden`, `agent-gh.golden`, `agent-pi.golden`, `agent-uppercase.golden`, `build-daemon-down.golden`, `build-help.golden`, `build-no-cache.golden`, `build-rejects-args.golden`, `build.golden`, `complete-agents.golden`, `complete-prompt.golden`, `config-overrides.golden`, `daemon-down.golden`, `docker-missing.golden`, `env-extra-names.golden`, `env-invalid.golden`, `env-padded-name.golden`, `help-list.golden`, `help-unknown-topic.golden`, `help.golden`, `home-unresolvable.golden`, `image-missing-builds-first.golden`, `list-rejects-args.golden`, `list.golden`, `no-mount-with-workdir.golden`, `prompt-claude.golden`, `prompt-codex.golden`, `prompt-copilot.golden`, `prompt-gemini.golden`, `prompt-gh.golden`, `prompt-pi.golden`, `run-exit-code.golden`, `run-mounts-denied.golden`, `shell-env.golden`, `shell-no-mount-no-config.golden`, `shell.golden`, `too-many-args.golden`, `unknown-flag.golden`, `unsupported-agent.golden`, `verbose-claude.golden`, `version.golden`, `workdir-flag.golden`, `workdir-home-allowed.golden`, `workdir-is-home.golden`, `workdir-missing.golden`, `workdir-relative.golden`, `workdir-root-allowed.golden`, `workdir-root.golden`
- `dev/` (1 files): `Dockerfile`
- `docs/` (3 files): `architecture.md`, `implementation.md`, `spec.md`
- `internal/agent/` (12 files): `agent.go`, `agent_test.go`, `builtin.go`, `claude.go`, `codex.go`, `copilot.go`, `definitions_test.go`, `gemini.go`, `gh.go`, `pi.go`, `registry.go`, `registry_test.go`
- `internal/agent/testdata/` (1 files): `builtin_agents.golden`
- `internal/app/` (4 files): `planner.go`, `planner_test.go`, `service.go`, `service_test.go`
- `internal/cli/` (3 files): `doc.go`, `root.go`, `root_test.go`
- `internal/docker/` (6 files): `args.go`, `args_test.go`, `client.go`, `client_test.go`, `doc.go`, `platform.go`
- `internal/hostconfig/` (4 files): `resolver.go`, `resolver_edges_test.go`, `resolver_test.go`, `tree.go`
- `internal/plan/` (2 files): `run.go`, `run_test.go`
- `internal/project/` (2 files): `mount.go`, `mount_test.go`
- `internal/runtime/` (5 files): `Dockerfile`, `assets.go`, `caged-entrypoint.sh`, `image.go`, `runtime_test.go`
- `internal/system/` (2 files): `system.go`, `system_test.go`


## Step 12: Add an architecture test that enforces the structure

**Goal:** Add a test that fails if a package imports something it should not, if OS access appears outside `internal/system`, or if package-level mutable state comes back.

**Why:** The structure is now checked on every `go test`, so it cannot quietly decay.

### Files touched

- `cmd/caged/architecture_test.go`: create this new file.

### Commands to run

No shell commands are needed in this step. You only edit and create files.

### File edits

#### File 1 of 1: `cmd/caged/architecture_test.go`

This is a NEW FILE (213 lines).

Imports: this new file has these import lines: `"go/ast"`, `"go/parser"`, `"go/token"`, `"io/fs"`, `"path/filepath"`, `"sort"`, `"strconv"`, `"strings"`, `"testing"`. (The file content below shows how they are grouped.)

Create the file `cmd/caged/architecture_test.go` with exactly this content:

````go
package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const modulePath = "github.com/JeremyOrr4/caged"

// allowedImports lists the caged packages each package may import. It is the
// dependency direction documented in docs/architecture.md: policy (app) never
// imports an adapter, and agent and system import nothing from caged.
func allowedImports() map[string][]string {
	return map[string][]string{
		"cmd/caged":           {"internal/agent", "internal/app", "internal/cli", "internal/docker", "internal/hostconfig", "internal/project", "internal/system"},
		"internal/cli":        {"internal/agent", "internal/app", "internal/runtime"},
		"internal/app":        {"internal/agent", "internal/plan", "internal/runtime"},
		"internal/docker":     {"internal/plan", "internal/system"},
		"internal/hostconfig": {"internal/agent", "internal/plan"},
		"internal/project":    {"internal/plan"},
		"internal/runtime":    {"internal/plan"},
		"internal/plan":       {"internal/agent"},
		"internal/agent":      {},
		"internal/system":     {},
	}
}

// osPackages give direct access to the operating system.
var osPackages = map[string]bool{"os": true, "os/exec": true, "os/signal": true, "os/user": true, "syscall": true, "runtime": true}

// filepathIO are the path/filepath functions that read the filesystem.
var filepathIO = map[string]bool{"Abs": true, "EvalSymlinks": true, "Glob": true, "Walk": true, "WalkDir": true}

type sourceFile struct {
	pkg  string // directory relative to the module root, e.g. "internal/app"
	path string
	ast  *ast.File
}

// productionFiles parses every non-test Go file in the module.
func productionFiles(t *testing.T) []sourceFile {
	t.Helper()
	root := filepath.Join("..", "..")
	var files []sourceFile
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && (entry.Name() == ".git" || entry.Name() == "testdata") {
			return filepath.SkipDir
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		files = append(files, sourceFile{pkg: filepath.ToSlash(rel), path: path, ast: parsed})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no Go files found")
	}
	return files
}

func importPaths(file *ast.File) []string {
	var paths []string
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err == nil {
			paths = append(paths, path)
		}
	}
	return paths
}

func TestPackageDependenciesPointInward(t *testing.T) {
	t.Parallel()

	allowed := allowedImports()
	for _, file := range productionFiles(t) {
		permitted, known := allowed[file.pkg]
		if !known {
			t.Errorf("%s: package %s is not in allowedImports; add it and document it in docs/architecture.md", file.path, file.pkg)
			continue
		}
		for _, path := range importPaths(file.ast) {
			if !strings.HasPrefix(path, modulePath+"/") {
				continue
			}
			target := strings.TrimPrefix(path, modulePath+"/")
			ok := false
			for _, p := range permitted {
				ok = ok || p == target
			}
			if !ok {
				t.Errorf("%s: %s must not import %s", file.path, file.pkg, target)
			}
		}
	}
}

func TestAllowedDependencyGraphHasNoCycles(t *testing.T) {
	t.Parallel()

	allowed := allowedImports()
	const (
		unvisited = iota
		visiting
		done
	)
	state := map[string]int{}
	var visit func(pkg string, trail []string)
	visit = func(pkg string, trail []string) {
		switch state[pkg] {
		case visiting:
			t.Fatalf("dependency cycle: %s -> %s", strings.Join(trail, " -> "), pkg)
		case done:
			return
		}
		state[pkg] = visiting
		for _, dep := range allowed[pkg] {
			visit(dep, append(trail, pkg))
		}
		state[pkg] = done
	}
	pkgs := make([]string, 0, len(allowed))
	for pkg := range allowed {
		pkgs = append(pkgs, pkg)
	}
	sort.Strings(pkgs)
	for _, pkg := range pkgs {
		visit(pkg, nil)
	}
}

func TestOnlySystemTouchesTheOperatingSystem(t *testing.T) {
	t.Parallel()

	for _, file := range productionFiles(t) {
		adapter := file.pkg == "internal/system"
		if adapter {
			continue
		}
		for _, path := range importPaths(file.ast) {
			if osPackages[path] && file.pkg != "cmd/caged" {
				t.Errorf("%s imports %q; only internal/system (and the composition root) may", file.path, path)
			}
		}
		ast.Inspect(file.ast, func(node ast.Node) bool {
			call, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if pkg, ok := call.X.(*ast.Ident); ok && pkg.Name == "filepath" && filepathIO[call.Sel.Name] {
				t.Errorf("%s calls filepath.%s, which reads the filesystem; use an injected FileSystem", file.path, call.Sel.Name)
			}
			return true
		})
	}
}

func TestNoPackageLevelMutableState(t *testing.T) {
	t.Parallel()

	for _, file := range productionFiles(t) {
		for _, decl := range file.ast.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			if embedsFile(gen) {
				continue // go:embed requires a var; the embedded strings are never assigned
			}
			for _, spec := range gen.Specs {
				for _, name := range spec.(*ast.ValueSpec).Names {
					if file.pkg == "cmd/caged" && name.Name == "version" {
						continue // set by -ldflags "-X main.version=..." at link time
					}
					t.Errorf("%s: package-level var %s; use a constant, a function, or a field", file.path, name.Name)
				}
			}
		}
	}
}

func embedsFile(decl *ast.GenDecl) bool {
	if decl.Doc == nil {
		return false
	}
	for _, comment := range decl.Doc.List {
		if strings.HasPrefix(comment.Text, "//go:embed ") {
			return true
		}
	}
	return false
}
````

(The unified diff of a new file is every line above with a `+` in front, so it is not repeated here.)

### Verify

Run these verification commands from the repository root:

````bash
gofmt -l .
go build ./...
go vet ./...
go test -count=1 ./...
````

Expected result:

- `gofmt -l .` prints nothing.
- `go build ./...` prints nothing.
- `go vet ./...` prints nothing.
- `go test -count=1 ./...` prints the lines below. The numbers of seconds at the end of the `ok` lines will be different on your machine, and lines may come in a different order. Every package must say `ok` or `[no test files]`:

````text
ok  	github.com/JeremyOrr4/caged/cmd/caged	1.023s
ok  	github.com/JeremyOrr4/caged/internal/agent	0.009s
ok  	github.com/JeremyOrr4/caged/internal/app	0.007s
ok  	github.com/JeremyOrr4/caged/internal/cli	0.006s
ok  	github.com/JeremyOrr4/caged/internal/docker	0.005s
ok  	github.com/JeremyOrr4/caged/internal/hostconfig	0.010s
ok  	github.com/JeremyOrr4/caged/internal/plan	0.003s
ok  	github.com/JeremyOrr4/caged/internal/project	0.009s
ok  	github.com/JeremyOrr4/caged/internal/runtime	0.003s
ok  	github.com/JeremyOrr4/caged/internal/system	0.005s
````

Count the passing tests and subtests with `go test -count=1 -v ./... 2>&1 | grep -c -- "--- PASS"`. On Linux the expected number is **254** (all 102 of them run on every OS).

Run this command to compute the fingerprint of the whole repository (on macOS use the second command):

````bash
find . -type f -not -path './.git/*' -not -name REFACTOR_INSTRUCTIONS.md -print0 | LC_ALL=C sort -z | xargs -0 sha256sum | sha256sum
````

````bash
find . -type f -not -path './.git/*' -not -name REFACTOR_INSTRUCTIONS.md -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256 | shasum -a 256
````

Expected output (exactly this line):

````text
faa20739e5c49b865b7149452561e535bdaa68fd3d57857dd833953cd0e8fae7  -
````

**If it fails:**

- Problem: `TestPackageDependenciesPointInward` reports `package ... is not in allowedImports`.
  Fix: A stray `.go` file exists somewhere, for example a backup copy. List all `.go` files with `find . -name '*.go' -not -path './.git/*'` and delete anything that is not in the checkpoint list.
- Problem: the fingerprint is different but every test passes.
  Fix: some file differs from these instructions by a character, often a tab turned into spaces or a missing final newline. Run `gofmt -w .` (it fixes Go indentation), then check the non-Go files of this step again.

### Checkpoint

At this point, the repo should have these 156 files (not counting `.git/` and this instructions file):

- `./` (5 files): `.gitignore`, `Makefile`, `README.md`, `go.mod`, `go.sum`
- `cmd/caged/` (4 files): `architecture_test.go`, `golden_test.go`, `main.go`, `main_test.go`
- `cmd/caged/testdata/golden/darwin/` (51 files): `agent-claude.golden`, `agent-codex.golden`, `agent-copilot.golden`, `agent-gemini.golden`, `agent-gh.golden`, `agent-pi.golden`, `agent-uppercase.golden`, `build-daemon-down.golden`, `build-help.golden`, `build-no-cache.golden`, `build-rejects-args.golden`, `build.golden`, `complete-agents.golden`, `complete-prompt.golden`, `config-overrides.golden`, `daemon-down.golden`, `docker-missing.golden`, `env-extra-names.golden`, `env-invalid.golden`, `env-padded-name.golden`, `help-list.golden`, `help-unknown-topic.golden`, `help.golden`, `home-unresolvable.golden`, `image-missing-builds-first.golden`, `list-rejects-args.golden`, `list.golden`, `no-mount-with-workdir.golden`, `prompt-claude.golden`, `prompt-codex.golden`, `prompt-copilot.golden`, `prompt-gemini.golden`, `prompt-gh.golden`, `prompt-pi.golden`, `run-exit-code.golden`, `run-mounts-denied.golden`, `shell-env.golden`, `shell-no-mount-no-config.golden`, `shell.golden`, `too-many-args.golden`, `unknown-flag.golden`, `unsupported-agent.golden`, `verbose-claude.golden`, `version.golden`, `workdir-flag.golden`, `workdir-home-allowed.golden`, `workdir-is-home.golden`, `workdir-missing.golden`, `workdir-relative.golden`, `workdir-root-allowed.golden`, `workdir-root.golden`
- `cmd/caged/testdata/golden/linux/` (51 files): `agent-claude.golden`, `agent-codex.golden`, `agent-copilot.golden`, `agent-gemini.golden`, `agent-gh.golden`, `agent-pi.golden`, `agent-uppercase.golden`, `build-daemon-down.golden`, `build-help.golden`, `build-no-cache.golden`, `build-rejects-args.golden`, `build.golden`, `complete-agents.golden`, `complete-prompt.golden`, `config-overrides.golden`, `daemon-down.golden`, `docker-missing.golden`, `env-extra-names.golden`, `env-invalid.golden`, `env-padded-name.golden`, `help-list.golden`, `help-unknown-topic.golden`, `help.golden`, `home-unresolvable.golden`, `image-missing-builds-first.golden`, `list-rejects-args.golden`, `list.golden`, `no-mount-with-workdir.golden`, `prompt-claude.golden`, `prompt-codex.golden`, `prompt-copilot.golden`, `prompt-gemini.golden`, `prompt-gh.golden`, `prompt-pi.golden`, `run-exit-code.golden`, `run-mounts-denied.golden`, `shell-env.golden`, `shell-no-mount-no-config.golden`, `shell.golden`, `too-many-args.golden`, `unknown-flag.golden`, `unsupported-agent.golden`, `verbose-claude.golden`, `version.golden`, `workdir-flag.golden`, `workdir-home-allowed.golden`, `workdir-is-home.golden`, `workdir-missing.golden`, `workdir-relative.golden`, `workdir-root-allowed.golden`, `workdir-root.golden`
- `dev/` (1 files): `Dockerfile`
- `docs/` (3 files): `architecture.md`, `implementation.md`, `spec.md`
- `internal/agent/` (12 files): `agent.go`, `agent_test.go`, `builtin.go`, `claude.go`, `codex.go`, `copilot.go`, `definitions_test.go`, `gemini.go`, `gh.go`, `pi.go`, `registry.go`, `registry_test.go`
- `internal/agent/testdata/` (1 files): `builtin_agents.golden`
- `internal/app/` (4 files): `planner.go`, `planner_test.go`, `service.go`, `service_test.go`
- `internal/cli/` (3 files): `doc.go`, `root.go`, `root_test.go`
- `internal/docker/` (6 files): `args.go`, `args_test.go`, `client.go`, `client_test.go`, `doc.go`, `platform.go`
- `internal/hostconfig/` (4 files): `resolver.go`, `resolver_edges_test.go`, `resolver_test.go`, `tree.go`
- `internal/plan/` (2 files): `run.go`, `run_test.go`
- `internal/project/` (2 files): `mount.go`, `mount_test.go`
- `internal/runtime/` (5 files): `Dockerfile`, `assets.go`, `caged-entrypoint.sh`, `image.go`, `runtime_test.go`
- `internal/system/` (2 files): `system.go`, `system_test.go`


## Step 13: Close the remaining test gap in plan

**Goal:** Add a test for the shell case and the two error cases of `plan.BuildRunSpec`.

**Why:** This brings `plan` to 100% coverage, so every package now has the same or higher coverage than before the refactor.

### Files touched

- `internal/plan/run_test.go`: modify.

### Commands to run

No shell commands are needed in this step. You only edit and create files.

### File edits

#### File 1 of 1: `internal/plan/run_test.go`

Imports: no import lines are added or removed in this file.

There is 1 edit in this file. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 1: lines 57-59.**

Find this exact text (it appears exactly once in the file):

````go
		t.Fatal("run spec mutated source inputs")
	}
}
````

Replace it with this exact text:

````go
		t.Fatal("run spec mutated source inputs")
	}
}

func TestBuildRunSpecShellAndErrors(t *testing.T) {
	t.Parallel()

	mount := &Mount{HostPath: "/project", ContainerPath: "/workspace/project"}
	spec, err := BuildRunSpec("image", nil, nil, mount, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Image != "image" || !reflect.DeepEqual(spec.Command, []string{"bash"}) || spec.ReturnToShell {
		t.Fatalf("shell spec = %+v, want bash without return-to-shell", spec)
	}
	spec.Mount.HostPath = "changed"
	if mount.HostPath != "/project" {
		t.Fatal("run spec shares the project mount with its input")
	}

	prompt := "hello"
	if _, err := BuildRunSpec("image", nil, &prompt, nil, nil, nil); err == nil || err.Error() != "a prompt requires an agent" {
		t.Fatalf("shell with prompt error = %v", err)
	}
	gh, err := agent.Builtin().Lookup("gh")
	if err != nil {
		t.Fatal(err)
	}
	if spec, err := BuildRunSpec("image", &gh, &prompt, nil, nil, nil); err == nil || err.Error() != "gh does not support prompt launch" || spec.Image != "" {
		t.Fatalf("gh with prompt = %+v, %v; want an error and an empty spec", spec, err)
	}
}
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/plan/run_test.go
+++ b/internal/plan/run_test.go
@@ -57,3 +57,32 @@
 		t.Fatal("run spec mutated source inputs")
 	}
 }
+
+func TestBuildRunSpecShellAndErrors(t *testing.T) {
+	t.Parallel()
+
+	mount := &Mount{HostPath: "/project", ContainerPath: "/workspace/project"}
+	spec, err := BuildRunSpec("image", nil, nil, mount, nil, nil)
+	if err != nil {
+		t.Fatal(err)
+	}
+	if spec.Image != "image" || !reflect.DeepEqual(spec.Command, []string{"bash"}) || spec.ReturnToShell {
+		t.Fatalf("shell spec = %+v, want bash without return-to-shell", spec)
+	}
+	spec.Mount.HostPath = "changed"
+	if mount.HostPath != "/project" {
+		t.Fatal("run spec shares the project mount with its input")
+	}
+
+	prompt := "hello"
+	if _, err := BuildRunSpec("image", nil, &prompt, nil, nil, nil); err == nil || err.Error() != "a prompt requires an agent" {
+		t.Fatalf("shell with prompt error = %v", err)
+	}
+	gh, err := agent.Builtin().Lookup("gh")
+	if err != nil {
+		t.Fatal(err)
+	}
+	if spec, err := BuildRunSpec("image", &gh, &prompt, nil, nil, nil); err == nil || err.Error() != "gh does not support prompt launch" || spec.Image != "" {
+		t.Fatalf("gh with prompt = %+v, %v; want an error and an empty spec", spec, err)
+	}
+}
````

### Verify

Run these verification commands from the repository root:

````bash
gofmt -l .
go build ./...
go vet ./...
go test -count=1 ./...
````

Expected result:

- `gofmt -l .` prints nothing.
- `go build ./...` prints nothing.
- `go vet ./...` prints nothing.
- `go test -count=1 ./...` prints the lines below. The numbers of seconds at the end of the `ok` lines will be different on your machine, and lines may come in a different order. Every package must say `ok` or `[no test files]`:

````text
ok  	github.com/JeremyOrr4/caged/cmd/caged	1.104s
ok  	github.com/JeremyOrr4/caged/internal/agent	0.005s
ok  	github.com/JeremyOrr4/caged/internal/app	0.005s
ok  	github.com/JeremyOrr4/caged/internal/cli	0.010s
ok  	github.com/JeremyOrr4/caged/internal/docker	0.010s
ok  	github.com/JeremyOrr4/caged/internal/hostconfig	0.051s
ok  	github.com/JeremyOrr4/caged/internal/plan	0.016s
ok  	github.com/JeremyOrr4/caged/internal/project	0.014s
ok  	github.com/JeremyOrr4/caged/internal/runtime	0.003s
ok  	github.com/JeremyOrr4/caged/internal/system	0.005s
````

Count the passing tests and subtests with `go test -count=1 -v ./... 2>&1 | grep -c -- "--- PASS"`. On Linux the expected number is **255** (all 102 of them run on every OS).

Run this command to compute the fingerprint of the whole repository (on macOS use the second command):

````bash
find . -type f -not -path './.git/*' -not -name REFACTOR_INSTRUCTIONS.md -print0 | LC_ALL=C sort -z | xargs -0 sha256sum | sha256sum
````

````bash
find . -type f -not -path './.git/*' -not -name REFACTOR_INSTRUCTIONS.md -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256 | shasum -a 256
````

Expected output (exactly this line):

````text
a7052c65b9c6d4d25065095ca88355eaee4794b181c804df9cb223aac27c85f4  -
````

**If it fails:**

- Problem: `go vet` says `undefined: agent` in `internal/plan/run_test.go`.
  Fix: The new test goes at the END of the existing file, which already imports `agent` and `reflect`. Do not create a new file.
- Problem: the fingerprint is different but every test passes.
  Fix: some file differs from these instructions by a character, often a tab turned into spaces or a missing final newline. Run `gofmt -w .` (it fixes Go indentation), then check the non-Go files of this step again.

### Checkpoint

At this point, the repo should have these 156 files (not counting `.git/` and this instructions file):

- `./` (5 files): `.gitignore`, `Makefile`, `README.md`, `go.mod`, `go.sum`
- `cmd/caged/` (4 files): `architecture_test.go`, `golden_test.go`, `main.go`, `main_test.go`
- `cmd/caged/testdata/golden/darwin/` (51 files): `agent-claude.golden`, `agent-codex.golden`, `agent-copilot.golden`, `agent-gemini.golden`, `agent-gh.golden`, `agent-pi.golden`, `agent-uppercase.golden`, `build-daemon-down.golden`, `build-help.golden`, `build-no-cache.golden`, `build-rejects-args.golden`, `build.golden`, `complete-agents.golden`, `complete-prompt.golden`, `config-overrides.golden`, `daemon-down.golden`, `docker-missing.golden`, `env-extra-names.golden`, `env-invalid.golden`, `env-padded-name.golden`, `help-list.golden`, `help-unknown-topic.golden`, `help.golden`, `home-unresolvable.golden`, `image-missing-builds-first.golden`, `list-rejects-args.golden`, `list.golden`, `no-mount-with-workdir.golden`, `prompt-claude.golden`, `prompt-codex.golden`, `prompt-copilot.golden`, `prompt-gemini.golden`, `prompt-gh.golden`, `prompt-pi.golden`, `run-exit-code.golden`, `run-mounts-denied.golden`, `shell-env.golden`, `shell-no-mount-no-config.golden`, `shell.golden`, `too-many-args.golden`, `unknown-flag.golden`, `unsupported-agent.golden`, `verbose-claude.golden`, `version.golden`, `workdir-flag.golden`, `workdir-home-allowed.golden`, `workdir-is-home.golden`, `workdir-missing.golden`, `workdir-relative.golden`, `workdir-root-allowed.golden`, `workdir-root.golden`
- `cmd/caged/testdata/golden/linux/` (51 files): `agent-claude.golden`, `agent-codex.golden`, `agent-copilot.golden`, `agent-gemini.golden`, `agent-gh.golden`, `agent-pi.golden`, `agent-uppercase.golden`, `build-daemon-down.golden`, `build-help.golden`, `build-no-cache.golden`, `build-rejects-args.golden`, `build.golden`, `complete-agents.golden`, `complete-prompt.golden`, `config-overrides.golden`, `daemon-down.golden`, `docker-missing.golden`, `env-extra-names.golden`, `env-invalid.golden`, `env-padded-name.golden`, `help-list.golden`, `help-unknown-topic.golden`, `help.golden`, `home-unresolvable.golden`, `image-missing-builds-first.golden`, `list-rejects-args.golden`, `list.golden`, `no-mount-with-workdir.golden`, `prompt-claude.golden`, `prompt-codex.golden`, `prompt-copilot.golden`, `prompt-gemini.golden`, `prompt-gh.golden`, `prompt-pi.golden`, `run-exit-code.golden`, `run-mounts-denied.golden`, `shell-env.golden`, `shell-no-mount-no-config.golden`, `shell.golden`, `too-many-args.golden`, `unknown-flag.golden`, `unsupported-agent.golden`, `verbose-claude.golden`, `version.golden`, `workdir-flag.golden`, `workdir-home-allowed.golden`, `workdir-is-home.golden`, `workdir-missing.golden`, `workdir-relative.golden`, `workdir-root-allowed.golden`, `workdir-root.golden`
- `dev/` (1 files): `Dockerfile`
- `docs/` (3 files): `architecture.md`, `implementation.md`, `spec.md`
- `internal/agent/` (12 files): `agent.go`, `agent_test.go`, `builtin.go`, `claude.go`, `codex.go`, `copilot.go`, `definitions_test.go`, `gemini.go`, `gh.go`, `pi.go`, `registry.go`, `registry_test.go`
- `internal/agent/testdata/` (1 files): `builtin_agents.golden`
- `internal/app/` (4 files): `planner.go`, `planner_test.go`, `service.go`, `service_test.go`
- `internal/cli/` (3 files): `doc.go`, `root.go`, `root_test.go`
- `internal/docker/` (6 files): `args.go`, `args_test.go`, `client.go`, `client_test.go`, `doc.go`, `platform.go`
- `internal/hostconfig/` (4 files): `resolver.go`, `resolver_edges_test.go`, `resolver_test.go`, `tree.go`
- `internal/plan/` (2 files): `run.go`, `run_test.go`
- `internal/project/` (2 files): `mount.go`, `mount_test.go`
- `internal/runtime/` (5 files): `Dockerfile`, `assets.go`, `caged-entrypoint.sh`, `image.go`, `runtime_test.go`
- `internal/system/` (2 files): `system.go`, `system_test.go`


## Step 14: Bring the docs up to date

**Goal:** Make `docs/architecture.md`, `docs/implementation.md`, `README.md` and the `docker` package comment describe the final code exactly.

**Why:** The dependency table in the architecture doc now mirrors the architecture test one-to-one.

### Files touched

- `README.md`: modify.
- `docs/architecture.md`: modify.
- `docs/implementation.md`: modify.
- `internal/docker/doc.go`: modify.

### Commands to run

No shell commands are needed in this step. You only edit and create files.

### File edits

#### File 1 of 4: `README.md`

Imports: this is not a Go file, so there are no imports.

There is 1 edit in this file. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 1: lines 70-73.**

Find this exact text (it appears exactly once in the file):

````markdown
```

The development image uses [dev/Dockerfile](dev/Dockerfile); the embedded
runtime image is defined in `internal/runtime/Dockerfile`.
````

Replace it with this exact text:

````markdown
```

The development image uses [dev/Dockerfile](dev/Dockerfile); the embedded
runtime image is defined in `internal/runtime/Dockerfile`. The package layout,
the dependency rules, and how to add an agent are described in
[docs/architecture.md](docs/architecture.md).
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/README.md
+++ b/README.md
@@ -70,4 +70,6 @@
 ```
 
 The development image uses [dev/Dockerfile](dev/Dockerfile); the embedded
-runtime image is defined in `internal/runtime/Dockerfile`.
+runtime image is defined in `internal/runtime/Dockerfile`. The package layout,
+the dependency rules, and how to add an agent are described in
+[docs/architecture.md](docs/architecture.md).
````

#### File 2 of 4: `docs/architecture.md`

Imports: this is not a Go file, so there are no imports.

There are 2 edits in this file. They are listed from the bottom of the file to the top. Do them in the order shown: each edit only changes lines below the next one, so the line numbers stay correct. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 2: lines 94-102.**

Find this exact text (it appears exactly once in the file):

````markdown
process inputs (`system.CurrentStdio()`, `system.CurrentPlatform()`,
`agent.Builtin()`) and passes them to `run`. `run` builds every component,
wires them together, executes the command line, and returns the exit code.
Tests call `run` with a fake `docker` on `PATH`, buffers for stdio, and an
injected platform. This lets the full Linux and macOS behaviour be checked
on any machine.

## Platform-specific behaviour

````

Replace it with this exact text:

````markdown
process inputs (`system.CurrentStdio()`, `system.CurrentPlatform()`,
`agent.Builtin()`) and passes them to `run`. `run` builds every component,
wires them together, executes the command line, and returns the exit code.
Tests call `run` with a fake `docker` on `PATH`, captured stdio, and an
injected platform. This lets the full Linux and macOS behaviour be checked
on any machine. Cobra keeps its default output writers on purpose: calling
`SetOut` would also move Cobra's stderr-bound usage text to stdout.

## Platform-specific behaviour

````

**Edit 2 of 2: lines 41-65.**

Find this exact text (it appears exactly once in the file):

````markdown

## Dependency direction

```text
cmd/caged ──> cli ──> app ──> plan ──> agent
   │           │       │
   │           │       └────> runtime ──> plan
   │           └────────────> agent, runtime
   ├──> hostconfig ──> agent, plan
   ├──> project ──────> plan
   ├──> docker ───────> plan, system
   └──> system (stdlib only)
```

Arrows point from policy to the things policy needs. `app` (the use cases)
imports no adapter package: it does not know about Docker, the filesystem, or
the environment. Those reach it through interfaces that `app` itself declares.
`agent` and `system` import only the standard library. There are no import
cycles. `cmd/caged/architecture_test.go` enforces these rules: it fails if a
package imports something outside its allowed list, or if any package other
than `internal/system` and `cmd/caged` imports `os`, `os/exec`, `syscall`, or
the standard `runtime` package.

## Key interfaces

````

Replace it with this exact text:

````markdown

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

````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/docs/architecture.md
+++ b/docs/architecture.md
@@ -41,25 +41,30 @@
 
 ## Dependency direction
 
-```text
-cmd/caged ──> cli ──> app ──> plan ──> agent
-   │           │       │
-   │           │       └────> runtime ──> plan
-   │           └────────────> agent, runtime
-   ├──> hostconfig ──> agent, plan
-   ├──> project ──────> plan
-   ├──> docker ───────> plan, system
-   └──> system (stdlib only)
-```
-
-Arrows point from policy to the things policy needs. `app` (the use cases)
-imports no adapter package: it does not know about Docker, the filesystem, or
-the environment. Those reach it through interfaces that `app` itself declares.
-`agent` and `system` import only the standard library. There are no import
-cycles. `cmd/caged/architecture_test.go` enforces these rules: it fails if a
-package imports something outside its allowed list, or if any package other
-than `internal/system` and `cmd/caged` imports `os`, `os/exec`, `syscall`, or
-the standard `runtime` package.
+| Package | May import (caged packages) |
+|---|---|
+| `cmd/caged` | `agent`, `app`, `cli`, `docker`, `hostconfig`, `project`, `system` |
+| `internal/cli` | `agent`, `app`, `runtime` |
+| `internal/app` | `agent`, `plan`, `runtime` |
+| `internal/docker` | `plan`, `system` |
+| `internal/hostconfig` | `agent`, `plan` |
+| `internal/project` | `plan` |
+| `internal/runtime` | `plan` |
+| `internal/plan` | `agent` |
+| `internal/agent` | nothing |
+| `internal/system` | nothing |
+
+Dependencies point from policy to the things policy needs. `app` (the use
+cases) imports no adapter package: it does not know about Docker, the
+filesystem, or the environment. Those reach it through interfaces that `app`
+itself declares. `agent` and `system` import only the standard library. There
+are no import cycles. `cmd/caged/architecture_test.go` enforces this table
+exactly. It also fails if any package other than `internal/system` and
+`cmd/caged` imports `os`, `os/exec`, `os/signal`, `os/user`, `syscall`, or the
+standard `runtime` package. It fails if any package other than
+`internal/system` calls `filepath.Abs`, `EvalSymlinks`, `Glob`, `Walk`, or
+`WalkDir`. It also fails on any package-level `var`, except `main.version`
+(set by `-ldflags`) and the `//go:embed` files.
 
 ## Key interfaces
 
@@ -94,9 +99,10 @@
 process inputs (`system.CurrentStdio()`, `system.CurrentPlatform()`,
 `agent.Builtin()`) and passes them to `run`. `run` builds every component,
 wires them together, executes the command line, and returns the exit code.
-Tests call `run` with a fake `docker` on `PATH`, buffers for stdio, and an
+Tests call `run` with a fake `docker` on `PATH`, captured stdio, and an
 injected platform. This lets the full Linux and macOS behaviour be checked
-on any machine.
+on any machine. Cobra keeps its default output writers on purpose: calling
+`SetOut` would also move Cobra's stderr-bound usage text to stdout.
 
 ## Platform-specific behaviour
 
````

#### File 3 of 4: `docs/implementation.md`

Imports: this is not a Go file, so there are no imports.

There is 1 edit in this file. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 1: lines 36-42.**

Find this exact text (it appears exactly once in the file):

````markdown
```

The development image is `dev/Dockerfile`; the runtime Dockerfile and entrypoint
under `internal/runtime/` are embedded in the binary. Agent data, host-config
resolution, project validation, run planning, Docker execution, and application
coordination are separated into `internal/agent`, `internal/hostconfig`,
`internal/project`, `internal/plan`, `internal/docker`, and `internal/app`.
````

Replace it with this exact text:

````markdown
```

The development image is `dev/Dockerfile`; the runtime Dockerfile and entrypoint
under `internal/runtime/` are embedded in the binary. Command-line parsing,
agent data, host-config resolution, project validation, run planning, Docker
execution, and application coordination are separated into `internal/cli`,
`internal/agent`, `internal/hostconfig`, `internal/project`, `internal/plan`,
`internal/docker`, and `internal/app`. Only `internal/system` touches the
operating system, and `cmd/caged/main.go` wires everything together. See
`docs/architecture.md` for the interfaces and the dependency rules.
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/docs/implementation.md
+++ b/docs/implementation.md
@@ -36,7 +36,10 @@
 ```
 
 The development image is `dev/Dockerfile`; the runtime Dockerfile and entrypoint
-under `internal/runtime/` are embedded in the binary. Agent data, host-config
-resolution, project validation, run planning, Docker execution, and application
-coordination are separated into `internal/agent`, `internal/hostconfig`,
-`internal/project`, `internal/plan`, `internal/docker`, and `internal/app`.
+under `internal/runtime/` are embedded in the binary. Command-line parsing,
+agent data, host-config resolution, project validation, run planning, Docker
+execution, and application coordination are separated into `internal/cli`,
+`internal/agent`, `internal/hostconfig`, `internal/project`, `internal/plan`,
+`internal/docker`, and `internal/app`. Only `internal/system` touches the
+operating system, and `cmd/caged/main.go` wires everything together. See
+`docs/architecture.md` for the interfaces and the dependency rules.
````

#### File 4 of 4: `internal/docker/doc.go`

Imports: no import lines are added or removed in this file.

There is 1 edit in this file. Line numbers are for the file as it is at the START of this step.

**Edit 1 of 1: lines 1-2.**

Find this exact text (it appears exactly once in the file):

````go
// Package docker translates run plans into Docker commands and executes containers.
package docker
````

Replace it with this exact text:

````go
// Package docker is the Docker CLI adapter. It turns run and build plans into
// docker commands and executes them through an injected Executor.
package docker
````

For reference, this is the unified diff of this file for this step (real `diff -u` output):

````diff
--- a/internal/docker/doc.go
+++ b/internal/docker/doc.go
@@ -1,2 +1,3 @@
-// Package docker translates run plans into Docker commands and executes containers.
+// Package docker is the Docker CLI adapter. It turns run and build plans into
+// docker commands and executes them through an injected Executor.
 package docker
````

### Verify

Run these verification commands from the repository root:

````bash
gofmt -l .
go build ./...
go vet ./...
go test -count=1 ./...
````

Expected result:

- `gofmt -l .` prints nothing.
- `go build ./...` prints nothing.
- `go vet ./...` prints nothing.
- `go test -count=1 ./...` prints the lines below. The numbers of seconds at the end of the `ok` lines will be different on your machine, and lines may come in a different order. Every package must say `ok` or `[no test files]`:

````text
ok  	github.com/JeremyOrr4/caged/cmd/caged	1.013s
ok  	github.com/JeremyOrr4/caged/internal/agent	0.009s
ok  	github.com/JeremyOrr4/caged/internal/app	0.008s
ok  	github.com/JeremyOrr4/caged/internal/cli	0.014s
ok  	github.com/JeremyOrr4/caged/internal/docker	0.006s
ok  	github.com/JeremyOrr4/caged/internal/hostconfig	0.011s
ok  	github.com/JeremyOrr4/caged/internal/plan	0.007s
ok  	github.com/JeremyOrr4/caged/internal/project	0.004s
ok  	github.com/JeremyOrr4/caged/internal/runtime	0.003s
ok  	github.com/JeremyOrr4/caged/internal/system	0.006s
````

Count the passing tests and subtests with `go test -count=1 -v ./... 2>&1 | grep -c -- "--- PASS"`. On Linux the expected number is **255** (all 102 of them run on every OS).

Run this command to compute the fingerprint of the whole repository (on macOS use the second command):

````bash
find . -type f -not -path './.git/*' -not -name REFACTOR_INSTRUCTIONS.md -print0 | LC_ALL=C sort -z | xargs -0 sha256sum | sha256sum
````

````bash
find . -type f -not -path './.git/*' -not -name REFACTOR_INSTRUCTIONS.md -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256 | shasum -a 256
````

Expected output (exactly this line):

````text
b2c48e4a79e4affc0882414ba6479c34b8fb3e7877ea42a04842f4bce05ba73f  -
````

**If it fails:**

- Problem: The fingerprint does not match although the tests pass.
  Fix: One of the four documents differs. Usually it is trailing spaces or a missing blank line. Compare each one with the full text in Appendix A.
- Problem: the fingerprint is different but every test passes.
  Fix: some file differs from these instructions by a character, often a tab turned into spaces or a missing final newline. Run `gofmt -w .` (it fixes Go indentation), then check the non-Go files of this step again.

### Checkpoint

At this point, the repo should have these 156 files (not counting `.git/` and this instructions file):

- `./` (5 files): `.gitignore`, `Makefile`, `README.md`, `go.mod`, `go.sum`
- `cmd/caged/` (4 files): `architecture_test.go`, `golden_test.go`, `main.go`, `main_test.go`
- `cmd/caged/testdata/golden/darwin/` (51 files): `agent-claude.golden`, `agent-codex.golden`, `agent-copilot.golden`, `agent-gemini.golden`, `agent-gh.golden`, `agent-pi.golden`, `agent-uppercase.golden`, `build-daemon-down.golden`, `build-help.golden`, `build-no-cache.golden`, `build-rejects-args.golden`, `build.golden`, `complete-agents.golden`, `complete-prompt.golden`, `config-overrides.golden`, `daemon-down.golden`, `docker-missing.golden`, `env-extra-names.golden`, `env-invalid.golden`, `env-padded-name.golden`, `help-list.golden`, `help-unknown-topic.golden`, `help.golden`, `home-unresolvable.golden`, `image-missing-builds-first.golden`, `list-rejects-args.golden`, `list.golden`, `no-mount-with-workdir.golden`, `prompt-claude.golden`, `prompt-codex.golden`, `prompt-copilot.golden`, `prompt-gemini.golden`, `prompt-gh.golden`, `prompt-pi.golden`, `run-exit-code.golden`, `run-mounts-denied.golden`, `shell-env.golden`, `shell-no-mount-no-config.golden`, `shell.golden`, `too-many-args.golden`, `unknown-flag.golden`, `unsupported-agent.golden`, `verbose-claude.golden`, `version.golden`, `workdir-flag.golden`, `workdir-home-allowed.golden`, `workdir-is-home.golden`, `workdir-missing.golden`, `workdir-relative.golden`, `workdir-root-allowed.golden`, `workdir-root.golden`
- `cmd/caged/testdata/golden/linux/` (51 files): `agent-claude.golden`, `agent-codex.golden`, `agent-copilot.golden`, `agent-gemini.golden`, `agent-gh.golden`, `agent-pi.golden`, `agent-uppercase.golden`, `build-daemon-down.golden`, `build-help.golden`, `build-no-cache.golden`, `build-rejects-args.golden`, `build.golden`, `complete-agents.golden`, `complete-prompt.golden`, `config-overrides.golden`, `daemon-down.golden`, `docker-missing.golden`, `env-extra-names.golden`, `env-invalid.golden`, `env-padded-name.golden`, `help-list.golden`, `help-unknown-topic.golden`, `help.golden`, `home-unresolvable.golden`, `image-missing-builds-first.golden`, `list-rejects-args.golden`, `list.golden`, `no-mount-with-workdir.golden`, `prompt-claude.golden`, `prompt-codex.golden`, `prompt-copilot.golden`, `prompt-gemini.golden`, `prompt-gh.golden`, `prompt-pi.golden`, `run-exit-code.golden`, `run-mounts-denied.golden`, `shell-env.golden`, `shell-no-mount-no-config.golden`, `shell.golden`, `too-many-args.golden`, `unknown-flag.golden`, `unsupported-agent.golden`, `verbose-claude.golden`, `version.golden`, `workdir-flag.golden`, `workdir-home-allowed.golden`, `workdir-is-home.golden`, `workdir-missing.golden`, `workdir-relative.golden`, `workdir-root-allowed.golden`, `workdir-root.golden`
- `dev/` (1 files): `Dockerfile`
- `docs/` (3 files): `architecture.md`, `implementation.md`, `spec.md`
- `internal/agent/` (12 files): `agent.go`, `agent_test.go`, `builtin.go`, `claude.go`, `codex.go`, `copilot.go`, `definitions_test.go`, `gemini.go`, `gh.go`, `pi.go`, `registry.go`, `registry_test.go`
- `internal/agent/testdata/` (1 files): `builtin_agents.golden`
- `internal/app/` (4 files): `planner.go`, `planner_test.go`, `service.go`, `service_test.go`
- `internal/cli/` (3 files): `doc.go`, `root.go`, `root_test.go`
- `internal/docker/` (6 files): `args.go`, `args_test.go`, `client.go`, `client_test.go`, `doc.go`, `platform.go`
- `internal/hostconfig/` (4 files): `resolver.go`, `resolver_edges_test.go`, `resolver_test.go`, `tree.go`
- `internal/plan/` (2 files): `run.go`, `run_test.go`
- `internal/project/` (2 files): `mount.go`, `mount_test.go`
- `internal/runtime/` (5 files): `Dockerfile`, `assets.go`, `caged-entrypoint.sh`, `image.go`, `runtime_test.go`
- `internal/system/` (2 files): `system.go`, `system_test.go`


# 4. Appendix A: full final file contents

This appendix holds the complete final text of every production file and every changed document. You can copy these instead of applying the patches. The final text of every test file is in section 5.4. The golden fixtures are created once (Step 1 for `linux/`, Step 11 for `darwin/`) and never change afterwards, so their text is in those steps. Section 5.3 has checksums to confirm them.

## A.1 `cmd/caged/main.go`

````go
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
````

## A.2 `internal/agent/agent.go`

````go
// Package agent defines the supported agent CLIs as data and keeps them in a Registry.
package agent

import (
	"fmt"
	"strings"
)

// ConfigPath is a host config path, relative to the home directory, that is
// mounted at the same relative path under the container home.
type ConfigPath struct {
	HostPath      string
	ContainerPath string
	Writable      bool
	EnvOverride   string
	MaskPaths     []string
}

// PromptStyle says how an agent accepts an initial prompt on its command line.
type PromptStyle int

const (
	PromptNone PromptStyle = iota
	PromptPositional
	PromptFlag
)

// Agent is the complete, data-only definition of a supported agent CLI.
type Agent struct {
	Name        string
	Command     []string
	Description string
	ConfigPaths []ConfigPath
	Prompt      PromptStyle
	PromptFlag  string
	Auth        AuthStrategy
}

// AuthStrategy describes how an agent can authenticate in the container.
// Keychain extraction is intentionally unsupported; host-provided tokens and
// mounted credential files are the v1 mechanisms.
type AuthStrategy struct {
	Environment         []string
	CredentialPaths     []string
	SupportsDeviceLogin bool
}

// LaunchCommand returns the container command that starts the agent. A
// non-nil prompt is passed as exactly one argument in the agent's prompt style.
func (a Agent) LaunchCommand(prompt *string) ([]string, error) {
	command := append([]string(nil), a.Command...)
	if prompt == nil {
		return command, nil
	}
	switch a.Prompt {
	case PromptPositional:
		return append(command, *prompt), nil
	case PromptFlag:
		if a.PromptFlag == "" {
			return nil, fmt.Errorf("%s prompt flag is not configured", a.Name)
		}
		return append(command, a.PromptFlag, *prompt), nil
	default:
		return nil, fmt.Errorf("%s does not support prompt launch", a.Name)
	}
}

// ValidateEnvName rejects names that are not valid environment variable names.
func ValidateEnvName(name string) error {
	if !validEnvName(strings.TrimSpace(name)) {
		return fmt.Errorf("invalid env name %q; must match ^[A-Za-z_][A-Za-z0-9_]*$", name)
	}
	return nil
}

// validEnvName reports whether name matches ^[A-Za-z_][A-Za-z0-9_]*$.
func validEnvName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case r == '_', 'A' <= r && r <= 'Z', 'a' <= r && r <= 'z':
		case i > 0 && '0' <= r && r <= '9':
		default:
			return false
		}
	}
	return true
}

// ResolveEnv returns the names from EnvNames that isSet reports as present in
// the host environment, or nil when there are none.
func (a Agent) ResolveEnv(requested []string, isSet func(string) bool) []string {
	allowed := make([]string, 0, len(a.Auth.Environment)+len(requested))
	for _, name := range a.EnvNames(requested) {
		if isSet(name) {
			allowed = append(allowed, name)
		}
	}
	if len(allowed) == 0 {
		return nil
	}
	return allowed
}

// EnvNames returns the agent's default environment names followed by the
// requested ones, trimmed, de-duplicated, and with invalid names dropped.
func (a Agent) EnvNames(requested []string) []string {
	seen := make(map[string]struct{}, len(a.Auth.Environment)+len(requested))
	names := make([]string, 0, len(a.Auth.Environment)+len(requested))
	for _, name := range append(append([]string{}, a.Auth.Environment...), requested...) {
		name = strings.TrimSpace(name)
		if !validEnvName(name) {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return names
}

// clone returns a deep copy so callers can never modify a registered definition.
func (a Agent) clone() Agent {
	a.Command = append([]string(nil), a.Command...)
	a.ConfigPaths = append([]ConfigPath(nil), a.ConfigPaths...)
	for i := range a.ConfigPaths {
		a.ConfigPaths[i].MaskPaths = append([]string(nil), a.ConfigPaths[i].MaskPaths...)
	}
	a.Auth.Environment = append([]string(nil), a.Auth.Environment...)
	a.Auth.CredentialPaths = append([]string(nil), a.Auth.CredentialPaths...)
	return a
}
````

## A.3 `internal/agent/builtin.go`

````go
package agent

// Builtin returns the agents that ship with caged. To add an agent, create a
// file in this package with a function that returns its Agent value, then add
// one line calling that function to the list below.
func Builtin() *Registry {
	registry, err := NewRegistry(
		claude(),
		codex(),
		copilot(),
		gemini(),
		gh(),
		pi(),
	)
	if err != nil {
		// The built-in definitions are fixed at compile time, and
		// TestBuiltinRegistryContract fails before a broken one can ship.
		panic(err)
	}
	return registry
}
````

## A.4 `internal/agent/claude.go`

````go
package agent

// claude is Anthropic's Claude Code CLI.
func claude() Agent {
	return Agent{
		Name:        "claude",
		Command:     []string{"claude"},
		Description: "Claude Code",
		ConfigPaths: []ConfigPath{
			{HostPath: ".claude", ContainerPath: ".claude", Writable: true, EnvOverride: "CLAUDE_CONFIG_DIR"},
		},
		Prompt: PromptPositional,
		Auth:   AuthStrategy{Environment: []string{"ANTHROPIC_API_KEY"}, CredentialPaths: []string{".claude/.credentials.json"}},
	}
}
````

## A.5 `internal/agent/codex.go`

````go
package agent

// codex is OpenAI's Codex CLI. Its home is mounted as one directory so atomic
// config writes work; the masked paths hide the host's daemon sockets and
// runtime binaries from the container.
func codex() Agent {
	return Agent{
		Name:        "codex",
		Command:     []string{"codex"},
		Description: "Codex CLI",
		ConfigPaths: []ConfigPath{
			{
				HostPath:      ".codex",
				ContainerPath: ".codex",
				Writable:      true,
				EnvOverride:   "CODEX_HOME",
				MaskPaths:     []string{"app-server-control", "app-server-daemon", "ipc", "packages/app-server-daemon", "tmp/arg0"},
			},
		},
		Prompt:     PromptFlag,
		PromptFlag: "exec",
		Auth:       AuthStrategy{Environment: []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY"}, CredentialPaths: []string{".codex/auth.json"}, SupportsDeviceLogin: true},
	}
}
````

## A.6 `internal/agent/copilot.go`

````go
package agent

// copilot is the GitHub Copilot CLI.
func copilot() Agent {
	return Agent{
		Name:        "copilot",
		Command:     []string{"copilot"},
		Description: "GitHub Copilot CLI",
		ConfigPaths: []ConfigPath{
			{HostPath: ".copilot", ContainerPath: ".copilot", Writable: true, EnvOverride: "COPILOT_HOME"},
		},
		Prompt:     PromptFlag,
		PromptFlag: "-p",
		Auth:       AuthStrategy{Environment: []string{"COPILOT_GITHUB_TOKEN", "GH_TOKEN", "GITHUB_TOKEN"}, CredentialPaths: []string{".copilot/"}, SupportsDeviceLogin: true},
	}
}
````

## A.7 `internal/agent/gemini.go`

````go
package agent

// gemini is Google's Gemini CLI.
func gemini() Agent {
	return Agent{
		Name:        "gemini",
		Command:     []string{"gemini"},
		Description: "Gemini CLI",
		ConfigPaths: []ConfigPath{
			{HostPath: ".gemini", ContainerPath: ".gemini", Writable: true},
		},
		Prompt:     PromptFlag,
		PromptFlag: "-i",
		Auth:       AuthStrategy{Environment: []string{"GEMINI_API_KEY", "GOOGLE_API_KEY"}, CredentialPaths: []string{".gemini/oauth_creds.json"}},
	}
}
````

## A.8 `internal/agent/gh.go`

````go
package agent

// gh is the GitHub CLI. It takes no initial prompt.
func gh() Agent {
	return Agent{
		Name:        "gh",
		Command:     []string{"gh"},
		Description: "GitHub CLI",
		ConfigPaths: []ConfigPath{
			{HostPath: ".config/gh", ContainerPath: ".config/gh", Writable: true},
		},
		Auth: AuthStrategy{Environment: []string{"GH_TOKEN", "GITHUB_TOKEN"}, CredentialPaths: []string{".config/gh/hosts.yml"}},
	}
}
````

## A.9 `internal/agent/pi.go`

````go
package agent

// pi is the Pi coding agent.
func pi() Agent {
	return Agent{
		Name:        "pi",
		Command:     []string{"pi"},
		Description: "Pi coding agent",
		ConfigPaths: []ConfigPath{
			{HostPath: ".config/pi", ContainerPath: ".config/pi", Writable: true},
			{HostPath: ".pi", ContainerPath: ".pi", Writable: true},
		},
		Prompt: PromptPositional,
		Auth:   AuthStrategy{Environment: []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY"}},
	}
}
````

## A.10 `internal/agent/registry.go`

````go
package agent

import (
	"fmt"
	"sort"
	"strings"
)

// Registry is an immutable set of agents that can be looked up by name.
type Registry struct {
	byName map[string]Agent
	names  []string
}

// NewRegistry builds a registry. Every agent needs a command and a unique,
// lowercase name without surrounding spaces, because Lookup normalizes user
// input the same way.
func NewRegistry(agents ...Agent) (*Registry, error) {
	registry := &Registry{byName: make(map[string]Agent, len(agents))}
	for _, definition := range agents {
		if definition.Name == "" || definition.Name != strings.TrimSpace(strings.ToLower(definition.Name)) {
			return nil, fmt.Errorf("agent name %q must be lowercase, non-empty, and without surrounding spaces", definition.Name)
		}
		if len(definition.Command) == 0 {
			return nil, fmt.Errorf("agent %q has no command", definition.Name)
		}
		if _, exists := registry.byName[definition.Name]; exists {
			return nil, fmt.Errorf("agent %q is registered more than once", definition.Name)
		}
		registry.byName[definition.Name] = definition.clone()
		registry.names = append(registry.names, definition.Name)
	}
	sort.Strings(registry.names)
	return registry, nil
}

// Lookup finds an agent by name, ignoring case and surrounding spaces.
func (r *Registry) Lookup(name string) (Agent, error) {
	key := strings.TrimSpace(strings.ToLower(name))
	if definition, ok := r.byName[key]; ok {
		return definition.clone(), nil
	}
	return Agent{}, fmt.Errorf("unsupported agent %q; supported agents: %s", name, strings.Join(r.names, ", "))
}

// Names returns the sorted agent names.
func (r *Registry) Names() []string {
	return append([]string(nil), r.names...)
}

// All returns every agent, sorted by name.
func (r *Registry) All() []Agent {
	agents := make([]Agent, 0, len(r.names))
	for _, name := range r.names {
		agents = append(agents, r.byName[name].clone())
	}
	return agents
}
````

## A.11 `internal/app/planner.go`

````go
package app

import (
	"fmt"
	"io"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/runtime"
)

// Env is the process environment the planner reads. system.Env implements it.
type Env interface {
	LookupEnv(key string) (string, bool)
	UserHomeDir() (string, error)
	Getwd() (string, error)
}

// ProjectResolver validates the project directory and returns its bind
// mount. project.Resolver implements it.
type ProjectResolver interface {
	Resolve(workDir, home string, allowBroadMount bool) (*plan.Mount, error)
}

// ConfigResolver finds the host config that can be mounted.
// hostconfig.Resolver implements it.
type ConfigResolver interface {
	Resolve(home string) plan.ConfigReport
}

// Planner turns a Request into a complete plan.RunSpec.
type Planner struct {
	env     Env
	project ProjectResolver
	configs ConfigResolver
	stderr  io.Writer
}

// NewPlanner returns a Planner that writes warnings and --verbose details to stderr.
func NewPlanner(env Env, project ProjectResolver, configs ConfigResolver, stderr io.Writer) *Planner {
	return &Planner{env: env, project: project, configs: configs, stderr: stderr}
}

// Plan resolves the project mount, the config mounts, and the environment
// names for request, and combines them into a RunSpec.
func (p *Planner) Plan(request Request) (plan.RunSpec, error) {
	if !request.NoConfig {
		fmt.Fprintln(p.stderr, "WARNING: allowlisted agent credentials are available inside the container.")
	}
	mount, err := p.projectMount(request)
	if err != nil {
		return plan.RunSpec{}, err
	}
	configMounts, err := p.configMounts(request)
	if err != nil {
		return plan.RunSpec{}, err
	}
	var selected agent.Agent
	if request.Agent != nil {
		selected = *request.Agent
	}
	envVars := selected.ResolveEnv(request.EnvVars, p.isSet)
	if request.Verbose {
		p.logEnvResolution(selected, request.EnvVars, envVars)
	}
	return plan.BuildRunSpec(runtime.Image, request.Agent, request.Prompt, mount, configMounts, envVars)
}

// projectMount returns the project bind mount, or nil with --no-mount.
func (p *Planner) projectMount(request Request) (*plan.Mount, error) {
	if request.NoMount && request.WorkDir != "" {
		return nil, fmt.Errorf("--workdir cannot be used with --no-mount")
	}
	if request.NoMount {
		return nil, nil
	}
	workDir := request.WorkDir
	if workDir == "" {
		var err error
		workDir, err = p.env.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get current working directory: %w", err)
		}
	}
	home, err := p.env.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("find home directory: %w", err)
	}
	return p.project.Resolve(workDir, home, request.AllowBroadMount)
}

// configMounts returns the host config mounts, or none with --no-config.
func (p *Planner) configMounts(request Request) ([]plan.ConfigMount, error) {
	if request.NoConfig {
		return nil, nil
	}
	home, err := p.env.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("find home directory: %w", err)
	}
	report := p.configs.Resolve(home)
	if request.Verbose {
		p.logConfigReport(report)
	}
	return report.Mounts, nil
}

func (p *Planner) isSet(name string) bool {
	_, ok := p.env.LookupEnv(name)
	return ok
}

func (p *Planner) logConfigReport(report plan.ConfigReport) {
	for _, mount := range report.Mounts {
		fmt.Fprintf(p.stderr, "config: pass %s -> %s\n", mount.HostPath, mount.DestPath)
	}
	for _, path := range report.Missing {
		fmt.Fprintf(p.stderr, "config: missing %s\n", path)
	}
	for _, path := range report.Refused {
		fmt.Fprintf(p.stderr, "config: refused %s\n", path)
	}
}

func (p *Planner) logEnvResolution(selected agent.Agent, requested, resolved []string) {
	passed := make(map[string]struct{}, len(resolved))
	for _, name := range resolved {
		passed[name] = struct{}{}
	}
	for _, name := range selected.EnvNames(requested) {
		if _, ok := passed[name]; ok {
			fmt.Fprintf(p.stderr, "env: pass %s\n", name)
		} else {
			fmt.Fprintf(p.stderr, "env: skipped %s (unset or disallowed)\n", name)
		}
	}
}
````

## A.12 `internal/app/service.go`

````go
// Package app holds caged's use cases: run an agent or shell in a container,
// and build the runtime image.
package app

import (
	"context"
	"fmt"
	"io"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/runtime"
)

// Engine is the container runtime caged drives. docker.Client implements it.
type Engine interface {
	ImageExists(ctx context.Context, image string) bool
	Build(ctx context.Context, spec plan.BuildSpec) error
	Run(ctx context.Context, spec plan.RunSpec) error
}

// Request is a parsed command line: what to run and how to set it up.
type Request struct {
	Agent                                       *agent.Agent
	Prompt                                      *string
	NoMount, NoConfig, Verbose, AllowBroadMount bool
	WorkDir                                     string
	EnvVars                                     []string
}

// Service runs caged's use cases.
type Service struct {
	engine  Engine
	planner *Planner
	stderr  io.Writer
}

// New returns a Service that plans runs with planner, executes them on
// engine, and reports progress on stderr.
func New(engine Engine, planner *Planner, stderr io.Writer) *Service {
	return &Service{engine: engine, planner: planner, stderr: stderr}
}

// Build refreshes the runtime image.
func (s *Service) Build(ctx context.Context, noCache bool) error {
	return s.engine.Build(ctx, runtime.BuildSpec(noCache))
}

// Run builds the runtime image if it is missing, plans the container, and
// runs it.
func (s *Service) Run(ctx context.Context, request Request) error {
	if !s.engine.ImageExists(ctx, runtime.Image) {
		fmt.Fprintf(s.stderr, "Runtime image %s not found; building it first...\n", runtime.Image)
		if err := s.engine.Build(ctx, runtime.BuildSpec(false)); err != nil {
			return err
		}
	}
	spec, err := s.planner.Plan(request)
	if err != nil {
		return err
	}
	return s.engine.Run(ctx, spec)
}
````

## A.13 `internal/cli/doc.go`

````go
// Package cli turns caged's command line into an app.Request. It parses
// arguments and flags with Cobra and leaves all work to injected services.
package cli
````

## A.14 `internal/cli/root.go`

````go
package cli

import (
	"context"
	"fmt"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/app"
	"github.com/JeremyOrr4/caged/internal/runtime"
	"github.com/spf13/cobra"
)

// Service runs and builds caged containers. app.Service implements it.
type Service interface {
	Run(ctx context.Context, request app.Request) error
	Build(ctx context.Context, noCache bool) error
}

// Agents finds supported agents. agent.Registry implements it.
type Agents interface {
	Lookup(name string) (agent.Agent, error)
	Names() []string
}

// NewRootCmd creates the root caged command. It only turns arguments and
// flags into an app.Request; agents and service do the work.
func NewRootCmd(version string, agents Agents, service Service) *cobra.Command {
	var flags app.Request

	cmd := &cobra.Command{
		Use:           "caged [agent [prompt]]",
		Short:         "Run AI coding agents in a disposable Docker container",
		Long:          "Run a bash shell or a supported AI coding agent in a disposable Docker container",
		Args:          cobra.MaximumNArgs(2),
		SilenceUsage:  true,
		SilenceErrors: true,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return agents.Names(), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			request, err := buildRequest(agents, flags, args)
			if err != nil {
				return err
			}
			return service.Run(cmd.Context(), request)
		},
	}

	cmd.AddCommand(newBuildCmd(service))
	cmd.AddCommand(newListCmd(agents))
	cmd.Flags().BoolVar(&flags.NoMount, "no-mount", false, "do not mount the current working directory")
	cmd.Flags().StringVarP(&flags.WorkDir, "workdir", "w", "", "mount this host directory at /workspace/<folder-name>")
	cmd.Flags().BoolVar(&flags.AllowBroadMount, "allow-broad-mount", false, "allow mounting the filesystem root or home directory")
	cmd.Flags().BoolVar(&flags.NoConfig, "no-config", false, "do not mount any host AI config into the container")
	cmd.Flags().BoolVar(&flags.Verbose, "verbose", false, "print which config files and environment names are passed")
	cmd.Flags().StringArrayVar(&flags.EnvVars, "env", nil, "pass a host environment variable into the container by NAME only")
	cmd.Version = version
	return cmd
}

// buildRequest adds the positional arguments to the parsed flags. No
// argument means a shell, the first names an agent, and the second is its
// prompt. Environment names are validated here, before anything runs.
func buildRequest(agents Agents, flags app.Request, args []string) (app.Request, error) {
	request := flags
	if len(args) > 0 {
		selected, err := agents.Lookup(args[0])
		if err != nil {
			return app.Request{}, err
		}
		request.Agent = &selected
	}
	for _, name := range request.EnvVars {
		if err := agent.ValidateEnvName(name); err != nil {
			return app.Request{}, err
		}
	}
	if len(args) == 2 {
		prompt := args[1]
		request.Prompt = &prompt
	}
	return request, nil
}

func newBuildCmd(service Service) *cobra.Command {
	var noCache bool
	cmd := &cobra.Command{
		Use:          "build",
		Short:        "Build the runtime image",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := service.Build(cmd.Context(), noCache); err != nil {
				return err
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "built runtime image", runtime.Image)
			return err
		},
	}
	cmd.Flags().BoolVar(&noCache, "no-cache", false, "build without using the Docker cache")
	return cmd
}

func newListCmd(agents Agents) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List supported AI agents",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, name := range agents.Names() {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), name); err != nil {
					return err
				}
			}
			return nil
		},
	}
	return cmd
}
````

## A.15 `internal/docker/args.go`

````go
package docker

import (
	"path/filepath"
	"strings"

	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
)

// runArgs translates a run plan into `docker run` arguments.
func runArgs(spec plan.RunSpec, platform system.Platform) []string {
	args := []string{"run", "-it", "--rm"}

	if spec.Mount != nil {
		args = append(args,
			"-v", spec.Mount.HostPath+":"+spec.Mount.ContainerPath,
			"-w", spec.Mount.ContainerPath,
		)
	}
	for _, mount := range spec.ConfigMounts {
		volume := mount.HostPath + ":" + mount.DestPath
		if !mount.Writable {
			volume += ":ro"
		}
		args = append(args, "-v", volume)
		for _, mask := range mount.MaskPaths {
			maskedPath := strings.TrimSuffix(mount.DestPath, "/") + "/" + strings.TrimPrefix(filepath.ToSlash(mask), "/")
			args = append(args, "--tmpfs", maskedPath)
		}
	}
	args = append(args, userArgs(platform)...)
	for _, env := range spec.EnvVars {
		args = append(args, "-e", env)
	}
	if spec.ReturnToShell {
		args = append(args, "-e", "CAGED_RETURN_TO_SHELL=1")
	}

	args = append(args, spec.Image)
	return append(args, spec.Command...)
}

// buildArgs translates a build plan into `docker build` arguments for the
// build context written to dir.
func buildArgs(spec plan.BuildSpec, dir string) []string {
	args := []string{"build", "-t", spec.Image}
	if spec.NoCache {
		args = append(args, "--no-cache")
	}
	return append(args, "-f", filepath.Join(dir, spec.Dockerfile), dir)
}
````

## A.16 `internal/docker/client.go`

````go
package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
)

// Executor runs host processes. system.Executor is the production implementation.
type Executor interface {
	LookPath(file string) (string, error)
	Run(ctx context.Context, cmd system.Command) error
}

// FileSystem holds the temporary build context. system.FS is the production implementation.
type FileSystem interface {
	MkdirTemp(dir, pattern string) (string, error)
	WriteFile(name string, data []byte, perm fs.FileMode) error
	RemoveAll(path string) error
}

// Client drives the docker CLI.
type Client struct {
	exec     Executor
	fs       FileSystem
	platform system.Platform
	stdio    system.Stdio
}

// NewClient returns a Client that runs docker through exec, writes build
// contexts through fsys, applies the rules for platform, and connects
// containers to stdio.
func NewClient(exec Executor, fsys FileSystem, platform system.Platform, stdio system.Stdio) *Client {
	return &Client{exec: exec, fs: fsys, platform: platform, stdio: stdio}
}

// Check distinguishes a missing docker CLI from a stopped Docker daemon.
func (c *Client) Check(ctx context.Context) error {
	if err := checkArch(c.platform); err != nil {
		return err
	}
	docker, err := c.exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("Docker is not installed or not on PATH: %w", err)
	}
	var output bytes.Buffer
	info := system.Command{Path: docker, Args: []string{"info", "--format", "{{.ServerVersion}}"}, Stdout: &output, Stderr: &output}
	if err := c.exec.Run(ctx, info); err != nil {
		return fmt.Errorf("Docker is installed but the daemon is unavailable; %s: %s", daemonLauncher(c.platform), strings.TrimSpace(output.String()))
	}
	return nil
}

// ImageExists reports whether image is in the local image store.
func (c *Client) ImageExists(ctx context.Context, image string) bool {
	return c.exec.Run(ctx, system.Command{Path: "docker", Args: []string{"image", "inspect", image}}) == nil
}

// Build writes the build context to a temporary directory, runs docker build,
// and removes the directory again.
func (c *Client) Build(ctx context.Context, spec plan.BuildSpec) error {
	if err := c.Check(ctx); err != nil {
		return err
	}
	docker, err := c.exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("Docker is not installed or not on PATH: %w", err)
	}
	dir, err := c.fs.MkdirTemp("", "caged-runtime-")
	if err != nil {
		return fmt.Errorf("create temporary build directory: %w", err)
	}
	defer c.fs.RemoveAll(dir)
	for _, file := range spec.Files {
		if err := c.fs.WriteFile(filepath.Join(dir, file.Name), file.Data, file.Mode); err != nil {
			return fmt.Errorf("write embedded %s: %w", file.Label, err)
		}
	}
	return c.exec.Run(ctx, system.Command{
		Path:   docker,
		Args:   buildArgs(spec, dir),
		Stdin:  c.stdio.In,
		Stdout: c.stdio.Out,
		Stderr: c.stdio.Err,
	})
}

// Run starts an interactive, disposable container and waits for it to exit.
// A non-zero container exit is returned unchanged so its code can be passed on.
func (c *Client) Run(ctx context.Context, spec plan.RunSpec) error {
	if err := c.Check(ctx); err != nil {
		return err
	}
	docker, err := c.exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("docker not found, is Docker installed and running?: %w", err)
	}
	var stderr bytes.Buffer
	err = c.exec.Run(ctx, system.Command{
		Path:   docker,
		Args:   runArgs(spec, c.platform),
		Stdin:  c.stdio.In,
		Stdout: c.stdio.Out,
		Stderr: io.MultiWriter(c.stdio.Err, &stderr),
	})
	if err != nil {
		return explainRunError(c.platform, stderr.String(), err)
	}
	return nil
}
````

## A.17 `internal/docker/doc.go`

````go
// Package docker is the Docker CLI adapter. It turns run and build plans into
// docker commands and executes them through an injected Executor.
package docker
````

## A.18 `internal/docker/platform.go`

````go
package docker

import (
	"fmt"
	"strings"

	"github.com/JeremyOrr4/caged/internal/system"
)

// This file holds every platform-specific Docker rule. The platform is
// injected, so the Linux and macOS branches are both tested on any host.

// checkArch rejects hosts that have no matching runtime image architecture.
func checkArch(platform system.Platform) error {
	if platform.Arch != "amd64" && platform.Arch != "arm64" {
		return fmt.Errorf("unsupported host architecture %q; caged supports amd64 and arm64 images", platform.Arch)
	}
	return nil
}

// userArgs runs the container as the host user on Linux, so files created in
// bind mounts belong to that user. Docker Desktop on macOS translates
// bind-mount ownership itself, so the image's own user is kept there.
func userArgs(platform system.Platform) []string {
	if platform.OS == "linux" {
		return []string{"--user", fmt.Sprintf("%d:%d", platform.UID, platform.GID)}
	}
	return nil
}

// daemonLauncher names what the user has to start when the daemon is down.
func daemonLauncher(platform system.Platform) string {
	if platform.OS == "darwin" {
		return "start Docker Desktop"
	}
	return "start the Docker daemon"
}

// explainRunError adds the Docker Desktop File Sharing hint on macOS when the
// project path is not shared with Docker Desktop's VM.
func explainRunError(platform system.Platform, stderr string, err error) error {
	if platform.OS == "darwin" && looksLikeSharedPathError(stderr) {
		return fmt.Errorf("Docker could not mount the project path; in Docker Desktop, add its parent directory under Settings > Resources > File Sharing: %w", err)
	}
	return err
}

func looksLikeSharedPathError(message string) bool {
	message = strings.ToLower(message)
	return strings.Contains(message, "mounts denied") || strings.Contains(message, "not shared") || strings.Contains(message, "mount denied")
}
````

## A.19 `internal/hostconfig/resolver.go`

````go
// Package hostconfig decides which host config paths are mounted into the
// container and refuses anything that is unsafe to expose.
package hostconfig

import (
	"io/fs"
	"path/filepath"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/plan"
)

// containerHome is the home directory of the user inside the runtime image.
const containerHome = "/home/caged"

// FileSystem is the read-only filesystem access the resolver needs.
type FileSystem interface {
	Stat(name string) (fs.FileInfo, error)
	Lstat(name string) (fs.FileInfo, error)
	ReadDir(name string) ([]fs.DirEntry, error)
	EvalSymlinks(path string) (string, error)
	WalkDir(root string, fn fs.WalkDirFunc) error
}

// Env looks up config-directory overrides such as CODEX_HOME.
type Env interface {
	LookupEnv(key string) (string, bool)
}

// SharedFile is a regular file in the home directory, such as .gitconfig,
// that all agents share. It is mounted read-only at the same relative path
// under the container home.
type SharedFile struct {
	Path        string // relative to the home directory
	Description string // used in refusal messages
}

// DefaultSharedFiles returns the shared files caged mounts.
func DefaultSharedFiles() []SharedFile {
	return []SharedFile{{Path: ".gitconfig", Description: "shared git config"}}
}

// Resolver finds the agent config paths and shared files that can be mounted.
type Resolver struct {
	fs     FileSystem
	env    Env
	agents []agent.Agent
	shared []SharedFile
}

// NewResolver returns a Resolver for the config paths of agents, in order,
// followed by the shared files.
func NewResolver(fsys FileSystem, env Env, agents []agent.Agent, shared []SharedFile) *Resolver {
	return &Resolver{fs: fsys, env: env, agents: agents, shared: shared}
}

// Resolve inspects every config path under home and reports what will be
// mounted, what is missing, and what is refused.
func (r *Resolver) Resolve(home string) plan.ConfigReport {
	home = filepath.Clean(home)
	resolvedHome, err := r.fs.EvalSymlinks(home)
	if err != nil {
		return plan.ConfigReport{Refused: []string{home + " (home cannot be resolved)"}}
	}
	pass := &resolution{fs: r.fs, home: home, resolvedHome: resolvedHome, seen: make(map[string]struct{})}
	for _, definition := range r.agents {
		for _, path := range definition.ConfigPaths {
			pass.agentPath(definition.Name, path, r.hostPath(home, path))
		}
	}
	for _, file := range r.shared {
		pass.sharedFile(file)
	}
	return pass.report
}

// hostPath is where a config path lives on the host. A set override variable
// (absolute, or relative to home) replaces the default location.
func (r *Resolver) hostPath(home string, path agent.ConfigPath) string {
	hostPath := filepath.Join(home, filepath.Clean(path.HostPath))
	if path.EnvOverride == "" {
		return hostPath
	}
	override, _ := r.env.LookupEnv(path.EnvOverride)
	if override == "" {
		return hostPath
	}
	if !filepath.IsAbs(override) {
		override = filepath.Join(home, override)
	}
	return filepath.Clean(override)
}

// resolution accumulates the report of one Resolve call.
type resolution struct {
	fs           FileSystem
	home         string
	resolvedHome string
	seen         map[string]struct{} // host path + "\x00" + container path already mounted
	report       plan.ConfigReport
}

// agentPath checks one agent config path and records its mounts, or why it
// is missing or refused.
func (r *resolution) agentPath(agentName string, path agent.ConfigPath, hostPath string) {
	if !r.exists(hostPath) {
		r.report.Missing = appendUnique(r.report.Missing, hostPath)
		return
	}
	resolvedHost, err := r.fs.EvalSymlinks(hostPath)
	if err != nil {
		r.refuse(hostPath + " (unresolvable symlink)")
		return
	}
	if filepath.Clean(resolvedHost) == filepath.Clean(r.resolvedHome) {
		r.refuse(hostPath + " (config path resolves to the home directory)")
		return
	}
	if !isWithin(resolvedHost, r.resolvedHome) {
		r.refuse(hostPath + " (resolves outside home)")
		return
	}
	name, dest := filepath.Clean(path.HostPath), filepath.Clean(path.ContainerPath)
	if len(path.MaskPaths) > 0 {
		// A masked tree is mounted whole; tmpfs mounts hide the masked entries.
		if err := validateMaskedConfigTree(r.fs, resolvedHost, path.MaskPaths); err != nil {
			r.refuse(hostPath + " (" + err.Error() + ")")
			return
		}
		r.addMount(agentName, name, resolvedHost, dest, path.Writable, append([]string(nil), path.MaskPaths...))
		return
	}
	mounts, refused := collectSafeMounts(r.fs, resolvedHost)
	for _, reason := range refused {
		r.refuse(reason)
	}
	for _, mountPath := range mounts {
		relative, err := filepath.Rel(resolvedHost, mountPath)
		if err != nil {
			r.refuse(hostPath + " (cannot map safe config path)")
			continue
		}
		mountDest := dest
		if relative != "." {
			mountDest = filepath.Join(dest, relative)
		}
		r.addMount(agentName, name, mountPath, mountDest, path.Writable, nil)
	}
}

// sharedFile checks one shared file. It must resolve inside home and be a
// regular file, and it is always mounted read-only.
func (r *resolution) sharedFile(file SharedFile) {
	name := filepath.Clean(file.Path)
	hostPath := filepath.Join(r.home, name)
	if !r.exists(hostPath) {
		r.report.Missing = appendUnique(r.report.Missing, hostPath)
		return
	}
	resolved, err := r.fs.EvalSymlinks(hostPath)
	if err != nil || !isWithin(resolved, r.resolvedHome) {
		r.refuse(hostPath + " (resolves outside home or cannot be resolved)")
		return
	}
	if info, err := r.fs.Stat(resolved); err != nil || !info.Mode().IsRegular() {
		r.refuse(hostPath + " (" + file.Description + " must be a regular file)")
		return
	}
	r.report.Mounts = append(r.report.Mounts, plan.ConfigMount{
		Agent:    "shared",
		Name:     name,
		HostPath: resolved,
		DestPath: filepath.ToSlash(filepath.Join(containerHome, name)),
		Writable: false,
	})
}

// addMount records a mount unless the same host path is already mounted at
// the same container path.
func (r *resolution) addMount(agentName, name, hostPath, dest string, writable bool, masks []string) {
	key := hostPath + "\x00" + dest
	if _, ok := r.seen[key]; ok {
		return
	}
	r.seen[key] = struct{}{}
	r.report.Mounts = append(r.report.Mounts, plan.ConfigMount{
		Agent:     agentName,
		Name:      name,
		HostPath:  hostPath,
		DestPath:  filepath.ToSlash(filepath.Join(containerHome, dest)),
		Writable:  writable,
		MaskPaths: masks,
	})
}

func (r *resolution) exists(path string) bool {
	_, err := r.fs.Stat(path)
	return err == nil
}

func (r *resolution) refuse(reason string) {
	r.report.Refused = appendUnique(r.report.Refused, reason)
}
````

## A.20 `internal/hostconfig/tree.go`

````go
package hostconfig

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// validateMaskedConfigTree accepts a directory that will be mounted whole. It
// fails on any symlink that leaves root and on any special file (socket, FIFO,
// device) that is not inside one of the masked paths.
func validateMaskedConfigTree(fsys FileSystem, root string, masks []string) error {
	rootInfo, err := fsys.Stat(root)
	if err != nil {
		return err
	}
	if !rootInfo.IsDir() {
		return fmt.Errorf("masked config root must be a directory")
	}
	maskPaths := make([]string, 0, len(masks))
	for _, mask := range masks {
		clean := filepath.Clean(mask)
		if filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("invalid config mask path %q", mask)
		}
		maskPaths = append(maskPaths, filepath.Join(root, clean))
	}
	return fsys.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path != root {
			for _, mask := range maskPaths {
				if path == mask || isWithin(path, mask) {
					if entry.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}
		}
		info, err := fsys.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			resolved, err := fsys.EvalSymlinks(path)
			if err != nil || !isWithin(resolved, root) {
				return fmt.Errorf("symlink %q resolves outside the config directory or cannot be resolved", path)
			}
			return nil
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("special file %q is not covered by a config mask", path)
		}
		return nil
	})
}

// collectSafeMounts returns the largest subtrees of root that contain only
// regular files and directories, plus a reason for every entry it skipped.
func collectSafeMounts(fsys FileSystem, root string) ([]string, []string) {
	var mounts, refused []string
	var collect func(string)
	collect = func(path string) {
		entries, err := fsys.ReadDir(path)
		if err != nil {
			info, statErr := fsys.Lstat(path)
			if statErr == nil && info.Mode().IsRegular() {
				mounts = append(mounts, path)
			} else {
				refused = append(refused, path+" (cannot inspect config entry safely)")
			}
			return
		}
		if subtreeIsSafe(fsys, path) {
			mounts = append(mounts, path)
			return
		}
		for _, entry := range entries {
			child := filepath.Join(path, entry.Name())
			info, err := fsys.Lstat(child)
			if err != nil {
				refused = append(refused, child+" (cannot inspect config entry safely)")
				continue
			}
			if info.IsDir() {
				collect(child)
			} else if info.Mode().IsRegular() {
				mounts = append(mounts, child)
			} else {
				refused = append(refused, child+" (symlinks and special files are excluded from config mounts)")
			}
		}
	}
	info, err := fsys.Lstat(root)
	if err != nil {
		return nil, []string{root + " (cannot inspect config root safely)"}
	}
	if info.Mode().IsRegular() {
		return []string{root}, nil
	}
	if !info.IsDir() {
		return nil, []string{root + " (config root is not a regular file or directory)"}
	}
	collect(root)
	return mounts, refused
}

// subtreeIsSafe reports whether root contains only regular files and directories.
func subtreeIsSafe(fsys FileSystem, root string) bool {
	entries, err := fsys.ReadDir(root)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		info, err := fsys.Lstat(path)
		if err != nil {
			return false
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return false
		}
		if info.IsDir() {
			if !subtreeIsSafe(fsys, path) {
				return false
			}
		} else if !info.Mode().IsRegular() {
			return false
		}
	}
	return true
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

// isWithin reports whether path is root itself or inside it.
func isWithin(path, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
````

## A.21 `internal/plan/run.go`

````go
// Package plan contains transport-neutral data for a disposable agent run.
package plan

import (
	"fmt"
	"io/fs"

	"github.com/JeremyOrr4/caged/internal/agent"
)

// Mount describes a host path exposed at a container path.
type Mount struct{ HostPath, ContainerPath string }

// ConfigMount includes ownership and access information for host configuration.
type ConfigMount struct {
	Agent, Name, HostPath, DestPath string
	Writable                        bool
	MaskPaths                       []string
}

// ConfigReport is the outcome of host config discovery: the mounts to make,
// the paths that do not exist, and the paths refused with their reasons.
type ConfigReport struct {
	Mounts  []ConfigMount
	Missing []string
	Refused []string
}

// RunSpec is an immutable-by-convention description consumed by a runtime adapter.
type RunSpec struct {
	Image         string
	Command       []string
	Mount         *Mount
	ConfigMounts  []ConfigMount
	EnvVars       []string
	ReturnToShell bool
}

// BuildSpec describes an image build from an in-memory build context.
type BuildSpec struct {
	Image      string // tag of the image to build
	Dockerfile string // name of the Dockerfile inside Files
	Files      []File // the build context, written in order
	NoCache    bool
}

// File is one file of an image build context.
type File struct {
	Name  string // file name inside the build context
	Label string // short name used in error messages
	Data  []byte
	Mode  fs.FileMode
}

// BuildRunSpec combines resolved inputs into a complete container invocation.
func BuildRunSpec(image string, selected *agent.Agent, prompt *string, projectMount *Mount, configMounts []ConfigMount, envVars []string) (RunSpec, error) {
	var mountCopy *Mount
	if projectMount != nil {
		copied := *projectMount
		mountCopy = &copied
	}
	configCopies := append([]ConfigMount(nil), configMounts...)
	for i := range configCopies {
		configCopies[i].MaskPaths = append([]string(nil), configCopies[i].MaskPaths...)
	}
	spec := RunSpec{Image: image, Command: []string{"bash"}, Mount: mountCopy, ConfigMounts: configCopies, EnvVars: append([]string(nil), envVars...)}
	if selected == nil {
		if prompt != nil {
			return RunSpec{}, fmt.Errorf("a prompt requires an agent")
		}
		return spec, nil
	}
	command, err := selected.LaunchCommand(prompt)
	if err != nil {
		return RunSpec{}, err
	}
	spec.Command = command
	spec.ReturnToShell = true
	return spec, nil
}
````

## A.22 `internal/project/mount.go`

````go
// Package project resolves and validates the host project directory bind mount.
package project

import (
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/JeremyOrr4/caged/internal/plan"
)

// FileSystem is the filesystem access the resolver needs.
type FileSystem interface {
	Abs(path string) (string, error)
	Stat(name string) (fs.FileInfo, error)
	EvalSymlinks(path string) (string, error)
}

// Resolver validates the host project directory and builds its bind mount.
type Resolver struct {
	fs FileSystem
}

// NewResolver returns a Resolver that inspects paths through fsys.
func NewResolver(fsys FileSystem) *Resolver {
	return &Resolver{fs: fsys}
}

// Resolve makes workDir absolute, checks that it is a directory, resolves
// symlinks in it and in home, and then applies the NewMount guard rails.
func (r *Resolver) Resolve(workDir, home string, allowBroadMount bool) (*plan.Mount, error) {
	workDir, err := r.fs.Abs(workDir)
	if err != nil {
		return nil, fmt.Errorf("resolve working directory: %w", err)
	}
	info, err := r.fs.Stat(workDir)
	if err != nil {
		return nil, fmt.Errorf("inspect working directory %q: %w", workDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("working directory %q is not a directory", workDir)
	}
	workDir, err = r.fs.EvalSymlinks(workDir)
	if err != nil {
		return nil, fmt.Errorf("resolve working directory symlinks: %w", err)
	}
	home, err = r.fs.EvalSymlinks(home)
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}
	return NewMount(workDir, home, allowBroadMount)
}

// NewMount refuses the filesystem root and the home directory unless
// allowBroadMount is set, and mounts cwd at /workspace/<folder-name>.
func NewMount(cwd, home string, allowBroadMount bool) (*plan.Mount, error) {
	cwd, home = filepath.Clean(cwd), filepath.Clean(home)
	if !allowBroadMount && cwd == string(filepath.Separator) {
		return nil, fmt.Errorf("refusing to mount the filesystem root; use --allow-broad-mount to override")
	}
	if !allowBroadMount && cwd == home {
		return nil, fmt.Errorf("refusing to mount your entire home directory; use --allow-broad-mount to override")
	}
	name := filepath.Base(cwd)
	if name == "." || name == string(filepath.Separator) || name == "" {
		name = "project"
	}
	return &plan.Mount{HostPath: cwd, ContainerPath: filepath.ToSlash(filepath.Join("/workspace", name))}, nil
}
````

## A.23 `internal/runtime/assets.go`

````go
package runtime

import (
	_ "embed"

	"github.com/JeremyOrr4/caged/internal/plan"
)

//go:embed Dockerfile
var runtimeDockerfile string

//go:embed caged-entrypoint.sh
var runtimeEntrypoint string

// BuildSpec describes how to build the runtime image from the embedded
// Dockerfile and entrypoint.
func BuildSpec(noCache bool) plan.BuildSpec {
	return plan.BuildSpec{
		Image:      Image,
		Dockerfile: "Dockerfile",
		NoCache:    noCache,
		Files: []plan.File{
			{Name: "Dockerfile", Label: "Dockerfile", Data: []byte(runtimeDockerfile), Mode: 0o644},
			{Name: "caged-entrypoint.sh", Label: "entrypoint", Data: []byte(runtimeEntrypoint), Mode: 0o755},
		},
	}
}
````

## A.24 `internal/runtime/image.go`

````go
// Package runtime describes the caged runtime image: its tag and the embedded
// files it is built from. It does not run Docker itself.
package runtime

// Image is the tag of the locally built runtime image.
const Image = "caged-runtime:latest"
````

## A.25 `internal/system/system.go`

````go
// Package system holds the thin adapters between caged and the host operating
// system. It is the only internal package that imports os, os/exec, or the
// standard runtime package. Other packages declare small interfaces that these
// types satisfy, so they can be tested with fakes.
package system

import (
	"context"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// FS is the real filesystem.
type FS struct{}

func (FS) Abs(path string) (string, error)              { return filepath.Abs(path) }
func (FS) Stat(name string) (fs.FileInfo, error)        { return os.Stat(name) }
func (FS) Lstat(name string) (fs.FileInfo, error)       { return os.Lstat(name) }
func (FS) ReadDir(name string) ([]fs.DirEntry, error)   { return os.ReadDir(name) }
func (FS) EvalSymlinks(path string) (string, error)     { return filepath.EvalSymlinks(path) }
func (FS) WalkDir(root string, fn fs.WalkDirFunc) error { return filepath.WalkDir(root, fn) }
func (FS) MkdirTemp(dir, pattern string) (string, error) {
	return os.MkdirTemp(dir, pattern)
}
func (FS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(name, data, perm)
}
func (FS) RemoveAll(path string) error { return os.RemoveAll(path) }

// Env is the real process environment.
type Env struct{}

func (Env) LookupEnv(key string) (string, bool) { return os.LookupEnv(key) }
func (Env) UserHomeDir() (string, error)        { return os.UserHomeDir() }
func (Env) Getwd() (string, error)              { return os.Getwd() }

// Command describes one process to run. Nil streams are connected to the null device.
type Command struct {
	Path   string
	Args   []string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Executor runs real processes.
type Executor struct{}

func (Executor) LookPath(file string) (string, error) { return exec.LookPath(file) }

// Run starts the command and waits for it. A non-zero exit status is returned
// as an *exec.ExitError so callers can pass the exit code through.
func (Executor) Run(ctx context.Context, c Command) error {
	cmd := exec.CommandContext(ctx, c.Path, c.Args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = c.Stdin, c.Stdout, c.Stderr
	return cmd.Run()
}

// Platform describes the host operating system, CPU architecture, and user.
type Platform struct {
	OS   string // a GOOS value such as "linux" or "darwin"
	Arch string // a GOARCH value such as "amd64" or "arm64"
	UID  int
	GID  int
}

// CurrentPlatform reports the platform of the running process.
func CurrentPlatform() Platform {
	return Platform{OS: runtime.GOOS, Arch: runtime.GOARCH, UID: os.Getuid(), GID: os.Getgid()}
}

// Stdio holds the standard streams of the process.
type Stdio struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
}

// CurrentStdio returns the real standard streams. They stay *os.File values so
// child processes inherit the terminal directly, which `docker run -it` needs.
func CurrentStdio() Stdio {
	return Stdio{In: os.Stdin, Out: os.Stdout, Err: os.Stderr}
}
````

## A.26 `README.md`

````markdown
# caged

`caged` runs Claude Code, Codex, Gemini CLI, Pi, or GitHub Copilot CLI in a disposable Docker
container. The GitHub CLI (`gh`) is also available inside its shell. It mounts the directory you started in at
`/workspace/<folder-name>` and uses one image containing every supported CLI.
Docker Desktop is the supported runtime on macOS; Linux is best effort.

## Usage

```sh
caged                         # interactive shell with all CLIs available
caged claude                  # launch an agent, then return to the container shell
caged codex 'fix the tests'   # launch with an initial prompt
caged copilot 'explain this code'  # launch GitHub Copilot CLI with a prompt
caged gh                           # run gh, then return to the container shell
caged build [--no-cache]      # build or refresh the runtime image
caged list                    # list supported agents
```

The initial prompt is passed as one argument according to each agent's launch
style. The prompt is not interpreted by a shell. After an agent exits, caged
opens a shell in the same container; `exit` removes it.

Existing `~/.claude`, `~/.codex`, `~/.gemini`, `~/.pi`,
`~/.copilot`, `~/.config/pi`, and `~/.config/gh` paths are mounted read-write. Changes,
including credentials created by logging in inside the container, persist on
the host. The shared `~/.gitconfig` is mounted read-only. On macOS, Claude
Code's Keychain login is not forwarded; log in once
inside caged or pass an API key. Credential files in mounted config folders are
available to the container. SSH keys, cloud credential directories, and the
Docker socket are never mounted. Use `--no-config` to disable all config mounts.
The container login credential is stored as a plaintext file in the mounted
config directory on the host.
Copilot CLI can authenticate with `copilot login --device-code`; its config
directory is mounted so the login can persist between caged runs. It can also
use `COPILOT_GITHUB_TOKEN`, `GH_TOKEN`, or `GITHUB_TOKEN` passed with `--env`.
For GitHub CLI authentication, create `~/.config/gh` on the host before the
first run if it does not exist, then run `gh auth login` inside caged. Its
configuration and login persist in that mounted directory. `GH_TOKEN` and
`GITHUB_TOKEN` are also forwarded when set on the host.
Claude Code uses `/home/caged/.claude` as its config directory inside the
container. This lets its configuration and credentials persist through the
directory mount; the host's `~/.claude.json` is intentionally not shared.

Environment variables are forwarded by name only, and only when set on the
host. Use repeatable `--env NAME` for additional names. Values are never shown
by caged.

```sh
caged claude --env ANTHROPIC_API_KEY
caged --workdir ~/code/my-project pi
caged --no-mount
```

For safety, mounting `/` or your entire home directory requires
`--allow-broad-mount`. Ports are not published, so services inside the
container are not reachable through the host's `localhost`. macOS bind mounts
can be slower for large dependency trees.

Install with `go install github.com/JeremyOrr4/caged/cmd/caged@latest` or build
from a checkout with `make build`. The runtime image is built locally on first
launch. `caged build --no-cache` refreshes the installed CLI versions.

## Development

```sh
make build
make test
make vet
```

The development image uses [dev/Dockerfile](dev/Dockerfile); the embedded
runtime image is defined in `internal/runtime/Dockerfile`. The package layout,
the dependency rules, and how to add an agent are described in
[docs/architecture.md](docs/architecture.md).
````

## A.27 `docs/architecture.md`

````markdown
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
````

## A.28 `docs/implementation.md`

````markdown
# Implementation notes

The implementation provides one disposable Docker runtime for Claude Code,
Codex, Gemini CLI, Pi, GitHub CLI (`gh`), GitHub Copilot CLI, and an interactive shell. Its image is embedded into the
Go executable and built locally on first launch.

## User behavior

- The project directory is mounted read-write at `/workspace/<folder-name>`.
- `caged <agent> '<prompt>'` passes the prompt as one argument using the
  registered launch style.
- After an agent exits, the container entrypoint opens an interactive shell.
- Existing allowlisted CLI config paths are mounted read-write; shared
  `.gitconfig` is read-only. Absent paths are skipped. Config roots outside the home
  directory and unsafe symlinks are refused. Special files are excluded or
  masked so they are never passed into the container.
- Codex's home is one directory mount so config updates can use atomic file
  replacement. Container tmpfs mounts hide its host daemon, socket, and runtime
  executable symlink directories.
- GitHub CLI uses the host's `~/.config/gh` directory and accepts `GH_TOKEN` or
  `GITHUB_TOKEN` when those variables are set. Create the directory before the
  first run if login needs to persist there.
- `--no-config` disables those mounts. There is no `--no-auth`; credentials in
  an existing mounted config are part of the read-write config behavior.
- Host environment values are passed by name only when set. Caged logs names,
  never values.
- Linux runs with the host UID/GID. macOS uses the image user and expects
  Docker Desktop bind-mount ownership translation.

## Development

```sh
make build
make test
make vet
```

The development image is `dev/Dockerfile`; the runtime Dockerfile and entrypoint
under `internal/runtime/` are embedded in the binary. Command-line parsing,
agent data, host-config resolution, project validation, run planning, Docker
execution, and application coordination are separated into `internal/cli`,
`internal/agent`, `internal/hostconfig`, `internal/project`, `internal/plan`,
`internal/docker`, and `internal/app`. Only `internal/system` touches the
operating system, and `cmd/caged/main.go` wires everything together. See
`docs/architecture.md` for the interfaces and the dependency rules.
````

# 5. Tests

## 5.1 Commands and expected results

Run every command from the repository root, after Step 14.

**Unit tests:**

````bash
go test -count=1 ./...
````

Expected output (the times will differ):

````text
ok  	github.com/JeremyOrr4/caged/cmd/caged	1.013s
ok  	github.com/JeremyOrr4/caged/internal/agent	0.009s
ok  	github.com/JeremyOrr4/caged/internal/app	0.008s
ok  	github.com/JeremyOrr4/caged/internal/cli	0.014s
ok  	github.com/JeremyOrr4/caged/internal/docker	0.006s
ok  	github.com/JeremyOrr4/caged/internal/hostconfig	0.011s
ok  	github.com/JeremyOrr4/caged/internal/plan	0.007s
ok  	github.com/JeremyOrr4/caged/internal/project	0.004s
ok  	github.com/JeremyOrr4/caged/internal/runtime	0.003s
ok  	github.com/JeremyOrr4/caged/internal/system	0.006s
````

There are 88 top-level test functions. With subtests, `go test -count=1 -v ./... 2>&1 | grep -c -- "--- PASS"` prints **255**.

Top-level tests per package (counted with `go test -count=1 -v ./<package> 2>&1 | grep -c -- "^--- PASS"`):

| Package | Top-level tests |
|---|---|
| `cmd/caged` | 7 |
| `internal/agent` | 15 |
| `internal/app` | 11 |
| `internal/cli` | 7 |
| `internal/docker` | 11 |
| `internal/hostconfig` | 16 |
| `internal/plan` | 3 |
| `internal/project` | 5 |
| `internal/runtime` | 9 |
| `internal/system` | 4 |

**Race detector:**

````bash
go test -count=1 -race ./...
````

Expected: every package prints `ok`, and nothing says `WARNING: DATA RACE`. The race detector makes the tests slower; about one second per package is normal.

**Coverage:**

````bash
go test -count=1 -cover ./...
go test -count=1 -coverprofile=cover.out ./... && go tool cover -func=cover.out | tail -1
rm cover.out
````

Coverage per package, before the refactor and after Step 14 (each package counted with its own tests only):

| Package | Before | After |
|---|---|---|
| `cmd/caged` | 0.0% | 89.5% |
| `internal/agent` | 80.6% | 98.6% |
| `internal/app` | 33.3% | 100.0% |
| `internal/cli` | 64.8% | 91.3% |
| `internal/docker` | 37.3% | 100.0% |
| `internal/hostconfig` | 77.2% | 89.7% |
| `internal/plan` | 73.9% | 100.0% |
| `internal/project` | 0.0% | 96.0% |
| `internal/runtime` | 0.0% | 100.0% |
| `internal/system` | (package did not exist) | 100.0% |

Total statement coverage: **59.2% before, 95.0% after**. With `-coverpkg=./...`, which counts code that is run by any package's tests: **60.5% before, 96.4% after**. Remove `cover.out` afterwards (the last command above does that), otherwise the fingerprint changes.

**macOS cross-compile and vet (from any OS):**

````bash
GOOS=darwin GOARCH=arm64 go build ./...
GOOS=darwin GOARCH=amd64 go build ./...
GOOS=darwin GOARCH=arm64 go vet ./...
GOOS=darwin GOARCH=amd64 go vet ./...
````

Expected: all four commands print nothing.

**Formatting:** `gofmt -l .` prints nothing.

## 5.2 Every test and what it proves

Status: **kept** means the same test, with only its call sites adapted to the new API. **moved** means the same assertions in another file. **renamed** means the same assertions under a new name. **new** means the test was added by the refactor. None of the original 36 tests was weakened or deleted.

### `cmd/caged/architecture_test.go`

| Test | Status | What it proves |
|---|---|---|
| `TestPackageDependenciesPointInward` | new | Every package imports only the caged packages in its allow-list, which is the dependency table in docs/architecture.md. |
| `TestAllowedDependencyGraphHasNoCycles` | new | The allow-list itself has no import cycle. |
| `TestOnlySystemTouchesTheOperatingSystem` | new | No package except internal/system (and the composition root) imports os, os/exec, os/signal, os/user, syscall or runtime, and only internal/system calls filepath functions that read the disk. |
| `TestNoPackageLevelMutableState` | new | No production package declares a package-level var, except main.version (set by -ldflags) and //go:embed files. |

### `cmd/caged/golden_test.go`

| Test | Status | What it proves |
|---|---|---|
| `TestGoldenDockerInvocations` | new | 51 complete command lines produce the same exit code, error, stdout, stderr, docker invocations (argument by argument), build-context checksums, and temp-dir cleanup as the original code. It runs once for an injected Linux platform (fixtures captured from the original) and once for macOS (102 subtests). |

### `cmd/caged/main_test.go`

| Test | Status | What it proves |
|---|---|---|
| `TestNewAgentWorksEndToEnd` | new | One extra agent works through the real composition root: the config resolver reports its path, its env name is forwarded, its prompt style is used, and `caged list` shows it. No other code knows about it. |
| `TestRunTreatsNilArgsAsNoArguments` | new | `run` never falls back to the test binary's own `os.Args`. |

### `internal/agent/agent_test.go`

| Test | Status | What it proves |
|---|---|---|
| `TestLookup` | kept | Known names resolve, an unknown name gives the exact "supported agents" list, and Names() is sorted. It now uses an explicit six-agent registry, so adding an agent cannot break it. |
| `TestClaudeConfigUsesDirectoryInsteadOfSingleFileMount` | kept | Claude mounts the ~/.claude directory, never the single file ~/.claude.json. |
| `TestCodexConfigUsesDirectoryMountAndMasksRuntimePaths` | kept | Codex mounts ~/.codex as one directory with the five masked runtime paths. |
| `TestGitHubCLIUsesHostConfigAndTokens` | kept | gh runs `gh`, mounts ~/.config/gh read-write, and forwards GH_TOKEN and GITHUB_TOKEN. |
| `TestResolveEnvVars` | kept | Only set variables are forwarded; names are de-duplicated and invalid names dropped (now through the ResolveEnv method). |
| `TestLaunchCommandWithoutPromptUsesRegistryCommand` | moved | Was cli's TestAgentCommandUsesRegistryCommand. Without a prompt the command is the registry command, returned as a copy. |
| `TestLaunchCommandPromptStyles` | new | Positional and flag prompt styles, a missing flag, and an agent without prompt support give the same results and errors as the old switch in plan. |
| `TestValidEnvNameMatchesDocumentedPattern` | new | The hand-written validEnvName agrees with the old regexp ^[A-Za-z_][A-Za-z0-9_]*$ on tricky inputs, and ValidateEnvName keeps its error text. |

### `internal/agent/definitions_test.go`

| Test | Status | What it proves |
|---|---|---|
| `TestBuiltinAgentDefinitionsAreUnchanged` | new | A %#v dump of all six built-in agents equals the dump captured from the original code, field for field. |

### `internal/agent/registry_test.go`

| Test | Status | What it proves |
|---|---|---|
| `TestBuiltinIncludesShippedAgents` | new | Builtin() contains the six shipped agents. |
| `TestBuiltinRegistryContract` | new | Every built-in agent follows the rules (sorted, described, prompt flag present, valid env names, clean relative config paths). A new agent file is checked automatically. |
| `TestNewRegistryRejectsInvalidDefinitions` | new | Empty, uppercase or padded names, a missing command, and duplicates are rejected. |
| `TestRegistryLookupNormalizesInput` | new | Lookup ignores case and surrounding spaces and quotes the original input in its error, as before. |
| `TestRegistryReturnsCopies` | new | Callers cannot change a registered agent through returned slices. |
| `TestNewAgentNeedsOnlyItsDefinition` | new | A new agent works through the registry without changes anywhere else. |

### `internal/app/planner_test.go`

| Test | Status | What it proves |
|---|---|---|
| `TestPlannerCombinesMountsConfigsAndEnv` | new | The full RunSpec and the exact --verbose log (warning, config pass/missing/refused, env pass/skipped) for a request. |
| `TestPlannerUsesWorkDirFlagWithoutReadingCwd` | new | --workdir is passed to the project resolver and the current directory is not read. |
| `TestNoMountAndWorkdirAreIncompatible` | moved | Was in cli. --no-mount together with --workdir is rejected with the same message. |
| `TestPlannerNoMountNoConfigNeedsNoHomeOrCwd` | new | --no-mount --no-config works without HOME or a current directory, and prints no credentials warning. |
| `TestPlannerErrors` | new | Every planner error message: cwd, home (both places), project, prompt without agent, agent without prompt support. |

### `internal/app/service_test.go`

| Test | Status | What it proves |
|---|---|---|
| `TestServicePlansAgentPromptAndDelegates` | kept | A Codex prompt becomes `codex exec <prompt>` and no build happens when the image exists. |
| `TestServiceBuildsMissingImageBeforeRun` | kept | A missing image is built first, with the same message, and the container still runs. |
| `TestServiceReturnsBuildFailureWithoutRunning` | kept | A failed build stops the run. |
| `TestServiceReturnsPlanFailureWithoutRunning` | new | A planning error stops the run. |
| `TestServiceReturnsRunErrorUnchanged` | new | The container's error is returned unchanged, so the exit code survives. |
| `TestServiceBuildHonorsNoCache` | new | `caged build [--no-cache]` sends the matching BuildSpec. |

### `internal/cli/root_test.go`

| Test | Status | What it proves |
|---|---|---|
| `TestRootCmdRejectsTooManyArguments` | kept | Three positional arguments are rejected and nothing runs. |
| `TestBuildRequest` | renamed | Was TestResolveTarget. No argument means a shell, the first names an agent, the second is the prompt; unknown agents and bad env names fail in the original order. |
| `TestRootCmdPassesFlagsToService` | new | Every flag reaches the app.Request. |
| `TestAgentNamesDoNotCollideWithSubcommands` | kept | No agent is named build, list, help or completion (help and completion were added to the check). |
| `TestInvalidEnvFlagRejected` | kept | --env bad-name is rejected before anything runs. |
| `TestBuildCommand` | new | `build --no-cache` calls Build(true) and prints "built runtime image caged-runtime:latest"; a failure prints nothing. |
| `TestListCommandUsesInjectedAgents` | new | `caged list` prints the injected registry, sorted. |

### `internal/docker/args_test.go`

| Test | Status | What it proves |
|---|---|---|
| `TestRunArgs` | renamed | Was TestBuildArgs. Exact `docker run` arguments for mounts, config mounts, read-only, env names, return-to-shell, and --user on Linux only. |
| `TestRunArgsExposeOnlyPlannedMounts` | renamed | Was TestBuildArgsExposeOnlyPlannedMounts. Only planned volumes, never --privileged or the Docker socket. |
| `TestRunArgsMaskCodexRuntimeDirectories` | renamed | Was TestBuildArgsMaskCodexRuntimeDirectories. Masked paths become --tmpfs mounts. |

### `internal/docker/client_test.go`

| Test | Status | What it proves |
|---|---|---|
| `TestCheckRunsDockerInfo` | new | `docker info --format {{.ServerVersion}}` runs with combined output and no input. |
| `TestCheckErrors` | new | Exact errors for an unsupported CPU, a missing CLI, and a stopped daemon on Linux and on macOS. |
| `TestImageExists` | new | `docker image inspect` decides whether the image exists. |
| `TestRunConnectsStdioAndPassesArgs` | new | docker run gets the real stdin and stdout and the exact arguments, and its stderr is passed through. |
| `TestRunErrors` | new | Exit errors are returned unchanged on Linux; on macOS the File Sharing hint wraps them; a failed check stops the run; docker vanishing after the check gives the old message. |
| `TestPlatformRules` | new | --user only on Linux, the Docker Desktop wording, the arch check, and the shared-path detection. |
| `TestBuildWritesContextRunsDockerBuildAndCleansUp` | new | Build writes both files with their modes, runs the exact `docker build` arguments with and without --no-cache, and removes the temp dir. |
| `TestBuildErrors` | new | Every build error message, and cleanup on every path. |

### `internal/hostconfig/resolver_edges_test.go`

| Test | Status | What it proves |
|---|---|---|
| `TestResolveRefusesUnresolvableHome` | new | An unresolvable home gives exactly one refusal and nothing else. |
| `TestResolveReportsMissingPathsInRegistryOrder` | new | Missing paths are reported in agent order, then .gitconfig. |
| `TestResolveHonorsOverrides` | new | Relative, absolute and empty override values. |
| `TestResolveRefusesUnresolvableConfigPath` | new | A config path whose symlinks cannot be resolved is refused. |
| `TestResolveReportsUninspectableEntries` | new | Unreadable entries and symlinks inside a tree are refused with the original messages. |
| `TestResolveMountsSingleFileConfigAndRejectsBadMasks` | new | A single-file config mounts; an escaping mask and a masked file root are refused. |
| `TestResolveDeduplicatesSharedConfigPaths` | new | Two agents sharing a path produce one mount. |
| `TestSharedFilesAreData` | new | Shared files are data: mounted read-only, refused when not regular or outside home, and reported when missing. |
| `TestIsWithin` | new | Path containment, including the ../ and prefix traps. |

### `internal/hostconfig/resolver_test.go`

| Test | Status | What it proves |
|---|---|---|
| `TestResolveAllConfigsMountsExistingConfigReadWrite` | kept | Existing configs mount read-write, .gitconfig read-only, and an empty home mounts nothing. |
| `TestConfigOverrideCannotMountHomeDirectory` | kept | CODEX_HOME=$HOME is refused (environment now injected). |
| `TestConfigSymlinkOutsideHomeIsRefused` | kept | A config symlink leaving home is refused. |
| `TestConfigSymlinkOutsideConfigDirectoryIsRefused` | kept | A symlink inside a config tree that leaves the tree is refused. |
| `TestSpecialFileInsideConfigTreeIsRefused` | kept | A FIFO inside a config tree is refused. |
| `TestCodexHomeMountPreservesDirectoryAndMasksRuntimeEntries` | kept | Codex mounts as one directory and its masked sockets and links are not refused. |
| `TestSharedGitConfigMustBeRegularFile` | kept | A directory named .gitconfig is refused. |

### `internal/plan/run_test.go`

| Test | Status | What it proves |
|---|---|---|
| `TestBuildRunSpecUsesAgentPromptStyle` | kept | Each agent's prompt style, plus return-to-shell. |
| `TestBuildRunSpecCopiesInputs` | kept | The spec does not share slices with its inputs. |
| `TestBuildRunSpecShellAndErrors` | new | The shell spec, a copied project mount, and both error paths. |

### `internal/project/mount_test.go`

| Test | Status | What it proves |
|---|---|---|
| `TestNewMountGuardRails` | moved | Was in cli. / and the home directory need --allow-broad-mount; the mount goes to /workspace/<folder>. |
| `TestResolveProjectMount` | moved | Was in cli. Real directories resolve, files are rejected, home is refused. |
| `TestResolverResolvesRelativePathsAndMacOSSymlinks` | new | A relative path resolves against the cwd, and the macOS /tmp -> /private/tmp style symlink is followed. |
| `TestResolverComparesHomeAfterResolvingSymlinks` | new | A symlinked home is still recognised as home. |
| `TestResolverErrors` | new | Every resolver error message. |

### `internal/runtime/runtime_test.go`

| Test | Status | What it proves |
|---|---|---|
| `TestEmbeddedDockerfileIsPresent` | kept | The Dockerfile is embedded. |
| `TestRuntimeAgentHomeDirectoriesAreWritableByMappedUsers` | kept | Agent home paths exist and are chmod 0777 in the image. |
| `TestEmbeddedEntrypointRunsAgentAndReturnsToShell` | kept | The entrypoint runs the command and returns to a shell. |
| `TestRuntimeCodexCommandDisablesDaemon` | kept | Codex is wrapped with --no-daemon. |
| `TestRuntimeImageLeavesClaudeConfigDestinationWritable` | kept | The image removes the installer's ~/.claude.json. |
| `TestClaudeUsesDirectoryConfigMount` | kept | CLAUDE_CONFIG_DIR points into the mounted directory. |
| `TestRuntimePiInstallDoesNotOverlapHostConfigMount` | kept | Pi is installed outside the mounted ~/.pi. |
| `TestRuntimeIncludesGithubCLIAndPersistsItsConfigDirectory` | kept | gh comes from GitHub's signed repository and its config dir exists. |
| `TestBuildSpecDescribesEmbeddedBuildContext` | new | BuildSpec describes the tag, the Dockerfile name, both files with labels and modes, and --no-cache, and returns fresh data. |

### `internal/system/system_test.go`

| Test | Status | What it proves |
|---|---|---|
| `TestFSUsesTheRealFilesystem` | new | Every FS method works on a real temp directory, including symlinks. |
| `TestEnvReadsTheProcessEnvironment` | new | LookupEnv, UserHomeDir and Getwd read the real process. |
| `TestExecutorRunsProcessesAndReportsExitCodes` | new | Stdin, stdout and stderr are wired, exit status 3 comes back as *exec.ExitError, and a cancelled context stops the start. |
| `TestCurrentPlatformAndStdioDescribeThisProcess` | new | Platform matches runtime.GOOS, GOARCH and the user ids, and Stdio holds the real *os.File streams (needed for the docker -it TTY). |

## 5.3 The golden tests and their fixtures

`cmd/caged/golden_test.go` creates a fake home folder with every supported config, a fake project, and a fake
`docker` shell script. It then runs 51 complete command lines through `run`, the real composition root. The fake
`docker` writes every call it receives, one argument per line, into a log. For each command line the test records:

- the exit code and the error;
- stdout and stderr;
- every docker call;
- the checksums of the build context;
- the number of leftover temporary entries.

That record is compared with one `.golden` file per command line.

- `cmd/caged/testdata/golden/linux/` (51 files) was recorded from the **original** code in Step 1. It never
  changes afterwards.
- `cmd/caged/testdata/golden/darwin/` (51 files) was recorded in Step 11 with an injected macOS platform. A
  script checked it against the Linux files. 22 files are identical. 29 differ only because the `--user` lines are
  missing (Docker Desktop maps file ownership). Two say "start Docker Desktop", and one adds the Docker Desktop File
  Sharing hint.
- `internal/agent/testdata/builtin_agents.golden` holds the `%#v` dump of the six built-in agents, recorded from the
  original code in Step 1.

The complete text of every fixture is in Step 1 (`linux/` and `builtin_agents.golden`) and Step 11 (`darwin/`).
To confirm all of them at once:


````bash
cat cmd/caged/testdata/golden/linux/*.golden | sha256sum
cat cmd/caged/testdata/golden/darwin/*.golden | sha256sum
sha256sum internal/agent/testdata/builtin_agents.golden
````

Expected output (on macOS use `shasum -a 256` instead of `sha256sum`; the shell sorts `*.golden` alphabetically):

````text
36f59b929ad6c27dcb450531dbe4c27976bbc8c9db541590ad7136a7104be81f  -
b733ea820ddc260dfff70f30cf17f9e8c427231db3c32d2c32d7db020d13ed8a  -
08f0a76c5efd51bbe44933a8063e662ef1bb507b13c213bcd20b2963c85b6f6a  internal/agent/testdata/builtin_agents.golden
````

If you ever need to re-create a missing golden file, delete it and run `CAGED_UPDATE_GOLDEN=1 go test ./cmd/caged/ ./internal/agent/`. That mode only creates files that are missing and never overwrites an existing one. Do not do this while following the steps: the files in this document were recorded from the original code.

## 5.4 Full code of every added or changed test file

### 5.4.1 `cmd/caged/architecture_test.go` (new file)

````go
package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const modulePath = "github.com/JeremyOrr4/caged"

// allowedImports lists the caged packages each package may import. It is the
// dependency direction documented in docs/architecture.md: policy (app) never
// imports an adapter, and agent and system import nothing from caged.
func allowedImports() map[string][]string {
	return map[string][]string{
		"cmd/caged":           {"internal/agent", "internal/app", "internal/cli", "internal/docker", "internal/hostconfig", "internal/project", "internal/system"},
		"internal/cli":        {"internal/agent", "internal/app", "internal/runtime"},
		"internal/app":        {"internal/agent", "internal/plan", "internal/runtime"},
		"internal/docker":     {"internal/plan", "internal/system"},
		"internal/hostconfig": {"internal/agent", "internal/plan"},
		"internal/project":    {"internal/plan"},
		"internal/runtime":    {"internal/plan"},
		"internal/plan":       {"internal/agent"},
		"internal/agent":      {},
		"internal/system":     {},
	}
}

// osPackages give direct access to the operating system.
var osPackages = map[string]bool{"os": true, "os/exec": true, "os/signal": true, "os/user": true, "syscall": true, "runtime": true}

// filepathIO are the path/filepath functions that read the filesystem.
var filepathIO = map[string]bool{"Abs": true, "EvalSymlinks": true, "Glob": true, "Walk": true, "WalkDir": true}

type sourceFile struct {
	pkg  string // directory relative to the module root, e.g. "internal/app"
	path string
	ast  *ast.File
}

// productionFiles parses every non-test Go file in the module.
func productionFiles(t *testing.T) []sourceFile {
	t.Helper()
	root := filepath.Join("..", "..")
	var files []sourceFile
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && (entry.Name() == ".git" || entry.Name() == "testdata") {
			return filepath.SkipDir
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		files = append(files, sourceFile{pkg: filepath.ToSlash(rel), path: path, ast: parsed})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no Go files found")
	}
	return files
}

func importPaths(file *ast.File) []string {
	var paths []string
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err == nil {
			paths = append(paths, path)
		}
	}
	return paths
}

func TestPackageDependenciesPointInward(t *testing.T) {
	t.Parallel()

	allowed := allowedImports()
	for _, file := range productionFiles(t) {
		permitted, known := allowed[file.pkg]
		if !known {
			t.Errorf("%s: package %s is not in allowedImports; add it and document it in docs/architecture.md", file.path, file.pkg)
			continue
		}
		for _, path := range importPaths(file.ast) {
			if !strings.HasPrefix(path, modulePath+"/") {
				continue
			}
			target := strings.TrimPrefix(path, modulePath+"/")
			ok := false
			for _, p := range permitted {
				ok = ok || p == target
			}
			if !ok {
				t.Errorf("%s: %s must not import %s", file.path, file.pkg, target)
			}
		}
	}
}

func TestAllowedDependencyGraphHasNoCycles(t *testing.T) {
	t.Parallel()

	allowed := allowedImports()
	const (
		unvisited = iota
		visiting
		done
	)
	state := map[string]int{}
	var visit func(pkg string, trail []string)
	visit = func(pkg string, trail []string) {
		switch state[pkg] {
		case visiting:
			t.Fatalf("dependency cycle: %s -> %s", strings.Join(trail, " -> "), pkg)
		case done:
			return
		}
		state[pkg] = visiting
		for _, dep := range allowed[pkg] {
			visit(dep, append(trail, pkg))
		}
		state[pkg] = done
	}
	pkgs := make([]string, 0, len(allowed))
	for pkg := range allowed {
		pkgs = append(pkgs, pkg)
	}
	sort.Strings(pkgs)
	for _, pkg := range pkgs {
		visit(pkg, nil)
	}
}

func TestOnlySystemTouchesTheOperatingSystem(t *testing.T) {
	t.Parallel()

	for _, file := range productionFiles(t) {
		adapter := file.pkg == "internal/system"
		if adapter {
			continue
		}
		for _, path := range importPaths(file.ast) {
			if osPackages[path] && file.pkg != "cmd/caged" {
				t.Errorf("%s imports %q; only internal/system (and the composition root) may", file.path, path)
			}
		}
		ast.Inspect(file.ast, func(node ast.Node) bool {
			call, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if pkg, ok := call.X.(*ast.Ident); ok && pkg.Name == "filepath" && filepathIO[call.Sel.Name] {
				t.Errorf("%s calls filepath.%s, which reads the filesystem; use an injected FileSystem", file.path, call.Sel.Name)
			}
			return true
		})
	}
}

func TestNoPackageLevelMutableState(t *testing.T) {
	t.Parallel()

	for _, file := range productionFiles(t) {
		for _, decl := range file.ast.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			if embedsFile(gen) {
				continue // go:embed requires a var; the embedded strings are never assigned
			}
			for _, spec := range gen.Specs {
				for _, name := range spec.(*ast.ValueSpec).Names {
					if file.pkg == "cmd/caged" && name.Name == "version" {
						continue // set by -ldflags "-X main.version=..." at link time
					}
					t.Errorf("%s: package-level var %s; use a constant, a function, or a field", file.path, name.Name)
				}
			}
		}
	}
}

func embedsFile(decl *ast.GenDecl) bool {
	if decl.Doc == nil {
		return false
	}
	for _, comment := range decl.Doc.List {
		if strings.HasPrefix(comment.Text, "//go:embed ") {
			return true
		}
	}
	return false
}
````

### 5.4.2 `cmd/caged/golden_test.go` (new file)

````go
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/system"
)

// fakeDocker stands in for the docker CLI. It appends every invocation to
// $FAKE_DOCKER_LOG (one argument per line, then "---") and fakes the replies
// caged depends on. FAKE_DOCKER_* variables select failure modes.
const fakeDocker = `#!/bin/sh
{ printf '%s\n' "$@"; echo '---'; } >> "$FAKE_DOCKER_LOG"
case "$1" in
info)
  if [ -n "$FAKE_DOCKER_DAEMON_DOWN" ]; then
    echo 'Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?' >&2
    exit 1
  fi
  echo '27.0.0'
  ;;
image)
  if [ -n "$FAKE_DOCKER_IMAGE_MISSING" ]; then exit 1; fi
  ;;
build)
  for arg in "$@"; do context="$arg"; done
  {
    echo "build context Dockerfile: $(cksum < "$context/Dockerfile")"
    echo "build context caged-entrypoint.sh: $(cksum < "$context/caged-entrypoint.sh")"
    if [ -x "$context/caged-entrypoint.sh" ]; then echo 'build context entrypoint is executable'; fi
    echo '---'
  } >> "$FAKE_DOCKER_LOG"
  echo 'fake docker build output'
  ;;
run)
  echo 'fake docker run output'
  if [ -n "$FAKE_DOCKER_RUN_STDERR" ]; then echo "$FAKE_DOCKER_RUN_STDERR" >&2; fi
  exit "${FAKE_DOCKER_RUN_EXIT:-0}"
  ;;
esac
exit 0
`

// goldenPrompt contains shell syntax to prove prompts are passed verbatim.
const goldenPrompt = `hello $(world) "quoted" 'single'`

type goldenCase struct {
	name string
	args []string
	// env is applied after the defaults; "$ROOT" is replaced with the scenario root.
	env map[string]string
	// noDocker hides the fake docker binary from PATH.
	noDocker bool
}

func goldenCases() []goldenCase {
	cases := []goldenCase{
		{name: "shell", args: []string{}},
		{name: "shell-no-mount-no-config", args: []string{"--no-mount", "--no-config"}},
		{name: "list", args: []string{"list"}},
		{name: "version", args: []string{"--version"}},
		{name: "help", args: []string{"--help"}},
		{name: "help-list", args: []string{"help", "list"}},
		{name: "help-unknown-topic", args: []string{"help", "nope"}},
		{name: "build-help", args: []string{"build", "--help"}},
		{name: "unknown-flag", args: []string{"--bogus"}},
		{name: "build", args: []string{"build"}},
		{name: "build-no-cache", args: []string{"build", "--no-cache"}},
		{name: "build-rejects-args", args: []string{"build", "extra"}},
		{name: "list-rejects-args", args: []string{"list", "extra"}},
		{name: "complete-agents", args: []string{"__complete", ""}},
		{name: "complete-prompt", args: []string{"__complete", "claude", ""}},
		{name: "agent-uppercase", args: []string{"Claude"}},
		{name: "unsupported-agent", args: []string{"aider"}},
		{name: "too-many-args", args: []string{"claude", "one", "two"}},
		{name: "env-invalid", args: []string{"--env", "bad-name", "claude"}},
		{name: "env-padded-name", args: []string{"--env", " CAGED_GOLDEN_SET ", "--verbose", "gemini"}, env: map[string]string{"CAGED_GOLDEN_SET": "x"}},
		{
			name: "shell-env",
			args: []string{"--env", "CAGED_GOLDEN_SET", "--env", "CAGED_GOLDEN_UNSET", "--verbose", "--no-config"},
			env:  map[string]string{"CAGED_GOLDEN_SET": "x", "ANTHROPIC_API_KEY": "not-forwarded-to-a-plain-shell"},
		},
		{
			name: "env-extra-names",
			args: []string{"--env", "CAGED_GOLDEN_SET", "--env", "CAGED_GOLDEN_UNSET", "--env", "ANTHROPIC_API_KEY", "--verbose", "codex"},
			env:  map[string]string{"CAGED_GOLDEN_SET": "value-is-never-printed", "OPENAI_API_KEY": "secret"},
		},
		{name: "verbose-claude", args: []string{"--verbose", "claude"}, env: map[string]string{"ANTHROPIC_API_KEY": "secret"}},
		{name: "no-mount-with-workdir", args: []string{"--no-mount", "-w", "$ROOT/work/other"}},
		{name: "workdir-flag", args: []string{"-w", "$ROOT/work/other", "pi"}},
		{name: "workdir-relative", args: []string{"--workdir", "../other", "--no-config"}},
		{name: "workdir-missing", args: []string{"-w", "$ROOT/work/missing"}},
		{name: "workdir-is-home", args: []string{"-w", "$ROOT/home"}},
		{name: "workdir-home-allowed", args: []string{"--allow-broad-mount", "-w", "$ROOT/home", "--no-config"}},
		{name: "workdir-root-allowed", args: []string{"--allow-broad-mount", "-w", "/", "--no-config"}},
		{name: "workdir-root", args: []string{"-w", "/"}},
		{
			name: "config-overrides",
			args: []string{"--verbose", "--no-mount"},
			env: map[string]string{
				"CODEX_HOME":        "alt-codex",
				"CLAUDE_CONFIG_DIR": "$ROOT/home",
				"COPILOT_HOME":      "$ROOT/outside/copilot",
			},
		},
		{name: "home-unresolvable", args: []string{"--verbose", "--no-mount"}, env: map[string]string{"HOME": "$ROOT/missing-home"}},
		{name: "image-missing-builds-first", args: []string{"gemini"}, env: map[string]string{"FAKE_DOCKER_IMAGE_MISSING": "1"}},
		{name: "run-exit-code", args: []string{"claude"}, env: map[string]string{"FAKE_DOCKER_RUN_EXIT": "7"}},
		{
			name: "run-mounts-denied",
			args: []string{"codex"},
			env: map[string]string{
				"FAKE_DOCKER_RUN_EXIT":   "125",
				"FAKE_DOCKER_RUN_STDERR": "docker: Error response from daemon: Mounts denied: the path is not shared from the host.",
			},
		},
		{name: "daemon-down", args: []string{"claude"}, env: map[string]string{"FAKE_DOCKER_DAEMON_DOWN": "1"}},
		{name: "build-daemon-down", args: []string{"build"}, env: map[string]string{"FAKE_DOCKER_DAEMON_DOWN": "1"}},
		{name: "docker-missing", args: []string{"claude"}, noDocker: true},
	}
	for _, name := range []string{"claude", "codex", "copilot", "gemini", "gh", "pi"} {
		cases = append(cases,
			goldenCase{name: "agent-" + name, args: []string{name}},
			goldenCase{name: "prompt-" + name, args: []string{name, goldenPrompt}},
		)
	}
	return cases
}

// goldenPlatforms are the hosts the golden files describe. The linux files
// were captured from the original code; the darwin files differ from them
// only by the macOS rules in internal/docker/platform.go.
var goldenPlatforms = []system.Platform{
	{OS: "linux", Arch: "amd64", UID: 4242, GID: 4343},
	{OS: "darwin", Arch: "arm64", UID: 501, GID: 20},
}

// TestGoldenDockerInvocations runs complete command lines against a fake
// docker binary and compares everything observable (exit code, error, stdout,
// stderr, and every docker invocation) with files in testdata/golden/<os>.
func TestGoldenDockerInvocations(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake docker binary is a POSIX shell script")
	}
	for _, platform := range goldenPlatforms {
		goldenDir, err := filepath.Abs(filepath.Join("testdata", "golden", platform.OS))
		if err != nil {
			t.Fatal(err)
		}
		for _, tc := range goldenCases() {
			t.Run(platform.OS+"/"+tc.name, func(t *testing.T) {
				got := runGoldenCase(t, tc, platform, originalAgents(t))
				compareGolden(t, filepath.Join(goldenDir, tc.name+".golden"), got)
			})
		}
	}
}

// originalAgents is a registry of the six agents the golden files were
// captured with, so adding a built-in agent does not change the fixtures.
func originalAgents(t *testing.T) *agent.Registry {
	t.Helper()
	builtin := agent.Builtin()
	var agents []agent.Agent
	for _, name := range []string{"claude", "codex", "copilot", "gemini", "gh", "pi"} {
		definition, err := builtin.Lookup(name)
		if err != nil {
			t.Fatal(err)
		}
		agents = append(agents, definition)
	}
	registry, err := agent.NewRegistry(agents...)
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func runGoldenCase(t *testing.T, tc goldenCase, platform system.Platform, agents *agent.Registry) string {
	root := t.TempDir()
	writeGoldenFixtures(t, root)
	logPath := filepath.Join(root, "docker.log")

	for _, name := range []string{
		"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY",
		"GH_TOKEN", "GITHUB_TOKEN", "COPILOT_GITHUB_TOKEN",
		"CLAUDE_CONFIG_DIR", "CODEX_HOME", "COPILOT_HOME",
		"CAGED_GOLDEN_SET", "CAGED_GOLDEN_UNSET",
		"FAKE_DOCKER_DAEMON_DOWN", "FAKE_DOCKER_IMAGE_MISSING", "FAKE_DOCKER_RUN_EXIT", "FAKE_DOCKER_RUN_STDERR",
	} {
		unsetEnv(t, name)
	}
	path := filepath.Join(root, "bin") + string(os.PathListSeparator) + os.Getenv("PATH")
	if tc.noDocker {
		path = filepath.Join(root, "emptybin")
	}
	t.Setenv("PATH", path)
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("TMPDIR", filepath.Join(root, "tmp"))
	t.Setenv("FAKE_DOCKER_LOG", logPath)
	for name, value := range tc.env {
		t.Setenv(name, strings.ReplaceAll(value, "$ROOT", root))
	}
	chdir(t, filepath.Join(root, "work", "project"))

	args := make([]string, 0, len(tc.args))
	for _, arg := range tc.args {
		args = append(args, strings.ReplaceAll(arg, "$ROOT", root))
	}
	stdout, stderr, code, err := executeRun(t, root, args, platform, agents)

	dockerLog, readErr := os.ReadFile(logPath)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		t.Fatal(readErr)
	}
	leftovers, readErr := os.ReadDir(filepath.Join(root, "tmp"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	errText := "<nil>"
	if err != nil {
		errText = err.Error()
	}
	var b strings.Builder
	fmt.Fprintf(&b, "args: %q\n", tc.args)
	fmt.Fprintf(&b, "exit code: %d\n", code)
	fmt.Fprintf(&b, "error: %s\n", errText)
	fmt.Fprintf(&b, "leftover temp entries: %d\n", len(leftovers))
	fmt.Fprintf(&b, "=== stdout\n%s", stdout)
	fmt.Fprintf(&b, "=== stderr\n%s", stderr)
	fmt.Fprintf(&b, "=== docker invocations\n%s", dockerLog)
	return normalizeGolden(t, b.String(), root, platform)
}

// executeRun calls run, the composition root, and captures the process-wide
// stdout and stderr: cobra writes there directly, and caged's own output is
// pointed at the same files through stdio.
func executeRun(t *testing.T, root string, args []string, platform system.Platform, agents *agent.Registry) (string, string, int, error) {
	stdoutFile, err := os.Create(filepath.Join(root, "stdout.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer stdoutFile.Close()
	stderrFile, err := os.Create(filepath.Join(root, "stderr.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer stderrFile.Close()

	oldStdout, oldStderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdoutFile, stderrFile
	stdio := system.Stdio{In: strings.NewReader(""), Out: stdoutFile, Err: stderrFile}
	code := run(context.Background(), "test", args, stdio, platform, agents)
	os.Stdout, os.Stderr = oldStdout, oldStderr

	stdout, err := os.ReadFile(stdoutFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := os.ReadFile(stderrFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(stdout), string(stderr), code, errorLine(string(stderr), code)
}

// errorLine recovers the error that run printed as the last stderr line.
func errorLine(stderr string, code int) error {
	if code == 0 {
		return nil
	}
	lines := strings.Split(strings.TrimSuffix(stderr, "\n"), "\n")
	return errors.New(lines[len(lines)-1])
}

// writeGoldenFixtures creates a fake home with every supported config, a
// project directory, and the fake docker binary.
func writeGoldenFixtures(t *testing.T, root string) {
	t.Helper()
	for _, dir := range []string{
		"bin", "emptybin", "tmp", "work/project", "work/other", "outside/pi", "outside/copilot",
		"home/.claude", "home/.codex/app-server-control", "home/.codex/packages/app-server-daemon",
		"home/.codex/tmp/arg0", "home/.gemini", "home/.pi/agent", "home/.config/gh", "home/.copilot",
		"home/alt-codex",
	} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for file, contents := range map[string]string{
		"work/project/main.go":           "package main\n",
		"outside/pi/settings.json":       "{}\n",
		"home/.claude/settings.json":     "{}\n",
		"home/.claude/.credentials.json": "{}\n",
		"home/.claude.json":              "{}\n",
		"home/.codex/config.toml":        "model = 'test'\n",
		"home/.codex/auth.json":          "{}\n",
		"home/.gemini/settings.json":     "{}\n",
		"home/.gemini/oauth_creds.json":  "{}\n",
		"home/.pi/agent/settings.json":   "{}\n",
		"home/.config/gh/hosts.yml":      "github.com: {}\n",
		"home/.gitconfig":                "[user]\n\tname = Test\n",
		"home/alt-codex/config.toml":     "model = 'alt'\n",
	} {
		if err := os.WriteFile(filepath.Join(root, file), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "docker"), []byte(fakeDocker), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(root, "home/.codex/app-server-control/daemon.sock"), 0o600); err != nil {
		t.Fatal(err)
	}
	for link, target := range map[string]string{
		"home/.codex/packages/app-server-daemon/current": "/opt/homebrew/bin/codex",
		"home/.gemini/current-settings.json":             "settings.json",
		"home/.config/pi":                                filepath.Join(root, "outside", "pi"),
	} {
		if err := os.Symlink(target, filepath.Join(root, link)); err != nil {
			t.Fatal(err)
		}
	}
}

var buildDirPattern = regexp.MustCompile(`caged-runtime-[0-9]+`)

// normalizeGolden replaces values that change between runs and machines.
func normalizeGolden(t *testing.T, text, root string, platform system.Platform) string {
	t.Helper()
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	text = strings.ReplaceAll(text, resolvedRoot, "$ROOT")
	text = strings.ReplaceAll(text, root, "$ROOT")
	text = strings.ReplaceAll(text, fmt.Sprintf("\n--user\n%d:%d\n", platform.UID, platform.GID), "\n--user\n$UID:$GID\n")
	return buildDirPattern.ReplaceAllString(text, "caged-runtime-N")
}

// compareGolden checks got against the golden file. With CAGED_UPDATE_GOLDEN=1
// a missing golden file is created; existing files are never overwritten.
func compareGolden(t *testing.T, path, got string) {
	t.Helper()
	want, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) && os.Getenv("CAGED_UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if err != nil {
		t.Fatalf("read golden file (run with CAGED_UPDATE_GOLDEN=1 to create it): %v", err)
	}
	if got != string(want) {
		t.Errorf("output differs from %s\n--- want\n%s\n--- got\n%s", path, want, got)
	}
}

// unsetEnv removes an environment variable for the duration of the test;
// t.Setenv restores the previous value afterwards.
func unsetEnv(t *testing.T, name string) {
	t.Helper()
	t.Setenv(name, "")
	if err := os.Unsetenv(name); err != nil {
		t.Fatal(err)
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Error(err)
		}
	})
}
````

### 5.4.3 `cmd/caged/main_test.go` (new file)

````go
package main

import (
	"bytes"
	"context"
	"runtime"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/system"
)

// TestNewAgentWorksEndToEnd registers one extra agent and runs it through the
// real composition root: the config resolver, env forwarding, prompt style,
// and `caged list` all pick it up with no other code change.
func TestNewAgentWorksEndToEnd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake docker binary is a POSIX shell script")
	}
	aider := agent.Agent{
		Name:        "aider",
		Command:     []string{"aider"},
		Description: "Aider",
		ConfigPaths: []agent.ConfigPath{{HostPath: ".aider", ContainerPath: ".aider", Writable: true}},
		Prompt:      agent.PromptFlag,
		PromptFlag:  "--message",
		Auth:        agent.AuthStrategy{Environment: []string{"OPENAI_API_KEY"}},
	}
	agents, err := agent.NewRegistry(append(originalAgents(t).All(), aider)...)
	if err != nil {
		t.Fatal(err)
	}
	linux := goldenPlatforms[0]

	got := runGoldenCase(t, goldenCase{args: []string{"--verbose", "aider", "add tests"}, env: map[string]string{"OPENAI_API_KEY": "x"}}, linux, agents)
	for _, want := range []string{
		"exit code: 0\n",
		"config: missing $ROOT/home/.aider\n",
		"env: pass OPENAI_API_KEY\n",
		"\n-e\nOPENAI_API_KEY\n",
		"\ncaged-runtime:latest\naider\n--message\nadd tests\n---\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output does not contain %q:\n%s", want, got)
		}
	}

	list := runGoldenCase(t, goldenCase{args: []string{"list"}}, linux, agents)
	if !strings.Contains(list, "=== stdout\naider\nclaude\ncodex\n") {
		t.Errorf("list does not show the new agent first:\n%s", list)
	}
}

// TestRunTreatsNilArgsAsNoArguments makes sure run never falls back to the
// test binary's own os.Args.
func TestRunTreatsNilArgsAsNoArguments(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // no docker: the shell run fails at the image check
	var stderr bytes.Buffer
	stdio := system.Stdio{In: strings.NewReader(""), Out: &bytes.Buffer{}, Err: &stderr}
	code := run(context.Background(), "test", nil, stdio, goldenPlatforms[0], originalAgents(t))
	if code != 1 || !strings.Contains(stderr.String(), "Docker is not installed or not on PATH") {
		t.Fatalf("run(nil) = %d, stderr %q; want the plain shell path", code, stderr.String())
	}
}
````

### 5.4.4 `internal/agent/agent_test.go` (changed file)

````go
package agent

import (
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestLookup(t *testing.T) {
	t.Parallel()

	// An explicit registry keeps the exact-list assertions below independent
	// of agents added to Builtin later.
	registry, err := NewRegistry(claude(), codex(), copilot(), gemini(), gh(), pi())
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		input    string
		wantName string
		wantErr  bool
	}{
		{name: "known claude", input: "claude", wantName: "claude"},
		{name: "known pi", input: "pi", wantName: "pi"},
		{name: "known gemini", input: "gemini", wantName: "gemini"},
		{name: "known codex", input: "codex", wantName: "codex"},
		{name: "known copilot", input: "copilot", wantName: "copilot"},
		{name: "unknown", input: "foo", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			agent, err := registry.Lookup(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("Lookup() error = nil, want an error")
				}
				if !strings.Contains(err.Error(), "supported agents: claude, codex, copilot, gemini, gh, pi") {
					t.Fatalf("Lookup() error = %q, want supported agents list", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Lookup(%q) error = %v", tt.input, err)
			}
			if agent.Name != tt.wantName {
				t.Fatalf("Lookup(%q) name = %q, want %q", tt.input, agent.Name, tt.wantName)
			}
		})
	}

	names := registry.Names()
	want := []string{"claude", "codex", "copilot", "gemini", "gh", "pi"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("Names() = %v, want %v", names, want)
	}
}

func TestClaudeConfigUsesDirectoryInsteadOfSingleFileMount(t *testing.T) {
	t.Parallel()

	claude, err := Builtin().Lookup("claude")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range claude.ConfigPaths {
		if path.HostPath == ".claude.json" {
			t.Fatal("Claude config still uses a single-file bind mount")
		}
	}
}

func TestCodexConfigUsesDirectoryMountAndMasksRuntimePaths(t *testing.T) {
	t.Parallel()
	codex, err := Builtin().Lookup("codex")
	if err != nil {
		t.Fatal(err)
	}
	if len(codex.ConfigPaths) != 1 || codex.ConfigPaths[0].HostPath != ".codex" || codex.ConfigPaths[0].ContainerPath != ".codex" {
		t.Fatalf("Codex config paths = %+v, want one .codex directory mount", codex.ConfigPaths)
	}
	wantMasks := []string{"app-server-control", "app-server-daemon", "ipc", "packages/app-server-daemon", "tmp/arg0"}
	if !reflect.DeepEqual(codex.ConfigPaths[0].MaskPaths, wantMasks) {
		t.Fatalf("Codex runtime masks = %v, want %v", codex.ConfigPaths[0].MaskPaths, wantMasks)
	}
}

func TestGitHubCLIUsesHostConfigAndTokens(t *testing.T) {
	t.Parallel()
	gh, err := Builtin().Lookup("gh")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gh.Command, []string{"gh"}) {
		t.Fatalf("GitHub CLI command = %v, want [gh]", gh.Command)
	}
	if len(gh.ConfigPaths) != 1 || gh.ConfigPaths[0].HostPath != ".config/gh" || gh.ConfigPaths[0].ContainerPath != ".config/gh" || !gh.ConfigPaths[0].Writable {
		t.Fatalf("GitHub CLI config paths = %+v", gh.ConfigPaths)
	}
	if !reflect.DeepEqual(gh.Auth.Environment, []string{"GH_TOKEN", "GITHUB_TOKEN"}) {
		t.Fatalf("GitHub CLI token variables = %v", gh.Auth.Environment)
	}
}

func TestResolveEnvVars(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		agent     Agent
		requested []string
		lookup    map[string]bool
		want      []string
	}{
		{
			name:      "set allowlisted vars and added names",
			agent:     Agent{Auth: AuthStrategy{Environment: []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY"}}},
			requested: []string{"GEMINI_API_KEY", "INVALID-NAME"},
			lookup: map[string]bool{
				"ANTHROPIC_API_KEY": true,
				"GEMINI_API_KEY":    true,
			},
			want: []string{"ANTHROPIC_API_KEY", "GEMINI_API_KEY"},
		},
		{
			name:   "skip unset vars",
			agent:  Agent{Auth: AuthStrategy{Environment: []string{"OPENAI_API_KEY"}}},
			lookup: map[string]bool{},
			want:   nil,
		},
		{
			name:      "dedupe and ignore invalid",
			agent:     Agent{Auth: AuthStrategy{Environment: []string{"ANTHROPIC_API_KEY"}}},
			requested: []string{"ANTHROPIC_API_KEY", "MY_VAR", "bad-name"},
			lookup: map[string]bool{
				"ANTHROPIC_API_KEY": true,
				"MY_VAR":            true,
			},
			want: []string{"ANTHROPIC_API_KEY", "MY_VAR"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			lookup := func(name string) bool { return tt.lookup[name] }
			got := tt.agent.ResolveEnv(tt.requested, lookup)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ResolveEnv() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLaunchCommandWithoutPromptUsesRegistryCommand(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"codex", "claude"} {
		definition, err := Builtin().Lookup(name)
		if err != nil {
			t.Fatal(err)
		}
		got, err := definition.LaunchCommand(nil)
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{name}; !reflect.DeepEqual(got, want) {
			t.Fatalf("LaunchCommand(nil) = %v, want %v", got, want)
		}
		got[0] = "changed"
		if definition.Command[0] != name {
			t.Fatal("LaunchCommand() returned the registry's own slice")
		}
	}
}

func TestLaunchCommandPromptStyles(t *testing.T) {
	t.Parallel()

	prompt := "fix $(this) verbatim"
	tests := []struct {
		name    string
		agent   Agent
		want    []string
		wantErr string
	}{
		{name: "positional", agent: Agent{Name: "a", Command: []string{"a"}, Prompt: PromptPositional}, want: []string{"a", prompt}},
		{name: "flag", agent: Agent{Name: "b", Command: []string{"b", "run"}, Prompt: PromptFlag, PromptFlag: "-p"}, want: []string{"b", "run", "-p", prompt}},
		{name: "flag missing", agent: Agent{Name: "c", Command: []string{"c"}, Prompt: PromptFlag}, wantErr: "c prompt flag is not configured"},
		{name: "unsupported", agent: Agent{Name: "d", Command: []string{"d"}}, wantErr: "d does not support prompt launch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := tt.agent.LaunchCommand(&prompt)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("LaunchCommand() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("LaunchCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidEnvNameMatchesDocumentedPattern(t *testing.T) {
	t.Parallel()

	pattern := regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	for _, name := range []string{
		"", "A", "_", "a1", "1a", "ANTHROPIC_API_KEY", "bad-name", "has space", "trailing\n",
		"é", "A\x00", "Z9_z", "9", "__", "a.b", "\xff",
	} {
		if got, want := validEnvName(name), pattern.MatchString(name); got != want {
			t.Errorf("validEnvName(%q) = %v, want %v", name, got, want)
		}
	}
	if err := ValidateEnvName("  PADDED  "); err != nil {
		t.Errorf("ValidateEnvName() rejected a name with surrounding spaces: %v", err)
	}
	if err := ValidateEnvName("bad-name"); err == nil || err.Error() != `invalid env name "bad-name"; must match ^[A-Za-z_][A-Za-z0-9_]*$` {
		t.Errorf("ValidateEnvName(bad-name) error = %v", err)
	}
}
````

### 5.4.5 `internal/agent/definitions_test.go` (new file)

````go
package agent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBuiltinAgentDefinitionsAreUnchanged pins every field of the built-in
// agents so that moving the definitions around cannot silently change them.
func TestBuiltinAgentDefinitionsAreUnchanged(t *testing.T) {
	t.Parallel()

	var b strings.Builder
	for _, name := range []string{"claude", "codex", "copilot", "gemini", "gh", "pi"} {
		definition, err := Builtin().Lookup(name)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&b, "%#v\n", definition)
	}
	path := filepath.Join("testdata", "builtin_agents.golden")
	want, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) && os.Getenv("CAGED_UPDATE_GOLDEN") == "1" {
		// Create a missing golden file; existing files are never overwritten.
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if err != nil {
		t.Fatalf("read golden file (run with CAGED_UPDATE_GOLDEN=1 to create it): %v", err)
	}
	if got := b.String(); got != string(want) {
		t.Errorf("built-in agent definitions changed\n--- want\n%s\n--- got\n%s", want, got)
	}
}
````

### 5.4.6 `internal/agent/registry_test.go` (new file)

````go
package agent

import (
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestBuiltinIncludesShippedAgents(t *testing.T) {
	t.Parallel()

	registry := Builtin()
	for _, name := range []string{"claude", "codex", "copilot", "gemini", "gh", "pi"} {
		if _, err := registry.Lookup(name); err != nil {
			t.Errorf("Builtin() is missing %q: %v", name, err)
		}
	}
}

// TestBuiltinRegistryContract checks rules every agent must follow, so a new
// agent file is validated without editing any test.
func TestBuiltinRegistryContract(t *testing.T) {
	t.Parallel()

	registry := Builtin()
	names := registry.Names()
	if !sort.StringsAreSorted(names) {
		t.Fatalf("Names() = %v, want sorted", names)
	}
	all := registry.All()
	if len(all) != len(names) {
		t.Fatalf("All() returned %d agents, Names() %d", len(all), len(names))
	}
	for i, definition := range all {
		if definition.Name != names[i] {
			t.Errorf("All()[%d] = %q, want %q", i, definition.Name, names[i])
		}
		if definition.Description == "" {
			t.Errorf("%s: missing description", definition.Name)
		}
		if definition.Prompt == PromptFlag && definition.PromptFlag == "" {
			t.Errorf("%s: PromptFlag style without a flag", definition.Name)
		}
		for _, name := range definition.Auth.Environment {
			if err := ValidateEnvName(name); err != nil {
				t.Errorf("%s: %v", definition.Name, err)
			}
		}
		for _, path := range definition.ConfigPaths {
			for _, rel := range []string{path.HostPath, path.ContainerPath} {
				if rel == "" || filepath.IsAbs(rel) || filepath.Clean(rel) != rel || strings.HasPrefix(rel, "..") {
					t.Errorf("%s: config path %q must be a clean path relative to the home directory", definition.Name, rel)
				}
			}
			if path.EnvOverride != "" {
				if err := ValidateEnvName(path.EnvOverride); err != nil {
					t.Errorf("%s: %v", definition.Name, err)
				}
			}
		}
	}
}

func TestNewRegistryRejectsInvalidDefinitions(t *testing.T) {
	t.Parallel()

	valid := Agent{Name: "valid", Command: []string{"valid"}}
	tests := []struct {
		name   string
		agents []Agent
		want   string
	}{
		{name: "empty name", agents: []Agent{{Command: []string{"x"}}}, want: `agent name "" must be lowercase`},
		{name: "uppercase name", agents: []Agent{{Name: "Aider", Command: []string{"aider"}}}, want: `agent name "Aider" must be lowercase`},
		{name: "padded name", agents: []Agent{{Name: " aider", Command: []string{"aider"}}}, want: `agent name " aider" must be lowercase`},
		{name: "no command", agents: []Agent{{Name: "aider"}}, want: `agent "aider" has no command`},
		{name: "duplicate", agents: []Agent{valid, valid}, want: `agent "valid" is registered more than once`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewRegistry(tt.agents...); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("NewRegistry() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestRegistryLookupNormalizesInput(t *testing.T) {
	t.Parallel()

	got, err := Builtin().Lookup("  Claude ")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "claude" {
		t.Fatalf("Lookup() = %q, want claude", got.Name)
	}
	if _, err := Builtin().Lookup("  Nope "); err == nil || !strings.HasPrefix(err.Error(), `unsupported agent "  Nope "; supported agents: `) {
		t.Fatalf("Lookup() error = %v, want the original input quoted", err)
	}
}

func TestRegistryReturnsCopies(t *testing.T) {
	t.Parallel()

	registry := Builtin()
	names := registry.Names()
	names[0] = "changed"
	first, err := registry.Lookup("codex")
	if err != nil {
		t.Fatal(err)
	}
	first.Command[0] = "changed"
	first.ConfigPaths[0].MaskPaths[0] = "changed"
	first.Auth.Environment[0] = "changed"
	all := registry.All()
	all[0].Command[0] = "changed"

	if registry.Names()[0] == "changed" {
		t.Error("Names() exposed the registry's slice")
	}
	second, err := registry.Lookup("codex")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(second, codex()) {
		t.Errorf("Lookup() exposed registry internals; got %+v", second)
	}
	if again := registry.All(); again[0].Command[0] == "changed" {
		t.Error("All() exposed registry internals")
	}
}

// TestNewAgentNeedsOnlyItsDefinition shows the open/closed property: a new
// agent is usable through the registry without changes anywhere else. The
// name is test-only so the test keeps passing when real agents are added.
func TestNewAgentNeedsOnlyItsDefinition(t *testing.T) {
	t.Parallel()

	extra := Agent{
		Name:        "test-only-agent",
		Command:     []string{"test-only-agent"},
		Description: "Test-only agent",
		ConfigPaths: []ConfigPath{{HostPath: ".test-only-agent", ContainerPath: ".test-only-agent", Writable: true}},
		Prompt:      PromptFlag,
		PromptFlag:  "--message",
		Auth:        AuthStrategy{Environment: []string{"OPENAI_API_KEY"}},
	}
	registry, err := NewRegistry(append(Builtin().All(), extra)...)
	if err != nil {
		t.Fatal(err)
	}
	got, err := registry.Lookup("test-only-agent")
	if err != nil {
		t.Fatal(err)
	}
	prompt := "add tests"
	command, err := got.LaunchCommand(&prompt)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"test-only-agent", "--message", "add tests"}; !reflect.DeepEqual(command, want) {
		t.Fatalf("LaunchCommand() = %q, want %q", command, want)
	}
	if names := registry.Names(); len(names) != len(Builtin().Names())+1 || !sort.StringsAreSorted(names) {
		t.Fatalf("Names() = %v, want every built-in plus the new agent, sorted", names)
	}
}
````

### 5.4.7 `internal/app/planner_test.go` (new file)

````go
package app

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/plan"
)

// fakeEnv is a fixed process environment.
type fakeEnv struct {
	vars    map[string]string
	home    string
	homeErr error
	wd      string
	wdErr   error
}

func (e *fakeEnv) LookupEnv(key string) (string, bool) {
	value, ok := e.vars[key]
	return value, ok
}
func (e *fakeEnv) UserHomeDir() (string, error) { return e.home, e.homeErr }
func (e *fakeEnv) Getwd() (string, error)       { return e.wd, e.wdErr }

// fakeProject records the arguments it was asked to resolve.
type fakeProject struct {
	gotWorkDir, gotHome string
	gotAllow            bool
	err                 error
}

func (f *fakeProject) Resolve(workDir, home string, allowBroadMount bool) (*plan.Mount, error) {
	f.gotWorkDir, f.gotHome, f.gotAllow = workDir, home, allowBroadMount
	if f.err != nil {
		return nil, f.err
	}
	return &plan.Mount{HostPath: workDir, ContainerPath: "/workspace/app"}, nil
}

// fakeConfigs returns a fixed report.
type fakeConfigs struct {
	report  plan.ConfigReport
	gotHome string
}

func (f *fakeConfigs) Resolve(home string) plan.ConfigReport {
	f.gotHome = home
	return f.report
}

func lookup(t *testing.T, name string) *agent.Agent {
	t.Helper()
	definition, err := agent.Builtin().Lookup(name)
	if err != nil {
		t.Fatal(err)
	}
	return &definition
}

func TestPlannerCombinesMountsConfigsAndEnv(t *testing.T) {
	t.Parallel()

	env := &fakeEnv{vars: map[string]string{"ANTHROPIC_API_KEY": "secret", "EXTRA": ""}, home: "/Users/dev", wd: "/Users/dev/app"}
	project := &fakeProject{}
	configs := &fakeConfigs{report: plan.ConfigReport{
		Mounts:  []plan.ConfigMount{{Agent: "claude", Name: ".claude", HostPath: "/Users/dev/.claude", DestPath: "/home/caged/.claude", Writable: true}},
		Missing: []string{"/Users/dev/.codex"},
		Refused: []string{"/Users/dev/.pi (resolves outside home)"},
	}}
	var stderr strings.Builder
	prompt := "hello"
	spec, err := NewPlanner(env, project, configs, &stderr).Plan(Request{
		Agent: lookup(t, "claude"), Prompt: &prompt, Verbose: true, AllowBroadMount: true, EnvVars: []string{"EXTRA", "MISSING"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := plan.RunSpec{
		Image:         "caged-runtime:latest",
		Command:       []string{"claude", "hello"},
		Mount:         &plan.Mount{HostPath: "/Users/dev/app", ContainerPath: "/workspace/app"},
		ConfigMounts:  configs.report.Mounts,
		EnvVars:       []string{"ANTHROPIC_API_KEY", "EXTRA"},
		ReturnToShell: true,
	}
	if !reflect.DeepEqual(spec, want) {
		t.Fatalf("Plan() = %+v, want %+v", spec, want)
	}
	if project.gotWorkDir != "/Users/dev/app" || project.gotHome != "/Users/dev" || !project.gotAllow || configs.gotHome != "/Users/dev" {
		t.Fatalf("resolvers got project %+v, configs home %q", project, configs.gotHome)
	}
	wantLog := `WARNING: allowlisted agent credentials are available inside the container.
config: pass /Users/dev/.claude -> /home/caged/.claude
config: missing /Users/dev/.codex
config: refused /Users/dev/.pi (resolves outside home)
env: pass ANTHROPIC_API_KEY
env: pass EXTRA
env: skipped MISSING (unset or disallowed)
`
	if stderr.String() != wantLog {
		t.Fatalf("stderr = %q, want %q", stderr.String(), wantLog)
	}
}

func TestPlannerUsesWorkDirFlagWithoutReadingCwd(t *testing.T) {
	t.Parallel()

	project := &fakeProject{}
	env := &fakeEnv{home: "/home/dev", wdErr: errors.New("cwd must not be read")}
	if _, err := NewPlanner(env, project, &fakeConfigs{}, &strings.Builder{}).Plan(Request{WorkDir: "../other", NoConfig: true}); err != nil {
		t.Fatal(err)
	}
	if project.gotWorkDir != "../other" {
		t.Fatalf("project resolver got %q, want the --workdir value", project.gotWorkDir)
	}
}

func TestNoMountAndWorkdirAreIncompatible(t *testing.T) {
	t.Parallel()

	request := Request{NoMount: true, WorkDir: "/tmp"}
	_, err := NewPlanner(&fakeEnv{}, &fakeProject{}, &fakeConfigs{}, &strings.Builder{}).Plan(request)
	if err == nil || !strings.Contains(err.Error(), "cannot be used with --no-mount") {
		t.Fatalf("Plan() error = %v, want incompatible-flags error", err)
	}
}

func TestPlannerNoMountNoConfigNeedsNoHomeOrCwd(t *testing.T) {
	t.Parallel()

	env := &fakeEnv{homeErr: errors.New("no home"), wdErr: errors.New("no cwd")}
	var stderr strings.Builder
	spec, err := NewPlanner(env, &fakeProject{}, &fakeConfigs{}, &stderr).Plan(Request{NoMount: true, NoConfig: true})
	if err != nil {
		t.Fatal(err)
	}
	if spec.Mount != nil || spec.ConfigMounts != nil || !reflect.DeepEqual(spec.Command, []string{"bash"}) || spec.ReturnToShell {
		t.Fatalf("Plan() = %+v, want a bare shell", spec)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want no credentials warning with --no-config", stderr.String())
	}
}

func TestPlannerErrors(t *testing.T) {
	t.Parallel()

	prompt := "hello"
	tests := []struct {
		name    string
		env     *fakeEnv
		project *fakeProject
		request Request
		want    string
	}{
		{name: "cwd", env: &fakeEnv{wdErr: errors.New("gone")}, request: Request{NoConfig: true}, want: "get current working directory: gone"},
		{name: "home for project", env: &fakeEnv{wd: "/w", homeErr: errors.New("unset")}, request: Request{NoConfig: true}, want: "find home directory: unset"},
		{name: "home for config", env: &fakeEnv{homeErr: errors.New("unset")}, request: Request{NoMount: true}, want: "find home directory: unset"},
		{name: "project", env: &fakeEnv{wd: "/w"}, project: &fakeProject{err: errors.New("refusing")}, request: Request{NoConfig: true}, want: "refusing"},
		{name: "prompt without agent", env: &fakeEnv{}, request: Request{Prompt: &prompt, NoMount: true, NoConfig: true}, want: "a prompt requires an agent"},
		{name: "agent without prompt support", env: &fakeEnv{}, request: Request{Agent: lookup(t, "gh"), Prompt: &prompt, NoMount: true, NoConfig: true}, want: "gh does not support prompt launch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			project := tt.project
			if project == nil {
				project = &fakeProject{}
			}
			if _, err := NewPlanner(tt.env, project, &fakeConfigs{}, &strings.Builder{}).Plan(tt.request); err == nil || err.Error() != tt.want {
				t.Fatalf("Plan() error = %v, want %q", err, tt.want)
			}
		})
	}
}
````

### 5.4.8 `internal/app/service_test.go` (changed file)

````go
package app

import (
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/runtime"
)

// fakeEngine records what the service asks the container runtime to do.
type fakeEngine struct {
	exists   bool
	builds   []plan.BuildSpec
	buildErr error
	ran      *plan.RunSpec
	runErr   error
}

func (f *fakeEngine) ImageExists(context.Context, string) bool { return f.exists }

func (f *fakeEngine) Build(_ context.Context, spec plan.BuildSpec) error {
	f.builds = append(f.builds, spec)
	return f.buildErr
}

func (f *fakeEngine) Run(_ context.Context, spec plan.RunSpec) error {
	f.ran = &spec
	return f.runErr
}

// newTestService wires a Service whose planner sees no environment at all;
// requests must use NoMount and NoConfig.
func newTestService(engine *fakeEngine, stderr io.Writer) *Service {
	planner := NewPlanner(&fakeEnv{homeErr: errors.New("no home"), wdErr: errors.New("no cwd")}, &fakeProject{}, &fakeConfigs{}, stderr)
	return New(engine, planner, stderr)
}

func TestServicePlansAgentPromptAndDelegates(t *testing.T) {
	engine := &fakeEngine{exists: true}
	service := newTestService(engine, io.Discard)
	agent, err := agent.Builtin().Lookup("codex")
	if err != nil {
		t.Fatal(err)
	}
	prompt := "fix $(this) verbatim"
	if err := service.Run(context.Background(), Request{Agent: &agent, Prompt: &prompt, NoMount: true, NoConfig: true}); err != nil {
		t.Fatal(err)
	}
	want := []string{"codex", "exec", prompt}
	if engine.ran == nil || !reflect.DeepEqual(engine.ran.Command, want) {
		t.Fatalf("command = %+v, want %q", engine.ran, want)
	}
	if len(engine.builds) != 0 {
		t.Fatalf("Build called %d times, want 0", len(engine.builds))
	}
}

func TestServiceBuildsMissingImageBeforeRun(t *testing.T) {
	engine := &fakeEngine{}
	var stderr strings.Builder
	service := newTestService(engine, &stderr)
	if err := service.Run(context.Background(), Request{NoMount: true, NoConfig: true}); err != nil {
		t.Fatal(err)
	}
	if len(engine.builds) != 1 || !reflect.DeepEqual(engine.builds[0], runtime.BuildSpec(false)) {
		t.Fatalf("builds = %+v, want one cached runtime image build", engine.builds)
	}
	if want := "Runtime image caged-runtime:latest not found; building it first...\n"; stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
	if engine.ran == nil {
		t.Fatal("container did not run after the image was built")
	}
}

func TestServiceReturnsBuildFailureWithoutRunning(t *testing.T) {
	engine := &fakeEngine{buildErr: errors.New("build failed")}
	service := newTestService(engine, io.Discard)
	if err := service.Run(context.Background(), Request{NoMount: true, NoConfig: true}); err == nil {
		t.Fatal("Run() error = nil, want build failure")
	}
	if engine.ran != nil {
		t.Fatal("runner called after image build failed")
	}
}

func TestServiceReturnsPlanFailureWithoutRunning(t *testing.T) {
	engine := &fakeEngine{exists: true}
	prompt := "hello"
	err := newTestService(engine, io.Discard).Run(context.Background(), Request{Prompt: &prompt, NoMount: true, NoConfig: true})
	if err == nil || err.Error() != "a prompt requires an agent" || engine.ran != nil {
		t.Fatalf("Run() error = %v, ran = %+v", err, engine.ran)
	}
}

func TestServiceReturnsRunErrorUnchanged(t *testing.T) {
	runErr := errors.New("exit status 3")
	engine := &fakeEngine{exists: true, runErr: runErr}
	if err := newTestService(engine, io.Discard).Run(context.Background(), Request{NoMount: true, NoConfig: true}); err != runErr {
		t.Fatalf("Run() error = %v, want %v", err, runErr)
	}
}

func TestServiceBuildHonorsNoCache(t *testing.T) {
	for _, noCache := range []bool{false, true} {
		engine := &fakeEngine{}
		if err := newTestService(engine, io.Discard).Build(context.Background(), noCache); err != nil {
			t.Fatal(err)
		}
		if len(engine.builds) != 1 || !reflect.DeepEqual(engine.builds[0], runtime.BuildSpec(noCache)) {
			t.Fatalf("Build(%v) sent %+v", noCache, engine.builds)
		}
	}
}
````

### 5.4.9 `internal/cli/root_test.go` (changed file)

````go
package cli

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/app"
)

// fakeService records the requests the CLI produces.
type fakeService struct {
	runs   []app.Request
	builds []bool
	err    error
}

func (f *fakeService) Run(_ context.Context, request app.Request) error {
	f.runs = append(f.runs, request)
	return f.err
}

func (f *fakeService) Build(_ context.Context, noCache bool) error {
	f.builds = append(f.builds, noCache)
	return f.err
}

// execute runs the root command with args and returns its stdout.
func execute(t *testing.T, service *fakeService, args ...string) (string, error) {
	t.Helper()
	cmd := NewRootCmd("test", agent.Builtin(), service)
	var stdout bytes.Buffer
	cmd.SetArgs(args)
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	return stdout.String(), err
}

func TestRootCmdRejectsTooManyArguments(t *testing.T) {
	t.Parallel()

	service := &fakeService{}
	_, err := execute(t, service, "claude", "prompt one", "extra")
	if err == nil {
		t.Fatal("Execute() error = nil, want an error for too many arguments")
	}
	if !strings.Contains(err.Error(), "accepts at most 2 arg") {
		t.Errorf("Execute() error = %q, want a too-many-args error", err)
	}
	if len(service.runs) != 0 {
		t.Fatal("service ran despite invalid arguments")
	}
}

func TestBuildRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		flags      app.Request
		args       []string
		wantAgent  string
		wantPrompt string
		wantErr    string
	}{
		{name: "shell", args: nil},
		{name: "agent", args: []string{"claude"}, wantAgent: "claude"},
		{name: "agent and prompt", args: []string{"codex", "fix it"}, wantAgent: "codex", wantPrompt: "fix it"},
		{name: "unsupported", args: []string{"foo"}, wantErr: "unsupported agent \"foo\""},
		{name: "invalid env", flags: app.Request{EnvVars: []string{"bad-name"}}, args: []string{"claude"}, wantErr: `invalid env name "bad-name"`},
		{name: "unsupported before invalid env", flags: app.Request{EnvVars: []string{"bad-name"}}, args: []string{"foo"}, wantErr: "unsupported agent"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			request, err := buildRequest(agent.Builtin(), tt.flags, tt.args)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("buildRequest(%v) error = %v, want %q", tt.args, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("buildRequest(%v) error = %v", tt.args, err)
			}
			if (request.Agent == nil) != (tt.wantAgent == "") || (request.Agent != nil && request.Agent.Name != tt.wantAgent) {
				t.Fatalf("buildRequest(%v) agent = %+v, want %q", tt.args, request.Agent, tt.wantAgent)
			}
			if (request.Prompt == nil) != (tt.wantPrompt == "") || (request.Prompt != nil && *request.Prompt != tt.wantPrompt) {
				t.Fatalf("buildRequest(%v) prompt = %v, want %q", tt.args, request.Prompt, tt.wantPrompt)
			}
		})
	}
}

func TestRootCmdPassesFlagsToService(t *testing.T) {
	t.Parallel()

	service := &fakeService{}
	_, err := execute(t, service, "--no-config", "--verbose", "--allow-broad-mount", "-w", "/work", "--env", "A", "--env", "B", "pi", "hi")
	if err != nil {
		t.Fatal(err)
	}
	if len(service.runs) != 1 {
		t.Fatalf("runs = %+v, want one", service.runs)
	}
	got := service.runs[0]
	pi, err := agent.Builtin().Lookup("pi")
	if err != nil {
		t.Fatal(err)
	}
	prompt := "hi"
	want := app.Request{Agent: &pi, Prompt: &prompt, NoConfig: true, Verbose: true, AllowBroadMount: true, WorkDir: "/work", EnvVars: []string{"A", "B"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("request = %+v, want %+v", got, want)
	}
}

func TestAgentNamesDoNotCollideWithSubcommands(t *testing.T) {
	t.Parallel()

	subcommands := map[string]struct{}{"build": {}, "list": {}, "help": {}, "completion": {}}
	for _, name := range agent.Builtin().Names() {
		if _, ok := subcommands[name]; ok {
			t.Errorf("agent %q collides with a root subcommand", name)
		}
	}
}

func TestInvalidEnvFlagRejected(t *testing.T) {
	t.Parallel()

	service := &fakeService{}
	_, err := execute(t, service, "claude", "--env", "bad-name")
	if err == nil || !strings.Contains(err.Error(), "invalid env name") {
		t.Fatalf("Execute() error = %v, want invalid env name rejection", err)
	}
	if len(service.runs) != 0 {
		t.Fatal("service ran despite an invalid env name")
	}
}

func TestBuildCommand(t *testing.T) {
	t.Parallel()

	service := &fakeService{}
	stdout, err := execute(t, service, "build", "--no-cache")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(service.builds, []bool{true}) || stdout != "built runtime image caged-runtime:latest\n" {
		t.Fatalf("builds = %v, stdout = %q", service.builds, stdout)
	}

	failing := &fakeService{err: errors.New("build failed")}
	if stdout, err := execute(t, failing, "build"); err == nil || err.Error() != "build failed" || stdout != "" {
		t.Fatalf("failed build: stdout = %q, err = %v", stdout, err)
	}
}

func TestListCommandUsesInjectedAgents(t *testing.T) {
	t.Parallel()

	agents, err := agent.NewRegistry(
		agent.Agent{Name: "zeta", Command: []string{"zeta"}},
		agent.Agent{Name: "alpha", Command: []string{"alpha"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	cmd := NewRootCmd("test", agents, &fakeService{})
	var stdout bytes.Buffer
	cmd.SetArgs([]string{"list"})
	cmd.SetOut(&stdout)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "alpha\nzeta\n" {
		t.Fatalf("list output = %q", stdout.String())
	}
}
````

### 5.4.10 `internal/docker/args_test.go` (changed file)

````go
package docker

import (
	"reflect"
	"testing"

	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/runtime"
	"github.com/JeremyOrr4/caged/internal/system"
)

func TestRunArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		options plan.RunSpec
		goos    string
		uid     int
		gid     int
		want    []string
	}{
		{
			name: "no mount on macOS",
			options: plan.RunSpec{
				Image:   "ubuntu:24.04",
				Command: []string{"bash"},
			},
			goos: "darwin",
			want: []string{"run", "-it", "--rm", "ubuntu:24.04", "bash"},
		},
		{
			name: "mount with spaces on macOS",
			options: plan.RunSpec{
				Image:   "ubuntu:24.04",
				Command: []string{"bash"},
				Mount: &plan.Mount{
					HostPath:      "/tmp/project with spaces",
					ContainerPath: "/workspace",
				},
			},
			goos: "darwin",
			want: []string{
				"run", "-it", "--rm",
				"-v", "/tmp/project with spaces:/workspace",
				"-w", "/workspace",
				"ubuntu:24.04", "bash",
			},
		},
		{
			name: "mount on Linux maps host user",
			options: plan.RunSpec{
				Image: "ubuntu:24.04",
				Mount: &plan.Mount{HostPath: "/work", ContainerPath: "/workspace"},
			},
			goos: "linux",
			uid:  1001,
			gid:  1002,
			want: []string{
				"run", "-it", "--rm",
				"-v", "/work:/workspace",
				"-w", "/workspace",
				"--user", "1001:1002",
				"ubuntu:24.04",
			},
		},
		{
			name: "agent command in runtime image",
			options: plan.RunSpec{
				Image:         runtime.Image,
				Command:       []string{"claude"},
				Mount:         &plan.Mount{HostPath: "/project", ContainerPath: "/workspace"},
				ReturnToShell: true,
			},
			goos: "linux",
			uid:  2001,
			gid:  2002,
			want: []string{
				"run", "-it", "--rm",
				"-v", "/project:/workspace",
				"-w", "/workspace",
				"--user", "2001:2002",
				"-e", "CAGED_RETURN_TO_SHELL=1",
				runtime.Image,
				"claude",
			},
		},
		{
			name: "config read-write mount and env names",
			options: plan.RunSpec{
				Image:         runtime.Image,
				Command:       []string{"claude"},
				EnvVars:       []string{"ANTHROPIC_API_KEY", "GEMINI_API_KEY"},
				ReturnToShell: true,
				ConfigMounts: []plan.ConfigMount{{
					Agent:    "claude",
					Name:     ".claude",
					HostPath: "/home/test/.claude",
					DestPath: "/home/caged/.claude",
					Writable: true,
				}},
			},
			goos: "linux",
			uid:  3001,
			gid:  3002,
			want: []string{
				"run", "-it", "--rm",
				"-v", "/home/test/.claude:/home/caged/.claude",
				"--user", "3001:3002",
				"-e", "ANTHROPIC_API_KEY",
				"-e", "GEMINI_API_KEY",
				"-e", "CAGED_RETURN_TO_SHELL=1",
				runtime.Image,
				"claude",
			},
		},
		{
			name: "Pi config mounts directly read-write",
			options: plan.RunSpec{
				Image: runtime.Image,
				ConfigMounts: []plan.ConfigMount{{
					Agent:    "pi",
					Name:     ".pi",
					HostPath: "/home/test/.pi",
					DestPath: "/home/caged/.pi",
					Writable: true,
				}},
			},
			goos: "darwin",
			uid:  501,
			gid:  20,
			want: []string{
				"run", "-it", "--rm",
				"-v", "/home/test/.pi:/home/caged/.pi",
				runtime.Image,
			},
		},
		{
			name: "config mount uses image user on macOS",
			options: plan.RunSpec{
				Image: runtime.Image,
				ConfigMounts: []plan.ConfigMount{{
					HostPath: "/tmp/caged-config",
					DestPath: "/home/caged/.codex",
					Writable: true,
				}},
			},
			goos: "darwin",
			uid:  501,
			gid:  20,
			want: []string{
				"run", "-it", "--rm",
				"-v", "/tmp/caged-config:/home/caged/.codex",
				runtime.Image,
			},
		},
		{
			name: "Pi config is writable",
			options: plan.RunSpec{
				Image: runtime.Image,
				ConfigMounts: []plan.ConfigMount{{
					Agent:    "pi",
					Name:     ".pi",
					HostPath: "/home/test/.pi",
					DestPath: "/home/caged/.pi",
					Writable: true,
				}},
			},
			goos: "darwin",
			uid:  501,
			gid:  20,
			want: []string{
				"run", "-it", "--rm",
				"-v", "/home/test/.pi:/home/caged/.pi",
				runtime.Image,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := runArgs(tt.options, system.Platform{OS: tt.goos, UID: tt.uid, GID: tt.gid})
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("runArgs() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRunArgsExposeOnlyPlannedMounts(t *testing.T) {
	t.Parallel()

	options := plan.RunSpec{
		Image: runtime.Image,
		Mount: &plan.Mount{HostPath: "/projects/current", ContainerPath: "/workspace/current"},
		ConfigMounts: []plan.ConfigMount{
			{HostPath: "/home/user/.claude", DestPath: "/home/caged/.claude", Writable: true},
			{HostPath: "/home/user/.gitconfig", DestPath: "/home/caged/.gitconfig", Writable: false},
		},
		Command: []string{"bash"},
	}
	args := runArgs(options, system.Platform{OS: "darwin"})
	volumeCount := 0
	for _, arg := range args {
		if arg == "-v" {
			volumeCount++
		}
		if arg == "--privileged" || arg == "/var/run/docker.sock" {
			t.Fatalf("unexpected privileged host access in Docker args: %q", arg)
		}
	}
	if want := 1 + len(options.ConfigMounts); volumeCount != want {
		t.Fatalf("Docker args contain %d volume mounts, want only the %d planned mounts: %q", volumeCount, want, args)
	}
}

func TestRunArgsMaskCodexRuntimeDirectories(t *testing.T) {
	t.Parallel()

	options := plan.RunSpec{Image: runtime.Image, ConfigMounts: []plan.ConfigMount{{
		HostPath: "/home/user/.codex", DestPath: "/home/caged/.codex", Writable: true,
		MaskPaths: []string{"app-server-control", "ipc"},
	}}}
	args := runArgs(options, system.Platform{OS: "darwin"})
	want := []string{"--tmpfs", "/home/caged/.codex/app-server-control", "--tmpfs", "/home/caged/.codex/ipc"}
	for i := 0; i < len(want); i += 2 {
		found := false
		for j := 0; j+1 < len(args); j++ {
			if args[j] == want[i] && args[j+1] == want[i+1] {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Docker args do not contain %q %q: %q", want[i], want[i+1], args)
		}
	}
}
````

### 5.4.11 `internal/docker/client_test.go` (new file)

````go
package docker

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
)

// fakeExecutor records commands instead of running them.
type fakeExecutor struct {
	lookPathErr error
	// lookPathFailAfter makes LookPath fail once it has succeeded this many times (0 = never).
	lookPathFailAfter int
	lookPathCalls     int
	// run answers a command; nil means success.
	run      func(system.Command) error
	commands []system.Command
}

func (f *fakeExecutor) LookPath(file string) (string, error) {
	f.lookPathCalls++
	if f.lookPathErr != nil || (f.lookPathFailAfter > 0 && f.lookPathCalls > f.lookPathFailAfter) {
		return "", errors.New(`exec: "` + file + `": executable file not found in $PATH`)
	}
	return "/usr/local/bin/" + file, nil
}

func (f *fakeExecutor) Run(_ context.Context, cmd system.Command) error {
	f.commands = append(f.commands, cmd)
	if f.run != nil {
		return f.run(cmd)
	}
	return nil
}

// fakeFS records the temporary build context instead of touching the disk.
type fakeFS struct {
	mkdirErr error
	writeErr map[string]error // by file name
	written  map[string]writtenFile
	removed  []string
}

type writtenFile struct {
	data string
	mode fs.FileMode
}

func (f *fakeFS) MkdirTemp(dir, pattern string) (string, error) {
	if f.mkdirErr != nil {
		return "", f.mkdirErr
	}
	return filepath.Join("/tmp", pattern+"123"), nil
}

func (f *fakeFS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	if err := f.writeErr[name]; err != nil {
		return err
	}
	if f.written == nil {
		f.written = map[string]writtenFile{}
	}
	f.written[name] = writtenFile{data: string(data), mode: perm}
	return nil
}

func (f *fakeFS) RemoveAll(path string) error {
	f.removed = append(f.removed, path)
	return nil
}

var (
	linux  = system.Platform{OS: "linux", Arch: "amd64", UID: 1000, GID: 1001}
	darwin = system.Platform{OS: "darwin", Arch: "arm64", UID: 501, GID: 20}
)

func TestCheckRunsDockerInfo(t *testing.T) {
	t.Parallel()

	exec := &fakeExecutor{}
	if err := NewClient(exec, &fakeFS{}, darwin, system.Stdio{}).Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(exec.commands) != 1 {
		t.Fatalf("commands = %+v, want one docker info", exec.commands)
	}
	info := exec.commands[0]
	if info.Path != "/usr/local/bin/docker" || !reflect.DeepEqual(info.Args, []string{"info", "--format", "{{.ServerVersion}}"}) {
		t.Fatalf("command = %+v", info)
	}
	// Combined output: stdout and stderr share one buffer, like exec.Cmd.CombinedOutput.
	if info.Stdout == nil || info.Stdout != info.Stderr || info.Stdin != nil {
		t.Fatalf("docker info must capture combined output and read no input: %+v", info)
	}
}

func TestCheckErrors(t *testing.T) {
	t.Parallel()

	daemonDown := func(cmd system.Command) error {
		io.WriteString(cmd.Stdout, "Cannot connect to the Docker daemon\n")
		return errors.New("exit status 1")
	}
	tests := []struct {
		name     string
		platform system.Platform
		exec     *fakeExecutor
		want     string
	}{
		{name: "unsupported arch", platform: system.Platform{OS: "linux", Arch: "386"}, exec: &fakeExecutor{}, want: `unsupported host architecture "386"; caged supports amd64 and arm64 images`},
		{name: "missing CLI", platform: linux, exec: &fakeExecutor{lookPathErr: errors.New("x")}, want: `Docker is not installed or not on PATH: exec: "docker": executable file not found in $PATH`},
		{name: "daemon down on Linux", platform: linux, exec: &fakeExecutor{run: daemonDown}, want: "Docker is installed but the daemon is unavailable; start the Docker daemon: Cannot connect to the Docker daemon"},
		{name: "daemon down on macOS", platform: darwin, exec: &fakeExecutor{run: daemonDown}, want: "Docker is installed but the daemon is unavailable; start Docker Desktop: Cannot connect to the Docker daemon"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := NewClient(tt.exec, &fakeFS{}, tt.platform, system.Stdio{}).Check(context.Background())
			if err == nil || err.Error() != tt.want {
				t.Fatalf("Check() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestImageExists(t *testing.T) {
	t.Parallel()

	present := &fakeExecutor{}
	if !NewClient(present, &fakeFS{}, linux, system.Stdio{}).ImageExists(context.Background(), "caged-runtime:latest") {
		t.Fatal("ImageExists() = false, want true")
	}
	want := system.Command{Path: "docker", Args: []string{"image", "inspect", "caged-runtime:latest"}}
	if !reflect.DeepEqual(present.commands, []system.Command{want}) {
		t.Fatalf("commands = %+v, want %+v", present.commands, want)
	}
	missing := &fakeExecutor{run: func(system.Command) error { return errors.New("exit status 1") }}
	if NewClient(missing, &fakeFS{}, linux, system.Stdio{}).ImageExists(context.Background(), "caged-runtime:latest") {
		t.Fatal("ImageExists() = true, want false")
	}
}

func TestRunConnectsStdioAndPassesArgs(t *testing.T) {
	t.Parallel()

	stdin := strings.NewReader("")
	var stdout, stderr bytes.Buffer
	exec := &fakeExecutor{run: func(cmd system.Command) error {
		if cmd.Args[0] == "run" {
			io.WriteString(cmd.Stderr, "container stderr\n")
		}
		return nil
	}}
	spec := plan.RunSpec{Image: "img", Command: []string{"claude"}, ReturnToShell: true}
	if err := NewClient(exec, &fakeFS{}, linux, system.Stdio{In: stdin, Out: &stdout, Err: &stderr}).Run(context.Background(), spec); err != nil {
		t.Fatal(err)
	}
	if len(exec.commands) != 2 {
		t.Fatalf("commands = %+v, want docker info then docker run", exec.commands)
	}
	run := exec.commands[1]
	wantArgs := []string{"run", "-it", "--rm", "--user", "1000:1001", "-e", "CAGED_RETURN_TO_SHELL=1", "img", "claude"}
	if run.Path != "/usr/local/bin/docker" || !reflect.DeepEqual(run.Args, wantArgs) {
		t.Fatalf("run = %+v, want args %q", run, wantArgs)
	}
	if run.Stdin != stdin || run.Stdout != &stdout {
		t.Fatal("docker run is not connected to the client's stdin and stdout")
	}
	if stderr.String() != "container stderr\n" {
		t.Fatalf("stderr = %q, want the container's stderr passed through", stderr.String())
	}
}

func TestRunErrors(t *testing.T) {
	t.Parallel()

	exitErr := errors.New("exit status 125")
	mountsDenied := func(cmd system.Command) error {
		if cmd.Args[0] != "run" {
			return nil
		}
		io.WriteString(cmd.Stderr, "docker: Error response from daemon: Mounts denied: path not shared\n")
		return exitErr
	}
	t.Run("linux returns the exit error unchanged", func(t *testing.T) {
		t.Parallel()
		err := NewClient(&fakeExecutor{run: mountsDenied}, &fakeFS{}, linux, system.Stdio{Err: io.Discard}).Run(context.Background(), plan.RunSpec{})
		if err != exitErr {
			t.Fatalf("Run() error = %v, want the original exit error", err)
		}
	})
	t.Run("macOS explains Docker Desktop file sharing", func(t *testing.T) {
		t.Parallel()
		err := NewClient(&fakeExecutor{run: mountsDenied}, &fakeFS{}, darwin, system.Stdio{Err: io.Discard}).Run(context.Background(), plan.RunSpec{})
		want := "Docker could not mount the project path; in Docker Desktop, add its parent directory under Settings > Resources > File Sharing: exit status 125"
		if err == nil || err.Error() != want || !errors.Is(err, exitErr) {
			t.Fatalf("Run() error = %v, want %q wrapping the exit error", err, want)
		}
	})
	t.Run("check failure stops before running", func(t *testing.T) {
		t.Parallel()
		exec := &fakeExecutor{lookPathErr: errors.New("x")}
		if err := NewClient(exec, &fakeFS{}, linux, system.Stdio{}).Run(context.Background(), plan.RunSpec{}); err == nil || !strings.HasPrefix(err.Error(), "Docker is not installed") {
			t.Fatalf("Run() error = %v", err)
		}
		if len(exec.commands) != 0 {
			t.Fatalf("commands = %+v, want none", exec.commands)
		}
	})
	t.Run("docker disappears after the check", func(t *testing.T) {
		t.Parallel()
		exec := &fakeExecutor{lookPathFailAfter: 1}
		err := NewClient(exec, &fakeFS{}, linux, system.Stdio{}).Run(context.Background(), plan.RunSpec{})
		if err == nil || err.Error() != `docker not found, is Docker installed and running?: exec: "docker": executable file not found in $PATH` {
			t.Fatalf("Run() error = %v", err)
		}
	})
}

func TestPlatformRules(t *testing.T) {
	t.Parallel()

	if got := userArgs(linux); !reflect.DeepEqual(got, []string{"--user", "1000:1001"}) {
		t.Errorf("userArgs(linux) = %q", got)
	}
	if got := userArgs(darwin); got != nil {
		t.Errorf("userArgs(darwin) = %q, want none: Docker Desktop maps ownership", got)
	}
	if got := daemonLauncher(darwin); got != "start Docker Desktop" {
		t.Errorf("daemonLauncher(darwin) = %q", got)
	}
	if got := daemonLauncher(linux); got != "start the Docker daemon" {
		t.Errorf("daemonLauncher(linux) = %q", got)
	}
	for _, arch := range []string{"amd64", "arm64"} {
		if err := checkArch(system.Platform{Arch: arch}); err != nil {
			t.Errorf("checkArch(%s) = %v", arch, err)
		}
	}
	for _, message := range []string{"Mounts denied", "path is NOT SHARED from the host", "mount denied"} {
		if !looksLikeSharedPathError(message) {
			t.Errorf("looksLikeSharedPathError(%q) = false", message)
		}
	}
	plain := errors.New("exit status 1")
	if got := explainRunError(darwin, "some other failure", plain); got != plain {
		t.Errorf("explainRunError() = %v, want the error unchanged", got)
	}
}

func testBuildSpec(noCache bool) plan.BuildSpec {
	return plan.BuildSpec{
		Image:      "caged-runtime:latest",
		Dockerfile: "Dockerfile",
		NoCache:    noCache,
		Files: []plan.File{
			{Name: "Dockerfile", Label: "Dockerfile", Data: []byte("FROM scratch\n"), Mode: 0o644},
			{Name: "caged-entrypoint.sh", Label: "entrypoint", Data: []byte("#!/bin/sh\n"), Mode: 0o755},
		},
	}
}

func TestBuildWritesContextRunsDockerBuildAndCleansUp(t *testing.T) {
	t.Parallel()

	for _, noCache := range []bool{false, true} {
		exec, fsys := &fakeExecutor{}, &fakeFS{}
		stdio := system.Stdio{In: strings.NewReader(""), Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
		if err := NewClient(exec, fsys, linux, stdio).Build(context.Background(), testBuildSpec(noCache)); err != nil {
			t.Fatal(err)
		}
		wantFiles := map[string]writtenFile{
			"/tmp/caged-runtime-123/Dockerfile":          {data: "FROM scratch\n", mode: 0o644},
			"/tmp/caged-runtime-123/caged-entrypoint.sh": {data: "#!/bin/sh\n", mode: 0o755},
		}
		if !reflect.DeepEqual(fsys.written, wantFiles) {
			t.Fatalf("written = %+v, want %+v", fsys.written, wantFiles)
		}
		if len(exec.commands) != 2 || exec.commands[0].Args[0] != "info" {
			t.Fatalf("commands = %+v, want docker info then docker build", exec.commands)
		}
		build := exec.commands[1]
		wantArgs := []string{"build", "-t", "caged-runtime:latest", "-f", "/tmp/caged-runtime-123/Dockerfile", "/tmp/caged-runtime-123"}
		if noCache {
			wantArgs = []string{"build", "-t", "caged-runtime:latest", "--no-cache", "-f", "/tmp/caged-runtime-123/Dockerfile", "/tmp/caged-runtime-123"}
		}
		if build.Path != "/usr/local/bin/docker" || !reflect.DeepEqual(build.Args, wantArgs) {
			t.Fatalf("build = %+v, want args %q", build, wantArgs)
		}
		if build.Stdin != stdio.In || build.Stdout != stdio.Out || build.Stderr != stdio.Err {
			t.Fatal("docker build is not connected to the client's stdio")
		}
		if !reflect.DeepEqual(fsys.removed, []string{"/tmp/caged-runtime-123"}) {
			t.Fatalf("removed = %v, want the build directory", fsys.removed)
		}
	}
}

func TestBuildErrors(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")
	t.Run("temporary directory", func(t *testing.T) {
		t.Parallel()
		exec := &fakeExecutor{}
		err := NewClient(exec, &fakeFS{mkdirErr: boom}, linux, system.Stdio{}).Build(context.Background(), testBuildSpec(false))
		if err == nil || err.Error() != "create temporary build directory: boom" || len(exec.commands) != 1 {
			t.Fatalf("Build() error = %v, commands = %+v", err, exec.commands)
		}
	})
	t.Run("write uses the file label", func(t *testing.T) {
		t.Parallel()
		fsys := &fakeFS{writeErr: map[string]error{"/tmp/caged-runtime-123/caged-entrypoint.sh": boom}}
		err := NewClient(&fakeExecutor{}, fsys, linux, system.Stdio{}).Build(context.Background(), testBuildSpec(false))
		if err == nil || err.Error() != "write embedded entrypoint: boom" || len(fsys.removed) != 1 {
			t.Fatalf("Build() error = %v, removed = %v", err, fsys.removed)
		}
	})
	t.Run("docker build failure is returned unchanged", func(t *testing.T) {
		t.Parallel()
		fsys := &fakeFS{}
		exec := &fakeExecutor{run: func(cmd system.Command) error {
			if cmd.Args[0] == "build" {
				return boom
			}
			return nil
		}}
		if err := NewClient(exec, fsys, linux, system.Stdio{}).Build(context.Background(), testBuildSpec(false)); err != boom || len(fsys.removed) != 1 {
			t.Fatalf("Build() error = %v, removed = %v", err, fsys.removed)
		}
	})
	t.Run("check failure creates nothing", func(t *testing.T) {
		t.Parallel()
		fsys := &fakeFS{}
		err := NewClient(&fakeExecutor{lookPathErr: boom}, fsys, linux, system.Stdio{}).Build(context.Background(), testBuildSpec(false))
		if err == nil || !strings.HasPrefix(err.Error(), "Docker is not installed or not on PATH") || fsys.written != nil || fsys.removed != nil {
			t.Fatalf("Build() error = %v, fs = %+v", err, fsys)
		}
	})
	t.Run("docker disappears after the check", func(t *testing.T) {
		t.Parallel()
		err := NewClient(&fakeExecutor{lookPathFailAfter: 1}, &fakeFS{}, linux, system.Stdio{}).Build(context.Background(), testBuildSpec(false))
		if err == nil || err.Error() != `Docker is not installed or not on PATH: exec: "docker": executable file not found in $PATH` {
			t.Fatalf("Build() error = %v", err)
		}
	})
}
````

### 5.4.12 `internal/hostconfig/resolver_edges_test.go` (new file)

````go
package hostconfig

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
)

// faultyFS is the real filesystem with selected calls failing.
type faultyFS struct {
	system.FS
	evalErr    map[string]bool
	lstatErr   map[string]bool
	readDirErr map[string]bool
}

var errInjected = errors.New("injected failure")

func (f faultyFS) EvalSymlinks(path string) (string, error) {
	if f.evalErr[path] {
		return "", errInjected
	}
	return f.FS.EvalSymlinks(path)
}

func (f faultyFS) Lstat(name string) (fs.FileInfo, error) {
	if f.lstatErr[name] {
		return nil, errInjected
	}
	return f.FS.Lstat(name)
}

func (f faultyFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if f.readDirErr[name] {
		return nil, errInjected
	}
	return f.FS.ReadDir(name)
}

// testHome creates a home directory and returns it with its resolved path
// (they differ on macOS, where temp dirs live behind the /var -> /private/var symlink).
func testHome(t *testing.T, dirs, files []string) (string, string) {
	t.Helper()
	home := t.TempDir()
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(home, dir), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range files {
		if err := os.WriteFile(filepath.Join(home, file), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	resolved, err := filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatal(err)
	}
	return home, resolved
}

func TestResolveRefusesUnresolvableHome(t *testing.T) {
	t.Parallel()

	fsys := faultyFS{evalErr: map[string]bool{"/no/such/home": true}}
	got := NewResolver(fsys, fakeEnv{}, agent.Builtin().All(), DefaultSharedFiles()).Resolve("/no/such/home/")
	want := plan.ConfigReport{Refused: []string{"/no/such/home (home cannot be resolved)"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Resolve() = %+v, want %+v", got, want)
	}
}

func TestResolveReportsMissingPathsInRegistryOrder(t *testing.T) {
	t.Parallel()

	// The six original agents, so that adding a built-in agent does not change this list.
	var agents []agent.Agent
	for _, name := range []string{"claude", "codex", "copilot", "gemini", "gh", "pi"} {
		definition, err := agent.Builtin().Lookup(name)
		if err != nil {
			t.Fatal(err)
		}
		agents = append(agents, definition)
	}
	home, _ := testHome(t, nil, nil)
	report := NewResolver(system.FS{}, fakeEnv{}, agents, DefaultSharedFiles()).Resolve(home)
	var want []string
	for _, rel := range []string{".claude", ".codex", ".copilot", ".gemini", ".config/gh", ".config/pi", ".pi", ".gitconfig"} {
		want = append(want, filepath.Join(home, rel))
	}
	if !reflect.DeepEqual(report.Missing, want) || report.Mounts != nil || report.Refused != nil {
		t.Fatalf("Resolve() = %+v, want only Missing %v", report, want)
	}
}

func TestResolveHonorsOverrides(t *testing.T) {
	t.Parallel()

	home, resolvedHome := testHome(t, []string{"alt-codex", "abs-copilot", ".claude"}, nil)
	report := resolveConfigs(home, fakeEnv{
		"CODEX_HOME":        "alt-codex",                        // relative to home
		"COPILOT_HOME":      filepath.Join(home, "abs-copilot"), // absolute
		"CLAUDE_CONFIG_DIR": "",                                 // empty means "use the default"
	})
	got := map[string]string{}
	for _, mount := range report.Mounts {
		got[mount.HostPath] = mount.DestPath
	}
	want := map[string]string{
		filepath.Join(resolvedHome, "alt-codex"):   "/home/caged/.codex",
		filepath.Join(resolvedHome, "abs-copilot"): "/home/caged/.copilot",
		filepath.Join(resolvedHome, ".claude"):     "/home/caged/.claude",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mounts = %v, want %v", got, want)
	}
}

func TestResolveRefusesUnresolvableConfigPath(t *testing.T) {
	t.Parallel()

	home, _ := testHome(t, []string{".gemini"}, nil)
	gemini := filepath.Join(home, ".gemini")
	fsys := faultyFS{evalErr: map[string]bool{gemini: true}}
	report := NewResolver(fsys, fakeEnv{}, agent.Builtin().All(), nil).Resolve(home)
	if want := []string{gemini + " (unresolvable symlink)"}; !reflect.DeepEqual(report.Refused, want) {
		t.Fatalf("Refused = %v, want %v", report.Refused, want)
	}
}

func TestResolveReportsUninspectableEntries(t *testing.T) {
	t.Parallel()

	home, resolvedHome := testHome(t, []string{".gemini/sub", ".gh"}, []string{".gemini/sub/a.json", ".gemini/b.json"})
	if err := os.Symlink("b.json", filepath.Join(home, ".gemini", "link")); err != nil {
		t.Fatal(err)
	}
	gemini := filepath.Join(resolvedHome, ".gemini")
	fsys := faultyFS{
		readDirErr: map[string]bool{filepath.Join(gemini, "sub"): true},
		lstatErr:   map[string]bool{filepath.Join(gemini, "b.json"): true},
	}
	report := NewResolver(fsys, fakeEnv{}, agent.Builtin().All(), nil).Resolve(home)
	wantRefused := []string{
		filepath.Join(gemini, "b.json") + " (cannot inspect config entry safely)",
		filepath.Join(gemini, "link") + " (symlinks and special files are excluded from config mounts)",
		filepath.Join(gemini, "sub") + " (cannot inspect config entry safely)",
	}
	if !reflect.DeepEqual(report.Refused, wantRefused) || len(report.Mounts) != 0 {
		t.Fatalf("Resolve() = %+v, want refused %v and no mounts", report, wantRefused)
	}

	root := filepath.Join(resolvedHome, ".gh")
	if mounts, refused := collectSafeMounts(faultyFS{lstatErr: map[string]bool{root: true}}, root); mounts != nil || !reflect.DeepEqual(refused, []string{root + " (cannot inspect config root safely)"}) {
		t.Fatalf("collectSafeMounts() = %v, %v", mounts, refused)
	}
}

func TestResolveMountsSingleFileConfigAndRejectsBadMasks(t *testing.T) {
	t.Parallel()

	home, resolvedHome := testHome(t, []string{"masked"}, []string{"tool.json", "masked-file"})
	agents := []agent.Agent{
		{Name: "file", Command: []string{"file"}, ConfigPaths: []agent.ConfigPath{{HostPath: "tool.json", ContainerPath: "tool.json"}}},
		{Name: "escape", Command: []string{"escape"}, ConfigPaths: []agent.ConfigPath{{HostPath: "masked", ContainerPath: "masked", MaskPaths: []string{"../outside"}}}},
		{Name: "notdir", Command: []string{"notdir"}, ConfigPaths: []agent.ConfigPath{{HostPath: "masked-file", ContainerPath: "masked-file", MaskPaths: []string{"tmp"}}}},
	}
	report := NewResolver(system.FS{}, fakeEnv{}, agents, nil).Resolve(home)
	wantMounts := []plan.ConfigMount{{Agent: "file", Name: "tool.json", HostPath: filepath.Join(resolvedHome, "tool.json"), DestPath: "/home/caged/tool.json"}}
	if !reflect.DeepEqual(report.Mounts, wantMounts) {
		t.Fatalf("Mounts = %+v, want %+v", report.Mounts, wantMounts)
	}
	wantRefused := []string{
		filepath.Join(home, "masked") + ` (invalid config mask path "../outside")`,
		filepath.Join(home, "masked-file") + " (masked config root must be a directory)",
	}
	if !reflect.DeepEqual(report.Refused, wantRefused) {
		t.Fatalf("Refused = %v, want %v", report.Refused, wantRefused)
	}
}

func TestResolveDeduplicatesSharedConfigPaths(t *testing.T) {
	t.Parallel()

	home, _ := testHome(t, []string{".shared"}, nil)
	path := agent.ConfigPath{HostPath: ".shared", ContainerPath: ".shared", Writable: true}
	agents := []agent.Agent{
		{Name: "one", Command: []string{"one"}, ConfigPaths: []agent.ConfigPath{path}},
		{Name: "two", Command: []string{"two"}, ConfigPaths: []agent.ConfigPath{path}},
	}
	report := NewResolver(system.FS{}, fakeEnv{}, agents, nil).Resolve(home)
	if len(report.Mounts) != 1 || report.Mounts[0].Agent != "one" {
		t.Fatalf("Mounts = %+v, want one mount owned by the first agent", report.Mounts)
	}
}

func TestSharedFilesAreData(t *testing.T) {
	t.Parallel()

	if got, want := DefaultSharedFiles(), []SharedFile{{Path: ".gitconfig", Description: "shared git config"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("DefaultSharedFiles() = %v, want %v", got, want)
	}

	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	home, resolvedHome := testHome(t, []string{".npmrc-dir"}, []string{".npmrc"})
	if err := os.Symlink(filepath.Join(outside, "secret"), filepath.Join(home, ".escape")); err != nil {
		t.Fatal(err)
	}
	shared := []SharedFile{
		{Path: ".npmrc", Description: "shared npm config"},
		{Path: ".npmrc-dir", Description: "shared npm config"},
		{Path: ".escape", Description: "shared escape"},
		{Path: ".absent", Description: "shared absent"},
	}
	report := NewResolver(system.FS{}, fakeEnv{}, nil, shared).Resolve(home)
	wantMounts := []plan.ConfigMount{{Agent: "shared", Name: ".npmrc", HostPath: filepath.Join(resolvedHome, ".npmrc"), DestPath: "/home/caged/.npmrc"}}
	if !reflect.DeepEqual(report.Mounts, wantMounts) {
		t.Fatalf("Mounts = %+v, want %+v", report.Mounts, wantMounts)
	}
	wantRefused := []string{
		filepath.Join(home, ".npmrc-dir") + " (shared npm config must be a regular file)",
		filepath.Join(home, ".escape") + " (resolves outside home or cannot be resolved)",
	}
	if !reflect.DeepEqual(report.Refused, wantRefused) {
		t.Fatalf("Refused = %v, want %v", report.Refused, wantRefused)
	}
	if want := []string{filepath.Join(home, ".absent")}; !reflect.DeepEqual(report.Missing, want) {
		t.Fatalf("Missing = %v, want %v", report.Missing, want)
	}
}

func TestIsWithin(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		path, root string
		want       bool
	}{
		{"/Users/me", "/Users/me", true},
		{"/Users/me/.codex", "/Users/me", true},
		{"/Users/me/../other", "/Users/me", false},
		{"/Users/meow", "/Users/me", false},
		{"/Users", "/Users/me", false},
		{"/Users/me/..hidden", "/Users/me", true},
	} {
		if got := isWithin(tt.path, tt.root); got != tt.want {
			t.Errorf("isWithin(%q, %q) = %v, want %v", tt.path, tt.root, got, tt.want)
		}
	}
	if strings.Contains(containerHome, "\\") {
		t.Fatal("container paths must use forward slashes")
	}
}
````

### 5.4.13 `internal/hostconfig/resolver_test.go` (changed file)

````go
package hostconfig

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
	"github.com/JeremyOrr4/caged/internal/plan"
	"github.com/JeremyOrr4/caged/internal/system"
)

// fakeEnv is a fixed environment for config-directory overrides.
type fakeEnv map[string]string

func (e fakeEnv) LookupEnv(key string) (string, bool) {
	value, ok := e[key]
	return value, ok
}

// resolveConfigs runs the production resolver on the real filesystem with the
// built-in agents and a controlled environment.
func resolveConfigs(home string, env fakeEnv) plan.ConfigReport {
	return NewResolver(system.FS{}, env, agent.Builtin().All(), DefaultSharedFiles()).Resolve(home)
}

func TestResolveAllConfigsMountsExistingConfigReadWrite(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	for _, dir := range []string{".claude", ".codex", ".gemini", ".pi/agent", ".config/pi", ".config/gh"} {
		if err := os.MkdirAll(filepath.Join(home, dir), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range []string{
		".claude/settings.json",
		".claude/.credentials.json",
		".claude.json",
		".codex/config.toml",
		".codex/auth.json",
		".gemini/settings.json",
		".gemini/oauth_creds.json",
		".pi/agent/settings.json",
		".pi/agent/auth.json",
		".config/gh/hosts.yml",
		".gitconfig",
	} {
		if err := os.WriteFile(filepath.Join(home, file), []byte("test"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	report := resolveConfigs(home, nil)
	defaultConfigs := report.Mounts
	defaultNames := make(map[string]struct{}, len(defaultConfigs))
	for _, mount := range defaultConfigs {
		defaultNames[mount.Name] = struct{}{}
	}
	for _, want := range []string{".claude", ".codex", ".gemini", ".pi", ".config/gh", ".gitconfig"} {
		if _, ok := defaultNames[want]; !ok {
			t.Errorf("default staged configs missing %q; got %v", want, defaultNames)
		}
	}
	for _, mount := range defaultConfigs {
		if mount.Name == ".gitconfig" && mount.Writable {
			t.Errorf("shared git config should mount read-only")
		}
		if mount.Name != ".gitconfig" && !mount.Writable {
			t.Errorf("config %q should mount read-write", mount.Name)
		}
	}

	missingHome := t.TempDir()
	if got := resolveConfigs(missingHome, nil).Mounts; len(got) != 0 {
		t.Errorf("Resolve() with no local configs = %v, want none", got)
	}
}

func TestConfigOverrideCannotMountHomeDirectory(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o700); err != nil {
		t.Fatal(err)
	}
	report := resolveConfigs(home, fakeEnv{"CODEX_HOME": home})
	for _, mount := range report.Mounts {
		if filepath.Clean(mount.HostPath) == filepath.Clean(home) {
			t.Fatal("agent config override mounted the entire home directory")
		}
	}
	foundRefusal := false
	for _, refusal := range report.Refused {
		if strings.Contains(refusal, "home directory") {
			foundRefusal = true
		}
	}
	if !foundRefusal {
		t.Fatalf("home-directory override was not reported as refused: %+v", report)
	}
}

func TestConfigSymlinkOutsideHomeIsRefused(t *testing.T) {
	t.Parallel()
	home, outside := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret"), []byte("no"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(home, ".codex")); err != nil {
		t.Fatal(err)
	}
	report := resolveConfigs(home, nil)
	for _, mount := range report.Mounts {
		if filepath.Clean(mount.HostPath) == filepath.Clean(outside) {
			t.Fatalf("mounted config symlink target outside home: %+v", mount)
		}
	}
	if len(report.Refused) == 0 {
		t.Fatalf("outside config symlink was not refused: %+v", report)
	}
}

func TestConfigSymlinkOutsideConfigDirectoryIsRefused(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	configDir := filepath.Join(home, ".codex")
	privateDir := filepath.Join(home, "private")
	if err := os.Mkdir(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(privateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(privateDir, "secret"), []byte("no"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(privateDir, filepath.Join(configDir, "private-link")); err != nil {
		t.Fatal(err)
	}
	report := resolveConfigs(home, nil)
	for _, mount := range report.Mounts {
		if mount.Name == ".codex" {
			t.Fatal("config tree with a symlink outside the config directory was mounted")
		}
	}
	if len(report.Refused) == 0 {
		t.Fatalf("config directory escape was not reported as refused: %+v", report)
	}
}

func TestSpecialFileInsideConfigTreeIsRefused(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	configDir := filepath.Join(home, ".gemini")
	if err := os.Mkdir(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(configDir, "agent.pipe"), 0o600); err != nil {
		t.Fatal(err)
	}
	report := resolveConfigs(home, nil)
	for _, mount := range report.Mounts {
		if mount.Name == ".gemini" {
			t.Fatal("config tree containing a socket was mounted")
		}
	}
	if len(report.Refused) == 0 {
		t.Fatalf("special file was not reported as refused: %+v", report)
	}
}

func TestCodexHomeMountPreservesDirectoryAndMasksRuntimeEntries(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	for _, dir := range []string{"app-server-control", "app-server-daemon", "ipc", "packages/app-server-daemon", "tmp/arg0"} {
		if err := os.MkdirAll(filepath.Join(codexHome, dir), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for name, contents := range map[string]string{
		"auth.json":   "credentials",
		"config.toml": "model = 'test'",
	} {
		if err := os.WriteFile(filepath.Join(codexHome, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := syscall.Mkfifo(filepath.Join(codexHome, "app-server-control", "daemon.sock"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/opt/homebrew/bin/codex", filepath.Join(codexHome, "packages/app-server-daemon/current")); err != nil {
		t.Fatal(err)
	}
	report := resolveConfigs(home, nil)
	var codexMount *plan.ConfigMount
	for _, mount := range report.Mounts {
		if mount.Agent == "codex" {
			codexMount = &mount
		}
	}
	if codexMount == nil {
		t.Fatalf("Codex home directory was not mounted: %+v", report)
	}
	resolvedCodexHome, err := filepath.EvalSymlinks(codexHome)
	if err != nil {
		t.Fatal(err)
	}
	if codexMount.HostPath != resolvedCodexHome || codexMount.DestPath != "/home/caged/.codex" {
		t.Fatalf("Codex config was not mounted as one directory: %+v", codexMount)
	}
	if !reflect.DeepEqual(codexMount.MaskPaths, []string{"app-server-control", "app-server-daemon", "ipc", "packages/app-server-daemon", "tmp/arg0"}) {
		t.Fatalf("Codex runtime mask paths = %v", codexMount.MaskPaths)
	}
	for _, refused := range report.Refused {
		if strings.Contains(refused, codexHome) {
			t.Fatalf("known masked Codex runtime entries were refused: %+v", report.Refused)
		}
	}
}

func TestSharedGitConfigMustBeRegularFile(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, ".gitconfig"), 0o700); err != nil {
		t.Fatal(err)
	}
	report := resolveConfigs(home, nil)
	for _, mount := range report.Mounts {
		if mount.Name == ".gitconfig" {
			t.Fatal("directory mounted as shared git config")
		}
	}
	if len(report.Refused) == 0 {
		t.Fatal("non-file gitconfig was not reported as refused")
	}
}
````

### 5.4.14 `internal/plan/run_test.go` (changed file)

````go
package plan

import (
	"reflect"
	"testing"

	"github.com/JeremyOrr4/caged/internal/agent"
)

func TestBuildRunSpecUsesAgentPromptStyle(t *testing.T) {
	for _, tc := range []struct {
		name  string
		agent string
		want  []string
	}{
		{"claude", "claude", []string{"claude", "hello $(world)"}},
		{"codex", "codex", []string{"codex", "exec", "hello $(world)"}},
		{"gemini", "gemini", []string{"gemini", "-i", "hello $(world)"}},
		{"pi", "pi", []string{"pi", "hello $(world)"}},
		{"copilot", "copilot", []string{"copilot", "-p", "hello $(world)"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			selected, err := agent.Builtin().Lookup(tc.agent)
			if err != nil {
				t.Fatal(err)
			}
			prompt := "hello $(world)"
			spec, err := BuildRunSpec("image", &selected, &prompt, nil, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(spec.Command, tc.want) {
				t.Fatalf("command = %q, want %q", spec.Command, tc.want)
			}
			if !spec.ReturnToShell {
				t.Fatal("agent runs must return to the container shell")
			}
		})
	}
}

func TestBuildRunSpecCopiesInputs(t *testing.T) {
	selected, _ := agent.Builtin().Lookup("claude")
	command := selected.Command[0]
	prompt := "prompt"
	mount := &Mount{HostPath: "/project", ContainerPath: "/workspace/project"}
	configMounts := []ConfigMount{{HostPath: "/home/user/.claude", DestPath: "/home/caged/.claude", Writable: true}}
	env := []string{"ANTHROPIC_API_KEY"}
	spec, err := BuildRunSpec("image", &selected, &prompt, mount, configMounts, env)
	if err != nil {
		t.Fatal(err)
	}
	spec.Command[0] = "changed"
	spec.ConfigMounts[0].Name = "changed"
	spec.EnvVars[0] = "changed"
	if selected.Command[0] != command || configMounts[0].Name == "changed" || env[0] == "changed" {
		t.Fatal("run spec mutated source inputs")
	}
}

func TestBuildRunSpecShellAndErrors(t *testing.T) {
	t.Parallel()

	mount := &Mount{HostPath: "/project", ContainerPath: "/workspace/project"}
	spec, err := BuildRunSpec("image", nil, nil, mount, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Image != "image" || !reflect.DeepEqual(spec.Command, []string{"bash"}) || spec.ReturnToShell {
		t.Fatalf("shell spec = %+v, want bash without return-to-shell", spec)
	}
	spec.Mount.HostPath = "changed"
	if mount.HostPath != "/project" {
		t.Fatal("run spec shares the project mount with its input")
	}

	prompt := "hello"
	if _, err := BuildRunSpec("image", nil, &prompt, nil, nil, nil); err == nil || err.Error() != "a prompt requires an agent" {
		t.Fatalf("shell with prompt error = %v", err)
	}
	gh, err := agent.Builtin().Lookup("gh")
	if err != nil {
		t.Fatal(err)
	}
	if spec, err := BuildRunSpec("image", &gh, &prompt, nil, nil, nil); err == nil || err.Error() != "gh does not support prompt launch" || spec.Image != "" {
		t.Fatalf("gh with prompt = %+v, %v; want an error and an empty spec", spec, err)
	}
}
````

### 5.4.15 `internal/project/mount_test.go` (new file)

````go
package project

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JeremyOrr4/caged/internal/system"
)

func TestNewMountGuardRails(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		cwd   string
		home  string
		allow bool
		want  string
	}{
		{name: "filesystem root", cwd: "/", home: "/Users/test", want: "filesystem root"},
		{name: "home directory", cwd: "/Users/test", home: "/Users/test", want: "entire home directory"},
		{name: "allowed home directory", cwd: "/Users/test", home: "/Users/test", allow: true},
		{name: "project directory", cwd: "/Users/test/code/caged", home: "/Users/test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mount, err := NewMount(tt.cwd, tt.home, tt.allow)
			if tt.want != "" {
				if err == nil || !strings.Contains(err.Error(), tt.want) {
					t.Fatalf("newMount() error = %v, want %q", err, tt.want)
				}
				return
			}
			if err != nil {
				t.Fatalf("newMount() error = %v", err)
			}
			wantPath := filepath.Join("/workspace", filepath.Base(filepath.Clean(tt.cwd)))
			if mount.ContainerPath != wantPath {
				t.Errorf("container path = %q, want %q", mount.ContainerPath, wantPath)
			}
		})
	}
}

func TestResolveProjectMount(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	projectPath := filepath.Join(t.TempDir(), "project")
	if err := os.Mkdir(projectPath, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(projectPath, "file.txt")
	if err := os.WriteFile(file, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	resolver := NewResolver(system.FS{})

	mount, err := resolver.Resolve(projectPath, home, false)
	if err != nil {
		t.Fatalf("resolveProjectMount() error = %v", err)
	}
	resolvedProject, err := filepath.EvalSymlinks(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if mount.HostPath != resolvedProject || mount.ContainerPath != "/workspace/project" {
		t.Fatalf("resolveProjectMount() = %+v, want host %q at /workspace/project", mount, resolvedProject)
	}

	if _, err := resolver.Resolve(file, home, false); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("resolveProjectMount(file) error = %v, want not-a-directory error", err)
	}

	if _, err := resolver.Resolve(home, home, false); err == nil || !strings.Contains(err.Error(), "entire home directory") {
		t.Fatalf("resolveProjectMount(home) error = %v, want broad-mount rejection", err)
	}
}

// fakeFS answers from maps. Paths missing from entries do not exist, and
// paths missing from resolved cannot be resolved.
type fakeFS struct {
	cwd      string
	entries  map[string]bool   // path -> is a directory
	resolved map[string]string // EvalSymlinks results
	absErr   error
}

func (f fakeFS) Abs(path string) (string, error) {
	if f.absErr != nil {
		return "", f.absErr
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	return filepath.Join(f.cwd, path), nil
}

func (f fakeFS) Stat(name string) (fs.FileInfo, error) {
	isDir, ok := f.entries[name]
	if !ok {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
	}
	return fakeInfo{name: filepath.Base(name), dir: isDir}, nil
}

func (f fakeFS) EvalSymlinks(path string) (string, error) {
	if resolved, ok := f.resolved[path]; ok {
		return resolved, nil
	}
	return "", &fs.PathError{Op: "lstat", Path: path, Err: fs.ErrNotExist}
}

type fakeInfo struct {
	name string
	dir  bool
}

func (i fakeInfo) Name() string       { return i.name }
func (i fakeInfo) Size() int64        { return 0 }
func (i fakeInfo) ModTime() time.Time { return time.Time{} }
func (i fakeInfo) IsDir() bool        { return i.dir }
func (i fakeInfo) Sys() any           { return nil }
func (i fakeInfo) Mode() fs.FileMode {
	if i.dir {
		return fs.ModeDir | 0o755
	}
	return 0o644
}

func TestResolverResolvesRelativePathsAndMacOSSymlinks(t *testing.T) {
	t.Parallel()

	// On macOS /tmp is a symlink to /private/tmp; the mount must use the real path.
	fsys := fakeFS{
		cwd:      "/tmp",
		entries:  map[string]bool{"/tmp/app": true},
		resolved: map[string]string{"/tmp/app": "/private/tmp/app", "/Users/dev": "/Users/dev"},
	}
	mount, err := NewResolver(fsys).Resolve("app", "/Users/dev", false)
	if err != nil {
		t.Fatal(err)
	}
	if mount.HostPath != "/private/tmp/app" || mount.ContainerPath != "/workspace/app" {
		t.Fatalf("Resolve() = %+v", mount)
	}
}

func TestResolverComparesHomeAfterResolvingSymlinks(t *testing.T) {
	t.Parallel()

	fsys := fakeFS{
		entries:  map[string]bool{"/home/dev": true},
		resolved: map[string]string{"/home/dev": "/data/home/dev"},
	}
	if _, err := NewResolver(fsys).Resolve("/home/dev", "/home/dev", false); err == nil || !strings.Contains(err.Error(), "entire home directory") {
		t.Fatalf("Resolve(home) error = %v, want broad-mount rejection", err)
	}
}

func TestResolverErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		fsys fakeFS
		want string
	}{
		{name: "abs fails", fsys: fakeFS{absErr: errors.New("boom")}, want: "resolve working directory: boom"},
		{name: "missing directory", fsys: fakeFS{}, want: `inspect working directory "/work/app": stat /work/app: file does not exist`},
		{name: "regular file", fsys: fakeFS{entries: map[string]bool{"/work/app": false}}, want: `working directory "/work/app" is not a directory`},
		{name: "unresolvable directory", fsys: fakeFS{entries: map[string]bool{"/work/app": true}}, want: "resolve working directory symlinks: lstat /work/app: file does not exist"},
		{
			name: "unresolvable home",
			fsys: fakeFS{entries: map[string]bool{"/work/app": true}, resolved: map[string]string{"/work/app": "/work/app"}},
			want: "resolve home directory: lstat /Users/dev: file does not exist",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewResolver(tt.fsys).Resolve("/work/app", "/Users/dev", false); err == nil || err.Error() != tt.want {
				t.Fatalf("Resolve() error = %v, want %q", err, tt.want)
			}
		})
	}
}
````

### 5.4.16 `internal/runtime/runtime_test.go` (changed file)

````go
package runtime

import (
	"reflect"
	"strings"
	"testing"

	"github.com/JeremyOrr4/caged/internal/plan"
)

func TestEmbeddedDockerfileIsPresent(t *testing.T) {
	t.Parallel()

	if len(runtimeDockerfile) == 0 {
		t.Fatal("runtime Dockerfile is empty")
	}
	if len(runtimeDockerfile) < 200 {
		t.Fatalf("runtime Dockerfile unexpectedly short: %d bytes", len(runtimeDockerfile))
	}
}

func TestRuntimeAgentHomeDirectoriesAreWritableByMappedUsers(t *testing.T) {
	t.Parallel()

	dockerfile := strings.ReplaceAll(string(runtimeDockerfile), "\\\n", " ")
	homePaths := []string{
		"/home/caged/.config/pi",
		"/home/caged/.pi",
		"/home/caged/.claude",
		"/home/caged/.codex",
		"/home/caged/.gemini",
	}
	for _, homePath := range homePaths {
		if !strings.Contains(dockerfile, homePath) {
			t.Errorf("runtime Dockerfile does not create agent home path %q", homePath)
		}
	}
	chmodStart := strings.Index(dockerfile, "chmod 0777 /home/caged")
	if chmodStart == -1 {
		t.Fatal("runtime Dockerfile does not make agent home paths writable by mapped users")
	}
	chmodEnd := strings.Index(dockerfile[chmodStart:], "&& git config")
	chmodLine := dockerfile[chmodStart:]
	if chmodEnd >= 0 {
		chmodLine = chmodLine[:chmodEnd]
	}
	for _, homePath := range homePaths {
		if !strings.Contains(chmodLine, homePath) {
			t.Errorf("runtime Dockerfile does not make agent home path %q writable", homePath)
		}
	}
}

func TestEmbeddedEntrypointRunsAgentAndReturnsToShell(t *testing.T) {
	t.Parallel()

	entrypoint := string(runtimeEntrypoint)
	if strings.Contains(entrypoint, "python3") || strings.Contains(entrypoint, "CAGED_CONFIG_MAP") {
		t.Error("embedded entrypoint still contains config-copy logic")
	}
	if !strings.Contains(entrypoint, `exec "$@"`) {
		t.Error("embedded entrypoint does not directly run the requested command")
	}
	if !strings.Contains(entrypoint, `CAGED_RETURN_TO_SHELL`) || !strings.Contains(entrypoint, "bash -i") {
		t.Error("embedded entrypoint does not open an interactive shell after agent exit")
	}
}

func TestRuntimeCodexCommandDisablesDaemon(t *testing.T) {
	t.Parallel()

	if !strings.Contains(string(runtimeDockerfile), `codex-real --no-daemon "$@"`) {
		t.Fatal("runtime image does not wrap Codex with --no-daemon for shell launches")
	}
}

func TestRuntimeImageLeavesClaudeConfigDestinationWritable(t *testing.T) {
	t.Parallel()

	if !strings.Contains(string(runtimeDockerfile), "rm -f /home/caged/.claude.json") {
		t.Fatal("runtime image does not remove the installer-created Claude config file")
	}
}

func TestClaudeUsesDirectoryConfigMount(t *testing.T) {
	t.Parallel()

	if !strings.Contains(string(runtimeDockerfile), "CLAUDE_CONFIG_DIR=/home/caged/.claude") {
		t.Fatal("runtime image does not keep Claude config writes inside the mounted directory")
	}
}

func TestRuntimePiInstallDoesNotOverlapHostConfigMount(t *testing.T) {
	t.Parallel()

	dockerfile := string(runtimeDockerfile)
	for _, expected := range []string{
		"HOME=/opt/caged/pi-home",
		"ln -sf /opt/caged/pi-home/.pi/agent/bin/pi /usr/local/bin/pi",
	} {
		if !strings.Contains(dockerfile, expected) {
			t.Errorf("runtime Dockerfile missing Pi install isolation %q", expected)
		}
	}
}

func TestRuntimeIncludesGithubCLIAndPersistsItsConfigDirectory(t *testing.T) {
	t.Parallel()
	dockerfile := string(runtimeDockerfile)
	for _, expected := range []string{
		"https://cli.github.com/packages/githubcli-archive-keyring.gpg",
		"apt-get install -y --no-install-recommends gh",
		"/home/caged/.config/gh",
	} {
		if !strings.Contains(dockerfile, expected) {
			t.Errorf("runtime Dockerfile missing GitHub CLI setup %q", expected)
		}
	}
}

func TestBuildSpecDescribesEmbeddedBuildContext(t *testing.T) {
	t.Parallel()

	want := plan.BuildSpec{
		Image:      "caged-runtime:latest",
		Dockerfile: "Dockerfile",
		NoCache:    true,
		Files: []plan.File{
			{Name: "Dockerfile", Label: "Dockerfile", Data: []byte(runtimeDockerfile), Mode: 0o644},
			{Name: "caged-entrypoint.sh", Label: "entrypoint", Data: []byte(runtimeEntrypoint), Mode: 0o755},
		},
	}
	got := BuildSpec(true)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildSpec(true) = %+v, want %+v", got, want)
	}
	got.Files[0].Data[0] = 'X'
	if BuildSpec(false).Files[0].Data[0] == 'X' || BuildSpec(false).NoCache {
		t.Fatal("BuildSpec must return fresh data and honor noCache")
	}
}
````

### 5.4.17 `internal/system/system_test.go` (new file)

````go
package system

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFSUsesTheRealFilesystem(t *testing.T) {
	t.Parallel()

	var fsys FS
	root, err := fsys.MkdirTemp(t.TempDir(), "system-test-")
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "file.txt")
	if err := fsys.WriteFile(file, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(file, link); err != nil {
		t.Fatal(err)
	}
	if info, err := fsys.Stat(link); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("Stat(link) = %v, %v; want the regular target file", info, err)
	}
	if info, err := fsys.Lstat(link); err != nil || info.Mode()&fs.ModeSymlink == 0 {
		t.Fatalf("Lstat(link) = %v, %v; want the symlink itself", info, err)
	}
	if entries, err := fsys.ReadDir(root); err != nil || len(entries) != 2 {
		t.Fatalf("ReadDir() = %v, %v; want two entries", entries, err)
	}
	resolvedFile, err := filepath.EvalSymlinks(file)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := fsys.EvalSymlinks(link); err != nil || got != resolvedFile {
		t.Fatalf("EvalSymlinks(link) = %q, %v; want %q", got, err, resolvedFile)
	}
	if got, err := fsys.Abs(file); err != nil || got != file {
		t.Fatalf("Abs(%q) = %q, %v", file, got, err)
	}
	visited := 0
	if err := fsys.WalkDir(root, func(string, fs.DirEntry, error) error { visited++; return nil }); err != nil || visited != 3 {
		t.Fatalf("WalkDir() visited %d entries, err %v; want 3", visited, err)
	}
	if err := fsys.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("RemoveAll() left %q behind: %v", root, err)
	}
}

func TestEnvReadsTheProcessEnvironment(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CAGED_SYSTEM_TEST", "value")

	var env Env
	if got, ok := env.LookupEnv("CAGED_SYSTEM_TEST"); !ok || got != "value" {
		t.Fatalf("LookupEnv() = %q, %v", got, ok)
	}
	if runtime.GOOS != "windows" {
		if got, err := env.UserHomeDir(); err != nil || got != home {
			t.Fatalf("UserHomeDir() = %q, %v; want %q", got, err, home)
		}
	}
	want, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if got, err := env.Getwd(); err != nil || got != want {
		t.Fatalf("Getwd() = %q, %v; want %q", got, err, want)
	}
}

func TestExecutorRunsProcessesAndReportsExitCodes(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("needs a POSIX shell")
	}

	var executor Executor
	sh, err := executor.LookPath("sh")
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	err = executor.Run(context.Background(), Command{
		Path:   sh,
		Args:   []string{"-c", "read line; echo \"out:$line\"; echo err >&2; exit 3"},
		Stdin:  bytes.NewBufferString("input\n"),
		Stdout: &stdout,
		Stderr: &stderr,
	})
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 3 {
		t.Fatalf("Run() error = %v, want exit status 3", err)
	}
	if stdout.String() != "out:input\n" || stderr.String() != "err\n" {
		t.Fatalf("stdout = %q, stderr = %q", stdout.String(), stderr.String())
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := executor.Run(ctx, Command{Path: sh, Args: []string{"-c", "exit 0"}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() with a cancelled context = %v, want context.Canceled", err)
	}
}

func TestCurrentPlatformAndStdioDescribeThisProcess(t *testing.T) {
	t.Parallel()

	want := Platform{OS: runtime.GOOS, Arch: runtime.GOARCH, UID: os.Getuid(), GID: os.Getgid()}
	if got := CurrentPlatform(); got != want {
		t.Fatalf("CurrentPlatform() = %+v, want %+v", got, want)
	}
	stdio := CurrentStdio()
	// docker run -it needs the real terminal file, not a copy through a pipe.
	if stdin, ok := stdio.In.(*os.File); !ok || stdin != os.Stdin {
		t.Fatal("CurrentStdio().In is not os.Stdin")
	}
	if stdout, ok := stdio.Out.(*os.File); !ok || stdout != os.Stdout {
		t.Fatal("CurrentStdio().Out is not os.Stdout")
	}
	if stderr, ok := stdio.Err.(*os.File); !ok || stderr != os.Stderr {
		t.Fatal("CurrentStdio().Err is not os.Stderr")
	}
}
````

# 6. macOS

## 6.1 What is different on a Mac

- **Fingerprints:** macOS has no `sha256sum`. Use the second fingerprint command of each step, which uses `shasum -a 256`.
  The expected value is the same.
- **Pass counts in Steps 1 to 10:** the golden test reads the real operating system in those steps, and its fixtures
  were recorded on Linux. So on macOS it prints `--- SKIP` and the number of `--- PASS` lines is 52 lower than the Linux
  number given in each step. From Step 11 on, the platform is injected, so all 102 golden subtests run on macOS too, and
  the count is the same as on Linux.
- **Temporary folders:** macOS temp folders live under `/var/folders/...`, which is a symlink to `/private/var/...`. The
  golden test replaces both spellings with `$ROOT`, and the resolver and project code compare paths only after
  `EvalSymlinks`. So the recordings are the same.
- **Behaviour of caged itself on macOS**, unchanged by the refactor and now kept in `internal/docker/platform.go`:
  - no `--user` flag, because Docker Desktop maps bind-mount ownership itself;
  - "start Docker Desktop" when the daemon is down;
  - a Docker Desktop File Sharing hint when a mount is denied.
- **Keychain:** on macOS, Claude Code keeps its login in the Keychain. caged does not forward it, before or after the
  refactor. There is no Keychain code, so nothing needed to move.

## 6.2 What was verified, and what could only be verified on a real Mac

Verified on Linux:

- `GOOS=darwin GOARCH=arm64` and `GOOS=darwin GOARCH=amd64` builds and `go vet` pass after every step.
- The complete golden suite runs with an injected macOS platform (`OS: "darwin"`), and every macOS branch has unit tests.
- The macOS recordings were checked by a script against the Linux recordings (see 5.3).

**Not verified (no Mac was available):**

- Running `go test ./...` natively on macOS. The tests use only POSIX features (`/bin/sh`, `mkfifo`, symlinks, `cksum`),
  but this was not executed on a Mac.
- Real Docker Desktop behaviour: file sharing errors, bind-mount ownership mapping, and the arm64 runtime image build.
  Docker is faked in every test.
- The exact text of `cksum` on macOS (BSD `cksum` is expected to print the same POSIX CRC).
- That following the steps on macOS gives the same fingerprints. The commands are standard, but this was replayed only
  on Linux.

# 7. Behaviour-preservation proof

## 7.1 Golden tests

The 51 Linux golden files were recorded from the untouched original code (Step 1). They pass, byte for byte, after
every step from 1 to 14. Each step's Verify section runs them.

## 7.2 Before and after binaries, side by side

The original and the refactored code were each built into a binary (`go build -o ... ./cmd/caged`). Both binaries were
run with the same command lines, against the same fake home folder, a fake project, and a fake `docker` on `PATH`. The
`--user` value is the real Linux user of that machine (1000:1000). The tables show the `docker run` or `docker build`
call that each binary made. `$ROOT` is the temporary test folder.


### `caged shell`

| | Docker call |
|---|---|
| Before | `docker run -it --rm -v $ROOT/work/project:/workspace/project -w /workspace/project -v $ROOT/home/.claude:/home/caged/.claude -v $ROOT/home/.codex:/home/caged/.codex --tmpfs /home/caged/.codex/app-server-control --tmpfs /home/caged/.codex/app-server-daemon --tmpfs /home/caged/.codex/ipc --tmpfs /home/caged/.codex/packages/app-server-daemon --tmpfs /home/caged/.codex/tmp/arg0 -v $ROOT/home/.config/gh:/home/caged/.config/gh -v $ROOT/home/.gitconfig:/home/caged/.gitconfig:ro --user 1000:1000 caged-runtime:latest bash` |
| After | `docker run -it --rm -v $ROOT/work/project:/workspace/project -w /workspace/project -v $ROOT/home/.claude:/home/caged/.claude -v $ROOT/home/.codex:/home/caged/.codex --tmpfs /home/caged/.codex/app-server-control --tmpfs /home/caged/.codex/app-server-daemon --tmpfs /home/caged/.codex/ipc --tmpfs /home/caged/.codex/packages/app-server-daemon --tmpfs /home/caged/.codex/tmp/arg0 -v $ROOT/home/.config/gh:/home/caged/.config/gh -v $ROOT/home/.gitconfig:/home/caged/.gitconfig:ro --user 1000:1000 caged-runtime:latest bash` |

Exit code before 0, after 0. Stderr identical: yes. Docker calls identical: yes.

### `caged shell --no-mount --no-config`

| | Docker call |
|---|---|
| Before | `docker run -it --rm --user 1000:1000 caged-runtime:latest bash` |
| After | `docker run -it --rm --user 1000:1000 caged-runtime:latest bash` |

Exit code before 0, after 0. Stderr identical: yes. Docker calls identical: yes.

### `caged claude`

| | Docker call |
|---|---|
| Before | `docker run -it --rm -v $ROOT/work/project:/workspace/project -w /workspace/project -v $ROOT/home/.claude:/home/caged/.claude -v $ROOT/home/.codex:/home/caged/.codex --tmpfs /home/caged/.codex/app-server-control --tmpfs /home/caged/.codex/app-server-daemon --tmpfs /home/caged/.codex/ipc --tmpfs /home/caged/.codex/packages/app-server-daemon --tmpfs /home/caged/.codex/tmp/arg0 -v $ROOT/home/.config/gh:/home/caged/.config/gh -v $ROOT/home/.gitconfig:/home/caged/.gitconfig:ro --user 1000:1000 -e CAGED_RETURN_TO_SHELL=1 caged-runtime:latest claude` |
| After | `docker run -it --rm -v $ROOT/work/project:/workspace/project -w /workspace/project -v $ROOT/home/.claude:/home/caged/.claude -v $ROOT/home/.codex:/home/caged/.codex --tmpfs /home/caged/.codex/app-server-control --tmpfs /home/caged/.codex/app-server-daemon --tmpfs /home/caged/.codex/ipc --tmpfs /home/caged/.codex/packages/app-server-daemon --tmpfs /home/caged/.codex/tmp/arg0 -v $ROOT/home/.config/gh:/home/caged/.config/gh -v $ROOT/home/.gitconfig:/home/caged/.gitconfig:ro --user 1000:1000 -e CAGED_RETURN_TO_SHELL=1 caged-runtime:latest claude` |

Exit code before 0, after 0. Stderr identical: yes. Docker calls identical: yes.

### `caged claude '<prompt>'`

| | Docker call |
|---|---|
| Before | `docker run -it --rm -v $ROOT/work/project:/workspace/project -w /workspace/project -v $ROOT/home/.claude:/home/caged/.claude -v $ROOT/home/.codex:/home/caged/.codex --tmpfs /home/caged/.codex/app-server-control --tmpfs /home/caged/.codex/app-server-daemon --tmpfs /home/caged/.codex/ipc --tmpfs /home/caged/.codex/packages/app-server-daemon --tmpfs /home/caged/.codex/tmp/arg0 -v $ROOT/home/.config/gh:/home/caged/.config/gh -v $ROOT/home/.gitconfig:/home/caged/.gitconfig:ro --user 1000:1000 -e CAGED_RETURN_TO_SHELL=1 caged-runtime:latest claude 'hello $(world) "quoted"'` |
| After | `docker run -it --rm -v $ROOT/work/project:/workspace/project -w /workspace/project -v $ROOT/home/.claude:/home/caged/.claude -v $ROOT/home/.codex:/home/caged/.codex --tmpfs /home/caged/.codex/app-server-control --tmpfs /home/caged/.codex/app-server-daemon --tmpfs /home/caged/.codex/ipc --tmpfs /home/caged/.codex/packages/app-server-daemon --tmpfs /home/caged/.codex/tmp/arg0 -v $ROOT/home/.config/gh:/home/caged/.config/gh -v $ROOT/home/.gitconfig:/home/caged/.gitconfig:ro --user 1000:1000 -e CAGED_RETURN_TO_SHELL=1 caged-runtime:latest claude 'hello $(world) "quoted"'` |

Exit code before 0, after 0. Stderr identical: yes. Docker calls identical: yes.

### `caged codex`

| | Docker call |
|---|---|
| Before | `docker run -it --rm -v $ROOT/work/project:/workspace/project -w /workspace/project -v $ROOT/home/.claude:/home/caged/.claude -v $ROOT/home/.codex:/home/caged/.codex --tmpfs /home/caged/.codex/app-server-control --tmpfs /home/caged/.codex/app-server-daemon --tmpfs /home/caged/.codex/ipc --tmpfs /home/caged/.codex/packages/app-server-daemon --tmpfs /home/caged/.codex/tmp/arg0 -v $ROOT/home/.config/gh:/home/caged/.config/gh -v $ROOT/home/.gitconfig:/home/caged/.gitconfig:ro --user 1000:1000 -e OPENAI_API_KEY -e CAGED_RETURN_TO_SHELL=1 caged-runtime:latest codex` |
| After | `docker run -it --rm -v $ROOT/work/project:/workspace/project -w /workspace/project -v $ROOT/home/.claude:/home/caged/.claude -v $ROOT/home/.codex:/home/caged/.codex --tmpfs /home/caged/.codex/app-server-control --tmpfs /home/caged/.codex/app-server-daemon --tmpfs /home/caged/.codex/ipc --tmpfs /home/caged/.codex/packages/app-server-daemon --tmpfs /home/caged/.codex/tmp/arg0 -v $ROOT/home/.config/gh:/home/caged/.config/gh -v $ROOT/home/.gitconfig:/home/caged/.gitconfig:ro --user 1000:1000 -e OPENAI_API_KEY -e CAGED_RETURN_TO_SHELL=1 caged-runtime:latest codex` |

Exit code before 0, after 0. Stderr identical: yes. Docker calls identical: yes.

### `caged codex '<prompt>'`

| | Docker call |
|---|---|
| Before | `docker run -it --rm -v $ROOT/work/project:/workspace/project -w /workspace/project -v $ROOT/home/.claude:/home/caged/.claude -v $ROOT/home/.codex:/home/caged/.codex --tmpfs /home/caged/.codex/app-server-control --tmpfs /home/caged/.codex/app-server-daemon --tmpfs /home/caged/.codex/ipc --tmpfs /home/caged/.codex/packages/app-server-daemon --tmpfs /home/caged/.codex/tmp/arg0 -v $ROOT/home/.config/gh:/home/caged/.config/gh -v $ROOT/home/.gitconfig:/home/caged/.gitconfig:ro --user 1000:1000 -e OPENAI_API_KEY -e CAGED_RETURN_TO_SHELL=1 caged-runtime:latest codex exec 'hello $(world) "quoted"'` |
| After | `docker run -it --rm -v $ROOT/work/project:/workspace/project -w /workspace/project -v $ROOT/home/.claude:/home/caged/.claude -v $ROOT/home/.codex:/home/caged/.codex --tmpfs /home/caged/.codex/app-server-control --tmpfs /home/caged/.codex/app-server-daemon --tmpfs /home/caged/.codex/ipc --tmpfs /home/caged/.codex/packages/app-server-daemon --tmpfs /home/caged/.codex/tmp/arg0 -v $ROOT/home/.config/gh:/home/caged/.config/gh -v $ROOT/home/.gitconfig:/home/caged/.gitconfig:ro --user 1000:1000 -e OPENAI_API_KEY -e CAGED_RETURN_TO_SHELL=1 caged-runtime:latest codex exec 'hello $(world) "quoted"'` |

Exit code before 0, after 0. Stderr identical: yes. Docker calls identical: yes.

### `caged copilot`

| | Docker call |
|---|---|
| Before | `docker run -it --rm -v $ROOT/work/project:/workspace/project -w /workspace/project -v $ROOT/home/.claude:/home/caged/.claude -v $ROOT/home/.codex:/home/caged/.codex --tmpfs /home/caged/.codex/app-server-control --tmpfs /home/caged/.codex/app-server-daemon --tmpfs /home/caged/.codex/ipc --tmpfs /home/caged/.codex/packages/app-server-daemon --tmpfs /home/caged/.codex/tmp/arg0 -v $ROOT/home/.config/gh:/home/caged/.config/gh -v $ROOT/home/.gitconfig:/home/caged/.gitconfig:ro --user 1000:1000 -e CAGED_RETURN_TO_SHELL=1 caged-runtime:latest copilot` |
| After | `docker run -it --rm -v $ROOT/work/project:/workspace/project -w /workspace/project -v $ROOT/home/.claude:/home/caged/.claude -v $ROOT/home/.codex:/home/caged/.codex --tmpfs /home/caged/.codex/app-server-control --tmpfs /home/caged/.codex/app-server-daemon --tmpfs /home/caged/.codex/ipc --tmpfs /home/caged/.codex/packages/app-server-daemon --tmpfs /home/caged/.codex/tmp/arg0 -v $ROOT/home/.config/gh:/home/caged/.config/gh -v $ROOT/home/.gitconfig:/home/caged/.gitconfig:ro --user 1000:1000 -e CAGED_RETURN_TO_SHELL=1 caged-runtime:latest copilot` |

Exit code before 0, after 0. Stderr identical: yes. Docker calls identical: yes.

### `caged copilot '<prompt>'`

| | Docker call |
|---|---|
| Before | `docker run -it --rm -v $ROOT/work/project:/workspace/project -w /workspace/project -v $ROOT/home/.claude:/home/caged/.claude -v $ROOT/home/.codex:/home/caged/.codex --tmpfs /home/caged/.codex/app-server-control --tmpfs /home/caged/.codex/app-server-daemon --tmpfs /home/caged/.codex/ipc --tmpfs /home/caged/.codex/packages/app-server-daemon --tmpfs /home/caged/.codex/tmp/arg0 -v $ROOT/home/.config/gh:/home/caged/.config/gh -v $ROOT/home/.gitconfig:/home/caged/.gitconfig:ro --user 1000:1000 -e CAGED_RETURN_TO_SHELL=1 caged-runtime:latest copilot -p 'hello $(world) "quoted"'` |
| After | `docker run -it --rm -v $ROOT/work/project:/workspace/project -w /workspace/project -v $ROOT/home/.claude:/home/caged/.claude -v $ROOT/home/.codex:/home/caged/.codex --tmpfs /home/caged/.codex/app-server-control --tmpfs /home/caged/.codex/app-server-daemon --tmpfs /home/caged/.codex/ipc --tmpfs /home/caged/.codex/packages/app-server-daemon --tmpfs /home/caged/.codex/tmp/arg0 -v $ROOT/home/.config/gh:/home/caged/.config/gh -v $ROOT/home/.gitconfig:/home/caged/.gitconfig:ro --user 1000:1000 -e CAGED_RETURN_TO_SHELL=1 caged-runtime:latest copilot -p 'hello $(world) "quoted"'` |

Exit code before 0, after 0. Stderr identical: yes. Docker calls identical: yes.

### `caged gemini`

| | Docker call |
|---|---|
| Before | `docker run -it --rm -v $ROOT/work/project:/workspace/project -w /workspace/project -v $ROOT/home/.claude:/home/caged/.claude -v $ROOT/home/.codex:/home/caged/.codex --tmpfs /home/caged/.codex/app-server-control --tmpfs /home/caged/.codex/app-server-daemon --tmpfs /home/caged/.codex/ipc --tmpfs /home/caged/.codex/packages/app-server-daemon --tmpfs /home/caged/.codex/tmp/arg0 -v $ROOT/home/.config/gh:/home/caged/.config/gh -v $ROOT/home/.gitconfig:/home/caged/.gitconfig:ro --user 1000:1000 -e CAGED_RETURN_TO_SHELL=1 caged-runtime:latest gemini` |
| After | `docker run -it --rm -v $ROOT/work/project:/workspace/project -w /workspace/project -v $ROOT/home/.claude:/home/caged/.claude -v $ROOT/home/.codex:/home/caged/.codex --tmpfs /home/caged/.codex/app-server-control --tmpfs /home/caged/.codex/app-server-daemon --tmpfs /home/caged/.codex/ipc --tmpfs /home/caged/.codex/packages/app-server-daemon --tmpfs /home/caged/.codex/tmp/arg0 -v $ROOT/home/.config/gh:/home/caged/.config/gh -v $ROOT/home/.gitconfig:/home/caged/.gitconfig:ro --user 1000:1000 -e CAGED_RETURN_TO_SHELL=1 caged-runtime:latest gemini` |

Exit code before 0, after 0. Stderr identical: yes. Docker calls identical: yes.

### `caged gemini '<prompt>'`

| | Docker call |
|---|---|
| Before | `docker run -it --rm -v $ROOT/work/project:/workspace/project -w /workspace/project -v $ROOT/home/.claude:/home/caged/.claude -v $ROOT/home/.codex:/home/caged/.codex --tmpfs /home/caged/.codex/app-server-control --tmpfs /home/caged/.codex/app-server-daemon --tmpfs /home/caged/.codex/ipc --tmpfs /home/caged/.codex/packages/app-server-daemon --tmpfs /home/caged/.codex/tmp/arg0 -v $ROOT/home/.config/gh:/home/caged/.config/gh -v $ROOT/home/.gitconfig:/home/caged/.gitconfig:ro --user 1000:1000 -e CAGED_RETURN_TO_SHELL=1 caged-runtime:latest gemini -i 'hello $(world) "quoted"'` |
| After | `docker run -it --rm -v $ROOT/work/project:/workspace/project -w /workspace/project -v $ROOT/home/.claude:/home/caged/.claude -v $ROOT/home/.codex:/home/caged/.codex --tmpfs /home/caged/.codex/app-server-control --tmpfs /home/caged/.codex/app-server-daemon --tmpfs /home/caged/.codex/ipc --tmpfs /home/caged/.codex/packages/app-server-daemon --tmpfs /home/caged/.codex/tmp/arg0 -v $ROOT/home/.config/gh:/home/caged/.config/gh -v $ROOT/home/.gitconfig:/home/caged/.gitconfig:ro --user 1000:1000 -e CAGED_RETURN_TO_SHELL=1 caged-runtime:latest gemini -i 'hello $(world) "quoted"'` |

Exit code before 0, after 0. Stderr identical: yes. Docker calls identical: yes.

### `caged gh`

| | Docker call |
|---|---|
| Before | `docker run -it --rm -v $ROOT/work/project:/workspace/project -w /workspace/project -v $ROOT/home/.claude:/home/caged/.claude -v $ROOT/home/.codex:/home/caged/.codex --tmpfs /home/caged/.codex/app-server-control --tmpfs /home/caged/.codex/app-server-daemon --tmpfs /home/caged/.codex/ipc --tmpfs /home/caged/.codex/packages/app-server-daemon --tmpfs /home/caged/.codex/tmp/arg0 -v $ROOT/home/.config/gh:/home/caged/.config/gh -v $ROOT/home/.gitconfig:/home/caged/.gitconfig:ro --user 1000:1000 -e CAGED_RETURN_TO_SHELL=1 caged-runtime:latest gh` |
| After | `docker run -it --rm -v $ROOT/work/project:/workspace/project -w /workspace/project -v $ROOT/home/.claude:/home/caged/.claude -v $ROOT/home/.codex:/home/caged/.codex --tmpfs /home/caged/.codex/app-server-control --tmpfs /home/caged/.codex/app-server-daemon --tmpfs /home/caged/.codex/ipc --tmpfs /home/caged/.codex/packages/app-server-daemon --tmpfs /home/caged/.codex/tmp/arg0 -v $ROOT/home/.config/gh:/home/caged/.config/gh -v $ROOT/home/.gitconfig:/home/caged/.gitconfig:ro --user 1000:1000 -e CAGED_RETURN_TO_SHELL=1 caged-runtime:latest gh` |

Exit code before 0, after 0. Stderr identical: yes. Docker calls identical: yes.

### `caged pi`

| | Docker call |
|---|---|
| Before | `docker run -it --rm -v $ROOT/work/project:/workspace/project -w /workspace/project -v $ROOT/home/.claude:/home/caged/.claude -v $ROOT/home/.codex:/home/caged/.codex --tmpfs /home/caged/.codex/app-server-control --tmpfs /home/caged/.codex/app-server-daemon --tmpfs /home/caged/.codex/ipc --tmpfs /home/caged/.codex/packages/app-server-daemon --tmpfs /home/caged/.codex/tmp/arg0 -v $ROOT/home/.config/gh:/home/caged/.config/gh -v $ROOT/home/.gitconfig:/home/caged/.gitconfig:ro --user 1000:1000 -e OPENAI_API_KEY -e CAGED_RETURN_TO_SHELL=1 caged-runtime:latest pi` |
| After | `docker run -it --rm -v $ROOT/work/project:/workspace/project -w /workspace/project -v $ROOT/home/.claude:/home/caged/.claude -v $ROOT/home/.codex:/home/caged/.codex --tmpfs /home/caged/.codex/app-server-control --tmpfs /home/caged/.codex/app-server-daemon --tmpfs /home/caged/.codex/ipc --tmpfs /home/caged/.codex/packages/app-server-daemon --tmpfs /home/caged/.codex/tmp/arg0 -v $ROOT/home/.config/gh:/home/caged/.config/gh -v $ROOT/home/.gitconfig:/home/caged/.gitconfig:ro --user 1000:1000 -e OPENAI_API_KEY -e CAGED_RETURN_TO_SHELL=1 caged-runtime:latest pi` |

Exit code before 0, after 0. Stderr identical: yes. Docker calls identical: yes.

### `caged pi '<prompt>'`

| | Docker call |
|---|---|
| Before | `docker run -it --rm -v $ROOT/work/project:/workspace/project -w /workspace/project -v $ROOT/home/.claude:/home/caged/.claude -v $ROOT/home/.codex:/home/caged/.codex --tmpfs /home/caged/.codex/app-server-control --tmpfs /home/caged/.codex/app-server-daemon --tmpfs /home/caged/.codex/ipc --tmpfs /home/caged/.codex/packages/app-server-daemon --tmpfs /home/caged/.codex/tmp/arg0 -v $ROOT/home/.config/gh:/home/caged/.config/gh -v $ROOT/home/.gitconfig:/home/caged/.gitconfig:ro --user 1000:1000 -e OPENAI_API_KEY -e CAGED_RETURN_TO_SHELL=1 caged-runtime:latest pi 'hello $(world) "quoted"'` |
| After | `docker run -it --rm -v $ROOT/work/project:/workspace/project -w /workspace/project -v $ROOT/home/.claude:/home/caged/.claude -v $ROOT/home/.codex:/home/caged/.codex --tmpfs /home/caged/.codex/app-server-control --tmpfs /home/caged/.codex/app-server-daemon --tmpfs /home/caged/.codex/ipc --tmpfs /home/caged/.codex/packages/app-server-daemon --tmpfs /home/caged/.codex/tmp/arg0 -v $ROOT/home/.config/gh:/home/caged/.config/gh -v $ROOT/home/.gitconfig:/home/caged/.gitconfig:ro --user 1000:1000 -e OPENAI_API_KEY -e CAGED_RETURN_TO_SHELL=1 caged-runtime:latest pi 'hello $(world) "quoted"'` |

Exit code before 0, after 0. Stderr identical: yes. Docker calls identical: yes.

### `caged build`

| | Docker call |
|---|---|
| Before | `docker build -t caged-runtime:latest -f $ROOT/tmp/caged-runtime-N/Dockerfile $ROOT/tmp/caged-runtime-N` |
| After | `docker build -t caged-runtime:latest -f $ROOT/tmp/caged-runtime-N/Dockerfile $ROOT/tmp/caged-runtime-N` |

Exit code before 0, after 0. Stderr identical: yes. Docker calls identical: yes.

### `caged build --no-cache`

| | Docker call |
|---|---|
| Before | `docker build -t caged-runtime:latest --no-cache -f $ROOT/tmp/caged-runtime-N/Dockerfile $ROOT/tmp/caged-runtime-N` |
| After | `docker build -t caged-runtime:latest --no-cache -f $ROOT/tmp/caged-runtime-N/Dockerfile $ROOT/tmp/caged-runtime-N` |

Exit code before 0, after 0. Stderr identical: yes. Docker calls identical: yes.

### `caged codex --env OPENAI_API_KEY --no-config`

| | Docker call |
|---|---|
| Before | `docker run -it --rm -v $ROOT/work/project:/workspace/project -w /workspace/project --user 1000:1000 -e OPENAI_API_KEY -e CAGED_RETURN_TO_SHELL=1 caged-runtime:latest codex` |
| After | `docker run -it --rm -v $ROOT/work/project:/workspace/project -w /workspace/project --user 1000:1000 -e OPENAI_API_KEY -e CAGED_RETURN_TO_SHELL=1 caged-runtime:latest codex` |

Exit code before 0, after 0. Stderr identical: yes. Docker calls identical: yes.

## 7.3 Independent review

A separate review compared every changed Go file line by line with the original. It ran 85 command lines through both
binaries and compared the exit code, stdout, stderr, every docker call and its arguments, the file descriptors docker
got, the file modes in the build context, and leftover temp dirs. All 85 matched. It found no behaviour difference
that can be reached from the command line.

## 7.4 Behaviour changes

**From the command line: none.** The following changes are visible only to Go code inside this module (these are
`internal` packages, so no other module can import them):

1. Exported functions were replaced by methods on new types. Section 2.2 lists every one.
2. `app.Service` no longer validates env-variable names. The CLI does it before calling the service, in the same order
   as before. A Go caller that skips the CLI now has invalid names silently dropped instead of rejected.
3. The defensive checks for missing dependencies were removed: "application service requires a runner and runtime"
   and "Docker availability check is required". Dependencies are now passed in constructors, so those states cannot
   happen through the CLI.
4. `Agent.ResolveEnv` has no fallback to `os.LookupEnv`; the caller always passes the lookup function.
5. The registry returns deep copies, so callers cannot change a registered agent.
6. The UID and GID are read once at start-up instead of at each `docker run`. This makes no difference within one
   process.

## 7.5 Replay test

This file was checked with a literal replay:

1. The original snapshot was copied to `/tmp/caged_replay`.
2. A script read this file and did exactly what it says, in order:
   - ran the step commands;
   - created the files;
   - applied every "Find this exact text" edit (the old text had to occur exactly once);
   - applied every whole-file replacement (the old text had to match the file exactly);
   - ran the Verify commands (each had to succeed, and `gofmt`/`go build`/`go vet` had to print nothing);
   - compared every fingerprint.
3. Then `diff -ru /tmp/caged_replay <refactored repo>` was run. Its output was empty, except for this file itself,
   which exists only in the refactored repository.


# 8. Troubleshooting

| What you see | Why | Exact fix |
|---|---|---|
| `go: command not found` | Go is not on your `PATH`. | Run the three `export` lines from section 1.3 in this terminal. |
| `go: downloading go1.2x...` or `toolchain not available` | Go tries to switch versions. | `export GOTOOLCHAIN=local` and use go1.23.12 (section 1.2). |
| `error obtaining VCS status` during `go build` | Go tries to call `git` to stamp the build. | `export GOFLAGS=-buildvcs=false` (section 1.3). |
| `cannot find module providing package github.com/spf13/cobra` | The module cache is empty and offline. | Connect to the internet once and run `go mod download`. Do not run `go mod tidy`. |
| `X declared and not used` or `"Y" imported and not used` | An edit was half applied, or an import line is wrong. | Re-read the **Imports** sentence for that file in the current step and compare the `import (...)` block. |
| `undefined: something` | A caller still uses a name that this step removed or renamed. | Search the repository for the old name (`grep -rn "oldName" --include=*.go .`) and apply the edits for that file in the current step again. |
| `expected 'package', found ...` or `syntax error` | Text was pasted with a missing or extra line, often near a `}`. | Compare the file with the "Replace it with this exact text" blocks, or with the full text in Appendix A (final step only). |
| A golden test says `output differs` | Behaviour changed, or a fixture was mistyped. | Read the `--- want` and `--- got` text in the failure. If the difference is in a fixture you typed in Step 1 or 11, re-create that `.golden` file exactly. Never edit a fixture to match new code. |
| Only `complete-agents.golden` differs | Its tabs became spaces. | Re-create the file and keep the TAB characters (Step 1 note). |
| The fingerprint differs but all tests pass | One file differs by a byte: tabs vs spaces, a trailing space, a missing final newline, or an extra file such as `bin/` or `cover.out`. | Run `gofmt -w .`, remove `bin/` and `cover.out`, then compare your file list with the step's **Checkpoint** list. |
| `mv: cannot stat ...` | The command already ran, or the previous step was not finished. | Check with `ls` whether the destination already exists. If it does, skip the `mv` and continue with the edits. |
| `TestEnvReadsTheProcessEnvironment` fails on `UserHomeDir` | `HOME` is unusual (for example, a sandbox). | Run the tests in a normal terminal session where `HOME` is set. |
| The golden tests print `SKIP` on macOS in Steps 1 to 10 | They are recorded on Linux and only run there until Step 11. | Nothing to fix; see section 6.1. |
| `-race` reports `unsupported` | The race detector needs cgo and a supported CPU. | Install a C compiler (Xcode command line tools on macOS, `gcc` on Linux), or run the race test on a supported machine. |

# 9. How to add a new agent (worked example: Aider)

**Do not do this while you reproduce the refactor.** The final fingerprint and the replay check expect the repository
without this example. Try it in a copy, for example `cp -r caged /tmp/caged-aider && cd /tmp/caged-aider`. The example
below was tested exactly this way: it passed `go vet ./...` and all of `go test ./...`.

## 9.1 Before the refactor (for comparison)

You had to edit the global map literal inside `internal/agent/agent.go`, and then edit two hard-coded agent lists in
`internal/agent/agent_test.go`. If the agent needed a new prompt style, you also had to edit the `switch` in
`internal/plan/run.go`.

## 9.2 After the refactor: one new file and one new line

Step A. Create the NEW FILE `internal/agent/aider.go` with exactly this content:

````go
package agent

// aider is the Aider AI pair-programming CLI.
func aider() Agent {
	return Agent{
		Name:        "aider",
		Command:     []string{"aider"},
		Description: "Aider",
		ConfigPaths: []ConfigPath{
			{HostPath: ".aider", ContainerPath: ".aider", Writable: true},
		},
		Prompt:     PromptFlag,
		PromptFlag: "--message",
		Auth:       AuthStrategy{Environment: []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY"}},
	}
}
````

- `Name` is what users type (`caged aider`). It must be lowercase, and it must not be `build`, `list`, `help` or `completion`.
- `Command` is what runs inside the container.
- `ConfigPaths` lists the host config folders, relative to the home directory. They are mounted read-write when they exist.
- `Prompt: PromptFlag` with `PromptFlag: "--message"` turns `caged aider 'add tests'` into `aider --message 'add tests'`.
  Use `PromptPositional` for `aider 'add tests'`, or leave `Prompt` out if the agent takes no prompt.
- `Auth.Environment` lists the host variables that are forwarded by name when they are set.

Step B. Register it in `internal/agent/builtin.go`. This is the only line you change anywhere.

Find this exact text:

````go
	registry, err := NewRegistry(
		claude(),
````

Replace it with this exact text:

````go
	registry, err := NewRegistry(
		aider(),
		claude(),
````

(The order of the list does not matter, because names are sorted. Alphabetical order just keeps it tidy.)

Step C. Check it:

````bash
gofmt -l .
go vet ./...
go test -count=1 ./...
````

Expected: `gofmt -l .` and `go vet ./...` print nothing, and every package prints `ok`. Nothing else needs to change:

- `TestBuiltinRegistryContract` checks the new definition automatically: sorted names, a description, a flag for
  `PromptFlag`, valid environment names, and clean relative config paths.
- `TestAgentNamesDoNotCollideWithSubcommands` checks that the name does not clash with a subcommand.
- The golden tests are unchanged, because they run with the six agents they were recorded with.
- The config resolver, `--verbose`, shell completion, and `caged list` pick the agent up from the registry.

Step D (outside Go): the agent's CLI must also be installed in the runtime image, `internal/runtime/Dockerfile`. That
file is outside the scope of this refactor. Add a `RUN` line that installs the tool, then run `caged build --no-cache`.

## 9.3 Adding a container runtime or a shared config file

- **A runtime such as Podman:** create `internal/podman/` with a type that has the three methods of `app.Engine`
  (`ImageExists`, `Build`, `Run`). Then change the one line `engine := docker.NewClient(...)` in `cmd/caged/main.go`,
  and add the new package to `allowedImports` in `cmd/caged/architecture_test.go` and to the table in
  `docs/architecture.md`.
- **A shared file such as `~/.npmrc`:** add one entry to the list in `hostconfig.DefaultSharedFiles()`:
  `{Path: ".npmrc", Description: "shared npm config"}`.

# 10. Final checklist

Tick every box. When all of them are ticked, your result matches the refactored repository.

Setup:

- [ ] `go version` prints `go version go1.23.12 ...`.
- [ ] `GOTOOLCHAIN=local` and `GOFLAGS=-buildvcs=false` are exported.
- [ ] The starting fingerprint matched `b745d84f849cf751...` (section 1.5).

Steps:

- [ ] Step 1 done, Verify passed, fingerprint `f5a3be7e50fdef97...` matched.
- [ ] Step 2 done, Verify passed, fingerprint `e13a944a4be7d28a...` matched.
- [ ] Step 3 done, Verify passed, fingerprint `d31848457b0c0474...` matched.
- [ ] Step 4 done, Verify passed, fingerprint `0cdb5a91af2307bf...` matched.
- [ ] Step 5 done, Verify passed, fingerprint `bd154c2e26939083...` matched.
- [ ] Step 6 done, Verify passed, fingerprint `f23d088f17adb8ce...` matched.
- [ ] Step 7 done, Verify passed, fingerprint `e34b2ac40f48f351...` matched.
- [ ] Step 8 done, Verify passed, fingerprint `9f0a631b2bc78c14...` matched.
- [ ] Step 9 done, Verify passed, fingerprint `8e3c0dbdce309762...` matched.
- [ ] Step 10 done, Verify passed, fingerprint `99844e275abc877d...` matched.
- [ ] Step 11 done, Verify passed, fingerprint `075a00033e2e4eb0...` matched.
- [ ] Step 12 done, Verify passed, fingerprint `faa20739e5c49b86...` matched.
- [ ] Step 13 done, Verify passed, fingerprint `a7052c65b9c6d4d2...` matched.
- [ ] Step 14 done, Verify passed, fingerprint `b2c48e4a79e4affc...` matched.

Final commands, all from the repository root:

- [ ] `gofmt -l .` prints nothing.
- [ ] `go build ./...` prints nothing.
- [ ] `go vet ./...` prints nothing.
- [ ] `go test -count=1 ./...`: every package prints `ok`.
- [ ] `go test -count=1 -v ./... 2>&1 | grep -c -- "--- PASS"` prints `255`.
- [ ] `go test -count=1 -race ./...`: every package prints `ok`.
- [ ] `go test -count=1 -cover ./...` shows at least the "After" coverage in section 5.1.
- [ ] `GOOS=darwin GOARCH=arm64 go build ./...` and `GOOS=darwin GOARCH=amd64 go build ./...` print nothing.
- [ ] `GOOS=darwin GOARCH=arm64 go vet ./...` and `GOOS=darwin GOARCH=amd64 go vet ./...` print nothing.
- [ ] The golden fixture checksums in section 5.3 match.
- [ ] The final fingerprint is exactly `b2c48e4a79e4affc0882414ba6479c34b8fb3e7877ea42a04842f4bce05ba73f`.

Files: the repository contains exactly these 156 files (plus `.git/` and, if you keep it there, this file):

- `./` (5 files): `.gitignore`, `Makefile`, `README.md`, `go.mod`, `go.sum`
- `cmd/caged/` (4 files): `architecture_test.go`, `golden_test.go`, `main.go`, `main_test.go`
- `cmd/caged/testdata/golden/darwin/` (51 files): `agent-claude.golden`, `agent-codex.golden`, `agent-copilot.golden`, `agent-gemini.golden`, `agent-gh.golden`, `agent-pi.golden`, `agent-uppercase.golden`, `build-daemon-down.golden`, `build-help.golden`, `build-no-cache.golden`, `build-rejects-args.golden`, `build.golden`, `complete-agents.golden`, `complete-prompt.golden`, `config-overrides.golden`, `daemon-down.golden`, `docker-missing.golden`, `env-extra-names.golden`, `env-invalid.golden`, `env-padded-name.golden`, `help-list.golden`, `help-unknown-topic.golden`, `help.golden`, `home-unresolvable.golden`, `image-missing-builds-first.golden`, `list-rejects-args.golden`, `list.golden`, `no-mount-with-workdir.golden`, `prompt-claude.golden`, `prompt-codex.golden`, `prompt-copilot.golden`, `prompt-gemini.golden`, `prompt-gh.golden`, `prompt-pi.golden`, `run-exit-code.golden`, `run-mounts-denied.golden`, `shell-env.golden`, `shell-no-mount-no-config.golden`, `shell.golden`, `too-many-args.golden`, `unknown-flag.golden`, `unsupported-agent.golden`, `verbose-claude.golden`, `version.golden`, `workdir-flag.golden`, `workdir-home-allowed.golden`, `workdir-is-home.golden`, `workdir-missing.golden`, `workdir-relative.golden`, `workdir-root-allowed.golden`, `workdir-root.golden`
- `cmd/caged/testdata/golden/linux/` (51 files): `agent-claude.golden`, `agent-codex.golden`, `agent-copilot.golden`, `agent-gemini.golden`, `agent-gh.golden`, `agent-pi.golden`, `agent-uppercase.golden`, `build-daemon-down.golden`, `build-help.golden`, `build-no-cache.golden`, `build-rejects-args.golden`, `build.golden`, `complete-agents.golden`, `complete-prompt.golden`, `config-overrides.golden`, `daemon-down.golden`, `docker-missing.golden`, `env-extra-names.golden`, `env-invalid.golden`, `env-padded-name.golden`, `help-list.golden`, `help-unknown-topic.golden`, `help.golden`, `home-unresolvable.golden`, `image-missing-builds-first.golden`, `list-rejects-args.golden`, `list.golden`, `no-mount-with-workdir.golden`, `prompt-claude.golden`, `prompt-codex.golden`, `prompt-copilot.golden`, `prompt-gemini.golden`, `prompt-gh.golden`, `prompt-pi.golden`, `run-exit-code.golden`, `run-mounts-denied.golden`, `shell-env.golden`, `shell-no-mount-no-config.golden`, `shell.golden`, `too-many-args.golden`, `unknown-flag.golden`, `unsupported-agent.golden`, `verbose-claude.golden`, `version.golden`, `workdir-flag.golden`, `workdir-home-allowed.golden`, `workdir-is-home.golden`, `workdir-missing.golden`, `workdir-relative.golden`, `workdir-root-allowed.golden`, `workdir-root.golden`
- `dev/` (1 files): `Dockerfile`
- `docs/` (3 files): `architecture.md`, `implementation.md`, `spec.md`
- `internal/agent/` (12 files): `agent.go`, `agent_test.go`, `builtin.go`, `claude.go`, `codex.go`, `copilot.go`, `definitions_test.go`, `gemini.go`, `gh.go`, `pi.go`, `registry.go`, `registry_test.go`
- `internal/agent/testdata/` (1 files): `builtin_agents.golden`
- `internal/app/` (4 files): `planner.go`, `planner_test.go`, `service.go`, `service_test.go`
- `internal/cli/` (3 files): `doc.go`, `root.go`, `root_test.go`
- `internal/docker/` (6 files): `args.go`, `args_test.go`, `client.go`, `client_test.go`, `doc.go`, `platform.go`
- `internal/hostconfig/` (4 files): `resolver.go`, `resolver_edges_test.go`, `resolver_test.go`, `tree.go`
- `internal/plan/` (2 files): `run.go`, `run_test.go`
- `internal/project/` (2 files): `mount.go`, `mount_test.go`
- `internal/runtime/` (5 files): `Dockerfile`, `assets.go`, `caged-entrypoint.sh`, `image.go`, `runtime_test.go`
- `internal/system/` (2 files): `system.go`, `system_test.go`
