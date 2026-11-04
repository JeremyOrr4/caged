# End-to-end tests

`e2e.sh` runs caged against real Docker: it builds several caged binaries from this checkout, builds
the runtime image from a stand-in Dockerfile, and checks what each command prints, mounts and leaves
behind. It runs on Linux and macOS (bash 3.2 or later).

| File | What it is |
|---|---|
| `e2e.sh` | the suite: sections of numbered checks, a summary line, exit 1 when any check fails |
| `guard.sh` | the image guard, sourced by `e2e.sh` (below) |
| `compat.sh` | the few commands that differ between GNU (Linux) and BSD (macOS) userland, chosen by `uname` |
| `selftest.sh` | tests of `guard.sh` and `compat.sh` with a fake `docker`; no Docker or network needed |
| `make-dockerfile.py` | writes the stand-in Dockerfile: the real final stage, with Node and gh from Ubuntu's apt archive and fake agent CLIs instead of HTTPS downloads |

## Running it

From the repository root:

```sh
make e2e-smoke    # selftest, then section S only: build, run, doctor, clean (about 2 minutes)
make e2e          # selftest, then every section (20 to 40 minutes; extension builds run apt)
make e2e-macos    # the same on a Mac, with the scratch folder under /Users (see docs/macos-testing.md)
```

Or directly: `dev/e2e/e2e.sh <scratch dir> <repo dir> [section letters]`, for example
`dev/e2e/e2e.sh ~/.cache/caged-e2e . AB`. The scratch folder must be outside the repository, because
the suite copies the repository into it. Sections always run in the file's order, whatever the order
of the letters:

| Section | Checks |
|---|---|
| S | smoke test: version, build, a shell, an agent, doctor, list, clean |
| A | basics and error paths (no Docker, daemon down, bad requests make no docker call) |
| B | image build and freshness labels, rebuilds, removal of the replaced image |
| C | run behaviour: mounts, user, env, agent config, masks, exit codes |
| D | ports on 127.0.0.1, the port-in-use message, `-p` validation |
| E | `image.apt` extension images with real apt installs |
| F | flags passed to the agent after `--` |
| G | doctor, list, clean |
| H | version resolution and packaging files |
| J | the first-run `config.yaml`, its writable mount, the change review |
| L | symlinked agent config (a Stow layout) and `links.allow` |
| W | `workdirs.allow` |
| M | macOS only (skipped elsewhere): `/Users` and `/var` paths, ownership, `localhost`, doctor, case-insensitive and Unicode names |
| I | leftovers: no caged images, containers or build folders remain |

Environment:
- `E2E_UNPIN=1` drops the `@sha256` pin from the base image (an offline daemon cannot verify it).
- `/vendor`, when present, is used for the Go modules (no network).
- `E2E_THROWAWAY_DAEMON=1` and `E2E_SAVE_IMAGES=1`: see the image guard.

## The image guard

The suite runs `caged clean` and removes `caged-runtime` images, so it must never meet a daemon whose
caged images someone still needs. Before it touches anything, `guard.sh`:
- runs the suite when the daemon holds no caged image, or when `E2E_THROWAWAY_DAEMON=1` says the
  daemon is disposable;
- with `E2E_SAVE_IMAGES=1` (the `make` targets set it), saves every caged image with `docker save` to
  `<scratch>/saved-caged-images.tar`, says so, and loads it back in an `EXIT` trap, also after Ctrl-C.
  If the save fails, it refuses to run (exit 2). If the load fails, it keeps the tar and prints the
  `docker load -i` command to run;
- otherwise refuses to run (exit 2) and says how to proceed.

Other images and containers are never touched; the suite uses only containers named `caged-e2e-*`,
ports 38000 to 38999, and `HOME=<scratch>/home`.

## Known environment limits

On a rootless Docker daemon, container user IDs are remapped, so the checks that write through a
bind-mounted host folder fail on correct code (19 checks in C2, C7, C8, J2 to J5, L2, L3, L5, L6b,
L6c and L10). Run the full suite on a normal daemon, or on Docker Desktop, to verify them.
