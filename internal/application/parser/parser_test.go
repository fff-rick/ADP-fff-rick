package parser

import (
	"context"
	"testing"

	"adp/internal/domain/model"
	"adp/internal/domain/policy"
	"adp/internal/domain/template"
)

func TestParseWithRules_MySQLBackup(t *testing.T) {
	p := NewParser(nil, template.NewEngine(), policy.NewEngine())

	tests := []struct {
		input          string
		wantIntent     string
		wantTargetType string
		wantRiskLevel  model.RiskLevel
	}{
		{"每天凌晨备份 MySQL 数据库", "create_scheduled_backup", "mysql", model.RiskLevelMedium},
		{"备份 MySQL 数据库 mydb", "create_scheduled_backup", "mysql", model.RiskLevelMedium},
		{"每天备份数据库", "create_scheduled_backup", "mysql", model.RiskLevelMedium},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			intent, err := p.Parse(context.Background(), tt.input)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if intent.Intent != tt.wantIntent {
				t.Errorf("intent = %s, want %s", intent.Intent, tt.wantIntent)
			}
			if intent.TargetType != tt.wantTargetType {
				t.Errorf("target_type = %s, want %s", intent.TargetType, tt.wantTargetType)
			}
			if intent.RiskLevel != tt.wantRiskLevel {
				t.Errorf("risk_level = %s, want %s", intent.RiskLevel, tt.wantRiskLevel)
			}
			if intent.MatchedTemplate != "mysql_backup" {
				t.Errorf("matched_template = %s, want mysql_backup", intent.MatchedTemplate)
			}
		})
	}
}

func TestParseWithRules_HTTPHealthCheck(t *testing.T) {
	p := NewParser(nil, template.NewEngine(), policy.NewEngine())

	tests := []string{
		"检查 HTTP 服务健康状态",
		"对网站做健康检查",
		"health check the service",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			intent, err := p.Parse(context.Background(), input)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if intent.Intent != "health_check" {
				t.Errorf("intent = %s, want health_check", intent.Intent)
			}
			if intent.TargetType != "http_service" {
				t.Errorf("target_type = %s, want http_service", intent.TargetType)
			}
			if intent.RiskLevel != model.RiskLevelLow {
				t.Errorf("risk_level = %s, want low", intent.RiskLevel)
			}
		})
	}
}

func TestParseWithRules_UnrecognizedInput(t *testing.T) {
	p := NewParser(nil, template.NewEngine(), policy.NewEngine())

	_, err := p.Parse(context.Background(), "random gibberish")
	if err == nil {
		t.Fatal("expected error for unrecognized input")
	}
}

func TestParseWithRules_EmptyInput(t *testing.T) {
	p := NewParser(nil, template.NewEngine(), policy.NewEngine())

	_, err := p.Parse(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestScheduleExtraction(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"每天凌晨备份", "0 0 * * *"},
		{"每日备份", "0 0 * * *"},
		{"每小时检查", "0 * * * *"},
		{"每周备份", "0 0 * * 0"},
		{"立即备份", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := extractCron(tt.input)
			if got != tt.want {
				t.Errorf("extractCron(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
