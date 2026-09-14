// Package node implements the HiveStack Node Agent.
//
// The node agent runs on each HiveStack Node (SLES 15 SP7 + KVM/QEMU/libvirt).
// It communicates with the Manager via TLS-secured gRPC, executes VM lifecycle
// commands, reports status, and manages local storage and networking.
//
// Usage:
//
//	HiveStack Node Agent — manages KVM virtualization on a host.
//
//	The node agent:
//	- Registers with the Manager on startup
//	- Reports host status, resource usage, and VM list periodically
//	- Executes VM lifecycle commands from the Manager (create, start, stop, migrate, etc.)
//	- Executes live migration commands
//	- Manages local storage and network configuration
//	- Runs as a systemd service
//
// Configuration:
//
//	The node agent is configured via /etc/hivestack/node.yaml:
//
//	    managerAddress: "hivestack-manager.example.com:8443"
//	    nodeId: "node-1"
//	    tls:
//	      certFile: "/etc/hivestack/tls/node.crt"
//	      keyFile: "/etc/hivestack/tls/node.key"
//	      caFile: "/etc/hivestack/tls/ca.crt"
//	    libvirt:
//	      uri: "qemu:///system"
//	    heartbeatInterval: 30s
//
// Build:
//
//	go build -o bin/hive-node ./node/
//
// Install:
//
//	sudo install bin/hive-node /usr/local/bin/hive-node
//	sudo cp contrib/systemd/hivestack-node.service /etc/systemd/system/
//	sudo systemctl enable --now hivestack-node
package node

import (
    "fmt"
    "os"
)

// Version information
var (
    Version   = "0.1.0"
    GitCommit = "unknown"
)

func main() {
    fmt.Printf("HiveStack Node Agent v%s (%s)\n", Version, GitCommit)
    fmt.Println("Node agent not yet implemented — see internal/node/agent.go")

    // TODO: Implement node agent
    // 1. Load configuration from /etc/hivestack/node.yaml
    // 2. Establish TLS-secured gRPC connection to Manager
    // 3. Register with Manager
    // 4. Start heartbeat loop
    // 5. Start command handler
    // 6. Start status reporter

    os.Exit(1)
}
