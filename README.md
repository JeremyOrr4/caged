# caged

`caged` runs AI coding CLIs in disposable Docker containers. It supports a
plain shell, agent launches, config staging from the host, and persistent agent
state volumes.

## Usage

`caged` opens an interactive Ubuntu shell with the current working directory
mounted at `/workspace` and starts there.

```sh
make build
./bin/caged
```

Launch a supported agent:

```sh
./bin/caged claude
./bin/caged pi
./bin/caged codex
./bin/caged gemini
```

Use `--no-mount` for a blank container with no project directory bind mount:

```sh
./bin/caged --no-mount
./bin/caged claude --no-mount
```

Pass environment variables by name only:

```sh
./bin/caged claude --env ANTHROPIC_API_KEY
./bin/caged --env OPENAI_API_KEY claude
```

Skip persistent state for a single run:

```sh
./bin/caged claude --no-persist
```

Reset a persisted agent volume:

```sh
./bin/caged reset claude --yes
./bin/caged reset --all --yes
```

For safety, caged refuses to mount `/` or your entire home directory. If that
is intentional, explicitly acknowledge it with `--allow-broad-mount`.

Host config files are staged read-only into the container and copied into the
agent's own home directory before startup. Credential files are omitted unless
`--with-auth` is passed. The copied files never overwrite the host. The runtime
keeps agent state in a named Docker volume (`caged-<agent>-home`) when an
allowlisted agent config exists on the host. Host-side config changes are copied
into the volume at startup; container-side edits do not sync back to the host.
If no agent config exists locally, the run is disposable and its state is
discarded when the container exits.

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
make build
./bin/caged
```

The repository is mounted at `/workspace`, so build output is written to
`./bin` on the host. You can leave the container with `exit`.

## Cross-compile in the development environment

From the container shell, write platform-specific binaries into the host's
`bin/` directory:

```sh
GOOS=darwin GOARCH=arm64 go build -o bin/caged-darwin-arm64 ./cmd/caged
GOOS=linux GOARCH=amd64 go build -o bin/caged-linux-amd64 ./cmd/caged
```
