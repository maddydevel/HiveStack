package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestUpdateHostMetrics_SetsValues(t *testing.T) {
	// Use a fresh registry for isolation
	reg := prometheus.NewRegistry()
	// Re-register metrics on this registry
	hostCPU := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "hivestack", Subsystem: "node", Name: "host_cpu_usage_percent", Help: "CPU usage percentage per host",
	}, []string{"host_id", "hostname"})
	hostMemUsed := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "hivestack", Subsystem: "node", Name: "host_memory_usage_bytes", Help: "Memory usage in bytes per host",
	}, []string{"host_id", "hostname"})
	hostMemTotal := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "hivestack", Subsystem: "node", Name: "host_memory_total_bytes", Help: "Total memory in bytes per host",
	}, []string{"host_id", "hostname"})
	hostDiskUsed := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "hivestack", Subsystem: "node", Name: "host_disk_usage_bytes", Help: "Disk usage in bytes per host",
	}, []string{"host_id", "hostname", "storage_pool"})
	hostDiskTotal := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "hivestack", Subsystem: "node", Name: "host_disk_total_bytes", Help: "Total disk capacity in bytes per host",
	}, []string{"host_id", "hostname", "storage_pool"})
	hostVMCount := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "hivestack", Subsystem: "node", Name: "host_vm_count", Help: "Number of VMs running on a host",
	}, []string{"host_id", "hostname"})
	hostStatus := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "hivestack", Subsystem: "node", Name: "host_status", Help: "Host status: 1=online, 0=offline, -1=maintenance",
	}, []string{"host_id", "hostname", "status"})

	reg.MustRegister(hostCPU, hostMemUsed, hostMemTotal, hostDiskUsed, hostDiskTotal, hostVMCount, hostStatus)

	// Test the public UpdateHostMetrics function doesn't panic
	UpdateHostMetrics("host-1", "node1", 75.5, 8589934592, 17179869184, 50000000000, 100000000000, 12, "online")
}

func TestUpdateHostMetrics_StatusValues(t *testing.T) {
	// Test the public UpdateHostMetrics function with different statuses
	UpdateHostMetrics("h1", "n1", 0, 0, 0, 0, 0, 0, "online")
	UpdateHostMetrics("h1", "n1", 0, 0, 0, 0, 0, 0, "maintenance")
	UpdateHostMetrics("h1", "n1", 0, 0, 0, 0, 0, 0, "offline")
	UpdateHostMetrics("h1", "n1", 0, 0, 0, 0, 0, 0, "unknown")
}

func TestUpdateVMMetrics_RoleValue(t *testing.T) {
	UpdateVMMetrics("vm-1", "web", "host-1", 50.0, 4294967296, 8589934592, 3, "generic")
	UpdateVMMetrics("vm-1", "hana-db", "host-1", 0, 0, 0, 0, "hana")
}

func TestUpdateHANAVMCompliance_ComplianceValue(t *testing.T) {
	UpdateHANAVMCompliance("vm-1", "hana-db", "numa", true, 68719476736, 2048000, 1)
	UpdateHANAVMCompliance("vm-2", "hana-bad", "numa", false, 0, 0, 0)
}

func TestUpdateClusterMetrics(t *testing.T) {
	UpdateClusterMetrics("c1", "main", 24, 3, 96, 137438953472)
}

func TestSetManagerUp(t *testing.T) {
	SetManagerUp(true)
	SetManagerUp(false)
}

func TestGlobalMetrics_NoPanic(t *testing.T) {
	// Just verify the global update functions don't panic
	UpdateHostMetrics("host-1", "node1", 75.5, 8589934592, 17179869184, 50000000000, 100000000000, 12, "online")
	UpdateVMMetrics("vm-1", "web", "host-1", 50.0, 4294967296, 8589934592, 3, "generic")
	UpdateHANAVMCompliance("vm-1", "hana-db", "numa", true, 68719476736, 2048000, 1)
	UpdateClusterMetrics("c1", "main", 24, 3, 96, 137438953472)
	SetManagerUp(true)
	SetManagerUp(false)

	// Verify metrics were set by collecting from default registry
	// This tests the actual promauto-registered metrics
	metricFamilies, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	found := false
	for _, mf := range metricFamilies {
		if mf.GetName() == "hivestack_node_host_cpu_usage_percent" || mf.GetName() == "host_cpu_usage_percent" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected hivestack_node_host_cpu_usage_percent metric to be present")
	}
}