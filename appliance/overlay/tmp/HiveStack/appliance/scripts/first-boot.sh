#!/bin/bash
# HiveStack Appliance First Boot Setup
# This script runs on the first boot of the installed appliance
# It initializes PostgreSQL, generates certificates, sets up services, and starts HiveStack

set -euo pipefail

FIRST_BOOT_MARKER="/etc/hivestack/.first-boot"
SETUP_LOG="/var/log/hivestack/first-boot.log"

# Redirect all output to log file
exec > >(tee -a "${SETUP_LOG}") 2>&1

echo "========================================="
echo "HiveStack Appliance First Boot Setup"
echo "Date: $(date)"
echo "========================================="

# Check if already run
if [ ! -f "${FIRST_BOOT_MARKER}" ]; then
    echo "First boot setup already completed. Exiting."
    exit 0
fi

# 1. Initialize PostgreSQL
echo "[1/10] Initializing PostgreSQL..."
systemctl start postgresql
sleep 5

# Create HiveStack database and user
sudo -u postgres psql <<'PGSQL'
CREATE USER hivestack WITH ENCRYPTED PASSWORD 'hivestack-secure-password-change-me';
CREATE DATABASE hivestack OWNER hivestack;
GRANT ALL PRIVILEGES ON DATABASE hivestack TO hivestack;
\q
PGSQL

# Run database migrations
if [ -f /opt/hivestack/bin/hive-manager ]; then
    /opt/hivestack/bin/hive-manager migrate up
fi

# 2. Generate TLS certificates for mTLS
echo "[2/10] Generating TLS certificates..."
mkdir -p /etc/hivestack/certs
cd /etc/hivestack/certs

# CA certificate
openssl req -x509 -newkey rsa:4096 -sha256 -days 3650 \
    -nodes -keyout ca.key -out ca.crt \
    -subj "/CN=HiveStack CA/O=Maddy AI Consultancy"

# Manager server certificate
openssl req -newkey rsa:2048 -nodes -keyout manager.key -out manager.csr \
    -subj "/CN=hivestack-manager/O=HiveStack"
openssl x509 -req -in manager.csr -CA ca.crt -CAkey ca.key -CAcreateserial \
    -out manager.crt -days 825 -sha256 \
    -extfile <(echo -e "subjectAltName=DNS:hivestack-manager,DNS:localhost,IP:127.0.0.1\nextendedKeyUsage=serverAuth")

# Node agent client certificate
openssl req -newkey rsa:2048 -nodes -keyout node-client.key -out node-client.csr \
    -subj "/CN=hivestack-node-client/O=HiveStack"
openssl x509 -req -in node-client.csr -CA ca.crt -CAkey ca.key -CAcreateserial \
    -out node-client.crt -days 825 -sha256 \
    -extfile <(echo -e "extendedKeyUsage=clientAuth")

# Node agent server certificate
openssl req -newkey rsa:2048 -nodes -keyout node-server.key -out node-server.csr \
    -subj "/CN=hivestack-node/O=HiveStack"
openssl x509 -req -in node-server.csr -CA ca.crt -CAkey ca.key -CAcreateserial \
    -out node-server.crt -days 825 -sha256 \
    -extfile <(echo -e "subjectAltName=DNS:hivestack-node,DNS:localhost,IP:127.0.0.1\nextendedKeyUsage=serverAuth")

# Set permissions
chmod 600 *.key
chmod 644 *.crt *.csr *.srl
chown -R hivestack:hivestack /etc/hivestack/certs

# 3. Create default HiveStack configuration
echo "[3/10] Creating default configuration..."
cat > /etc/hivestack/manager.yaml <<'EOF'
# HiveStack Manager Configuration
server:
  host: "0.0.0.0"
  port: 8080
  tls_enabled: true
  tls_cert_file: "/etc/hivestack/certs/manager.crt"
  tls_key_file: "/etc/hivestack/certs/manager.key"

database:
  dsn: "postgres://hivestack:hivestack-secure-password-change-me@localhost:5432/hivestack?sslmode=disable"

auth:
  jwt_secret: "change-this-in-production-use-strong-random-secret"
  token_expiry: "24h"

grpc:
  host: "0.0.0.0"
  port: 8443
  tls_enabled: true
  tls_cert_file: "/etc/hivestack/certs/manager.crt"
  tls_key_file: "/etc/hivestack/certs/manager.key"
  tls_ca_file: "/etc/hivestack/certs/ca.crt"

logging:
  level: "info"
  format: "json"
  output: "/var/log/hivestack/manager.log"
EOF

cat > /etc/hivestack/node.yaml <<'EOF'
# HiveStack Node Agent Configuration
manager_address: "hivestack-manager:8443"
grpc_address: "0.0.0.0:9090"
node_id: ""  # Auto-generated from hostname
heartbeat_interval: "30s"
libvirt_uri: "qemu:///system"

tls:
  cert_file: "/etc/hivestack/certs/node-server.crt"
  key_file: "/etc/hivestack/certs/node-server.key"
  ca_file: "/etc/hivestack/certs/ca.crt"

logging:
  level: "info"
  format: "json"
  output: "/var/log/hivestack/node.log"
EOF

chown hivestack:hivestack /etc/hivestack/manager.yaml /etc/hivestack/node.yaml
chmod 640 /etc/hivestack/manager.yaml /etc/hivestack/node.yaml

# 4. Copy HiveStack binaries (if not already in place)
echo "[4/10] Installing HiveStack binaries..."
if [ ! -f /opt/hivestack/bin/hive-manager ]; then
    # In production, these would be installed from RPM or copied from build
    # For now, create placeholder scripts
    mkdir -p /opt/hivestack/bin
    
    cat > /opt/hivestack/bin/hive-manager <<'BIN'
#!/bin/bash
# Placeholder for hive-manager binary
# In production, replace with actual compiled binary
echo "HiveStack Manager - not installed (placeholder)"
echo "Build from source: go build -o hive-manager ./cmd/hive-manager"
exit 1
BIN

    cat > /opt/hivestack/bin/hive-node <<'BIN'
#!/bin/bash
# Placeholder for hive-node binary
echo "HiveStack Node Agent - not installed (placeholder)"
echo "Build from source: go build -o hive-node ./node"
exit 1
BIN

    cat > /opt/hivestack/bin/hive <<'BIN'
#!/bin/bash
# Placeholder for hive CLI
echo "HiveStack CLI - not installed (placeholder)"
echo "Build from source: go build -o hive ./cmd/hive"
exit 1
BIN

    chmod +x /opt/hivestack/bin/hive-manager /opt/hivestack/bin/hive-node /opt/hivestack/bin/hive
fi

# 5. Install systemd service files
echo "[5/10] Installing systemd service files..."
mkdir -p /etc/systemd/system

cat > /etc/systemd/system/hivestack-manager.service <<'EOF'
[Unit]
Description=HiveStack Manager
Documentation=https://github.com/maddydevel/HiveStack
After=network.target postgresql.service
Wants=postgresql.service

[Service]
Type=simple
User=hivestack
Group=hivestack
Environment=HIVESTACK_CONFIG=/etc/hivestack/manager.yaml
ExecStart=/opt/hivestack/bin/hive-manager --config /etc/hivestack/manager.yaml
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=hivestack-manager

# Security hardening
ProtectSystem=strict
ProtectHome=true
NoNewPrivileges=yes
PrivateTmp=true
PrivateDevices=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX
RestrictNamespaces=yes
LockPersonality=yes
MemoryDenyWriteExecute=yes
RestrictRealtime=yes
RestrictSUIDSGID=yes

# Capabilities for KVM management
CapabilityBoundingSet=CAP_DAC_OVERRIDE CAP_SETGID CAP_SETUID CAP_NET_BIND_SERVICE
AmbientCapabilities=CAP_DAC_OVERRIDE CAP_SETGID CAP_SETUID CAP_NET_BIND_SERVICE

LimitNOFILE=65536
LimitNPROC=32768
LimitMEMLOCK=infinity
TasksMax=infinity

[Install]
WantedBy=multi-user.target
EOF

cat > /etc/systemd/system/hivestack-node.service <<'EOF'
[Unit]
Description=HiveStack Node Agent
Documentation=https://github.com/maddydevel/HiveStack
After=network.target libvirtd.service
Wants=libvirtd.service

[Service]
Type=simple
User=hivestack-node
Group=hivestack-node
Environment=HIVESTACK_CONFIG=/etc/hivestack/node.yaml
ExecStart=/opt/hivestack/bin/hive-node --config /etc/hivestack/node.yaml
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=hivestack-node

# Security hardening
ProtectSystem=strict
ProtectHome=true
NoNewPrivileges=yes
PrivateTmp=true
PrivateDevices=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX
RestrictNamespaces=yes
LockPersonality=yes
MemoryDenyWriteExecute=yes
RestrictRealtime=yes
RestrictSUIDSGID=yes

# Capabilities for KVM management
CapabilityBoundingSet=CAP_DAC_OVERRIDE CAP_SETGID CAP_SETUID CAP_NET_BIND_SERVICE CAP_SYS_ADMIN CAP_SYS_RESOURCE
AmbientCapabilities=CAP_DAC_OVERRIDE CAP_SETGID CAP_SETUID CAP_NET_BIND_SERVICE CAP_SYS_ADMIN CAP_SYS_RESOURCE

LimitNOFILE=65536
LimitNPROC=32768
LimitMEMLOCK=infinity
TasksMax=infinity

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload

# 6. Configure libvirt for HiveStack
echo "[6/10] Configuring libvirt..."
# Enable and start libvirtd
systemctl enable libvirtd
systemctl start libvirtd

# Wait for libvirtd to be ready
for i in {1..30}; do
    if virsh -c qemu:///system list >/dev/null 2>&1; then
        break
    fi
    sleep 1
done

# Create default storage pool for HiveStack
virsh -c qemu:///system pool-define-as --name hivestack --type dir --target /var/lib/libvirt/images/hivestack 2>/dev/null || true
virsh -c qemu:///system pool-start hivestack 2>/dev/null || true
virsh -c qemu:///system pool-autostart hivestack 2>/dev/null || true

# Create default network for HiveStack VMs
cat > /tmp/hivestack-network.xml <<'NETXML'
<network>
  <name>hivestack</name>
  <forward mode='nat'>
    <nat>
      <port start='1024' end='65535'/>
    </nat>
  </forward>
  <bridge name='virbr-hivestack' stp='on' delay='0'/>
  <mac address='52:54:00:hs:00:01'/>
  <ip address='192.168.100.1' netmask='255.255.255.0'>
    <dhcp>
      <range start='192.168.100.10' end='192.168.100.254'/>
    </dhcp>
  </ip>
</network>
NETXML

virsh -c qemu:///system net-define /tmp/hivestack-network.xml 2>/dev/null || true
virsh -c qemu:///system net-start hivestack 2>/dev/null || true
virsh -c qemu:///system net-autostart hivestack 2>/dev/null || true

# 7. Configure hugepages for HANA VMs
echo "[7/10] Configuring hugepages..."
# Reserve 2GB hugepages (1024 * 2MB)
echo 1024 > /proc/sys/vm/nr_hugepages
echo "vm.nr_hugepages = 1024" >> /etc/sysctl.d/99-hivestack.conf

# Mount hugetlbfs
mkdir -p /dev/hugepages
mount -t hugetlbfs nodev /dev/hugepages 2>/dev/null || true
echo "nodev /dev/hugepages hugetlbfs defaults 0 0" >> /etc/fstab

# 8. Start HiveStack services
echo "[8/10] Starting HiveStack services..."
systemctl enable hivestack-manager
systemctl enable hivestack-node
systemctl start hivestack-manager
systemctl start hivestack-node

# Wait for services to start
sleep 10

# 9. Verify services are running
echo "[9/10] Verifying services..."
SERVICES=("postgresql" "libvirtd" "hivestack-manager" "hivestack-node" "cockpit" "prometheus-node-exporter")
for svc in "${SERVICES[@]}"; do
    if systemctl is-active --quiet "$svc"; then
        echo "  ✓ $svc is running"
    else
        echo "  ✗ $svc is NOT running"
        systemctl status "$svc" --no-pager
    fi
done

# 10. Test API endpoint
echo "[10/10] Testing API endpoint..."
sleep 5
if curl -k -s -o /dev/null -w "%{http_code}" https://localhost:8080/api/v1/health 2>/dev/null | grep -q "200"; then
    echo "  ✓ API health check passed"
else
    echo "  ⚠ API health check failed (may need manual verification)"
fi

# Remove first-boot marker
rm -f "${FIRST_BOOT_MARKER}"

echo "========================================="
echo "HiveStack Appliance First Boot Setup Complete"
echo "Date: $(date)"
echo "========================================="
echo ""
echo "Next steps:"
echo "1. Change default passwords in /etc/hivestack/*.yaml"
echo "2. Access Manager API at https://<ip>:8080"
echo "3. Access Cockpit at https://<ip>:9090"
echo "4. Register additional nodes via API"
echo ""
echo "Logs: ${SETUP_LOG}"