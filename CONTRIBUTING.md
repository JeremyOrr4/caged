# Contributing to caged

## Branches

| Branch | Purpose | Accepts |
|---|---|---|
| `main` | releases only; every tag is cut from it | merge commits, from `dev` or `hotfix/*` |
| `dev` | integration branch; open your PRs here | squash merges only |
| `feat/*`, `fix/*`, `refactor/*`, `docs/*`, `ci/*`, `chore/*` | your work | n/a |
| `hotfix/*` | an urgent patch for the released version | PR into `main` |

Never push to `dev` or `main` directly, and never force-push.

## Pull requests

1. Branch from `dev` and open the PR against `dev`.
2. Give the PR a [Conventional Commits](https://www.conventionalcommits.org/)
   title. With squash merges the title becomes the commit subject on `dev`,
   and the CHANGELOG is written from those subjects:

   ```text
   feat(ports): publish container ports with -p
   fix: report a missing Docker CLI before looking for the image
   ```

   Allowed types: `feat`, `fix`, `docs`, `test`, `refactor`, `perf`, `build`,
   `ci`, `chore`. Add `!` before the colon for a breaking change.
3. Keep each PR to one change.
4. Run `make check` first. CI runs the same steps (`ci-ok` is the one required
   check, next to `pr-source`).

A PR into `main` must come from `dev` (a release) or from `hotfix/*` (a patch).

## Hotfixes

1. Branch `hotfix/<name>` from `main` and open a PR into `main`.
2. After it merges, bring the fix back to `dev` with a normal PR (a
   cherry-pick on a `fix/*` branch), because `dev` does not accept merge
   commits.

## Local checks

```sh
make check
```

runs `fmt-check`, `tidy-check`, `vet`, `test-race`, `lint` (golangci-lint v2),
`vuln` (govulncheck) and `cross-build` (darwin and linux, amd64 and arm64).
Install the two tools with:

```sh
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
go install golang.org/x/vuln/cmd/govulncheck@latest
```

The package layout and the dependency rules that `cmd/caged/architecture_test.go`
enforces are in [docs/architecture.md](docs/architecture.md).
