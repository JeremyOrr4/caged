#!/bin/bash
# bsd-userland.sh: runs a command under macOS's userland rules on a Linux host, to catch GNU-only
# options before a Mac does. A folder put first on PATH holds wrappers that
# - fail loudly (exit 97, "BSD-SHIM:" on stderr and in the log) on options BSD userland lacks: readlink -f,
#   stat -c, sed -i/-r, grep -P and a BRE \|, date -d, hostname -I, xargs -r, find -printf, GNU long options;
# - translate the BSD spellings compat.sh uses (stat -f, script -q FILE CMD..., ipconfig getifaddr) to GNU;
# - pad `wc` counts to eight columns, as BSD wc does;
# - make `uname -s` print Darwin, so compat.sh takes its macOS branches;
# - leave out md5sum and timeout, which macOS does not have.
# usage: bsd-userland.sh cmd args...    e.g.  dev/e2e/bsd-userland.sh bash dev/e2e/selftest.sh
# Exits with cmd's status, or 97 when any wrapper refused a GNU-only use, even one cmd's own
# redirections hid. The log of refusals is kept in $BSD_SHIM_LOG (default: a temp file, printed).
set -u
SHIM=$(mktemp -d "${TMPDIR:-/tmp}/bsd-userland.XXXXXX") || exit 2
trap 'rm -rf "$SHIM"' EXIT
export BSD_SHIM_LOG=${BSD_SHIM_LOG:-$SHIM.log}
: > "$BSD_SHIM_LOG"

# wrapper NAME BODY: writes $SHIM/NAME; BODY sees REAL (the GNU tool) and refuse "reason".
wrapper() {
  local real; real=$(command -v "$1") || real=/nonexistent/$1
  { printf '#!/bin/bash\nREAL=%q\n' "$real"
    printf 'ARGS="$*"\nrefuse() { echo "BSD-SHIM: %s $ARGS: $1" | tee -a %q >&2; exit 97; }\n' "$1" "$BSD_SHIM_LOG"
    printf '%s\n' "$2"
  } > "$SHIM/$1"
  chmod +x "$SHIM/$1"
}
# no_long_options: GNU long options (--foo) do not exist in BSD userland; "--" alone ends the options.
NO_LONG='for a in "$@"; do case "$a" in --) break;; --*) refuse "no GNU long option $a in BSD userland";; esac; done'

wrapper uname 'for a in "$@"; do [ "$a" = -s ] && { echo Darwin; exit 0; }; done; exec "$REAL" "$@"'
wrapper readlink "$NO_LONG"'
for a in "$@"; do case "$a" in -f|-e|-m|-*[fem]*) refuse "readlink -f/-e/-m are GNU-only";; esac; done; exec "$REAL" "$@"'
wrapper stat "$NO_LONG"'
[ "${1:-}" = -f ] || refuse "only stat -f FORMAT FILE is portable (no -c)"
case "$2" in %Lp) fmt=%a;; %u) fmt=%u;; %z) fmt=%s;; *) refuse "format $2 is not one compat.sh uses";; esac
shift 2; exec "$REAL" -c "$fmt" "$@"'
wrapper sed "$NO_LONG"'
ere=; for a in "$@"; do case "$a" in -i*) refuse "sed -i differs (BSD needs a suffix argument)";; -r) refuse "use sed -E, not -r";; -E) ere=1;; esac; done
if [ -z "$ere" ]; then for a in "$@"; do case "$a" in *"\\|"*|*"\\+"*|*"\\?"*) refuse "\\| \\+ \\? are GNU BRE extensions";; esac; done; fi
exec "$REAL" "$@"'
wrapper grep "$NO_LONG"'
ere=; for a in "$@"; do case "$a" in -P|-*P*) refuse "grep -P is GNU-only";; -*E*) ere=1;; esac; done
if [ -z "$ere" ]; then for a in "$@"; do case "$a" in *"\\|"*|*"\\+"*|*"\\?"*) refuse "\\| \\+ \\? are GNU BRE extensions (use grep -E)";; esac; done; fi
exec "$REAL" "$@"'
wrapper date "$NO_LONG"'
for a in "$@"; do case "$a" in -d|-d*) refuse "date -d is GNU-only";; esac; done; exec "$REAL" "$@"'
wrapper hostname "$NO_LONG"'
for a in "$@"; do case "$a" in -I|-i) refuse "hostname -I/-i are GNU-only";; esac; done; exec "$REAL" "$@"'
wrapper xargs "$NO_LONG"'
for a in "$@"; do case "$a" in -r|-*r*) refuse "xargs -r is GNU-only";; esac; done; exec "$REAL" "$@"'
wrapper find "$NO_LONG"'
for a in "$@"; do case "$a" in -printf|-fprintf|-regextype) refuse "find $a is GNU-only";; esac; done; exec "$REAL" "$@"'
wrapper cp "$NO_LONG"'
exec "$REAL" "$@"'
wrapper mktemp "$NO_LONG"'
exec "$REAL" "$@"'
wrapper script '
q=; while case "${1:-}" in -*) true;; *) false;; esac; do
  [ "$1" = -q ] || refuse "script $1: only BSD script -q FILE CMD... is portable (no util-linux -c/-e)"; q=-q; shift
done
file=$1; shift
if [ $# = 0 ]; then exec "$REAL" $q "$file"; fi
exec "$REAL" $q -ec "$(printf "%q " "$@")" "$file"'
wrapper ipconfig "HOSTNAME_REAL=$(command -v hostname)"'
[ "${1:-}" = getifaddr ] || refuse "only ipconfig getifaddr IFACE is emulated"
ip=$("$HOSTNAME_REAL" -I 2>/dev/null | awk "{print \$1}"); [ -n "$ip" ] && [ "$2" = en0 ] && { echo "$ip"; exit 0; }; exit 1'
wrapper wc '"$REAL" "$@" | awk "{ for (i = 1; i <= NF; i++) if (\$i ~ /^[0-9]+\$/) \$i = sprintf(\"%8d\", \$i); print }"; exit "${PIPESTATUS[0]}"'
for absent in md5sum timeout; do
  wrapper "$absent" 'refuse "does not exist on macOS"'
done

PATH="$SHIM:$PATH" "$@"; rc=$?
if [ -s "$BSD_SHIM_LOG" ]; then
  echo "bsd-userland: GNU-only uses refused (log: $BSD_SHIM_LOG):" >&2; sort -u "$BSD_SHIM_LOG" >&2; exit 97
fi
echo "bsd-userland: no GNU-only use (log: $BSD_SHIM_LOG)" >&2
exit "$rc"
