# 上游同步评审记录（2026-05-20）

## 基线

- 当前分支：`codex/upstream-sync-20260520`
- 合入提交：`13cf7e94a3d98135eb214db8d802b36ff2adcdcd`
- 合入前本地基线：`03ad66f7365da60fbdf2c2d1208fd007314b3035`
- 上游引用：`upstream/main`
- 上游 SHA：`7ec61eb2f5d4d7c182ca3e465dfe0c4f806a0ad8`
- merge-base：`62ccd0ff39d1d2dc80a6616adbd20a54d5f264f2`
- 同步方式：`git merge upstream/main`
- upstream 最新 release tag：`v0.1.127`
- upstream/main 版本文件：`0.1.127`
- 本地最终 VERSION：`0.1.127`

## 上游更新摘要

- 吸收 upstream `v0.1.127`，版本文件从 `0.1.126` 对齐到 `0.1.127`；`upstream/main` 的最新 release tag、上游 `VERSION` 与本地最终 `VERSION` 三者一致。
- 新增钉钉 OAuth 登录链路，包含后端配置校验、OAuth client/callback、`internal_only` 用户属性同步、前端登录入口与回调页。
- 新增通知邮件与邮件模板管理能力，覆盖余额提醒、订阅提醒、支付成功通知、退订入口、后台模板编辑器与多语言初始化。
- 新增 Channel Monitor OpenAI API 模式，监控与模板均可持久化 API mode，并补内置 OpenAI 检测模板。
- 新增上游模型同步能力、用户用量按平台拆分、兑换码有效期、图片生成 `n` / `size` 计费元数据、内容审计关键词拦截、OpenAI silent refusal failover、Responses 到 Chat Completions fallback、Codex OAuth 浏览器 User-Agent 自动改写。
- 吸收 upstream 对 ops retry replay 的移除、Ops SLA/错误分类调整、敏感 credentials redaction、支付二维码/移动端支付修复、安装脚本 Bash 版本检查、Docker 前端 builder pnpm pinning。
- 命中的高风险模块包括：`backend/internal/service/openai_gateway_service.go`、`backend/internal/service/openai_codex_transform.go`、`backend/internal/service/setting_service.go`、`backend/internal/handler/admin/setting_handler.go`、`backend/internal/service/channel_monitor_checker.go`、`backend/internal/handler/gateway_handler*.go`、`backend/internal/handler/openai_gateway_handler.go`、`frontend/src/views/admin/SettingsView.vue`、`frontend/src/components/account/EditAccountModal.vue`、`frontend/src/components/account/ModelWhitelistSelector.vue`、`deploy/docker-compose.yml`、`backend/cmd/server/VERSION`。

## 本地定制保护点

- 品牌与文案：`site_name` 默认仍为 `APIPool`；主 `README.md` 保留 APIPool 自定义说明，没有接受 upstream sponsors/英文介绍覆盖；`README_CN.md` / `README_JA.md` 跟随 upstream sponsors 更新。
- 购买入口：`purchase_subscription_enabled` / `purchase_subscription_url` 仍贯通公开设置、后台设置和前端 `/purchase` iframe 购买入口；upstream 内建支付修复未接管 APIPool 的外部订阅购买入口。
- 跑马灯：`marquee_enabled` / `marquee_messages` 后端设置、公开设置和前端展示链路仍保留。
- Kiro / OpenClaw：`KiroTokenProvider`、Kiro OAuth onboarding、账号测试、刷新协议、前端 Kiro 表单与 OpenClaw 配置导入仍存在；Wire 注入时同时保留 Kiro 和 upstream 新增 notification email / admin setting handler。
- OpenAI / Codex 兼容：保留 APIPool 的公共错误码、脱敏账号选择错误文案、Codex/OpenAI OAuth 兼容、fallback 版本与 User-Agent 改写，同时吸收 upstream 的 ops capacity marker 与 Responses fallback 修复。
- 部署/回滚/版本链路：`backend/cmd/server/VERSION` 为 `0.1.127`；`deploy/version_resolver.sh resolve .` 输出 `0.1.127`；DigitalOcean/Compose 相关部署与回滚文件保留本地语义。

## 冲突与取舍

- `README.md`：保留 APIPool 主 README，避免 upstream sponsors/项目介绍覆盖本地品牌与使用说明；`README_CN.md` / `README_JA.md` 自动吸收 upstream sponsors 更新。
- `backend/internal/handler/wire.go`、`backend/internal/service/wire.go`、`backend/cmd/server/wire_gen.go`：合并 Wire 注入，保留 `KiroTokenProvider`，同时接入 upstream `NotificationEmailService` 与 `ProvideAdminSettingHandler`。
- `backend/internal/handler/admin/setting_handler.go`、`backend/internal/service/setting_service.go`、`backend/internal/handler/dto/settings.go`：合并 upstream 钉钉 OAuth、邮件模板、通知邮件、OpenAI Codex UA、上游模型同步等设置字段，并保留 APIPool purchase iframe、跑马灯和本地默认值。
- `backend/internal/handler/gateway_handler.go`、`backend/internal/handler/gateway_handler_chat_completions.go`、`backend/internal/handler/gateway_handler_responses.go`、`backend/internal/handler/openai_gateway_handler.go`、`backend/internal/handler/gemini_v1beta_handler.go`：加入 upstream ops capacity marker，但响应继续走 APIPool 的公开错误码与脱敏 helper。
- `backend/internal/service/channel_monitor_checker.go`：同包 helper `stringFromAny` 与 Kiro 本地 helper 重名；为保留 Kiro 更强的结构化 JSON 转字符串逻辑，将 channel monitor helper 重命名为 `channelMonitorStringFromAny`。
- `backend/internal/service/openai_gateway_service.go`：`OpenAIUsageResult` 同时保留 APIPool `UpstreamErrorEvent` 与 upstream 图片生成相关字段。
- `frontend/vitest.config.ts`：`setupFiles` 合并为 `['src/test/setup.ts', './src/__tests__/setup.ts']`，同时保留 upstream 与本地测试初始化。
- `frontend/src/components/account/ModelWhitelistSelector.vue`、`frontend/src/components/account/EditAccountModal.vue`、`frontend/src/components/account/__tests__/EditAccountModal.spec.ts`：同时保留 APIPool OpenAI snapshot aliases / Kiro 分支与 upstream model restriction / sync state 变更。
- 测试修复：`NewAccountTestService`、`NewGatewayService` 等测试构造器补齐新增依赖；`api_contract_test` 增加 `dingtalk_oauth_enabled`；OpenAI plan sync 测试按 upstream credentials redaction 调整为检查 `credentials_status.has_access_token`。

## 测试记录

- `git diff --cached --check`：通过。
- `git tag --merged upstream/main --sort=-version:refname | head -1`：`v0.1.127`。
- `git show upstream/main:backend/cmd/server/VERSION`：`0.1.127`。
- `cat backend/cmd/server/VERSION`：`0.1.127`。
- `go test -tags=unit ./...`：通过。
- `golangci-lint run ./...`：通过，`0 issues`。
- `pnpm --dir frontend install --frozen-lockfile`：通过；pnpm 输出版本升级提示和 ignored build scripts warning，不影响结果。
- `pnpm --dir frontend run lint:check`：通过。
- `pnpm --dir frontend run typecheck`：通过。
- `pnpm --dir frontend run test:run`：通过，`106 passed (106)` test files，`642 passed (642)` tests；有若干测试预期 stderr/warnings。
- `go test -tags=integration ./...`：第一次 Docker daemon 未运行；启动 Docker Desktop 后全量重跑时 `internal/server/routes` testcontainers/Ryuk 瞬时失败；单独重跑 `go test -tags=integration ./internal/server/routes` 通过，再次全量 `go test -tags=integration ./...` 通过。
- `POSTGRES_PASSWORD=dummy docker compose -f deploy/docker-compose.deploy.yml config -q`：通过。
- `POSTGRES_PASSWORD=dummy docker compose -f deploy/docker-compose.local.yml config -q`：通过。
- `bash deploy/version_resolver.sh resolve .`：`0.1.127`。
- `make build`：通过；Vite 输出动态导入/chunk size warning，无构建失败。
- `scripts/collect_upstream_sync_context.sh --no-fetch`：通过；merge 后 `head=13cf7e94a3d98135eb214db8d802b36ff2adcdcd`，`local_version=0.1.127`，`upstream_version=0.1.127`。
- `make test-unit` / `make test-integration`：未单独执行；本轮直接执行对应的 `go test -tags=unit ./...` 与 `go test -tags=integration ./...`。
- 部署后版本输出 / 页面版本人工核对：本轮未部署生产，未执行线上版本核对。

## 剩余风险与观察点

- 钉钉 OAuth、通知邮件、邮件模板、channel monitor API mode、上游模型同步、内容审计关键词、兑换码有效期、图片生成计费、OpenAI Codex UA 改写等 upstream 新能力已通过自动化测试/构建覆盖，但未做生产端到端验证。
- `ops_retry` replay 存储与控制已跟随 upstream 移除；上线后需要重点观察 Ops 清理、告警聚合和历史队列相关后台页面是否符合预期。
- credentials redaction 会改变管理端账号接口返回形态；本轮已调整测试，但生产上若有外部脚本直接依赖明文 `access_token` 响应，需要提前改造。
- Docker integration 测试起步依赖 Docker Desktop 与 testcontainers/Ryuk；本轮环境启动后最终通过，但 CI/本机若 Docker 不可用仍会失败。
- 本轮只完成临时 worktree 分支同步与本地验证，未 push、未部署；如发布生产，仍需使用 `apipool-push-deploy` 补齐 GitHub Actions、live commit、容器健康、运行时版本、备份和回滚元数据核验。

## 结论

- 建议保留当前 upstream sync 结果。`upstream/main` 已合入到临时分支 `codex/upstream-sync-20260520`，版本链路一致；APIPool 品牌、购买入口、跑马灯、Kiro/OpenClaw、OpenAI/Codex 错误语义和部署配置均已复核并保留。当前剩余风险主要集中在 upstream 新能力的生产端到端验证与后续发布闭环。
