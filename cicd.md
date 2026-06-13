# ADP CI/CD 操作手册

本文档面向当前仓库的实际部署方式，目标环境为：

- GitHub Actions 触发
- 远程主机 `43.136.82.118`
- 单节点 Kubernetes
- 容器运行时为 `containerd`
- 镜像在远程主机本地构建，再导入节点本机 `containerd`

这套方案适合单节点集群。如果以后扩容为多节点，建议改为推送镜像仓库。

## 流程概览

当前工作流文件：

- `.github/workflows/pr-cicd.yml`

远程部署脚本：

- `scripts/remote_pr_deploy.sh`

执行顺序：

1. PR 触发 GitHub Actions
2. GitHub Runner 执行 `golangci-lint`
3. GitHub Runner 执行 `go test ./...`
4. PR 阶段只做校验，不执行远程部署
5. 代码 merge 到 `main` 后，由 `push` 事件触发部署
6. GitHub Runner 通过 SSH 登录 `43.136.82.118`
7. 远程主机拉取 `main` 分支对应最新代码
8. 远程主机本地构建 `adp-server`、`adp-worker` 镜像
9. 远程主机执行 `sudo ctr -n k8s.io images import` 导入镜像到节点运行时
10. 远程主机把 `/etc/adp/adp.env` 同步为 K8s Secret `adp-runtime`
11. 远程主机 `kubectl apply` 基础 manifests
12. 远程主机 `kubectl set image` 触发滚动更新
13. 远程主机等待 `Deployment` rollout 成功

## 远程主机准备

以下步骤在 `43.136.82.118` 上执行，假设当前登录用户为 `ubuntu`，且已具备 `sudo` 权限。

### 1. 创建部署用户

```bash
sudo adduser adpdeploy
sudo usermod -aG sudo adpdeploy
sudo groupadd docker 2>/dev/null || true
sudo usermod -aG docker adpdeploy
```

### 2. 复制 kubeconfig 给部署用户

如果当前 `ubuntu` 用户可以正常执行 `kubectl get nodes`，可直接复制：

```bash
sudo mkdir -p /home/adpdeploy/.kube
sudo cp /home/ubuntu/.kube/config /home/adpdeploy/.kube/config
sudo chown -R adpdeploy:adpdeploy /home/adpdeploy/.kube
sudo chmod 700 /home/adpdeploy/.kube
sudo chmod 600 /home/adpdeploy/.kube/config
```

如果 kubeconfig 不在 `/home/ubuntu/.kube/config`，先执行：

```bash
echo "$KUBECONFIG"
ls -la ~/.kube/config
```

然后改成真实路径再复制。

### 3. 允许部署用户导入 containerd 镜像

当前脚本在 `containerd` 模式下会执行：

- `sudo ctr -n k8s.io images import ...`

因此需要给部署用户一条最小 sudo 权限：

```bash
sudo visudo -f /etc/sudoers.d/adpdeploy-containerd
```

写入：

```sudoers
adpdeploy ALL=(ALL) NOPASSWD: /usr/bin/ctr, /bin/ctr
```

### 4. 创建远程代码目录与运行时配置目录

```bash
sudo mkdir -p /srv/adp
sudo chown -R adpdeploy:adpdeploy /srv/adp

sudo mkdir -p /etc/adp
sudo touch /etc/adp/adp.env
sudo chown root:adpdeploy /etc/adp/adp.env
sudo chmod 640 /etc/adp/adp.env
```

### 5. 配置运行时环境变量

编辑：

```bash
sudo nano /etc/adp/adp.env
```

至少写入：

```env
ADP_SERVER_ADDR=:8080
ADP_ADMIN_USERNAME=admin
ADP_ADMIN_PASSWORD=change-this-password
ADP_AUTH_SECRET=change-this-to-a-long-random-secret
ADP_WORKER_SHARED_TOKEN=change-this-to-a-long-random-token
ADP_SERVER_URL=http://adp-server:8080
ADP_WORKER_NAME=worker-k8s-1
ADP_WORKER_TYPE=shell
ADP_WORKER_POLL_INTERVAL=5s
```

说明：

- 这个项目当前没有把管理员账号持久化到数据库
- `admin` 用户会在服务启动时由环境变量初始化
- 实际管理员密码就是 `/etc/adp/adp.env` 中的 `ADP_ADMIN_PASSWORD`

### 6. 验证部署用户权限

```bash
su - adpdeploy
docker ps
kubectl get nodes
sudo ctr -n k8s.io images ls | head
```

如果这三步都正常，远程主机准备基本完成。

## 远程主机访问仓库源码

远程部署脚本在首次执行时会 `git clone` 仓库。

`ADP_DEPLOY_REPO_URL` 必须填写仓库的 clone URL，而不是仓库主页 URL。

可选格式：

- HTTPS clone URL：`https://github.com/<owner>/<repo>.git`
- SSH clone URL：`git@github.com:<owner>/<repo>.git`

不要填写：

- `https://github.com/<owner>/<repo>`

如果仓库是私有的，推荐在远程主机的 `adpdeploy` 用户下配置一个只读 deploy key。

## GitHub Actions 登录远程主机

建议在你自己的电脑上生成一把专门给 GitHub Actions 使用的 SSH 密钥：

```bash
ssh-keygen -t ed25519 -C "github-actions-adp-deploy" -f ./adp_github_actions
```

会得到：

- 私钥：`./adp_github_actions`
- 公钥：`./adp_github_actions.pub`

把公钥追加到远程主机部署用户：

```bash
cat ./adp_github_actions.pub | ssh ubuntu@43.136.82.118 'sudo -u adpdeploy tee -a /home/adpdeploy/.ssh/authorized_keys >/dev/null'
ssh ubuntu@43.136.82.118 'sudo chown -R adpdeploy:adpdeploy /home/adpdeploy/.ssh && sudo chmod 700 /home/adpdeploy/.ssh && sudo chmod 600 /home/adpdeploy/.ssh/authorized_keys'
```

验证：

```bash
ssh -i ./adp_github_actions adpdeploy@43.136.82.118
```

### SSH 密钥放置规则

这里很容易配反，规则固定如下：

- GitHub Secret `ADP_DEPLOY_SSH_KEY`：放私钥全文
- 远程主机 `/home/adpdeploy/.ssh/authorized_keys`：放对应公钥

不要把公钥填进 `ADP_DEPLOY_SSH_KEY`，也不要把私钥追加到 `authorized_keys`。

如果 Actions 报错：

```text
Permission denied (publickey,password)
```

优先检查：

1. `ADP_DEPLOY_SSH_KEY` 是否真的是私钥全文
2. 对应公钥是否已经追加到 `adpdeploy` 用户的 `authorized_keys`
3. 你本地是否能先用这把私钥手动登录：

```bash
ssh -i ./adp_github_actions -p 22 adpdeploy@43.136.82.118
```

## GitHub 仓库配置

进入：

- `Settings`
- `Secrets and variables`
- `Actions`
- `Secrets`

新增以下 Repository Secrets：

- `ADP_DEPLOY_USER`
  - `adpdeploy`
- `ADP_DEPLOY_SSH_KEY`
  - `adp_github_actions` 私钥全文
- `ADP_DEPLOY_REPO_DIR`
  - `/srv/adp`
- `ADP_DEPLOY_REPO_URL`
  - 仓库 clone URL
- `ADP_DEPLOY_PORT`
  - `22`
- `ADP_K8S_RUNTIME`
  - `containerd`
- `ADP_K8S_ENV_FILE`
  - `/etc/adp/adp.env`
- `ADP_CONTAINERD_NAMESPACE`
  - `k8s.io`

## 仓库内版本配置

编辑：

- `deploy/k8s/release.env`

保持为：

```env
ADP_IMAGE_TAG=0.1.0
ADP_IMAGE_REPOSITORY_PREFIX=
ADP_PUSH_IMAGES=false
```

说明：

- `ADP_PUSH_IMAGES=false` 表示不推送镜像仓库
- `ADP_IMAGE_REPOSITORY_PREFIX` 留空，表示使用本地镜像名
- 每次部署前应更新 `ADP_IMAGE_TAG`

## 第一次触发部署

建议按下面顺序验证：

1. 创建分支
2. 修改 `deploy/k8s/release.env` 中的 `ADP_IMAGE_TAG`
3. 提交并推送分支
4. 创建非 draft PR，观察 `lint` 和 `test`
5. 确认 PR 阶段不会执行 `deploy`
6. 将 PR merge 到 `main`
7. merge 后观察 `main` 分支上的 `push` workflow 执行 `deploy`

工作流会依次执行：

1. PR 阶段：
   - `lint`
   - `test`
2. merge 到 `main` 后：
   - `lint`
   - `test`
   - `deploy`

## 部署后检查

在远程主机执行：

```bash
kubectl -n adp get pods -o wide
kubectl -n adp get deployments
kubectl -n adp rollout status deployment/adp-server
kubectl -n adp rollout status deployment/adp-worker
kubectl -n adp logs deployment/adp-server --tail=100
kubectl -n adp logs deployment/adp-worker --tail=100
```

如果需要确认环境中的管理员密码：

```bash
sudo grep '^ADP_ADMIN_PASSWORD=' /etc/adp/adp.env
```

## 常见问题

### 1. 为什么 PR 阶段就开始部署了

旧版 workflow 会在同仓库 PR 更新时直接执行 `deploy`。当前仓库已经调整为：

- PR 只做 `lint` 和 `test`
- 只有 `main` 分支收到新的 `push` 才执行部署

如果你仍然在 PR 阶段看到部署，通常说明触发运行的还是旧版 workflow，需要等包含新 workflow 的提交合并后再看下一次执行结果。

### 2. `ADP_DEPLOY_REPO_URL` 应该填什么

必须填写仓库的 clone URL，不是仓库主页 URL。

正确示例：

- `https://github.com/<owner>/<repo>.git`
- `git@github.com:<owner>/<repo>.git`

错误示例：

- `https://github.com/<owner>/<repo>`

## 注意事项

- 该方案适合单节点 Kubernetes
- 如果未来扩容为多节点，其他节点拿不到这份本地镜像，需要改为镜像仓库方案
- 当前项目用户体系为内存态，服务重启后，运行时创建的用户不会持久化
- 长期有效的管理员账号密码以 `/etc/adp/adp.env` 为准
