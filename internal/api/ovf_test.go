package api

import (
	"context"
	"strings"
	"testing"
)

func TestParseOVF(t *testing.T) {
	ovfXML := `<?xml version="1.0" encoding="UTF-8"?>
<Envelope xmlns="http://schemas.dmtf.org/ovf/envelope/1">
  <References>
    <File ovf:id="file1" ovf:href="disk1.vmdk" ovf:size="1073741824"/>
  </References>
  <DiskSection>
    <Info>Virtual disk information</Info>
    <Disk ovf:diskId="vmdisk1" ovf:fileRef="file1" ovf:capacity="1073741824" ovf:format="http://www.vmware.com/specifications/vmdk.html#sparse"/>
  </DiskSection>
  <NetworkSection>
    <Info>The list of logical networks</Info>
    <Network ovf:name="VM Network">
      <Description>Default network</Description>
    </Network>
  </NetworkSection>
  <VirtualSystem ovf:id="vm1">
    <Info>A virtual machine</Info>
    <Name>TestVM</Name>
    <OperatingSystemSection ovf:id="100">
      <Info>Guest OS</Info>
      <Description>Ubuntu 22.04</Description>
    </OperatingSystemSection>
    <VirtualHardwareSection>
      <Info>Virtual hardware requirements</Info>
      <System>
        <VirtualSystemType>vmx-19</VirtualSystemType>
      </System>
      <Item>
        <ResourceType>3</ResourceType>
        <InstanceID>1</InstanceID>
        <ElementName>Virtual CPU</ElementName>
        <VirtualQuantity>2</VirtualQuantity>
      </Item>
      <Item>
        <ResourceType>4</ResourceType>
        <InstanceID>2</InstanceID>
        <ElementName>Memory</ElementName>
        <AllocationUnits>byte * 2^20</AllocationUnits>
        <VirtualQuantity>4096</VirtualQuantity>
      </Item>
      <Item>
        <ResourceType>10</ResourceType>
        <InstanceID>3</InstanceID>
        <Connection>VM Network</Connection>
        <ElementName>Ethernet Adapter</ElementName>
      </Item>
      <Item>
        <ResourceType>17</ResourceType>
        <InstanceID>4</InstanceID>
        <HostResource>ovf:/disk/vmdisk1</HostResource>
        <ElementName>Hard Disk</ElementName>
      </Item>
    </VirtualHardwareSection>
  </VirtualSystem>
</Envelope>`

	parsed, err := ParseOVF(strings.NewReader(ovfXML))
	if err != nil {
		t.Fatalf("ParseOVF failed: %v", err)
	}

	if len(parsed.VMs) != 1 {
		t.Fatalf("expected 1 VM, got %d", len(parsed.VMs))
	}

	vm := parsed.VMs[0]
	if vm.ID != "vm1" {
		t.Errorf("expected VM ID 'vm1', got '%s'", vm.ID)
	}
	if vm.Name != "TestVM" {
		t.Errorf("expected VM name 'TestVM', got '%s'", vm.Name)
	}
	if vm.CPUs != 2 {
		t.Errorf("expected 2 CPUs, got %d", vm.CPUs)
	}
	if vm.MemoryMB != 4096 {
		t.Errorf("expected 4096 MB memory, got %d", vm.MemoryMB)
	}
	if vm.OperatingSystem != "Ubuntu 22.04" {
		t.Errorf("expected OS 'Ubuntu 22.04', got '%s'", vm.OperatingSystem)
	}
	if len(vm.DiskRefs) != 1 || vm.DiskRefs[0] != "vmdisk1" {
		t.Errorf("expected disk ref 'vmdisk1', got %v", vm.DiskRefs)
	}
	if len(vm.NetworkRefs) != 1 || vm.NetworkRefs[0] != "VM Network" {
		t.Errorf("expected network ref 'VM Network', got %v", vm.NetworkRefs)
	}
	if len(parsed.Disks) != 1 {
		t.Errorf("expected 1 disk, got %d", len(parsed.Disks))
	}
	if len(parsed.Networks) != 1 || parsed.Networks[0] != "VM Network" {
		t.Errorf("expected network 'VM Network', got %v", parsed.Networks)
	}
}

func TestParseOVFCollection(t *testing.T) {
	ovfXML := `<?xml version="1.0" encoding="UTF-8"?>
<Envelope xmlns="http://schemas.dmtf.org/ovf/envelope/1">
  <References>
    <File ovf:id="file1" ovf:href="disk1.vmdk" ovf:size="1073741824"/>
    <File ovf:id="file2" ovf:href="disk2.vmdk" ovf:size="2147483648"/>
  </References>
  <DiskSection>
    <Disk ovf:diskId="vmdisk1" ovf:fileRef="file1" ovf:capacity="1073741824"/>
    <Disk ovf:diskId="vmdisk2" ovf:fileRef="file2" ovf:capacity="2147483648"/>
  </DiskSection>
  <VirtualSystemCollection ovf:id="multi-vm">
    <Info>Multiple VMs</Info>
    <VirtualSystem ovf:id="vm1">
      <Name>WebServer</Name>
      <OperatingSystemSection ovf:id="100">
        <Description>Ubuntu 22.04</Description>
      </OperatingSystemSection>
      <VirtualHardwareSection>
        <System>
          <VirtualSystemType>vmx-19</VirtualSystemType>
        </System>
        <Item>
          <ResourceType>3</ResourceType>
          <VirtualQuantity>2</VirtualQuantity>
        </Item>
        <Item>
          <ResourceType>4</ResourceType>
          <VirtualQuantity>2097152</VirtualQuantity>
        </Item>
      </VirtualHardwareSection>
    </VirtualSystem>
    <VirtualSystem ovf:id="vm2">
      <Name>DBServer</Name>
      <OperatingSystemSection ovf:id="100">
        <Description>CentOS 8</Description>
      </OperatingSystemSection>
      <VirtualHardwareSection>
        <System>
          <VirtualSystemType>vmx-19</VirtualSystemType>
        </System>
        <Item>
          <ResourceType>3</ResourceType>
          <VirtualQuantity>4</VirtualQuantity>
        </Item>
        <Item>
          <ResourceType>4</ResourceType>
          <VirtualQuantity>8388608</VirtualQuantity>
        </Item>
      </VirtualHardwareSection>
    </VirtualSystem>
  </VirtualSystemCollection>
</Envelope>`

	parsed, err := ParseOVF(strings.NewReader(ovfXML))
	if err != nil {
		t.Fatalf("ParseOVF failed: %v", err)
	}

	if len(parsed.VMs) != 2 {
		t.Fatalf("expected 2 VMs, got %d", len(parsed.VMs))
	}

	if parsed.VMs[0].Name != "WebServer" {
		t.Errorf("expected first VM 'WebServer', got '%s'", parsed.VMs[0].Name)
	}
	if parsed.VMs[1].Name != "DBServer" {
		t.Errorf("expected second VM 'DBServer', got '%s'", parsed.VMs[1].Name)
	}
	if parsed.VMs[1].CPUs != 4 {
		t.Errorf("expected DBServer to have 4 CPUs, got %d", parsed.VMs[1].CPUs)
	}
}

func TestParseOVFInvalid(t *testing.T) {
	tests := []struct {
		name string
		xml  string
	}{
		{
			name: "empty",
			xml:  "",
		},
		{
			name: "invalid xml",
			xml:  "<not-valid>",
		},
		{
			name: "no virtual systems",
			xml:  `<Envelope xmlns="http://schemas.dmtf.org/ovf/envelope/1"><References></References></Envelope>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseOVF(strings.NewReader(tt.xml))
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestMigrationService(t *testing.T) {
	hosts := []string{"host1", "host2", "host3"}
	svc := NewMigrationService(hosts)

	// Test CreateImportJob
	job := svc.CreateImportJob("test.ovf", "host1", map[string]string{"env": "prod"})
	if job.ID == "" {
		t.Fatal("expected non-empty job ID")
	}
	if job.State != ImportJobPending {
		t.Errorf("expected state pending, got %s", job.State)
	}

	// Test GetImportJob
	got, ok := svc.GetImportJob(job.ID)
	if !ok {
		t.Fatal("expected to find job")
	}
	if got.ID != job.ID {
		t.Errorf("expected job ID %s, got %s", job.ID, got.ID)
	}

	// Test ListImportJobs
	jobs := svc.ListImportJobs()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}

	// Test UpdateJobState
	if err := svc.UpdateJobState(job.ID, ImportJobParsing, 50, ""); err != nil {
		t.Fatalf("UpdateJobState failed: %v", err)
	}
	got, _ = svc.GetImportJob(job.ID)
	if got.State != ImportJobParsing {
		t.Errorf("expected state parsing, got %s", got.State)
	}
	if got.Progress != 50 {
		t.Errorf("expected progress 50, got %d", got.Progress)
	}

	// Test CancelImportJob
	if err := svc.CancelImportJob(job.ID); err != nil {
		t.Fatalf("CancelImportJob failed: %v", err)
	}
	got, _ = svc.GetImportJob(job.ID)
	if got.State != ImportJobCancelled {
		t.Errorf("expected state cancelled, got %s", got.State)
	}
	if got.CompletedAt == nil {
		t.Error("expected CompletedAt to be set after cancellation")
	}

	// Test DeleteImportJob
	if err := svc.DeleteImportJob(job.ID); err != nil {
		t.Fatalf("DeleteImportJob failed: %v", err)
	}
	_, ok = svc.GetImportJob(job.ID)
	if ok {
		t.Error("expected job to be deleted")
	}
}

func TestPreflightChecks(t *testing.T) {
	hosts := []string{"host1", "host2"}
	svc := NewMigrationService(hosts)

	parsed := &ParsedOVFResult{
		VMs: []ParsedVM{
			{ID: "vm1", Name: "TestVM", CPUs: 2, MemoryMB: 4096},
		},
		Disks: []ParsedDisk{
			{ID: "disk1", FileRef: "file1", SizeBytes: 1073741824},
		},
		Networks: []string{"VM Network"},
	}

	// Test with valid target host
	job := svc.CreateImportJob("test.ovf", "host1", nil)
	preflight := svc.RunPreflightChecks(context.Background(), job, parsed)
	if !preflight.Compatible {
		t.Error("expected compatible preflight with valid host")
	}
	if len(preflight.Checks) == 0 {
		t.Error("expected preflight checks to be populated")
	}

	// Test with invalid target host
	job2 := svc.CreateImportJob("test.ovf", "nonexistent", nil)
	preflight2 := svc.RunPreflightChecks(context.Background(), job2, parsed)
	if preflight2.Compatible {
		t.Error("expected incompatible preflight with invalid host")
	}
}

func TestParseLabels(t *testing.T) {
	tests := []struct {
		input    string
		expected map[string]string
	}{
		{"", map[string]string{}},
		{"env=prod", map[string]string{"env": "prod"}},
		{"env=prod,team=ops", map[string]string{"env": "prod", "team": "ops"}},
		{" env = prod , team = ops ", map[string]string{"env": "prod", "team": "ops"}},
	}

	for _, tt := range tests {
		result := parseLabels(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("parseLabels(%q): expected %d labels, got %d", tt.input, len(tt.expected), len(result))
		}
		for k, v := range tt.expected {
			if result[k] != v {
				t.Errorf("parseLabels(%q): expected %s=%s, got %s", tt.input, k, v, result[k])
			}
		}
	}
}
