package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"adp/internal/domain/model"
)

type createPlanRequest struct {
	Description string `json:"description"`
}

func (s *Server) handleCreateDiagnosisPlan(w http.ResponseWriter, r *http.Request) {
	var req createPlanRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if req.Description == "" {
		writeError(w, http.StatusBadRequest, errors.New("description is required"))
		return
	}

	plan, err := s.planner.GeneratePlan(r.Context(), req.Description)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}

	// Persist to repo if available.
	if s.repo != nil {
		_ = s.repo.CreatePlan(*plan)
	}

	user := currentUser(r)
	s.recordAudit("user", user.Username, "diagnosis.plan.created", "diagnosis_plan", plan.ID, map[string]any{
		"description": req.Description,
		"trigger":     plan.TriggerType,
	})

	writeJSON(w, http.StatusCreated, plan)
}

func (s *Server) handleDiagnosisPlanActions(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/diagnosis/plan/")
	parts := strings.Split(path, "/")

	if len(parts) < 1 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, errors.New("plan id is required"))
		return
	}

	planID := parts[0]

	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		s.handleGetDiagnosisPlan(w, planID)
	case len(parts) == 2 && parts[1] == "execute" && r.Method == http.MethodPost:
		s.handleExecuteDiagnosisPlan(w, r, planID)
	case len(parts) == 2 && parts[1] == "analyze" && r.Method == http.MethodPost:
		s.handleAnalyzeDiagnosisPlan(w, r, planID)
	default:
		writeError(w, http.StatusNotFound, errors.New("unsupported diagnosis route"))
	}
}

func (s *Server) handleGetDiagnosisPlan(w http.ResponseWriter, planID string) {
	var plan model.DiagnosisPlan
	var ok bool

	if s.repo != nil {
		p, err := s.repo.GetPlan(planID)
		if err == nil {
			plan = p
			ok = true
		}
	}
	if !ok {
		// Try the in-memory planner store.
		p, found := s.planner.Store().Get(planID)
		if !found {
			writeError(w, http.StatusNotFound, errors.New("plan not found"))
			return
		}
		plan = p
	}

	plan = s.syncPlanWithJobs(plan)
	writeJSON(w, http.StatusOK, plan)
}

func (s *Server) handleExecuteDiagnosisPlan(w http.ResponseWriter, r *http.Request, planID string) {
	var updatedPlan model.DiagnosisPlan
	var found bool

	if s.repo != nil {
		p, err := s.repo.GetPlan(planID)
		if err == nil {
			updatedPlan = p
			found = true
		}
	}
	if !found {
		p, ok := s.planner.Store().Get(planID)
		if !ok {
			writeError(w, http.StatusNotFound, errors.New("plan not found"))
			return
		}
		updatedPlan = p
	}

	type stepJob struct {
		StepNo      int             `json:"step_no"`
		JobID       string          `json:"job_id"`
		Status      model.JobStatus `json:"status"`
		RiskLevel   model.RiskLevel `json:"risk_level"`
		NeedsReview bool            `json:"needs_review"`
	}

	user := currentUser(r)
	var created []stepJob
	needsApproval := false

	for i, step := range updatedPlan.Steps {
		if err := model.ValidateNoInlineSecrets(step.Parameters); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("step %d: %w", step.StepNo, err))
			return
		}
		if err := model.ValidateServiceProfile(step.TemplateCode, step.Parameters); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("step %d: %w", step.StepNo, err))
			return
		}
		if err := s.policyEng.ValidateTemplate(step.TemplateCode); err != nil {
			writeError(w, http.StatusForbidden, fmt.Errorf("step %d: %w", step.StepNo, err))
			return
		}

		tmpl, cmd, err := s.templateEng.Render(step.TemplateCode, step.Parameters)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("step %d: %w", step.StepNo, err))
			return
		}

		if err := s.policyEng.ValidateCommand(cmd); err != nil {
			writeError(w, http.StatusForbidden, fmt.Errorf("step %d: %w", step.StepNo, err))
			return
		}

		riskLevel := s.policyEng.MergeRisk(tmpl.RiskLevel)
		waitingApproval := s.policyEng.RequiresManualApproval(riskLevel)
		jobStatus := model.JobStatusPending
		approvalStatus := model.ApprovalStatusNotRequired
		if waitingApproval {
			jobStatus = model.JobStatusWaitingApproval
			approvalStatus = model.ApprovalStatusPending
			needsApproval = true
		}

		if s.repo == nil {
			writeError(w, http.StatusInternalServerError, errors.New("no store configured"))
			return
		}

		job := model.Job{
			Name:             fmt.Sprintf("[diagnosis:%s] step %d: %s", planID, step.StepNo, step.Name),
			WorkerType:       tmpl.ToolType,
			Command:          cmd,
			Status:           jobStatus,
			RiskLevel:        riskLevel,
			ApprovalRequired: waitingApproval,
			ApprovalStatus:   approvalStatus,
			TemplateCode:     step.TemplateCode,
			Parameters:       cloneStringMap(step.Parameters),
			SourceType:       "diagnosis_plan",
			SourceID:         planID,
		}

		createdJob, err := s.repo.CreateJob(job)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}

		updatedPlan.Steps[i].JobID = createdJob.ID
		updatedPlan.Steps[i].Status = createdJob.Status
		created = append(created, stepJob{
			StepNo:      step.StepNo,
			JobID:       createdJob.ID,
			Status:      createdJob.Status,
			RiskLevel:   riskLevel,
			NeedsReview: waitingApproval,
		})

		action := "diagnosis.step.queued"
		if waitingApproval {
			action = "diagnosis.step.waiting_approval"
		}
		s.recordAudit("user", user.Username, action, "job", createdJob.ID, map[string]any{
			"plan_id":       planID,
			"step_no":       step.StepNo,
			"template_code": step.TemplateCode,
			"risk_level":    riskLevel,
		})
	}

	updatedPlan.Status = model.PlanStatusRunning
	if needsApproval {
		updatedPlan.Status = model.PlanStatusWaitingApproval
	}
	updatedPlan.UpdatedAt = time.Now()

	// Persist updated plan.
	if s.repo != nil {
		_ = s.repo.UpdatePlan(planID, updatedPlan)
	}
	s.planner.Store().Update(planID, func(plan *model.DiagnosisPlan) {
		plan.Steps = updatedPlan.Steps
		plan.Status = updatedPlan.Status
		plan.UpdatedAt = updatedPlan.UpdatedAt
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"plan_id":           planID,
		"status":            updatedPlan.Status,
		"approval_required": needsApproval,
		"jobs":              created,
	})
}

func (s *Server) handleAnalyzeDiagnosisPlan(w http.ResponseWriter, r *http.Request, planID string) {
	var plan model.DiagnosisPlan
	var ok bool

	if s.repo != nil {
		p, err := s.repo.GetPlan(planID)
		if err == nil {
			plan = p
			ok = true
		}
	}
	if !ok {
		p, found := s.planner.Store().Get(planID)
		if !found {
			writeError(w, http.StatusNotFound, errors.New("plan not found"))
			return
		}
		plan = p
	}

	plan = s.syncPlanWithJobs(plan)

	report, err := s.analyzer.Analyze(r.Context(), plan)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if s.repo != nil {
		cases, _ := s.repo.FindSimilarIncidentCases(plan.Description, plan.TriggerType, report.FaultType, 3)
		report.ReferenceCases = cases
		report.HistoricalHints = buildHistoricalHints(cases)

		incidentCase := model.IncidentCase{
			Title:          plan.Title,
			TriggerType:    plan.TriggerType,
			FaultType:      report.FaultType,
			Summary:        strings.Join(report.PossibleCauses, "; "),
			PossibleCauses: report.PossibleCauses,
			Suggestions:    report.Suggestions,
			Confidence:     report.Confidence,
		}
		if cs, err := s.repo.UpsertIncidentCase(planID, incidentCase); err == nil {
			_ = cs // case stored
		}
	} else {
		report.ReferenceCases = []model.IncidentCase{}
		report.HistoricalHints = []string{}
	}

	// Update plan status.
	if s.repo != nil {
		plan.Status = model.PlanStatusCompleted
		plan.UpdatedAt = time.Now()
		_ = s.repo.UpdatePlan(planID, plan)
	}
	s.planner.Store().Update(planID, func(p *model.DiagnosisPlan) {
		p.Status = model.PlanStatusCompleted
		p.UpdatedAt = time.Now()
	})

	user := currentUser(r)
	s.recordAudit("user", user.Username, "diagnosis.plan.analyzed", "diagnosis_plan", planID, map[string]any{
		"fault_type": report.FaultType,
		"confidence": report.Confidence,
	})

	writeJSON(w, http.StatusOK, report)
}

func (s *Server) syncPlanWithJobs(plan model.DiagnosisPlan) model.DiagnosisPlan {
	anyWaitingApproval := false
	anyRunning := false
	allFinished := len(plan.Steps) > 0
	anyFailed := false

	for i, step := range plan.Steps {
		if step.JobID == "" {
			allFinished = false
			continue
		}

		var job model.Job
		var jobOK bool
		if s.repo != nil {
			j, err := s.repo.GetJob(step.JobID)
			if err == nil {
				job = j
				jobOK = true
			}
		}
		if !jobOK {
			allFinished = false
			continue
		}

		plan.Steps[i].Status = job.Status
		switch job.Status {
		case model.JobStatusWaitingApproval:
			anyWaitingApproval = true
			allFinished = false
		case model.JobStatusQueued, model.JobStatusRunning, model.JobStatusPending:
			anyRunning = true
			allFinished = false
		case model.JobStatusFailed, model.JobStatusCancelled:
			anyFailed = true
		}

		if job.Status == model.JobStatusSuccess || job.Status == model.JobStatusFailed {
			plan.Steps[i].Result = &model.StepResult{
				Stdout:   job.Output,
				Success:  job.Status == model.JobStatusSuccess,
				ExitCode: exitCodeFromStatus(job.Status),
			}
		}
	}

	switch {
	case anyWaitingApproval:
		plan.Status = model.PlanStatusWaitingApproval
	case anyRunning:
		plan.Status = model.PlanStatusRunning
	case anyFailed:
		plan.Status = model.PlanStatusFailed
	case allFinished:
		plan.Status = model.PlanStatusCompleted
	}

	return plan
}

func exitCodeFromStatus(status model.JobStatus) int {
	switch status {
	case model.JobStatusSuccess:
		return 0
	default:
		return 1
	}
}
