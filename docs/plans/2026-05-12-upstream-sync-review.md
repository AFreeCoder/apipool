# 上游同步评审记录（2026-05-12）

## 基线

- 当前分支：`main`
- 当前 HEAD：`26dc29d844096eb026a1e78d0cc3cc8d37249ab8`
- 合入前本地基线：`bf7ae781078b68a9110f5af6cc20d3b7cb4d9601`
- 上游引用：`upstream/main`
- 上游 SHA：`62ccd0ff39d1d2dc80a6616adbd20a54d5f264f2`
- merge-base：`dbc8ae658cfc1c012160752582925e45115e2f3a`
- 同步方式：`git merge upstream/main`
- upstream 最新 release tag：`v0.1.126`
- upstream/main 版本文件：`0.1.126`
- 本地最终 VERSION：`0.1.126`

## 上游更新摘要

- 吸收 upstream `v0.1.126`，版本文件从 `0.1.125` 对齐到 `0.1.126`；本轮 upstream/main 上的 `VERSION` 与最新 release tag 一致，因此版本对齐直接随 upstream merge 进入，没有拆独立 `chore(version)` 提交。
- 新增 Airwallex 支付与多币种金额处理：后端新增 Airwallex provider、币种/费率/订单快照逻辑，前端新增 Airwallex 支付页、支付方式配置、图标和相关测试。
- 新增可配置 Antigravity User-Agent 版本：后端 settings runtime 读取并缓存 `antigravity_user_agent_version`，前端设置页新增配置项，同时保留环境变量兜底。
- 吸收 OpenAI/Codex 相关修复：messages continuation 保留多工具上下文、工具名改写测试补强、unpriced model 零成本 usage、OpenAI 429 `plan_type` 回写、replay tool output continuation。
- 吸收支付结果金额 NaN 修复、Gemini Vertex token exchange 代理修复、ccswitch import deeplink 的 Codex model 参数。
- 命中的高风险模块包括：`backend/internal/service/openai_gateway_service.go`、`backend/internal/service/ratelimit_service.go`、`backend/internal/service/setting_service.go`、`frontend/src/views/admin/SettingsView.vue`、`frontend/src/views/user/KeysView.vue`、`deploy/*.yml`、`backend/cmd/server/VERSION`。

## 本地定制保护点

- 品牌与文案：`site_name` 默认仍为 `APIPool`；ccswitch 导入 provider name 冲突处保留 `APIPool` 兜底，同时采用 upstream 新的 `buildCcSwitchImportDeeplink` 实现。
- 购买入口：`/purchase` 仍指向 APIPool 的 `PurchaseSubscriptionView.vue` iframe 订阅购买页，`purchase_subscription_enabled` / `purchase_subscription_url` 仍贯通后端公开设置、后台设置和侧边栏；Airwallex 只新增内部支付 `/payment/airwallex`，未接管 `/purchase`。
- 部署/回滚/版本链路：DigitalOcean/Compose 相关本地部署文件保留；`deploy/version_resolver.sh resolve .` 输出 `0.1.126`；`docker-compose.deploy.yml` 与 `docker-compose.local.yml` 在提供必填 `POSTGRES_PASSWORD` 后配置校验通过。
- OpenAI OAuth / Codex 兼容：OpenAI gateway、messages continuation、tool rewrite、usage record、429 plan type sync 均保留本地 Codex/OAuth 兼容路径；本地 `MergeCredentials` test stub 与 upstream `BulkUpdate` test stub 已合并。
- Kiro / OpenClaw：`KiroAuthService`、`KiroTokenProvider`、admin `/kiro/oauth/*` 路由、前端 Kiro 表单、`useKiroOAuth`、`openclawConfig` 均仍存在；upstream 支付和 settings 变更未删除 Kiro 扩展。
- 默认配置：表格分页、购买订阅、跑马灯、风险控制、支付显示方式、Antigravity UA、cache_control rewrite 等默认值均保留当前项目语义；`rewrite_message_cache_control` 默认仍由后端配置函数控制。

## 冲突与取舍

- `backend/internal/config/config.go`：合并 CSP。保留 APIPool 对 `frame-src https:` 的宽松 iframe 购买入口支持，同时吸收 Airwallex `script-src`、`style-src`、`frame-src` 域名。
- `backend/internal/service/account_test_service_openai_test.go`：合并测试 repo stub。本地 `MergeCredentials` 与错误记录字段保留，upstream 新增 `BulkUpdate` 捕获用于 OpenAI 429 plan type sync 测试。
- `frontend/src/api/admin/settings.ts`：合并 settings 请求字段。本地 `purchase_subscription_enabled` / `purchase_subscription_url` 保留，upstream `rewrite_message_cache_control` / `antigravity_user_agent_version` 吸收。
- `frontend/src/views/user/KeysView.vue`：保留 `APIPool` provider name 兜底，采用 upstream `buildCcSwitchImportDeeplink`，从而同时保留品牌与新 Codex model deeplink 行为。
- 无 Git 冲突但已复核的热点：`frontend/src/router/index.ts`、`frontend/src/views/admin/SettingsView.vue`、`backend/internal/service/setting_service.go`、`backend/internal/service/ratelimit_service.go`、`backend/internal/pkg/antigravity/oauth.go`、`backend/internal/server/routes/payment.go`、`deploy/docker-compose*.yml`。

## 测试记录

- `cd backend && go test -tags=unit ./...`：通过。
- `cd backend && make test-unit`：通过。
- `cd backend && go test -tags=integration ./...`：第一次因 Docker daemon 未启动失败；启动 Docker Desktop 后重跑通过。
- `cd backend && make test-integration`：Docker daemon 可用后通过。
- `cd backend && golangci-lint run ./...`：初次发现若干 test-only SA5011；已在测试断言 `t.Fatal` 后补 `return`，重跑通过，`0 issues`。
- `go test -tags=unit ./internal/service`：补跑受影响 service 测试包，通过。
- `pnpm --dir frontend run lint:check`：通过。
- `pnpm --dir frontend run typecheck`：第一次因本地 `node_modules` 未安装 upstream 新增 `@airwallex/components-sdk` 失败；执行 `pnpm --dir frontend install --frozen-lockfile` 后重跑通过。
- `pnpm --dir frontend run test:run`：第一次同样因缺少 `@airwallex/components-sdk` 失败；安装依赖后重跑通过，`102 passed (102)`，`599 passed (599)`。
- `git tag --merged upstream/main --sort=-version:refname | head -1`：`v0.1.126`。
- `cat backend/cmd/server/VERSION`：`0.1.126`。
- `make build`：通过；Vite 输出既有动态导入/chunk size warning，无构建失败。
- `POSTGRES_PASSWORD=dummy docker compose -f deploy/docker-compose.deploy.yml config -q`：通过。
- `POSTGRES_PASSWORD=dummy docker compose -f deploy/docker-compose.local.yml config -q`：通过。
- `bash deploy/version_resolver.sh resolve .`：`0.1.126`。
- `bash scripts/collect_upstream_sync_context.sh --no-fetch`：通过；merge 后 `head=26dc29d844096eb026a1e78d0cc3cc8d37249ab8`，`local_version=0.1.126`，`upstream_version=0.1.126`。
- 部署后版本输出 / 页面版本人工核对：本轮未部署生产，未执行线上版本核对。

## 剩余风险与观察点

- Airwallex 支付链路为 upstream 新增功能，本轮覆盖了单测、前端测试、构建和路由/CSP 复核，但未做真实 Airwallex sandbox 端到端支付回调。
- 内部 payment 页面和 APIPool `/purchase` iframe 购买入口继续并存；上线后建议观察侧边栏入口、`/purchase`、`/payment/airwallex`、`/payment/result` 的跳转是否符合生产配置。
- Docker daemon 启动前 integration 会失败；本轮已启动 Docker 后复跑通过，不归因于代码。
- 本轮未执行生产部署；如后续发布，需要用 `apipool-push-deploy` 补生产镜像、容器健康、运行时版本和回滚元数据验证。

## 结论

- 建议保留当前 upstream sync 结果。上游 `v0.1.126` 已合入，版本链路一致；APIPool 品牌、`/purchase` 订阅购买、Kiro/OpenClaw、OpenAI/Codex 兼容、部署配置均已复核并保留。当前剩余风险主要集中在 Airwallex 真实支付链路上线前验证。
