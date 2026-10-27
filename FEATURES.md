# caged: features and where we are

Last updated **2026-10-16**. Details: [docs/CAGED-GUIDE.md](docs/CAGED-GUIDE.md) for the reasoning, [README](README.md) for usage.

**Status:** v0.1.0 is released (tag, GitHub release with four archives, Homebrew tap). All code is committed on `origin/dev` and `origin/main`. `make check` passes and CI is green.

**Test key:** U = unit, G = golden (fake `docker`, Linux and macOS rules), E = end-to-end on real Docker (Linux only). No feature has been checked on a real Mac.

## Shipped in v0.1.0

**Run agents** (U G E)
- `caged` opens a shell. `caged claude|codex|gemini|pi|copilot|gh` starts that agent, and you stay in the container shell after it exits.
- A prompt goes in as one argument, in each agent's own style: `caged codex 'fix tests'`.
- `--` sends everything after it to the agent, never through a shell: `caged claude 'x' -- --model opus`.

**Mounts and config** (U G E)
- The current folder mounts read-write at `/workspace/<name>`. `-w DIR` picks another one and `--no-mount` skips it. `/` and `~` are refused without `--allow-broad-mount`.
- Agent config folders mount read-write so logins persist. `.gitconfig` is read-only. `--no-config` skips them. Env overrides such as `CODEX_HOME` are honored.
- `config.yaml` (`~/.config/caged/` or `$XDG_CONFIG_HOME/caged/`) is one optional file. Errors name the file and line. It replaces `image.json`.
- Symlinks in agent config folders (Stow-style) are followed and mounted where the link points.
- Link policy is strict by default. A target outside its own config folder needs `links.allow`. `links.deny` extends a built-in deny list (`~/.ssh`, `~/.aws`) that always wins. `read_only` is per root. Refusals print the exact line to add.
- `workdirs.allow` limits which project folders can be mounted, and `--allow-broad-mount` cannot bypass it.

**Env and ports** (U G E)
- `--env NAME` forwards by name only, and values are never printed. A warning shows when credentials are available.
- `-p PORT` or `-p HOST:CONTAINER` publishes TCP on `127.0.0.1`. It is validated before Docker runs, and a port in use gets a clear message.

**Image** (U G E)
- Built locally from an embedded Dockerfile, with Ubuntu pinned by digest.
- Labeled with a hash and rebuilt automatically when stale or unlabeled. The replaced image is removed.
- `image.apt` adds packages in a second image, `caged-runtime:ext`.
- `caged build [--no-cache]`.

**Commands** (U G E)
- `caged doctor` checks Docker, daemon, image, config, links, architecture, project mount and CLI versions. It exits 1 only on FAIL.
- `caged list` shows each agent, its detected host config, and which auth variables are set (names only).
- `caged clean [--dry-run]` removes only caged's images.
- `--verbose` shows mounts, ports, env names, the final command and image state. `--version`, help and shell completion are included.

**Packaging and CI**
- GoReleaser archives for macOS and Linux (amd64, arm64), MIT licence, CHANGELOG.
- The Homebrew formula in `homebrew-tap` builds from source, so it needs Go. It is not a binary install.
- Release versioning script tests pass under `bash`. The tap has update and CI workflows.
- `scripts/run-local.sh` builds and runs `bin/caged`.
- CI covers tidy, format, vet, race tests on Ubuntu and macOS, lint, vuln scan, cross-builds, PR policy, a weekly image build and Dependabot.

## Next (in priority order)

- **Check the release on a real machine**: `brew install` from the tap and `caged --version`. Decide whether the formula should ship binaries instead of building from source. Needs a Mac or clean Linux box.
- **Easy local testing**: `make e2e-smoke`, `make e2e`, `make try`. The `Makefile` has none of these yet. Needs decisions 1 to 5 in the guide, section 7.7.
- **E2E on Linux and macOS for every feature**:
  - `dev/e2e/e2e.sh` is in git but Linux-only (GNU `readlink -f`, `sed -i`). It last passed on 2026-10-15 with 322 checks and was not re-run today.
  - The macOS section is not started.
- **Check everything on a real Mac**: ports on `localhost`, shared paths, arm64, Homebrew install, Gatekeeper.
- **Real-image checks** with real installers and CLIs. Blocked by the corporate proxy and skipped on purpose.
- **`SECURITY.md`** with private vulnerability reporting. The file does not exist yet.
- **Small fixes**: see Known issues below.

## Later (agreed, not planned in detail)

- **Named sessions**: `--name`, `caged attach`, `caged ps`, `caged rm`.
- **Pinned CLI versions**, so one caged version always builds the same image.
- **Ports**: `HOST_IP:` binding and port ranges.
- **Extensions**: per-project files, npm, pipx, go install, cargo, an opt-in Dockerfile fragment.
- **Distribution**: homebrew-core, `.deb` and `.rpm`, signed and notarized macOS binaries, automatic tap updates.
- **macOS keychain login**: `--keychain`.
- **More agents**: OpenCode, Amp, Qwen Code and so on, one registry entry and one install step each.
- **`caged update`** to refresh the CLIs without a full rebuild.
- **Network and resource limits**: egress control, memory, CPU.
- **Local-LLM wiring**, not designed.

## Ideas on the shelf (nothing decided)

- `--dry-run` to print the final `docker run`.
- `--readonly-project` and `--exclude '.env*'`.
- Package-cache volumes for npm, pip and go.
- `--yolo` to skip approval prompts. Only sensible with a network allowlist.
- **Network egress allowlist**, the feature caged lacks most, medium to large.
- `.caged.toml` project config. Held back on purpose, because `config.yaml` is user-level so a repo cannot change what gets mounted.
- Git worktree mode.
- `--same-path` mount, so `claude --resume` sees the same sessions.
- Small ideas on demand: SSH agent forwarding, resource limits, hardening defaults, `--env-file`, `--add-dir`, done notification, clipboard paste.

## Ruled out

- `--no-network`, rejected 2026-10-14 for no use case.
- Windows, microVMs, a web UI and cloud backends are out of scope.
- OrbStack, Colima and other runtimes are untested and unsupported. Docker Desktop only for now.

## Known issues

- `caged doctor` prints "1 checks failed" instead of "1 check failed" (`internal/app/doctor.go:71`).
- `caged list` shows a split config root as `(N files)`, which is noisy for a real `~/.claude` (over 100).
- The minimum Docker Engine version (25.0) is a placeholder that needs checking.
- Publishing ports on `[::1]` as well depends on a Mac check.
- Shell completion after a leading `--` lists subcommands.
- Docker leaves empty root-owned folders in `~/.codex` for the Codex masks.
- The deny list does not apply to files directly inside an agent config folder, and paths are checked at plan time, not when Docker starts.
- `CHANGELOG.md` still says `[0.1.0] - unreleased`.

## Numbers (counted 2026-10-16)

- 180 Go tests in 13 packages, all passing with `-race`.
- 89 golden command lines on each of Linux and macOS rules.
- E2E on Linux: 309 checks plus 13 smoke checks, passing on 2026-10-15 and not re-run since.

## Your additions

- (add yours here)
