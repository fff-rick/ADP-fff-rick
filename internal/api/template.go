package api

import (
	"errors"
	"fmt"
	"net/http"
	"sort"

	"adp/internal/model"
	"adp/internal/scheduler"
)

// handleListTemplates returns all available command templates.
func (s *Server) handleListTemplates(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.templateEng.ListTemplates())
}

func (s *Server) handleListTaskJobs(w http.ResponseWriter, _ *http.Request) {
	jobs := s.store.ListJobs()
	filtered := make([]model.Job, 0, len(jobs))
	for _, job := range jobs {
		if job.SourceType == "task" {
			filtered = append(filtered, job)
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})

	writeJSON(w, http.StatusOK, filtered)
}

type parseTaskRequest struct {
	Input string `json:"input"`
}

func (s *Server) handleParseTask(w http.ResponseWriter, r *http.Request) {
	var req parseTaskRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if req.Input == "" {
		writeError(w, http.StatusBadRequest, errors.New("input is required"))
		return
	}

	intent, err := s.taskParser.Parse(r.Context(), req.Input)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}

	// Run policy risk assessment.
	intent.RiskLevel = s.policyEng.AssessRisk(*intent)

	writeJSON(w, http.StatusOK, intent)
}

type runTaskRequest struct {
	Input      string            `json:"input"`
	Parameters map[string]string `json:"parameters,omitempty"`
}

func (s *Server) handleRunTask(w http.ResponseWriter, r *http.Request) {
	var req runTaskRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if req.Input == "" {
		writeError(w, http.StatusBadRequest, errors.New("input is required"))
		return
	}

	// 1. Parse NL input.
	intent, err := s.taskParser.Parse(r.Context(), req.Input)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}

	// 2. Policy engine risk assessment.
	intent.RiskLevel = s.policyEng.AssessRisk(*intent)

	// 3. Resolve template.
	tmplCode := intent.MatchedTemplate
	if tmplCode == "" {
		writeError(w, http.StatusBadRequest, errors.New("no matching template for parsed intent"))
		return
	}

	if err := s.policyEng.ValidateTemplate(tmplCode); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}

	// 4. Render command from template.
	tmpl, cmd, err := s.templateEng.Render(tmplCode, req.Parameters)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	// 5. Validate rendered command against tool whitelist.
	if err := s.policyEng.ValidateCommand(cmd); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}

	riskLevel := s.policyEng.MergeRisk(intent.RiskLevel, tmpl.RiskLevel)
	intent.RiskLevel = riskLevel
	needsApproval := s.policyEng.RequiresManualApproval(riskLevel)
	jobStatus := model.JobStatusQueued
	approvalStatus := model.ApprovalStatusNotRequired
	if needsApproval {
		jobStatus = model.JobStatusWaitingApproval
		approvalStatus = model.ApprovalStatusPending
	}

	// 6. Create job and either enqueue or wait for approval.
	job := s.store.CreateJobWithOptions(
		fmt.Sprintf("[%s] %s", intent.Intent, req.Input),
		tmpl.ToolType,
		cmd,
		scheduler.CreateJobOptions{
			Status:           jobStatus,
			RiskLevel:        riskLevel,
			ApprovalRequired: needsApproval,
			ApprovalStatus:   approvalStatus,
			TemplateCode:     tmplCode,
			SourceType:       "task",
		},
	)
	user := currentUser(r)
	action := "job.queued"
	if needsApproval {
		action = "job.waiting_approval"
	}
	s.recordAudit("user", user.Username, action, "job", job.ID, map[string]any{
		"template_code": tmplCode,
		"risk_level":    riskLevel,
		"input":         req.Input,
	})

	statusCode := http.StatusCreated
	if needsApproval {
		statusCode = http.StatusAccepted
	}

	writeJSON(w, statusCode, map[string]any{
		"job":               job,
		"approval_required": needsApproval,
		"parsed_intent":     intent,
		"template_code":     tmplCode,
		"rendered_command":  cmd,
	})
}
