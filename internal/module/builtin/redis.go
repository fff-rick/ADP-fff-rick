package builtin

import (
	"fmt"
	"os/exec"
	"strings"

	"adp/internal/domain/model"
	"adp/internal/module"
)

// ── redis_ping ──

type RedisPing struct{}

func (m *RedisPing) Code() string               { return "redis_ping" }
func (m *RedisPing) Name() string               { return "Redis PING" }
func (m *RedisPing) Description() string        { return "向 Redis 发送 PING 命令检查连通性" }
func (m *RedisPing) ToolType() string           { return "shell" }
func (m *RedisPing) RiskLevel() model.RiskLevel { return model.RiskLevelLow }
func (m *RedisPing) RiskProfile() module.RiskProfile {
	return module.RiskProfile{Level: model.RiskLevelLow, Reversible: true, ImpactScope: "single_host"}
}
func (m *RedisPing) Parameters() []module.ParamDef {
	return []module.ParamDef{{Name: "Host", Description: "Redis 主机", Required: false, Default: "127.0.0.1"}}
}
func (m *RedisPing) Check(_ module.ExecContext) (module.CheckResult, error) {
	return module.CheckResult{NeedsChange: true, CurrentState: "will check Redis connectivity"}, nil
}
func (m *RedisPing) Execute(ctx module.ExecContext) (module.Result, error) {
	host := paramDefault(ctx.Params, "Host", "127.0.0.1")
	port := paramDefault(ctx.Params, "Port", "6379")
	out, err := exec.Command("redis-cli", "-h", host, "-p", port, "PING").CombinedOutput()
	if err != nil {
		return module.Result{Success: false, Output: string(out)}, nil
	}
	result := strings.TrimSpace(string(out))
	success := result == "PONG"
	return module.Result{Success: success, Output: result, Changed: false}, nil
}
func (m *RedisPing) DryRun(ctx module.ExecContext) (module.Result, error) {
	return module.Result{Success: true, Output: fmt.Sprintf("would redis-cli -h %s PING", paramDefault(ctx.Params, "Host", "127.0.0.1")), Changed: false}, nil
}

// ── redis_info ──

type RedisInfo struct{}

func (m *RedisInfo) Code() string               { return "redis_info" }
func (m *RedisInfo) Name() string               { return "Redis INFO" }
func (m *RedisInfo) Description() string        { return "获取 Redis INFO 信息（内存、连接等）" }
func (m *RedisInfo) ToolType() string           { return "shell" }
func (m *RedisInfo) RiskLevel() model.RiskLevel { return model.RiskLevelLow }
func (m *RedisInfo) RiskProfile() module.RiskProfile {
	return module.RiskProfile{Level: model.RiskLevelLow, Reversible: true, ImpactScope: "single_host"}
}
func (m *RedisInfo) Parameters() []module.ParamDef {
	return []module.ParamDef{{Name: "Host", Description: "Redis 主机", Required: false, Default: "127.0.0.1"}}
}
func (m *RedisInfo) Check(_ module.ExecContext) (module.CheckResult, error) {
	return module.CheckResult{NeedsChange: true, CurrentState: "will collect Redis INFO"}, nil
}
func (m *RedisInfo) Execute(ctx module.ExecContext) (module.Result, error) {
	host := paramDefault(ctx.Params, "Host", "127.0.0.1")
	port := paramDefault(ctx.Params, "Port", "6379")
	section := paramDefault(ctx.Params, "Section", "memory")
	out, err := exec.Command("redis-cli", "-h", host, "-p", port, "INFO", section).CombinedOutput()
	if err != nil {
		return module.Result{Success: false, Output: string(out)}, nil
	}
	return module.Result{Success: true, Output: string(out), Changed: false, Facts: map[string]string{"redis_info": "collected"}}, nil
}
func (m *RedisInfo) DryRun(ctx module.ExecContext) (module.Result, error) {
	return module.Result{Success: true, Output: fmt.Sprintf("would redis-cli -h %s INFO memory", paramDefault(ctx.Params, "Host", "127.0.0.1")), Changed: false}, nil
}

// ── redis_slowlog_get ──

type RedisSlowlogGet struct{}

func (m *RedisSlowlogGet) Code() string               { return "redis_slowlog_get" }
func (m *RedisSlowlogGet) Name() string               { return "Redis 慢查询" }
func (m *RedisSlowlogGet) Description() string        { return "获取 Redis 慢查询日志" }
func (m *RedisSlowlogGet) ToolType() string           { return "shell" }
func (m *RedisSlowlogGet) RiskLevel() model.RiskLevel { return model.RiskLevelLow }
func (m *RedisSlowlogGet) RiskProfile() module.RiskProfile {
	return module.RiskProfile{Level: model.RiskLevelLow, Reversible: true, ImpactScope: "single_host"}
}
func (m *RedisSlowlogGet) Parameters() []module.ParamDef {
	return []module.ParamDef{
		{Name: "Host", Description: "Redis 主机", Required: false, Default: "127.0.0.1"},
		{Name: "Count", Description: "获取条数", Required: false, Default: "10"},
	}
}
func (m *RedisSlowlogGet) Check(_ module.ExecContext) (module.CheckResult, error) {
	return module.CheckResult{NeedsChange: true, CurrentState: "will fetch Redis slowlog"}, nil
}
func (m *RedisSlowlogGet) Execute(ctx module.ExecContext) (module.Result, error) {
	host := paramDefault(ctx.Params, "Host", "127.0.0.1")
	port := paramDefault(ctx.Params, "Port", "6379")
	count := paramDefault(ctx.Params, "Count", "10")
	out, err := exec.Command("redis-cli", "-h", host, "-p", port, "SLOWLOG", "GET", count).CombinedOutput()
	if err != nil {
		return module.Result{Success: false, Output: string(out)}, nil
	}
	result := strings.TrimSpace(string(out))
	hasEntries := result != "" && result != "(empty array)" && result != "(empty list or set)"
	return module.Result{Success: true, Output: result, Changed: false, Facts: map[string]string{"has_slowlog": fmt.Sprintf("%t", hasEntries)}}, nil
}
func (m *RedisSlowlogGet) DryRun(ctx module.ExecContext) (module.Result, error) {
	return module.Result{Success: true, Output: fmt.Sprintf("would redis-cli SLOWLOG GET %s", paramDefault(ctx.Params, "Count", "10")), Changed: false}, nil
}

// ── redis_client_list ──

type RedisClientList struct{}

func (m *RedisClientList) Code() string               { return "redis_client_list" }
func (m *RedisClientList) Name() string               { return "Redis 客户端列表" }
func (m *RedisClientList) Description() string        { return "获取 Redis 客户端连接列表" }
func (m *RedisClientList) ToolType() string           { return "shell" }
func (m *RedisClientList) RiskLevel() model.RiskLevel { return model.RiskLevelLow }
func (m *RedisClientList) RiskProfile() module.RiskProfile {
	return module.RiskProfile{Level: model.RiskLevelLow, Reversible: true, ImpactScope: "single_host"}
}
func (m *RedisClientList) Parameters() []module.ParamDef {
	return []module.ParamDef{{Name: "Host", Description: "Redis 主机", Required: false, Default: "127.0.0.1"}}
}
func (m *RedisClientList) Check(_ module.ExecContext) (module.CheckResult, error) {
	return module.CheckResult{NeedsChange: true, CurrentState: "will fetch Redis client list"}, nil
}
func (m *RedisClientList) Execute(ctx module.ExecContext) (module.Result, error) {
	host := paramDefault(ctx.Params, "Host", "127.0.0.1")
	port := paramDefault(ctx.Params, "Port", "6379")
	out, err := exec.Command("redis-cli", "-h", host, "-p", port, "CLIENT", "LIST").CombinedOutput()
	if err != nil {
		return module.Result{Success: false, Output: string(out)}, nil
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	return module.Result{Success: true, Output: string(out), Changed: false, Facts: map[string]string{"client_count": fmt.Sprintf("%d", len(lines))}}, nil
}
func (m *RedisClientList) DryRun(ctx module.ExecContext) (module.Result, error) {
	return module.Result{Success: true, Output: fmt.Sprintf("would redis-cli -h %s CLIENT LIST", paramDefault(ctx.Params, "Host", "127.0.0.1")), Changed: false}, nil
}
