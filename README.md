# caged

`caged` runs AI coding CLIs in disposable Docker containers. It supports a
plain shell, agent launches, and host config integration.

## Usage

`caged` opens one interactive Ubuntu container with the current working
directory mounted at `/workspace`, all supported agent CLIs installed, and
existing allowlisted config for each CLI made available in the container.

```sh
go run ./cmd/caged
```

Start and switch between agents directly in that same container shell:

```sh
claude
pi
codex
gemini
```

The project directory is a bind mount, so edits in `/workspace` are immediately
visible on the host. On Linux, files created there use your host UID/GID.
Git is configured to trust `/workspace`.

You can also start an agent immediately:

```sh
go run ./cmd/caged claude
```

When that agent exits, caged leaves you in a shell in the same container, where
you can launch another CLI. Type `exit` to remove the container.

Codex is wrapped with `--no-daemon`, including when started from the container
shell.

Build or list the runtime agents from the host:

```sh
go run ./cmd/caged build
go run ./cmd/caged list
```

`caged-runtime:latest` is the one image used for the shell and all four agents.
If it is missing, `caged` builds it on the first run. `caged build` rebuilds
that image; `caged build --no-cache` skips Docker's build cache.

Use `--no-mount` for a blank container with no project directory bind mount:

```sh
go run ./cmd/caged --no-mount
go run ./cmd/caged claude --no-mount
```

Pass environment variables by name only:

```sh
go run ./cmd/caged claude --env ANTHROPIC_API_KEY
go run ./cmd/caged --env OPENAI_API_KEY claude
```

Mount a particular host project directory instead of the current one with
`--workdir` / `-w`:

```sh
go run ./cmd/caged --workdir ~/code/my-project
go run ./cmd/caged -w ~/code/my-project pi
```

For safety, caged refuses to mount `/` or your entire home directory. If that
is intentional, explicitly acknowledge it with `--allow-broad-mount`.

Pi's existing `~/.pi` and `~/.config/pi` directories are mounted read-write at
their matching paths in the container. Pi config and login changes therefore
persist on the host immediately. Other supported agents' allowlisted config is
staged read-only and copied into the container; changes to those configs are
discarded when it exits. Local CLI executables are not copied into the
container; the runtime image supplies the agent commands. Config staging skips
sockets and other special files. `--no-auth` keeps allowlisted credential files
out of the container; `--no-config` disables all host config mounts. Caged does
not create or mount named Docker volumes.

By default, Pi has direct access to its local config and credentials through
the read-write mounts. Use `--no-auth` to avoid mounting Pi config write-through
and to exclude credential files from the staged configs for other agents.

Use `--verbose` to see which config paths and environment variable names are
passed, skipped, or excluded. Environment variable values are never printed.

Install it to your Go binary directory to run it from anywhere:

```sh
make install
caged
```

## Build and check locally

```sh
make build
make test
make vet
```

## Build the development environment

Start Docker Desktop first, then build the development image and open a shell
with this repository mounted at `/workspace`:

```sh
make docker-build
make docker-shell
```

Inside the container, build and run caged:

```sh
go run ./cmd/caged
```

The repository is mounted at `/workspace`. You can leave the container with
`exit`.

## Cross-compile in the development environment

From the container shell, write platform-specific binaries into the host's
`bin/` directory:

```sh
GOOS=darwin GOARCH=arm64 go build -o caged-darwin-arm64 ./cmd/caged
GOOS=linux GOARCH=amd64 go build -o caged-linux-amd64 ./cmd/caged
```
