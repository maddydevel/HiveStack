// Package node implements the HiveStack Node Agent.
//
// The node agent runs on each HiveStack Node (SLES 15 SP7 + KVM/QEMU/libvirt).
// It communicates with the Manager via TLS-secured gRPC, executes VM lifecycle
// commands, reports status, and manages local storage and networking.
//
// Build:
//
//    go build -o bin/hive-node ./node/
//
// Run:
//
//    hive-node --config /etc/hivestack/node.yaml
//
// gRPC API:
//
//    The node agent exposes a gRPC server on localhost:9090 by default.
//    The Manager connects to it via TLS. The agent registers with the Manager
//    on startup and receives commands via the gRPC API.
//
//    The gRPC API is defined in proto/node.proto:
//
//        service NodeAgent {
//          rpc Register (RegisterRequest) returns (RegisterResponse);
//          rpc Heartbeat (HeartbeatRequest) returns (HeartbeatResponse);
//          rpc ExecuteCommand (ExecuteCommandRequest) returns (ExecuteCommandResponse);
//          rpc GetStatus (GetStatusRequest) returns (GetStatusResponse);
//          rpc ListVMs (ListVMsRequest) returns (ListVMsResponse);
//        }
//
// Authentication:
//
//    The node agent authenticates with the Manager using a client certificate.
//    The certificate is provisioned by the Manager during node registration.
package node

import (
    "context"
    "fmt"
    "log"
    "os"
    "sync"
    "time"

    "google.golang.org/grpc"

    "github.com/maddydevel/HiveStack/internal/config"
    "github.com/maddydevel/HiveStack/internal/libvirt"
)

// Agent holds the node agent state and its dependencies.
type Agent struct {
    mu          sync.Mutex
    config      *config.NodeConfig
    hostname    string
    nodeID      string
    libvirt     *libvirt.Libvirt
    grpcServer  *grpcServer
    shutdownCh  chan struct{}
    wg          sync.WaitGroup
    running     bool
}

// New creates a new Node Agent.
func New(cfg *config.NodeConfig) (*Agent, error) {
    hn, _ := os.Hostname()

    a := &Agent{
        config:     cfg,
        hostname:   hn,
        nodeID:     cfg.NodeID,
        shutdownCh: make(chan struct{}),
    }

    // Connect to libvirt
    lv, err := libvirt.NewLibvirt(cfg.LibvirtURI)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to libvirt: %w", err)
    }
    a.libvirt = lv

    return a, nil
}

// Run runs the node agent: starts gRPC server, connects to libvirt, starts all loops, waits for shutdown.
func (a *Agent) Run(ctx context.Context) error {
    a.mu.Lock()
    if a.running {
        a.mu.Unlock()
        return fmt.Errorf("agent already running")
    }
    a.running = true
    a.mu.Unlock()

    // Start gRPC server first (before libvirt connection so Manager can reach us)
    if a.config.GRPCAddress != "" {
        var srvOpts []grpc.ServerOption
        tlsOpt, err := ServerTLSOption(a.config.TLSConfig)
        if err != nil {
            return fmt.Errorf("configure gRPC TLS: %w", err)
        }
        if tlsOpt != nil {
            srvOpts = append(srvOpts, tlsOpt)
        } else {
            log.Printf("[gRPC] WARNING: no TLS configured; serving node agent gRPC in plaintext without client authentication")
        }
        grpcSrv, err := NewGRPCServer(a, a.config.GRPCAddress, srvOpts...)
        if err != nil {
            return fmt.Errorf("create gRPC server: %w", err)
        }
        a.grpcServer = grpcSrv
        if err := grpcSrv.Start(ctx); err != nil {
            return fmt.Errorf("start gRPC server: %w", err)
        }
        log.Printf("[gRPC] Node agent gRPC server started on %s", a.config.GRPCAddress)
    }

    // Connect to libvirt
    if err := a.libvirt.Connect(); err != nil {
        return fmt.Errorf("connect to libvirt: %w", err)
    }
    log.Println("Node agent: connected to libvirt")

    // Start status reporter loop
    a.wg.Add(1)
    go a.startStatusReporter(ctx)

    // Start command handler loop
    a.wg.Add(1)
    go a.startCommandHandler(ctx)

    log.Printf("Node agent running — NodeID=%s, Host=%s, Manager=%s",
        a.nodeID, a.hostname, a.config.ManagerAddress)

    // Block until context is cancelled
    <-ctx.Done()
    log.Println("Node agent: shutdown signal received")

    // Signal all goroutines to stop
    close(a.shutdownCh)

    // Wait for goroutines to finish
    a.wg.Wait()

    // Clean up
    if err := a.libvirt.Close(); err != nil {
        log.Printf("Warning: error closing libvirt: %v", err)
    }

    log.Println("Node agent: stopped")
    return nil
}

// startStatusReporter periodically reports the agent's status to the Manager.
func (a *Agent) startStatusReporter(ctx context.Context) {
    defer a.wg.Done()
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-a.shutdownCh:
            return
        case <-ticker.C:
            if err := a.reportStatus(context.Background()); err != nil {
                log.Printf("Error reporting status: %v", err)
            }
        }
    }
}

// reportStatus collects and reports the agent's status.
func (a *Agent) reportStatus(ctx context.Context) error {
    info, err := a.libvirt.GetHostInfo(ctx)
    if err != nil {
        return err
    }

    log.Printf("[status] Node %s: host=%s cpus=%d memory=%dMB numa=%d hugepages=%dMB storage=%dGB",
        a.nodeID, info.Hostname, info.CPU.Count,
        info.Memory.Total/1024/1024, info.NUMANodeCount,
        info.HugepagesTotalMB, info.StorageTotalGB)

    // In production: send heartbeat to Manager via gRPC
    // Include: node ID, host info, VM count, storage status
    return nil
}

// startCommandHandler listens for commands from the Manager.
func (a *Agent) startCommandHandler(ctx context.Context) {
    defer a.wg.Done()
    for {
        select {
        case <-ctx.Done():
            return
        case <-a.shutdownCh:
            return
        default:
            // In production: gRPC server would receive commands here
            // For now: simulated command polling
            time.Sleep(5 * time.Second)
        }
    }
}

// CreateStoragePool creates a storage pool on the node.
func (a *Agent) CreateStoragePool(ctx context.Context, id, name, poolType, path string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.running {
		return fmt.Errorf("agent not running")
	}
	return a.libvirt.CreateStoragePool(ctx, id, name, poolType, path)
}

// DeleteStoragePool deletes a storage pool on the node.
func (a *Agent) DeleteStoragePool(ctx context.Context, id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.running {
		return fmt.Errorf("agent not running")
	}
	return a.libvirt.DeleteStoragePool(ctx, id)
}

// HostInfo returns the current host information from libvirt.
func (a *Agent) HostInfo(ctx context.Context) (*libvirt.HostInfo, error) {
    a.mu.Lock()
    defer a.mu.Unlock()
    if !a.running {
        return nil, fmt.Errorf("agent not running")
    }
    return a.libvirt.GetHostInfo(ctx)
}

// ListVMs returns all VMs managed by this node.
func (a *Agent) ListVMs(ctx context.Context) ([]libvirt.VMInfo, error) {
    a.mu.Lock()
    defer a.mu.Unlock()
    if !a.running {
        return nil, fmt.Errorf("agent not running")
    }
    return a.libvirt.ListVMs(ctx)
}

// StartVM starts a VM by ID.
func (a *Agent) StartVM(ctx context.Context, id string) error {
    a.mu.Lock()
    defer a.mu.Unlock()
    if !a.running {
        return fmt.Errorf("agent not running")
    }
    return a.libvirt.StartVM(ctx, id)
}

// StopVM stops a VM by ID.
func (a *Agent) StopVM(ctx context.Context, id string) error {
    a.mu.Lock()
    defer a.mu.Unlock()
    if !a.running {
        return fmt.Errorf("agent not running")
    }
    return a.libvirt.StopVM(ctx, id)
}

// DestroyVM destroys a VM by ID.
func (a *Agent) DestroyVM(ctx context.Context, id string) error {
    a.mu.Lock()
    defer a.mu.Unlock()
    if !a.running {
        return fmt.Errorf("agent not running")
    }
    return a.libvirt.DestroyVM(ctx, id)
}

// Shutdown gracefully stops the agent.
func (a *Agent) Shutdown() {
    close(a.shutdownCh)
    a.libvirt.Close()
}
