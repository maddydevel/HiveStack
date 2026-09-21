#!/bin/bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
BUILD_DIR="${PROJECT_ROOT}/build/iso"
ISO_NAME="hivestack-demo-${HIVESTACK_VERSION:-1.0.0}-amd64.iso"

# Build binaries
cd "${PROJECT_ROOT}"
go build -o ${BUILD_DIR}/squashfs/hive-manager ./cmd/hive-manager 2>/dev/null || mkdir -p ${BUILD_DIR}/squashfs && go build -o ${BUILD_DIR}/squashfs/hive-manager ./cmd/hive-manager
go build -o ${BUILD_DIR}/squashfs/hive-node ./node/
go build -o ${BUILD_DIR}/squashfs/hive ./cmd/hive
mkdir -p ${BUILD_DIR}/squashfs && cp -r internal/db/migrations ${BUILD_DIR}/squashfs/ 2>/dev/null || true

# Squashfs
rm -f ${BUILD_DIR}/boot/hive.sqsh
mksquashfs ${BUILD_DIR}/squashfs ${BUILD_DIR}/boot/hive.sqsh -comp xz

# Kernel
cp /boot/vmlinuz-6.14.0-37-generic ${BUILD_DIR}/boot/vmlinuz
cp /boot/initrd.img-6.14.0-37-generic ${BUILD_DIR}/boot/initrd.img

# GRUB config
mkdir -p ${BUILD_DIR}/boot/grub
cat > ${BUILD_DIR}/boot/grub/grub.cfg <<'GRUBCFG'
set timeout=5
set default=0
menuentry "HiveStack Demo Appliance" {
    linux /boot/vmlinuz boot=live
    initrd /boot/initrd.img
}
GRUBCFG

# Build ISO with grub-mkrescue (requires xorriso)
# Install xorriso if missing: apt install xorriso
grub-mkrescue -o ${BUILD_DIR}/${ISO_NAME} ${BUILD_DIR}/ 2>&1 | tail -3
echo "ISO: ${BUILD_DIR}/${ISO_NAME}"
