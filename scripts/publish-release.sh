#!/usr/bin/env bash
# Publish or recover a stable GitHub release and notify the Homebrew tap.
set -euo pipefail

tag="${1:?usage: publish-release.sh vMAJOR.MINOR.PATCH}"
if ! [[ "$tag" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]; then
  echo "not a stable release tag: $tag" >&2
  exit 2
fi

version="${tag#v}"
repo="${GITHUB_REPOSITORY:-JeremyOrr4/caged}"
token="${GH_TOKEN:-${GITHUB_TOKEN:-}}"
if [ -z "$token" ]; then
  echo "GH_TOKEN or GITHUB_TOKEN is required" >&2
  exit 2
fi

if ! git merge-base --is-ancestor "$tag" origin/main; then
  echo "release tag $tag is not reachable from origin/main" >&2
  exit 1
fi

release_json="$(GH_TOKEN="$token" gh api "repos/$repo/releases/tags/$tag" 2>/dev/null || true)"
draft=""
if [ -n "$release_json" ]; then
  draft="$(jq -r '.draft' <<<"$release_json")"
fi

tmp="$(mktemp -d)"
release_checkout="$tmp/release-source"
cleanup() {
  git worktree remove --force "$release_checkout" >/dev/null 2>&1 || true
  rm -rf "$tmp"
}
trap cleanup EXIT

if [ "$draft" != false ]; then
  git worktree add --detach "$release_checkout" "$tag"
  (cd "$release_checkout" && GITHUB_TOKEN="$token" goreleaser release --clean)
fi

release_json="$(GH_TOKEN="$token" gh api "repos/$repo/releases/tags/$tag")"
release_id="$(jq -r '.id' <<<"$release_json")"
if [ "$release_id" = null ]; then
  echo "GoReleaser did not create a release for $tag" >&2
  exit 1
fi

mkdir "$tmp/assets"
GH_TOKEN="$token" gh release download "$tag" --repo "$repo" --dir "$tmp/assets"

assets="$tmp/assets"
asset_count="$(find "$assets" -maxdepth 1 -type f -name "caged_${version}_*.tar.gz" | wc -l | tr -d ' ')"
if [ "$asset_count" -ne 4 ] || [ ! -f "$assets/checksums.txt" ]; then
  echo "release $tag must contain four platform archives and checksums.txt" >&2
  exit 1
fi
(cd "$assets" && shasum -a 256 -c checksums.txt)

mkdir "$tmp/linux-amd64"
tar -xzf "$assets/caged_${version}_linux_amd64.tar.gz" -C "$tmp/linux-amd64" caged
reported="$("$tmp/linux-amd64/caged" --version)"
if [ "$reported" != "caged version $version" ]; then
  echo "Linux binary reports '$reported'; expected 'caged version $version'" >&2
  exit 1
fi

if [ "$(jq -r '.draft' <<<"$release_json")" = true ]; then
  GH_TOKEN="$token" gh release edit "$tag" --repo "$repo" --draft=false --latest
fi

module="github.com/JeremyOrr4/caged/cmd/caged@v$version"
mkdir "$tmp/gobin"
for attempt in $(seq 1 10); do
  if go list -m "github.com/JeremyOrr4/caged@v$version" \
    && GOBIN="$tmp/gobin" go install "$module" \
    && reported="$("$tmp/gobin/caged" --version)" \
    && [ "$reported" = "caged version $version" ]; then
    break
  fi
  if [ "$attempt" -eq 10 ]; then
    echo "Go module proxy check failed for v$version" >&2
    exit 1
  fi
  sleep 30
done

GH_TOKEN="$token" gh api --method POST repos/JeremyOrr4/homebrew-tap/dispatches \
  -f event_type=caged_release \
  -F "client_payload[version]=v$version"
