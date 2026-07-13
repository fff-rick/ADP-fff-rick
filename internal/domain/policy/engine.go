package policy

import (
	"fmt"
	"strings"

	"adp/internal/domain/model"
)

// Engine enforces execution policies: tool whitelist, template whitelist,
// and risk-level based gating.
type Engine struct {
	allowedTools     map[string]bool
	allowedTemplates map[string]bool
}

// NewEngine creates a policy engine with a default whitelist.
func NewEngine() *Engine {
	return &Engine{
		allowedTools: map[string]bool{
			"mysqldump": true,
			"curl":      true,
			"ping":      true,
			"redis-cli": true,
			"mysql":     true,
			"echo":      true,
			"cat":       true,
			"grep":      true,
			"df":        true,
			"free":      true,
			"uptime":    true,
			"netstat":   true,
			"ss":        true,
			"head":      true,
			"tail":      true,
			"wc":        true,
			"sort":      true,
			"uniq":      true,
			"ps":        true,
			"awk":       true,
		},
		allowedTemplates: map[string]bool{
			"mysql_backup":      true,
			"http_health_check": true,
			"check_process":     true,
			"check_port":        true,
			"read_log_tail":     true,
			"redis_ping":        true,
			"redis_info":        true,
			"redis_slowlog_get": true,
			"redis_client_list": true,
		},
	}
}

// ValidateTemplate checks whether a template code is allowed.
func (e *Engine) ValidateTemplate(code string) error {
	if !e.allowedTemplates[code] {
		return fmt.Errorf("template not in whitelist: %s", code)
	}
	return nil
}

// ValidateCommand checks whether the leading tool in a command is allowed.
func (e *Engine) ValidateCommand(cmd string) error {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return fmt.Errorf("command is empty")
	}

	tool := strings.Split(cmd, " ")[0]
	if !e.allowedTools[tool] {
		return fmt.Errorf("tool not in whitelist: %s", tool)
	}
	return nil
}

// AssessRisk returns a risk level for the given intent.
// High-risk intents (data deletion, service restart, config changes)
// require human approval before execution.
func (e *Engine) AssessRisk(intent model.TaskIntent) model.RiskLevel {
	if intent.RiskLevel == model.RiskLevelHigh {
		return model.RiskLevelHigh
	}

	highRiskKeywords := []string{"delete", "drop", "restart", "reboot", "shutdown", "kill", "rm ", "mkfs", "dd "}
	combined := intent.Intent + " " + intent.TargetType
	for _, kw := range highRiskKeywords {
		if strings.Contains(strings.ToLower(combined), kw) {
			return model.RiskLevelHigh
		}
	}

	return intent.RiskLevel
}

// IsHighRisk is a convenience check.
func (e *Engine) IsHighRisk(level model.RiskLevel) bool {
	return level == model.RiskLevelHigh
}

func (e *Engine) MergeRisk(levels ...model.RiskLevel) model.RiskLevel {
	result := model.RiskLevelLow
	for _, level := range levels {
		switch level {
		case model.RiskLevelHigh:
			return model.RiskLevelHigh
		case model.RiskLevelMedium:
			if result != model.RiskLevelHigh {
				result = model.RiskLevelMedium
			}
		}
	}
	return result
}

func (e *Engine) RequiresManualApproval(level model.RiskLevel) bool {
	return level == model.RiskLevelMedium || level == model.RiskLevelHigh
}
