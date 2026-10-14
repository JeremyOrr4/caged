# caged

[![CI](https://github.com/JeremyOrr4/caged/actions/workflows/ci.yml/badge.svg?branch=dev)](https://github.com/JeremyOrr4/caged/actions/workflows/ci.yml)

Run Claude Code, Codex, Gemini CLI, Pi, GitHub Copilot CLI, or `gh` in a disposable Docker container. Caged mounts your project into the container and keeps agent logins in your host config directories.

Docker Desktop on macOS is the supported setup. Linux is best effort.

## Install

```sh
brew install jeremyorr4/tap/caged
# or
go install github.com/JeremyOrr4/caged/cmd/caged@latest
```

You can also download a macOS or Linux binary (amd64 or arm64) from the [releases page](https://github.com/JeremyOrr4/caged/releases). From a checkout, run `make build` to create `bin/caged`.

Requires Docker Engine 25 or newer. The runtime image builds automatically on first launch. Use `caged build --no-cache` to refresh the agent CLIs in that image.

## Quick start

```sh
caged                         # open a shell with the supported CLIs
caged claude                  # launch Claude, then return to the container shell
caged codex "fix the tests"   # start an agent with a prompt
caged list                    # show agents and available credentials
caged doctor                  # check Docker and the project setup
```

When the agent exits, you return to the container shell. Run `exit` to remove the container.

## Usage

```text
caged [caged options] [agent [prompt]] [-- agent options...]
```

Supported agents: `claude`, `codex`, `gemini`, `pi`, `copilot`, and `gh`. With no agent, caged opens a shell. A prompt is passed directly to the agent, not through a shell. For example, Codex prompts use `codex exec`.

Options go before the agent. Everything after `--` goes to the agent unchanged:

```sh
caged claude -- --resume
caged codex "fix tests" -- --model o4
caged --workdir ~/code/my-project pi
caged --env ANTHROPIC_API_KEY claude
caged -p 8080:3000 claude
```

| Option | Purpose |
| --- | --- |
| `-w, --workdir DIR` | Mount `DIR` as the project directory. Defaults to the current directory. |
| `--no-mount` | Do not mount a project directory. |
| `--no-config` | Do not mount agent or Git configuration. |
| `--env NAME` | Forward a set host environment variable by name. Repeat as needed. |
| `-p, --publish PORT` | Publish a TCP port on `127.0.0.1`. Accepts `PORT` or `HOST_PORT:CONTAINER_PORT`; repeat as needed. |
| `--allow-broad-mount` | Allow mounting `/` or your entire home directory. |
| `--verbose` | Show config files and environment variable names passed to the container. |

For published ports, services inside the container must listen on `0.0.0.0`. Without `-p`, container ports are not available on the host.

## Files and credentials

- The project directory is mounted read-write at `/workspace/<folder-name>`.
- Existing agent config directories (including `~/.claude`, `~/.codex`, `~/.gemini`, `~/.pi`, `~/.copilot`, `~/.config/pi`, and `~/.config/gh`) are mounted read-write. Logins and config changes persist on the host.
- `~/.gitconfig` is mounted read-only. SSH keys, cloud credential directories, and the Docker socket are not mounted.
- Only selected environment variable names are forwarded, and only when set on the host. Values are never printed. Use `--env NAME` to pass another variable.
- Mounting `/` or your whole home directory requires `--allow-broad-mount`.

On macOS, Claude Code’s Keychain login is not forwarded; log in inside caged or pass an API key. The host’s `~/.claude.json` is not mounted. For GitHub CLI login, create `~/.config/gh` on the host, then run `gh auth login` inside caged.

## Customize the image

To install extra apt packages, create `~/.config/caged/image.json` (or `$XDG_CONFIG_HOME/caged/image.json` when `XDG_CONFIG_HOME` is an absolute path):

```json
{
  "apt": ["neovim", "tmux", "shellcheck"]
}
```

Only plain apt package names are supported. This file is read from your home config and is never mounted into the container. Caged builds an extension image when the file changes; remove it to use the default image again.

## Maintenance

```sh
caged build [--no-cache]     # build or refresh the image
caged doctor                 # diagnose Docker, image, and mount setup
caged list                   # show agents and which credentials are set
caged clean --dry-run        # preview images that would be removed
caged clean                  # remove caged images
```

`doctor` does not build or remove anything. `clean` leaves containers, volumes, Docker’s build cache, and your agent configs alone. Images used by a running container are kept.

## Development

```sh
make build
make test
make check
```

`make check` runs formatting, dependency, vet, race, lint, vulnerability, and cross-build checks. It requires `golangci-lint` v2 and `govulncheck` on `PATH`. `go install` requires Go 1.27 or newer. Pull requests target `dev`; see [CONTRIBUTING.md](CONTRIBUTING.md). For architecture and agent development, see [docs/architecture.md](docs/architecture.md).
