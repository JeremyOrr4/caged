# Changelog

All notable changes to caged are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and caged follows
[Semantic Versioning](https://semver.org/spec/v2.0.0.html). While the version is
0.x, flags and output can still change; such changes are called out below.

Each pull request into `dev` adds a line under `Unreleased`. The release PR moves
those lines under the new version.

## [Unreleased]

## [0.1.0] - unreleased

The first release.

### Added

- `-p`/`--publish` publishes container ports on `127.0.0.1` (`PORT` or
  `HOST_PORT:CONTAINER_PORT`, TCP only), validated before Docker runs.
- The runtime image is labeled with a hash of its build files. caged rebuilds it
  automatically when the hash differs (or when the image has no label), removes
  the image it replaced, and says why it is rebuilding.
- Image extensions: `~/.config/caged/image.json` (or
  `$XDG_CONFIG_HOME/caged/image.json`) adds apt packages in a second image,
  `caged-runtime:ext`.
- `--` passes arguments to the agent unchanged, after the prompt:
  `caged claude 'fix tests' -- --model opus`.
- `caged doctor` checks Docker, the images, the extension file, the image
  architecture, the project mount, and the CLI versions inside the image.
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
- The request is planned before Docker is called, so a bad flag or extension file
  fails immediately instead of after an image build. A missing Docker CLI is
  reported as such, not as a missing image.
- The `ubuntu:24.04` base image is pinned by digest.
- Go 1.27 is the minimum Go version.

### Security

- `go.mod` moved off Go 1.23, which no longer receives security fixes.
