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
		// --- Diagnosis templates ---
		{
			Code:        "check_process",
			Name:        "进程存活检查",
			Description: "检查指定进程是否在运行",
			ToolType:    "shell",
			Command:     "ps aux | grep -v grep | grep {{.ProcessName}}",
			Parameters: []model.TemplateParam{
				{Name: "ProcessName", Description: "进程名称", Required: true, Default: "nginx"},
			},
			RiskLevel: model.RiskLevelLow,
		},
		{
			Code:        "check_port",
			Name:        "端口监听检查",
			Description: "检查指定端口是否处于监听状态",
			ToolType:    "shell",
			Command:     "ss -tlnp 2>/dev/null | grep :{{.Port}} || netstat -tlnp 2>/dev/null | grep :{{.Port}}",
			Parameters: []model.TemplateParam{
				{Name: "Port", Description: "端口号", Required: true, Default: "80"},
			},
			RiskLevel: model.RiskLevelLow,
		},
		{
			Code:        "read_log_tail",
			Name:        "日志尾部读取",
			Description: "读取日志文件末尾若干行",
			ToolType:    "shell",
			Command:     "tail -{{.Lines}} {{.LogFile}} 2>/dev/null || echo 'log file not found'",
			Parameters: []model.TemplateParam{
				{Name: "LogFile", Description: "日志文件路径", Required: true, Default: "/var/log/nginx/error.log"},
				{Name: "Lines", Description: "读取行数", Required: true, Default: "50"},
			},
			RiskLevel: model.RiskLevelLow,
		},
		{
			Code:        "redis_ping",
			Name:        "Redis PING 检查",
			Description: "检查 Redis 服务是否存活",
			ToolType:    "shell",
			Command:     "redis-cli -h {{.Host}} -p {{.Port}} PING 2>&1",
			Parameters: []model.TemplateParam{
				{Name: "Host", Description: "Redis 主机地址", Required: true, Default: "127.0.0.1"},
				{Name: "Port", Description: "Redis 端口", Required: true, Default: "6379"},
			},
			RiskLevel: model.RiskLevelLow,
		},
		{
			Code:        "redis_info",
			Name:        "Redis INFO 查询",
			Description: "获取 Redis 指定段信息",
			ToolType:    "shell",
			Command:     "redis-cli -h {{.Host}} -p {{.Port}} INFO {{.Section}} 2>&1",
			Parameters: []model.TemplateParam{
				{Name: "Host", Description: "Redis 主机地址", Required: true, Default: "127.0.0.1"},
				{Name: "Port", Description: "Redis 端口", Required: true, Default: "6379"},
				{Name: "Section", Description: "INFO 段名", Required: true, Default: "memory"},
			},
			RiskLevel: model.RiskLevelLow,
		},
		{
			Code:        "redis_slowlog_get",
			Name:        "Redis 慢日志查询",
			Description: "获取 Redis 慢日志",
			ToolType:    "shell",
			Command:     "redis-cli -h {{.Host}} -p {{.Port}} SLOWLOG GET {{.Count}} 2>&1",
			Parameters: []model.TemplateParam{
				{Name: "Host", Description: "Redis 主机地址", Required: true, Default: "127.0.0.1"},
				{Name: "Port", Description: "Redis 端口", Required: true, Default: "6379"},
				{Name: "Count", Description: "获取条数", Required: true, Default: "10"},
			},
			RiskLevel: model.RiskLevelLow,
		},
		{
			Code:        "redis_client_list",
			Name:        "Redis 客户端列表",
			Description: "获取 Redis 当前连接客户端列表",
			ToolType:    "shell",
			Command:     "redis-cli -h {{.Host}} -p {{.Port}} CLIENT LIST 2>&1",
			Parameters: []model.TemplateParam{
				{Name: "Host", Description: "Redis 主机地址", Required: true, Default: "127.0.0.1"},
				{Name: "Port", Description: "Redis 端口", Required: true, Default: "6379"},
			},
			RiskLevel: model.RiskLevelLow,
		},
	}
}
