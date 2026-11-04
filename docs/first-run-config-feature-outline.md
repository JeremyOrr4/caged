# Caged first-use configuration and container mount

## Objective

Create a starter Caged configuration for users who do not already have one, and make that configuration directory available for editing from runtime containers.

## Required behavior

- Use `~/.config/caged/config.yaml` by default.
- Use `$XDG_CONFIG_HOME/caged/config.yaml` when `XDG_CONFIG_HOME` is absolute.
- On the first command that needs Caged's configuration, create the directory and file only if they do not exist.
- The new file contains commented examples only, preserving current defaults until a user makes changes.
- Never replace, append to, or change permissions on an existing configuration.
- Make concurrent first-use creation safe. If the folder cannot be created, warn clearly and go on without it (nothing is mounted); if the folder exists but the file cannot be created or read, return a clear, actionable error.
- Commands that do not need configuration, such as help, version, and clean, should not create it.
- Mount the selected host configuration directory read/write at `/home/caged/.config/caged` in runtime containers. Directory mounting allows atomic file-save operations.
- Respect `--no-config`; do not mount the directory when config mounts are disabled.
- Keep the configuration out of the image and Docker build context. Changes made in the container persist on the host and are read on subsequent Caged invocations.
- Preserve existing symlink protections. Define and test safe handling for a symlinked `config.yaml`; do not expose a broader host directory or weaken the `~/.config/caged` symlink deny rule.

## Implementation areas

- `internal/userconfig`: first-use creation, config path selection, parsing, and preservation of existing files.
- `internal/system`: filesystem operations for private directory creation and exclusive file creation.
- `internal/hostconfig` and `internal/plan`: resolve and represent the selected Caged config directory as a writable mount, including absolute XDG paths outside the home directory.
- `internal/docker`: verify writable mount arguments.
- `internal/images/Dockerfile`: ensure the container mount destination exists.
- `README.md`, `docs/spec.md`, and `docs/architecture.md`: document first-use creation, the mount path, writable behavior, and host-side effects.

## Verification

- Unit tests: starter template validity, permissions, existing-file preservation, XDG paths, concurrent creation, errors, and symlink behavior.
- Resolver/planner tests: correct host source, container destination, writable mode, and no mount conflicts.
- Docker argument and runtime-image tests.
- Docker end-to-end test: verify the config is visible in the container, can be atomically edited there, and changes persist on the host.
- Update doctor output and golden tests where first-use creation changes reported config status.
- Run `gofmt`, `make check`, and Docker end-to-end tests where available.
