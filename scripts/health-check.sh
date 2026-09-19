#!/bin/bash
#
# health-check.sh - Health check for HiveStack Manager / Node deployments
#
# Probes systemd services, the Manager REST API, gRPC and metrics ports,
# PostgreSQL connectivity, libvirtd/virsh, TLS certificate expiry, and
# disk usage on HiveStack data/log paths. Designed to run on a HiveStack
# appliance host (systemd + optionally libvirt/PostgreSQL installed) as a
# cron job, systemd timer, or manual operator check per docs/RUNBOOKS.md.
#
# Usage:
#   ./scripts/health-check.sh [options]
#
# Options:
#   -m, --manager        Force Manager checks even if the service unit is absent
#   -n, --node           Force Node checks even if the service unit is absent
#   -j, --json           Emit a single-line JSON summary instead of text
#   -q, --quiet          Only print WARN/CRIT/UNKNOWN lines (suppress OK)
#   -t, --timeout SEC    Network timeout in seconds for curl/TCP checks (default: 5)
#   -u, --url URL        Override the Manager health URL (default: probes 8443 then 8080)
#   -h, --help           Show this help and exit
#
# Environment variables:
#   HIVESTACK_TLS_DIR     - TLS certificate directory (default: /etc/hivestack/tls)
#   HIVESTACK_DATA_DIR    - Data directory to check disk usage for (default: /var/lib/hivestack)
#   HIVESTACK_LOG_DIR     - Log directory to check disk usage for (default: /var/log/hivestack)
#   HIVESTACK_DISK_WARN   - Disk usage warning threshold, percent (default: 80)
#   HIVESTACK_DISK_CRIT   - Disk usage critical threshold, percent (default: 90)
#   HIVESTACK_CERT_WARN_DAYS - TLS cert expiry warning threshold, days (default: 30)
#   HIVESTACK_CERT_CRIT_DAYS - TLS cert expiry critical threshold, days (default: 7)
#
# Exit codes (Nagios/monitoring-plugin convention):
#   0 = OK        all checks passed
#   1 = WARNING   at least one check degraded but nothing critical
#   2 = CRITICAL  at least one check failed
#   3 = UNKNOWN   usage error or no checks could be run

set -uo pipefail

# --- Defaults ---
TLS_DIR="${HIVESTACK_TLS_DIR:-/etc/hivestack/tls}"
DATA_DIR="${HIVESTACK_DATA_DIR:-/var/lib/hivestack}"
LOG_DIR="${HIVESTACK_LOG_DIR:-/var/log/hivestack}"
DISK_WARN="${HIVESTACK_DISK_WARN:-80}"
DISK_CRIT="${HIVESTACK_DISK_CRIT:-90}"
CERT_WARN_DAYS="${HIVESTACK_CERT_WARN_DAYS:-30}"
CERT_CRIT_DAYS="${HIVESTACK_CERT_CRIT_DAYS:-7}"

TIMEOUT=5
JSON_OUTPUT=0
QUIET=0
FORCE_MANAGER=0
FORCE_NODE=0
MANAGER_URL_OVERRIDE=""

# --- Colors (disabled when not a tty or in JSON mode) ---
if [[ -t 1 ]]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[1;33m'
    BLUE='\033[0;34m'
    NC='\033[0m'
else
    RED=''; GREEN=''; YELLOW=''; BLUE=''; NC=''
fi

usage() {
    sed -n '2,33p' "$0" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        -m|--manager) FORCE_MANAGER=1; shift ;;
        -n|--node) FORCE_NODE=1; shift ;;
        -j|--json) JSON_OUTPUT=1; shift ;;
        -q|--quiet) QUIET=1; shift ;;
        -t|--timeout) TIMEOUT="${2:?--timeout requires a value}"; shift 2 ;;
        -u|--url) MANAGER_URL_OVERRIDE="${2:?--url requires a value}"; shift 2 ;;
        -h|--help) usage; exit 0 ;;
        *) echo "Unknown option: $1" >&2; usage; exit 3 ;;
    esac
done

# --- Result tracking ---
# level: 0=OK 1=WARN 2=CRIT 3=SKIP (unavailable tool/unit; does not affect exit code)
WORST_LEVEL=0
HAVE_RESULT=0
declare -a RESULTS=() # each entry: "LEVEL|component|message"

record() {
    local level="$1" component="$2" message="$3"
    RESULTS+=("${level}|${component}|${message}")
    if (( level <= 2 )); then
        HAVE_RESULT=1
        if (( level > WORST_LEVEL )); then
            WORST_LEVEL=$level
        fi
    fi
    if (( JSON_OUTPUT == 1 )); then
        return
    fi
    local label color
    case "$level" in
        0) label="OK  "; color="$GREEN" ;;
        1) label="WARN"; color="$YELLOW" ;;
        2) label="CRIT"; color="$RED" ;;
        *) label="SKIP"; color="$BLUE" ;;
    esac
    if (( QUIET == 1 && level == 0 )); then
        return
    fi
    printf "${color}[%s]${NC} %-16s %s\n" "$label" "$component" "$message"
}

have_cmd() { command -v "$1" >/dev/null 2>&1; }

# --- Role detection ---
HAS_MANAGER_UNIT=0
HAS_NODE_UNIT=0
have_cmd systemctl && systemctl list-unit-files hivestack-manager.service >/dev/null 2>&1 && HAS_MANAGER_UNIT=1
have_cmd systemctl && systemctl list-unit-files hivestack-node.service >/dev/null 2>&1 && HAS_NODE_UNIT=1

CHECK_MANAGER=0
CHECK_NODE=0
(( HAS_MANAGER_UNIT == 1 || FORCE_MANAGER == 1 )) && CHECK_MANAGER=1
(( HAS_NODE_UNIT == 1 || FORCE_NODE == 1 )) && CHECK_NODE=1

# If neither role was detected or forced, check both (best-effort probe).
if (( CHECK_MANAGER == 0 && CHECK_NODE == 0 )); then
    CHECK_MANAGER=1
    CHECK_NODE=1
fi

# --- systemd service checks ---
check_service() {
    local unit="$1" label="$2"
    if ! have_cmd systemctl; then
        record 3 "$label" "systemctl not available, skipped"
        return
    fi
    if ! systemctl list-unit-files "$unit" >/dev/null 2>&1; then
        record 3 "$label" "unit not installed, skipped"
        return
    fi
    if systemctl is-active --quiet "$unit"; then
        record 0 "$label" "active"
    else
        local state
        state=$(systemctl is-active "$unit" 2>/dev/null || true)
        record 2 "$label" "not active (state: ${state:-unknown})"
    fi
}

(( CHECK_MANAGER == 1 )) && check_service hivestack-manager.service "manager-svc"
(( CHECK_NODE == 1 )) && check_service hivestack-node.service "node-svc"
check_service libvirtd.service "libvirtd"
check_service postgresql.service "postgresql"

# --- TCP port reachability ---
check_tcp_port() {
    local host="$1" port="$2" label="$3"
    if timeout "$TIMEOUT" bash -c "exec 3<>/dev/tcp/${host}/${port}" 2>/dev/null; then
        exec 3>&- 3<&- 2>/dev/null || true
        record 0 "$label" "listening on ${host}:${port}"
    else
        record 2 "$label" "not reachable on ${host}:${port}"
    fi
}

(( CHECK_MANAGER == 1 )) && check_tcp_port "localhost" 9090 "manager-grpc"
(( CHECK_NODE == 1 )) && check_tcp_port "localhost" 9090 "node-grpc"

# --- Manager REST API health endpoint ---
if (( CHECK_MANAGER == 1 )); then
    if have_cmd curl; then
        manager_ok=0
        if [[ -n "$MANAGER_URL_OVERRIDE" ]]; then
            urls=("$MANAGER_URL_OVERRIDE")
        else
            urls=("https://localhost:8443/health" "http://localhost:8080/health")
        fi
        for url in "${urls[@]}"; do
            code=$(curl -k -s -o /dev/null -w '%{http_code}' --max-time "$TIMEOUT" "$url" 2>/dev/null || echo "000")
            if [[ "$code" == "200" ]]; then
                record 0 "manager-api" "GET ${url} -> 200"
                manager_ok=1
                break
            fi
        done
        if (( manager_ok == 0 )); then
            record 2 "manager-api" "health endpoint unreachable or unhealthy (tried: ${urls[*]})"
        fi
    else
        record 3 "manager-api" "curl not available, skipped"
    fi
fi

# --- Prometheus metrics endpoints (informational, warn-only) ---
check_metrics() {
    local port="$1" label="$2"
    if ! have_cmd curl; then
        record 3 "$label" "curl not available, skipped"
        return
    fi
    local code
    code=$(curl -s -o /dev/null -w '%{http_code}' --max-time "$TIMEOUT" "http://localhost:${port}/metrics" 2>/dev/null || echo "000")
    if [[ "$code" == "200" ]]; then
        record 0 "$label" "http://localhost:${port}/metrics -> 200"
    else
        record 1 "$label" "metrics endpoint not responding on :${port}"
    fi
}

(( CHECK_MANAGER == 1 )) && check_metrics 9091 "manager-metrics"
(( CHECK_NODE == 1 )) && check_metrics 9092 "node-metrics"

# --- PostgreSQL connectivity (manager only) ---
if (( CHECK_MANAGER == 1 )); then
    if have_cmd pg_isready; then
        if pg_isready -h localhost -p 5432 -t "$TIMEOUT" >/dev/null 2>&1; then
            record 0 "postgres-conn" "accepting connections on localhost:5432"
        else
            record 2 "postgres-conn" "not accepting connections on localhost:5432"
        fi
    else
        record 3 "postgres-conn" "pg_isready not available, skipped"
    fi
fi

# --- libvirt / virsh connectivity (node only) ---
if (( CHECK_NODE == 1 )); then
    if have_cmd virsh; then
        if timeout "$TIMEOUT" virsh -c qemu:///system list >/dev/null 2>&1; then
            record 0 "libvirt-conn" "qemu:///system reachable via virsh"
        else
            record 2 "libvirt-conn" "cannot connect to qemu:///system"
        fi
    else
        record 3 "libvirt-conn" "virsh not available, skipped"
    fi
fi

# --- TLS certificate expiry ---
check_cert_expiry() {
    local cert="$1" label="$2"
    if [[ ! -f "$cert" ]]; then
        record 3 "$label" "${cert} not found, skipped"
        return
    fi
    if ! have_cmd openssl; then
        record 3 "$label" "openssl not available, skipped"
        return
    fi
    local end_date end_epoch now_epoch days_left
    end_date=$(openssl x509 -enddate -noout -in "$cert" 2>/dev/null | cut -d= -f2)
    if [[ -z "$end_date" ]]; then
        record 3 "$label" "could not parse certificate ${cert}"
        return
    fi
    end_epoch=$(date -d "$end_date" +%s 2>/dev/null || echo "")
    now_epoch=$(date +%s)
    if [[ -z "$end_epoch" ]]; then
        record 3 "$label" "could not parse expiry date for ${cert}"
        return
    fi
    days_left=$(( (end_epoch - now_epoch) / 86400 ))
    if (( days_left < CERT_CRIT_DAYS )); then
        record 2 "$label" "expires in ${days_left}d (< ${CERT_CRIT_DAYS}d) — ${cert}"
    elif (( days_left < CERT_WARN_DAYS )); then
        record 1 "$label" "expires in ${days_left}d (< ${CERT_WARN_DAYS}d) — ${cert}"
    else
        record 0 "$label" "valid for ${days_left}d — ${cert}"
    fi
}

(( CHECK_MANAGER == 1 )) && check_cert_expiry "${TLS_DIR}/manager.crt" "manager-cert"
(( CHECK_NODE == 1 )) && check_cert_expiry "${TLS_DIR}/node.crt" "node-cert"

# --- Disk usage ---
check_disk() {
    local path="$1" label="$2"
    if [[ ! -d "$path" ]]; then
        record 3 "$label" "${path} does not exist, skipped"
        return
    fi
    local usage
    usage=$(df -P "$path" 2>/dev/null | awk 'NR==2 {gsub("%","",$5); print $5}')
    if [[ -z "$usage" ]]; then
        record 3 "$label" "could not determine disk usage for ${path}"
        return
    fi
    if (( usage >= DISK_CRIT )); then
        record 2 "$label" "${usage}% used on ${path} (>= ${DISK_CRIT}%)"
    elif (( usage >= DISK_WARN )); then
        record 1 "$label" "${usage}% used on ${path} (>= ${DISK_WARN}%)"
    else
        record 0 "$label" "${usage}% used on ${path}"
    fi
}

check_disk "$DATA_DIR" "disk-data"
check_disk "$LOG_DIR" "disk-log"

# --- Summary / exit ---
EXIT_CODE=$WORST_LEVEL
if (( HAVE_RESULT == 0 )); then
    EXIT_CODE=3
fi

case "$EXIT_CODE" in
    0) SUMMARY="OK" ;;
    1) SUMMARY="WARNING" ;;
    2) SUMMARY="CRITICAL" ;;
    *) SUMMARY="UNKNOWN" ;;
esac

if (( JSON_OUTPUT == 1 )); then
    checks_json=""
    for entry in "${RESULTS[@]}"; do
        IFS='|' read -r lvl comp msg <<< "$entry"
        case "$lvl" in
            0) status="ok" ;; 1) status="warn" ;; 2) status="crit" ;; *) status="skip" ;;
        esac
        esc_msg=$(printf '%s' "$msg" | sed 's/\\/\\\\/g; s/"/\\"/g')
        checks_json+="{\"component\":\"${comp}\",\"status\":\"${status}\",\"message\":\"${esc_msg}\"},"
    done
    checks_json="[${checks_json%,}]"
    printf '{"summary":"%s","exit_code":%d,"checks":%s}\n' "$SUMMARY" "$EXIT_CODE" "$checks_json"
else
    echo "---"
    echo -e "Overall: ${SUMMARY} (exit ${EXIT_CODE})"
fi

exit "$EXIT_CODE"
