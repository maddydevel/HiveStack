#!/bin/bash
# HiveStack First Boot Initialization
# Runs on first boot to set up the appliance
#
# This script:
# 1. Initializes PostgreSQL database
# 2. Generates TLS certificates for mTLS
# 3. Creates default configuration files
# 4. Sets up libvirt storage and networking
# 5. Configures hugepages
# 6. Starts and verifies all services

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

# Get configuration from environment or use defaults
DB_PASSWORD="${HIVESTACK_DB_PASSWORD:-hivestack-secure-password-change-me}"
JWT_SECRET="${HIVESTACK_JWT_SECRET:-change-this-in-production-use-strong-random-secret}"
MANAGER_HOST="${HIVESTACK_MANAGER_HOST:-hivestack-manager}"
NODE_HOSTNAME="$(hostname)"

# 1. Initialize PostgreSQL
echo "[1/10] Initializing PostgreSQL..."
systemctl start postgresql
sleep 5

# Wait for PostgreSQL to be ready
for i in {1..30}; do
    if sudo -u postgres psql -c "SELECT 1" >/dev/null 2>&1; then
        break
    fi
    sleep 1
done

# Create HiveStack database and user
sudo -u postgres psql <<PGSQL
DO \$\$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'hivestack') THEN
        CREATE USER hivestack WITH ENCRYPTED PASSWORD '${DB_PASSWORD}';
    ELSE
        ALTER USER hivestack WITH ENCRYPTED PASSWORD '${DB_PASSWORD}';
    END IF;
END
\$\$;

SELECT 'CREATE DATABASE hivestack OWNER hivestack'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'hivestack')\gexec

GRANT ALL PRIVILEGES ON DATABASE hivestack TO hivestack;
PGSQL

# Run database migrations if hive-manager supports it
if [ -f /opt/hivestack/bin/hive-manager ]; then
    echo "Running database migrations..."
    /opt/hivestack/bin/hive-manager migrate up 2>/dev/null || echo "Migration skipped (not available)"
fi

# 2. Generate TLS certificates for mTLS
echo "[2/10] Generating TLS certificates..."
mkdir -p /etc/hivestack/certs
cd /etc/hivestack/certs

# Generate CA if not present
if [ ! -f ca.crt ]; then
    openssl ecparam -genkey -name prime256v1 -out ca.key
    openssl req -new -x509 -key ca.key -out ca.crt \
        -days 3650 -subj "/CN=HiveStack Root CA/O=HiveStack"
    chmod 600 ca.key
    chmod 644 ca.crt
fi

# Generate Manager server certificate
if [ ! -f manager.crt ]; then
    openssl ecparam -genkey -name prime256v1 -out manager.key
    openssl req -new -key manager.key -out manager.csr \
        -subj "/CN=${MANAGER_HOST}/O=HiveStack"
    openssl x509 -req -in manager.csr -CA ca.crt -CAkey ca.key -CAcreateserial \
        -out manager.crt -days 365 -sha256 \
        -extfile <(printf "subjectAltName=DNS:${MANAGER_HOST},DNS:localhost,IP:127.0.0.1")
    rm -f manager.csr
    chmod 600 manager.key
    chmod 644 manager.crt
fi

# Generate Node server certificate
if [ ! -f node-server.crt ]; then
    openssl ecparam -genkey -name prime256v1 -out node-server.key
    openssl req -new -key node-server.key -out node-server.csr \
        -subj "/CN=${NODE_HOSTNAME}/O=HiveStack"
    openssl x509 -req -in node-server.csr -CA ca.crt -CAkey ca.key -CAcreateserial \
        -out node-server.crt -days 365 -sha256 \
        -extfile <(printf "subjectAltName=DNS:${NODE_HOSTNAME},DNS:localhost,IP:127.0.0.1")
    rm -f node-server.csr
    chmod 600 node-server.key
    chmod 644 node-server.crt
fi

# Generate Node client certificate
if [ ! -f node-client.crt ]; then
    openssl ecparam -genkey -name prime256v1 -out node-client.key
    openssl req -new -key node-client.key -out node-client.csr \
        -subj "/CN=${NODE_HOSTNAME}-client/O=HiveStack"
    openssl x509 -req -in node-client.csr -CA ca.crt -CAkey ca.key -CAcreateserial \
        -out node-client.crt -days 365 -sha256 \
        -extfile <(printf "extendedKeyUsage=clientAuth")
    rm -f node-client.csr
    chmod 600 node-client.key
    chmod 644 node-client.crt
fi

# Set permissions
chown -R hivestack:hivestack /etc/hivestack/certs
echo "TLS certificates generated."

# 3. Create default HiveStack configuration
echo "[3/10] Creating default configuration..."

# Generate manager.yaml if not present
if [ ! -f /etc/hivestack/manager.yaml ]; then
    cat > /etc/hivestack/manager.yaml <<EOF
# HiveStack Manager Configuration
# Generated on first boot - $(date)

server:
  host: "0.0.0.0"
  port: 8080
  tls_enabled: true
  tls_cert_file: "/etc/hivestack/certs/manager.crt"
  tls_key_file: "/etc/hivestack/certs/manager.key"

grpc:
  host: "0.0.0.0"
  port: 8443
  tls_enabled: true
  tls_cert_file: "/etc/hivestack/certs/manager.crt"
  tls_key_file: "/etc/hivestack/certs/manager.key"
  tls_ca_file: "/etc/hivestack/certs/ca.crt"

database:
  dsn: "postgres://hivestack:${DB_PASSWORD}@localhost:5432/hivestack?sslmode=disable"

auth:
  jwt_secret: "${JWT_SECRET}"
  token_expiry: "24h"

logging:
  level: "info"
  format: "json"
  output: "/var/log/hivestack/manager.log"
EOF
fi

# Generate node.yaml if not present
if [ ! -f /etc/hivestack/node.yaml ]; then
    cat > /etc/hivestack/node.yaml <<EOF
# HiveStack Node Agent Configuration
# Generated on first boot - $(date)

manager_address: "${MANAGER_HOST}:8443"
grpc_address: "0.0.0.0:9090"
node_id: ""
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
fi

chown hivestack:hivestack /etc/hivestack/manager.yaml /etc/hivestack/node.yaml
chmod 640 /etc/hivestack/manager.yaml /etc/hivestack/node.yaml

# 4. Install HiveStack binaries (if not already in place)
echo "[4/10] Checking HiveStack binaries..."
if [ ! -f /opt/hivestack/bin/hive-manager ]; then
    echo "WARNING: hive-manager binary not found at /opt/hivestack/bin/hive-manager"
    echo "Build from source: go build -o hive-manager ./cmd/hive-manager"
fi

# 5. Install systemd service files
echo "[5/10] Installing systemd service files..."
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [ -f "${SCRIPT_DIR}/../systemd/hivestack-manager.service" ]; then
    cp "${SCRIPT_DIR}/../systemd/hivestack-manager.service" /etc/systemd/system/
fi

if [ -f "${SCRIPT_DIR}/../systemd/hivestack-node.service" ]; then
    cp "${SCRIPT_DIR}/../systemd/hivestack-node.service" /etc/systemd/system/
fi

systemctl daemon-reload

# 6. Configure libvirt for HiveStack
echo "[6/10] Configuring libvirt..."
systemctl enable libvirtd
systemctl start libvirtd

# Wait for libvirtd to be ready
for i in {1..30}; do
    if virsh -c qemu:///system list >/dev/null 2>&1; then
        break
    fi
    sleep 1
done

# Create default storage pool
if ! virsh -c qemu:///system pool-info hivestack >/dev/null 2>&1; then
    mkdir -p /var/lib/libvirt/images/hivestack
    virsh -c qemu:///system pool-define-as --name hivestack --type dir --target /var/lib/libvirt/images/hivestack
    virsh -c qemu:///system pool-start hivestack
    virsh -c qemu:///system pool-autostart hivestack
fi

# Create default network
if ! virsh -c qemu:///system net-info hivestack >/dev/null 2>&1; then
    cat > /tmp/hivestack-network.xml <<'NETXML'
<network>
  <name>hivestack</name>
  <forward mode='nat'>
    <nat>
      <port start='1024' end='65535'/>
    </nat>
  </forward>
  <bridge name='virbr-hivestack' stp='on' delay='0'/>
  <ip address='192.168.100.1' netmask='255.255.255.0'>
    <dhcp>
      <range start='192.168.100.10' end='192.168.100.254'/>
    </dhcp>
  </ip>
</network>
NETXML
    virsh -c qemu:///system net-define /tmp/hivestack-network.xml
    virsh -c qemu:///system net-start hivestack
    virsh -c qemu:///system net-autostart hivestack
    rm -f /tmp/hivestack-network.xml
fi

# 7. Configure hugepages for HANA VMs
echo "[7/10] Configuring hugepages..."
HUGEPAGES="${HIVESTACK_HUGEPAGES:-1024}"
echo "${HUGEPAGES}" > /proc/sys/vm/nr_hugepages 2>/dev/null || echo "WARNING: Could not set hugepages (may need host access)"
echo "vm.nr_hugepages = ${HUGEPAGES}" > /etc/sysctl.d/99-hivestack-hugepages.conf

# Mount hugetlbfs
mkdir -p /dev/hugepages
mount -t hugetlbfs nodev /dev/hugepages 2>/dev/null || true
grep -q "hugetlbfs" /etc/fstab || echo "nodev /dev/hugepages hugetlbfs defaults 0 0" >> /etc/fstab

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
SERVICES=("postgresql" "libvirtd" "hivestack-manager" "hivestack-node")
ALL_OK=true
for svc in "${SERVICES[@]}"; do
    if systemctl is-active --quiet "$svc"; then
        echo "  ✓ $svc is running"
    else
        echo "  ✗ $svc is NOT running"
        systemctl status "$svc" --no-pager || true
        ALL_OK=false
    fi
done

# 10. Test API endpoint
echo "[10/10] Testing API endpoint..."
sleep 5
if curl -kfs https://localhost:8080/api/v1/health >/dev/null 2>&1; then
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
echo "  1. Change default passwords in /etc/hivestack/*.yaml"
echo "  2. Access Manager API at https://<ip>:8080"
echo "  3. Access Cockpit at https://<ip>:9090"
echo "  4. Register additional nodes via API"
echo ""
echo "Logs: ${SETUP_LOG}"

if [ "$ALL_OK" = false ]; then
    echo ""
    echo "WARNING: Some services failed to start. Check logs with:"
    echo "  journalctl -u hivestack-manager -f"
    echo "  journalctl -u hivestack-node -f"
fi