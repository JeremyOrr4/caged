# caged

[![ci](https://github.com/JeremyOrr4/caged/actions/workflows/ci.yml/badge.svg?branch=dev)](https://github.com/JeremyOrr4/caged/actions/workflows/ci.yml)

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
caged claude -- --resume      # pass flags to the agent after --
caged codex 'fix tests' -- --model o4  # prompt first, then the agent's flags
caged build [--no-cache]      # build or refresh the runtime image
caged list                    # list agents, their detected host config, and which auth env vars are set
caged doctor                  # check Docker, the images, and that this directory can be mounted
caged clean [--dry-run]       # remove caged's Docker images
```

The initial prompt is passed as one argument according to each agent's launch
style. The prompt is not interpreted by a shell. After an agent exits, caged
opens a shell in the same container; `exit` removes it.

Everything after `--` goes to the agent unchanged, after the prompt, and never
through a shell. The order is: caged's flags, the agent, the prompt, `--`, the
agent's flags. caged's flags (`--verbose`, `-p`, ...) are not recognized after
the agent name or after `--`; there they belong to the agent. `--` needs an
agent before it (`caged -- --resume` is an error). With a prompt, Codex runs
`codex exec`, so its flags must be `exec` flags. `--verbose` prints the
final command as `command: [...]`.

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
caged -p 3000 claude          # publish container port 3000 on 127.0.0.1:3000
caged -p 8080:3000 -p 5173    # host port 8080 -> container 3000; 5173 -> 5173
```

For safety, mounting `/` or your entire home directory requires
`--allow-broad-mount`. macOS bind mounts can be slower for large dependency
trees.

### Doctor, list, and clean

`caged doctor` checks the setup, one line per check: `docker` (installed), `daemon` (running, and its
engine version: Docker Engine 25.0 or newer is expected), `image` (current, missing, or stale),
`extension` and `ext-image` (the image.json file and its image), `arch` (the image matches Docker's
CPU architecture), `project` (this directory, or `-w DIR`, can be mounted) and `versions` (the CLI
versions inside the image). A check that depends on a failed one is skipped. It exits 1 only when a
check fails, and it never builds or removes anything. Paste its output into a bug report: the first
line is the caged version and platform, and it never prints environment values.

`caged list` shows each agent, the host config a run would mount for it (with `~` for your home
directory), and which of its credential variables are set. Only names appear, never values.

### Uninstall

`caged clean` removes caged's images (the runtime image, the extension image, and any replaced or
older ones). Use `caged clean --dry-run` to see what it would remove first. An image that a container
still uses is kept and reported. It never touches containers, volumes, the Docker build cache
(`docker builder prune` clears that), `~/.config/caged`, or your agents' own config. To uninstall:

```sh
caged clean
# remove the binary the way you installed it, e.g. rm "$(command -v caged)"
rm -rf ~/.config/caged     # only if you created an image.json
```

Your agents' config (`~/.claude`, `~/.codex`, ...) is yours and is left alone.

### Extra packages

To add apt packages to the image, create `~/.config/caged/image.json` (or
`$XDG_CONFIG_HOME/caged/image.json` when `XDG_CONFIG_HOME` is set to an
absolute path):

```json
{
  "apt": ["neovim", "tmux", "shellcheck"]
}
```

Only `apt` is supported, with plain package names (no version or architecture
suffix). The file is yours alone: a project cannot add one, and it is never
mounted into the container. caged builds a second image, `caged-runtime:ext`,
on top of the runtime image, and every run and `caged build` uses it. The
first run after you edit the file takes a minute to rebuild; reordering the
packages does not rebuild. A mistake in the file (a bad name, or a package apt
cannot find) stops every run until you fix or remove it. Remove the file to go
back to the plain image.

### Ports

`-p PORT` and `-p HOST_PORT:CONTAINER_PORT` (also `--publish`, repeatable)
publish a container port over TCP, always on `127.0.0.1`, so nothing on your
network can reach it. There is no host-address form, no port range, and no
`/udp`. A server inside the container must listen on `0.0.0.0` (not on
`localhost`) to be reachable, for example `vite --host 0.0.0.0`,
`next dev -H 0.0.0.0` or `python3 -m http.server --bind 0.0.0.0`. If
`http://localhost:PORT` does not connect, use `http://127.0.0.1:PORT`. Without
`-p`, no port is published, so services in the container are not reachable
through the host's `localhost`.

## Install

```sh
brew install jeremyorr4/tap/caged          # builds from source with Homebrew's Go
go install github.com/JeremyOrr4/caged/cmd/caged@latest
```

Or download an archive for macOS or Linux (amd64 or arm64) from the
[releases page](https://github.com/JeremyOrr4/caged/releases), and put `caged` on your `PATH`.
A binary downloaded from a browser may be quarantined by macOS; clear that with
`xattr -d com.apple.quarantine caged`. The Homebrew install builds locally and avoids it.
From a checkout: `make build` (the binary is `bin/caged`).

The runtime image is built locally on first launch (a few minutes), and rebuilt
automatically after an upgrade that changes it (once, with a message saying why).
`caged build --no-cache` refreshes the installed CLI versions. To upgrade:
`brew upgrade caged`, or install again with `go install`.

## Requirements

- Docker: Docker Desktop on macOS (the supported setup), or Docker Engine on Linux (best effort).
  Docker Engine 25.0 or newer is expected; `caged doctor` warns about older versions and never blocks a run.
- macOS or Linux, on amd64 or arm64. Homebrew builds caged with its own Go; `go install` needs
  Go 1.27 or newer.

## Development

```sh
make build
make test
make check   # everything CI runs: format, tidy, vet, race tests, lint, vuln, cross-build
```

`make check` needs `golangci-lint` (v2) and `govulncheck` on your `PATH`:

```sh
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
go install golang.org/x/vuln/cmd/govulncheck@latest
```

Pull requests target `dev`; see [CONTRIBUTING.md](CONTRIBUTING.md).

The development image uses [dev/Dockerfile](dev/Dockerfile); the embedded
runtime image is defined in `internal/images/Dockerfile`. The package layout,
the dependency rules, and how to add an agent are described in
[docs/architecture.md](docs/architecture.md).
