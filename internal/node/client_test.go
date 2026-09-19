package node

import (
	"context"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/maddydevel/HiveStack/internal/config"
)

// startLoopback starts a real gRPC server for a fresh agent on 127.0.0.1 and
// returns a Client dialed to it. If running is true the agent is marked as
// running and its libvirt connection is opened, as Agent.Run would do.
func startLoopback(t *testing.T, running bool) (*Agent, *Client) {
	t.Helper()

	agent, err := New(&config.NodeConfig{LibvirtURI: "test:///default"})
	if err != nil {
		t.Fatal(err)
	}
	if running {
		if err := agent.libvirt.Connect(); err != nil {
			t.Fatal(err)
		}
		agent.mu.Lock()
		agent.running = true
		agent.mu.Unlock()
	}

	gs, err := NewGRPCServer(agent, "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := gs.Start(ctx); err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(cancel)
	t.Cleanup(gs.Stop)

	client, err := Dial(ctx, gs.lis.Addr().String(), "node-1",
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	return agent, client
}

func TestClient_VMLifecycle_Loopback(t *testing.T) {
	_, client := startLoopback(t, true)

	tests := []struct {
		name string
		call func(ctx context.Context, id string) error
	}{
		{"StartVM", client.StartVM},
		{"StopVM", client.StopVM},
		{"DestroyVM", client.DestroyVM},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(context.Background(), "vm1"); err != nil {
				t.Fatalf("%s over gRPC: %v", tt.name, err)
			}
		})
	}
}

func TestClient_VMLifecycle_AgentNotRunning(t *testing.T) {
	_, client := startLoopback(t, false)

	tests := []struct {
		name string
		call func(ctx context.Context, id string) error
	}{
		{"StartVM", client.StartVM},
		{"StopVM", client.StopVM},
		{"DestroyVM", client.DestroyVM},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call(context.Background(), "vm1")
			if err == nil {
				t.Fatalf("%s: expected error from a non-running agent", tt.name)
			}
			// The agent's failure travels in the response body and must
			// reach the caller as an error carrying the original message.
			if !strings.Contains(err.Error(), "agent not running") {
				t.Errorf("%s: error %q does not mention %q", tt.name, err, "agent not running")
			}
		})
	}
}

func TestClient_EmptyVMID(t *testing.T) {
	_, client := startLoopback(t, true)

	err := client.StartVM(context.Background(), "")
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("StartVM with empty id: err = %v, want InvalidArgument status", err)
	}
}

func TestClient_ContextCancelled(t *testing.T) {
	_, client := startLoopback(t, true)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := client.StartVM(ctx, "vm1"); err == nil {
		t.Fatal("expected error with cancelled context")
	}
}

func TestDial_RequiresNodeID(t *testing.T) {
	if _, err := Dial(context.Background(), "127.0.0.1:0", ""); err == nil {
		t.Fatal("expected error for empty node id")
	}
}

// The agent does not implement live migration yet, so a migrate command must
// come back as a failure that names the target, which shows target_host made it
// through the client to the server.
func TestClient_MigrateVM_Loopback(t *testing.T) {
	_, client := startLoopback(t, true)

	err := client.MigrateVM(context.Background(), "vm1", "host-b.example")
	if err == nil {
		t.Fatal("expected error: agent migration is not implemented")
	}
	if !strings.Contains(err.Error(), "host-b.example") {
		t.Errorf("error %q does not mention the target host", err)
	}
}

func TestClient_MigrateVM_Validation(t *testing.T) {
	_, client := startLoopback(t, true)

	tests := []struct {
		name         string
		vmID, target string
	}{
		{"empty target", "vm1", ""},
		{"empty vm id", "", "host-b.example"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := client.MigrateVM(context.Background(), tt.vmID, tt.target); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestClient_MigrateVM_UsesMigrateTimeout(t *testing.T) {
	_, client := startLoopback(t, true)

	// The regular call timeout must not cap a migration.
	client.timeout = time.Nanosecond
	err := client.MigrateVM(context.Background(), "vm1", "host-b.example")
	if err == nil || strings.Contains(err.Error(), "DeadlineExceeded") {
		t.Fatalf("migrate was cut short by the regular call timeout: %v", err)
	}

	client.migrateTimeout = time.Nanosecond
	err = client.MigrateVM(context.Background(), "vm1", "host-b.example")
	if err == nil || !strings.Contains(err.Error(), "DeadlineExceeded") {
		t.Fatalf("expected DeadlineExceeded from the migrate timeout, got %v", err)
	}
}
