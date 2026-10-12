FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive \
    HOME=/home/caged \
    PATH=/home/caged/.local/bin:/home/caged/.pi/agent/bin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    curl \
    ca-certificates \
    build-essential \
    ripgrep \
    jq \
    nano \
    python3 \
    bash \
    && curl -fsSL https://deb.nodesource.com/setup_lts.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/*

RUN useradd -m -d /home/caged -s /bin/bash caged \
    && mkdir -p /workspace \
    && chmod 0777 /home/caged \
    && git config --system --add safe.directory /workspace \
    && echo 'export PATH="/home/caged/.local/bin:/home/caged/.pi/agent/bin:$PATH"' >> /home/caged/.profile

RUN curl -fsSL https://claude.ai/install.sh | bash \
    && curl -fsSL https://pi.dev/install.sh | sh \
    && curl -fsSL https://chatgpt.com/codex/install.sh | sh \
    && npm install -g @google/gemini-cli \
    && ln -sf /home/caged/.local/bin/claude /usr/local/bin/claude \
    && ln -sf /home/caged/.local/bin/codex /usr/local/bin/codex \
    && ln -sf /home/caged/.pi/agent/bin/pi /usr/local/bin/pi

COPY ./caged-entrypoint.sh /usr/local/bin/caged-entrypoint
RUN chmod 0755 /usr/local/bin/caged-entrypoint

WORKDIR /workspace

USER caged

ENTRYPOINT ["/usr/local/bin/caged-entrypoint"]
CMD ["bash"]
