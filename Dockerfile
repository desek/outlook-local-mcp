# check=skip=SecretsUsedInArgOrEnv
# Dockerfile for the Outlook Local MCP Server.
#
# This file serves two distinct build paths:
#
#  1. Release path (GoReleaser). GoReleaser builds the statically linked binary
#     (CGO_ENABLED=0) and places it in the Docker build context; the
#     runtime-scratch, runtime-distroless, and runtime-debug stages just copy
#     it in. These stages CANNOT be built on their own, because the binary is
#     not in a clean checkout. CI and the release workflow always select them
#     explicitly with --target.
#
#  2. Standalone path (default). The `build` stage compiles from source inside
#     the image, so `docker build .` works from a plain `git clone` with no
#     GoReleaser step. `standalone` is deliberately the LAST stage in this file
#     so that a bare build with no --target resolves to it. This is what
#     third-party build services (e.g. the Glama MCP directory) and
#     docker-compose use.
#
# Multi-stage:
#   alpine:3 (CA certificates) -> runtime-scratch | runtime-distroless | runtime-debug
#   golang (compile from source) -> standalone

# ---------------------------------------------------------------------------
# CA certificates stage (scratch variant needs these; distroless includes them)
# ---------------------------------------------------------------------------
FROM --platform=$BUILDPLATFORM alpine:3 AS certs

RUN apk add --no-cache ca-certificates

# ---------------------------------------------------------------------------
# runtime-scratch: minimal scratch image, root UID, ca-certs from alpine.
# Smallest possible attack surface. This is the :latest / :vX.Y.Z image.
# ---------------------------------------------------------------------------
FROM scratch AS runtime-scratch

ARG TARGETOS
ARG TARGETARCH

COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY ${TARGETOS}/${TARGETARCH}/outlook-local-mcp /usr/local/bin/outlook-local-mcp

LABEL org.opencontainers.image.title="Outlook Local MCP Server" \
      org.opencontainers.image.description="Model Context Protocol server for Outlook via Microsoft Graph API" \
      org.opencontainers.image.source="https://github.com/desek/outlook-local-mcp" \
      org.opencontainers.image.licenses="MIT"

ENV OUTLOOK_MCP_AUTH_RECORD_PATH=/data/auth/auth_record.json \
    RUNNING_IN_CONTAINER=1

ENTRYPOINT ["/usr/local/bin/outlook-local-mcp"]

# ---------------------------------------------------------------------------
# runtime-distroless: gcr.io/distroless/static-debian12:nonroot, UID 65532.
# ca-certs and tzdata are included by the base image. No shell.
# This is the :distroless / :vX.Y.Z-distroless image.
# ---------------------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot AS runtime-distroless

ARG TARGETOS
ARG TARGETARCH

COPY ${TARGETOS}/${TARGETARCH}/outlook-local-mcp /usr/local/bin/outlook-local-mcp

LABEL org.opencontainers.image.title="Outlook Local MCP Server" \
      org.opencontainers.image.description="Model Context Protocol server for Outlook via Microsoft Graph API" \
      org.opencontainers.image.source="https://github.com/desek/outlook-local-mcp" \
      org.opencontainers.image.licenses="MIT"

ENV OUTLOOK_MCP_AUTH_RECORD_PATH=/data/auth/auth_record.json \
    RUNNING_IN_CONTAINER=1

ENTRYPOINT ["/usr/local/bin/outlook-local-mcp"]

# ---------------------------------------------------------------------------
# runtime-debug: gcr.io/distroless/static-debian12:debug, which runs as root
# (UID 0) -- unlike the :debug-nonroot variant. Includes busybox for incident
# response. Not recommended for production.
# This is the :debug / :vX.Y.Z-debug image.
# ---------------------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:debug AS runtime-debug

ARG TARGETOS
ARG TARGETARCH

COPY ${TARGETOS}/${TARGETARCH}/outlook-local-mcp /usr/local/bin/outlook-local-mcp

LABEL org.opencontainers.image.title="Outlook Local MCP Server" \
      org.opencontainers.image.description="Model Context Protocol server for Outlook via Microsoft Graph API" \
      org.opencontainers.image.source="https://github.com/desek/outlook-local-mcp" \
      org.opencontainers.image.licenses="MIT"

ENV OUTLOOK_MCP_AUTH_RECORD_PATH=/data/auth/auth_record.json \
    RUNNING_IN_CONTAINER=1

ENTRYPOINT ["/usr/local/bin/outlook-local-mcp"]

# ---------------------------------------------------------------------------
# build: compile the server from source. Used only by the standalone path, so
# the release stages above never pay for a Go toolchain pull.
#
# Runs on BUILDPLATFORM and cross-compiles via GOOS/GOARCH so multi-arch builds
# do not need QEMU emulation of the compiler. CGO_ENABLED=0 matches the
# `container` GoReleaser build: a static binary, and the file-backed token cache
# rather than the CGO keyring backend.
# ---------------------------------------------------------------------------
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build

WORKDIR /src

# Copy the module files first so dependency download is cached independently of
# source edits.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH

# Overridable so a packager can stamp a real release identity; the default
# matches the ldflags-free `go build` value reported by system.about.
ARG VERSION=dev

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o /out/outlook-local-mcp ./cmd/outlook-local-mcp

# Pre-create the token cache directory owned by the distroless nonroot UID.
# The runtime base has no shell, so it cannot mkdir; staging the directory here
# and copying it across avoids the "AuthRecordPath parent directory does not
# exist" warning on every start.
RUN mkdir -p /out/data/auth && chown -R 65532:65532 /out/data

# ---------------------------------------------------------------------------
# standalone: DEFAULT TARGET. Same distroless nonroot base as
# runtime-distroless, but built from source rather than a staged binary, so it
# needs no GoReleaser step. Keep this stage LAST — a bare `docker build .`
# resolves to the final stage in the file.
# ---------------------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot AS standalone

COPY --from=build /out/outlook-local-mcp /usr/local/bin/outlook-local-mcp
COPY --from=build --chown=65532:65532 /out/data /data

LABEL org.opencontainers.image.title="Outlook Local MCP Server" \
      org.opencontainers.image.description="Model Context Protocol server for Outlook via Microsoft Graph API" \
      org.opencontainers.image.source="https://github.com/desek/outlook-local-mcp" \
      org.opencontainers.image.licenses="MIT"

ENV OUTLOOK_MCP_AUTH_RECORD_PATH=/data/auth/auth_record.json \
    RUNNING_IN_CONTAINER=1

ENTRYPOINT ["/usr/local/bin/outlook-local-mcp"]
