#!/bin/bash
set -euo pipefail

echo "=== Setting up PostgreSQL ==="

apt-get install -y postgresql postgresql-contrib

# Configure PostgreSQL
cat >> /etc/postgresql/*/main/conf.d/hivestack.conf 2>/dev/null <<EOF
listen_addresses = '*'
max_connections = 200
shared_buffers = 1GB
EOF

# Create database and user
su - postgres -c "psql -c \"CREATE USER hivestack WITH PASSWORD 'hivestack';\"" 2>/dev/null || true
su - postgres -c "psql -c \"CREATE DATABASE hivestack OWNER hivestack;\"" 2>/dev/null || true
su - postgres -c "psql -c \"GRANT ALL PRIVILEGES ON DATABASE hivestack TO hivestack;\"" 2>/dev/null || true

systemctl enable postgresql
systemctl start postgresql

echo "PostgreSQL setup complete."
