package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"adp/internal/auth"
	"adp/internal/model"
	"adp/internal/scheduler"
)

type Config struct {
	Addr              string
	AdminUsername     string
	AdminPassword     string
	AuthSecret        string
	WorkerSharedToken string
}

type Server struct {
	config      Config
	authService *auth.Service
	store       *scheduler.Store
	httpServer  *http.Server
}

func NewServer(cfg Config) *Server {
	server := &Server{
		config:      cfg,
		authService: auth.NewService(cfg.AdminUsername, cfg.AdminPassword, cfg.AuthSecret),
		store:       scheduler.NewStore(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", server.handleHealthz)
	mux.HandleFunc("POST /api/v1/auth/login", server.handleLogin)
	mux.HandleFunc("POST /api/v1/jobs", server.withUserAuth(server.handleCreateJob))
	mux.HandleFunc("GET /api/v1/jobs", server.withUserAuth(server.handleListJobs))
	mux.HandleFunc("GET /api/v1/jobs/", server.withUserAuth(server.handleGetJob))
	mux.HandleFunc("GET /api/v1/workers", server.withUserAuth(server.handleListWorkers))
	mux.HandleFunc("POST /api/v1/workers/register", server.withWorkerAuth(server.handleRegisterWorker))
	mux.HandleFunc("POST /api/v1/workers/", server.withWorkerAuth(server.handleWorkerActions))

	server.httpServer = &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return server
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string     `json:"token"`
	User  model.User `json:"user"`
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	token, user, err := s.authService.Login(req.Username, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	writeJSON(w, http.StatusOK, loginResponse{
		Token: token,
		User:  user,
	})
}

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

	job := s.store.CreateJob(req.Name, req.WorkerType, req.Command)
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

	writeJSON(w, http.StatusOK, job)
}

func (s *Server) withUserAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r.Header.Get("Authorization"))
		if token == "" {
			writeError(w, http.StatusUnauthorized, errors.New("missing bearer token"))
			return
		}

		if _, err := s.authService.ParseToken(token); err != nil {
			writeError(w, http.StatusUnauthorized, err)
			return
		}

		next(w, r)
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

func bearerToken(header string) string {
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(header, "Bearer ")
}

func decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeJSON(w http.ResponseWriter, statusCode int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, statusCode int, err error) {
	writeJSON(w, statusCode, map[string]string{
		"error": err.Error(),
	})
}

func (s *Server) String() string {
	return fmt.Sprintf("server(addr=%s)", s.config.Addr)
}
