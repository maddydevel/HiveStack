#!/bin/bash
# HiveStack Appliance ISO Builder — SLES 15 SP7
# Creates a bootable ISO with HiveStack pre-installed on SLES 15 SP7
#
# IMPORTANT: This script MUST run on SLES 15 SP7 or openSUSE Leap 15.5+
# SAP HANA requires SLES for KVM certification — Ubuntu is NOT supported.
#
# Prerequisites:
#   - SLES 15 SP7 host (registered with SUSE)
#   - kiwi-ng (zypper install kiwi-ng python3-kiwi)
#   - Root access
#   - 20GB free disk space
#
# Usage:
#   sudo ./appliance/scripts/build-iso.sh
#
# Output:
#   build/iso/output/hivestack-appliance-<version>-<date>-x86_64.iso

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
BUILD_DIR="${PROJECT_ROOT}/build/iso"
OUTPUT_DIR="${BUILD_DIR}/output"
VERSION="${HIVESTACK_VERSION:-1.0.0}"
BUILD_DATE="$(date +%Y%m%d)"
ISO_NAME="hivestack-appliance-${VERSION}-${BUILD_DATE}-x86_64.iso"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info()  { echo -e "${GREEN}[INFO]${NC}  $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }
log_step()  { echo -e "${BLUE}[STEP]${NC}  $*"; }

check_root() {
    if [ "$EUID" -ne 0 ]; then
        log_error "This script must be run as root"
        exit 1
    fi
}

check_sles() {
    if [ -f /etc/os-release ]; then
        source /etc/os-release
        log_info "Building on: $PRETTY_NAME"
        if [[ ! "$VERSION_ID" =~ ^15\. ]]; then
            log_error "This appliance requires SLES 15 SP7"
            log_error "Current: $PRETTY_NAME"
            exit 1
        fi
    fi
}

check_dependencies() {
    local missing=()
    for cmd in kiwi-ng; do
        if ! command -v "$cmd" &>/dev/null; then
            missing+=("$cmd")
        fi
    done
    if [ ${#missing[@]} -gt 0 ]; then
        log_error "Missing: ${missing[*]}"
        log_error "Install with: zypper install kiwi-ng python3-kiwi"
        exit 1
    fi
}

build_binaries() {
    log_step "Building HiveStack binaries..."
    cd "${PROJECT_ROOT}"
    if ! command -v go &>/dev/null; then
        log_error "Go not found. Install Go 1.23+ first."
        exit 1
    fi
    mkdir -p "${PROJECT_ROOT}/dist"
    CGO_ENABLED=0 go build -ldflags="-w -s" -o "${PROJECT_ROOT}/dist/hive-manager" ./cmd/hive-manager
    CGO_ENABLED=0 go build -ldflags="-w -s" -o "${PROJECT_ROOT}/dist/hive-node" ./node/
    CGO_ENABLED=0 go build -ldflags="-w -s" -o "${PROJECT_ROOT}/dist/hive" ./cmd/hive
    log_info "Binaries built."
}

prepare_kiwi_overlay() {
    log_step "Preparing KIWI overlay with HiveStack..."
    
    local overlay_dir="${PROJECT_ROOT}/appliance/overlay"
    
    mkdir -p "${overlay_dir}/opt/hivestack/"{bin,config,certs,logs,migrations,scripts}
    mkdir -p "${overlay_dir}/etc/hivestack"
    
    cp "${PROJECT_ROOT}/dist/hive-manager" "${overlay_dir}/opt/hivestack/bin/"
    cp "${PROJECT_ROOT}/dist/hive-node" "${overlay_dir}/opt/hivestack/bin/"
    cp "${PROJECT_ROOT}/dist/hive" "${overlay_dir}/opt/hivestack/bin/"
    chmod +x "${overlay_dir}/opt/hivestack/bin/"*
    
    if [ -d "${PROJECT_ROOT}/internal/db/migrations" ]; then
        cp "${PROJECT_ROOT}/internal/db/migrations/"*.sql "${overlay_dir}/opt/hivestack/migrations/"
    fi
    
    cp "${SCRIPT_DIR}/first-boot.sh" "${overlay_dir}/opt/hivestack/scripts/"
    chmod +x "${overlay_dir}/opt/hivestack/scripts/first-boot.sh"
    
    mkdir -p "${overlay_dir}/etc/systemd/system"
    cat > "${overlay_dir}/etc/systemd/system/hivestack-manager.service" <<'SVCEOF'
[Unit]
Description=HiveStack Manager
After=network.target postgresql.service
Wants=postgresql.service

[Service]
Type=simple
User=hivestack
Group=hivestack
ExecStart=/opt/hivestack/bin/hive-manager --config /etc/hivestack/manager.yaml
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
SVCEOF

    cat > "${overlay_dir}/etc/systemd/system/hivestack-node.service" <<'SVCEOF'
[Unit]
Description=HiveStack Node Agent
After=network.target libvirtd.service
Wants=libvirtd.service

[Service]
Type=simple
ExecStart=/opt/hivestack/bin/hive-node --config /etc/hivestack/node.yaml
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
SVCEOF

    cat > "${overlay_dir}/etc/systemd/system/hivestack-first-boot.service" <<'SVCEOF'
[Unit]
Description=HiveStack First Boot Setup
After=network.target postgresql.service
Before=hivestack-manager.service
ConditionPathExists=!/etc/hivestack/.first-boot-complete

[Service]
Type=oneshot
ExecStart=/opt/hivestack/scripts/first-boot.sh
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
SVCEOF

    cat > "${overlay_dir}/etc/hivestack/manager.yaml.example" <<'CFGEOF'
dsn: "postgres://hivestack:hivestack@localhost:5432/hivestack?sslmode=disable"
jwt_secret: "change-me-use-random-32-chars"
tls_cert_file: "/etc/hivestack/certs/manager.crt"
tls_key_file: "/etc/hivestack/certs/manager.key"
tls_ca_file: "/etc/hivestack/certs/ca.crt"
listen_address: ":8080"
listen_tls_address: ":8443"
grpc_port: 44567
log_level: "info"
log_format: "json"
CFGEOF

    cat > "${overlay_dir}/etc/hivestack/node.yaml.example" <<'CFGEOF'
node_id: "node-001"
manager_address: "localhost:44567"
tls_cert_file: "/etc/hivestack/certs/node-server.crt"
tls_key_file: "/etc/hivestack/certs/node-server.key"
tls_ca_file: "/etc/hivestack/certs/ca.crt"
heartbeat_interval: 30
log_level: "info"
log_format: "json"
CFGEOF

    log_info "Overlay prepared."
}

build_iso() {
    log_step "Building ISO with KIWI..."
    
    cd "${PROJECT_ROOT}/appliance"
    
    rm -rf "${BUILD_DIR}"
    mkdir -p "${BUILD_DIR}" "${OUTPUT_DIR}"
    
    kiwi-ng --type iso \
        --description . \
        --target-dir "${BUILD_DIR}" \
        --profile Standard \
        --allow-existing-root \
        build
    
    local iso_file
    iso_file=$(find "${BUILD_DIR}" -name "*.iso" -type f | head -1)
    
    if [ -z "$iso_file" ]; then
        log_error "Build failed - no ISO found"
        exit 1
    fi
    
    mv "$iso_file" "${OUTPUT_DIR}/${ISO_NAME}"
    
    cd "${OUTPUT_DIR}"
    sha256sum "${ISO_NAME}" > "${ISO_NAME}.sha256"
    md5sum "${ISO_NAME}" > "${ISO_NAME}.md5"
    
    log_info "ISO built successfully!"
}

print_results() {
    echo ""
    echo "================================================"
    echo "  HiveStack Appliance ISO Built Successfully!"
    echo "  Base: SLES 15 SP7 (SAP HANA Certified)"
    echo "================================================"
    echo ""
    echo "  ISO:      ${OUTPUT_DIR}/${ISO_NAME}"
    echo "  Size:     $(du -h "${OUTPUT_DIR}/${ISO_NAME}" | cut -f1)"
    echo "  SHA256:   $(cat "${OUTPUT_DIR}/${ISO_NAME}.sha256" | cut -d' ' -f1)"
    echo ""
    echo "  Test in QEMU:"
    echo "    qemu-system-x86_64 -m 4G -cdrom ${OUTPUT_DIR}/${ISO_NAME} -boot d"
    echo ""
    echo "  Write to USB:"
    echo "    sudo dd if=${OUTPUT_DIR}/${ISO_NAME} of=/dev/sdX bs=4M status=progress"
    echo ""
    echo "================================================"
}

main() {
    echo ""
    echo "================================================"
    echo "  HiveStack Appliance ISO Builder"
    echo "  Base OS: SLES 15 SP7"
    echo "  Version: ${VERSION}"
    echo "  Date:    ${BUILD_DATE}"
    echo "================================================"
    echo ""
    
    check_root
    check_sles
    check_dependencies
    build_binaries
    prepare_kiwi_overlay
    build_iso
    print_results
}

main "$@"
