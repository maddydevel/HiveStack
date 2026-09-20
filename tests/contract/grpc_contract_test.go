// Package contract provides gRPC contract tests for the Node Agent service.
//
// These tests verify the gRPC service contract by:
// 1. Starting a real gRPC server with a mock NodeAgentServer implementation
// 2. Connecting a client to it
// 3. Verifying request/response shapes and error behavior
//
// This follows httptest-style patterns: real server, real client, mock handler.
package contract

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/maddydevel/HiveStack/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// mockNodeAgent is a test double for the NodeAgentServer interface.
// Each method can be overridden by tests to control behavior.
type mockNodeAgent struct {
	proto.UnimplementedNodeAgentServer

	RegisterFunc       func(ctx context.Context, req *proto.RegisterRequest) (*proto.RegisterResponse, error)
	HeartbeatFunc      func(ctx context.Context, req *proto.HeartbeatRequest) (*proto.HeartbeatResponse, error)
	ExecuteCommandFunc func(ctx context.Context, req *proto.ExecuteCommandRequest) (*proto.ExecuteCommandResponse, error)
	GetStatusFunc      func(ctx context.Context, req *proto.GetStatusRequest) (*proto.GetStatusResponse, error)
	ListVMsFunc        func(ctx context.Context, req *proto.ListVMsRequest) (*proto.ListVMsResponse, error)
}

func (m *mockNodeAgent) Register(ctx context.Context, req *proto.RegisterRequest) (*proto.RegisterResponse, error) {
	if m.RegisterFunc != nil {
		return m.RegisterFunc(ctx, req)
	}
	return nil, nil
}

func (m *mockNodeAgent) Heartbeat(ctx context.Context, req *proto.HeartbeatRequest) (*proto.HeartbeatResponse, error) {
	if m.HeartbeatFunc != nil {
		return m.HeartbeatFunc(ctx, req)
	}
	return nil, nil
}

func (m *mockNodeAgent) ExecuteCommand(ctx context.Context, req *proto.ExecuteCommandRequest) (*proto.ExecuteCommandResponse, error) {
	if m.ExecuteCommandFunc != nil {
		return m.ExecuteCommandFunc(ctx, req)
	}
	return nil, nil
}

func (m *mockNodeAgent) GetStatus(ctx context.Context, req *proto.GetStatusRequest) (*proto.GetStatusResponse, error) {
	if m.GetStatusFunc != nil {
		return m.GetStatusFunc(ctx, req)
	}
	return nil, nil
}

func (m *mockNodeAgent) ListVMs(ctx context.Context, req *proto.ListVMsRequest) (*proto.ListVMsResponse, error) {
	if m.ListVMsFunc != nil {
		return m.ListVMsFunc(ctx, req)
	}
	return nil, nil
}

// newTestServer creates a gRPC server with the given mock handler and returns
// a connected client. Uses bufconn for in-memory communication (no real TCP).
func newTestServer(t *testing.T, handler proto.NodeAgentServer) (proto.NodeAgentClient, func()) {
	t.Helper()

	bufSize := 1024 * 1024
	lis := bufconn.Listen(bufSize)

	srv := grpc.NewServer()
	proto.RegisterNodeAgentServer(srv, handler)

	go func() {
		if err := srv.Serve(lis); err != nil {
			// Server stopped — expected during cleanup
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx,
		"bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Fatalf("failed to dial bufnet: %v", err)
	}

	client := proto.NewNodeAgentClient(conn)

	cleanup := func() {
		conn.Close()
		srv.Stop()
	}

	return client, cleanup
}

// ─── TestRegister ─────────────────────────────────────────────────────────────

func TestRegister(t *testing.T) {
	t.Run("successful registration", func(t *testing.T) {
		var receivedReq *proto.RegisterRequest

		mock := &mockNodeAgent{
			RegisterFunc: func(ctx context.Context, req *proto.RegisterRequest) (*proto.RegisterResponse, error) {
				receivedReq = req
				return &proto.RegisterResponse{
					NodeId:    req.NodeId,
					ManagerId: "manager-001",
					Status:    "registered",
				}, nil
			},
		}

		client, cleanup := newTestServer(t, mock)
		defer cleanup()

		req := &proto.RegisterRequest{
			NodeId:       "node-001",
			Hostname:     "host-001.example.com",
			NodeVersion:  "1.0.0",
			NodeAgent:    "hive-node",
			Capabilities: []string{"kvm", "ceph", "nfs"},
		}

		resp, err := client.Register(context.Background(), req)
		if err != nil {
			t.Fatalf("Register() error = %v", err)
		}

		// Verify request was received correctly
		if receivedReq.NodeId != "node-001" {
			t.Errorf("received NodeId = %q, want %q", receivedReq.NodeId, "node-001")
		}
		if receivedReq.Hostname != "host-001.example.com" {
			t.Errorf("received Hostname = %q", receivedReq.Hostname)
		}
		if len(receivedReq.Capabilities) != 3 {
			t.Errorf("received %d capabilities, want 3", len(receivedReq.Capabilities))
		}

		// Verify response
		if resp.NodeId != "node-001" {
			t.Errorf("NodeId = %q, want %q", resp.NodeId, "node-001")
		}
		if resp.ManagerId != "manager-001" {
			t.Errorf("ManagerId = %q", resp.ManagerId)
		}
		if resp.Status != "registered" {
			t.Errorf("Status = %q", resp.Status)
		}
	})

	t.Run("empty node_id rejected", func(t *testing.T) {
		mock := &mockNodeAgent{
			RegisterFunc: func(ctx context.Context, req *proto.RegisterRequest) (*proto.RegisterResponse, error) {
				if req.NodeId == "" {
					return nil, fmt.Errorf("node_id is required")
				}
				return &proto.RegisterResponse{Status: "registered"}, nil
			},
		}

		client, cleanup := newTestServer(t, mock)
		defer cleanup()

		req := &proto.RegisterRequest{
			Hostname: "host-001",
		}

		_, err := client.Register(context.Background(), req)
		if err == nil {
			t.Fatal("expected error for empty node_id")
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		mock := &mockNodeAgent{
			RegisterFunc: func(ctx context.Context, req *proto.RegisterRequest) (*proto.RegisterResponse, error) {
				return &proto.RegisterResponse{Status: "registered"}, nil
			},
		}

		client, cleanup := newTestServer(t, mock)
		defer cleanup()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := client.Register(ctx, &proto.RegisterRequest{NodeId: "node-001"})
		if err == nil {
			t.Fatal("expected error for cancelled context")
		}
	})
}

// ─── TestHeartbeat ────────────────────────────────────────────────────────────

func TestHeartbeat(t *testing.T) {
	t.Run("heartbeat accepted", func(t *testing.T) {
		var receivedReq *proto.HeartbeatRequest

		mock := &mockNodeAgent{
			HeartbeatFunc: func(ctx context.Context, req *proto.HeartbeatRequest) (*proto.HeartbeatResponse, error) {
				receivedReq = req
				return &proto.HeartbeatResponse{
					NodeId:   req.NodeId,
					Accepted: true,
					Message:  "ok",
				}, nil
			},
		}

		client, cleanup := newTestServer(t, mock)
		defer cleanup()

		req := &proto.HeartbeatRequest{
			NodeId: "node-001",
			Status: &proto.NodeStatus{
				NodeId:      "node-001",
				Hostname:    "host-001",
				Status:      proto.NodeStatus_Online,
				CpuCount:    8,
				MemoryTotal: 34359738368,
				MemoryFree:  17179869184,
				DiskTotal:   2199023255552,
				DiskFree:    1099511627776,
				Timestamp:   time.Now().Unix(),
			},
			Since: time.Now().Unix(),
		}

		resp, err := client.Heartbeat(context.Background(), req)
		if err != nil {
			t.Fatalf("Heartbeat() error = %v", err)
		}

		if !resp.Accepted {
			t.Error("expected heartbeat to be accepted")
		}
		if resp.NodeId != "node-001" {
			t.Errorf("NodeId = %q", resp.NodeId)
		}
		if receivedReq.Status.Status != proto.NodeStatus_Online {
			t.Errorf("received status = %v, want Online", receivedReq.Status.Status)
		}
	})

	t.Run("heartbeat with VMs", func(t *testing.T) {
		mock := &mockNodeAgent{
			HeartbeatFunc: func(ctx context.Context, req *proto.HeartbeatRequest) (*proto.HeartbeatResponse, error) {
				return &proto.HeartbeatResponse{
					NodeId:   req.NodeId,
					Accepted: true,
					Message:  "ok",
				}, nil
			},
		}

		client, cleanup := newTestServer(t, mock)
		defer cleanup()

		req := &proto.HeartbeatRequest{
			NodeId: "node-001",
			Status: &proto.NodeStatus{
				NodeId: "node-001",
				Vms: []*proto.VMInfo{
					{
						VmId:     "vm-001",
						Name:     "web-server",
						CpuCount: 2,
						Memory:   4294967296,
						State:    proto.VMState_Running,
						Status:   proto.VMStatus_Running,
					},
					{
						VmId:     "vm-002",
						Name:     "db-server",
						CpuCount: 4,
						Memory:   8589934592,
						State:    proto.VMState_Running,
						Status:   proto.VMStatus_Running,
					},
				},
			},
		}

		resp, err := client.Heartbeat(context.Background(), req)
		if err != nil {
			t.Fatalf("Heartbeat() error = %v", err)
		}
		if !resp.Accepted {
			t.Error("expected heartbeat accepted")
		}
	})

	t.Run("heartbeat rejected for unknown node", func(t *testing.T) {
		mock := &mockNodeAgent{
			HeartbeatFunc: func(ctx context.Context, req *proto.HeartbeatRequest) (*proto.HeartbeatResponse, error) {
				return &proto.HeartbeatResponse{
					NodeId:   req.NodeId,
					Accepted: false,
					Message:  "unknown node",
				}, nil
			},
		}

		client, cleanup := newTestServer(t, mock)
		defer cleanup()

		req := &proto.HeartbeatRequest{NodeId: "unknown-node"}
		resp, err := client.Heartbeat(context.Background(), req)
		if err != nil {
			t.Fatalf("Heartbeat() error = %v", err)
		}
		if resp.Accepted {
			t.Error("expected heartbeat rejected for unknown node")
		}
	})
}

// ─── TestExecuteCommand ───────────────────────────────────────────────────────

func TestExecuteCommand(t *testing.T) {
	t.Run("VM create command", func(t *testing.T) {
		var receivedReq *proto.ExecuteCommandRequest

		mock := &mockNodeAgent{
			ExecuteCommandFunc: func(ctx context.Context, req *proto.ExecuteCommandRequest) (*proto.ExecuteCommandResponse, error) {
				receivedReq = req
				return &proto.ExecuteCommandResponse{
					CommandId:   req.CommandId,
					Status:      proto.CommandStatus_Success,
					Result:      "vm-001 created",
					CompletedAt: time.Now().Unix(),
				}, nil
			},
		}

		client, cleanup := newTestServer(t, mock)
		defer cleanup()

		req := &proto.ExecuteCommandRequest{
			NodeId:      "node-001",
			CommandId:   "cmd-001",
			CommandType: proto.CommandType_VMCreate,
			VmId:        "vm-001",
			Params: map[string]string{
				"name":   "web-server",
				"cpus":   "2",
				"memory": "4294967296",
			},
			CreatedAt: time.Now().Unix(),
		}

		resp, err := client.ExecuteCommand(context.Background(), req)
		if err != nil {
			t.Fatalf("ExecuteCommand() error = %v", err)
		}

		if receivedReq.CommandType != proto.CommandType_VMCreate {
			t.Errorf("CommandType = %v, want VMCreate", receivedReq.CommandType)
		}
		if resp.Status != proto.CommandStatus_Success {
			t.Errorf("Status = %v, want Success", resp.Status)
		}
		if resp.Result != "vm-001 created" {
			t.Errorf("Result = %q", resp.Result)
		}
	})

	t.Run("VM stop command", func(t *testing.T) {
		mock := &mockNodeAgent{
			ExecuteCommandFunc: func(ctx context.Context, req *proto.ExecuteCommandRequest) (*proto.ExecuteCommandResponse, error) {
				return &proto.ExecuteCommandResponse{
					CommandId:   req.CommandId,
					Status:      proto.CommandStatus_Success,
					Result:      "vm stopped",
					CompletedAt: time.Now().Unix(),
				}, nil
			},
		}

		client, cleanup := newTestServer(t, mock)
		defer cleanup()

		req := &proto.ExecuteCommandRequest{
			NodeId:      "node-001",
			CommandId:   "cmd-002",
			CommandType: proto.CommandType_VMStop,
			VmId:        "vm-001",
		}

		resp, err := client.ExecuteCommand(context.Background(), req)
		if err != nil {
			t.Fatalf("ExecuteCommand() error = %v", err)
		}
		if resp.Status != proto.CommandStatus_Success {
			t.Errorf("Status = %v, want Success", resp.Status)
		}
	})

	t.Run("command failure", func(t *testing.T) {
		mock := &mockNodeAgent{
			ExecuteCommandFunc: func(ctx context.Context, req *proto.ExecuteCommandRequest) (*proto.ExecuteCommandResponse, error) {
				return &proto.ExecuteCommandResponse{
					CommandId: req.CommandId,
					Status:    proto.CommandStatus_Failed,
					Error:     "VM not found",
				}, nil
			},
		}

		client, cleanup := newTestServer(t, mock)
		defer cleanup()

		req := &proto.ExecuteCommandRequest{
			NodeId:      "node-001",
			CommandId:   "cmd-003",
			CommandType: proto.CommandType_VMStart,
			VmId:        "nonexistent-vm",
		}

		resp, err := client.ExecuteCommand(context.Background(), req)
		if err != nil {
			t.Fatalf("ExecuteCommand() error = %v", err)
		}
		if resp.Status != proto.CommandStatus_Failed {
			t.Errorf("Status = %v, want Failed", resp.Status)
		}
		if resp.Error != "VM not found" {
			t.Errorf("Error = %q", resp.Error)
		}
	})

	t.Run("all command types", func(t *testing.T) {
		mock := &mockNodeAgent{
			ExecuteCommandFunc: func(ctx context.Context, req *proto.ExecuteCommandRequest) (*proto.ExecuteCommandResponse, error) {
				return &proto.ExecuteCommandResponse{
					CommandId: req.CommandId,
					Status:    proto.CommandStatus_Success,
				}, nil
			},
		}

		client, cleanup := newTestServer(t, mock)
		defer cleanup()

		commandTypes := []proto.CommandType{
			proto.CommandType_VMCreate,
			proto.CommandType_VMStart,
			proto.CommandType_VMStop,
			proto.CommandType_VMRestart,
			proto.CommandType_VMMigrate,
			proto.CommandType_VMSnapshot,
			proto.CommandType_VMDestroy,
			proto.CommandType_VMResize,
			proto.CommandType_VMGetStats,
			proto.CommandType_StorageCreate,
			proto.CommandType_StorageDelete,
			proto.CommandType_NetworkCreate,
			proto.CommandType_NetworkDelete,
		}

		for _, ct := range commandTypes {
			t.Run(fmt.Sprintf("CommandType_%d", ct), func(t *testing.T) {
				req := &proto.ExecuteCommandRequest{
					NodeId:      "node-001",
					CommandId:   "cmd-test",
					CommandType: ct,
				}
				resp, err := client.ExecuteCommand(context.Background(), req)
				if err != nil {
					t.Fatalf("ExecuteCommand(%v) error = %v", ct, err)
				}
				if resp.Status != proto.CommandStatus_Success {
					t.Errorf("Status = %v for %v", resp.Status, ct)
				}
			})
		}
	})
}

// ─── TestGetStatus ────────────────────────────────────────────────────────────

func TestGetStatus(t *testing.T) {
	t.Run("returns node status", func(t *testing.T) {
		mock := &mockNodeAgent{
			GetStatusFunc: func(ctx context.Context, req *proto.GetStatusRequest) (*proto.GetStatusResponse, error) {
				return &proto.GetStatusResponse{
					Status: &proto.NodeStatus{
						NodeId:      req.NodeId,
						Hostname:    "host-001",
						Status:      proto.NodeStatus_Online,
						CpuCount:    8,
						MemoryTotal: 34359738368,
						MemoryFree:  17179869184,
						DiskTotal:   2199023255552,
						DiskFree:    1099511627776,
						Vms: []*proto.VMInfo{
							{
								VmId:     "vm-001",
								Name:     "web-server",
								CpuCount: 2,
								Memory:   4294967296,
								State:    proto.VMState_Running,
								Status:   proto.VMStatus_Running,
							},
						},
						Timestamp: time.Now().Unix(),
					},
				}, nil
			},
		}

		client, cleanup := newTestServer(t, mock)
		defer cleanup()

		req := &proto.GetStatusRequest{NodeId: "node-001"}
		resp, err := client.GetStatus(context.Background(), req)
		if err != nil {
			t.Fatalf("GetStatus() error = %v", err)
		}

		if resp.Status.NodeId != "node-001" {
			t.Errorf("NodeId = %q", resp.Status.NodeId)
		}
		if resp.Status.Status != proto.NodeStatus_Online {
			t.Errorf("Status = %v, want Online", resp.Status.Status)
		}
		if resp.Status.CpuCount != 8 {
			t.Errorf("CpuCount = %d", resp.Status.CpuCount)
		}
		if len(resp.Status.Vms) != 1 {
			t.Errorf("VMs count = %d, want 1", len(resp.Status.Vms))
		}
	})

	t.Run("node not found", func(t *testing.T) {
		mock := &mockNodeAgent{
			GetStatusFunc: func(ctx context.Context, req *proto.GetStatusRequest) (*proto.GetStatusResponse, error) {
				return nil, fmt.Errorf("node %s not found", req.NodeId)
			},
		}

		client, cleanup := newTestServer(t, mock)
		defer cleanup()

		req := &proto.GetStatusRequest{NodeId: "nonexistent"}
		_, err := client.GetStatus(context.Background(), req)
		if err == nil {
			t.Fatal("expected error for nonexistent node")
		}
	})

	t.Run("maintenance mode", func(t *testing.T) {
		mock := &mockNodeAgent{
			GetStatusFunc: func(ctx context.Context, req *proto.GetStatusRequest) (*proto.GetStatusResponse, error) {
				return &proto.GetStatusResponse{
					Status: &proto.NodeStatus{
						NodeId: req.NodeId,
						Status: proto.NodeStatus_Maintenance,
					},
				}, nil
			},
		}

		client, cleanup := newTestServer(t, mock)
		defer cleanup()

		req := &proto.GetStatusRequest{NodeId: "node-001"}
		resp, err := client.GetStatus(context.Background(), req)
		if err != nil {
			t.Fatalf("GetStatus() error = %v", err)
		}
		if resp.Status.Status != proto.NodeStatus_Maintenance {
			t.Errorf("Status = %v, want Maintenance", resp.Status.Status)
		}
	})
}

// ─── TestListVMs ──────────────────────────────────────────────────────────────

func TestListVMs(t *testing.T) {
	t.Run("returns VM list", func(t *testing.T) {
		mock := &mockNodeAgent{
			ListVMsFunc: func(ctx context.Context, req *proto.ListVMsRequest) (*proto.ListVMsResponse, error) {
				return &proto.ListVMsResponse{
					Vms: []*proto.VMInfo{
						{
							VmId:     "vm-001",
							Name:     "web-server",
							CpuCount: 2,
							Memory:   4294967296,
							State:    proto.VMState_Running,
							Status:   proto.VMStatus_Running,
						},
						{
							VmId:     "vm-002",
							Name:     "db-server",
							CpuCount: 4,
							Memory:   8589934592,
							State:    proto.VMState_Running,
							Status:   proto.VMStatus_Running,
						},
						{
							VmId:     "vm-003",
							Name:     "cache-server",
							CpuCount: 1,
							Memory:   2147483648,
							State:    proto.VMState_ShutOff,
							Status:   proto.VMStatus_Stopped,
						},
					},
				}, nil
			},
		}

		client, cleanup := newTestServer(t, mock)
		defer cleanup()

		req := &proto.ListVMsRequest{NodeId: "node-001"}
		resp, err := client.ListVMs(context.Background(), req)
		if err != nil {
			t.Fatalf("ListVMs() error = %v", err)
		}

		if len(resp.Vms) != 3 {
			t.Errorf("VMs count = %d, want 3", len(resp.Vms))
		}

		// Verify VM states
		expectedStates := map[string]proto.VMState{
			"vm-001": proto.VMState_Running,
			"vm-002": proto.VMState_Running,
			"vm-003": proto.VMState_ShutOff,
		}
		for _, vm := range resp.Vms {
			if vm.State != expectedStates[vm.VmId] {
				t.Errorf("VM %s state = %v, want %v", vm.VmId, vm.State, expectedStates[vm.VmId])
			}
		}
	})

	t.Run("empty VM list", func(t *testing.T) {
		mock := &mockNodeAgent{
			ListVMsFunc: func(ctx context.Context, req *proto.ListVMsRequest) (*proto.ListVMsResponse, error) {
				return &proto.ListVMsResponse{Vms: []*proto.VMInfo{}}, nil
			},
		}

		client, cleanup := newTestServer(t, mock)
		defer cleanup()

		req := &proto.ListVMsRequest{NodeId: "node-001"}
		resp, err := client.ListVMs(context.Background(), req)
		if err != nil {
			t.Fatalf("ListVMs() error = %v", err)
		}
		if len(resp.Vms) != 0 {
			t.Errorf("expected empty VM list, got %d", len(resp.Vms))
		}
	})

	t.Run("node with migrating VM", func(t *testing.T) {
		mock := &mockNodeAgent{
			ListVMsFunc: func(ctx context.Context, req *proto.ListVMsRequest) (*proto.ListVMsResponse, error) {
				return &proto.ListVMsResponse{
					Vms: []*proto.VMInfo{
						{
							VmId:   "vm-001",
							Name:   "migrating-vm",
							State:  proto.VMState_Running,
							Status: proto.VMStatus_Migrating,
						},
					},
				}, nil
			},
		}

		client, cleanup := newTestServer(t, mock)
		defer cleanup()

		req := &proto.ListVMsRequest{NodeId: "node-001"}
		resp, err := client.ListVMs(context.Background(), req)
		if err != nil {
			t.Fatalf("ListVMs() error = %v", err)
		}
		if len(resp.Vms) != 1 {
			t.Fatalf("expected 1 VM, got %d", len(resp.Vms))
		}
		if resp.Vms[0].Status != proto.VMStatus_Migrating {
			t.Errorf("Status = %v, want Migrating", resp.Vms[0].Status)
		}
	})
}

// ─── Additional contract tests ────────────────────────────────────────────────

func TestContract_AllNodeStatusTypes(t *testing.T) {
	// Verify all NodeStatusType values are transmitted correctly
	types := []proto.NodeStatusType{
		proto.NodeStatus_Unknown,
		proto.NodeStatus_Online,
		proto.NodeStatus_Offline,
		proto.NodeStatus_Maintenance,
		proto.NodeStatus_Draining,
	}

	for _, status := range types {
		t.Run(fmt.Sprintf("Status_%d", status), func(t *testing.T) {
			mock := &mockNodeAgent{
				GetStatusFunc: func(ctx context.Context, req *proto.GetStatusRequest) (*proto.GetStatusResponse, error) {
					return &proto.GetStatusResponse{
						Status: &proto.NodeStatus{
							NodeId: req.NodeId,
							Status: status,
						},
					}, nil
				},
			}

			client, cleanup := newTestServer(t, mock)

			resp, err := client.GetStatus(context.Background(), &proto.GetStatusRequest{NodeId: "node-001"})
			if err != nil {
				cleanup()
				t.Fatalf("GetStatus() error = %v", err)
			}
			if resp.Status.Status != status {
				t.Errorf("Status = %v, want %v", resp.Status.Status, status)
			}
			cleanup()
		})
	}
}

func TestContract_AllVMStates(t *testing.T) {
	states := []proto.VMState{
		proto.VMState_Unknown,
		proto.VMState_Running,
		proto.VMState_Paused,
		proto.VMState_Static,
		proto.VMState_ShutOff,
		proto.VMState_Other,
	}

	for _, state := range states {
		t.Run(fmt.Sprintf("State_%d", state), func(t *testing.T) {
			mock := &mockNodeAgent{
				ListVMsFunc: func(ctx context.Context, req *proto.ListVMsRequest) (*proto.ListVMsResponse, error) {
					return &proto.ListVMsResponse{
						Vms: []*proto.VMInfo{
							{VmId: "vm-001", State: state},
						},
					}, nil
				},
			}

			client, cleanup := newTestServer(t, mock)

			resp, err := client.ListVMs(context.Background(), &proto.ListVMsRequest{NodeId: "node-001"})
			if err != nil {
				cleanup()
				t.Fatalf("ListVMs() error = %v", err)
			}
			if resp.Vms[0].State != state {
				t.Errorf("State = %v, want %v", resp.Vms[0].State, state)
			}
			cleanup()
		})
	}
}

func TestContract_ConcurrentRequests(t *testing.T) {
	// Verify the server handles concurrent requests correctly
	mock := &mockNodeAgent{
		HeartbeatFunc: func(ctx context.Context, req *proto.HeartbeatRequest) (*proto.HeartbeatResponse, error) {
			return &proto.HeartbeatResponse{NodeId: req.NodeId, Accepted: true}, nil
		},
		ListVMsFunc: func(ctx context.Context, req *proto.ListVMsRequest) (*proto.ListVMsResponse, error) {
			return &proto.ListVMsResponse{Vms: []*proto.VMInfo{}}, nil
		},
	}

	client, cleanup := newTestServer(t, mock)
	defer cleanup()

	// Send concurrent heartbeats and list requests
	done := make(chan error, 10)
	for i := 0; i < 5; i++ {
		go func() {
			_, err := client.Heartbeat(context.Background(), &proto.HeartbeatRequest{NodeId: "node-001"})
			done <- err
		}()
		go func() {
			_, err := client.ListVMs(context.Background(), &proto.ListVMsRequest{NodeId: "node-001"})
			done <- err
		}()
	}

	for i := 0; i < 10; i++ {
		if err := <-done; err != nil {
			t.Errorf("concurrent request %d failed: %v", i, err)
		}
	}
}
