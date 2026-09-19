package vcenter

import (
	"context"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		cfg     ClientConfig
		wantErr bool
	}{
		{
			name:    "empty host returns error",
			cfg:     ClientConfig{Username: "admin"},
			wantErr: true,
		},
		{
			name:    "empty username returns error",
			cfg:     ClientConfig{Host: "vcenter.local"},
			wantErr: true,
		},
		{
			name: "valid config succeeds",
			cfg: ClientConfig{
				Host:     "vcenter.local",
				Username: "admin",
				Password: "secret",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewClient(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if c == nil {
				t.Fatal("expected non-nil client")
			}
			if c.host != tt.cfg.Host {
				t.Errorf("expected host %s, got %s", tt.cfg.Host, c.host)
			}
		})
	}
}

func TestClientConnectDisconnect(t *testing.T) {
	c, err := NewClient(ClientConfig{
		Host:     "vcenter.local",
		Port:     443,
		Username: "admin",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	ctx := context.Background()

	if c.IsConnected() {
		t.Error("expected client to be disconnected initially")
	}

	if err := c.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	if !c.IsConnected() {
		t.Error("expected client to be connected after Connect")
	}

	if err := c.Disconnect(); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}

	if c.IsConnected() {
		t.Error("expected client to be disconnected after Disconnect")
	}
}

func TestDiscover(t *testing.T) {
	c, err := NewClient(ClientConfig{
		Host:     "vcenter.local",
		Username: "admin",
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	ctx := context.Background()

	// Should fail without connect
	_, err = c.Discover(ctx)
	if err == nil {
		t.Fatal("expected error when not connected")
	}

	if err := c.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Disconnect()

	result, err := c.Discover(ctx)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	if len(result.Datacenters) == 0 {
		t.Error("expected at least one datacenter")
	}

	if result.RetrievedAt.IsZero() {
		t.Error("expected RetrievedAt to be set")
	}

	dc := result.Datacenters[0]
	if dc.Name == "" {
		t.Error("expected datacenter name")
	}
	if len(dc.Hosts) == 0 {
		t.Error("expected at least one host")
	}
	if len(dc.VMs) == 0 {
		t.Error("expected at least one VM")
	}
}

func TestGetVM(t *testing.T) {
	c, err := NewClient(ClientConfig{
		Host:     "vcenter.local",
		Username: "admin",
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	ctx := context.Background()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Disconnect()

	vm, err := c.GetVM(ctx, "vm-1001")
	if err != nil {
		t.Fatalf("GetVM: %v", err)
	}
	if vm.ID != "vm-1001" {
		t.Errorf("expected VM ID vm-1001, got %s", vm.ID)
	}
	if vm.Name != "SAP-APP-01" {
		t.Errorf("expected VM name SAP-APP-01, got %s", vm.Name)
	}

	// Non-existent VM
	_, err = c.GetVM(ctx, "vm-nonexistent")
	if err == nil {
		t.Error("expected error for non-existent VM")
	}
}

func TestSessionExpiry(t *testing.T) {
	c, err := NewClient(ClientConfig{
		Host:     "vcenter.local",
		Username: "admin",
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	ctx := context.Background()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	// Simulate session expiry
	c.sessionExpiry = time.Now().Add(-1 * time.Second)
	if c.IsConnected() {
		t.Error("expected session to be expired")
	}
}
