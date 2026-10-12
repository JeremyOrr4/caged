# caged

`caged` runs AI coding CLIs in Docker containers. This step opens a disposable
Ubuntu shell with your current working directory mounted at `/workspace`.

## Usage

`caged` opens an interactive Ubuntu container shell. The directory where you
run the command is available in the container at `/workspace`, and the shell
starts there. Leave the shell with `exit` or Ctrl-D; the container is removed
automatically.

```sh
make build
./bin/caged
```

Use `--no-mount` for the previous blank-container behavior:

```sh
./bin/caged --no-mount
```

For safety, caged refuses to mount `/` or your entire home directory. If that
is intentional, explicitly acknowledge it with `--allow-broad-mount`.

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
