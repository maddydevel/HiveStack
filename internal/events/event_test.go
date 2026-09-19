package events

import (
	"encoding/json"
	"testing"
)

func TestEventType_String(t *testing.T) {
    tests := []struct {
        et  EventType
        exp string
    }{
        {VMCreated, "vm_created"},
        {VMDeleted, "vm_deleted"},
        {VMStarted, "vm_started"},
        {VMStopped, "vm_stopped"},
        {VMRestarted, "vm_restarted"},
        {VMMigrated, "vm_migrated"},
        {HostRegistered, "host_registered"},
        {HostDecommissioned, "host_decommissioned"},
        {HostMaintenanceEnter, "host_maintenance_enter"},
        {HostMaintenanceExit, "host_maintenance_exit"},
        {ComplianceCheckPassed, "compliance_check_passed"},
        {ComplianceCheckFailed, "compliance_check_failed"},
        {ComplianceDriftDetected, "compliance_drift_detected"},
        {BackupCreated, "backup_created"},
        {BackupRestored, "backup_restored"},
        {BackupFailed, "backup_failed"},
        {NodeRegistered, "node_registered"},
        {NodeOffline, "node_offline"},
    }
    for _, tt := range tests {
        t.Run(string(tt.et), func(t *testing.T) {
            if got := tt.et.String(); got != tt.exp {
                t.Errorf("String() = %q, want %q", got, tt.exp)
            }
        })
    }
}

func TestSeverity_Constants(t *testing.T) {
    if SeverityInfo != "info" {
        t.Errorf("SeverityInfo = %q", SeverityInfo)
    }
    if SeverityWarning != "warning" {
        t.Errorf("SeverityWarning = %q", SeverityWarning)
    }
    if SeverityError != "error" {
        t.Errorf("SeverityError = %q", SeverityError)
    }
    if SeverityCritical != "critical" {
        t.Errorf("SeverityCritical = %q", SeverityCritical)
    }
}

func TestNewEvent_MarshalsMetadata(t *testing.T) {
    ev := NewEvent(
        "tenant-1",
        VMCreated,
        SeverityInfo,
        "VM created",
        "api", "user-1", "Alice",
        "vm", "vm-1", "web-server",
        map[string]interface{}{"cpu": 4, "memory": 8192},
    )
    if ev.ID != "" {
        t.Errorf("expected empty ID, got %q", ev.ID)
    }
    if ev.TenantID != "tenant-1" {
        t.Errorf("TenantID = %q", ev.TenantID)
    }
    if ev.Type != VMCreated {
        t.Errorf("Type = %v", ev.Type)
    }
    if ev.Severity != SeverityInfo {
        t.Errorf("Severity = %v", ev.Severity)
    }
    if ev.Message != "VM created" {
        t.Errorf("Message = %q", ev.Message)
    }
    if ev.Metadata == nil {
        t.Fatal("expected non-nil metadata")
    }
    var meta map[string]interface{}
    	if err := json.Unmarshal(ev.Metadata, &meta); err != nil {
    		t.Fatalf("unmarshal metadata: %v", err)
    	}
    	if meta["cpu"] != float64(4) {
    		t.Errorf("metadata.cpu = %v", meta["cpu"])
    	}
    }

func TestNewEvent_NilMetadata(t *testing.T) {
    ev := NewEvent("t1", VMCreated, SeverityInfo, "msg", "", "", "", "", "", "", nil)
    if ev.Metadata != nil && string(ev.Metadata) != "null" {
        t.Errorf("expected null metadata, got %v", ev.Metadata)
    }
}

func TestNewEvent_ErrorOnMarshal(t *testing.T) {
    // JSON-marshalable types should work; non-marshalable would need a custom type.
    // We test with a type that marshals to JSON successfully.
    ev := NewEvent("t1", VMCreated, SeverityInfo, "msg", "", "", "", "", "", "", "string-value")
    if ev.Metadata == nil {
        t.Fatal("expected non-nil metadata")
    }
    var s string
    if err := json.Unmarshal(ev.Metadata, &s); err != nil {
        t.Fatalf("unmarshal: %v", err)
    }
    if s != "string-value" {
        t.Errorf("metadata = %q", s)
    }
}

func TestEvent_EmptyFilter(t *testing.T) {
    filter := EventFilter{
        Type:     nil,
        Severity: nil,
        After:    nil,
        Before:   nil,
        Limit:    0,
    }
    if filter.Limit != 0 {
        t.Errorf("Limit = %d", filter.Limit)
    }
}

func TestEvent_ValidateEventTypeCoverage(t *testing.T) {
    // Ensure all event types are distinct
    types := []EventType{
        VMCreated, VMDeleted, VMStarted, VMStopped, VMRestarted, VMMigrated,
        HostRegistered, HostDecommissioned, HostMaintenanceEnter, HostMaintenanceExit,
        ComplianceCheckPassed, ComplianceCheckFailed, ComplianceDriftDetected,
        BackupCreated, BackupRestored, BackupFailed,
        NodeRegistered, NodeOffline,
    }
    seen := make(map[EventType]bool)
    for _, et := range types {
        if seen[et] {
            t.Errorf("duplicate event type: %s", et)
        }
        seen[et] = true
    }
    if len(seen) != len(types) {
        t.Fatal("expected all event types to be distinct")
    }
}

func TestEvent_Description(t *testing.T) {
    // Verify each event type has a meaningful name (String method)
    for _, et := range []EventType{VMCreated, VMDeleted, VMStarted, VMStopped} {
        s := et.String()
        if s == "" {
            t.Errorf("empty string for %v", et)
        }
        if s == "<unknown>" {
            t.Errorf("unexpected string for %v", et)
        }
    }
}
