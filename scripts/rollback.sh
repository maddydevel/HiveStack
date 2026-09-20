#!/usr/bin/env bash
# rollback.sh — Rollback HiveStack to a previous git tag
#
# This script:
#   1. Lists available tags for rollback
#   2. Checks out the specified previous tag
#   3. Re-deploys using the rollback tag
#   4. Verifies health after rollback
#
# Usage:
#   ./scripts/rollback.sh [options] [tag]
#
# Options:
#   --tag <tag>          Specific tag to rollback to (default: previous tag)
#   --list               List available tags and exit
#   --force              Skip confirmation prompt
#   --no-health-check    Skip post-rollback health verification
#   --health-timeout <s> Timeout for health check in seconds (default: 300)
#   --deploy-cmd <cmd>   Custom deployment command (default: docker compose)
#   --dry-run            Show what would be done without executing
#   -h, --help           Show this help message
#
# Exit codes:
#   0 - Success
#   1 - General failure
#   2 - Invalid arguments
#   3 - Health check failed

set -euo pipefail

# ============================================================================
# Configuration
# ============================================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Defaults
TARGET_TAG=""
LIST_TAGS=false
FORCE=false
HEALTH_CHECK=true
HEALTH_TIMEOUT=300
DEPLOY_CMD=""
DRY_RUN=false

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
        --tag)              TARGET_TAG="$2"; shift 2 ;;
        --list)             LIST_TAGS=true; shift ;;
        --force)            FORCE=true; shift ;;
        --no-health-check) HEALTH_CHECK=false; shift ;;
        --health-timeout)   HEALTH_TIMEOUT="$2"; shift 2 ;;
        --deploy-cmd)       DEPLOY_CMD="$2"; shift 2 ;;
        --dry-run)          DRY_RUN=true; shift ;;
        -h|--help)          show_help ;;
        v*)                 TARGET_TAG="$1"; shift ;;
        *)                  die "Unknown option: $1" 2 ;;
    esac
done

# ============================================================================
# Git Tag Functions
# ============================================================================

get_current_tag() {
    git -C "$PROJECT_ROOT" describe --tags --exact-match HEAD 2>/dev/null || echo "HEAD"
}

list_tags() {
    log_info "Available tags (most recent first):"
    echo ""
    git -C "$PROJECT_ROOT" tag --sort=-v:refname --format='%(refname:short) %(creatordate:short) %(objectname:short)' | \
        head -20 | \
        while read -r tag date hash; do
            local current=""
            if [[ "$(get_current_tag)" == "$tag" ]]; then
                current=" ${GREEN}<-- current${NC}"
            fi
            echo -e "  ${BLUE}${tag}${NC}  ${date}  ${hash}${current}"
        done
    echo ""
}

get_previous_tag() {
    local current_tag="$1"
    git -C "$PROJECT_ROOT" tag --sort=-v:refname | grep -A1 "^${current_tag}$" | tail -1
}

# ============================================================================
# Health Check Functions
# ============================================================================

verify_health() {
    if [[ "$HEALTH_CHECK" == "false" ]]; then
        log_warn "Health check disabled, skipping verification"
        return 0
    fi

    log_info "Verifying health after rollback (timeout: ${HEALTH_TIMEOUT}s)..."

    local health_url="${HEALTH_URL:-https://localhost:8443/api/health}"
    local elapsed=0
    local interval=10

    while [[ $elapsed -lt $HEALTH_TIMEOUT ]]; do
        local http_code
        http_code=$(curl -sk -o /dev/null -w "%{http_code}" --max-time 5 "$health_url" 2>/dev/null || echo "000")

        if [[ "$http_code" == "200" ]]; then
            log_success "Health check passed (HTTP 200)"
            return 0
        fi

        log_info "Waiting for service to be healthy... (${elapsed}s elapsed)"
        sleep "$interval"
        elapsed=$((elapsed + interval))
    done

    log_error "Health check failed after ${HEALTH_TIMEOUT}s"
    log_error "Service at ${health_url} did not return HTTP 200"
    return 1
}

# ============================================================================
# Rollback Functions
# ============================================================================

perform_rollback() {
    local from_tag="$1"
    local to_tag="$2"

    echo ""
    echo "=============================================="
    log_warn "ROLLBACK: ${from_tag} → ${to_tag}"
    echo "=============================================="
    echo ""

    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "[DRY RUN] Would checkout tag: ${to_tag}"
        log_info "[DRY RUN] Would redeploy using: ${to_tag}"
        log_info "[DRY RUN] Would verify health"
        return 0
    fi

    # Checkout the target tag
    log_info "Checking out tag: ${to_tag}..."
    if ! git -C "$PROJECT_ROOT" checkout "$to_tag" 2>&1; then
        die "Failed to checkout tag: ${to_tag}" 1
    fi
    log_success "Checked out ${to_tag}"

    # Redeploy
    if [[ -n "$DEPLOY_CMD" ]]; then
        log_info "Redeploying with custom command: ${DEPLOY_CMD}"
        if ! eval "$DEPLOY_CMD" 2>&1; then
            die "Deployment command failed" 1
        fi
    elif [[ -f "${PROJECT_ROOT}/appliance/docker-compose.yaml" ]]; then
        log_info "Redeploying with docker compose..."
        if ! (cd "$PROJECT_ROOT" && docker compose -f appliance/docker-compose.yaml up -d --build 2>&1); then
            die "Docker compose deployment failed" 1
        fi
    else
        log_warn "No deployment method configured. Redeployment must be done manually."
    fi

    log_success "Redeployment complete"

    # Verify health
    if ! verify_health; then
        die "Rollback health check failed! Manual intervention required." 3
    fi

    echo ""
    log_success "Rollback to ${to_tag} completed successfully"
}

# ============================================================================
# Main
# ============================================================================

main() {
    echo ""
    echo "=============================================="
    echo "  HiveStack Rollback Tool"
    echo "=============================================="
    echo ""

    # Verify we're in a git repository
    if ! git -C "$PROJECT_ROOT" rev-parse --git-dir &>/dev/null; then
        die "Not in a git repository: ${PROJECT_ROOT}" 1
    fi

    # List tags if requested
    if [[ "$LIST_TAGS" == "true" ]]; then
        list_tags
        exit 0
    fi

    # Get current tag
    local current_tag
    current_tag=$(get_current_tag)
    log_info "Current version: ${current_tag}"

    # Determine target tag
    if [[ -z "$TARGET_TAG" ]]; then
        # Find previous tag
        TARGET_TAG=$(get_previous_tag "$current_tag")
        if [[ -z "$TARGET_TAG" ]]; then
            die "No previous tag found. Use --tag to specify a target." 1
        fi
        log_info "Rollback target: ${TARGET_TAG} (auto-detected)"
    else
        # Verify specified tag exists
        if ! git -C "$PROJECT_ROOT" rev-parse "$TARGET_TAG" &>/dev/null; then
            die "Tag not found: ${TARGET_TAG}" 1
        fi
        log_info "Rollback target: ${TARGET_TAG}"
    fi

    # Don't rollback to the same tag
    if [[ "$current_tag" == "$TARGET_TAG" ]]; then
        die "Already at tag ${TARGET_TAG}. Nothing to rollback." 2
    fi

    # Show available tags
    echo ""
    log_info "Available rollback targets:"
    echo ""
    git -C "$PROJECT_ROOT" tag --sort=-v:refname | head -5 | while read -r tag; do
        local marker=""
        [[ "$tag" == "$TARGET_TAG" ]] && marker=" ${GREEN}<-- selected${NC}"
        echo -e "    ${tag}${marker}"
    done

    # Confirm rollback
    echo ""
    if [[ "$FORCE" == "false" && "$DRY_RUN" == "false" ]]; then
        read -rp "Proceed with rollback? [y/N] " confirm
        if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
            log_info "Rollback cancelled"
            exit 0
        fi
    fi

    # Perform rollback
    perform_rollback "$current_tag" "$TARGET_TAG"
}

main
