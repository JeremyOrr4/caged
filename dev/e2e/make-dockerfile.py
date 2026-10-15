#!/usr/bin/env python3
"""Build a realistic stand-in for caged's runtime Dockerfile that needs no HTTPS downloads.

usage: make-dockerfile.py <real Dockerfile> <output Dockerfile> [extra-comment]

It keeps the real final stage (user setup, entrypoint, versions loop, pinned Ubuntu digest, apt packages) and
replaces only what downloads over HTTPS: Node and gh come from Ubuntu's apt archive, and the six agent CLIs are
fake scripts laid out where the real installers put them. Fake CLIs: --version prints a version line (anywhere in
the arguments), --exit-code N exits with N, anything else prints its arguments one per line.
"""
import re, sys

real = open(sys.argv[1]).read()
pin = re.search(r'^FROM (ubuntu:24.04@sha256:[0-9a-f]{64}) AS caged-build', real, re.M).group(1)
final = real[real.index('\nFROM ubuntu:24.04@') + 1:]

SH = '''#!/bin/sh
case " $* " in *" --version "*) echo "{v}"; exit 0;; esac
if [ "$1" = "--exit-code" ]; then exit "$2"; fi
echo "FAKE {n} ARGS:"
for a in "$@"; do echo "  [$a]"; done
'''
JS = '''#!/usr/bin/env node
const args = process.argv.slice(2);
if (args.includes("--version")) { console.log("{v}"); process.exit(0); }
if (args[0] === "--exit-code") process.exit(Number(args[1]));
console.log("FAKE {n} ARGS:"); for (const a of args) console.log("  [" + a + "]");
'''

def write(path, tmpl, name, version):
    lines = tmpl.replace("{n}", name).replace("{v}", version).rstrip('\n').split('\n')
    quoted = ' '.join("'" + l.replace("'", "'\\''") + "'" for l in lines)
    return "printf '%s\\n' " + quoted + " > " + path

build = f'''FROM {pin} AS caged-build
# STAND-IN build stage: the real one downloads over HTTPS. This lays out fake CLIs where the real installers put them.
ENV DEBIAN_FRONTEND=noninteractive HOME=/home/caged
RUN apt-get update && apt-get install -y --no-install-recommends nodejs && rm -rf /var/lib/apt/lists/*
RUN useradd -m -d /home/caged -s /bin/bash caged \\
    && mkdir -p /opt/caged/pi-home /home/caged/.local/bin /opt/caged/pi-home/.pi/agent/bin \\
        /home/caged/.codex/packages/standalone/releases/0.46.0/bin \\
        /usr/lib/node_modules/@google/gemini-cli/bundle /usr/lib/node_modules/@github/copilot \\
    && {write('/home/caged/.local/bin/claude', SH, 'claude', '2.1.3 (Claude Code)')} \\
    && {write('/opt/caged/pi-home/.pi/agent/bin/pi', SH, 'pi', '0.12.0')} \\
    && {write('/home/caged/.codex/packages/standalone/releases/0.46.0/bin/codex', SH, 'codex', 'codex-cli 0.46.0')} \\
    && {write('/usr/lib/node_modules/@google/gemini-cli/bundle/gemini.js', JS, 'gemini', '0.9.0')} \\
    && {write('/usr/lib/node_modules/@github/copilot/npm-loader.js', JS, 'copilot', 'GitHub Copilot CLI 0.0.330')} \\
    && printf '%s\\n' '#!/bin/sh' 'exec /usr/local/bin/codex-real --no-daemon "$@"' > /usr/local/bin/codex \\
    && chmod 0755 /home/caged/.local/bin/claude /opt/caged/pi-home/.pi/agent/bin/pi \\
        /home/caged/.codex/packages/standalone/releases/0.46.0/bin/codex \\
        /usr/lib/node_modules/@google/gemini-cli/bundle/gemini.js /usr/lib/node_modules/@github/copilot/npm-loader.js \\
        /usr/local/bin/codex \\
    && chmod 0777 /home/caged

'''
final = final.replace('''    bash \\
    && curl -fsSL https://deb.nodesource.com/setup_lts.x | bash - \\
    && apt-get install -y --no-install-recommends nodejs \\
    && rm -rf /var/lib/apt/lists/*''', '''    bash \\
    nodejs \\
    && rm -rf /var/lib/apt/lists/*''')
a, b = final.index('# Install GitHub CLI'), final.index('RUN useradd')
final = final[:a] + '''# STAND-IN: the real image adds GitHub's apt repository over HTTPS; here gh comes from Ubuntu's archive.
RUN apt-get update && apt-get install -y --no-install-recommends gh && rm -rf /var/lib/apt/lists/*

''' + final[b:]
out = build + final
if len(sys.argv) > 3:
    out += '\n# ' + sys.argv[3] + '\n'
open(sys.argv[2], 'w').write(out)
