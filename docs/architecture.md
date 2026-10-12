# caged architecture

## Overview

`caged` is a Go CLI that starts a short-lived Docker container for a shell or
one supported coding agent: Claude Code, Pi, Codex, or Gemini CLI.

```text
Host caged CLI
  ├── validates the requested agent and options
  ├── builds caged-runtime:latest if it is not already present
  └── runs docker run --rm
        ├── project directory -> /workspace (unless --no-mount)
        ├── Pi config directories <-> host (read-write)
        ├── other allowlisted configs -> temporary copies mounted read-only
        │     (unless --no-config)
        ├── selected host environment variables (by name)
        └── embedded runtime entrypoint -> runs shell/agent
                                                  └── agent exits -> interactive Bash
```

Each run uses a new container. On exit, Docker removes that container. Caged
does not create or mount named Docker volumes. State created only inside the
container is temporary and is discarded when the run ends; Pi state under its
mounted config directories persists on the host.

## Main components

### CLI

[`internal/cli/root.go`](../internal/cli/root.go) defines the Cobra root command
and its subcommands:

- `caged` starts `bash`.
- `caged <agent>` starts the selected agent.
- `caged build [--no-cache]` builds the runtime image.
- `caged list` lists supported agents.

Run flags include `--no-mount`, `--workdir`, `--allow-broad-mount`, `--no-config`,
`--no-auth`, `--verbose`, and repeatable `--env NAME`.

The default `caged` command starts one shell container with every CLI ready.
Run `claude`, `pi`, `codex`, or `gemini` from that shell to switch agents
without starting or nesting another container. The Codex executable is wrapped
so it runs with `--no-daemon` from both direct launches and interactive shells.

### Agent and config registry

[`internal/config/agents.go`](../internal/config/agents.go) stores each agent's
executable command, host config allowlist, and automatically forwarded
environment variable names. The registry is used for argument lookup,
completion, config selection, and env-var selection. Every run makes existing
allowlisted config for all supported agents available, with Pi's config
directories mounted read-write so Pi's changes persist on the host.
Config resolution produces one report of valid mounts, missing paths, excluded
credentials, and refused symlinks. The CLI uses that same report for Docker
arguments and verbose output.
Existing allowlisted config is passed through by default. Pi's two config roots
are direct read-write mounts; other agents' config is copied to a temporary
host directory and mounted read-only. `--no-auth` disables Pi write-through and
excludes allowlisted credentials from those copies.
Host CLI executables are not transferred; the single runtime image provides
the agent commands. During config staging, regular files and symlinks are
copied, while sockets and other special files are skipped.

The allowlisted host config roots are:

| Agent | Host paths | Notes |
|---|---|---|
| Claude | `~/.claude`, `~/.claude.json` | Includes allowlisted credentials by default |
| Pi | `~/.config/pi`, `~/.pi` | Direct read-write mounts; edits and auth changes persist on host |
| Codex | `~/.codex` | Includes `auth.json` by default |
| Gemini | `~/.gemini` | Includes `oauth_creds.json` by default |
| All agents | `~/.gitconfig` | Shared Git identity/config |

Missing paths are skipped. Caged resolves symlinks and refuses a config path
whose resolved target is outside the host user's home directory. No entire host
home, SSH directory, cloud credential directory, or Docker socket is mounted.

### Runtime image and embedding

[`internal/container/runtime/Dockerfile`](../internal/container/runtime/Dockerfile)
defines an Ubuntu 24.04 image with the command-line tools and supported agent
CLIs installed. Pi is installed under `/opt/caged/pi-home` so its executable
is not hidden by the host's `~/.pi` config mount. The image uses `/home/caged`
as `HOME`, sets `/workspace` as the working directory, and configures Git to
trust `/workspace`.

[`internal/container/runtime.go`](../internal/container/runtime.go) embeds both
the Dockerfile and
[`internal/container/runtime/caged-entrypoint.sh`](../internal/container/runtime/caged-entrypoint.sh)
into the Go binary with `go:embed`. Image builds write those embedded files to
a temporary build directory and invoke Docker, so a built caged executable
doesn't need the repository checkout at runtime. Both the shell and all agent
launches use the same `caged-runtime:latest` image. The entrypoint only launches
the requested command and returns to Bash after an agent exits.

If the image is absent when caged starts, it attempts to build it. Running
`caged build` explicitly rebuilds it.

## Container startup flow

1. The CLI resolves the target and validates every explicit `--env` name
   against `^[A-Za-z_][A-Za-z0-9_]*$`.
2. Caged constructs Docker arguments. The current directory, or the directory
   selected with `--workdir`, is bind-mounted at `/workspace`, unless
   `--no-mount` is set. Root and the entire home directory are refused by
   default unless `--allow-broad-mount` is provided.
3. Caged checks each explicitly allowlisted host path and skips paths that do
   not exist. Pi's `~/.pi` and `~/.config/pi` paths are mounted read-write at
   their destinations. Other existing paths are copied into a temporary
   directory on the host and mounted read-only at their destinations. Special
   files such as Codex's live IPC socket are skipped. Configs for all supported
   agents are included, not only the selected agent.
4. On Linux, Docker receives `--user <host-uid>:<host-gid>` so files written in
   `/workspace` and Pi's config directories have the host user's ownership. On
   macOS, host UID/GID mapping is enabled whenever host config directories are
   mounted. On Windows, that flag is omitted.
5. The entrypoint starts the requested shell or agent process.
6. The default launch is an interactive shell, with all agent commands
   available in it. For `caged <agent>`, when the agent exits, the entrypoint
   opens an interactive Bash shell in that same container. Exiting Bash ends it.
7. The container runs interactively with `--rm`; Docker removes it after exit.

The writable permissions on selected `/home/caged` directories in the image
allow those destinations to be written when Linux supplies an arbitrary host
UID/GID. These are ordinary directories in the container image, not Docker
volumes.

## Config and credential behavior

Pi config roots are mounted read-write, so Pi changes—including auth or
settings changes—are immediately reflected in the host config directories.
Other agents' configs are copied to a temporary host directory and mounted
read-only into the container. Changes made to those copies disappear when the
container is removed.

- `--no-config` disables all host config mounts and staging.
- Allowlisted config and credential files are available by default so the
  installed agents can use existing host config.
- `--no-auth` disables Pi's direct config mounts and excludes credential files
  from staged config copies; `--no-config` disables all host config handling.
- Other agent configs are read-only inputs, not a bidirectional sync
  mechanism.

## Environment variable behavior

For an agent, caged automatically selects the environment names listed in that
agent's `EnvVars`. Repeat `--env NAME` to request additional names. Only names
that are set in the host environment are forwarded; names are deduplicated.
With no agent, only explicitly requested names are considered.

Caged passes each variable to Docker using `-e NAME`, not `-e NAME=value`, and
does not print values. This keeps values out of caged's own Docker argument
strings. Docker does make a forwarded variable available inside the container;
host-level Docker inspection or privileged host processes may still be able to
inspect container environment metadata. Do not forward a secret unless that
exposure is acceptable.

`--verbose` reports config paths and environment variable names only. It never
prints environment values.

## Bind mounts versus named volumes

The `-v` option in Docker arguments is used only for bind mounts:

- project directory to `/workspace`;
- Pi's existing config directories to their container home paths (read-write);
- other selected config paths to read-only staging locations.

There is no named-volume option, persistent state storage, `--no-persist`
option, or `reset` command in the current implementation. Any existing named
volumes left by an older build are not used by this version.

## Useful source files

- [`internal/cli/root.go`](../internal/cli/root.go): commands and flags
- [`internal/config/agents.go`](../internal/config/agents.go): agent registry,
  config allowlists, and environment-variable policy
- [`internal/container/run.go`](../internal/container/run.go): Docker argument
  generation and process invocation
- [`internal/container/runtime.go`](../internal/container/runtime.go): embedded
  runtime files, image checks, and image builds
- [`internal/container/runtime/Dockerfile`](../internal/container/runtime/Dockerfile):
  runtime image contents
- [`internal/container/runtime/caged-entrypoint.sh`](../internal/container/runtime/caged-entrypoint.sh):
  agent launch and return-to-shell behavior
