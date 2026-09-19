#!/bin/bash
# HiveStack Container Build Script
# Builds Docker/Podman container images for HiveStack
#
# Usage:
#   ./build-container.sh [manager|node|all] [tag]
#
# Examples:
#   ./build-container.sh all latest
#   ./build-container.sh manager v1.0.0
#   ./build-container.sh node 0.1.0-dev

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
APP_DIR="${PROJECT_ROOT}/appliance"

# Configuration
REGISTRY="${REGISTRY:-}"
IMAGE_NAME="${IMAGE_NAME:-hivestack}"
VERSION="${VERSION:-$(git -C "${PROJECT_ROOT}" describe --tags --always --dirty 2>/dev/null || echo '0.1.0')}"
BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
GIT_COMMIT="$(git -C "${PROJECT_ROOT}" rev-parse --short HEAD 2>/dev/null || echo 'unknown')"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[BUILD]${NC} $*"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }

# Detect container runtime
detect_runtime() {
    if command -v docker &>/dev/null; then
        echo "docker"
    elif command -v podman &>/dev/null; then
        echo "podman"
    else
        log_error "Neither Docker nor Podman found"
        exit 1
    fi
}

# Build the container image
build_image() {
    local component="${1:-all}"
    local tag="${2:-${VERSION}}"
    local runtime
    runtime="$(detect_runtime)"

    local full_tag="${IMAGE_NAME}:${tag}"
    if [ -n "${REGISTRY}" ]; then
        full_tag="${REGISTRY}/${full_tag}"
    fi

    log_info "Building HiveStack container image"
    log_info "Runtime: ${runtime}"
    log_info "Component: ${component}"
    log_info "Tag: ${full_tag}"
    log_info "Version: ${VERSION}"
    log_info "Build Date: ${BUILD_DATE}"
    log_info "Git Commit: ${GIT_COMMIT}"
    log_info "Project Root: ${PROJECT_ROOT}"
    echo ""

    # Build arguments
    local build_args=(
        "--build-arg" "VERSION=${VERSION}"
        "--build-arg" "BUILD_DATE=${BUILD_DATE}"
        "--build-arg" "GIT_COMMIT=${GIT_COMMIT}"
        "--file" "${APP_DIR}/Dockerfile"
        "--tag" "${full_tag}"
        "--label" "org.opencontainers.image.created=${BUILD_DATE}"
        "--label" "org.opencontainers.image.version=${VERSION}"
        "--label" "org.opencontainers.image.revision=${GIT_COMMIT}"
    )

    # Add component-specific tags
    if [ "${component}" != "all" ]; then
        build_args+=("--tag" "${IMAGE_NAME}:${component}-${tag}")
    fi

    # Build
    log_info "Starting build..."
    "${runtime}" build "${build_args[@]}" "${PROJECT_ROOT}"

    echo ""
    log_info "Build completed: ${full_tag}"

    # Show image info
    "${runtime}" images "${full_tag}" --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}\t{{.CreatedSince}}" 2>/dev/null || \
        "${runtime}" images "${full_tag}"

    echo ""
    log_info "To run the container:"
    echo "  ${runtime} run -d --name hivestack-manager \\"
    echo "    -p 8080:8080 -p 8443:8443 \\"
    echo "    -v hivestack-data:/var/lib/hivestack \\"
    echo "    ${full_tag} manager"
    echo ""
    log_info "To push to registry:"
    echo "  ${runtime} push ${full_tag}"
}

# Build Go binaries first (optional)
build_binaries() {
    log_info "Building Go binaries..."
    cd "${PROJECT_ROOT}"

    mkdir -p bin

    CGO_ENABLED=0 GOOS=linux go build \
        -ldflags="-w -s -X main.version=${VERSION} -X main.buildDate=${BUILD_DATE} -X main.gitCommit=${GIT_COMMIT}" \
        -o bin/hive-manager ./cmd/hive-manager

    CGO_ENABLED=0 GOOS=linux go build \
        -ldflags="-w -s -X main.version=${VERSION} -X main.buildDate=${BUILD_DATE} -X main.gitCommit=${GIT_COMMIT}" \
        -o bin/hive-node ./node

    CGO_ENABLED=0 GOOS=linux go build \
        -ldflags="-w -s -X main.version=${VERSION} -X main.buildDate=${BUILD_DATE} -X main.gitCommit=${GIT_COMMIT}" \
        -o bin/hive ./cmd/hive

    log_info "Binaries built successfully:"
    ls -la bin/
}

# Main
main() {
    local component="${1:-all}"
    local tag="${2:-${VERSION}}"

    case "${component}" in
        all|manager|node)
            build_image "${component}" "${tag}"
            ;;
        binaries)
            build_binaries
            ;;
        *)
            echo "Usage: $0 [all|manager|node|binaries] [tag]"
            echo ""
            echo "Examples:"
            echo "  $0 all latest          # Build all components with 'latest' tag"
            echo "  $0 manager v1.0.0      # Build manager with version tag"
            echo "  $0 node                # Build node with default version tag"
            echo "  $0 binaries            # Build Go binaries only"
            echo ""
            echo "Environment variables:"
            echo "  REGISTRY   - Container registry prefix (e.g., ghcr.io/maddydevel)"
            echo "  IMAGE_NAME - Image name (default: hivestack)"
            echo "  VERSION    - Version tag (default: from git describe)"
            exit 1
            ;;
    esac
}

main "$@"
