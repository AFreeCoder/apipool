# 上游同步评审记录（2026-05-26）

## 基线

- 当前分支：`main`
- 当前 HEAD：`36b34824eaffe4a4e3a5635844e6eda687b26994`
- 合入前本地基线：`7a7fbddd30a67ea2ed8a4f909923ec5a6c8bec78`
- 上游引用：`upstream/main`
- 上游 SHA：`2f70d965bf5b046ad6e9474a77a493bf4fb60801`
- merge-base：`f5a2ad688af7451a90eec0d9a221ef4442ed54f6`
- 同步方式：`git merge upstream/main`
- upstream 最新 release tag：`v0.1.130`
- upstream/main 版本文件：`0.1.130`
- 本地最终 VERSION：`0.1.130`

## 上游更新摘要

- 本轮从 `upstream/main` 合入 70 个上游提交，版本从 `0.1.128` 对齐到 `0.1.130`。
- 主要吸收内容：
  - OpenAI Responses 流式错误收口：`response.failed` SSE 写入、已开始写响应后的错误补发、裸 `/responses` 路由识别、Codex tool output 识别。
  - OpenAI / Codex 调度修复：OpenAI 账号冷却调度优化、runtime block fast path、WebSocket continuation 与 fatal error 处理补强。
  - 风控与注册：内容审计支持按模型生效、同一用户消息重复审计去重、邮箱白名单后缀通配符。
  - 账号与用量：Chat Completions 测试连接路径、redeem code 批量更新、用户 API Key 用量按日明细、分组账号可用计数修正。
  - Bedrock / Claude Code 兼容、渠道监控 Responses reasoning 输出、`x/net` / `js-cookie` 安全依赖升级。
- 命中的高风险模块：OpenAI gateway / WebSocket、`ratelimit_service`、OIDC 登录、Settings、`deploy/config.example.yaml`、前端 Settings / i18n、`backend/cmd/server/VERSION`、README 多语言文档。

## 本地定制保护点

- 品牌与文案：保留 APIPool 主 README 的生产部署说明、`api.apipool.dev` 指引、iframe 充值并存说明和默认站点名 `APIPool`；未把上游 Sub2API sponsor/英文主页块合入 APIPool 主 README。
- 部署/回滚/版本链路：`backend/cmd/server/VERSION` 已与 upstream tag 对齐到 `0.1.130`；部署 workflow、rollback 脚本、deploy compose、version resolver 未在本轮被上游覆盖为 Sub2API 默认部署模型。
- OpenAI OAuth / Codex 兼容：保留 APIPool 本地 OpenAI/Kiro token provider、Codex stable error code、WebSocket fatal error 处理和上游新补的 Responses 错误事件逻辑。
- 后台入口与默认配置：保留 `purchase_subscription_*` iframe 充值入口、表格分页系统设置、APIPool 默认站点名，同时接入 upstream 新增的 `api_key_acl_trust_forwarded_ip` 与邮箱通配白名单行为。

## 冲突与取舍

- Git 冲突文件：
  - `README.md`：保留 APIPool 主 README；放弃上游 Sub2API sponsor/英文主页块在本仓库主 README 的覆盖。
  - `backend/cmd/server/wire_gen.go`：不手工拼生成文件，解决相关源码后执行 `go generate ./cmd/server` 重新生成。
  - `backend/internal/service/account_test_service_openai_test.go`：保留本地 401 deactivated 账号错误用例，同时接入 upstream Chat Completions fallback 测试用例。
  - `backend/internal/service/openai_ws_forwarder_ingress_session_test.go`：保留本地 error event / fatal event 回归用例，同时接入 upstream `tool_search_output` 自动补 `previous_response_id` 用例。
  - `backend/internal/service/setting_service.go`：默认站点名继续为 `APIPool`，同时合入 upstream `APIKeyACLTrustForwardedIP` 设置。
  - `backend/internal/service/setting_service_public_test.go`：保留 APIPool 默认站点名断言，同时接受 upstream `*.edu.cn` 通配白名单解析。
- 无冲突但复核的热点：`openai_gateway_service.go`、`ratelimit_service.go`、`auth_oidc_oauth.go`、`channel_monitor_checker.go`、`SettingsView.vue`、`deploy/config.example.yaml`、`frontend/pnpm-lock.yaml`。

## 测试记录

- 通过：`cd backend && go test -tags=unit ./internal/service -run 'TestAccountTestService_OpenAI|TestOpenAIGatewayService_ProxyResponsesWebSocketFromClient|TestSettingService_GetPublicSettings'`
- 通过：`cd backend && go test -tags=unit ./cmd/server`
- 通过：`cd backend && go test -tags=unit ./...`
- 通过：`cd backend && make test-unit`
- 通过：`cd backend && go test -tags=integration ./...`
- 通过：`cd backend && make test-integration`
- 通过：`cd backend && golangci-lint run ./...`
- 通过：`pnpm --dir frontend run lint:check`
- 通过：`pnpm --dir frontend run typecheck`
- 通过：`pnpm --dir frontend run test:run`（110 个 test files / 652 tests）
- 通过：`git tag --merged upstream/main --sort=-version:refname | head -1` -> `v0.1.130`
- 通过：`cat backend/cmd/server/VERSION` -> `0.1.130`
- 通过：`make build`（Vite 仅输出既有 chunk size / dynamic import warning）
- 通过：`POSTGRES_PASSWORD=dummy DATABASE_PASSWORD=dummy REDIS_PASSWORD=dummy JWT_SECRET=dummy docker compose -f deploy/docker-compose.deploy.yml config -q`
- 通过：`POSTGRES_PASSWORD=dummy DATABASE_PASSWORD=dummy REDIS_PASSWORD=dummy JWT_SECRET=dummy docker compose -f deploy/docker-compose.local.yml config -q`
- 通过：`bash deploy/version_resolver.sh resolve .` -> `0.1.130`
- 通过：`bash scripts/collect_upstream_sync_context.sh --no-fetch`
- 待部署后核对：线上容器健康、线上版本输出、公开 health endpoint。

## 剩余风险与观察点

- 线上风险主要集中在 OpenAI Responses/WebSocket 错误事件收口、账号 runtime block / 冷却调度、内容审计按模型生效、Chat Completions 测试路径和注册邮箱通配白名单。
- 本轮未触碰生产数据库 schema 的既有本地定义，但合入了 upstream 新 migration `140_extend_user_provider_default_grants_check.sql`、`141_subscription_expiry_notify_enabled.sql`；部署时需关注迁移与启动日志。
- 当前本地仍有 4 个未跟踪 `docs/superpowers/...` 草稿文件，未纳入本次 sync 提交。
- 回滚建议仍以现有部署链路为准：若新容器异常，优先 `cd /opt/sub2api/deploy && ./rollback.sh image`，仅在数据状态明确异常时再考虑 DB restore。

## 结论

- 建议保留当前合入结果并进入代码评审/部署流程。版本链路已对齐 `v0.1.130`，本地 APIPool 关键定制未被上游覆盖，完整本地回归和部署配置校验通过。
