package proto

import (
    "context"
    "encoding/json"

    "google.golang.org/grpc"
    "google.golang.org/grpc/encoding"
)

// CodecName is the content-subtype of the JSON codec ("application/grpc+json").
const CodecName = "json"

// jsonCodec marshals the hand-written message structs as JSON. The structs are
// not generated protobuf messages, so the default proto codec cannot handle
// them. This is a stopgap until the service moves to real protobuf.
type jsonCodec struct{}

func (jsonCodec) Marshal(v interface{}) ([]byte, error)      { return json.Marshal(v) }
func (jsonCodec) Unmarshal(data []byte, v interface{}) error { return json.Unmarshal(data, v) }
func (jsonCodec) Name() string                               { return CodecName }

// JSONCodec returns the codec used on both sides of the NodeAgent service.
func JSONCodec() encoding.Codec { return jsonCodec{} }

func init() {
    // Registered under its own name, so the default "proto" codec is untouched.
    encoding.RegisterCodec(JSONCodec())
}

// NodeAgentClient is the client API for the NodeAgent service.
type NodeAgentClient interface {
    Register(ctx context.Context, in *RegisterRequest, opts ...grpc.CallOption) (*RegisterResponse, error)
    Heartbeat(ctx context.Context, in *HeartbeatRequest, opts ...grpc.CallOption) (*HeartbeatResponse, error)
    ExecuteCommand(ctx context.Context, in *ExecuteCommandRequest, opts ...grpc.CallOption) (*ExecuteCommandResponse, error)
    GetStatus(ctx context.Context, in *GetStatusRequest, opts ...grpc.CallOption) (*GetStatusResponse, error)
    ListVMs(ctx context.Context, in *ListVMsRequest, opts ...grpc.CallOption) (*ListVMsResponse, error)
}

type nodeAgentClient struct {
    cc grpc.ClientConnInterface
}

// NewNodeAgentClient returns a NodeAgentClient that calls the service over cc
// using the JSON codec.
func NewNodeAgentClient(cc grpc.ClientConnInterface) NodeAgentClient {
    return &nodeAgentClient{cc: cc}
}

// invoke calls the named NodeAgent method, selecting the JSON codec first so
// callers cannot override it by accident.
func (c *nodeAgentClient) invoke(ctx context.Context, method string, in, out interface{}, opts []grpc.CallOption) error {
    opts = append(opts[:len(opts):len(opts)], grpc.CallContentSubtype(CodecName))
    return c.cc.Invoke(ctx, "/"+NodeAgentServiceDesc.ServiceName+"/"+method, in, out, opts...)
}

func (c *nodeAgentClient) Register(ctx context.Context, in *RegisterRequest, opts ...grpc.CallOption) (*RegisterResponse, error) {
    out := new(RegisterResponse)
    if err := c.invoke(ctx, "Register", in, out, opts); err != nil {
        return nil, err
    }
    return out, nil
}

func (c *nodeAgentClient) Heartbeat(ctx context.Context, in *HeartbeatRequest, opts ...grpc.CallOption) (*HeartbeatResponse, error) {
    out := new(HeartbeatResponse)
    if err := c.invoke(ctx, "Heartbeat", in, out, opts); err != nil {
        return nil, err
    }
    return out, nil
}

func (c *nodeAgentClient) ExecuteCommand(ctx context.Context, in *ExecuteCommandRequest, opts ...grpc.CallOption) (*ExecuteCommandResponse, error) {
    out := new(ExecuteCommandResponse)
    if err := c.invoke(ctx, "ExecuteCommand", in, out, opts); err != nil {
        return nil, err
    }
    return out, nil
}

func (c *nodeAgentClient) GetStatus(ctx context.Context, in *GetStatusRequest, opts ...grpc.CallOption) (*GetStatusResponse, error) {
    out := new(GetStatusResponse)
    if err := c.invoke(ctx, "GetStatus", in, out, opts); err != nil {
        return nil, err
    }
    return out, nil
}

func (c *nodeAgentClient) ListVMs(ctx context.Context, in *ListVMsRequest, opts ...grpc.CallOption) (*ListVMsResponse, error) {
    out := new(ListVMsResponse)
    if err := c.invoke(ctx, "ListVMs", in, out, opts); err != nil {
        return nil, err
    }
    return out, nil
}
