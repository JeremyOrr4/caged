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
config. Keychain extraction, ports, GitHub CLI, and non-Docker-Desktop runtimes
are out of scope for v1. Claude Code's `CLAUDE_CONFIG_DIR` points to
`/home/caged/.claude` so config writes stay inside the directory bind mount;
the host's `~/.claude.json` is not shared because Docker Desktop cannot replace
a mounted single file by rename.

## Runtime image

`internal/runtime` embeds the runtime Dockerfile and entrypoint. The image
installs Claude Code, Pi, Codex, Gemini CLI, and GitHub Copilot CLI. It is built
on first use when absent. `caged build --no-cache` refreshes the latest
installer versions. The development-only Dockerfile is `dev/Dockerfile`.
