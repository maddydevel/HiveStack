#!/usr/bin/env bash
# install-systemd.sh — Validate and install HiveStack systemd service files
#
# This script:
#   1. Validates service file syntax (systemd-analyze if available)
#   2. Installs service files with proper permissions (0644)
#   3. Reloads systemd daemon
#   4. Enables and starts services (optional)
#
# Usage:
#   sudo ./scripts/install-systemd.sh [options]
#
# Options:
#   --services-dir <dir>    Directory containing .service files (default: ./pkg/systemd)
#   --no-enable             Don't enable services (only install)
#   --no-start              Don't start services (only install/enable)
#   --dry-run               Show what would be done without making changes
#   -h, --help              Show this help message
#
# Exit codes:
#   0 - Success
#   1 - General failure
#   2 - Invalid arguments
#   3 - Validation failure

set -euo pipefail

# ============================================================================
# Configuration
# ============================================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Defaults
SERVICES_DIR="${PROJECT_ROOT}/pkg/systemd"
SYSTEMD_DIR="/etc/systemd/system"
ENABLE_SERVICES=true
START_SERVICES=true
DRY_RUN=false

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# State
INSTALLED_FILES=()
VALIDATED_FILES=()
FAILED_FILES=()

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

# Check if running as root
check_root() {
    if [[ $EUID -ne 0 ]]; then
        die "This script must be run as root (use sudo)" 1
    fi
}

# Validate a single service file
validate_service_file() {
    local service_file="$1"
    local filename
    filename=$(basename "$service_file")

    log_info "Validating: ${filename}"

    # Check file exists and is readable
    if [[ ! -f "$service_file" ]]; then
        log_error "File not found: ${service_file}"
        FAILED_FILES+=("$filename")
        return 1
    fi

    if [[ ! -r "$service_file" ]]; then
        log_error "File not readable: ${service_file}"
        FAILED_FILES+=("$filename")
        return 1
    fi

    # Check for required systemd sections
    if ! grep -q '^\[Unit\]' "$service_file"; then
        log_error "Missing [Unit] section in ${filename}"
        FAILED_FILES+=("$filename")
        return 1
    fi

    if ! grep -q '^\[Service\]' "$service_file"; then
        log_error "Missing [Service] section in ${filename}"
        FAILED_FILES+=("$filename")
        return 1
    fi

    if ! grep -q '^\[Install\]' "$service_file"; then
        log_error "Missing [Install] section in ${filename}"
        FAILED_FILES+=("$filename")
        return 1
    fi

    # Check for ExecStart directive
    if ! grep -q '^ExecStart=' "$service_file"; then
        log_error "Missing ExecStart directive in ${filename}"
        FAILED_FILES+=("$filename")
        return 1
    fi

    # Check file permissions (should be 0644)
    local perms
    perms=$(stat -c '%a' "$service_file" 2>/dev/null || stat -f '%Lp' "$service_file" 2>/dev/null)
    if [[ "$perms" != "644" ]]; then
        log_warn "File permissions are ${perms}, expected 644 (will fix on install)"
    fi

    # Validate with systemd-analyze if available
    if command -v systemd-analyze &>/dev/null; then
        log_info "Running systemd-analyze verify on ${filename}..."
        if systemd-analyze verify "$service_file" 2>&1; then
            log_success "systemd-analyze validation passed for ${filename}"
        else
            # Some warnings are acceptable (e.g., paths that don't exist yet)
            local exit_code=$?
            if [[ $exit_code -eq 1 ]]; then
                log_error "systemd-analyze validation failed for ${filename}"
                FAILED_FILES+=("$filename")
                return 1
            fi
        fi
    else
        log_warn "systemd-analyze not available, skipping deep validation"
    fi

    # Check for security best practices
    local security_score=0

    if grep -q 'ProtectSystem=strict' "$service_file"; then
        security_score=$((security_score + 1))
    else
        log_warn "${filename}: Missing ProtectSystem=strict"
    fi

    if grep -q 'ProtectHome=yes' "$service_file"; then
        security_score=$((security_score + 1))
    else
        log_warn "${filename}: Missing ProtectHome=yes"
    fi

    if grep -q 'NoNewPrivileges=yes' "$service_file"; then
        security_score=$((security_score + 1))
    else
        log_warn "${filename}: Missing NoNewPrivileges=yes"
    fi

    if grep -q 'CapabilityBoundingSet=' "$service_file"; then
        security_score=$((security_score + 1))
    else
        log_warn "${filename}: Missing CapabilityBoundingSet"
    fi

    if [[ $security_score -ge 3 ]]; then
        log_success "Security hardening: ${security_score}/4 checks passed for ${filename}"
    else
        log_warn "Security hardening: only ${security_score}/4 checks passed for ${filename}"
    fi

    VALIDATED_FILES+=("$filename")
    return 0
}

# Install a single service file
install_service_file() {
    local service_file="$1"
    local filename
    filename=$(basename "$service_file")
    local dest="${SYSTEMD_DIR}/${filename}"

    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "[DRY RUN] Would install ${filename} to ${dest} with permissions 0644"
        INSTALLED_FILES+=("$filename")
        return 0
    fi

    # Copy with correct permissions
    install -m 0644 -o root -g root "$service_file" "$dest"
    log_success "Installed ${filename} → ${dest} (0644)"
    INSTALLED_FILES+=("$filename")
}

# Enable and start services
manage_services() {
    local action="$1"  # enable, start, or both
    shift
    local services=("$@")

    if [[ ${#services[@]} -eq 0 ]]; then
        return 0
    fi

    # Reload systemd daemon
    if [[ "$DRY_RUN" == "false" ]]; then
        log_info "Reloading systemd daemon..."
        systemctl daemon-reload
    else
        log_info "[DRY RUN] Would reload systemd daemon"
    fi

    for service in "${services[@]}"; do
        local service_name="${service%.service}"

        if [[ "$action" == "enable" || "$action" == "both" ]]; then
            if [[ "$ENABLE_SERVICES" == "true" ]]; then
                if [[ "$DRY_RUN" == "true" ]]; then
                    log_info "[DRY RUN] Would enable ${service_name}"
                else
                    if systemctl enable "$service_name" 2>&1; then
                        log_success "Enabled ${service_name}"
                    else
                        log_error "Failed to enable ${service_name}"
                    fi
                fi
            fi
        fi

        if [[ "$action" == "start" || "$action" == "both" ]]; then
            if [[ "$START_SERVICES" == "true" ]]; then
                if [[ "$DRY_RUN" == "true" ]]; then
                    log_info "[DRY RUN] Would start ${service_name}"
                else
                    if systemctl start "$service_name" 2>&1; then
                        log_success "Started ${service_name}"
                    else
                        log_warn "Failed to start ${service_name} (may require additional setup)"
                    fi
                fi
            fi
        fi
    done
}

# Print summary
print_summary() {
    echo ""
    echo "=============================================="
    echo "  Installation Summary"
    echo "=============================================="
    echo ""
    log_info "Validated: ${#VALIDATED_FILES[@]} file(s)"
    for f in "${VALIDATED_FILES[@]}"; do
        echo -e "    ${GREEN}✓${NC} ${f}"
    done
    echo ""
    log_info "Installed: ${#INSTALLED_FILES[@]} file(s)"
    for f in "${INSTALLED_FILES[@]}"; do
        echo -e "    ${GREEN}✓${NC} ${f}"
    done
    if [[ ${#FAILED_FILES[@]} -gt 0 ]]; then
        echo ""
        log_error "Failed: ${#FAILED_FILES[@]} file(s)"
        for f in "${FAILED_FILES[@]}"; do
            echo -e "    ${RED}✗${NC} ${f}"
        done
    fi
    echo ""
}

# ============================================================================
# Parse Arguments
# ============================================================================

while [[ $# -gt 0 ]]; do
    case $1 in
        --services-dir) SERVICES_DIR="$2"; shift 2 ;;
        --no-enable)    ENABLE_SERVICES=false; shift ;;
        --no-start)     START_SERVICES=false; shift ;;
        --dry-run)      DRY_RUN=true; shift ;;
        -h|--help)      show_help ;;
        *)              die "Unknown option: $1" 2 ;;
    esac
done

# ============================================================================
# Main
# ============================================================================

main() {
    echo ""
    echo "=============================================="
    echo "  HiveStack systemd Service Installer"
    echo "=============================================="
    echo ""

    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "DRY RUN MODE — no changes will be made"
        echo ""
    else
        check_root
    fi

    # Check services directory exists
    if [[ ! -d "$SERVICES_DIR" ]]; then
        die "Services directory not found: ${SERVICES_DIR}" 1
    fi

    # Find all .service files
    local service_files=()
    while IFS= read -r -d '' file; do
        service_files+=("$file")
    done < <(find "$SERVICES_DIR" -maxdepth 1 -name '*.service' -print0 2>/dev/null | sort -z)

    if [[ ${#service_files[@]} -eq 0 ]]; then
        die "No .service files found in ${SERVICES_DIR}" 1
    fi

    log_info "Found ${#service_files[@]} service file(s) in ${SERVICES_DIR}"
    echo ""

    # Validate all service files first
    local validation_failed=false
    for service_file in "${service_files[@]}"; do
        if ! validate_service_file "$service_file"; then
            validation_failed=true
        fi
        echo ""
    done

    if [[ "$validation_failed" == "true" ]]; then
        die "Validation failed. Fix service file errors before installing." 3
    fi

    log_success "All service files validated successfully"
    echo ""

    # Install service files
    for service_file in "${service_files[@]}"; do
        install_service_file "$service_file"
    done

    echo ""

    # Enable and start services
    if [[ "$ENABLE_SERVICES" == "true" || "$START_SERVICES" == "true" ]]; then
        local action="both"
        if [[ "$ENABLE_SERVICES" == "false" ]]; then
            action="start"
        elif [[ "$START_SERVICES" == "false" ]]; then
            action="enable"
        fi

        local service_names=()
        for service_file in "${service_files[@]}"; do
            service_names+=("$(basename "$service_file")")
        done

        manage_services "$action" "${service_names[@]}"
    fi

    echo ""
    print_summary

    if [[ ${#FAILED_FILES[@]} -gt 0 ]]; then
        exit 1
    fi

    exit 0
}

main
