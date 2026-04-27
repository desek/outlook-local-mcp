# check=skip=SecretsUsedInArgOrEnv
# Dockerfile for the Outlook Local MCP Server (GoReleaser).
#
# GoReleaser builds the statically linked binary (CGO_ENABLED=0) and places it
# in the Docker build context. This Dockerfile exposes two named runtime stages
# so GoReleaser can produce both the scratch and distroless image variants from
# the same binary.
#
# Multi-stage: alpine:3 (CA certificates) -> runtime-scratch | runtime-distroless

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
# runtime-debug: gcr.io/distroless/static-debian12:debug, UID 65532.
# Includes busybox for incident response. Not recommended for production.
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
