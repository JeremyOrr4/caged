#!/usr/bin/env bash
set -euo pipefail
umask 000

run_as=""
if [ "${1:-}" = "--caged-run-as" ]; then
  run_as="${2:-}"
  if [[ ! "${run_as}" =~ ^[0-9]+:[0-9]+$ ]]; then
    echo "invalid internal run-as identity" >&2
    exit 2
  fi
  shift 2
  if [ "${1:-}" = "--" ]; then
    shift
  fi
fi

prepare_state_directories() {
  [ -n "${run_as}" ] || return 0
  [ "$(id -u)" -eq 0 ] || {
    echo "cannot prepare persistent state volumes without root privileges" >&2
    exit 1
  }

  for path in \
    /home/caged/.config/pi \
    /home/caged/.pi \
    /home/caged/.claude \
    /home/caged/.codex \
    /home/caged/.gemini
  do
    mkdir -p "${path}"
    chmod -R a+rwX "${path}"
  done
}

copy_host_config() {
  if [ -z "${CAGED_CONFIG_MAP:-}" ] || [ "${CAGED_CONFIG_MAP}" = "null" ]; then
    return 0
  fi

  python3 - "$CAGED_CONFIG_MAP" <<'PY' | while IFS=$'\t' read -r src dst; do
import fnmatch
import json
import os
import sys

payload = json.loads(sys.argv[1] or "[]")
for item in payload:
    staged = item.get("staged")
    dest = item.get("dest")
    if not staged or not dest:
        continue
    excludes = item.get("excludes", []) or []
    if not os.path.exists(staged):
        continue

    if os.path.isdir(staged):
        for root, dirs, files in os.walk(staged):
            rel_root = os.path.relpath(root, staged)
            if rel_root == ".":
                rel_root = ""
            new_dirs = []
            for d in dirs:
                rel = os.path.join(rel_root, d) if rel_root else d
                if any(fnmatch.fnmatch(rel, pat) or fnmatch.fnmatch(d, pat) for pat in excludes):
                    continue
                new_dirs.append(d)
            dirs[:] = new_dirs
            for name in files:
                rel = os.path.join(rel_root, name) if rel_root else name
                if any(fnmatch.fnmatch(rel, pat) or fnmatch.fnmatch(name, pat) for pat in excludes):
                    continue
                src = os.path.join(root, name)
                dst = os.path.join(dest, rel)
                os.makedirs(os.path.dirname(dst), exist_ok=True)
                print(f"{src}\t{dst}")
        continue

    name = os.path.basename(staged)
    if any(fnmatch.fnmatch(name, pat) for pat in excludes):
        continue
    os.makedirs(os.path.dirname(dest), exist_ok=True)
    print(f"{staged}\t{dest}")
PY
    [ -n "${src:-}" ] || continue
    mkdir -p "$(dirname "${dst}")"
    mkdir -p "$(dirname "${dst}")"
    chmod -R a+rwX "$(dirname "${dst}")" 2>/dev/null || true
    cp -a "${src}" "${dst}"
    chmod -R a+rwX "$(dirname "${dst}")" 2>/dev/null || true
  done
}

prepare_state_directories
copy_host_config

if [ -n "${run_as}" ]; then
  uid="${run_as%%:*}"
  gid="${run_as##*:}"
  exec setpriv --reuid "${uid}" --regid "${gid}" --clear-groups -- "$@"
fi

if [ "$#" -gt 0 ]; then
  exec "$@"
fi

exec bash
