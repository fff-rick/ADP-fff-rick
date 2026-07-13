package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"adp/internal/domain/model"
	"adp/internal/interfaces/http"
)

func TestPhase6Acceptance(t *testing.T) {
	srv := api.NewServer(api.Config{
		Addr:              ":0",
		AdminUsername:     "admin",
		AdminPassword:     "admin123",
		AuthSecret:        "secret",
		WorkerSharedToken: "worker-secret",
	})
	app := httptest.NewServer(srv.Handler())
	defer app.Close()

	token := loginForIntegration(t, app.URL)

	t.Run("mysql backup approval flow", func(t *testing.T) {
		runResp := struct {
			Job              model.Job `json:"job"`
			ApprovalRequired bool      `json:"approval_required"`
		}{}
		status := doUserJSON(t, app.Client(), http.MethodPost, app.URL+"/api/v1/tasks/run", token, map[string]any{
			"input": "backup mysql database daily",
			"parameters": map[string]string{
				"Password": "secret",
				"Database": "demo",
			},
		}, &runResp)
		if status != http.StatusAccepted {
			t.Fatalf("tasks/run status = %d, want %d", status, http.StatusAccepted)
		}
		if !runResp.ApprovalRequired || runResp.Job.Status != model.JobStatusWaitingApproval {
			t.Fatalf("expected waiting approval job, got approval_required=%t status=%s", runResp.ApprovalRequired, runResp.Job.Status)
		}

		approveResp := model.Job{}
		status = doUserJSON(t, app.Client(), http.MethodPost, app.URL+"/api/v1/approvals/jobs/"+runResp.Job.ID, token, map[string]any{
			"approved": true,
			"comment":  "phase6 acceptance",
		}, &approveResp)
		if status != http.StatusOK {
			t.Fatalf("approve status = %d, want %d", status, http.StatusOK)
		}
		if approveResp.Status != model.JobStatusQueued {
			t.Fatalf("approved status = %s, want queued", approveResp.Status)
		}

		worker := registerWorkerForIntegration(t, app.URL)
		job := pollJobForIntegration(t, app.URL, worker.ID)
		if job == nil || job.ID != runResp.Job.ID {
			t.Fatalf("expected approved backup job to be assigned, got %+v", job)
		}

		status = doWorkerJSON(t, app.Client(), http.MethodPost, fmt.Sprintf("%s/api/v1/workers/%s/jobs/%s/complete", app.URL, worker.ID, job.ID), map[string]any{
			"success": true,
			"output":  "backup completed",
		}, nil)
		if status != http.StatusOK {
			t.Fatalf("complete status = %d, want %d", status, http.StatusOK)
		}

		finalJob := model.Job{}
		status = doUserJSON(t, app.Client(), http.MethodGet, app.URL+"/api/v1/jobs/"+job.ID, token, nil, &finalJob)
		if status != http.StatusOK {
			t.Fatalf("get job status = %d, want %d", status, http.StatusOK)
		}
		if finalJob.Status != model.JobStatusSuccess {
			t.Fatalf("final job status = %s, want success", finalJob.Status)
		}
	})

	t.Run("nginx diagnosis acceptance", func(t *testing.T) {
		planID := createPlanForIntegration(t, app.URL, token, "nginx is unreachable")
		executePlanForIntegration(t, app.URL, token, planID)
		plan := getPlanForIntegration(t, app.URL, token, planID)
		worker := registerWorkerForIntegration(t, app.URL)
		runDiagnosisJobs(t, app.URL, worker.ID, plan, map[string]workerCompletion{
			"check_process":     {Success: false, Output: ""},
			"check_port":        {Success: false, Output: ""},
			"read_log_tail":     {Success: true, Output: "permission denied"},
			"http_health_check": {Success: false, Output: "000"},
		})

		report := analyzePlanForIntegration(t, app.URL, token, planID)
		if report.FaultType == "" {
			t.Fatal("expected nginx analysis report fault type")
		}

		var cases []model.IncidentCase
		status := doUserJSON(t, app.Client(), http.MethodGet, app.URL+"/api/v1/cases?trigger_type=nginx_unreachable", token, nil, &cases)
		if status != http.StatusOK {
			t.Fatalf("cases status = %d, want %d", status, http.StatusOK)
		}
		if len(cases) == 0 {
			t.Fatal("expected nginx incident case to be stored")
		}
	})

	t.Run("redis diagnosis acceptance", func(t *testing.T) {
		planID := createPlanForIntegration(t, app.URL, token, "redis is slow")
		executePlanForIntegration(t, app.URL, token, planID)
		plan := getPlanForIntegration(t, app.URL, token, planID)
		worker := registerWorkerForIntegration(t, app.URL)
		runDiagnosisJobs(t, app.URL, worker.ID, plan, map[string]workerCompletion{
			"redis_ping":        {Success: true, Output: "PONG"},
			"redis_info":        {Success: true, Output: "used_memory:104857600"},
			"redis_slowlog_get": {Success: true, Output: "1) 1\n2) GET user:1"},
			"redis_client_list": {Success: true, Output: strings.Repeat("id=1 addr=127.0.0.1:6379\n", 55)},
		})

		report := analyzePlanForIntegration(t, app.URL, token, planID)
		if report.FaultType == "" {
			t.Fatal("expected redis analysis report fault type")
		}

		suggestions := struct {
			ReferenceCases  []model.IncidentCase `json:"reference_cases"`
			HistoricalHints []string             `json:"historical_hints"`
		}{}
		status := doUserJSON(t, app.Client(), http.MethodGet, app.URL+"/api/v1/cases/suggestions?description=redis+is+slow&trigger_type=redis_slow", token, nil, &suggestions)
		if status != http.StatusOK {
			t.Fatalf("case suggestions status = %d, want %d", status, http.StatusOK)
		}
		if len(suggestions.ReferenceCases) == 0 {
			t.Fatal("expected redis reference cases")
		}
		if len(suggestions.HistoricalHints) == 0 {
			t.Fatal("expected redis historical hints")
		}
	})
}

type workerCompletion struct {
	Success bool
	Output  string
}

func loginForIntegration(t *testing.T, baseURL string) string {
	t.Helper()

	resp := struct {
		Token string `json:"token"`
	}{}
	status := doRawJSON(t, http.DefaultClient, http.MethodPost, baseURL+"/api/v1/auth/login", map[string]any{
		"username": "admin",
		"password": "admin123",
	}, nil, &resp)
	if status != http.StatusOK {
		t.Fatalf("login status = %d, want %d", status, http.StatusOK)
	}
	if resp.Token == "" {
		t.Fatal("expected login token")
	}
	return resp.Token
}

func registerWorkerForIntegration(t *testing.T, baseURL string) model.Worker {
	t.Helper()

	worker := model.Worker{}
	status := doWorkerJSON(t, http.DefaultClient, http.MethodPost, baseURL+"/api/v1/workers/register", map[string]string{
		"name":        "worker-1",
		"worker_type": "shell",
	}, &worker)
	if status != http.StatusCreated {
		t.Fatalf("register worker status = %d, want %d", status, http.StatusCreated)
	}
	return worker
}

func pollJobForIntegration(t *testing.T, baseURL, workerID string) *model.Job {
	t.Helper()

	resp := struct {
		Job *model.Job `json:"job"`
	}{}
	status := doWorkerJSON(t, http.DefaultClient, http.MethodPost, fmt.Sprintf("%s/api/v1/workers/%s/poll", baseURL, workerID), map[string]string{}, &resp)
	if status != http.StatusOK {
		t.Fatalf("worker poll status = %d, want %d", status, http.StatusOK)
	}
	return resp.Job
}

func createPlanForIntegration(t *testing.T, baseURL, token, description string) string {
	t.Helper()

	plan := model.DiagnosisPlan{}
	status := doUserJSON(t, http.DefaultClient, http.MethodPost, baseURL+"/api/v1/diagnosis/plan", token, map[string]string{
		"description": description,
	}, &plan)
	if status != http.StatusCreated {
		t.Fatalf("create plan status = %d, want %d", status, http.StatusCreated)
	}
	if plan.ID == "" {
		t.Fatal("expected plan id")
	}
	return plan.ID
}

func executePlanForIntegration(t *testing.T, baseURL, token, planID string) {
	t.Helper()
	status := doUserJSON(t, http.DefaultClient, http.MethodPost, fmt.Sprintf("%s/api/v1/diagnosis/plan/%s/execute", baseURL, planID), token, map[string]string{}, nil)
	if status != http.StatusOK {
		t.Fatalf("execute plan status = %d, want %d", status, http.StatusOK)
	}
}

func getPlanForIntegration(t *testing.T, baseURL, token, planID string) model.DiagnosisPlan {
	t.Helper()

	plan := model.DiagnosisPlan{}
	status := doUserJSON(t, http.DefaultClient, http.MethodGet, fmt.Sprintf("%s/api/v1/diagnosis/plan/%s", baseURL, planID), token, nil, &plan)
	if status != http.StatusOK {
		t.Fatalf("get plan status = %d, want %d", status, http.StatusOK)
	}
	return plan
}

func analyzePlanForIntegration(t *testing.T, baseURL, token, planID string) model.AnalysisReport {
	t.Helper()

	report := model.AnalysisReport{}
	status := doUserJSON(t, http.DefaultClient, http.MethodPost, fmt.Sprintf("%s/api/v1/diagnosis/plan/%s/analyze", baseURL, planID), token, map[string]string{}, &report)
	if status != http.StatusOK {
		t.Fatalf("analyze plan status = %d, want %d", status, http.StatusOK)
	}
	return report
}

func runDiagnosisJobs(t *testing.T, baseURL, workerID string, plan model.DiagnosisPlan, completions map[string]workerCompletion) {
	t.Helper()

	byJobID := make(map[string]workerCompletion, len(plan.Steps))
	for _, step := range plan.Steps {
		completion, ok := completions[step.TemplateCode]
		if !ok {
			t.Fatalf("missing completion payload for template %s", step.TemplateCode)
		}
		byJobID[step.JobID] = completion
	}

	for range plan.Steps {
		job := pollJobForIntegration(t, baseURL, workerID)
		if job == nil {
			t.Fatal("expected diagnosis job to be assigned")
		}

		completion, ok := byJobID[job.ID]
		if !ok {
			t.Fatalf("unexpected diagnosis job id %s", job.ID)
		}

		status := doWorkerJSON(t, http.DefaultClient, http.MethodPost, fmt.Sprintf("%s/api/v1/workers/%s/jobs/%s/complete", baseURL, workerID, job.ID), map[string]any{
			"success": completion.Success,
			"output":  completion.Output,
		}, nil)
		if status != http.StatusOK {
			t.Fatalf("complete diagnosis job status = %d, want %d", status, http.StatusOK)
		}
		delete(byJobID, job.ID)
	}

	if len(byJobID) != 0 {
		t.Fatalf("some diagnosis jobs were not completed: %d", len(byJobID))
	}
}

func doUserJSON(t *testing.T, client *http.Client, method, url, token string, body any, target any) int {
	t.Helper()
	headers := map[string]string{
		"Authorization": "Bearer " + token,
		"Content-Type":  "application/json",
	}
	return doRawJSON(t, client, method, url, body, headers, target)
}

func doWorkerJSON(t *testing.T, client *http.Client, method, url string, body any, target any) int {
	t.Helper()
	headers := map[string]string{
		"X-Worker-Token": "worker-secret",
		"Content-Type":   "application/json",
	}
	return doRawJSON(t, client, method, url, body, headers, target)
}

func doRawJSON(t *testing.T, client *http.Client, method, url string, body any, headers map[string]string, target any) int {
	t.Helper()

	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
	}

	req, err := http.NewRequest(method, url, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("client.Do() error = %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if target != nil {
		if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
	} else {
		_, _ = io.Copy(io.Discard, resp.Body)
	}

	return resp.StatusCode
}
