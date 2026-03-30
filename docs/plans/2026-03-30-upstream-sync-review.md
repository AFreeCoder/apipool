# 上游同步评审记录（2026-03-30）

## 基线

- 当前分支：`main`
- 当前 HEAD：`1057f303b3bf2f6a88c3a722fd00e41b6a4f1523`
- 合入前本地基线：`1057f303b3bf2f6a88c3a722fd00e41b6a4f1523`
- 上游引用：`upstream/main`
- 上游 SHA：`1dfd9744329929c717d5bc9b7956c9505f774930`
- merge-base：`fdd8499ffc3f434f597b61decd4f7db3bd82e304`
- 同步方式：`git merge upstream/main`
- upstream 最新 release tag：`v0.1.106`
- upstream/main 版本文件：`0.1.106`
- 本地最终 VERSION：`0.1.106`

## 上游更新摘要

- 本轮吸收了 `upstream/main` 自 `fdd8499f` 之后的 35 个提交，核心变化包括：
  - OpenAI / Codex 兼容：规范化 `gpt-5.4-xhigh` 等兼容模型路由，计费回到“按用户请求模型”而非映射后的上游模型。
  - OpenAI OAuth 刷新：当账号无 `refresh_token` 但仍保留可用 `access_token` 时，不再误判为可重试失败；对应后台刷新逻辑把 `no refresh token available` 视为不可重试状态，避免重复临时不可调度。
  - Claude OAuth：新增 `user:file_upload` scope，并改用标准 PKCE code verifier 生成方式。
  - 后台与账号：清除账号错误时一并清除限流、临时不可调度和 antigravity quota/model rate limit 状态；软删除 API Key 时释放原 key 唯一约束；Sora S3 文案补齐 `bucket` 字段。
  - Antigravity：新增连续 INTERNAL 500 渐进惩罚缓存与测试。
  - 配置与文档：`VERSION` 推进到 `0.1.106`，README/多语言文档更新，定价数据源默认改为跟随 `model-price-repo` 主分支。
- 本轮命中的高风险模块：
  - `README.md`
  - `backend/internal/handler/openai_gateway_handler.go`
  - `backend/internal/service/openai_gateway_service.go`
  - `backend/internal/service/openai_oauth_service.go`
  - `backend/internal/service/token_refresh_service.go`
  - `backend/internal/config/config.go`
  - `deploy/config.example.yaml`

## 本地定制保护点

- 品牌与文案：
  - `README.md` 冲突处保留了 APIPool 的品牌、DigitalOcean 部署说明、运维命令、本地开发凭据和常见坑点，没有被上游 Sub2API README 重新覆盖。
  - `README_CN.md` / `README_JA.md` 吸收了上游新增赞助位，但未影响 APIPool 主 README 的品牌主线。
- 部署 / 回滚 / 版本链路：
  - 本轮未让上游覆盖 APIPool 的 `deploy/`、回滚流程和 DigitalOcean 单实例 Compose 假设。
  - `backend/cmd/server/VERSION` 在 merge 结果中保持为 `0.1.106`，与 `upstream/main` 和最新 release tag 一致。
  - 版本对齐未拆成独立提交，因为 `upstream/main` 自身已经携带 `VERSION=0.1.106`，merge 结果天然对齐 `v0.1.106`。
- OpenAI OAuth / Codex 兼容：
  - 保留了 APIPool 已有的错误映射、failover、session 推导和 OpenAI/Codex 兼容路径，同时吸收了上游对 compat model 路由、无 refresh token 处理和 OAuth scope/PKCE 的修复。
  - `token_refresh_service_test.go` 冲突解决时同时保留了本地 plan sync 测试桩与上游新增的 `SetTempUnschedulable` 测试能力。
- 后台入口与默认配置：
  - 上游这轮没有触及 `SettingsView`、`BackupView`、`deploy/rollback.sh` 等 APIPool 高风险后台/部署定制文件。
  - 对定价配置默认值做了额外对齐：把 `config.go` 与 `deploy/config.example.yaml` 统一到主分支 URL，并修正文案避免继续写成“固定 commit”。

## 冲突与取舍

- Git 冲突文件与处理方式：
  - `README.md`：放弃上游 README 主体重写，保留 APIPool README 全量本地语义，仅继续吸收其它文档更新到多语言 README。
  - `backend/internal/service/token_refresh_service_test.go`：手动合并本地 `tokenRefreshPlanSweepRepo` 测试桩与上游新增的 `SetTempUnschedulable` stub / `no refresh token` 断言。
- 无冲突但做过语义复核的热点文件：
  - `backend/internal/handler/openai_gateway_handler.go`：确认 compat model 规范化只影响消息路由，不覆盖 APIPool 现有 failover/错误透传。
  - `backend/internal/service/openai_gateway_service.go`：确认“按原始请求模型计费”的上游修复与本地零倍率免单逻辑不冲突。
  - `backend/internal/service/openai_oauth_service.go` 与 `backend/internal/service/token_refresh_service.go`：确认无 refresh token 时不再误打临时不可调度，并保留 plan sync / privacy 等本地行为。
  - `backend/internal/config/config.go` 与 `deploy/config.example.yaml`：额外修正 URL 与注释，使默认值、样例和当前 pricing 同步逻辑一致。
  - `backend/internal/service/admin_service.go`、`backend/internal/repository/api_key_repo.go`：接受上游修复，分别用于清理账号限流状态与释放软删除 API Key 的唯一键。

## 测试记录

- 后端：
  - `cd backend && go test ./...`：通过
  - `cd backend && make test-unit`：通过
  - `cd backend && make test-integration`：失败，原因是当前环境 Docker daemon 不可用，`testcontainers-go` 无法连接 `unix:///var/run/docker.sock` / `unix:///Users/afreecoder/.docker/run/docker.sock`
  - 当前直接暴露的失败用例包括：
    - `internal/middleware.TestRateLimiterSetsTTLAndDoesNotRefresh`
    - `internal/server/routes.TestAuthRegisterRateLimitThresholdHitReturns429`
  - `cd backend && golangci-lint run ./...`：通过（`0 issues`）
- 前端：
  - `cd frontend && pnpm run lint:check`：通过
  - `cd frontend && pnpm run typecheck`：通过
  - `cd frontend && pnpm run test:run`：通过（53 个测试文件、318 个测试全部通过）
- 版本链路与部署校验：
  - `bash deploy/version_resolver.sh resolve .`：输出 `0.1.106`
  - `POSTGRES_PASSWORD=dummy docker compose -f deploy/docker-compose.deploy.yml config -q`：通过
  - `POSTGRES_PASSWORD=dummy docker compose -f deploy/docker-compose.local.yml config -q`：通过
  - `make build`：通过；Vite 构建过程中仅出现既有的大 chunk 警告，未出现构建失败
  - `scripts/collect_upstream_sync_context.sh --no-fetch`：通过，版本链路复核为 `v0.1.106` / `0.1.106` / `0.1.106`

## 剩余风险与观察点

- 尚未验证的风险：
  - 未在当前会话内启动 Docker daemon，因此 `make test-integration` 无法完成，仍需在可用 Docker 环境补跑。
  - 未执行真实部署，尚未核对线上容器 `--version` 输出及页面左上角展示版本。
  - `make build` 的前端产物仍有 >500 kB chunk 警告，这不是本轮引入的问题，但值得继续观察打包体积。
- 灰度 / 回滚建议：
  - 推送前先在有 Docker daemon 的环境重跑 `cd backend && make test-integration`。
  - 部署后优先核对 4 件事：`git tag` 最新版本、`backend/cmd/server/VERSION`、`deploy/version_resolver.sh` 解析值、线上运行中的二进制/页面展示版本。
  - 如新引入的 OpenAI compat model 路由或 OAuth 刷新行为出现异常，优先观察 OpenAI 网关日志与 token refresh 周期日志；若需要回退，沿用现有 `deploy/rollback.sh image` / `db-restore --with-image` 流程。

## 结论

- 建议保留当前 merge 结果。
- 在可用 Docker 环境补跑 `make test-integration` 并完成一次实际部署版本核对后，即可作为本轮 upstream sync 结果提交 / 推送。
