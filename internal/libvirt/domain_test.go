package libvirt

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestGenerateDomainXML(t *testing.T) {
	tests := []struct {
		name    string
		spec    DomainSpec
		wantErr bool
	}{
		{
			name:    "valid spec",
			spec:    DomainSpec{Name: "test-vm", MemoryBytes: 4 * 1024 * 1024 * 1024, VCPUs: 2},
			wantErr: false,
		},
		{
			name:    "empty name",
			spec:    DomainSpec{Name: "", MemoryBytes: 4 * 1024 * 1024 * 1024, VCPUs: 2},
			wantErr: true,
		},
		{
			name:    "zero memory",
			spec:    DomainSpec{Name: "test-vm", MemoryBytes: 0, VCPUs: 2},
			wantErr: true,
		},
		{
			name:    "negative memory",
			spec:    DomainSpec{Name: "test-vm", MemoryBytes: -1, VCPUs: 2},
			wantErr: true,
		},
		{
			name:    "zero vcpus",
			spec:    DomainSpec{Name: "test-vm", MemoryBytes: 4 * 1024 * 1024 * 1024, VCPUs: 0},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateDomainXML(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateDomainXML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				// Verify XML is well-formed
				var d domain
				if err := xml.Unmarshal([]byte(got), &d); err != nil {
					t.Errorf("GenerateDomainXML() output is not valid XML: %v", err)
				}
				// Verify required fields
				if d.Type != "kvm" {
					t.Errorf("domain type = %q, want kvm", d.Type)
				}
				if d.Name != tt.spec.Name {
					t.Errorf("domain name = %q, want %q", d.Name, tt.spec.Name)
				}
				if d.Memory.Size != tt.spec.MemoryBytes {
					t.Errorf("domain memory = %d, want %d", d.Memory.Size, tt.spec.MemoryBytes)
				}
				if d.VCPU != tt.spec.VCPUs {
					t.Errorf("domain vcpu = %d, want %d", d.VCPU, tt.spec.VCPUs)
				}
				if d.OS.Type != "hvm" {
					t.Errorf("os type = %q, want hvm", d.OS.Type)
				}
				if d.Devices.Graphics.Type != "vnc" {
					t.Errorf("graphics type = %q, want vnc", d.Devices.Graphics.Type)
				}
				if d.Devices.Graphics.Listen != "127.0.0.1" {
					t.Errorf("graphics listen = %q, want 127.0.0.1", d.Devices.Graphics.Listen)
				}
				if d.Devices.Emulator == "" {
					t.Error("emulator path should not be empty")
				}
			}
		})
	}
}

func TestGenerateDomainXML_XMLEscaping(t *testing.T) {
	spec := DomainSpec{Name: "vm-with-<special>&\"chars\"", MemoryBytes: 1024 * 1024 * 1024, VCPUs: 1}
	out, err := GenerateDomainXML(spec)
	if err != nil {
		t.Fatalf("GenerateDomainXML() failed: %v", err)
	}
	if !strings.Contains(out, "&lt;special&gt;") {
		t.Errorf("XML output should escape special chars, got: %s", out)
	}
}

func TestGenerateDomainXML_UUIDUniqueness(t *testing.T) {
	spec := DomainSpec{Name: "test-vm", MemoryBytes: 1024 * 1024 * 1024, VCPUs: 1}

	uuids := make(map[string]bool)
	for i := 0; i < 10; i++ {
		out, err := GenerateDomainXML(spec)
		if err != nil {
			t.Fatalf("GenerateDomainXML() failed: %v", err)
		}
		var d domain
		xml.Unmarshal([]byte(out), &d)
		if uuids[d.UUID] {
			t.Errorf("duplicate UUID generated: %s", d.UUID)
		}
		uuids[d.UUID] = true
	}
}

func TestGenerateDomainXML_Devices(t *testing.T) {
	spec := DomainSpec{Name: "full-vm", MemoryBytes: 8 * 1024 * 1024 * 1024, VCPUs: 4}
	out, err := GenerateDomainXML(spec)
	if err != nil {
		t.Fatalf("GenerateDomainXML() failed: %v", err)
	}
	var d domain
	if err := xml.Unmarshal([]byte(out), &d); err != nil {
		t.Fatalf("Failed to parse XML: %v", err)
	}

	if len(d.Devices.Consoles) == 0 {
		t.Error("console device missing")
	}
	if len(d.Devices.Inputs) == 0 {
		t.Error("input device missing")
	}
	if d.Devices.Graphics.Type != "vnc" {
		t.Errorf("graphics type = %q, want vnc", d.Devices.Graphics.Type)
	}
	if d.Devices.Video.Model.Type != "qxl" {
		t.Errorf("video model = %q, want qxl", d.Devices.Video.Model.Type)
	}
	if d.Devices.Memballoon == nil {
		// Ballooning disabled — expected for HANA compliance
	} else if d.Devices.Memballoon.Model != "virtio" {
		t.Errorf("memballoon model = %q, want virtio", d.Devices.Memballoon.Model)
	}
}
