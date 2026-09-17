// Package proto contains generated gRPC types for the HiveStack Node Agent.
//
// These are hand-written stubs matching the proto/node.proto service definition.
// In production, generate from .proto files with protoc-gen-go.
//
// gRPC service registration is provided via UnimplementedNodeAgentServer and
// RegisterNodeAgentServer — mimicking what protoc-gen-go generates.
package proto

import (
    "context"
    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)

// NodeAgentServer is the gRPC service interface for the Node Agent.
type NodeAgentServer interface {
    Register(context.Context, *RegisterRequest) (*RegisterResponse, error)
    Heartbeat(context.Context, *HeartbeatRequest) (*HeartbeatResponse, error)
    ExecuteCommand(context.Context, *ExecuteCommandRequest) (*ExecuteCommandResponse, error)
    GetStatus(context.Context, *GetStatusRequest) (*GetStatusResponse, error)
    ListVMs(context.Context, *ListVMsRequest) (*ListVMsResponse, error)
    mustEmbedUnimplementedNodeAgentServer()
}

// UnimplementedNodeAgentServer embeds into server structs to get default
// implementations for all service methods (returns unimplemented status).
type UnimplementedNodeAgentServer struct{}

func (UnimplementedNodeAgentServer) Register(context.Context, *RegisterRequest) (*RegisterResponse, error) {
    return nil, statusError(codes.Unimplemented, "Register not implemented")
}
func (UnimplementedNodeAgentServer) Heartbeat(context.Context, *HeartbeatRequest) (*HeartbeatResponse, error) {
    return nil, statusError(codes.Unimplemented, "Heartbeat not implemented")
}
func (UnimplementedNodeAgentServer) ExecuteCommand(context.Context, *ExecuteCommandRequest) (*ExecuteCommandResponse, error) {
    return nil, statusError(codes.Unimplemented, "ExecuteCommand not implemented")
}
func (UnimplementedNodeAgentServer) GetStatus(context.Context, *GetStatusRequest) (*GetStatusResponse, error) {
    return nil, statusError(codes.Unimplemented, "GetStatus not implemented")
}
func (UnimplementedNodeAgentServer) ListVMs(context.Context, *ListVMsRequest) (*ListVMsResponse, error) {
    return nil, statusError(codes.Unimplemented, "ListVMs not implemented")
}
func (UnimplementedNodeAgentServer) mustEmbedUnimplementedNodeAgentServer() {}

// RegisterNodeAgentServer registers a NodeAgentServer with a gRPC server.
func RegisterNodeAgentServer(s *grpc.Server, srv NodeAgentServer) {
    s.RegisterService(&NodeAgentServiceDesc, srv)
}

// NodeAgentServiceDesc is the gRPC service descriptor for NodeAgent.
var NodeAgentServiceDesc = grpc.ServiceDesc{
    ServiceName: "proto.NodeAgent",
    HandlerType: (*NodeAgentServer)(nil),
    Methods: []grpc.MethodDesc{
        {MethodName: "Register", Handler: registerHandler},
        {MethodName: "Heartbeat", Handler: heartbeatHandler},
        {MethodName: "ExecuteCommand", Handler: executeCommandHandler},
        {MethodName: "GetStatus", Handler: getStatusHandler},
        {MethodName: "ListVMs", Handler: listVMsHandler},
    },
    Streams:  []grpc.StreamDesc{},
    Metadata: "proto/node.proto",
}

// ─── Handler stubs ────────────────────────────────────────────────────────────
// These match the grpc.methodHandler signature used in ServiceDesc.Methods.
// gRPC unmarshals the request into `req` via `dec`, then calls the handler.
// If an interceptor is registered, gRPC wraps the handler chain automatically.

func registerHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
    req := &RegisterRequest{}
    if err := dec(req); err != nil {
        return nil, err
    }
    return srv.(NodeAgentServer).Register(ctx, req)
}

func heartbeatHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
    req := &HeartbeatRequest{}
    if err := dec(req); err != nil {
        return nil, err
    }
    return srv.(NodeAgentServer).Heartbeat(ctx, req)
}

func executeCommandHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
    req := &ExecuteCommandRequest{}
    if err := dec(req); err != nil {
        return nil, err
    }
    return srv.(NodeAgentServer).ExecuteCommand(ctx, req)
}

func getStatusHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
    req := &GetStatusRequest{}
    if err := dec(req); err != nil {
        return nil, err
    }
    return srv.(NodeAgentServer).GetStatus(ctx, req)
}

func listVMsHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
    req := &ListVMsRequest{}
    if err := dec(req); err != nil {
        return nil, err
    }
    return srv.(NodeAgentServer).ListVMs(ctx, req)
}

// statusError creates a gRPC status error.
func statusError(c codes.Code, msg string) error {
    return status.Errorf(c, "%s", msg)
}

// ─── Data types ────────────────────────────────────────────────────────────────

// RegisterRequest is the registration request from a node agent.
type RegisterRequest struct {
    NodeId       string   `protobuf:"bytes,1,opt,name=node_id" json:"node_id,omitempty"`
    Hostname     string   `protobuf:"bytes,2,opt,name=hostname" json:"hostname,omitempty"`
    NodeVersion  string   `protobuf:"bytes,3,opt,name=node_version" json:"node_version,omitempty"`
    NodeAgent    string   `protobuf:"bytes,4,opt,name=node_agent" json:"node_agent,omitempty"`
    Capabilities []string `protobuf:"bytes,5,rep,name=capabilities" json:"capabilities,omitempty"`
}

// RegisterResponse is the registration response from the manager.
type RegisterResponse struct {
    NodeId    string `protobuf:"bytes,1,opt,name=node_id" json:"node_id,omitempty"`
    ManagerId string `protobuf:"bytes,2,opt,name=manager_id" json:"manager_id,omitempty"`
    Status    string `protobuf:"bytes,3,opt,name=status" json:"status,omitempty"`
}

// HeartbeatRequest is the periodic heartbeat from a node.
type HeartbeatRequest struct {
    NodeId string      `protobuf:"bytes,1,opt,name=node_id" json:"node_id,omitempty"`
    Status *NodeStatus `protobuf:"bytes,2,opt,name=status" json:"status,omitempty"`
    Since  int64       `protobuf:"varint,3,opt,name=since" json:"since,omitempty"`
}

// HeartbeatResponse is the heartbeat acknowledgement.
type HeartbeatResponse struct {
    NodeId    string `protobuf:"bytes,1,opt,name=node_id" json:"node_id,omitempty"`
    Accepted  bool   `protobuf:"varint,2,opt,name=accepted" json:"accepted,omitempty"`
    Message   string `protobuf:"bytes,3,opt,name=message" json:"message,omitempty"`
}

// NodeStatus holds the current node status.
type NodeStatus struct {
    NodeId           string        `protobuf:"bytes,1,opt,name=node_id" json:"node_id,omitempty"`
    Hostname         string        `protobuf:"bytes,2,opt,name=hostname" json:"hostname,omitempty"`
    Status           NodeStatusType `protobuf:"varint,3,opt,name=status,enum=NodeStatusType" json:"status,omitempty"`
    CpuCount         int32         `protobuf:"varint,4,opt,name=cpu_count" json:"cpu_count,omitempty"`
    MemoryTotal      uint64        `protobuf:"varint,5,opt,name=memory_total" json:"memory_total,omitempty"`
    MemoryFree       uint64        `protobuf:"varint,6,opt,name=memory_free" json:"memory_free,omitempty"`
    DiskTotal        uint64        `protobuf:"varint,7,opt,name=disk_total" json:"disk_total,omitempty"`
    DiskFree         uint64        `protobuf:"varint,8,opt,name=disk_free" json:"disk_free,omitempty"`
    Vms              []*VMInfo     `protobuf:"bytes,9,rep,name=vms" json:"vms,omitempty"`
    Timestamp        int64         `protobuf:"varint,10,opt,name=timestamp" json:"timestamp,omitempty"`
}

// NodeStatusType is the node status enum.
type NodeStatusType int32

const (
    NodeStatus_Unknown     NodeStatusType = 0
    NodeStatus_Online      NodeStatusType = 1
    NodeStatus_Offline     NodeStatusType = 2
    NodeStatus_Maintenance NodeStatusType = 3
    NodeStatus_Draining    NodeStatusType = 4
)

// VMInfo holds VM information from the node.
type VMInfo struct {
    VmId     string      `protobuf:"bytes,1,opt,name=vm_id" json:"vm_id,omitempty"`
    Name     string      `protobuf:"bytes,2,opt,name=name" json:"name,omitempty"`
    CpuCount int32       `protobuf:"varint,3,opt,name=cpu_count" json:"cpu_count,omitempty"`
    Memory   uint64      `protobuf:"varint,4,opt,name=memory" json:"memory,omitempty"`
    State    VMState     `protobuf:"varint,5,opt,name=state,enum=VMState" json:"state,omitempty"`
    Status   VMStatusType `protobuf:"varint,6,opt,name=status,enum=VMStatusType" json:"status,omitempty"`
}

// VMState is the VM power state.
type VMState int32

const (
    VMState_Unknown VMState = 0
    VMState_Running VMState = 1
    VMState_Paused  VMState = 2
    VMState_Static  VMState = 3
    VMState_ShutOff VMState = 4
    VMState_Other   VMState = 5
)

// VMStatusType is the VM lifecycle status.
type VMStatusType int32

const (
    VMStatus_Unknown      VMStatusType = 0
    VMStatus_Creating     VMStatusType = 1
    VMStatus_Running      VMStatusType = 2
    VMStatus_Paused       VMStatusType = 3
    VMStatus_StopPending  VMStatusType = 4
    VMStatus_Stopped      VMStatusType = 5
    VMStatus_Migrating    VMStatusType = 6
    VMStatus_Snapshotting VMStatusType = 7
)

// ExecuteCommandRequest is a command from the manager to the node.
type ExecuteCommandRequest struct {
    NodeId      string            `protobuf:"bytes,1,opt,name=node_id" json:"node_id,omitempty"`
    CommandId   string            `protobuf:"bytes,2,opt,name=command_id" json:"command_id,omitempty"`
    CommandType CommandType       `protobuf:"varint,3,opt,name=command_type,enum=CommandType" json:"command_type,omitempty"`
    VmId        string            `protobuf:"bytes,4,opt,name=vm_id" json:"vm_id,omitempty"`
    Params      map[string]string `protobuf:"bytes,5,rep,name=params" json:"params,omitempty"`
    CreatedAt   int64             `protobuf:"varint,6,opt,name=created_at" json:"created_at,omitempty"`
}

// ExecuteCommandResponse is the result of a command execution.
type ExecuteCommandResponse struct {
    CommandId   string        `protobuf:"bytes,1,opt,name=command_id" json:"command_id,omitempty"`
    Status      CommandStatus `protobuf:"varint,2,opt,name=status,enum=CommandStatus" json:"status,omitempty"`
    Result      string        `protobuf:"bytes,3,opt,name=result" json:"result,omitempty"`
    Error       string        `protobuf:"bytes,4,opt,name=error" json:"error,omitempty"`
    CompletedAt int64         `protobuf:"varint,5,opt,name=completed_at" json:"completed_at,omitempty"`
}

// CommandType is the type of command.
type CommandType int32

const (
    CommandType_Unknown        CommandType = 0
    CommandType_VMCreate       CommandType = 1
    CommandType_VMStart        CommandType = 2
    CommandType_VMStop         CommandType = 3
    CommandType_VMRestart      CommandType = 4
    CommandType_VMMigrate      CommandType = 5
    CommandType_VMSnapshot     CommandType = 6
    CommandType_VMDestroy      CommandType = 7
    CommandType_VMResize       CommandType = 8
    CommandType_StorageCreate  CommandType = 100
    CommandType_StorageDelete  CommandType = 101
    CommandType_NetworkCreate  CommandType = 200
    CommandType_NetworkDelete  CommandType = 201
)

// CommandStatus is the command execution status.
type CommandStatus int32

const (
    CommandStatus_Pending   CommandStatus = 0
    CommandStatus_Running   CommandStatus = 1
    CommandStatus_Success   CommandStatus = 2
    CommandStatus_Failed    CommandStatus = 3
    CommandStatus_Cancelled CommandStatus = 4
    CommandStatus_Timeout   CommandStatus = 5
)

// GetStatusRequest requests the current node status.
type GetStatusRequest struct {
    NodeId string `protobuf:"bytes,1,opt,name=node_id" json:"node_id,omitempty"`
}

// GetStatusResponse returns the current node status.
type GetStatusResponse struct {
    Status *NodeStatus `protobuf:"bytes,1,opt,name=status" json:"status,omitempty"`
}

// ListVMsRequest requests the list of VMs on a node.
type ListVMsRequest struct {
    NodeId string `protobuf:"bytes,1,opt,name=node_id" json:"node_id,omitempty"`
}

// ListVMsResponse returns the list of VMs.
type ListVMsResponse struct {
    Vms []*VMInfo `protobuf:"bytes,1,rep,name=vms" json:"vms,omitempty"`
}
