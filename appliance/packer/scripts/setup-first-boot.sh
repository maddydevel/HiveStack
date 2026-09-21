#!/bin/bash
set -euo pipefail

echo "=== Setting up first-boot ==="

cat > /etc/systemd/system/hivestack-first-boot.service <<EOF
[Unit]
Description=HiveStack First Boot Setup
After=network.target postgresql.service
Before=hivestack-manager.service
ConditionPathExists=!/etc/hivestack/.first-boot-complete

[Service]
Type=oneshot
ExecStart=/opt/hivestack/scripts/first-boot.sh
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
EOF

cp /tmp/HiveStack/appliance/scripts/first-boot.sh /opt/hivestack/scripts/ 2>/dev/null || true
chmod +x /opt/hivestack/scripts/first-boot.sh 2>/dev/null || true

systemctl daemon-reload
systemctl enable hivestack-first-boot

echo "First-boot setup complete."
