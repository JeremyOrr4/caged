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
  `.gitconfig` is read-only. Absent paths are skipped. Agent config roots outside the home
  directory and unsafe symlinks are refused; a root reached through a symlink
  under `links.allow` may resolve outside home, and caged's own config folder
  may live outside home. Special files are never mounted: they are left out of
  the mount or masked, and a Codex home with one outside its masks is refused.
- Pi's config, when it needs masks or nested link mounts, runs from a
  symlink-free copy; after the container exits, its changes are written back to
  the host and the copy is removed.
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
under `internal/images/` are embedded in the binary. Command-line parsing,
agent data, the user's config file, host-config resolution, project validation,
run planning, config staging, Docker execution, and application coordination are
separated into `internal/cli`, `internal/agent`, `internal/userconfig`,
`internal/hostconfig`, `internal/project`, `internal/plan`,
`internal/configstage`, `internal/docker`, and `internal/app`. Only `internal/system` touches the
operating system, and `cmd/caged/main.go` wires everything together. See
`docs/architecture.md` for the interfaces and the dependency rules.
