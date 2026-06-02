FROM debian:bookworm-slim

# ── System packages ──────────────────────────────────────────────────────────
RUN apt-get update && apt-get install -y \
    curl \
    nano \
    tree \
    git \
    ca-certificates \
    procps \
    htop \
    lsof \
    make \
    bash-completion \
    && rm -rf /var/lib/apt/lists/*

# ── Install Go ────────────────────────────────────────────────────────────────
ARG GO_VERSION=1.26.0
RUN curl -fsSL https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz \
    | tar -C /usr/local -xz

# ── Go env ────────────────────────────────────────────────────────────────────
ENV GOPATH="/root/go"
ENV GOBIN="/root/go/bin"
ENV PATH="/usr/local/go/bin:${GOBIN}:${PATH}"

# ── Install `asynq` CLI (for queue monitoring via `docker exec`) ──────────────
RUN go install github.com/hibiken/asynq/tools/asynq@latest

# ── Install Node.js + npm + pm2 ──────────────────────────────────────────────
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - \
    && apt-get install -y nodejs \
    && rm -rf /var/lib/apt/lists/* \
    && npm install -g pm2

# ── Bash aliases & completion ────────────────────────────────────────────────
RUN echo "alias ll='ls -lah'" >> /root/.bashrc \
    && echo '[ -f /usr/share/bash-completion/bash_completion ] && . /usr/share/bash-completion/bash_completion' >> /root/.bashrc \
    && echo 'complete -W "$(make -qp 2>/dev/null | awk -F: "/^[a-zA-Z_-]+:/{print \$1}" | sort -u)" make' >> /root/.bashrc

# ── Working dir ───────────────────────────────────────────────────────────────
WORKDIR /app

# ── Keep container alive ──────────────────────────────────────────────────────
CMD ["bash", "-c", "tail -f /dev/null"]