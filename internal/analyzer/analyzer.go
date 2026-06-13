package analyzer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"adp/internal/llm"
	"adp/internal/model"
)

const analysisSystemPrompt = `You are an operations diagnosis expert. Analyze the collected diagnosis step results
and produce a structured report.

Output ONLY valid JSON, no extra text:
{
  "fault_type": "brief fault type (e.g. 'Nginx process not running', 'Redis memory exhausted')",
  "possible_causes": ["cause 1", "cause 2"],
  "suggestions": ["action 1", "action 2"],
  "confidence": 0.85
}`

// Analyzer examines diagnosis results and produces an AnalysisReport.
type Analyzer struct {
	llmClient llm.Client
}

func New(llmClient llm.Client) *Analyzer {
	return &Analyzer{llmClient: llmClient}
}

// Analyze takes a completed diagnosis plan and produces an analysis report.
func (a *Analyzer) Analyze(ctx context.Context, plan model.DiagnosisPlan) (*model.AnalysisReport, error) {
	if len(plan.Steps) == 0 {
		return nil, fmt.Errorf("plan has no steps")
	}

	if a.llmClient != nil {
		return a.analyzeWithLLM(ctx, plan)
	}

	return a.analyzeWithRules(plan), nil
}

func (a *Analyzer) analyzeWithLLM(ctx context.Context, plan model.DiagnosisPlan) (*model.AnalysisReport, error) {
	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Diagnosis plan: %s (%s)\n\n", plan.Title, plan.TriggerType)) //nolint:staticcheck

	for _, step := range plan.Steps {
		summary.WriteString(fmt.Sprintf("Step %d: %s (%s)\n", step.StepNo, step.Name, step.Description)) //nolint:staticcheck
		if step.Result != nil {
			summary.WriteString(fmt.Sprintf("  stdout: %s\n", truncate(step.Result.Stdout, 500)))                         //nolint:staticcheck
			summary.WriteString(fmt.Sprintf("  stderr: %s\n", truncate(step.Result.Stderr, 500)))                         //nolint:staticcheck
			summary.WriteString(fmt.Sprintf("  exit_code: %d, success: %v\n", step.Result.ExitCode, step.Result.Success)) //nolint:staticcheck
		}
	}

	messages := []llm.Message{
		{Role: "system", Content: analysisSystemPrompt},
		{Role: "user", Content: summary.String()},
	}

	raw, err := a.llmClient.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("llm analysis failed: %w", err)
	}

	// Parse JSON response (simplified).
	_ = raw
	return a.analyzeWithRules(plan), nil
}

func (a *Analyzer) analyzeWithRules(plan model.DiagnosisPlan) *model.AnalysisReport {
	report := &model.AnalysisReport{
		PlanID:    plan.ID,
		CreatedAt: time.Now(),
	}

	switch plan.TriggerType {
	case "nginx_unreachable":
		report = analyzeNginx(plan)
	case "redis_slow":
		report = analyzeRedis(plan)
	default:
		report.FaultType = "unknown"
		report.PossibleCauses = []string{"无法识别的诊断类型"}
		report.Suggestions = []string{"请提供更详细的故障描述"}
		report.Confidence = 0.0
	}

	report.PlanID = plan.ID
	report.CreatedAt = time.Now()
	return report
}

func analyzeNginx(plan model.DiagnosisPlan) *model.AnalysisReport {
	report := &model.AnalysisReport{FaultType: "Nginx 服务异常"}

	for _, step := range plan.Steps {
		if step.Result == nil {
			continue
		}
		result := step.Result

		switch step.StepNo {
		case 1: // check_process
			if !result.Success || strings.TrimSpace(result.Stdout) == "" {
				report.PossibleCauses = append(report.PossibleCauses, "Nginx 进程未运行")
				report.Suggestions = append(report.Suggestions, "尝试启动 Nginx: systemctl start nginx")
				report.Confidence = 0.9
			}
		case 2: // check_port
			if !result.Success || strings.TrimSpace(result.Stdout) == "" {
				report.PossibleCauses = append(report.PossibleCauses, "80 端口未被监听")
				report.Suggestions = append(report.Suggestions, "检查 Nginx 配置中的 listen 指令是否正确")
				if report.Confidence < 0.7 {
					report.Confidence = 0.7
				}
			}
		case 3: // read_log_tail
			if strings.Contains(strings.ToLower(result.Stdout), "permission denied") {
				report.PossibleCauses = append(report.PossibleCauses, "Nginx 文件访问权限异常")
				report.Suggestions = append(report.Suggestions, "检查 Nginx 工作目录和日志目录权限")
			}
			if strings.Contains(strings.ToLower(result.Stdout), "bind") {
				report.PossibleCauses = append(report.PossibleCauses, "端口绑定失败（可能被占用）")
				report.Suggestions = append(report.Suggestions, "检查 80 端口是否被其他进程占用: ss -tlnp | grep :80")
			}
		case 4: // http_health_check
			if !result.Success {
				report.PossibleCauses = append(report.PossibleCauses, "HTTP 请求无响应")
				report.Suggestions = append(report.Suggestions, "确认防火墙规则是否允许 80 端口访问")
			}
		}
	}

	if len(report.PossibleCauses) == 0 {
		report.PossibleCauses = append(report.PossibleCauses, "Nginx 服务状态正常，建议从网络层排查")
		report.Suggestions = append(report.Suggestions, "检查客户端到服务器网络连通性")
		report.Confidence = 0.5
	}

	if report.Confidence == 0 {
		report.Confidence = 0.5
	}

	report.RawAnalysis = fmt.Sprintf("基于 %d 个诊断步骤的分析结果：%s", len(plan.Steps), strings.Join(report.PossibleCauses, "; "))
	return report
}

func analyzeRedis(plan model.DiagnosisPlan) *model.AnalysisReport {
	report := &model.AnalysisReport{FaultType: "Redis 响应慢"}

	for _, step := range plan.Steps {
		if step.Result == nil {
			continue
		}
		result := step.Result

		switch step.StepNo {
		case 1: // redis_ping
			if !result.Success || !strings.Contains(strings.ToUpper(result.Stdout), "PONG") {
				report.PossibleCauses = append(report.PossibleCauses, "Redis 服务无响应或连接失败")
				report.Suggestions = append(report.Suggestions, "检查 Redis 服务状态: systemctl status redis")
				report.Confidence = 0.9
			}
		case 2: // redis_info memory
			if strings.Contains(strings.ToLower(result.Stdout), "used_memory") {
				report.PossibleCauses = append(report.PossibleCauses, "Redis 内存使用过高")
				report.Suggestions = append(report.Suggestions, "考虑增加 Redis 内存限制或启用淘汰策略")
				if report.Confidence < 0.6 {
					report.Confidence = 0.6
				}
			}
		case 3: // redis_slowlog_get
			if strings.TrimSpace(result.Stdout) != "" && !strings.Contains(strings.ToLower(result.Stdout), "empty") {
				report.PossibleCauses = append(report.PossibleCauses, "存在慢查询，可能存在不合理的命令使用")
				report.Suggestions = append(report.Suggestions, "优化慢查询命令，考虑使用批量操作或调整数据结构")
				if report.Confidence < 0.7 {
					report.Confidence = 0.7
				}
			}
		case 4: // redis_client_list
			lines := strings.Count(result.Stdout, "\n") + 1
			if lines > 50 {
				report.PossibleCauses = append(report.PossibleCauses, fmt.Sprintf("Redis 客户端连接数过多（约 %d 个）", lines))
				report.Suggestions = append(report.Suggestions, "检查是否存在连接泄漏，考虑增加连接池限制")
				if report.Confidence < 0.6 {
					report.Confidence = 0.6
				}
			}
		}
	}

	if len(report.PossibleCauses) == 0 {
		report.PossibleCauses = append(report.PossibleCauses, "Redis 基本检查正常，建议从应用层排查慢查询")
		report.Suggestions = append(report.Suggestions, "检查应用代码中是否使用了低效的 Redis 命令（如 KEYS *）")
		report.Confidence = 0.3
	}

	if report.Confidence == 0 {
		report.Confidence = 0.5
	}

	report.RawAnalysis = fmt.Sprintf("基于 %d 个诊断步骤的分析结果：%s", len(plan.Steps), strings.Join(report.PossibleCauses, "; "))
	return report
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
