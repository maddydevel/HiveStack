#!/bin/bash
set -euo pipefail

echo "=== Setting up KVM/QEMU/libvirt ==="

# Install virtualization packages
apt-get install -y \
    qemu-kvm \
    qemu-utils \
    libvirt-daemon-system \
    libvirt-clients \
    bridge-utils \
    virtinst \
    virt-manager \
    libguestfs-tools

# Enable and start libvirt
systemctl enable libvirtd
systemctl start libvirtd

# Add hivestack to libvirt group
usermod -aG libvirt hivestack

# Create default storage pool
mkdir -p /var/lib/hivestack/images
cat > /tmp/default-pool.xml <<'EOF'
<pool type='dir'>
  <name>default</name>
  <target>
    <path>/var/lib/hivestack/images</path>
  </target>
</pool>
EOF
virsh pool-define /tmp/default-pool.xml 2>/dev/null || true
virsh pool-start default 2>/dev/null || true
virsh pool-autostart default 2>/dev/null || true

# Create default network
cat > /tmp/default-net.xml <<'EOF'
<network>
  <name>default</name>
  <bridge name='virbr0' stp='on' delay='0'/>
  <ip address='192.168.122.1' netmask='255.255.255.0'>
    <dhcp>
      <range start='192.168.122.100' end='192.168.122.200'/>
    </dhcp>
  </ip>
</network>
EOF
virsh net-define /tmp/default-net.xml 2>/dev/null || true
virsh net-start default 2>/dev/null || true
virsh net-autostart default 2>/dev/null || true

# Configure hugepages
echo "vm.nr_hugepages = 1024" >> /etc/sysctl.conf

echo "KVM/libvirt setup complete."
