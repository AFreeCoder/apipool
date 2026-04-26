# GHCR 构建与服务器拉取部署设计

## 背景

当前生产部署由 `.github/workflows/deploy.yml` 在 push 到 `main` 后通过 SSH 登录 DigitalOcean 服务器完成。服务器会拉取仓库源码，然后执行 `docker compose -f docker-compose.deploy.yml build` 在服务器本机打包前端、编译后端并构建镜像。

这个流程会在生产服务器上消耗 CPU、内存、磁盘和 Docker build cache。部署期间服务器既承担线上流量，又承担构建任务，资源争用明显，也让构建失败和运行失败混在同一个环境中。

## 目标

- GitHub Actions runner 负责构建并推送应用镜像到 GHCR。
- 生产服务器部署阶段只拉取指定镜像、更新本地部署别名并重启容器。
- 保留现有部署前数据库备份、健康检查、企业版实例同步更新和快速镜像回滚能力。
- 部署仍由 push 到 `main` 自动触发，人工操作习惯不变。
- 服务器不再执行应用镜像构建，也不再依赖 Docker build cache。

## 非目标

- 不改变 PostgreSQL、Redis、卷、网络和运行时环境变量的现有行为。
- 不把生产部署改为 tag-only release 流程。
- 不重写发布 workflow，也不要求先创建 GitHub Release 才能上线。
- 不在本次设计中引入多区域、蓝绿部署或金丝雀发布。

## 推荐方案

采用方案 A：在同一个 `.github/workflows/deploy.yml` 中先构建并推送 GHCR 镜像，再通过 SSH 让服务器拉取该镜像完成更新。

镜像仓库默认使用：

```text
ghcr.io/afreecoder/apipool
```

每次部署至少推送三个标签：

```text
ghcr.io/afreecoder/apipool:sha-<12位commit>
ghcr.io/afreecoder/apipool:main
ghcr.io/afreecoder/apipool:latest
```

服务器部署时使用 `sha-<12位commit>` 精确标签，避免 `latest` 在拉取与重启之间发生漂移。拉取成功后将远程镜像重新打本地标签：

```text
deploy-sub2api:latest
```

这样 `docker-compose.deploy.yml` 和 `rollback.sh` 可以继续围绕本地稳定镜像名工作，已有 `deploy-sub2api:rollback-latest` 和 `deploy-sub2api:rollback-*` 语义保持不变。

## Workflow 设计

`.github/workflows/deploy.yml` 保留触发条件：

- `push` 到 `main`
- `workflow_dispatch`

新增构建阶段：

1. `actions/checkout` 拉取源码，保留足够 commit 信息用于版本元数据。
2. 计算部署镜像名和短 commit：
   - `IMAGE_REPO=ghcr.io/afreecoder/apipool`
   - `APP_COMMIT=${GITHUB_SHA:0:12}`
   - `IMAGE_TAG=sha-${APP_COMMIT}`
3. 读取应用版本：
   - 优先使用 `backend/cmd/server/VERSION`
   - 与当前 Dockerfile 的 `VERSION` build arg 保持一致
4. 设置 Docker Buildx。
5. 使用 `GITHUB_TOKEN` 登录 GHCR。仓库需要 `permissions: packages: write`。
6. 执行 `docker/build-push-action`：
   - `context: .`
   - `file: Dockerfile`
   - `push: true`
   - `platforms: linux/amd64`
   - `build-args` 注入 `VERSION`、`COMMIT`、`DATE`
   - `tags` 写入 `sha-<commit>`、`main`、`latest`
   - 使用 GitHub Actions cache 减少 runner 构建耗时

服务器 SSH 阶段接收 `DEPLOY_IMAGE` 和 `DEPLOY_COMMIT`，不再执行 `docker compose build`。

## 服务器部署流程

服务器脚本保留现有前置检查和备份顺序：

1. 检查 `git`、`docker`、`gzip` 等命令。
2. 读取 `$DEPLOY_DIR/.env` 中的部署参数。
3. 检查 `/opt` 可用磁盘空间。阈值可以继续保持 5GB，避免拉取镜像或写备份时空间不足。
4. 如 PostgreSQL 容器存在，先执行 pre-deploy 数据库备份。
5. 如服务器尚未克隆仓库，仍克隆 `https://github.com/AFreeCoder/apipool.git`，用于保留部署脚本、compose 文件和回滚脚本。
6. 在更新代码前，对当前本地 `deploy-sub2api:latest` 创建 rollback tag。
7. 拉取 `origin/main` 并 `git reset --hard origin/main`，让服务器上的 compose、rollback 脚本和文档保持最新。
8. 执行：

```bash
docker pull "$DEPLOY_IMAGE"
docker tag "$DEPLOY_IMAGE" deploy-sub2api:latest
```

9. 执行 `docker compose -f docker-compose.deploy.yml config -q`。
10. 执行 `docker compose -f docker-compose.deploy.yml up -d --remove-orphans`。
11. 等待 `sub2api-postgres`、`sub2api-redis`、`sub2api` 健康。
12. 清理悬空镜像和过旧 rollback tags。
13. 如果 `docker-compose.biz.yml` 和 `.env.biz` 存在，企业版实例继续使用同一个 `deploy-sub2api:latest` 本地镜像重启。

关键失败语义：

- GHCR 构建或推送失败时，不进入 SSH 部署阶段。
- 服务器 `docker pull` 失败时，不改动 `deploy-sub2api:latest`，当前线上容器继续运行。
- `docker tag` 成功但健康检查失败时，已存在的 `deploy-sub2api:rollback-latest` 可用于快速回滚。

## Compose 设计

`deploy/docker-compose.deploy.yml` 的 `sub2api` 服务从源码构建改成镜像引用：

```yaml
services:
  sub2api:
    image: ${SUB2API_IMAGE:-deploy-sub2api:latest}
```

默认值为 `deploy-sub2api:latest`，保证服务器部署和回滚脚本不需要额外环境变量即可工作。`SUB2API_IMAGE` 作为覆盖入口保留，便于临时验证远程镜像或排查问题。

`deploy/docker-compose.biz.yml` 继续使用：

```yaml
image: deploy-sub2api:latest
```

企业版实例不独立拉取镜像，只复用主部署流程已拉取并标记的本地镜像。

## 回滚设计

现有快速回滚依赖本地镜像标签：

- `deploy-sub2api:latest`
- `deploy-sub2api:rollback-latest`
- `deploy-sub2api:rollback-YYYYmmdd_HHMMSS-<commit>`

新流程会在每次部署前继续对当前 `deploy-sub2api:latest` 打 rollback tag。部署新版本时先拉取远程 `sha-<commit>` 镜像，再将其标记为 `deploy-sub2api:latest`。

`rollback.sh image` 不需要改变核心语义：把 rollback 镜像重新标记为 `deploy-sub2api:latest`，然后 force recreate 应用容器。

`rollback.sh source <commit>` 的行为需要调整或降级说明。旧流程会回退源码并在服务器重新构建镜像，这与“服务器不构建镜像”的目标冲突。新设计中推荐将生产回滚路径收敛为：

- 首选：`rollback.sh image`
- 数据库恢复后应用回退：`rollback.sh db-restore --with-image`

`rollback.sh source` 可以暂时保留给紧急手工场景，但不应作为自动化部署推荐路径。实施时需要同步更新 `deploy/ROLLBACK_CN.md`，避免文档继续把源码回滚描述成常规路径。

## 配置与权限

GitHub Actions 需要：

```yaml
permissions:
  contents: read
  packages: write
```

如果 GHCR package 是私有的，生产服务器需要能拉取私有镜像。推荐在服务器上提前执行一次：

```bash
docker login ghcr.io -u AFreeCoder
```

密码使用具备 package read 权限的 token。部署脚本不保存 token，也不在 workflow 日志中输出 token。

如果 GHCR package 设为公开，服务器无需额外登录即可拉取。

## 测试与验证

本地验证：

- `docker compose -f deploy/docker-compose.deploy.yml config -q`
- 检查 `docker compose config` 输出中的 `sub2api` 不再包含 `build`
- 检查 `sub2api` 镜像默认为 `deploy-sub2api:latest`

Workflow 验证：

- 检查 YAML 语法。
- 确认 deploy job 的构建步骤在 SSH 步骤之前。
- 确认 SSH 脚本中不存在 `docker compose ... build`。
- 确认传给服务器的镜像标签为 commit 精确标签，而不是只用 `latest`。

生产验证：

- 首次部署前确认服务器可 `docker pull ghcr.io/afreecoder/apipool:sha-<commit>`。
- 部署后确认 `docker image inspect deploy-sub2api:latest` 存在。
- 部署后确认 `sub2api` 健康检查通过。
- 部署后确认 `rollback.sh image` 仍能基于 `deploy-sub2api:rollback-latest` 回滚。

## 风险与缓解

- GHCR 私有镜像拉取失败：上线前在服务器完成 `docker login ghcr.io`，并用一次手工 `docker pull` 验证。
- `latest` 标签漂移：服务器部署只使用 `sha-<commit>` 精确标签，`latest` 仅作为便捷标签保留。
- 回滚镜像不存在：首次部署或历史流程中没有 `deploy-sub2api:latest` 时，继续跳过回滚打标；后续部署会自动建立 rollback 链路。
- 服务器仓库代码仍需更新：保留 `git fetch` 和 `git reset --hard origin/main`，但它只用于部署脚本和 compose 文件，不再用于构建镜像。
- 源码回滚路径与新目标冲突：文档中把 `rollback.sh source` 降为紧急手工路径，常规回滚使用镜像回滚。
