#!/bin/bash
#
# HiveStack Comprehensive Health Check Script
#
# Performs health checks on all HiveStack components and reports status.
# Can be run manually or via cron for continuous monitoring.
#
# Usage: ./health-check.sh [--json] [--verbose] [--component <name>]
#
# Exit codes:
#   0 - All checks passed
#   1 - Warning threshold exceeded
#   2 - Critical threshold exceeded
#   3 - Script error

set -euo pipefail

# ============================================================================
# Configuration
# ============================================================================

HIVESTACK_HOME="${HIVESTACK_HOME:-/opt/hivestack}"
MANAGER_ADDR="${MANAGER_ADDR:-https://localhost:8443}"
API_TOKEN="${API_TOKEN:-}"
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-hivestack}"
DB_USER="${DB_USER:-hivestack}"
DB_PASS="${DB_PASS:-}"
LOG_FILE="${LOG_FILE:-/var/log/hivestack/health-check.log}"
VERBOSE=0
OUTPUT_JSON=0
COMPONENT=""

# Thresholds
CPU_WARN=70
CPU_CRIT=90
MEM_WARN=80
MEM_CRIT=95
DISK_WARN=75
DISK_CRIT=90
API_LATENCY_WARN=200   # ms
API_LATENCY_CRIT=500   # ms
HEARTBEAT_AGE_WARN=120  # seconds
HEARTBEAT_AGE_CRIT=300  # seconds

# ============================================================================
# Color codes for terminal output
# ============================================================================

RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# ============================================================================
# Global state
# ============================================================================

declare -A CHECK_STATUS
declare -A CHECK_MESSAGE
declare -A CHECK_DETAILS
TOTAL_CHECKS=0
PASS_COUNT=0
WARN_COUNT=0
CRIT_COUNT=0
OVERALL_STATUS="OK"

# ============================================================================
# Functions
# ============================================================================

log() {
    local level="$1"
    shift
    local msg="$*"
    local timestamp
    timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    echo "[$timestamp] [$level] $msg" >> "$LOG_FILE" 2>/dev/null || true
}

print_header() {
    if [[ $OUTPUT_JSON -eq 0 ]]; then
        echo -e "${BLUE}========================================${NC}"
        echo -e "${BLUE}  HiveStack Health Check${NC}"
        echo -e "${BLUE}  $(date '+%Y-%m-%d %H:%M:%S %Z')${NC}"
        echo -e "${BLUE}========================================${NC}"
        echo ""
    fi
}

print_check() {
    local name="$1"
    local status="$2"
    local message="$3"
    local details="${4:-}"

    TOTAL_CHECKS=$((TOTAL_CHECKS + 1))

    case "$status" in
        OK)
            PASS_COUNT=$((PASS_COUNT + 1))
            CHECK_STATUS["$name"]="OK"
            ;;
        WARNING)
            WARN_COUNT=$((WARN_COUNT + 1))
            CHECK_STATUS["$name"]="WARNING"
            [[ "$OVERALL_STATUS" != "CRITICAL" ]] && OVERALL_STATUS="WARNING"
            ;;
        CRITICAL)
            CRIT_COUNT=$((CRIT_COUNT + 1))
            CHECK_STATUS["$name"]="CRITICAL"
            OVERALL_STATUS="CRITICAL"
            ;;
    esac

    CHECK_MESSAGE["$name"]="$message"
    CHECK_DETAILS["$name"]="$details"

    if [[ $OUTPUT_JSON -eq 0 ]]; then
        case "$status" in
            OK)       color="$GREEN" ;;
            WARNING)  color="$YELLOW" ;;
            CRITICAL) color="$RED" ;;
        esac
        printf "  ${color}%-12s${NC} %-30s %s\n" "[$status]" "$name" "$message"
        if [[ $VERBOSE -eq 1 && -n "$details" ]]; then
            echo "               $details"
        fi
    fi
}

check_prerequisites() {
    local missing=()
    for cmd in curl jq; do
        if ! command -v "$cmd" &>/dev/null; then
            missing+=("$cmd")
        fi
    done

    if [[ ${#missing[@]} -gt 0 ]]; then
        echo "ERROR: Missing required commands: ${missing[*]}" >&2
        echo "Install with: sudo zypper install ${missing[*]}" >&2
        exit 3
    fi
}

# ============================================================================
# Component Checks
# ============================================================================

check_manager_service() {
    local name="Manager Service"
    local status="OK"
    local message=""
    local details=""

    if systemctl is-active --quiet hivestack-manager 2>/dev/null; then
        local uptime
        uptime=$(systemctl show hivestack-manager --property=ActiveEnterTimestamp --value 2>/dev/null || echo "unknown")
        message="Running (since $uptime)"
    elif pgrep -f "hivestack-manager" &>/dev/null; then
        message="Running (not managed by systemd)"
    else
        status="CRITICAL"
        message="Not running"
        details="Start with: sudo systemctl start hivestack-manager"
    fi

    print_check "$name" "$status" "$message" "$details"
}

check_api_endpoint() {
    local name="API Endpoint"
    local status="OK"
    local message=""
    local details=""

    local start_time end_time duration
    start_time=$(date +%s%N 2>/dev/null || echo "0")

    local http_code
    http_code=$(curl -sk -o /dev/null -w "%{http_code}" \
        --max-time 10 \
        "${MANAGER_ADDR}/api/health" 2>/dev/null || echo "000")

    end_time=$(date +%s%N 2>/dev/null || echo "0")

    if [[ "$start_time" != "0" && "$end_time" != "0" ]]; then
        duration=$(( (end_time - start_time) / 1000000 ))
    else
        duration=0
    fi

    if [[ "$http_code" == "200" ]]; then
        message="HTTP 200 (${duration}ms)"
        if [[ $duration -gt $API_LATENCY_CRIT ]]; then
            status="CRITICAL"
            message="HTTP 200 but slow (${duration}ms > ${API_LATENCY_CRIT}ms)"
        elif [[ $duration -gt $API_LATENCY_WARN ]]; then
            status="WARNING"
            message="HTTP 200 but elevated latency (${duration}ms > ${API_LATENCY_WARN}ms)"
        fi
    elif [[ "$http_code" == "000" ]]; then
        status="CRITICAL"
        message="Connection failed"
        details="Check if Manager is running and accessible at $MANAGER_ADDR"
    else
        status="CRITICAL"
        message="HTTP $http_code"
        details="API returned unexpected status code"
    fi

    print_check "$name" "$status" "$message" "$details"
}

check_database() {
    local name="Database"
    local status="OK"
    local message=""
    local details=""

    if command -v psql &>/dev/null; then
        local conn_result
        if conn_result=$(PGPASSWORD="$DB_PASS" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "SELECT 1" 2>&1); then
            local db_size
            db_size=$(PGPASSWORD="$DB_PASS" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -tAc "SELECT pg_size_pretty(pg_database_size('$DB_NAME'))" 2>/dev/null || echo "unknown")
            message="Connected (size: $db_size)"
        else
            status="CRITICAL"
            message="Connection failed"
            details="$conn_result"
        fi
    else
        # Fallback: check if PostgreSQL is listening
        if pgrep -x postgres &>/dev/null; then
            message="PostgreSQL process running (psql not available for detailed check)"
        else
            status="CRITICAL"
            message="PostgreSQL not running"
        fi
    fi

    print_check "$name" "$status" "$message" "$details"
}

check_node_agents() {
    local name="Node Agents"
    local status="OK"
    local message=""
    local details=""

    # Query API for node status
    local node_data
    node_data=$(curl -sk --max-time 10 \
        -H "Authorization: Bearer $API_TOKEN" \
        "${MANAGER_ADDR}/api/v1/hosts" 2>/dev/null || echo "[]")

    local total_nodes online_nodes offline_nodes suspect_nodes
    total_nodes=$(echo "$node_data" | jq 'length' 2>/dev/null || echo "0")
    online_nodes=$(echo "$node_data" | jq '[.[] | select(.status == "online")] | length' 2>/dev/null || echo "0")
    offline_nodes=$(echo "$node_data" | jq '[.[] | select(.status == "offline")] | length' 2>/dev/null || echo "0")
    suspect_nodes=$(echo "$node_data" | jq '[.[] | select(.status == "suspect")] | length' 2>/dev/null || echo "0")

    if [[ "$total_nodes" == "0" ]]; then
        message="No nodes registered"
    else
        message="$online_nodes/$total_nodes online"
        if [[ "$offline_nodes" -gt 0 ]]; then
            message+=", $offline_nodes offline"
            status="WARNING"
        fi
        if [[ "$suspect_nodes" -gt 0 ]]; then
            message+=", $suspect_nodes suspect"
            [[ "$status" != "WARNING" ]] && status="WARNING"
        fi
    fi

    details="Total: $total_nodes, Online: $online_nodes, Offline: $offline_nodes, Suspect: $suspect_nodes"
    print_check "$name" "$status" "$message" "$details"
}

check_vm_status() {
    local name="VM Status"
    local status="OK"
    local message=""
    local details=""

    local vm_data
    vm_data=$(curl -sk --max-time 10 \
        -H "Authorization: Bearer $API_TOKEN" \
        "${MANAGER_ADDR}/api/v1/vms" 2>/dev/null || echo "[]")

    local total_vms running_vms stopped_vms error_vms
    total_vms=$(echo "$vm_data" | jq 'length' 2>/dev/null || echo "0")
    running_vms=$(echo "$vm_data" | jq '[.[] | select(.status == "running")] | length' 2>/dev/null || echo "0")
    stopped_vms=$(echo "$vm_data" | jq '[.[] | select(.status == "stopped")] | length' 2>/dev/null || echo "0")
    error_vms=$(echo "$vm_data" | jq '[.[] | select(.status == "error")] | length' 2>/dev/null || echo "0")

    message="$running_vms running, $stopped_vms stopped"
    if [[ "$error_vms" -gt 0 ]]; then
        message+=", $error_vms in error state"
        status="WARNING"
    fi

    details="Total VMs: $total_vms"
    print_check "$name" "$status" "$message" "$details"
}

check_cpu_usage() {
    local name="CPU Usage"
    local status="OK"
    local message=""
    local details=""

    local cpu_usage
    cpu_usage=$(top -bn1 | grep "Cpu(s)" | awk '{print int($2)}' 2>/dev/null || echo "0")

    message="${cpu_usage}% used"
    if [[ "$cpu_usage" -ge "$CPU_CRIT" ]]; then
        status="CRITICAL"
    elif [[ "$cpu_usage" -ge "$CPU_WARN" ]]; then
        status="WARNING"
    fi

    local load_avg
    load_avg=$(cat /proc/loadavg | awk '{print $1, $2, $3}')
    details="Load average: $load_avg"

    print_check "$name" "$status" "$message" "$details"
}

check_memory_usage() {
    local name="Memory Usage"
    local status="OK"
    local message=""
    local details=""

    local mem_info
    mem_info=$(free -m | awk 'NR==2{printf "%.0f", $3*100/$2}')
    local mem_total mem_used
    mem_total=$(free -m | awk 'NR==2{print $2}')
    mem_used=$(free -m | awk 'NR==2{print $3}')

    message="${mem_info}% used (${mem_used}MB / ${mem_total}MB)"
    if [[ "$mem_info" -ge "$MEM_CRIT" ]]; then
        status="CRITICAL"
    elif [[ "$mem_info" -ge "$MEM_WARN" ]]; then
        status="WARNING"
    fi

    local swap_used
    swap_used=$(free -m | awk 'NR==3{print $3}')
    details="Swap used: ${swap_used}MB"

    print_check "$name" "$status" "$message" "$details"
}

check_disk_usage() {
    local name="Disk Usage"
    local status="OK"
    local message=""
    local details=""

    local root_usage
    root_usage=$(df -h / | awk 'NR==2{gsub(/%/,""); print $5}')
    local data_usage
    data_usage=$(df -h /var/lib/hivestack 2>/dev/null | awk 'NR==2{gsub(/%/,""); print $5}' || echo "N/A")

    message="Root: ${root_usage}%"
    if [[ "$data_usage" != "N/A" ]]; then
        message+=", Data: ${data_usage}%"
    fi

    if [[ "$root_usage" -ge "$DISK_CRIT" ]] || [[ "$data_usage" != "N/A" && "$data_usage" -ge "$DISK_CRIT" ]]; then
        status="CRITICAL"
    elif [[ "$root_usage" -ge "$DISK_WARN" ]] || [[ "$data_usage" != "N/A" && "$data_usage" -ge "$DISK_WARN" ]]; then
        status="WARNING"
    fi

    details=$(df -h / /var/lib/hivestack 2>/dev/null | awk 'NR>1{print $6": "$5" used ("$4" free)"}' | tr '\n' ' ')
    print_check "$name" "$status" "$message" "$details"
}

check_zfs_pools() {
    local name="ZFS Pools"
    local status="OK"
    local message=""
    local details=""

    if ! command -v zpool &>/dev/null; then
        print_check "$name" "OK" "ZFS not installed (skipped)" ""
        return
    fi

    local pools
    pools=$(zpool list -H -o name,health,capacity 2>/dev/null || echo "")

    if [[ -z "$pools" ]]; then
        print_check "$name" "OK" "No ZFS pools configured" ""
        return
    fi

    local pool_count=0
    local degraded_pools=0
    local pool_details=""

    while IFS=$'\t' read -r pool_name pool_health pool_cap; do
        pool_count=$((pool_count + 1))
        pool_details+="$pool_name($pool_health,${pool_cap}) "
        if [[ "$pool_health" != "ONLINE" ]]; then
            degraded_pools=$((degraded_pools + 1))
        fi
    done <<< "$pools"

    if [[ "$degraded_pools" -gt 0 ]]; then
        status="CRITICAL"
        message="$degraded_pools/$pool_count pools degraded"
    else
        message="$pool_count pools healthy"
    fi

    details="$pool_details"
    print_check "$name" "$status" "$message" "$details"
}

check_ha_controller() {
    local name="HA Controller"
    local status="OK"
    local message=""
    local details=""

    if systemctl is-active --quiet hivestack-ha 2>/dev/null; then
        message="Running"
    elif pgrep -f "hivestack-ha" &>/dev/null; then
        message="Running (not systemd managed)"
    else
        status="WARNING"
        message="Not running"
        details="HA protection disabled - VMs will not auto-restart on failure"
    fi

    print_check "$name" "$status" "$message" "$details"
}

check_migration_service() {
    local name="Migration Service"
    local status="OK"
    local message=""
    local details=""

    local migration_data
    migration_data=$(curl -sk --max-time 10 \
        -H "Authorization: Bearer $API_TOKEN" \
        "${MANAGER_ADDR}/api/v1/migration/jobs?active=true" 2>/dev/null || echo "{}")

    local active_jobs failed_jobs
    active_jobs=$(echo "$migration_data" | jq '.active // 0' 2>/dev/null || echo "0")
    failed_jobs=$(echo "$migration_data" | jq '[.jobs[]? | select(.state == "failed")] | length' 2>/dev/null || echo "0")

    message="$active_jobs active jobs"
    if [[ "$failed_jobs" -gt 0 ]]; then
        message+=", $failed_jobs failed (needs attention)"
        status="WARNING"
    fi

    print_check "$name" "$status" "$message" "$details"
}

check_certificates() {
    local name="Certificates"
    local status="OK"
    local message=""
    local details=""

    local cert_file="/etc/hivestack/tls/server.crt"
    if [[ ! -f "$cert_file" ]]; then
        cert_file="/opt/hivestack/tls/server.crt"
    fi

    if [[ -f "$cert_file" ]]; then
        local expiry_date days_until_expiry
        expiry_date=$(openssl x509 -enddate -noout -in "$cert_file" 2>/dev/null | cut -d= -f2)
        days_until_expiry=$(( ($(date -d "$expiry_date" +%s 2>/dev/null || echo "0") - $(date +%s)) / 86400 ))

        if [[ "$days_until_expiry" -lt 0 ]]; then
            status="CRITICAL"
            message="EXPIRED (${days_until_expiry#-} days ago)"
        elif [[ "$days_until_expiry" -lt 7 ]]; then
            status="CRITICAL"
            message="Expires in $days_until_expiry days"
        elif [[ "$days_until_expiry" -lt 30 ]]; then
            status="WARNING"
            message="Expires in $days_until_expiry days"
        else
            message="Valid for $days_until_expiry days"
        fi

        details="Certificate: $cert_file, Expires: $expiry_date"
    else
        status="WARNING"
        message="No TLS certificate found"
        details="Expected at $cert_file"
    fi

    print_check "$name" "$status" "$message" "$details"
}

check_backup_status() {
    local name="Backup Status"
    local status="OK"
    local message=""
    local details=""

    local backup_data
    backup_data=$(curl -sk --max-time 10 \
        -H "Authorization: Bearer $API_TOKEN" \
        "${MANAGER_ADDR}/api/v1/backups?recent=24h" 2>/dev/null || echo "{}")

    local total_backups failed_backups latest_backup
    total_backups=$(echo "$backup_data" | jq '.total // 0' 2>/dev/null || echo "0")
    failed_backups=$(echo "$backup_data" | jq '[.backups[]? | select(.status == "failed")] | length' 2>/dev/null || echo "0")
    latest_backup=$(echo "$backup_data" | jq -r '.backups[0].created_at // "unknown"' 2>/dev/null || echo "unknown")

    message="$total_backups backups in last 24h"
    if [[ "$failed_backups" -gt 0 ]]; then
        message+=", $failed_backups failed"
        status="WARNING"
    fi
    details="Latest: $latest_backup"

    print_check "$name" "$status" "$message" "$details"
}

check_time_sync() {
    local name="Time Sync"
    local status="OK"
    local message=""
    local details=""

    if command -v chronyc &>/dev/null; then
        local chrony_status
        chrony_status=$(chronyc tracking 2>/dev/null | grep "Leap status" | awk '{print $NF}')
        if [[ "$chrony_status" == "Normal" ]]; then
            message="Synchronized (chrony)"
        else
            status="WARNING"
            message="Not synchronized: $chrony_status"
        fi
    elif command -v ntpq &>/dev/null; then
        if ntpq -p &>/dev/null; then
            message="Synchronized (ntpd)"
        else
            status="WARNING"
            message="ntpd not responding"
        fi
    else
        message="No NTP client found"
    fi

    print_check "$name" "$status" "$message" "$details"
}

# ============================================================================
# Output
# ============================================================================

print_summary() {
    if [[ $OUTPUT_JSON -eq 1 ]]; then
        # JSON output
        echo "{"
        echo "  \"timestamp\": \"$(date -Iseconds)\","
        echo "  \"hostname\": \"$(hostname)\","
        echo "  \"overall_status\": \"$OVERALL_STATUS\","
        echo "  \"summary\": {"
        echo "    \"total\": $TOTAL_CHECKS,"
        echo "    \"passed\": $PASS_COUNT,"
        echo "    \"warnings\": $WARN_COUNT,"
        echo "    \"critical\": $CRIT_COUNT"
        echo "  },"
        echo "  \"checks\": {"

        local first=1
        for key in "${!CHECK_STATUS[@]}"; do
            if [[ $first -eq 0 ]]; then echo ","; fi
            first=0
            local status="${CHECK_STATUS[$key]}"
            local msg="${CHECK_MESSAGE[$key]}"
            local det="${CHECK_DETAILS[$key]}"
            # Escape special characters for JSON
            msg=$(echo "$msg" | sed 's/"/\\"/g' | tr '\n' ' ')
            det=$(echo "$det" | sed 's/"/\\"/g' | tr '\n' ' ')
            echo -n "    \"$key\": {\"status\": \"$status\", \"message\": \"$msg\", \"details\": \"$det\"}"
        done
        echo ""
        echo "  }"
        echo "}"
    else
        # Human-readable output
        echo ""
        echo -e "${BLUE}----------------------------------------${NC}"
        echo -e "  Total: $TOTAL_CHECKS | ${GREEN}Passed: $PASS_COUNT${NC} | ${YELLOW}Warnings: $WARN_COUNT${NC} | ${RED}Critical: $CRIT_COUNT${NC}"
        echo -e "  Overall: $(if [[ "$OVERALL_STATUS" == "OK" ]]; then echo -e "${GREEN}$OVERALL_STATUS${NC}"; elif [[ "$OVERALL_STATUS" == "WARNING" ]]; then echo -e "${YELLOW}$OVERALL_STATUS${NC}"; else echo -e "${RED}$OVERALL_STATUS${NC}"; fi)"
        echo -e "${BLUE}----------------------------------------${NC}"
    fi
}

# ============================================================================
# Main
# ============================================================================

main() {
    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --json)
                OUTPUT_JSON=1
                shift
                ;;
            --verbose|-v)
                VERBOSE=1
                shift
                ;;
            --component|-c)
                COMPONENT="$2"
                shift 2
                ;;
            --help|-h)
                echo "Usage: $0 [--json] [--verbose] [--component <name>]"
                echo ""
                echo "Options:"
                echo "  --json          Output results in JSON format"
                echo "  --verbose       Show detailed information"
                echo "  --component     Run only specific check (manager, api, db, nodes, vms, cpu, memory, disk, zfs, ha, migration, certs, backup, time)"
                echo "  --help          Show this help message"
                exit 0
                ;;
            *)
                echo "Unknown option: $1"
                exit 3
                ;;
        esac
    done

    # Ensure log directory exists
    mkdir -p "$(dirname "$LOG_FILE")" 2>/dev/null || true

    check_prerequisites
    print_header

    # Run checks based on component filter or all
    case "${COMPONENT}" in
        manager)    check_manager_service ;;
        api)        check_api_endpoint ;;
        db)         check_database ;;
        nodes)      check_node_agents ;;
        vms)        check_vm_status ;;
        cpu)        check_cpu_usage ;;
        memory)     check_memory_usage ;;
        disk)       check_disk_usage ;;
        zfs)        check_zfs_pools ;;
        ha)         check_ha_controller ;;
        migration)  check_migration_service ;;
        certs)      check_certificates ;;
        backup)     check_backup_status ;;
        time)       check_time_sync ;;
        "")
            # Run all checks
            check_manager_service
            check_api_endpoint
            check_database
            check_node_agents
            check_vm_status
            check_cpu_usage
            check_memory_usage
            check_disk_usage
            check_zfs_pools
            check_ha_controller
            check_migration_service
            check_certificates
            check_backup_status
            check_time_sync
            ;;
        *)
            echo "Unknown component: $COMPONENT"
            echo "Valid components: manager, api, db, nodes, vms, cpu, memory, disk, zfs, ha, migration, certs, backup, time"
            exit 3
            ;;
    esac

    print_summary

    # Log result
    log "INFO" "Health check completed: $OVERALL_STATUS (Total: $TOTAL_CHECKS, Passed: $PASS_COUNT, Warnings: $WARN_COUNT, Critical: $CRIT_COUNT)"

    # Exit with appropriate code
    case "$OVERALL_STATUS" in
        OK)       exit 0 ;;
        WARNING)  exit 1 ;;
        CRITICAL) exit 2 ;;
        *)        exit 3 ;;
    esac
}

main "$@"
