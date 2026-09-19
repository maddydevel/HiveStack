package preflight

import (
	"testing"

	"github.com/maddydevel/HiveStack/internal/vcenter"
)

func TestDefaultChecker(t *testing.T) {
	c := DefaultChecker()
	if c.TargetMaxCPUs == 0 {
		t.Error("expected non-zero TargetMaxCPUs")
	}
	if c.TargetMaxMemoryMB == 0 {
		t.Error("expected non-zero TargetMaxMemoryMB")
	}
	if len(c.SupportedGuestOS) == 0 {
		t.Error("expected non-empty SupportedGuestOS")
	}
	if len(c.NetworkMappings) == 0 {
		t.Error("expected non-empty NetworkMappings")
	}
}

func TestCheckCPU(t *testing.T) {
	c := DefaultChecker()

	tests := []struct {
		name    string
		cpu     int
		wantOK  bool
		severity string
	}{
		{"valid 4 CPUs", 4, true, SeverityInfo},
		{"valid 128 CPUs", 128, true, SeverityInfo},
		{"zero CPUs", 0, false, SeverityCritical},
		{"negative CPUs", -1, false, SeverityCritical},
		{"exceeds max", 512, false, SeverityCritical},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := vcenter.VM{
				ID:      "test-vm",
				Name:    "test",
				NumCPUs: tt.cpu,
			}
			result := c.CheckVM(vm)
			cpuFound := false
			for _, check := range result.Checks {
				if check.Name == "cpu_compatibility" {
					cpuFound = true
					if check.Passed != tt.wantOK {
						t.Errorf("expected Passed=%v, got %v", tt.wantOK, check.Passed)
					}
					if check.Severity != tt.severity {
						t.Errorf("expected severity %s, got %s", tt.severity, check.Severity)
					}
				}
			}
			if !cpuFound {
				t.Error("expected cpu_compatibility check in results")
			}
		})
	}
}

func TestCheckMemory(t *testing.T) {
	c := DefaultChecker()

	tests := []struct {
		name   string
		memMB  int64
		wantOK bool
	}{
		{"valid 4GB", 4 * 1024, true},
		{"valid 128GB", 128 * 1024, true},
		{"zero memory", 0, false},
		{"negative memory", -1, false},
		{"exceeds max", c.TargetMaxMemoryMB + 1, false},
		{"hugepage misaligned", 4097, true}, // passes but warning
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := vcenter.VM{
				ID:       "test-vm",
				Name:     "test",
				MemoryMB: tt.memMB,
			}
			result := c.CheckVM(vm)
			memFound := false
			for _, check := range result.Checks {
				if check.Name == "memory_configuration" {
					memFound = true
					if check.Passed != tt.wantOK {
						t.Errorf("expected Passed=%v, got %v", tt.wantOK, check.Passed)
					}
				}
			}
			if !memFound {
				t.Error("expected memory_configuration check in results")
			}
		})
	}
}

func TestCheckDisk(t *testing.T) {
	c := DefaultChecker()

	tests := []struct {
		name    string
		diskGB  int64
		wantOK  bool
	}{
		{"valid 100GB", 100, true},
		{"valid 1TB", 1024, true},
		{"zero disk", 0, false},
		{"exceeds max", c.TargetMaxDiskGB + 1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := vcenter.VM{
				ID:      "test-vm",
				Name:    "test",
				DiskGB:  tt.diskGB,
			}
			result := c.CheckVM(vm)
			diskFound := false
			for _, check := range result.Checks {
				if check.Name == "disk_compatibility" {
					diskFound = true
					if check.Passed != tt.wantOK {
						t.Errorf("expected Passed=%v, got %v", tt.wantOK, check.Passed)
					}
				}
			}
			if !diskFound {
				t.Error("expected disk_compatibility check in results")
			}
		})
	}
}

func TestCheckGuestOS(t *testing.T) {
	c := DefaultChecker()

	tests := []struct {
		name    string
		guestOS string
		wantOK  bool
	}{
		{"SLES 15", "sles15_64Guest", true},
		{"RHEL 9", "rhel9_64Guest", true},
		{"Unknown OS", "freebsd13_64Guest", false},
		{"Empty OS", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := vcenter.VM{
				ID:       "test-vm",
				Name:     "test",
				GuestOS:  tt.guestOS,
			}
			result := c.CheckVM(vm)
			osFound := false
			for _, check := range result.Checks {
				if check.Name == "guest_os_support" {
					osFound = true
					if check.Passed != tt.wantOK {
						t.Errorf("expected Passed=%v, got %v", tt.wantOK, check.Passed)
					}
				}
			}
			if !osFound {
				t.Error("expected guest_os_support check in results")
			}
		})
	}
}

func TestCheckNetworks(t *testing.T) {
	c := DefaultChecker()

	tests := []struct {
		name    string
		networks []string
		wantOK  bool
	}{
		{"mapped network", []string{"VM Network"}, true},
		{"multiple mapped", []string{"VM Network", "SAP-DB"}, true},
		{"unmapped network", []string{"Unknown Network"}, false},
		{"no networks", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := vcenter.VM{
				ID:       "test-vm",
				Name:     "test",
				Networks: tt.networks,
			}
			result := c.CheckVM(vm)
			netFound := false
			for _, check := range result.Checks {
				if check.Name == "network_mapping" {
					netFound = true
					if check.Passed != tt.wantOK {
						t.Errorf("expected Passed=%v, got %v", tt.wantOK, check.Passed)
					}
				}
			}
			if !netFound {
				t.Error("expected network_mapping check in results")
			}
		})
	}
}

func TestCheckAll(t *testing.T) {
	c := DefaultChecker()

	vms := []vcenter.VM{
		{
			ID:      "vm-1",
			Name:    "test1",
			NumCPUs: 2,
			MemoryMB: 4096,
			DiskGB:  50,
			GuestOS: "sles15_64Guest",
			Networks: []string{"VM Network"},
		},
		{
			ID:      "vm-2",
			Name:    "test2",
			NumCPUs: 4,
			MemoryMB: 8192,
			DiskGB:  100,
			GuestOS: "rhel9_64Guest",
			Networks: []string{"SAP-App"},
		},
	}

	results := c.CheckAll(vms)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	for _, r := range results {
		if !r.Passed {
			t.Errorf("expected VM %s to pass preflight", r.VMID)
		}
	}
}

func TestSAPPass(t *testing.T) {
	c := DefaultChecker()

	sapVM := vcenter.VM{
		ID:               "vm-sap-1",
		Name:            "SAP-APP-01",
		PowerState:       "poweredOn",
		GuestOS:          "sles15_64Guest",
		NumCPUs:          8,
		MemoryMB:         32768,
		DiskGB:           200,
		VMwareTools:      "toolsOk",
		Networks:         []string{"VM Network", "SAP-App"},
		ProvisioningType: "thin",
	}

	result := c.CheckVM(sapVM)
	if !result.Passed {
		t.Errorf("SAP VM should pass preflight: %+v", result.Checks)
	}
}
