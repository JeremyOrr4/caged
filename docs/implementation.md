# caged implementation status

## Purpose

`caged` is a Go CLI intended to run AI coding CLIs in disposable Docker
containers. It mounts the current project directory, launches a custom runtime
image with the AI CLIs installed, and passes a limited set of host settings and
credentials into the container, with Pi config changes written back to the host.

## Implemented

### Project foundation

- Go module: `github.com/JeremyOrr4/caged`
- Standard Go layout with the executable in `cmd/caged` and internal packages
  for CLI, container, and configuration behavior.
- Custom runtime image builder with embedded Dockerfile and entrypoint script.
- Agent registry for `claude`, `pi`, `codex`, and `gemini`.
- Cobra-based command structure with subcommands for `build` and `list`.

### Runtime image and agent launch

```sh
caged
```

- Opens an interactive, disposable Ubuntu 24.04 Docker container.
- Installs all supported agent CLIs in the runtime image. From the shell, run
  `claude`, `pi`, `codex`, or `gemini` to switch agents without starting a new
  container.
- Uses the same embedded runtime image `caged-runtime:latest` for the shell and
  all agent launches, building it automatically when missing.
- `caged build` rebuilds that image; `caged build --no-cache` disables Docker
  build cache.
- Mounts the current working directory at `/workspace` unless `--no-mount` is
  supplied; `--workdir PATH` selects a different host directory.
- Starts the requested AI CLI in `/workspace` with HOME under `/home/caged`.
  Docker uses the host UID/GID on Linux for project/config file ownership and
  on macOS when Pi's writable config mounts are present.
- When `caged <agent>` is used, opens a shell in that same container after the
  agent exits.

### Agent configuration

The runtime mounts Pi's existing `~/.pi` and `~/.config/pi` directories
read-write at `/home/caged/.pi` and `/home/caged/.config/pi`. Pi config and
credential edits therefore update the host immediately. Existing allowlisted
config for the other supported agents is copied to a temporary host directory
and mounted read-only into the container. Those copies are temporary. Paths
that are missing locally are skipped, and paths that resolve outside the
user's home directory are rejected. The CLI uses one resolution result for
Docker mounts and `--verbose` reporting. Staging copies regular files and
symlinks while skipping sockets and other special files.
Host-installed CLI executables are not copied; the runtime image provides the
commands.

- `--no-config` disables host config pass-through.
- `--no-auth` disables Pi's write-through config mounts and excludes credentials
  from staged configs.
- `--verbose` reports which files were passed, skipped, or excluded.

The project bind mount is read-write, so edits under `/workspace` appear on the
host. Pi config mounts are also read-write. Staged configs and other agent state
inside the container do not sync back to the host.

### Environment passthrough

```sh
caged claude --env ANTHROPIC_API_KEY
```

- `--env NAME` passes environment variables to the container by NAME only.
- Names are validated against the shell-safe variable pattern.
- Only set variables are passed through.
- Caged does not create or mount named Docker volumes. Agent state and copied
  host config are discarded when the disposable container exits; Pi config
  data on its direct bind mounts persists on the host.

### Mount safeguards

For safety, caged refuses to mount either of these locations by default:

- the filesystem root (`/`)
- the user's entire home directory

If a broad mount is intentional, it must be explicitly enabled:

```sh
caged --allow-broad-mount
```

### Bind-mount file ownership

Caged supplies Docker with the host `--user <uid>:<gid>` on Linux. It also does
so on macOS when Pi config directories are mounted read-write. This keeps
changes on those bind mounts owned by the host user. The flag is omitted on
Windows.

## Development commands

```sh
# Build a local binary.
make build

# Run tests and static checks.
make test
make vet

# Install to the Go binary directory.
make install

# Build and enter the development container.
make docker-build
make docker-shell
```

## Tests

Unit coverage includes:

- Docker argument construction with and without mounts.
- Runtime image embedding checks.
- Agent lookup and supported-agent list behavior.
- Environment-pass-through validation and rejection of invalid names.
- Mount path safeguards and agent selection logic.

## Scope for this phase

This implementation covers the runtime image, staged host config handling,
environment pass-through, and disposable agent state. It deliberately excludes
named Docker volumes, local LLM wiring, a permanent `caged` config file, and any
further multi-step workflow automation.
