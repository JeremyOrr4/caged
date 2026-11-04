#!/usr/bin/env bash
# Print the next stable version without a leading v, or nothing when there is no release.
set -euo pipefail

INITIAL_VERSION="${INITIAL_VERSION:-0.1.0}"

last_tag="$(git tag --list 'v*' --merged HEAD --sort=-v:refname \
  | grep -E '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$' | head -n1 || true)"

range=HEAD
if [ -n "$last_tag" ]; then
  range="${last_tag}..HEAD"
fi

if [ -z "$last_tag" ]; then
  echo "$INITIAL_VERSION"
  exit 0
fi

if [ "$(git rev-list --count "$range")" -eq 0 ]; then
  exit 0
fi

IFS=. read -r major minor patch <<<"${last_tag#v}"
patch=$((patch + 1))

echo "${major}.${minor}.${patch}"
