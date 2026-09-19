// Package preflight provides compatibility checks for imported VMs.
//
// Before migrating a VMware VM to KVM, we verify that the target host
// can support the VM's configuration: CPU features, memory layout,
// disk format, network interfaces, and guest OS compatibility.
package preflight

import (
	"fmt"
	"strings"

	"github.com/maddydevel/HiveStack/internal/vcenter"
)

// Checker performs pre-flight compatibility checks.
type Checker struct {
	// TargetCPUFeatures lists CPU features supported by KVM hosts.
	TargetCPUFeatures []string
	// TargetMaxCPUs is the maximum vCPUs per VM KVM supports.
	TargetMaxCPUs int
	// TargetMaxMemoryMB is the maximum memory per VM KVM supports.
	TargetMaxMemoryMB int64
	// TargetMaxDiskGB is the maximum disk size per VM.
	TargetMaxDiskGB int64
	// NetworkMappings maps vCenter port group names to KVM bridge names.
	NetworkMappings map[string]string
	// SupportedGuestOS lists guest OS identifiers KVM can run.
	SupportedGuestOS []string
}

// DefaultChecker returns a Checker with reasonable defaults.
func DefaultChecker() *Checker {
	return &Checker{
		TargetCPUFeatures: []string{"vmx", "sse4_2", "avx", "x2apic", "aes"},
		TargetMaxCPUs:     256,
		TargetMaxMemoryMB: 4 * 1024 * 1024, // 4 TB
		TargetMaxDiskGB:   64 * 1024,       // 64 TB
		NetworkMappings: map[string]string{
			"VM Network": "br0",
			"vMotion":    "br-vmotion",
			"SAP-App":    "br-sap-app",
			"SAP-DB":     "br-sap-db",
		},
		SupportedGuestOS: []string{
			"sles15_64Guest",
			"sles12_64Guest",
			"rhel8_64Guest",
			"rhel9_64Guest",
			"ubuntu64Guest",
			"windows2019srv_64Guest",
			"windows2022srvNext_64Guest",
		},
	}
}

// Result aggregates all checks for a single VM.
type Result struct {
	VMID     string        `json:"vm_id"`
	VMName   string        `json:"vm_name"`
	Passed   bool          `json:"passed"`
	Checks   []CheckDetail `json:"checks"`
}

// CheckDetail is a single compatibility check.
type CheckDetail struct {
	Name     string `json:"name"`
	Passed   bool   `json:"passed"`
	Severity string `json:"severity"` // "info", "warning", "critical"
	Message  string `json:"message"`
}

// Severity constants.
const (
	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityCritical = "critical"
)

// CheckVM performs all pre-flight checks on a single VM.
func (c *Checker) CheckVM(vm vcenter.VM) Result {
	result := Result{
		VMID:   vm.ID,
		VMName: vm.Name,
		Checks: make([]CheckDetail, 0),
	}

	// Check 1: CPU compatibility
	result.Checks = append(result.Checks, c.checkCPU(vm))

	// Check 2: Memory configuration
	result.Checks = append(result.Checks, c.checkMemory(vm))

	// Check 3: Disk size and format
	result.Checks = append(result.Checks, c.checkDisk(vm))

	// Check 4: Guest OS compatibility
	result.Checks = append(result.Checks, c.checkGuestOS(vm))

	// Check 5: Network mapping
	result.Checks = append(result.Checks, c.checkNetworks(vm))

	// Check 6: VMware Tools
	result.Checks = append(result.Checks, c.checkVMwareTools(vm))

	// Check 7: Power state
	result.Checks = append(result.Checks, c.checkPowerState(vm))

	// Check 8: Disk provisioning type
	result.Checks = append(result.Checks, c.checkProvisioningType(vm))

	// Determine overall pass/fail
	result.Passed = true
	for _, check := range result.Checks {
		if !check.Passed && check.Severity == SeverityCritical {
			result.Passed = false
			break
		}
	}

	return result
}

// CheckAll runs pre-flight checks on all VMs.
func (c *Checker) CheckAll(vms []vcenter.VM) []Result {
	results := make([]Result, 0, len(vms))
	for _, vm := range vms {
		results = append(results, c.CheckVM(vm))
	}
	return results
}

// checkCPU verifies CPU configuration is compatible with KVM.
func (c *Checker) checkCPU(vm vcenter.VM) CheckDetail {
	name := "cpu_compatibility"
	if vm.NumCPUs <= 0 {
		return CheckDetail{
			Name:     name,
			Passed:   false,
			Severity: SeverityCritical,
			Message:  fmt.Sprintf("VM has invalid CPU count: %d", vm.NumCPUs),
		}
	}

	if vm.NumCPUs > c.TargetMaxCPUs {
		return CheckDetail{
			Name:     name,
			Passed:   false,
			Severity: SeverityCritical,
			Message:  fmt.Sprintf("VM has %d vCPUs, exceeds KVM maximum of %d", vm.NumCPUs, c.TargetMaxCPUs),
		}
	}

	return CheckDetail{
		Name:     name,
		Passed:   true,
		Severity: SeverityInfo,
		Message:  fmt.Sprintf("%d vCPUs within KVM limit of %d", vm.NumCPUs, c.TargetMaxCPUs),
	}
}

// checkMemory verifies memory configuration is compatible.
func (c *Checker) checkMemory(vm vcenter.VM) CheckDetail {
	name := "memory_configuration"
	if vm.MemoryMB <= 0 {
		return CheckDetail{
			Name:     name,
			Passed:   false,
			Severity: SeverityCritical,
			Message:  fmt.Sprintf("VM has invalid memory: %d MB", vm.MemoryMB),
		}
	}

	if vm.MemoryMB > c.TargetMaxMemoryMB {
		return CheckDetail{
			Name:     name,
			Passed:   false,
			Severity: SeverityCritical,
			Message:  fmt.Sprintf("VM has %d MB memory, exceeds KVM maximum of %d MB", vm.MemoryMB, c.TargetMaxMemoryMB),
		}
	}

	// Warn if memory is not aligned to 2 MB (hugepage requirement)
	if vm.MemoryMB%(2*1024) != 0 && vm.MemoryMB > 4*1024 {
		return CheckDetail{
			Name:     name,
			Passed:   true,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("%d MB memory is not hugepage-aligned; consider adjusting to nearest 2 GB boundary", vm.MemoryMB),
		}
	}

	return CheckDetail{
		Name:     name,
		Passed:   true,
		Severity: SeverityInfo,
		Message:  fmt.Sprintf("%d MB memory within KVM limit of %d MB", vm.MemoryMB, c.TargetMaxMemoryMB),
	}
}

// checkDisk verifies disk configuration is compatible.
func (c *Checker) checkDisk(vm vcenter.VM) CheckDetail {
	name := "disk_compatibility"
	if vm.DiskGB <= 0 {
		return CheckDetail{
			Name:     name,
			Passed:   false,
			Severity: SeverityCritical,
			Message:  fmt.Sprintf("VM has invalid disk size: %d GB", vm.DiskGB),
		}
	}

	if vm.DiskGB > c.TargetMaxDiskGB {
		return CheckDetail{
			Name:     name,
			Passed:   false,
			Severity: SeverityCritical,
			Message:  fmt.Sprintf("VM has %d GB disk, exceeds KVM maximum of %d GB", vm.DiskGB, c.TargetMaxDiskGB),
		}
	}

	return CheckDetail{
		Name:     name,
		Passed:   true,
		Severity: SeverityInfo,
		Message:  fmt.Sprintf("%d GB disk within KVM limit of %d GB; VMDK will be converted to QCOW2", vm.DiskGB, c.TargetMaxDiskGB),
	}
}

// checkGuestOS verifies the guest OS is supported.
func (c *Checker) checkGuestOS(vm vcenter.VM) CheckDetail {
	name := "guest_os_support"
	if vm.GuestOS == "" {
		return CheckDetail{
			Name:     name,
			Passed:   false,
			Severity: SeverityWarning,
			Message:  "Unknown guest OS; will use default KVM settings",
		}
	}

	for _, supported := range c.SupportedGuestOS {
		if vm.GuestOS == supported {
			return CheckDetail{
				Name:     name,
				Passed:   true,
				Severity: SeverityInfo,
				Message:  fmt.Sprintf("Guest OS '%s' is fully supported", vm.GuestOS),
			}
		}
	}

	// Check for partial matches (e.g., SLES variants)
	for _, supported := range c.SupportedGuestOS {
		parts := strings.SplitN(supported, "_", 2)
		if len(parts) > 0 && strings.HasPrefix(vm.GuestOS, parts[0]) {
			return CheckDetail{
				Name:     name,
				Passed:   true,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("Guest OS '%s' is similar to supported OS '%s'", vm.GuestOS, supported),
			}
		}
	}

	return CheckDetail{
		Name:     name,
		Passed:   false,
		Severity: SeverityWarning,
		Message:  fmt.Sprintf("Guest OS '%s' is not in the supported list; manual review recommended", vm.GuestOS),
	}
}

// checkNetworks verifies all VM networks have KVM mappings.
func (c *Checker) checkNetworks(vm vcenter.VM) CheckDetail {
	name := "network_mapping"
	if len(vm.Networks) == 0 {
		return CheckDetail{
			Name:     name,
			Passed:   true,
			Severity: SeverityInfo,
			Message:  "VM has no networks configured",
		}
	}

	unmapped := make([]string, 0)
	mapped := make([]string, 0)
	for _, net := range vm.Networks {
		if bridge, ok := c.NetworkMappings[net]; ok {
			mapped = append(mapped, fmt.Sprintf("%s -> %s", net, bridge))
		} else {
			unmapped = append(unmapped, net)
		}
	}

	if len(unmapped) > 0 {
		return CheckDetail{
			Name:     name,
			Passed:   false,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("Networks without KVM mapping: %s; mapped: %s", strings.Join(unmapped, ", "), strings.Join(mapped, ", ")),
		}
	}

	return CheckDetail{
		Name:     name,
		Passed:   true,
		Severity: SeverityInfo,
		Message:  fmt.Sprintf("All networks mapped: %s", strings.Join(mapped, ", ")),
	}
}

// checkVMwareTools verifies VMware Tools status.
func (c *Checker) checkVMwareTools(vm vcenter.VM) CheckDetail {
	name := "vmware_tools"
	switch vm.VMwareTools {
	case "toolsOk":
		return CheckDetail{
			Name:     name,
			Passed:   true,
			Severity: SeverityInfo,
			Message:  "VMware Tools running normally; guest quiescence available during migration",
		}
	case "toolsOld":
		return CheckDetail{
			Name:     name,
			Passed:   true,
			Severity: SeverityWarning,
			Message:  "VMware Tools is outdated; consider upgrading before migration",
		}
	case "toolsNotRunning":
		return CheckDetail{
			Name:     name,
			Passed:   true,
			Severity: SeverityWarning,
			Message:  "VMware Tools not running; migration will use fallback network copy",
		}
	default:
		return CheckDetail{
			Name:     name,
			Passed:   true,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("Unknown VMware Tools status: %s; migration will use fallback", vm.VMwareTools),
		}
	}
}

// checkPowerState reports on VM power state for migration planning.
func (c *Checker) checkPowerState(vm vcenter.VM) CheckDetail {
	name := "power_state"
	switch vm.PowerState {
	case "poweredOn":
		return CheckDetail{
			Name:     name,
			Passed:   true,
			Severity: SeverityInfo,
			Message:  "VM is powered on; supports live migration (vMotion equivalent)",
		}
	case "poweredOff":
		return CheckDetail{
			Name:     name,
			Passed:   true,
			Severity: SeverityInfo,
			Message:  "VM is powered off; cold migration will be used (simpler, faster)",
		}
	case "suspended":
		return CheckDetail{
			Name:     name,
			Passed:   false,
			Severity: SeverityWarning,
			Message:  "VM is suspended; must power on before migration",
		}
	default:
		return CheckDetail{
			Name:     name,
			Passed:   false,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("Unknown power state: %s", vm.PowerState),
		}
	}
}

// checkProvisioningType reports on disk provisioning.
func (c *Checker) checkProvisioningType(vm vcenter.VM) CheckDetail {
	name := "provisioning_type"
	switch vm.ProvisioningType {
	case "thin":
		return CheckDetail{
			Name:     name,
			Passed:   true,
			Severity: SeverityInfo,
			Message:  "Thin-provisioned disk will be converted to QCOW2; space-efficient",
		}
	case "thick":
		return CheckDetail{
			Name:     name,
			Passed:   true,
			Severity: SeverityWarning,
			Message:  "Thick-provisioned disk will be converted to QCOW2; initial import size is full disk",
		}
	case "eagerZeroed":
		return CheckDetail{
			Name:     name,
			Passed:   true,
			Severity: SeverityInfo,
			Message:  "Eager-zeroed thick disk will be converted to QCOW2; best performance after import",
		}
	default:
		return CheckDetail{
			Name:     name,
			Passed:   true,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("Unknown provisioning type: %s", vm.ProvisioningType),
		}
	}
}
