package planner

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"adp/internal/domain/model"
	"adp/internal/domain/template"
	"adp/internal/infrastructure/llm"
)

const planSystemPrompt = `You are a fault diagnosis planner for an operations system.
Given a fault description, generate a multi-step diagnosis plan.

Supported trigger types:
- nginx_unreachable: Nginx/service unreachable
- redis_slow: Redis performance issues

Output ONLY valid JSON, no extra text:
{
  "title": "...",
  "trigger_type": "nginx_unreachable|redis_slow",
  "steps": [
    {"name": "...", "template_code": "...", "parameters": {...}, "timeout_seconds": 30}
  ]
}

Available templates for steps:
- check_process: parameter ProcessName
- check_port: parameter Port
- read_log_tail: parameters LogFile, Lines
- redis_ping: parameters Host, Port
- redis_info: parameters Host, Port, Section
- redis_slowlog_get: parameters Host, Port, Count
- redis_client_list: parameters Host, Port
- http_health_check: parameters URL, Timeout`

// predefinedPlans maps trigger types to diagnosis step sequences.
var predefinedPlans = map[string]struct {
	Title string
	Steps []model.DiagnosisStep
}{
	"nginx_unreachable": {
		Title: "Nginx 不可访问诊断",
		Steps: []model.DiagnosisStep{
			{
				StepNo: 1, Name: "检查 Nginx 进程存活状态",
				Description:  "确认 Nginx 主进程是否在运行",
				TemplateCode: "check_process",
				Parameters:   map[string]string{"ProcessName": "nginx"},
				TimeoutSec:   15,
			},
			{
				StepNo: 2, Name: "检查 80 端口监听状态",
				Description:  "确认是否有进程监听 80 端口",
				TemplateCode: "check_port",
				Parameters:   map[string]string{"Port": "80"},
				TimeoutSec:   15,
			},
			{
				StepNo: 3, Name: "读取 Nginx 错误日志",
				Description:  "获取最近 50 行错误日志排查异常",
				TemplateCode: "read_log_tail",
				Parameters:   map[string]string{"LogFile": "/var/log/nginx/error.log", "Lines": "50"},
				TimeoutSec:   15,
			},
			{
				StepNo: 4, Name: "HTTP 健康检查",
				Description:  "对本地 Nginx 发起 HTTP 请求验证响应",
				TemplateCode: "http_health_check",
				Parameters:   map[string]string{"URL": "http://127.0.0.1:80", "Timeout": "10"},
				TimeoutSec:   20,
			},
		},
	},
	"redis_slow": {
		Title: "Redis 响应慢诊断",
		Steps: []model.DiagnosisStep{
			{
				StepNo: 1, Name: "Redis 存活检查",
				Description:  "通过 PING 确认 Redis 服务是否正常响应",
				TemplateCode: "redis_ping",
				Parameters:   map[string]string{"Host": "127.0.0.1", "Port": "6379"},
				TimeoutSec:   10,
			},
			{
				StepNo: 2, Name: "Redis 内存使用分析",
				Description:  "获取 Redis 内存使用情况，排查是否因内存不足导致变慢",
				TemplateCode: "redis_info",
				Parameters:   map[string]string{"Host": "127.0.0.1", "Port": "6379", "Section": "memory"},
				TimeoutSec:   15,
			},
			{
				StepNo: 3, Name: "Redis 慢日志查询",
				Description:  "获取最近 10 条慢日志，定位执行耗时的命令",
				TemplateCode: "redis_slowlog_get",
				Parameters:   map[string]string{"Host": "127.0.0.1", "Port": "6379", "Count": "10"},
				TimeoutSec:   15,
			},
			{
				StepNo: 4, Name: "Redis 当前连接数检查",
				Description:  "查看当前客户端连接数，排查连接数过高问题",
				TemplateCode: "redis_client_list",
				Parameters:   map[string]string{"Host": "127.0.0.1", "Port": "6379"},
				TimeoutSec:   10,
			},
		},
	},
}

// PlanStore persists diagnosis plans in memory.
type PlanStore struct {
	mu     sync.RWMutex
	plans  map[string]model.DiagnosisPlan
	nextID int
}

func NewPlanStore() *PlanStore {
	return &PlanStore{
		plans:  make(map[string]model.DiagnosisPlan),
		nextID: 1,
	}
}

func (s *PlanStore) Save(plan model.DiagnosisPlan) model.DiagnosisPlan {
	s.mu.Lock()
	defer s.mu.Unlock()
	if plan.ID == "" {
		plan.ID = fmt.Sprintf("plan-%06d", s.nextID)
		s.nextID++
	}
	s.plans[plan.ID] = plan
	return plan
}

func (s *PlanStore) Get(id string) (model.DiagnosisPlan, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	plan, ok := s.plans[id]
	return plan, ok
}

func (s *PlanStore) Update(id string, fn func(plan *model.DiagnosisPlan)) (model.DiagnosisPlan, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	plan, ok := s.plans[id]
	if !ok {
		return model.DiagnosisPlan{}, false
	}
	fn(&plan)
	s.plans[id] = plan
	return plan, true
}

// Planner generates diagnosis plans from fault descriptions.
type Planner struct {
	llmClient llm.Client
	templates *template.Engine
	store     *PlanStore
}

func New(llmClient llm.Client, templates *template.Engine, store *PlanStore) *Planner {
	return &Planner{
		llmClient: llmClient,
		templates: templates,
		store:     store,
	}
}

// Store returns the plan store.
func (p *Planner) Store() *PlanStore {
	return p.store
}

// GeneratePlan creates a diagnosis plan from a natural language fault description.
func (p *Planner) GeneratePlan(ctx context.Context, description string) (*model.DiagnosisPlan, error) {
	description = strings.TrimSpace(description)
	if description == "" {
		return nil, fmt.Errorf("description is empty")
	}

	triggerType := classifyTrigger(description)
	predefined, ok := predefinedPlans[triggerType]

	if ok {
		return p.buildFromPredefined(description, triggerType, predefined), nil
	}

	if p.llmClient != nil {
		return p.buildFromLLM(ctx, description)
	}

	return nil, fmt.Errorf("no predefined plan for trigger type: %s (and LLM not configured)", triggerType)
}

func (p *Planner) buildFromPredefined(description, triggerType string, predef struct {
	Title string
	Steps []model.DiagnosisStep
}) *model.DiagnosisPlan {
	now := time.Now()
	steps := make([]model.DiagnosisStep, len(predef.Steps))
	copy(steps, predef.Steps)
	for i := range steps {
		steps[i].Status = model.JobStatusPending
	}

	plan := model.DiagnosisPlan{
		Title:       predef.Title,
		Description: description,
		TriggerType: triggerType,
		Steps:       steps,
		Status:      model.PlanStatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	plan = p.store.Save(plan)
	return &plan
}

func (p *Planner) buildFromLLM(ctx context.Context, description string) (*model.DiagnosisPlan, error) {
	messages := []llm.Message{
		{Role: "system", Content: planSystemPrompt},
		{Role: "user", Content: description},
	}

	raw, err := p.llmClient.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("llm call failed: %w", err)
	}

	// Parse LLM response (simplified: expect JSON)
	raw = extractJSON(raw)
	// For now, return error if no predefined plan and LLM response can't be parsed.
	// Full LLM parsing would require a more robust approach.
	_ = raw
	return nil, fmt.Errorf("LLM-based plan generation not yet implemented for custom scenarios; supported types: nginx_unreachable, redis_slow")
}

func classifyTrigger(description string) string {
	lower := strings.ToLower(description)

	nginxKeywords := []string{"nginx", "网站", "网页", "http", "web", "不可访问", "无法访问", "打不开", "unreachable"}
	redisKeywords := []string{"redis", "缓存", "cache", "响应慢", "slow", "慢查询", "性能"}

	for _, kw := range nginxKeywords {
		if strings.Contains(lower, kw) {
			return "nginx_unreachable"
		}
	}
	for _, kw := range redisKeywords {
		if strings.Contains(lower, kw) {
			return "redis_slow"
		}
	}
	return lower
}

func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)
	}
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start != -1 && end > start {
		s = s[start : end+1]
	}
	return s
}
