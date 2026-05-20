package api

import (
	"fmt"
	"net/http"
)

func (s *Server) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	snapshot := s.store.MetricsSnapshot()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintf(w, "# HELP adp_jobs_total Total number of jobs created.\n")
	fmt.Fprintf(w, "# TYPE adp_jobs_total gauge\n")
	fmt.Fprintf(w, "adp_jobs_total %d\n", snapshot.JobsTotal)
	fmt.Fprintf(w, "# HELP adp_jobs_success_total Total number of successful jobs.\n")
	fmt.Fprintf(w, "# TYPE adp_jobs_success_total gauge\n")
	fmt.Fprintf(w, "adp_jobs_success_total %d\n", snapshot.JobsSuccess)
	fmt.Fprintf(w, "# HELP adp_jobs_failed_total Total number of failed jobs.\n")
	fmt.Fprintf(w, "# TYPE adp_jobs_failed_total gauge\n")
	fmt.Fprintf(w, "adp_jobs_failed_total %d\n", snapshot.JobsFailed)
	fmt.Fprintf(w, "# HELP adp_jobs_waiting_approval Total number of jobs waiting for approval.\n")
	fmt.Fprintf(w, "# TYPE adp_jobs_waiting_approval gauge\n")
	fmt.Fprintf(w, "adp_jobs_waiting_approval %d\n", snapshot.JobsWaitingApproval)
	fmt.Fprintf(w, "# HELP adp_workers_online Current number of online workers.\n")
	fmt.Fprintf(w, "# TYPE adp_workers_online gauge\n")
	fmt.Fprintf(w, "adp_workers_online %d\n", snapshot.WorkersOnline)
	fmt.Fprintf(w, "# HELP adp_incident_cases_total Total number of stored incident cases.\n")
	fmt.Fprintf(w, "# TYPE adp_incident_cases_total gauge\n")
	fmt.Fprintf(w, "adp_incident_cases_total %d\n", snapshot.IncidentCasesTotal)
	fmt.Fprintf(w, "# HELP adp_job_success_rate Success rate of completed jobs.\n")
	fmt.Fprintf(w, "# TYPE adp_job_success_rate gauge\n")
	fmt.Fprintf(w, "adp_job_success_rate %.6f\n", snapshot.JobSuccessRate)
	fmt.Fprintf(w, "# HELP adp_job_failure_rate Failure rate of completed jobs.\n")
	fmt.Fprintf(w, "# TYPE adp_job_failure_rate gauge\n")
	fmt.Fprintf(w, "adp_job_failure_rate %.6f\n", snapshot.JobFailureRate)
	fmt.Fprintf(w, "# HELP adp_job_schedule_latency_seconds_avg Average queue-to-start latency in seconds.\n")
	fmt.Fprintf(w, "# TYPE adp_job_schedule_latency_seconds_avg gauge\n")
	fmt.Fprintf(w, "adp_job_schedule_latency_seconds_avg %.6f\n", snapshot.AvgScheduleLatencySeconds)

	logEvent("metrics", "scrape", map[string]any{
		"jobs_total":           snapshot.JobsTotal,
		"workers_online":       snapshot.WorkersOnline,
		"incident_cases_total": snapshot.IncidentCasesTotal,
	})
}
