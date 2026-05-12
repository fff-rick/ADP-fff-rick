package scheduler

import (
	"testing"

	"adp/internal/model"
)

func TestAssignAndCompleteJob(t *testing.T) {
	store := NewStore()
	worker := store.RegisterWorker("worker-1", "shell")
	job := store.CreateJob("job-1", "shell", "echo demo")

	assignedJob, ok := store.AssignNextJob(worker.ID)
	if !ok {
		t.Fatal("expected a job assignment")
	}

	if assignedJob.ID != job.ID {
		t.Fatalf("unexpected job id: %s", assignedJob.ID)
	}

	if assignedJob.Status != model.JobStatusRunning {
		t.Fatalf("unexpected job status: %s", assignedJob.Status)
	}

	completedJob, err := store.CompleteJob(worker.ID, job.ID, "done", true)
	if err != nil {
		t.Fatalf("CompleteJob() error = %v", err)
	}

	if completedJob.Status != model.JobStatusSuccess {
		t.Fatalf("unexpected completion status: %s", completedJob.Status)
	}
}
