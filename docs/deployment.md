# APIPool 生产部署

本文档是 APIPool 当前生产发布与回滚的唯一常青入口。禁止在仓库中写入密码、
token、私钥、生产 `.env`、客户数据或备份内容。

## 生产目标

- 发布分支：`main`
- GitHub Environment：`production`
- 目标主机 SSH alias：`apipool_vps`
- 生产目录：`/opt/sub2api`
- 主站：`https://apipool.dev`
- API 健康检查：`https://api.apipool.dev/health`
- 目标运行单元：`sub2api`、`sub2api-postgres`、`sub2api-redis`
- biz：只保留独立备份，不在目标机安装 `.env.biz`、Compose、容器或 Caddy 路由

DigitalOcean 旧机上的 legacy 数据已落后且容器已停止，只保留观察期备份，既不是
`main` 的发布目标，也不得再作为生产回退入口。

## GitHub 发布链

`.github/workflows/deploy.yml` 在 `main` push 或 `main` 上手动
`workflow_dispatch` 时执行：

1. GitHub-hosted Runner 构建 `linux/amd64` 镜像并推送
   `ghcr.io/afreecoder/apipool:sha-<40位commit>`。
2. 独立仓库级 Runner `sub2api-prod-deploy` 在 `apipool_vps` 接收 deploy job。
3. Runner 以非特权用户运行，只能 sudo 调用固定的
   `/usr/local/sbin/sub2api-runner-deploy`。
4. 固定包装器验证仓库 origin、干净 checkout、完整 SHA、root-owned 工具链和
   生产 `.env` 权限，并逐文件检查仓库工具链与服务器固定副本一致。
5. `/opt/sub2api/deploy/target-deploy.sh` 串行执行 Caddy 校验、发布前数据库备份、
   精确镜像拉取、Compose 更新和健康检查。
6. 已健康运行同一 SHA 时发布幂等跳过，不重建容器。

发布并非多副本滚动发布。普通新 SHA 会重建单个 `sub2api` 容器，客户端可能遇到
一个短暂、可重试的连接窗口；PostgreSQL 和 Redis 不应因应用发布而重建。

## Runner 与工具链所有权

仓库是 Runner 配置和部署工具的唯一事实源：

- `deploy/install-github-runner.sh`
- `deploy/runner-deploy.sh`
- `deploy/install-production-tooling.sh`
- `deploy/target-deploy.sh`
- `deploy/configure-caddy.sh`
- `deploy/caddy-runtime-contract`
- `deploy/docker-compose.deploy.yml`
- `deploy/rollback.sh`

Runner 服务必须在目标机一次性引导安装，原因是尚未运行的 Runner 无法接收自己的
安装任务。普通应用发布不得直接把 checkout 中的任意脚本提升为 root；否则一次仓库
或 workflow 泄露会变成宿主机 root 权限。工具链更新采用以下边界：

1. 在独立 worktree 审查精确 commit；
2. owner 将该 commit 的上述部署件传到目标机临时目录；
3. owner 显式运行仓库内安装器；
4. 后续每次 CI 发布比较 checkout 与固定副本，任何漂移都 fail-closed。

Runner 安装需要一个短期仓库注册 token，通过 stdin 传入，不写入命令行、仓库或磁盘。
Runner 用户不得加入 `docker` 组；其出站 nftables 规则只允许 DNS 与 HTTPS，并拒绝
link-local/metadata 网络。

## 目标目录与权限

```text
/opt/sub2api/
├── backups/                    root:root 0700
└── deploy/                     root:root 0755
    ├── .env                    root:root 0600
    ├── docker-compose.deploy.yml
    ├── configure-caddy.sh
    ├── caddy-runtime-contract
    ├── release.env             root:root 0600
    ├── rollback.sh
    ├── target-deploy.sh
    └── version_resolver.sh
```

硬约束：

- 活动目录不得出现 `.env.biz` 或 `docker-compose.biz.yml`。
- Compose project 固定为 `sub2api`，卷名为
  `sub2api_postgres_data`、`sub2api_redis_data`、`sub2api_sub2api_data`。
- 应用端口只绑定 `127.0.0.1:8080`；5432/6379 不发布到宿主机。
- PostgreSQL 18 与 Redis 8 固定为迁移时源机的精确 digest。
- 应用上限 2GiB、PostgreSQL 2GiB、Redis 512MiB；8GiB 宿主机建议另配受控 swap
  和 OOM/磁盘告警。

## Caddy 共存

目标机同时承载 APIPool_v2 和本服务，Caddy 必须使用共享分片：

```caddyfile
# /etc/caddy/Caddyfile
{
	grace_period 15m
}

import /etc/caddy/sites-enabled/*.caddy
```

- v2 只拥有 `apipool-v2.caddy`。
- 本服务只拥有 `apipool-legacy.caddy`。
- 两个写入器共用 `/run/apipool-caddy.lock`。
- 禁止配置 `auto_https ignore_loaded_certs`，否则 Caddy 会在已有手工证书时仍重复
  发起公网 ACME。legacy 脚本发现该选项时 fail-closed。
- 每次变更先复制全部现有分片，组装完整候选树并执行 `caddy validate`；验证通过后
  才原子替换自己的分片并应用配置。
- 候选分片与线上分片完全一致时直接短路，不 reload Caddy。
- 本服务在目标机只配置 `apipool.dev`，不得配置 API 或 biz 域名。qingyun 转发
  `api.apipool.dev` 时固定使用 `apipool.dev` 作为上游 Host/SNI。
- Caddy 必须在自身存储中管理 `apipool.dev` 的有效公开证书；发布前检查证书存在且
  未临近过期。legacy 分片不再加载会覆盖 v2 子域名的 Origin wildcard。
- APIPool_v2 是共享 Caddy 运行时的唯一 owner；本仓库通过 root-owned
  `/opt/apipool-v2/deploy/caddy-runtime-lib.sh` 校验精确二进制并执行安全 reload，
  只拥有 legacy 分片。
- 固定工具链必须安装 `caddy-runtime-contract`，其契约值为
  `apipool-caddy-runtime-v1`；legacy 写入器还会核对 APIPool_v2 runtime lib 暴露的
  同名契约。升级共享运行时前先安装 APIPool_v2 工具链，再安装本仓库工具链；契约不符
  时共享升级和普通 legacy 发布都 fail-closed。
- 生产运行时为 Caddy `2.11.4`（Go `1.26.5`）。legacy 反代设置
  `stream_close_delay 15m`，共享根设置 `grace_period 15m`，systemd 停止超时为
  16 分钟；reload 后必须确认 MainPID 未变化、进程持续 active 且 journal 无崩溃签名。

## 发布前检查

按变更风险扩大检查范围。部署、账单、认证、数据库迁移、模型路由和公开 API 变更
至少执行：

```bash
git status -sb
git branch --show-current
git remote -v
git log --oneline --decorate -n 5

bash deploy/tests/target-deploy-test.sh
POSTGRES_PASSWORD=dummy docker compose -f deploy/docker-compose.deploy.yml config -q

cd backend
go test -tags=unit ./...
go test -tags=integration ./...
golangci-lint run ./...

cd ../frontend
pnpm install --frozen-lockfile
pnpm run lint:check
pnpm run typecheck
```

若某项无法运行，发布记录必须写明阻断、替代证据和剩余风险。

## 迁移完成状态

- `apipool_vps` 是 legacy PostgreSQL、Redis 与应用的唯一生产主端。
- `apipool.dev` 经 Cloudflare 到目标机；`api.apipool.dev` 经轻云互联到目标机，
  域名和用户入口未改变。
- DigitalOcean legacy 应用保持停止，旧数据不得重新接流；biz 不在目标机部署，仅保留
  已验证的独立备份。
- 后续 `main` push 只由专用 `sub2api-prod-deploy` Runner 发布到目标机。
- 生产 deploy 显式依赖同一 workflow 内的 deployment-contract 检查；独立 CI 尚未
  完成或相关部署测试失败时不得进入自托管 Runner。
- 当前单实例应用发布仍可能带来一个短暂、可重试的容器换代窗口；Caddy 运行时升级则
  必须使用 APIPool_v2 的候选实例透明切流流程，不得直接 restart。

## 发布监控

```bash
gh run list -R AFreeCoder/apipool --workflow 'Deploy to apipool_vps' --limit 3
gh run watch -R AFreeCoder/apipool <run-id> --exit-status
gh run view -R AFreeCoder/apipool <run-id> \
  --json status,conclusion,displayTitle,headSha,jobs

ssh apipool_vps \
  'docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Image}}"'
ssh apipool_vps 'cat /opt/sub2api/deploy/release.env'
ssh apipool_vps 'ls -lt /opt/sub2api/backups | head'
ssh apipool_vps 'docker logs --since 5m sub2api 2>&1 | tail -200'

curl -fsS https://apipool.dev/
curl -fsS https://api.apipool.dev/health
```

成功必须同时满足：

- Actions run 成功且 `release.env` 是预期完整 SHA；
- 运行中 app image ID 与 GHCR 候选 digest 一致；
- 三个容器 healthy，端口仍只在 loopback；
- 发布窗口的新备份非空且通过 `gzip -t`；
- Caddy 完整配置有效，v2 三域名与两个主入口 smoke 均通过；
- 一个低成本真实流式 API 请求完成，调度、认证、用量、余额/账单和幂等记录正确；
- 日志无新的迁移、认证、权限、网络、OOM 或上游出口错误。

## 回滚边界

DigitalOcean 已不是可用回退主端。禁止只回 Cloudflare/qingyun 或重新启动旧容器；
这会让请求落到过期数据库并形成数据分叉。普通故障优先在目标机回滚应用镜像或从目标机
备份恢复；跨主机灾备恢复必须另行制定数据恢复方案并确认写入边界。

应用镜像快速回退：

```bash
ssh apipool_vps 'cd /opt/sub2api/deploy && ./rollback.sh image'
```

数据库恢复是高风险最后手段，必须使用：

```bash
ssh apipool_vps \
  'cd /opt/sub2api/deploy && ./rollback.sh db-restore --with-image'
```

`db-restore` 使用 `psql --set=ON_ERROR_STOP=1`，任一 SQL 错误即失败。数据库恢复、
卷删除、凭据轮换、环境重建以及跨回滚分界的数据反同步必须重新确认精确目标和影响。

## 迁移后观察

- 至少保留旧机和全部迁移备份 7 天，不销毁卷/实例。
- 观察错误率、流式中断、数据库/Redis 延迟、OOM、磁盘、上游认证与余额 24 小时。
- 确认 `main` 后续 push 只发布 `apipool_vps`，同 SHA 幂等发布不会重启。
- 删除临时复制角色、slot、SSH key、隧道和临时 allowlist；收窄腾讯云侧 SSH 规则。
- biz 备份完成后不在目标机启动；其域名后续是否下线是单独决定，不与主站迁移混做。
