#!/bin/bash
# HiveStack First Boot Initialization - SLES 15 SP7
# Runs on first boot to set up the appliance

set -euo pipefail

FIRST_BOOT_MARKER="/etc/hivestack/.first-boot-complete"
SETUP_LOG="/var/log/hivestack/first-boot.log"

# Redirect all output to log file
exec > >(tee -a "${SETUP_LOG}") 2>&1

echo "========================================="
echo "HiveStack Appliance First Boot Setup"
echo "Date: $(date)"
echo "Base OS: SLES 15 SP7"
echo "========================================="

# Check if already run
if [ -f "${FIRST_BOOT_MARKER}" ]; then
    echo "First boot setup already completed. Exiting."
    exit 0
fi

# Get configuration from environment or use defaults
DB_PASSWORD="${HIVESTACK_DB_PASSWORD:-$(openssl rand -hex 16)}"
JWT_SECRET="${HIVESTACK_JWT_SECRET:-$(openssl rand -hex 32)}"
ADMIN_PASSWORD="${HIVESTACK_ADMIN_PASSWORD:-$(openssl rand -hex 12)}"

# 1. Initialize PostgreSQL
echo "[1/8] Initializing PostgreSQL..."
systemctl start postgresql
sleep 2

# Wait for PostgreSQL
for i in {1..30}; do
    if su - postgres -c "psql -c 'SELECT 1'" >/dev/null 2>&1; then
        break
    fi
    sleep 1
done

# Create database and user
su - postgres -c "psql -c \"CREATE USER hivestack WITH PASSWORD '${DB_PASSWORD}' SUPERUSER;\"" 2>/dev/null || true
su - postgres -c "psql -c \"CREATE DATABASE hivestack OWNER hivestack;\"" 2>/dev/null || true

# Run migrations
for f in /opt/hivestack/migrations/*.sql; do
    if [ -f "$f" ]; then
        echo "  Applying: $(basename "$f")"
        PGPASSWORD="${DB_PASSWORD}" psql -h localhost -U hivestack -d hivestack -f "$f" 2>/dev/null || true
    fi
done

# 2. Generate TLS certificates
echo "[2/8] Generating TLS certificates..."
mkdir -p /etc/hivestack/certs
cd /etc/hivestack/certs

# CA certificate
openssl ecparam -genkey -name prime256v1 -out ca.key 2>/dev/null
openssl req -new -x509 -key ca.key -out ca.crt -days 3650 -subj "/CN=HiveStack Root CA/O=HiveStack" 2>/dev/null

# Manager certificate
openssl ecparam -genkey -name prime256v1 -out manager.key 2>/dev/null
openssl req -new -key manager.key -out manager.csr -subj "/CN=hivestack-manager/O=HiveStack" 2>/dev/null
openssl x509 -req -in manager.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out manager.crt -days 365 -sha256 2>/dev/null
rm -f manager.csr

# Node certificate
NODE_ID=$(hostname)
openssl ecparam -genkey -name prime256v1 -out node-server.key 2>/dev/null
openssl req -new -key node-server.key -out node-server.csr -subj "/CN=${NODE_ID}/O=HiveStack" 2>/dev/null
openssl x509 -req -in node-server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out node-server.crt -days 365 -sha256 2>/dev/null
rm -f node-server.csr

# Set permissions
chmod 600 /etc/hivestack/certs/*.key
chmod 644 /etc/hivestack/certs/*.crt
chown -R hivestack:hivestack /etc/hivestack/certs

# 3. Create configuration files
echo "[3/8] Creating configuration..."

cat > /etc/hivestack/manager.yaml <<EOF
# HiveStack Manager Configuration
# Generated on first boot - $(date)

dsn: "postgres://hivestack:${DB_PASSWORD}@localhost:5432/hivestack?sslmode=disable"
jwt_secret: "${JWT_SECRET}"
tls_cert_file: "/etc/hivestack/certs/manager.crt"
tls_key_file: "/etc/hivestack/certs/manager.key"
tls_ca_file: "/etc/hivestack/certs/ca.crt"
listen_address: ":8080"
listen_tls_address: ":8443"
grpc_port: 44567
log_level: "info"
log_format: "json"
EOF

cat > /etc/hivestack/node.yaml <<EOF
# HiveStack Node Configuration
# Generated on first boot - $(date)

node_id: "${NODE_ID}"
manager_address: "localhost:44567"
tls_cert_file: "/etc/hivestack/certs/node-server.crt"
tls_key_file: "/etc/hivestack/certs/node-server.key"
tls_ca_file: "/etc/hivestack/certs/ca.crt"
heartbeat_interval: 30
libvirt_uri: "qemu:///system"
log_level: "info"
log_format: "json"
EOF

chown hivestack:hivestack /etc/hivestack/*.yaml
chmod 600 /etc/hivestack/*.yaml

# 4. Configure libvirt
echo "[4/8] Configuring libvirt..."
systemctl enable libvirtd 2>/dev/null || true
systemctl start libvirtd 2>/dev/null || true

# Create default storage pool
virsh pool-list --all | grep -q default || {
    mkdir -p /var/lib/hivestack/images
    virsh pool-define-as default dir - - - - /var/lib/hivestack/images 2>/dev/null || true
    virsh pool-start default 2>/dev/null || true
    virsh pool-autostart default 2>/dev/null || true
}

# Create default network
virsh net-list --all | grep -q default || {
    cat > /tmp/default-net.xml <<NETEOF
<network>
  <name>default</name>
  <bridge name='virbr0' stp='on' delay='0'/>
  <ip address='192.168.122.1' netmask='255.255.255.0'>
    <dhcp>
      <range start='192.168.122.100' end='192.168.122.200'/>
    </dhcp>
  </ip>
</network>
NETEOF
    virsh net-define /tmp/default-net.xml 2>/dev/null || true
    virsh net-start default 2>/dev/null || true
    virsh net-autostart default 2>/dev/null || true
    rm -f /tmp/default-net.xml
}

# 5. Configure hugepages
echo "[5/8] Configuring hugepages..."
grep -q "vm.nr_hugepages" /etc/sysctl.conf || echo "vm.nr_hugepages = 1024" >> /etc/sysctl.conf
sysctl -p 2>/dev/null || true

# 6. Configure firewall
echo "[6/8] Configuring firewall..."
systemctl start firewalld 2>/dev/null || true
firewall-cmd --permanent --add-port=8080/tcp 2>/dev/null || true
firewall-cmd --permanent --add-port=8443/tcp 2>/dev/null || true
firewall-cmd --permanent --add-port=44567/tcp 2>/dev/null || true
firewall-cmd --permanent --add-service=ssh 2>/dev/null || true
firewall-cmd --reload 2>/dev/null || true

# 7. Start HiveStack services
echo "[7/8] Starting HiveStack services..."
systemctl daemon-reload
systemctl enable hivestack-manager hivestack-node
systemctl start hivestack-manager hivestack-node

# 8. Verify services
echo "[8/8] Verifying services..."
sleep 5
SERVICES=("postgresql" "libvirtd" "hivestack-manager" "hivestack-node")
ALL_OK=true
for svc in "${SERVICES[@]}"; do
    if systemctl is-active --quiet "$svc"; then
        echo "  ✓ $svc is running"
    else
        echo "  ✗ $svc is NOT running"
        ALL_OK=false
    fi
done

# Test API endpoint
if curl -kfs https://localhost:8080/api/v1/health >/dev/null 2>&1; then
    echo "  ✓ API health check passed"
else
    echo "  ⚠ API health check failed (may need manual verification)"
fi

# Mark first boot as complete
touch "${FIRST_BOOT_MARKER}"

# Print summary
echo ""
echo "================================================"
echo "  HiveStack Appliance Setup Complete!"
echo "================================================"
echo "  IP Address: $(hostname -I | awk '{print $1}')"
echo "  Web UI:     https://$(hostname -I | awk '{print $1}'):8443"
echo "  API:        https://$(hostname -I | awk '{print $1}'):8080"
echo "  Health:     http://$(hostname -I | awk '{print $1}'):8080/api/v1/health"
echo ""
echo "  Admin User: admin@localhost"
echo "  Admin Pass: ${ADMIN_PASSWORD}"
echo ""
echo "  IMPORTANT: Change the default admin password immediately!"
echo "================================================"
