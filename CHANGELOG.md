# Changelog

This file records the initial project history. Starting with automated
releases, generated release notes are published on the
[GitHub Releases page](https://github.com/JeremyOrr4/caged/releases). While the
version is 0.x, flags and output can still change.

## [Unreleased]

## [0.1.0] - unreleased

The first release.

### Added

- `-p`/`--publish` publishes container ports on `127.0.0.1` (`PORT` or
  `HOST_PORT:CONTAINER_PORT`, TCP only), validated before Docker runs.
- The runtime image is labeled with a hash of its build files. caged rebuilds it
  automatically when the hash differs (or when the image has no label), removes
  the image it replaced, and says why it is rebuilding.
- A config file, `~/.config/caged/config.yaml` (or
  `$XDG_CONFIG_HOME/caged/config.yaml`). `image.apt` adds apt packages in a second
  image, `caged-runtime:ext`. Errors name the file and the line.
- Symlinks in agent config directories, such as Stow-managed dotfiles, are
  followed on the host and their targets are mounted where the links point
  inside the container. A target outside the link's own config folder must be
  under a `links.allow` root (`links.deny` and `read_only` are also available);
  a built-in deny list that includes `~/.ssh` always wins. A refused link
  carries a hint that says what to add.
- `workdirs.allow` limits which project directories caged will mount.
  `--allow-broad-mount` does not bypass it.
- `--` passes arguments to the agent unchanged, after the prompt:
  `caged claude 'fix tests' -- --model opus`.
- `caged doctor` checks Docker, the images, the config file, the image
  architecture, the project mount, and the CLI versions inside the image. Its
  `config` and `links` rows report the config file, a leftover `image.json`, and
  symlinked config that was refused.
- `caged list` shows each agent's detected host config and which auth
  environment variables are set (names only).
- `caged clean [--dry-run]` removes caged's Docker images.
- `--verbose` prints the final `command:`, the published ports, and the image
  state.
- Shell completion describes each agent.
- MIT license; release archives for macOS and Linux (amd64 and arm64); a
  Homebrew formula built from source; `go install` reports the module version.

### Changed

- **`caged list` now prints a table** (`AGENT`, `CONFIG`, `AUTH ENV SET`)
  instead of agent names only.
- The request is planned before Docker is called, so a bad flag or config file
  fails immediately instead of after an image build. A missing Docker CLI is
  reported as such, not as a missing image.
- The `ubuntu:24.04` base image is pinned by digest.
- Go 1.27 is the minimum Go version.
- `image.json` is no longer read; move its packages to `image.apt` in
  `config.yaml`. `caged doctor` warns when an `image.json` is still there.
- A symlink inside an agent config directory used to be refused (or made caged
  refuse the whole directory). It is now followed under the link policy above.

### Security

- `go.mod` moved off Go 1.23, which no longer receives security fixes.
