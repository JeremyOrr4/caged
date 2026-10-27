# caged — Architecture & Requirements Spec

**Status:** v1.0. Describes caged 0.1.0. **Primary platform: macOS (Docker Desktop).**
**Legend:** **[DONE]** implemented and matches intent. **[PARTIAL]** exists but differs from intent. **[MISSING]** not built (planned later). **[OPEN]** is still undecided (Section 12). **[VERIFY]** needs checking against the real tool before we rely on it.

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
| Caged's own config | `config.yaml` is created on first use (commented examples only) and its folder is mounted read-write at `/home/caged/.config/caged`. Edits are shown as a diff and need approval on the next run (FR-30 to FR-35) |
| Project mount | Current folder at `/workspace/<folder-name>`, read-write |
| After a CLI exits | Drop into a shell in the same container. `exit` removes the container |
| Logins | API keys via `--env`; subscription logins done once inside the container and persisted by the mount. Keychain extraction deferred |
| Flag passthrough | `--` passes arguments to the agent unchanged, after the prompt |
| Prompt launch | `caged <cli> 'prompt'` starts the CLI with the prompt, in that CLI's prompt style |
| Image freshness | Hash label; rebuild automatically; unlabeled = stale |
| Image extensions | User-level `config.yaml` (`image.apt`), apt only, tag `caged-runtime:ext` |
| Ports | Opt-in `-p`/`--publish`, bound to 127.0.0.1; no host address or ranges |
| Distribution | GitHub release binaries, `go install`, and a Homebrew tap (formula built from source) from v0.1.0 |
| Refactor | Restructuring keeps behaviour: flags, help and output text, exit codes, docker arguments and file formats stay as they are (the golden tests in `cmd/caged/testdata` pin them) |
| Later | GitHub CLI and more harnesses |

## 3. Implemented

caged 0.1.0 does what the intended workflow in Section 2 describes:

- `caged` opens a shell in `caged-runtime:latest` (Ubuntu 24.04, pinned by digest, with every supported CLI);
  `caged <cli>` launches that CLI, `caged <cli> 'prompt'` launches it with a prompt, and after the CLI exits you are
  in a shell in the same container.
- The project folder is mounted read-write at `/workspace/<folder-name>`; `/` and the bare home directory are refused
  unless `--allow-broad-mount`. A project or config path containing `:` is refused, because Docker's `-v` splits at it.
- The host configs of **all** supported CLIs are mounted read-write on every run, so every CLI is ready and changes
  persist. Missing configs are skipped; unsafe paths are refused. The GitHub CLI (`gh`) is included.
- One image holds all CLIs. It is built locally from an embedded Dockerfile, labeled with a hash of its build files,
  rebuilt automatically when that hash changes, and extendable with apt packages from `image.apt` in a user-level `config.yaml`.
- Ports (`-p`), flag passthrough (`--`), `caged doctor`, `caged list`, and `caged clean` exist (Section 5).
- Releases are tagged, with GitHub binaries and a Homebrew tap (NFR-7).

### Decisions taken from the code

| Question | Answer |
|---|---|
| Language | Go (the newest stable release), Cobra |
| Image distribution | Built locally from an embedded Dockerfile on first run (`go:embed`). `caged build [--no-cache]` rebuilds |
| Fresh-install login persistence | Ephemeral. No named volumes, by design |
| Which configs get mounted | Configs for **all** agents on every run, so you can switch CLIs |
| Container lifetime | `docker run --rm`, one new container per run |
| Project path in container | `/workspace/<project-folder-name>` (FR-8) |
| Safety on `~` and `/` | Refused by default |
| Env vars | Names-only passthrough (`-e NAME`), per-agent defaults plus `--env NAME` |

## 3A. macOS Is the Primary Platform

**Decided:** macOS is the primary target and **Docker Desktop is the supported runtime for v1** (Apple Silicon first, Intel supported). Linux is best-effort. Windows is out of scope, and caged has no Windows code path. OrbStack and Colima may work but are untested and unsupported.

The repo was written Linux-first, and several things behave differently on a Mac. Items marked **[VERIFY]** should be tested on a real Mac before we build on them.

| # | macOS issue | Why it matters | Proposed handling |
|---|---|---|---|
| M1 | **Claude Code keeps its login in the macOS Keychain** (service `Claude Code-credentials`). The file `~/.claude/.credentials.json` is only the Linux location | Mounting `~/.claude` carries settings, history and MCP config, but **not the login**. The container will ask you to log in | See "Auth strategy" below |
| M2 | **Codex** stores its login in `~/.codex/auth.json` **or** the OS keyring, depending on `cli_auth_credentials_store` (`file`, `keyring`, `auto`) | If your host Codex uses keyring/auto on a Mac, `~/.codex` may have no `auth.json` to carry over **[VERIFY which one yours uses]** | Same options as M1; Codex also supports `codex login --device-auth` |
| M3 | **Gemini** credentials: `~/.gemini/oauth_creds.json` today | **[VERIFY]** that newer versions don't use the Keychain. `GEMINI_API_KEY` works either way | Same options as M1 |
| M4 | **Browser login inside the container does not work well.** Login flows start a callback server on `localhost` (Codex uses port 1455) and try to open a browser. Neither reaches your Mac without publishing ports, and there's no `open` in the container | Fresh-install logins are painful, which strengthens the case for forwarding auth from the host | Prefer device-code flow, API keys/tokens, or host-side extraction |
| M5 | **Docker runtime differs** between Docker Desktop, OrbStack and Colima | Behavior varies | **Decided:** Docker Desktop only for v1. Error messages should say "start Docker Desktop" |
| M6 | **UID/GID mapping.** On Linux caged passes `--user <host UID>:<host GID>`, a user that does not exist in the image, which is why the image does `chmod 0777` on `/home/caged` and the config dirs | On Docker Desktop, bind-mount ownership is translated automatically, so `--user` is unnecessary on a Mac **[VERIFY on other runtimes, Colima especially]** | **Done:** macOS keeps the image's `caged` user (no `--user`); the `0777` workaround stays for Linux |
| M7 | **Apple Silicon = `linux/arm64` image.** Every installer must publish arm64 builds, and the image must not silently fall back to amd64 emulation | Slow or broken installs | Build locally (already the case) so the arch matches the host. Check each installer on arm64; checked by `caged doctor` |
| M8 | **Single-file bind mount of `~/.claude.json`.** If Claude replaces the file via write-temp-then-rename, renaming over a bind-mounted file fails with "device or resource busy" | Config writes fail, or onboarding state is lost | **Done:** `CLAUDE_CONFIG_DIR` points at the mounted `~/.claude` dir, so the JSON lives inside a directory mount; the host's `~/.claude.json` is not shared |
| M9 | **Docker Desktop shares only some host paths by default** (`/Users`, `/Volumes`, `/private`, `/tmp`, `/var/folders`) | A project on another path gives a confusing mount error | Detect and print a hint; checked by `caged doctor` |
| M10 | **Bind-mount file I/O is slower** on macOS than on Linux, especially for huge `node_modules` trees | Not a blocker, but sets expectations | Document it |
| M11 | **`localhost` in the container is not your Mac.** A dev server started by the agent isn't reachable from the host | Common surprise | **Decided:** `-p` publishes on 127.0.0.1; servers must listen on 0.0.0.0. Documented in the README |
| M12 | **Distribution:** Go cross-compiles to `darwin-arm64` and `darwin-amd64` | Natural fit for a Homebrew tap | **Decided:** a Homebrew tap (`JeremyOrr4/homebrew-tap`, formula built from source) from v0.1.0, plus GitHub release binaries (GoReleaser) and `go install` (Q13) |

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
- OrbStack/Colima/Windows support, and Keychain extraction (all deferred).
- Local-LLM wiring. The permanent `caged` config file is `~/.config/caged/config.yaml`, which holds `image.apt`, the symlink policy (`links`), and `workdirs`; no other settings are read from a file. The file is created on first use and its folder is mounted read-write (FR-30 to FR-35).

## 5. User-Facing Interface

### 5.1 Commands

| Command | Behavior | Status |
|---|---|---|
| `caged` | Interactive shell in the container; all CLIs available to run | **[DONE]** |
| `caged <cli>` | Launch that CLI | **[DONE]** |
| `caged <cli> '<prompt>'` | Launch the CLI with an initial prompt | **[DONE]** |
| `caged build [--no-cache]` | Build/rebuild the runtime image | **[DONE]** |
| `caged list` | List supported CLIs, the host config detected for each, and which auth environment variables are set (names only) | **[DONE]** |
| `caged doctor [-w DIR]` | Check Docker, the images, the config file, the symlinked config, the image architecture, the project mount, and the CLI versions in the image; exits 1 when a check fails | **[DONE]** |
| `caged clean [--dry-run]` | Remove caged's Docker images (extension, runtime, replaced, and older unlabeled ones); never touches containers, volumes, the build cache, or config | **[DONE]** |

### 5.2 Flags (existing)

`--no-mount`, `--workdir/-w`, `--allow-broad-mount`, `--no-config`, `--trust-config`, `--verbose`, `--env NAME` (repeatable), `-p/--publish PORT|HOST_PORT:CONTAINER_PORT` (repeatable), `--version/-v`, `--help/-h`. `caged build` takes `--no-cache` and `--trust-config`, `caged doctor` takes `-w`, and `caged clean` takes `--dry-run`.

### 5.3 Supported CLIs

| CLI | In image | Host config paths | Prompt launch style | Example with flags |
|---|---|---|---|---|
| claude | yes | `~/.claude/`, `~/.claude.json` | positional **[VERIFY]** | `caged claude 'fix tests' -- --model opus` |
| codex | yes (wrapped with `--no-daemon`) | `~/.codex/` | `exec <prompt>` | `caged codex 'fix tests' -- --model o4` (`exec` flags) |
| gemini | yes | `~/.gemini/` | `-i <prompt>` | `caged gemini 'explain' -- --model gemini-2.5-pro` |
| pi | yes | `~/.pi/`, `~/.config/pi/` | positional | `caged pi 'hi' -- --provider openai` |
| copilot (GitHub Copilot CLI) | yes | `~/.copilot/` | `-p <prompt>` | `caged copilot 'review' -- --model gpt-5` |
| gh (GitHub CLI) | yes | `~/.config/gh/` | available in the interactive shell | `caged gh -- pr list` |

Shared across all: `~/.gitconfig`.

The prompt style differs per tool, so the registry needs a per-agent "how to pass an initial prompt" field. A single hard-coded rule will break for at least one of them.

## 6. Functional Requirements

### 6.1 Container and image
| ID | Requirement | Status |
|---|---|---|
| FR-1 | Single image containing all CLIs | DONE |
| FR-2 | Image built on first run if missing, and rebuilt when its runtime-hash label does not match the binary | DONE |
| FR-3 | Container runs as a non-root user. Linux: the host UID/GID (`--user`). macOS: the image's `caged` user, since Docker Desktop translates ownership (M6) | DONE |
| FR-4 | Container is interactive (TTY) and removed on exit | DONE |
| FR-5 | CLI versions are latest at build time (`caged build --no-cache` to update), recorded in `/opt/caged/versions`, and shown by `caged doctor` | DONE (pinning is deferred) |
| FR-6 | Clear error when Docker is not installed, and when it is installed but the daemon is not running. On macOS the message says to start Docker Desktop | DONE |

### 6.2 Project mount
| ID | Requirement | Status |
|---|---|---|
| FR-7 | CWD bind-mounted read-write; container workdir set to it | DONE (path changes per FR-8) |
| FR-8 | The project mounts at `/workspace/<name of the current folder>`, e.g. running in `~/code/my-app` gives `/workspace/my-app`. The container's working directory is that path. Two folders with the same name are separate runs, so no clash | DONE |
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
| FR-11 | Existing host config is mounted **read-write** for every supported CLI so changes persist on the host (`Writable` on each `agent.ConfigPath`; a Pi tree that needs masks or nested link mounts runs from a staged copy that is written back, FR-13) | DONE |
| FR-12 | Missing host config is skipped and never created on the host (caged's own `config.yaml` is the one exception, FR-30) | DONE |
| FR-13 | Symlinks in agent config are followed on the host and their targets mounted where the links point; a target outside the link's own config folder must be under `links.allow`, and the built-in deny list always wins, for a config folder that is not a symlink too; sockets and special files are never exposed | DONE |
| FR-14 | `--no-config` disables all config mounts | DONE |
| FR-15 | There is no `--no-auth`: with direct read-write mounts it could only mean "mount no configs", which `--no-config` already does | DONE (flag removed) |
| FR-16 | Every run mounts the existing config for **all** supported CLIs (plain `caged` included), so you can switch CLIs from the shell | DONE (decided) |
| FR-30 | The first command that needs config (run, build, doctor, list) creates `~/.config/caged/` (0700) and a starter `config.yaml` (0600, commented examples only) when there is none; `$XDG_CONFIG_HOME/caged/` when `XDG_CONFIG_HOME` is absolute. An existing file is never replaced, appended to, or re-permissioned; concurrent creation is safe; a folder that cannot be created is an actionable warning and the command goes on without it, while a file that cannot be written into an existing folder, or a `config.yaml` that exists but cannot be read, is an actionable error. help, version, clean and `--no-config` never create it | DONE |
| FR-31 | The selected config folder, even outside home, is mounted read-write at `/home/caged/.config/caged` (a directory mount, so atomic saves work); it is not mounted with `--no-config`, and is never in the image or build context | DONE |
| FR-32 | A symlinked config folder or `config.yaml` is followed only when its target is under `links.allow` (`read_only` honored); the built-in deny of caged's config folder (wherever `XDG_CONFIG_HOME` puts it) still refuses links from agent config into it, and the folder itself must not be the home directory, touch a denied path, or overlap the approval state folder (`XDG_CONFIG_HOME` equal to `XDG_STATE_HOME`) | DONE |
| FR-33 | caged keeps the last approved copy of `config.yaml` in `$XDG_STATE_HOME/caged` (never mounted). A changed file is shown as a diff before the command goes on, and needs `y` at a terminal or `--trust-config`; otherwise the command stops | DONE |
| FR-34 | `doctor` and `list` show a pending change but never ask and never approve a change (like run and build, they record the first copy, FR-35); `--no-config` still reviews the file | DONE |
| FR-35 | The first copy caged sees (including the starter file it created) is recorded without asking; when that record cannot be written, `doctor` and `list` warn and go on | DONE |
| FR-17 | Honor config-dir env overrides (`CLAUDE_CONFIG_DIR`, `CODEX_HOME`, `COPILOT_HOME`); an override that resolves outside home or names a denied folder is refused | DONE |

Two more things to be aware of with direct mounts:
- Claude's `~/.claude.json` is a **single file**. Single-file bind mounts can break when a tool replaces the file instead of editing it in place (a write-temp-then-rename pattern). Needs testing. **[VERIFY]**
- On **macOS, Claude Code keeps its login in the Keychain**, not in `~/.claude`, so a mounted config will not carry the login into the container. See Section 3A (M1 and the auth strategy options).

### 6.4 Launching CLIs and prompts
| ID | Requirement | Status |
|---|---|---|
| FR-18 | `caged <cli>` starts that CLI in the project dir | DONE |
| FR-19 | `caged <cli> '<prompt>'` starts it with the prompt, passed verbatim (no shell re-interpretation) | DONE |
| FR-20 | Registry declares how each CLI accepts an initial prompt (`Agent.Prompt`, `Agent.PromptFlag`) | DONE |
| FR-21 | Arguments after `--` are passed to the agent unchanged, after the prompt, never through a shell (`caged claude -- --resume`) | DONE |
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
| FR-28 | Adding a CLI = one registry entry + one image install step; no core-logic change | DONE: one file in `internal/agent` and one line in its `builtin.go`, plus an install step in `internal/images/Dockerfile` (architecture.md, "Adding a new agent") |
| FR-29 | GitHub CLI installed in image, auth via `~/.config/gh` mount or token | DONE |
| FR-36 | Other harnesses/tools via the registry | MISSING (later) |
| FR-37 | Publish container ports on 127.0.0.1 with repeatable `-p`, validated before Docker runs | DONE |
| FR-38 | `caged doctor` runs the checks docker, daemon, image, config, links, ext-image, arch, project and versions in order; a check that depends on a failed one is skipped; it exits 1 only when a check fails and never builds, pulls, or removes anything | DONE |
| FR-39 | `caged clean [--dry-run]` removes caged's images, extension first, never with `-f`; an image in use is kept and reported | DONE |
| FR-40 | Extra apt packages from `image.apt` in a user-level `config.yaml`, built as `caged-runtime:ext` on top of the runtime image, validated before Docker runs | DONE |

## 7. Non-Functional Requirements

| ID | Requirement |
|---|---|
| NFR-1 | After the first build, startup takes seconds |
| NFR-2 | Host needs Docker Desktop for Mac. **macOS (Apple Silicon + Intel) is primary**, Linux best-effort, Windows out of scope for v1 |
| NFR-3 | Actionable errors for: no Docker, daemon down, build failure, bad paths. `caged doctor` reports each of them with its fix |
| NFR-4 | `--verbose` shows exactly what is mounted, which ports are published, and which env names are passed (never values). A `--dry-run` that prints the final `docker run` is easy to add given a pure planner |
| NFR-5 | No leftovers: no stray containers, temp dirs always cleaned, a replaced runtime image is removed, and `caged clean` removes every caged image |
| NFR-6 | Core logic (parsing, resolving, planning) is unit-testable without Docker |
| NFR-7 | `go install`, GitHub release binaries (macOS and Linux, amd64 and arm64), and a Homebrew tap with a formula built from source. Tagged releases wire the version in via `-ldflags`; `go install` falls back to the module version |

## 8. Architecture

The package layout, the dependency rules between packages, and the interfaces are in [architecture.md](architecture.md); an architecture test keeps its dependency table accurate.

### 8.1 Pipeline

The CLI parses the command line into a request; the planner turns it into a `plan.RunSpec` (image, command, project
mount, ports, config mounts, environment names) from the agent registry, the project resolver and the config resolver;
only `internal/docker` turns that spec into `docker` commands, and passes the exit code back. Each step is in
[architecture.md](architecture.md), which follows `caged codex 'fix tests'` from `main` to `docker run`, naming the
function at each step.

### 8.2 Registry shape

```go
type Agent struct {
    Name        string
    Command     []string
    Description string
    ConfigPaths []ConfigPath // each: host path, container path, Writable, Staged, env override, masks
    Prompt      PromptStyle  // none, positional, or a flag
    PromptFlag  string       // the flag (or subcommand, as Codex's exec) before the prompt
    Auth        AuthStrategy // the environment names the agent reads
}
```

Mount mode and staging are **data on each path**; no code checks an agent's name.

## 9. Security Considerations

| Risk | Stance |
|---|---|
| Container can modify or delete anything in the project dir | Accepted by design. Recommend git |
| With FR-11, the container can modify the host's real CLI configs and read their tokens | Accepted by design for CLIs that exist. The README says so, and a warning is printed when credentials are available in the container (FR-27) |
| Docker socket, SSH keys, cloud creds | Never mounted |
| Privileged mode or extra capabilities | Never |
| Network | Full access (agents need it) |
| Published ports | Loopback only, never the LAN |
| Extension packages | apt only, from the image's configured repositories (Ubuntu, NodeSource, GitHub CLI); names validated; user-level file only, a repository cannot change it |
| Non-root | Yes: the host UID/GID on Linux, the image's `caged` user on macOS |
| Image supply chain | Installers are `curl \| bash` from third-party URLs, unpinned |
| macOS Keychain extraction (auth option B) | Reads a secret out of the Keychain and places it in a temp file and a container. Must be opt-in, use a `0600` temp file, clean up on exit, and never print the value |
| `chmod 0777` on `/home/caged` and config dirs in the image | Works around arbitrary UIDs; acceptable for a throwaway container but worth a comment |

## 10. Cleanup and Refactor Plan

The cleanup that this section used to plan was done before 0.1.0: prompt support and per-agent prompt styles, read-write
config mounts for every agent, the split of the old `config` and `container` packages into `agent`, `hostconfig`,
`project`, `plan`, `docker`, and `images`, a neutral `RunSpec`, per-path `Writable` data, one shared `.gitconfig`
entry, removal of the unused `DefaultImage`, the dev Dockerfile in `dev/`, the daemon-running check, the
`--no-auth` removal, the arm64 and File Sharing checks (now in `caged doctor`), and comments in the Dockerfile for
the `.claude.json` removal and the Codex wrapper. The remaining ideas are tracked as planned work:

- Pin CLI versions (they are latest at build time and recorded in `/opt/caged/versions`), and split the long `RUN` so
  one CLI update does not invalidate the whole layer cache.
- A per-agent auth strategy in the registry (env token, Keychain extraction, file), instead of assuming every login is a
  file under the home directory.

## 11. Out of Scope Now, Planned Later

Extra harnesses, homebrew-core, .deb and .rpm packages, signed and notarized binaries, CLI version pinning, Keychain extraction (auth option B), `caged update`, per-project overrides, network restrictions or resource limits, port ranges and host-address binding, per-project extension files, other package managers, a raw Dockerfile fragment, other Docker runtimes, local-LLM wiring.

## 12. Open Questions

Ordered by how much they change the design.

1. **Persistence and auth.** Confirm the v1 plan in 3A: read-write mounts for every CLI that has host config, API keys via `--env`, and subscription users log in once inside the container (persisting through the mount). Is a plaintext credential file in `~/.claude` acceptable?
2. ~~Project path~~ **Decided:** `/workspace/<project-folder-name>`. (Claude Code's per-path history is then separate per project name, which also avoids the collision I was worried about.)
3. ~~Which configs to mount~~ **Decided:** all, on every run.
4. ~~After the CLI exits~~ **Decided:** drop into a shell.
5. ~~`--no-auth`~~ **Decided:** removed. With direct read-write mounts it can only mean "don't mount configs", which `--no-config` already does.
6. ~~Passing flags~~ **Decided:** `--` passthrough, after the prompt.
7. **Env defaults.** *Assumed default:* keep today's per-agent lists plus `--env NAME`.
8. **Permission modes.** *Assumed default:* not in v1. Each CLI starts in its normal mode.
9. **Scope of v1 CLIs.** Keep Claude, Codex, Gemini, Pi, GitHub Copilot CLI, and GitHub CLI (`gh`).
10. **Version policy.** *Assumed default:* latest at build time. `caged build --no-cache` is the update path. No `caged update` in v1.
11. ~~Platforms~~ **Decided:** macOS primary, Linux best-effort, Windows dropped.
12. ~~`caged list`~~ **Decided:** shows detected config and auth env names, never values.
13. ~~Distribution~~ **Decided:** the tap from the first release (v0.1.0).
14. **GitHub CLI.** Included in v1 per the user's priority. Install `gh` from GitHub's signed repository and mount `~/.config/gh` read-write like the other CLIs.
15. ~~Refactor freedom~~ **Decided:** caged is released (v0.1.0), so a refactor may rename and move code but keeps every flag, message, exit code, docker argument and file format; the golden tests pin them.
16. ~~Docker runtime~~ **Decided:** Docker Desktop only.
17. ~~Ports~~ **Decided:** `-p`, 127.0.0.1 only.
