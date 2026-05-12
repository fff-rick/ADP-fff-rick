# Phase 1 实现说明

## 目标

Phase 1 的目标是把项目从“只有文档和目录骨架”推进到“可运行的最小后端原型”。

本阶段优先完成：

- Go 工程初始化
- 基础 HTTP 服务
- 简化鉴权能力
- Worker 注册与心跳接口
- 不依赖 AI 的最小任务调度闭环

## 当前实现策略

为了尽快跑通主链路，Phase 1 先采用 Go 标准库实现基础能力，不额外引入第三方依赖。

这样做的原因：

- 可以减少环境准备复杂度
- 可以避免在最初阶段把问题扩大到依赖管理
- 可以更快验证接口设计和模块边界是否合理

后续阶段如果需要，可以平滑替换为：

- Gin 作为 HTTP 框架
- gRPC 作为 Server 与 Worker 通信协议
- MySQL 与 Redis 作为真实存储与队列

## 已实现能力

### 1. 工程初始化

- 创建 `go.mod`
- 提供 `configs/app.env.example`

### 2. 服务端能力

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

### 3. 鉴权能力

- 用户侧采用简化的 JWT 鉴权
- Worker 侧采用共享令牌 `X-Worker-Token`

### 4. 最小调度闭环

当前闭环如下：

1. 用户登录并获取 Token
2. 用户创建任务
3. Worker 注册
4. Worker 周期性发送心跳
5. Worker 拉取匹配类型任务
6. Worker 回传任务完成结果

## 当前约束

- 当前存储为内存实现，服务重启后数据会丢失
- 当前 Worker 执行为模拟执行，不涉及真实 Shell/SSH 命令
- 当前鉴权为开发期简化实现，后续应接入真实用户存储与密码哈希

## 下一步建议

Phase 2 可以优先补以下内容：

- MySQL 持久化
- Redis 队列
- 更完整的任务状态机
- 更真实的 Worker 执行模型
- AI 解析与任务模板能力
