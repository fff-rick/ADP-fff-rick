package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"adp/internal/domain/model"
)

// handleGenerateYAML uses AI to convert NL input into YAML job definition.
func (s *Server) handleGenerateYAML(w http.ResponseWriter, r *http.Request) {
	var req parseTaskRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Input == "" {
		writeError(w, http.StatusBadRequest, errors.New("input is required"))
		return
	}

	yamlResult, parsed, usedAI, aiErr := s.generateYAMLFromInput(r, req.Input)

	resp := map[string]any{
		"yaml":        yamlResult,
		"used_ai":     usedAI,
		"description": req.Input,
	}
	if aiErr != nil {
		resp["ai_error"] = aiErr.Error()
	}
	if parsed != nil {
		resp["parsed_name"] = parsed.Name
		resp["task_count"] = len(parsed.Tasks)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) generateYAMLFromInput(_ *http.Request, input string) (string, *YAMLJobSpec, bool, error) {
	var aiErr error
	// Try LLM if configured.
	if s.config.LLMBaseURL != "" {
		prompt := s.injectAIContextIntoPrompt(s.promptOrDefault("yaml", yamlSystemPrompt))
		yamlStr, err := callLLMForYAML(s.config.LLMBaseURL, s.config.LLMAPIKey, s.config.LLMModel, prompt, input)
		if err != nil {
			aiErr = err
		} else {
			yamlStr = stripMarkdownFence(yamlStr)
			spec := &YAMLJobSpec{}
			if err := yaml.Unmarshal([]byte(yamlStr), spec); err != nil {
				aiErr = fmt.Errorf("LLM returned invalid YAML: %w", err)
			} else if len(spec.Tasks) == 0 {
				aiErr = errors.New("LLM returned YAML without tasks")
			} else if err := s.validateAndFixYAML(spec); err != nil {
				aiErr = fmt.Errorf("LLM YAML validation failed: %w", err)
			} else {
				return yamlStr, spec, true, nil
			}
		}
	}

	yamlStr, spec := ruleBasedYAML(input)
	s.validateAndFixYAML(spec) //nolint:errcheck
	return yamlStr, spec, false, aiErr
}

func callLLMForYAML(baseURL, apiKey, model, systemPrompt, input string) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": input},
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create LLM request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("call LLM: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		var apiErr struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error.Message != "" {
			return "", fmt.Errorf("LLM API status %d: %s", resp.StatusCode, apiErr.Error.Message)
		}
		return "", fmt.Errorf("LLM API status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode LLM response: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", errors.New("LLM returned no choices")
	}
	content := strings.TrimSpace(result.Choices[0].Message.Content)
	if content == "" {
		return "", errors.New("LLM returned empty content")
	}
	return content, nil
}

func stripMarkdownFence(value string) string {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "```") {
		return value
	}
	lines := strings.Split(value, "\n")
	if len(lines) >= 2 {
		lines = lines[1:]
	}
	if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
		lines = lines[:len(lines)-1]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

const yamlSystemPrompt = `你是 ADP 运维平台的 YAML 任务生成器。将用户的中文/英文输入转为 ADP YAML。

## 可用模块（只能从以下选择，禁止编造）
- mysql_backup: MySQL 备份 (params: Database*, ServiceProfile*)
- http_health_check: HTTP 健康检查 (params: ServiceProfile*)
- check_process: 检查进程 (params: ServiceProfile*)
- check_port: 检查端口 (params: ServiceProfile*)
- read_log_tail: 读取日志 (params: ServiceProfile*, Lines)
- redis_ping: Redis PING (params: ServiceProfile*)
- redis_info: Redis INFO (params: ServiceProfile*, Section)
- redis_slowlog_get: Redis 慢查询 (params: ServiceProfile*, Count)
- redis_client_list: Redis 客户端 (params: ServiceProfile*)

## 输出格式（纯 YAML，禁止 markdown 代码块）
name: <任务名>
tasks:
  - name: <步骤描述>
    template: <模块code>
    parameters:
      key: value
worker_type: shell
workers:
  - all

## 规则
1. 只用上面列出的模块 code
2. 参数值从用户输入提取，必填参数给合理默认值
3. 输出合法 YAML，不要额外文字`

func ruleBasedYAML(input string) (string, *YAMLJobSpec) {
	lower := strings.ToLower(input)
	spec := &YAMLJobSpec{WorkerType: "shell", Workers: []string{"all"}}

	switch {
	case strings.Contains(lower, "mysql") || strings.Contains(lower, "数据库"):
		if strings.Contains(lower, "备份") || strings.Contains(lower, "backup") {
			spec.Name = "MySQL 数据库备份"
			spec.Tasks = []YAMLTask{{
				Name: "备份 MySQL", Template: "mysql_backup",
				Parameters: map[string]string{"Database": "mydb", "ServiceProfile": "mysql_prod"},
			}}
		}
	case strings.Contains(lower, "nginx"):
		spec.Name = "Nginx 诊断"
		spec.Tasks = []YAMLTask{
			{Name: "检查进程", Template: "check_process", Parameters: map[string]string{"ServiceProfile": "nginx_prod"}},
			{Name: "检查端口", Template: "check_port", Parameters: map[string]string{"ServiceProfile": "nginx_prod"}},
			{Name: "健康检查", Template: "http_health_check", Parameters: map[string]string{"ServiceProfile": "adp_http"}},
		}
	case strings.Contains(lower, "redis"):
		spec.Name = "Redis 诊断"
		spec.Tasks = []YAMLTask{
			{Name: "PING", Template: "redis_ping", Parameters: map[string]string{"ServiceProfile": "redis_prod"}},
			{Name: "内存信息", Template: "redis_info", Parameters: map[string]string{"ServiceProfile": "redis_prod"}},
			{Name: "慢查询", Template: "redis_slowlog_get", Parameters: map[string]string{"ServiceProfile": "redis_prod", "Count": "10"}},
		}
	case strings.Contains(lower, "端口") || strings.Contains(lower, "port"):
		spec.Name = "端口检查"
		spec.Tasks = []YAMLTask{{Name: "检查端口", Template: "check_port", Parameters: map[string]string{"ServiceProfile": "nginx_prod"}}}
	case strings.Contains(lower, "进程") || strings.Contains(lower, "process"):
		spec.Tasks = []YAMLTask{{Name: "检查进程", Template: "check_process", Parameters: map[string]string{"ServiceProfile": "nginx_prod"}}}
	default:
		spec.Name = input
		spec.Tasks = []YAMLTask{{Name: "健康检查", Template: "http_health_check", Parameters: map[string]string{"ServiceProfile": "adp_http"}}}
	}

	// Build YAML string.
	var sb strings.Builder
	sb.WriteString("name: " + spec.Name + "\n")
	sb.WriteString("tasks:\n")
	for _, t := range spec.Tasks {
		sb.WriteString("  - name: " + t.Name + "\n")
		sb.WriteString("    template: " + t.Template + "\n")
		if len(t.Parameters) > 0 {
			sb.WriteString("    parameters:\n")
			for k, v := range t.Parameters {
				sb.WriteString("      " + k + ": " + v + "\n")
			}
		}
	}
	sb.WriteString("worker_type: " + spec.WorkerType + "\n")
	sb.WriteString("workers:\n")
	for _, w := range spec.Workers {
		sb.WriteString("  - " + w + "\n")
	}

	return sb.String(), spec
}

// handleSaveYAML saves a YAML definition to the database.
func (s *Server) handleSaveYAML(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		YAMLContent string `json:"yaml_content"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.YAMLContent == "" {
		writeError(w, http.StatusBadRequest, errors.New("yaml_content is required"))
		return
	}
	if req.Name == "" {
		req.Name = "untitled"
	}

	jy, err := s.repo.SaveJobYAML(model.JobYAML{
		Name:        req.Name,
		Description: req.Description,
		YAMLContent: req.YAMLContent,
		Source:      "manual",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, jy)
}

// handleListYAMLs returns all stored YAML definitions.
func (s *Server) handleListYAMLs(w http.ResponseWriter, _ *http.Request) {
	yamls, _ := s.repo.ListJobYAMLs()
	writeJSON(w, http.StatusOK, yamls)
}

// handleRunYAML creates jobs from a stored YAML definition.
func (s *Server) handleRunYAML(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/yamls/")
	id = strings.TrimSuffix(id, "/run")
	if id == "" {
		writeError(w, http.StatusBadRequest, errors.New("yaml id is required"))
		return
	}

	jy, err := s.repo.GetJobYAML(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}

	// Use the existing YAML job creation logic.
	var spec YAMLJobSpec
	if err := yaml.Unmarshal([]byte(jy.YAMLContent), &spec); err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid stored yaml: "+err.Error()))
		return
	}
	if err := s.validateAndFixYAML(&spec); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.createJobsFromSpec(w, r, spec)
}

// createJobsFromSpec creates jobs from a YAMLJobSpec and writes the response.
func (s *Server) createJobsFromSpec(w http.ResponseWriter, r *http.Request, spec YAMLJobSpec) {
	if s.repo == nil {
		writeError(w, http.StatusInternalServerError, errors.New("no store configured"))
		return
	}

	workerIDs := spec.Workers
	if len(workerIDs) == 1 && strings.ToLower(workerIDs[0]) == "all" {
		allWorkers, _ := s.repo.ListWorkers()
		workerIDs = nil
		for _, w := range allWorkers {
			if w.WorkerType == spec.WorkerType && w.Status == model.WorkerStatusOnline {
				workerIDs = append(workerIDs, w.ID)
			}
		}
	}

	var results []model.Job
	targets := workerIDs
	if len(targets) == 0 {
		targets = []string{""}
	}
	for _, task := range spec.Tasks {
		if err := model.ValidateNoInlineSecrets(task.Parameters); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		tmpl, cmd, err := s.templateEng.Render(task.Template, task.Parameters)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := s.policyEng.ValidateTemplate(task.Template); err != nil {
			writeError(w, http.StatusForbidden, err)
			return
		}
		if err := s.policyEng.ValidateCommand(cmd); err != nil {
			writeError(w, http.StatusForbidden, err)
			return
		}
		for _, wid := range targets {
			jobStatus := model.JobStatusPending
			name := fmt.Sprintf("[yaml:%s] %s", spec.Name, task.Name)
			if wid != "" {
				name = fmt.Sprintf("[yaml:%s][w:%s] %s", spec.Name, wid, task.Name)
			}
			job := model.Job{
				Name: name, WorkerType: spec.WorkerType, Command: cmd,
				Status: jobStatus, RiskLevel: tmpl.RiskLevel,
				ApprovalRequired: false, ApprovalStatus: model.ApprovalStatusNotRequired,
				TemplateCode: task.Template, Parameters: cloneStringMap(task.Parameters), SourceType: "yaml_job",
			}
			j, err := s.repo.CreateJob(job)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			if wid != "" {
				if dispatched, err := s.dispatchJobToWorker(j.ID, wid); err == nil {
					j = dispatched
					s.workerHub.PushJob(wid, j)
				}
			}
			results = append(results, j)
		}
	}
	writeJSON(w, http.StatusCreated, map[string]any{"jobs": results, "total": len(results)})
}

// handleYAMLActions routes YAML collection endpoints.
func (s *Server) handleYAMLActions(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/yamls/")
	if strings.HasSuffix(path, "/run") {
		s.handleRunYAML(w, r)
		return
	}
	if r.Method == http.MethodDelete {
		s.handleDeleteYAML(w, r)
		return
	}
	writeError(w, http.StatusNotFound, errors.New("unsupported yaml route"))
}

// validModules is the whitelist of allowed template codes.
// validateAndFixYAML validates a parsed YAML spec and fills in missing parameters.
func (s *Server) validateAndFixYAML(spec *YAMLJobSpec) error {
	if len(spec.Tasks) == 0 {
		return errors.New("tasks list is empty")
	}
	for i, task := range spec.Tasks {
		if task.Parameters == nil {
			task.Parameters = make(map[string]string)
			spec.Tasks[i].Parameters = task.Parameters
		}
		if _, ok := s.templateEng.GetTemplate(task.Template); !ok {
			return fmt.Errorf("task %d: unknown template '%s'", i+1, task.Template)
		}
		// Fill defaults from AI context.
		if s.aiContext != nil {
			s.aiContext.FillDefaults(task.Parameters, task.Template)
		}
		// Validate required params by checking the module's parameter definitions.
		if mod, err := s.moduleReg.Get(task.Template); err == nil {
			for _, p := range mod.Parameters() {
				if p.Required && task.Parameters[p.Name] == "" {
					// Auto-fill from context or use default.
					if p.Default != "" {
						task.Parameters[p.Name] = p.Default
					}
				}
			}
		}
		if err := model.ValidateServiceProfile(task.Template, task.Parameters); err != nil {
			return fmt.Errorf("task %d: %w", i+1, err)
		}
	}
	if spec.WorkerType == "" {
		spec.WorkerType = "shell"
	}
	return nil
}

// injectAIContextIntoPrompt prepends AI context configuration to the base prompt.
func (s *Server) injectAIContextIntoPrompt(basePrompt string) string {
	if s.aiContext == nil {
		return basePrompt
	}
	return s.aiContext.ToPromptSection() + "\n" + basePrompt
}

// handleDeleteYAML deletes a stored YAML.
func (s *Server) handleDeleteYAML(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/yamls/")
	if id == "" {
		writeError(w, http.StatusBadRequest, errors.New("yaml id is required"))
		return
	}
	if err := s.repo.DeleteJobYAML(id); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
