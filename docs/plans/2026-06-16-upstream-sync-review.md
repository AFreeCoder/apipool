# 上游同步评审记录（2026-06-16）

## 基线

- 当前分支：`main`
- 合并提交：`fc1a9eb342915926ef93183600d22e5932379753`
- 合入前本地基线：`9604ce2d7d102e30be34d05c93f2c31f42e1e95e`
- 上游引用：`upstream/main`
- 上游 SHA：`4a5665da5b2c6b83c4597844ea6e573746c821b1`
- merge-base：`e34ad2b19424844cbd1bcffb599e5b0002ad3c50`
- 同步方式：`git merge --no-ff upstream/main`
- upstream 最新 release tag：`v0.1.137`
- upstream/main 版本文件：`0.1.137`
- 本地最终 VERSION：`0.1.137`

## 上游更新摘要

- OpenAI / gateway：保留 SSE `event:error` body、非 JSON 2xx failover、zstd 解压、image server error failover、Responses fallback anchor、Cyber policy 透传/审计/计费。
- Thinking / reasoning：国产模型 thinking-enabled 自动填充 `reasoning_effort`，`max` 归一到 `xhigh`，Anthropic-compatible thinking block 按 mapped model 路径过滤。
- 计费：增加 GLM、Kimi、MiniMax、DeepSeek V4、Doubao embedding vision 等 fallback pricing。
- OpenAI 账号：新增 quota 查询与 reset rate-limit credits，账号列表展示 account id。
- 调度 / outbox：scheduler outbox dedup、claim/release、typed-nil、cleanup grace、invalid index recover。
- 监控 / 风控：channel monitor jitter 配置，auth IP ACL denial message 带真实 client IP。
- 构建 / 前端：form-data override、docs/legal Docker context、SettingsView 更新。

## 本地定制保护点

- 保留 APIPool 品牌、首页/文档入口、购买订阅 iframe、OpenClaw/Kiro/OpenAI OAuth 兼容链路。
- 保留 DigitalOcean GHCR 部署链路、部署前 DB 备份、企业版 DB 预备份、rollback image/source/db 回退脚本。
- 保留 `docs/legal` 构建上下文：`.dockerignore` 使用 `docs/*` + `!docs/legal/` + `!docs/legal/*.md`，`Dockerfile` 与 `deploy/Dockerfile` 均保留法律文档复制。
- 保留 reqlog 管理后台入口与生产 compose 默认总开关；当前 Redis 初始化走 host/port 配置，不走 `redis.ParseURL`。

## 冲突与取舍

- `.dockerignore`：吸收上游 docs/legal 修复，同时保留本地文档忽略策略。
- `backend/internal/repository/migrations_runner.go`：同时保留本地 `150_account_group_scheduler_indexes_notx.sql` 预处理和上游 `153_scheduler_outbox_pending_dedup_key_index_notx.sql` 预处理。
- `backend/internal/repository/migrations_runner_notx_test.go`：拆分测试，分别覆盖 150 与 153 的 invalid index retry。
- 评审后补充：`151_account_autopause_expiry_index_notx.sql` 也使用 `CREATE INDEX CONCURRENTLY IF NOT EXISTS`，已增加 invalid index 预清理和回归测试，避免部署中断后重试留下 invalid index。

## 代码评审处理

- requesting-code-review subagent：指出 OpenAI OAuth handler 测试签名未同步、API key forwarded IP trusted proxy 测试配置不一致。已修复并通过定向测试与 `make test-unit`。
- gstack-review subagent：指出 151 autopause `_notx` 迁移缺少 invalid index retry 预处理。已修复并补 `TestApplyMigrationsFS_AccountAutopauseExpiryIndexMigration_DropsInvalidIndexBeforeRetry`。
- gstack-review subagent 对 reqlog 生产默认开启提出部署前门禁。当前 `reqlog_redis.go` 使用 `RedisConfig` host/port 初始化；已用 `OPS_REQUEST_LOG_ENABLED=true REDIS_HOST=redis REDIS_PORT=6379` 跑过定向 config/repository 测试，compose config 中也确认生产环境提供 `REDIS_HOST=redis`、`REDIS_PORT=6379`。
- 额外收敛：`golangci-lint` 指出的 reqlog depguard/errcheck 问题已修复，handler 测试改用 service store stub，不再直接依赖 repository/redis。

## 测试记录

- `go generate ./ent`：通过，无输出。
- `go test -tags=unit ./internal/repository -run 'TestApplyMigrationsFS_(AccountAutopauseExpiryIndexMigration|AccountGroupSchedulerIndexesMigration|SchedulerOutboxPendingDedupKeyMigration)' -count=1`：通过。
- `go test -tags=unit ./internal/handler/admin ./internal/server/middleware -count=1`：通过。
- `go test -tags=unit ./internal/handler/admin ./internal/service -run 'TestOpsRequestLogHandlerListActiveRequestLogs|TestReqLog' -count=1`：通过。
- `make test-unit`：通过。
- `go test -tags=integration ./...`：未通过，原因是本机 Docker daemon 不可用，testcontainers 初始化失败：`Cannot connect to the Docker daemon at unix:///Users/afreecoder/.docker/run/docker.sock` / `unix:///var/run/docker.sock`。
- `docker info`：失败，确认本机 Docker daemon 不可用。
- `golangci-lint run ./...`：通过，`0 issues`。
- `pnpm --dir frontend run lint:check`：通过。
- `pnpm --dir frontend run typecheck`：通过。
- `pnpm --dir frontend run test:run`：通过，121 files / 741 tests。
- `git tag --merged upstream/main --sort=-version:refname | head -1`：`v0.1.137`。
- `cat backend/cmd/server/VERSION`：`0.1.137`。
- `bash deploy/version_resolver.sh resolve .`：`0.1.137`。
- `POSTGRES_PASSWORD=dummy docker compose -f deploy/docker-compose.deploy.yml config -q`：通过。
- `POSTGRES_PASSWORD=dummy docker compose -f deploy/docker-compose.local.yml config -q`：通过。
- `make build`：通过；前端构建只有既有 Vite chunk / Browserslist 警告。
- `bash scripts/collect_upstream_sync_context.sh --no-fetch`：通过，最终上下文已确认。

## 剩余风险与观察点

- 本机 integration 未完成，唯一已知阻断是 Docker daemon 不可用；需要依赖 CI/服务器环境或本机启动 Docker 后复跑。
- 迁移目录存在两个 `151_*.sql` 文件；runner 按完整文件名排序并用 filename 记录，两个迁移作用域不同，评审未发现顺序依赖风险。
- reqlog 生产默认开启仍建议在部署后重点看应用启动日志、Redis 连接、`/health` 和后台请求明细日志入口。
- 最快回退路径：`cd /opt/sub2api/deploy && ./rollback.sh image`；需要 DB 回退时使用 `rollback.sh db-restore --with-image` 并选择部署前备份。

## 结论

- 建议保留本轮同步结果并推送部署。
- 推送后必须核对 GitHub Actions 部署结果、服务器 commit/version、容器健康、部署前 DB 备份、rollback image 记录和 live `/health`。
