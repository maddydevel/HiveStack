#!/bin/bash
# KIWI config.sh - Customize the HiveStack appliance during build
# This script runs inside the chroot during image build

set -euo pipefail

echo "=== HiveStack Appliance: config.sh started ==="

# 1. Create HiveStack directories
mkdir -p /etc/hivestack
mkdir -p /var/lib/hivestack/{manager,node,backups,snapshots}
mkdir -p /var/log/hivestack
mkdir -p /opt/hivestack/{bin,lib}
mkdir -p /run/hivestack

# 2. Set permissions for hivestack users
chown -R hivestack:hivestack /var/lib/hivestack /var/log/hivestack /run/hivestack
chown -R hivestack-node:hivestack-node /var/lib/hivestack-node
chmod 750 /var/lib/hivestack /var/log/hivestack
chmod 750 /var/lib/hivestack-node

# 3. Configure libvirt for HiveStack
cat > /etc/libvirt/libvirtd.conf <<'EOF'
# HiveStack libvirt configuration
listen_tls = 0
listen_tcp = 1
tcp_port = "16509"
auth_tcp = "sasl"
unix_sock_group = "libvirt"
unix_sock_rw_perms = "0770"
log_level = 3
log_filters = "3:qemu 3:remote 3:network 3:storage"
EOF

# 4. Configure libvirt SASL for authentication
mkdir -p /etc/sasl2
cat > /etc/sasl2/libvirt.conf <<'EOF'
mech_list: digest-md5
sasldb_path: /etc/libvirt/passwd.db
EOF

# 5. Create libvirt storage pool directory
mkdir -p /var/lib/libvirt/images/hivestack
chown -R qemu:qemu /var/lib/libvirt/images/hivestack

# 6. Configure PostgreSQL for HiveStack
systemctl enable postgresql
# Initialize will happen on first boot

# 7. Configure chrony for NTP
cat > /etc/chrony.conf <<'EOF'
# HiveStack NTP configuration
pool time.google.com iburst
pool time.cloudflare.com iburst
driftfile /var/lib/chrony/drift
makestep 1.0 3
rtcsync
logdir /var/log/chrony
EOF

# 8. Configure AppArmor for libvirt
# Allow libvirt to manage HiveStack VMs
cat > /etc/apparmor.d/local/abstraction.hivestack-libvirt <<'EOF'
# HiveStack libvirt abstractions
/var/lib/hivestack/** rw,
/var/lib/hivestack-node/** rw,
/opt/hivestack/bin/** ix,
/run/hivestack/** rw,
EOF

# 9. Configure kernel parameters for KVM and HANA
cat > /etc/sysctl.d/99-hivestack.conf <<'EOF'
# HiveStack kernel tuning for KVM and SAP HANA VMs

# Virtualization
kernel.kvm.ignore_msrs = 1
kernel.modules_disabled = 0

# Memory management for HANA
vm.nr_hugepages = 1024
vm.hugetlb_shm_group = 1001  # libvirt-qemu group
vm.swappiness = 1
vm.dirty_background_ratio = 5
vm.dirty_ratio = 10

# Network for VM migration
net.core.somaxconn = 4096
net.core.netdev_max_backlog = 5000
net.ipv4.tcp_max_syn_backlog = 8192

# Filesystem
fs.file-max = 2097152
fs.inotify.max_user_watches = 524288
fs.inotify.max_user_instances = 8192
EOF

# 10. Configure systemd limits for HiveStack services
mkdir -p /etc/systemd/system.conf.d
cat > /etc/systemd/system.conf.d/hivestack-limits.conf <<'EOF'
[Manager]
DefaultLimitNOFILE=65536
DefaultLimitNPROC=32768
DefaultLimitMEMLOCK=infinity
DefaultTasksMax=infinity
EOF

# 11. Setup logrotate for HiveStack
cat > /etc/logrotate.d/hivestack <<'EOF'
/var/log/hivestack/*.log {
    daily
    rotate 30
    compress
    delaycompress
    missingok
    notifempty
    create 0640 hivestack hivestack
    sharedscripts
    postrotate
        systemctl reload hivestack-manager hivestack-node > /dev/null 2>&1 || true
    endscript
}
EOF

# 12. Generate SSH host keys (will be regenerated on first boot)
ssh-keygen -A

# 13. Configure firewall (firewalld)
systemctl enable firewalld
cat > /etc/firewalld/zones/hivestack.xml <<'EOF'
<?xml version="1.0" encoding="utf-8"?>
<zone>
  <short>HiveStack</short>
  <description>HiveStack management and VM traffic</description>
  <service name="ssh"/>
  <service name="cockpit"/>
  <service name="libvirt"/>
  <service name="libvirt-tls"/>
  <port port="8080" protocol="tcp"/>  <!-- HiveStack Manager API -->
  <port port="8443" protocol="tcp"/>  <!-- HiveStack Manager gRPC -->
  <port port="9090" protocol="tcp"/>  <!-- HiveStack Node Agent gRPC -->
  <port port="5900-6923" protocol="tcp"/>  <!-- VNC/Spice consoles -->
  <port port="16509" protocol="tcp"/>  <!-- libvirt TCP -->
  <port port="9100" protocol="tcp"/>  <!-- Prometheus node-exporter -->
</zone>
EOF

# 14. Set default zone to hivestack
sed -i 's/^DefaultZone=.*/DefaultZone=hivestack/' /etc/firewalld/firewalld.conf

# 15. Create first-boot setup script marker
touch /etc/hivestack/.first-boot

echo "=== HiveStack Appliance: config.sh completed ==="