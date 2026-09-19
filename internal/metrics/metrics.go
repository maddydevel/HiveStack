// Package metrics provides Prometheus metrics for HiveStack.
//
// Exposes host-level and VM-level metrics via a /metrics HTTP endpoint
// for Prometheus scraping.
package metrics

import (
    "time"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

// Namespaces
const (
    namespace = "hivestack"
    subsystem = "node"
)

// Host metrics
var (
    HostCPUUsagePercent = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "host_cpu_usage_percent",
            Help: "CPU usage percentage per host",
        },
        []string{"host_id", "hostname"},
    )

    HostMemoryUsageBytes = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "host_memory_usage_bytes",
            Help: "Memory usage in bytes per host",
        },
        []string{"host_id", "hostname"},
    )

    HostMemoryTotalBytes = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "host_memory_total_bytes",
            Help: "Total memory in bytes per host",
        },
        []string{"host_id", "hostname"},
    )

    HostDiskUsageBytes = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "host_disk_usage_bytes",
            Help: "Disk usage in bytes per host",
        },
        []string{"host_id", "hostname", "storage_pool"},
    )

    HostDiskTotalBytes = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "host_disk_total_bytes",
            Help: "Total disk capacity in bytes per host",
        },
        []string{"host_id", "hostname", "storage_pool"},
    )

    HostVMCount = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "host_vm_count",
            Help: "Number of VMs running on a host",
        },
        []string{"host_id", "hostname"},
    )

    HostStatus = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "host_status",
            Help: "Host status: 1=online, 0=offline, -1=maintenance",
        },
        []string{"host_id", "hostname", "status"},
    )
)

// VM metrics
var (
    VMCPUUsagePercent = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "vm_cpu_usage_percent",
            Help: "CPU usage percentage per VM",
        },
        []string{"vm_id", "vm_name", "host_id"},
    )

    VMMemoryUsageBytes = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "vm_memory_usage_bytes",
            Help: "Memory usage in bytes per VM",
        },
        []string{"vm_id", "vm_name", "host_id"},
    )

    VMMemoryTotalBytes = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "vm_memory_total_bytes",
            Help: "Total memory allocated to a VM in bytes",
        },
    )

    VMSnapshotCount = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "vm_snapshot_count",
            Help: "Number of snapshots per VM",
        },
        []string{"vm_id", "vm_name"},
    )

    VMRole = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "vm_role",
            Help: "VM role: 1=generic, 2=hana",
        },
        []string{"vm_id", "vm_name", "role"},
    )
)

// Cluster metrics
var (
    ClusterVMCount = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "cluster_vm_count",
            Help: "Number of VMs in a cluster",
        },
        []string{"cluster_id", "cluster_name"},
    )

    ClusterHostCount = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "cluster_host_count",
            Help: "Number of hosts in a cluster",
        },
        []string{"cluster_id", "cluster_name"},
    )

    ClusterTotalCPU = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "cluster_total_cpu_cores",
            Help: "Total CPU cores in a cluster",
        },
        []string{"cluster_id", "cluster_name"},
    )

    ClusterTotalMemoryBytes = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "cluster_total_memory_bytes",
            Help: "Total memory in a cluster in bytes",
        },
        []string{"cluster_id", "cluster_name"},
    )
)

// HANA-specific metrics
var (
    HANAVMComplianceStatus = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "hana_vm_compliance_status",
            Help: "HANA VM compliance: 1=compliant, 0=non-compliant",
        },
        []string{"vm_id", "vm_name", "check_type"},
    )

    HANAVMMemoryReservationBytes = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "hana_vm_memory_reservation_bytes",
            Help: "HANA VM memory reservation in bytes (must equal total)",
        },
        []string{"vm_id", "vm_name"},
    )

    HANAVMHugepagesKB = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "hana_vm_hugepages_kb",
            Help: "HANA VM hugepages allocation in KB",
        },
        []string{"vm_id", "vm_name"},
    )

    HANAVMNUMANodes = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "hana_vm_numa_nodes",
            Help: "HANA VM NUMA nodes used (should be 1)",
        },
        []string{"vm_id", "vm_name"},
    )
)

// HAClusterHostID is the host_id label value used for cluster-wide HA node
// counts, which are aggregated across every monitored node.
const HAClusterHostID = "cluster"

// HA metrics
var (
    HANodesOnline = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "ha_nodes_online",
            Help: "Number of HA-monitored nodes in the online state",
        },
        []string{"host_id"},
    )

    HANodesSuspect = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "ha_nodes_suspect",
            Help: "Number of HA-monitored nodes in the suspect state (missed heartbeats)",
        },
        []string{"host_id"},
    )

    HANodesOffline = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "ha_nodes_offline",
            Help: "Number of HA-monitored nodes in the offline state",
        },
        []string{"host_id"},
    )

    HAFailoverTotal = promauto.NewCounter(
        prometheus.CounterOpts{
            Name: "ha_failover_total",
            Help: "Total number of completed HA failovers",
        },
    )

    HAFailoverDurationSeconds = promauto.NewHistogram(
        prometheus.HistogramOpts{
            Name:    "ha_failover_duration_seconds",
            Help:    "Duration of completed HA failovers in seconds",
            Buckets: []float64{1, 5, 10, 30, 60, 120, 180, 300},
        },
    )

    HAFencingTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "ha_fencing_total",
            Help: "Total number of successful node fencing operations by method",
        },
        []string{"method"},
    )
)

// System metrics
var (
    ManagerUp = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "manager_up",
            Help: "Manager service health: 1=up, 0=down",
        },
    )

    DatabaseConnectionsActive = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "database_connections_active",
            Help: "Number of active database connections",
        },
    )

    APIRequestsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "api_requests_total",
            Help: "Total API requests by method and path",
        },
        []string{"method", "path"},
    )

    APIRequestsDurationSeconds = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "api_request_duration_seconds",
            Help:    "API request duration in seconds",
            Buckets: prometheus.DefBuckets,
        },
        []string{"method", "path"},
    )

    EventPublishedTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "events_published_total",
            Help: "Total events published by type",
        },
        []string{"event_type", "severity"},
    )
)

// UpdateHostMetrics updates host-related Prometheus metrics.
func UpdateHostMetrics(hostID, hostname string, cpuPercent float64, memoryUsed, memoryTotal int64,
    diskUsed, diskTotal int64, vmCount int, status string) {
    HostCPUUsagePercent.WithLabelValues(hostID, hostname).Set(cpuPercent)
    HostMemoryUsageBytes.WithLabelValues(hostID, hostname).Set(float64(memoryUsed))
    HostMemoryTotalBytes.WithLabelValues(hostID, hostname).Set(float64(memoryTotal))
    HostDiskUsageBytes.WithLabelValues(hostID, hostname, "primary").Set(float64(diskUsed))
    HostDiskTotalBytes.WithLabelValues(hostID, hostname, "primary").Set(float64(diskTotal))
    HostVMCount.WithLabelValues(hostID, hostname).Set(float64(vmCount))
    HostStatus.WithLabelValues(hostID, hostname, status).Set(statusValue(status))
}

// UpdateVMMetrics updates VM-related Prometheus metrics.
func UpdateVMMetrics(vmID, vmName, hostID string, cpuPercent float64, memoryUsed, memoryTotal int64,
    snapshotCount int, role string) {
    VMCPUUsagePercent.WithLabelValues(vmID, vmName, hostID).Set(cpuPercent)
    VMMemoryUsageBytes.WithLabelValues(vmID, vmName, hostID).Set(float64(memoryUsed))
    VMSnapshotCount.WithLabelValues(vmID, vmName).Set(float64(snapshotCount))
    VMRole.WithLabelValues(vmID, vmName, role).Set(roleValue(role))
}

// UpdateHANAVMCompliance records HANA VM compliance status.
func UpdateHANAVMCompliance(vmID, vmName, checkType string, compliant bool, memoryReservation, hugepagesKB int64, numaNodes int) {
    HANAVMComplianceStatus.WithLabelValues(vmID, vmName, checkType).Set(complianceValue(compliant))
    HANAVMMemoryReservationBytes.WithLabelValues(vmID, vmName).Set(float64(memoryReservation))
    HANAVMHugepagesKB.WithLabelValues(vmID, vmName).Set(float64(hugepagesKB))
    HANAVMNUMANodes.WithLabelValues(vmID, vmName).Set(float64(numaNodes))
}

// UpdateClusterMetrics updates cluster-level Prometheus metrics.
func UpdateClusterMetrics(clusterID, clusterName string, vmCount, hostCount, totalCPU int, totalMemory int64) {
    ClusterVMCount.WithLabelValues(clusterID, clusterName).Set(float64(vmCount))
    ClusterHostCount.WithLabelValues(clusterID, clusterName).Set(float64(hostCount))
    ClusterTotalCPU.WithLabelValues(clusterID, clusterName).Set(float64(totalCPU))
    ClusterTotalMemoryBytes.WithLabelValues(clusterID, clusterName).Set(float64(totalMemory))
}

// UpdateHAHealthMetrics records the cluster-wide count of HA-monitored nodes
// in each health state.
func UpdateHAHealthMetrics(online, suspect, offline int) {
    HANodesOnline.WithLabelValues(HAClusterHostID).Set(float64(online))
    HANodesSuspect.WithLabelValues(HAClusterHostID).Set(float64(suspect))
    HANodesOffline.WithLabelValues(HAClusterHostID).Set(float64(offline))
}

// RecordFailover records a completed HA failover and how long it took.
func RecordFailover(duration time.Duration) {
    HAFailoverTotal.Inc()
    HAFailoverDurationSeconds.Observe(duration.Seconds())
}

// RecordFencing records a successful fencing operation for the given method
// (e.g. "ipmi", "redfish", "ssh").
func RecordFencing(method string) {
    HAFencingTotal.WithLabelValues(method).Inc()
}

// SetManagerUp sets the manager health metric.
func SetManagerUp(up bool) {
    if up {
        ManagerUp.Set(1)
    } else {
        ManagerUp.Set(0)
    }
}

// helper functions
func statusValue(s string) float64 {
    switch s {
    case "online":
        return 1
    case "maintenance":
        return -1
    default:
        return 0
    }
}

func roleValue(r string) float64 {
    switch r {
    case "hana":
        return 2
    default:
        return 1
    }
}

func complianceValue(c bool) float64 {
    if c {
        return 1
    }
    return 0
}
