# caged — Architecture & Requirements Spec

**Status:** Draft v0.5. Based on the repo at commit `1e55ce4` ("refactor"). **Primary platform: macOS (Docker Desktop).**
**Legend:** **[DONE]** implemented and matches intent. **[PARTIAL]** exists but differs from intent. **[MISSING]** not built. **[OPEN]** needs your decision (Section 12). **[VERIFY]** needs checking against the real tool before we rely on it.

---

## 1. Purpose

`caged` is a CLI that launches AI coding CLIs (Claude Code, Codex, and others) inside a disposable Docker container, so they run isolated from the host while working on the user's current project, using the user's existing tool configs.

**One-line pitch:** *Run any AI coding CLI in a cage, in the folder you're already in, with the settings you already have.*

## 2. Your Intended Workflow (source of truth)

1. `caged` opens a lightweight container.
2. `caged claude`, `caged codex` opens the container with that CLI.
3. `caged codex 'prompt'` opens that CLI and starts it with the prompt.
4. One Docker image has every supported CLI installed.
5. If a CLI's config exists on the host, mount it so changes in the container persist on the host. If it doesn't exist, don't mount, and the CLI starts fresh in the container.
6. `caged` always opens in the directory it was called from, and mounts it so code changes land on the host.
7. Later: GitHub CLI and other harnesses/tools.

## 2A. Decisions Log

| Topic | Decision |
|---|---|
| Platform | macOS primary, Docker Desktop only. Linux best-effort. Windows dropped |
| Language and build | Go. Image built locally from the embedded Dockerfile on first run |
| Config mounting | **All** supported CLIs' host configs are mounted **read-write on every run**, so every CLI is ready and changes persist on the host. Missing configs are skipped |
| Project mount | Current folder at `/workspace/<folder-name>`, read-write |
| After a CLI exits | Drop into a shell in the same container. `exit` removes the container |
| Logins | API keys via `--env`; subscription logins done once inside the container and persisted by the mount. Keychain extraction deferred |
| Flag passthrough | Not in v1 |
| Prompt launch | `caged <cli> 'prompt'` is wanted (not built yet) |
| Ports | Out of scope |
| Distribution | v1: `go install` and GitHub release binaries. Later: Homebrew formula you maintain |
| Refactor | Free to restructure and change flags (unreleased project) |
| Later | GitHub CLI and more harnesses |

## 3. Current State vs. Intent

This is the most important section for the cleanup. Based on the repo today:

| # | Intent | Current behavior | Verdict |
|---|---|---|---|
| 1 | `caged` opens a container | Opens interactive bash in `caged-runtime:latest` (Ubuntu 24.04, all CLIs installed) | **[DONE]** |
| 2 | `caged <cli>` launches that CLI | Works for `claude`, `pi`, `codex`, `gemini`. After the agent exits you are dropped into a bash shell in the same container | **[DONE]**, but the return-to-shell behavior was never in your description. **[OPEN Q4]** |
| 3 | `caged codex 'prompt'` | CLI accepts **at most one argument** (`cobra.MaximumNArgs(1)`). No prompt support at all | **[MISSING]** |
| 4 | One image with all CLIs | `caged-runtime:latest`, built from an embedded Dockerfile on first run | **[DONE]** |
| 5a | Config exists on host -> mount it, **changes persist** | Only **Pi** is mounted read-write. Claude, Codex and Gemini configs are **copied to a temp dir and mounted read-only**; changes are thrown away on exit | **[PARTIAL]**. This is the biggest gap |
| 5b | No host config -> don't mount, fresh install | Missing paths are skipped and the CLI starts fresh. Nothing is written to the host | **[DONE]** |
| 6 | Current directory mounted | Mounted read-write at a fixed `/workspace`. `--workdir` and `--no-mount` flags exist. Refuses `/` and bare `$HOME` unless `--allow-broad-mount` | **[PARTIAL]**: works, but the container path should become `/workspace/<project-folder-name>` (FR-8) |
| 7 | GitHub CLI and more later | Not installed. `~/.gitconfig` is already passed through | **[MISSING]** (planned later) |

### Answers to my earlier questions that the repo already gives us

| My earlier question | What the repo says | Treat as decided? |
|---|---|---|
| Language | Go 1.23, Cobra | Yes |
| Image distribution | Built locally from an embedded Dockerfile on first run (`go:embed`). `caged build [--no-cache]` rebuilds | Yes, unless you want a registry later |
| Fresh-install login persistence | Ephemeral. No named volumes, by design | Yes (see Q1 for the Claude/macOS wrinkle) |
| Which configs get mounted | Configs for **all** agents on every run | **Decided:** keep. All configs are always available so you can switch CLIs |
| Container lifetime | `docker run --rm`, one new container per run | Yes |
| Project path in container | Fixed `/workspace` | **Decided:** `/workspace/<project-folder-name>` (FR-8) |
| Safety on `~` and `/` | Refused by default | Yes |
| Env vars | Names-only passthrough (`-e NAME`), per-agent defaults plus `--env NAME` | Mostly, see Q7 |

## 3A. macOS Is the Primary Platform

**Decided:** macOS is the primary target and **Docker Desktop is the supported runtime for v1** (Apple Silicon first, Intel supported). Linux is best-effort. Windows is out of scope, so the Windows branch in `buildArgs` can go. OrbStack and Colima may work but are untested and unsupported.

The repo was written Linux-first, and several things behave differently on a Mac. Items marked **[VERIFY]** should be tested on a real Mac before we build on them.

| # | macOS issue | Why it matters | Proposed handling |
|---|---|---|---|
| M1 | **Claude Code keeps its login in the macOS Keychain** (service `Claude Code-credentials`). The file `~/.claude/.credentials.json` is only the Linux location | Mounting `~/.claude` carries settings, history and MCP config, but **not the login**. The container will ask you to log in | See "Auth strategy" below |
| M2 | **Codex** stores its login in `~/.codex/auth.json` **or** the OS keyring, depending on `cli_auth_credentials_store` (`file`, `keyring`, `auto`) | If your host Codex uses keyring/auto on a Mac, `~/.codex` may have no `auth.json` to carry over **[VERIFY which one yours uses]** | Same options as M1; Codex also supports `codex login --device-auth` |
| M3 | **Gemini** credentials: `~/.gemini/oauth_creds.json` today | **[VERIFY]** that newer versions don't use the Keychain. `GEMINI_API_KEY` works either way | Same options as M1 |
| M4 | **Browser login inside the container does not work well.** Login flows start a callback server on `localhost` (Codex uses port 1455) and try to open a browser. Neither reaches your Mac without publishing ports, and there's no `open` in the container | Fresh-install logins are painful, which strengthens the case for forwarding auth from the host | Prefer device-code flow, API keys/tokens, or host-side extraction |
| M5 | **Docker runtime differs** between Docker Desktop, OrbStack and Colima | Behavior varies | **Decided:** Docker Desktop only for v1. Error messages should say "start Docker Desktop" |
| M6 | **UID/GID mapping.** Today caged passes `--user 501:20` on macOS whenever configs are mounted. UID 501 doesn't exist in the image, which is likely why the image does `chmod 0777` on `/home/caged` and config dirs | On Docker Desktop and OrbStack, bind-mount ownership is translated automatically, so `--user` is probably unnecessary on a Mac **[VERIFY per runtime, Colima especially]** | Run as the image's `caged` user on macOS; remove the `0777` workaround if tests pass |
| M7 | **Apple Silicon = `linux/arm64` image.** Every installer must publish arm64 builds, and the image must not silently fall back to amd64 emulation | Slow or broken installs | Build locally (already the case) so the arch matches the host. Check each installer on arm64 |
| M8 | **Single-file bind mount of `~/.claude.json`.** If Claude replaces the file via write-temp-then-rename, renaming over a bind-mounted file fails with "device or resource busy" **[VERIFY]** | Config writes fail, or onboarding state is lost | Test. Alternative: point `CLAUDE_CONFIG_DIR` at the mounted `~/.claude` dir so the JSON lives inside a directory mount (trade-off: no longer shared with the host's `~/.claude.json`) **[VERIFY]** |
| M9 | **Docker Desktop shares only some host paths by default** (`/Users`, `/Volumes`, `/private`, `/tmp`, `/var/folders`) | A project on another path gives a confusing mount error | Detect and print a hint |
| M10 | **Bind-mount file I/O is slower** on macOS than on Linux, especially for huge `node_modules` trees | Not a blocker, but sets expectations | Document it |
| M11 | **`localhost` in the container is not your Mac.** A dev server started by the agent isn't reachable from the host | Common surprise | **Decided:** port publishing is out of scope. Document the limitation |
| M12 | **Distribution:** Go cross-compiles to `darwin-arm64` and `darwin-amd64` | Natural fit for a Homebrew tap | **Decided:** Homebrew is the eventual install path, maintained by you. v1 ships via `go install` / GitHub release binaries (Q13) |

### Auth strategy options (the main macOS decision)

Config mounting alone gives a logged-in session on Linux but not on a Mac, so we need a deliberate auth story per CLI.

| Option | How it works | Pros | Cons |
|---|---|---|---|
| **A. Env token / API key** | Forward `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GEMINI_API_KEY` (already supported). Claude also has a long-lived token flow (`claude setup-token`, then `CLAUDE_CODE_OAUTH_TOKEN`) **[VERIFY]** | Simple, no secrets on disk, works the same on every OS | API keys bill per token instead of using your subscription. Subscription users need the token flow |
| **B. Extract from Keychain at launch** | caged runs `security find-generic-password -s "Claude Code-credentials" -w`, writes the result to a temp file, and mounts it as `.credentials.json` in the container. Community tools already do this | Uses your existing subscription login, no second login | macOS shows a Keychain prompt. The container may refresh the token, and the refreshed token is **not written back to the Keychain**. If tokens rotate, your host session could end up invalidated **[VERIFY]**. The secret briefly lives in a temp file |
| **C. Log in once inside the container** | Container writes `.credentials.json` into the mounted `~/.claude` (rw), so it persists | No Keychain handling, fits the "mount rw, persist changes" model | Browser login is awkward (M4). Leaves a plaintext credential file in `~/.claude` on your Mac, and it's a separate login from the host's Keychain one |

**Decided for v1: ship A + C, defer B.**
- Mount host configs read-write (settings, history, MCP, and any credential file already there).
- Forward API keys by name (`--env`, already built).
- For subscription logins on a Mac: log in once inside the container. The login is written into the mounted `~/.claude` (or `~/.codex`, `~/.gemini`) and **persists across runs**, so it is a one-time step per CLI. The README must say this plainly.
- **Keychain extraction (B) is deferred** to a later release, since it adds a macOS prompt and token-refresh risk.
- **Known trade-off to document:** the credential file sits in plaintext in `~/.claude` on your Mac. Your host Claude ignores it (it uses the Keychain), so the container login is a separate session.
- **[VERIFY] before release:** that each CLI's headless login works from inside the container. Claude Code should print a URL and accept a pasted code. Codex has `codex login --device-auth`. Gemini may need `NO_BROWSER=true`.

## 4. Goals and Non-Goals

**Goals**
1. One command to start a sandboxed session in the current directory.
2. One image with every supported CLI.
3. Reuse existing host configs and persist changes back to the host.
4. Fresh start inside the container when there is no host config, without touching the host.
5. Adding a CLI or tool later is a small, local change.


**Non-goals (v1)**
- A hardened security boundary against a hostile agent. It limits blast radius; it is not a VM. (Section 9)
- Remote or cloud execution, GUI, multi-session orchestration.
- Port publishing, OrbStack/Colima/Windows support, and Keychain extraction (all deferred).
- Local-LLM wiring and a permanent `caged` config file (the repo already defers these).

## 5. User-Facing Interface

### 5.1 Commands

| Command | Behavior | Status |
|---|---|---|
| `caged` | Interactive shell in the container; all CLIs available to run | **[DONE]** |
| `caged <cli>` | Launch that CLI | **[DONE]** |
| `caged <cli> '<prompt>'` | Launch the CLI with an initial prompt | **[DONE]** |
| `caged build [--no-cache]` | Build/rebuild the runtime image | **[DONE]** |
| `caged list` | List supported CLIs | **[DONE]**; should also show whether host config was detected. **[OPEN Q12]** |

### 5.2 Flags (existing)

`--no-mount`, `--workdir/-w`, `--allow-broad-mount`, `--no-config`, `--verbose`, `--env NAME` (repeatable).

### 5.3 Supported CLIs

| CLI | In image | Host config paths | Prompt launch style |
|---|---|---|---|
| claude | yes | `~/.claude/`, `~/.claude.json` | positional **[VERIFY]** |
| codex | yes (wrapped with `--no-daemon`) | `~/.codex/` | `exec <prompt>` |
| gemini | yes | `~/.gemini/` | `-i <prompt>` |
| pi | yes | `~/.pi/`, `~/.config/pi/` | positional |
| copilot (GitHub Copilot CLI) | yes | `~/.copilot/` | `-p <prompt>` |
| gh (GitHub CLI) | no | `~/.config/gh/` | n/a, later |

Shared across all: `~/.gitconfig`.

The prompt style differs per tool, so the registry needs a per-agent "how to pass an initial prompt" field. A single hard-coded rule will break for at least one of them.

## 6. Functional Requirements

### 6.1 Container and image
| ID | Requirement | Status |
|---|---|---|
| FR-1 | Single image containing all CLIs | DONE |
| FR-2 | Image auto-built on first run if missing | DONE |
| FR-3 | Container runs as non-root user `caged`. Linux: host UID/GID mapped. macOS: mapping probably unnecessary (see M6) | PARTIAL |
| FR-4 | Container is interactive (TTY) and removed on exit | DONE |
| FR-5 | CLI versions are reproducible or deliberately updatable (`caged build --no-cache`, or pinned versions). Today the Dockerfile uses unpinned `curl \| bash` installers | PARTIAL **[OPEN Q10]** |
| FR-6 | Clear error when Docker is installed but the daemon is not running (today only "docker not found" is checked). On macOS the message should say to start Docker Desktop / OrbStack / Colima | PARTIAL |

### 6.2 Project mount
| ID | Requirement | Status |
|---|---|---|
| FR-7 | CWD bind-mounted read-write; container workdir set to it | DONE (path changes per FR-8) |
| FR-8 | The project mounts at `/workspace/<name of the current folder>`, e.g. running in `~/code/my-app` gives `/workspace/my-app`. The container's working directory is that path | **MISSING** (today: fixed `/workspace`). Two folders with the same name are separate runs, so no clash |
| FR-9 | Refuse `/` and `$HOME` unless explicitly overridden | DONE |
| FR-10 | Files created are owned by the host user | DONE |

### 6.3 Config mounting and persistence (the core gap)

Target logic, per supported CLI, per launch:

```
if host config path exists (and resolves inside $HOME):
    bind-mount READ-WRITE at the same path under /home/caged
    -> container changes persist on the host
else:
    mount nothing; CLI starts fresh from the image
    -> nothing is written to the host
```

| ID | Requirement | Status |
|---|---|---|
| FR-11 | Existing host config is mounted **read-write** for every supported CLI so changes persist on the host | **MISSING** (only Pi today) |
| FR-12 | Missing host config is skipped and never created on the host | DONE |
| FR-13 | Symlinks that resolve outside `$HOME` are refused; sockets and special files are never exposed | DONE (in the staging copy path; must be re-checked once staging is removed) |
| FR-14 | `--no-config` disables all config mounts | DONE |
| FR-15 | `--no-auth` keeps credential files out of the container | DONE for staged copies; **needs redefinition for direct mounts** (see below) |
| FR-16 | Every run mounts the existing config for **all** supported CLIs (plain `caged` included), so you can switch CLIs from the shell | DONE (decided) |
| FR-17 | Honor config-dir env overrides (`CLAUDE_CONFIG_DIR`, `CODEX_HOME`, ...) | MISSING **[VERIFY]** |

**Consequence of FR-11 to plan for:** a direct read-write bind mount cannot hide individual files. The current `--no-auth` works by copying a config dir and leaving the credential file out. With direct mounts, `--no-auth` would have to mean "don't mount that tool's config at all" (coarser), or keep the copy-and-exclude path as a separate mode. **[OPEN Q5]**

Two more things to be aware of with direct mounts:
- Claude's `~/.claude.json` is a **single file**. Single-file bind mounts can break when a tool replaces the file instead of editing it in place (a write-temp-then-rename pattern). Needs testing. **[VERIFY]**
- On **macOS, Claude Code keeps its login in the Keychain**, not in `~/.claude`, so a mounted config will not carry the login into the container. See Section 3A (M1 and the auth strategy options).

### 6.4 Launching CLIs and prompts
| ID | Requirement | Status |
|---|---|---|
| FR-18 | `caged <cli>` starts that CLI in the project dir | DONE |
| FR-19 | `caged <cli> '<prompt>'` starts it with the prompt, passed verbatim (no shell re-interpretation) | MISSING |
| FR-20 | Registry declares how each CLI accepts an initial prompt | MISSING |
| FR-21 | Passing extra flags through to the CLI (`caged claude --resume`) | **Out of scope for v1** (decided). Start the CLI, then use its own flags from the shell if needed |
| FR-22 | After the CLI exits, drop into a shell in the same container; `exit` ends it | DONE (decided) |

### 6.5 Environment and credentials
| ID | Requirement | Status |
|---|---|---|
| FR-23 | Env vars forwarded by name only, never printed; only if set on host | DONE |
| FR-24 | Per-agent default list plus `--env NAME` | DONE (Pi and Codex each default to 4 API keys, see Q7) |
| FR-25 | `~/.gitconfig` available in container | DONE (mounted via every agent, read-only copy) |
| FR-26 | SSH keys, cloud creds, Docker socket never mounted | DONE |
| FR-27 | Warning printed when credentials are available in the container | DONE |

### 6.6 Extensibility and later items
| ID | Requirement | Status |
|---|---|---|
| FR-28 | Adding a CLI = one registry entry + one image install step; no core-logic change | PARTIAL: the registry is data, but `Writable: agent.Name == "pi"` is hard-coded in the resolver, and the install step is hand-edited in a single `RUN` line |
| FR-29 | GitHub CLI installed in image, auth via `~/.config/gh` mount or token | MISSING (later) |
| FR-30 | Other harnesses/tools via the registry | MISSING (later) |

## 7. Non-Functional Requirements

| ID | Requirement |
|---|---|
| NFR-1 | After the first build, startup takes seconds |
| NFR-2 | Host needs Docker Desktop for Mac. **macOS (Apple Silicon + Intel) is primary**, Linux best-effort, Windows out of scope for v1 |
| NFR-3 | Actionable errors for: no Docker, daemon down, build failure, bad paths |
| NFR-4 | `--verbose` shows exactly what is mounted and which env names are passed (never values). A `--dry-run` that prints the final `docker run` is easy to add given a pure planner |
| NFR-5 | No leftovers: no stray containers, temp dirs always cleaned |
| NFR-6 | Core logic (parsing, resolving, planning) is unit-testable without Docker |
| NFR-7 | v1: `go install` / `make install` plus GitHub release binaries (`darwin-arm64`, `darwin-amd64`). Later: a Homebrew formula you maintain, which means tagged releases and a version string wired in via `-ldflags` (the Makefile already does this) |

## 8. Architecture

### 8.1 Current structure

```
cmd/caged/main.go            entrypoint, exit-code passthrough
internal/cli/root.go         Cobra commands + flags + ALSO orchestration + mount safety + logging
internal/config/agents.go    agent registry + env logic + config resolution/policy + path safety
internal/config/runtime.go   RuntimeImage const
internal/config/defaults.go  DefaultImage const (unused)
internal/container/run.go    docker arg building + run
internal/container/config_stage.go   copy-to-temp staging of configs
internal/container/runtime.go        go:embed + image build/exists
internal/container/runtime/Dockerfile, caged-entrypoint.sh
Dockerfile (repo root)       DEV image (golang), unrelated to the runtime image
```

### 8.2 Pipeline (this part of the design is good, keep it)

```
 caged codex 'fix tests'
        |
  [CLI parse] -> intent {agent, prompt, flags}
        |
  [Registry] -> agent definition (command, config paths, env names, prompt style)
        |
  [Config resolver] -> which host paths exist/are safe -> mounts + report
        |
  [Project mount] -> cwd validated -> /workspace mount (or none)
        |
  [Run planner] -> immutable RunSpec {image, mounts, env, user, command}   (pure)
        |
  [Docker runner] -> ensure image, `docker run -it --rm ...`, pass exit code
```

Only the last box talks to Docker. Everything before it is pure and testable.

### 8.3 Proposed structure after cleanup

```
cmd/caged/                 main only
internal/cli/              Cobra wiring ONLY: parse args/flags -> call planner -> call runner
internal/agent/            registry: Agent{Name, Command, PromptStyle, ConfigPaths, EnvVars}  (data only)
internal/hostconfig/       resolver: (home, agents, options) -> Plan{Mounts, Report}   (policy lives here)
internal/project/          project-dir mount + safety checks (/, $HOME)
internal/plan/             builds RunSpec from the above (pure)
internal/docker/           Run, Build, ImageExists, buildArgs, daemon check
internal/runtime/          embedded Dockerfile + entrypoint (+ later install scripts)
dev/Dockerfile             the dev image, moved out of the repo root
```

### 8.4 Registry shape (target)

```go
type Agent struct {
    Name        string
    Command     []string
    Description string
    Prompt      PromptStyle   // positional | flag("-i") | none
    ConfigPaths []ConfigPath  // each: host, container, mode(rw|ro), secrets
    EnvVars     []string
}
```

Mode (rw or ro) becomes **data on each path**, not an `agent.Name == "pi"` check.

## 9. Security Considerations

| Risk | Stance |
|---|---|
| Container can modify or delete anything in the project dir | Accepted by design. Recommend git |
| With FR-11, the container can modify the host's real CLI configs and read their tokens | Accepted by design for CLIs that exist. This is a **behavior change from today** (read-only copies). Make it explicit in the README and in the startup warning |
| Docker socket, SSH keys, cloud creds | Never mounted |
| Privileged mode or extra capabilities | Never |
| Network | Full access (agents need it) |
| Non-root | Yes, host UID/GID mapped |
| Image supply chain | Installers are `curl \| bash` from third-party URLs, unpinned |
| macOS Keychain extraction (auth option B) | Reads a secret out of the Keychain and places it in a temp file and a container. Must be opt-in, use a `0600` temp file, clean up on exit, and never print the value |
| `chmod 0777` on `/home/caged` and config dirs in the image | Works around arbitrary UIDs; acceptable for a throwaway container but worth a comment |

## 10. Cleanup and Refactor Plan

Findings from reading the code, roughly in priority order.

**Behavior gaps (do first)**
1. Add prompt support (FR-19/20), including a per-agent prompt style.
2. Make config mounts read-write for every agent (FR-11) and rework `--no-auth` accordingly.
3. Re-test Claude's single-file `~/.claude.json` mount and the macOS Keychain case.

**Structure**
4. `internal/cli/root.go` does too much: flag parsing, mount safety (`resolveProjectMount`, `newMount`), orchestration (`runContainer`, `configureMounts`) and logging. Reduce it to wiring.
5. `internal/config` mixes four jobs: the agent registry, env-var policy, config-resolution policy, and path-safety helpers. Split into `agent` and `hostconfig`.
6. `container.Options` imports `config.ResolvedMount`. The runner should depend on a neutral `RunSpec`, not on the config package.
7. Hard-coded `Writable: agent.Name == "pi"` becomes per-path data.
8. Auth handling uses two mechanisms (`IsAuth` on a path for Claude's `.claude.json`, `AuthExcludes` patterns for the rest). Unify.
9. `.gitconfig` is repeated in every agent's list and de-duplicated afterwards. Make it a shared path.
10. `ResolveAllConfigs` is a wrapper used only by tests. Remove it or keep only one resolver entry point.

**Dead and stale code**
11. `config.DefaultImage` is never used. `config/runtime.go` holds one constant. Fold into one place.
12. Three `doc.go` files say the package "will contain" things. Update them.
13. Root `Dockerfile` is the Go dev image, which is confusing next to `internal/container/runtime/Dockerfile`. Move to `dev/`.
14. README examples use `go run ./cmd/caged` instead of the installed `caged`.

**Image**
15. Pin or at least record installed CLI versions. Split the one long `RUN` so a single CLI update doesn't invalidate the whole layer cache.
16. The `rm -f /home/caged/.claude.json` and the `codex` wrapper script (`--no-daemon`) need comments explaining why they exist.
17. Check the daemon-running case, not just that `docker` is on the PATH.

**macOS**
18. Remove the Windows branch in `buildArgs` and the `goos` plumbing it needs. Revisit the macOS `--user` logic (M6).
19. Add a per-agent auth strategy to the registry (env token, Keychain extraction, file) instead of assuming every login is a file under the home directory.
20. Add the arm64 check (M7) and the Docker Desktop shared-path hint (M9).

## 11. Out of Scope Now, Planned Later

GitHub CLI and extra harnesses, Homebrew formula, Keychain extraction (auth option B), `caged update`, per-project overrides, network restrictions or resource limits, port publishing, other Docker runtimes, local-LLM wiring.

## 12. Open Questions

Ordered by how much they change the design.

1. **Persistence and auth.** Confirm the v1 plan in 3A: read-write mounts for every CLI that has host config, API keys via `--env`, and subscription users log in once inside the container (persisting through the mount). Is a plaintext credential file in `~/.claude` acceptable?
2. ~~Project path~~ **Decided:** `/workspace/<project-folder-name>`. (Claude Code's per-path history is then separate per project name, which also avoids the collision I was worried about.)
3. ~~Which configs to mount~~ **Decided:** all, on every run.
4. ~~After the CLI exits~~ **Decided:** drop into a shell.
5. **`--no-auth`.** *Assumed default:* remove it. With direct read-write mounts it can only mean "don't mount configs", which `--no-config` already does.
6. ~~Passing flags to the CLI~~ **Decided:** not needed for v1. (The `caged <cli> 'prompt'` form is still wanted.)
7. **Env defaults.** *Assumed default:* keep today's per-agent lists plus `--env NAME`.
8. **Permission modes.** *Assumed default:* not in v1. Each CLI starts in its normal mode.
9. **Scope of v1 CLIs.** Keep Claude, Codex, Gemini, Pi, and GitHub Copilot CLI. GitHub CLI (`gh`) remains later.
10. **Version policy.** *Assumed default:* latest at build time. `caged build --no-cache` is the update path. No `caged update` in v1.
11. ~~Platforms~~ **Decided:** macOS primary, Linux best-effort, Windows dropped.
12. **`caged list`.** *Assumed default:* unchanged for v1. Showing detected configs is a later nicety.
13. **Distribution.** Homebrew later is decided. For the first public release, are GitHub release binaries enough, or do you want the tap from day one?
14. **GitHub CLI.** *Assumed default:* later. When it arrives, install it and mount `~/.config/gh` read-write like the others.
15. **Refactor freedom.** *Assumed default:* yes. The project is unreleased (9 commits), so the cleanup may rename and move code and change or remove flags freely.
16. ~~Docker runtime~~ **Decided:** Docker Desktop only.
17. ~~Ports~~ **Decided:** out of scope.
