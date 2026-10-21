# Contributing to caged

## Branches

| Branch | Purpose | Accepts |
|---|---|---|
| `main` | releases only; automation tags successful releases | merge commits, from `dev` or `hotfix/*` |
| `dev` | integration branch; open your PRs here | squash merges only |
| `feat/*`, `fix/*`, `refactor/*`, `docs/*`, `ci/*`, `chore/*` | your work | n/a |
| `hotfix/*` | an urgent patch for the released version | PR into `main` |

Never push to `dev` or `main` directly, and never force-push.

## Pull requests

1. Branch from `dev` and open the PR against `dev`.
2. Use a clear PR title. With squash merges, the title becomes the commit
   subject on `dev`; release automation uses Conventional Commit prefixes to
   select a version and generate release notes. Use a prefix when the change
   should trigger a release:

   ```text
   feat(ports): publish container ports with -p
   fix: report a missing Docker CLI before looking for the image
   ```

   `feat`, `fix`, and `perf` trigger releases. Mark breaking changes with `!`
   before the colon or a `BREAKING CHANGE:` footer; they trigger releases for
   any prefix.
3. Keep each PR to one change.
4. Run `make check` first. CI runs the same checks; `ci-ok` is the required
   aggregate check. PR source-branch rules are checked separately.

A PR into `main` must come from `dev` (a release) or from `hotfix/*` (a patch).
Once merged, `ci-ok` gates the automated release. Release notes are generated on GitHub;
the `CHANGELOG.md` file is not updated for each release.

## Hotfixes

1. Branch `hotfix/<name>` from `main` and open a PR into `main`.
2. After it merges and the automatic release completes, bring the fix back to
   `dev` with a normal PR (a cherry-pick on a `fix/*` branch), because `dev`
   does not accept merge commits. Use a `chore:` PR title for a back-merge so
   the already released fix does not trigger a duplicate version.

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
