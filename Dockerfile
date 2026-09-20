# HiveStack Manager — Local Development Dockerfile
#
# Build:
#   docker build -t hivestack-manager:dev .
#
# Run:
#   docker run -d --name hivestack-manager \
#     -p 8080:8080 -p 8443:8443 \
#     -v hivestack-data:/var/lib/hivestack \
#     -e HIVESTACK_DSN="postgres://hivestack:hivestack@postgres:5432/hivestack?sslmode=disable" \
#     hivestack-manager:dev

# ============================================================================
# Stage 1: Build
# ============================================================================
FROM golang:1.23-alpine AS builder

ARG VERSION=0.1.0
ARG BUILD_DATE=unknown
ARG GIT_COMMIT=unknown

RUN apk add --no-cache git make bash ca-certificates tzdata

WORKDIR /src

# Copy dependency files first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build the manager binary (static, no CGO)
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X main.version=${VERSION} -X main.buildDate=${BUILD_DATE} -X main.gitCommit=${GIT_COMMIT}" \
    -o /bin/hive-manager ./cmd/hive-manager

# ============================================================================
# Stage 2: Runtime
# ============================================================================
FROM alpine:3.20

LABEL maintainer="Maddy AI Consultancy <contact@maddyai.dev>"
LABEL org.opencontainers.image.title="HiveStack Manager"
LABEL org.opencontainers.image.description="HiveStack Manager — central control plane (local dev)"
LABEL org.opencontainers.image.source="https://github.com/maddydevel/HiveStack"
LABEL org.opencontainers.image.licenses="Apache-2.0"

# Install runtime dependencies: CA certs for TLS, postgresql-client for health checks & migrations
RUN apk add --no-cache \
    ca-certificates \
    postgresql-client \
    curl \
    tzdata \
    && rm -rf /var/cache/apk/*

# Create HiveStack system user and group (non-root)
RUN addgroup -S hivestack && \
    adduser -S hivestack -G hivestack -h /var/lib/hivestack -s /sbin/nologin

# Create required directories
RUN mkdir -p /etc/hivestack /var/lib/hivestack /var/log/hivestack /run/hivestack && \
    chown -R hivestack:hivestack /var/lib/hivestack /var/log/hivestack /run/hivestack && \
    chmod 750 /var/lib/hivestack /var/log/hivestack

# Copy binary from builder
COPY --from=builder /bin/hive-manager /usr/local/bin/hive-manager
RUN chmod +x /usr/local/bin/hive-manager

# Copy database migrations
COPY internal/db/migrations/ /opt/hivestack/migrations/

# Expose ports:
#   8080 - REST API
#   8443 - gRPC / TLS
EXPOSE 8080 8443

# Health check: verify the REST API health endpoint responds
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD curl -fs http://localhost:8080/api/v1/health || exit 1

# Switch to non-root user
USER hivestack:hivestack

# Working directory for runtime data
WORKDIR /var/lib/hivestack

# Volume for persistent data
VOLUME ["/var/lib/hivestack", "/etc/hivestack", "/var/log/hivestack"]

ENTRYPOINT ["/usr/local/bin/hive-manager"]
CMD ["run", "--config", "/etc/hivestack/manager.yaml"]
