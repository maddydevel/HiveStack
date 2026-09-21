// Package main implements the HiveStack Node Agent entry point.
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
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/maddydevel/HiveStack/internal/config"
	hiveNode "github.com/maddydevel/HiveStack/internal/node"
)

func main() {
	cfgPath := flag.String("config", "/etc/hivestack/node.yaml", "path to node config file")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("HiveStack Node Agent v0.1.0 (development build)\n")
		os.Exit(0)
	}

	log.Printf("HiveStack Node Agent starting (config: %s)", *cfgPath)

	// Load configuration
	cfg, err := config.LoadNodeConfig(*cfgPath)
	if err != nil {
		log.Printf("Warning: could not load config from %s: %v — using defaults", *cfgPath, err)
		cfg = config.DefaultNodeConfig()
	}

	log.Printf("Node ID: %s", cfg.NodeID)
	log.Printf("Manager address: %s", cfg.ManagerAddress)
	log.Printf("Libvirt URI: %s", cfg.LibvirtURI)

	// Create agent
	agent, err := hiveNode.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create node agent: %v", err)
	}

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v, shutting down...", sig)
		cancel()
	}()

	// Run agent
	if err := agent.Run(ctx); err != nil && err != context.Canceled {
		log.Fatalf("Node agent error: %v", err)
	}

	log.Println("Node agent stopped")
}
