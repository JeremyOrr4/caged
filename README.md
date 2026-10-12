# caged

`caged` will run AI coding CLIs in Docker containers while mounting the current
working directory. This first step provides the project scaffold and a
hello-world executable only.

## Run it locally

Build the binary, then pass the AI CLI name as its argument:

```sh
make build
./bin/caged pi
```

For a one-off run without creating a binary:

```sh
go run ./cmd/caged claude
```

At this first stage, the command only prints a hello message and the argument
it received; it does not start Docker or an AI CLI yet.

## Build and check locally

```sh
make build
make test
make vet
```

## Run it in Docker

Start Docker Desktop first, then build the development image and open a shell
with this repository mounted at `/workspace`:

```sh
make docker-build
make docker-shell
```

Inside the container, build and run the same command:

```sh
make build
./bin/caged pi
```

The repository is mounted at `/workspace`, so build output is written to
`./bin` on the host. You can leave the container with `exit`.

## Cross-compile in Docker

From the container shell, write platform-specific binaries into the host's
`bin/` directory:

```sh
GOOS=darwin GOARCH=arm64 go build -o bin/caged-darwin-arm64 ./cmd/caged
GOOS=linux GOARCH=amd64 go build -o bin/caged-linux-amd64 ./cmd/caged
```
