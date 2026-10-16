#!/bin/bash
# End-to-end tests of caged against REAL Docker, using a stand-in runtime image (see README.md).
# Normally run inside the test container by `make e2e`; it can also run directly on a Linux host with Docker.
# usage: e2e.sh <scratch dir> <repo dir> [section letters, default: all]   e.g.  e2e.sh /tmp/e2e ~/git/misc/caged AB
# Touches only caged-runtime:* images and containers named caged-e2e-*, ports 38000-38999, and HOME=<scratch>/home.
# Environment: E2E_UNPIN=1 drops the @sha256 pin from the base image (an offline daemon cannot verify it);
#              /vendor, when present, is used for the Go modules (no network).
set -u
S=$(readlink -f "$1"); REPO=$(readlink -f "$2"); SECTIONS=${3:-ABCDEFGHLWI}
command -v go >/dev/null 2>&1 || export PATH=$HOME/sdk/go1.27.2/bin:$HOME/go/bin:$PATH
export GOTOOLCHAIN=local GOFLAGS=-buildvcs=false
REALHOME=$HOME; [ -d "$REALHOME/.docker" ] && export DOCKER_CONFIG=$REALHOME/.docker
HERE=$(dirname "$(readlink -f "$0")")
pass=0; fail=0; failed=()
ok()   { pass=$((pass+1)); printf '  ok   %s\n' "$1"; }
bad()  { fail=$((fail+1)); failed+=("$1"); printf '  FAIL %s\n         %s\n' "$1" "${2:0:400}"; }
eq()   { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1" "got [$2] want [$3]"; fi; }
has()  { case "$2" in *"$3"*) ok "$1";; *) bad "$1" "missing [$3] in: $2";; esac; }
hasnt(){ case "$2" in *"$3"*) bad "$1" "unexpected [$3] in: $2";; *) ok "$1";; esac; }
section(){ printf '\n== %s\n' "$1"; }
inS()  { case "$SECTIONS" in *"$1"*) return 0;; *) return 1;; esac; }

# tty "<stdin>" cmd args...   -> OUT (output without CRs), RC (caged's exit code). Runs on a pseudo-terminal.
tty() { local input=$1; shift; local f=$S/tty.out
  printf '%s\n' "$input" | script -qec "$(printf '%q ' "$@")" /dev/null > "$f" 2>&1; RC=${PIPESTATUS[1]}; OUT=$(tr -d '\r' < "$f"); }
# plain cmd args... -> OUT (stdout+stderr), RC, no terminal
plain() { OUT=$("$@" 2>&1); RC=$?; }
imgid() { docker image inspect "$1" --format '{{.Id}}' 2>/dev/null; }
label() { docker image inspect "$1" --format "{{index .Config.Labels \"io.github.jeremyorr4.caged.$2\"}}" 2>/dev/null; }
ncaged() { { docker images 'caged-runtime*' -q; docker images -q --filter label=io.github.jeremyorr4.caged.role; } | sort -u | wc -l | tr -d ' '; }
others() { docker images --format '{{.Repository}}:{{.Tag}}' | grep -v '^caged-runtime:' | sort | md5sum | cut -c1-12; }

# ---------------------------------------------------------------- preparation
mkdir -p "$S"/{proj,home,xdg,bin,shim,other}
rsync -a --delete --exclude .git --exclude bin --exclude dist --exclude docs/local --exclude vendor "$REPO"/ "$S/src/"
if [ -d /vendor ]; then rm -rf "$S/src/vendor"; cp -r /vendor "$S/src/vendor"; export GOFLAGS="-buildvcs=false -mod=vendor"; fi
build_variant() { # name version dockerfile-comment [extra dockerfile line]
  python3 "$HERE/make-dockerfile.py" "$REPO/internal/images/Dockerfile" "$S/src/internal/images/Dockerfile" "$3"
  [ -n "${E2E_UNPIN:-}" ] && sed -i -E 's/^(FROM ubuntu:24\.04)@sha256:[0-9a-f]{64}/\1/' "$S/src/internal/images/Dockerfile"
  [ -n "${4:-}" ] && printf '%s\n' "$4" >> "$S/src/internal/images/Dockerfile"
  (cd "$S/src" && go build -ldflags "-X main.version=$2" -o "$S/bin/$1" ./cmd/caged) || { echo "build of $1 failed"; exit 2; }
}
build_variant r1  r1 "variant r1"
build_variant r2  r2 "variant r2"
build_variant r2b r2 "variant r2b"
build_variant r3bad r3 "variant r3" "RUN false"
build_variant r1b r1 "variant r1 again" # same version, other files
(cd "$S/src" && go build -o "$S/bin/nover" ./cmd/caged && go build -ldflags "-X main.version=v1.2.3" -o "$S/bin/vtag" ./cmd/caged)
C1=$S/bin/r1; C2=$S/bin/r2; C2B=$S/bin/r2b; C3=$S/bin/r3bad; C1B=$S/bin/r1b
printf '#!/bin/sh\necho "$@" >> %s/shim.log\nexit 1\n' "$S" > "$S/shim/docker"; chmod +x "$S/shim/docker"
cd "$S/proj"; echo "project file" > file.txt
tar -cf "$S/empty.tar" -T /dev/null  # an empty image archive, for the sections that docker-import one
export HOME=$S/home; export XDG_CONFIG_HOME=$S/xdg
UIDN=$(id -u); GIDN=$(id -g)
OTHERS_BEFORE=$(others)
$C1 clean >/dev/null 2>&1; docker rm -f caged-e2e-hold >/dev/null 2>&1
echo "caged images at the start: $(ncaged)"

# ---------------------------------------------------------------- S: quick smoke test (about 2 minutes)
if inS S; then section "S. smoke test: build, run, doctor, clean"
  plain $C1 --version; eq "S1 version" "$OUT" "caged version r1"
  plain $C1 build; eq "S2 build" "$RC" 0; has "S2 build says what it built" "$OUT" "built runtime image caged-runtime:latest"
  tty 'echo SMOKE-OK; id -un; exit' $C1 --no-config; has "S3 a shell runs in the container" "$OUT" "SMOKE-OK"; eq "S3 exit 0" "$RC" 0
  tty 'exit' $C1 --no-config claude 'hello' -- --model m; has "S4 an agent gets prompt and flags" "$OUT" "[hello]"; has "S4 flags after the prompt" "$OUT" "[--model]"
  plain $C1 doctor; eq "S5 doctor passes" "$RC" 0; hasnt "S5 no FAIL" "$OUT" "FAIL"; has "S5 versions read from the image" "$OUT" "PASS  versions"
  plain $C1 list; has "S6 list" "$OUT" "AGENT"
  plain $C1 clean; has "S7 clean removes the image" "$OUT" "removed caged-runtime:latest"; eq "S7 nothing left" "$(ncaged)" 0
fi

# ---------------------------------------------------------------- A: errors and basics
if inS A; then section "A. basics and error paths (R1, R3, R5, R8)"
  plain $C1 --version; eq "A1 --version prints the linked version" "$OUT" "caged version r1"
  plain env PATH=/nonexistent $C1 --no-config --no-mount; has "A2 docker missing is reported as such" "$OUT" "Docker is not installed or not on PATH"; eq "A2 exit code 1" "$RC" 1
  hasnt "A2 not reported as a missing image" "$OUT" "not found; building"
  for cmd in "--no-config --no-mount" "build" "clean" "doctor"; do
    plain env DOCKER_HOST=unix:///nonexistent $C1 $cmd; has "A3 daemon down: caged $cmd" "$OUT" "daemon is unavailable"; eq "A3 exit 1 ($cmd)" "$RC" 1
  done
  rm -f "$S/shim.log"
  for args in "-w /nonexistent" "-p 70000" "--no-mount -w /tmp" "--env bad-name" "-- --resume" "claude a b" "claude -p 3000-3005"; do
    plain env PATH="$S/shim:$PATH" $C1 $args; eq "A4 exit 1 for: $args" "$RC" 1
  done
  [ -e "$S/shim.log" ] && bad "A4 bad requests make no docker call" "$(cat $S/shim.log)" || ok "A4 bad requests make no docker call"
  plain env PATH=/nonexistent $C1 list; eq "A5 list needs no docker" "$RC" 0; has "A5 list shows agents" "$OUT" "claude"
  plain $C1 completion bash; eq "A6 completion bash generates" "$RC" 0; echo "$OUT" | bash -n && ok "A6 bash completion parses" || bad "A6 bash completion parses" ""
  plain $C1 __complete ""; has "A7 completion lists agents with descriptions" "$OUT" "claude	Claude Code"; has "A7 completion lists doctor" "$OUT" "doctor	"
  plain $C1 __complete claude ""; hasnt "A7 nothing suggested after the agent" "$OUT" "codex"
  plain $C1 help build; has "A8 help build" "$OUT" "--no-cache"
fi

# ---------------------------------------------------------------- B: build and freshness
if inS B; then section "B. image build and freshness (F2)"
  eq "B0 starts with no caged image" "$(ncaged)" 0
  plain $C1 build; has "B1 build says what it built" "$OUT" "built runtime image caged-runtime:latest"; eq "B1 exit 0" "$RC" 0
  eq "B2 role label" "$(label caged-runtime:latest role)" runtime
  eq "B2 version label" "$(label caged-runtime:latest version)" r1
  H=$(label caged-runtime:latest runtime-hash); eq "B2 hash label is 64 hex" "$(echo "$H" | grep -Ec '^[0-9a-f]{64}$')" 1
  want_arch=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/'); eq "B3 image architecture is the host's" "$(docker image inspect caged-runtime:latest --format '{{.Architecture}}')" "$want_arch"
  tty 'exit' $C1 --no-config --verbose; has "B4 current image: verbose says so" "$OUT" "caged-runtime:latest is current (runtime ${H:0:12})"; hasnt "B4 no rebuild" "$OUT" "rebuilding"; hasnt "B4 no build notice" "$OUT" "building it first"
  ID1=$(imgid caged-runtime:latest); tty 'exit' $C1 --no-config; eq "B5 image unchanged by a run" "$(imgid caged-runtime:latest)" "$ID1"
  docker image rm caged-runtime:latest >/dev/null; tty 'exit' $C1 --no-config; has "B6 missing image is built first" "$OUT" "Runtime image caged-runtime:latest not found; building it first..."; eq "B6 image exists afterwards" "$([ -n "$(imgid caged-runtime:latest)" ] && echo yes)" yes
  OLD=$(imgid caged-runtime:latest); tty 'exit' $C2 --no-config; has "B7 another caged version: both versions named" "$OUT" "was built by caged r1; rebuilding it for caged r2 (this can take a few minutes)..."
  eq "B7 new version label" "$(label caged-runtime:latest version)" r2; docker image inspect "$OLD" >/dev/null 2>&1 && bad "B7 old image removed" "still there" || ok "B7 old image removed"
  eq "B7 only one caged image left" "$(ncaged)" 1
  OLD=$(imgid caged-runtime:latest); tty 'exit' $C2B --no-config; has "B8 same version, other files" "$OUT" "is out of date (its runtime files changed); rebuilding..."; docker image inspect "$OLD" >/dev/null 2>&1 && bad "B8 old removed" "still there" || ok "B8 old image removed"
  OLD=$(imgid caged-runtime:latest); docker image rm caged-runtime:latest >/dev/null
  tar -cf "$S/empty.tar" -T /dev/null; docker import "$S/empty.tar" caged-runtime:latest >/dev/null
  tty 'exit' $C2 --no-config; has "B9 unlabeled image counts as old" "$OUT" "was built by an older caged; rebuilding it once"; eq "B9 labeled afterwards" "$(label caged-runtime:latest role)" runtime
  eq "B9 unlabeled image removed" "$(ncaged)" 1
  KEEP=$(imgid caged-runtime:latest); rm -f "$S/marker"
  tty "touch $S/marker; exit" $C3 --no-config; has "B10 failing rebuild reports the build error" "$OUT" "did not complete successfully"; eq "B10 exit code 1" "$RC" 1
  eq "B10 the old image is kept" "$(imgid caged-runtime:latest)" "$KEEP"; [ ! -e "$S/marker" ] && ok "B10 nothing ran after the failed build" || bad "B10 nothing ran" "marker exists"
  plain $C3 doctor; has "B11 doctor says the image is out of date" "$OUT" "WARN  image      caged-runtime:latest is out of date"
  OLD=$(imgid caged-runtime:latest); plain $C2 build --no-cache; has "B12 --no-cache build" "$OUT" "built runtime image"; [ "$(imgid caged-runtime:latest)" != "$OLD" ] && ok "B12 new image" || bad "B12 new image" same
  docker image inspect "$OLD" >/dev/null 2>&1 && bad "B12 old image removed" "still there" || ok "B12 old image removed"
  docker run -d --name caged-e2e-hold --entrypoint sleep caged-runtime:latest 300 >/dev/null
  tty 'exit' $C1 --no-config --verbose; has "B13 a container uses the old image: the rebuild still happens" "$OUT" "was built by caged r2; rebuilding it for caged r1"
  eq "B13 the run went on" "$RC" 0; hasnt "B13 no misleading 'kept old image' for an image the engine already dropped" "$OUT" "kept old image"; eq "B13 new image is the current one" "$(label caged-runtime:latest version)" r1
  docker rm -f caged-e2e-hold >/dev/null
  plain $C1 clean; has "B14 clean removes what is left" "$OUT" "removed"; eq "B14 nothing left" "$(ncaged)" 0
  plain $C1 build; eq "B15 rebuild from nothing" "$RC" 0
fi

# ---------------------------------------------------------------- C: run behavior
if inS C; then section "C. run behavior: mounts, user, env, config, exit codes (R-steps, existing features)"
  [ "$(ncaged)" = 0 ] && $C1 build >/dev/null 2>&1
  H=$HOME; mkdir -p $H/.claude $H/.codex/app-server-control $H/.codex/tmp/arg0 $H/.gemini $H/.config/gh $S/outside
  echo '{"a":1}' > $H/.claude/settings.json; echo 'model="x"' > $H/.codex/config.toml; echo '[user]' > $H/.gitconfig; echo '{}' > $H/.gemini/settings.json
  echo 'hosts' > $H/.config/gh/hosts.yml; ln -sfn $S/outside $H/.pi 2>/dev/null; echo secret > $S/outside/x
  tty 'pwd; ls; id -u; exit' $C1 --no-config; has "C1 project mounted at /workspace/<name>" "$OUT" "/workspace/proj"; has "C1 project files visible" "$OUT" "file.txt"; has "C1 runs as the host uid ($UIDN)" "$OUT" "$UIDN"
  tty 'echo made-in-container > /workspace/proj/new.txt; exit' $C1 --no-config; eq "C2 file written in the container lands on the host" "$(cat new.txt 2>/dev/null)" made-in-container
  eq "C2 owned by the host user" "$(stat -c %u new.txt)" "$UIDN"; rm -f new.txt
  tty 'ls -A /workspace; exit' $C1 --no-config --no-mount; hasnt "C3 --no-mount: project absent" "$OUT" "proj"
  echo other > $S/other/o.txt; tty 'pwd; ls; exit' $C1 --no-config -w $S/other; has "C4 -w mounts another directory" "$OUT" "/workspace/other"; has "C4 contents" "$OUT" "o.txt"
  tty 'pwd; exit' $C1 --no-config -w ../other; has "C4 relative --workdir" "$OUT" "/workspace/other"
  plain $C1 --no-config -w $HOME; has "C5 home directory refused" "$OUT" "allow-broad-mount"; plain $C1 --no-config -w /; has "C5 / refused" "$OUT" "allow-broad-mount"
  tty 'pwd; exit' $C1 --no-config --allow-broad-mount -w $HOME; has "C5b --allow-broad-mount mounts the home directory" "$OUT" "/workspace/home"
  tty 'ls -A ~/.claude ~/.codex ~/.gemini; cat ~/.gitconfig; cat ~/.config/gh/hosts.yml; exit' $C1 --verbose; has "C6 claude config mounted" "$OUT" "settings.json"; has "C6 codex config mounted" "$OUT" "config.toml"; has "C6 gitconfig mounted" "$OUT" "[user]"; has "C6 gh config mounted" "$OUT" "hosts"
  has "C6 WARNING shown when config is mounted" "$OUT" "WARNING: allowlisted agent credentials"; has "C6 verbose lists mounts" "$OUT" "config: pass $H/.claude -> /home/caged/.claude"; has "C6 unsafe symlink refused" "$OUT" "config: refused $H/.pi"
  has "C6 missing config reported" "$OUT" "config: missing $H/.copilot"
  tty 'echo changed > ~/.claude/settings.json; echo new > ~/.claude/created.txt; exit' $C1; eq "C7 agent config is writable: edit persists" "$(cat $H/.claude/settings.json)" changed; eq "C7 new file persists" "$(cat $H/.claude/created.txt)" new; rm -f $H/.claude/created.txt; echo '{"a":1}' > $H/.claude/settings.json
  tty 'echo x >> ~/.gitconfig; echo rc=$?; exit' $C1; has "C8 .gitconfig is read-only" "$OUT" "Read-only file system"; eq "C8 host .gitconfig unchanged" "$(cat $H/.gitconfig)" "[user]"
  tty 'ls -A ~/.claude ~/.codex 2>&1 | head -3; exit' $C1 --no-config; hasnt "C9 --no-config mounts nothing" "$OUT" "settings.json"; hasnt "C9 and shows no warning" "$OUT" "WARNING"
  mkdir -p $H/alt-codex && echo alt > $H/alt-codex/config.toml; tty 'cat ~/.codex/config.toml; exit' env CODEX_HOME=$H/alt-codex $C1; has "C10 CODEX_HOME override (inside home) is mounted" "$OUT" "alt"
  mkdir -p $S/alt-outside && echo out > $S/alt-outside/config.toml; tty 'cat ~/.codex/config.toml 2>&1; exit' env CODEX_HOME=$S/alt-outside $C1 --verbose; has "C10b CODEX_HOME outside the home directory is refused" "$OUT" "config: refused $S/alt-outside (resolves outside home)"
  mkfifo $H/.codex/app-server-control/daemon.sock 2>/dev/null; tty 'ls -A ~/.codex/app-server-control | wc -l; ls ~/.codex/config.toml; exit' $C1; has "C11 codex's daemon socket directory is masked (empty inside)" "$OUT" "0"; has "C11 the rest of ~/.codex is there" "$OUT" "/home/caged/.codex/config.toml"; hasnt "C11 the socket itself is hidden" "$OUT" "daemon.sock"
  tty 'echo V=$CAGED_E2E_VAL; exit' env CAGED_E2E_VAL=s3cr3t-value $C1 --no-config --env CAGED_E2E_VAL --verbose; has "C12 --env passes the variable into the container" "$OUT" "V=s3cr3t-value"; has "C12 verbose names it" "$OUT" "env: pass CAGED_E2E_VAL"
  hasnt "C12 caged itself never prints the value" "$(echo "$OUT" | grep -a '^env:\|^config:\|^image:\|^command:\|^port:\|^WARNING')" "s3cr3t-value"
  tty 'echo V=$CAGED_E2E_UNSET; exit' $C1 --no-config --env CAGED_E2E_UNSET --verbose; has "C13 an unset variable is skipped" "$OUT" "env: skipped CAGED_E2E_UNSET"
  tty 'exit 7' $C1 --no-config; eq "C14 container exit code passes through" "$RC" 7
  tty 'exit' $C1 --no-config claude -- --exit-code 3; has "C15 agent exit announced" "$OUT" "Agent exited (status 3)"; eq "C15 agent exit code passes through" "$RC" 3
  tty 'echo STILL-IN-CONTAINER; exit' $C1 --no-config claude 'hello'; has "C16 shell opens after the agent exits" "$OUT" "STILL-IN-CONTAINER"
  plain $C1 --no-config --verbose --no-mount --env 'bad name' ; has "C17 invalid --env rejected" "$OUT" "invalid env name"
  plain $C1 --no-config notanagent; has "C18 unsupported agent" "$OUT" 'unsupported agent "notanagent"'
  tty 'exit' $C1 --no-config Claude; eq "C18 agent names are case-insensitive" "$RC" 0; has "C18 ...and run the agent" "$OUT" "Agent exited (status 0)"
  rm -f $H/.pi
fi

# ---------------------------------------------------------------- D: ports
if inS D; then section "D. publishing ports (F1)"
  [ "$(ncaged)" = 0 ] && $C1 build >/dev/null 2>&1
  LAN=$(hostname -I | awk '{print $1}')
  ( printf 'cd /workspace/proj; python3 -m http.server 8080 --bind 0.0.0.0 >/dev/null 2>&1 &\nsleep 12\nexit\n' ) | script -qec "$C1 --no-config -p 38080:8080 -p 38090 --verbose" /dev/null > $S/ports.out 2>&1 &
  sleep 6
  PORTS=$(docker ps --format '{{.Ports}}' | head -2); has "D1 docker maps 127.0.0.1:38080->8080" "$PORTS" "127.0.0.1:38080->8080/tcp"; has "D1 and 127.0.0.1:38090->38090" "$PORTS" "127.0.0.1:38090->38090/tcp"
  eq "D2 host reaches the container's server on 127.0.0.1" "$(curl -s -m 4 http://127.0.0.1:38080/file.txt)" "project file"
  curl -s -m 4 -o /dev/null http://$LAN:38080/file.txt; eq "D3 the LAN address is refused" "$?" 7
  wait; has "D4 verbose lists the ports" "$(tr -d '\r' < $S/ports.out)" "port: publish 127.0.0.1:38080 -> container 8080"
  python3 -m http.server 38081 --bind 127.0.0.1 >/dev/null 2>&1 & LP=$!; sleep 1
  tty 'exit' $C1 --no-config -p 38081:8080; has "D5 port in use: caged's message" "$OUT" "host port 38081 is already in use on 127.0.0.1; stop the process using it or pick another host port, e.g. -p 38082:8080"; eq "D5 exit code 125" "$RC" 125
  kill $LP 2>/dev/null
  for bad in "70000|out of range 1-65535" "0|out of range" "abc|use PORT or HOST_PORT" "3000-3005|port ranges are not supported" "0.0.0.0:80:80|a host address is not supported" "3000/udp|protocols are not supported" "8080:3000/tcp|protocols are not supported"; do
    plain env PATH="$S/shim:$PATH" $C1 --no-config -p "${bad%%|*}"; has "D6 -p ${bad%%|*}" "$OUT" "${bad#*|}"; done
  plain env PATH="$S/shim:$PATH" $C1 --no-config -p 3000 -p 3000:4000; has "D7 duplicate host port" "$OUT" "host port 3000 is published more than once"
  plain env PATH="$S/shim:$PATH" $C1 --no-config -p ""; has "D8 empty port" "$OUT" 'invalid port ""'
fi

# ---------------------------------------------------------------- E: extensions
if inS E; then section "E. image extensions with REAL apt installs (F3)"
  $C1 clean >/dev/null 2>&1; $C1 build >/dev/null 2>&1; mkdir -p $XDG_CONFIG_HOME/caged; EXT=$XDG_CONFIG_HOME/caged/config.yaml
  printf 'image:\n  apt: [%s]\n' 'tmux, neovim' > $EXT
  tty 'nvim --version | head -1; tmux -V; id -u; exit' $C1 --no-config --verbose; has "E1 notice names the file and sorted packages" "$OUT" "Building image extension from $EXT (apt: neovim, tmux)..."
  has "E1 neovim installed and runs in the container" "$OUT" "NVIM v"; has "E1 tmux installed and runs" "$OUT" "tmux 3"; eq "E1 exit 0" "$RC" 0
  eq "E2 ext role label" "$(label caged-runtime:ext role)" extension; eq "E2 base-id is the base image id" "$(label caged-runtime:ext base-id)" "$(imgid caged-runtime:latest)"
  eq "E2 two images" "$(ncaged)" 2
  tty 'exit' $C1 --no-config --verbose; hasnt "E3 second run builds nothing" "$OUT" "Building"; has "E3 verbose describes it" "$OUT" "image: extension from $EXT (apt: neovim, tmux) -> caged-runtime:ext"
  printf 'image:\n  apt: [%s]\n' 'neovim, tmux, tmux, neovim' > $EXT; tty 'exit' $C1 --no-config; hasnt "E4 reorder and duplicates: no rebuild" "$OUT" "Building"
  BASE=$(imgid caged-runtime:latest); OLDEXT=$(imgid caged-runtime:ext); printf 'image:\n  apt: [%s]\n' 'neovim, tmux, jq' > $EXT
  tty 'jq --version; exit' $C1 --no-config; has "E5 added package: extension rebuilt" "$OUT" "(apt: jq, neovim, tmux)"; has "E5 jq works" "$OUT" "jq-"
  eq "E5 base untouched" "$(imgid caged-runtime:latest)" "$BASE"; docker image inspect "$OLDEXT" >/dev/null 2>&1 && bad "E5 old extension removed" "still there" || ok "E5 old extension image removed"
  eq "E5 still two images" "$(ncaged)" 2
  OLDEXT=$(imgid caged-runtime:ext); OLDBASE=$(imgid caged-runtime:latest)
  tty 'exit' $C2 --no-config --verbose; has "E6 base rebuilt with an extension" "$OUT" "rebuilding it for caged r2"; has "E6 extension rebuilt on the new base" "$OUT" "Building image extension"
  eq "E6 ext base-id is the NEW base" "$(label caged-runtime:ext base-id)" "$(imgid caged-runtime:latest)"
  hasnt "E6 nothing had to be kept (ext was removed before its base)" "$OUT" "kept old image"; docker image inspect "$OLDBASE" >/dev/null 2>&1 && bad "E6 old base removed" "still there" || ok "E6 old base image removed"
  docker image inspect "$OLDEXT" >/dev/null 2>&1 && bad "E6 old ext removed" "still there" || ok "E6 old extension image removed"; eq "E6 exactly two images" "$(ncaged)" 2
  plain $C2 build; has "E7 caged build: runtime line" "$OUT" "built runtime image caged-runtime:latest"; has "E7 caged build: extension line" "$OUT" "built extension image caged-runtime:ext (apt: jq, neovim, tmux)"
  plain $C2 build --no-cache; has "E7b --no-cache builds both" "$OUT" "built extension image"
  printf 'image:\n  apt: [%s]\n' 'notarealpackage-xyz' > $EXT; KEEPEXT=$(imgid caged-runtime:ext); rm -f $S/marker
  tty "touch $S/marker; exit" $C2 --no-config; has "E8 a package apt cannot find: file named, advice given" "$OUT" "building the image extension from $EXT failed; fix or remove that file"; eq "E8 exit 1" "$RC" 1
  eq "E8 the previous extension image is kept" "$(imgid caged-runtime:ext)" "$KEEPEXT"; [ ! -e $S/marker ] && ok "E8 nothing ran" || bad "E8 nothing ran" marker
  rm -f "$S/shim.log"
  for c in 'image:\n  apt: ["vim; curl x"]\n|invalid apt package name "vim; curl x" in image.apt' 'npm: [x]\n|unknown field "npm"' 'image: [\n|invalid YAML' 'image:\n  apt: ["vim=2:9"]\n|invalid apt package name "vim=2:9"' 'image:\n  apt: [Vim]\n|invalid apt package name "Vim"' 'version: 2\n|unsupported version 2' 'image: [x]\n|wrong type' 'version: 1\n---\nversion: 1\n|only one YAML document'; do
    printf '%b' "${c%%|*}" > $EXT; plain env PATH="$S/shim:$PATH" $C2 --no-config; has "E9 invalid file: ${c%%|*}" "$OUT" "${c#*|}"; eq "E9 exit 1" "$RC" 1; done
  [ -e "$S/shim.log" ] && bad "E9 invalid files make no docker call" "$(cat $S/shim.log)" || ok "E9 invalid files make no docker call"
  printf 'image:\n  apt: [jq]\n' > $EXT; chmod 000 $EXT; plain env PATH="$S/shim:$PATH" $C2 --no-config; chmod 644 $EXT; has "E10 unreadable file" "$OUT" "read $EXT: "
  rm -f $EXT; XD=$XDG_CONFIG_HOME; unset XDG_CONFIG_HOME; mkdir -p $HOME/.config/caged; printf 'image:\n  apt: [jq]\n' > $HOME/.config/caged/config.yaml
  tty 'exit' $C2 --no-config --verbose; has "E11 default location ~/.config/caged/config.yaml is used, shown with ~" "$OUT" "(apt: jq)"; has "E11 path shown with ~" "$OUT" "~/.config/caged/config.yaml"
  export XDG_CONFIG_HOME=relative/dir; tty 'exit' $C2 --no-config --verbose; has "E12 a relative XDG_CONFIG_HOME is ignored" "$OUT" "~/.config/caged/config.yaml"; export XDG_CONFIG_HOME=$XD
  rm -f $HOME/.config/caged/config.yaml; tty 'exit' $C2 --no-config --verbose; has "E13 removing the file: runs use the runtime image" "$OUT" "image: no image.apt packages in"
  plain $C2 doctor; has "E14 doctor flags the leftover extension image" "$OUT" "caged-runtime:ext is no longer used"; plain $C2 clean; has "E14 clean removes the leftover" "$OUT" "removed caged-runtime:ext"
  $C2 clean >/dev/null 2>&1
fi

# ---------------------------------------------------------------- F: passthrough
if inS F; then section "F. passing flags to the agent with -- (F4)"
  $C1 clean >/dev/null 2>&1; $C1 build >/dev/null 2>&1
  args() { echo "$OUT" | grep -a '^  \[' | tr -d '\n' | sed 's/\]  \[/|/g;s/^  \[//;s/\]$//'; }
  tty 'exit' $C1 --no-config claude 'fix tests' -- --model opus --add-dir ../lib; eq "F1 claude: prompt then flags" "$(args)" "fix tests|--model|opus|--add-dir|../lib"
  tty 'exit' $C1 --no-config claude -- --resume; eq "F2 claude: flags only" "$(args)" "--resume"
  tty 'exit' $C1 --no-config codex 'x' -- --model o4; eq "F3 codex: wrapper adds --no-daemon, prompt uses exec" "$(args)" "--no-daemon|exec|x|--model|o4"
  tty 'exit' $C1 --no-config codex -- --model o4; eq "F3b codex without a prompt" "$(args)" "--no-daemon|--model|o4"
  tty 'exit' $C1 --no-config gemini 'explain' -- --model g; eq "F4 gemini: -i prompt then flags" "$(args)" "-i|explain|--model|g"
  tty 'exit' $C1 --no-config copilot 'review' -- --model m; eq "F5 copilot: -p prompt then flags" "$(args)" "-p|review|--model|m"
  tty 'exit' $C1 --no-config pi 'hi' -- --provider openai; eq "F6 pi: prompt then flags" "$(args)" "hi|--provider|openai"
  tty 'exit' $C1 --no-config gh -- --version; has "F7 gh (the real gh): --version" "$OUT" "gh version 2."
  tty 'exit' $C1 --no-config gh -- pr --help; has "F7b gh passthrough runs the real gh" "$OUT" "Work with GitHub pull requests"
  plain $C1 --no-config gh 'x'; has "F8 gh with a prompt is still an error" "$OUT" "gh does not support prompt launch"
  tty 'exit' $C1 --no-config pi -- '$(id)' '"q"' "it's" ''; eq "F9 shell syntax and empty args stay literal" "$(args)" '$(id)|"q"|it'"'"'s|'
  tty 'exit' $C1 --no-config claude -- --verbose -p 3000 --no-mount -w /x; eq "F10 caged's flags after -- belong to the agent" "$(args)" "--verbose|-p|3000|--no-mount|-w|/x"
  tty 'ls /workspace; exit' $C1 --no-config claude -- --no-mount; has "F10b ...and the project is still mounted" "$OUT" "proj"
  tty 'exit' $C1 --no-config claude -- a -- b; eq "F11 a second -- is just an argument" "$(args)" "a|--|b"
  tty 'exit' $C1 --no-config claude --; eq "F12 'claude --' is the same as 'claude'" "$(args)" ""; eq "F12 exit 0" "$RC" 0
  tty 'echo SHELL-OK; exit' $C1 --no-config --; has "F13 'caged --' opens the shell" "$OUT" "SHELL-OK"
  plain $C1 -- --resume; has "F14 no agent before --" "$OUT" "arguments after -- are passed to an agent; name one first, e.g. caged claude -- --resume"
  plain $C1 -- build; has "F15 '-- build' is not the build command" "$OUT" "name one first"
  plain $C1 claude a b -- x; has "F16 too many arguments before --" "$OUT" "accepts at most 2 arg(s), received 3"
  plain $C1 claude --resume; has "F17 unknown flag gets the hint" "$OUT" "pass agent flags after --, e.g. caged claude -- --resume"; plain $C1 -x claude; has "F17b short flag too" "$OUT" "unknown shorthand flag: 'x' in -x ("
  tty 'exit' $C1 --no-config --verbose claude 'fix tests' -- --resume; has "F18 verbose prints the final command" "$OUT" 'command: ["claude" "fix tests" "--resume"]'
  tty 'exit' $C1 --no-config --verbose; has "F18b verbose prints the shell command too" "$OUT" 'command: ["bash"]'
fi

# ---------------------------------------------------------------- G: doctor, list, clean
if inS G; then section "G. doctor, list, clean (F5)"
  $C1 clean >/dev/null 2>&1; $C1 build >/dev/null 2>&1; mkdir -p $XDG_CONFIG_HOME/caged; EXT=$XDG_CONFIG_HOME/caged/config.yaml; rm -f $EXT
  plain $C1 doctor; eq "G1 doctor: exit 0 when healthy" "$RC" 0
  has "G1 header" "$OUT" "caged r1 (linux/amd64)"; has "G1 docker row" "$OUT" "PASS  docker     $(command -v docker)"; has "G1 daemon row" "$OUT" "PASS  daemon     "; has "G1 image row" "$OUT" "PASS  image      caged-runtime:latest is current (runtime $(label caged-runtime:latest runtime-hash | cut -c1-12), built by caged r1)"
  has "G1 config row" "$OUT" "PASS  config     no config file ($EXT)"; has "G1 links row" "$OUT" "PASS  links      no symlinked config"; has "G1 arch row" "$OUT" "PASS  arch       image linux/amd64 matches the Docker VM (amd64)"
  has "G1 project row (real mount probe)" "$OUT" "PASS  project    $S/proj can be mounted"; has "G1 versions row (read from the image)" "$OUT" "PASS  versions   claude 2.1.3 (Claude Code), codex codex-cli 0.46.0, copilot GitHub Copilot CLI 0.0.330, gemini 0.9.0, gh gh version 2.45.0"
  hasnt "G1 no FAIL or WARN" "$OUT" "FAIL"; hasnt "G1 no WARN" "$OUT" "WARN"
  plain $C1 doctor -w $S/other; has "G2 -w probes another directory" "$OUT" "PASS  project    $S/other can be mounted"
  plain $C1 doctor -w /nonexistent; has "G3 -w nonexistent: project FAIL" "$OUT" "FAIL  project    inspect working directory \"/nonexistent\""; eq "G3 exit 1" "$RC" 1; has "G3 summary on stderr" "$OUT" "doctor: 1 checks failed"
  plain $C1 doctor -w $HOME; has "G4 home directory: WARN, not probed" "$OUT" "WARN  project"; eq "G4 exit 0 (warn only)" "$RC" 0
  plain env DOCKER_HOST=unix:///nonexistent $C1 doctor; has "G5 daemon down: FAIL" "$OUT" "FAIL  daemon"; has "G5 dependants skipped" "$OUT" "SKIP  image      needs daemon"; has "G5 config check still runs" "$OUT" "PASS  config"; eq "G5 exit 1" "$RC" 1
  plain env PATH=/nonexistent $C1 doctor; has "G6 docker missing" "$OUT" "FAIL  docker     Docker is not installed"; has "G6 daemon skipped" "$OUT" "SKIP  daemon     needs docker"; eq "G6 exit 1" "$RC" 1
  printf 'image:\n  apt: [jq]\n' > $EXT; plain $C1 doctor; has "G7 extension configured, not built" "$OUT" "WARN  ext-image  caged-runtime:ext is not built yet; the next run builds it"; eq "G7 exit 0" "$RC" 0
  tty 'exit' $C1 --no-config; plain $C1 doctor; has "G8 extension built" "$OUT" "PASS  ext-image  caged-runtime:ext is current (apt: jq)"; has "G8 extension row" "$OUT" "(apt: jq)"
  printf 'image:\n  apt: [jq, tmux]\n' > $EXT; plain $C1 doctor; has "G9 extension edited: stale" "$OUT" "WARN  ext-image  caged-runtime:ext is out of date; the next run rebuilds it"
  printf 'nope: 1\n' > $EXT; plain $C1 doctor; has "G10 invalid file: FAIL" "$OUT" "FAIL  config"; has "G10 ext-image skipped" "$OUT" "SKIP  ext-image  needs config"; eq "G10 exit 1" "$RC" 1; rm -f $EXT; $C1 clean >/dev/null 2>&1
  plain $C1 doctor; eq "G11 no images: exit 0" "$RC" 0; has "G11 image WARN" "$OUT" "WARN  image      caged-runtime:latest is not built yet; the next run builds it, or run caged build"; has "G11 arch skipped" "$OUT" "SKIP  arch       no image built yet"; has "G11 project skipped" "$OUT" "SKIP  project    no image built yet to probe with"
  docker import --platform linux/arm64 $S/empty.tar caged-runtime:latest >/dev/null 2>&1; plain $C1 doctor; has "G12 wrong-architecture image: FAIL" "$OUT" "FAIL  arch       image linux/arm64 does not match Docker (amd64); it runs under emulation; run caged build --no-cache"; eq "G12 exit 1" "$RC" 1
  $C1 clean >/dev/null 2>&1; $C1 build >/dev/null 2>&1
  H=$HOME; mkdir -p $H/.claude $H/.config/gh; echo x > $H/.claude/s.json
  plain env ANTHROPIC_API_KEY=very-secret-123 OPENAI_API_KEY= $C1 list; has "G13 list header" "$OUT" "AGENT"; has "G13 claude row" "$OUT" "claude   ~/.claude"; has "G13 claude env name" "$OUT" "ANTHROPIC_API_KEY"; hasnt "G13 the secret value never appears" "$OUT" "very-secret-123"
  has "G13 an agent without config says not found" "$OUT" "not found"; has "G13 set-but-empty variable counts as set" "$OUT" "OPENAI_API_KEY"
  mkdir -p $HOME/alt-codex; echo a > $HOME/alt-codex/config.toml; plain env CODEX_HOME=$HOME/alt-codex $C1 list; has "G14 CODEX_HOME override shown, with ~" "$OUT" "codex    ~/alt-codex"; rm -rf $HOME/alt-codex
  plain $C1 list extra; eq "G15 list takes no arguments" "$RC" 1
  OTHERS_NOW=$(others)
  docker tag caged-runtime:latest caged-runtime:second; plain $C1 clean --dry-run; has "G16 dry run lists both tags" "$OUT" "would remove caged-runtime:latest"; has "G16 and the second" "$OUT" "would remove caged-runtime:second"; eq "G16 dry run removed nothing" "$(docker images 'caged-runtime*' -q | wc -l | tr -d ' ')" 2
  docker run -d --name caged-e2e-hold --entrypoint sleep caged-runtime:latest 300 >/dev/null; plain $C1 clean; has "G17 image used by a container is kept" "$OUT" "kept caged-runtime:"; has "G17 reason given" "$OUT" "in use by a container (remove it with docker rm first)"; eq "G17 exit 0" "$RC" 0
  docker rm -f caged-e2e-hold >/dev/null; plain $C1 clean; has "G18 after the container is gone: removed" "$OUT" "removed caged-runtime:"; eq "G18 no caged images" "$(ncaged)" 0
  plain $C1 clean; eq "G19 nothing to clean" "$OUT" "nothing to clean"; eq "G19 exit 0" "$RC" 0; eq "G20 other images untouched" "$(others)" "$OTHERS_NOW"
  docker import $S/empty.tar caged-runtime:old >/dev/null; plain $C1 clean; has "G21 older unlabeled caged-runtime images are cleaned" "$OUT" "removed caged-runtime:old"
  mkdir -p $XDG_CONFIG_HOME/caged; printf '{"apt":["jq"]}' > $XDG_CONFIG_HOME/caged/image.json; plain $C1 doctor
  has "G22 a leftover image.json is a warning" "$OUT" "WARN  config"; has "G22 and says it is no longer read" "$OUT" "image.json is no longer read: move its packages to image.apt in $EXT"; eq "G22 exit 0 (warn only)" "$RC" 0
  rm -f $XDG_CONFIG_HOME/caged/image.json
fi

# ---------------------------------------------------------------- H: version and packaging
if inS H; then section "H. version resolution and packaging (F6)"
  plain $S/bin/nover --version; eq "H1 no version linked: dev" "$OUT" "caged version dev"
  plain $S/bin/vtag --version; eq "H2 linked v1.2.3: leading v stripped" "$OUT" "caged version 1.2.3"
  plain $C1 --version; eq "H3 linked r1" "$OUT" "caged version r1"
  plain $C1 doctor -w $S/other; has "H4 doctor header carries the version" "$OUT" "caged r1 (linux/"
  $C1 clean >/dev/null 2>&1; $C1 build >/dev/null 2>&1; eq "H5 image version label is the resolved version" "$(label caged-runtime:latest version)" r1
  $S/bin/vtag clean >/dev/null 2>&1; $S/bin/vtag build >/dev/null 2>&1; eq "H5b v-prefixed link is stripped in the label" "$(label caged-runtime:latest version)" "1.2.3"
  (cd "$REPO" && git ls-files --error-unmatch LICENSE CHANGELOG.md >/dev/null 2>&1; true)
  [ -s "$REPO/LICENSE" ] && head -1 "$REPO/LICENSE" | grep -q "MIT License" && ok "H6 LICENSE is MIT" || bad "H6 LICENSE is MIT" ""
  grep -q "Copyright (c) 2026 Jeremy Orr" "$REPO/LICENSE" && ok "H6 copyright line" || bad "H6 copyright line" ""
  awk -v v=0.1.0 '/^## \[/ { if (found) exit; found = index($0, "[" v "]") > 0; next } found { print }' "$REPO/CHANGELOG.md" | grep -q "caged doctor" && ok "H7 release notes extract from CHANGELOG" || bad "H7 release notes extract" ""
  $C1 clean >/dev/null 2>&1; $S/bin/vtag clean >/dev/null 2>&1
fi

# ---------------------------------------------------------------- L: symlinked config (Stow layout)
if inS L; then section "L. symlinked agent config and links.allow"
  [ "$(ncaged)" = 0 ] && $C1 build >/dev/null 2>&1
  H=$HOME; CFG=$XDG_CONFIG_HOME/caged/config.yaml; mkdir -p $H/dotfiles/claude $H/.claude $H/.ssh $XDG_CONFIG_HOME/caged
  echo hi > $H/dotfiles/claude/CLAUDE.md
  ln -s ../dotfiles/claude/CLAUDE.md $H/.claude/CLAUDE.md
  printf 'links:\n  allow:\n    - path: ~/dotfiles\n' > $CFG
  tty 'echo L1:$(cat ~/.claude/CLAUDE.md); exit' $C1; has "L1 a symlinked config file reads through the link" "$OUT" "L1:hi"
  tty 'echo appended >> ~/.claude/CLAUDE.md; exit' $C1; has "L2 a write in the container reaches the dotfiles file" "$(cat $H/dotfiles/claude/CLAUDE.md)" "appended"
  printf 'links:\n  allow:\n    - path: ~/dotfiles\n      read_only: true\n' > $CFG
  tty 'echo ro-write-attempt >> ~/.claude/CLAUDE.md; exit' $C1; has "L3 read_only: the write fails" "$OUT" "Read-only file system"; hasnt "L3 the dotfiles file is unchanged" "$(cat $H/dotfiles/claude/CLAUDE.md)" "ro-write-attempt"
  printf 'links:\n  allow:\n    - path: ~/dotfiles\n' > $CFG
  echo SECRET > $H/.ssh/id_e2e; ln -s ../.ssh/id_e2e $H/.claude/key
  tty 'cat ~/.claude/key 2>&1; exit' $C1 --verbose; has "L4 a link into ~/.ssh is refused by the built-in deny list" "$OUT" "built-in deny list"; hasnt "L4 the secret is not readable in the container" "$OUT" "SECRET"
  # L5 a linked directory, read and written through
  mkdir -p $H/dotfiles/claude/agents; echo agent-a > $H/dotfiles/claude/agents/a.md; ln -s ../dotfiles/claude/agents $H/.claude/agents
  tty 'cat ~/.claude/agents/a.md; echo more >> ~/.claude/agents/a.md; exit' $C1 --verbose
  has "L5 a linked directory reads through the link" "$OUT" "agent-a"; has "L5 verbose reports the link" "$OUT" "(via link $H/.claude/agents)"; has "L5 a write through the linked directory reaches the dotfiles" "$(cat $H/dotfiles/claude/agents/a.md)" "more"
  # L6 a config root that is itself a link (Stow's ~/.pi)
  rm -rf $H/.pi; mkdir -p $H/dotfiles/pi/.pi/agent; echo '{"stow":1}' > $H/dotfiles/pi/.pi/agent/settings-target.json; ln -s settings-target.json $H/dotfiles/pi/.pi/agent/settings.json; ln -s dotfiles/pi/.pi $H/.pi
  tty 'cat ~/.pi/agent/settings.json; exit' $C1 --verbose; has "L6 a symlinked config root is mounted" "$OUT" '{"stow":1}'; has "L6 and reported as a link" "$OUT" "(via link $H/.pi)"
  mkdir -p $H/dotfiles/pi/.pi/agent/extensions-target; ln -s extensions-target $H/dotfiles/pi/.pi/agent/extensions-link
  tty 'printf "{}\n" > ~/.pi/agent/keybindings.json; exit' $C1
  has "L6b a newly created Pi config file reaches the host" "$(cat $H/dotfiles/pi/.pi/agent/keybindings.json)" '{}'
  # L6c symlinks added after an earlier run are resolved again on the next run.
  ln -s settings-target.json $H/dotfiles/pi/.pi/agent/future-settings.json
  ln -s extensions-target $H/dotfiles/pi/.pi/agent/future-extensions
  tty 'printf "updated\n" > ~/.pi/agent/future-settings.json; printf "new extension\n" > ~/.pi/agent/future-extensions/new.js; exit' $C1
  eq "L6c a newly added file link writes through to its target" "$(cat $H/dotfiles/pi/.pi/agent/settings-target.json)" updated
  eq "L6c a newly added directory link persists new files" "$(cat $H/dotfiles/pi/.pi/agent/extensions-target/new.js)" "new extension"
  # L7 a link inside a linked directory, to a root that is not allowed, then allowed
  mkdir -p $H/dotfiles2; echo shared-note > $H/dotfiles2/shared.md; ln -s ../../../dotfiles2/shared.md $H/dotfiles/claude/agents/shared.md
  tty 'cat ~/.claude/agents/shared.md 2>&1; exit' $C1 --verbose
  has "L7 a nested link outside links.allow is refused with the fix" "$OUT" "add \"- path: ~/dotfiles2\" under links.allow in $CFG"; hasnt "L7 and its target is not readable" "$OUT" "shared-note"
  printf 'links:\n  allow:\n    - path: ~/dotfiles\n    - path: ~/dotfiles2\n' > $CFG
  tty 'cat ~/.claude/agents/shared.md; exit' $C1; has "L7 after allowing the root, the nested link works" "$OUT" "shared-note"
  # L8 Codex is mounted whole, so its link resolves inside the container
  mkdir -p $H/dotfiles/codex $H/.codex; echo codex-rules > $H/dotfiles/codex/AGENTS.md; ln -s ../dotfiles/codex/AGENTS.md $H/.codex/AGENTS.md
  tty 'cat ~/.codex/AGENTS.md; exit' $C1 --verbose; has "L8 a Codex link resolves inside the container" "$OUT" "codex-rules"; has "L8 the target is mounted where the link points" "$OUT" "-> /home/caged/dotfiles/codex/AGENTS.md (via link"
  # L9 refused links show on the doctor row, and list keeps working
  printf 'links:\n  allow: []\n' > $CFG
  plain $C1 doctor; has "L9 doctor warns about refused links" "$OUT" "WARN  links"; has "L9 with the fix" "$OUT" "under links.allow in $CFG"; eq "L9 exit 0 (warn only)" "$RC" 0
  plain $C1 list; eq "L9 list works" "$RC" 0; has "L9 list shows the agents" "$OUT" "claude"
  # L10 read_only on a linked directory
  printf 'links:\n  allow:\n    - path: ~/dotfiles\n      read_only: true\n' > $CFG
  tty 'echo ro-dir-write >> ~/.claude/agents/a.md; exit' $C1; has "L10 read_only applies to a linked directory" "$OUT" "Read-only file system"; hasnt "L10 and the dotfiles file is unchanged" "$(cat $H/dotfiles/claude/agents/a.md)" "ro-dir-write"
  rm -f $H/.claude/CLAUDE.md $H/.claude/key $H/.claude/agents $H/.pi $H/.codex/AGENTS.md $H/.ssh/id_e2e $CFG; rm -rf $H/dotfiles $H/dotfiles2; rmdir $H/.ssh 2>/dev/null
fi

# ---------------------------------------------------------------- W: workdirs.allow
if inS W; then section "W. workdirs.allow limits the project directory"
  [ "$(ncaged)" = 0 ] && $C1 build >/dev/null 2>&1
  CFG=$XDG_CONFIG_HOME/caged/config.yaml; mkdir -p $XDG_CONFIG_HOME/caged $S/other; rm -f "$S/shim.log"
  printf 'workdirs:\n  allow:\n    - path: %s\n' "$S/proj" > $CFG
  tty 'pwd; exit' $C1 --no-config; has "W1 a directory under workdirs.allow is mounted" "$OUT" "/workspace/proj"
  plain env PATH="$S/shim:$PATH" $C1 --no-config -w $S/other; eq "W2 a directory outside it is refused" "$RC" 1
  has "W2 the message names the key and the fix" "$OUT" "refusing to mount $S/other: it is not under workdirs.allow; add \"- path: $S/other\" under workdirs.allow in $CFG"
  plain env PATH="$S/shim:$PATH" $C1 --no-config --allow-broad-mount -w $S/other; has "W3 --allow-broad-mount does not bypass it" "$OUT" "not under workdirs.allow"
  [ -e "$S/shim.log" ] && bad "W4 a refused directory makes no docker call" "$(cat $S/shim.log)" || ok "W4 a refused directory makes no docker call"
  plain $C1 doctor -w $S/other; has "W5 doctor reports it on the project row" "$OUT" "FAIL  project    refusing to mount $S/other"; eq "W5 exit 1" "$RC" 1
  printf 'workdirs:\n  allow: []\n' > $CFG; plain env PATH="$S/shim:$PATH" $C1 --no-config; eq "W6 an empty list refuses every directory" "$RC" 1
  printf 'image:\n  apt: []\n' > $CFG; tty 'pwd; exit' $C1 --no-config -w $S/other; has "W7 without a workdirs key any directory works" "$OUT" "/workspace/other"
  rm -f $CFG
fi

# ---------------------------------------------------------------- I: leftovers
if inS I; then section "I. leftovers"
  $C1 clean >/dev/null 2>&1
  eq "I1 no caged images remain" "$(ncaged)" 0
  eq "I2 no caged-e2e containers remain" "$(docker ps -a --format '{{.Names}}' | grep -c caged-e2e)" 0
  eq "I3 no leftover caged build directories in /tmp" "$(ls /tmp | grep -c '^caged-\(runtime\|extension\)-')" 0
  eq "I4 other images untouched" "$(others)" "$OTHERS_BEFORE"
  [ ! -e "$REALHOME/.config/caged" ] && ok "I5 the real ~/.config/caged was never created" || bad "I5 real ~/.config/caged" "exists"
fi

printf '\n==== %d passed, %d failed\n' "$pass" "$fail"; for f in "${failed[@]:-}"; do [ -n "$f" ] && printf '  - %s\n' "$f"; done
[ "$fail" = 0 ]
