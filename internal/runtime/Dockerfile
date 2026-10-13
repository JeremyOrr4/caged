FROM ubuntu:24.04 AS caged-build

ENV DEBIAN_FRONTEND=noninteractive \
    HOME=/home/caged \
    PATH=/home/caged/.local/bin:/home/caged/.pi/agent/bin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

# Compilers are needed to build some CLI dependencies, not to run the installed tools.
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    git \
    curl \
    ca-certificates \
    ripgrep \
    jq \
    nano \
    python3 \
    bash \
    && curl -fsSL https://deb.nodesource.com/setup_lts.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/*

# Agent config directories must support arbitrary Linux host UID/GID mapping;
# Docker Desktop translates ownership for macOS bind mounts.
RUN useradd -m -d /home/caged -s /bin/bash caged \
    && mkdir -p /opt/caged/pi-home /home/caged/.local \
    && chmod 0777 /home/caged

RUN curl -fsSL https://claude.ai/install.sh | bash
RUN HOME=/opt/caged/pi-home sh -c 'curl -fsSL https://pi.dev/install.sh | sh' \
    && ln -sf /opt/caged/pi-home/.pi/agent/bin/pi /usr/local/bin/pi \
    && npm cache clean --force \
    && rm -rf /opt/caged/pi-home/.npm
# Keep Codex's long-lived daemon from outliving the interactive caged command.
RUN curl -fsSL https://chatgpt.com/codex/install.sh | sh \
    && mv /home/caged/.local/bin/codex /usr/local/bin/codex-real \
    && printf '%s\n' '#!/bin/sh' 'exec /usr/local/bin/codex-real --no-daemon "$@"' > /usr/local/bin/codex \
    && chmod 0755 /usr/local/bin/codex
RUN npm install -g --allow-scripts=@github/keytar,node-pty @google/gemini-cli \
    && npm cache clean --force \
    && rm -rf /home/caged/.npm /home/caged/.cache/node-gyp /root/.npm /root/.cache/node-gyp
RUN npm install -g @github/copilot \
    && npm cache clean --force \
    && rm -rf /home/caged/.npm /home/caged/.cache/node-gyp /root/.npm /root/.cache/node-gyp

FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive \
    HOME=/home/caged \
    CLAUDE_CONFIG_DIR=/home/caged/.claude \
    PATH=/home/caged/.local/bin:/home/caged/.pi/agent/bin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

# The final image has runtime dependencies only; compilers remain in the build stage.
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    curl \
    ca-certificates \
    ripgrep \
    jq \
    nano \
    python3 \
    make \
    bash \
    && curl -fsSL https://deb.nodesource.com/setup_lts.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/*

# Install GitHub CLI from GitHub's signed package repository.
RUN install -d -m 0755 /etc/apt/keyrings \
    && curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg -o /etc/apt/keyrings/githubcli-archive-keyring.gpg \
    && chmod a+r /etc/apt/keyrings/githubcli-archive-keyring.gpg \
    && printf 'deb [arch=%s signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main\n' "$(dpkg --print-architecture)" > /etc/apt/sources.list.d/github-cli.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends gh \
    && rm -rf /var/lib/apt/lists/*

RUN useradd -m -d /home/caged -s /bin/bash caged \
    && mkdir -p /workspace /home/caged/.config/pi /home/caged/.config/gh /home/caged/.pi /home/caged/.claude \
        /home/caged/.codex /home/caged/.gemini /home/caged/.copilot /home/caged/.cache \
    && chmod 0777 /home/caged /home/caged/.config /home/caged/.config/pi /home/caged/.config/gh /home/caged/.pi \
        /home/caged/.claude /home/caged/.codex /home/caged/.gemini /home/caged/.copilot \
    && chown caged:caged /home/caged/.cache \
    && git config --system --add safe.directory '/workspace/*' \
    && echo 'export PATH="/home/caged/.local/bin:/home/caged/.pi/agent/bin:$PATH"' >> /home/caged/.profile

COPY --from=caged-build /home/caged/.local /home/caged/.local
COPY --from=caged-build /home/caged/.codex/packages/standalone /opt/caged/codex-cli
COPY --from=caged-build /opt/caged /opt/caged
COPY --from=caged-build /usr/local/bin/codex /usr/local/bin/codex
COPY --from=caged-build /usr/lib/node_modules/@google /usr/lib/node_modules/@google
COPY --from=caged-build /usr/lib/node_modules/@github /usr/lib/node_modules/@github

COPY ./caged-entrypoint.sh /usr/local/bin/caged-entrypoint
RUN codex_release="$(find /opt/caged/codex-cli/releases -mindepth 1 -maxdepth 1 -type d -print -quit)" \
    && ln -sfn "$codex_release" /opt/caged/codex-cli/current \
    && ln -sf /opt/caged/codex-cli/current/bin/codex /usr/local/bin/codex-real \
    && ln -sf /opt/caged/pi-home/.pi/agent/bin/pi /usr/local/bin/pi \
    && ln -sf ../lib/node_modules/@google/gemini-cli/bundle/gemini.js /usr/bin/gemini \
    && ln -sf ../lib/node_modules/@github/copilot/npm-loader.js /usr/bin/copilot \
    && chmod 0755 /usr/local/bin/caged-entrypoint \
    && rm -f /home/caged/.claude.json

WORKDIR /workspace
USER caged

ENTRYPOINT ["/usr/local/bin/caged-entrypoint"]
CMD ["bash"]
