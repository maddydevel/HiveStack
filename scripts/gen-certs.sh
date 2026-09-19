#!/bin/bash
#
# gen-certs.sh - Generate TLS certificates for HiveStack
#
# This script generates:
#   1. A self-signed Root CA certificate
#   2. A server certificate for the Manager
#   3. A client certificate for each Node Agent
#
# Usage:
#   ./scripts/gen-certs.sh [--ca-dir DIR] [--node-id ID] [--domain DOMAIN]
#
# Environment variables:
#   HIVESTACK_CA_DIR    - Directory to store CA certs (default: /etc/hivestack/tls)
#   HIVESTACK_ORG       - Organization name (default: HiveStack)
#   HIVESTACK_CA_CN     - CA Common Name (default: HiveStack Root CA)
#   HIVESTACK_DOMAIN    - Domain name (default: hivestack.local)
#   HIVESTACK_NODE_ID   - Node ID for client cert (default: auto-generated)

set -euo pipefail

# Defaults
CA_DIR="${HIVESTACK_CA_DIR:-/etc/hivestack/tls}"
ORG="${HIVESTACK_ORG:-HiveStack}"
CA_CN="${HIVESTACK_CA_CN:-HiveStack Root CA}"
DOMAIN="${HIVESTACK_DOMAIN:-hivestack.local}"
NODE_ID="${HIVESTACK_NODE_ID:-}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() { echo -e "${GREEN}[INFO]${NC} $*"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }

# --- Functions ---

generate_ca() {
    log_info "Generating CA private key (ECDSA P-256)..."
    openssl ecparam -genkey -name prime256v1 -noout -out "${CA_KEY}"
    chmod 600 "${CA_KEY}"

    log_info "Generating self-signed CA certificate..."
    openssl req -x509 -new -nodes \
        -key "${CA_KEY}" \
        -sha256 \
        -days 3650 \
        -subj "/O=${ORG}/CN=${CA_CN}" \
        -out "${CA_CERT}"

    chmod 644 "${CA_CERT}"
    log_info "CA certificate: ${CA_CERT}"
}

generate_server_cert() {
    log_info "Generating server private key..."
    openssl ecparam -genkey -name prime256v1 -noout -out "${SERVER_KEY}"
    chmod 600 "${SERVER_KEY}"

    # Create SAN extension
    SAN_FILE=$(mktemp)
    cat > "${SAN_FILE}" <<EOF
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
DNS.2 = manager.${DOMAIN}
DNS.3 = *.${DOMAIN}
IP.1 = 127.0.0.1
IP.2 = ::1
EOF

    log_info "Generating server certificate signing request..."
    openssl req -new -nodes \
        -key "${SERVER_KEY}" \
        -subj "/O=${ORG}/CN=manager.${DOMAIN}" \
        -out "${SERVER_CSR}"

    log_info "Generating server certificate (signed by CA)..."
    openssl x509 -req \
        -in "${SERVER_CSR}" \
        -CA "${CA_CERT}" \
        -CAkey "${CA_KEY}" \
        -CAcreateserial \
        -days 365 \
        -sha256 \
        -extfile "${SAN_FILE}" \
        -out "${SERVER_CERT}"

    rm -f "${SAN_FILE}" "${SERVER_CSR}"
    chmod 644 "${SERVER_CERT}"
    log_info "Server certificate: ${SERVER_CERT}"
}

generate_client_cert() {
    local node_id="$1"

    log_info "Generating client private key for ${node_id}..."
    openssl ecparam -genkey -name prime256v1 -noout -out "${CLIENT_KEY}"
    chmod 600 "${CLIENT_KEY}"

    # Create client cert extensions
    CLIENT_EXT_FILE=$(mktemp)
    cat > "${CLIENT_EXT_FILE}" <<EOF
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid,issuer
EOF

    log_info "Generating client certificate signing request..."
    openssl req -new -nodes \
        -key "${CLIENT_KEY}" \
        -subj "/O=${ORG}/CN=${node_id}" \
        -out "${CLIENT_CSR}"

    log_info "Generating client certificate (signed by CA)..."
    openssl x509 -req \
        -in "${CLIENT_CSR}" \
        -CA "${CA_CERT}" \
        -CAkey "${CA_KEY}" \
        -CAcreateserial \
        -days 365 \
        -sha256 \
        -extfile "${CLIENT_EXT_FILE}" \
        -out "${CLIENT_CERT}"

    rm -f "${CLIENT_EXT_FILE}" "${CLIENT_CSR}"
    chmod 644 "${CLIENT_CERT}"
    log_info "Client certificate: ${CLIENT_CERT}"
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case "$1" in
        --ca-dir)
            CA_DIR="$2"
            shift 2
            ;;
        --node-id)
            NODE_ID="$2"
            shift 2
            ;;
        --domain)
            DOMAIN="$2"
            shift 2
            ;;
        --help|-h)
            head -20 "$0"
            exit 0
            ;;
        *)
            log_error "Unknown argument: $1"
            exit 1
            ;;
    esac
done

# Validate environment
if ! command -v openssl &>/dev/null; then
    log_error "openssl is required but not installed"
    exit 1
fi

# File paths
CA_KEY="${CA_DIR}/ca.key"
CA_CERT="${CA_DIR}/ca.crt"
SERVER_KEY="${CA_DIR}/server.key"
SERVER_CERT="${CA_DIR}/server.crt"
SERVER_CSR="${CA_DIR}/server.csr"
CLIENT_KEY="${CA_DIR}/client.key"
CLIENT_CERT="${CA_DIR}/client.crt"
CLIENT_CSR="${CA_DIR}/client.csr"

log_info "Certificate directory: ${CA_DIR}"
log_info "Organization: ${ORG}"
log_info "Domain: ${DOMAIN}"

# Create directory
mkdir -p "${CA_DIR}"

# Check if CA already exists
if [[ -f "${CA_CERT}" ]]; then
    log_warn "CA certificate already exists at ${CA_CERT}"
    read -rp "Overwrite? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_info "Keeping existing CA certificate"
    else
        log_info "Generating new CA..."
        generate_ca
    fi
else
    generate_ca
fi

# Generate server certificate for Manager
generate_server_cert

# Generate client certificate for Node Agent
if [[ -z "${NODE_ID}" ]]; then
    NODE_ID="node-$(hostname -s | tr '[:upper:]' '[:lower:]')"
    log_warn "No NODE_ID specified, using: ${NODE_ID}"
fi
generate_client_cert "${NODE_ID}"

log_info "Certificates generated successfully!"
log_info "  CA:     ${CA_CERT}"
log_info "  Server: ${SERVER_CERT}"
log_info "  Client: ${CLIENT_CERT} (${NODE_ID})"
log_info ""
log_info "To distribute to nodes:"
log_info "  1. Copy ${CA_CERT} to /etc/hivestack/tls/ca.crt"
log_info "  2. Copy ${CLIENT_CERT} to /etc/hivestack/tls/client.crt"
log_info "  3. Copy ${CLIENT_KEY} to /etc/hivestack/tls/client.key"
