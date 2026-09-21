package libvirt

import (
	"context"
	"sync"
	"testing"
)

func TestNewLibvirt(t *testing.T) {
	lv, err := NewLibvirt("qemu:///system")
	if err != nil {
		t.Fatal(err)
	}
	if lv == nil {
		t.Fatal("expected non-nil Libvirt")
	}
	if lv.uri != "qemu:///system" {
		t.Errorf("uri = %q", lv.uri)
	}
	if lv.connected {
		t.Error("expected not connected initially")
	}
}

func TestLibvirt_Connect(t *testing.T) {
	lv, _ := NewLibvirt("qemu:///system")
	if err := lv.Connect(); err != nil {
		t.Fatal(err)
	}
	if !lv.connected {
		t.Error("expected connected after Connect()")
	}
}

func TestLibvirt_Connect_ThreadSafe(t *testing.T) {
	lv, _ := NewLibvirt("qemu:///system")
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := lv.Connect(); err != nil {
				t.Errorf("Connect error: %v", err)
			}
		}()
	}
	wg.Wait()
	if !lv.connected {
		t.Error("expected connected after concurrent Connects")
	}
}

func TestLibvirt_ActionsWithoutConnect(t *testing.T) {
	lv, _ := NewLibvirt("test:///default")
	// Not connected — all actions should fail
	ctx := context.Background()

	tests := []struct {
		name string
		fn   func() error
	}{
		{"GetHostInfo", func() error { _, err := lv.GetHostInfo(ctx); return err }},
		{"ListVMs", func() error { _, err := lv.ListVMs(ctx); return err }},
		{"DefineVM", func() error { return lv.DefineVM(ctx, "<xml/>") }},
		{"StartVM", func() error { return lv.StartVM(ctx, "vm1") }},
		{"StopVM", func() error { return lv.StopVM(ctx, "vm1") }},
		{"DestroyVM", func() error { return lv.DestroyVM(ctx, "vm1") }},
		{"CreateVolume", func() error { return lv.CreateVolume(ctx, "pool", "vol", "qcow2", 1024) }},
		{"CreateNetwork", func() error { return lv.CreateNetwork(ctx, "net", "br0") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			if err == nil {
				t.Errorf("%s: expected error when not connected", tt.name)
			}
			if err.Error() != "not connected" {
				t.Errorf("expected 'not connected' error, got %q", err.Error())
			}
		})
	}
}

func TestLibvirt_ActionsAfterConnect(t *testing.T) {
	lv, _ := NewLibvirt("test:///default")
	lv.Connect()
	ctx := context.Background()

	// After connecting, actions should succeed (stub mode)
	info, err := lv.GetHostInfo(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if info.CPU.Count != 4 {
		t.Errorf("CPU.Count = %d", info.CPU.Count)
	}
	if info.Memory.Total != 17179869184 {
		t.Errorf("Memory.Total = %d", info.Memory.Total)
	}

	vms, err := lv.ListVMs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(vms) != 0 {
		t.Errorf("expected 0 VMs, got %d", len(vms))
	}

	if err := lv.DefineVM(ctx, "<domain/>"); err != nil {
		t.Fatal(err)
	}
	if err := lv.StartVM(ctx, "vm1"); err != nil {
		t.Fatal(err)
	}
	if err := lv.StopVM(ctx, "vm1"); err != nil {
		t.Fatal(err)
	}
	if err := lv.DestroyVM(ctx, "vm1"); err != nil {
		t.Fatal(err)
	}
	if err := lv.CreateVolume(ctx, "pool", "vol", "qcow2", 1024); err != nil {
		t.Fatal(err)
	}
	if err := lv.CreateNetwork(ctx, "net", "br0"); err != nil {
		t.Fatal(err)
	}
}

func TestLibvirt_Close(t *testing.T) {
	lv, _ := NewLibvirt("test:///default")
	lv.Connect()
	if err := lv.Close(); err != nil {
		t.Fatal(err)
	}
	if lv.connected {
		t.Error("expected disconnected after Close()")
	}
}

func TestLibvirt_DoubleClose(t *testing.T) {
	lv, _ := NewLibvirt("test:///default")
	lv.Connect()
	lv.Close()
	// Second close should not panic
	if err := lv.Close(); err != nil {
		t.Fatalf("double close error: %v", err)
	}
}

func TestLibvirt_HostInfo_DefaultValues(t *testing.T) {
	lv, _ := NewLibvirt("test:///default")
	lv.Connect()
	info, _ := lv.GetHostInfo(context.Background())

	if info.Hostname != "" {
		t.Errorf("Hostname = %q", info.Hostname)
	}
	if info.CPU.Count != 4 {
		t.Errorf("CPU.Count = %d", info.CPU.Count)
	}
	if info.CPU.Model != "x86_64" {
		t.Errorf("CPU.Model = %q", info.CPU.Model)
	}
	if info.Memory.Total != 17179869184 {
		t.Errorf("Memory.Total = %d", info.Memory.Total)
	}
	if info.Memory.Free != 8589934592 {
		t.Errorf("Memory.Free = %d", info.Memory.Free)
	}
	if info.Disk.Total != 1099511627776 {
		t.Errorf("Disk.Total = %d", info.Disk.Total)
	}
	if info.Disk.Free != 549755813888 {
		t.Errorf("Disk.Free = %d", info.Disk.Free)
	}
	if info.NUMANodeCount != 0 {
		t.Errorf("NUMANodeCount = %d", info.NUMANodeCount)
	}
	if info.HugepagesTotalMB != 0 {
		t.Errorf("HugepagesTotalMB = %d", info.HugepagesTotalMB)
	}
	if info.HugepagesFreeMB != 0 {
		t.Errorf("HugepagesFreeMB = %d", info.HugepagesFreeMB)
	}
	if info.StorageTotalGB != 0 {
		t.Errorf("StorageTotalGB = %d", info.StorageTotalGB)
	}
	if info.StorageFreeGB != 0 {
		t.Errorf("StorageFreeGB = %d", info.StorageFreeGB)
	}
	if len(info.NetworkInterfaces) != 0 {
		t.Errorf("NetworkInterfaces len = %d", len(info.NetworkInterfaces))
	}
	if len(info.NUMANodes) != 0 {
		t.Errorf("NUMANodes len = %d", len(info.NUMANodes))
	}
}

func TestLibvirt_VMInfo_DefaultEmpty(t *testing.T) {
	lv, _ := NewLibvirt("test:///default")
	lv.Connect()
	vms, _ := lv.ListVMs(context.Background())
	if len(vms) != 0 {
		t.Fatalf("expected empty VM list, got %d VMs", len(vms))
	}
}

func TestLibvirt_Concurrency(t *testing.T) {
	lv, _ := NewLibvirt("test:///default")
	lv.Connect()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := lv.GetHostInfo(context.Background()); err != nil {
				t.Errorf("GetHostInfo error: %v", err)
			}
		}()
	}
	wg.Wait()
}

func TestCPUInfo(t *testing.T) {
	cpu := CPUInfo{Count: 8, Model: "x86_64"}
	if cpu.Count != 8 {
		t.Errorf("Count = %d", cpu.Count)
	}
	if cpu.Model != "x86_64" {
		t.Errorf("Model = %q", cpu.Model)
	}
}

func TestMemoryInfo(t *testing.T) {
	// Simulate 16GB:
	mem := MemoryInfo{Total: 17179869184, Free: 8589934592}
	if mem.Total != 17179869184 {
		t.Errorf("Total = %d", mem.Total)
	}
	if mem.Free != 8589934592 {
		t.Errorf("Free = %d", mem.Free)
	}
}

const (
	KB = 1024
	MB = 1024 * KB
	GB = 1024 * MB
)

func TestDiskInfo(t *testing.T) {
	disk := DiskInfo{Total: 100 * GB, Free: 50 * GB}
	if disk.Total != 100*GB {
		t.Errorf("Total = %d", disk.Total)
	}
	if disk.Free != 50*GB {
		t.Errorf("Free = %d", disk.Free)
	}
}

func TestNumaNodeInfo(t *testing.T) {
	node := NumaNodeInfo{NodeID: 0, CPUs: []int{0, 1, 2, 3}, MemoryMB: 8192}
	if node.NodeID != 0 {
		t.Errorf("NodeID = %d", node.NodeID)
	}
	if len(node.CPUs) != 4 {
		t.Errorf("CPUs len = %d", len(node.CPUs))
	}
	if node.MemoryMB != 8192 {
		t.Errorf("MemoryMB = %d", node.MemoryMB)
	}
}
