# check=skip=SecretsUsedInArgOrEnv
# Dockerfile for the Outlook Local MCP Server (GoReleaser).
#
# GoReleaser builds the statically linked binary (CGO_ENABLED=0) and places it
# in the Docker build context. This Dockerfile copies the binary and CA
# certificates into a scratch image for minimal size and attack surface.
#
# Multi-stage: alpine:3 (CA certificates) -> scratch (runtime).

# ---------------------------------------------------------------------------
# CA certificates stage
# ---------------------------------------------------------------------------
FROM --platform=$BUILDPLATFORM alpine:3 AS certs

RUN apk add --no-cache ca-certificates

# ---------------------------------------------------------------------------
# Runtime stage
# ---------------------------------------------------------------------------
FROM scratch

ARG TARGETOS
ARG TARGETARCH

COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY ${TARGETOS}/${TARGETARCH}/outlook-local-mcp /usr/local/bin/outlook-local-mcp

LABEL org.opencontainers.image.title="Outlook Local MCP Server" \
      org.opencontainers.image.description="Model Context Protocol server for Outlook via Microsoft Graph API" \
      org.opencontainers.image.source="https://github.com/desek/outlook-local-mcp" \
      org.opencontainers.image.licenses="MIT"

ENV OUTLOOK_MCP_AUTH_RECORD_PATH=/data/auth/auth_record.json

ENTRYPOINT ["/usr/local/bin/outlook-local-mcp"]
