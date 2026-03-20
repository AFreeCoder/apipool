# Upstream Sync 测试矩阵

本文件定义 APIPool 做 upstream 同步时的默认回归基线。

## 1. 默认全量基线

后端：

- `cd backend && go test -tags=unit ./...`
- `cd backend && go test -tags=integration ./...`
- `cd backend && golangci-lint run ./...`

前端：

- `cd frontend && pnpm install --frozen-lockfile`
- `cd frontend && pnpm lint:check`
- `cd frontend && pnpm typecheck`
- `cd frontend && pnpm test:run`

## 2. 版本链路专项

命中 `VERSION` / release / 部署脚本时，额外检查：

- `cat backend/cmd/server/VERSION`
- `git tag --merged upstream/main --sort=-version:refname | head -1`
- `git show upstream/main:backend/cmd/server/VERSION`

若部署链路被改动，再补：

- `cd backend && go build -tags embed -o /tmp/apipool-sync-check ./cmd/server`
- `docker compose -f deploy/docker-compose.deploy.yml config -q`

## 3. 高风险模块专项

命中 OpenAI OAuth / Codex：

- 复核 `openai_gateway_service.go`、`openai_gateway_chat_completions.go`、`ratelimit_service.go`
- 必要时重跑相关集成/单测，确认 passthrough、prompt cache key、401/403 语义无回退

命中后台设置与品牌：

- 复核 `frontend/src/views/admin/SettingsView.vue`
- 复核 `frontend/src/components/layout/AppHeader.vue`
- 复核 `README.md` / `deploy/README.md`

命中批量编辑与分组逻辑：

- 复核 `frontend/src/components/account/BulkEditAccountModal.vue`
- 复核 `backend/internal/service/admin_service.go`
- 复核 `backend/internal/handler/admin/account_handler.go`

## 4. 本次同步（2026-03-20）实跑结果基线

- 后端 `unit`：通过
- 后端 `integration`：通过
- 后端 `golangci-lint`：通过
- 前端 `pnpm lint:check`：通过
- 前端 `pnpm typecheck`：通过
- 前端 `pnpm test:run`：通过

## 5. 备注

- 当前 CI 工作流里的 Go 版本是 `1.26.1`；若 README / AGENTS 仍写旧版本，需要一并同步
- 当工作区存在用户未提交改动时，不要为了跑 merge 直接 `reset` / `stash`；先确认这些改动是否与 upstream 自分叉点后的变更重叠
