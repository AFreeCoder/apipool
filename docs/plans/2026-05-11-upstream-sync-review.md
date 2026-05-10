# 上游同步评审记录（2026-05-11）

## 基线

- 当前分支：`main`
- 当前 HEAD：`1b751035413c2fb005a4b0e51c6d39878e8b46f0`
- 合入前本地基线：`1b751035413c2fb005a4b0e51c6d39878e8b46f0`
- 上游引用：`upstream/main`
- 上游 SHA：`dbc8ae658cfc1c012160752582925e45115e2f3a`
- merge-base：`4de28fec8c061ee5f0bad93e885c07fced41c864`
- 同步方式：`git merge upstream/main`
- upstream 最新 release tag：`v0.1.125`
- upstream/main 版本文件：`0.1.125`
- 本地最终 VERSION：`0.1.125`

## 上游更新摘要

- 吸收 `merge-base 4de28fec` 之后的 43 个 upstream 提交，覆盖 `v0.1.124`、`v0.1.125` 两个 release tag。
- 主要功能变化：
  - 登录注册：新增登录条款确认、GitHub/Google 邮箱快捷登录、OAuth pending flow 与邮箱验证契约更新。
  - OpenAI/Codex：新增 Codex image bridge、OpenAI image generation controls、Messages compatibility、`previous_response_id` function_call_output 处理，以及停止默认注入 redact-thinking beta。
  - 风控与审计：新增内容审核服务、后台风控中心页面、hash cache、审计日志与相关 migration。
  - 运营/配置：新增 redeem code affiliate rebate、batch concurrency API、markdown page rendering、自定义页面图片路径加固、账号模型白名单更新。
  - CI/依赖/文档：更新 Go toolchain 相关 CI、security scan、release workflow、axios/openspec 清理、赞助商文档。
- 命中的高风险模块：`README*`、`.github/workflows/*`、`deploy/*`、`backend/cmd/server/VERSION`、`backend/internal/service/openai_*`、`backend/internal/handler/openai_*`、`backend/internal/service/ratelimit_service.go`、`frontend/src/views/admin/SettingsView.vue`、`frontend/src/components/layout/AppSidebar.vue`。

## 本地定制保护点

- 品牌与文案：
  - `README.md` 保留 APIPool 自定义说明、DigitalOcean 部署说明、iframe 充值页共存说明；未接受 upstream 英文 README 中大段 Sub2API overview/sponsor 区块。
  - 公开设置、contract 测试和默认 `site_name` 保持 `APIPool`。
  - `README_CN.md`、`README_JA.md` 在本地基线中仍是 upstream Sub2API 文档，本轮仅吸收 SilkAPI referral link 更新，未做额外品牌改写。
- 部署/回滚/版本链路：
  - 保留 APIPool 现有 Compose/DigitalOcean 发布假设和 pre-deploy 备份相关本地提交。
  - `backend/cmd/server/VERSION` 已随 upstream merge 对齐到 `0.1.125`；本轮 upstream/main 的 VERSION 与最新 tag 一致，因此未拆额外 `chore(version)` 提交。
  - `deploy/version_resolver.sh resolve .` 输出 `0.1.125`，与 VERSION/tag 一致。
- OpenAI OAuth / Codex 兼容：
  - 保留本地 `gpt-5.5`、Codex fallback `0.125.0`、OAuth input 归一化、未知 input item 类型日志、tool role/message content normalization、tool continuation 保留 reference/id 的逻辑。
  - 合入 upstream 的 Codex image bridge、Spark image unsupported instructions、PreserveToolCallIDs、Messages bridge session-only 行为和 image usage mandatory record fallback。
- Kiro/OpenClaw 扩展：
  - `kiroOAuthHandler`、Kiro admin route、Kiro provider/wire 链路继续保留。
  - `CreateAccountModal` 合并时保留 Kiro social OAuth 的 platform display 与隐藏 Anthropic helper/cookie 多账号选项，同时吸收 OpenAI Codex session import 控制。
- 后台入口与默认配置：
  - `/purchase` 仍由 `purchase_subscription_enabled` / `purchase_subscription_url` 驱动 iframe 充值入口，未被 upstream 内建支付页接管。
  - 设置页保留 APIPool header marquee 配置区，并吸收 upstream 登录条款/GitHub/Google/风控设置。
  - `authSourceDefaults` 兼容 upstream 新增 `github`、`google` source；保存 payload 时对缺失 source 使用默认值兜底，避免旧状态对象运行时报错。

## 冲突与取舍

- Git 内容冲突文件与处理：
  - `README.md`：保留 APIPool README 和 Nginx 下划线 header 说明，拒绝 upstream Sub2API overview/sponsor 区块覆盖。
  - `backend/cmd/server/wire_gen.go`：同时保留 Kiro OAuth handler 和 upstream ContentModeration handler 注入。
  - `backend/internal/config/config.go`：吸收 upstream image stream timeout 与 `max_line_size=500MB` 默认值。
  - `backend/internal/handler/dto/settings.go`、`backend/internal/service/setting_service.go`、`backend/internal/server/api_contract_test.go`：合并本地 marquee/ForceEmail/APIPool 默认值和 upstream 登录条款/GitHub/Google/风控字段。
  - `backend/internal/handler/openai_gateway_handler.go`：合并本地 upstream error 不记成功 usage 的保护与 upstream image result mandatory usage 兜底。
  - `backend/internal/service/openai_account_scheduler.go`：保留本地 `ModelUnsupported` 统计，同时采用 upstream 带 context 的 compatibility 判断。
  - `backend/internal/service/openai_codex_transform.go`：保留 APIPool OAuth input normalize / unknown item logging，同时吸收 upstream transform options / PreserveToolCallIDs。
  - `backend/internal/service/openai_gateway_service_test.go`：保留本地 compact fallback version 测试并吸收 upstream Messages bridge session-only 测试。
  - `frontend/src/api/admin/settings.ts`：合并 marquee type import、登录条款 type import和 auth source 默认兜底。
  - `frontend/src/components/account/CreateAccountModal.vue`、`OAuthAuthorizationFlow.vue`：保留 Kiro display platform，同时吸收 Codex session import option。
  - `frontend/src/views/admin/SettingsView.vue`：保留 Header Marquee 卡片，并接入 upstream Login Agreement tab。
- 无冲突但复核的热点：
  - `deploy/*.yml` / `deploy/*.yaml`：Compose 配置可解析；本地发布链路未被重置为 upstream 默认。
  - `frontend/src/router/index.ts`、`AppSidebar.vue`：`/purchase` 入口和用户路由仍存在。
  - `backend/internal/handler/wire.go`、`backend/internal/server/routes/admin.go`：Kiro 和 ContentModeration admin routes 同时存在。
  - `frontend/src/utils/featureFlags.ts`：`purchase_subscription_enabled`、`risk_control_enabled` 等 feature flags 有对应公开设置。

## 测试记录

- 通过：
  - `cd backend && go test -tags=unit ./...`
  - `cd backend && make test-unit`
  - `cd backend && golangci-lint run ./...`
  - `pnpm --dir frontend run lint:check`
  - `pnpm --dir frontend run typecheck`
  - `pnpm --dir frontend run test:run`（98 files / 578 tests）
  - `cd backend && make build`
  - `POSTGRES_PASSWORD=dummy JWT_SECRET=dummy ENCRYPTION_KEY=dummy docker compose -f deploy/docker-compose.deploy.yml config -q`
  - `POSTGRES_PASSWORD=dummy JWT_SECRET=dummy ENCRYPTION_KEY=dummy docker compose -f deploy/docker-compose.local.yml config -q`
  - `POSTGRES_PASSWORD=dummy JWT_SECRET=dummy ENCRYPTION_KEY=dummy docker compose -f deploy/docker-compose.yml config -q`
  - `bash deploy/version_resolver.sh resolve .` → `0.1.125`
  - `bash scripts/collect_upstream_sync_context.sh --no-fetch`
- 版本链路：
  - `git tag --merged upstream/main --sort=-version:refname | head -1` → `v0.1.125`
  - `cat backend/cmd/server/VERSION` → `0.1.125`
  - `deploy/version_resolver.sh resolve .` → `0.1.125`
- 环境受限：
  - `cd backend && go test -tags=integration ./...` 未通过，失败点是 `internal/middleware` 与 `internal/server/routes` 中 testcontainers 启 Redis，错误为本机 Docker daemon 不可用：`Cannot connect to the Docker daemon at unix:///Users/afreecoder/.docker/run/docker.sock`。
  - `cd backend && make test-integration` 同样因 Docker daemon 不可用失败。
  - 本轮未执行部署后线上二进制版本或页面版本核对；这是本地 upstream merge，不包含生产发布。

## 剩余风险与观察点

- integration 测试需要 Docker daemon；当前本机 Docker 未运行，未覆盖 Redis/testcontainers 相关集成断言。上线前或 CI 中应确保 Docker 可用后重跑。
- 本轮吸收内容审核、登录条款、GitHub/Google 登录、账号 Codex 导入、图片生成计费/并发控制，都是跨后端/前端/配置的业务面。发布后应重点观察：
  - OpenAI Responses / compact / WS v2 / Codex image bridge 请求是否正常记录 usage，尤其是 upstream error event 与 image partial result 组合。
  - `/api/v1/settings/public` 与 HTML injection 的 feature flags 是否一致，避免菜单刷新闪烁或隐藏。
  - 风控中心默认关闭时不应影响现有请求；启用后重点看 audit logs、hash cache、用户解封。
  - GitHub/Google 快捷登录默认关闭，配置为空时不应在登录页曝光入口。
  - APIPool `/purchase` iframe 入口继续按现有系统设置驱动，不应被内建 payment 路由替换。
- 本轮保留现有未跟踪 `docs/superpowers/...` 文件，不纳入 merge，不做清理。

## 结论

- 建议保留当前 merge 结果。上游 `v0.1.125` 的功能与修复已吸收，APIPool 的品牌、部署、购买入口、Kiro/OpenClaw、OpenAI/Codex 兼容和后台默认值已做语义保护。
- 发布前剩余必做项是：在 Docker daemon 可用环境重跑 integration；如果要部署生产，再按 `apipool-push-deploy` 流程执行备份、推送、Actions、live commit 与运行时版本核对。
