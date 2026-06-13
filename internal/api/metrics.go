package api

import (
	"fmt"
	"net/http"
)

func (s *Server) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	snapshot := s.store.MetricsSnapshot()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintf(w, "# HELP adp_jobs_total Total number of jobs created.\n")                                    //nolint:errcheck
	fmt.Fprintf(w, "# TYPE adp_jobs_total gauge\n")                                                            //nolint:errcheck
	fmt.Fprintf(w, "adp_jobs_total %d\n", snapshot.JobsTotal)                                                  //nolint:errcheck
	fmt.Fprintf(w, "# HELP adp_jobs_success_total Total number of successful jobs.\n")                         //nolint:errcheck
	fmt.Fprintf(w, "# TYPE adp_jobs_success_total gauge\n")                                                    //nolint:errcheck
	fmt.Fprintf(w, "adp_jobs_success_total %d\n", snapshot.JobsSuccess)                                        //nolint:errcheck
	fmt.Fprintf(w, "# HELP adp_jobs_failed_total Total number of failed jobs.\n")                              //nolint:errcheck
	fmt.Fprintf(w, "# TYPE adp_jobs_failed_total gauge\n")                                                     //nolint:errcheck
	fmt.Fprintf(w, "adp_jobs_failed_total %d\n", snapshot.JobsFailed)                                          //nolint:errcheck
	fmt.Fprintf(w, "# HELP adp_jobs_waiting_approval Total number of jobs waiting for approval.\n")            //nolint:errcheck
	fmt.Fprintf(w, "# TYPE adp_jobs_waiting_approval gauge\n")                                                 //nolint:errcheck
	fmt.Fprintf(w, "adp_jobs_waiting_approval %d\n", snapshot.JobsWaitingApproval)                             //nolint:errcheck
	fmt.Fprintf(w, "# HELP adp_workers_online Current number of online workers.\n")                            //nolint:errcheck
	fmt.Fprintf(w, "# TYPE adp_workers_online gauge\n")                                                        //nolint:errcheck
	fmt.Fprintf(w, "adp_workers_online %d\n", snapshot.WorkersOnline)                                          //nolint:errcheck
	fmt.Fprintf(w, "# HELP adp_incident_cases_total Total number of stored incident cases.\n")                 //nolint:errcheck
	fmt.Fprintf(w, "# TYPE adp_incident_cases_total gauge\n")                                                  //nolint:errcheck
	fmt.Fprintf(w, "adp_incident_cases_total %d\n", snapshot.IncidentCasesTotal)                               //nolint:errcheck
	fmt.Fprintf(w, "# HELP adp_job_success_rate Success rate of completed jobs.\n")                            //nolint:errcheck
	fmt.Fprintf(w, "# TYPE adp_job_success_rate gauge\n")                                                      //nolint:errcheck
	fmt.Fprintf(w, "adp_job_success_rate %.6f\n", snapshot.JobSuccessRate)                                     //nolint:errcheck
	fmt.Fprintf(w, "# HELP adp_job_failure_rate Failure rate of completed jobs.\n")                            //nolint:errcheck
	fmt.Fprintf(w, "# TYPE adp_job_failure_rate gauge\n")                                                      //nolint:errcheck
	fmt.Fprintf(w, "adp_job_failure_rate %.6f\n", snapshot.JobFailureRate)                                     //nolint:errcheck
	fmt.Fprintf(w, "# HELP adp_job_schedule_latency_seconds_avg Average queue-to-start latency in seconds.\n") //nolint:errcheck
	fmt.Fprintf(w, "# TYPE adp_job_schedule_latency_seconds_avg gauge\n")                                      //nolint:errcheck
	fmt.Fprintf(w, "adp_job_schedule_latency_seconds_avg %.6f\n", snapshot.AvgScheduleLatencySeconds)          //nolint:errcheck

	logEvent("metrics", "scrape", map[string]any{
		"jobs_total":           snapshot.JobsTotal,
		"workers_online":       snapshot.WorkersOnline,
		"incident_cases_total": snapshot.IncidentCasesTotal,
	})
}
