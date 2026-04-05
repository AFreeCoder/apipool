# 上游同步评审记录（2026-04-06）

## 基线

- 当前分支：`main`
- 当前 HEAD：`8470669c99a6196f67395361a5522331cb9c1cd6`
- 合入前本地基线：`940d428f22d75b35ffed7e4e9dc6a2ee14c20a90`
- 上游引用：`upstream/main`
- 上游 SHA：`339d906e547e34fd7536586275205cb355817729`
- merge-base：`1dfd9744329929c717d5bc9b7956c9505f774930`
- 同步方式：`git merge upstream/main`
- upstream 最新 release tag：`v0.1.108`
- upstream/main 版本文件：`0.1.108`
- 本地最终 VERSION：`0.1.108`

## 上游更新摘要

- 本轮吸收了 `upstream/main` 自 `1dfd9744` 之后的 132 个提交，merge 结果相对本地基线共涉及 270 个文件、`14396` 行新增、`28277` 行删除，核心变化包括：
  - 渠道管理全链路落地：新增后台渠道管理 API、仓储与服务层、前端 `ChannelsView` 和渠道表单组件，覆盖模型映射、定价、平台限制、缓存策略和使用统计展示。
  - 计费与用量链路重构：引入 `billing_mode`、图片输出 token 计费、按次计费与渠道映射计费源，补充 `model_pricing_resolver`、渠道定价解析和 usage log 扩展。
  - OpenAI OAuth / Codex / 调度链路增强：继续推进 compat model 归一化、401 永久鉴权失败识别、plan type/订阅过期时间同步、隐私/分组过滤与账号调度修复。
  - Sora 功能彻底下线：后端 handler/service/repository、前端页面/API、测试与数据库 schema 一并移除，并新增 `backend/migrations/090_drop_sora.sql` 清理持久化结构。
  - 文档与版本推进：`upstream/main` 自带 `VERSION=0.1.108`，并更新多语言 README 与合作伙伴资源。
- 本轮命中的高风险模块：
  - `README.md`
  - `backend/internal/config/config.go`
  - `backend/internal/service/openai_oauth_service.go`
  - `backend/internal/service/ratelimit_service.go`
  - `backend/internal/service/account_test_service.go`
  - `backend/internal/server/routes/admin.go`
  - `frontend/src/router/index.ts`
  - `frontend/src/views/admin/SettingsView.vue`
  - `frontend/src/i18n/locales/en.ts`
  - `frontend/src/i18n/locales/zh.ts`

## 本地定制保护点

- 品牌与文案：
  - `README.md` 冲突处保留了 APIPool 的主 README 语义、部署说明和本地开发提示，没有被上游 Sub2API README 覆盖。
  - `README_CN.md` / `README_JA.md` 跟随上游更新，多语言补充不影响 APIPool 主品牌入口。
- 部署 / 回滚 / 版本链路：
  - 本轮未让上游覆盖 APIPool 的 `deploy/`、回滚脚本和 DigitalOcean 单实例 Compose 假设。
  - `backend/cmd/server/VERSION` 在 merge 结果中保持为 `0.1.108`，与 `upstream/main` 和最新 release tag `v0.1.108` 一致。
  - 版本对齐未拆成独立提交，因为 `upstream/main` 自身已经携带 `VERSION=0.1.108`，merge 结果天然完成对齐。
- OpenAI OAuth / Codex 兼容：
  - 保留了 APIPool 现有的 Codex/OpenAI OAuth 兼容主线、错误映射与计划巡检相关行为，同时吸收上游对 plan type/订阅信息同步和鉴权失败识别的修复。
  - 额外修复了 `account_test_service.go` 的语义回退，避免 OpenAI OAuth 专用的 `account_deactivated` 状态又被通用 401 文案覆盖。
- 后台入口与默认配置：
  - 接受上游移除 Sora 入口、Sora OAuth、Sora Client 开关与相关设置项；这是一次有意识的取舍，已与当前项目确认“现网未使用 Sora”。
  - 在 `backend/internal/config/config.go` 中保留 APIPool 本地的计划巡检与免费账号自动下线阈值配置，删除已失效的 `SyncLinkedSoraAccounts`。
  - 复查 `SettingsView` / `admin.go` 后确认：备份页与备份路由仍保留，新增的是渠道管理入口，不存在把 APIPool 备份能力一并删掉的回退。

## 冲突与取舍

- Git 冲突文件与处理方式：
  - `README.md`：放弃上游 README 主体重写，保留 APIPool README 的本地语义。
  - `backend/internal/config/config.go`：接受上游删除 Sora 配置，同时保留 APIPool 的 `PlanSyncIntervalMinutes`、`AutoUnscheduleFree`、`FreeUnscheduleThreshold` 等调度配置。
  - `backend/internal/service/openai_oauth_service.go`：保留“plan_type 不再从 id_token 写入”的本地语义说明，同时接受上游把 `plan_type` / `subscription_expires_at` 写入 credentials 的逻辑。
  - `backend/internal/service/billing_service.go` / `backend/internal/service/billing_service_test.go`：接受上游统一计费与渠道定价改造，但保留 APIPool 的“零倍率免单”语义，把 `<= 0` 改回 `< 0`，避免 `0` 倍率被重置成 `1`。
  - `frontend/src/i18n/locales/en.ts`、`frontend/src/i18n/locales/zh.ts`、`frontend/src/components/keys/__tests__/UseKeyModal.spec.ts`、`frontend/src/utils/__tests__/openclawConfig.spec.ts`：按最终平台集合移除 Sora 相关文案与测试断言，避免编译和类型残留。
- 无冲突但做过语义复核的热点文件：
  - `backend/cmd/server/wire.go`：移除 `SoraMediaCleanupService` 注入和 stop hook，确保编译链路与运行时容器关闭流程不再引用已删除服务。
  - `backend/internal/service/ratelimit_service.go`：将 `soraerror` 迁移为 `httputil` 后复查调用点，确认错误映射没有被带偏。
  - `backend/internal/service/account_test_service.go` 与 `backend/internal/service/account_test_service_openai_test.go`：修复 OAuth 401 语义覆盖，并清理误留的 Sora 测试上下文。
  - `backend/internal/handler/concurrency_error_mapping_test.go`：移除 `SoraGatewayHandler` 断言，避免删除 Sora 后测试仍引用旧 handler。
  - `backend/internal/server/routes/admin.go`、`frontend/src/router/index.ts`、`frontend/src/views/admin/SettingsView.vue`：确认最终结果是“删除 Sora 入口、保留备份、增加渠道管理”，不是误删后台核心功能。

## 测试记录

- 后端：
  - `cd backend && go test ./...`：通过
  - `cd backend && make test-unit`：通过
  - `cd backend && make test-integration`：失败，原因是当前环境没有可用 Docker daemon，`testcontainers-go` 无法连接本地 Docker socket，不属于代码断言失败
  - `cd backend && golangci-lint run ./...`：通过（`0 issues`）
- 前端：
  - `pnpm --dir frontend run lint:check`：通过
  - `pnpm --dir frontend run typecheck`：通过
  - `pnpm --dir frontend run test:run`：通过
- 版本链路与部署校验：
  - `git tag --merged upstream/main --sort=-version:refname | head -1`：`v0.1.108`
  - `cat backend/cmd/server/VERSION`：`0.1.108`
  - `bash deploy/version_resolver.sh resolve .`：`0.1.108`
  - `set -a; source deploy/.env.example; set +a; docker compose -f deploy/docker-compose.deploy.yml config -q`：通过
  - `set -a; source deploy/.env.example; set +a; docker compose -f deploy/docker-compose.local.yml config -q`：通过
  - `make build`：通过；前端构建仅出现既有的 Vite chunk size 警告，没有构建失败
  - `./scripts/collect_upstream_sync_context.sh --no-fetch`：通过，版本链路复核为 `v0.1.108` / `0.1.108` / `0.1.108`

## 剩余风险与观察点

- 尚未验证的风险：
  - 仍需在有 Docker daemon 的环境补跑 `cd backend && make test-integration`，当前会话无法完成集成测试闭环。
  - 这轮变更面很大，渠道管理、统一计费和 Sora 全量移除同时落地，部署后应重点观察渠道定价、模型映射、用量统计和 OpenAI OAuth 刷新日志。
  - 因为已接受删除 Sora，任何仍依赖 `/sora`、`/admin/sora/*` 或 Sora 存储配置的外部脚本都会开始返回 404，需要确保现网侧确实没有遗留调用。
  - 尚未做真实部署，因此还没有核对线上容器 `--version` 输出和页面左上角展示版本。
- 灰度 / 回滚建议：
  - 推送或部署前，优先在可用 Docker 环境补跑 `cd backend && make test-integration`。
  - 部署后至少核对 4 件事：最新 upstream release tag、`backend/cmd/server/VERSION`、`deploy/version_resolver.sh` 解析值、线上运行中二进制/页面展示版本。
  - 若渠道计费或 OpenAI OAuth 刷新出现异常，优先观察新引入的渠道映射与调度日志；必要时沿用现有 `deploy/rollback.sh image` / `db-restore --with-image` 流程回退。

## 结论

- 建议保留当前 merge 结果。
- 在有 Docker daemon 的环境补跑 `make test-integration`，并完成一次真实部署后的版本与渠道链路核对后，即可作为本轮 upstream sync 的正式结果推送。
