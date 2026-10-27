#!/bin/bash
# selftest.sh: tests of the e2e suite's own helpers (guard.sh, compat.sh) that need no Docker and no
# network: a fake `docker` on PATH stands in for the daemon. Runs on Linux and macOS (bash 3.2).
# usage: selftest.sh        exit 0 when every check passes
set -u
HERE=$(cd "$(dirname "$0")" && pwd -P)
T=$(mktemp -d "${TMPDIR:-/tmp}/caged-e2e-selftest.XXXXXX") || exit 2
trap 'rm -rf "$T"' EXIT
pass=0; fail=0
ok()  { pass=$((pass+1)); printf '  ok   %s\n' "$1"; }
bad() { fail=$((fail+1)); printf '  FAIL %s\n         %s\n' "$1" "$2"; }
eq()  { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1" "got [$2] want [$3]"; fi; }
has() { case "$2" in *"$3"*) ok "$1";; *) bad "$1" "missing [$3] in: $2";; esac; }
hasnt(){ case "$2" in *"$3"*) bad "$1" "unexpected [$3] in: $2";; *) ok "$1";; esac; }

# ---------------------------------------------------------------- guard.sh, against a fake docker
# The fake lists FAKE_IMAGES ("repo:tag id" lines) for `docker images`, fails `save` when FAKE_SAVE_FAIL=1
# and `load` when FAKE_LOAD_FAIL=1, and logs every call to $T/docker.log.
mkdir -p "$T/fake"
cat > "$T/fake/docker" <<'EOF'
#!/bin/bash
echo "$*" >> "$FAKE_LOG"
case "$1" in
  info) echo /var/lib/docker ;;
  images) list=${FAKE_IMAGES:-}; [ -e "$FAKE_LOG.loaded" ] && list=${FAKE_IMAGES_AFTER_LOAD:-$list}
    case " $* " in
      *" -q "*) [ -n "$list" ] && printf '%s\n' "$list" | awk '{ print $2 }' ;;
      *) [ -n "$list" ] && printf '%s\n' "$list" ;;
    esac ;;
  save) [ "${FAKE_SAVE_FAIL:-}" = 1 ] && exit 1; [ "$2" = -o ] && echo tar > "$3" ;;
  load) [ "${FAKE_LOAD_FAIL:-}" = 1 ] && exit 1; [ -f "$3" ] || exit 1; : > "$FAKE_LOG.loaded" ;;
esac
exit 0
EOF
chmod +x "$T/fake/docker"
# guard <env assignments...>: runs image_guard in a child bash, then a stand-in for the suite.
guard() {
  rm -f "$T/docker.log" "$T/docker.log.loaded" "$T/saved-caged-images.tar"
  OUT=$(env PATH="$T/fake:$PATH" FAKE_LOG="$T/docker.log" "$@" bash -c '. "$1/guard.sh"; image_guard "$2"; echo SUITE-RAN; exit 5' _ "$HERE" "$T" 2>&1)
  RC=$?; LOG=$(cat "$T/docker.log" 2>/dev/null)
}
TWO='caged-runtime:latest sha256:aaa
<none>:<none> sha256:bbb'

guard FAKE_IMAGES=; eq "G1 no caged images: the suite runs" "$RC" 5; hasnt "G1 nothing saved" "$LOG" save
guard FAKE_IMAGES="$TWO"; eq "G2 caged images: refused with exit 2" "$RC" 2; has "G2 says why" "$OUT" "refusing to run"
has "G2 names the way out" "$OUT" "E2E_SAVE_IMAGES=1"; hasnt "G2 the suite did not run" "$OUT" SUITE-RAN; hasnt "G2 nothing saved" "$LOG" save
guard FAKE_IMAGES="$TWO" E2E_THROWAWAY_DAEMON=1; eq "G3 a throwaway daemon runs" "$RC" 5; hasnt "G3 nothing saved" "$LOG" save
guard FAKE_IMAGES="$TWO" E2E_SAVE_IMAGES=1; eq "G4 save and restore: the suite runs, its exit code kept" "$RC" 5
has "G4 saves the tag and the untagged image by ID" "$LOG" "save -o $T/saved-caged-images.tar caged-runtime:latest sha256:bbb"
has "G4 says so" "$OUT" "saved 2 caged images"; has "G4 loads them back at exit" "$LOG" "load -i $T/saved-caged-images.tar"
has "G4 reports the restore" "$OUT" "restored the saved caged images"; [ ! -e "$T/saved-caged-images.tar" ] && ok "G4 the tar is removed after the restore" || bad "G4 tar removed" "still there"
guard FAKE_IMAGES="$TWO" E2E_SAVE_IMAGES=1 FAKE_SAVE_FAIL=1; eq "G5 a failed save refuses with exit 2" "$RC" 2
has "G5 says the save failed" "$OUT" "could not save the caged images"; hasnt "G5 the suite did not run" "$OUT" SUITE-RAN; hasnt "G5 nothing to load" "$LOG" load
guard FAKE_IMAGES="$TWO" E2E_SAVE_IMAGES=1 FAKE_LOAD_FAIL=1; eq "G6 a failed restore keeps the exit code" "$RC" 5
has "G6 tells how to restore by hand" "$OUT" "docker load -i $T/saved-caged-images.tar"; [ -e "$T/saved-caged-images.tar" ] && ok "G6 and keeps the tar" || bad "G6 keeps the tar" "removed"
OUT=$(env PATH="$T/fake:$PATH" FAKE_LOG="$T/docker.log" FAKE_IMAGES="$TWO" E2E_SAVE_IMAGES=1 bash -c '. "$1/guard.sh"; image_guard "$2"; echo SUITE-RAN' _ "$HERE" "$T" 2>&1); RC=$?
eq "G7 a tar left by an earlier run refuses with exit 2" "$RC" 2; has "G7 says to load it first" "$OUT" "from an earlier run is still there"; hasnt "G7 the suite did not run" "$OUT" SUITE-RAN
guard FAKE_IMAGES="$TWO" E2E_SAVE_IMAGES=1 FAKE_IMAGES_AFTER_LOAD="$TWO
caged-runtime:ext sha256:ccc"
has "G8 after the load, a caged image the suite left is removed" "$LOG" "rmi caged-runtime:ext"; hasnt "G8 the saved ones are kept" "$LOG" "rmi caged-runtime:latest"; hasnt "G8 the saved untagged one too" "$LOG" "rmi sha256:bbb"

# ---------------------------------------------------------------- compat.sh, on this host
. "$HERE/compat.sh"
case "$HOST_OS" in linux|darwin) ok "K1 HOST_OS is $HOST_OS";; *) bad "K1 HOST_OS" "unexpected [$HOST_OS]";; esac
case "$HOST_ARCH" in amd64|arm64) ok "K1 HOST_ARCH is $HOST_ARCH";; *) bad "K1 HOST_ARCH" "unexpected [$HOST_ARCH]";; esac
mkdir -p "$T/real/sub"; ln -s real "$T/link"; REAL=$(cd "$T/real" && pwd -P)
eq "K2 abspath resolves a linked directory" "$(abspath "$T/link")" "$REAL"
eq "K2 abspath: the last component need not exist" "$(abspath "$T/link/new")" "$REAL/new"
eq "K2 abspath of a relative path" "$(cd "$T" && abspath link/sub)" "$REAL/sub"
eq "K2 abspath of a file in /" "$(abspath /nonexistent-e2e)" "/nonexistent-e2e"
: > "$T/f"; chmod 640 "$T/f"; chmod 700 "$T/real"
eq "K3 file_mode of a file" "$(file_mode "$T/f")" 640; eq "K3 file_mode of a directory" "$(file_mode "$T/real")" 700
eq "K4 file_uid is the owner" "$(file_uid "$T/f")" "$(id -u)"
IP=$(lan_ip); case "$IP" in "") ok "K5 lan_ip: none";; 127.*) bad "K5 lan_ip" "loopback [$IP]";;
  *.*.*.*) ok "K5 lan_ip is an IPv4 address ($IP)";; *) bad "K5 lan_ip" "not IPv4 [$IP]";; esac
OUT=$(printf 'typed\n' | run_on_tty sh -c 'test -t 0 && echo ON-TTY; read x; echo "got:$x"; exit 7' 2>&1); RC=$?
has "K6 run_on_tty runs on a terminal" "$OUT" ON-TTY; has "K6 run_on_tty passes stdin" "$OUT" "got:typed"; eq "K6 run_on_tty keeps the exit code" "$RC" 7
OUT=$(run_on_tty printf '%s|' 'a b' '$(id)' "it's" 2>&1); has "K6 run_on_tty passes arguments literally" "$(echo "$OUT" | tr -d '\r')" "a b|\$(id)|it's|"
printf 'FROM x@sha256:1\nRUN y\n' > "$T/D"; sed_in_place -E 's/^(FROM x)@sha256:1/\1/' "$T/D"
eq "K7 sed_in_place edits the file" "$(cat "$T/D")" "$(printf 'FROM x\nRUN y')"; [ ! -e "$T/D.sed-tmp" ] && ok "K7 and leaves no temp file" || bad "K7 no temp file" "$T/D.sed-tmp exists"

printf '\n==== selftest: %d passed, %d failed\n' "$pass" "$fail"
[ "$fail" = 0 ]
