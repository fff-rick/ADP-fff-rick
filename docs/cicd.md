# GitHub PR CI/CD

本仓库提供了一条面向 PR 的 GitHub Actions 流水线：

1. PR 打开、重新打开、更新提交时触发
2. 在 GitHub Runner 上执行 `golangci-lint`
3. 在 GitHub Runner 上执行 `go test ./...`
4. 通过 SSH 连接远程主机 `43.136.82.118`
5. 在远程主机同步最新 PR 代码
6. 按 `deploy/k8s/release.env` 中声明的版本构建 `server`、`worker` 镜像
7. 将 Deployment 镜像更新到新版本，并等待滚动发布完成

## 版本文件

PR 中需要更新的镜像版本位于：

- `deploy/k8s/release.env`

当前至少需要维护：

```env
ADP_IMAGE_TAG=0.1.0
```

建议每次触发部署时都递增或变更这个 tag，避免集群继续复用旧镜像缓存。

## Lint 规则

PR 会自动执行 `.golangci.yml` 中定义的基础静态检查，当前启用：

- `errcheck`
- `gofmt`
- `govet`
- `ineffassign`
- `staticcheck`
- `unused`

配置文件位置：

- `.golangci.yml`

## GitHub Secrets

在仓库 `Settings -> Secrets and variables -> Actions` 中添加：

- `ADP_DEPLOY_USER`
  说明：远程主机 SSH 用户名
- `ADP_DEPLOY_SSH_KEY`
  说明：用于登录远程主机的私钥内容
- `ADP_DEPLOY_REPO_DIR`
  说明：远程主机上的仓库目录，例如 `/srv/adp`
- `ADP_DEPLOY_REPO_URL`
  说明：可选；当远程目录不存在时用于首次 `git clone`
- `ADP_DEPLOY_PORT`
  说明：可选；SSH 端口，默认 `22`
- `ADP_K8S_RUNTIME`
  说明：可选；支持 `docker`、`kind`、`k3s`，默认 `docker`
- `ADP_K8S_ENV_FILE`
  说明：可选；远程主机上的运行时 env 文件路径，默认 `/etc/adp/adp.env`

## 远程主机准备

远程主机需要具备：

- `git`
- `docker`
- `kubectl`
- 可以访问目标 Kubernetes 集群的 kubeconfig
- 可以访问仓库源码的 Git 凭据

如果 `ADP_K8S_RUNTIME=docker`，默认假设 Kubernetes 运行时能够直接读取远程主机本地构建出的 Docker 镜像。

如果你的集群不能直接读取本地 Docker 镜像，建议两种做法二选一：

1. 使用 `kind` 或 `k3s`，并把 `ADP_K8S_RUNTIME` 设为对应值
2. 在 `deploy/k8s/release.env` 中设置 `ADP_IMAGE_REPOSITORY_PREFIX`，同时把 `ADP_PUSH_IMAGES=true`，让远程主机在构建后推送到镜像仓库

示例：

```env
ADP_IMAGE_TAG=0.1.3
ADP_IMAGE_REPOSITORY_PREFIX=registry.example.com/adp
ADP_PUSH_IMAGES=true
```

## 远程运行时配置

部署脚本会把 `ADP_K8S_ENV_FILE` 里的环境变量同步成 K8s Secret `adp-runtime`。

建议远程主机上的 env 文件至少包含：

```env
ADP_SERVER_ADDR=:8080
ADP_ADMIN_USERNAME=admin
ADP_ADMIN_PASSWORD=replace-me
ADP_AUTH_SECRET=replace-me
ADP_WORKER_SHARED_TOKEN=replace-me
ADP_SERVER_URL=http://adp-server:8080
ADP_WORKER_NAME=worker-k8s-1
ADP_WORKER_TYPE=shell
ADP_WORKER_POLL_INTERVAL=5s
```

如需启用 LLM：

```env
ADP_LLM_BASE_URL=https://api.openai.com/v1
ADP_LLM_API_KEY=sk-...
ADP_LLM_MODEL=gpt-4
```

## Kubernetes 资源

基础资源定义位于：

- `deploy/k8s/manifests/server-deployment.yaml`
- `deploy/k8s/manifests/server-service.yaml`
- `deploy/k8s/manifests/worker-deployment.yaml`

部署脚本会：

1. 确保 namespace 存在
2. 更新 `adp-runtime` Secret
3. `kubectl apply` 基础 manifests
4. `kubectl set image` 触发 Deployment 滚动更新
5. 等待两个 Deployment rollout 完成

## 触发边界

由于 GitHub 对 `pull_request` 事件中的 secrets 有保护限制：

- 同仓库 PR：会执行完整部署
- fork PR：不会进入远程部署步骤

这能避免把远程 SSH 凭据暴露给不受信任的 PR 代码。
