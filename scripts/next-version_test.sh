#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
next_version="$script_dir/next-version.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

new_repo() {
  local name="$1"
  mkdir -p "$tmp/$name"
  git -C "$tmp/$name" init -q
  git -C "$tmp/$name" config user.name "Release Script Test"
  git -C "$tmp/$name" config user.email "release-test@example.invalid"
  git -C "$tmp/$name" commit --allow-empty -q -m "test: establish history"
  repo="$tmp/$name"
}

commit_subject() {
  local subject="$1"
  local body="${2:-}"
  if [ -n "$body" ]; then
    git -C "$repo" commit --allow-empty -q -m "$subject" -m "$body"
  else
    git -C "$repo" commit --allow-empty -q -m "$subject"
  fi
}

expect_version() {
  local expected="$1"
  local actual
  actual="$(cd "$repo" && "$next_version")"
  if [ "$actual" != "$expected" ]; then
    fail "expected '$expected', got '$actual' in $repo"
  fi
}

new_repo initial
commit_subject 'fix: initial release change'
expect_version 0.1.0

new_repo docs_only
commit_subject 'docs: update examples'
expect_version ''

new_repo slow_feature
git -C "$repo" tag v0.1.0
commit_subject 'feat: add a capability'
expect_version 0.1.1

new_repo slow_breaking_subject
git -C "$repo" tag v0.1.0
commit_subject 'fix!: change behavior'
expect_version 0.2.0

new_repo slow_breaking_footer
git -C "$repo" tag v0.1.0
commit_subject 'fix: change behavior' 'BREAKING CHANGE: callers must adapt'
expect_version 0.2.0

new_repo breaking_footer_on_docs
git -C "$repo" tag v0.1.0
commit_subject 'docs: document a breaking change' 'BREAKING CHANGE: callers must adapt'
expect_version 0.2.0

new_repo stable_minor
git -C "$repo" tag v1.2.3
commit_subject 'feat: add a capability'
expect_version 1.3.0

new_repo stable_patch
git -C "$repo" tag v1.2.3
commit_subject 'fix: correct behavior'
expect_version 1.2.4

new_repo stable_major
git -C "$repo" tag v1.2.3
commit_subject 'feat!: change behavior'
expect_version 2.0.0

new_repo ignores_non_semver_tags
git -C "$repo" tag v9.9.9-rc.1
git -C "$repo" tag vnot-semver
git -C "$repo" tag v01.0.0
commit_subject 'fix: initial stable release change'
expect_version 0.1.0

echo "All release version script checks passed."
