package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDashboardUIRoutesAndSummary(t *testing.T) {
	server := NewServer(Config{
		Addr:              ":0",
		AdminUsername:     "admin",
		AdminPassword:     "admin123",
		AuthSecret:        "secret",
		WorkerSharedToken: "worker-secret",
	})
	app := httptest.NewServer(server.httpServer.Handler)
	defer app.Close()

	for _, route := range []string{"/", "/login", "/users", "/workers", "/jobs", "/tasks"} {
		resp, err := app.Client().Get(app.URL + route)
		if err != nil {
			t.Fatalf("GET %s error = %v", route, err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("ReadAll(%s) error = %v", route, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s status = %d, want %d", route, resp.StatusCode, http.StatusOK)
		}
		if !strings.Contains(string(body), "ADP") {
			t.Fatalf("ui page missing expected content for %s: %s", route, string(body))
		}
	}

	staticResp, err := app.Client().Get(app.URL + "/static/app.css")
	if err != nil {
		t.Fatalf("GET /static/app.css error = %v", err)
	}
	defer staticResp.Body.Close()
	if staticResp.StatusCode != http.StatusOK {
		t.Fatalf("GET /static/app.css status = %d, want %d", staticResp.StatusCode, http.StatusOK)
	}

	token, _, err := server.authService.Login("admin", "admin123")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	server.store.RegisterWorker("worker-1", "shell")
	server.store.CreateJob("demo-job", "shell", "echo demo")

	summary := dashboardSummaryResponse{}
	status := mustJSONRequest(t, app.Client(), http.MethodGet, app.URL+"/api/v1/dashboard/summary", token, nil, &summary)
	if status != http.StatusOK {
		t.Fatalf("dashboard summary status = %d, want %d", status, http.StatusOK)
	}
	if summary.Metrics.JobsTotal != 1 {
		t.Fatalf("jobs_total = %d, want 1", summary.Metrics.JobsTotal)
	}
	if len(summary.Workers) != 1 {
		t.Fatalf("workers len = %d, want 1", len(summary.Workers))
	}
	if summary.TemplatesTotal == 0 {
		t.Fatal("expected templates_total > 0")
	}
}
