#!/bin/bash
# KIWI images.sh - Define output image types for HiveStack appliance

set -euo pipefail

echo "=== HiveStack Appliance: images.sh started ==="

# Build ISO image for SLES 15 SP7
# This creates a bootable ISO that can be used to install the appliance

# The main appliance ISO
IMAGE_NAME="hivestack-appliance"
VERSION="1.0.0"
BUILD_DATE=$(date +%Y%m%d)

# Output directory
OUT_DIR="/tmp/hivestack-build"
mkdir -p "${OUT_DIR}"

# Build the ISO using kiwi-ng
# Note: This requires kiwi-ng installed on the build host

cat <<'EOF' > /tmp/build-appliance.sh
#!/bin/bash
# Build script for HiveStack appliance ISO
# Run this on a SLES 15 SP7 build host with kiwi-ng installed

set -euo pipefail

APP_DIR="/tmp/HiveStack/appliance"
OUT_DIR="/tmp/hivestack-build"
mkdir -p "${OUT_DIR}"

echo "Building HiveStack appliance ISO..."
echo "Source: ${APP_DIR}"
echo "Output: ${OUT_DIR}"

# Install kiwi-ng if not present
if ! command -v kiwi-ng &> /dev/null; then
    echo "Installing kiwi-ng..."
    zypper -n install kiwi-ng python3-kiwi
fi

# Build the appliance
cd "${APP_DIR}"

# Build ISO
kiwi-ng --type iso \
    --description . \
    --target-dir "${OUT_DIR}" \
    --set-repo obs://SUSE:SLE-15-SP7:GA/standard,SLES15-SP7-Pool \
    --set-repo obs://SUSE:SLE-15-SP7:Update/standard,SLES15-SP7-Updates \
    --set-repo obs://Virtualization:kvm/SLE_15_SP7,Virtualization \
    --set-repo obs://systemsmanagement/SLE_15_SP7,SystemsManagement \
    --profile Standard \
    --allow-existing-root \
    build

# Rename output
ISO_FILE=$(find "${OUT_DIR}" -name "*.iso" -type f | head -1)
if [ -n "${ISO_FILE}" ]; then
    NEW_NAME="${OUT_DIR}/hivestack-appliance-${VERSION}-${BUILD_DATE}-x86_64.iso"
    mv "${ISO_FILE}" "${NEW_NAME}"
    echo "ISO built: ${NEW_NAME}"
    
    # Generate checksums
    sha256sum "${NEW_NAME}" > "${NEW_NAME}.sha256"
    echo "Checksum: $(cat ${NEW_NAME}.sha256)"
else
    echo "ERROR: No ISO found in output directory"
    exit 1
fi
EOF

chmod +x /tmp/build-appliance.sh

echo "Build script created at /tmp/build-appliance.sh"
echo "Run it on a SLES 15 SP7 build host to create the ISO"
echo ""
echo "=== HiveStack Appliance: images.sh completed ==="