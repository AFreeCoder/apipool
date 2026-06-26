# 上游同步评审记录（2026-06-27）

## 基线

- 当前分支：`main`
- 合入前本地基线：`3917f6841acdf2a5ebbb31b4da38eceaa9fcd11e`
- 上游引用：`upstream/main`
- 上游 SHA：`c275422251e72750bebe53e41fcf59db7f83fe6b`
- merge-base：`d430343f513aa811be4ef949a945d3d69e3dd0df`
- 同步方式：`git merge upstream/main`
- upstream 最新 release tag：`v0.1.139`
- upstream/main 版本文件：`0.1.139`
- 本地最终 VERSION：`0.1.139`

## 上游更新摘要

- 吸收 upstream `v0.1.139`：Grok/xAI OAuth、quota probe、OpenAI gateway/model availability、Codex PAT/allow-list hardening、payment/currency 展示、Antigravity project ID、usage/billing 统计和多处后台 UI 改进。
- 高风险重叠集中在 OpenAI/Codex gateway、账号调度与 token cache、后台账号创建/编辑 OAuth 流程、SettingsView、计费配置、README/部署文档。
- 同步后保留 APIPool 本地 Kiro/OpenClaw、gpt-image-2 计费展示、请求日志 capture、后台默认分页、部署/回滚说明等定制。

## 本地定制保护点

- 品牌与文档：`README.md` 继续使用 APIPool 语境，补入 Grok/xAI 与 Antigravity 说明；未回退为 upstream Sub2API 默认首页。
- 版本链路：`backend/cmd/server/VERSION` 已升到 `0.1.139`，`make build` 后后端使用 `-X main.Version=0.1.139`。
- 部署配置：`deploy/config.example.yaml` 同时保留本地 `billing.gpt_image_2_token_billing_enabled` 和 upstream `billing.minimum_balance_reserve`。
- OpenAI/Codex：保留本地稳定错误码、runtime block fastpath、response failed terminal handling、Codex CLI/App Server allow-list 语义，并吸收 upstream 404 `model_not_found` 与 function call arguments normalization。
- 账号平台：Kiro/OpenClaw 与 Grok/xAI 并存；wire、routes、TokenRefreshService、frontend OAuth flow 均保持并列注册。

## 冲突与取舍

- 本轮 merge 产生 36 个冲突文件，已全部手工解决；`git diff --name-only --diff-filter=U` 为空。
- `backend/internal/handler/no_account_error.go`：新增 `Code` 字段以同时支持 upstream `model_not_found` 和本地 `no_available_accounts`/`model_not_supported_in_group` 错误码。
- `backend/internal/service/token_cache_invalidator.go`：去掉 merge 后重复的 `PlatformAnthropic` case；Anthropic/Kiro 分流和 Grok cache key 均保留。
- `backend/internal/service/openai_gateway_service.go`：同时执行本地 Responses event output JSON normalization 和 upstream function call arguments normalization。
- `frontend/src/components/account/CreateAccountModal.vue`、`OAuthAuthorizationFlow.vue`、`credentialsBuilder.ts`：Kiro social OAuth、Grok OAuth、Codex PAT、Antigravity project helper 合并共存。
- `frontend/src/components/account/__tests__/AccountUsageCell.spec.ts`：常规 OpenAI usage 加载断言调整为 `getUsage(id)`，匹配组件当前单参数调用；主动查询仍由 `getUsage(id, 'active', true)` 覆盖。

## 测试记录

- 通过：`cd backend && go generate ./cmd/server`
- 通过：`cd backend && go generate ./ent`
- 通过：`cd backend && go test -tags=unit ./...`
- 通过：`cd backend && make test-unit`
- 通过：`cd backend && go test -tags=integration ./...`
- 通过：`cd backend && golangci-lint run ./...`
- 通过：`pnpm --dir frontend run lint:check`
- 通过：`pnpm --dir frontend run typecheck`
- 通过：`pnpm --dir frontend exec vitest run src/components/account/__tests__/AccountUsageCell.spec.ts`
- 通过：`pnpm --dir frontend run test:run`（125 files / 761 tests）
- 通过：`git diff --check`
- 通过：`git tag --merged upstream/main --sort=-version:refname | head -1` 输出 `v0.1.139`
- 通过：`cat backend/cmd/server/VERSION` 输出 `0.1.139`
- 通过：`bash deploy/version_resolver.sh resolve .` 输出 `0.1.139`
- 通过：`bash scripts/collect_upstream_sync_context.sh --no-fetch`
- 通过：`POSTGRES_PASSWORD=dummy docker compose -f deploy/docker-compose.deploy.yml config -q`
- 通过：`POSTGRES_PASSWORD=dummy docker compose -f deploy/docker-compose.local.yml config -q`
- 通过：`make build`
- 未单独重复：`cd backend && make test-integration`，该目标等价于已执行的 `go test -tags=integration ./...`。
- 待部署后验证：Actions 结果、服务器版本、备份文件、rollback image、容器健康、线上健康接口。

## Subagent 评审与处理

- requesting-code-review 与 gstack review 均完成；两边都判定当前结果 Not Ready，且集中指出两项问题。
- 已修复：Grok/OpenAI-compatible `/responses` 与 `/chat/completions` no-account 分类改为按 group platform 诊断，避免 Grok unsupported model 误按 OpenAI 池返回 503。
- 已修复：`PUT /admin/settings` 的 Codex hardening 字符串字段改成 optional pointer；未传字段保留 `previousSettings`，显式传空字符串仍可清空。
- 已补回归：`TestClassifyOpenAICompatibleNoAccountError_GrokGroupUsesGrokPlatform`。
- 已补回归：`TestUpdateSettings_PartialUpdatePreservesCodexHardening`。
- 修复后已通过：`cd backend && go test -tags=unit ./internal/handler ./internal/handler/admin`。
- 修复后已通过：`cd backend && go test -tags=unit ./...`。

## 剩余风险与观察点

- Grok/xAI 新平台路径较大，部署后重点观察 OAuth 回调、quota probe、token refresh、gateway failover 日志。
- OpenAI/Codex gateway 同时合并了本地 normalize 与 upstream normalize，部署后重点观察 Responses streaming、tool call arguments、no-account 错误码形态。
- payment/currency 与 SettingsView 改动覆盖前后端，部署后重点抽查后台设置页和订单/退款展示。
- compose config 校验使用 dummy `POSTGRES_PASSWORD` 仅验证模板可解析，不代表生产 secret 内容。

## 结论

- 双 subagent 评审反馈已处理，建议进入提交与部署阶段；部署后继续按上面的观察点做线上验收。
