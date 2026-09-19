package migration

import (
	"context"
	"testing"
	"time"

	"github.com/maddydevel/HiveStack/internal/vcenter"
)

func TestTrackerCreateJob(t *testing.T) {
	tracker := NewTracker()

	cfg := vcenter.ClientConfig{
		Host:     "vcenter.local",
		Username: "admin",
	}

	job, err := tracker.CreateJob("test-job-1", cfg, []string{"vm-1", "vm-2"})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	if job.ID != "test-job-1" {
		t.Errorf("expected job ID 'test-job-1', got %s", job.ID)
	}
	if job.State != JobStateQueued {
		t.Errorf("expected state %s, got %s", JobStateQueued, job.State)
	}
	if len(job.VMIDs) != 2 {
		t.Errorf("expected 2 VMs, got %d", len(job.VMIDs))
	}

	// Duplicate ID should fail
	_, err = tracker.CreateJob("test-job-1", cfg, []string{"vm-3"})
	if err == nil {
		t.Error("expected error for duplicate job ID")
	}
}

func TestTrackerJobLifecycle(t *testing.T) {
	tracker := NewTracker()
	cfg := vcenter.ClientConfig{Host: "vcenter.local", Username: "admin"}

	job, _ := tracker.CreateJob("lifecycle-job", cfg, []string{"vm-1"})

	// Update state
	if err := tracker.UpdateJobState(job.ID, JobStateDiscovering); err != nil {
		t.Fatalf("UpdateJobState: %v", err)
	}

	updated, _ := tracker.GetJob(job.ID)
	if updated.State != JobStateDiscovering {
		t.Errorf("expected state %s, got %s", JobStateDiscovering, updated.State)
	}

	// Update VM status
	if err := tracker.UpdateVMStatus(job.ID, "vm-1", func(s *VMImportStatus) {
		s.State = "copying"
		s.Progress = 50
	}); err != nil {
		t.Fatalf("UpdateVMStatus: %v", err)
	}

	job2, _ := tracker.GetJob(job.ID)
	if job2.VMStatuses["vm-1"].State != "copying" {
		t.Errorf("expected VM state 'copying', got %s", job2.VMStatuses["vm-1"].State)
	}

	// Complete job
	if err := tracker.UpdateJobState(job.ID, JobStateCompleted); err != nil {
		t.Fatalf("UpdateJobState to completed: %v", err)
	}

	job3, _ := tracker.GetJob(job.ID)
	if job3.State != JobStateCompleted {
		t.Errorf("expected state %s, got %s", JobStateCompleted, job3.State)
	}
	if job3.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestTrackerProgress(t *testing.T) {
	tracker := NewTracker()
	cfg := vcenter.ClientConfig{Host: "vcenter.local", Username: "admin"}

	job, _ := tracker.CreateJob("progress-job", cfg, []string{"vm-1", "vm-2"})
	tracker.SetTotalBytes(job.ID, 1000)

	tracker.AddCopiedBytes(job.ID, 250)
	tracker.UpdateVMStatus(job.ID, "vm-1", func(s *VMImportStatus) {
		s.State = "done"
		s.Progress = 100
	})

	progress, ok := tracker.GetProgress(job.ID)
	if !ok {
		t.Fatal("GetProgress returned false")
	}

	if progress.BytesCopied != 250 {
		t.Errorf("expected 250 bytes copied, got %d", progress.BytesCopied)
	}
	if progress.Progress != 25.0 {
		t.Errorf("expected 25%% progress, got %f", progress.Progress)
	}
	if progress.DoneVMs != 1 {
		t.Errorf("expected 1 done VM, got %d", progress.DoneVMs)
	}
}

func TestTrackerCancel(t *testing.T) {
	tracker := NewTracker()
	cfg := vcenter.ClientConfig{Host: "vcenter.local", Username: "admin"}

	job, _ := tracker.CreateJob("cancel-job", cfg, []string{"vm-1"})
	tracker.UpdateJobState(job.ID, JobStateImporting)

	if err := tracker.CancelJob(job.ID); err != nil {
		t.Fatalf("CancelJob: %v", err)
	}

	updated, _ := tracker.GetJob(job.ID)
	if updated.State != JobStateCancelled {
		t.Errorf("expected state %s, got %s", JobStateCancelled, updated.State)
	}

	// Cannot cancel completed job
	tracker2 := NewTracker()
	job2, _ := tracker2.CreateJob("completed-job", cfg, []string{"vm-1"})
	tracker2.UpdateJobState(job2.ID, JobStateCompleted)
	err := tracker2.CancelJob(job2.ID)
	if err == nil {
		t.Error("expected error cancelling completed job")
	}
}

func TestTrackerListJobs(t *testing.T) {
	tracker := NewTracker()
	cfg := vcenter.ClientConfig{Host: "vcenter.local", Username: "admin"}

	tracker.CreateJob("job-1", cfg, []string{"vm-1"})
	tracker.CreateJob("job-2", cfg, []string{"vm-2"})
	tracker.CreateJob("job-3", cfg, []string{"vm-3"})

	jobs := tracker.ListJobs()
	if len(jobs) != 3 {
		t.Errorf("expected 3 jobs, got %d", len(jobs))
	}

	// Filter by state
	queued := tracker.ListJobsByState(JobStateQueued)
	if len(queued) != 3 {
		t.Errorf("expected 3 queued jobs, got %d", len(queued))
	}
}

func TestJobState(t *testing.T) {
	tests := []struct {
		state JobState
		valid bool
	}{
		{JobStateQueued, true},
		{JobStateDiscovering, true},
		{JobStatePreflight, true},
		{JobStateImporting, true},
		{JobStateValidating, true},
		{JobStateCompleted, true},
		{JobStateFailed, true},
		{JobStateCancelled, true},
		{JobState("invalid"), false},
		{JobState(""), false},
	}

	for _, tt := range tests {
		if tt.state.IsValid() != tt.valid {
			t.Errorf("JobState(%q).IsValid() = %v, want %v", tt.state, tt.state.IsValid(), tt.valid)
		}
	}
}

func TestJobProgressCalc(t *testing.T) {
	job := &Job{TotalBytes: 1000, CopiedBytes: 250}
	if job.Progress() != 25.0 {
		t.Errorf("expected 25%% progress, got %f", job.Progress())
	}

	job2 := &Job{TotalBytes: 0}
	if job2.Progress() != 0 {
		t.Errorf("expected 0%% progress with no total, got %f", job2.Progress())
	}
}

func TestJobExecutor(t *testing.T) {
	tracker := NewTracker()
	cfg := vcenter.ClientConfig{
		Host:     "vcenter.local",
		Username: "admin",
	}
	client, err := vcenter.NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	job, _ := tracker.CreateJob("exec-job", cfg, []string{"vm-1001"})

	executor := NewJobExecutor(tracker, client)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	executor.Run(ctx, job.ID)

	// Wait for job to complete
	for i := 0; i < 100; i++ {
		j, _ := tracker.GetJob(job.ID)
		if j.State == JobStateCompleted || j.State == JobStateFailed {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	finalJob, _ := tracker.GetJob(job.ID)
	if finalJob.State != JobStateCompleted {
		t.Errorf("expected job to complete, got state %s (error: %s)", finalJob.State, finalJob.Error)
	}

	// Check progress
	progress, _ := tracker.GetProgress(job.ID)
	if progress.Progress != 100 {
		t.Errorf("expected 100%% progress, got %f", progress.Progress)
	}
}
