# 上游同步评审记录（2026-05-04）

## 基线

- 当前分支：`main`
- 当前 HEAD：`ff495ff7ddcf2989da2087cac4ee5fe221f0ca15`
- 合入前本地基线：`e62bf1ba4fabc304d472b64210f5592cbf6af240`
- 上游引用：`upstream/main`
- 上游 SHA：`df722c9a6e97312491232c11bf305d5f93b45e04`
- merge-base：`8ad099baa6057f0dfed32ded1f04fc5ea5a38717`
- 同步方式：`git merge upstream/main`
- upstream 最新 release tag：`v0.1.122`
- upstream/main 版本文件：`0.1.122`
- 本地最终 VERSION：`0.1.122`

## 上游更新摘要

- 吸收 `upstream/main` 从 `8ad099ba` 到 `df722c9a` 的 31 个提交，版本从 `0.1.120` 对齐到 `0.1.122`。
- OpenAI 网关：引入 APIKey 上游 Responses 能力探测、对不支持 Responses 的兼容上游直转 `/v1/chat/completions`、raw CC usage 提取、流式断连后继续 drain 以保留计费、WS passthrough usage log 的 User-Agent / reasoning effort 记录、零 usage log 保留、Codex 模型归一化更新。
- 管理后台与业务：新增 affiliate 管理记录页、修正返利审计来源、成熟 quota 统计、余额历史展示；新增 Anthropic cache TTL 1h 注入开关；新增 migration `134_affiliate_ledger_audit_snapshots.sql`。
- 高风险模块命中：`backend/internal/handler/openai_gateway_handler.go`、`backend/internal/service/openai_*`、`backend/internal/service/openai_codex_transform.go`、`frontend/src/components/layout/AppSidebar.vue`、`frontend/src/views/admin/SettingsView.vue`、`backend/cmd/server/VERSION`。

## 本地定制保护点

- 品牌与文档：APIPool 本地文档、帮助入口、`frontend/src/docs/`、`frontend/src/views/docs/`、品牌资源未被上游删除覆盖。
- 部署/回滚/版本链路：`.github/workflows/deploy.yml` 的 GHCR pull deploy、`deploy/rollback.sh`、`deploy/version_resolver.sh`、biz compose 文件仍保留；`backend/cmd/server/VERSION` 与 upstream tag 均为 `0.1.122`，本轮不需要额外 `chore(version)` 提交。
- OpenAI / Codex：保留 APIPool 的精确账号选择错误语义；Chat Completions 接受上游“移除未知模型 fallback”方向，但仍对 group 不支持模型返回 `model_not_supported_in_group`，不塌缩为泛化可用性错误。
- Kiro / OpenClaw：`kiro_*` service、`gateway_service_kiro.go`、Kiro OAuth handler/routes、前端 Kiro OAuth 和 OpenClaw 配置导入文件仍保留，并已纳入 wire。
- 后台入口与默认配置：`/purchase` iframe 充值入口和 `purchase_subscription_enabled` / `purchase_subscription_url` 保留；上游 `/orders` 内建支付入口可并存但未接管 purchase；表格分页默认值继续由后台 `table_default_page_size` / `table_page_size_options` 统一控制，不使用全局 localStorage 覆盖。

## 冲突与取舍

- `backend/internal/handler/openai_chat_completions.go`：移除 Chat Completions default model fallback，接受上游 raw CC endpoint usage 记录；保留 `publicOpenAIAccountSelectionError` 精确错误码返回。
- `backend/internal/handler/openai_gateway_handler_test.go`：保留 APIPool 账号选择错误测试；删除已失效的 `resolveOpenAIForwardDefaultMappedModel` 测试；接受上游 WS passthrough usage log 新测试。
- `frontend/src/api/admin/settings.ts`：同时保留 APIPool purchase 字段和上游新增 `enable_anthropic_cache_ttl_1h_injection` 字段。
- 无 Git 冲突但人工复核：Kiro wire/routes、GHCR deploy/rollback、`AppSidebar` purchase/orders 并存、`SettingsView` purchase + Anthropic TTL 表单、`usePersistedPageSize` 表格默认值语义。
- 发现并修正语义回退：上游恢复 `table-page-size` localStorage 持久化会覆盖 APIPool 后台统一默认值；本轮已恢复系统配置优先，并更新 upstream sync 参考资产。

## 测试记录

- 通过：`cd backend && go test -tags=unit ./internal/handler ./internal/service ./internal/pkg/openai_compat`
- 通过：`cd backend && go test -tags=unit ./...`
- 环境失败：`cd backend && go test -tags=integration ./...` 在 `internal/server/routes.TestAuthRegisterRateLimitThresholdHitReturns429` 因 testcontainers 无法连接 Docker daemon 失败；`docker info` 同样确认 `unix:///var/run/docker.sock` 不可用。失败前后其他已执行 integration 包通过，未见代码断言失败。
- 未单独执行：`cd backend && make test-unit`、`cd backend && make test-integration`；已执行等价底层 `go test` 命令，其中 integration 受 Docker 环境限制。
- 通过：`cd backend && golangci-lint run ./...`，输出 `0 issues.`
- 通过：`pnpm --dir frontend run lint:check`
- 通过：`pnpm --dir frontend run typecheck`
- 通过：`pnpm --dir frontend run test:run`，最终 `97 passed (97)`、`569 passed (569)`；首次运行暴露 `usePersistedPageSize` 语义回退，修复后全量通过。
- 通过：`git tag --merged upstream/main --sort=-version:refname | head -1` 输出 `v0.1.122`
- 通过：`cat backend/cmd/server/VERSION` 输出 `0.1.122`
- 通过：`make build`，构建版本参数为 `main.Version=0.1.122`；Vite 仅输出既有大 chunk / dynamic import 静态混用警告。
- 通过：`POSTGRES_PASSWORD=dummy docker compose -f deploy/docker-compose.deploy.yml config -q`
- 通过：`POSTGRES_PASSWORD=dummy docker compose -f deploy/docker-compose.local.yml config -q`
- 通过：`bash deploy/version_resolver.sh resolve .` 输出 `0.1.122`
- 通过：`bash scripts/collect_upstream_sync_context.sh --no-fetch`
- 未执行：部署后线上版本输出 / 页面版本人工核对，本轮仅完成本地合入与验证，尚未 push/deploy。

## 剩余风险与观察点

- integration 需要 Docker daemon 后重跑，尤其是 `internal/server/routes` 的 Redis/testcontainers 路由限流用例。
- 上游 raw Chat Completions 直转改变了 APIKey 兼容上游的实际 upstream endpoint，应在发布后关注 `ops_error_logs` 中 OpenAI APIKey 账号的 usage、failover、raw CC upstream endpoint 记录是否符合预期。
- affiliate 管理记录页和 migration `134` 涉及返利审计数据，发布后需要观察迁移执行结果和 admin affiliate 页面查询。
- 版本链路已对齐到 `0.1.122`；若后续 push/deploy，应按 `apipool-push-deploy` 继续确认 Actions、GHCR 镜像、容器健康、备份与 rollback metadata。

## 结论

- 建议保留当前合入结果。本轮接受上游 `v0.1.122` 的网关、affiliate、设置项与版本更新，同时保住 APIPool 的 Kiro/OpenClaw、GHCR deploy、purchase iframe、精确错误语义和表格默认值定制。唯一未完全验证项是 Docker 依赖的 integration 测试。
