# Dockerfile for the logx CLI.
#
# This file does NOT build the binary — goreleaser builds it on the host and
# COPYs the prebuilt artifact in. That makes multi-arch builds fast and keeps
# the final image to the binary plus a CA bundle.
#
# Manual build for testing (uses goreleaser's snapshot output):
#   make snapshot
#   docker build --build-arg LOGX_BIN=dist/logx-amd64_linux_amd64_v1/logx -t logx:dev .

FROM gcr.io/distroless/static-debian12:nonroot

LABEL org.opencontainers.image.title="logx"
LABEL org.opencontainers.image.description="Pretty-print JSON slog logs"
LABEL org.opencontainers.image.source="https://github.com/AyoubTadlaoui/GoLogX"
LABEL org.opencontainers.image.licenses="MIT"

# goreleaser places the built binary at the build context root.
COPY logx /usr/local/bin/logx

ENTRYPOINT ["/usr/local/bin/logx"]
