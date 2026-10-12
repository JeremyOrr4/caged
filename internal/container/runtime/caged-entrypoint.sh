#!/usr/bin/env bash
set -euo pipefail
umask 000

copy_host_config() {
  if [ -z "${CAGED_CONFIG_MAP:-}" ] || [ "${CAGED_CONFIG_MAP}" = "null" ]; then
    return 0
  fi

  python3 - "$CAGED_CONFIG_MAP" <<'PY'
import fnmatch
import json
import os
import shutil
import sys

payload = json.loads(sys.argv[1] or "[]")

def is_excluded(relative, name, patterns):
    return any(
        fnmatch.fnmatch(relative, pattern) or fnmatch.fnmatch(name, pattern)
        for pattern in patterns
    )

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
            dirs[:] = [
                name for name in dirs
                if not is_excluded(os.path.join(rel_root, name) if rel_root else name, name, excludes)
            ]
            target_root = os.path.join(dest, rel_root) if rel_root else dest
            os.makedirs(target_root, exist_ok=True)
            for name in files:
                rel = os.path.join(rel_root, name) if rel_root else name
                if is_excluded(rel, name, excludes):
                    continue
                shutil.copy2(
                    os.path.join(root, name),
                    os.path.join(target_root, name),
                    follow_symlinks=False,
                )
        continue

    name = os.path.basename(staged)
    if is_excluded(name, name, excludes):
        continue
    os.makedirs(os.path.dirname(dest), exist_ok=True)
    shutil.copy2(staged, dest, follow_symlinks=False)
PY
}

copy_host_config

if [ "$#" -gt 0 ]; then
  if [ "${CAGED_RETURN_TO_SHELL:-}" = "1" ]; then
    set +e
    "$@"
    agent_status=$?
    set -e
    printf '\nAgent exited (status %s). Opening a container shell; type exit to finish.\n' "${agent_status}"
    bash -i
    exit "$agent_status"
  fi

  exec "$@"
fi

exec bash
