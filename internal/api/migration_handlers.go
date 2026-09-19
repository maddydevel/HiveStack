// Package api provides REST API handlers for HiveStack migration operations.
package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/maddydevel/HiveStack/internal/auth"
	"github.com/maddydevel/HiveStack/internal/compliance"
	"github.com/maddydevel/HiveStack/internal/db"
	"github.com/maddydevel/HiveStack/migration"
)

// migrationManager defines the operations the migration handlers need from the
// manager layer to import a parsed VMX into HiveStack.
type migrationManager interface {
	// CreateVMFromVMX creates a VM from a parsed VMX specification.
	CreateVMFromVMX(ctx context.Context, spec migration.VMXImportSpec, tenantID string) (string, error)
}

// MigrationJob tracks the state of an in-flight VMX import.
type MigrationJob struct {
	ID          string                 `json:"id"`
	Status      string                 `json:"status"` // "pending", "parsing", "validating", "creating", "completed", "failed"
	SourceVMX   string                 `json:"source_vmx"`
	VMName      string                 `json:"vm_name,omitempty"`
	VMID        string                 `json:"vm_id,omitempty"`
	TenantID    string                 `json:"tenant_id"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
}

// migrationJobTracker provides in-memory tracking of migration jobs.
type migrationJobTracker struct {
	mu    sync.RWMutex
	jobs  map[string]*MigrationJob
	order []string // insertion order for listing
}

// newMigrationJobTracker creates a new migration job tracker.
func newMigrationJobTracker() *migrationJobTracker {
	return &migrationJobTracker{
		jobs:  make(map[string]*MigrationJob),
		order: make([]string, 0),
	}
}

// Create adds a new migration job and returns its ID.
func (t *migrationJobTracker) Create(sourceVMX, tenantID string) *MigrationJob {
	t.mu.Lock()
	defer t.mu.Unlock()

	id := fmt.Sprintf("migration-%d", time.Now().UnixNano())
	job := &MigrationJob{
		ID:        id,
		Status:    "pending",
		SourceVMX: sourceVMX,
		TenantID:  tenantID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Details:   make(map[string]interface{}),
	}
	t.jobs[id] = job
	t.order = append(t.order, id)
	return job
}

// Get returns a migration job by ID.
func (t *migrationJobTracker) Get(id string) (*MigrationJob, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	job, ok := t.jobs[id]
	return job, ok
}

// Update modifies a migration job's status and optional fields.
func (t *migrationJobTracker) Update(id, status string, updates map[string]interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()
	job, ok := t.jobs[id]
	if !ok {
		return
	}
	job.Status = status
	job.UpdatedAt = time.Now()
	if vmName, ok := updates["vm_name"].(string); ok {
		job.VMName = vmName
	}
	if vmID, ok := updates["vm_id"].(string); ok {
		job.VMID = vmID
	}
	if errMsg, ok := updates["error"].(string); ok {
		job.Error = errMsg
	}
	if details, ok := updates["details"].(map[string]interface{}); ok {
		job.Details = details
	}
	if status == "completed" || status == "failed" {
		now := time.Now()
		job.CompletedAt = &now
	}
}

// List returns all migration jobs, most recent first.
func (t *migrationJobTracker) List() []*MigrationJob {
	t.mu.RLock()
	defer t.mu.RUnlock()
	result := make([]*MigrationJob, 0, len(t.order))
	for i := len(t.order) - 1; i >= 0; i-- {
		if job, ok := t.jobs[t.order[i]]; ok {
			result = append(result, job)
		}
	}
	return result
}

// VMXImportSpec wraps a parsed VMX for import.
type VMXImportSpec struct {
	ParsedVM *migration.ParsedVM `json:"parsed_vm"`
	HostID   string              `json:"host_id"`
	ClusterID string             `json:"cluster_id,omitempty"`
}

// registerMigrationRoutes mounts migration endpoints on the API server.
func (s *APIServer) registerMigrationRoutes() {
	s.mux.HandleFunc("POST /api/v1/migration/import/vmx", auth.RequireAuth(s.handleVMXImport))
	s.mux.HandleFunc("GET /api/v1/migration/jobs/{jobId}", auth.RequireAuth(s.handleGetMigrationJob))
	s.mux.HandleFunc("GET /api/v1/migration/jobs", auth.RequireAuth(s.handleListMigrationJobs))
	s.mux.HandleFunc("POST /api/v1/migration/import/vmx/precheck", auth.RequireAuth(s.handleVMXPrecheck))
}

// SetMigrationHandler sets the migration manager for API handlers.
func (s *APIServer) SetMigrationHandler(h migrationManager) {
	s.migrationHandler = h
}

// handleVMXImport accepts a VMX file upload, parses it, validates compliance,
// and creates a VM via the migration manager.
func (s *APIServer) handleVMXImport(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Parse multipart form (max 32MB for VMX file)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to parse form: " + err.Error()})
		return
	}

	hostID := r.FormValue("host_id")
	if hostID == "" {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "host_id is required"})
		return
	}

	clusterID := r.FormValue("cluster_id")

	// Get the uploaded VMX file
	file, header, err := r.FormFile("vmx_file")
	if err != nil {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "vmx_file is required: " + err.Error()})
		return
	}
	defer file.Close()

	// Validate file extension
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".vmx") {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "file must have .vmx extension"})
		return
	}

	// Create migration job
	job := s.migrationJobTracker.Create(header.Filename, claims.TenantID)

	// Write uploaded file to temp location
	tmpDir, err := os.MkdirTemp("", "vmx-import-*")
	if err != nil {
		s.migrationJobTracker.Update(job.ID, "failed", map[string]interface{}{"error": "create temp dir: " + err.Error()})
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	defer os.RemoveAll(tmpDir)

	tmpPath := filepath.Join(tmpDir, header.Filename)
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		s.migrationJobTracker.Update(job.ID, "failed", map[string]interface{}{"error": "write temp file: " + err.Error()})
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	if _, err := io.Copy(tmpFile, file); err != nil {
		tmpFile.Close()
		s.migrationJobTracker.Update(job.ID, "failed", map[string]interface{}{"error": "save upload: " + err.Error()})
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	tmpFile.Close()

	// Parse VMX
	s.migrationJobTracker.Update(job.ID, "parsing", nil)
	parsedVM, err := migration.ParseVMX(tmpPath)
	if err != nil {
		s.migrationJobTracker.Update(job.ID, "failed", map[string]interface{}{"error": "parse VMX: " + err.Error()})
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to parse VMX: " + err.Error()})
		return
	}

	// Pre-flight compliance check
	s.migrationJobTracker.Update(job.ID, "validating", map[string]interface{}{
		"vm_name": parsedVM.DisplayName,
		"cpus":    parsedVM.CPUs,
		"memory_mb": parsedVM.MemoryMB,
	})

	vmProfile := &compliance.VMProfile{
		Role:              "generic", // VMX import defaults to generic; user can change later
		CPUs:              parsedVM.CPUs,
		CPUAllocation:     "shared",
		MemoryBytes:       int64(parsedVM.MemoryMB) * 1024 * 1024,
		HugepagesEnabled:  false,
		BallooningAllowed: true,
		SwapAllowed:       true,
	}

	complianceResult := compliance.ValidateHANAProfile(vmProfile)
	s.migrationJobTracker.Update(job.ID, "validating", map[string]interface{}{
		"vm_name":     parsedVM.DisplayName,
		"cpus":        parsedVM.CPUs,
		"memory_mb":   parsedVM.MemoryMB,
		"compliance":  complianceResult.Passed,
		"violations":  len(complianceResult.Violations),
	})

	// Create VM via migration handler
	s.migrationJobTracker.Update(job.ID, "creating", map[string]interface{}{
		"vm_name": parsedVM.DisplayName,
	})

	if s.migrationHandler == nil {
		s.migrationJobTracker.Update(job.ID, "failed", map[string]interface{}{"error": "migration handler not configured"})
		s.respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "migration handler not available"})
		return
	}

	spec := migration.VMXImportSpec{
		ParsedVM:  parsedVM,
		HostID:    hostID,
		ClusterID: clusterID,
	}

	vmID, err := s.migrationHandler.CreateVMFromVMX(r.Context(), spec, claims.TenantID)
	if err != nil {
		s.migrationJobTracker.Update(job.ID, "failed", map[string]interface{}{"error": "create VM: " + err.Error()})
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create VM: " + err.Error()})
		return
	}

	s.migrationJobTracker.Update(job.ID, "completed", map[string]interface{}{
		"vm_id":   vmID,
		"vm_name": parsedVM.DisplayName,
	})

	s.respondJSON(w, http.StatusCreated, map[string]interface{}{
		"job_id":  job.ID,
		"vm_id":   vmID,
		"vm_name": parsedVM.DisplayName,
		"status":  "completed",
	})
}

// handleGetMigrationJob returns the status of a migration job.
func (s *APIServer) handleGetMigrationJob(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	jobID := r.PathValue("jobId")
	if jobID == "" {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "job ID is required"})
		return
	}

	job, ok := s.migrationJobTracker.Get(jobID)
	if !ok {
		s.respondJSON(w, http.StatusNotFound, map[string]string{"error": "migration job not found"})
		return
	}

	s.respondJSON(w, http.StatusOK, job)
}

// handleListMigrationJobs returns all migration jobs.
func (s *APIServer) handleListMigrationJobs(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.ClaimsFromContext(r.Context()); !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	jobs := s.migrationJobTracker.List()
	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"jobs": jobs,
		"count": len(jobs),
	})
}

// handleVMXPrecheck performs a pre-flight compliance check on a VMX file
// without actually creating the VM.
func (s *APIServer) handleVMXPrecheck(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to parse form: " + err.Error()})
		return
	}

	// Get the uploaded VMX file
	file, header, err := r.FormFile("vmx_file")
	if err != nil {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "vmx_file is required: " + err.Error()})
		return
	}
	defer file.Close()

	// Validate file extension
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".vmx") {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "file must have .vmx extension"})
		return
	}

	// Write to temp file
	tmpDir, err := os.MkdirTemp("", "vmx-precheck-*")
	if err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	defer os.RemoveAll(tmpDir)

	tmpPath := filepath.Join(tmpDir, header.Filename)
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	if _, err := io.Copy(tmpFile, file); err != nil {
		tmpFile.Close()
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	tmpFile.Close()

	// Parse VMX
	parsedVM, err := migration.ParseVMX(tmpPath)
	if err != nil {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to parse VMX: " + err.Error()})
		return
	}

	// Build compliance profile from parsed VMX
	role := r.FormValue("role")
	if role == "" {
		role = "generic"
	}
	cpuAllocation := r.FormValue("cpu_allocation")
	if cpuAllocation == "" {
		cpuAllocation = "shared"
	}

	vmProfile := &compliance.VMProfile{
		Role:              role,
		CPUs:              parsedVM.CPUs,
		CPUAllocation:     cpuAllocation,
		MemoryBytes:       int64(parsedVM.MemoryMB) * 1024 * 1024,
		HugepagesEnabled:  r.FormValue("hugepages") == "true",
		BallooningAllowed: r.FormValue("ballooning") != "false",
		SwapAllowed:       r.FormValue("swap") == "true",
	}

	result := compliance.ValidateHANAProfile(vmProfile)

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"file":        header.Filename,
		"vm_name":     parsedVM.DisplayName,
		"guest_os":    parsedVM.GuestOS,
		"cpus":        parsedVM.CPUs,
		"memory_mb":   parsedVM.MemoryMB,
		"disks":       len(parsedVM.Disks),
		"networks":    len(parsedVM.Networks),
		"role":        role,
		"compliant":   result.Passed,
		"violations":  result.Violations,
		"tenant_id":   claims.TenantID,
	})
}

// VMXImportResult holds the result of a VMX import operation.
type VMXImportResult struct {
	JobID   string `json:"job_id"`
	VMID    string `json:"vm_id"`
	VMName  string `json:"vm_name"`
	Status  string `json:"status"`
}

// Ensure db.VM import is used (avoid unused import).
var _ = db.VM{}
