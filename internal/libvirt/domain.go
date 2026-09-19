// Package libvirt wraps host virtualization management (KVM/QEMU/libvirt).
//
// Domain XML generation for KVM guests.
package libvirt

import (
	"encoding/xml"
	"fmt"

	"github.com/google/uuid"
)

// DomainSpec holds the specification for generating KVM domain XML.
type DomainSpec struct {
	Name        string
	MemoryBytes int64
	VCPUs       int
}

// domain is the marshalled representation of a KVM domain.
type domain struct {
	XMLName    xml.Name `xml:"domain"`
	Type       string   `xml:"type,attr"`
	Name       string   `xml:"name"`
	UUID       string   `xml:"uuid"`
	Memory     memEl    `xml:"memory"`
	MemoryCur  memEl    `xml:"currentMemory"`
	VCPU       int      `xml:"vcpu"`
	OS         osEl     `xml:"os"`
	Features   featEl   `xml:"features"`
	Devices    devEl    `xml:"devices"`
}

type memEl struct {
	Unit string `xml:"unit,attr"`
	Size int64  `xml:",chardata"`
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
	Emulator  string    `xml:"emulator"`
	Consoles  []console `xml:"console"`
	Inputs    []input   `xml:"input"`
	Graphics  graphics  `xml:"graphics"`
	Video     video     `xml:"video"`
	Memballoon balloon  `xml:"memballoon"`
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

// GenerateDomainXML generates a KVM domain XML document with the given spec.
// A UUID is auto-generated. The XML includes KVM features, serial console,
// tablet input, and VNC graphics.
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
		Devices: devEl{
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
			Memballoon: balloon{Model: "virtio"},
		},
	}

	out, err := xml.MarshalIndent(d, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal domain XML: %w", err)
	}
	return xml.Header + string(out), nil
}
