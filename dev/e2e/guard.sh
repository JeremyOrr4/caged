# shellcheck shell=bash
# guard.sh: the e2e suite's image guard. Sourced by e2e.sh and selftest.sh; needs bash 3.2 or later.
# The suite runs `caged clean` and removes caged-runtime images, so it must never meet a daemon whose
# caged images someone still needs. image_guard refuses such a daemon (exit 2), unless the caller
# declares it throwaway (E2E_THROWAWAY_DAEMON=1) or asks for the images to be saved first
# (E2E_SAVE_IMAGES=1): then they are written to a tar with `docker save` and loaded back when the
# suite exits, and a failed save refuses the run.

CAGED_LABEL=io.github.jeremyorr4.caged.role

# caged_images: one line per caged image, its repo:tag, or its ID when it has no tag.
caged_images() {
  { docker images 'caged-runtime*' --format '{{.Repository}}:{{.Tag}} {{.ID}}'
    docker images --filter "label=$CAGED_LABEL" --format '{{.Repository}}:{{.Tag}} {{.ID}}'
  } | awk '{ print ($1 == "<none>:<none>" ? $2 : $1) }' | sort -u
}

# ncaged: how many distinct caged images the daemon holds.
ncaged() { { docker images 'caged-runtime*' -q; docker images -q --filter "label=$CAGED_LABEL"; } | sort -u | wc -l | tr -d ' '; }

# image_guard <scratch dir>: refuse, or save and arrange the restore, before the suite touches anything.
image_guard() {
  [ "${E2E_THROWAWAY_DAEMON:-}" = 1 ] && return 0
  [ "$(ncaged)" = 0 ] && return 0
  local root; root=$(docker info --format '{{.DockerRootDir}}' 2>/dev/null)
  if [ "${E2E_SAVE_IMAGES:-}" != 1 ]; then
    echo "e2e: refusing to run: the Docker daemon at $root holds caged images that this suite would delete." >&2
    echo "e2e: set E2E_SAVE_IMAGES=1 to save them first and restore them at the end, point DOCKER_HOST at a throwaway daemon (then set E2E_THROWAWAY_DAEMON=1), or remove those images yourself first." >&2
    exit 2
  fi
  SAVED_IMAGES=$1/saved-caged-images.tar
  if [ -e "$SAVED_IMAGES" ]; then  # an earlier restore failed: saving again would overwrite those images
    echo "e2e: refusing to run: $SAVED_IMAGES from an earlier run is still there; load it with docker load -i $SAVED_IMAGES, then delete it." >&2
    exit 2
  fi
  local refs; refs=$(caged_images); SAVED_REFS=$refs
  # Word splitting of $refs is intended: one argument per image.
  # shellcheck disable=SC2086
  if ! docker save -o "$SAVED_IMAGES" $refs; then
    echo "e2e: refusing to run: could not save the caged images of the daemon at $root to $SAVED_IMAGES." >&2
    rm -f "$SAVED_IMAGES"
    exit 2
  fi
  echo "e2e: saved $(echo "$refs" | wc -l | tr -d ' ') caged images to $SAVED_IMAGES; they are loaded back when the suite exits."
  trap restore_images EXIT
  trap 'exit 130' INT
  trap 'exit 143' TERM
}

# restore_images: the EXIT trap. It keeps the suite's exit status and keeps the tar when loading fails.
# After the load it removes the caged images the suite left that were not saved (tags such as :ext).
restore_images() {
  local rc=$? ref
  trap '' INT TERM  # a second Ctrl-C must not cut the restore short
  if docker load -i "$SAVED_IMAGES" >/dev/null; then
    echo "e2e: restored the saved caged images."
    rm -f "$SAVED_IMAGES"
    for ref in $(caged_images); do
      case "
$SAVED_REFS
" in *"
$ref
"*) ;; *) docker rmi "$ref" >/dev/null 2>&1 || true ;; esac  # best effort: an image still tagged stays
    done
  else
    echo "e2e: could not restore the saved caged images; load them yourself with: docker load -i $SAVED_IMAGES" >&2
  fi
  return $rc
}
