package api

import (
	"errors"
	"net/http"
	"strings"
)

func (s *Server) handleListWorkers(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.store.ListWorkers())
}

type registerWorkerRequest struct {
	Name       string `json:"name"`
	WorkerType string `json:"worker_type"`
}

func (s *Server) handleRegisterWorker(w http.ResponseWriter, r *http.Request) {
	var req registerWorkerRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if req.Name == "" || req.WorkerType == "" {
		writeError(w, http.StatusBadRequest, errors.New("name and worker_type are required"))
		return
	}

	worker := s.store.RegisterWorker(req.Name, req.WorkerType)
	s.recordAudit("worker", worker.ID, "worker.registered", "worker", worker.ID, map[string]any{
		"name":        worker.Name,
		"worker_type": worker.WorkerType,
	})
	writeJSON(w, http.StatusCreated, worker)
}

// 请求方式有三种
// 状态检查：post ip:8080//api/v1/workers/worker-000001/heartbeat
// 获取job： post ip:8080//api/v1/workers/worker-000001/poll   // 只能加入同类 jobs
// 完成job:	 post ip:8080//api/v1/workers/worker-000001/jobs/jobID/status
func (s *Server) handleWorkerActions(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/workers/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		writeError(w, http.StatusNotFound, errors.New("unsupported worker route"))
		return
	}

	workerID := parts[0]
	action := parts[1]

	switch {
	case action == "heartbeat" && r.Method == http.MethodPost:
		s.handleWorkerHeartbeat(w, workerID)
	case action == "poll" && r.Method == http.MethodPost:
		s.handleWorkerPoll(w, workerID)
	case action == "jobs" && len(parts) == 4 && parts[3] == "complete" && r.Method == http.MethodPost:
		s.handleWorkerCompleteJob(w, workerID, parts[2], r)
	default:
		writeError(w, http.StatusNotFound, errors.New("unsupported worker route"))
	}
}

func (s *Server) handleWorkerHeartbeat(w http.ResponseWriter, workerID string) {
	worker, ok := s.store.HeartbeatWorker(workerID)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("worker not found"))
		return
	}

	writeJSON(w, http.StatusOK, worker)
}

func (s *Server) handleWorkerPoll(w http.ResponseWriter, workerID string) {
	job, ok := s.store.AssignNextJob(workerID)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{
			"job": nil,
		})
		return
	}

	s.recordAudit("worker", workerID, "job.assigned", "job", job.ID, map[string]any{
		"worker_id": workerID,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"job": job,
	})
}

type completeJobRequest struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
}

func (s *Server) handleWorkerCompleteJob(w http.ResponseWriter, workerID, jobID string, r *http.Request) {
	var req completeJobRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	job, err := s.store.CompleteJob(workerID, jobID, req.Output, req.Success)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	action := "job.completed"
	if !req.Success {
		action = "job.failed"
	}
	s.recordAudit("worker", workerID, action, "job", job.ID, map[string]any{
		"success": req.Success,
	})

	writeJSON(w, http.StatusOK, job)
}

func (s *Server) withWorkerAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Worker-Token")
		if token == "" || token != s.config.WorkerSharedToken {
			writeError(w, http.StatusUnauthorized, errors.New("invalid worker token"))
			return
		}

		next(w, r)
	}
}
