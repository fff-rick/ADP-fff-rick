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
