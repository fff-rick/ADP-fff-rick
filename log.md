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
