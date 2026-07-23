package workerstream

import (
	"errors"
	"fmt"
	"io"
	"log"
	"strings"

	adpv1 "adp/api/proto/adp/v1"
	"adp/internal/domain/model"
	"adp/internal/infrastructure/db"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Service implements the worker gRPC bidirectional stream.
type Service struct {
	adpv1.UnimplementedWorkerServiceServer

	repo        db.Repository
	workerToken string
	hub         *Hub
}

// NewService creates a WorkerService implementation.
func NewService(repo db.Repository, workerToken string, hub *Hub) *Service {
	return &Service{repo: repo, workerToken: workerToken, hub: hub}
}

// Stream is the long-lived bidirectional channel between one worker and server.
func (s *Service) Stream(stream adpv1.WorkerService_StreamServer) error {
	if err := s.authorize(stream); err != nil {
		return err
	}

	first, err := stream.Recv()
	if err != nil {
		return err
	}
	if first.GetRegister() == nil {
		return status.Error(codes.InvalidArgument, "first worker stream message must be register")
	}

	req := first.GetRegister()
	if strings.TrimSpace(req.GetName()) == "" || strings.TrimSpace(req.GetWorkerType()) == "" {
		return status.Error(codes.InvalidArgument, "worker name and type are required")
	}

	worker, err := s.repo.RegisterWorker(req.GetName(), req.GetWorkerType())
	if err != nil {
		return status.Errorf(codes.Internal, "register worker: %v", err)
	}
	_ = s.repo.AddAuditLog(model.AuditLog{
		ActorType:    "worker",
		ActorID:      worker.ID,
		Action:       "worker.registered",
		ResourceType: "worker",
		ResourceID:   worker.ID,
		Details: map[string]any{
			"name":        worker.Name,
			"worker_type": worker.WorkerType,
			"transport":   "grpc_stream",
		},
	})

	outbound := s.hub.register(worker.ID)
	defer s.hub.unregister(worker.ID, outbound)

	if err := stream.Send(&adpv1.ServerEnvelope{
		Payload: &adpv1.ServerEnvelope_Registered{
			Registered: &adpv1.RegisterResponse{
				WorkerId:   worker.ID,
				Name:       worker.Name,
				WorkerType: worker.WorkerType,
			},
		},
	}); err != nil {
		return err
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.receiveLoop(worker.ID, stream)
	}()

	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case err := <-errCh:
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		case msg, ok := <-outbound:
			if !ok {
				return nil
			}
			if err := stream.Send(msg); err != nil {
				return err
			}
		}
	}
}

func (s *Service) authorize(stream adpv1.WorkerService_StreamServer) error {
	if s.workerToken == "" {
		return nil
	}
	md, ok := metadata.FromIncomingContext(stream.Context())
	if !ok {
		return status.Error(codes.Unauthenticated, "missing worker token")
	}
	tokens := md.Get("x-worker-token")
	if len(tokens) == 0 || tokens[0] != s.workerToken {
		return status.Error(codes.Unauthenticated, "invalid worker token")
	}
	return nil
}

func (s *Service) receiveLoop(workerID string, stream adpv1.WorkerService_StreamServer) error {
	for {
		msg, err := stream.Recv()
		if err != nil {
			return err
		}
		switch payload := msg.GetPayload().(type) {
		case *adpv1.WorkerEnvelope_Heartbeat:
			if err := s.handleHeartbeat(workerID, payload.Heartbeat); err != nil {
				return err
			}
		case *adpv1.WorkerEnvelope_JobResult:
			if err := s.handleJobResult(workerID, payload.JobResult); err != nil {
				return err
			}
		case *adpv1.WorkerEnvelope_CommandAck:
			log.Printf("grpc worker stream: worker %s ack command=%s success=%t", workerID, payload.CommandAck.GetCommand(), payload.CommandAck.GetSuccess())
		default:
			log.Printf("grpc worker stream: worker %s ignored empty message", workerID)
		}
	}
}

func (s *Service) handleHeartbeat(workerID string, heartbeat *adpv1.Heartbeat) error {
	if heartbeat == nil {
		return nil
	}
	info := protoHostInfo(heartbeat.GetHostInfo())
	if _, err := s.repo.HeartbeatWorker(workerID, &info); err != nil {
		return status.Errorf(codes.NotFound, "heartbeat worker: %v", err)
	}
	return nil
}

func (s *Service) handleJobResult(workerID string, result *adpv1.JobResult) error {
	if result == nil {
		return nil
	}
	job, err := s.repo.CompleteJob(workerID, result.GetJobId(), result.GetOutput(), result.GetSuccess())
	if err != nil {
		return status.Errorf(codes.FailedPrecondition, "complete job: %v", err)
	}
	action := "job.completed"
	if !result.GetSuccess() {
		action = "job.failed"
	}
	_ = s.repo.AddAuditLog(model.AuditLog{
		ActorType:    "worker",
		ActorID:      workerID,
		Action:       action,
		ResourceType: "job",
		ResourceID:   job.ID,
		Details: map[string]any{
			"success":   result.GetSuccess(),
			"transport": "grpc_stream",
		},
	})
	if result.GetLogToDb() {
		_ = s.repo.AddWorkerLog(model.WorkerLog{
			WorkerID: workerID,
			JobID:    result.GetJobId(),
			Command:  result.GetCommand(),
			Progress: "completed",
			Result:   result.GetOutput(),
			Success:  result.GetSuccess(),
		})
	}
	log.Printf("grpc worker stream: job %s completed by worker %s success=%t", result.GetJobId(), workerID, result.GetSuccess())
	return nil
}

func protoHostInfo(info *adpv1.HostInfo) model.HostInfo {
	if info == nil {
		return model.HostInfo{}
	}
	return model.HostInfo{
		Hostname:     info.GetHostname(),
		IPAddress:    info.GetIpAddress(),
		CPUUsage:     info.GetCpuUsage(),
		StorageUsage: info.GetStorageUsage(),
	}
}

// ConnectedWorkers returns active worker IDs for diagnostics and tests.
func (s *Service) ConnectedWorkers() []string {
	s.hub.mu.RLock()
	defer s.hub.mu.RUnlock()
	ids := make([]string, 0, len(s.hub.workers))
	for id := range s.hub.workers {
		ids = append(ids, id)
	}
	return ids
}

func (s *Service) String() string {
	return fmt.Sprintf("workerstream.Service(connected=%d)", len(s.ConnectedWorkers()))
}
