# ADP

ADP 是一个面向智能运维场景的原型项目，当前定义为“基于 AI 的辅助式任务调度平台”。

本仓库目前完成的是 `Phase 0`，重点不是功能开发，而是先把项目范围、架构边界、技术栈选择和仓库骨架搭建清楚，为后续实现打基础。

## Phase 0 范围

首版能力刻意收敛为以下 3 个典型场景：

1. MySQL 定时备份
2. Nginx 可用性诊断
3. Redis 性能诊断

首版不以支持以下能力为目标：

- 不受限制的自由命令执行
- 面向任意任务的通用自治运维
- 全量云原生集群治理
- 生产级多租户平台能力
- 高风险操作的全自动修复

## 模块边界

- `Web/API`：认证、请求入口、结果查询
- `Control Plane`：任务解析、任务规划、策略校验、调度控制、结果分析
- `Scheduler`：任务入队、任务分发、失败重试、超时处理、执行追踪
- `Worker`：只负责受控执行，不做自主决策
- `LLM Gateway`：统一模型调用抽象层
- `MySQL`：保存元数据、任务定义、任务执行记录、审计日志、故障案例
- `Redis`：承担队列、缓存和轻量协调能力

## 建议技术栈

- Go `1.24.x`
- Gin `1.10.x`
- gRPC `1.70.x`
- MySQL `8.0.x`
- Redis `7.2.x`
- Docker Compose `v2`
- Prometheus `2.x`

版本策略：

- 基础设施版本优先选择稳定、主流方案
- 第一阶段尽量减少依赖数量
- 向量数据库、ELK 等非核心组件后置

## 初始目录结构

```text
ADP/
  cmd/
    server/
    worker/
  internal/
    analyzer/
    api/
    auth/
    model/
    planner/
    policy/
    scheduler/
    worker/
  api/
    proto/
  configs/
  deploy/
    docker-compose/
  docs/
  scripts/
  tests/
    integration/
  README.md
  log.md
```

## Phase 0 交付物

- 明确 V1 业务范围
- 明确系统架构边界
- 固定初始技术栈与建议版本
- 初始化仓库目录结构
- 在 [log.md](./log.md) 中记录操作过程

说明：

- `go.mod` 暂未在 Phase 0 创建，因为模块路径最好与最终仓库命名或远端地址保持一致，等仓库信息稳定后再初始化更稳妥

## 下一步

完成 Phase 0 后，建议进入 Phase 1：

- 后端工程初始化
- 鉴权与基础数据模型实现
- Worker 注册与心跳能力实现
- 跑通不依赖 AI 的最小调度闭环

## 当前进度

目前已经完成 Phase 1 和 Phase 2：

- Phase 1：最小调度闭环（HTTP API、JWT 鉴权、Worker 注册/心跳、任务创建/分发/完成）
- Phase 2：AI 解析与受控执行（LLM 调用接口、自然语言解析、命令模板、工具白名单、策略引擎、MySQL 备份/HTTP 健康检查模板）

详细实现说明见 [docs/phase1.md](./docs/phase1.md) 和 [log.md](./log.md)。

## 本地运行

1. 启动服务端：

```bash
go run ./cmd/server
```

2. 启动 Worker：

```bash
go run ./cmd/worker
```

3. 默认开发账号：

- 用户名：`admin`
- 密码：`admin123`

4. 配置参考：

- 环境变量示例见 [configs/app.env.example](./configs/app.env.example)

## Phase 2 新增 API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/templates` | 列出所有可用命令模板 |
| POST | `/api/v1/tasks/parse` | 将自然语言解析为结构化任务 |
| POST | `/api/v1/tasks/run` | 全链路执行（解析→模板渲染→白名单校验→入队） |

示例：

```bash
# 解析自然语言任务
curl -X POST http://127.0.0.1:8080/api/v1/tasks/parse \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"input":"每天凌晨备份 MySQL 数据库"}'

# 执行自然语言任务（全链路）
curl -X POST http://127.0.0.1:8080/api/v1/tasks/run \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"input":"每天凌晨备份 MySQL 数据库","parameters":{"Password":"mypass","Database":"mydb"}}'
```

## 下一步

继续进入 Phase 3 时，建议优先补齐：

- AI 任务规划模块（多步骤诊断计划）
- Nginx 不可访问诊断步骤模板
- Redis 响应慢诊断步骤模板
- 执行结果采集与 AI 分析
