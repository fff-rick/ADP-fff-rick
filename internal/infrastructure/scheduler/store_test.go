package scheduler

import (
	"testing"
	"time"

	"adp/internal/domain/model"
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

func TestApprovalLifecycle(t *testing.T) {
	store := NewStore()

	job := store.CreateJobWithOptions("backup", "shell", "mysqldump demo", CreateJobOptions{
		Status:           model.JobStatusWaitingApproval,
		RiskLevel:        model.RiskLevelMedium,
		ApprovalRequired: true,
		ApprovalStatus:   model.ApprovalStatusPending,
	})

	pending := store.ListPendingApprovalJobs()
	if len(pending) != 1 || pending[0].ID != job.ID {
		t.Fatalf("expected waiting approval job to be listed")
	}

	approved, err := store.ApproveJob(job.ID, "admin", "looks safe")
	if err != nil {
		t.Fatalf("ApproveJob() error = %v", err)
	}
	if approved.Status != model.JobStatusQueued {
		t.Fatalf("approved status = %s, want queued", approved.Status)
	}
	if approved.ApprovalStatus != model.ApprovalStatusApproved {
		t.Fatalf("approval status = %s, want approved", approved.ApprovalStatus)
	}

	rejectJob := store.CreateJobWithOptions("restart", "shell", "systemctl restart nginx", CreateJobOptions{
		Status:           model.JobStatusWaitingApproval,
		RiskLevel:        model.RiskLevelHigh,
		ApprovalRequired: true,
		ApprovalStatus:   model.ApprovalStatusPending,
	})
	rejected, err := store.RejectJob(rejectJob.ID, "admin", "not now")
	if err != nil {
		t.Fatalf("RejectJob() error = %v", err)
	}
	if rejected.Status != model.JobStatusCancelled {
		t.Fatalf("rejected status = %s, want cancelled", rejected.Status)
	}
	if rejected.ApprovalStatus != model.ApprovalStatusRejected {
		t.Fatalf("approval status = %s, want rejected", rejected.ApprovalStatus)
	}
}

func TestAuditLogs(t *testing.T) {
	store := NewStore()

	entry := store.AddAuditLog("user", "admin", "job.created", "job", "job-1", map[string]any{
		"risk_level": "medium",
	})
	if entry.ID == "" {
		t.Fatal("expected audit log to have an id")
	}

	logs := store.ListAuditLogs("job", "job-1")
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if logs[0].Action != "job.created" {
		t.Fatalf("unexpected action: %s", logs[0].Action)
	}
}

func TestIncidentCasesAndSuggestions(t *testing.T) {
	store := NewStore()

	now := time.Now()
	plan := model.DiagnosisPlan{
		ID:          "plan-1",
		Title:       "Nginx 不可访问诊断",
		Description: "nginx 无法访问",
		TriggerType: "nginx_unreachable",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	report := model.AnalysisReport{
		PlanID:         plan.ID,
		FaultType:      "Nginx 服务异常",
		PossibleCauses: []string{"Nginx 进程未运行"},
		Suggestions:    []string{"启动 nginx"},
		Confidence:     0.9,
		RawAnalysis:    "Nginx 进程未运行",
		CreatedAt:      now,
	}

	saved := store.UpsertIncidentCase(plan, report)
	if saved.ID == "" {
		t.Fatal("expected incident case to have an id")
	}

	report.Suggestions = []string{"systemctl start nginx"}
	updated := store.UpsertIncidentCase(plan, report)
	if updated.ID != saved.ID {
		t.Fatalf("expected upsert to reuse id, got %s vs %s", updated.ID, saved.ID)
	}

	listed := store.ListIncidentCases(model.IncidentCaseFilter{
		TriggerType: "nginx_unreachable",
	})
	if len(listed) != 1 {
		t.Fatalf("expected 1 case, got %d", len(listed))
	}
	if listed[0].Suggestions[0] != "systemctl start nginx" {
		t.Fatalf("unexpected suggestion: %v", listed[0].Suggestions)
	}

	similar := store.FindSimilarIncidentCases("nginx 页面无法访问", "nginx_unreachable", "Nginx 服务异常", 3)
	if len(similar) != 1 {
		t.Fatalf("expected 1 similar case, got %d", len(similar))
	}
}

func TestMetricsSnapshot(t *testing.T) {
	store := NewStore()
	worker := store.RegisterWorker("worker-1", "shell")

	job1 := store.CreateJob("job-1", "shell", "echo 1")
	job2 := store.CreateJob("job-2", "shell", "echo 2")
	store.CreateJobWithOptions("job-3", "shell", "echo 3", CreateJobOptions{
		Status:           model.JobStatusWaitingApproval,
		ApprovalRequired: true,
		ApprovalStatus:   model.ApprovalStatusPending,
	})

	assigned1, ok := store.AssignNextJob(worker.ID)
	if !ok || assigned1.ID != job1.ID {
		t.Fatalf("expected first job to be assigned")
	}
	if _, err := store.CompleteJob(worker.ID, job1.ID, "done", true); err != nil {
		t.Fatalf("CompleteJob(job1) error = %v", err)
	}

	assigned2, ok := store.AssignNextJob(worker.ID)
	if !ok || assigned2.ID != job2.ID {
		t.Fatalf("expected second job to be assigned")
	}
	if _, err := store.CompleteJob(worker.ID, job2.ID, "failed", false); err != nil {
		t.Fatalf("CompleteJob(job2) error = %v", err)
	}

	snapshot := store.MetricsSnapshot()
	if snapshot.JobsTotal != 3 {
		t.Fatalf("JobsTotal = %d, want 3", snapshot.JobsTotal)
	}
	if snapshot.JobsSuccess != 1 {
		t.Fatalf("JobsSuccess = %d, want 1", snapshot.JobsSuccess)
	}
	if snapshot.JobsFailed != 1 {
		t.Fatalf("JobsFailed = %d, want 1", snapshot.JobsFailed)
	}
	if snapshot.JobsWaitingApproval != 1 {
		t.Fatalf("JobsWaitingApproval = %d, want 1", snapshot.JobsWaitingApproval)
	}
	if snapshot.WorkersOnline != 1 {
		t.Fatalf("WorkersOnline = %d, want 1", snapshot.WorkersOnline)
	}
	if snapshot.JobSuccessRate != 0.5 {
		t.Fatalf("JobSuccessRate = %f, want 0.5", snapshot.JobSuccessRate)
	}
	if snapshot.JobFailureRate != 0.5 {
		t.Fatalf("JobFailureRate = %f, want 0.5", snapshot.JobFailureRate)
	}
	if snapshot.AvgScheduleLatencySeconds < 0 {
		t.Fatalf("AvgScheduleLatencySeconds = %f, want >= 0", snapshot.AvgScheduleLatencySeconds)
	}
}
