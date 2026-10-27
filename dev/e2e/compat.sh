# shellcheck shell=bash
# compat.sh: the commands whose options differ between GNU (Linux) and BSD (macOS) userland, chosen once
# by `uname`. Sourced by e2e.sh and selftest.sh; needs bash 3.2 or later. Everything else the suite
# uses is POSIX or has the same options on both (cksum instead of md5sum, grep -E instead of \|).

HOST_OS=$(uname -s | tr '[:upper:]' '[:lower:]')               # Go's names, as caged prints them: linux or darwin
# shellcheck disable=SC2034 # read by the scripts that source this file
HOST_ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/') # Docker's names: amd64 or arm64

# abspath <path>: the absolute path with every symlink resolved, like GNU `readlink -f`: the last
# component need not exist, its parent must.
abspath() {
  if [ -d "$1" ]; then (cd "$1" && pwd -P); else
    local parent; parent=$(cd "$(dirname "$1")" && pwd -P) || return 1
    printf '%s/%s\n' "${parent%/}" "$(basename "$1")"
  fi
}

# file_mode <path>: the permission bits in octal, e.g. 600. file_uid <path>: the owner's user ID.
if [ "$HOST_OS" = darwin ]; then
  file_mode() { stat -f %Lp "$1"; }
  file_uid()  { stat -f %u "$1"; }
else
  file_mode() { stat -c %a "$1"; }
  file_uid()  { stat -c %u "$1"; }
fi

# lan_ip: the host's first non-loopback IPv4 address, or nothing when it has none.
lan_ip() {
  if [ "$HOST_OS" = darwin ]; then
    ipconfig getifaddr en0 2>/dev/null || ipconfig getifaddr en1 2>/dev/null
  else
    hostname -I 2>/dev/null | awk '{print $1}'
  fi
}

# run_on_tty cmd args...: runs cmd on a pseudo-terminal with this stdin, and exits with cmd's status.
# util-linux script takes the command as one string after -c; BSD script takes it as arguments.
run_on_tty() {
  if [ "$HOST_OS" = darwin ]; then
    script -q /dev/null "$@"
  else
    script -qec "$(printf '%q ' "$@")" /dev/null
  fi
}

# sed_in_place <sed args...> <file>: sed -i without GNU's optional suffix or BSD's required one.
sed_in_place() {
  local file=${!#}
  local tmp="$file.sed-tmp"
  sed "$@" > "$tmp" && mv "$tmp" "$file"
}
