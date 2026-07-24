package builtin

import (
	"fmt"
	"os/exec"
	"strings"

	"adp/internal/domain/model"
	"adp/internal/module"
)

// ── check_process ──

type CheckProcess struct{}

func (m *CheckProcess) Code() string               { return "check_process" }
func (m *CheckProcess) Name() string               { return "检查进程" }
func (m *CheckProcess) Description() string        { return "检查指定进程是否运行" }
func (m *CheckProcess) ToolType() string           { return "shell" }
func (m *CheckProcess) RiskLevel() model.RiskLevel { return model.RiskLevelLow }
func (m *CheckProcess) RiskProfile() module.RiskProfile {
	return module.RiskProfile{Level: model.RiskLevelLow, Reversible: true, ImpactScope: "single_host"}
}
func (m *CheckProcess) Parameters() []module.ParamDef {
	return []module.ParamDef{{Name: "ServiceProfile", Description: "Worker services.cnf 中的 Nginx Profile", Required: true}}
}
func (m *CheckProcess) Check(ctx module.ExecContext) (module.CheckResult, error) {
	proc := paramFirst(ctx.Params, "", "Process", "ProcessName")
	out, err := exec.Command("pgrep", proc).CombinedOutput()
	if err == nil && strings.TrimSpace(string(out)) != "" {
		return module.CheckResult{NeedsChange: false, CurrentState: fmt.Sprintf("process %s is running", proc)}, nil
	}
	return module.CheckResult{NeedsChange: true, CurrentState: fmt.Sprintf("process %s is not running", proc)}, nil
}
func (m *CheckProcess) Execute(ctx module.ExecContext) (module.Result, error) {
	cr, _ := m.Check(ctx)
	if !cr.NeedsChange {
		return module.Result{Success: true, Output: cr.CurrentState, Changed: false}, nil
	}
	return module.Result{Success: true, Output: cr.CurrentState, Changed: true, Facts: map[string]string{"process_status": "not_running"}}, nil
}
func (m *CheckProcess) DryRun(ctx module.ExecContext) (module.Result, error) {
	return m.Execute(ctx)
}

// ── check_port ──

type CheckPort struct{}

func (m *CheckPort) Code() string               { return "check_port" }
func (m *CheckPort) Name() string               { return "检查端口" }
func (m *CheckPort) Description() string        { return "检查指定端口是否监听" }
func (m *CheckPort) ToolType() string           { return "shell" }
func (m *CheckPort) RiskLevel() model.RiskLevel { return model.RiskLevelLow }
func (m *CheckPort) RiskProfile() module.RiskProfile {
	return module.RiskProfile{Level: model.RiskLevelLow, Reversible: true, ImpactScope: "single_host"}
}
func (m *CheckPort) Parameters() []module.ParamDef {
	return []module.ParamDef{{Name: "ServiceProfile", Description: "Worker services.cnf 中的 Nginx Profile", Required: true}}
}
func (m *CheckPort) Check(ctx module.ExecContext) (module.CheckResult, error) {
	port := ctx.Params["Port"]
	out, err := exec.Command("ss", "-tlnp").CombinedOutput()
	if err == nil && strings.Contains(string(out), ":"+port) {
		return module.CheckResult{NeedsChange: false, CurrentState: fmt.Sprintf("port %s is listening", port)}, nil
	}
	return module.CheckResult{NeedsChange: true, CurrentState: fmt.Sprintf("port %s is not listening", port)}, nil
}
func (m *CheckPort) Execute(ctx module.ExecContext) (module.Result, error) {
	cr, _ := m.Check(ctx)
	if !cr.NeedsChange {
		return module.Result{Success: true, Output: cr.CurrentState, Changed: false}, nil
	}
	return module.Result{Success: true, Output: cr.CurrentState, Changed: true}, nil
}
func (m *CheckPort) DryRun(ctx module.ExecContext) (module.Result, error) {
	return m.Execute(ctx)
}

// ── http_health_check ──

type HTTPHealthCheck struct{}

func (m *HTTPHealthCheck) Code() string { return "http_health_check" }
func (m *HTTPHealthCheck) Name() string { return "HTTP 健康检查" }
func (m *HTTPHealthCheck) Description() string {
	return "对指定 URL 发起 HTTP 请求检查是否可达"
}
func (m *HTTPHealthCheck) ToolType() string           { return "shell" }
func (m *HTTPHealthCheck) RiskLevel() model.RiskLevel { return model.RiskLevelLow }
func (m *HTTPHealthCheck) RiskProfile() module.RiskProfile {
	return module.RiskProfile{Level: model.RiskLevelLow, Reversible: true, ImpactScope: "single_host"}
}
func (m *HTTPHealthCheck) Parameters() []module.ParamDef {
	return []module.ParamDef{{Name: "ServiceProfile", Description: "Worker services.cnf 中的 HTTP Profile", Required: true}}
}
func (m *HTTPHealthCheck) Check(_ module.ExecContext) (module.CheckResult, error) {
	return module.CheckResult{NeedsChange: true, CurrentState: "will check HTTP endpoint"}, nil
}
func (m *HTTPHealthCheck) Execute(ctx module.ExecContext) (module.Result, error) {
	url := ctx.Params["URL"]
	out, err := exec.Command("curl", "-sI", "--max-time", "10", url).CombinedOutput()
	if err != nil {
		return module.Result{Success: false, Output: string(out)}, nil
	}
	statusLine := strings.SplitN(string(out), "\n", 2)[0]
	return module.Result{Success: true, Output: statusLine, Changed: false, Facts: map[string]string{"http_status": statusLine}}, nil
}
func (m *HTTPHealthCheck) DryRun(ctx module.ExecContext) (module.Result, error) {
	return module.Result{Success: true, Output: fmt.Sprintf("would curl %s", ctx.Params["URL"]), Changed: false}, nil
}

// ── read_log_tail ──

type ReadLogTail struct{}

func (m *ReadLogTail) Code() string               { return "read_log_tail" }
func (m *ReadLogTail) Name() string               { return "读取日志尾部" }
func (m *ReadLogTail) Description() string        { return "读取指定日志文件的末尾行" }
func (m *ReadLogTail) ToolType() string           { return "shell" }
func (m *ReadLogTail) RiskLevel() model.RiskLevel { return model.RiskLevelLow }
func (m *ReadLogTail) RiskProfile() module.RiskProfile {
	return module.RiskProfile{Level: model.RiskLevelLow, Reversible: true, ImpactScope: "single_host"}
}
func (m *ReadLogTail) Parameters() []module.ParamDef {
	return []module.ParamDef{
		{Name: "ServiceProfile", Description: "Worker services.cnf 中的 Nginx Profile", Required: true},
		{Name: "Lines", Description: "读取行数", Required: false, Default: "50"},
	}
}
func (m *ReadLogTail) Check(_ module.ExecContext) (module.CheckResult, error) {
	return module.CheckResult{NeedsChange: true, CurrentState: "will read log file"}, nil
}
func (m *ReadLogTail) Execute(ctx module.ExecContext) (module.Result, error) {
	logFile := ctx.Params["LogFile"]
	lines := paramDefault(ctx.Params, "Lines", "50")
	out, err := exec.Command("tail", "-n", lines, logFile).CombinedOutput()
	if err != nil {
		return module.Result{Success: false, Output: string(out)}, nil
	}
	return module.Result{Success: true, Output: string(out), Changed: false, Facts: map[string]string{"log_lines": lines}}, nil
}
func (m *ReadLogTail) DryRun(ctx module.ExecContext) (module.Result, error) {
	return module.Result{Success: true, Output: fmt.Sprintf("would read %s lines from %s", paramDefault(ctx.Params, "Lines", "50"), ctx.Params["LogFile"]), Changed: false}, nil
}
