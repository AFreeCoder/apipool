# 上游同步评审记录（2026-06-10）

## 基线

- 当前分支：`main`
- 合入提交：`48345d41baf9`
- 合入前本地基线：`c5bc1564353901dc7bed386e5759ee39610e1de2`
- 上游引用：`upstream/main`
- 上游 SHA：`e34ad2b19424844cbd1bcffb599e5b0002ad3c50`
- merge-base：`0aad6030130c02cadb8b70e6cc90c9ed04bb1a7a`
- 同步方式：`git merge upstream/main`
- upstream 最新 release tag：`v0.1.136`
- upstream/main 版本文件：`0.1.136`
- 本地最终 VERSION：`0.1.136`

## 上游更新摘要

- 合入 `upstream/main` 到 `e34ad2b19424844cbd1bcffb599e5b0002ad3c50`，最新 upstream release tag 为 `v0.1.136`。
- 吸收后台合规确认门：新增 admin compliance API、中间件、前端确认弹窗、法律文档入口和状态 store。
- 吸收 `/admin/users` 按用户 API Key 所在分组过滤能力，以及对应后端 repository/query、前端筛选项和测试。
- 吸收 OpenAI / gateway 侧修复：prompt cache key 透传、failover 时模型 body 替换预计算、非流式错误 double-write 防护、响应已提交标记覆盖更多平台。
- 吸收 Bedrock Claude Code compat 修复、Claude Fable 5 / Antigravity 模型映射、idempotency UTF-8 截断修复、调度日志循环和账号分组调度索引优化。
- 吸收 upstream README / 赞助商资产变化；APIPool 侧保留自有 README 语义，仅接入合规/免责声明提示。
- 命中的高风险模块：`backend/cmd/server/VERSION`、`backend/internal/handler/openai_gateway_handler.go`、`backend/internal/service/openai_gateway_service.go`、`README.md`、`README_CN.md`、`README_JA.md`。

## 本地定制保护点

- 品牌与文案：`README.md` 保留 APIPool 项目说明、生产部署口径、本地开发口径和 iframe 充值入口说明；未接受 upstream 将主 README 拉回 Sub2API 英文赞助商页。
- 文档债务：接受 upstream 删除 `README_CN.md`，因为当前 `README.md` 已是 APIPool 中文主文档，继续保留旧中文 Sub2API README 会扩大品牌债务。
- 部署/回滚/版本链路：本轮未改 `.github/workflows/deploy.yml`、`deploy/rollback.sh`、`deploy/docker-compose.deploy.yml`、`deploy/version_resolver.sh`；`backend/cmd/server/VERSION` 已随 upstream 对齐到 `0.1.136`。
- OpenAI OAuth / Codex 兼容：保留 APIPool 既有流式终止事件保护和 Codex `response.failed` fallback，同时吸收 upstream `ResponseCommitted` double-write 防护。
- Kiro / OpenClaw 扩展：`wire_gen.go` 冲突中同时保留本地 `kiroOAuthHandler` 与 upstream `complianceHandler`，避免 Kiro OAuth 路由在无 Git 冲突的依赖注入层被移除。
- 后台入口与默认配置：保留 `/purchase` iframe 充值入口、表格分页默认值、OpenClaw 配置导入、Codex GPT-5.5 默认配置；吸收用户 API Key 分组过滤和 admin compliance UI。

## 冲突与取舍

- `README.md`：以 APIPool 本地文档为基底，加入 upstream 合规/免责声明提示；移除指向已删除 `README_CN.md` 的语言链接。
- `README_CN.md`：接受 upstream 删除，避免继续发布旧 Sub2API 中文说明。
- `backend/cmd/server/wire_gen.go`：运行 `go generate ./cmd/server` 后确认依赖注入同时包含 `kiroOAuthHandler` 和 `complianceHandler`。
- `backend/internal/handler/openai_gateway_handler.go`：保留 `HasOpenAIStreamTerminalEventForwarded` 检查，同时加入 upstream `IsResponseCommitted` 检查；避免已发终止事件或已写错误响应后再次兜底写入。
- `frontend/src/components/keys/__tests__/UseKeyModal.spec.ts`：同时保留本地 Codex GPT-5.5 默认配置断言和 upstream Claude Fable 5 OpenCode 配置断言。
- 语义复核：精确扫描 `^<<<<<<<|^=======|^>>>>>>>` 无 merge marker。
- 语义复核：`scripts/collect_upstream_sync_context.sh --no-fetch` 确认 upstream tag、upstream VERSION、本地 VERSION 均为 `0.1.136`。

## 测试记录

- 通过：`cd backend && go generate ./cmd/server`。
- 通过：`cd backend && go test -tags=unit ./...`。
- 通过：`cd backend && make test-unit`。
- 环境阻塞：`cd backend && go test -tags=integration ./...` 失败于本机 Docker daemon 不可用，testcontainers 无法连接 `unix:///Users/afreecoder/.docker/run/docker.sock`；失败包为 `internal/middleware` 与 `internal/server/routes` 的 Redis/testcontainers integration 用例。
- 环境阻塞：`cd backend && make test-integration` 同样失败于本机 Docker daemon 不可用。
- 通过：`cd backend && golangci-lint run ./...`，输出 `0 issues`。
- 通过：`pnpm --dir frontend run lint:check`。
- 通过：`pnpm --dir frontend run typecheck`。
- 通过：`pnpm --dir frontend run test:run`，121 个测试文件、740 个用例通过；输出包含既有预期错误场景 stderr、Element Plus stub warning、Browserslist 过期提示和 i18n compiler warning。
- 通过：`pnpm --dir frontend run build`，存在既有 dynamic import / chunk size warning。
- 通过：`git tag --merged upstream/main --sort=-version:refname | head -1` 输出 `v0.1.136`。
- 通过：`cat backend/cmd/server/VERSION` 输出 `0.1.136`。
- 通过：`cd backend && make build`，Go build 使用 `main.Version=0.1.136`。
- 通过：`POSTGRES_PASSWORD=dummy docker compose -f deploy/docker-compose.deploy.yml config -q`。
- 通过：`POSTGRES_PASSWORD=dummy docker compose -f deploy/docker-compose.local.yml config -q`。
- 通过：`bash deploy/version_resolver.sh resolve .` 输出 `0.1.136`。
- 通过：`bash scripts/collect_upstream_sync_context.sh --no-fetch`，确认 upstream tag/version/local version 均为 `0.1.136`。
- 待部署后核对：线上 `docker exec sub2api /app/sub2api --version` 或等价版本输出、页面左上角版本、容器健康和备份/回滚元数据。

## Subagent 评审与接收记录

- `requesting-code-review` subagent 结论：无 Critical / Important blocker；指出首次 admin compliance 确认后，已失败的后台数据请求不会自动重试。该问题成立但属于后续 UX 优化，不阻塞本轮同步部署。
- `gstack review` subagent P1：`150_account_group_scheduler_indexes_notx.sql` 使用 `CREATE INDEX CONCURRENTLY IF NOT EXISTS`，若并发建索引中断可能留下同名 invalid index，后续重跑会被 `IF NOT EXISTS` 跳过。已采纳并修复：`migrations_runner.go` 在执行该 non-transactional migration 前检查并 `DROP INDEX CONCURRENTLY IF EXISTS` 两个目标 invalid index，新增 sqlmock 单元测试覆盖重试路径。
- `gstack review` subagent P2：admin compliance gate 的文档 URL、确认短语和法律文档主体仍指向 upstream `Wei-Shaw/sub2api` / `Sub2API`，与 APIPool 自部署实例的品牌和责任记录不一致。已采纳并修复：后端 status、前端 fallback URL/phrase、弹窗链接和中英文法律文档主体统一为 APIPool，并保留“基于 Sub2API 开源软件”的来源说明。

## 接收评审后补充验证

- 通过：`cd backend && go test -tags=unit ./internal/repository -run 'AccountGroupScheduler|NonTransactionalMigration|PaymentOrdersOutTradeNoUnique'`。
- 通过：`cd backend && go test -tags=unit ./internal/service -run AdminCompliance`。
- 通过：`pnpm --dir frontend run typecheck`。
- 通过：`pnpm --dir frontend run test:run -- src/api/__tests__/client.spec.ts`，实际执行全量 Vitest，121 个测试文件、740 个用例通过；输出包含既有预期错误场景 stderr、Element Plus stub warning、Browserslist 过期提示和 i18n compiler warning。
- 通过：`git diff --check`。

## 部署构建修复记录

- 首次推送触发的 GitHub Actions run `27262263413` 在 Docker `frontend-builder` 阶段失败，错误为 `Could not resolve "../../../../docs/legal/admin-compliance.zh.md?raw"`；SSH 部署步骤未执行，线上未变更。
- 根因：Dockerfile 仅复制 `frontend/` 到 `/app/frontend`，而 `AdminComplianceDialog.vue` 与 `LegalDocumentView.vue` 从 `/app/docs/legal` 读取合规 Markdown；同时 `.dockerignore` 排除了整个 `docs/` 目录。
- 已修复：`.dockerignore` 仅放行 `docs/legal/admin-compliance.*.md`，Dockerfile 在 frontend build 前复制 `docs/legal/` 到 `/app/docs/legal/`。
- 通过：本机 Docker daemon 不可用时，在临时目录模拟 Docker `/app/frontend` + `/app/docs/legal` 布局并执行 `pnpm install --frozen-lockfile`、`pnpm run build`，确认 raw import 可以解析。

## 剩余风险与观察点

- 本地 integration 未完成，原因是本机 Docker daemon 不可用；需要以后在 Docker 可用环境或 CI/部署环境继续以 integration 结果补强。
- 本轮新增 admin compliance gate，部署后需确认管理员页面首次访问的确认弹窗不会阻断正常后台操作，确认后状态可持久化。
- 本轮触及 OpenAI / gateway 错误透传和 response committed 标记，部署后重点观察 `/v1/responses`、非流式上游错误、failover 后模型 body 替换和 Codex 客户端报错率。
- 本轮新增用户列表按 API Key 分组过滤，部署后可在后台用户管理页抽查筛选项是否正常加载、筛选结果是否符合预期。
- 快速回滚建议仍是先走镜像回滚：`cd /opt/sub2api/deploy && ./rollback.sh image`。仅当确认数据迁移/持久状态异常时才考虑 `db-restore --with-image`。

## 结论

- 已完成上游 `v0.1.136` 同步、双 subagent review、receiving-code-review 接收、Docker 构建修复与本地可验证检查。代码层面已通过 unit、lint、前端 lint/typecheck/Vitest/build、后端 build、compose config、版本解析和接收评审后的针对性验证；未完成项限定在本机 Docker 环境阻塞的 integration。建议重新触发远程部署。
