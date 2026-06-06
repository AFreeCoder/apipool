# 上游同步评审记录（2026-06-06）

## 基线

- 当前分支：`main`
- 当前 HEAD：`872a3942143259005ae0666acdddf9616e38c950`
- 合入前本地基线：`99a5f97d023e7385911e38d97f473225597d97a8`
- 上游引用：`upstream/main`
- 上游 SHA：`635ad81cdcad5fced96afd70afa6c1483dc0f118`
- merge-base：`f18451e56f15b31ef602ab238037b56c3522b19f`
- 同步方式：`git merge upstream/main`
- upstream 最新 release tag：`v0.1.134`
- upstream/main 版本文件：`0.1.134`
- 本地最终 VERSION：`0.1.134`

## 上游更新摘要

- 吸收 `upstream/main` 到 `635ad81c`，对应 upstream 最新 tag `v0.1.134`，本地 `backend/cmd/server/VERSION` 已同步为 `0.1.134`。
- 主要上游变化集中在 OpenAI / Codex Responses 兼容、Claude Code 指纹模拟、Responses to Chat Completions 桥接、失败请求记录与错误日志、账号/用量/调度、Linux DO 登录、支付与风险控制、Go toolchain 更新。
- 命中的高风险模块：
  - `backend/internal/service/openai_gateway_service.go`
  - `backend/internal/service/openai_codex_transform.go`
  - `backend/internal/service/openai_ws_forwarder.go`
  - `backend/internal/handler/gateway_handler_responses.go`
  - `backend/internal/handler/ops_error_logger.go`
  - `backend/internal/service/gateway_service_kiro.go`
  - `frontend/src/components/admin/usage/UsageTable.vue`
  - `frontend/src/views/user/UsageView.vue`
  - `deploy/Dockerfile` / `.github/workflows/backend-ci.yml`

## 本地定制保护点

- 品牌与文案：`README.md` 保留 APIPool 中文入口、部署/回滚说明和本地运营说明，没有被 upstream sponsor 文案覆盖。
- 部署/回滚/版本链路：保留 `.github/workflows/deploy.yml` 的 GHCR 镜像构建、DigitalOcean SSH 部署、`deploy-sub2api:rollback-latest`、pre-deploy DB backup、biz compose 重启逻辑；`deploy/rollback.sh` 和 `deploy/version_resolver.sh` 未被回退。
- OpenAI OAuth / Codex 兼容：保留本地 `codexCLIVersion=0.125.0`、Responses `output:null` 归一化、OpenAI model replacement、WS error event 收口和 account binding 逻辑，同时接入 upstream 对 terminal output 重建、response ID 绑定和 Claude Code 指纹的更新。
- 后台入口与默认配置：保留 APIPool 管理端设置、用户错误请求可见性、错误日志归因、Kiro 账号链路和前端用量展示的本地定制行为。

## 冲突与取舍

- Git 冲突文件：
  - `README.md`：保留 APIPool 定制说明，吸收 upstream Go 版本展示更新到 `1.26.4`。
  - `backend/cmd/server/wire_gen.go`：用 `go run -mod=mod github.com/google/wire/cmd/wire ./cmd/server` 重新生成，`backend/go.sum` 补充 wire 依赖校验和。
  - `backend/internal/handler/gateway_handler_responses.go`：保留渠道级模型映射，并使用带 image intent 的 `requestCtx`。
  - `backend/internal/handler/ops_error_logger.go`：同时保留本地错误码推断和 upstream API key fallback。
  - `backend/internal/service/claude_code_validator.go`：保留 count_tokens UA-only 行为，合并 upstream Claude Code 识别注释。
  - `backend/internal/service/openai_codex_transform.go`：保留本地 `sync` / `logger` 逻辑，同时接入 upstream `openai` 包改动。
  - `backend/internal/service/openai_gateway_service.go`：保留本地 Codex CLI 版本和 `output:null` 归一化，同时接入 upstream streaming terminal output 重建、response ID 提取和 account binding。
  - `backend/internal/service/openai_ws_forwarder.go`：保留本地 error event turn cleanup / lease reset，同时吸收 upstream success replay / strict-state / response ID 逻辑。
- 合并后语义修复：
  - `backend/internal/service/gateway_service_kiro.go` 和测试适配 upstream `RequestBodyRef`。
  - `backend/internal/service/claude_code_validator_test.go` 删除重复测试声明。
  - `backend/internal/server/api_contract_test.go` 补充 public settings 新字段 `allow_user_view_error_requests`。
  - `frontend/src/utils/billingMode.ts`、`UsageTable.vue`、`UsageView.vue` 修复历史图片用量缺 `billing_mode` 时被误显示为 Token 计费的问题。

## 测试记录

- 通过：`cd backend && go test -tags=unit ./cmd/server ./internal/service ./internal/handler`
- 通过：`cd backend && go test -tags=unit ./internal/service -run 'TestOpenAIStreamingNormalizesTerminalNullOutput|TestOpenAIStreamingPassthroughNormalizesTerminalNullOutput|TestOpenAINonStreamingNormalizesNullOutput|TestOpenAINonStreamingPassthroughNormalizesNullOutput'`
- 通过：`cd backend && go test -tags=unit ./...`
- 通过：`cd backend && make test-unit`
- 未通过（环境 / 外部依赖）：`cd backend && go test -tags=integration ./...`
  - `internal/middleware` 的 Redis testcontainers 用例失败，原因是 Docker daemon 不可用：`Cannot connect to the Docker daemon at unix:///Users/afreecoder/.docker/run/docker.sock`。
  - `internal/pkg/tlsfingerprint` 访问 `https://tls.peet.ws/api/all` 时 TLS 证书校验失败：`x509: certificate signed by unknown authority`。
  - 后续测试长时间无新增输出，已中断；不作为本轮代码回归结论。
- 未单独运行：`cd backend && make test-integration`
  - 原因：直接入口 `go test -tags=integration ./...` 已因 Docker daemon 和外部 TLS 依赖失败，重复 wrapper 不会增加新信号。
- 通过：`cd backend && golangci-lint run ./...`，结果 `0 issues.`。
- 通过：`pnpm --dir frontend run lint:check`
- 通过：`pnpm --dir frontend run typecheck`
- 通过：`pnpm --dir frontend run test:run`，119 个 test files / 721 tests 通过。
- 通过：`git tag --merged upstream/main --sort=-version:refname`，首项 `v0.1.134`。
- 通过：`cat backend/cmd/server/VERSION`，结果 `0.1.134`。
- 通过：`make build`
  - 后端编译参数为 `-X main.Version=0.1.134`。
  - 前端 Vite build 通过，仅有既有 dynamic import / chunk size / browserslist 警告。
- 未完成（环境阻塞）：`docker compose -f deploy/docker-compose.deploy.yml config -q`
  - Docker CLI 长时间无输出，已终止。
- 未完成（环境阻塞）：`docker compose -f deploy/docker-compose.local.yml config -q`
  - Docker CLI 长时间无输出，已终止。
- 通过：`bash deploy/version_resolver.sh resolve .`，结果 `0.1.134`。
- 通过：`bash scripts/collect_upstream_sync_context.sh --no-fetch`
- 待部署后验证：GitHub Actions deploy run、DigitalOcean 远端 HEAD、pre-deploy DB backup、rollback image metadata、容器健康、线上 `/health` 与页面版本。

## 剩余风险与观察点

- Go 版本事实源存在差异：本轮 upstream 和当前远端链路已升级到 Go `1.26.4`，注入的项目说明仍写 `1.25.7`。本轮选择跟随已同步的 upstream/origin 事实源；如需要固定到 `1.25.7`，应单独开一次工具链回退评估。
- integration 未全绿，主要受本机 Docker daemon 与外部 TLS 探针影响。部署前不能把本机 integration 当作强通过证据，需要依赖 unit/lint/frontend/build/version，加上部署后的生产健康检查。
- OpenAI / Codex / Responses / WebSocket 是本轮最大风险面，已通过定向单测和全量 unit 覆盖 `output:null`、streaming terminal output、WS error event 和 account binding 热点，但部署后仍需观察 Codex OAuth、Responses streaming、失败请求记录和用量统计。
- Kiro `RequestBodyRef` 适配是合并后的语义修复点，已通过 service unit 覆盖，但部署后如出现 Kiro 转发 body 为空或签名异常，应优先回看该路径。
- 前端历史图片用量行的展示规则已修复并通过全量 Vitest；部署后建议抽查管理端和用户端用量明细中 image 计费 tooltip。
- 回滚建议仍以现有部署链路为准：若新容器异常，优先 `cd /opt/sub2api/deploy && ./rollback.sh image`；只有确认数据状态异常时再考虑 DB restore。

## 结论

- 建议保留本轮同步结果并进入双评审 / push deploy 流程。
- 当前阻塞不是代码构建或单元测试失败，而是本机 Docker / 外部 TLS 依赖导致 integration 与 compose config 未能完整跑完。
- 发布前必须继续按 `apipool-push-deploy` 检查部署 workflow、rollback metadata、远端备份和线上健康状态。
