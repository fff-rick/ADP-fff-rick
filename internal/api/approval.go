package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"adp/internal/model"
)

type approveJobRequest struct {
	Approved *bool  `json:"approved"`
	Comment  string `json:"comment,omitempty"`
}

func (s *Server) handleListPendingApprovalJobs(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.store.ListPendingApprovalJobs())
}

func (s *Server) handleApproveJob(w http.ResponseWriter, r *http.Request) {
	jobID := strings.TrimPrefix(r.URL.Path, "/api/v1/approvals/jobs/")
	if jobID == "" {
		writeError(w, http.StatusBadRequest, errors.New("job id is required"))
		return
	}

	var req approveJobRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Approved == nil {
		writeError(w, http.StatusBadRequest, errors.New("approved is required"))
		return
	}

	user := currentUser(r)
	var (
		job model.Job
		err error
	)
	if *req.Approved {
		job, err = s.store.ApproveJob(jobID, user.Username, req.Comment)
	} else {
		job, err = s.store.RejectJob(jobID, user.Username, req.Comment)
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if job.SourceType == "diagnosis_plan" {
		s.updatePlanStatusAfterApproval(job.SourceID)
	}

	action := "job.approval.rejected"
	if *req.Approved {
		action = "job.approval.approved"
	}
	s.recordAudit("user", user.Username, action, "job", job.ID, map[string]any{
		"comment":     req.Comment,
		"source_type": job.SourceType,
		"source_id":   job.SourceID,
		"status":      job.Status,
	})

	writeJSON(w, http.StatusOK, job)
}

func (s *Server) updatePlanStatusAfterApproval(planID string) {
	if planID == "" {
		return
	}

	s.planner.Store().Update(planID, func(plan *model.DiagnosisPlan) {
		synced := s.syncPlanWithJobs(*plan)
		plan.Status = synced.Status
		plan.Steps = synced.Steps
		plan.UpdatedAt = time.Now()
	})
}
