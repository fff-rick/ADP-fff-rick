package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"adp/internal/domain/model"
	"adp/internal/domain/policy"
	"adp/internal/domain/template"
	"adp/internal/infrastructure/llm"
)

const systemPrompt = `You are a task parsing assistant for an operations scheduling system.
Parse the user's natural language input into structured JSON.

Supported intents:
- create_scheduled_backup: periodic backup task (target_type: mysql)
- health_check: service health check (target_type: http_service)
- diagnose: performance diagnosis (target_type: nginx, redis)

Output ONLY valid JSON, no extra text. Format:
{"intent":"...","target_type":"...","schedule":"...","risk_level":"low|medium|high","parameters":{}}

Rules:
- schedule: use standard 5-field cron if periodic, empty string if immediate
- risk_level: "high" for delete/drop/restart/reboot/shutdown/kill; "medium" for backup/write operations; "low" for read-only queries and checks
- If input is unrecognizable or asks for destructive actions, intent = "unsupported", risk_level = "high"
- parameters: extract key-value details from the input (host, port, database name, url, etc.)`

// intentRegexes maps keywords to (intent, target_type) for rule-based matching.
var intentRegexes = []struct {
	pattern    *regexp.Regexp
	intent     string
	targetType string
	riskLevel  model.RiskLevel
}{
	{regexp.MustCompile(`(mysql|数据库|mariadb).*(备份|backup|dump)`), "create_scheduled_backup", "mysql", model.RiskLevelMedium},
	{regexp.MustCompile(`(备份|backup|dump).*(mysql|数据库|mariadb)`), "create_scheduled_backup", "mysql", model.RiskLevelMedium},
	{regexp.MustCompile(`健康检查|health.?check|(检查|检测).*(http|网站|服务|nginx|url)`), "health_check", "http_service", model.RiskLevelLow},
	{regexp.MustCompile(`(http|网站|服务|nginx|url).*(检查|检测|可用|存活)`), "health_check", "http_service", model.RiskLevelLow},
	{regexp.MustCompile(`(redis).*(慢|slow|性能|诊断|响应|排查)`), "diagnose", "redis", model.RiskLevelLow},
	{regexp.MustCompile(`(nginx).*(不可用|无法访问|诊断|排查|故障|问题)`), "diagnose", "nginx", model.RiskLevelLow},
}

// Parser converts natural language into structured TaskIntent.
type Parser struct {
	llmClient llm.Client
	templates *template.Engine
	policy    *policy.Engine
}

// NewParser creates a Parser. llmClient may be nil; rule-based fallback is used in that case.
func NewParser(llmClient llm.Client, templates *template.Engine, policy *policy.Engine) *Parser {
	return &Parser{
		llmClient: llmClient,
		templates: templates,
		policy:    policy,
	}
}

// Parse converts natural language input into a TaskIntent.
func (p *Parser) Parse(ctx context.Context, input string) (*model.TaskIntent, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("input is empty")
	}

	var intent *model.TaskIntent
	var err error

	if p.llmClient != nil {
		intent, err = p.parseWithLLM(ctx, input)
		if err != nil {
			// Fall back to rule-based on LLM failure.
			intent, err = p.parseWithRules(input)
		}
	} else {
		intent, err = p.parseWithRules(input)
	}

	if err != nil {
		return nil, fmt.Errorf("parse failed: %w", err)
	}

	if intent.Intent == "unsupported" {
		return nil, fmt.Errorf("unrecognized or unsupported task: %s", input)
	}

	p.matchTemplate(intent)

	return intent, nil
}

func (p *Parser) parseWithLLM(ctx context.Context, input string) (*model.TaskIntent, error) {
	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: input},
	}

	raw, err := p.llmClient.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("llm call failed: %w", err)
	}

	// Extract JSON from response (handle markdown code blocks).
	raw = extractJSON(raw)

	var intent model.TaskIntent
	if err := json.Unmarshal([]byte(raw), &intent); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response as JSON: %w (raw=%s)", err, truncate(raw, 200))
	}

	if intent.Intent == "" {
		return nil, fmt.Errorf("LLM returned empty intent")
	}

	return &intent, nil
}

func (p *Parser) parseWithRules(input string) (*model.TaskIntent, error) {
	lower := strings.ToLower(input)

	for _, r := range intentRegexes {
		if r.pattern.MatchString(lower) {
			intent := &model.TaskIntent{
				Intent:     r.intent,
				TargetType: r.targetType,
				RiskLevel:  r.riskLevel,
			}
			intent.Schedule = extractCron(lower)
			intent.Parameters = extractParams(lower, r.targetType)
			return intent, nil
		}
	}

	return nil, fmt.Errorf("unable to parse with rule-based parser: %s", input)
}

// matchTemplate finds the best-matching command template for a parsed intent.
func (p *Parser) matchTemplate(intent *model.TaskIntent) {
	intentToTemplate := map[string]string{
		"create_scheduled_backup": "mysql_backup",
		"health_check":            "http_health_check",
	}
	if code, ok := intentToTemplate[intent.Intent]; ok {
		intent.MatchedTemplate = code
	}
}

// extractJSON extracts JSON from a string that may contain markdown fences.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)

	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)
	}

	// Find the outermost { }.
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start != -1 && end > start {
		s = s[start : end+1]
	}

	return s
}

func extractCron(input string) string {
	if strings.Contains(input, "每天") || strings.Contains(input, "每日") || strings.Contains(input, "daily") {
		return "0 0 * * *"
	}
	if strings.Contains(input, "每小时") || strings.Contains(input, "hourly") {
		return "0 * * * *"
	}
	if strings.Contains(input, "每周") || strings.Contains(input, "weekly") {
		return "0 0 * * 0"
	}
	return ""
}

func extractParams(input string, targetType string) map[string]string {
	params := make(map[string]string)
	lower := strings.ToLower(input)

	switch targetType {
	case "mysql":
		params["Database"] = extractAfter(lower, "数据库", "database", "db")
		if params["Database"] == "" {
			params["Database"] = "mydb"
		}
		params["Host"] = "127.0.0.1"
		params["Port"] = "3306"
		params["OutputFile"] = "/tmp/backup.sql"
	case "http_service":
		params["URL"] = extractAfter(lower, "url", "地址", "网址")
		if params["URL"] == "" {
			params["URL"] = "http://127.0.0.1:80"
		}
		params["Timeout"] = "10"
	}

	return params
}

func extractAfter(input string, keywords ...string) string {
	for _, kw := range keywords {
		idx := strings.Index(input, kw)
		if idx == -1 {
			continue
		}
		rest := strings.TrimSpace(input[idx+len(kw):])
		// Take the next word/token.
		parts := strings.Fields(rest)
		if len(parts) > 0 {
			return strings.Trim(parts[0], ".,;:!\"'（）()")
		}
	}
	return ""
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
