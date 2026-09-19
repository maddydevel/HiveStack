// Package migration tracks and orchestrates VMware vCenter VM import jobs.
package migration

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/maddydevel/HiveStack/internal/vcenter"
)

// JobState represents the current phase of a migration job.
type JobState string

const (
	// JobStateQueued is the initial state when a job is created.
	JobStateQueued JobState = "queued"
	// JobStateDiscovering connects to vCenter and retrieves inventory.
	JobStateDiscovering JobState = "discovering"
	// JobStatePreflight runs compatibility checks on selected VMs.
	JobStatePreflight JobState = "preflight"
	// JobStateImporting performs the actual VM data copy and conversion.
	JobStateImporting JobState = "importing"
	// JobStateValidating verifies imported VMs boot correctly.
	JobStateValidating JobState = "validating"
	// JobStateCompleted indicates successful completion.
	JobStateCompleted JobState = "completed"
	// JobStateFailed indicates the job failed (see Error field).
	JobStateFailed JobState = "failed"
	// JobStateCancelled indicates the job was cancelled by operator.
	JobStateCancelled JobState = "cancelled"
)

// IsValid returns true if s is a recognized job state.
func (s JobState) IsValid() bool {
	switch s {
	case JobStateQueued, JobStateDiscovering, JobStatePreflight,
		JobStateImporting, JobStateValidating, JobStateCompleted,
		JobStateFailed, JobStateCancelled:
		return true
	}
	return false
}

// String implements fmt.Stringer.
func (s JobState) String() string { return string(s) }

// VMImportStatus tracks the progress of a single VM import.
type VMImportStatus struct {
	VMID         string        `json:"vm_id"`
	VMName       string        `json:"vm_name"`
	State        string        `json:"state"` // "pending", "copying", "converting", "booting", "done", "failed"
	Progress     float64       `json:"progress"` // 0-100
	BytesTotal   int64         `json:"bytes_total"`
	BytesCopied  int64         `json:"bytes_copied"`
	StartedAt    *time.Time    `json:"started_at,omitempty"`
	CompletedAt  *time.Time    `json:"completed_at,omitempty"`
	Error        string        `json:"error,omitempty"`
	PreflightChecks []CheckResult `json:"preflight_checks,omitempty"`
}

// CheckResult captures the outcome of a pre-flight compatibility check.
type CheckResult struct {
	Name      string `json:"name"`
	Passed    bool   `json:"passed"`
	Severity  string `json:"severity"` // "info", "warning", "critical"
	Message   string `json:"message"`
}

// Job represents a single vCenter import migration.
type Job struct {
	ID          string                     `json:"id"`
	State       JobState                  `json:"state"`
	Source      vcenter.ClientConfig      `json:"source"`
	VMStatuses  map[string]*VMImportStatus `json:"vm_statuses"`
	VMIDs       []string                   `json:"vm_ids"`
	TotalBytes  int64                     `json:"total_bytes"`
	CopiedBytes int64                     `json:"copied_bytes"`
	Error       string                    `json:"error,omitempty"`
	CreatedAt   time.Time                 `json:"created_at"`
	UpdatedAt   time.Time                 `json:"updated_at"`
	StartedAt   *time.Time                `json:"started_at,omitempty"`
	CompletedAt *time.Time                `json:"completed_at,omitempty"`
}

// Progress returns overall job progress as a percentage (0-100).
func (j *Job) Progress() float64 {
	if j.TotalBytes == 0 {
		return 0
	}
	return float64(j.CopiedBytes) / float64(j.TotalBytes) * 100
}

// VMCount returns the total number of VMs in the job.
func (j *Job) VMCount() int {
	return len(j.VMIDs)
}

// CompletedVMs returns the count of VMs in "done" state.
func (j *Job) CompletedVMs() int {
	count := 0
	for _, v := range j.VMStatuses {
		if v.State == "done" {
			count++
		}
	}
	return count
}

// FailedVMs returns the count of VMs in "failed" state.
func (j *Job) FailedVMs() int {
	count := 0
	for _, v := range j.VMStatuses {
		if v.State == "failed" {
			count++
		}
	}
	return count
}

// Tracker manages migration jobs.
type Tracker struct {
	mu    sync.RWMutex
	jobs  map[string]*Job
	order []string // job creation order
}

// NewTracker creates a migration job tracker.
func NewTracker() *Tracker {
	return &Tracker{
		jobs:  make(map[string]*Job),
		order: make([]string, 0),
	}
}

// CreateJob creates a new migration job in queued state.
func (t *Tracker) CreateJob(id string, source vcenter.ClientConfig, vmIDs []string) (*Job, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.jobs[id]; exists {
		return nil, fmt.Errorf("migration job %s already exists", id)
	}

	now := time.Now()
	job := &Job{
		ID:         id,
		State:      JobStateQueued,
		Source:     source,
		VMIDs:      vmIDs,
		VMStatuses: make(map[string]*VMImportStatus),
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	for _, vmID := range vmIDs {
		job.VMStatuses[vmID] = &VMImportStatus{
			VMID:  vmID,
			State: "pending",
		}
	}

	t.jobs[id] = job
	t.order = append(t.order, id)

	log.Printf("[Migration] Created job %s with %d VM(s)", id, len(vmIDs))
	return job, nil
}

// GetJob returns a copy of the job by ID.
func (t *Tracker) GetJob(id string) (*Job, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	job, ok := t.jobs[id]
	if !ok {
		return nil, false
	}
	return copyJob(job), true
}

// ListJobs returns all jobs in creation order.
func (t *Tracker) ListJobs() []*Job {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]*Job, 0, len(t.order))
	for _, id := range t.order {
		if job, ok := t.jobs[id]; ok {
			result = append(result, copyJob(job))
		}
	}
	return result
}

// ListJobsByState returns jobs filtered by state.
func (t *Tracker) ListJobsByState(state JobState) []*Job {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]*Job, 0)
	for _, id := range t.order {
		if job, ok := t.jobs[id]; ok && job.State == state {
			result = append(result, copyJob(job))
		}
	}
	return result
}

// UpdateJobState transitions a job to a new state.
func (t *Tracker) UpdateJobState(id string, state JobState) error {
	if !state.IsValid() {
		return fmt.Errorf("invalid job state: %s", state)
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	job, ok := t.jobs[id]
	if !ok {
		return fmt.Errorf("migration job %s not found", id)
	}

	now := time.Now()
	oldState := job.State
	job.State = state
	job.UpdatedAt = now

	if state == JobStateImporting && job.StartedAt == nil {
		job.StartedAt = &now
	}
	if state == JobStateCompleted || state == JobStateFailed || state == JobStateCancelled {
		job.CompletedAt = &now
	}

	log.Printf("[Migration] Job %s state: %s -> %s", id, oldState, state)
	return nil
}

// UpdateVMStatus updates the import status of a VM within a job.
func (t *Tracker) UpdateVMStatus(jobID, vmID string, update func(*VMImportStatus)) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	job, ok := t.jobs[jobID]
	if !ok {
		return fmt.Errorf("migration job %s not found", jobID)
	}

	status, ok := job.VMStatuses[vmID]
	if !ok {
		return fmt.Errorf("VM %s not in job %s", vmID, jobID)
	}

	update(status)
	job.UpdatedAt = time.Now()
	return nil
}

// AddCopiedBytes adds to the copied byte counter for progress tracking.
func (t *Tracker) AddCopiedBytes(jobID string, bytes int64) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	job, ok := t.jobs[jobID]
	if !ok {
		return fmt.Errorf("migration job %s not found", jobID)
	}

	job.CopiedBytes += bytes
	job.UpdatedAt = time.Now()
	return nil
}

// SetTotalBytes sets the total bytes for a job.
func (t *Tracker) SetTotalBytes(jobID string, bytes int64) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	job, ok := t.jobs[jobID]
	if !ok {
		return fmt.Errorf("migration job %s not found", jobID)
	}

	job.TotalBytes = bytes
	job.UpdatedAt = time.Now()
	return nil
}

// SetJobError marks a job as failed with the given error.
func (t *Tracker) SetJobError(id string, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	job, ok := t.jobs[id]
	if !ok {
		return
	}

	now := time.Now()
	job.State = JobStateFailed
	job.Error = err.Error()
	job.CompletedAt = &now
	job.UpdatedAt = now
	log.Printf("[Migration] Job %s failed: %v", id, err)
}

// CancelJob cancels a running job.
func (t *Tracker) CancelJob(id string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	job, ok := t.jobs[id]
	if !ok {
		return fmt.Errorf("migration job %s not found", id)
	}

	if job.State == JobStateCompleted || job.State == JobStateFailed || job.State == JobStateCancelled {
		return fmt.Errorf("cannot cancel job in state %s", job.State)
	}

	now := time.Now()
	job.State = JobStateCancelled
	job.CompletedAt = &now
	job.UpdatedAt = now
	log.Printf("[Migration] Job %s cancelled", id)
	return nil
}

// DeleteJob removes a job from the tracker.
func (t *Tracker) DeleteJob(id string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, ok := t.jobs[id]; !ok {
		return fmt.Errorf("migration job %s not found", id)
	}

	delete(t.jobs, id)
	for i, o := range t.order {
		if o == id {
			t.order = append(t.order[:i], t.order[i+1:]...)
			break
		}
	}
	return nil
}

// ActiveJobs returns the count of non-terminal jobs.
func (t *Tracker) ActiveJobs() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	count := 0
	for _, job := range t.jobs {
		if job.State != JobStateCompleted && job.State != JobStateFailed && job.State != JobStateCancelled {
			count++
		}
	}
	return count
}

// copyJob returns a deep copy of a Job.
func copyJob(j *Job) *Job {
	c := *j
	c.VMIDs = make([]string, len(j.VMIDs))
	copy(c.VMIDs, j.VMIDs)

	c.VMStatuses = make(map[string]*VMImportStatus, len(j.VMStatuses))
	for k, v := range j.VMStatuses {
		vc := *v
		if v.PreflightChecks != nil {
			vc.PreflightChecks = make([]CheckResult, len(v.PreflightChecks))
			copy(vc.PreflightChecks, v.PreflightChecks)
		}
		c.VMStatuses[k] = &vc
	}

	if j.StartedAt != nil {
		t := *j.StartedAt
		c.StartedAt = &t
	}
	if j.CompletedAt != nil {
		t := *j.CompletedAt
		c.CompletedAt = &t
	}
	return &c
}

// ProgressUpdate is a snapshot of job progress for reporting.
type ProgressUpdate struct {
	JobID       string    `json:"job_id"`
	State       JobState  `json:"state"`
	Progress    float64   `json:"progress"`
	TotalVMs    int       `json:"total_vms"`
	DoneVMs     int       `json:"done_vms"`
	FailedVMs   int       `json:"failed_vm"`
	BytesTotal  int64     `json:"bytes_total"`
	BytesCopied int64     `json:"bytes_copied"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// GetProgress returns a progress snapshot for a job.
func (t *Tracker) GetProgress(jobID string) (*ProgressUpdate, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	job, ok := t.jobs[jobID]
	if !ok {
		return nil, false
	}

	return &ProgressUpdate{
		JobID:       job.ID,
		State:       job.State,
		Progress:    job.Progress(),
		TotalVMs:    job.VMCount(),
		DoneVMs:     job.CompletedVMs(),
		FailedVMs:   job.FailedVMs(),
		BytesTotal:  job.TotalBytes,
		BytesCopied: job.CopiedBytes,
		UpdatedAt:   job.UpdatedAt,
	}, true
}

// JobExecutor runs migration jobs asynchronously.
type JobExecutor struct {
	tracker *Tracker
	client  *vcenter.Client
}

// NewJobExecutor creates a new executor.
func NewJobExecutor(tracker *Tracker, client *vcenter.Client) *JobExecutor {
	return &JobExecutor{
		tracker: tracker,
		client:  client,
	}
}

// Run starts executing a job asynchronously.
func (e *JobExecutor) Run(ctx context.Context, jobID string) {
	go e.execute(ctx, jobID)
}

func (e *JobExecutor) execute(ctx context.Context, jobID string) {
	job, ok := e.tracker.GetJob(jobID)
	if !ok {
		log.Printf("[Migration] Job %s not found for execution", jobID)
		return
	}

	log.Printf("[Migration] Starting execution of job %s", jobID)

	// Transition to discovering
	if err := e.tracker.UpdateJobState(jobID, JobStateDiscovering); err != nil {
		e.tracker.SetJobError(jobID, err)
		return
	}

	// Connect to vCenter
	if err := e.client.Connect(ctx); err != nil {
		e.tracker.SetJobError(jobID, fmt.Errorf("connect to vCenter: %w", err))
		return
	}
	defer e.client.Disconnect()

	// Discover inventory
	discovery, err := e.client.Discover(ctx)
	if err != nil {
		e.tracker.SetJobError(jobID, fmt.Errorf("discover inventory: %w", err))
		return
	}

	// Validate requested VMs exist
	totalBytes := int64(0)
	for _, vmID := range job.VMIDs {
		found := false
		for _, dc := range discovery.Datacenters {
			for _, vm := range dc.VMs {
				if vm.ID == vmID {
					found = true
					totalBytes += vm.DiskGB * 1024 * 1024 * 1024
					e.tracker.UpdateVMStatus(jobID, vmID, func(s *VMImportStatus) {
						s.VMName = vm.Name
					})
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			e.tracker.SetJobError(jobID, fmt.Errorf("VM %s not found in vCenter inventory", vmID))
			return
		}
	}
	e.tracker.SetTotalBytes(jobID, totalBytes)

	// Transition to preflight
	if err := e.tracker.UpdateJobState(jobID, JobStatePreflight); err != nil {
		e.tracker.SetJobError(jobID, err)
		return
	}

	// Run pre-flight checks (simulated)
	for _, vmID := range job.VMIDs {
		e.tracker.UpdateVMStatus(jobID, vmID, func(s *VMImportStatus) {
			s.PreflightChecks = []CheckResult{
				{Name: "cpu_compat", Passed: true, Severity: "info", Message: "CPU features compatible with KVM"},
				{Name: "memory_hotplug", Passed: true, Severity: "info", Message: "Memory configuration supported"},
				{Name: "disk_format", Passed: true, Severity: "warning", Message: "VMDK will be converted to QCOW2"},
				{Name: "network_mapping", Passed: true, Severity: "info", Message: "Network 'VM Network' mapped to 'br0'"},
				{Name: "tools_check", Passed: true, Severity: "info", Message: "VMware Tools status: toolsOk"},
			}
		})
	}

	// Transition to importing
	if err := e.tracker.UpdateJobState(jobID, JobStateImporting); err != nil {
		e.tracker.SetJobError(jobID, err)
		return
	}

	// Simulate import progress
	for _, vmID := range job.VMIDs {
		e.tracker.UpdateVMStatus(jobID, vmID, func(s *VMImportStatus) {
			now := time.Now()
			s.StartedAt = &now
			s.State = "copying"
		})

		// Simulate copy progress (4 steps: 25, 50, 75, 100)
		for progress := float64(25); progress <= 100; progress += 25 {
			select {
			case <-ctx.Done():
				e.tracker.SetJobError(jobID, ctx.Err())
				return
			case <-time.After(10 * time.Millisecond):
			}
			e.tracker.UpdateVMStatus(jobID, vmID, func(s *VMImportStatus) {
				s.Progress = progress
			})
			e.tracker.AddCopiedBytes(jobID, totalBytes/4)
		}

		e.tracker.UpdateVMStatus(jobID, vmID, func(s *VMImportStatus) {
			s.State = "done"
			s.Progress = 100
			now := time.Now()
			s.CompletedAt = &now
		})
	}

	// Transition to validating
	if err := e.tracker.UpdateJobState(jobID, JobStateValidating); err != nil {
		e.tracker.SetJobError(jobID, err)
		return
	}

	// Simulated validation
	select {
	case <-ctx.Done():
		e.tracker.SetJobError(jobID, ctx.Err())
		return
	case <-time.After(10 * time.Millisecond):
	}

	// Complete
	if err := e.tracker.UpdateJobState(jobID, JobStateCompleted); err != nil {
		e.tracker.SetJobError(jobID, err)
		return
	}

	log.Printf("[Migration] Job %s completed successfully", jobID)
}
