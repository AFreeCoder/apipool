# 上游同步评审记录（2026-04-24）

## 基线

- 当前分支：`main`
- 当前 HEAD：`f14fc531f2ad7dd82c2b2519e05e6a7c6861427a`
- 合入前本地基线：`12e61f70f86429b8a49d8a8afe57121d438cded3`
- 上游引用：`upstream/main`
- 上游 SHA：`d162604f326043e8b9933f68bf214696c78ecf52`
- merge-base：`6449da6c8daf2a443854cf25de96f3a972e3297c`
- 同步方式：`git merge upstream/main`
- upstream 最新 release tag：`v0.1.117`
- upstream/main 版本文件：`0.1.117`
- 本地最终 VERSION：`0.1.117`

## 上游更新摘要

- 本轮从 `upstream/main` 吸收 70 个上游提交，运行时版本从 `0.1.115` 对齐到 `0.1.117`。
- 主要上游变化：
  - Channel Monitor：新增管理端监控、用户端状态页、请求模板、日聚合、历史记录与 SSRF 防护。
  - Available Channels：新增可用渠道聚合视图、平台分区、模型/价格/订阅组展示和开关控制。
  - RPM 限流：新增用户与分组 RPM 配置、缓存和管理端状态能力。
  - OpenAI / Codex：新增 GPT-5.5 默认模型、Codex 生图桥接、图片请求处理修复和 403 处理调整。
  - 支付与计费：修复未知订单 webhook 应答、配额跨越时调度快照入队和支付二维码 fallback。
  - 部署与治理：同步 release workflow、Docker pull tag 通知修正、`VERSION` 到 `0.1.117`。
- 本轮命中的高风险模块：
  - `backend/cmd/server/VERSION`
  - `backend/cmd/server/wire_gen.go`
  - `backend/internal/handler/gateway_handler.go`
  - `backend/internal/handler/handler.go`
  - `backend/internal/handler/wire.go`
  - `backend/internal/service/openai_codex_transform.go`
  - `backend/internal/service/ratelimit_service.go`
  - `backend/internal/service/wire.go`
  - `frontend/src/components/layout/AppSidebar.vue`
  - `frontend/src/views/admin/SettingsView.vue`
  - `scripts/collect_upstream_sync_context.sh`

## 本地定制保护点

- 品牌与文案：
  - `frontend/src/views/admin/SettingsView.vue` 保留默认 `site_name: "APIPool"`。
  - 支付产品名前缀的 placeholder / preview 默认值保留 `APIPool`，避免回退到上游 `Sub2API` 品牌。
- 部署、回滚与版本链路：
  - `backend/cmd/server/VERSION` 最终为 `0.1.117`，与 `upstream/main` 和最新上游 tag `v0.1.117` 对齐。
  - 保留本地 DigitalOcean 部署、备份、镜像回滚和 biz 容器重建链路；未让上游 release workflow 覆盖本地部署约定。
  - `bash deploy/version_resolver.sh resolve .` 返回 `0.1.117`。
- OpenAI OAuth / Codex 兼容逻辑：
  - `openai_codex_transform.go` 同时保留本地 unknown OAuth input item type 日志和上游 Codex image generation bridge。
  - `ratelimit_service.go` 保留 APIPool 对 Cloudflare / `cf-ray` / 终端 OAuth 错误的区分处理，同时吸收上游 OpenAI 403 连续计数逻辑。
  - `gateway_handler.go` 同时保留上游 `select_account_no_available` 日志和本地 `publicGatewayAccountSelectionError` 稳定错误映射。
- 后台入口与默认配置：
  - `AppSidebar.vue` 接受上游 feature flag 架构，同时新增 `purchaseSubscription` 开关，确保 `/purchase` 仍由 `purchase_subscription_enabled` 控制。
  - `/orders` 继续由 `payment_enabled` 控制，避免上游内建 payment 入口接管 APIPool iframe 充值入口。
  - `SettingsView.vue` 接受上游 `default_user_rpm_limit: 0`，同时保留 APIPool 品牌默认值。
- Kiro / OpenClaw 本地扩展：
  - `handler` / `wire` 冲突解决时保留 `KiroOAuth`、`NewKiroAuthService`、OpenClaw 相关前端入口与测试约定。

## 冲突与取舍

- 显式 Git 冲突文件：
  - `backend/cmd/server/wire_gen.go`
  - `backend/internal/handler/gateway_handler.go`
  - `backend/internal/handler/handler.go`
  - `backend/internal/handler/wire.go`
  - `backend/internal/repository/usage_billing_repo_integration_test.go`
  - `backend/internal/service/openai_codex_transform.go`
  - `backend/internal/service/ratelimit_service.go`
  - `backend/internal/service/ratelimit_service_401_test.go`
  - `backend/internal/service/wire.go`
  - `frontend/src/components/layout/AppSidebar.vue`
  - `frontend/src/views/admin/SettingsView.vue`
- 主要冲突处理：
  - `handler.AdminHandlers` / `ProvideAdminHandlers` 同时保留本地 Kiro OAuth 和上游 Channel Monitor / Channel Monitor Template handler。
  - `service.ProviderSet` 保留 `NewKiroAuthService`，吸收上游 provider，并删除重复 `ProvideOAuthRefreshAPI` 定义。
  - `wire_gen.go` 通过 `go generate ./cmd/server` 重生成，避免手工拼接依赖注入结果。
  - `usage_billing_repo_integration_test.go` 同时保留 Kiro quota 测试和上游 scheduler outbox quota crossing 测试。
  - `ratelimit_service_401_test.go` 的测试 stub 同时保留 `lastTempMsg` 与 `lastTempReason` 字段。
- 语义复核过的热点文件：
  - `backend/internal/handler/openai_gateway_handler.go`
  - `backend/internal/service/openai_gateway_service.go`
  - `backend/internal/service/openai_codex_transform_test.go`
  - `backend/internal/service/ratelimit_service_openai_test.go`
  - `frontend/src/router/index.ts`
  - `frontend/src/stores/app.ts`
- post-merge 修复：
  - 首轮 unit test 暴露 `ProvideOAuthRefreshAPI` 重复定义，已删除本地重复 provider 并重新生成 wire。
  - 上游 OpenAI 403 raw-body fallback 测试与 APIPool 临时冷却测试存在契约差异，最终采用“有 counter 时走临时冷却/连续计数，无 counter 时 fallback SetError”的兼容实现，并让 APIPool 临时冷却测试显式注入 counter。
  - `golangci-lint` 暴露 `openAIOAuth403ChallengeCooldownMinutes` 死代码，已删除。
- 评审辅助资产更新：
  - 仓库内 `scripts/collect_upstream_sync_context.sh` 的高风险 pattern 新增 `AvailableChannels`、`ChannelMonitor`、`kiro`、`openclaw`。
  - 技能资产 `/Users/afreecoder/.cc-switch/skills/apipool-sync-upstream/references/local-customizations.md` 已补充 Kiro / OpenClaw 长期定制点。
  - 技能资产 `/Users/afreecoder/.cc-switch/skills/apipool-sync-upstream/references/testing-matrix.md` 已补充 Kiro / OpenClaw 受影响时的专项测试建议。

## 测试记录

- 通过：`cd backend && go generate ./cmd/server`
- 通过：`cd backend && go test -tags=unit ./...`
- 通过：`cd backend && make test-unit`
- 首轮失败但已归因并复跑覆盖：`cd backend && go test -tags=integration ./...`
  - 失败点：`backend/internal/pkg/tlsfingerprint` 的 `TestDialerAgainstCaptureServer` 访问 `https://tls.sub2api.org:8090` 时 TLS handshake EOF。
  - 复核：`cd backend && go test -tags=integration ./internal/pkg/tlsfingerprint -count=1 -run TestDialerAgainstCaptureServer -v` 随后通过，判定为瞬时外部探针/网络问题。
- 通过：`cd backend && make test-integration`
- 通过：`cd backend && golangci-lint run ./...`，输出 `0 issues.`
- 通过：`cd frontend && pnpm install --frozen-lockfile`
- 通过：`cd frontend && pnpm run lint:check`
- 通过：`cd frontend && pnpm run typecheck`
- 通过：`cd frontend && pnpm run test:run`
  - 结果：`92` 个测试文件通过，`542` 个用例通过。
  - 备注：测试过程中存在预期 stderr / Vue warning / 测试日志，最终 exit 0。
- 通过：`git tag --merged upstream/main --sort=-version:refname | head -1`
  - 结果：`v0.1.117`
- 通过：`cat backend/cmd/server/VERSION`
  - 结果：`0.1.117`
- 通过：`make build`
  - 后端构建使用 `-X main.Version=0.1.117`。
  - 前端 Vite build 成功，仅有 chunk size / dynamic import warning。
- 通过：`bash deploy/version_resolver.sh resolve .`
  - 结果：`0.1.117`
- 配置校验：
  - 未带环境变量时，`docker compose -f deploy/docker-compose.deploy.yml config -q` 与 `docker compose -f deploy/docker-compose.local.yml config -q` 均因缺少必填 `POSTGRES_PASSWORD` 失败，符合当前 compose 约束。
  - 通过：`POSTGRES_PASSWORD=dummy docker compose -f deploy/docker-compose.deploy.yml config -q`
  - 通过：`POSTGRES_PASSWORD=dummy docker compose -f deploy/docker-compose.local.yml config -q`
- 通过：`git diff --check HEAD~1..HEAD`
- 通过：`bash scripts/collect_upstream_sync_context.sh --no-fetch`
  - 当前 HEAD：`f14fc531f2ad7dd82c2b2519e05e6a7c6861427a`
  - 上游 SHA：`d162604f326043e8b9933f68bf214696c78ecf52`
  - `latest_upstream_tag`：`v0.1.117`
  - `local_version` / `upstream_version`：`0.1.117` / `0.1.117`
- 未执行：部署后容器版本输出 / 页面版本人工核对
  - 原因：本轮只完成本地 merge、回归与本地构建，尚未推送和触发真实部署。

## 剩余风险与观察点

- Channel Monitor、Available Channels 与 RPM 限流是本轮新增的大块功能，已通过本地回归，但线上仍需重点观察：
  - 新迁移 `125` 到 `129` 的执行结果。
  - 监控 runner 生命周期、批量请求频率、SSRF 防护和 Redis/cache 行为。
  - 用户/分组 RPM 默认值是否影响现有账号池吞吐。
- OpenAI / Codex 路径需观察：
  - Codex 生图桥接是否覆盖真实客户端请求形态。
  - OpenAI 403 连续计数与 APIPool OAuth 终端错误停用逻辑是否符合生产预期。
  - 账号选择失败的公开错误码是否仍被前端/客户端稳定识别。
- 前端入口需人工验收：
  - `/purchase` iframe 充值入口。
  - `/orders` 内建支付订单入口。
  - `/available-channels` 与 `/channel-status` 在开关关闭/开启时的展示。
- 部署前建议：
  - 推送前再次确认 GitHub Actions 自动部署会执行数据库备份和旧镜像保留。
  - 真实部署后核对容器版本、页面展示版本、`/purchase`、`/orders`、Channel Monitor 后台页和用户状态页。

## 结论

- 建议保留当前 merge 结果，并在推送前进入最后部署风险检查。
- 当前结果已经完成：
  - 合入 `upstream/main` 至 `d162604f326043e8b9933f68bf214696c78ecf52`。
  - 运行时版本对齐到 `0.1.117`。
  - 保留 APIPool 品牌、部署回滚链路、Kiro / OpenClaw 扩展、OpenAI OAuth / Codex 兼容和 purchase iframe 入口。
  - 后端 unit / integration / lint、前端 install / lint / typecheck / test、整体 build、compose config 与版本解析校验。
- 当前尚未完成：
  - 尚未推送 `origin/main`。
  - 尚未触发线上部署和部署后版本/入口人工核对。

## 二次评审（Claude Code · 2026-04-24）

**总体：质量很高，建议采纳结论，推送前只需补一两项轻量检查。** 文档中的 SHA、计数、冲突清单与本地定制保护点都与当前 HEAD 状态一致，没有发现夸大或掩盖。

### 已交叉核验的关键声明（全部属实）

- **范围/版本**：`git log 6449da6c..d162604f` 正好 70 个上游提交；`backend/cmd/server/VERSION = 0.1.117`；HEAD 的 merge-base 等于 `upstream/main`，融合完整。
- **Kiro × ChannelMonitor 共存**：`backend/internal/handler/handler.go:20,35-36,49` 与 `backend/internal/handler/wire.go:23,38-39,54,69-70` 中 `KiroOAuth` / `ChannelMonitor` / `ChannelMonitorTemplate` / `ChannelMonitorUserHandler` 全部注册到位，未丢一侧。
- **wire.go 去重**：`grep -c 'func ProvideOAuthRefreshAPI' backend/internal/service/wire.go` 仅 1 次，文档所述"删除重复 provider"属实；`backend/internal/service/wire.go:452-453` 既有 `NewKiroAuthService` 又有 `ProvideOAuthRefreshAPI`。
- **RateLimit 双分支**：`backend/internal/service/ratelimit_service.go:271,726-738,753-789` 同时保留 APIPool 的 `cf-ray` / Cloudflare 临时冷却路径 **和** 上游 `openAI403CounterCache` 连续计数路径；"有 counter → 连续计数/临时冷却，无 counter → fallback SetError" 的兼容切分清晰。
- **Codex transform 双分支**：`backend/internal/service/openai_codex_transform.go:440` 保留本地 `[Codex OAuth] Unknown input item type preserved…` 日志；`backend/internal/service/openai_codex_transform.go:54-55` 同时吸收上游 `codexImageGenerationBridgeMarker` 生图桥接。
- **Gateway 错误映射**：`backend/internal/handler/gateway_handler.go:307-313,543-550,1495` 两处 `select_account_no_available` 日志与 `publicGatewayAccountSelectionError` 并存。
- **品牌兜底**：`frontend/src/views/admin/SettingsView.vue:3259,3995,4017,5058` 默认 `site_name: "APIPool"` + `payment_product_name_prefix` 占位/预览全部是 `APIPool`，没有被上游 Sub2API 默认覆盖。
- **入口开关语义**：`frontend/src/utils/featureFlags.ts:96-117` 显示 `/purchase` 走 `purchase_subscription_enabled` (opt-in)、`/orders` 走 `payment_enabled` (opt-out)；`frontend/src/components/layout/AppSidebar.vue:657-658` 的绑定与之一致，iframe 充值入口的本地语义被保留。
- **本地部署链路未受上游污染**：merge 对 `.github/workflows/release.yml` 的全部改动只有 4 行，是 `TAG_NAME → VERSION` 的 docker pull 消息文案修复；本地 `.github/workflows/deploy.yml`（DO 部署）未被触碰，"未让上游 release workflow 覆盖本地部署约定"属实。
- **测试 stub 补齐**：`backend/internal/service/ratelimit_service_401_test.go:23-24,35-36,230-271` 同时含 `lastTempMsg` / `lastTempReason`，Cloudflare / cf-ray 用例保留。
- **辅助资产**：`scripts/collect_upstream_sync_context.sh:46` 的 `HIGH_RISK_PATTERN` 已新增 `AvailableChannels`、`ChannelMonitor`、`kiro`、`openclaw`。
- **编译健康**：本地补跑 `go vet ./...` 通过（exit 0）。

### 值得肯定的质量特征

1. **冲突处理是语义级而非机械级**：OpenAI 403 新旧契约的兼容切分（counter 可选依赖 + fallback）、`wire_gen.go` 走 `go generate` 重生成而非手改、死代码 `openAIOAuth403ChallengeCooldownMinutes` 被 lint 暴露后清除——这三处都展示了真正的根因排查。
2. **长期定制有沉淀到技能资产**：`apipool-sync-upstream` 的 `local-customizations.md` / `testing-matrix.md` 被同步更新，下一轮合并会更快。
3. **对"未完成"坦诚**：明确列出"未推送、未部署、未做部署后人工核对"，没有用成功的本地构建冒充线上验证。

### 三个可以改进的点

1. **外部 TLS 探针的脆弱性需要单独登账**：集成测试 `TestDialerAgainstCaptureServer` 依赖 `tls.sub2api.org:8090`，本轮首次失败被归因为"瞬时"。这是上游控制的域名，任何时点下线都会阻断我们自己的 CI——建议后续评估：要么 `skip-if-unreachable`，要么本地起 capture server，否则每次同步都会在这里吃一发假阳性。
2. **前端测试的 Vue/stderr warning 没有被量化**：`pnpm run test:run` 通过 542 条用例但"存在预期 stderr / Vue warning"。本轮改动触碰了 Channel Monitor / Available Channels / RPM / Purchase 四个前端入口，推送前建议 `pnpm run test:run 2>&1 | grep -iE 'warn|error' | wc -l` 做一次数量基线对比，避免淹没新回归。
3. **迁移 125→129 的运行顺序没有记录**：剩余风险里提到了这一批新迁移，但没有记录实际 `migrate up` 的 dry-run 结果或字段名清单。这是部署前最容易出事的一步，值得补一条"上线前验证"checklist 条目（例如在 staging 上先跑 `atlas migrate apply --dry-run` 或等价命令）。

### 二次评审结论

**可以按原文档的建议保留这份 merge 结果。** 推送前最小增量：

1. 执行一次新前端 warning 数量基线比对；
2. 对 migration 125→129 做一次 staging dry-run；
3. 对 `tls.sub2api.org:8090` 探针的长期处置方案单独开 issue。

然后再进入 GitHub Actions 自动部署 + 部署后人工核对的流程。

## 二次评审处理记录（Codex · 2026-04-24）

- 评审意见复核结论：
  - 三个改进点方向成立，适合作为推送/部署前补充检查。
  - 第 3 点中提到的 `atlas migrate apply --dry-run` 不匹配当前仓库实际迁移链路；当前项目使用 `backend/internal/repository/migrations_runner.go` 的自定义 runner，按嵌入 SQL 文件名字典序执行，并记录到 `schema_migrations`。
- 已补前端 warning/error 数量基线：
  - 命令：`cd frontend && pnpm run test:run`
  - 结果：exit 0，`Test Files 92 passed (92)`，`Tests 542 passed (542)`。
  - `grep -iE 'warn|error'` 命中 `20` 行。
  - 命中样例主要是既有测试期望中的 `router-link` Vue warn、无效 JSON、网络错误、表格加载错误和 dashboard mock 缺方法日志。
- 已补迁移 runner 本地 clean DB 级别验证：
  - 命令：`cd backend && go test -tags=integration ./internal/repository -run 'TestMigrationsRunner' -count=1 -v`
  - 结果：exit 0。
  - 该测试通过 testcontainers 启动 PostgreSQL / Redis，启动时执行 `ApplyMigrations`，并在 `TestMigrationsRunner_IsIdempotent_AndSchemaIsUpToDate` 中再次执行 `ApplyMigrations` 验证幂等性。
- 125→129 迁移执行顺序按文件名字典序为：
  - `125_add_channel_monitors.sql`
  - `125_add_group_rpm_limit.sql`
  - `126_add_channel_monitor_aggregation.sql`
  - `126_add_user_rpm_limit.sql`
  - `127_add_user_group_rpm_override.sql`
  - `127_drop_channel_monitor_deleted_at.sql`
  - `128_add_channel_monitor_request_templates.sql`
  - `129_seed_claude_code_template.sql`
- 125→129 关键 schema 影响：
  - 新增 `channel_monitors`、`channel_monitor_histories`、`channel_monitor_daily_rollups`、`channel_monitor_aggregation_watermark`、`channel_monitor_request_templates`。
  - 新增 `groups.rpm_limit`、`users.rpm_limit`、`user_group_rate_multipliers.rpm_override`。
  - `channel_monitor_histories` 和 `channel_monitor_daily_rollups` 曾在 `126` 加入 `deleted_at`，随后 `127_drop_channel_monitor_deleted_at.sql` 删除，对应上游最终状态。
  - `channel_monitors` 在 `128` 增加 request template 相关字段与索引，并由 `129` seed Claude Code 模板。
- 已补静态检查：
  - 命令：`cd backend && go vet ./...`
  - 结果：exit 0。
- 尚未执行的外部动作：
  - 未执行 staging migration dry-run：本轮没有 staging DSN/凭据，且仓库没有现成 atlas dry-run 入口；上线前应在 staging 或部署预检中用真实 runner 跑一次。
  - 未创建 GitHub issue：这是外部副作用，本轮只在评审文档中登记；推送部署前建议单独登记 `tls.sub2api.org:8090` 探针长期处置方案。
