#!/bin/bash
set -euo pipefail

echo "=== Setting up networking ==="

apt-get install -y ufw
ufw --force reset
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp
ufw allow 8080/tcp
ufw allow 8443/tcp
ufw allow 44567/tcp
ufw --force enable

echo "Networking setup complete."
