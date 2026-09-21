#!/bin/bash
set -euo pipefail

HIVESTACK_VERSION="${HIVESTACK_VERSION:-0.1.0}"
HIVESTACK_URL="https://github.com/maddydevel/HiveStack/releases/download/v${HIVESTACK_VERSION}/hivestack-${HIVESTACK_VERSION}-linux-amd64.tar.gz"

echo "=== Installing HiveStack ==="

cd /tmp
if wget -q "$HIVESTACK_URL" -O hivestack.tar.gz 2>/dev/null; then
    tar -xzf hivestack.tar.gz -C /opt/hivestack/bin/
    chmod +x /opt/hivestack/bin/*
else
    echo "WARNING: Could not download release. Creating stubs."
    echo '#!/bin/bash' > /opt/hivestack/bin/hive-manager
    echo 'echo "HiveStack Manager stub"' >> /opt/hivestack/bin/hive-manager
    chmod +x /opt/hivestack/bin/hive-manager
    ln -sf /opt/hivestack/bin/hive-manager /opt/hivestack/bin/hive-node 2>/dev/null || true
fi

# Install systemd services
cat > /etc/systemd/system/hivestack-manager.service <<EOF
[Unit]
Description=HiveStack Manager
After=network.target postgresql.service
Requires=postgresql.service

[Service]
Type=simple
User=hivestack
ExecStart=/opt/hivestack/bin/hive-manager --config /etc/hivestack/manager.yaml
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

cat > /etc/systemd/system/hivestack-node.service <<EOF
[Unit]
Description=HiveStack Node Agent
After=network.target libvirtd.service
Requires=libvirtd.service

[Service]
Type=simple
ExecStart=/opt/hivestack/bin/hive-node --config /etc/hivestack/node.yaml
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable hivestack-manager hivestack-node

echo "HiveStack installation complete."
