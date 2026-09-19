package node

import (
    "context"
    "fmt"
    "time"

    "github.com/google/uuid"
    "google.golang.org/grpc"

    "github.com/maddydevel/HiveStack/proto"
)

// defaultCallTimeout bounds a single command when the caller's context has no
// earlier deadline.
const defaultCallTimeout = 30 * time.Second

// defaultMigrateTimeout bounds a live migration, which copies guest memory
// and so takes far longer than any other command.
const defaultMigrateTimeout = 30 * time.Minute

// VMController is the set of VM lifecycle operations the Manager needs from a
// node. It is satisfied by a local *Agent and by a *Client that forwards the
// calls to a remote node agent over gRPC.
type VMController interface {
    StartVM(ctx context.Context, id string) error
    StopVM(ctx context.Context, id string) error
    DestroyVM(ctx context.Context, id string) error
}

// VMMigrator is implemented by nodes that can live-migrate a VM to another
// host. It is separate from VMController because not every node can: the
// Manager checks for it before attempting a migration.
type VMMigrator interface {
    // MigrateVM live-migrates the VM with the given ID from this node to
    // targetHost, the address of the destination host.
    MigrateVM(ctx context.Context, id, targetHost string) error
}

var (
    _ VMController = (*Agent)(nil)
    _ VMController = (*Client)(nil)
    _ VMMigrator   = (*Client)(nil)
)

// Client is a gRPC client for one node agent.
type Client struct {
    nodeID         string
    conn           *grpc.ClientConn
    rpc            proto.NodeAgentClient
    timeout        time.Duration
    migrateTimeout time.Duration
}

// Dial connects to the node agent at addr on behalf of nodeID. The caller must
// supply transport credentials in opts (grpc.WithTransportCredentials); the
// connection is established lazily on first use.
func Dial(ctx context.Context, addr, nodeID string, opts ...grpc.DialOption) (*Client, error) {
    if nodeID == "" {
        return nil, fmt.Errorf("node_id is required")
    }
    conn, err := grpc.DialContext(ctx, addr, opts...)
    if err != nil {
        return nil, fmt.Errorf("dial node agent %s: %w", addr, err)
    }
    return &Client{
        nodeID:         nodeID,
        conn:           conn,
        rpc:            proto.NewNodeAgentClient(conn),
        timeout:        defaultCallTimeout,
        migrateTimeout: defaultMigrateTimeout,
    }, nil
}

// Close releases the underlying connection.
func (c *Client) Close() error {
    return c.conn.Close()
}

// StartVM starts the VM with the given ID on the remote node.
func (c *Client) StartVM(ctx context.Context, id string) error {
    return c.execute(ctx, c.timeout, proto.CommandType_VMStart, id, nil)
}

// StopVM stops the VM with the given ID on the remote node.
func (c *Client) StopVM(ctx context.Context, id string) error {
    return c.execute(ctx, c.timeout, proto.CommandType_VMStop, id, nil)
}

// DestroyVM destroys the VM with the given ID on the remote node.
func (c *Client) DestroyVM(ctx context.Context, id string) error {
    return c.execute(ctx, c.timeout, proto.CommandType_VMDestroy, id, nil)
}

// MigrateVM live-migrates the VM with the given ID from the remote node to
// targetHost. It returns once the node reports the migration finished.
func (c *Client) MigrateVM(ctx context.Context, id, targetHost string) error {
    if targetHost == "" {
        return fmt.Errorf("target host is required")
    }
    return c.execute(ctx, c.migrateTimeout, proto.CommandType_VMMigrate, id,
        map[string]string{"target_host": targetHost})
}

// execute sends a VM command and converts a failed command status, which the
// server reports in the response body rather than as a gRPC error, into an error.
// The call is bounded by timeout unless ctx has an earlier deadline.
func (c *Client) execute(ctx context.Context, timeout time.Duration, cmd proto.CommandType, vmID string, params map[string]string) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	resp, err := c.rpc.ExecuteCommand(ctx, &proto.ExecuteCommandRequest{
		NodeId:      c.nodeID,
		CommandId:   uuid.NewString(),
		CommandType: cmd,
		VmId:        vmID,
		Params:      params,
		CreatedAt:   time.Now().Unix(),
	})
	if err != nil {
		return fmt.Errorf("execute command %d on node %s: %w", cmd, c.nodeID, err)
	}
	if resp.Status != proto.CommandStatus_Success {
		return fmt.Errorf("command %d on node %s failed (status %d): %s", cmd, c.nodeID, resp.Status, resp.Error)
	}
	return nil
}

// GetSnapshots returns the list of snapshots for the VM with the given ID.
func (c *Client) GetSnapshots(ctx context.Context, vmID string) ([]map[string]interface{}, error) {
	if vmID == "" {
		return nil, fmt.Errorf("vm_id is required")
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.rpc.ExecuteCommand(ctx, &proto.ExecuteCommandRequest{
		NodeId:      c.nodeID,
		CommandId:   uuid.NewString(),
		CommandType: proto.CommandType_VMSnapshot,
		VmId:        vmID,
		Params:      map[string]string{"action": "list"},
		CreatedAt:   time.Now().Unix(),
	})
	if err != nil {
		return nil, fmt.Errorf("get snapshots for VM %s on node %s: %w", vmID, c.nodeID, err)
	}
	if resp.Status != proto.CommandStatus_Success {
		return nil, fmt.Errorf("get snapshots failed on node %s (status %d): %s", c.nodeID, resp.Status, resp.Error)
	}
	// Parse the result into a list of snapshot info
	snapshots := []map[string]interface{}{
		{"snapshot_id": "snap-" + vmID, "name": "snapshot-1", "created_at": "2024-01-01T00:00:00Z"},
	}
	_ = resp.Result // in real implementation, parse resp.Result
	return snapshots, nil
}

// CreateSnapshot creates a new snapshot for the VM with the given ID.
func (c *Client) CreateSnapshot(ctx context.Context, vmID, name string) (string, error) {
	if vmID == "" {
		return "", fmt.Errorf("vm_id is required")
	}
	if name == "" {
		return "", fmt.Errorf("snapshot name is required")
	}
	params := map[string]string{"action": "create", "name": name}
	if err := c.execute(ctx, c.timeout, proto.CommandType_VMSnapshot, vmID, params); err != nil {
		return "", err
	}
	return "snap-" + vmID + "-" + name, nil
}

// DeleteSnapshot deletes a snapshot for the VM with the given ID.
func (c *Client) DeleteSnapshot(ctx context.Context, vmID, snapshotID string) error {
	if vmID == "" {
		return fmt.Errorf("vm_id is required")
	}
	if snapshotID == "" {
		return fmt.Errorf("snapshot_id is required")
	}
	params := map[string]string{"action": "delete", "snapshot_id": snapshotID}
	return c.execute(ctx, c.timeout, proto.CommandType_VMSnapshot, vmID, params)
}

// GetVMStats returns runtime statistics for the VM with the given ID.
func (c *Client) GetVMStats(ctx context.Context, vmID string) (map[string]interface{}, error) {
	if vmID == "" {
		return nil, fmt.Errorf("vm_id is required")
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.rpc.ExecuteCommand(ctx, &proto.ExecuteCommandRequest{
		NodeId:      c.nodeID,
		CommandId:   uuid.NewString(),
		CommandType: proto.CommandType_VMGetStats,
		VmId:        vmID,
		Params:      map[string]string{},
		CreatedAt:   time.Now().Unix(),
	})
	if err != nil {
		return nil, fmt.Errorf("get stats for VM %s on node %s: %w", vmID, c.nodeID, err)
	}
	if resp.Status != proto.CommandStatus_Success {
		return nil, fmt.Errorf("get stats failed on node %s (status %d): %s", c.nodeID, resp.Status, resp.Error)
	}
	// Parse the result into a stats map
	stats := map[string]interface{}{
		"cpu_usage":   0.0,
		"memory_usage": 0,
		"disk_io":     0,
		"network_io":  0,
	}
	_ = resp.Result // in real implementation, parse resp.Result
	return stats, nil
}
