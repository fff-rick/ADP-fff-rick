package model

import "time"

type User struct {
	Username string `json:"username"`
	Role     string `json:"role"`
}

type WorkerStatus string

const (
	WorkerStatusOnline  WorkerStatus = "online"
	WorkerStatusOffline WorkerStatus = "offline"
)

type Worker struct {
	ID              string       `json:"id"`
	Name            string       `json:"name"`
	WorkerType      string       `json:"worker_type"`
	Status          WorkerStatus `json:"status"`
	LastHeartbeatAt time.Time    `json:"last_heartbeat_at"`
	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at"`
}

type JobStatus string

const (
	JobStatusPending         JobStatus = "pending"
	JobStatusWaitingApproval JobStatus = "waiting_approval"
	JobStatusQueued          JobStatus = "queued"
	JobStatusRunning         JobStatus = "running"
	JobStatusSuccess         JobStatus = "success"
	JobStatusFailed          JobStatus = "failed"
	JobStatusCancelled       JobStatus = "cancelled"
)

type ApprovalStatus string

const (
	ApprovalStatusNotRequired ApprovalStatus = "not_required"
	ApprovalStatusPending     ApprovalStatus = "pending"
	ApprovalStatusApproved    ApprovalStatus = "approved"
	ApprovalStatusRejected    ApprovalStatus = "rejected"
)

type Job struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	WorkerType       string         `json:"worker_type"`
	Command          string         `json:"command"`
	Status           JobStatus      `json:"status"`
	RiskLevel        RiskLevel      `json:"risk_level,omitempty"`
	ApprovalRequired bool           `json:"approval_required"`
	ApprovalStatus   ApprovalStatus `json:"approval_status,omitempty"`
	ApprovalComment  string         `json:"approval_comment,omitempty"`
	ApprovedBy       string         `json:"approved_by,omitempty"`
	ApprovedAt       *time.Time     `json:"approved_at,omitempty"`
	RejectedBy       string         `json:"rejected_by,omitempty"`
	RejectedAt       *time.Time     `json:"rejected_at,omitempty"`
	TemplateCode     string         `json:"template_code,omitempty"`
	SourceType       string         `json:"source_type,omitempty"`
	SourceID         string         `json:"source_id,omitempty"`
	AssignedWorkerID string         `json:"assigned_worker_id,omitempty"`
	Output           string         `json:"output,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	StartedAt        *time.Time     `json:"started_at,omitempty"`
	FinishedAt       *time.Time     `json:"finished_at,omitempty"`
}

// RiskLevel represents the risk classification of a task.
type RiskLevel string

const (
	RiskLevelLow    RiskLevel = "low"
	RiskLevelMedium RiskLevel = "medium"
	RiskLevelHigh   RiskLevel = "high"
)

// TaskIntent is the structured result of parsing a natural language task.
type TaskIntent struct {
	Intent          string            `json:"intent"`
	TargetType      string            `json:"target_type"`
	Schedule        string            `json:"schedule,omitempty"`
	RiskLevel       RiskLevel         `json:"risk_level"`
	Parameters      map[string]string `json:"parameters,omitempty"`
	MatchedTemplate string            `json:"matched_template,omitempty"`
}

// CommandTemplate defines a reusable, parameterized command template.
type CommandTemplate struct {
	Code        string          `json:"code"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	ToolType    string          `json:"tool_type"`
	Command     string          `json:"command"`
	Parameters  []TemplateParam `json:"parameters"`
	RiskLevel   RiskLevel       `json:"risk_level"`
}

// TemplateParam defines a single parameter within a command template.
type TemplateParam struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
}

// PlanStatus tracks the lifecycle of a diagnosis plan.
type PlanStatus string

const (
	PlanStatusPending         PlanStatus = "pending"
	PlanStatusWaitingApproval PlanStatus = "waiting_approval"
	PlanStatusRunning         PlanStatus = "running"
	PlanStatusCompleted       PlanStatus = "completed"
	PlanStatusFailed          PlanStatus = "failed"
)

// DiagnosisPlan is a multi-step plan for diagnosing a fault.
type DiagnosisPlan struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	TriggerType string          `json:"trigger_type"`
	Steps       []DiagnosisStep `json:"steps"`
	Status      PlanStatus      `json:"status"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// DiagnosisStep is one step within a diagnosis plan.
type DiagnosisStep struct {
	StepNo       int               `json:"step_no"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	TemplateCode string            `json:"template_code"`
	Parameters   map[string]string `json:"parameters"`
	TimeoutSec   int               `json:"timeout_seconds"`
	Status       JobStatus         `json:"status"`
	JobID        string            `json:"job_id,omitempty"`
	Result       *StepResult       `json:"result,omitempty"`
}

// StepResult captures the execution output of a diagnosis step.
type StepResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
	Success  bool   `json:"success"`
	Summary  string `json:"summary,omitempty"`
}

// AnalysisReport is the AI-generated analysis of diagnosis results.
type AnalysisReport struct {
	PlanID          string         `json:"plan_id"`
	FaultType       string         `json:"fault_type"`
	PossibleCauses  []string       `json:"possible_causes"`
	Suggestions     []string       `json:"suggestions"`
	Confidence      float64        `json:"confidence"`
	RawAnalysis     string         `json:"raw_analysis"`
	ReferenceCases  []IncidentCase `json:"reference_cases,omitempty"`
	HistoricalHints []string       `json:"historical_hints,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
}

type AuditLog struct {
	ID           string         `json:"id"`
	ActorType    string         `json:"actor_type"`
	ActorID      string         `json:"actor_id"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type"`
	ResourceID   string         `json:"resource_id"`
	Details      map[string]any `json:"details,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}

type IncidentCase struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	TriggerType    string    `json:"trigger_type"`
	FaultType      string    `json:"fault_type"`
	Summary        string    `json:"summary"`
	PossibleCauses []string  `json:"possible_causes"`
	Suggestions    []string  `json:"suggestions"`
	Confidence     float64   `json:"confidence"`
	SourcePlanID   string    `json:"source_plan_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type IncidentCaseFilter struct {
	Query       string
	TriggerType string
	FaultType   string
	Limit       int
}

type MetricsSnapshot struct {
	JobsTotal                 int     `json:"jobs_total"`
	JobsSuccess               int     `json:"jobs_success"`
	JobsFailed                int     `json:"jobs_failed"`
	JobsWaitingApproval       int     `json:"jobs_waiting_approval"`
	WorkersOnline             int     `json:"workers_online"`
	IncidentCasesTotal        int     `json:"incident_cases_total"`
	JobSuccessRate            float64 `json:"job_success_rate"`
	JobFailureRate            float64 `json:"job_failure_rate"`
	AvgScheduleLatencySeconds float64 `json:"avg_schedule_latency_seconds"`
}
