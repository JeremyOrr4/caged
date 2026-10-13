# Implementation notes

The implementation provides one disposable Docker runtime for Claude Code,
Codex, Gemini CLI, Pi, GitHub CLI (`gh`), GitHub Copilot CLI, and an interactive shell. Its image is embedded into the
Go executable and built locally on first launch.

## User behavior

- The project directory is mounted read-write at `/workspace/<folder-name>`.
- `caged <agent> '<prompt>'` passes the prompt as one argument using the
  registered launch style.
- After an agent exits, the container entrypoint opens an interactive shell.
- Existing allowlisted CLI config paths are mounted read-write; shared
  `.gitconfig` is read-only. Absent paths are skipped. Config roots outside the home
  directory and unsafe symlinks are refused. Special files are excluded or
  masked so they are never passed into the container.
- Codex's home is one directory mount so config updates can use atomic file
  replacement. Container tmpfs mounts hide its host daemon, socket, and runtime
  executable symlink directories.
- GitHub CLI uses the host's `~/.config/gh` directory and accepts `GH_TOKEN` or
  `GITHUB_TOKEN` when those variables are set. Create the directory before the
  first run if login needs to persist there.
- `--no-config` disables those mounts. There is no `--no-auth`; credentials in
  an existing mounted config are part of the read-write config behavior.
- Host environment values are passed by name only when set. Caged logs names,
  never values.
- Linux runs with the host UID/GID. macOS uses the image user and expects
  Docker Desktop bind-mount ownership translation.

## Development

```sh
make build
make test
make vet
```

The development image is `dev/Dockerfile`; the runtime Dockerfile and entrypoint
under `internal/runtime/` are embedded in the binary. Command-line parsing,
agent data, host-config resolution, project validation, run planning, Docker
execution, and application coordination are separated into `internal/cli`,
`internal/agent`, `internal/hostconfig`, `internal/project`, `internal/plan`,
`internal/docker`, and `internal/app`. Only `internal/system` touches the
operating system, and `cmd/caged/main.go` wires everything together. See
`docs/architecture.md` for the interfaces and the dependency rules.
