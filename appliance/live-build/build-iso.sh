#!/bin/bash
# HiveStack Appliance ISO Builder
# Uses live-build to create a bootable HiveStack appliance ISO
#
# Prerequisites:
#   - live-build (apt install live-build)
#   - squashfs-tools
#   - xorriso
#   - grub-efi-amd64
#
# Usage:
#   cd appliance/live-build
#   sudo ./build-iso.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUILD_DIR="${SCRIPT_DIR}/build"
OUTPUT_DIR="${SCRIPT_DIR}/output"
VERSION="${HIVE_VERSION:-1.0.0}"
BUILD_DATE=$(date +%Y%m%d)
ISO_NAME="hivestack-appliance-${VERSION}-${BUILD_DATE}-amd64.iso"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $*"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }

# Check root
if [ "$EUID" -ne 0 ]; then
    log_error "This script must be run as root (use sudo)"
    exit 1
fi

# Check dependencies
for cmd in lb xorriso grub-mkrescue mksquashfs; do
    if ! command -v "$cmd" &>/dev/null; then
        log_error "Missing dependency: $cmd"
        log_error "Install with: apt install live-build xorriso grub-efi-amd64-bin squashfs-tools"
        exit 1
    fi
done

# Check disk space
AVAILABLE=$(df -BG "$SCRIPT_DIR" | tail -1 | awk '{print $4}' | sed 's/G//')
if [ "$AVAILABLE" -lt 20 ]; then
    log_error "Need at least 20GB free space (have ${AVAILABLE}GB)"
    exit 1
fi

log_info "Building HiveStack Appliance ISO"
log_info "Version: $VERSION"
log_info "Build Date: $BUILD_DATE"
log_info "Output: $OUTPUT_DIR"

# Clean previous build
rm -rf "$BUILD_DIR" "$OUTPUT_DIR"
mkdir -p "$BUILD_DIR" "$OUTPUT_DIR"

# Configure live-build
cd "$BUILD_DIR"
SCRIPT_DIR="${SCRIPT_DIR}" HIVE_VERSION="${VERSION}" lb config

# Copy hooks and includes
if [ -d "$SCRIPT_DIR/config/hooks" ]; then
    cp -a "$SCRIPT_DIR/config/hooks/"* "$BUILD_DIR/config/hooks/" 2>/dev/null || true
fi

if [ -d "$SCRIPT_DIR/config/includes.chroot" ]; then
    cp -a "$SCRIPT_DIR/config/includes.chroot/"* "$BUILD_DIR/config/includes.chroot/" 2>/dev/null || true
fi

# Build
log_info "Starting live-build (this may take 20-40 minutes)..."
lb build 2>&1 | tee "$OUTPUT_DIR/build.log"

# Check result
if [ -f "$BUILD_DIR/live-image-amd64.hybrid.iso" ]; then
    mv "$BUILD_DIR/live-image-amd64.hybrid.iso" "$OUTPUT_DIR/$ISO_NAME"
    log_info "ISO built successfully!"
    log_info "Output: $OUTPUT_DIR/$ISO_NAME"
    log_info "Size: $(du -h "$OUTPUT_DIR/$ISO_NAME" | cut -f1)"
    log_info ""
    log_info "To test in QEMU:"
    log_info "  qemu-system-x86_64 -m 4G -cdrom $OUTPUT_DIR/$ISO_NAME -boot d"
    log_info ""
    log_info "To write to USB:"
    log_info "  sudo dd if=$OUTPUT_DIR/$ISO_NAME of=/dev/sdX bs=4M status=progress"
else
    log_error "Build failed! Check $OUTPUT_DIR/build.log for details."
    exit 1
fi
