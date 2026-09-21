#!/bin/bash
# HiveStack Appliance ISO Builder
# Creates a bootable ISO 9660 image with HiveStack pre-installed
#
# This script MUST be run as root on an Ubuntu 24.04 build host.
#
# Prerequisites (install with: sudo apt-get install -y ...):
#   - genisoimage        (ISO 9660 creation)
#   - squashfs-tools     (compressed rootfs)
#   - syslinux           (BIOS boot)
#   - syslinux-efi       (UEFI boot)
#   - grub-efi-amd64-bin (GRUB EFI)
#   - grub-pc-bin        (GRUB BIOS)
#   - debootstrap        (minimal rootfs)
#   - qemu-system-x86    (testing)
#   - openssl            (certificates)
#   - wget, curl         (downloads)
#
# Usage:
#   sudo ./appliance/scripts/build-iso.sh
#
# Output:
#   build/iso/hivestack-appliance-<version>-<date>-amd64.iso
#
# Test:
#   qemu-system-x86_64 -m 4G -cdrom build/iso/hivestack-appliance-*.iso -boot d
#
# Write to USB:
#   sudo dd if=build/iso/hivestack-appliance-*.iso of=/dev/sdX bs=4M status=progress

set -euo pipefail

# ============================================================================
# Configuration
# ============================================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
BUILD_DIR="${PROJECT_ROOT}/build/iso"
ROOTFS_DIR="${BUILD_DIR}/rootfs"
ISO_DIR="${BUILD_DIR}/image"
OUTPUT_DIR="${BUILD_DIR}/output"
VERSION="${HIVESTACK_VERSION:-1.0.0}"
BUILD_DATE="$(date +%Y%m%d)"
ISO_NAME="hivestack-appliance-${VERSION}-${BUILD_DATE}-amd64.iso"
HIVESTACK_BINARIES="${PROJECT_ROOT}/dist"

# ============================================================================
# Colors
# ============================================================================
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info()  { echo -e "${GREEN}[INFO]${NC}  $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }
log_step()  { echo -e "${BLUE}[STEP]${NC}  $*"; }

# ============================================================================
# Preflight checks
# ============================================================================
check_root() {
    if [ "$EUID" -ne 0 ]; then
        log_error "This script must be run as root"
        log_error "Usage: sudo $0"
        exit 1
    fi
}

check_dependencies() {
    local missing=()
    
    for cmd in genisoimage mksquashfs syslinux grub-mkrescue debootstrap openssl; do
        if ! command -v "$cmd" &>/dev/null; then
            missing+=("$cmd")
        fi
    done
    
    if [ ${#missing[@]} -gt 0 ]; then
        log_error "Missing dependencies: ${missing[*]}"
        log_error "Install with: sudo apt-get install -y ${missing[*]}"
        exit 1
    fi
}

# ============================================================================
# Build HiveStack binaries
# ============================================================================
build_binaries() {
    log_step "Building HiveStack binaries..."
    
    cd "${PROJECT_ROOT}"
    
    # Ensure Go is available
    if ! command -v go &>/dev/null; then
        log_error "Go not found. Install Go 1.23+ first."
        exit 1
    fi
    
    mkdir -p "${HIVESTACK_BINARIES}"
    
    # Build manager
    log_info "Building hive-manager..."
    CGO_ENABLED=0 go build -ldflags="-w -s" \
        -o "${HIVESTACK_BINARIES}/hive-manager" ./cmd/hive-manager
    
    # Build node agent
    log_info "Building hive-node..."
    CGO_ENABLED=0 go build -ldflags="-w -s" \
        -o "${HIVESTACK_BINARIES}/hive-node" ./node/
    
    # Build CLI
    log_info "Building hive CLI..."
    CGO_ENABLED=0 go build -ldflags="-w -s" \
        -o "${HIVESTACK_BINARIES}/hive" ./cmd/hive
    
    log_info "Binaries built successfully."
    ls -la "${HIVESTACK_BINARIES}/"
}

# ============================================================================
# Create minimal rootfs with debootstrap
# ============================================================================
create_rootfs() {
    log_step "Creating minimal Ubuntu 24.04 rootfs..."
    
    # Clean previous
    if [ -d "${ROOTFS_DIR}" ]; then
        log_warn "Removing existing rootfs..."
        umount -Rf "${ROOTFS_DIR}/dev/pts" 2>/dev/null || true
        umount -Rf "${ROOTFS_DIR}/dev" 2>/dev/null || true
        umount -Rf "${ROOTFS_DIR}/proc" 2>/dev/null || true
        umount -Rf "${ROOTFS_DIR}/sys" 2>/dev/null || true
        umount -Rf "${ROOTFS_DIR}/run" 2>/dev/null || true
        rm -rf "${ROOTFS_DIR}"
    fi
    
    mkdir -p "${ROOTFS_DIR}"
    
    # Debootstrap minimal Ubuntu
    log_info "Running debootstrap (this may take 5-10 minutes)..."
    debootstrap --arch amd64 --variant=minbase \
        --include=systemd,systemd-sysv,openssh-server,sudo,curl,wget,ca-certificates,\
iproute2,net-tools,bridge-utils,vlan,openvswitch-switch,dnsutils,iputils-ping,\
vim-tiny,nano,less,jq,openssl,postgresql,postgresql-contrib,\
qemu-kvm,qemu-utils,libvirt-daemon-system,libvirt-clients,virtinst,\
open-iscsi,lvm2,parted,gdisk,kmod,util-linux,dbus,udev,chrony,linux-image-generic \
        noble "${ROOTFS_DIR}" http://archive.ubuntu.com/ubuntu/
    
    log_info "Rootfs created. Size: $(du -sh "${ROOTFS_DIR}" | cut -f1)"
}

# ============================================================================
# Configure rootfs
# ============================================================================
configure_rootfs() {
    log_step "Configuring rootfs for HiveStack..."
    
    # Mount pseudo-filesystems for chroot
    mount --bind /dev "${ROOTFS_DIR}/dev"
    mount --bind /dev/pts "${ROOTFS_DIR}/dev/pts"
    mount --bind /proc "${ROOTFS_DIR}/proc"
    mount --bind /sys "${ROOTFS_DIR}/sys"
    mount --bind /run "${ROOTFS_DIR}/run"
    
    # Set up DNS
    cp /etc/resolv.conf "${ROOTFS_DIR}/etc/resolv.conf"
    
    # Create HiveStack directories
    mkdir -p "${ROOTFS_DIR}/opt/hivestack/"{bin,config,certs,logs,migrations,scripts}
    mkdir -p "${ROOTFS_DIR}/var/lib/hivestack"
    mkdir -p "${ROOTFS_DIR}/var/log/hivestack"
    mkdir -p "${ROOTFS_DIR}/etc/hivestack"
    
    # Install HiveStack binaries
    cp "${HIVESTACK_BINARIES}/hive-manager" "${ROOTFS_DIR}/opt/hivestack/bin/"
    cp "${HIVESTACK_BINARIES}/hive-node" "${ROOTFS_DIR}/opt/hivestack/bin/"
    cp "${HIVESTACK_BINARIES}/hive" "${ROOTFS_DIR}/opt/hivestack/bin/"
    chmod +x "${ROOTFS_DIR}/opt/hivestack/bin/"*
    
    # Install DB migrations
    if [ -d "${PROJECT_ROOT}/internal/db/migrations" ]; then
        cp "${PROJECT_ROOT}/internal/db/migrations/"*.sql "${ROOTFS_DIR}/opt/hivestack/migrations/"
    fi
    
    # Create hivestack user
    chroot "${ROOTFS_DIR}" /bin/bash -c "
        useradd -r -m -s /bin/bash -U hivestack 2>/dev/null || true
        echo 'hivestack ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/hivestack
        chmod 0440 /etc/sudoers.d/hivestack
        usermod -aG libvirt hivestack
        chown -R hivestack:hivestack /opt/hivestack /var/lib/hivestack /var/log/hivestack
    "
    
    # Install systemd services
    cat > "${ROOTFS_DIR}/etc/systemd/system/hivestack-manager.service" <<'SVCEOF'
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

    cat > "${ROOTFS_DIR}/etc/systemd/system/hivestack-node.service" <<'SVCEOF'
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

    cat > "${ROOTFS_DIR}/etc/systemd/system/hivestack-first-boot.service" <<'SVCEOF'
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

    # Enable services
    chroot "${ROOTFS_DIR}" /bin/bash -c "
        systemctl enable hivestack-manager hivestack-node hivestack-first-boot
        systemctl enable libvirtd postgresql
        systemctl enable serial-getty@ttyS0
    "
    
    # Copy first-boot script
    if [ -f "${SCRIPT_DIR}/first-boot.sh" ]; then
        cp "${SCRIPT_DIR}/first-boot.sh" "${ROOTFS_DIR}/opt/hivestack/scripts/"
        chmod +x "${ROOTFS_DIR}/opt/hivestack/scripts/first-boot.sh"
    fi
    
    # Create boot menu script (runs in initramfs)
    cat > "${ROOTFS_DIR}/opt/hivestack/scripts/live-boot.sh" <<'LIVEEOF'
#!/bin/bash
# Live boot wrapper - sets up overlay and starts HiveStack

# Mount the squashfs
mkdir -p /mnt/squash
mount -t iso9660 /dev/cdrom /mnt/squash 2>/dev/null || \
mount -t iso9660 /dev/sr0 /mnt/squash 2>/dev/null || true

# Start services
systemctl start postgresql 2>/dev/null || true
systemctl start libvirtd 2>/dev/null || true

# Run first-boot
[ -f /opt/hivestack/scripts/first-boot.sh ] && /opt/hivestack/scripts/first-boot.sh

# Start HiveStack
systemctl start hivestack-manager hivestack-node 2>/dev/null || true

echo ""
echo "================================================"
echo "  HiveStack Appliance is ready!"
echo "================================================"
echo "  IP Address: $(hostname -I | awk '{print $1}')"
echo "  API:        https://$(hostname -I | awk '{print $1}'):8080"
echo "  gRPC:       $(hostname -I | awk '{print $1}'):44567"
echo "  Health:     http://$(hostname -I | awk '{print $1}'):8080/api/v1/health"
echo "================================================"
echo ""
LIVEEOF
    chmod +x "${ROOTFS_DIR}/opt/hivestack/scripts/live-boot.sh"
    
    # Create config templates
    cat > "${ROOTFS_DIR}/etc/hivestack/manager.yaml.example" <<'CFGEOF'
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

    cat > "${ROOTFS_DIR}/etc/hivestack/node.yaml.example" <<'CFGEOF'
node_id: "node-001"
manager_address: "localhost:44567"
tls_cert_file: "/etc/hivestack/certs/node-server.crt"
tls_key_file: "/etc/hivestack/certs/node-server.key"
tls_ca_file: "/etc/hivestack/certs/ca.crt"
heartbeat_interval: 30
log_level: "info"
log_format: "json"
CFGEOF
    
    # Configure network
    cat > "${ROOTFS_DIR}/etc/netplan/01-netcfg.yaml" <<'NETEOF'
network:
  version: 2
  ethernets:
    all-en:
      match:
        name: "en*"
      dhcp4: true
    all-eth:
      match:
        name: "eth*"
      dhcp4: true
NETEOF
    
    # Configure firewall
    cat > "${ROOTFS_DIR}/etc/ufw/applications.d/hivestack" <<'UFEOF'
[HiveStack]
title=HiveStack
description=HiveStack Management Platform
ports=8080,8443,44567/tcp
UFEOF
    
    # Clean up chroot
    rm -f "${ROOTFS_DIR}/etc/resolv.conf"
    umount -Rf "${ROOTFS_DIR}/dev/pts" 2>/dev/null || true
    umount -Rf "${ROOTFS_DIR}/dev" 2>/dev/null || true
    umount -Rf "${ROOTFS_DIR}/proc" 2>/dev/null || true
    umount -Rf "${ROOTFS_DIR}/sys" 2>/dev/null || true
    umount -Rf "${ROOTFS_DIR}/run" 2>/dev/null || true
    
    log_info "Rootfs configured."
}

# ============================================================================
# Create squashfs
# ============================================================================
create_squashfs() {
    log_step "Creating squashfs image..."
    
    if [ -f "${BUILD_DIR}/filesystem.squashfs" ]; then
        rm -f "${BUILD_DIR}/filesystem.squashfs"
    fi
    
    mksquashfs "${ROOTFS_DIR}" "${BUILD_DIR}/filesystem.squashfs" \
        -comp xz -b 1M -Xdict-size 100% \
        -e boot \
        -e proc \
        -e sys \
        -e dev \
        -e run \
        -e tmp \
        -e mnt \
        -e media \
        -e lost+found
    
    log_info "Squashfs created: $(du -h "${BUILD_DIR}/filesystem.squashfs" | cut -f1)"
}

# ============================================================================
# Create bootloader configs
# ============================================================================
create_bootloader_configs() {
    log_step "Creating bootloader configurations..."
    
    # BIOS: syslinux
    mkdir -p "${ISO_DIR}/isolinux"
    
    # Copy syslinux modules
    cp /usr/lib/ISOLINUX/isolinux.bin "${ISO_DIR}/isolinux/" 2>/dev/null || \
        cp /usr/lib/syslinux/modules/isolinux.bin "${ISO_DIR}/isolinux/" 2>/dev/null || true
    cp /usr/lib/syslinux/modules/bios/ldlinux.c32 "${ISO_DIR}/isolinux/" 2>/dev/null || true
    cp /usr/lib/syslinux/modules/bios/libcom32.c32 "${ISO_DIR}/isolinux/" 2>/dev/null || true
    cp /usr/lib/syslinux/modules/bios/libutil.c32 "${ISO_DIR}/isolinux/" 2>/dev/null || true
    cp /usr/lib/syslinux/modules/bios/menu.c32 "${ISO_DIR}/isolinux/" 2>/dev/null || true
    cp /usr/lib/syslinux/modules/bios/vesamenu.c32 "${ISO_DIR}/isolinux/" 2>/dev/null || true
    
    cat > "${ISO_DIR}/isolinux/isolinux.cfg" <<'ISOCFG'
DEFAULT hivestack
PROMPT 0
TIMEOUT 100
UI menu.c32

MENU TITLE HiveStack Appliance
MENU BACKGROUND splash.png
MENU COLOR title   1;37;44  #c00090f0 #00000000 std
MENU COLOR unsel    37;44   #90ffffff #00000000 std
MENU COLOR sel      7;37;40 #e0000000 #20ffffff all

LABEL hivestack
    MENU LABEL ^HiveStack Appliance
    MENU DEFAULT
    KERNEL /boot/vmlinuz
    APPEND initrd=/boot/initrd.img boot=casper hostname=hivestack username=hivestack quiet splash ---
    TEXT HELP
    Boot HiveStack Appliance with full virtualization support.
    ENDTEXT

LABEL hivestack-safe
    MENU LABEL HiveStack Appliance (^Safe Mode)
    KERNEL /boot/vmlinuz
    APPEND initrd=/boot/initrd.img boot=casper hostname=hivestack username=hivestack nomodeset quiet ---
    TEXT HELP
    Boot in safe mode (no KVM acceleration, for debugging).
    ENDTEXT

LABEL hivestack-install
    MENU LABEL ^Install to Disk
    KERNEL /boot/vmlinuz
    APPEND initrd=/boot/initrd.img boot=casper hostname=hivestack username=hivestack install quiet ---
    TEXT HELP
    Install HiveStack to disk (destructive!).
    ENDTEXT

LABEL local
    MENU LABEL Boot from ^Drive
    LOCALBOOT 0
ISOCFG

    # UEFI: GRUB
    mkdir -p "${ISO_DIR}/efi/boot"
    
    # Create GRUB EFI image
    grub-mkstandalone \
        --format=x86_64-efi \
        --output="${ISO_DIR}/efi/boot/bootx64.efi" \
        --locales="" \
        --fonts="" \
        "boot/grub/grub.cfg=${ISO_DIR}/efi/boot/grub.cfg" \
        2>/dev/null || true
    
    cat > "${ISO_DIR}/efi/boot/grub.cfg" <<'GRUBCFG'
set timeout=10
set default=0

menuentry "HiveStack Appliance" {
    linux /boot/vmlinuz boot=casper hostname=hivestack username=hivestack quiet splash
    initrd /boot/initrd.img
}

menuentry "HiveStack Appliance (Safe Mode)" {
    linux /boot/vmlinuz boot=casper hostname=hivestack username=hivestack nomodeset quiet
    initrd /boot/initrd.img
}

menuentry "Install to Disk" {
    linux /boot/vmlinuz boot=casper hostname=hivestack username=hivestack install quiet
    initrd /boot/initrd.img
}
GRUBCFG

    # Create EFI system partition image
    dd if=/dev/zero of="${ISO_DIR}/efi/efi.img" bs=1M count=16 2>/dev/null
    mkfs.vfat "${ISO_DIR}/efi/efi.img" 2>/dev/null || true
    
    # Mount and copy GRUB
    mkdir -p /tmp/efi-mount
    mount -o loop "${ISO_DIR}/efi/efi.img" /tmp/efi-mount 2>/dev/null || true
    mkdir -p /tmp/efi-mount/efi/boot
    cp -r "${ISO_DIR}/efi/boot/"* /tmp/efi-mount/efi/boot/ 2>/dev/null || true
    umount /tmp/efi-mount 2>/dev/null || true
    
    log_info "Bootloader configs created."
}

# ============================================================================
# Assemble ISO
# ============================================================================
assemble_iso() {
    log_step "Assembling ISO..."
    
    # Copy kernel and initrd
    mkdir -p "${ISO_DIR}/boot"
    
    # Use host kernel/initrd (Ubuntu 24.04 compatible)
    if [ -f /boot/vmlinuz ] && [ -f /boot/initrd.img ]; then
        cp /boot/vmlinuz "${ISO_DIR}/boot/vmlinuz"
        cp /boot/initrd.img "${ISO_DIR}/boot/initrd.img"
    else
        log_error "No kernel/initrd found in /boot"
        exit 1
    fi
    
    # Copy squashfs
    mkdir -p "${ISO_DIR}/squashfs"
    cp "${BUILD_DIR}/filesystem.squashfs" "${ISO_DIR}/squashfs/filesystem.squashfs"
    
    # Copy HiveStack files
    mkdir -p "${ISO_DIR}/hive"
    cp -r "${ROOTFS_DIR}/opt/hivestack/"* "${ISO_DIR}/hive/" 2>/dev/null || true
    
    # Create the ISO
    mkdir -p "${OUTPUT_DIR}"
    
    genisoimage \
        -o "${OUTPUT_DIR}/${ISO_NAME}" \
        -b isolinux/isolinux.bin \
        -c isolinux/boot.cat \
        -no-emul-boot \
        -boot-load-size 4 \
        -boot-info-table \
        -J -R -V "HIVESTACK" \
        -eltorito-alt-boot \
        -e efi/efi.img \
        -no-emul-boot \
        -isohybrid-gpt-basdat \
        "${ISO_DIR}"
    
    # Make hybrid (bootable from USB)
    if command -v isohybrid &>/dev/null; then
        isohybrid --uefi "${OUTPUT_DIR}/${ISO_NAME}" 2>/dev/null || true
    fi
    
    # Generate checksums
    cd "${OUTPUT_DIR}"
    sha256sum "${ISO_NAME}" > "${ISO_NAME}.sha256"
    md5sum "${ISO_NAME}" > "${ISO_NAME}.md5"
    
    log_info "ISO assembled successfully!"
}

# ============================================================================
# Print results
# ============================================================================
print_results() {
    echo ""
    echo "================================================"
    echo "  HiveStack Appliance ISO Built Successfully!"
    echo "================================================"
    echo ""
    echo "  ISO:      ${OUTPUT_DIR}/${ISO_NAME}"
    echo "  Size:     $(du -h "${OUTPUT_DIR}/${ISO_NAME}" | cut -f1)"
    echo "  SHA256:   $(cat "${OUTPUT_DIR}/${ISO_NAME}.sha256" | cut -d' ' -f1)"
    echo "  MD5:      $(cat "${OUTPUT_DIR}/${ISO_NAME}.md5" | cut -d' ' -f1)"
    echo ""
    echo "  Test in QEMU:"
    echo "    qemu-system-x86_64 -m 4G -cdrom ${OUTPUT_DIR}/${ISO_NAME} -boot d -enable-kvm"
    echo ""
    echo "  Write to USB:"
    echo "    sudo dd if=${OUTPUT_DIR}/${ISO_NAME} of=/dev/sdX bs=4M status=progress"
    echo ""
    echo "  Burn to DVD:"
    echo "    wodim dev=/dev/sr0 -v -data ${OUTPUT_DIR}/${ISO_NAME}"
    echo ""
    echo "================================================"
}

# ============================================================================
# Main
# ============================================================================
main() {
    echo ""
    echo "================================================"
    echo "  HiveStack Appliance ISO Builder"
    echo "  Version: ${VERSION}"
    echo "  Date:    ${BUILD_DATE}"
    echo "================================================"
    echo ""
    
    check_root
    check_dependencies
    
    # Clean build directory
    rm -rf "${BUILD_DIR}"
    mkdir -p "${BUILD_DIR}" "${ISO_DIR}" "${OUTPUT_DIR}"
    
    # Build steps
    build_binaries
    create_rootfs
    configure_rootfs
    create_squashfs
    create_bootloader_configs
    assemble_iso
    print_results
}

main "$@"
