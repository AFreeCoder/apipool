# APIPool 长期本地定制点

本文件用于 upstream 同步时快速识别“不能被无脑覆盖”的 APIPool 长期定制区域。

## 1. 品牌与默认文案

- `README.md`
- `README_CN.md`
- `frontend/src/components/layout/AppHeader.vue`
- `frontend/src/views/admin/SettingsView.vue`
- `frontend/src/i18n/locales/en.ts`
- `frontend/src/i18n/locales/zh.ts`

同步时要保留：

- 品牌名默认值为 `APIPool`，不要回退到 `Sub2API`
- GitHub 链接指向 `AFreeCoder/apipool`
- 支付/文档下载入口保持指向本仓库 `docs/`

## 2. 部署、回滚与版本链路

- `backend/cmd/server/VERSION`
- `.github/workflows/deploy.yml`
- `.github/workflows/release.yml`
- `deploy/README.md`
- `deploy/ROLLBACK_CN.md`
- `deploy/rollback.sh`
- `deploy/docker-compose.deploy.yml`
- `deploy/version_resolver.sh`

同步时要保留：

- DigitalOcean 单机 Docker Compose 部署链路
- 部署前数据库备份与镜像回退标签逻辑
- 运行时版本与页面版本展示的一致性
- 合并后必须复核 `backend/cmd/server/VERSION`

## 3. OpenAI OAuth / Codex / ChatGPT 兼容

- `backend/internal/service/openai_gateway_service.go`
- `backend/internal/service/openai_gateway_chat_completions.go`
- `backend/internal/service/ratelimit_service.go`
- `backend/internal/service/token_refresh_service.go`
- `backend/internal/service/account_usage_service.go`
- `backend/internal/handler/openai_gateway_handler.go`

同步时要保留：

- OAuth passthrough input 归一化
- Codex / ChatGPT compat prompt cache key 注入
- OpenAI OAuth 401/403 与 Cloudflare challenge 的本地错误策略
- 账户状态持久化与临时不可调度语义

## 4. 管理后台约束与默认配置

- `backend/internal/server/routes/admin.go`
- `backend/internal/service/admin_service.go`
- `backend/internal/handler/admin/account_handler.go`
- `frontend/src/components/account/BulkEditAccountModal.vue`
- `frontend/src/views/admin/SettingsView.vue`
- `frontend/src/views/admin/UsersView.vue`

同步时要保留：

- 批量改账号时的混合渠道风控检查
- 分组替换、专属分组迁移等 APIPool 后台操作约束
- 后台默认站点名和定制入口
- 不要误把已禁用/已取舍的后台入口重新暴露

## 5. Upstream Sync 时必须复核的热点文件

- `README.md`
- `deploy/README.md`
- `backend/internal/service/ratelimit_service.go`
- `backend/internal/service/openai_gateway_service.go`
- `backend/internal/service/openai_gateway_chat_completions.go`
- `frontend/src/views/admin/SettingsView.vue`
- `frontend/src/components/layout/AppHeader.vue`
- `frontend/src/components/account/BulkEditAccountModal.vue`

## 6. 本次同步（2026-03-20）新增关注点

- `.github/workflows/backend-ci.yml` 与 `security-scan.yml` 当前已切到 Go `1.26.1`
- `backend/.golangci.yml` 已排除 `gosec` 的 `G704`
- `frontend/src/components/account/AccountUsageCell.vue` / `AccountUsageCell.spec.ts` 现有 `/usage` API 调用签名为 `getUsage(id, source?)`
