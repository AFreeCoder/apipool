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
