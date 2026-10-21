# caged

[![CI](https://github.com/JeremyOrr4/caged/actions/workflows/ci.yml/badge.svg?branch=dev)](https://github.com/JeremyOrr4/caged/actions/workflows/ci.yml)

Run Claude Code, Codex, Gemini CLI, Pi, GitHub Copilot CLI, or `gh` in a disposable Docker container. Caged mounts your project into the container and keeps agent logins in your host config directories.

Docker Desktop on macOS is the supported setup. Linux is best effort.

## Install

```sh
brew install jeremyorr4/tap/caged
```

Upgrade with `brew upgrade caged`.

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
- Symlinks inside those directories are followed only as described under [Configuration](#configuration).
- `~/.gitconfig` is mounted read-only. SSH keys, cloud credential directories, and the Docker socket are not mounted.
- Only selected environment variable names are forwarded, and only when set on the host. Values are never printed. Use `--env NAME` to pass another variable.
- Mounting `/` or your whole home directory requires `--allow-broad-mount`.

On macOS, Claude Code’s Keychain login is not forwarded; log in inside caged or pass an API key. The host’s `~/.claude.json` is not mounted. For GitHub CLI login, create `~/.config/gh` on the host, then run `gh auth login` inside caged.

## Configuration

Caged reads one optional file, `~/.config/caged/config.yaml` (or `$XDG_CONFIG_HOME/caged/config.yaml` when `XDG_CONFIG_HOME` is an absolute path). It is read from your home config on the host and is never mounted into the container. Every key is optional, and a missing file means the defaults.

```yaml
version: 1                 # optional; if present it must be 1
image:
  apt: [postgresql-client] # extra apt packages in the runtime image
links:
  allow:                   # roots that symlinks in agent config may point into
    - path: ~/git/misc/dotfiles
      read_only: false     # true: mounts through this root are read-only
  deny: [~/secrets]        # added to the built-in deny list
workdirs:                  # optional; without it any project directory may be mounted
  allow:
    - path: ~/git
```

Paths must be absolute or start with `~/`; caged does not expand variables. Quote a lone `"~"`, because an unquoted `~` is YAML for null. An unknown key, a wrong type, or an invalid package name stops caged before it calls Docker, and the error names the file and line. The old `image.json` is no longer read; `caged doctor` warns if one is still there.

### Extra packages

`image.apt` lists plain apt package names (no versions, no shell syntax). Caged builds an extension image when the list changes; remove the list to use the default image again.

### Symlinked agent config

Agent config directories such as `~/.claude` can hold symlinks, for example when dotfiles are managed with GNU Stow. Caged follows each symlink on the host, checks the target, and mounts the target at the place the link points inside the container. The container never sees the host's links.

- A link whose target stays inside its own config folder needs nothing.
- Any other target must be under a root in `links.allow`. This is the strict default: without the file, only links inside their own folder work.
- The built-in deny list always wins and cannot be overridden: `~/.ssh`, `~/.gnupg`, `~/.aws`, `~/.azure`, `~/.kube`, `~/.docker`, `~/.config/gcloud`, `~/.password-store`, `~/.netrc`, `~/.git-credentials`, `~/.local/share/keyrings`, `~/Library/Keychains`, `~/.config/caged`, `/etc`, `/run` and `/var/run`. A link to a directory that contains one of them is refused too.
- A root that contains your home directory (`/` or `~`) is refused as too broad.
- Targets are mounted read-write, so an agent can edit your dotfiles through the link. Set `read_only: true` on a root to mount through it read-only.

A refused link is not mounted, and the message says what to add. See them with `caged --verbose`, or with the `links` row of `caged doctor`:

```text
config: refused ~/.claude/CLAUDE.md (symlink target ~/dotfiles/claude/CLAUDE.md is not under links.allow); add "- path: ~/dotfiles/claude" under links.allow in ~/.config/caged/config.yaml
```

With GNU Stow, one root usually covers everything:

```yaml
links:
  allow:
    - path: ~/dotfiles
```

### Project directories

When `workdirs.allow` is set, the project directory must be under one of its roots, or caged refuses to start. `--allow-broad-mount` does not bypass it. An empty list refuses every directory.

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
