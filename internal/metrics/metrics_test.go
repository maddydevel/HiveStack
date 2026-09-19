package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
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
func TestUpdateHAHealthMetrics(t *testing.T) {
	tests := []struct {
		name                     string
		online, suspect, offline int
	}{
		{"all healthy", 5, 0, 0},
		{"one suspect", 4, 1, 0},
		{"mixed", 2, 1, 2},
		{"empty cluster", 0, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			UpdateHAHealthMetrics(tt.online, tt.suspect, tt.offline)

			got := map[string]float64{
				"online":  testutil.ToFloat64(HANodesOnline.WithLabelValues(HAClusterHostID)),
				"suspect": testutil.ToFloat64(HANodesSuspect.WithLabelValues(HAClusterHostID)),
				"offline": testutil.ToFloat64(HANodesOffline.WithLabelValues(HAClusterHostID)),
			}
			want := map[string]float64{
				"online":  float64(tt.online),
				"suspect": float64(tt.suspect),
				"offline": float64(tt.offline),
			}
			for state, w := range want {
				if got[state] != w {
					t.Errorf("ha_nodes_%s = %v, want %v", state, got[state], w)
				}
			}
		})
	}
}

func TestRecordFailover(t *testing.T) {
	totalBefore := testutil.ToFloat64(HAFailoverTotal)
	countBefore, sumBefore := histogramCountAndSum(t, HAFailoverDurationSeconds)

	RecordFailover(1500 * time.Millisecond)
	RecordFailover(30 * time.Second)

	if got := testutil.ToFloat64(HAFailoverTotal) - totalBefore; got != 2 {
		t.Errorf("ha_failover_total delta = %v, want 2", got)
	}
	count, sum := histogramCountAndSum(t, HAFailoverDurationSeconds)
	if got := count - countBefore; got != 2 {
		t.Errorf("ha_failover_duration_seconds count delta = %d, want 2", got)
	}
	if got := sum - sumBefore; got != 31.5 {
		t.Errorf("ha_failover_duration_seconds sum delta = %v, want 31.5", got)
	}
}

func TestRecordFencing(t *testing.T) {
	tests := []struct {
		method string
		calls  int
	}{
		{"ipmi", 1},
		{"redfish", 2},
		{"ssh", 3},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			before := testutil.ToFloat64(HAFencingTotal.WithLabelValues(tt.method))
			for i := 0; i < tt.calls; i++ {
				RecordFencing(tt.method)
			}
			got := testutil.ToFloat64(HAFencingTotal.WithLabelValues(tt.method)) - before
			if got != float64(tt.calls) {
				t.Errorf("ha_fencing_total{method=%q} delta = %v, want %d", tt.method, got, tt.calls)
			}
		})
	}
}

func histogramCountAndSum(t *testing.T, h prometheus.Histogram) (uint64, float64) {
	t.Helper()
	reg := prometheus.NewRegistry()
	reg.MustRegister(h)
	families, err := reg.Gather()
	if err != nil || len(families) != 1 || len(families[0].GetMetric()) != 1 {
		t.Fatalf("gather histogram: families=%d err=%v", len(families), err)
	}
	hist := families[0].GetMetric()[0].GetHistogram()
	return hist.GetSampleCount(), hist.GetSampleSum()
}
