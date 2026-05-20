package analyzer

import (
	"context"
	"testing"

	"adp/internal/model"
)

func TestAnalyze_NginxUnreachable_ProcessDown(t *testing.T) {
	a := New(nil)

	plan := model.DiagnosisPlan{
		ID:          "plan-000001",
		Title:       "Nginx 不可访问诊断",
		TriggerType: "nginx_unreachable",
		Steps: []model.DiagnosisStep{
			{
				StepNo: 1, Name: "检查 Nginx 进程存活状态", TemplateCode: "check_process",
				Result: &model.StepResult{Stdout: "", Stderr: "", ExitCode: 1, Success: false},
			},
			{
				StepNo: 2, Name: "检查 80 端口监听状态", TemplateCode: "check_port",
				Result: &model.StepResult{Stdout: "", Stderr: "", ExitCode: 1, Success: false},
			},
			{
				StepNo: 3, Name: "读取 Nginx 错误日志", TemplateCode: "read_log_tail",
				Result: &model.StepResult{Stdout: "permission denied", Stderr: "", ExitCode: 0, Success: true},
			},
			{
				StepNo: 4, Name: "HTTP 健康检查", TemplateCode: "http_health_check",
				Result: &model.StepResult{Stdout: "", Stderr: "Connection refused", ExitCode: 7, Success: false},
			},
		},
	}

	report, err := a.Analyze(context.Background(), plan)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if report == nil {
		t.Fatal("expected non-nil report")
	}
	if report.FaultType == "" {
		t.Error("expected non-empty fault_type")
	}
	if len(report.PossibleCauses) == 0 {
		t.Error("expected at least one possible cause")
	}
	if len(report.Suggestions) == 0 {
		t.Error("expected at least one suggestion")
	}

	t.Logf("Fault type: %s", report.FaultType)
	t.Logf("Causes: %v", report.PossibleCauses)
	t.Logf("Suggestions: %v", report.Suggestions)
	t.Logf("Confidence: %.2f", report.Confidence)
}

func TestAnalyze_RedisSlow(t *testing.T) {
	a := New(nil)

	plan := model.DiagnosisPlan{
		ID:          "plan-000002",
		Title:       "Redis 响应慢诊断",
		TriggerType: "redis_slow",
		Steps: []model.DiagnosisStep{
			{
				StepNo: 1, Name: "Redis 存活检查", TemplateCode: "redis_ping",
				Result: &model.StepResult{Stdout: "PONG", Stderr: "", ExitCode: 0, Success: true},
			},
			{
				StepNo: 2, Name: "Redis 内存使用分析", TemplateCode: "redis_info",
				Result: &model.StepResult{Stdout: "used_memory_human:1.5G\nused_memory_peak_human:2.0G", Stderr: "", ExitCode: 0, Success: true},
			},
			{
				StepNo: 3, Name: "Redis 慢日志查询", TemplateCode: "redis_slowlog_get",
				Result: &model.StepResult{Stdout: "1) 1) (integer) 1234\n   2) (integer) 1234567\n   3) (integer) 500000\n   4) 1) \"KEYS\"\n      2) \"*\"", Stderr: "", ExitCode: 0, Success: true},
			},
			{
				StepNo: 4, Name: "Redis 当前连接数检查", TemplateCode: "redis_client_list",
				Result: &model.StepResult{Stdout: stringsRepeat("id=xxx addr=xxx\n", 60), Stderr: "", ExitCode: 0, Success: true},
			},
		},
	}

	report, err := a.Analyze(context.Background(), plan)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	t.Logf("Fault type: %s", report.FaultType)
	t.Logf("Causes: %v", report.PossibleCauses)
	t.Logf("Suggestions: %v", report.Suggestions)
	t.Logf("Confidence: %.2f", report.Confidence)
}

func stringsRepeat(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}
