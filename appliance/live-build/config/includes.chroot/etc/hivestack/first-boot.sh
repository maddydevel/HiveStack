#!/bin/bash
# Minimal first-boot for live ISO (full version on installed system)
set -euo pipefail

MARKER="/var/lib/hivestack/.first-boot-complete"

if [ -f "$MARKER" ]; then
    exit 0
fi

# Ensure directories exist
mkdir -p /var/lib/hivestack /var/log/hivestack

# Generate random passwords
DB_PASSWORD=$(openssl rand -hex 16)
JWT_SECRET=$(openssl rand -hex 32)

# Update config if it exists
if [ -f /etc/hivestack/manager.yaml ]; then
    sed -i "s|jwt_secret:.*|jwt_secret: \"${JWT_SECRET}\"|" /etc/hivestack/manager.yaml
fi

# Start PostgreSQL if not running
systemctl start postgresql 2>/dev/null || true

# Create database
su - postgres -c "psql -c \"ALTER USER hivestack WITH PASSWORD '${DB_PASSWORD}';\"" 2>/dev/null || true
su - postgres -c "psql -c \"CREATE DATABASE hivestack OWNER hivestack;\"" 2>/dev/null || true

# Start services
systemctl start hivestack-manager hivestack-node 2>/dev/null || true

# Mark complete
touch "$MARKER"

# Print info
echo "HiveStack is starting..."
echo "Access: https://$(hostname -I | awk '{print $1}'):8080"
