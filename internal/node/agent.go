// Package node implements the HiveStack Node Agent.
//
// The node agent runs on each HiveStack Node (SLES 15 SP7 + KVM/QEMU/libvirt).
// It communicates with the Manager via TLS-secured gRPC, executes VM lifecycle
// commands, reports status, and manages local storage and networking.
//
// Build:
//
//	go build -o bin/hive-node ./node/
//
// Run:
//
//	hive-node --config /etc/hivestack/node.yaml
//
// gRPC API:
//
//	The node agent exposes a gRPC server on localhost:9090 by default.
//	The Manager connects to it via TLS. The agent registers with the Manager
//	on startup and receives commands via the gRPC API.
//
//	The gRPC API is defined in proto/node.proto:
//
//	    service NodeAgent {
//	      rpc Register (RegisterRequest) returns (RegisterResponse);
//	      rpc Heartbeat (HeartbeatRequest) returns (HeartbeatResponse);
//	      rpc ExecuteCommand (ExecuteCommandRequest) returns (ExecuteCommandResponse);
//	      rpc GetStatus (GetStatusRequest) returns (GetStatusResponse);
//	      rpc ListVMs (ListVMsRequest) returns (ListVMsResponse);
//	    }
//
// Authentication:
//
//	The node agent authenticates with the Manager using a client certificate.
//	The certificate is provisioned by the Manager during node registration.
package node

import (
    "context"
    "fmt"
    "os"
    "sync"
    "time"

    "github.com/maddydevel/HiveStack/internal/config"
    "github.com/maddydevel/HiveStack/internal/libvirt"
)

// Agent is the HiveStack Node Agent.
type Agent struct {
    mu sync.Mutex

    // config holds the agent configuration.
    config *config.NodeConfig

    // hostname is the host's hostname.
    hostname string

    // nodeID is the node's ID.
    nodeID string

    // libvirt wraps the libvirt connection.
    libvirt *libvirt.Libvirt
}

// New creates a new Node Agent.
func New(cfg *config.NodeConfig) (*Agent, error) {
    hn, _ := os.Hostname()

    a := &Agent{
        config:  cfg,
        hostname: hn,
        nodeID:   cfg.NodeID,
    }

    // Connect to libvirt
    libvirt, err := libvirt.NewLibvirt(cfg.LibvirtURI)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to libvirt: %w", err)
    }
    a.libvirt = libvirt

    return a, nil
}

// Run runs the node agent.
func (a *Agent) Run(ctx context.Context) error {
    // TODO: Connect to Manager via gRPC
    // TODO: Register with Manager
    // TODO: Start status reporter loop
    // TODO: Start command handler loop

    // Block until context is cancelled
    <-ctx.Done()

    return ctx.Err()
}

// startStatusReporter starts the status reporter loop.
func (a *Agent) startStatusReporter(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if err := a.reportStatus(context.Background()); err != nil {
                // log.Printf("Error reporting status: %v", err)
            }
        }
    }
}

// reportStatus reports the agent's status to the Manager.
func (a *Agent) reportStatus(ctx context.Context) error {
    info, err := a.libvirt.GetHostInfo()
    if err != nil {
        return err
    }

    // TODO: Send heartbeat to Manager via gRPC
    _ = info

    return nil
}

// startCommandHandler starts the command handler loop.
func (a *Agent) startCommandHandler(ctx context.Context) {
    // TODO: Listen for commands from Manager via gRPC
    // and execute them on the KVM host.
}

// stop stops the agent.
func (a *Agent) stop() error {
    if a.libvirt != nil {
        a.libvirt.Close()
    }
    return nil
}
