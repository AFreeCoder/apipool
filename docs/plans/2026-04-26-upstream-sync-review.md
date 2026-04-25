# 上游同步评审记录（2026-04-26）

## 基线

- 当前分支：`main`
- 当前 HEAD：`9ee9b4dbb49652c44470c841343b35e6e0073c5b`
- 合入前本地基线：`e738b74e21d05dab15b38615bbaeef43acbd8138`
- 上游引用：`upstream/main`
- 上游 SHA：`496469ac4e22a90f417ce1f1b48ff8868f938183`
- merge-base：`d162604f326043e8b9933f68bf214696c78ecf52`
- 同步方式：`git merge upstream/main`
- upstream 最新 release tag：`v0.1.118`
- upstream/main 版本文件：`0.1.118`
- 本地最终 VERSION：`0.1.118`

## 上游更新摘要

- 合入 `upstream/main` 从 `d162604f326043e8b9933f68bf214696c78ecf52` 到 `496469ac4e22a90f417ce1f1b48ff8868f938183`，共 45 个上游提交。
- 主要吸收内容：
  - OpenAI `/responses/compact` 账号支持、compact-only model mapping、pre-output failover 和流保活修复。
  - Codex / Claude Code mimicry 更新，包括 tool call id 保留、tool-name obfuscation、cache breakpoint、billing attribution block、CLI/beta header 更新。
  - affiliate 邀请返利流程、后台设置、仓储和迁移 `130`-`132`。
  - payment 修复：Stripe 路由守卫绕过、易支付与 Stripe 同时启用时按钮展示。
  - `backend/cmd/server/VERSION` 从 `0.1.117` 同步到 `0.1.118`。
- 命中的高风险模块：
  - `backend/internal/handler/openai_gateway_handler.go`
  - `backend/internal/service/openai_codex_transform.go`
  - `backend/internal/service/openai_gateway_service.go`
  - `backend/internal/service/openai_oauth_passthrough_test.go`
  - `backend/internal/service/ratelimit_service.go`
  - `frontend/src/components/layout/AppSidebar.vue`
  - `frontend/src/views/admin/SettingsView.vue`
  - `backend/cmd/server/VERSION`

## 本地定制保护点

- 品牌与文案：本轮上游未覆盖 APIPool 首页、帮助文档、Logo 和本地文档入口；`PurchaseSubscriptionView.vue`、`frontend/src/docs/`、`frontend/src/views/docs/` 均仍保留在本地。
- 部署/回滚/版本链路：`.github/workflows/deploy.yml`、`deploy/rollback.sh`、`deploy/version_resolver.sh`、`deploy/docker-compose.deploy.yml`、`deploy/docker-compose.local.yml` 仍保留 APIPool 的 DigitalOcean/Compose 生产约定。版本文件与 upstream tag 一致，未拆独立版本对齐提交。
- OpenAI OAuth / Codex 兼容：保留 APIPool 的更细账号选择错误、OpenAI OAuth 停用/403 状态映射和 Codex transcript item 归一化；compact fallback 版本已从本地 `0.125.0-alpha.3` 对齐到 upstream `0.125.0`。同时吸收上游 compact、tool role message、non-string message content、Spark image 限制提示等变更。
- Kiro / OpenClaw：`KiroOAuthHandler`、`KiroTokenProvider`、`gateway_service_kiro.go`、`useKiroOAuth`、`openclawConfig` 和 Kiro 前端账号编辑流程仍存在；`wire_gen.go` 已同时保留 `kiroOAuthHandler` 并新增 `affiliateHandler`。
- 后台入口与默认配置：`purchase_subscription_enabled` / `purchase_subscription_url`、购买 iframe 路由、SettingsView 中本地设置项仍保留；上游 affiliate / payment 设置作为新增能力合入，没有接管 APIPool 既有充值入口。

## 冲突与取舍

- Git 冲突文件：
  - `backend/cmd/server/wire_gen.go`：合并上游 `affiliateHandler`，同时保留本地 `kiroOAuthHandler` 注入。
  - `backend/internal/handler/openai_gateway_handler.go`：保留 APIPool 公开账号选择错误码；新增 compact-only 账号不可用时的 `compact_not_supported` 处理；补齐 fallback model 调用的 `requireCompact=false`。
  - `backend/internal/service/account_test_service.go`：合入上游 429 reconcile；OAuth 401 先保留 APIPool 精确停用/403 状态，若没有精确状态再落普通 401 永久错误。
  - `backend/internal/service/account_test_service_openai_test.go`：合并 upstream `ClearError` / `SetError` stub 与本地错误状态断言。
  - `backend/internal/service/account_usage_service.go`、`backend/internal/service/openai_gateway_service.go`、`backend/internal/service/openai_oauth_passthrough_test.go`：将 `codexCLIVersion` 对齐 upstream `0.125.0`，并继续通过派生的 `codexCLIUserAgent` 统一 compact probe/请求构造。
  - `backend/internal/service/openai_account_scheduler.go`：保留 APIPool 的 `buildOpenAISelectionFailureError` 和 unsupported-model 详细错误，同时吸收 compact account selection 流程。
  - `backend/internal/service/openai_codex_transform.go`：合并上游 tool role message、tool_choice、non-string content、Spark image 限制处理；保留 APIPool transcript item 归一化、未知 top-level item 记录和 tool continuation 规则。修正 `web_search_call` / `image_generation_call` 不应获得或保留 `call_id` 的语义冲突。
  - `backend/internal/service/openai_gateway_service.go`：保留 APIPool OAuth passthrough / Codex 兼容逻辑；compact fallback 版本对齐 upstream `0.125.0`；吸收上游 compact account support、stream failover、mimicry header/body 更新。
  - `frontend/src/components/account/EditAccountModal.vue`：同时保留 Kiro credentials / quota / pool mode 编辑能力，并接入上游 OpenAI compact mode、TLS fingerprint profile 加载和时间展示。
  - `frontend/src/components/account/__tests__/EditAccountModal.spec.ts`：保留 Kiro 编辑测试，新增上游 compact mode 测试与 `tlsFingerprintProfiles` mock。
- 无 Git 冲突但做过语义复核的热点：
  - `frontend/src/components/layout/AppSidebar.vue`：确认本地购买入口与上游 affiliate 入口并存。
  - `frontend/src/views/admin/SettingsView.vue`：确认 APIPool 购买 iframe 设置、后台设置项与上游 affiliate/payment 新增字段并存。
  - `backend/internal/server/routes/admin.go`、`backend/internal/handler/wire.go`、`backend/internal/service/wire.go`：确认 Kiro、affiliate、payment、OpenAI compact provider set 同时保留。
  - `deploy/` 与 `.github/workflows/deploy.yml`：确认上游未删除本地生产部署/回滚资产。

## 测试记录

- `cd backend && go test -tags=unit ./...`：通过。第一次完整运行暴露 `TestApplyCodexOAuthTransform_ImageAndWebSearchCallsDoNotGainCallID` 失败，已修复并重跑通过。
- `cd backend && make test-unit`：通过。
- `cd backend && go test -tags=integration ./...`：通过。
- `cd backend && make test-integration`：通过。
- `cd backend && golangci-lint run ./...`：通过，`0 issues`。
- `pnpm --dir frontend run lint:check`：通过。
- `pnpm --dir frontend run typecheck`：通过。
- `pnpm --dir frontend run test:run`：通过，`93 passed / 547 tests`。
- `pnpm --dir frontend exec vitest run src/components/account/__tests__/EditAccountModal.spec.ts`：通过，覆盖 Kiro 编辑与 OpenAI compact mode。
- `git tag --merged upstream/main --sort=-version:refname | head -1`：`v0.1.118`。
- `cat backend/cmd/server/VERSION`：`0.1.118`。
- `make build`：通过；构建命令使用 `-X main.Version=0.1.118`，Vite 仅输出既有 chunk/dynamic import warning。
- `POSTGRES_PASSWORD=dummy docker compose -f deploy/docker-compose.deploy.yml config -q`：通过。
- `POSTGRES_PASSWORD=dummy docker compose -f deploy/docker-compose.local.yml config -q`：通过。
- `bash deploy/version_resolver.sh resolve .`：输出 `0.1.118`。
- `bash scripts/collect_upstream_sync_context.sh --no-fetch`：通过；merge 后 `local_version=0.1.118`、`upstream_version=0.1.118`、`latest_upstream_tag=v0.1.118`。
- 部署后版本输出 / 页面版本人工核对：本轮未部署，未执行线上 `docker exec sub2api /app/sub2api --version` 或页面版本人工核对。
- 追加验证（对齐 `codexCLIVersion=0.125.0` 后）：`cd backend && go test -count=1 -tags=unit ./internal/service -run 'TestOpenAIBuildUpstreamRequest.*CompactUsesCurrentFallbackVersion|TestOpenAIGatewayService_OAuthPassthrough_NonCodexUAFallbackToCodexUA|TestAccountTestService_OpenAICompact'`：通过。
- 追加验证（对齐 `codexCLIVersion=0.125.0` 后）：`rg -n "0\\.125\\.0-alpha\\.3" backend frontend deploy scripts`：代码与部署脚本无残留（评审文档保留旧值作为决策上下文）。

## 剩余风险与观察点

- 上游 OpenAI compact 与 APIPool OAuth/Codex 兼容逻辑高度重叠，已用 unit/integration/build 覆盖；上线后仍建议重点观察 `/v1/responses`、`/v1/responses/compact`、Codex tool continuation、web search/image generation transcript item。
- 上游 affiliate 迁移 `130`-`132` 已通过测试和构建，但生产首次迁移后应观察返利相关表、后台设置保存和用户侧 affiliate 页面。
- `codexCLIVersion` 已从本地 `0.125.0-alpha.3` 对齐到 upstream `0.125.0`；上线后建议重点观察 Codex compact fallback 请求的 `Version` / `User-Agent` 兼容性、prompt caching 与账号探测结果。
- 本轮未执行真实部署，因此运行中容器版本、页面左上角版本和线上回滚流程仍需在部署阶段复核。
- 技能资产复核：`references/local-customizations.md`、`references/testing-matrix.md`、`scripts/collect_upstream_sync_context.sh`、`scripts/scaffold_sync_review_doc.sh` 当前仍覆盖本轮暴露的高风险模式，无需修改。

## 结论

- 建议保留当前 merge 结果。上游 `v0.1.118` 已合入，APIPool 的 Kiro/OpenClaw、购买 iframe、部署/回滚、OpenAI OAuth/Codex 细化逻辑均已保留，当前本地回归与构建校验通过。

## 二次评审意见（2026-04-26 追加）

整体上本轮按 skill 流程走得比较扎实（基线、冲突取舍、测试记录、定制保护点都有覆盖），但评审文档已 commit 后又出现了与文档矛盾的工作区改动，必须处理。

### 做得好的地方

- **结构完整**：本文档严格覆盖了 skill 要求的所有字段（基线 refs、上游摘要、本地定制保护、冲突取舍、测试记录、剩余风险、版本链路四件事）。
- **冲突取舍有理有据**：11 个 Git 冲突文件都给出了"保留 X / 吸收 Y"的清晰说明，例如 `openai_account_scheduler.go` 保留 `buildOpenAISelectionFailureError` 同时吸收 compact 流程。
- **语义冲突主动识别**：主动指出 `web_search_call` / `image_generation_call` 不应保留 `call_id`，并在测试中暴露后修复（`TestApplyCodexOAuthTransform_ImageAndWebSearchCallsDoNotGainCallID` 第一次失败 → 修复 → 复跑通过），符合"Git 无冲突但有业务冲突"的复核要求。
- **版本链路四件事齐全**：upstream tag `v0.1.118`、upstream/main `VERSION=0.1.118`、本地 `VERSION=0.1.118`、`version_resolver.sh` 输出 `0.1.118` 全部一致，无需独立版本对齐提交（合理，因为上游本身已自带 `9d1751ec chore: sync VERSION to 0.1.118`）。
- **测试矩阵覆盖度**：unit / integration / lint / 前端 lint / typecheck / test / build / docker compose config / version_resolver / collect_upstream_sync_context 全跑通，符合 skill 中"完整回归作为基线"的要求。

### 必须修正的问题

#### 1. 评审文档与代码状态自相矛盾（critical）

本文档第 39、50 行明确声称"保留本地 `codexCLIVersion = 0.125.0-alpha.3`"，但当前工作区已经把它改回 `0.125.0`：

```diff
# backend/internal/service/openai_gateway_service.go
- codexCLIVersion = "0.125.0-alpha.3"
+ codexCLIVersion = "0.125.0"
```

```diff
# backend/internal/service/openai_gateway_service_test.go
- require.Equal(t, "0.125.0-alpha.3", req.Header.Get("Version"))
+ require.Equal(t, codexCLIVersion, req.Header.Get("Version"))
```

这个改动有两个问题：

- **未提交**，工作区是脏的。
- **未在评审文档中体现**，且与文档结论直接冲突。

更关键的是：本轮 merge 已吸收上游 `1e57e88e fix(openai): bump codex CLI version from 0.104.0 to 0.125.0`，上游正式版已是 `0.125.0`，本地继续保留 `0.125.0-alpha.3` 反而失去原本的 fallback 意义。所以改动方向是对的，但执行方式不对：要么应在 merge 当时就吸收掉这一步并写进文档，要么需要补一个独立 commit + 更新评审文档。

#### 2. 测试记录不再覆盖当前工作区（high）

本文档"测试记录"小节的所有 `go test`、`golangci-lint`、`pnpm test:run` 都是在 `codexCLIVersion = 0.125.0-alpha.3` 状态下跑通的。改动后未复跑。虽然测试已改成引用 `codexCLIVersion` 常量、理论上不会因为版本字面量失败，但 skill 明确要求"evidence before assertions"，需要补跑确认。

#### 3. 决策缺失上下文

如果"对齐到上游 `0.125.0`"是有意调整，"本地定制保护点"一节应改成"`codex CLI` fallback 版本由本地 `0.125.0-alpha.3` 升级对齐至 upstream `0.125.0`"，并在"剩余风险"中记录上线后需观察 codex compact 行为。否则后续阅读者无法判断这是有意为之还是漏改。

### 处理建议

按下面顺序补救：

1. 确认改动意图：保留 `0.125.0`（有道理）还是回退到 `0.125.0-alpha.3`（与文档一致）。
2. 选定后：
   - 选 `0.125.0`：补独立 commit `chore(openai): align codex CLI fallback version to 0.125.0`，并修改本文档第 39、50 行 + 在"剩余风险"中加观察项。
   - 选 `0.125.0-alpha.3`：`git checkout` 这两个文件即可。
3. 复跑 `cd backend && go test -tags=unit ./internal/service/...` 至少覆盖受影响测试，把结果补进"测试记录"。
4. 47 commits ahead 未推送是后续事项，不是本轮评审范围。

### 处理结果

- 选择对齐 upstream：`codexCLIVersion = "0.125.0"`，`codexCLIUserAgent` 继续由该常量派生。
- `backend/internal/service/openai_gateway_service_test.go` 中 compact fallback 版本断言改为引用 `codexCLIVersion`，避免后续再出现常量与测试字面量不一致。
- 已补充本文档的本地定制保护点、冲突取舍、测试记录和剩余风险，消除 `0.125.0-alpha.3` 与当前代码状态的矛盾。
