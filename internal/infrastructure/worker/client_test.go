package worker

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"adp/internal/domain/model"
)

func TestExecuteJobPassesJobParametersToModule(t *testing.T) {
	const procName = "adp-definitely-missing-process"

	var completion struct {
		Success bool   `json:"success"`
		Output  string `json:"output"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workers/worker-1/jobs/job-1/complete" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&completion); err != nil {
			t.Fatalf("decode completion: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "worker-secret", "worker-1", "shell", time.Second)
	client.registeredID = "worker-1"
	client.executeJob(model.Job{
		ID:           "job-1",
		WorkerType:   "shell",
		TemplateCode: "check_process",
		Parameters: map[string]string{
			"ProcessName": procName,
		},
	})

	if !completion.Success {
		t.Fatalf("completion success = false, output=%q", completion.Output)
	}
	if !strings.Contains(completion.Output, procName) {
		t.Fatalf("expected output to contain process parameter %q, got %q", procName, completion.Output)
	}
}
