package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"adp/internal/domain/model"
)

func (s *Server) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	var snapshot model.MetricsSnapshot
	if s.repo != nil {
		snapshot, _ = s.repo.MetricsSnapshot()
	}
	// Recalculate online workers in real-time.
	if s.repo != nil {
		workers, _ := s.repo.ListWorkers()
		snapshot.WorkersOnline = 0
		threshold := time.Now().Add(-30 * time.Second)
		for _, w := range workers {
			if w.LastHeartbeatAt.After(threshold) {
				snapshot.WorkersOnline++
			}
		}
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	var out strings.Builder
	out.WriteString("# HELP adp_jobs_total Total number of jobs created.\n")
	out.WriteString("# TYPE adp_jobs_total gauge\n")
	writeMetricf(&out, "adp_jobs_total %d\n", snapshot.JobsTotal)
	out.WriteString("# HELP adp_jobs_success_total Total number of successful jobs.\n")
	out.WriteString("# TYPE adp_jobs_success_total gauge\n")
	writeMetricf(&out, "adp_jobs_success_total %d\n", snapshot.JobsSuccess)
	out.WriteString("# HELP adp_jobs_failed_total Total number of failed jobs.\n")
	out.WriteString("# TYPE adp_jobs_failed_total gauge\n")
	writeMetricf(&out, "adp_jobs_failed_total %d\n", snapshot.JobsFailed)
	out.WriteString("# HELP adp_jobs_waiting_approval Total number of jobs waiting for approval.\n")
	out.WriteString("# TYPE adp_jobs_waiting_approval gauge\n")
	writeMetricf(&out, "adp_jobs_waiting_approval %d\n", snapshot.JobsWaitingApproval)
	out.WriteString("# HELP adp_workers_online Current number of online workers.\n")
	out.WriteString("# TYPE adp_workers_online gauge\n")
	writeMetricf(&out, "adp_workers_online %d\n", snapshot.WorkersOnline)
	out.WriteString("# HELP adp_incident_cases_total Total number of stored incident cases.\n")
	out.WriteString("# TYPE adp_incident_cases_total gauge\n")
	writeMetricf(&out, "adp_incident_cases_total %d\n", snapshot.IncidentCasesTotal)
	out.WriteString("# HELP adp_job_success_rate Success rate of completed jobs.\n")
	out.WriteString("# TYPE adp_job_success_rate gauge\n")
	writeMetricf(&out, "adp_job_success_rate %.6f\n", snapshot.JobSuccessRate)
	out.WriteString("# HELP adp_job_failure_rate Failure rate of completed jobs.\n")
	out.WriteString("# TYPE adp_job_failure_rate gauge\n")
	writeMetricf(&out, "adp_job_failure_rate %.6f\n", snapshot.JobFailureRate)
	out.WriteString("# HELP adp_job_schedule_latency_seconds_avg Average queue-to-start latency in seconds.\n")
	out.WriteString("# TYPE adp_job_schedule_latency_seconds_avg gauge\n")
	writeMetricf(&out, "adp_job_schedule_latency_seconds_avg %.6f\n", snapshot.AvgScheduleLatencySeconds)
	if _, err := w.Write([]byte(out.String())); err != nil {
		return
	}

	logEvent("metrics", "scrape", map[string]any{
		"jobs_total":           snapshot.JobsTotal,
		"workers_online":       snapshot.WorkersOnline,
		"incident_cases_total": snapshot.IncidentCasesTotal,
	})
}

func writeMetricf(out *strings.Builder, format string, args ...any) {
	_, _ = fmt.Fprintf(out, format, args...)
}
