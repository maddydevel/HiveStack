// Package manager provides the HiveStack Manager core services.
//
// The Manager is the central control plane for HiveStack. It:
//   - Authenticates users and enforces RBAC
//   - Manages tenant isolation
//   - Orchestrates VM lifecycle across nodes
//   - Enforces HANA compliance guardrails
//   - Coordinates with Node Agents via gRPC
//   - Serves the REST API for the web UI and CLI
package manager

import (
    "context"
    "fmt"
    "log"
    "sync"

    "github.com/maddydevel/HiveStack/internal/api"
    "github.com/maddydevel/HiveStack/internal/auth"
    "github.com/maddydevel/HiveStack/internal/compliance"
    "github.com/maddydevel/HiveStack/internal/db"
    "github.com/maddydevel/HiveStack/internal/node"
)

// Manager holds the HiveStack Manager state and all its subsystems.
type Manager struct {
    mu         sync.Mutex
    db         *db.DB
    rbac       *auth.RBACEngine
    compliance *compliance.ComplianceStore
    apiServer  *api.APIServer
    nodes      map[string]*node.Agent
    shutdownCh chan struct{}
    wg         sync.WaitGroup
    running    bool
}

// New creates a new HiveStack Manager.
func New(database *db.DB) (*Manager, error) {
    rbac := auth.NewRBACEngine()
    compliance := compliance.NewComplianceStore(database)
    nodes := make(map[string]*node.Agent)

    return &Manager{
        db:         database,
        rbac:       rbac,
        compliance: compliance,
        nodes:      nodes,
        shutdownCh: make(chan struct{}),
    }, nil
}

// Run starts the Manager: initializes the API server and starts the main service loop.
func (m *Manager) Run(ctx context.Context, apiAddr string) error {
    m.mu.Lock()
    if m.running {
        m.mu.Unlock()
        return fmt.Errorf("manager already running")
    }
    m.running = true
    m.mu.Unlock()

    log.Println("HiveStack Manager starting...")

    cfg := &api.Config{
        Server: api.ServerConfig{
            Host:       "0.0.0.0",
            Port:       8080,
            TLSEnabled: false,
        },
        Database: api.DatabaseConfig{
            DSN: "postgres://hivestack:hivestack@localhost:5432/hivestack?sslmode=disable",
        },
        Auth: api.AuthConfig{
            JWTSecret:   "change-me-in-production",
            TokenExpiry: "24h",
        },
        Node: api.NodeConfig{
            GRPCAddress: "hivestack-manager:9090",
        },
    }

    apiServer, err := api.New(cfg, m.db)
    if err != nil {
        return fmt.Errorf("create API server: %w", err)
    }
    m.apiServer = apiServer

    m.wg.Add(1)
    go func() {
        defer m.wg.Done()
        log.Printf("API server listening on %s", apiAddr)
        if err := apiServer.Run(); err != nil && err != context.Canceled {
            log.Printf("API server error: %v", err)
        }
    }()

    log.Println("HiveStack Manager running — API server active, RBAC engine loaded, compliance store ready")

    <-ctx.Done()
    log.Println("Manager: shutdown signal received")
    close(m.shutdownCh)

    m.wg.Wait()
    log.Println("Manager: stopped")
    return nil
}

// RBAC returns the RBAC engine for permission checks.
func (m *Manager) RBAC() *auth.RBACEngine {
    return m.rbac
}

// Compliance returns the compliance store for HANA guardrail operations.
func (m *Manager) Compliance() *compliance.ComplianceStore {
    return m.compliance
}

// DB returns the database connection.
func (m *Manager) DB() *db.DB {
    return m.db
}

// RegisterNode registers a node agent with the manager.
func (m *Manager) RegisterNode(id string, agent *node.Agent) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.nodes[id] = agent
    log.Printf("Node registered: %s", id)
}

// UnregisterNode removes a node agent from the manager.
func (m *Manager) UnregisterNode(id string) {
    m.mu.Lock()
    defer m.mu.Unlock()
    delete(m.nodes, id)
    log.Printf("Node unregistered: %s", id)
}

// ListNodes returns all registered node agents.
func (m *Manager) ListNodes() map[string]*node.Agent {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.nodes
}

// GetNode returns a node agent by ID.
func (m *Manager) GetNode(id string) (*node.Agent, bool) {
    m.mu.Lock()
    defer m.mu.Unlock()
    agent, ok := m.nodes[id]
    return agent, ok
}

// Shutdown gracefully stops the manager.
func (m *Manager) Shutdown() {
    close(m.shutdownCh)
    if m.apiServer != nil {
        m.apiServer.Shutdown()
    }
}
