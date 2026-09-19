package libvirt

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// cpuSampleInterval is the gap between the two CPU-time samples used to derive
// CPU utilisation.
const cpuSampleInterval = 500 * time.Millisecond

// VMStats is a point-in-time view of a VM's resource usage.
type VMStats struct {
	// CPUUsage is the guest CPU utilisation in percent of all its vCPUs (0-100).
	CPUUsage float64 `json:"cpu_usage"`
	// MemoryUsage is the memory in use by the guest, in bytes.
	MemoryUsage uint64 `json:"memory_usage"`
	// MemoryTotal is the memory currently given to the guest, in bytes.
	MemoryTotal uint64 `json:"memory_total"`
	// DiskReadBytes and DiskWriteBytes are cumulative bytes over all block devices.
	DiskReadBytes  uint64 `json:"disk_read_bytes"`
	DiskWriteBytes uint64 `json:"disk_write_bytes"`
	// DiskIO is DiskReadBytes + DiskWriteBytes.
	DiskIO uint64 `json:"disk_io"`
	// NetRxBytes and NetTxBytes are cumulative bytes over all interfaces.
	NetRxBytes uint64 `json:"net_rx_bytes"`
	NetTxBytes uint64 `json:"net_tx_bytes"`
	// NetworkIO is NetRxBytes + NetTxBytes.
	NetworkIO uint64 `json:"network_io"`
}

// domStats holds the raw counters reported by `virsh domstats` for one domain.
type domStats struct {
	cpuTimeNs      uint64 // cumulative CPU time of all vCPUs, nanoseconds
	vcpus          uint64
	memCurrentKiB  uint64
	memUnusedKiB   uint64
	memRSSKiB      uint64
	hasMemUnused   bool
	diskReadBytes  uint64
	diskWriteBytes uint64
	netRxBytes     uint64
	netTxBytes     uint64
}

// domstatsFunc returns the raw `virsh domstats` output for the domain id.
type domstatsFunc func(ctx context.Context, uri, id string) (string, error)

// virshDomstats runs `virsh domstats` for one domain against the libvirt uri.
func virshDomstats(ctx context.Context, uri, id string) (string, error) {
	args := []string{"--connect", uri, "domstats", "--cpu-total", "--balloon", "--vcpu", "--interface", "--block", id}
	out, err := exec.CommandContext(ctx, "virsh", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("virsh domstats %s: %w: %s", id, err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// parseDomStats parses the output of `virsh domstats` for a single
// domain. Per-device counters (block.N.*, net.N.*) are summed. It fails if the
// output has no domain section, which is what virsh prints for an unknown VM.
func parseDomStats(out string) (domStats, error) {
	var ds domStats
	sawDomain := false

	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "Domain:") {
			sawDomain = true
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		n, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			continue // non-numeric fields such as net.0.name
		}
		switch {
		case key == "cpu.time":
			ds.cpuTimeNs = n
		case key == "vcpu.current":
			ds.vcpus = n
		case key == "balloon.current":
			ds.memCurrentKiB = n
		case key == "balloon.unused":
			ds.memUnusedKiB, ds.hasMemUnused = n, true
		case key == "balloon.rss":
			ds.memRSSKiB = n
		case strings.HasPrefix(key, "block.") && strings.HasSuffix(key, ".rd.bytes"):
			ds.diskReadBytes += n
		case strings.HasPrefix(key, "block.") && strings.HasSuffix(key, ".wr.bytes"):
			ds.diskWriteBytes += n
		case strings.HasPrefix(key, "net.") && strings.HasSuffix(key, ".rx.bytes"):
			ds.netRxBytes += n
		case strings.HasPrefix(key, "net.") && strings.HasSuffix(key, ".tx.bytes"):
			ds.netTxBytes += n
		}
	}
	if err := sc.Err(); err != nil {
		return domStats{}, err
	}
	if !sawDomain {
		return domStats{}, fmt.Errorf("no statistics reported for domain")
	}
	return ds, nil
}

// stats converts the raw counters into VMStats. cpuPercent is supplied by the
// caller because it needs two samples.
func (ds domStats) stats(cpuPercent float64) *VMStats {
	// Prefer the guest-reported figure (current - unused); fall back to the
	// resident set size of the QEMU process when the balloon driver does not
	// report it.
	used := ds.memRSSKiB
	if ds.hasMemUnused && ds.memUnusedKiB <= ds.memCurrentKiB {
		used = ds.memCurrentKiB - ds.memUnusedKiB
	}
	return &VMStats{
		CPUUsage:       cpuPercent,
		MemoryUsage:    used * 1024,
		MemoryTotal:    ds.memCurrentKiB * 1024,
		DiskReadBytes:  ds.diskReadBytes,
		DiskWriteBytes: ds.diskWriteBytes,
		DiskIO:         ds.diskReadBytes + ds.diskWriteBytes,
		NetRxBytes:     ds.netRxBytes,
		NetTxBytes:     ds.netTxBytes,
		NetworkIO:      ds.netRxBytes + ds.netTxBytes,
	}
}

// cpuPercent returns the utilisation between two samples taken elapsed apart,
// as a percentage of all vCPUs, clamped to 0-100.
func cpuPercent(first, second domStats, elapsed time.Duration) float64 {
	vcpus := second.vcpus
	if vcpus == 0 {
		vcpus = 1
	}
	if elapsed <= 0 || second.cpuTimeNs < first.cpuTimeNs {
		return 0
	}
	pct := float64(second.cpuTimeNs-first.cpuTimeNs) / (float64(elapsed.Nanoseconds()) * float64(vcpus)) * 100
	return min(pct, 100)
}

// GetVMStats returns the current resource usage of the VM with the given ID
// (a libvirt domain name, numeric ID or UUID). CPU utilisation is measured over
// a short sampling window, so the call takes about half a second.
func (l *Libvirt) GetVMStats(ctx context.Context, id string) (*VMStats, error) {
	if id == "" {
		return nil, fmt.Errorf("vm id is required")
	}
	if strings.HasPrefix(id, "-") {
		return nil, fmt.Errorf("invalid vm id %q", id)
	}

	l.mu.Lock()
	connected, uri, run := l.connected, l.uri, l.domstats
	l.mu.Unlock()
	if !connected {
		return nil, fmt.Errorf("not connected")
	}
	if run == nil {
		run = virshDomstats
	}

	sample := func() (domStats, error) {
		out, err := run(ctx, uri, id)
		if err != nil {
			return domStats{}, err
		}
		ds, err := parseDomStats(out)
		if err != nil {
			return domStats{}, fmt.Errorf("vm %s: %w", id, err)
		}
		return ds, nil
	}

	first, err := sample()
	if err != nil {
		return nil, err
	}
	firstAt := time.Now()
	timer := time.NewTimer(cpuSampleInterval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
	}
	second, err := sample()
	if err != nil {
		return nil, err
	}
	return second.stats(cpuPercent(first, second, time.Since(firstAt))), nil
}
