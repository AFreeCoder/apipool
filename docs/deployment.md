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

DigitalOcean 旧机只在迁移观察期充当回退入口，不再是 `main` 的发布目标。

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
import /etc/caddy/sites-enabled/*.caddy
```

- v2 只拥有 `apipool-v2.caddy`。
- 本服务只拥有 `apipool-legacy.caddy`。
- 两个写入器共用 `/run/apipool-caddy.lock`。
- 每次变更先复制全部现有分片，组装完整候选树并执行 `caddy validate`；验证通过后
  才原子替换自己的分片和 reload。
- 本服务只配置 `apipool.dev` 与 `api.apipool.dev`，不得配置 biz 域名。
- Cloudflare Origin 证书与私钥放在 `/etc/caddy/certs/`；私钥必须为
  `root:caddy 0640`，只让 root 与 Caddy 运行组读取，不进入 Git。部署脚本会在
  reload 前以 Caddy 运行用户做实际可读性检查，避免仅 root 静态校验通过。

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

## 迁移执行顺序

### 1. 不影响旧站的准备

- 保持 DigitalOcean 的 main、biz、PostgreSQL、Redis 和原域名全部运行。
- 在目标机先暂停无真实用户的 v2 容器；保留其数据和备份。
- 修复目标 SSH 固定来源规则，并从 owner 与 DigitalOcean 各建立第二条并行连接；
  防火墙脚本的自动回滚确认前不得关闭 Web/TAT 会话。
- 安装 Caddy 分片工具，在不改变任何 DNS/源站的前提下完成
  `caddy validate`、reload 和 v2 三域名 smoke。
- 安装独立 APIPool Runner，但在目标数据库仍为 standby 时禁止触发应用发布。
- 从 feature commit 预构建精确候选镜像并记录 digest，提前拉到目标机。

### 2. 在线数据预同步

- PostgreSQL 使用专用临时复制角色、最小 `pg_hba` 和物理 slot，通过 SSH 隧道执行
  `pg_basebackup -R -X stream`；目标确认
  `pg_is_in_recovery()=true`、system identifier 一致、replay lag 持续收敛。
- `max_slot_wal_keep_size=-1` 时必须持续监控 retained WAL 和源盘；源盘空闲低于
  20GiB 或 retained WAL 超过预设阈值时自动中止并删除 slot，不能让旧站磁盘被写满。
- Redis 通过同一类仅内网可达的 SSH 隧道建立 replica；确认
  `master_link_status=up`、全量同步结束、offset 持续追平。
- app-data 先做一次在线 rsync；最终增量在写屏障内完成。
- 在线生成 main 与 biz 的逻辑备份，并归档 Redis、两个 app-data、`.env*`、
  Compose、Caddy、镜像 digest 与 SHA-256 清单。biz 只进入备份区。

### 3. 短写屏障与提升

选择实测低流量窗口：

1. 停止旧机 main 和 biz 应用，不先停 PostgreSQL/Redis。
2. 确认无 active/idle-in-transaction、WAL/事务提交停止增长，等待异步计费与缓存写入
   排空。
3. PostgreSQL 等待 replay LSN 与源端 flush LSN 相等；Redis 等待复制 offset 相等。
4. 完成 app-data 最终 rsync 和 biz 最终独立逻辑备份。
5. 停止目标复制，提升目标 PostgreSQL 与 Redis，验证 timeline/role。
6. 以预拉取的精确候选镜像受控启动目标 main；先通过本机 Host/SNI、真实上游流式请求、
   账单和幂等检查。
7. 让旧 DigitalOcean Caddy 临时桥接目标机，使仍命中旧入口的请求也到达新主站。
8. 先改 qingyun 的 `api.apipool.dev` 上游，再改 Cloudflare 的
   `apipool.dev` origin；域名本身不变。
9. 旧机 main/数据库/Redis 保持停止且禁止自动重启；biz 按本次范围保持停止，
   不在目标机部署，域名配置不变，仅保留已验证的独立备份，
   但其数据已经独立归档且不会进入目标机。

当前单主架构不能承诺“任何长流请求都绝不重试”。验收目标是域名持续可达、数据零丢失，
写屏障为演练确认的秒级窗口，极少数切换瞬间请求最多重试一次。若要求单请求严格零中断，
必须先实现应用多副本 drain 与数据层 HA，这属于独立架构改造。

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

### 目标开始写入前

旧机仍是唯一数据主端，可停止目标副本、恢复旧入口，不需要反向同步。

### 目标应用开始写入后

这是不可直接回 DNS 的分界。即使没有用户请求，后台任务也可能写 PostgreSQL/Redis。
回滚必须：

1. 停止目标应用；
2. 用 `pg_rewind` 或重新物理同步让旧 PostgreSQL 追上目标；
3. 反向同步 Redis 与 app-data；
4. 验证旧端数据一致后再恢复入口。

只回 Cloudflare 或 qingyun 而不处理数据层会形成双主和数据丢失。

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
