#!/bin/bash
# HiveStack Demo ISO Builder
# Creates a bootable ISO that runs HiveStack from RAM
# Uses host kernel + initrd with a custom squashfs root
#
# This is a DEMO/EVALUATION ISO for Ubuntu hosts.
# For production SAP HANA workloads, use the SLES 15 SP7 build.
#
# Prerequisites: genisoimage, mksquashfs, syslinux, grub-efi-amd64-signed
#
# Usage: ./appliance/scripts/build-demo-iso.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
BUILD_DIR="${PROJECT_ROOT}/build/iso"
VERSION="${HIVESTACK_VERSION:-1.0.0}"
BUILD_DATE="$(date +%Y%m%d)"
ISO_NAME="hivestack-demo-${VERSION}-${BUILD_DATE}-amd64.iso"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; NC='\033[0m'
log_info() { echo -e "${GREEN}[INFO]${NC} $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }
log_step() { echo -e "${BLUE}[STEP]${NC} $*"; }

# Build binaries
log_step "Building HiveStack binaries..."
cd "${PROJECT_ROOT}"
mkdir -p dist
go build -o dist/hive-manager ./cmd/hive-manager
go build -o dist/hive-node ./node/
go build -o dist/hive ./cmd/hive

# Create squashfs root
log_step "Creating squashfs root..."
rm -rf "${BUILD_DIR}/rootfs"
mkdir -p "${BUILD_DIR}/rootfs/"{bin,sbin,etc,proc,sys,dev,tmp,opt/hivestack,var}

# Copy binaries
cp dist/* "${BUILD_DIR}/rootfs/opt/hivestack/bin/" 2>/dev/null || true
cp -r "${PROJECT_ROOT}/internal/db/migrations" "${BUILD_DIR}/rootfs/opt/hivestack/" 2>/dev/null || true

# Copy busybox
cp /bin/busybox "${BUILD_DIR}/rootfs/bin/" 2>/dev/null || true

# Create init script
cat > "${BUILD_DIR}/rootfs/init" <<'INIT'
#!/bin/bash
echo "HiveStack Demo Appliance"
echo "======================="
echo "Binaries: /opt/hivestack/bin/"
echo "Manager:  /opt/hivestack/bin/hive-manager"
echo "Node:     /opt/hivestack/bin/hive-node"
echo ""
echo "This is a demo ISO. For production, use SLES 15 SP7."
exec /bin/busybox sh
INIT
chmod +x "${BUILD_DIR}/rootfs/init"

# Create squashfs
mksquashfs "${BUILD_DIR}/rootfs" "${BUILD_DIR}/root.sqsh" -comp xz

# Setup boot
mkdir -p "${BUILD_DIR}/iso/"{boot,isolinux,efi/boot}
cp /boot/vmlinuz-7.0.0-31-generic "${BUILD_DIR}/iso/boot/vmlinuz"
cp /boot/initrd.img-7.0.0-31-generic "${BUILD_DIR}/iso/boot/initrd.img"
cp "${BUILD_DIR}/root.sqsh" "${BUILD_DIR}/iso/boot/root.sqsh"

# Syslinux
cp /usr/lib/ISOLINUX/isolinux.bin "${BUILD_DIR}/iso/isolinux/" 2>/dev/null || true
cp /usr/lib/syslinux/modules/bios/*.c32 "${BUILD_DIR}/iso/isolinux/" 2>/dev/null || true

cat > "${BUILD_DIR}/iso/isolinux/isolinux.cfg" <<'CFG'
DEFAULT hivestack
PROMPT 0
TIMEOUT 50

LABEL hivestack
    MENU LABEL HiveStack Demo
    KERNEL /boot/vmlinuz
    APPEND initrd=/boot/initrd.img root=/dev/ram0 init=/init ---
CFG

# Build ISO
log_step "Building ISO..."
genisoimage -o "${BUILD_DIR}/${ISO_NAME}" \
    -b isolinux/isolinux.bin -c isolinux/boot.cat \
    -no-emul-boot -boot-load-size 4 -boot-info-table \
    -J -R -V "HIVESTACK" "${BUILD_DIR}/iso"

cd "${BUILD_DIR}"
sha256sum "${ISO_NAME}" > "${ISO_NAME}.sha256"

log_info "ISO built: ${BUILD_DIR}/${ISO_NAME}"
log_info "Size: $(du -h "${ISO_NAME}" | cut -f1)"
log_info "SHA256: $(cat ${ISO_NAME}.sha256)"
log_info "Test: qemu-system-x86_64 -m 2G -cdrom ${BUILD_DIR}/${ISO_NAME} -boot d"
