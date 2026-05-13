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
	JobStatusPending JobStatus = "pending"
	JobStatusQueued  JobStatus = "queued"
	JobStatusRunning JobStatus = "running"
	JobStatusSuccess JobStatus = "success"
	JobStatusFailed  JobStatus = "failed"
)

type Job struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	WorkerType       string     `json:"worker_type"`
	Command          string     `json:"command"`
	Status           JobStatus  `json:"status"`
	AssignedWorkerID string     `json:"assigned_worker_id,omitempty"`
	Output           string     `json:"output,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
	FinishedAt       *time.Time `json:"finished_at,omitempty"`
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
