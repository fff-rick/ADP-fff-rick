package api

import (
	"errors"
	"net/http"
	"strings"

	"adp/internal/model"
	"adp/internal/scheduler"
)

type createJobRequest struct {
	Name       string `json:"name"`
	WorkerType string `json:"worker_type"`
	Command    string `json:"command"`
}

func (s *Server) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	var req createJobRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if req.Name == "" || req.WorkerType == "" {
		writeError(w, http.StatusBadRequest, errors.New("name and worker_type are required"))
		return
	}

	job := s.store.CreateJobWithOptions(req.Name, req.WorkerType, req.Command, scheduler.CreateJobOptions{
		Status:         model.JobStatusQueued,
		ApprovalStatus: model.ApprovalStatusNotRequired,
		SourceType:     "manual_job",
	})
	user := currentUser(r)
	s.recordAudit("user", user.Username, "job.created", "job", job.ID, map[string]any{
		"worker_type": req.WorkerType,
	})
	writeJSON(w, http.StatusCreated, job)
}

func (s *Server) handleListJobs(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.store.ListJobs())
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/jobs/")
	if id == "" {
		writeError(w, http.StatusBadRequest, errors.New("job id is required"))
		return
	}

	job, ok := s.store.GetJob(id)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("job not found"))
		return
	}

	writeJSON(w, http.StatusOK, job)
}
