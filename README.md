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
