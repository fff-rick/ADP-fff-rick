package workerstream

import (
	"context"
	"net"
	"testing"
	"time"

	adpv1 "adp/api/proto/adp/v1"
	"adp/internal/domain/model"
	"adp/internal/infrastructure/db"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
)

func TestWorkerStreamDispatchAndComplete(t *testing.T) {
	repo := db.NewMemoryRepository()
	hub := NewHub()
	grpcServer := grpc.NewServer()
	adpv1.RegisterWorkerServiceServer(grpcServer, NewService(repo, "worker-secret", hub))

	listener := bufconn.Listen(1024 * 1024)
	defer func() { _ = listener.Close() }()
	go func() {
		_ = grpcServer.Serve(listener)
	}()
	defer grpcServer.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer conn.Close() //nolint:errcheck

	streamCtx := metadata.AppendToOutgoingContext(ctx, "x-worker-token", "worker-secret")
	stream, err := adpv1.NewWorkerServiceClient(conn).Stream(streamCtx)
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	if err := stream.Send(&adpv1.WorkerEnvelope{
		Payload: &adpv1.WorkerEnvelope_Register{
			Register: &adpv1.RegisterRequest{
				Name:       "worker-1",
				WorkerType: "shell",
			},
		},
	}); err != nil {
		t.Fatalf("send register: %v", err)
	}

	registered, err := stream.Recv()
	if err != nil {
		t.Fatalf("recv registered: %v", err)
	}
	workerID := registered.GetRegistered().GetWorkerId()
	if workerID == "" {
		t.Fatalf("expected worker id, got %+v", registered)
	}

	job, err := repo.CreateJob(model.Job{
		Name:       "echo",
		WorkerType: "shell",
		Command:    "echo ok",
		Status:     model.JobStatusPending,
	})
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
	if err := repo.AssignJobToWorkers(job.ID, []string{workerID}); err != nil {
		t.Fatalf("AssignJobToWorkers() error = %v", err)
	}
	job, err = repo.GetJob(job.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}

	if !hub.PushJob(workerID, job) {
		t.Fatal("expected PushJob to connected worker")
	}
	pushed, err := stream.Recv()
	if err != nil {
		t.Fatalf("recv job: %v", err)
	}
	if pushed.GetJob().GetId() != job.ID {
		t.Fatalf("pushed job id = %s, want %s", pushed.GetJob().GetId(), job.ID)
	}

	if err := stream.Send(&adpv1.WorkerEnvelope{
		Payload: &adpv1.WorkerEnvelope_JobResult{
			JobResult: &adpv1.JobResult{
				WorkerId: workerID,
				JobId:    job.ID,
				Command:  job.Command,
				Success:  true,
				Output:   "ok",
			},
		},
	}); err != nil {
		t.Fatalf("send job result: %v", err)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		completed, err := repo.GetJob(job.ID)
		if err != nil {
			t.Fatalf("GetJob(completed) error = %v", err)
		}
		if completed.Status == model.JobStatusSuccess {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job did not become success")
}
