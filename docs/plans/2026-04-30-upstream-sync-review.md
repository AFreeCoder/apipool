# 上游同步评审记录（2026-04-30）

## 基线

- 当前分支：`main`
- 当前 HEAD：`ec41b5e091c685a182a6a72cabc7ecd896444f4c`
- 合入前本地基线：`d6b21d2bd4e697694718073c687f466772e16abb`
- 上游引用：`upstream/main`
- 上游 SHA：`8ad099baa6057f0dfed32ded1f04fc5ea5a38717`
- merge-base：`c056db740d56ce008292a7b414c804cc6f308208`
- 同步方式：`git merge upstream/main`
- upstream 最新 release tag：`v0.1.120`
- upstream/main 版本文件：`0.1.120`
- 本地最终 VERSION：`0.1.120`

## 上游更新摘要

- 本轮从 `upstream/main` 合入 52 个 upstream-only commits，对齐到 `v0.1.120` / `backend/cmd/server/VERSION=0.1.120`。
- 主要新增/修复：
  - Vertex Service Account 支持，并接入 Anthropic/Gemini service account 创建、编辑、测试与用量展示。
  - OpenAI Fast/Flex Policy（HTTP + WebSocket + Admin）策略配置。
  - 账号批量编辑支持 filter-target 范围选择，并补充 compact settings 字段。
  - `httputil` 支持 zstd/gzip/deflate 请求体解压，并加入 decompression bomb guard。
  - Anthropic/Responses 兼容修复：`Read.pages=""` 清理、Responses-compatible `tool_choice`、Anthropic SSE error event 标准化、stream EOF 进入 failover。
  - OpenAI/Codex 修复：OAuth path 丢弃 replayed reasoning items，保留 compact payload 字段，避免 explicit tool replay 被误判为 WS continuation，versioned image base URL 支持。
  - ops cleanup 支持 retention days = 0 表示每次定时清空目标表。
  - 上游 sponsors 文档加入 PatewayAI logo 与文案。
- 命中的高风险模块：
  - `README.md`
  - `backend/cmd/server/VERSION`
  - `backend/cmd/server/wire_gen.go`
  - `backend/internal/domain/constants.go`
  - `backend/internal/handler/admin/account_handler.go`
  - `backend/internal/service/account_test_service.go`
  - `backend/internal/service/gateway_service.go`
  - `backend/internal/service/openai_codex_transform.go`
  - `backend/internal/service/openai_gateway_service.go`
  - `frontend/src/components/account/CreateAccountModal.vue`
  - `frontend/src/components/account/EditAccountModal.vue`
  - `frontend/src/views/admin/SettingsView.vue`

## 本地定制保护点

- 品牌与文案：
  - `README.md` 保留 APIPool 自定义首页、生产部署说明、Nginx header 说明、iframe 充值并存说明；未回退到 upstream 英文 README 主体。
  - `README_CN.md` / `README_JA.md` 吸收 upstream sponsor 追加，不改 APIPool 主 README 定位。
- 部署/回滚/版本链路：
  - `.github/workflows/deploy.yml`、`deploy/rollback.sh`、`deploy/docker-compose.deploy.yml`、`deploy/version_resolver.sh` 没有被 upstream 覆盖。
  - 当前仍是 GitHub Actions 构建 GHCR 精确 `sha-<commit>` 镜像，服务器 pull 后标记 `deploy-sub2api:latest`。
  - 部署前数据库备份、`deploy-sub2api:rollback-latest`、`last-rollback-image.txt` 逻辑仍在。
  - `backend/cmd/server/VERSION` 与 upstream/main / upstream latest release tag 同步为 `0.1.120`。
- OpenAI OAuth / Codex 兼容：
  - 保留 APIPool 对 Codex model alias、compact fallback、unknown top-level transcript item preserve、image_generation bridge、Spark image unsupported instruction 等本地逻辑。
  - 接受 upstream 对 OAuth path replayed `reasoning` item 的修正：由于 `store=false` 下 ChatGPT internal 不持久化 `rs_*`，转发会产生 404，因此最终 `filterCodexInput` 丢弃 `reasoning` item。
  - 保留 compact 分支删除 `store` / `stream`，非 compact 分支强制 `store=false` 且 `stream=true` 的 APIPool 行为。
- Kiro / OpenClaw：
  - `kiro` account type 与 upstream `service_account` 并存。
  - `NewAccountTestService`、`wire_gen.go`、`GatewayService.GetAccessToken` 同时接入 `KiroTokenProvider` 与 `ClaudeTokenProvider`。
  - 前端账号创建/编辑保留 Kiro social/idc credential UI，同时吸收 Vertex Service Account UI。
- 后台入口与默认配置：
  - 保留 APIPool 设置页的 marquee、iframe purchase、OpenClaw/Kiro 相关入口。
  - 吸收 upstream OpenAI Fast/Flex Policy 设置项，未替换现有 purchase iframe 路由。

## 冲突与取舍

- Git 冲突文件与处理方式：
  - `README.md`：保留 APIPool 主 README，不采用 upstream 英文 README 主体；中文/日文 README 的 sponsor 增量保留。
  - `backend/cmd/server/wire_gen.go`：同时保留 Kiro provider 与新增 Claude/Vertex provider；`NewAccountTestService` 参数调整为同时接收 `kiroTokenProvider` 和 `claudeTokenProvider`。
  - `backend/internal/domain/constants.go`、`backend/internal/service/domain_constants.go`、`backend/internal/handler/admin/account_handler.go`、`frontend/src/types/index.ts`、`frontend/src/components/common/PlatformTypeBadge.vue`：账号类型合并为 `kiro` 与 `service_account` 并存。
  - `backend/internal/service/account_test_service.go`：保留 Kiro account test，同时接入 Vertex Service Account test。
  - `backend/internal/service/gateway_service.go`：Kiro token 获取与 Vertex service account token 获取并存；Anthropic service account 走 Vertex URL/body builder。
  - `backend/internal/service/openai_codex_transform_test.go`：保留未知 top-level item preserve 测试，接受 upstream drop reasoning item 测试；删除旧的“reasoning 必须保留”期望。
  - `backend/internal/service/openai_gateway_service.go`：保留 APIPool compact/non-compact normalization 语义，同时吸收 upstream 删除 unsupported fields 的说明与实现。
  - `frontend/src/components/account/CreateAccountModal.vue`、`EditAccountModal.vue`、`AccountUsageCell.spec.ts`、`en.ts`、`zh.ts`：Kiro 与 Vertex UI、文案、测试并存。
- 无冲突但已复核的热点文件：
  - `frontend/src/views/admin/SettingsView.vue`：确认 OpenAI Fast/Flex Policy 被吸收，marquee / purchase iframe 设置仍在。
  - `frontend/src/views/admin/AccountsView.vue`、`BulkEditAccountModal.vue`：确认 filter-target bulk edit 与 APIPool 账号分组风险提示并存。
  - `backend/internal/service/openai_codex_transform.go`：确认 OAuth path reasoning drop 是本轮接受的 upstream 行为变化，避免 `rs_*` replay 404。
  - `backend/internal/service/openai_gateway_service.go`：确认 upstream stream error sanitization 和 failover 包装保留，并保留 APIPool compact path。

## 测试记录

- 通过：`cd backend && go test -tags=unit ./internal/service ./internal/handler/admin ./cmd/server`
  - `internal/service`、`internal/handler/admin`、`cmd/server` 均通过。
- 通过：`cd backend && go test -tags=unit ./...`
  - 全量 unit 通过。
- 未单独运行：`cd backend && make test-unit`
  - 本轮已直接运行其等价入口 `go test -tags=unit ./...`。
- 受环境限制：`cd backend && go test -tags=integration ./...`
  - 已执行的包多数通过。
  - 失败点：`github.com/Wei-Shaw/sub2api/internal/server/routes` 的 `TestAuthRegisterRateLimitThresholdHitReturns429` 在 testcontainers 启动 Redis 前 panic。
  - 失败原因：本机 Docker daemon 未运行，`docker info` 同样返回 `Cannot connect to the Docker daemon at unix:///var/run/docker.sock`。
  - 当前判断：这是本地环境限制，不是代码断言失败；需要 Docker daemon 后重跑 integration。
- 未单独运行：`cd backend && make test-integration`
  - 因 Docker daemon 不可用，直接入口已在 testcontainers 处失败。
- 通过：`cd backend && golangci-lint run ./...`
  - 输出：`0 issues.`
- 通过：`pnpm --dir frontend run lint:check`
- 通过：`pnpm --dir frontend run typecheck`
- 通过：`pnpm --dir frontend run test:run`
  - `97` 个测试文件、`567` 个测试通过。
- 通过：`git tag --merged upstream/main --sort=-version:refname | head -1`
  - 输出：`v0.1.120`
- 通过：`cat backend/cmd/server/VERSION`
  - 输出：`0.1.120`
- 通过：`bash deploy/version_resolver.sh resolve .`
  - 输出：`0.1.120`
- 通过：`POSTGRES_PASSWORD=dummy docker compose -f deploy/docker-compose.deploy.yml config -q`
- 通过：`POSTGRES_PASSWORD=dummy docker compose -f deploy/docker-compose.local.yml config -q`
- 通过：`make build`
  - Go build 使用 `-X main.Version=0.1.120`。
  - 前端 Vite build 成功；输出既有 chunk-size / dynamic import warning，无构建失败。
- 通过：`bash scripts/collect_upstream_sync_context.sh --no-fetch`
  - 合入后 HEAD：`ec41b5e091c685a182a6a72cabc7ecd896444f4c`
  - `local_version=0.1.120`
  - `upstream_version=0.1.120`
  - `latest_upstream_tag=v0.1.120`
- 部署后版本输出 / 页面版本人工核对：
  - 尚未执行，属于后续 `apipool-push-deploy` 发布验证范围。

## 剩余风险与观察点

- 本轮是 upstream sync，线上行为变化面较大，重点观察：
  - OpenAI OAuth/Codex：`reasoning` replay item 被丢弃后，应减少 `rs_* not found` 404；同时需观察 compact / tool replay / WebSocket continuation 是否有回归。
  - OpenAI Fast/Flex Policy：新增 admin 配置项和 gateway 策略，默认无规则时不应改变现有请求。
  - Vertex Service Account：新增账号类型，不应影响现有 Anthropic OAuth/API Key/Kiro 账号调度。
  - Anthropic stream EOF/failover：上游断流时 client-visible error 变为 Anthropic 标准 SSE error；需观察客户端解析差异。
  - ops retention days = 0：该语义会清空目标表，应避免生产误设。
  - account bulk update filter-target：仍需坚持按平台/账号类型分组批量修改，避免模型映射覆盖。
- 尚未完成的验证：
  - Docker daemon 不可用，integration 的 testcontainers 路径未完整跑完。
  - 本地没有执行生产部署后 `docker exec sub2api /app/sub2api --version` 或页面版本核对。
- 回滚建议：
  - 若部署后应用启动失败或网关出现大面积异常，优先执行镜像快速回滚：`cd /opt/sub2api/deploy && ./rollback.sh image`。
  - 只有在确认数据状态也被本次发布破坏时，才考虑 `./rollback.sh db-restore --with-image`。

## 结论

- 建议保留当前 merge 结果并进入发布流程。
- 理由：
  - 冲突已按“APIPool 本地定制 + upstream v0.1.120 新能力并存”处理。
  - unit、lint、frontend typecheck/test/build、compose/version 链路均已通过。
  - 唯一未通过项是 integration 对 Docker daemon/testcontainers 的环境依赖，不是本轮代码断言失败。
