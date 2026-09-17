// Package node implements gRPC server for the HiveStack Node Agent.
//
// This provides the gRPC server that the Manager connects to for node
// registration, heartbeat, command execution, status queries, and VM listing.
//
// The gRPC API mirrors proto/node.proto service definition using hand-written
// Go structs with protobuf tags (no protoc needed — these are serialization-
// compatible stubs).
//
// In production, generate from .proto files with protoc-gen-go for proper
// gRPC service registration. These stubs provide the data types and a working
// gRPC server framework.
package node

import (
    "context"
    "fmt"
    "log"
    "net"
    "sync"

    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"

    "github.com/maddydevel/HiveStack/proto"
)

// grpcServer holds the gRPC server state.
type grpcServer struct {
    mu      sync.Mutex
    agent   *Agent
    lis     net.Listener
    srv     *grpc.Server
    addr    string
    running bool
}

// nodeService implements the gRPC NodeAgent service.
type nodeService struct {
    grpcServer *grpcServer
    proto.UnimplementedNodeAgentServer
}

// NewGRPCServer creates a gRPC server for the node agent.
func NewGRPCServer(agent *Agent, addr string) (*grpcServer, error) {
    s := &grpcServer{
        agent: agent,
        addr:  addr,
    }
    return s, nil
}

// Start starts the gRPC server.
func (gs *grpcServer) Start(ctx context.Context) error {
    gs.mu.Lock()
    if gs.running {
        gs.mu.Unlock()
        return fmt.Errorf("gRPC server already running")
    }
    gs.running = true
    gs.mu.Unlock()

    lis, err := net.Listen("tcp", gs.addr)
    if err != nil {
        return fmt.Errorf("listen on %s: %w", gs.addr, err)
    }
    gs.lis = lis

    gs.srv = grpc.NewServer()
    proto.RegisterNodeAgentServer(gs.srv, &nodeService{grpcServer: gs})

    log.Printf("[gRPC] Node agent gRPC server listening on %s", gs.addr)

    go func() {
        <-ctx.Done()
        gs.Stop()
    }()

    go func() {
        if err := gs.srv.Serve(gs.lis); err != nil && err != context.Canceled {
            log.Printf("[gRPC] Server error: %v", err)
        }
    }()

    return nil
}

// Stop stops the gRPC server.
func (gs *grpcServer) Stop() {
    gs.mu.Lock()
    if !gs.running {
        gs.mu.Unlock()
        return
    }
    gs.running = false
    gs.mu.Unlock()

    if gs.srv != nil {
        gs.srv.GracefulStop()
    }
    if gs.lis != nil {
        gs.lis.Close()
    }
    log.Printf("[gRPC] Node agent gRPC server stopped")
}

// getAgent returns the agent reference (thread-safe).
func (gs *grpcServer) getAgent() *Agent {
    gs.mu.Lock()
    defer gs.mu.Unlock()
    return gs.agent
}

// ─── gRPC Handlers ────────────────────────────────────────────────────────────

// Register handles node registration from the Manager.
func (s *nodeService) Register(ctx context.Context, req *proto.RegisterRequest) (*proto.RegisterResponse, error) {
    if req.NodeId == "" {
        return nil, status.Error(codes.InvalidArgument, "node_id is required")
    }

    agent := s.grpcServer.getAgent()
    agent.mu.Lock()
    agent.nodeID = req.NodeId
    if req.Hostname != "" {
        agent.hostname = req.Hostname
    }
    agent.mu.Unlock()

    log.Printf("[gRPC] Node %s registered (hostname=%s, version=%s)",
        req.NodeId, req.Hostname, req.NodeVersion)

    return &proto.RegisterResponse{
        NodeId:    req.NodeId,
        ManagerId: "manager-001",
        Status:    "registered",
    }, nil
}

// Heartbeat handles periodic heartbeat from the node.
func (s *nodeService) Heartbeat(ctx context.Context, req *proto.HeartbeatRequest) (*proto.HeartbeatResponse, error) {
    if req.NodeId == "" {
        return nil, status.Error(codes.InvalidArgument, "node_id is required")
    }

    agent := s.grpcServer.getAgent()
    info, err := agent.libvirt.GetHostInfo(context.Background())
    if err != nil {
        log.Printf("[gRPC] Heartbeat: GetHostInfo error: %v", err)
    }

    vmList, err := agent.libvirt.ListVMs(context.Background())
    if err != nil {
        log.Printf("[gRPC] Heartbeat: ListVMs error: %v", err)
    }

    // Convert libvirt VMs to proto VMs
    protoVMs := make([]*proto.VMInfo, 0, len(vmList))
    for _, vm := range vmList {
        state := proto.VMState_ShutOff
        if vm.State == "running" {
            state = proto.VMState_Running
        }
        protoVMs = append(protoVMs, &proto.VMInfo{
            VmId:     vm.ID,
            Name:     vm.Name,
            CpuCount: int32(vm.CPUs),
            Memory:   vm.Memory,
            State:    state,
            Status:   proto.VMStatus_Stopped,
        })
    }

    log.Printf("[gRPC] Heartbeat from %s: cpus=%d mem=%dMB disk=%dGB vms=%d",
        req.NodeId, info.CPU.Count, info.Memory.Total/1024/1024,
        info.Disk.Total/1024/1024/1024, len(vmList))

    return &proto.HeartbeatResponse{
        NodeId:   req.NodeId,
        Accepted: true,
        Message:  "heartbeat accepted",
    }, nil
}

// ExecuteCommand executes a VM lifecycle command on the node.
func (s *nodeService) ExecuteCommand(ctx context.Context, req *proto.ExecuteCommandRequest) (*proto.ExecuteCommandResponse, error) {
    if req.NodeId == "" {
        return nil, status.Error(codes.InvalidArgument, "node_id is required")
    }
    if req.CommandType == proto.CommandType_Unknown {
        return nil, status.Error(codes.InvalidArgument, "command_type is required")
    }
    if req.VmId == "" && req.CommandType < proto.CommandType_StorageCreate {
        return nil, status.Error(codes.InvalidArgument, "vm_id is required for VM commands")
    }

    agent := s.grpcServer.getAgent()
    var result string
    var err error

    switch req.CommandType {
    case proto.CommandType_VMStart:
        err = agent.StartVM(context.Background(), req.VmId)
        result = "VM started"
    case proto.CommandType_VMStop:
        err = agent.StopVM(context.Background(), req.VmId)
        result = "VM stopped"
    case proto.CommandType_VMRestart:
        err = agent.StopVM(context.Background(), req.VmId)
        if err == nil {
            err = agent.StartVM(context.Background(), req.VmId)
        }
        result = "VM restarted"
    case proto.CommandType_VMDestroy:
        err = agent.DestroyVM(context.Background(), req.VmId)
        result = "VM destroyed"
    case proto.CommandType_VMMigrate:
        target := req.Params["target_host"]
        if target == "" {
            err = fmt.Errorf("target_host parameter required for migrate")
        } else {
            err = fmt.Errorf("live migration not yet implemented (migrate to %s)", target)
        }
        result = "migration requested"
    case proto.CommandType_VMSnapshot:
        action := req.Params["action"]
        name := req.Params["name"]
        if action == "create" && name == "" {
            err = fmt.Errorf("name parameter required for snapshot create")
        } else {
            result = fmt.Sprintf("snapshot %s: %s (simulated)", action, name)
        }
    case proto.CommandType_VMResize:
        size := req.Params["size"]
        if size == "" {
            err = fmt.Errorf("size parameter required for resize")
        } else {
            result = fmt.Sprintf("resized to %s bytes (simulated)", size)
        }
    case proto.CommandType_StorageCreate:
        pool := req.Params["pool"]
        name := req.Params["name"]
        size := req.Params["size"]
        result = fmt.Sprintf("storage volume %s created in pool %s (simulated, size=%s)", name, pool, size)
    case proto.CommandType_StorageDelete:
        name := req.Params["name"]
        result = fmt.Sprintf("storage volume %s deleted (simulated)", name)
    case proto.CommandType_NetworkCreate:
        name := req.Params["name"]
        bridge := req.Params["bridge"]
        result = fmt.Sprintf("network %s created on bridge %s (simulated)", name, bridge)
    default:
        err = fmt.Errorf("unsupported command type: %d", req.CommandType)
    }

    if err != nil {
        log.Printf("[gRPC] ExecuteCommand(%d, %s) error: %v", req.CommandType, req.VmId, err)
        return &proto.ExecuteCommandResponse{
            CommandId:   req.CommandId,
            Status:      proto.CommandStatus_Failed,
            Error:       err.Error(),
            CompletedAt: 0,
        }, nil
    }

    log.Printf("[gRPC] ExecuteCommand(%d, %s) OK: %s", req.CommandType, req.VmId, result)

    return &proto.ExecuteCommandResponse{
        CommandId:   req.CommandId,
        Status:      proto.CommandStatus_Success,
        Result:      result,
        CompletedAt: 0,
    }, nil
}

// GetStatus returns the current node status.
func (s *nodeService) GetStatus(ctx context.Context, req *proto.GetStatusRequest) (*proto.GetStatusResponse, error) {
    if req.NodeId == "" {
        return nil, status.Error(codes.InvalidArgument, "node_id is required")
    }

    agent := s.grpcServer.getAgent()
    agent.mu.Lock()
    running := agent.running
    nodeID := agent.nodeID
    hostname := agent.hostname
    agent.mu.Unlock()

    if !running {
        return nil, status.Error(codes.Unavailable, "agent not running")
    }

    info, err := agent.libvirt.GetHostInfo(context.Background())
    if err != nil {
        return nil, status.Errorf(codes.Internal, "GetHostInfo: %v", err)
    }

    vmList, err := agent.libvirt.ListVMs(context.Background())
    if err != nil {
        log.Printf("[gRPC] GetStatus: ListVMs error: %v", err)
    }

    protoVMs := make([]*proto.VMInfo, 0, len(vmList))
    for _, vm := range vmList {
        state := proto.VMState_ShutOff
        if vm.State == "running" {
            state = proto.VMState_Running
        }
        protoVMs = append(protoVMs, &proto.VMInfo{
            VmId:     vm.ID,
            Name:     vm.Name,
            CpuCount: int32(vm.CPUs),
            Memory:   vm.Memory,
            State:    state,
            Status:   proto.VMStatus_Stopped,
        })
    }

    return &proto.GetStatusResponse{
        Status: &proto.NodeStatus{
            NodeId:           nodeID,
            Hostname:         hostname,
            Status:           proto.NodeStatus_Online,
            CpuCount:         int32(info.CPU.Count),
            MemoryTotal:      info.Memory.Total,
            MemoryFree:       info.Memory.Free,
            DiskTotal:        info.Disk.Total,
            DiskFree:         info.Disk.Free,
            Vms:              protoVMs,
            Timestamp:        0,
        },
    }, nil
}

// ListVMs returns all VMs on the node.
func (s *nodeService) ListVMs(ctx context.Context, req *proto.ListVMsRequest) (*proto.ListVMsResponse, error) {
    if req.NodeId == "" {
        return nil, status.Error(codes.InvalidArgument, "node_id is required")
    }

    agent := s.grpcServer.getAgent()
    vms, err := agent.libvirt.ListVMs(context.Background())
    if err != nil {
        return nil, status.Errorf(codes.Internal, "ListVMs: %v", err)
    }

    protoVMs := make([]*proto.VMInfo, 0, len(vms))
    for _, vm := range vms {
        state := proto.VMState_ShutOff
        if vm.State == "running" {
            state = proto.VMState_Running
        }
        protoVMs = append(protoVMs, &proto.VMInfo{
            VmId:     vm.ID,
            Name:     vm.Name,
            CpuCount: int32(vm.CPUs),
            Memory:   vm.Memory,
            State:    state,
            Status:   proto.VMStatus_Stopped,
        })
    }

    return &proto.ListVMsResponse{
        Vms: protoVMs,
    }, nil
}

// Shutdown stops the gRPC server.
func (gs *grpcServer) Shutdown() {
    gs.Stop()
}
