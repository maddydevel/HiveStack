#!/bin/bash
# HiveStack SLES 15 SP7 Appliance Builder using KIWI
#
# This script builds a bootable ISO using kiwi-ng on SLES 15 SP7.
#
# PREREQUISITES:
#   - SLES 15 SP7 host or Docker with SLES 15 SP7 image
#   - kiwi-ng installed (zypper install kiwi-ng)
#   - SLES 15 SP7 ISO (sles-15-sp7-x86_64.iso)
#   - Root/sudo access
#
# USAGE:
#   sudo ./appliance/kiwi/build.sh
#
# OUTPUT:
#   /tmp/kiwi-build/hivestack-appliance-sles15sp7.iso

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
BUILD_DIR="/tmp/kiwi-build"
OUTPUT_DIR="${BUILD_DIR}/output"

echo "========================================="
echo "  HiveStack SLES 15 SP7 Appliance Builder"
echo "========================================="
echo ""

# Check prerequisites
if ! command -v kiwi-ng &>/dev/null; then
    echo "ERROR: kiwi-ng not found"
    echo "Install with: zypper install kiwi-ng"
    exit 1
fi

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "ERROR: This script must be run as root"
    echo "Usage: sudo $0"
    exit 1
fi

# Clean previous build
rm -rf "${BUILD_DIR}"
mkdir -p "${BUILD_DIR}" "${OUTPUT_DIR}"

# Copy kiwi config to build directory
cp -r "${SCRIPT_DIR}" "${BUILD_DIR}/kiwi"

# Copy HiveStack binaries to overlay
mkdir -p "${BUILD_DIR}/kiwi/root/opt/hiveestack/bin"
cp "${PROJECT_ROOT}/dist/hive-manager" "${BUILD_DIR}/kiwi/root/opt/hivestack/bin/"
cp "${PROJECT_ROOT}/dist/hive-node" "${BUILD_DIR}/kiwi/root/opt/hivestack/bin/"
cp "${PROJECT_ROOT}/dist/hive" "${BUILD_DIR}/kiwi/root/opt/hivestack/bin/"

# Copy migrations
mkdir -p "${BUILD_DIR}/kiwi/root/opt/hivestack/migrations"
cp -r "${PROJECT_ROOT}/internal/db/migrations/"*.sql "${BUILD_DIR}/kiwi/root/opt/hivestack/migrations/"

# Copy systemd services
mkdir -p "${BUILD_DIR}/kiwi/root/usr/lib/systemd/system"
cp "${SCRIPT_DIR}/root/usr/lib/systemd/system/"*.service "${BUILD_DIR}/kiwi/root/usr/lib/systemd/system/"

# Copy first-boot script
mkdir -p "${BUILD_DIR}/kiwi/root/opt/hivestack/scripts"
cp "${SCRIPT_DIR}/root/opt/hivestack/scripts/first-boot.sh" "${BUILD_DIR}/kiwi/root/opt/hivestack/scripts/"

# Build ISO with kiwi-ng
echo "Building ISO with kiwi-ng..."
echo "Build directory: ${BUILD_DIR}"
echo ""

cd "${BUILD_DIR}"
kiwi-ng --type iso \
    --description kiwi/ \
    --target-dir "${OUTPUT_DIR}" \
    build

# Check result
ISO_FILE=$(find "${OUTPUT_DIR}" -name "*.iso" -type f | head -1)

if [ -z "$ISO_FILE" ]; then
    echo "ERROR: Build failed - no ISO found"
    exit 1
fi

# Generate checksums
cd "${OUTPUT_DIR}"
sha256sum "$(basename "$ISO_FILE")" > "${ISO_FILE}.sha256"

echo ""
echo "================================================"
echo "  Build Complete!"
echo "================================================"
echo "  ISO:      ${ISO_FILE}"
echo "  Size:     $(du -h "$ISO_FILE" | cut -f1)"
echo "  SHA256:   $(cat ${ISO_FILE}.sha256)"
echo ""
echo "  Test in QEMU:"
echo "    qemu-system-x86_64 -m 4G -cdrom ${ISO_FILE} -boot d"
echo ""
echo "  Write to USB:"
echo "    sudo dd if=${ISO_FILE} of=/dev/sdX bs=4M status=progress"
echo "================================================"
