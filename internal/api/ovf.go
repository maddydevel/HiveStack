// Package api provides REST API handlers for VM migration and import operations.
package api

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// OVF namespace constants
const (
	ovfNamespace = "http://schemas.dmtf.org/ovf/envelope/1"
)

type OVFEnvelope struct {
	XMLName                 xml.Name                 `xml:"Envelope"`
	Namespace               string                   `xml:"xmlns,attr,omitempty"`
	References              References               `xml:"References"`
	DiskSection             *DiskSection             `xml:"DiskSection,omitempty"`
	NetworkSection          *NetworkSection          `xml:"NetworkSection,omitempty"`
	VirtualSystem           *VirtualSystem           `xml:"VirtualSystem,omitempty"`
	VirtualSystemCollection *VirtualSystemCollection `xml:"VirtualSystemCollection,omitempty"`
}

// References contains file references in the OVF.
type References struct {
	Files []OVFFile `xml:"File"`
}

// OVFFile represents a file reference in the OVF descriptor.
type OVFFile struct {
	ID          string `xml:"id,attr"`
	Href        string `xml:"href,attr"`
	Size        uint64 `xml:"size,attr"`
	Compression string `xml:"compression,attr,omitempty"`
}

// DiskSection defines virtual disks.
type DiskSection struct {
	Disks []OVFDiskInfo `xml:"Disk"`
}

// OVFDiskInfo describes a virtual disk.
type OVFDiskInfo struct {
	DiskID    string `xml:"diskId,attr"`
	FileRef   string `xml:"fileRef,attr"`
	Capacity  uint64 `xml:"capacity,attr"`
	Allocated uint64 `xml:"populatedSize,attr,omitempty"`
	Format    string `xml:"format,attr,omitempty"`
}

// NetworkSection defines available networks.
type NetworkSection struct {
	Networks []OVFNetwork `xml:"Network"`
}

// OVFNetwork describes a network.
type OVFNetwork struct {
	Name        string `xml:"name,attr"`
	Description string `xml:"Description,omitempty"`
}

// VirtualSystem defines a single VM.
type VirtualSystem struct {
	ID              string                   `xml:"id,attr"`
	Name            string                   `xml:"Name,omitempty"`
	Info            string                   `xml:"Info,omitempty"`
	OperatingSystem *OperatingSystemSection  `xml:"OperatingSystemSection,omitempty"`
	VirtualHardware []VirtualHardwareSection `xml:"VirtualHardwareSection,omitempty"`
	Product         *ProductSection          `xml:"ProductSection,omitempty"`
}

// VirtualSystemCollection defines a collection of VMs.
type VirtualSystemCollection struct {
	ID             string          `xml:"id,attr"`
	Name           string          `xml:"Name,omitempty"`
	Info           string          `xml:"Info,omitempty"`
	VirtualSystems []VirtualSystem `xml:"VirtualSystem,omitempty"`
}

// OperatingSystemSection defines the OS of a VM.
type OperatingSystemSection struct {
	ID          int    `xml:"id,attr"`
	OSVersion   string `xml:"version,attr,omitempty"`
	Description string `xml:"Description,omitempty"`
}

// VirtualHardwareSection defines VM hardware.
type VirtualHardwareSection struct {
	ID     string      `xml:"id,attr,omitempty"`
	Info   string      `xml:"Info,omitempty"`
	System *VSSDSystem `xml:"System,omitempty"`
	Items  []RASDItem  `xml:"Item,omitempty"`
}

// VSSDSystem describes virtual system type.
type VSSDSystem struct {
	VirtualSystemType string `xml:"VirtualSystemType,omitempty"`
	VirtualSystemID   string `xml:"VirtualSystemID,omitempty"`
}

// RASDItem describes a resource allocation setting.
type RASDItem struct {
	ResourceType        string  `xml:"ResourceType,omitempty"`
	InstanceID          string  `xml:"InstanceID,omitempty"`
	ElementName         string  `xml:"ElementName,omitempty"`
	Description         string  `xml:"Description,omitempty"`
	AllocationUnits     string  `xml:"AllocationUnits,omitempty"`
	VirtualQuantity     *uint64 `xml:"VirtualQuantity,omitempty"`
	HostResource        string  `xml:"HostResource,omitempty"`
	Parent              string  `xml:"Parent,omitempty"`
	Address             string  `xml:"Address,omitempty"`
	Connection          string  `xml:"Connection,omitempty"`
	AutomaticAllocation *bool   `xml:"AutomaticAllocation,omitempty"`
}

// ProductSection contains product metadata.
type ProductSection struct {
	Vendor      string `xml:"Vendor,omitempty"`
	Product     string `xml:"Product,omitempty"`
	Version     string `xml:"Version,omitempty"`
	FullVersion string `xml:"FullVersion,omitempty"`
}

// OVFResourceTypes map OVF resource type codes to human-readable names.
var OVFResourceTypes = map[string]string{
	"1":  "Other",
	"2":  "Computer System",
	"3":  "Processor",
	"4":  "Memory",
	"5":  "IDE Controller",
	"6":  "Parallel SCSI HBA",
	"7":  "FC HBA",
	"8":  "iSCSI HBA",
	"9":  "IB HCA",
	"10": "Ethernet Adapter",
	"11": "Other Network Adapter",
	"12": "I/O Slot",
	"13": "I/O Device",
	"14": "Floppy Drive",
	"15": "CD Drive",
	"16": "DVD Drive",
	"17": "Disk Drive",
	"18": "Tape Drive",
	"19": "Storage Extent",
	"20": "Other Storage Device",
	"21": "Serial Port",
	"22": "Parallel Port",
	"23": "USB Controller",
	"24": "Graphics Controller",
}

// ParsedOVFResult contains the simplified VM information extracted from an OVF.
type ParsedOVFResult struct {
	VMs        []ParsedVM   `json:"vms"`
	Disks      []ParsedDisk `json:"disks"`
	Networks   []string     `json:"networks"`
	RawSize    uint64       `json:"raw_size_bytes"`
	OVFVersion string       `json:"ovf_version"`
}

// ParsedVM holds simplified VM info extracted from OVF.
type ParsedVM struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	OperatingSystem   string            `json:"operating_system,omitempty"`
	OSType            string            `json:"os_type,omitempty"`
	CPUs              int               `json:"cpus"`
	MemoryMB          uint64            `json:"memory_mb"`
	DiskRefs          []string          `json:"disk_refs"`
	NetworkRefs       []string          `json:"network_refs"`
	VirtualSystemType string            `json:"virtual_system_type,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
}

// ParsedDisk holds disk info extracted from OVF.
type ParsedDisk struct {
	ID        string `json:"id"`
	FileRef   string `json:"file_ref"`
	SizeBytes uint64 `json:"size_bytes"`
	Format    string `json:"format,omitempty"`
}

// ParseOVF parses an OVF XML document and extracts basic VM information.
// This is a simplified parser that extracts the most important fields.
func ParseOVF(r io.Reader) (*ParsedOVFResult, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read OVF data: %w", err)
	}

	var env OVFEnvelope
	if err := xml.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("parse OVF XML: %w", err)
	}

	result := &ParsedOVFResult{
		VMs:      make([]ParsedVM, 0),
		Disks:    make([]ParsedDisk, 0),
		Networks: make([]string, 0),
	}

	// Build disk map
	diskMap := make(map[string]OVFDiskInfo)
	if env.DiskSection != nil {
		for _, d := range env.DiskSection.Disks {
			diskMap[d.DiskID] = d
			result.Disks = append(result.Disks, ParsedDisk{
				ID:        d.DiskID,
				FileRef:   d.FileRef,
				SizeBytes: d.Capacity,
				Format:    d.Format,
			})
			result.RawSize += d.Capacity
		}
	}

	// Build file map
	fileMap := make(map[string]OVFFile)
	for _, f := range env.References.Files {
		fileMap[f.ID] = f
	}

	// Extract networks
	if env.NetworkSection != nil {
		for _, n := range env.NetworkSection.Networks {
			result.Networks = append(result.Networks, n.Name)
		}
	}

	// Extract VMs from VirtualSystem
	if env.VirtualSystem != nil {
		vm, err := extractVM(env.VirtualSystem, diskMap, fileMap)
		if err != nil {
			return nil, fmt.Errorf("extract VM: %w", err)
		}
		result.VMs = append(result.VMs, vm)
	}

	// Extract VMs from VirtualSystemCollection
	if env.VirtualSystemCollection != nil {
		for _, vs := range env.VirtualSystemCollection.VirtualSystems {
			vm, err := extractVM(&vs, diskMap, fileMap)
			if err != nil {
				return nil, fmt.Errorf("extract VM: %w", err)
			}
			result.VMs = append(result.VMs, vm)
		}
	}

	if len(result.VMs) == 0 {
		return nil, fmt.Errorf("OVF contains no virtual systems")
	}

	return result, nil
}

// extractVM extracts simplified VM info from a VirtualSystem element.
func extractVM(vs *VirtualSystem, diskMap map[string]OVFDiskInfo, fileMap map[string]OVFFile) (ParsedVM, error) {
	vm := ParsedVM{
		ID:     vs.ID,
		Name:   vs.Name,
		Labels: make(map[string]string),
	}

	if vs.OperatingSystem != nil {
		vm.OperatingSystem = vs.OperatingSystem.Description
		vm.OSType = fmt.Sprintf("%d", vs.OperatingSystem.ID)
	}

	// Extract product info
	if vs.Product != nil {
		if vs.Product.Product != "" {
			vm.Labels["product"] = vs.Product.Product
		}
		if vs.Product.Vendor != "" {
			vm.Labels["vendor"] = vs.Product.Vendor
		}
	}

	// Parse virtual hardware
	for _, hw := range vs.VirtualHardware {
		if hw.System != nil && hw.System.VirtualSystemType != "" {
			vm.VirtualSystemType = hw.System.VirtualSystemType
		}

		for _, item := range hw.Items {
			rt := item.ResourceType

			switch rt {
			case "3": // Processor
				if item.VirtualQuantity != nil {
					vm.CPUs = int(*item.VirtualQuantity)
				}
			case "4": // Memory
				if item.VirtualQuantity != nil {
					// Memory unit is determined by AllocationUnits
					// Common formats: "byte * 2^20" = MB, "byte * 2^30" = GB, "byte" = bytes
					unit := parseMemoryUnit(item.AllocationUnits)
					vm.MemoryMB = (*item.VirtualQuantity * unit) / (1024 * 1024)
					if vm.MemoryMB == 0 && *item.VirtualQuantity > 0 {
						vm.MemoryMB = *item.VirtualQuantity // treat as bytes if unknown
					}
				}
			case "10": // Ethernet Adapter
				if item.Connection != "" {
					vm.NetworkRefs = append(vm.NetworkRefs, item.Connection)
				}
			case "17": // Disk Drive
				if item.HostResource != "" {
					// Parse disk reference
					ref := parseDiskRef(item.HostResource)
					if ref != "" {
						vm.DiskRefs = append(vm.DiskRefs, ref)
					}
				}
			}
		}
	}

	// Ensure minimums
	if vm.CPUs == 0 {
		vm.CPUs = 1
	}
	if vm.MemoryMB == 0 {
		vm.MemoryMB = 512
	}
	if vm.Name == "" {
		vm.Name = vm.ID
		if vm.Name == "" {
			vm.Name = "unnamed-vm"
		}
	}

	return vm, nil
}

// parseMemoryUnit parses the AllocationUnits string and returns the multiplier
// for the quantity value.
// Common formats: "byte * 2^20" (MB), "byte * 2^30" (GB), "byte" (bytes)
func parseMemoryUnit(units string) uint64 {
	if units == "" {
		return 1 // default: bytes
	}

	units = strings.ToLower(strings.TrimSpace(units))

	// Try to parse scientific notation: "byte * 2^N"
	if idx := strings.Index(units, "*"); idx > 0 {
		unitPart := strings.TrimSpace(units[:idx])
		expPart := strings.TrimSpace(units[idx+1:])

		// Only handle "byte" base unit
		if unitPart == "byte" || unitPart == "bytes" || unitPart == "octets" {
			exp := parseExponent(expPart)
			if exp > 0 {
				return uint64(1) << exp // 2^exp
			}
		}
	}

	// Check for explicit byte markers
	if strings.Contains(units, "byte") {
		return 1
	}

	return 1
}

// parseExponent extracts the exponent value from strings like "2^20" or "2e6".
func parseExponent(s string) uint64 {
	s = strings.TrimSpace(s)

	// Format: "2^N"
	if idx := strings.Index(s, "^"); idx > 0 {
		base := strings.TrimSpace(s[:idx])
		exp := strings.TrimSpace(s[idx+1:])
		if base == "2" {
			var result uint64
			for _, ch := range exp {
				if ch >= '0' && ch <= '9' {
					result = result*10 + uint64(ch-'0')
				} else {
					return 0
				}
			}
			return result
		}
	}

	// Format: scientific notation like "2e6"
	if idx := strings.IndexAny(s, "eE"); idx > 0 {
		exp := strings.TrimSpace(s[idx+1:])
		var result uint64
		for _, ch := range exp {
			if ch >= '0' && ch <= '9' {
				result = result*10 + uint64(ch-'0')
			} else {
				return 0
			}
		}
		return result
	}

	return 0
}

// parseDiskRef extracts disk ID from a HostResource reference.
func parseDiskRef(ref string) string {
	// OVF host resource refs can be like:
	// "ovf:/disk/vmdisk1" or "vmdisk1"
	ref = strings.TrimPrefix(ref, "ovf:/disk/")
	ref = strings.TrimPrefix(ref, "ovf:disk/")
	return ref
}

// parseLabels parses a comma-separated list of key=value pairs.
func parseLabels(s string) map[string]string {
	result := make(map[string]string)
	if strings.TrimSpace(s) == "" {
		return result
	}
	pairs := strings.Split(s, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if key != "" {
			result[key] = val
		}
	}
	return result
}
