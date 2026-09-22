#!/bin/bash
# KIWI images.sh - HiveStack SLES 15 SP7 Image Preparation
# Runs before the image is packed

set -e

# Update GRUB configuration
cat > /etc/default/grub <<'EOF'
GRUB_DEFAULT=0
GRUB_TIMEOUT=5
GRUB_DISTRIBUTOR="HiveStack"
GRUB_CMDLINE_LINUX_DEFAULT="console=tty0 console=ttyS0,115200n8 quiet"
GRUB_CMDLINE_LINUX=""
GRUB_TERMINAL="console"
GRUB_GFXMODE="auto"
GRUB_VIDEO_BACKEND="all"
GRUB_DISABLE_RECOVERY="true"
EOF

# Update initramfs
dracut --force

# Configure network
cat > /etc/sysconfig/network/ifcfg-eth0 <<'EOF'
BOOTPROTO='dhcp'
STARTMODE='auto'
EOF

# Set hostname
echo "hivestack" > /etc/HOSTNAME

# Configure NTP
cat > /etc/chrony.conf <<'EOF'
pool pool.ntp.org iburst
driftfile /var/lib/chrony/drift
makestep 1.0 3
rtcsync
logdir /var/log/chrony
EOF

# Clean up sensitive files
rm -f /etc/ssh/ssh_host_*
rm -f /root/.bash_history
rm -f /home/*/.bash_history 2>/dev/null || true

# Clean package cache
zypper clean -a

echo "HiveStack SLES 15 SP7 image preparation complete"
