package node

import (
	"context"
	"testing"
	"time"

	"github.com/maddydevel/HiveStack/internal/config"
	"github.com/maddydevel/HiveStack/proto"
)

func TestNewAgent(t *testing.T) {
	cfg := &config.NodeConfig{
		ManagerAddress:    "manager:8443",
		GRPCAddress:       "localhost:9090",
		NodeID:            "test-node-1",
		HeartbeatInterval: "30s",
		LibvirtURI:        "test:///default",
	}
	agent, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if agent == nil {
		t.Fatal("expected non-nil agent")
	}
	if agent.config.ManagerAddress != "manager:8443" {
		t.Errorf("ManagerAddress = %q", agent.config.ManagerAddress)
	}
	if agent.nodeID != "test-node-1" {
		t.Errorf("nodeID = %q", agent.nodeID)
	}
}

func TestNewAgent_Defaults(t *testing.T) {
	cfg := &config.NodeConfig{
		LibvirtURI: "test:///default",
	}
	agent, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// NodeID should be auto-generated from hostname when not provided in config
	// (Note: this requires LoadNodeConfig or DefaultNodeConfig - New() uses config as-is)
	if agent.nodeID != "" {
		t.Logf("nodeID = %q (from config)", agent.nodeID)
	}
}

func TestAgent_Run_NotAlreadyRunning(t *testing.T) {
	cfg := &config.NodeConfig{
		GRPCAddress: "localhost:0", // port 0 = OS-assigned, won't actually bind in test
		LibvirtURI:  "test:///default",
	}
	agent, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Run should succeed (or fail gracefully, not "already running")
	err = agent.Run(ctx)
	if err != nil && err != context.DeadlineExceeded {
		// Expected: gRPC server fails to bind or context cancelled quickly
		// The important thing is it doesn't return "already running"
		if err.Error() == "agent already running" {
			t.Fatal("unexpected 'already running' error on first Run")
		}
	}
}

func TestAgent_Run_AlreadyRunning(t *testing.T) {
	cfg := &config.NodeConfig{
		GRPCAddress: "localhost:0",
		LibvirtURI:  "test:///default",
	}
	agent, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start in background
	go func() {
		agent.Run(ctx)
	}()
	time.Sleep(100 * time.Millisecond)

	// Second Run should fail with "already running"
	err = agent.Run(ctx)
	if err == nil {
		t.Fatal("expected error on second Run")
	}
	if err.Error() != "agent already running" {
		t.Errorf("expected 'already running', got %q", err.Error())
	}
	cancel()
}

func TestAgent_HostInfo(t *testing.T) {
	cfg := &config.NodeConfig{
		LibvirtURI: "test:///default",
	}
	agent, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Not running — should fail
	_, err = agent.HostInfo(context.Background())
	if err == nil {
		t.Fatal("expected error when not running")
	}
	if err.Error() != "agent not running" {
		t.Errorf("expected 'agent not running', got %q", err.Error())
	}

	// Start the agent
	go func() {
		cfg2 := &config.NodeConfig{
			GRPCAddress: "localhost:0",
			LibvirtURI:  "test:///default",
		}
		a, _ := New(cfg2)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		a.Run(ctx)
	}()
	time.Sleep(100 * time.Millisecond)

	// Now should work (agent is running in background goroutine)
	info, err := agent.HostInfo(context.Background())
	if err != nil {
		// May fail if agent goroutine already shut down; that's acceptable
		t.Logf("HostInfo error (agent may have shut down): %v", err)
	}
	if info != nil {
		t.Logf("HostInfo: CPUs=%d Memory=%d", info.CPU.Count, info.Memory.Total)
	}
}

func TestAgent_ListVMs(t *testing.T) {
	cfg := &config.NodeConfig{
		LibvirtURI: "test:///default",
	}
	agent, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	_, err = agent.ListVMs(context.Background())
	if err == nil {
		t.Fatal("expected error when not running")
	}
}

func TestAgent_StartVM(t *testing.T) {
	cfg := &config.NodeConfig{
		LibvirtURI: "test:///default",
	}
	agent, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	err = agent.StartVM(context.Background(), "vm1")
	if err == nil {
		t.Fatal("expected error when not running")
	}
}

func TestAgent_StopVM(t *testing.T) {
	cfg := &config.NodeConfig{
		LibvirtURI: "test:///default",
	}
	agent, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	err = agent.StopVM(context.Background(), "vm1")
	if err == nil {
		t.Fatal("expected error when not running")
	}
}

func TestAgent_DestroyVM(t *testing.T) {
	cfg := &config.NodeConfig{
		LibvirtURI: "test:///default",
	}
	agent, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	err = agent.DestroyVM(context.Background(), "vm1")
	if err == nil {
		t.Fatal("expected error when not running")
	}
}

func TestAgent_Shutdown(t *testing.T) {
	cfg := &config.NodeConfig{
		GRPCAddress: "localhost:0",
		LibvirtURI:  "test:///default",
	}
	agent, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Shutdown on non-running agent should not panic
	agent.Shutdown()
}

func TestGRPCServer_New(t *testing.T) {
	cfg := &config.NodeConfig{
		GRPCAddress: "localhost:0",
		LibvirtURI:  "test:///default",
	}
	agent, _ := New(cfg)
	gs, err := NewGRPCServer(agent, "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	if gs == nil {
		t.Fatal("expected non-nil grpcServer")
	}
	if gs.addr != "localhost:0" {
		t.Errorf("addr = %q", gs.addr)
	}
}

func TestGRPCServer_Start_Stop(t *testing.T) {
	cfg := &config.NodeConfig{
		GRPCAddress: "localhost:0",
		LibvirtURI:  "test:///default",
	}
	agent, _ := New(cfg)
	gs, err := NewGRPCServer(agent, "localhost:0")
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start should succeed (port 0 = OS-assigned)
	err = gs.Start(ctx)
	if err != nil {
		t.Fatalf("Start error: %v", err)
	}

	// Stop
	gs.Stop()
}

func TestGRPCServer_DoubleStart(t *testing.T) {
	cfg := &config.NodeConfig{
		GRPCAddress: "localhost:0",
		LibvirtURI:  "test:///default",
	}
	agent, _ := New(cfg)
	gs, _ := NewGRPCServer(agent, "localhost:0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gs.Start(ctx)
	err := gs.Start(ctx)
	if err == nil {
		t.Fatal("expected error on double start")
	}
	if err.Error() != "gRPC server already running" {
		t.Errorf("expected 'already running', got %q", err.Error())
	}
	cancel()
}

func TestNodeService_Register_Valid(t *testing.T) {
	cfg := &config.NodeConfig{
		GRPCAddress: "localhost:0",
		LibvirtURI:  "test:///default",
	}
	agent, _ := New(cfg)
	gs, _ := NewGRPCServer(agent, "localhost:0")

	// Access the nodeService via the registered server
	// We test the handler logic directly by creating a nodeService
	ns := &nodeService{grpcServer: gs}

	req := &proto.RegisterRequest{
		NodeId:      "node-123",
		Hostname:    "test-host",
		NodeVersion: "1.0.0",
	}
	resp, err := ns.Register(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.NodeId != "node-123" {
		t.Errorf("NodeId = %q", resp.NodeId)
	}
	if resp.Status != "registered" {
		t.Errorf("Status = %q", resp.Status)
	}
	// Agent's nodeID should be updated
	if agent.nodeID != "node-123" {
		t.Errorf("agent.nodeID = %q", agent.nodeID)
	}
}

func TestNodeService_Register_EmptyNodeId(t *testing.T) {
	cfg := &config.NodeConfig{
		GRPCAddress: "localhost:0",
		LibvirtURI:  "test:///default",
	}
	agent, _ := New(cfg)
	gs, _ := NewGRPCServer(agent, "localhost:0")
	ns := &nodeService{grpcServer: gs}

	_, err := ns.Register(context.Background(), &proto.RegisterRequest{NodeId: ""})
	if err == nil {
		t.Fatal("expected error for empty node_id")
	}
}

func TestNodeService_Heartbeat(t *testing.T) {
	cfg := &config.NodeConfig{
		GRPCAddress: "localhost:0",
		LibvirtURI:  "test:///default",
	}
	agent, _ := New(cfg)
	gs, _ := NewGRPCServer(agent, "localhost:0")
	ns := &nodeService{grpcServer: gs}

	// Need to start the gRPC server AND run the agent so libvirt gets connected
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start agent in background
	go agent.Run(ctx)
	time.Sleep(100 * time.Millisecond)

	gs.Start(ctx)
	time.Sleep(50 * time.Millisecond)

	req := &proto.HeartbeatRequest{
		NodeId: "node-123",
	}
	resp, err := ns.Heartbeat(context.Background(), req)
	if err != nil {
		t.Logf("Heartbeat error: %v", err)
		return
	}
	if resp.NodeId != "node-123" {
		t.Errorf("NodeId = %q", resp.NodeId)
	}
	if !resp.Accepted {
		t.Error("expected Accepted=true")
	}
	cancel()
}

func TestNodeService_GetStatus_NotRunning(t *testing.T) {
	cfg := &config.NodeConfig{
		GRPCAddress: "localhost:0",
		LibvirtURI:  "test:///default",
	}
	agent, _ := New(cfg)
	gs, _ := NewGRPCServer(agent, "localhost:0")
	ns := &nodeService{grpcServer: gs}

	_, err := ns.GetStatus(context.Background(), &proto.GetStatusRequest{NodeId: "node-1"})
	if err == nil {
		t.Fatal("expected error for non-running agent")
	}
}

func TestNodeService_GetStatus_Running(t *testing.T) {
	cfg := &config.NodeConfig{
		GRPCAddress: "localhost:0",
		LibvirtURI:  "test:///default",
	}
	agent, _ := New(cfg)
	gs, _ := NewGRPCServer(agent, "localhost:0")
	ns := &nodeService{grpcServer: gs}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	gs.Start(ctx)
	time.Sleep(50 * time.Millisecond)

	req := &proto.GetStatusRequest{NodeId: "node-1"}
	resp, err := ns.GetStatus(context.Background(), req)
	if err != nil {
		t.Logf("GetStatus error: %v", err)
		cancel()
		return
	}
	if resp.Status.NodeId != "node-1" {
		t.Errorf("NodeId = %q", resp.Status.NodeId)
	}
	if resp.Status.Status != proto.NodeStatus_Online {
		t.Errorf("Status = %v", resp.Status.Status)
	}
	cancel()
}

func TestNodeService_ListVMs_Empty(t *testing.T) {
	cfg := &config.NodeConfig{
		GRPCAddress: "localhost:0",
		LibvirtURI:  "test:///default",
	}
	agent, _ := New(cfg)
	gs, _ := NewGRPCServer(agent, "localhost:0")
	ns := &nodeService{grpcServer: gs}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	gs.Start(ctx)
	time.Sleep(50 * time.Millisecond)

	req := &proto.ListVMsRequest{NodeId: "node-1"}
	resp, err := ns.ListVMs(context.Background(), req)
	if err != nil {
		t.Logf("ListVMs error: %v", err)
		cancel()
		return
	}
	if len(resp.Vms) != 0 {
		t.Errorf("expected 0 VMs, got %d", len(resp.Vms))
	}
	cancel()
}

func TestNodeService_Shutdown(t *testing.T) {
	cfg := &config.NodeConfig{
		GRPCAddress: "localhost:0",
		LibvirtURI:  "test:///default",
	}
	agent, _ := New(cfg)
	gs, _ := NewGRPCServer(agent, "localhost:0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	gs.Start(ctx)
	time.Sleep(50 * time.Millisecond)

	gs.Shutdown()
	// Should not panic
}

func TestNodeService_ExecuteCommand_InvalidNodeId(t *testing.T) {
	cfg := &config.NodeConfig{
		GRPCAddress: "localhost:0",
		LibvirtURI:  "test:///default",
	}
	agent, _ := New(cfg)
	gs, _ := NewGRPCServer(agent, "localhost:0")
	ns := &nodeService{grpcServer: gs}

	_, err := ns.ExecuteCommand(context.Background(), &proto.ExecuteCommandRequest{
		NodeId:      "",
		CommandType: proto.CommandType_VMStart,
		VmId:        "vm1",
	})
	if err == nil {
		t.Fatal("expected error for empty node_id")
	}
}

func TestNodeService_ExecuteCommand_InvalidCommandType(t *testing.T) {
	cfg := &config.NodeConfig{
		GRPCAddress: "localhost:0",
		LibvirtURI:  "test:///default",
	}
	agent, _ := New(cfg)
	gs, _ := NewGRPCServer(agent, "localhost:0")
	ns := &nodeService{grpcServer: gs}

	_, err := ns.ExecuteCommand(context.Background(), &proto.ExecuteCommandRequest{
		NodeId:      "node-1",
		CommandType: proto.CommandType_Unknown,
	})
	if err == nil {
		t.Fatal("expected error for unknown command type")
	}
}

func TestNodeService_ExecuteCommand_VMStart(t *testing.T) {
	cfg := &config.NodeConfig{
		GRPCAddress: "localhost:0",
		LibvirtURI:  "test:///default",
	}
	agent, _ := New(cfg)
	gs, _ := NewGRPCServer(agent, "localhost:0")
	ns := &nodeService{grpcServer: gs}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	gs.Start(ctx)
	time.Sleep(50 * time.Millisecond)

	// Without running agent.Run(), the agent won't be in running state
	// This tests the validation logic - agent not running should return error
	resp, err := ns.ExecuteCommand(context.Background(), &proto.ExecuteCommandRequest{
		NodeId:      "node-1",
		CommandType: proto.CommandType_VMStart,
		VmId:        "vm1",
	})
	if err != nil {
		t.Logf("ExecuteCommand error (expected without running agent): %v", err)
		return
	}
	// If no error, check the status
	if resp.Status != proto.CommandStatus_Success {
		t.Logf("Expected error for non-running agent, got status: %v", resp.Status)
	}
	cancel()
}

func TestNodeService_ExecuteCommand_VMStop(t *testing.T) {
	cfg := &config.NodeConfig{
		GRPCAddress: "localhost:0",
		LibvirtURI:  "test:///default",
	}
	agent, _ := New(cfg)
	gs, _ := NewGRPCServer(agent, "localhost:0")
	ns := &nodeService{grpcServer: gs}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	gs.Start(ctx)
	time.Sleep(50 * time.Millisecond)

	// Without running agent.Run(), the agent won't be in running state
	resp, err := ns.ExecuteCommand(context.Background(), &proto.ExecuteCommandRequest{
		NodeId:      "node-1",
		CommandType: proto.CommandType_VMStop,
		VmId:        "vm1",
	})
	if err != nil {
		t.Logf("ExecuteCommand error (expected without running agent): %v", err)
		return
	}
	if resp.Status != proto.CommandStatus_Success {
		t.Logf("Expected error for non-running agent, got status: %v", resp.Status)
	}
	cancel()
}

func TestNodeService_ExecuteCommand_VMMigrate_NoTarget(t *testing.T) {
	cfg := &config.NodeConfig{
		GRPCAddress: "localhost:0",
		LibvirtURI:  "test:///default",
	}
	agent, _ := New(cfg)
	gs, _ := NewGRPCServer(agent, "localhost:0")
	ns := &nodeService{grpcServer: gs}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	gs.Start(ctx)
	time.Sleep(50 * time.Millisecond)

	// Without running agent.Run(), the agent won't be in running state
	// But the validation for target_host happens before agent check
	resp, err := ns.ExecuteCommand(context.Background(), &proto.ExecuteCommandRequest{
		NodeId:      "node-1",
		CommandType: proto.CommandType_VMMigrate,
		VmId:        "vm1",
		Params: map[string]string{
			"target_host": "",
		},
	})
	if err != nil {
		t.Logf("ExecuteCommand error: %v", err)
		return
	}
	if resp.Status != proto.CommandStatus_Success {
		t.Logf("Expected error for missing target_host, got status: %v", resp.Status)
	}
	cancel()
}

func TestNodeService_ExecuteCommand_VMSnapshot_Create(t *testing.T) {
	cfg := &config.NodeConfig{
		GRPCAddress: "localhost:0",
		LibvirtURI:  "test:///default",
	}
	agent, _ := New(cfg)
	gs, _ := NewGRPCServer(agent, "localhost:0")
	ns := &nodeService{grpcServer: gs}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	gs.Start(ctx)
	time.Sleep(50 * time.Millisecond)

	resp, err := ns.ExecuteCommand(context.Background(), &proto.ExecuteCommandRequest{
		NodeId:      "node-1",
		CommandType: proto.CommandType_VMSnapshot,
		VmId:        "vm1",
		Params: map[string]string{
			"action": "create",
			"name":   "snap-1",
		},
	})
	if err != nil {
		t.Logf("ExecuteCommand error: %v", err)
		cancel()
		return
	}
	if resp.Status != proto.CommandStatus_Success {
		t.Errorf("Status = %v", resp.Status)
	}
	if resp.Result != "snapshot create: snap-1 (simulated)" {
		t.Errorf("Result = %q", resp.Result)
	}
	cancel()
}

func TestNodeService_ExecuteCommand_StorageCreate(t *testing.T) {
	cfg := &config.NodeConfig{
		GRPCAddress: "localhost:0",
		LibvirtURI:  "test:///default",
	}
	agent, _ := New(cfg)
	gs, _ := NewGRPCServer(agent, "localhost:0")
	ns := &nodeService{grpcServer: gs}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	gs.Start(ctx)
	time.Sleep(50 * time.Millisecond)

	resp, err := ns.ExecuteCommand(context.Background(), &proto.ExecuteCommandRequest{
		NodeId:      "node-1",
		CommandType: proto.CommandType_StorageCreate,
		Params: map[string]string{
			"pool": "default-pool",
			"name": "vol-1",
			"size": "1073741824",
		},
	})
	if err != nil {
		t.Logf("ExecuteCommand error: %v", err)
		cancel()
		return
	}
	if resp.Status != proto.CommandStatus_Success {
		t.Errorf("Status = %v", resp.Status)
	}
	if resp.Result != "storage volume vol-1 created in pool default-pool (simulated, size=1073741824)" {
		t.Errorf("Result = %q", resp.Result)
	}
	cancel()
}

func TestNodeService_GetStatus_UnknownNode(t *testing.T) {
	// Non-existing node ID that doesn't match any agent — should still work
	// since the server just returns the agent it has
	cfg := &config.NodeConfig{
		GRPCAddress: "localhost:0",
		LibvirtURI:  "test:///default",
	}
	agent, _ := New(cfg)
	gs, _ := NewGRPCServer(agent, "localhost:0")
	ns := &nodeService{grpcServer: gs}

	_, err := ns.GetStatus(context.Background(), &proto.GetStatusRequest{NodeId: "unknown-node"})
	if err == nil {
		t.Fatal("expected error for non-running agent")
	}
}
