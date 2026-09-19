#!/bin/bash
# HiveStack Container Entrypoint
# Handles initialization and launches the appropriate component

set -euo pipefail

HIVESTACK_CONFIG="${HIVESTACK_CONFIG:-/etc/hivestack/manager.yaml}"
COMPONENT="${1:-manager}"

log() {
    echo "[hivestack-entrypoint] $*"
}

# Generate TLS certificates if not present
generate_certs() {
    local cert_dir="/etc/hivestack/certs"
    mkdir -p "${cert_dir}"

    if [ ! -f "${cert_dir}/ca.crt" ]; then
        log "Generating CA certificate..."
        openssl ecparam -genkey -name prime256v1 -out "${cert_dir}/ca.key"
        openssl req -new -x509 -key "${cert_dir}/ca.key" -out "${cert_dir}/ca.crt" \
            -days 3650 -subj "/CN=HiveStack Root CA/O=HiveStack"
        chmod 600 "${cert_dir}/ca.key"
        chmod 644 "${cert_dir}/ca.crt"
    fi

    if [ ! -f "${cert_dir}/manager.crt" ]; then
        log "Generating Manager server certificate..."
        openssl ecparam -genkey -name prime256v1 -out "${cert_dir}/manager.key"
        openssl req -new -key "${cert_dir}/manager.key" -out "${cert_dir}/manager.csr" \
            -subj "/CN=hivestack-manager/O=HiveStack"
        openssl x509 -req -in "${cert_dir}/manager.csr" -CA "${cert_dir}/ca.crt" \
            -CAkey "${cert_dir}/ca.key" -CAcreateserial -out "${cert_dir}/manager.crt" \
            -days 365 -sha256 \
            -extfile <(printf "subjectAltName=DNS:hivestack-manager,DNS:localhost,IP:127.0.0.1")
        rm -f "${cert_dir}/manager.csr"
        chmod 600 "${cert_dir}/manager.key"
        chmod 644 "${cert_dir}/manager.crt"
    fi

    if [ ! -f "${cert_dir}/node-server.crt" ]; then
        log "Generating Node server certificate..."
        local node_id
        node_id="$(hostname)"
        openssl ecparam -genkey -name prime256v1 -out "${cert_dir}/node-server.key"
        openssl req -new -key "${cert_dir}/node-server.key" -out "${cert_dir}/node-server.csr" \
            -subj "/CN=${node_id}/O=HiveStack"
        openssl x509 -req -in "${cert_dir}/node-server.csr" -CA "${cert_dir}/ca.crt" \
            -CAkey "${cert_dir}/ca.key" -CAcreateserial -out "${cert_dir}/node-server.crt" \
            -days 365 -sha256 \
            -extfile <(printf "subjectAltName=DNS:${node_id},DNS:localhost,IP:127.0.0.1")
        rm -f "${cert_dir}/node-server.csr"
        chmod 600 "${cert_dir}/node-server.key"
        chmod 644 "${cert_dir}/node-server.crt"
    fi

    if [ ! -f "${cert_dir}/node-client.crt" ]; then
        log "Generating Node client certificate..."
        local node_id
        node_id="$(hostname)"
        openssl ecparam -genkey -name prime256v1 -out "${cert_dir}/node-client.key"
        openssl req -new -key "${cert_dir}/node-client.key" -out "${cert_dir}/node-client.csr" \
            -subj "/CN=${node_id}-client/O=HiveStack"
        openssl x509 -req -in "${cert_dir}/node-client.csr" -CA "${cert_dir}/ca.crt" \
            -CAkey "${cert_dir}/ca.key" -CAcreateserial -out "${cert_dir}/node-client.crt" \
            -days 365 -sha256 \
            -extfile <(printf "extendedKeyUsage=clientAuth")
        rm -f "${cert_dir}/node-client.csr"
        chmod 600 "${cert_dir}/node-client.key"
        chmod 644 "${cert_dir}/node-client.crt"
    fi

    chown -R hivestack:hivestack "${cert_dir}"
}

# Wait for PostgreSQL to be ready
wait_for_postgres() {
    local max_attempts=30
    local attempt=0

    while [ $attempt -lt $max_attempts ]; do
        if pg_isready -h "${POSTGRES_HOST:-localhost}" -p "${POSTGRES_PORT:-5432}" -U "${POSTGRES_USER:-hivestack}" >/dev/null 2>&1; then
            log "PostgreSQL is ready"
            return 0
        fi
        attempt=$((attempt + 1))
        log "Waiting for PostgreSQL (attempt ${attempt}/${max_attempts})..."
        sleep 2
    done

    log "WARNING: PostgreSQL not available, continuing anyway..."
    return 0
}

# Setup configuration from environment
setup_config() {
    local config_file="$1"

    if [ ! -f "${config_file}" ]; then
        log "Config file not found: ${config_file}"
        return 1
    fi

    # Update DSN with environment variables
    if [ -n "${POSTGRES_HOST:-}" ] && [ -n "${POSTGRES_PASSWORD:-}" ]; then
        local dsn="postgres://${POSTGRES_USER:-hivestack}:${POSTGRES_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT:-5432}/${POSTGRES_DB:-hivestack}?sslmode=disable"
        # Use sed to update the DSN in the config
        sed -i "s|dsn:.*|dsn: \"${dsn}\"|" "${config_file}" 2>/dev/null || true
    fi

    # Update JWT secret
    if [ -n "${JWT_SECRET:-}" ]; then
        sed -i "s|jwt_secret:.*|jwt_secret: \"${JWT_SECRET}\"|" "${config_file}" 2>/dev/null || true
    fi

    # Update manager address for node
    if [ -n "${MANAGER_ADDRESS:-}" ] && [ "${COMPONENT}" = "node" ]; then
        sed -i "s|manager_address:.*|manager_address: \"${MANAGER_ADDRESS}\"|" "${config_file}" 2>/dev/null || true
    fi

    # Update node ID
    if [ -n "${NODE_ID:-}" ] && [ "${COMPONENT}" = "node" ]; then
        sed -i "s|node_id:.*|node_id: \"${NODE_ID}\"|" "${config_file}" 2>/dev/null || true
    fi
}

# Main
main() {
    log "Starting HiveStack ${COMPONENT}"
    log "Config: ${HIVESTACK_CONFIG}"

    # Generate certificates
    generate_certs

    # Wait for dependencies
    case "${COMPONENT}" in
        manager)
            wait_for_postgres
            setup_config "/etc/hivestack/manager.yaml"
            log "Starting hive-manager..."
            exec /opt/hivestack/bin/hive-manager --config /etc/hivestack/manager.yaml
            ;;
        node)
            setup_config "/etc/hivestack/node.yaml"
            log "Starting hive-node..."
            exec /opt/hivestack/bin/hive-node --config /etc/hivestack/node.yaml
            ;;
        *)
            log "Unknown component: ${COMPONENT}"
            log "Usage: $0 [manager|node]"
            exit 1
            ;;
    esac
}

main "$@"
