package api

import (
	"errors"
	"net/http"
	"sort"
	"strconv"
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

func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	jobs := s.store.ListJobs()
	sourceType := strings.TrimSpace(r.URL.Query().Get("source_type"))
	workerType := strings.TrimSpace(r.URL.Query().Get("worker_type"))
	status := strings.TrimSpace(r.URL.Query().Get("status"))

	filtered := make([]model.Job, 0, len(jobs))
	for _, job := range jobs {
		if sourceType != "" && job.SourceType != sourceType {
			continue
		}
		if workerType != "" && job.WorkerType != workerType {
			continue
		}
		if status != "" && string(job.Status) != status {
			continue
		}
		filtered = append(filtered, job)
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})

	if limitValue := strings.TrimSpace(r.URL.Query().Get("limit")); limitValue != "" {
		limit, err := strconv.Atoi(limitValue)
		if err != nil || limit <= 0 {
			writeError(w, http.StatusBadRequest, errors.New("limit must be a positive integer"))
			return
		}
		if len(filtered) > limit {
			filtered = filtered[:limit]
		}
	}

	writeJSON(w, http.StatusOK, filtered)
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
