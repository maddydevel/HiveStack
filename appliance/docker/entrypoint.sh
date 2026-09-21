#!/bin/bash
# HiveStack Docker Entrypoint
set -euo pipefail

# Determine component to run
COMPONENT="${1:-all}"

log() {
    echo "[hivestack-entrypoint] $*"
}

# Start PostgreSQL
start_postgres() {
    log "Starting PostgreSQL..."
    
    # Initialize DB if needed
    if [ ! -d "/var/lib/postgresql/data/base" ]; then
        chown -R postgres:postgres /var/lib/postgresql/data 2>/dev/null || true
        su - postgres -c "/usr/lib/postgresql/*/bin/initdb -D /var/lib/postgresql/data" 2>/dev/null || true
        # Update to listen on all addresses
        echo "listen_addresses = '*'" >> /var/lib/postgresql/data/postgresql.conf
    fi
    
    # Configure authentication
    echo "host all all 0.0.0.0/0 md5" >> /var/lib/postgresql/data/pg_hba.conf 2>/dev/null || true
    echo "local all all trust" >> /var/lib/postgresql/data/pg_hba.conf 2>/dev/null || true
    
    su - postgres -c "/usr/lib/postgresql/*/bin/pg_ctl -D /var/lib/postgresql/data -l /var/log/hivestack/pg.log -o '-p 5432' start" 2>/dev/null || true
    
    # Wait for PostgreSQL
    for i in {1..30}; do
        if pg_isready -h localhost -p 5432 -U postgres >/dev/null 2>&1; then
            break
        fi
        sleep 1
    done
    
    # Create hivestack user and database
    su - postgres -c "psql -tc \"SELECT 1 FROM pg_roles WHERE rolname='hivestack'\" | grep -q 1 || \
        psql -c \"CREATE USER hivestack WITH PASSWORD 'hivestack' SUPERUSER;\"" 2>/dev/null || true
    su - postgres -c "psql -tc \"SELECT 1 FROM pg_database WHERE datname='hivestack'\" | grep -q 1 || \
        psql -c \"CREATE DATABASE hivestack OWNER hivestack;\"" 2>/dev/null || true
    
    # Run migrations
    if [ -d /opt/hivestack/migrations ]; then
        for f in /opt/hivestack/migrations/*.sql; do
            if [ -f "$f" ]; then
                su - postgres -c "psql -d hivestack -f '$f'" 2>/dev/null || true
            fi
        done
    fi
}

# Generate TLS certificates if not present
generate_certs() {
    local cert_dir="/etc/hivestack/certs"
    mkdir -p "$cert_dir"
    
    if [ ! -f "$cert_dir/ca.crt" ]; then
        log "Generating TLS certificates..."
        openssl ecparam -genkey -name prime256v1 -out "$cert_dir/ca.key" 2>/dev/null
        openssl req -new -x509 -key "$cert_dir/ca.key" -out "$cert_dir/ca.crt" \
            -days 3650 -subj "/CN=HiveStack Root CA/O=HiveStack" 2>/dev/null
        
        for name in manager node-server; do
            openssl ecparam -genkey -name prime256v1 -out "$cert_dir/${name}.key" 2>/dev/null
            openssl req -new -key "$cert_dir/${name}.key" -out "$cert_dir/${name}.csr" \
                -subj "/CN=hivestack-${name}/O=HiveStack" 2>/dev/null
            openssl x509 -req -in "$cert_dir/${name}.csr" -CA "$cert_dir/ca.crt" \
                -CAkey "$cert_dir/ca.key" -CAcreateserial -out "$cert_dir/${name}.crt" \
                -days 365 -sha256 2>/dev/null
            rm -f "$cert_dir/${name}.csr"
        done
        
        chmod 600 "$cert_dir"/*.key
        chmod 644 "$cert_dir"/*.crt
    fi
}

# Create manager config if not present
setup_config() {
    local jwt_secret
    jwt_secret=$(openssl rand -hex 32 2>/dev/null || head -c 64 /dev/urandom | xxd -p | tr -d '\n')
    
    if [ ! -f /etc/hivestack/manager.yaml ]; then
        cat > /etc/hivestack/manager.yaml <<EOF
dsn: "postgres://hivestack:hivestack@localhost:5432/hivestack?sslmode=disable"
jwt_secret: "${jwt_secret}"
tls_cert_file: "/etc/hivestack/certs/manager.crt"
tls_key_file: "/etc/hivestack/certs/manager.key"
tls_ca_file: "/etc/hivestack/certs/ca.crt"
listen_address: ":8080"
listen_tls_address: ":8443"
grpc_port: 44567
log_level: "info"
log_format: "json"
EOF
        chown hivestack:hivestack /etc/hivestack/manager.yaml
    fi
    
    if [ ! -f /etc/hivestack/node.yaml ]; then
        cat > /etc/hivestack/node.yaml <<EOF
node_id: "$(hostname)"
manager_address: "localhost:44567"
tls_cert_file: "/etc/hivestack/certs/node-server.crt"
tls_key_file: "/etc/hivestack/certs/node-server.key"
tls_ca_file: "/etc/hivestack/certs/ca.crt"
heartbeat_interval: 30
log_level: "info"
log_format: "json"
EOF
        chown hivestack:hivestack /etc/hivestack/node.yaml
    fi
}

# Start HiveStack services
start_services() {
    log "Starting HiveStack services..."
    
    if [ "$COMPONENT" = "manager" ] || [ "$COMPONENT" = "all" ]; then
        log "Starting Manager..."
        su - hivestack -c "/opt/hivestack/bin/hive-manager --config /etc/hivestack/manager.yaml" &
    fi
    
    if [ "$COMPONENT" = "node" ] || [ "$COMPONENT" = "all" ]; then
        log "Starting Node Agent..."
        /opt/hivestack/bin/hive-node --config /etc/hivestack/node.yaml &
    fi
}

# Main execution
main() {
    log "Starting HiveStack ${COMPONENT}"
    
    # Start PostgreSQL
    start_postgres
    
    # Generate TLS certs
    generate_certs
    
    # Setup config
    setup_config
    
    # Start services
    start_services
    
    # Print summary
    log ""
    log "============================================"
    log "  HiveStack is starting..."
    log "============================================"
    log "  HTTP:      http://localhost:8080"
    log "  HTTPS:     https://localhost:8443"
    log "  Health:    http://localhost:8080/api/v1/health"
    log "  PostgreSQL: localhost:5432 (hivestack/hivestack)"
    log ""
    log "  Admin:     admin@localhost (default password: admin)"
    log ""
    log "  Logs:      docker logs -f <container>"
    log "============================================"
    log ""
    
    # Wait for all background processes
    wait
}

main "$@"
