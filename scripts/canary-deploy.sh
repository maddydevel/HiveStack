#!/usr/bin/env bash
# canary-deploy.sh — Canary deployment for HiveStack
#
# Usage:
#   ./canary-deploy.sh [options]
#
# Options:
#   --image <image>        Container image to deploy (default: ghcr.io/maddydevel/hivestack:latest)
#   --version <version>    Version to deploy (used in logs)
#   --nodes <nodes>        Comma-separated list of nodes to deploy (default: all)
#   --percentage <int>     Percentage of nodes to deploy in first wave (default: 25)
#   --wave-size <int>      Number of nodes per subsequent wave (default: 1)
#   --timeout <seconds>    Timeout per wave before proceeding (default: 300)
#   --auto-promote         Automatically promote to next wave after timeout
#   --dry-run              Show what would be done without executing
#   --skip-tests           Skip post-deployment verification tests
#   --rollback             Rollback to previous version on any node
#   -h, --help             Show this help message
#
# Exit codes:
#   0 - Success
#   1 - General failure
#   2 - Invalid arguments
#   3 - Health check failed
#   4 - Timeout

set -euo pipefail

# ============================================================================
# Configuration
# ============================================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Defaults
IMAGE="ghcr.io/maddydevel/hivestack:latest"
VERSION="unknown"
NODES_LIST=""
PERCENTAGE=25
WAVE_SIZE=1
TIMEOUT=300
AUTO_PROMOTE=false
DRY_RUN=false
SKIP_TESTS=false
ROLLBACK=false

# State tracking
DEPLOYED_NODES=()
FAILED_NODES=()
WAVE_NUMBER=0
PREVIOUS_VERSIONS=()

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# ============================================================================
# Functions
# ============================================================================

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[OK]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

show_help() {
    sed -n '2,20p' "$0" | sed 's/^# \?//'
    exit 0
}

die() {
    log_error "$1"
    exit "${2:-1}"
}

# ============================================================================
# Parse Arguments
# ============================================================================

while [[ $# -gt 0 ]]; do
    case $1 in
        --image)      IMAGE="$2"; shift 2 ;;
        --version)    VERSION="$2"; shift 2 ;;
        --nodes)      NODES_LIST="$2"; shift 2 ;;
        --percentage) PERCENTAGE="$2"; shift 2 ;;
        --wave-size)  WAVE_SIZE="$2"; shift 2 ;;
        --timeout)    TIMEOUT="$2"; shift 2 ;;
        --auto-promote)   AUTO_PROMOTE=true; shift ;;
        --dry-run)        DRY_RUN=true; shift ;;
        --skip-tests)     SKIP_TESTS=true; shift ;;
        --rollback)       ROLLBACK=true; shift ;;
        -h|--help)    show_help ;;
        *)            die "Unknown option: $1" 2 ;;
    esac
done

# ============================================================================
# Pre-flight Checks
# ============================================================================

preflight() {
    log_info "Running pre-flight checks..."

    # Check dependencies
    local deps=("docker" "curl" "jq" "hive")
    for dep in "${deps[@]}"; do
        if ! command -v "$dep" &> /dev/null; then
            die "Required dependency not found: $dep"
        fi
    done

    # Validate image exists
    if ! docker pull "$IMAGE" --quiet 2>/dev/null; then
        die "Cannot pull container image: $IMAGE"
    fi

    # Get node list
    if [[ -z "$NODES_LIST" ]]; then
        # Discover nodes from hive
        NODES_LIST=$(hive node list --format json 2>/dev/null | jq -r '.[].id' | tr '\n' ',' | sed 's/,$//')
        if [[ -z "$NODES_LIST" ]]; then
            die "No nodes found. Use --nodes to specify explicitly."
        fi
    fi

    IFS=',' read -ra ALL_NODES <<< "$NODES_LIST"
    log_info "Total nodes: ${#ALL_NODES[@]}"
    log_info "Image: ${IMAGE}"

    # Store previous versions for rollback
    if [[ "$ROLLBACK" == "false" ]]; then
        for node in "${ALL_NODES[@]}"; do
            local ver
            ver=$(docker exec "hivestack-node-${node}" /opt/hivestack/bin/hive-node --version 2>/dev/null || echo "unknown")
            PREVIOUS_VERSIONS+=("$node:$ver")
        done
    fi

    log_success "Pre-flight checks passed"
}

# ============================================================================
# Wave Calculation
# ============================================================================

calculate_wave_size() {
    local total=$1
    local percent=$2

    # Calculate first wave size
    local wave_size=$(( total * percent / 100 ))

    # Ensure at least 1 node in first wave
    if [[ $wave_size -lt 1 ]]; then
        wave_size=1
    fi

    echo "$wave_size"
}

# ============================================================================
# Deploy to Single Node
# ============================================================================

deploy_to_node() {
    local node=$1
    local image=$2

    log_info "Deploying to node: ${node}"

    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "[DRY RUN] Would deploy ${image} to ${node}"
        return 0
    fi

    # Drain node (migrate VMs away)
    log_info "Draining node ${node}..."
    if ! hive node maintenance enable "$node" --timeout 600; then
        log_error "Failed to drain node ${node}"
        return 1
    fi

    # Wait for node to be drained
    local wait_count=0
    while true; do
        local vm_count
        vm_count=$(hive vm list --node "$node" --format json 2>/dev/null | jq 'length')
        if [[ "$vm_count" -eq 0 ]]; then
            break
        fi
        wait_count=$((wait_count + 1))
        if [[ $wait_count -gt 60 ]]; then
            log_error "Timeout waiting for node ${node} to drain"
            return 1
        fi
        sleep 10
    done

    # Pull new image
    log_info "Pulling image on ${node}..."
    if ! docker pull "$image" --quiet; then
        log_error "Failed to pull image on ${node}"
        return 1
    fi

    # Stop current container
    log_info "Stopping current container on ${node}..."
    docker stop "hivestack-node-${node}" 2>/dev/null || true
    docker rm "hivestack-node-${node}" 2>/dev/null || true

    # Start new container
    log_info "Starting new container on ${node}..."
    if ! docker run -d \
        --name "hivestack-node-${node}" \
        --privileged \
        -v /dev/kvm:/dev/kvm \
        -v "hivestack-node-data-${node}:/var/lib/hivestack" \
        -v "hivestack-node-certs-${node}:/etc/hivestack/certs" \
        -e "NODE_ID=${node}" \
        -e "MANAGER_ADDRESS=hivestack-manager:9090" \
        --network hivestack-internal \
        --restart unless-stopped \
        "$image" node; then
        log_error "Failed to start new container on ${node}"
        return 1
    fi

    # Wait for node to be ready
    log_info "Waiting for node ${node} to be ready..."
    local ready_count=0
    while true; do
        if docker exec "hivestack-node-${node}" /opt/hivestack/bin/hive-node --version &>/dev/null; then
            break
        fi
        ready_count=$((ready_count + 1))
        if [[ $ready_count -gt 30 ]]; then
            log_error "Timeout waiting for node ${node} to be ready"
            return 1
        fi
        sleep 5
    done

    # Disable maintenance mode
    log_info "Disabling maintenance mode on ${node}..."
    if ! hive node maintenance disable "$node"; then
        log_warn "Failed to disable maintenance on ${node} (may already be disabled)"
    fi

    log_success "Node ${node} deployed successfully"
    return 0
}

# ============================================================================
# Health Check
# ============================================================================

health_check() {
    local node=$1

    if [[ "$SKIP_TESTS" == "true" ]]; then
        log_info "Skipping health check for ${node}"
        return 0
    fi

    log_info "Running health check on ${node}..."

    # Check node is connected
    local status
    status=$(hive node status "$node" --format json 2>/dev/null | jq -r '.status')
    if [[ "$status" != "connected" ]]; then
        log_error "Node ${node} status: ${status} (expected: connected)"
        return 1
    fi

    # Check node version
    local node_version
    node_version=$(docker exec "hivestack-node-${node}" /opt/hivestack/bin/hive-node --version 2>/dev/null)
    if [[ "$node_version" != *"$VERSION"* ]] && [[ "$VERSION" != "unknown" ]]; then
        log_warn "Node ${node} version mismatch: ${node_version} (expected: ${VERSION})"
    fi

    # Check node can list VMs (basic functionality)
    if ! hive vm list --node "$node" --format json &>/dev/null; then
        log_error "Node ${node} cannot list VMs"
        return 1
    fi

    log_success "Health check passed for ${node}"
    return 0
}

# ============================================================================
# Rollback
# ============================================================================

rollback_node() {
    local node=$1
    local previous_version=$2

    log_warn "Rolling back node ${node} to ${previous_version}..."

    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "[DRY RUN] Would rollback ${node} to ${previous_version}"
        return 0
    fi

    # Find previous image
    local previous_image="ghcr.io/maddydevel/hivestack:${previous_version}"

    # Drain node
    hive node maintenance enable "$node" --timeout 600 || true

    # Stop and remove current container
    docker stop "hivestack-node-${node}" 2>/dev/null || true
    docker rm "hivestack-node-${node}" 2>/dev/null || true

    # Start previous version
    docker run -d \
        --name "hivestack-node-${node}" \
        --privileged \
        -v /dev/kvm:/dev/kvm \
        -v "hivestack-node-data-${node}:/var/lib/hivestack" \
        -v "hivestack-node-certs-${node}:/etc/hivestack/certs" \
        -e "NODE_ID=${node}" \
        -e "MANAGER_ADDRESS=hivestack-manager:9090" \
        --network hivestack-internal \
        --restart unless-stopped \
        "$previous_image" node

    # Disable maintenance
    hive node maintenance disable "$node" || true

    log_success "Node ${node} rolled back to ${previous_version}"
}

# ============================================================================
# Deploy Wave
# ============================================================================

deploy_wave() {
    local -a nodes=("$@")
    WAVE_NUMBER=$((WAVE_NUMBER + 1))

    echo ""
    log_info "========================================="
    log_info "Wave ${WAVE_NUMBER}: Deploying to ${#nodes[@]} node(s)"
    log_info "========================================="

    local failed=0

    for node in "${nodes[@]}"; do
        echo ""
        if ! deploy_to_node "$node" "$IMAGE"; then
            FAILED_NODES+=("$node")
            failed=$((failed + 1))
            continue
        fi
        DEPLOYED_NODES+=("$node")
    done

    # Health check all nodes in this wave
    if [[ "$failed" -eq 0 ]]; then
        echo ""
        log_info "Running health checks for wave ${WAVE_NUMBER}..."
        for node in "${nodes[@]}"; do
            if ! health_check "$node"; then
                failed=$((failed + 1))
            fi
        done
    fi

    return "$failed"
}

# ============================================================================
# Main
# ============================================================================

main() {
    echo ""
    echo "=============================================="
    echo "  HiveStack Canary Deployment"
    echo "=============================================="
    echo ""

    # Pre-flight
    preflight

    # Calculate wave sizes
    local total_nodes=${#ALL_NODES[@]}
    local first_wave_size
    first_wave_size=$(calculate_wave_size "$total_nodes" "$PERCENTAGE")

    log_info "Deployment plan:"
    log_info "  Total nodes: ${total_nodes}"
    log_info "  First wave: ${first_wave_size} node(s) (${PERCENTAGE}%)"
    log_info "  Subsequent waves: ${WAVE_SIZE} node(s) each"
    log_info "  Timeout per wave: ${TIMEOUT}s"
    log_info "  Auto-promote: ${AUTO_PROMOTE}"
    echo ""

    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "DRY RUN MODE — no changes will be made"
        echo ""
    fi

    # Confirm deployment
    if [[ "$DRY_RUN" == "false" ]] && [[ "$AUTO_PROMOTE" == "false" ]]; then
        read -rp "Proceed with deployment? [y/N] " confirm
        if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
            log_info "Deployment cancelled"
            exit 0
        fi
    fi

    # First wave
    local -a first_wave=("${ALL_NODES[@]:0:${first_wave_size}}")
    if ! deploy_wave "${first_wave[@]}"; then
        log_error "First wave failed. Initiating rollback..."
        for node in "${DEPLOYED_NODES[@]}"; do
            local prev_ver="unknown"
            for entry in "${PREVIOUS_VERSIONS[@]}"; do
                if [[ "$entry" == "${node}:"* ]]; then
                    prev_ver="${entry#*:}"
                    break
                fi
            done
            rollback_node "$node" "$prev_ver"
        done
        die "Deployment failed and rolled back" 3
    fi

    # Wait for first wave to stabilize
    if [[ "$AUTO_PROMOTE" == "true" ]]; then
        log_info "Waiting ${TIMEOUT}s for first wave to stabilize..."
        sleep "$TIMEOUT"
    else
        echo ""
        read -rp "First wave deployed. Proceed to next wave? [y/N] " proceed
        if [[ "$proceed" != "y" && "$proceed" != "Y" ]]; then
            log_info "Deployment paused at wave 1. ${#DEPLOYED_NODES[@]} node(s) updated."
            exit 0
        fi
    fi

    # Subsequent waves
    local remaining_start=$first_wave_size
    while [[ $remaining_start -lt $total_nodes ]]; do
        local -a wave_nodes=()
        local i
        for ((i = 0; i < WAVE_SIZE && remaining_start + i < total_nodes; i++)); do
            wave_nodes+=("${ALL_NODES[remaining_start + i]}")
        done

        if ! deploy_wave "${wave_nodes[@]}"; then
            log_error "Wave ${WAVE_NUMBER} failed. Initiating rollback..."
            for node in "${DEPLOYED_NODES[@]}"; do
                local prev_ver="unknown"
                for entry in "${PREVIOUS_VERSIONS[@]}"; do
                    if [[ "$entry" == "${node}:"* ]]; then
                        prev_ver="${entry#*:}"
                        break
                    fi
                done
                rollback_node "$node" "$prev_ver"
            done
            die "Deployment failed and rolled back" 3
        fi

        remaining_start=$((remaining_start + WAVE_SIZE))

        # Wait between waves
        if [[ $remaining_start -lt $total_nodes ]]; then
            if [[ "$AUTO_PROMOTE" == "true" ]]; then
                log_info "Waiting ${TIMEOUT}s before next wave..."
                sleep "$TIMEOUT"
            else
                echo ""
                read -rp "Wave ${WAVE_NUMBER} complete. Proceed to next wave? [y/N] " proceed
                if [[ "$proceed" != "y" && "$proceed" != "Y" ]]; then
                    log_info "Deployment paused at wave ${WAVE_NUMBER}. ${#DEPLOYED_NODES[@]} node(s) updated."
                    exit 0
                fi
            fi
        fi
    done

    # Summary
    echo ""
    echo "=============================================="
    log_success "Canary deployment complete!"
    echo "=============================================="
    echo ""
    log_info "Summary:"
    log_info "  Total waves: ${WAVE_NUMBER}"
    log_info "  Nodes deployed: ${#DEPLOYED_NODES[@]}"
    log_info "  Nodes failed: ${#FAILED_NODES[@]}"
    log_info "  Image: ${IMAGE}"
    echo ""

    if [[ ${#FAILED_NODES[@]} -gt 0 ]]; then
        log_warn "Failed nodes: ${FAILED_NODES[*]}"
    fi

    exit 0
}

# Run
main
