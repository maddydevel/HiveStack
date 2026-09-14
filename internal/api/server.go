// Package api implements the HiveStack REST API server.
//
// The API server provides the REST API for HiveStack Manager.
// It handles authentication, RBAC, inventory management, VM lifecycle,
// storage, networking, backups, and migration operations.
//
// Usage:
//
//	HiveStack REST API Server — management API for HiveStack.
//
//	The API server:
//	- Listens on :8080 (HTTP) or :8443 (HTTPS) by default
//	- Serves OpenAPI documentation at /api/v1/
//	- Handles authentication via JWT tokens
//	- Enforces RBAC on all endpoints
//	- Routes requests to appropriate services
//	- Communicates with Node agents via gRPC
//
// Configuration:
//
//	The API server is configured via config.yaml:
//
//	    server:
//	      host: "0.0.0.0"
//	      port: 8080
//	      tls:
//	        enabled: false
//	        certFile: "/etc/hivestack/tls/server.crt"
//	        keyFile: "/etc/hivestack/tls/server.key"
//	    database:
//	      dsn: "postgres://hivestack:***@localhost:5432/hivestack?sslmode=disable"
//	    auth:
//	      jwtSecret: "your-jwt-secret-here"
//	      tokenExpiry: 24h
//	    node:
//	      grpcAddress: "hivestack-manager.example.com:9090"
//
// Build:
//
//	go build -o bin/hive-api ./api/
//
// Run:
//
//	./bin/hive-api --config config.yaml
//
// Endpoints:
//
//	POST   /api/v1/auth/login              - Log in
//	POST   /api/v1/auth/logout             - Log out
//	GET    /api/v1/auth/me                 - Current user
//	GET    /api/v1/users                   - List users
//	POST   /api/v1/users                   - Create user
//	GET    /api/v1/users/{id}              - Get user
//	PUT    /api/v1/users/{id}              - Update user
//	DELETE /api/v1/users/{id}              - Delete user
//	GET    /api/v1/datacenters             - List datacenters
//	POST   /api/v1/datacenters             - Create datacenter
//	GET    /api/v1/datacenters/{id}        - Get datacenter
//	DELETE /api/v1/datacenters/{id}        - Delete datacenter
//	GET    /api/v1/clusters                - List clusters
//	POST   /api/v1/clusters                - Create cluster
//	GET    /api/v1/clusters/{id}           - Get cluster
//	GET    /api/v1/clusters/{id}/hosts     - List hosts in cluster
//	GET    /api/v1/hosts                   - List hosts
//	POST   /api/v1/hosts                   - Register host
//	GET    /api/v1/hosts/{id}              - Get host
//	PUT    /api/v1/hosts/{id}              - Update host
//	DELETE /api/v1/hosts/{id}              - Remove host
//	GET    /api/v1/hosts/{id}/status       - Host status
//	POST   /api/v1/hosts/{id}/maintenance  - Enter maintenance
//	DELETE /api/v1/hosts/{id}/maintenance  - Exit maintenance
//	GET    /api/v1/vms                     - List VMs
//	POST   /api/v1/vms                     - Create VM
//	GET    /api/v1/vms/{id}                - Get VM
//	PUT    /api/v1/vms/{id}                - Update VM
//	DELETE /api/v1/vms/{id}                - Delete VM
//	POST   /api/v1/vms/{id}/start          - Start VM
//	POST   /api/v1/vms/{id}/stop           - Stop VM
//	POST   /api/v1/vms/{id}/restart        - Restart VM
//	POST   /api/v1/vms/{id}/migrate        - Migrate VM
//	GET    /api/v1/vms/{id}/snapshots      - List snapshots
//	POST   /api/v1/vms/{id}/snapshots      - Create snapshot
//	DELETE /api/v1/vms/{id}/snapshots/{sid} - Delete snapshot
//	GET    /api/v1/vms/{id}/console        - Console URL
//	GET    /api/v1/vms/{id}/stats          - VM statistics
//	GET    /api/v1/storage-pools           - List storage pools
//	POST   /api/v1/storage-pools           - Create storage pool
//	GET    /api/v1/storage-pools/{id}      - Get storage pool
//	DELETE /api/v1/storage-pools/{id}      - Delete storage pool
//	GET    /api/v1/storage-pools/{id}/disks - List disks
//	GET    /api/v1/disks/{diskId}          - Get disk
//	PUT    /api/v1/disks/{diskId}          - Resize disk
//	GET    /api/v1/networks                - List networks
//	POST   /api/v1/networks                - Create network
//	GET    /api/v1/networks/{id}           - Get network
//	DELETE /api/v1/networks/{id}           - Delete network
//	GET    /api/v1/backups                 - List backups
//	POST   /api/v1/backups                 - Create backup
//	GET    /api/v1/backups/{id}            - Get backup
//	POST   /api/v1/backups/{id}/restore    - Restore backup
//	POST   /api/v1/backups/{id}/cancel     - Cancel backup
//	POST   /api/v1/migration/import/vcenter - Import from vCenter
//	POST   /api/v1/migration/import/ovf    - Import OVF/OVA
//	POST   /api/v1/migration/import/vmx    - Import VMX
//	GET    /api/v1/migration/jobs/{jobId}  - Get migration job
//	POST   /api/v1/migration/discovery     - Discover VMware
//	GET    /api/v1/events                  - List events
//	GET    /api/v1/health                  - Health check
package api

import "fmt"

// Version information
var (
    Version   = "0.1.0"
    GitCommit = "unknown"
)

// APIServer represents the HiveStack REST API server.
type APIServer struct {
    Config   *Config
    Router   *Router
    Services *Services
}

// Config holds the API server configuration.
type Config struct {
    Server   ServerConfig
    Database DatabaseConfig
    Auth     AuthConfig
    Node     NodeConfig
}

// ServerConfig holds the server configuration.
type ServerConfig struct {
    Host         string
    Port         int
    TLSEnabled   bool
    TLSCertFile  string
    TLSKeyFile   string
}

// DatabaseConfig holds the database configuration.
type DatabaseConfig struct {
    DSN string
}

// AuthConfig holds the authentication configuration.
type AuthConfig struct {
    JWTSecret    string
    TokenExpiry  string
    EnableHTTPS  bool
}

// NodeConfig holds the node agent configuration.
type NodeConfig struct {
    GRPCAddress string
}

// Services holds the API services.
type Services struct {
    AuthService         *AuthService
    RBACService         *RBACService
    InventoryService    *InventoryService
    VMLifecycleService  *VMLifecycleService
    StorageService      *StorageService
    NetworkService      *NetworkService
    BackupService       *BackupService
    MigrationService    *MigrationService
}

// Router represents the API router.
type Router struct{}

// AuthService handles authentication.
type AuthService struct{}

// RBACService handles role-based access control.
type RBACService struct{}

// InventoryService handles inventory management.
type InventoryService struct{}

// VMLifecycleService handles VM lifecycle operations.
type VMLifecycleService struct{}

// StorageService handles storage operations.
type StorageService struct{}

// NetworkService handles network operations.
type NetworkService struct{}

// BackupService handles backup operations.
type BackupService struct{}

// MigrationService handles VMware migration.
type MigrationService struct{}

// New creates a new API server.
func New(cfg *Config) (*APIServer, error) {
    s := &APIServer{
        Config: cfg,
        Router: &Router{},
        Services: &Services{
            AuthService:         &AuthService{},
            RBACService:         &RBACService{},
            InventoryService:    &InventoryService{},
            VMLifecycleService:  &VMLifecycleService{},
            StorageService:      &StorageService{},
            NetworkService:      &NetworkService{},
            BackupService:       &BackupService{},
            MigrationService:    &MigrationService{},
        },
    }

    return s, nil
}

// Run starts the API server.
func (s *APIServer) Run() error {
    addr := fmt.Sprintf("%s:%d", s.Config.Server.Host, s.Config.Server.Port)

    if s.Config.Server.TLSEnabled {
        fmt.Printf("Starting HTTPS server on %s\n", addr)
        return fmt.Errorf("TLS server not yet implemented")
    }

    fmt.Printf("Starting HTTP server on %s\n", addr)
    return fmt.Errorf("server not yet implemented — see api/server.go")
}

// Shutdown gracefully shuts down the server.
func (s *APIServer) Shutdown() error {
    return nil
}
