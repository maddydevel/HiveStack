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

# 15. Configure LUKS disk encryption for appliance builds
# Create LUKS key directory for encrypted volumes
mkdir -p /etc/hivestack/luks-keys
chmod 700 /etc/hivestack/luks-keys

# Configure crypttab for encrypted volumes (if using encrypted root)
cat > /etc/crypttab.hivestack <<'EOF'
# HiveStack LUKS encrypted volumes
# Format: <name> <device> <key_file> <options>
# hivestack-vm-data /dev/disk/by-uuid/XXXX-XXXX /etc/hivestack/luks-keys/vm-data.key luks,discard,noauto
# hivestack-backups /dev/disk/by/uuid/YYYY-YYYY /etc/hivestack/luks-keys/backups.key luks,discard,noauto
EOF

# 16. Install security packages
# Ensure all security-related packages are installed
zypper install -y \
    apparmor-utils \
    apparmor-profiles \
    audit \
    audit-libs \
    cryptsetup \
    openssl \
    ca-certificates \
    sops \
    age

# 17. Configure auditd for HiveStack
cat > /etc/audit/rules.d/hivestack.rules <<'EOF'
# HiveStack audit rules
# Monitor HiveStack configuration changes
-w /etc/hivestack/ -p wa -k hivestack_config
-w /etc/hivestack/tls/ -p wa -k hivestack_tls
-w /etc/hivestack/luks-keys/ -p rwxa -k hivestack_keys

# Monitor HiveStack binaries
-w /usr/sbin/hivestack-manager -p x -k hivestack_exec
-w /usr/sbin/hivestack-node -p x -k hivestack_exec

# Monitor HiveStack data directories
-w /var/lib/hivestack/ -p wa -k hivestack_data

# Monitor authentication events
-w /etc/pam.d/ -p wa -k hivestack_auth
-w /etc/shadow -p wa -k hivestack_auth

# Monitor network configuration changes
-w /etc/hosts -p wa -k hivestack_network
-w /etc/resolv.conf -p wa -k hivestack_network

# Monitor user/group modifications
-w /etc/passwd -p wa -k hivestack_users
-w /etc/group -p wa -k hivestack_users

# Monitor sudoers
-w /etc/sudoers -p wa -k hivestack_sudo
-w /etc/sudoers.d/ -p wa -k hivestack_sudo

# Make audit log immutable (requires reboot to change)
-e 2
EOF

# Enable and start auditd
systemctl enable auditd

# 18. Configure AppArmor profiles for HiveStack services
# Write Manager AppArmor profile
cat > /etc/apparmor.d/usr.sbin.hivestack-manager <<'EOF'
#include <tunables/global>

/usr/sbin/hivestack-manager {
  #include <abstractions/base>
  #include <abstractions/nameservice>
  #include <abstractions/ssl_certs>

  # Binary
  /usr/sbin/hivestack-manager mr,

  # Configuration
  /etc/hivestack/ r,
  /etc/hivestack/** r,

  # Data directories
  /var/lib/hivestack/ r,
  /var/lib/hivestack/** rw,

  # Logs
  /var/log/hivestack/ r,
  /var/log/hivestack/** rw,

  # TLS certificates
  /etc/hivestack/tls/ r,
  /etc/hivestack/tls/** r,

  # PID file
  /run/hivestack/ r,
  /run/hivestack/** rw,

  # PostgreSQL (local socket or TCP)
  stream_connect unix /var/run/postgresql/.s.PGSQL.*,
  network inet stream,
  network inet6 stream,

  # Deny dangerous operations
  deny /usr/** w,
  deny /boot/** w,
  deny /etc/shadow r,
  deny /etc/passwd w,

  # Capability restrictions
  capability net_bind_service,
  capability dac_read_search,
  deny capability sys_admin,
  deny capability sys_ptrace,
  deny capability sys_module,
  deny capability dac_override,
}
EOF

# Write Node Agent AppArmor profile
cat > /etc/apparmor.d/usr.sbin.hivestack-node <<'EOF'
#include <tunables/global>

/usr/sbin/hivestack-node {
  #include <abstractions/base>
  #include <abstractions/nameservice>
  #include <abstractions/ssl_certs>

  # Binary
  /usr/sbin/hivestack-node mr,

  # Configuration
  /etc/hivestack/ r,
  /etc/hivestack/** r,

  # Data directories
  /var/lib/hivestack/ r,
  /var/lib/hivestack/** rw,
  /var/lib/hivestack/images/ r,
  /var/lib/hivestack/images/** rw,

  # Logs
  /var/log/hivestack/ r,
  /var/log/hivestack/** rw,

  # TLS certificates
  /etc/hivestack/tls/ r,
  /etc/hivestack/tls/** r,

  # PID file
  /run/hivestack/ r,
  /run/hivestack/** rw,

  # Libvirt socket
  /var/run/libvirt/libvirt-sock rw,

  # Network - connect to Manager
  network inet stream,
  network inet6 stream,

  # Deny dangerous operations
  deny /usr/** w,
  deny /boot/** w,
  deny /etc/shadow r,
  deny /etc/passwd w,

  # Capability restrictions (VM management needs some)
  capability net_bind_service,
  capability dac_read_search,
  capability chown,
  capability fowner,
  deny capability sys_admin,
  deny capability sys_ptrace,
  deny capability sys_module,
}
EOF

# Enable AppArmor
systemctl enable apparmor

# 19. Configure secure TLS defaults
mkdir -p /etc/hivestack/tls
chmod 750 /etc/hivestack/tls

# Generate self-signed certificates on first boot if not present
cat > /etc/hivestack/tls/generate-certs.sh <<'CERTEOF'
#!/bin/bash
# Generate HiveStack TLS certificates on first boot
set -euo pipefail

TLS_DIR="/etc/hivestack/tls"
COMMON_NAME="${HIVESTACK_TLS_CN:-hivestack-manager}"

if [ ! -f "$TLS_DIR/ca.crt" ]; then
    echo "Generating HiveStack CA certificate..."
    openssl ecparam -genkey -name prime256v1 -out "$TLS_DIR/ca.key"
    openssl req -new -x509 -key "$TLS_DIR/ca.key" -out "$TLS_DIR/ca.crt" \
        -days 3650 -subj "/CN=HiveStack Root CA/O=HiveStack"
    chmod 600 "$TLS_DIR/ca.key"
    chmod 644 "$TLS_DIR/ca.crt"
fi

if [ ! -f "$TLS_DIR/server.crt" ]; then
    echo "Generating HiveStack server certificate..."
    openssl ecparam -genkey -name prime256v1 -out "$TLS_DIR/server.key"
    openssl req -new -key "$TLS_DIR/server.key" -out "$TLS_DIR/server.csr" \
        -subj "/CN=$COMMON_NAME/O=HiveStack"
    openssl x509 -req -in "$TLS_DIR/server.csr" -CA "$TLS_DIR/ca.crt" \
        -CAkey "$TLS_DIR/ca.key" -CAcreateserial -out "$TLS_DIR/server.crt" \
        -days 365 -sha256 \
        -extfile <(printf "subjectAltName=DNS:$COMMON_NAME,DNS:localhost,IP:127.0.0.1")
    rm -f "$TLS_DIR/server.csr"
    chmod 600 "$TLS_DIR/server.key"
    chmod 644 "$TLS_DIR/server.crt"
fi

echo "TLS certificates ready in $TLS_DIR"
CERTEOF
chmod 700 /etc/hivestack/tls/generate-certs.sh

# 20. Configure secure SSH settings
cat > /etc/ssh/sshd_config.d/hivestack.conf <<'EOF'
# HiveStack SSH hardening
PermitRootLogin no
PasswordAuthentication no
ChallengeResponseAuthentication no
PubkeyAuthentication yes
MaxAuthTries 3
LoginGraceTime 30
ClientAliveInterval 300
ClientAliveCountMax 2
AllowUsers hivestack-admin
X11Forwarding no
AllowTcpForwarding no
PermitTunnel no
EOF

# 21. Configure kernel security parameters
cat >> /etc/sysctl.d/99-hivestack.conf <<'EOF'

# Security hardening
kernel.randomize_va_space = 2
kernel.kptr_restrict = 2
kernel.dmesg_restrict = 1
kernel.yama.ptrace_scope = 2
kernel.unprivileged_bpf_disabled = 1
net.core.bpf_jit_harden = 2
kernel.sysrq = 0
kernel.unprivileged_userns_clone = 0
EOF

# 22. Create first-boot setup script marker
touch /etc/hivestack/.first-boot

echo "=== HiveStack Appliance: config.sh completed ==="