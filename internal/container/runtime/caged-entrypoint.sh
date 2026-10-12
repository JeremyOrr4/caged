#!/bin/sh

if [ "${CAGED_RETURN_TO_SHELL:-}" != "1" ]; then
  exec "$@"
fi

"$@"
status=$?
printf '\nAgent exited (status %s). Opening a container shell; type exit to finish.\n' "$status"
bash -i
exit "$status"
