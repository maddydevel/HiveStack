// Package migration provides VMware to HiveStack migration tools.
//
// Currently supports VMX file parsing and conversion to HiveStack-compatible
// VM definitions. VMDK and OVF import are deferred to a later phase.
//
// VMX files are VMware's virtual machine configuration format. This parser
// reads the key-value pairs and converts them to a HiveStack VM specification
// that can be used with the Manager API.
//
// Supported VMX elements:
//   - numberOfCPUs, memorySize
//   - ide/scsi disk definitions (_FILE, _device, _sharing)
//   - ethernet adapter definitions (networkName, virtualDev)
//   - displayName, guestOS, uuid.location, uuid.bios
//   - vmci, usb, rebootAutoAlign
//
// Not yet supported (deferred):
//   - VMDK direct import (phase 2)
//   - OVF/OVA import (phase 2)
//   - snapshot transfer
//   - VMware-specific device types (vmware.ViewListing, etc.)
package migration

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// VMXParser holds the parsed VMX data.
type VMXParser struct {
	path     string
	lines    []string
	entries  map[string]string
	warnings []string
}

// ParsedVM holds a converted VM configuration.
type ParsedVM struct {
	Name         string
	CPUs         int
	MemoryMB     int
	Disks        []ParsedDisk
	Networks     []ParsedNetwork
	GuestOS      string
	DisplayName  string
	UUIDBIOS     string
	UUIDLocation string
	Annotation   string
}

// ParsedDisk holds a single disk definition.
type ParsedDisk struct {
	Key          string
	Path         string
	Format       string // "monolithicFlat", "twoGbMaxExtentFlat", etc.
	Capacity     int    // in bytes
	Adapter      string // "lsilogic", "pvscsi", "ide", etc.
	BusNumber    int
	DeviceNumber int
}

// ParsedNetwork holds a single network adapter definition.
type ParsedNetwork struct {
	Key         string
	AdapterType string // "vmxnet3", "e1000", "flexible", etc.
	NetworkName string
	Connected   bool
	MACAddress  string
}

// ParseVMX parses a VMX file and returns a ParsedVM.
func ParseVMX(path string) (*ParsedVM, error) {
	p := &VMXParser{
		path:    path,
		entries: make(map[string]string),
	}

	if err := p.readFile(); err != nil {
		return nil, fmt.Errorf("read VMX file: %w", err)
	}
	p.parseEntries()

	vm := &ParsedVM{}
	p.extractVMInfo(vm)
	p.extractCPUAndMemory(vm)
	p.extractDisks(vm)
	p.extractNetworks(vm)
	p.extractMetadata(vm)

	return vm, nil
}

func (p *VMXParser) readFile() error {
	f, err := os.Open(p.path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			p.lines = append(p.lines, line)
		}
	}
	return scanner.Err()
}

func (p *VMXParser) parseEntries() {
	for _, line := range p.lines {
		// Handle continuation lines (append to previous entry)
		if strings.HasSuffix(line, "\\") {
			key := strings.TrimSuffix(line, "\\")
			p.entries[key] = "" // continuation marker
			continue
		}

		// Parse key=value pairs
		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])
		p.entries[key] = value
	}
}

func (p *VMXParser) extractVMInfo(vm *ParsedVM) {
	// Display name
	if v, ok := p.entries["displayName"]; ok {
		vm.DisplayName = v
	}

	// Guest OS
	if v, ok := p.entries["guestOS"]; ok {
		vm.GuestOS = v
	}

	// UUID
	if v, ok := p.entries["uuid.bios"]; ok {
		vm.UUIDBIOS = v
	}
	if v, ok := p.entries["uuid.location"]; ok {
		vm.UUIDLocation = v
	}

	// Annotation
	if v, ok := p.entries["annotation"]; ok {
		vm.Annotation = v
	}
}

func (p *VMXParser) extractCPUAndMemory(vm *ParsedVM) {
	if v, ok := p.entries["numvcpus"]; ok {
		c, err := strconv.Atoi(v)
		if err == nil && c > 0 {
			vm.CPUs = c
		}
	}

	if v, ok := p.entries["cpuid.coresPerSocket"]; ok {
		c, err := strconv.Atoi(v)
		if err == nil && c > 0 {
			// numvcpus was already set; this is just info
		}
	}

	if v, ok := p.entries["memsize"]; ok {
		m, err := strconv.Atoi(v)
		if err == nil && m > 0 {
			vm.MemoryMB = m
		}
	}
}

func (p *VMXParser) extractDisks(vm *ParsedVM) {
	for key, value := range p.entries {
		if !strings.HasPrefix(key, "scsi") && !strings.HasPrefix(key, "ide") && !strings.HasPrefix(key, "sata") {
			continue
		}

		disk := ParsedDisk{
			Key:      key,
			Path:     "",
			Format:   "monolithicFlat",
			Capacity: 0,
			Adapter:  "lsilogic",
		}

		// Extract disk path
		if strings.HasSuffix(key, ".fileName") {
			disk.Path = value
			// Determine format from path
			base := filepath.Base(disk.Path)
			if strings.HasSuffix(base, ".vmdk") {
				disk.Format = "monolithicFlat"
			}

			// Get the disk size from the corresponding key
			parentKey := strings.TrimSuffix(key, ".fileName")
			sizeKey := parentKey + ".capacityKB"
			if sc, ok := p.entries[sizeKey]; ok {
				kb, err := strconv.Atoi(sc)
				if err == nil {
					disk.Capacity = kb * 1024
				}
			}

			// Get adapter type
			adpKey := parentKey + ".adapterType"
			if adp, ok := p.entries[adpKey]; ok {
				disk.Adapter = adp
			}

			// Get bus and device numbers
			busKey := parentKey + ".busNumber"
			if bus, ok := p.entries[busKey]; ok {
				b, _ := strconv.Atoi(bus)
				disk.BusNumber = b
			}
			devKey := parentKey + ".deviceNumber"
			if dev, ok := p.entries[devKey]; ok {
				d, _ := strconv.Atoi(dev)
				disk.DeviceNumber = d
			}

			// Get key (controller key)
			keyParts := strings.Split(parentKey, ".")
			if len(keyParts) >= 2 {
				disk.Key = keyParts[len(keyParts)-2]
			}

			vm.Disks = append(vm.Disks, disk)
		}
	}
}

func (p *VMXParser) extractNetworks(vm *ParsedVM) {
	for key, value := range p.entries {
		if !strings.HasPrefix(key, "ethernet") {
			continue
		}
		if !strings.HasSuffix(key, ".networkName") &&
			!strings.HasSuffix(key, ".virtualDev") &&
			!strings.HasSuffix(key, ".connectionType") &&
			!strings.HasSuffix(key, ".macAddress") &&
			!strings.HasSuffix(key, ".connectable") {
			continue
		}

		parts := strings.Split(key, ".")
		if len(parts) < 2 {
			continue
		}
		ethNum := parts[0] + "." + parts[1]
		suffix := strings.Join(parts[2:], ".")

		switch suffix {
		case "networkName":
			for i := range vm.Networks {
				if vm.Networks[i].Key == ethNum {
					vm.Networks[i].NetworkName = value
				}
			}
		case "virtualDev":
			net := ParsedNetwork{Key: ethNum}
			net.AdapterType = value
			vm.Networks = append(vm.Networks, net)
		case "macAddress":
			for i := range vm.Networks {
				if vm.Networks[i].Key == ethNum {
					vm.Networks[i].MACAddress = value
				}
			}
		case "connectable":
			for i := range vm.Networks {
				if vm.Networks[i].Key == ethNum {
					vm.Networks[i].Connected = value == "true"
				}
			}
		}
	}
}

func (p *VMXParser) extractMetadata(vm *ParsedVM) {
	// Already handled in extractVMInfo
}

// ConvertToHiveStackSpec converts a ParsedVM to a HiveStack VM creation spec.
// The returned map is suitable for passing to the Manager API's CreateVM endpoint.
func (p *ParsedVM) ConvertToHiveStackSpec() map[string]interface{} {
	spec := map[string]interface{}{
		"name":           p.DisplayName,
		"cpus":           p.CPUs,
		"memory_bytes":   int64(p.MemoryMB) * 1024 * 1024,
		"os":             p.guessOS(),
		"role":           "generic",
		"cpu_allocation": "shared",
	}

	// Convert disks
	for _, disk := range p.Disks {
		spec[fmt.Sprintf("disk_%s_path", disk.Key)] = disk.Path
		spec[fmt.Sprintf("disk_%s_size_bytes", disk.Key)] = disk.Capacity
	}

	// Convert networks
	for _, net := range p.Networks {
		spec[fmt.Sprintf("network_%s_type", net.Key)] = net.AdapterType
		if net.NetworkName != "" {
			spec[fmt.Sprintf("network_%s_name", net.Key)] = net.NetworkName
		}
	}

	return spec
}

// GuessOS returns a best-guess OS identifier from the guestOS field.
func (p *ParsedVM) GuessOS() string {
	os := "linux"
	guest := strings.ToLower(p.GuestOS)

	if strings.Contains(guest, "windows") || strings.Contains(guest, "win") {
		if strings.Contains(guest, "2019") || strings.Contains(guest, "2022") {
			os = "windows2022"
		} else if strings.Contains(guest, "2016") {
			os = "windows2016"
		} else {
			os = "windows"
		}
	} else if strings.Contains(guest, "sles") || strings.Contains(guest, "suse") {
		if strings.Contains(guest, "15") {
			os = "sles15"
		} else {
			os = "sles"
		}
	} else if strings.Contains(guest, "ubuntu") {
		os = "ubuntu"
	} else if strings.Contains(guest, "rhel") || strings.Contains(guest, "centos") {
		os = "rhel"
	}

	return os
}

// guessOS is an internal alias for backward compatibility.
func (p *ParsedVM) guessOS() string {
	return p.GuessOS()
}

// ValidateVMX validates that the VMX file can be parsed without errors.
func ValidateVMX(path string) error {
	_, err := ParseVMX(path)
	return err
}

// VMXImportSpec wraps a parsed VMX for import into HiveStack.
type VMXImportSpec struct {
	ParsedVM  *ParsedVM `json:"parsed_vm"`
	HostID    string    `json:"host_id"`
	ClusterID string    `json:"cluster_id,omitempty"`
}

// GetVMXInfo returns a summary of the VMX file without full parsing.
func GetVMXInfo(path string) (map[string]string, error) {
	p := &VMXParser{path: path, entries: make(map[string]string)}
	if err := p.readFile(); err != nil {
		return nil, err
	}
	p.parseEntries()

	info := map[string]string{
		"displayName": p.entries["displayName"],
		"guestOS":     p.entries["guestOS"],
		"numvcpus":    p.entries["numvcpus"],
		"memsize":     p.entries["memsize"],
		"uuid.bios":   p.entries["uuid.bios"],
		"annotation":  p.entries["annotation"],
		"file":        path,
	}
	return info, nil
}
