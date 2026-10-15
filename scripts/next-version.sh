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

subjects="$(git log --no-merges --format=%s "$range")"
bodies="$(git log --no-merges --format=%b "$range")"

releasable='^(feat|fix|perf)(\([^)]+\))?!?:'
breaking_subject='^[a-z]+(\([^)]+\))?!:'
breaking_body='^BREAKING[ -]CHANGE:'

# Breaking-change footers count regardless of the subject type.
grep -Eq "${releasable}|${breaking_subject}" <<<"$subjects" \
  || grep -Eq "$breaking_body" <<<"$bodies" \
  || exit 0

breaking=false
if grep -Eq "$breaking_subject" <<<"$subjects" || grep -Eq "$breaking_body" <<<"$bodies"; then
  breaking=true
fi

feature=false
if grep -Eq '^feat(\([^)]+\))?!?:' <<<"$subjects"; then
  feature=true
fi

if [ -z "$last_tag" ]; then
  echo "$INITIAL_VERSION"
  exit 0
fi

IFS=. read -r major minor patch <<<"${last_tag#v}"

if [ "$major" -eq 0 ]; then
  # Slow pre-1.0 policy: releasable = patch, breaking = minor.
  if $breaking; then
    minor=$((minor + 1))
    patch=0
  else
    patch=$((patch + 1))
  fi
else
  if $breaking; then
    major=$((major + 1))
    minor=0
    patch=0
  elif $feature; then
    minor=$((minor + 1))
    patch=0
  else
    patch=$((patch + 1))
  fi
fi

echo "${major}.${minor}.${patch}"
