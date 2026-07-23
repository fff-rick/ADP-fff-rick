package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"adp/internal/domain/model"
)

func TestManagedTemplateConfigReload(t *testing.T) {
	server := NewServer(Config{
		Addr:              ":0",
		AdminUsername:     "admin",
		AdminPassword:     "admin123",
		AuthSecret:        "secret",
		WorkerSharedToken: "worker-secret",
	}, nil, nil)
	app := httptest.NewServer(server.httpServer.Handler)
	defer app.Close()

	token, _, err := server.authService.Login("admin", "admin123")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	status := mustJSONRequest(t, app.Client(), http.MethodPost, app.URL+"/api/v1/configs/templates", token, map[string]any{
		"yaml_content": `code: custom_echo
name: Custom Echo
description: Echo from managed config
tool_type: shell
command: echo {{.Message}}
risk_level: low
parameters:
  - name: Message
    required: true
`,
	}, nil)
	if status != http.StatusCreated {
		t.Fatalf("save config status = %d, want %d", status, http.StatusCreated)
	}

	var templates []model.CommandTemplate
	status = mustJSONRequest(t, app.Client(), http.MethodGet, app.URL+"/api/v1/templates", token, nil, &templates)
	if status != http.StatusOK {
		t.Fatalf("templates status = %d, want %d", status, http.StatusOK)
	}
	for _, tmpl := range templates {
		if tmpl.Code == "custom_echo" && tmpl.Command == "echo {{.Message}}" {
			return
		}
	}
	t.Fatalf("managed template was not loaded: %+v", templates)
}
