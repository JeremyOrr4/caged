FROM golang:1.27-bookworm

RUN apt-get update \
    && apt-get install -y --no-install-recommends git make \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /workspace

CMD ["bash"]
