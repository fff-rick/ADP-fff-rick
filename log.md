# Phase 0 操作日志

日期：2026-05-12
工作目录：`/home/xin/work/ADP`

## 目标

开始落实 ADP 项目的 Phase 0，并记录操作过程。

## 操作记录

1. 检查工作目录，确认 `ADP` 目录已存在，但初始状态为空。
2. 锁定第一版项目范围，仅保留 3 个典型场景：
   - MySQL 定时备份
   - Nginx 可用性诊断
   - Redis 性能诊断
3. 明确第一阶段非目标范围，避免首版过度扩张。
4. 文档化模块边界，覆盖 `Web/API`、`Control Plane`、`Scheduler`、`Worker`、`LLM Gateway`、`MySQL`、`Redis`。
5. 确定首版建议技术栈与版本：
   - Go `1.24.x`
   - Gin `1.10.x`
   - gRPC `1.70.x`
   - MySQL `8.0.x`
   - Redis `7.2.x`
   - Docker Compose `v2`
   - Prometheus `2.x`
6. 初始化项目文档基线：
   - `README.md`
   - `docs/architecture.md`
   - `log.md`
7. 规划下一阶段所需的初始源码目录结构：
   - `cmd/server`
   - `cmd/worker`
   - `internal/*`
   - `api/proto`
   - `configs`
   - `deploy/docker-compose`
   - `scripts`
   - `tests/integration`
8. 使用 `.gitkeep` 创建目录骨架，保证仓库结构在当前阶段可见。
9. 添加基础 `.gitignore`，忽略构建产物、本地环境文件和编辑器元数据。
10. 初始化完成后，核对目录结构和文件清单，确认 Phase 0 骨架落地成功。
11. 暂缓创建 `go.mod`，原因是模块路径最好与最终仓库命名或远端地址保持一致。
12. 根据要求，将项目文档统一改写为中文版本，保持原有结论和目录结构不变。

## 当前结果

Phase 0 的中文文档基线已经完成，当前具备：

- 明确的 V1 范围
- 清晰的模块边界
- 稳定的首版技术栈建议
- 可继续开发的仓库目录骨架
- 可追溯的操作日志

## 下一步建议

进入 Phase 1，优先完成：

- 后端工程初始化
- 鉴权与基础数据模型
- Worker 注册与心跳
- 不依赖 AI 的最小调度闭环

---

# Phase 1 操作日志

日期：2026-05-12
工作目录：`/home/xin/work/ADP`

## 目标

将项目从文档和目录骨架推进到“可运行的最小后端原型”，并继续记录实现过程。

## 实现原则

1. 先跑通主链路，再逐步替换为完整技术栈。
2. 当前阶段优先使用 Go 标准库，减少依赖管理复杂度。
3. 保持模块边界清晰，避免在第一阶段过度设计。

## 操作记录

1. 检查项目当前状态，确认 Phase 0 产物完整，尚未开始代码实现。
2. 读取本机 Go 版本，确认当前环境为 `go1.26.2 linux/amd64`。
3. 基于本地环境开始初始化 Go 工程，创建 `go.mod`，模块名暂定为 `adp`。
4. 添加配置示例文件 `configs/app.env.example`，统一开发阶段默认环境变量。
5. 在 `internal/model` 中定义基础领域模型，包括：
   - `User`
   - `Worker`
   - `Job`
   - `WorkerStatus`
   - `JobStatus`
6. 在 `internal/auth` 中实现简化鉴权服务：
   - 登录校验
   - 基于 HMAC-SHA256 的简化 JWT 生成与解析
7. 为鉴权能力补充基础单元测试。
8. 在 `internal/scheduler` 中实现内存版调度存储：
   - Worker 注册
   - Worker 心跳
   - 任务创建
   - 任务分配
   - 任务完成回写
9. 为调度存储补充基础单元测试。
10. 在 `internal/api` 中实现最小 HTTP 服务，提供以下接口：
   - `GET /healthz`
   - `POST /api/v1/auth/login`
   - `POST /api/v1/jobs`
   - `GET /api/v1/jobs`
   - `GET /api/v1/jobs/{id}`
   - `GET /api/v1/workers`
   - `POST /api/v1/workers/register`
   - `POST /api/v1/workers/{id}/heartbeat`
   - `POST /api/v1/workers/{id}/poll`
   - `POST /api/v1/workers/{id}/jobs/{jobId}/complete`
11. 在 `cmd/server` 中实现服务启动入口。
12. 在 `internal/worker` 与 `cmd/worker` 中实现最小 Worker 客户端：
   - 自动注册
   - 定时心跳
   - 周期性拉取任务
   - 任务完成后回传结果
13. 当前 Worker 执行模式为“模拟执行”，用于优先验证调度闭环，而非真实 Shell/SSH 执行。
14. 新增 Phase 1 文档说明文件 `docs/phase1.md`。
15. 对新增 Go 文件执行 `gofmt` 格式化。
16. 运行 `go test ./...`，测试通过，说明当前代码结构可编译且单元测试正常。
17. 启动本地服务端进行烟测。
18. 调用 `GET /healthz`，返回 `{"status":"ok"}`，确认服务存活。
19. 调用登录接口，成功获取开发阶段 Token。
20. 启动本地 Worker 进程，确认 Worker 成功注册并保持在线。
21. 通过 API 创建演示任务 `phase1-demo-job`。
22. 查询 Worker 列表，确认本地 Worker 注册状态正常。
23. 查询演示任务状态，确认任务被 Worker 拉取并成功完成，状态为 `success`，并回传模拟执行结果。
24. 烟测完成后，停止本地测试进程。
25. 按要求将 Phase 1 结果同步回中文 README 和操作日志。

## 当前结果

Phase 1 已完成最小可运行原型，当前具备：

- 可启动的后端服务
- 可用的简化登录鉴权
- Worker 注册与心跳机制
- 用户创建任务能力
- Worker 拉取并完成任务的最小调度闭环
- 基础单元测试与本地烟测验证

## 当前限制

- 存储仍为内存实现，服务重启后数据会丢失
- Worker 当前为模拟执行，不涉及真实命令下发
- 用户与权限体系仍为开发期简化实现
- 尚未接入 MySQL、Redis、Gin、gRPC

## 下一步建议

继续进入下一阶段时，建议优先补齐：

- MySQL 持久化
- Redis 队列
- 更完整的任务状态机
- 更细的权限控制
- AI 解析与任务模板能力

---

# Phase 2 操作日志

日期：2026-05-13
工作目录：`/home/xin/work/ADP`

## 目标

在 Phase 1 的最小调度闭环之上，增加 AI 解析与受控执行能力，实现“自然语言 -> 结构化任务 -> 调度执行”的完整链路。

## 实现原则

1. 继续使用 Go 标准库为主，仅 LLM 调用使用 `net/http`。
2. AI 负责解析与规划，调度系统负责执行控制，两者职责解耦。
3. 所有命令必须通过模板渲染 + 白名单校验，禁止自由命令直接执行。
4. 保留规则匹配作为 LLM 的降级方案，确保无 LLM 环境也能跑通核心链路。

## Phase 2 完成清单

对照 ADP_To_Do_List.md Phase 2 逐项检查：

| 需求项 | 状态 | 实现位置 |
|--------|------|----------|
| 封装统一 LLM 调用接口 | ✅ | `internal/llm/client.go` — 接口 `Client` + OpenAI 兼容 HTTP 实现 |
| 实现自然语言任务解析模块 | ✅ | `internal/parser/parser.go` — LLM 优先，规则匹配降级 |
| 定义结构化任务 JSON 格式 | ✅ | `internal/model/model.go` — `TaskIntent` 结构体 |
| 任务解析失败和高风险输入拦截 | ✅ | `parser.go` — 返回 unsupported + `policy/engine.go` — 风险关键词检测 |
| 建立命令白名单 | ✅ | `internal/policy/engine.go` — 工具白名单（mysqldump、curl 等 20 个） |
| 建立工具白名单 | ✅ | `internal/policy/engine.go` — 模板白名单 |
| 建立命令模板机制 | ✅ | `internal/template/engine.go` — 模板注册、参数校验、命令渲染 |
| 完成 MySQL 备份模板任务 | ✅ | `internal/template/templates.go` — `mysql_backup` 模板 |
| 完成 HTTP 健康检查模板任务 | ✅ | `internal/template/templates.go` — `http_health_check` 模板 |
| 跑通"自然语言 -> 结构化任务 -> 调度执行" | ✅ | `POST /api/v1/tasks/run` — 全链路打通，烟测通过 |

## 操作记录

1. 阅读 Phase 1 产物和 Phase 2 需求，确认要先补齐的模块边界。
2. 在 `internal/model/model.go` 新增 Phase 2 领域模型：
   - `RiskLevel`（low / medium / high）
   - `TaskIntent`（意图识别结果，包含 intent、target_type、schedule、risk_level、parameters、matched_template）
   - `CommandTemplate`（命令模板定义，包含 code、name、description、tool_type、command 模板字符串、参数列表、风险等级）
   - `TemplateParam`（模板参数，包含 name、description、required、default）
3. 创建 `internal/llm/client.go`：
   - 定义 `Client` 接口，`Chat(ctx, messages) (string, error)`
   - 实现 `HTTPClient`，对接 OpenAI 兼容的 `/chat/completions` 端点
   - 支持通过环境变量配置 `ADP_LLM_BASE_URL`、`ADP_LLM_API_KEY`、`ADP_LLM_MODEL`
4. 创建 `internal/template/templates.go` 和 `internal/template/engine.go`：
   - 内置 2 个命令模板：`mysql_backup`（mysqldump）和 `http_health_check`（curl）
   - Engine 支持模板注册、列表查询、参数校验与默认值注入、基于 `text/template` 的命令渲染
5. 创建 `internal/policy/engine.go`：
   - 工具白名单：mysqldump、curl、ping、redis-cli、mysql、echo、cat、grep、df、free、uptime、netstat、ss、head、tail、wc、sort、uniq
   - 模板白名单：mysql_backup、http_health_check
   - 风险评估：基于 TaskIntent 的 risk_level 字段 + 高风险关键词（delete、drop、restart 等）
   - `IsHighRisk()` 用于判断是否需要人工确认
6. 创建 `internal/parser/parser.go`：
   - 解析策略：LLM 优先，失败或未配置时降级为规则匹配
   - LLM 模式：构造 system prompt 定义支持的 intent 和 JSON 输出格式，调用 LLM 后提取 JSON
   - 规则模式：基于正则表达式匹配中文/英文关键词，覆盖 MySQL 备份、HTTP 健康检查、Redis 诊断、Nginx 诊断
   - 支持 schedule 提取（每天→`0 0 * * *`，每小时→`0 * * * *`，每周→`0 0 * * 0`）
   - 支持参数提取（从输入中抽取数据库名、URL 等关键信息）
   - 自动匹配最佳命令模板（intent → template_code 映射）
7. 创建 `internal/parser/parser_test.go`：
   - MySQL 备份场景测试（3 个变体）
   - HTTP 健康检查场景测试（3 个变体）
   - 不可识别输入测试
   - 空输入测试
   - Schedule 提取单元测试
8. 更新 `internal/api/server.go`：
   - `Config` 新增 `LLMBaseURL`、`LLMAPIKey`、`LLMModel` 字段
   - `Server` 新增 `templateEng`、`policyEng`、`taskParser` 字段
   - `NewServer` 自动创建模板引擎、策略引擎、解析器（根据 LLM 配置决定是否启用 LLM）
   - 新增 3 个 API 端点：
     - `GET /api/v1/templates` — 列出所有可用命令模板
     - `POST /api/v1/tasks/parse` — 解析自然语言为结构化任务
     - `POST /api/v1/tasks/run` — 全链路执行（解析→风险评估→模板渲染→白名单校验→任务入队）
   - `handleRunTask` 完整流程：解析 NL → 风险评估 → 高风险拦截 → 模板解析 → 命令渲染 → 工具白名单校验 → 创建 Job
9. 更新 `cmd/server/main.go`：
   - 从环境变量读取 `ADP_LLM_BASE_URL`、`ADP_LLM_API_KEY`、`ADP_LLM_MODEL`
10. 更新 `configs/app.env.example`：
    - 新增 LLM 配置示例（含注释说明）
11. 执行 `go fmt ./...` 格式化所有新增和修改的 Go 文件。
12. 执行 `go test ./...`，所有测试通过（auth、parser、scheduler）。
13. 启动本地服务进行烟测，验证以下 5 个场景：
    - 模板列表查询 → 返回 2 个内置模板
    - MySQL 备份任务解析 → 正确识别意图、提取参数、匹配模板
    - HTTP 健康检查解析 → 正确识别意图、低风险等级
    - 全链路执行（NL → Job）→ 模板参数正确渲染，命令通过白名单校验，Job 成功入队
    - 不可识别输入拦截 → "delete all production data" 被拒绝并返回错误

## 当前结果

Phase 2 已完成 AI 解析与受控执行能力，当前具备：

- 统一的 LLM 调用接口（OpenAI 兼容，可选启用）
- 基于规则匹配的自然语言解析（不依赖 LLM 也可工作）
- 结构化的 TaskIntent JSON 格式
- 高风险输入拦截（关键词检测 + 风险评级）
- 命令模板机制（参数化 + 默认值 + 必填校验）
- 工具白名单（20 个常用运维工具）
- 模板白名单（2 个内置模板）
- MySQL 备份模板任务（mysqldump，6 个参数）
- HTTP 健康检查模板任务（curl，2 个参数）
- 完整的 "自然语言 -> 结构化任务 -> 调度执行" 链路

## 新增文件清单

```
internal/llm/client.go          — LLM 调用接口与 OpenAI 兼容 HTTP 实现
internal/template/engine.go     — 命令模板引擎
internal/template/templates.go  — 内置模板定义
internal/policy/engine.go       — 策略引擎（白名单 + 风险评估）
internal/parser/parser.go       — 自然语言任务解析器
internal/parser/parser_test.go  — 解析器单元测试
```

## 修改文件清单

```
internal/model/model.go         — 新增 RiskLevel、TaskIntent、CommandTemplate、TemplateParam
internal/api/server.go          — 新增 3 个端点、AI 组件集成
cmd/server/main.go              — 新增 LLM 环境变量读取
configs/app.env.example         — 新增 LLM 配置示例
```

## 架构说明

```
用户输入 (NL)
    │
    ▼
┌──────────────┐     ┌──────────────┐
│   Parser     │────▶│  LLM Client  │ (可选)
│  (规则降级)   │     │  (OpenAI API) │
└──────┬───────┘     └──────────────┘
       │ TaskIntent
       ▼
┌──────────────┐
│Policy Engine │──▶ 风险评级 + 高风险拦截
└──────┬───────┘
       │ 通过
       ▼
┌──────────────────┐
│ Template Engine  │──▶ 模板匹配 + 参数渲染
└──────┬───────────┘
       │ 渲染后的命令
       ▼
┌──────────────────┐
│ Policy Engine    │──▶ 工具白名单校验
└──────┬───────────┘
       │ 通过
       ▼
┌──────────────────┐
│   Scheduler      │──▶ 任务入队 → Worker 执行
└──────────────────┘
```

## 当前限制

- 规则匹配覆盖的场景有限（MySQL 备份、HTTP 健康检查），复杂诊断场景需要 LLM
- LLM 模式下未做响应内容的深度安全审计（如 prompt injection 防御）
- 模板数量有限（2 个），后续可按需扩展
- 高风险任务仅做拦截提示，尚未实现 `waiting_approval` 状态和审批流程
- 存储仍为内存实现

Phase 2 解决了什么问题？
Phase 1 里，用户创建任务必须手写具体命令：


curl -X POST /api/v1/jobs -d '{
  "name": "backup",
  "worker_type": "shell",
  "command": "mysqldump -h 127.0.0.1 -u root -p123 mydb > /tmp/backup.sql"
}'
这要求用户既要知道用什么工具，又要记得住参数顺序，还得确保命令安全。Phase 2 让用户只需要用自然语言说想干什么，系统自动完成解析、模板匹配、参数填充和安全校验。

怎么用？
1. 查看系统支持哪些操作

TOKEN=$(curl -s -X POST http://127.0.0.1:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}' | grep -o '"token":"[^"]*"' | cut -d'"' -f4)

curl http://127.0.0.1:8080/api/v1/templates \
  -H "Authorization: Bearer $TOKEN"
返回两个内置模板：mysql_backup 和 http_health_check，每个模板列出了需要的参数和默认值。

2. 用自然语言描述任务（先解析看看）

curl -X POST http://127.0.0.1:8080/api/v1/tasks/parse \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"input":"每天凌晨备份 MySQL 数据库"}'
系统会返回结构化的解析结果，让你确认是否正确理解：


{
  "intent": "create_scheduled_backup",
  "target_type": "mysql",
  "schedule": "0 0 * * *",
  "risk_level": "medium",
  "matched_template": "mysql_backup"
}
3. 一步执行（解析 + 模板渲染 + 安全检查 + 入队）

curl -X POST http://127.0.0.1:8080/api/v1/tasks/run \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "input": "每天凌晨备份 MySQL 数据库",
    "parameters": {
      "Password": "mypass",
      "Database": "mydb"
    }
  }'
全程自动完成：


"每天凌晨备份 MySQL 数据库"
        │
        ▼  规则匹配（或 LLM）
  {intent: "create_scheduled_backup", target_type: "mysql"}
        │
        ▼  模板映射
  mysql_backup 模板（mysqldump -h {{.Host}} ...）
        │
        ▼  参数填充（用户给的 + 默认值）
  mysqldump -h 127.0.0.1 -P 3306 -u root -pmypass mydb > /tmp/backup.sql
        │
        ▼  白名单校验（mysqldump 在允许列表中 ✓）
        │
        ▼  风险评级（medium，无需拦截）
        │
        ▼  入队 → Worker 拉取执行
4. 安全拦截示例

curl -X POST http://127.0.0.1:8080/api/v1/tasks/run \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"input":"delete all production data"}'
# → {"error":"parse failed: unable to parse with rule-based parser: ..."}

# 即使输入被解析为高风险操作，run 端点也会拦截：
# → {"error":"high risk task requires manual approval","risk_level":"high"}
相比 Phase 1 的核心区别
Phase 1	Phase 2
创建任务	手写命令字符串	自然语言描述
安全检查	无	工具白名单 + 模板白名单 + 风险评级
命令规范	任意字符串都可以	只能执行已注册模板 + 允许的工具
参数校验	无	必填参数检查 + 默认值注入
扩展方式	无	注册新模板即可（后续可加更多模板）
简单说就是：Phase 1 是"你来写命令，我来跑"，Phase 2 是"你来说意图，我来选模板、填参数、做安检、再执行"。

## 下一步建议

继续进入 Phase 3 时，建议优先补齐：

- AI 任务规划模块（多步骤诊断计划生成）
- Nginx 不可访问诊断步骤模板
- Redis 响应慢诊断步骤模板
- 执行结果采集与分析
- AI 结果分析模块（故障类型推测、可能原因排序、建议输出）

---
