#!/bin/sh
# Build the logx-mcp MCP server inside Glama's debian:trixie-slim build image.
#
# Glama runs each "Build step" as a separate RUN with WORKDIR /app (the repo
# root it cloned). Keeping the real command here, in a committed file, avoids
# the whitespace mangling that happens when a multi-token shell command is
# pasted into Glama's JSON build-step field.
#
# Configure on Glama as a single build step:  ["sh ./scripts/glama-build.sh"]
set -eu

GO_VERSION=go1.22.6
ARCH="$(dpkg --print-architecture)"        # amd64 / arm64 — matches Go's naming

curl -fsSL "https://go.dev/dl/${GO_VERSION}.linux-${ARCH}.tar.gz" -o /tmp/go.tgz
tar -C /usr/local -xzf /tmp/go.tgz

export PATH="/usr/local/go/bin:${PATH}"
export CGO_ENABLED=0
export GOCACHE=/tmp/gocache
export GOPATH=/tmp/gopath

go build -trimpath -o /app/logx-mcp ./cmd/logx-mcp
