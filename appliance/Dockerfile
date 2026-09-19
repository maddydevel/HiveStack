# HiveStack Container Image
# Multi-stage build for HiveStack Manager and Node Agent
#
# Build:
#   docker build -t hivestack:latest .
#   podman build -t hivestack:latest .
#
# Run Manager:
#   docker run -d --name hivestack-manager \
#     -p 8080:8080 -p 8443:8443 \
#     -v hivestack-data:/var/lib/hivestack \
#     -v hivestack-certs:/etc/hivestack/certs \
#     hivestack:latest manager
#
# Run Node Agent:
#   docker run -d --name hivestack-node \
#     --privileged \
#     -p 9090:9090 \
#     -v /dev/kvm:/dev/kvm \
#     -v hivestack-data:/var/lib/hivestack \
#     hivestack:latest node

# ============================================================================
# Stage 1: Build Go binaries
# ============================================================================
FROM golang:1.26-alpine AS builder

ARG VERSION=0.1.0
ARG BUILD_DATE=unknown
ARG GIT_COMMIT=unknown

RUN apk add --no-cache git make bash

WORKDIR /src

# Copy dependency files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build all binaries
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X main.version=${VERSION} -X main.buildDate=${BUILD_DATE} -X main.gitCommit=${GIT_COMMIT}" \
    -o /bin/hive-manager ./cmd/hive-manager

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X main.version=${VERSION} -X main.buildDate=${BUILD_DATE} -X main.gitCommit=${GIT_COMMIT}" \
    -o /bin/hive-node ./node

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X main.version=${VERSION} -X main.buildDate=${BUILD_DATE} -X main.gitCommit=${GIT_COMMIT}" \
    -o /bin/hive ./cmd/hive

# ============================================================================
# Stage 2: Runtime image
# ============================================================================
FROM rockylinux:9-minimal AS runtime

LABEL maintainer="Maddy AI Consultancy <contact@maddyai.dev>"
LABEL org.opencontainers.image.title="HiveStack"
LABEL org.opencontainers.image.description="KVM Virtualization Management Platform"
LABEL org.opencontainers.image.source="https://github.com/maddydevel/HiveStack"
LABEL org.opencontainers.image.licenses="Apache-2.0"

# Install runtime dependencies
RUN microdnf install -y \
    openssl \
    ca-certificates \
    postgresql \
    qemu-kvm \
    libvirt \
    libvirt-client \
    iproute \
    iputils \
    curl \
    procps-ng \
    && microdnf clean all

# Create HiveStack system user and group
RUN groupadd -r hivestack && \
    useradd -r -g hivestack -d /var/lib/hivestack -s /sbin/nologin hivestack && \
    groupadd -r hivestack-node && \
    useradd -r -g hivestack-node -d /var/lib/hivestack-node -s /sbin/nologin hivestack-node

# Create HiveStack directories
RUN mkdir -p \
    /etc/hivestack \
    /etc/hivestack/certs \
    /var/lib/hivestack \
    /var/lib/hivestack-node \
    /var/log/hivestack \
    /opt/hivestack/bin \
    /run/hivestack \
    /var/lib/libvirt/images/hivestack \
    && chown -R hivestack:hivestack /var/lib/hivestack /var/log/hivestack /run/hivestack \
    && chown -R hivestack-node:hivestack-node /var/lib/hivestack-node \
    && chmod 750 /var/lib/hivestack /var/log/hivestack

# Copy binaries from builder
COPY --from=builder /bin/hive-manager /opt/hivestack/bin/hive-manager
COPY --from=builder /bin/hive-node /opt/hivestack/bin/hive-node
COPY --from=builder /bin/hive /opt/hivestack/bin/hive
RUN chmod +x /opt/hivestack/bin/*

# Copy configuration examples
COPY config/manager.yaml.example /etc/hivestack/manager.yaml
COPY config/node.yaml.example /etc/hivestack/node.yaml

# Copy first-boot script
COPY scripts/first-boot.sh /opt/hivestack/bin/first-boot.sh
RUN chmod +x /opt/hivestack/bin/first-boot.sh

# Copy systemd service files (for reference; not used in container)
COPY systemd/ /etc/systemd/system/

# Copy migration files
COPY migration/ /opt/hivestack/migration/

# Kernel tuning for KVM
RUN echo "vm.nr_hugepages = 1024" > /etc/sysctl.d/99-hivestack.conf && \
    echo "net.core.somaxconn = 4096" >> /etc/sysctl.d/99-hivestack.conf && \
    echo "fs.file-max = 2097152" >> /etc/sysctl.d/99-hivestack.conf

# Expose ports
# 8080 - HiveStack Manager REST API
# 8443 - HiveStack Manager gRPC
# 9090 - HiveStack Node Agent gRPC
EXPOSE 8080 8443 9090

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD curl -kfs https://localhost:8080/api/v1/health || exit 1

# Volume mounts for persistent data
VOLUME ["/var/lib/hivestack", "/var/lib/hivestack-node", "/etc/hivestack", "/var/log/hivestack"]

# Entrypoint
COPY scripts/entrypoint.sh /opt/hivestack/bin/entrypoint.sh
RUN chmod +x /opt/hivestack/bin/entrypoint.sh

ENTRYPOINT ["/opt/hivestack/bin/entrypoint.sh"]
CMD ["manager"]
