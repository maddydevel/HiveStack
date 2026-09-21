#!/bin/bash
set -euo pipefail

export DEBIAN_FRONTEND=noninteractive

echo "=== HiveStack Base Setup ==="

# Update system
apt-get update
apt-get upgrade -y
apt-get install -y \
    curl \
    wget \
    gnupg2 \
    software-properties-common \
    ca-certificates \
    apt-transport-https \
    jq \
    net-tools \
    iproute2 \
    bridge-utils \
    vlan \
    openvswitch-switch \
    qemu-guest-agent

# Create hivestack system user
useradd -r -m -s /bin/bash -U hivestack 2>/dev/null || true
echo "hivestack ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/hivestack
chmod 0440 /etc/sudoers.d/hivestack

# Create directory structure
mkdir -p /opt/hivestack/{bin,config,certs,logs,migrations}
mkdir -p /var/lib/hivestack
mkdir -p /var/log/hivestack
mkdir -p /etc/hivestack

chown -R hivestack:hivestack /opt/hivestack /var/lib/hivestack /var/log/hivestack

echo "Base setup complete."
