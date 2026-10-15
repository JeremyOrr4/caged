#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
check_script="$script_dir/check-pr-commits.sh"
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
  git -C "$tmp/$name" config user.name "PR Policy Test"
  git -C "$tmp/$name" config user.email "pr-policy-test@example.invalid"
  git -C "$tmp/$name" commit --allow-empty -q -m "chore: establish history"
  repo="$tmp/$name"
  base_sha="$(git -C "$repo" rev-parse HEAD)"
}

add_commit() {
  git -C "$repo" commit --allow-empty -q -m "$1"
  head_sha="$(git -C "$repo" rev-parse HEAD)"
}

run_check() {
  local base="$1" title="$2" branch="$3" head_repo="$4" base_repo="$5"
  (cd "$repo" && BASE="$base" TITLE="$title" HEAD_BRANCH="$branch" \
    HEAD_REPO="$head_repo" BASE_REPO="$base_repo" \
    BASE_SHA="$base_sha" HEAD_SHA="$head_sha" \
    bash "$check_script")
}

new_repo valid_dev
add_commit 'feat(cli): add a command'
run_check dev 'feat(cli): add a command' feature-branch owner/caged owner/caged \
  || fail 'valid Conventional Commit PR into dev was rejected'
if run_check dev 'add a command' feature-branch owner/caged owner/caged >/dev/null 2>&1; then
  fail 'invalid PR title into dev was accepted'
fi
if run_check main 'feat: release' feature-branch owner/caged owner/caged >/dev/null 2>&1; then
  fail 'feature branch into main was accepted'
fi
run_check main 'feat: release' dev owner/caged owner/caged \
  || fail 'valid dev PR into main was rejected'
if run_check dev 'chore: back merge' main owner/caged owner/caged >/dev/null 2>&1; then
  fail 'main back-merge into dev was accepted'
fi
if run_check main 'feat: release' hotfix/fix owner/fork owner/caged >/dev/null 2>&1; then
  fail 'fork hotfix into main was accepted'
fi

new_repo invalid_commit
add_commit 'work in progress'
if run_check dev 'feat: valid title' feature-branch owner/caged owner/caged >/dev/null 2>&1; then
  fail 'invalid commit subject was accepted'
fi

new_repo valid_hotfix
add_commit 'fix: correct a release issue'
run_check main 'fix: correct a release issue' hotfix/fix owner/caged owner/caged \
  || fail 'valid same-repository hotfix PR into main was rejected'

echo 'All PR commit policy checks passed.'
