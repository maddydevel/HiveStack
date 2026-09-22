#!/bin/bash
# KIWI config.sh - HiveStack SLES 15 SP7 Appliance
# This script runs during image preparation

# Exit on error
set -e

# Update system
zypper --non-interactive refresh
zypper --non-interactive update -y

# Install additional packages that might not be in repos
zypper --non-interactive install -y \
    openssl \
    ca-certificates-mozilla

# Create HiveStack directories
mkdir -p /opt/hivestack/{bin,config,certs,logs,migrations}
mkdir -p /etc/hivestack
mkdir -p /var/lib/hivestack
mkdir -p /var/log/hivestack

# Create hivestack user if not exists
if ! id -u hivestack >/dev/null 2>&1; then
    useradd -r -m -s /bin/bash -U hivestack
    usermod -aG libvirt hivestack
fi

# Set permissions
chown -R hivestack:hivestack /opt/hivestack /var/lib/hivestack /var/log/hivestack
chmod 750 /etc/hivestack
chmod 600 /etc/hivestack/*.yaml 2>/dev/null || true

# Enable services
systemctl enable sshd
systemctl enable libvirtd
systemctl enable postgresql
systemctl enable hivestack-manager
systemctl enable hivestack-node
systemctl enable hivestack-first-boot
systemctl enable firewalld
systemctl enable prometheus-node-exporter

# Configure firewall
firewall-cmd --permanent --add-port=8080/tcp 2>/dev/null || true
firewall-cmd --permanent --add-port=8443/tcp 2>/dev/null || true
firewall-cmd --permanent --add-port=44567/tcp 2>/dev/null || true
firewall-cmd --permanent --add-service=ssh 2>/dev/null || true

# Configure libvirt
cat > /etc/sysconfig/libvirtd <<'EOF'
LIBVIRTD_ARGS="--listen"
EOF

# Configure hugepages for HANA
echo "vm.nr_hugepages = 1024" > /etc/sysctl.d/99-hivestack-hugepages.conf

# Configure postgresql
su - postgres -c "initdb -D /var/lib/postgresql/data" 2>/dev/null || true

# Clean up
zypper clean -a
rm -rf /var/cache/zypper/*
rm -rf /tmp/*

echo "HiveStack SLES 15 SP7 image preparation complete"
