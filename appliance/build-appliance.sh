#!/bin/bash
# HiveStack Appliance Build Script
# Run this on a SLES 15 SP7 build host with root privileges

set -euo pipefail

# Configuration
APP_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT_DIR="/tmp/hivestack-build"
VERSION="1.0.0"
BUILD_DATE=$(date +%Y%m%d)
ISO_NAME="hivestack-appliance-${VERSION}-${BUILD_DATE}-x86_64.iso"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() { echo -e "${GREEN}[INFO]${NC} $*"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }

# Check root
if [ "$EUID" -ne 0 ]; then
    log_error "This script must be run as root"
    exit 1
fi

# Check SLES version
if [ -f /etc/os-release ]; then
    source /etc/os-release
    log_info "Building on: $PRETTY_NAME"
    if [[ ! "$VERSION_ID" =~ ^15\. ]]; then
        log_warn "This appliance is designed for SLES 15 SP7. Current: $VERSION_ID"
    fi
fi

# Check disk space
AVAILABLE=$(df -BG "$OUT_DIR" 2>/dev/null | tail -1 | awk '{print $4}' | sed 's/G//' || echo "0")
if [ "$AVAILABLE" -lt 20 ]; then
    log_error "Need at least 20GB free space in $OUT_DIR (have ${AVAILABLE}GB)"
    exit 1
fi

log_info "HiveStack Appliance Build"
log_info "Version: $VERSION"
log_info "Build Date: $BUILD_DATE"
log_info "Source: $APP_DIR"
log_info "Output: $OUT_DIR"
log_info "ISO Name: $ISO_NAME"
echo ""

# Install kiwi-ng if needed
if ! command -v kiwi-ng &> /dev/null; then
    log_info "Installing kiwi-ng..."
    zypper -n refresh
    zypper -n install kiwi-ng python3-kiwi
fi

# Clean previous build
log_info "Cleaning previous build..."
rm -rf "$OUT_DIR"
mkdir -p "$OUT_DIR"

# Build the appliance
log_info "Starting KIWI build..."
cd "$APP_DIR"

kiwi-ng --type iso \
    --description . \
    --target-dir "$OUT_DIR" \
    --profile Standard \
    --allow-existing-root \
    build

# Check result
ISO_FILE=$(find "$OUT_DIR" -name "*.iso" -type f | head -1)
if [ -z "$ISO_FILE" ]; then
    log_error "Build failed - no ISO found in $OUT_DIR"
    exit 1
fi

# Rename to standard name
FINAL_ISO="$OUT_DIR/$ISO_NAME"
mv "$ISO_FILE" "$FINAL_ISO"

# Generate checksums
log_info "Generating checksums..."
sha256sum "$FINAL_ISO" > "$FINAL_ISO.sha256"
md5sum "$FINAL_ISO" > "$FINAL_ISO.md5"

# Show results
log_info "Build completed successfully!"
echo ""
echo "=========================================="
echo "HiveStack Appliance ISO Ready"
echo "=========================================="
echo "ISO:      $FINAL_ISO"
echo "SHA256:   $(cat $FINAL_ISO.sha256)"
echo "MD5:      $(cat $FINAL_ISO.md5)"
echo "Size:     $(du -h $FINAL_ISO | cut -f1)"
echo ""
echo "To test in a VM:"
echo "  qemu-system-x86_64 -m 4G -cdrom $FINAL_ISO -boot d"
echo ""
echo "To install on hardware:"
echo "  1. Write to USB: dd if=$FINAL_ISO of=/dev/sdX bs=4M status=progress"
echo "  2. Boot from USB and follow installer"
echo ""

# Verify ISO is bootable
if command -v xorriso &> /dev/null; then
    log_info "Verifying ISO bootability..."
    xorriso -indev "$FINAL_ISO" -report_el_torito as_mkisofs 2>/dev/null | head -20
fi