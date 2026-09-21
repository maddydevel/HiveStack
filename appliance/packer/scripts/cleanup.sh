#!/bin/bash
set -euo pipefail

echo "=== Cleanup ==="

apt-get autoremove -y
apt-get clean

rm -rf /tmp/*
rm -rf /root/.cache
rm -rf /home/hivestack/.cache

# Clean logs
find /var/log -type f -name "*.gz" -delete 2>/dev/null || true
find /var/log -type f -exec truncate -s 0 {} \; 2>/dev/null || true

# Clean SSH host keys (regenerated on first boot)
rm -f /etc/ssh/ssh_host_*

# Clear machine ID
rm -f /etc/machine-id
touch /etc/machine-id

# Clear bash history
history -c 2>/dev/null || true
rm -f /root/.bash_history /home/hivestack/.bash_history 2>/dev/null || true

echo "Cleanup complete."
