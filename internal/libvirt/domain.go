// Package libvirt wraps host virtualization management (KVM/QEMU/libvirt).
//
// Domain XML generation for KVM guests with HANA guardrail enforcement.
package libvirt

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// DomainSpec holds the specification for generating KVM domain XML.
type DomainSpec struct {
	Name                   string
	MemoryBytes            int64
	VCPUs                  int
	NUMAPolicy             string            // "centered", "bind", or "" for non-HANA
	HugepagesEnabled       bool              // true for HANA
	CPUPinning             map[string]string // vcpu -> host CPU mapping
	BallooningAllowed      bool              // HANA: always false
	DedicatedCPU           bool              // HANA: true for dedicated allocation
}

// domain is the marshalled representation of a KVM domain.
type domain struct {
	XMLName       xml.Name    `xml:"domain"`
	Type          string      `xml:"type,attr"`
	Name          string      `xml:"name"`
	UUID          string      `xml:"uuid"`
	Memory        memEl       `xml:"memory"`
	MemoryCur     memEl       `xml:"currentMemory"`
	VCPU          int         `xml:"vcpu"`
	CPU           *cpuEl      `xml:"cpu,omitempty"`
	NUMATuning    *numaTuning `xml:"numatune,omitempty"`
	VCPUPlacement *vcpuPin    `xml:"cputune,omitempty"`
	OS            osEl        `xml:"os"`
	Features      featEl      `xml:"features"`
	Devices       devEl       `xml:"devices"`
	OnPoweroff    string      `xml:"on_poweroff"`
	OnReboot      string      `xml:"on_reboot"`
	OnCrash       string      `xml:"on_crash"`
}

type memEl struct {
	Unit string `xml:"unit,attr"`
	Size int64  `xml:",chardata"`
}

type cpuEl struct {
	Mode    string      `xml:"mode,attr"`
	Match   string      `xml:"match,attr"`
	Check   string      `xml:"check,attr,omitempty"`
	Topology topologyEl `xml:"topology,omitempty"`
}

type topologyEl struct {
	Sockets int `xml:"sockets,attr"`
	Cores   int `xml:"cores,attr"`
	Threads int `xml:"threads,attr"`
}

type numaTuning struct {
	Memory numaMemory `xml:"memory"`
}

type numaMemory struct {
	Mode    string `xml:"mode,attr"`
	Nodeset string `xml:"nodeset,attr"`
}

type vcpuPin struct {
	VCPUPlacements []vcpuPlacement `xml:"vcpupin"`
}

type vcpuPlacement struct {
	VCPU   int `xml:"vcpu,attr"`
	CPUSet string `xml:"cpuset,attr"`
}

type osEl struct {
	Type string `xml:"type"`
	Boot bootEl `xml:"boot"`
}

type bootEl struct {
	Dev string `xml:"dev,attr"`
}

type featEl struct {
	ACPI string `xml:"acpi"`
	APIC string `xml:"apic"`
}

type devEl struct {
	Emulator   string    `xml:"emulator"`
	Consoles   []console `xml:"console"`
	Inputs     []input   `xml:"input"`
	Graphics   graphics  `xml:"graphics"`
	Video      video     `xml:"video"`
	Memballoon *balloon  `xml:"memballoon,omitempty"`
	Disks      []disk    `xml:"disk"`
	Interfaces []iface   `xml:"interface"`
}

type console struct {
	Type   string `xml:"type,attr"`
	Target target `xml:"target"`
	Port   int    `xml:"port,attr"`
}

type target struct {
	Type string `xml:"type,attr"`
	Port int    `xml:"port,attr"`
}

type input struct {
	Type string `xml:"type,attr"`
	Bus  string `xml:"bus,attr"`
}

type graphics struct {
	Type     string `xml:"type,attr"`
	Port     int    `xml:"port,attr"`
	Autoport string `xml:"autoport,attr"`
	Listen   string `xml:"listen,attr"`
}

type video struct {
	Model model `xml:"model"`
}

type model struct {
	Type  string `xml:"type,attr"`
	VRAM  int    `xml:"vram,attr"`
	Heads int    `xml:"heads,attr"`
}

type balloon struct {
	Model string `xml:"model,attr"`
}

type disk struct {
	Type   string     `xml:"type,attr"`
	Device string     `xml:"device,attr"`
	Driver diskDriver `xml:"driver"`
	Source diskSource `xml:"source"`
	Target diskTarget `xml:"target"`
}

type diskDriver struct {
	Name string `xml:"name,attr"`
	Type string `xml:"type,attr"`
}

type diskSource struct {
	File string `xml:"file,attr"`
}

type diskTarget struct {
	Dev string `xml:"dev,attr"`
	Bus string `xml:"bus,attr"`
}

type iface struct {
	Type   string     `xml:"type,attr"`
	MAC    ifaceMAC   `xml:"mac"`
	Source ifaceSource `xml:"source"`
	Model  ifaceModel  `xml:"model"`
}

type ifaceMAC struct {
	Address string `xml:"address,attr"`
}

type ifaceSource struct {
	Bridge string `xml:"bridge,attr"`
}

type ifaceModel struct {
	Type string `xml:"type,attr"`
}

// GenerateDomainXML generates a KVM domain XML document with the given spec.
// A UUID is auto-generated. The XML includes KVM features, serial console,
// tablet input, and VNC graphics. When HANA guardrails are enabled (via
// DomainSpec fields), it adds NUMA tuning, hugepages backing, CPU pinning,
// and disables memory ballooning.
func GenerateDomainXML(spec DomainSpec) (string, error) {
	if spec.Name == "" {
		return "", fmt.Errorf("domain name is required")
	}
	if spec.MemoryBytes <= 0 {
		return "", fmt.Errorf("memory must be positive, got %d", spec.MemoryBytes)
	}
	if spec.VCPUs <= 0 {
		return "", fmt.Errorf("vcpus must be positive, got %d", spec.VCPUs)
	}

	d := domain{
		Type:      "kvm",
		Name:      spec.Name,
		UUID:      uuid.New().String(),
		Memory:    memEl{Unit: "bytes", Size: spec.MemoryBytes},
		MemoryCur: memEl{Unit: "bytes", Size: spec.MemoryBytes},
		VCPU:      spec.VCPUs,
		OS:        osEl{Type: "hvm", Boot: bootEl{Dev: "hd"}},
		Features:  featEl{ACPI: "", APIC: ""},
		OnPoweroff: "destroy",
		OnReboot:   "restart",
		OnCrash:    "restart",
	}

	// CPU topology and mode
	d.CPU = &cpuEl{
		Mode:  "host-passthrough",
		Match: "exact",
		Check: "full",
		Topology: topologyEl{
			Sockets: 1,
			Cores:   spec.VCPUs,
			Threads: 1,
		},
	}

	// HANA: hugepages backing
	if spec.HugepagesEnabled {
		// Add hugepages via memory backing element
		// We use a separate MemoryBacking struct
		d.Memory = memEl{Unit: "bytes", Size: spec.MemoryBytes}
	}

	// HANA: NUMA tuning
	if spec.NUMAPolicy == "centered" || spec.NUMAPolicy == "bind" {
		d.NUMATuning = &numaTuning{
			Memory: numaMemory{
				Mode:    "strict",
				Nodeset: "0",
			},
		}
	}

	// HANA: CPU pinning
	if len(spec.CPUPinning) > 0 {
		placements := make([]vcpuPlacement, 0, len(spec.CPUPinning))
		for vcpu, cpuset := range spec.CPUPinning {
			vcpuNum := 0
			fmt.Sscanf(vcpu, "vcpu%d", &vcpuNum)
			placements = append(placements, vcpuPlacement{
				VCPU:   vcpuNum,
				CPUSet: cpuset,
			})
		}
		d.VCPUPlacement = &vcpuPin{VCPUPlacements: placements}
	}

	// Devices
	devices := devEl{
		Emulator: "/usr/bin/qemu-system-x86_64",
		Consoles: []console{
			{Type: "pty", Port: 0, Target: target{Type: "serial", Port: 0}},
		},
		Inputs: []input{
			{Type: "tablet", Bus: "usb"},
		},
		Graphics: graphics{
			Type:     "vnc",
			Port:     -1,
			Autoport: "yes",
			Listen:   "127.0.0.1",
		},
		Video: video{
			Model: model{Type: "qxl", VRAM: 65536, Heads: 1},
		},
	}

	// HANA: no ballooning — omit memballoon device entirely
	if spec.BallooningAllowed {
		devices.Memballoon = &balloon{Model: "virtio"}
	}

	d.Devices = devices

	out, err := xml.MarshalIndent(d, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal domain XML: %w", err)
	}

	result := xml.Header + string(out)

	// Post-process: insert <memoryBacking><hugepages/></memoryBacking> if enabled
	if spec.HugepagesEnabled {
		hugepagesBlock := `
  <memoryBacking>
    <hugepages>
      <page size="2048" unit="KiB"/>
    </hugepages>
  </memoryBacking>`
		// Insert after the opening <domain> tag line
		idx := strings.Index(result, "<domain")
		if idx != -1 {
			// Find the end of the opening tag
			tagEnd := strings.Index(result[idx:], ">")
			if tagEnd != -1 {
				insertAt := idx + tagEnd + 1
				result = result[:insertAt] + hugepagesBlock + result[insertAt:]
			}
		}
	}

	return result, nil
}
