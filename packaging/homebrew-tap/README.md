# Homebrew tap workflow files

Copy the contents of `.github/workflows/` here into the root of
`JeremyOrr4/homebrew-tap`. The release update workflow creates
`Formula/caged.rb` on the first published stable release, calculates the source
archive SHA-256, validates the formula, and commits it after checks pass.

The tap repository needs Actions write access for the update workflow. Its
default branch rules must permit the workflow's checked commit, or the update
job will fail without changing the formula. The release GitHub App installed
on `caged` and `homebrew-tap` sends the `caged_release` repository-dispatch
event. The daily schedule catches a missed event.
