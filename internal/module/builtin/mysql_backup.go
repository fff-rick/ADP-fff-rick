package builtin

import (
	"fmt"
	"os/exec"
	"time"

	"adp/internal/domain/model"
	"adp/internal/module"
)

type MySQLBackup struct{}

func (m *MySQLBackup) Code() string               { return "mysql_backup" }
func (m *MySQLBackup) Name() string               { return "MySQL 备份" }
func (m *MySQLBackup) Description() string        { return "使用 mysqldump 备份 MySQL 数据库" }
func (m *MySQLBackup) ToolType() string           { return "shell" }
func (m *MySQLBackup) RiskLevel() model.RiskLevel { return model.RiskLevelMedium }
func (m *MySQLBackup) RiskProfile() module.RiskProfile {
	return module.RiskProfile{Level: model.RiskLevelMedium, Reversible: true, ImpactScope: "single_host"}
}
func (m *MySQLBackup) Parameters() []module.ParamDef {
	return []module.ParamDef{
		{Name: "Database", Description: "数据库名", Required: true},
		{Name: "Password", Description: "数据库密码", Required: true},
		{Name: "User", Description: "数据库用户", Required: false, Default: "root"},
		{Name: "Host", Description: "数据库主机", Required: false, Default: "127.0.0.1"},
	}
}
func (m *MySQLBackup) Check(ctx module.ExecContext) (module.CheckResult, error) {
	// Backup is always needed.
	return module.CheckResult{NeedsChange: true, CurrentState: "no recent backup found"}, nil
}
func (m *MySQLBackup) Execute(ctx module.ExecContext) (module.Result, error) {
	db := ctx.Params["Database"]
	pw := ctx.Params["Password"]
	user := paramDefault(ctx.Params, "User", "root")
	host := paramDefault(ctx.Params, "Host", "127.0.0.1")
	port := paramDefault(ctx.Params, "Port", "3306")
	filename := paramDefault(ctx.Params, "OutputFile", fmt.Sprintf("/tmp/%s_backup_%s.sql", db, time.Now().Format("20060102_150405")))
	cmd := fmt.Sprintf("mysqldump -h %s -P %s -u %s -p'%s' %s > %s 2>&1", host, port, user, pw, db, filename)
	out, err := exec.Command("sh", "-c", cmd).CombinedOutput()
	if err != nil {
		return module.Result{Success: false, Output: string(out), Facts: map[string]string{"backup_file": filename}}, nil
	}
	return module.Result{Success: true, Output: string(out), Changed: true, Facts: map[string]string{"backup_file": filename}}, nil
}
func (m *MySQLBackup) DryRun(ctx module.ExecContext) (module.Result, error) {
	db := paramDefault(ctx.Params, "Database", "???")
	return module.Result{Success: true, Output: fmt.Sprintf("would backup database %s with mysqldump", db), Changed: true}, nil
}
