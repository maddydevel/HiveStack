#!/bin/bash
# HiveStack Demo ISO Builder
# Creates a bootable ISO with grub-mkrescue + xorriso
#
# This is a DEMO/EVALUATION ISO for Ubuntu hosts.
# For production SAP HANA workloads, use the SLES 15 SP7 build.
#
# Prerequisites: genisoimage, mksquashfs, grub-efi-amd64-signed, xorriso
#
# Usage: ./appliance/scripts/build-demo-iso.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
VERSION="${HIVESTACK_VERSION:-1.0.0}"
ISO_NAME="hivestack-demo-${VERSION}-amd64.iso"

echo "=== HiveStack Demo ISO Builder ==="

# Build binaries
cd "${PROJECT_ROOT}"
mkdir -p dist
go build -o dist/hive-manager ./cmd/hive-manager
go build -o dist/hive-node ./node/
go build -o dist/hive ./cmd/hive

# Create initramfs with busybox + dynamic linker
echo "Creating initramfs..."
INITRD_DIR=$(mktemp -d)
trap "rm -rf ${INITRD_DIR}" EXIT

mkdir -p "${INITRD_DIR}"/{bin,etc,proc,sys,dev,tmp,root,lib,lib64,usr/bin,opt}

# Busybox for shell and utilities
cp /bin/busybox "${INITRD_DIR}/bin/sh"

# Dynamic linker (required for Go binaries which use libc on some platforms)
cp /lib/x86_64-linux-gnu/libc.so.6 "${INITRD_DIR}/lib/" 2>/dev/null || true
cp /lib64/ld-linux-x86-64.so.2 "${INITRD_DIR}/lib64/" 2>/dev/null || true

# HiveStack binaries
cp dist/hive-manager dist/hive-node dist/hive "${INITRD_DIR}/usr/bin/"

# Database migrations
mkdir -p "${INITRD_DIR}/opt"
cp -r internal/db/migrations "${INITRD_DIR}/opt/" 2>/dev/null || true

# Init script
cat > "${INITRD_DIR}/init" <<'INIT'
#!/bin/sh
echo "HiveStack Demo Appliance"
echo "======================="
echo "Binaries: /usr/bin/hive-*"
ls -la /usr/bin/hive-* 2>/dev/null || echo "No hive binaries"
exec /bin/sh
INIT
chmod +x "${INITRD_DIR}/init"

# Create initramfs image
(cd "${INITRD_DIR}" && find . | cpio -o -H newc 2>/dev/null | gzip > "${PROJECT_ROOT}/build/initramfs.img")

# Create ISO structure
echo "Creating ISO..."
ISO_DIR=$(mktemp -d)
mkdir -p "${ISO_DIR}/boot/grub"

# Kernel
cp /boot/vmlinuz-6.14.0-37-generic "${ISO_DIR}/boot/vmlinuz"
cp "${PROJECT_ROOT}/build/initramfs.img" "${ISO_DIR}/boot/initramfs.img"

# GRUB config
cat > "${ISO_DIR}/boot/grub/grub.cfg" <<'GRUBCFG'
set timeout=5
set default=0

menuentry "HiveStack Demo Appliance" {
    linux /boot/vmlinuz console=ttyS0
    initrd /boot/initramfs.img
}

menuentry "HiveStack Demo (Debug)" {
    linux /boot/vmlinuz console=ttyS0 init=/bin/sh
    initrd /boot/initramfs.img
}
GRUBCFG

# Build ISO with grub-mkrescue
# Note: Requires xorriso (apt install xorriso)
PATH="/tmp/xorriso-install/bin:${PATH}" grub-mkrescue -o "${PROJECT_ROOT}/build/${ISO_NAME}" "${ISO_DIR}/" 2>&1 | tail -3

rm -rf "${ISO_DIR}"
echo ""
echo "================================================"
echo "  HiveStack Demo ISO Built Successfully!"
echo "  Base: Ubuntu 24.04 (Demo/Evaluation)"
echo "================================================"
echo ""
echo "  ISO: ${PROJECT_ROOT}/build/${ISO_NAME}"
echo "  Size: $(du -h "${PROJECT_ROOT}/build/${ISO_NAME}" | cut -f1)"
echo "  SHA256: $(sha256sum "${PROJECT_ROOT}/build/${ISO_NAME}" | cut -d' ' -f1)"
echo ""
echo "  Test in QEMU:"
echo "    qemu-system-x86_64 -m 4G -cdrom ${PROJECT_ROOT}/build/${ISO_NAME} -boot d"
echo ""
echo "  Write to USB:"
echo "    sudo dd if=${PROJECT_ROOT}/build/${ISO_NAME} of=/dev/sdX bs=4M status=progress"
echo ""
echo "================================================"
