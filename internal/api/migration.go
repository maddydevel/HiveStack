package api

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ImportJobState represents the state of an import job.
type ImportJobState string

const (
	ImportJobPending   ImportJobState = "pending"
	ImportJobPreflight ImportJobState = "preflight"
	ImportJobParsing   ImportJobState = "parsing"
	ImportJobImporting ImportJobState = "importing"
	ImportJobCompleted ImportJobState = "completed"
	ImportJobFailed    ImportJobState = "failed"
	ImportJobCancelled ImportJobState = "cancelled"
)

// ImportJob tracks the progress of an OVF/OVA import.
type ImportJob struct {
	ID          string            `json:"id"`
	State       ImportJobState    `json:"state"`
	SourceURL   string            `json:"source_url"`
	OVFInfo     *ParsedOVFResult  `json:"ovf_info,omitempty"`
	Progress    int               `json:"progress_percent"` // 0-100
	StartedAt   time.Time         `json:"started_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
	Error       string            `json:"error,omitempty"`
	TargetHost  string            `json:"target_host,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Preflight   *PreflightResult  `json:"preflight,omitempty"`
}

// PreflightResult holds the results of a compatibility pre-flight check.
type PreflightResult struct {
	Compatible bool      `json:"compatible"`
	Checks     []Check   `json:"checks"`
	CheckedAt  time.Time `json:"checked_at"`
}

// Check is a single pre-flight compatibility check result.
type Check struct {
	Name     string `json:"name"`
	Passed   bool   `json:"passed"`
	Severity string `json:"severity"` // "error", "warning", "info"
	Message  string `json:"message"`
}

// MigrationService manages import jobs and migration operations.
type MigrationService struct {
	mu    sync.Mutex
	jobs  map[string]*ImportJob
	hosts []string
}

// NewMigrationService creates a new migration service.
func NewMigrationService(hosts []string) *MigrationService {
	return &MigrationService{
		jobs:  make(map[string]*ImportJob),
		hosts: hosts,
	}
}

// CreateImportJob creates a new import job.
func (s *MigrationService) CreateImportJob(sourceURL, targetHost string, labels map[string]string) *ImportJob {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	job := &ImportJob{
		ID:         fmt.Sprintf("import-%d", now.UnixNano()),
		State:      ImportJobPending,
		SourceURL:  sourceURL,
		TargetHost: targetHost,
		Labels:     labels,
		Progress:   0,
		StartedAt:  now,
		UpdatedAt:  now,
	}
	s.jobs[job.ID] = job
	return job
}

// GetImportJob retrieves an import job by ID.
func (s *MigrationService) GetImportJob(id string) (*ImportJob, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	return job, ok
}

// ListImportJobs returns all import jobs.
func (s *MigrationService) ListImportJobs() []ImportJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	jobs := make([]ImportJob, 0, len(s.jobs))
	for _, j := range s.jobs {
		jobs = append(jobs, *j)
	}
	return jobs
}

// UpdateJobState updates the state and progress of an import job.
func (s *MigrationService) UpdateJobState(id string, state ImportJobState, progress int, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return fmt.Errorf("import job not found: %s", id)
	}

	job.State = state
	job.Progress = progress
	job.UpdatedAt = time.Now()
	if errMsg != "" {
		job.Error = errMsg
	}
	if state == ImportJobCompleted || state == ImportJobFailed || state == ImportJobCancelled {
		now := time.Now()
		job.CompletedAt = &now
	}
	return nil
}

// CancelImportJob marks an import job as cancelled.
func (s *MigrationService) CancelImportJob(id string) error {
	return s.UpdateJobState(id, ImportJobCancelled, 0, "")
}

// DeleteImportJob removes an import job.
func (s *MigrationService) DeleteImportJob(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.jobs[id]; !ok {
		return fmt.Errorf("import job not found: %s", id)
	}
	delete(s.jobs, id)
	return nil
}

// RunPreflightChecks performs compatibility pre-flight checks for an import job.
func (s *MigrationService) RunPreflightChecks(ctx context.Context, job *ImportJob, parsed *ParsedOVFResult) *PreflightResult {
	checks := []Check{}
	compatible := true

	// Check 1: Target host exists (simplified - check against known hosts)
	targetExists := false
	for _, h := range s.hosts {
		if h == job.TargetHost {
			targetExists = true
			break
		}
	}
	if job.TargetHost != "" && !targetExists {
		checks = append(checks, Check{
			Name:     "target_host_exists",
			Passed:   false,
			Severity: "error",
			Message:  fmt.Sprintf("target host %s is not a known host", job.TargetHost),
		})
		compatible = false
	} else {
		checks = append(checks, Check{
			Name:     "target_host_exists",
			Passed:   true,
			Severity: "info",
			Message:  "target host is valid",
		})
	}

	// Check 2: OVF has at least one VM
	if len(parsed.VMs) == 0 {
		checks = append(checks, Check{
			Name:     "has_vms",
			Passed:   false,
			Severity: "error",
			Message:  "OVF contains no virtual systems",
		})
		compatible = false
	} else {
		checks = append(checks, Check{
			Name:     "has_vms",
			Passed:   true,
			Severity: "info",
			Message:  fmt.Sprintf("OVF contains %d virtual system(s)", len(parsed.VMs)),
		})
	}

	// Check 3: Memory requirements feasible
	totalMemory := uint64(0)
	for _, vm := range parsed.VMs {
		totalMemory += vm.MemoryMB
	}
	if totalMemory > 0 {
		// Simplified: assume hosts have 64GB each
		const hostMemoryMB = 64 * 1024
		if totalMemory > hostMemoryMB {
			checks = append(checks, Check{
				Name:     "memory_feasible",
				Passed:   false,
				Severity: "warning",
				Message:  fmt.Sprintf("total memory requirement %d MB may exceed typical host capacity", totalMemory),
			})
			// Not a hard fail, just a warning
		} else {
			checks = append(checks, Check{
				Name:     "memory_feasible",
				Passed:   true,
				Severity: "info",
				Message:  fmt.Sprintf("total memory %d MB is within expected capacity", totalMemory),
			})
		}
	}

	// Check 4: CPU requirements feasible
	totalCPUs := 0
	for _, vm := range parsed.VMs {
		totalCPUs += vm.CPUs
	}
	if totalCPUs > 0 {
		const maxCPUs = 64
		if totalCPUs > maxCPUs {
			checks = append(checks, Check{
				Name:     "cpu_feasible",
				Passed:   false,
				Severity: "warning",
				Message:  fmt.Sprintf("total CPU count %d may exceed typical host capacity", totalCPUs),
			})
		} else {
			checks = append(checks, Check{
				Name:     "cpu_feasible",
				Passed:   true,
				Severity: "info",
				Message:  fmt.Sprintf("total CPUs %d is within expected capacity", totalCPUs),
			})
		}
	}

	// Check 5: Disk format check
	hasUnknownFormat := false
	for _, d := range parsed.Disks {
		if d.Format != "" && !isKnownDiskFormat(d.Format) {
			hasUnknownFormat = true
		}
	}
	if hasUnknownFormat {
		checks = append(checks, Check{
			Name:     "disk_format",
			Passed:   true,
			Severity: "warning",
			Message:  "some disks use non-standard formats that may need conversion",
		})
	} else if len(parsed.Disks) > 0 {
		checks = append(checks, Check{
			Name:     "disk_format",
			Passed:   true,
			Severity: "info",
			Message:  "all disk formats appear compatible",
		})
	}

	// Check 6: Virtual system type compatibility
	for _, vm := range parsed.VMs {
		if vm.VirtualSystemType != "" && !isKnownVSType(vm.VirtualSystemType) {
			checks = append(checks, Check{
				Name:     "virtual_system_type",
				Passed:   true,
				Severity: "warning",
				Message:  fmt.Sprintf("VM %s uses virtual system type '%s' which may need conversion", vm.Name, vm.VirtualSystemType),
			})
			break
		}
	}

	return &PreflightResult{
		Compatible: compatible,
		Checks:     checks,
		CheckedAt:  time.Now(),
	}
}

// isKnownDiskFormat checks if the disk format string is recognized.
func isKnownDiskFormat(format string) bool {
	known := []string{
		"http://www.vmware.com/specifications/vmdk.html#sparse",
		"http://www.vmware.com/specifications/vmdk.html#streamOptimized",
		"http://www.vmware.com/specifications/vmdk.html#monolithicSparse",
		"http://www.vmware.com/specifications/vmdk.html#monolithicFlat",
		"http://www.vmware.com/specifications/vmdk.html#twoGbMaxExtentSparse",
		"http://www.vmware.com/specifications/vmdk.html#twoGbMaxExtentFlat",
		"http://www.vmware.com/specifications/vmdk.html#vmfs",
		"http://www.vmware.com/specifications/vmdk.html#thin",
		"http://www.vmware.com/specifications/vmdk.html#thick",
		"http://www.vmware.com/specifications/vmdk.html#eagerZeroedThick",
		"http://www.vmware.com/interfaces/specifications/vmdk.html#seSparse",
		"ova:tar",
	}
	for _, k := range known {
		if format == k {
			return true
		}
	}
	// Also allow raw format and others commonly used
	return false
}

// isKnownVSType checks if the virtual system type is recognized.
func isKnownVSType(vsType string) bool {
	known := []string{
		"vmx-07", "vmx-08", "vmx-09", "vmx-10", "vmx-11",
		"vmx-12", "vmx-13", "vmx-14", "vmx-15", "vmx-17",
		"vmx-18", "vmx-19", "vmx-20", "vmx-21",
		"xen", "xenServer",
		"kvm", "qemu",
	}
	for _, k := range known {
		if vsType == k {
			return true
		}
	}
	return false
}
