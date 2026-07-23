package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"adp/internal/config"
	"adp/internal/domain/model"
	"adp/internal/domain/policy"
	"adp/internal/domain/template"
	"adp/internal/infrastructure/llm"
)

func buildSystemPrompt(ctx *config.AIContext) string {
	var sb strings.Builder

	// Part 1: Environment context (if available)
	if ctx != nil {
		sb.WriteString(ctx.ToPromptSection())
	}

	// Part 2: Module reference + format + examples
	sb.WriteString(`你是 ADP 运维平台的任务意图解析器。将用户的中文/英文输入转为结构化 JSON。

## 支持的 intent

- create_scheduled_backup: MySQL/数据库备份任务
- health_check: 单一 HTTP/URL 健康检查任务
- read_logs: 单一日志读取任务
- diagnose: 多步骤故障诊断或排查任务，例如 nginx 不可访问、nginx 进程和日志检查、redis 响应慢
- unsupported: 无法识别或不支持的任务

## target_type

- mysql
- http_service
- log_file
- nginx
- redis
- unknown

## 输出格式（严格 JSON，不要 markdown 代码块，不要额外解释）

{
  "intent": "create_scheduled_backup|health_check|read_logs|diagnose|unsupported",
  "target_type": "mysql|http_service|log_file|nginx|redis|unknown",
  "schedule": "",
  "risk_level": "low|medium|high",
  "parameters": {}
}

## 示例

输入: "每天凌晨备份生产数据库 demo"
输出: {"intent":"create_scheduled_backup","target_type":"mysql","schedule":"0 0 * * *","risk_level":"medium","parameters":{"Database":"demo"}}

输入: "检查 http://127.0.0.1:8080 是否正常"
输出: {"intent":"health_check","target_type":"http_service","risk_level":"low","parameters":{"URL":"http://127.0.0.1:8080","Timeout":"10"}}

输入: "帮我检查 nginx 是否正常运行，并查看错误日志"
输出: {"intent":"diagnose","target_type":"nginx","risk_level":"low","parameters":{"ServiceType":"nginx"}}

输入: "查看 nginx 错误日志"
输出: {"intent":"read_logs","target_type":"log_file","risk_level":"low","parameters":{"LogFile":"/var/log/nginx/error.log","Lines":"50"}}

## 严格规则
1. 只输出 JSON
2. 不要输出 YAML
3. 多步骤排查场景优先归类为 diagnose
4. 如果输入完全无法理解，输出 unsupported`)

	return sb.String()
}

// BuildTaskIntentPrompt returns the enhanced prompt for task intent parsing.
func BuildTaskIntentPrompt(ctx *config.AIContext) string {
	return buildSystemPrompt(ctx)
}

// BuildYAMLPrompt is kept for compatibility. New code should use BuildTaskIntentPrompt.
func BuildYAMLPrompt(ctx *config.AIContext) string {
	return BuildTaskIntentPrompt(ctx)
}

// intentRegexes maps keywords to (intent, target_type) for rule-based matching.
var intentRegexes = []struct {
	pattern    *regexp.Regexp
	intent     string
	targetType string
	riskLevel  model.RiskLevel
}{
	{regexp.MustCompile(`(mysql|数据库|mariadb).*(备份|backup|dump)`), "create_scheduled_backup", "mysql", model.RiskLevelMedium},
	{regexp.MustCompile(`(备份|backup|dump).*(mysql|数据库|mariadb)`), "create_scheduled_backup", "mysql", model.RiskLevelMedium},
	{regexp.MustCompile(`(redis).*(慢|slow|性能|诊断|响应|排查|日志|log)`), "diagnose", "redis", model.RiskLevelLow},
	{regexp.MustCompile(`(nginx).*(不可用|无法访问|诊断|排查|故障|问题|日志|log|错误|error|进程|process|运行)`), "diagnose", "nginx", model.RiskLevelLow},
	{regexp.MustCompile(`(检查|检测).*(nginx).*(日志|log|错误|error|进程|process|运行)`), "diagnose", "nginx", model.RiskLevelLow},
	{regexp.MustCompile(`(日志|log|tail).*(redis|nginx|获取|查看|读取|get)`), "read_logs", "log_file", model.RiskLevelLow},
	{regexp.MustCompile(`(获取|查看|读取|get).*(日志|log)`), "read_logs", "log_file", model.RiskLevelLow},
	{regexp.MustCompile(`健康检查|health.?check|(检查|检测).*(http|网站|服务|url)`), "health_check", "http_service", model.RiskLevelLow},
	{regexp.MustCompile(`(http|网站|服务|url).*(检查|检测|可用|存活)`), "health_check", "http_service", model.RiskLevelLow},
}

// Parser converts natural language into structured TaskIntent.
type Parser struct {
	llmClient    llm.Client
	templates    *template.Engine
	policy       *policy.Engine
	systemPrompt string
}

// NewParser creates a Parser. llmClient may be nil; rule-based fallback is used in that case.
func NewParser(llmClient llm.Client, templates *template.Engine, policy *policy.Engine) *Parser {
	return &Parser{
		llmClient:    llmClient,
		templates:    templates,
		policy:       policy,
		systemPrompt: buildSystemPrompt(nil),
	}
}

// SetAIContext injects AI context configuration into the parser's system prompt.
func (p *Parser) SetAIContext(ctx *config.AIContext) {
	if ctx != nil {
		p.systemPrompt = buildSystemPrompt(ctx)
	}
}

// SetSystemPrompt replaces the parser system prompt at runtime.
func (p *Parser) SetSystemPrompt(prompt string) {
	prompt = strings.TrimSpace(prompt)
	if prompt != "" {
		p.systemPrompt = prompt
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
		{Role: "system", Content: p.systemPrompt},
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
		"read_logs":               "read_log_tail",
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
	case "log_file":
		if strings.Contains(lower, "redis") {
			params["LogFile"] = "/var/log/redis/redis-server.log"
		} else if strings.Contains(lower, "nginx") {
			params["LogFile"] = "/var/log/nginx/error.log"
		} else {
			params["LogFile"] = "/var/log/syslog"
		}
		params["Lines"] = "50"
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
