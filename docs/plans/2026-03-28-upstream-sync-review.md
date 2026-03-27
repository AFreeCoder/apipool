# 上游同步评审记录（2026-03-28）

## 基线

- 当前分支：`analysis/upstream-sync-20260328`
- 合入前基线：`HEAD`
- 合入前 SHA：`d90050613ce1f7cf4cd19d24e16bf7bf031c79f6`
- 上游引用：`upstream/main`
- 上游 SHA：`fdd8499ffc3f434f597b61decd4f7db3bd82e304`
- merge-base：`94bba415b1e5b3f8ed36a49ac818d2443074333e`
- 同步方式：`git merge upstream/main`
- 当前运行时版本：`0.1.104`
- upstream/main 版本文件：`0.1.105`
- upstream 最新 release tag：`v0.1.105`
- 最终运行时版本：`0.1.105`
- 版本对齐方式：未拆独立 `chore(version)`；因为 upstream/main 的 `VERSION` 已与 `v0.1.105` 对齐

## 上游更新摘要

- 上游引入了 3 条主线：OpenAI / Antigravity 隐私模式与 plan_type 同步、TLS Fingerprint Profile 管理、Responses/ChatCompletions 路由与格式兼容增强。
- 后台设置页新增了自定义端点、网关转发开关、API Key Signature Rectifier 等配置入口；运维错误观测字段与 UI 也有增强。
- OpenAI 账号链路新增了 Mobile RT 手动输入、精确匹配账号、刷新时自动设置隐私、429 持久化限流、401/402 终态识别。
- 命中的高风险模块包括：`README.md`、`backend/internal/server/routes/admin.go`、`backend/internal/service/openai_oauth_service.go`、`backend/internal/service/token_refresh_service.go`、`backend/internal/service/ratelimit_service.go`、`backend/internal/service/openai_gateway_service.go`、`frontend/src/views/admin/SettingsView.vue`。
- 需要额外吸收但不能覆盖本地逻辑的点：OpenAI 刷新后 `plan_type` 同步、APIPool 品牌文案、后台已禁用的备份恢复入口、OpenClaw / GPT-5.2 别名测试、Codex passthrough 输入归一化测试。
- 版本链路无额外偏差：upstream 最新 tag、`backend/cmd/server/VERSION`、`deploy/version_resolver.sh resolve .` 都是 `0.1.105`。

## 本地定制保护点

- 品牌/APIPool 文案与入口：保留 APIPool README 主体、品牌说明和部署说明，同时只吸收 upstream 的多语言 README 入口与日文 README。
- 部署、回滚、版本链路：保留 APIPool 现有 `deploy/`、回滚脚本和 Compose 约定；本轮仅确认上游 `.dockerignore` 调整不会影响现有部署。`VERSION` 已随 upstream 对齐到 `0.1.105`。
- OpenAI OAuth / Codex 兼容逻辑：保留刷新后 `SyncOpenAIPlanType`、passthrough 输入归一化测试、公开错误映射测试；同时吸收 upstream 的隐私模式、429 限流持久化、401 token_invalidated/token_revoked 与 402 deactivated_workspace 识别。
- 管理后台行为与默认配置：确认 `admin.go` 只新增 TLS Fingerprint Profile 与 `set-privacy` 路由，没有重新暴露备份恢复入口；设置页保留 APIPool 既有结构并吸收 upstream 新配置项。

## 冲突与取舍

- 显式 Git 冲突文件：
  - `README.md`
  - `backend/internal/handler/admin/account_handler.go`
  - `backend/internal/handler/admin/admin_service_stub_test.go`
  - `backend/internal/handler/openai_gateway_handler_test.go`
  - `backend/internal/service/admin_service.go`
  - `backend/internal/service/openai_oauth_passthrough_test.go`
  - `backend/internal/service/ratelimit_service.go`
  - `backend/internal/service/ratelimit_service_401_test.go`
  - `backend/internal/service/token_refresh_service.go`
  - `frontend/src/components/account/__tests__/BulkEditAccountModal.spec.ts`
  - `frontend/src/components/keys/__tests__/UseKeyModal.spec.ts`
- 无冲突但做了语义审查的文件：
  - `backend/internal/server/routes/admin.go`
  - `backend/internal/service/openai_oauth_service.go`
  - `frontend/src/views/admin/SettingsView.vue`
  - `frontend/src/components/account/EditAccountModal.vue`
- 最终保留的 APIPool 行为：
  - OpenAI 刷新后 `plan_type` 同步
  - APIPool README/品牌语义
  - 公开 OpenAI 账号选择错误映射测试
  - passthrough 输入归一化测试
  - OpenClaw / GPT-5.2 别名相关前端测试覆盖
  - 未重新启用备份恢复入口
- 最终吸收的 upstream 修复/特性：
  - Antigravity 隐私设置与强制重试接口
  - OpenAI 账号隐私模式同步与 429/401/402 处理增强
  - TLS Fingerprint Profile 后台 CRUD
  - Responses/ChatCompletions 路由与格式兼容增强
  - 设置页自定义端点、网关 forwarding、API Key Signature Rectifier
- 本轮未补独立版本提交；`VERSION` 已由 upstream merge 本身带到 `0.1.105`。

## 测试记录

- 通过：`cd backend && GOTOOLCHAIN=go1.26.1 go test -tags=unit ./...`
- 未通过（环境限制）：`cd backend && GOTOOLCHAIN=go1.26.1 go test -tags=integration ./...`
  - `internal/middleware`、`internal/server/routes` 依赖 testcontainers + Docker daemon，当前环境无法连接 Docker socket
  - `internal/pkg/tlsfingerprint` 依赖外部 `https://tls.peet.ws/api/all`，当前环境 TLS handshake 返回 EOF
- 未通过（与当前主工作区已有未提交修复相关）：`cd backend && GOTOOLCHAIN=go1.26.1 golangci-lint run ./...`
  - 仅剩 `backend/internal/handler/concurrency_error_mapping_test.go` 的 `errcheck` 报告；该文件不在本轮 upstream overlap 内，且用户当前主工作区已存在未提交修复
- 通过：`pnpm --dir frontend install --frozen-lockfile`
- 通过：`pnpm --dir frontend run lint:check`
- 通过：`pnpm --dir frontend run typecheck`
- 通过：`pnpm --dir frontend run test:run`
  - 期间修复了 `frontend/src/components/account/EditAccountModal.vue` 中 `loadTLSProfiles` 被 `watch(..., { immediate: true })` 提前调用的初始化顺序问题
- 通过：`git tag --merged upstream/main --sort=-version:refname | head -1` -> `v0.1.105`
- 通过：`cat backend/cmd/server/VERSION` -> `0.1.105`
- 通过：`GOTOOLCHAIN=go1.26.1 make build`
  - 前端构建有 chunk size warning，但构建成功
- 通过（使用占位环境变量 `POSTGRES_PASSWORD=dummy`）：
  - `docker compose -f deploy/docker-compose.deploy.yml config -q`
  - `docker compose -f deploy/docker-compose.local.yml config -q`
- 通过：`bash deploy/version_resolver.sh resolve .` -> `0.1.105`
- 未做：部署后 `docker exec ... --version` / 页面左上角版本人工核对（当前未实际部署）

## 剩余风险与观察点

- 当前仓库说明存在 Go 版本链路不一致：`backend/go.mod`、CI workflow、`deploy/Dockerfile` 已要求 `1.26.1`，本轮同步里我已把 `README.md` 对齐到 `1.26.1`，但主工作区的项目说明文件仍需一并复核。
- `golangci-lint` 的唯一剩余问题在 `backend/internal/handler/concurrency_error_mapping_test.go`，当前主工作区已经有未提交修复；如果要把本次 upstream sync 真正落到主工作区，建议和该本地改动一起处理，避免重复劳动。
- integration 未在当前环境完整验证，主要受 Docker daemon 与外部 TLS 指纹服务可达性限制；上线前仍建议在有 Docker 和公网连通的环境复跑。
- `make build` 成功，但前端构建出现 chunk size warning；这不是阻塞项，但后续可以观察首页与设置页首屏体积。

## 结论

- 建议保留当前 merge 结果，作为可审阅的 upstream sync 候选分支。
- 当前结果位于临时 worktree 分支 `analysis/upstream-sync-20260328`，尚未落回主工作区 `main`，因为主工作区存在用户未提交改动，需要先决定如何处理再真正合回。
- 评审文档路径：`docs/plans/2026-03-28-upstream-sync-review.md`
