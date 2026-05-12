package scheduler

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"adp/internal/model"
)

type Store struct {
	mu      sync.RWMutex
	workers map[string]model.Worker
	jobs    map[string]model.Job
	nextID  atomic.Uint64
}

func NewStore() *Store {
	return &Store{
		workers: make(map[string]model.Worker),
		jobs:    make(map[string]model.Job),
	}
}

func (s *Store) RegisterWorker(name, workerType string) model.Worker {
	now := time.Now()
	worker := model.Worker{
		ID:              s.newID("worker"),
		Name:            name,
		WorkerType:      workerType,
		Status:          model.WorkerStatusOnline,
		LastHeartbeatAt: now,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.workers[worker.ID] = worker

	return worker
}

func (s *Store) HeartbeatWorker(id string) (model.Worker, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	worker, ok := s.workers[id]
	if !ok {
		return model.Worker{}, false
	}

	now := time.Now()
	worker.LastHeartbeatAt = now
	worker.Status = model.WorkerStatusOnline
	worker.UpdatedAt = now
	s.workers[id] = worker

	return worker, true
}

func (s *Store) ListWorkers() []model.Worker {
	s.mu.RLock()
	defer s.mu.RUnlock()

	workers := make([]model.Worker, 0, len(s.workers))
	for _, worker := range s.workers {
		workers = append(workers, worker)
	}
	return workers
}

func (s *Store) CreateJob(name, workerType, command string) model.Job {
	now := time.Now()
	job := model.Job{
		ID:         s.newID("job"),
		Name:       name,
		WorkerType: workerType,
		Command:    command,
		Status:     model.JobStatusQueued,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.ID] = job

	return job
}

func (s *Store) ListJobs() []model.Job {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make([]model.Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

func (s *Store) GetJob(id string) (model.Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	job, ok := s.jobs[id]
	return job, ok
}

func (s *Store) AssignNextJob(workerID string) (model.Job, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	worker, ok := s.workers[workerID]
	if !ok {
		return model.Job{}, false
	}

	for id, job := range s.jobs {
		if job.Status != model.JobStatusQueued || job.WorkerType != worker.WorkerType {
			continue
		}

		now := time.Now()
		job.Status = model.JobStatusRunning
		job.AssignedWorkerID = workerID
		job.StartedAt = &now
		job.UpdatedAt = now
		s.jobs[id] = job
		return job, true
	}

	return model.Job{}, false
}

func (s *Store) CompleteJob(workerID, jobID, output string, success bool) (model.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[jobID]
	if !ok {
		return model.Job{}, fmt.Errorf("job not found")
	}

	if job.AssignedWorkerID != workerID {
		return model.Job{}, fmt.Errorf("job is not assigned to worker")
	}

	now := time.Now()
	job.Output = output
	job.FinishedAt = &now
	job.UpdatedAt = now
	if success {
		job.Status = model.JobStatusSuccess
	} else {
		job.Status = model.JobStatusFailed
	}

	s.jobs[jobID] = job
	return job, nil
}

func (s *Store) newID(prefix string) string {
	value := s.nextID.Add(1)
	return fmt.Sprintf("%s-%06d", prefix, value)
}
