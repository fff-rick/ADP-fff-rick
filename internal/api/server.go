package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"adp/internal/analyzer"
	"adp/internal/auth"
	"adp/internal/llm"
	"adp/internal/parser"
	"adp/internal/planner"
	"adp/internal/policy"
	"adp/internal/scheduler"
	"adp/internal/template"
)

type Config struct {
	Addr              string
	AdminUsername     string
	AdminPassword     string
	AuthSecret        string
	WorkerSharedToken string
	LLMBaseURL        string
	LLMAPIKey         string
	LLMModel          string
}

type Server struct {
	config      Config
	authService *auth.Service
	store       *scheduler.Store
	templateEng *template.Engine
	policyEng   *policy.Engine
	taskParser  *parser.Parser
	planner     *planner.Planner
	analyzer    *analyzer.Analyzer
	httpServer  *http.Server
}

func NewServer(cfg Config) *Server {
	templateEng := template.NewEngine()
	policyEng := policy.NewEngine()

	var llmClient llm.Client
	if cfg.LLMBaseURL != "" {
		llmClient = llm.NewHTTPClient(cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.LLMModel)
	}

	taskParser := parser.NewParser(llmClient, templateEng, policyEng)
	planStore := planner.NewPlanStore()
	dPlanner := planner.New(llmClient, templateEng, planStore)
	dAnalyzer := analyzer.New(llmClient)

	server := &Server{
		config:      cfg,
		authService: auth.NewService(cfg.AdminUsername, cfg.AdminPassword, cfg.AuthSecret),
		store:       scheduler.NewStore(),
		templateEng: templateEng,
		policyEng:   policyEng,
		taskParser:  taskParser,
		planner:     dPlanner,
		analyzer:    dAnalyzer,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", server.handleDashboardPage)
	mux.HandleFunc("GET /login", server.handleDashboardPage)
	mux.HandleFunc("GET /users", server.handleDashboardPage)
	mux.HandleFunc("GET /workers", server.handleDashboardPage)
	mux.HandleFunc("GET /jobs", server.handleDashboardPage)
	mux.HandleFunc("GET /tasks", server.handleDashboardPage)
	mux.Handle("GET /static/", server.staticAssetsHandler())
	mux.HandleFunc("GET /healthz", server.handleHealthz)
	mux.HandleFunc("GET /metrics", server.handleMetrics)
	mux.HandleFunc("POST /api/v1/auth/login", server.handleLogin)
	mux.HandleFunc("GET /api/v1/users", server.withUserAuth(server.handleListUsers))
	mux.HandleFunc("POST /api/v1/users", server.withUserAuth(server.handleCreateUser))
	mux.HandleFunc("GET /api/v1/dashboard/summary", server.withUserAuth(server.handleDashboardSummary))
	mux.HandleFunc("POST /api/v1/jobs", server.withUserAuth(server.handleCreateJob))
	mux.HandleFunc("GET /api/v1/jobs", server.withUserAuth(server.handleListJobs))
	mux.HandleFunc("GET /api/v1/jobs/", server.withUserAuth(server.handleGetJob))
	mux.HandleFunc("POST /api/v1/workers", server.withUserAuth(server.handleCreateWorker))
	mux.HandleFunc("GET /api/v1/workers", server.withUserAuth(server.handleListWorkers))
	mux.HandleFunc("POST /api/v1/workers/register", server.withWorkerAuth(server.handleRegisterWorker))
	mux.HandleFunc("POST /api/v1/workers/", server.withWorkerAuth(server.handleWorkerActions))
	mux.HandleFunc("GET /api/v1/templates", server.withUserAuth(server.handleListTemplates))
	mux.HandleFunc("GET /api/v1/tasks", server.withUserAuth(server.handleListTaskJobs))
	mux.HandleFunc("POST /api/v1/tasks/parse", server.withUserAuth(server.handleParseTask))
	mux.HandleFunc("POST /api/v1/tasks/run", server.withUserAuth(server.handleRunTask))
	mux.HandleFunc("GET /api/v1/approvals/jobs", server.withUserAuth(server.handleListPendingApprovalJobs))
	mux.HandleFunc("POST /api/v1/approvals/jobs/", server.withUserAuth(server.handleApproveJob))
	mux.HandleFunc("GET /api/v1/audit/logs", server.withUserAuth(server.handleListAuditLogs))
	mux.HandleFunc("GET /api/v1/cases", server.withUserAuth(server.handleListIncidentCases))
	mux.HandleFunc("GET /api/v1/cases/suggestions", server.withUserAuth(server.handleSuggestIncidentCases))
	mux.HandleFunc("POST /api/v1/diagnosis/plan", server.withUserAuth(server.handleCreateDiagnosisPlan))
	mux.HandleFunc("GET /api/v1/diagnosis/plan/", server.withUserAuth(server.handleDiagnosisPlanActions))
	mux.HandleFunc("POST /api/v1/diagnosis/plan/", server.withUserAuth(server.handleDiagnosisPlanActions))

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

func (s *Server) Handler() http.Handler {
	return s.httpServer.Handler
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
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

func logEvent(component, action string, fields map[string]any) {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var parts []string
	parts = append(parts, "level=INFO")
	parts = append(parts, fmt.Sprintf("component=%s", component))
	parts = append(parts, fmt.Sprintf("action=%s", action))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", key, fields[key]))
	}

	log.Print(strings.Join(parts, " "))
}
