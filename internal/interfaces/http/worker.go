package api

import (
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"adp/internal/domain/model"
	"adp/internal/infrastructure/deploy"
)

func (s *Server) handleListWorkers(w http.ResponseWriter, _ *http.Request) {
	if s.repo != nil {
		workers, err := s.repo.ListWorkers()
		if err == nil {
			writeJSON(w, http.StatusOK, workers)
			return
		}
	}
	writeJSON(w, http.StatusOK, []model.Worker{})
}

type registerWorkerRequest struct {
	Name       string `json:"name"`
	WorkerType string `json:"worker_type"`

	// SSH 远程部署参数 (可选)
	SSHHost     string `json:"ssh_host,omitempty"`
	SSHPort     int    `json:"ssh_port,omitempty"` // 默认 22
	SSHUser     string `json:"ssh_user,omitempty"`
	SSHPassword string `json:"ssh_password,omitempty"`
	SSHKeyFile  string `json:"ssh_key_file,omitempty"`
	LogToDB     bool   `json:"log_to_db,omitempty"` // Worker 日志落库
}

func (s *Server) handleCreateWorker(w http.ResponseWriter, r *http.Request) {
	var req registerWorkerRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if req.Name == "" || req.WorkerType == "" {
		writeError(w, http.StatusBadRequest, errors.New("name and worker_type are required"))
		return
	}

	// ── 远程部署分支 ──
	if req.SSHHost != "" {
		if req.SSHUser == "" {
			writeError(w, http.StatusBadRequest, errors.New("ssh_user is required for remote deployment"))
			return
		}
		if req.SSHPassword == "" && req.SSHKeyFile == "" {
			writeError(w, http.StatusBadRequest, errors.New("ssh_password or ssh_key_file is required for remote deployment"))
			return
		}

		// 获取当前可执行文件路径，作为 worker 二进制上传到目标机。
		localBinary, err := os.Executable()
		if err != nil {
			writeError(w, http.StatusInternalServerError, errors.New("cannot locate server binary: "+err.Error()))
			return
		}

		target := deploy.Target{
			Host:     req.SSHHost,
			Port:     req.SSHPort,
			User:     req.SSHUser,
			Password: req.SSHPassword,
			KeyFile:  req.SSHKeyFile,
		}

		spec := deploy.WorkerSpec{
			ServerURL:   "http://" + s.config.Addr, // 需要外部可达地址
			WorkerToken: s.config.WorkerSharedToken,
			WorkerName:  req.Name,
			WorkerType:  req.WorkerType,
			LogToDB:     req.LogToDB,
		}

		go func() {
			// 后台异步部署，不阻塞 HTTP 响应。
			if err := deploy.DeployWorker(target, spec, localBinary); err != nil {
				log.Printf("deploy worker to %s failed: %v", req.SSHHost, err)
			}
		}()

		// 先在本地创建 Worker 记录。
		if s.repo != nil {
			worker, err := s.repo.RegisterWorker(req.Name, req.WorkerType)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			user := currentUser(r)
			s.recordAudit("user", user.Username, "worker.deploy_requested", "worker", worker.ID, map[string]any{
				"name":        worker.Name,
				"worker_type": worker.WorkerType,
				"ssh_host":    req.SSHHost,
			})
			writeJSON(w, http.StatusAccepted, map[string]any{
				"worker":    worker,
				"deploying": true,
				"ssh_host":  req.SSHHost,
			})
			return
		}
		writeError(w, http.StatusInternalServerError, errors.New("no store configured"))
		return
	}

	// ── 仅创建记录 (原有逻辑) ──
	if s.repo != nil {
		worker, err := s.repo.RegisterWorker(req.Name, req.WorkerType)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		user := currentUser(r)
		s.recordAudit("user", user.Username, "worker.created", "worker", worker.ID, map[string]any{
			"name":        worker.Name,
			"worker_type": worker.WorkerType,
		})
		writeJSON(w, http.StatusCreated, worker)
		return
	}

	writeError(w, http.StatusInternalServerError, errors.New("no store configured"))
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

	if s.repo != nil {
		worker, err := s.repo.RegisterWorker(req.Name, req.WorkerType)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		s.recordAudit("worker", worker.ID, "worker.registered", "worker", worker.ID, map[string]any{
			"name":        worker.Name,
			"worker_type": worker.WorkerType,
		})
		writeJSON(w, http.StatusCreated, worker)
		return
	}

	writeError(w, http.StatusInternalServerError, errors.New("no store configured"))
}

// handleWorkerActions routes worker API calls.
// Paths handled:
//
//	POST /api/v1/workers/{id}/heartbeat
//	POST /api/v1/workers/{id}/poll
//	POST /api/v1/workers/{id}/jobs/{jobId}/complete
//	POST /api/v1/workers/{id}/stop
//	POST /api/v1/workers/{id}/restart
//	PUT  /api/v1/workers/{id}/hostinfo
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
		s.handleWorkerHeartbeat(w, r, workerID)
	case action == "poll" && r.Method == http.MethodPost:
		s.handleWorkerPoll(w, workerID)
	case action == "jobs" && len(parts) == 4 && parts[3] == "complete" && r.Method == http.MethodPost:
		s.handleWorkerCompleteJob(w, workerID, parts[2], r)
	case action == "stop" && r.Method == http.MethodPost:
		s.handleStopWorker(w, r, workerID)
	case action == "restart" && r.Method == http.MethodPost:
		s.handleRestartWorker(w, r, workerID)
	case action == "hostinfo" && r.Method == http.MethodPut:
		s.handleUpdateHostInfo(w, r, workerID)
	case action == "command-ack" && r.Method == http.MethodPost:
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	default:
		writeError(w, http.StatusNotFound, errors.New("unsupported worker route"))
	}
}

type heartbeatRequest struct {
	HostInfo *model.HostInfo `json:"host_info,omitempty"`
}

func (s *Server) handleWorkerHeartbeat(w http.ResponseWriter, r *http.Request, workerID string) {
	var req heartbeatRequest
	// Body is optional for heartbeat.
	_ = decodeJSON(r, &req)

	if s.repo != nil {
		worker, err := s.repo.HeartbeatWorker(workerID, req.HostInfo)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, worker)
		return
	}

	writeError(w, http.StatusNotFound, errors.New("worker not found"))
}

func (s *Server) handleWorkerPoll(w http.ResponseWriter, workerID string) {
	if s.repo != nil {
		// Poll no longer assigns jobs. Workers execute only explicit gRPC stream pushes.
		worker, err := s.repo.GetWorker(workerID)
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]any{"job": nil})
			return
		}

		command := ""
		switch worker.Status {
		case model.WorkerStatus("stopping"):
			command = "stop"
		case model.WorkerStatus("restarting"):
			command = "restart"
		case model.WorkerStatus("force_stopping"):
			command = "force_stop"
		}

		writeJSON(w, http.StatusOK, map[string]any{"job": nil, "command": command})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"job": nil})
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

	if s.repo != nil {
		job, err := s.repo.CompleteJob(workerID, jobID, req.Output, req.Success)
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
		return
	}

	writeError(w, http.StatusInternalServerError, errors.New("no store configured"))
}

// handleUpdateHostInfo receives host info from a worker.
func (s *Server) handleUpdateHostInfo(w http.ResponseWriter, r *http.Request, workerID string) {
	var info model.HostInfo
	if err := decodeJSON(r, &info); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if s.repo != nil {
		_, err := s.repo.HeartbeatWorker(workerID, &info)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleDeleteWorker removes a worker record from the database.
func (s *Server) handleDeleteWorker(w http.ResponseWriter, r *http.Request, workerID string) {
	// Disconnect stream if connected.
	s.workerHub.PushCommand(workerID, "force_stop")

	if s.repo != nil {
		if err := s.repo.DeleteWorker(workerID); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		user := currentUser(r)
		s.recordAudit("user", user.Username, "worker.deleted", "worker", workerID, nil)
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
		return
	}
	writeError(w, http.StatusInternalServerError, errors.New("no store configured"))
}

// handleStopWorker sets a worker to stopping status.
func (s *Server) handleStopWorker(w http.ResponseWriter, r *http.Request, workerID string) {
	force := strings.TrimSpace(r.URL.Query().Get("force")) == "true"
	targetStatus := model.WorkerStatus("stopping")
	if force {
		targetStatus = model.WorkerStatus("force_stopping")
	}

	if s.repo != nil {
		if err := s.repo.UpdateWorkerStatus(workerID, targetStatus); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		// Push via gRPC stream for instant delivery.
		cmd := "stop"
		if force {
			cmd = "force_stop"
		}
		s.workerHub.PushCommand(workerID, cmd)
		user := currentUser(r)
		s.recordAudit("user", user.Username, "worker.stop_requested", "worker", workerID, map[string]any{
			"force": force,
		})
		writeJSON(w, http.StatusAccepted, map[string]string{
			"status": string(targetStatus),
		})
		return
	}

	writeError(w, http.StatusInternalServerError, errors.New("no store configured"))
}

// handleRestartWorker sets a worker to restarting status.
func (s *Server) handleRestartWorker(w http.ResponseWriter, r *http.Request, workerID string) {
	if s.repo != nil {
		if err := s.repo.UpdateWorkerStatus(workerID, model.WorkerStatus("restarting")); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		s.workerHub.PushCommand(workerID, "restart")
		user := currentUser(r)
		s.recordAudit("user", user.Username, "worker.restart_requested", "worker", workerID, nil)
		writeJSON(w, http.StatusAccepted, map[string]string{
			"status": "restarting",
		})
		return
	}

	writeError(w, http.StatusInternalServerError, errors.New("no store configured"))
}

// handleWorkerUserAction handles user-authenticated worker operations (stop, restart).
// Routes: POST /api/v1/workers/{id}/stop, POST /api/v1/workers/{id}/restart
func (s *Server) handleWorkerUserAction(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/workers/")
	parts := strings.Split(path, "/")
	if len(parts) < 1 || parts[0] == "" {
		writeError(w, http.StatusNotFound, errors.New("unsupported route"))
		return
	}

	workerID := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch {
	case action == "stop" && r.Method == http.MethodPost:
		s.handleStopWorker(w, r, workerID)
	case action == "restart" && r.Method == http.MethodPost:
		s.handleRestartWorker(w, r, workerID)
	case action == "" && r.Method == http.MethodDelete:
		s.handleDeleteWorker(w, r, workerID)
	default:
		writeError(w, http.StatusNotFound, errors.New("unsupported route"))
	}
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
