package api

import (
	"net/http"
	"sort"
	"time"

	"adp/internal/domain/model"
)

type dashboardSummaryResponse struct {
	User             model.User            `json:"user"`
	CurrentTime      string                `json:"current_time"`
	Metrics          model.MetricsSnapshot `json:"metrics"`
	RecentJobs       []model.Job           `json:"recent_jobs"`
	Workers          []model.Worker        `json:"workers"`
	PendingApprovals []model.Job           `json:"pending_approvals"`
	RecentCases      []model.IncidentCase  `json:"recent_cases"`
	RecentAuditLogs  []model.AuditLog      `json:"recent_audit_logs"`
	TemplatesTotal   int                   `json:"templates_total"`
}

func (s *Server) handleDashboardSummary(w http.ResponseWriter, r *http.Request) {
	jobs := s.store.ListJobs()
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].CreatedAt.After(jobs[j].CreatedAt)
	})
	jobs = limitJobs(jobs, 8)

	workers := s.store.ListWorkers()
	sort.Slice(workers, func(i, j int) bool {
		return workers[i].UpdatedAt.After(workers[j].UpdatedAt)
	})

	pending := s.store.ListPendingApprovalJobs()
	sort.Slice(pending, func(i, j int) bool {
		return pending[i].CreatedAt.After(pending[j].CreatedAt)
	})
	pending = limitJobs(pending, 6)

	cases := s.store.ListIncidentCases(model.IncidentCaseFilter{Limit: 4})

	logs := s.store.ListAuditLogs("", "")
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].CreatedAt.After(logs[j].CreatedAt)
	})
	logs = limitAuditLogs(logs, 8)

	writeJSON(w, http.StatusOK, dashboardSummaryResponse{
		User:             currentUser(r),
		CurrentTime:      time.Now().Format(time.RFC3339),
		Metrics:          s.store.MetricsSnapshot(),
		RecentJobs:       jobs,
		Workers:          workers,
		PendingApprovals: pending,
		RecentCases:      cases,
		RecentAuditLogs:  logs,
		TemplatesTotal:   len(s.templateEng.ListTemplates()),
	})
}

func limitJobs(values []model.Job, max int) []model.Job {
	if len(values) <= max {
		return values
	}
	return values[:max]
}

func limitAuditLogs(values []model.AuditLog, max int) []model.AuditLog {
	if len(values) <= max {
		return values
	}
	return values[:max]
}
