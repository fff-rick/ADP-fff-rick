package template

import "adp/internal/model"

func builtinTemplates() []model.CommandTemplate {
	return []model.CommandTemplate{
		{
			Code:        "mysql_backup",
			Name:        "MySQL 数据库备份",
			Description: "使用 mysqldump 备份 MySQL 数据库",
			ToolType:    "shell",
			Command:     "mysqldump -h {{.Host}} -P {{.Port}} -u {{.User}} -p{{.Password}} {{.Database}} > {{.OutputFile}}",
			Parameters: []model.TemplateParam{
				{Name: "Host", Description: "MySQL 主机地址", Required: true, Default: "127.0.0.1"},
				{Name: "Port", Description: "MySQL 端口", Required: true, Default: "3306"},
				{Name: "User", Description: "MySQL 用户名", Required: true, Default: "root"},
				{Name: "Password", Description: "MySQL 密码", Required: true},
				{Name: "Database", Description: "要备份的数据库名", Required: true},
				{Name: "OutputFile", Description: "备份输出文件路径", Required: true, Default: "/tmp/backup.sql"},
			},
			RiskLevel: model.RiskLevelMedium,
		},
		{
			Code:        "http_health_check",
			Name:        "HTTP 健康检查",
			Description: "对指定 URL 执行 HTTP 健康检查，返回状态码",
			ToolType:    "shell",
			Command:     "curl -s -o /dev/null -w '%{http_code}' --max-time {{.Timeout}} {{.URL}}",
			Parameters: []model.TemplateParam{
				{Name: "URL", Description: "要检查的 URL", Required: true, Default: "http://127.0.0.1:80"},
				{Name: "Timeout", Description: "超时时间（秒）", Required: true, Default: "10"},
			},
			RiskLevel: model.RiskLevelLow,
		},
	}
}
