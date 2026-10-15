#!/usr/bin/env bash
set -euo pipefail

pattern='^(feat|fix|docs|test|refactor|perf|build|ci|chore|revert)(\([^)]+\))?!?: .+'

if [ "$BASE" = main ]; then
  if [ "$HEAD_REPO" != "$BASE_REPO" ] || { [ "$HEAD_BRANCH" != dev ] && [[ "$HEAD_BRANCH" != hotfix/* ]]; }; then
    echo "PRs into main must come from this repository's dev or hotfix/* branch" >&2
    exit 1
  fi
fi

if [ "$BASE" = dev ] && [ "$HEAD_BRANCH" = main ]; then
  echo "PRs into dev must not come from main" >&2
  exit 1
fi

if [ "$BASE" = dev ] && ! grep -Eq "$pattern" <<<"$TITLE"; then
  echo "PR title must follow Conventional Commits (for example: feat(ports): publish ports)" >&2
  exit 1
fi

subjects="$(git log --no-merges --format=%s "${BASE_SHA}..${HEAD_SHA}")"
while IFS= read -r subject; do
  [ -n "$subject" ] || continue
  if ! grep -Eq "$pattern" <<<"$subject"; then
    echo "commit subject does not follow Conventional Commits: $subject" >&2
    exit 1
  fi
done <<<"$subjects"
