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
//	server:
//	  host: "0.0.0.0"
//	  port: 8080
//	  tls:
//	    enabled: false
//	    certFile: "/etc/hivestack/tls/server.crt"
//	    keyFile: "/etc/hivestack/tls/server.key"
//	database:
//	  dsn: "postgres://hivestack:***@localhost:5432/hivestack?sslmode=disable"
//	auth:
//	  jwtSecret: "your-jwt-secret-here"
//	  tokenExpiry: 24h
//	node:
//	  grpcAddress: "hivestack-manager.example.com:9090"
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

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/maddydevel/HiveStack/internal/auth"
	"github.com/maddydevel/HiveStack/internal/compliance"
	"github.com/maddydevel/HiveStack/internal/db"
	"github.com/maddydevel/HiveStack/internal/ha"
	"github.com/maddydevel/HiveStack/internal/metrics"
	hivetls "github.com/maddydevel/HiveStack/internal/tls"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Version information
var (
	Version   = "0.1.0"
	GitCommit = "unknown"
)

// dbInterface covers the subset of *db.DB methods used by API handlers.
// It exists so tests can substitute a mock in place of a real database
// connection; *db.DB satisfies it implicitly.
type dbInterface interface {
	ListUsers(ctx context.Context, tenantID string) ([]db.User, error)
	CreateUser(ctx context.Context, u *db.User) (string, error)
	GetUser(ctx context.Context, id string) (*db.User, error)
	UpdateUser(ctx context.Context, id string, updates map[string]interface{}) error
	DeleteUser(ctx context.Context, id string) error

	ListHosts(ctx context.Context, tenantID string) ([]db.Host, error)
	CreateHost(ctx context.Context, h *db.Host) (string, error)
	GetHost(ctx context.Context, id string) (*db.Host, error)
	UpdateHost(ctx context.Context, id string, updates map[string]interface{}) error
	DeleteHost(ctx context.Context, id string) error

	ListVMs(ctx context.Context, tenantID string) ([]db.VM, error)
	CreateVM(ctx context.Context, vm *db.VM) (string, error)
	GetVM(ctx context.Context, id string) (*db.VM, error)
	UpdateVM(ctx context.Context, id string, updates map[string]interface{}) error
	DeleteVM(ctx context.Context, id string) error

	ListStoragePools(ctx context.Context, tenantID string) ([]db.StoragePool, error)
	CreateStoragePool(ctx context.Context, sp *db.StoragePool) (string, error)
	GetStoragePool(ctx context.Context, id string) (*db.StoragePool, error)

	ListNetworks(ctx context.Context, tenantID string) ([]db.Network, error)
	CreateNetwork(ctx context.Context, n *db.Network) (string, error)
	GetNetwork(ctx context.Context, id string) (*db.Network, error)
	DeleteNetwork(ctx context.Context, id string) error

	ListBackups(ctx context.Context, tenantID string) ([]db.Backup, error)
	CreateBackup(ctx context.Context, b *db.Backup) (string, error)
	GetBackup(ctx context.Context, id string) (*db.Backup, error)
	UpdateBackup(ctx context.Context, id string, updates map[string]interface{}) error

	ListEvents(ctx context.Context, tenantID string, limit int) ([]db.Event, error)

	ListDatacenters(ctx context.Context, tenantID string) ([]db.Datacenter, error)
	CreateDatacenter(ctx context.Context, dc *db.Datacenter) (string, error)
	GetDatacenter(ctx context.Context, id string) (*db.Datacenter, error)
	DeleteDatacenter(ctx context.Context, id string) error
}

// APIServer represents the HiveStack REST API server.
type APIServer struct {
	Config         *Config
	db             dbInterface
	rbac           *auth.RBACEngine
	compliance     compliance.ComplianceStore
	vmHandler      VMHandler
	storageHandler StorageHandler
	networkHandler NetworkHandler
	mux            *http.ServeMux
	// Transport: httpServer is built by New; tlsConfig is nil unless TLS is enabled.
	httpServer *http.Server
	tlsConfig  *tls.Config
	// HA integration
	haController   haControllerIface
	haOrchestrator haOrchestratorIface
	policyManager  policyManagerIface
	// Migration integration
	migrationHandler    migrationManager
	migrationJobTracker *migrationJobTracker
}

// VMHandler defines the VM lifecycle operations the API server delegates to.
type VMHandler interface {
	MigrateVM(ctx context.Context, id, targetHostID string) error
	GetSnapshots(ctx context.Context, vmID string) ([]map[string]interface{}, error)
	CreateSnapshot(ctx context.Context, vmID, name string) (string, error)
	DeleteSnapshot(ctx context.Context, vmID, snapshotID string) error
	GetVMStats(ctx context.Context, vmID string) (map[string]interface{}, error)
}

// StorageHandler defines storage operations the API server delegates to.
type StorageHandler interface {
	CreateStoragePool(ctx context.Context, id, name, poolType, path, hostID string) error
	DeleteStoragePool(ctx context.Context, id, hostID string) error
	ResizeDisk(ctx context.Context, diskID string, newSizeBytes int64) error
}

// NetworkHandler defines network operations the API server delegates to.
type NetworkHandler interface {
	CreateNetwork(ctx context.Context, n *db.Network) (string, error)
	DeleteNetwork(ctx context.Context, id string) error
}

// Interfaces for HA integration (avoid circular imports).
type haControllerIface interface {
	GetStatus() map[string]interface{}
	GetAllHealth() map[string]*ha.NodeHealth
	TriggerFailover(ctx context.Context, nodeID string) error
}

type haOrchestratorIface interface {
	GetActiveFailovers() []ha.FailoverRecord
	GetFailoverHistory(limit int) []ha.FailoverRecord
}

type policyManagerIface interface {
	GetPolicy(vmID string) (ha.HAPolicy, bool)
	SetPolicy(vmID string, policy ha.HAPolicy, actor string) error
}

// Config holds the API server configuration.
type Config struct {
	Server ServerConfig
	// TLS holds the certificate paths used when Server.TLSEnabled is set.
	TLS      hivetls.CertConfig
	Database DatabaseConfig
	Auth     AuthConfig
	Node     NodeConfig
}

// ServerConfig holds the server configuration.
type ServerConfig struct {
	Host string
	Port int
	// TLSEnabled serves HTTPS using Config.TLS. Disabled by default.
	TLSEnabled bool
	// TLSDevMode generates a self-signed certificate instead of requiring
	// Config.TLS files to exist. Development only; ignored unless TLSEnabled.
	TLSDevMode bool
}

// DatabaseConfig holds the database configuration.
type DatabaseConfig struct {
	DSN string
}

// AuthConfig holds the authentication configuration.
type AuthConfig struct {
	JWTSecret   string
	TokenExpiry string
}

// NodeConfig holds the node agent configuration.
type NodeConfig struct {
	GRPCAddress string
}

// New creates a new API server.
func New(cfg *Config, database *db.DB) (*APIServer, error) {
	s := &APIServer{
		Config:     cfg,
		db:         database,
		rbac:       auth.NewRBACEngine(),
		compliance: *compliance.NewComplianceStore(database),
	}
	s.mux = new(http.ServeMux)
	s.registerRoutes()
	s.migrationJobTracker = newMigrationJobTracker()
	if err := s.setupHTTPServer(); err != nil {
		return nil, err
	}
	return s, nil
}

// registerRoutes mounts all HTTP handlers.
func (s *APIServer) registerRoutes() {
	// Health
	s.mux.HandleFunc("GET /health", s.handleHealth)

	// Auth
	s.mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	s.mux.HandleFunc("GET /api/v1/auth/me", auth.RequireAuth(s.handleMe))
	s.mux.HandleFunc("POST /api/v1/auth/logout", s.handleLogout)

	// Users
	s.mux.HandleFunc("GET /api/v1/users", auth.RequireAuth(s.handleListUsers))
	s.mux.HandleFunc("POST /api/v1/users", auth.RequireAuth(s.handleCreateUser))
	s.mux.HandleFunc("GET /api/v1/users/{id}", auth.RequireAuth(s.handleGetUser))
	s.mux.HandleFunc("PUT /api/v1/users/{id}", auth.RequireAuth(s.handleUpdateUser))
	s.mux.HandleFunc("DELETE /api/v1/users/{id}", auth.RequireAuth(s.handleDeleteUser))

	// Datacenters
	s.mux.HandleFunc("GET /api/v1/datacenters", auth.RequireAuth(s.handleListDCs))
	s.mux.HandleFunc("POST /api/v1/datacenters", auth.RequireAuth(s.handleCreateDC))
	s.mux.HandleFunc("GET /api/v1/datacenters/{id}", auth.RequireAuth(s.handleGetDC))
	s.mux.HandleFunc("DELETE /api/v1/datacenters/{id}", auth.RequireAuth(s.handleDeleteDC))

	// Clusters
	s.mux.HandleFunc("GET /api/v1/clusters", auth.RequireAuth(s.handleListClusters))
	s.mux.HandleFunc("POST /api/v1/clusters", auth.RequireAuth(s.handleCreateCluster))
	s.mux.HandleFunc("GET /api/v1/clusters/{id}", auth.RequireAuth(s.handleGetCluster))
	s.mux.HandleFunc("GET /api/v1/clusters/{id}/hosts", auth.RequireAuth(s.handleClusterHosts))

	// Hosts
	s.mux.HandleFunc("GET /api/v1/hosts", auth.RequireAuth(s.handleListHosts))
	s.mux.HandleFunc("POST /api/v1/hosts", auth.RequireAuth(s.handleRegisterHost))
	s.mux.HandleFunc("GET /api/v1/hosts/{id}", auth.RequireAuth(s.handleGetHost))
	s.mux.HandleFunc("PUT /api/v1/hosts/{id}", auth.RequireAuth(s.handleUpdateHost))
	s.mux.HandleFunc("DELETE /api/v1/hosts/{id}", auth.RequireAuth(s.handleDeleteHost))
	s.mux.HandleFunc("GET /api/v1/hosts/{id}/status", auth.RequireAuth(s.handleHostStatus))
	s.mux.HandleFunc("POST /api/v1/hosts/{id}/maintenance", auth.RequireAuth(s.handleHostMaintenance))
	s.mux.HandleFunc("DELETE /api/v1/hosts/{id}/maintenance", auth.RequireAuth(s.handleHostExitMaintenance))

	// VMs
	s.mux.HandleFunc("GET /api/v1/vms", auth.RequireAuth(s.handleListVMs))
	s.mux.HandleFunc("POST /api/v1/vms", auth.RequireAuth(s.handleCreateVM))
	s.mux.HandleFunc("GET /api/v1/vms/{id}", auth.RequireAuth(s.handleGetVM))
	s.mux.HandleFunc("PUT /api/v1/vms/{id}", auth.RequireAuth(s.handleUpdateVM))
	s.mux.HandleFunc("DELETE /api/v1/vms/{id}", auth.RequireAuth(s.handleDeleteVM))
	s.mux.HandleFunc("POST /api/v1/vms/{id}/start", auth.RequireAuth(s.handleVMStart))
	s.mux.HandleFunc("POST /api/v1/vms/{id}/stop", auth.RequireAuth(s.handleVMStop))
	s.mux.HandleFunc("POST /api/v1/vms/{id}/restart", auth.RequireAuth(s.handleVMRestart))
	s.mux.HandleFunc("POST /api/v1/vms/{id}/migrate", auth.RequireAuth(s.handleVMMigrate))
	s.mux.HandleFunc("GET /api/v1/vms/{id}/snapshots", auth.RequireAuth(s.handleVMStackTrace))
	s.mux.HandleFunc("POST /api/v1/vms/{id}/snapshots", auth.RequireAuth(s.handleVMCreateSnapshot))
	s.mux.HandleFunc("DELETE /api/v1/vms/{id}/snapshots/{sid}", auth.RequireAuth(s.handleVMDeleteSnapshot))
	s.mux.HandleFunc("GET /api/v1/vms/{id}/console", auth.RequireAuth(s.handleVMConsole))
	s.mux.HandleFunc("GET /api/v1/vms/{id}/stats", auth.RequireAuth(s.handleVMStats))

	// Storage pools
	s.mux.HandleFunc("GET /api/v1/storage-pools", auth.RequireAuth(s.handleListStoragePools))
	s.mux.HandleFunc("POST /api/v1/storage-pools", auth.RequireAuth(s.handleCreateStoragePool))
	s.mux.HandleFunc("GET /api/v1/storage-pools/{id}", auth.RequireAuth(s.handleGetStoragePool))
	s.mux.HandleFunc("DELETE /api/v1/storage-pools/{id}", auth.RequireAuth(s.handleDeleteStoragePool))
	s.mux.HandleFunc("GET /api/v1/storage-pools/{id}/disks", auth.RequireAuth(s.handleStoragePoolDisks))

	// Disks
	s.mux.HandleFunc("GET /api/v1/disks/{diskId}", auth.RequireAuth(s.handleGetDisk))
	s.mux.HandleFunc("PUT /api/v1/disks/{diskId}/resize", auth.RequireAuth(s.handleResizeDisk))

	// Networks
	s.mux.HandleFunc("GET /api/v1/networks", auth.RequireAuth(s.handleListNetworks))
	s.mux.HandleFunc("POST /api/v1/networks", auth.RequireAuth(s.handleCreateNetwork))
	s.mux.HandleFunc("GET /api/v1/networks/{id}", auth.RequireAuth(s.handleGetNetwork))
	s.mux.HandleFunc("DELETE /api/v1/networks/{id}", auth.RequireAuth(s.handleDeleteNetwork))

	// Backups
	s.mux.HandleFunc("GET /api/v1/backups", auth.RequireAuth(s.handleListBackups))
	s.mux.HandleFunc("POST /api/v1/backups", auth.RequireAuth(s.handleCreateBackup))
	s.mux.HandleFunc("GET /api/v1/backups/{id}", auth.RequireAuth(s.handleGetBackup))
	s.mux.HandleFunc("POST /api/v1/backups/{id}/restore", auth.RequireAuth(s.handleBackupRestore))
	s.mux.HandleFunc("POST /api/v1/backups/{id}/cancel", auth.RequireAuth(s.handleBackupCancel))

	// Events
	s.mux.HandleFunc("GET /api/v1/events", auth.RequireAuth(s.handleListEvents))

	// Compliance
	s.mux.HandleFunc("GET /api/v1/compliance/vms/{id}", auth.RequireAuth(s.handleComplianceCheck))
	s.mux.HandleFunc("GET /api/v1/compliance/evidence/{id}", auth.RequireAuth(s.handleComplianceEvidence))
	s.mux.HandleFunc("GET /api/v1/compliance/drift", auth.RequireAuth(s.handleComplianceDrift))

	// Register HA routes
	s.registerHARoutes()

	// Register migration routes
	s.registerMigrationRoutes()

	// Metrics (Prometheus scraping - no auth required)
	s.mux.HandleFunc("GET /metrics", s.handleMetrics)
}

// handleMetrics serves Prometheus-formatted metrics.
func (s *APIServer) handleMetrics(w http.ResponseWriter, r *http.Request) {
	metrics.SetManagerUp(true)
	metrics.UpdateHostMetrics("p-"+s.Config.Server.Host, "hivestack-manager", 0, 0, 0, 0, 0, 0, "online")
	promhttp.Handler().ServeHTTP(w, r)
}

// SetHAController sets the HA controller for API handlers.
func (s *APIServer) SetHAController(ctrl haControllerIface) {
	s.haController = ctrl
}

// SetHAOrchestrator sets the HA orchestrator for API handlers.
func (s *APIServer) SetHAOrchestrator(orch haOrchestratorIface) {
	s.haOrchestrator = orch
}

// SetPolicyManager sets the policy manager for API handlers.
func (s *APIServer) SetPolicyManager(pm policyManagerIface) {
	s.policyManager = pm
}

// SetVMHandler sets the VM lifecycle handler for API handlers.
func (s *APIServer) SetVMHandler(h VMHandler) {
	s.vmHandler = h
}

// SetStorageHandler sets the storage handler for API handlers.
func (s *APIServer) SetStorageHandler(h StorageHandler) {
	s.storageHandler = h
}

// SetNetworkHandler sets the network handler for API handlers.
func (s *APIServer) SetNetworkHandler(h NetworkHandler) {
	s.networkHandler = h
}

// UpdateHostMetrics records Prometheus metrics for a host.
func (s *APIServer) UpdateHostMetrics(hostID, hostname string, cpuPercent float64,
	memoryUsed, memoryTotal uint64, diskUsed, diskTotal uint64, vmCount int) {
	metrics.UpdateHostMetrics(hostID, hostname, cpuPercent, int64(memoryUsed), int64(memoryTotal),
		int64(diskUsed), int64(diskTotal), vmCount, "online")
}

// UpdateVMMetrics records Prometheus metrics for a VM.
func (s *APIServer) UpdateVMMetrics(vmID, vmName, hostID string, cpuPercent float64,
	memoryUsed, memoryTotal uint64, snapshotCount int, role string) {
	metrics.UpdateVMMetrics(vmID, vmName, hostID, cpuPercent, int64(memoryUsed), int64(memoryTotal),
		snapshotCount, role)
}

// respondJSON writes a JSON response.
func (s *APIServer) respondJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// handleHealth returns server health status.
func (s *APIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":     "ok",
		"version":    Version,
		"git_commit": GitCommit,
	})
}

// handleLogin authenticates a user and returns a JWT.
func (s *APIServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.Email == "" || req.Password == "" {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "email and password required"})
		return
	}

	claims, ok := auth.ClaimsFromContext(r.Context())
	if ok && claims != nil {
		s.respondJSON(w, http.StatusOK, map[string]interface{}{
			"token":     "already-authenticated",
			"user_id":   claims.UserID,
			"tenant_id": claims.TenantID,
		})
		return
	}

	userID := fmt.Sprintf("user-%s", req.Email)
	tenantID := "default"
	token, err := auth.GenerateToken(userID, tenantID, []string{"viewer"}, 0)
	if err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate token"})
		return
	}
	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"token":      token,
		"user_id":    userID,
		"tenant_id":  tenantID,
		"expires_in": 86400,
	})
}

// handleMe returns the current authenticated user.
func (s *APIServer) handleMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"user_id":   claims.UserID,
		"tenant_id": claims.TenantID,
		"scopes":    claims.Scopes,
	})
}

// handleListUsers returns all users in the tenant.
func (s *APIServer) handleListUsers(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	users, err := s.db.ListUsers(context.Background(), claims.TenantID)
	if err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.respondJSON(w, http.StatusOK, users)
}

// handleCreateUser creates a new user.
func (s *APIServer) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	var req struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.Name == "" || req.Email == "" || req.Password == "" {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "name, email, and password required"})
		return
	}
	role := req.Role
	if role == "" {
		role = "viewer"
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to hash password"})
		return
	}
	user := &db.User{
		TenantID:     claims.TenantID,
		Name:         req.Name,
		Email:        req.Email,
		PasswordHash: hash,
		Role:         role,
	}
	id, err := s.db.CreateUser(context.Background(), user)
	if err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.respondJSON(w, http.StatusCreated, map[string]string{"id": id})
}

// handleGetUser returns a user by ID.
func (s *APIServer) handleGetUser(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	user, err := s.db.GetUser(context.Background(), id)
	if err != nil {
		s.respondJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}
	if user.TenantID != claims.TenantID {
		s.respondJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	user.PasswordHash = ""
	s.respondJSON(w, http.StatusOK, user)
}

// handleUpdateUser updates a user.
func (s *APIServer) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	user, err := s.db.GetUser(context.Background(), id)
	if err != nil {
		s.respondJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}
	if user.TenantID != claims.TenantID {
		s.respondJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	var req struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Email != "" {
		updates["email"] = req.Email
	}
	if req.Role != "" {
		updates["role"] = req.Role
	}
	if err := s.db.UpdateUser(context.Background(), id, updates); err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// handleDeleteUser deletes a user.
func (s *APIServer) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	user, err := s.db.GetUser(context.Background(), id)
	if err != nil {
		s.respondJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}
	if user.TenantID != claims.TenantID {
		s.respondJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if err := s.db.DeleteUser(context.Background(), id); err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleListDCs returns all datacenters.
func (s *APIServer) handleListDCs(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	s.respondJSON(w, http.StatusOK, []map[string]string{})
}

// handleCreateDC creates a datacenter.
func (s *APIServer) handleCreateDC(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	s.respondJSON(w, http.StatusCreated, map[string]string{"id": "dc-new", "name": req.Name})
}

// handleGetDC returns a datacenter.
func (s *APIServer) handleGetDC(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	if id == "" {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}
	dc, err := s.db.GetDatacenter(context.Background(), id)
	if err != nil {
		s.respondJSON(w, http.StatusNotFound, map[string]string{"error": "datacenter not found"})
		return
	}
	s.respondJSON(w, http.StatusOK, dc)
}

// handleDeleteDC deletes a datacenter.
func (s *APIServer) handleDeleteDC(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	s.respondJSON(w, http.StatusOK, map[string]string{"status": "deleted", "id": id})
}

// handleListClusters returns all clusters.
func (s *APIServer) handleListClusters(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	s.respondJSON(w, http.StatusOK, []map[string]string{})
}

// handleCreateCluster creates a cluster.
func (s *APIServer) handleCreateCluster(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		DCID        string `json:"datacenter_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	s.respondJSON(w, http.StatusCreated, map[string]string{"id": "cluster-new", "name": req.Name})
}

// handleGetCluster returns a cluster.
func (s *APIServer) handleGetCluster(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	s.respondJSON(w, http.StatusOK, map[string]string{"id": id, "name": "cluster-" + id})
}

// handleClusterHosts returns hosts in a cluster.
func (s *APIServer) handleClusterHosts(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	s.respondJSON(w, http.StatusOK, []map[string]string{})
}

// handleListHosts returns all hosts.
func (s *APIServer) handleListHosts(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	hosts, err := s.db.ListHosts(context.Background(), claims.TenantID)
	if err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.respondJSON(w, http.StatusOK, hosts)
}

// handleRegisterHost registers a new host.
func (s *APIServer) handleRegisterHost(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	var req struct {
		Name      string `json:"name"`
		Hostname  string `json:"hostname"`
		IPAddress string `json:"ip_address"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	host := &db.Host{
		TenantID:  claims.TenantID,
		Name:      req.Name,
		Hostname:  req.Hostname,
		IPAddress: req.IPAddress,
		Status:    "pending",
	}
	id, err := s.db.CreateHost(context.Background(), host)
	if err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.respondJSON(w, http.StatusCreated, map[string]string{"id": id})
}

// handleGetHost returns a host.
func (s *APIServer) handleGetHost(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	host, err := s.db.GetHost(context.Background(), id)
	if err != nil {
		s.respondJSON(w, http.StatusNotFound, map[string]string{"error": "host not found"})
		return
	}
	s.respondJSON(w, http.StatusOK, host)
}

// handleUpdateHost updates a host.
func (s *APIServer) handleUpdateHost(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if err := s.db.UpdateHost(context.Background(), id, updates); err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// handleDeleteHost removes a host.
func (s *APIServer) handleDeleteHost(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.db.DeleteHost(context.Background(), id); err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleHostStatus returns host status.
func (s *APIServer) handleHostStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	host, err := s.db.GetHost(context.Background(), id)
	if err != nil {
		s.respondJSON(w, http.StatusNotFound, map[string]string{"error": "host not found"})
		return
	}
	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"id":             host.ID,
		"name":           host.Name,
		"hostname":       host.Hostname,
		"ip_address":     host.IPAddress,
		"status":         host.Status,
		"cpu_count":      host.CPUCount,
		"memory_bytes":   host.MemoryTotalBytes,
		"storage_bytes":  host.StorageTotalBytes,
		"numa_nodes":     host.NUMANodeCount,
		"hugepages_kb":   host.HugepagesTotalKB,
		"maintenance":    host.MaintenanceMode,
		"last_heartbeat": host.LastHeartbeat,
		"joined_at":      host.JoinedAt,
	})
}

// handleHostMaintenance puts a host in maintenance mode.
func (s *APIServer) handleHostMaintenance(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	if err := s.db.UpdateHost(context.Background(), id, map[string]interface{}{
		"maintenance_mode": true,
		"status":           "maintenance",
	}); err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.respondJSON(w, http.StatusOK, map[string]string{"status": "maintenance"})
}

// handleHostExitMaintenance exits maintenance mode.
func (s *APIServer) handleHostExitMaintenance(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	if err := s.db.UpdateHost(context.Background(), id, map[string]interface{}{
		"maintenance_mode": false,
		"status":           "online",
	}); err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.respondJSON(w, http.StatusOK, map[string]string{"status": "online"})
}

// handleListVMs returns all VMs.
func (s *APIServer) handleListVMs(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	vms, err := s.db.ListVMs(context.Background(), claims.TenantID)
	if err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.respondJSON(w, http.StatusOK, vms)
}

// handleCreateVM creates a new VM.
func (s *APIServer) handleCreateVM(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	var req struct {
		Name              string `json:"name"`
		Description       string `json:"description"`
		CPUs              int    `json:"cpus"`
		CPUAllocation     string `json:"cpu_allocation"`
		MemoryBytes       int64  `json:"memory_bytes"`
		Role              string `json:"role"`
		OS                string `json:"os"`
		HostID            string `json:"host_id"`
		ClusterID         string `json:"cluster_id"`
		NUMAPolicy        string `json:"numa_policy"`
		HugepagesEnabled  bool   `json:"hugepages_enabled"`
		BallooningAllowed bool   `json:"ballooning_allowed"`
		SwapAllowed       bool   `json:"swap_allowed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.Name == "" {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "name required"})
		return
	}
	role := db.VMRoleGeneric
	if req.Role == "hana" {
		role = db.VMRoleHANA
	}
	numapolicy := (*string)(nil)
	if req.NUMAPolicy != "" {
		np := req.NUMAPolicy
		numapolicy = &np
	}
	vm := &db.VM{
		TenantID:          claims.TenantID,
		Name:              req.Name,
		Description:       req.Description,
		CPUs:              req.CPUs,
		CPUAllocation:     req.CPUAllocation,
		MemoryBytes:       req.MemoryBytes,
		Role:              role,
		OS:                req.OS,
		HostID:            nil,
		ClusterID:         nil,
		NUMAPolicy:        numapolicy,
		HugepagesEnabled:  req.HugepagesEnabled,
		BallooningAllowed: req.BallooningAllowed,
		SwapAllowed:       req.SwapAllowed,
		Status:            "pending",
	}
	if req.HostID != "" {
		vm.HostID = &req.HostID
	}
	if req.ClusterID != "" {
		vm.ClusterID = &req.ClusterID
	}
	id, err := s.db.CreateVM(context.Background(), vm)
	if err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.respondJSON(w, http.StatusCreated, map[string]string{"id": id})
}

// handleGetVM returns a VM.
func (s *APIServer) handleGetVM(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	vm, err := s.db.GetVM(context.Background(), id)
	if err != nil {
		s.respondJSON(w, http.StatusNotFound, map[string]string{"error": "vm not found"})
		return
	}
	vm.CPUPinning = nil // don't expose CPU pinning details
	s.respondJSON(w, http.StatusOK, vm)
}

// handleUpdateVM updates a VM.
func (s *APIServer) handleUpdateVM(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	_, err := s.db.GetVM(context.Background(), id)
	if err != nil {
		s.respondJSON(w, http.StatusNotFound, map[string]string{"error": "vm not found"})
		return
	}
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Status      string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if err := s.db.UpdateVM(context.Background(), id, updates); err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// handleDeleteVM deletes a VM.
func (s *APIServer) handleDeleteVM(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	if err := s.db.DeleteVM(context.Background(), id); err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleVMStart starts a VM.
func (s *APIServer) handleVMStart(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	if err := s.db.UpdateVM(context.Background(), id, map[string]interface{}{"status": "running"}); err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.respondJSON(w, http.StatusOK, map[string]string{"status": "running"})
}

// handleVMStop stops a VM.
func (s *APIServer) handleVMStop(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	if err := s.db.UpdateVM(context.Background(), id, map[string]interface{}{"status": "stopped"}); err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.respondJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

// handleVMRestart restarts a VM.
func (s *APIServer) handleVMRestart(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	if err := s.db.UpdateVM(context.Background(), id, map[string]interface{}{"status": "restarting"}); err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.respondJSON(w, http.StatusOK, map[string]string{"status": "restarting"})
}

// handleVMMigrate migrates a VM to another host.
func (s *APIServer) handleVMMigrate(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	var req struct {
		TargetHost string `json:"target_host"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.TargetHost == "" {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "target_host required"})
		return
	}
	if s.vmHandler == nil {
		s.respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "manager not available"})
		return
	}
	if err := s.vmHandler.MigrateVM(r.Context(), id, req.TargetHost); err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.respondJSON(w, http.StatusAccepted, map[string]interface{}{
		"status": "migrating",
		"vm_id":  id,
		"target": req.TargetHost,
	})
}

// handleVMStackTrace returns snapshots for a VM.
func (s *APIServer) handleVMStackTrace(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	if s.vmHandler == nil {
		s.respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "manager not available"})
		return
	}
	snapshots, err := s.vmHandler.GetSnapshots(r.Context(), id)
	if err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.respondJSON(w, http.StatusOK, snapshots)
}

// handleVMCreateSnapshot creates a VM snapshot.
func (s *APIServer) handleVMCreateSnapshot(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if s.vmHandler == nil {
		s.respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "manager not available"})
		return
	}
	snapshotID, err := s.vmHandler.CreateSnapshot(r.Context(), id, req.Name)
	if err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.respondJSON(w, http.StatusCreated, map[string]string{"snapshot_id": snapshotID})
}

// handleVMDeleteSnapshot deletes a VM snapshot.
func (s *APIServer) handleVMDeleteSnapshot(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	sid := r.PathValue("sid")
	if s.vmHandler == nil {
		s.respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "manager not available"})
		return
	}
	if err := s.vmHandler.DeleteSnapshot(r.Context(), id, sid); err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleVMConsole returns the console URL for a VM.
func (s *APIServer) handleVMConsole(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	_ = r.PathValue("id")
	s.respondJSON(w, http.StatusOK, map[string]string{"console_url": ""})
}

// handleVMStats returns VM statistics.
func (s *APIServer) handleVMStats(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	if s.vmHandler == nil {
		s.respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "handler not available"})
		return
	}
	stats, err := s.vmHandler.GetVMStats(r.Context(), id)
	if err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.respondJSON(w, http.StatusOK, stats)
}

// handleListStoragePools returns all storage pools.
func (s *APIServer) handleListStoragePools(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	pools, err := s.db.ListStoragePools(context.Background(), claims.TenantID)
	if err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.respondJSON(w, http.StatusOK, pools)
}

// handleCreateStoragePool creates a storage pool.
func (s *APIServer) handleCreateStoragePool(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	var req struct {
		Name   string `json:"name"`
		Type   string `json:"type"`
		Path   string `json:"path"`
		HostID string `json:"host_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.Name == "" {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "name required"})
		return
	}
	if req.HostID == "" {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "host_id required"})
		return
	}

	// Create DB record first
	sp := &db.StoragePool{
		TenantID: claims.TenantID,
		Name:     req.Name,
		Type:     req.Type,
		Path:     req.Path,
		Status:   "creating",
	}
	id, err := s.db.CreateStoragePool(context.Background(), sp)
	if err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Delegate to the node agent via the storage handler
	if s.storageHandler != nil {
		if err := s.storageHandler.CreateStoragePool(r.Context(), id, req.Name, req.Type, req.Path, req.HostID); err != nil {
			s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	s.respondJSON(w, http.StatusCreated, map[string]string{"id": id})
}

// handleGetStoragePool returns a storage pool.
func (s *APIServer) handleGetStoragePool(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	pool, err := s.db.GetStoragePool(context.Background(), id)
	if err != nil {
		s.respondJSON(w, http.StatusNotFound, map[string]string{"error": "pool not found"})
		return
	}
	s.respondJSON(w, http.StatusOK, pool)
}

// handleDeleteStoragePool deletes a storage pool.
func (s *APIServer) handleDeleteStoragePool(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	var req struct {
		HostID string `json:"host_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.HostID == "" {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "host_id required"})
		return
	}
	if s.storageHandler != nil {
		if err := s.storageHandler.DeleteStoragePool(r.Context(), id, req.HostID); err != nil {
			s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}
	s.respondJSON(w, http.StatusOK, map[string]string{"status": "deleted", "id": id})
}

// handleStoragePoolDisks returns disks in a storage pool.
func (s *APIServer) handleStoragePoolDisks(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	s.respondJSON(w, http.StatusOK, []map[string]string{})
}

// handleGetDisk returns a disk.
func (s *APIServer) handleGetDisk(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	s.respondJSON(w, http.StatusOK, map[string]string{"id": id, "size": "10GB"})
}

// handleResizeDisk resizes a disk.
func (s *APIServer) handleResizeDisk(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	var req struct {
		NewSizeBytes int64 `json:"new_size_bytes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.NewSizeBytes <= 0 {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "new_size_bytes must be positive"})
		return
	}
	if s.storageHandler == nil {
		s.respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "storage handler not available"})
		return
	}
	if err := s.storageHandler.ResizeDisk(r.Context(), id, req.NewSizeBytes); err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.respondJSON(w, http.StatusOK, map[string]string{"id": id, "status": "resized"})
}

// handleListNetworks returns all networks.
func (s *APIServer) handleListNetworks(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	networks, err := s.db.ListNetworks(context.Background(), claims.TenantID)
	if err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.respondJSON(w, http.StatusOK, networks)
}

// handleCreateNetwork creates a network.
func (s *APIServer) handleCreateNetwork(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	if s.networkHandler == nil {
		s.respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "network handler not available"})
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	var req struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Type        string   `json:"type"`
		BridgeName  string   `json:"bridge_name"`
		Subnet      string   `json:"subnet"`
		Gateway     string   `json:"gateway"`
		DHCP        bool     `json:"dhcp"`
		DNS         []string `json:"dns"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	n := &db.Network{
		TenantID:    claims.TenantID,
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		BridgeName:  req.BridgeName,
		Subnet:      req.Subnet,
		Gateway:     req.Gateway,
		DHCP:        req.DHCP,
		DNS:         req.DNS,
		Status:      "active",
	}
	id, err := s.networkHandler.CreateNetwork(context.Background(), n)
	if err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.respondJSON(w, http.StatusCreated, map[string]string{"id": id})
}

// handleGetNetwork returns a network.
func (s *APIServer) handleGetNetwork(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	network, err := s.db.GetNetwork(context.Background(), id)
	if err != nil {
		s.respondJSON(w, http.StatusNotFound, map[string]string{"error": "network not found"})
		return
	}
	s.respondJSON(w, http.StatusOK, network)
}

// handleDeleteNetwork deletes a network.
func (s *APIServer) handleDeleteNetwork(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	if err := s.networkHandler.DeleteNetwork(context.Background(), id); err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleListBackups returns all backups.
func (s *APIServer) handleListBackups(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	backups, err := s.db.ListBackups(context.Background(), claims.TenantID)
	if err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.respondJSON(w, http.StatusOK, backups)
}

// handleCreateBackup creates a backup.
func (s *APIServer) handleCreateBackup(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	var req struct {
		VMID        string `json:"vm_id"`
		Name        string `json:"name"`
		Type        string `json:"type"`
		StoragePath string `json:"storage_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.VMID == "" || req.Name == "" {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "vm_id and name required"})
		return
	}
	b := &db.Backup{
		TenantID:    claims.TenantID,
		VMID:        req.VMID,
		Name:        req.Name,
		Type:        req.Type,
		StoragePath: req.StoragePath,
		Status:      "creating",
		Progress:    0,
	}
	id, err := s.db.CreateBackup(context.Background(), b)
	if err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.respondJSON(w, http.StatusCreated, map[string]string{"id": id, "status": "creating"})
}

// handleGetBackup returns a backup.
func (s *APIServer) handleGetBackup(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	backup, err := s.db.GetBackup(context.Background(), id)
	if err != nil {
		s.respondJSON(w, http.StatusNotFound, map[string]string{"error": "backup not found"})
		return
	}
	backup.Message = "" // don't expose internal message
	s.respondJSON(w, http.StatusOK, backup)
}

// handleBackupRestore restores a backup.
func (s *APIServer) handleBackupRestore(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	if err := s.db.UpdateBackup(context.Background(), id, map[string]interface{}{
		"status": "restoring",
	}); err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.respondJSON(w, http.StatusOK, map[string]string{"status": "restoring"})
}

// handleBackupCancel cancels a backup.
func (s *APIServer) handleBackupCancel(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	if err := s.db.UpdateBackup(context.Background(), id, map[string]interface{}{
		"status": "cancelled",
	}); err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.respondJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// handleListEvents returns all events.
func (s *APIServer) handleListEvents(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	events, err := s.db.ListEvents(context.Background(), claims.TenantID, 50)
	if err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.respondJSON(w, http.StatusOK, events)
}

// handleComplianceCheck returns compliance check for a VM.
func (s *APIServer) handleComplianceCheck(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	vm, err := s.db.GetVM(context.Background(), id)
	if err != nil {
		s.respondJSON(w, http.StatusNotFound, map[string]string{"error": "vm not found"})
		return
	}
	profile := &compliance.VMProfile{
		Role:                   string(vm.Role),
		CPUs:                   vm.CPUs,
		CPUAllocation:          vm.CPUAllocation,
		MemoryBytes:            vm.MemoryBytes,
		NUMAPolicy:             vm.NUMAPolicy,
		HugepagesEnabled:       vm.HugepagesEnabled,
		CPUPinning:             vm.CPUPinning,
		MemoryReservationBytes: vm.MemoryReservationBytes,
		BallooningAllowed:      vm.BallooningAllowed,
		SwapAllowed:            vm.SwapAllowed,
	}
	result := compliance.ValidateHANAProfile(profile)
	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"id":         id,
		"compliant":  result.Passed,
		"role":       string(vm.Role),
		"violations": result.Violations,
	})
}

// handleComplianceEvidence returns compliance evidence for a VM.
func (s *APIServer) handleComplianceEvidence(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	vm, err := s.db.GetVM(context.Background(), id)
	if err != nil {
		s.respondJSON(w, http.StatusNotFound, map[string]string{"error": "vm not found"})
		return
	}
	evidence, err := s.compliance.GetEvidence(context.Background(), vm.TenantID, id)
	if err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"id":       id,
		"evidence": evidence,
	})
}

// handleComplianceDrift returns compliance drift report.
func (s *APIServer) handleComplianceDrift(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	report, err := s.compliance.GenerateDriftReport(context.Background(), claims.TenantID)
	if err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.respondJSON(w, http.StatusOK, report)
}

// handleLogout clears the current authenticated session.
func (s *APIServer) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}
