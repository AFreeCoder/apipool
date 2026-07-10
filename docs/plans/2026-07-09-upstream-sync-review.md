# 上游同步评审记录（2026-07-09）

## 基线

- 当前分支：`main`
- 当前 HEAD：`42d971a349da615102454f2c420bce1476e54889`
- 合入前本地基线：`42d971a349da615102454f2c420bce1476e54889`
- 上游引用：`upstream/main`
- 上游 SHA：`12d811bd76572836d6df6e1fa8aa5ff91be3b12e`
- merge-base：`6f43986c376d76144cb39c7a562c179e19ac7439`
- 同步方式：`git merge upstream/main`
- upstream 最新 release tag：`v0.1.149`
- upstream/main 版本文件：`0.1.149`
- 本地最终 VERSION：`0.1.149`

## 上游更新摘要

- 合入 `upstream/main` 从 `6f43986c376d76144cb39c7a562c179e19ac7439` 到 `12d811bd76572836d6df6e1fa8aa5ff91be3b12e`，覆盖 upstream `v0.1.147` / `v0.1.149`。
- 主要吸收：Codex models manifest 透传接口、compact body-signal SSE bridge、`response.failed` 透传与 failover 对齐、Grok 4.5 / Grok video per-second billing、用量页延迟健康与用户 Token 排行、用户角色编辑、site logo / doc_url sanitize、版本回退指引。
- 高风险模块：OpenAI gateway / passthrough / usage billing / rate limit、ops error logging、GitHub release/version service、admin usage UI、AppHeader/AppSidebar、README_CN/README_JA、`backend/cmd/server/VERSION`。
- upstream 将 Go 工具链推进到 1.26.5；本仓库 AGENTS 仍要求 CI 使用 Go 1.26.4，因此本次保留本地 Go 1.26.4 约束，只合入业务代码与测试变化。

## 本地定制保护点

- 品牌与文案：英文 `README.md` 保留 APIPool 入口、域名与部署说明；`HomeView` / `KeyUsageView` 默认站点名仍为 `APIPool`，但吸收 upstream 对 `site_logo` / `doc_url` 的 `sanitizeUrl` 处理。`README_CN.md` / `README_JA.md` 仍是已知 Sub2API 文档债务，本轮只接受 upstream sponsor 增量，不做局部品牌重写。
- 部署/回滚/版本链路：保留本地 `.github/workflows/deploy.yml`、`deploy/rollback.sh`、`deploy/version_resolver.sh`、`deploy/docker-compose.deploy.yml` 的 DigitalOcean/GHCR/备份/rollback 语义；最终运行时版本为 `0.1.149`，与 upstream tag 对齐。
- OpenAI OAuth / Codex 兼容：保留 APIPool 的 `codexCLIVersion=0.125.0` 作为统一版本源，新增 Codex models manifest 默认值改用该常量；passthrough 同时保留本地 `normalizeOpenAIResponsesOutputJSONBytes` 与 upstream compact SSE bridge。
- 后台入口与默认配置：保留 AppHeader 跑马灯、Kiro/OpenClaw 相关代码与部署请求日志总开关；UsageTable 同时保留默认关闭 IP Geo 和 upstream `flat` 嵌入模式。

## 冲突与取舍

- Git 冲突文件：`README.md`、`backend/internal/server/middleware/api_key_auth_google.go`、`backend/internal/service/openai_gateway_passthrough.go`、`backend/internal/service/openai_gateway_usage.go`、`backend/internal/service/ops_upstream_context.go`、`frontend/src/components/admin/usage/UsageTable.vue`、`frontend/src/components/layout/AppHeader.vue`、`frontend/src/utils/billingMode.ts`、`frontend/src/views/HomeView.vue`、`frontend/src/views/KeyUsageView.vue`。
- `openai_gateway_usage.go` 同时保留 APIPool 的 `gpt-image-2` token billing 开关和 upstream Grok video per-second billing；video 非 token 配置优先走视频计费，image 逻辑继续尊重渠道显式模式与全局开关。
- `ops_upstream_context.go` 同时保留本地 `OpenAIStreamTerminalEventForwarded` 标记和 upstream `OpsStreamError`，并修复 `shouldSkipOpsErrorLog` 调用签名。
- `openai_codex_models_service.go` 修复 upstream 新增接口引用不存在的 `openAICodexProbeVersion`，改用本地既有 `codexCLIVersion`。
- 无冲突但复核的热点：Kiro/OpenClaw 文件未出现删除型 diff；deploy/rollback/version 关键文件未被 upstream 覆盖；Go 1.26.5 相关 workflow/Docker/go.mod 变更已按本仓库约束回退。

## 测试记录

- `cd backend && go generate ./ent`：通过；补齐 `backend/go.sum` 中生成工具链依赖校验条目。
- `cd backend && go test -tags=unit ./...`：通过。前两轮暴露并已修复 `openAICodexProbeVersion` 未定义、`shouldSkipOpsErrorLog` 调用参数不足。
- `cd backend && make test-unit`：通过。
- `cd backend && go test -tags=integration ./...`：通过。
- `cd backend && make test-integration`：通过。
- `cd backend && golangci-lint run ./...`：通过，`0 issues`。
- `pnpm --dir frontend run lint:check`：通过。
- `pnpm --dir frontend run typecheck`：通过。
- `pnpm --dir frontend run test:run`：通过，`146` 个测试文件、`932` 个用例通过；输出包含既有测试 stderr / Browserslist 过期提示。
- `git tag --merged upstream/main --sort=-version:refname | head -1`：`v0.1.149`。
- `cat backend/cmd/server/VERSION`：`0.1.149`。
- `make build`：通过；Vite 输出既有动态导入/chunk size warning。
- `POSTGRES_PASSWORD=dummy docker compose -f deploy/docker-compose.deploy.yml config -q`：通过。
- `POSTGRES_PASSWORD=dummy docker compose -f deploy/docker-compose.local.yml config -q`：通过。
- `bash deploy/version_resolver.sh resolve .`：`0.1.149`。
- `bash scripts/collect_upstream_sync_context.sh --no-fetch`：通过；显示最终 `local_version=0.1.149`、`upstream_version=0.1.149`。
- 部署后版本输出 / 页面版本人工核对：待 push-deploy 后通过 DigitalOcean 容器与线上 `/health` 验证。

## 剩余风险与观察点

- 上游改动覆盖 OpenAI/Grok/Codex 计费与错误透传主链路，虽本地回归通过，仍建议部署后重点观察 `ops_error_logs`、Grok video usage billing、Codex manifest 路由和 `response.failed` 流式错误透传。
- `README_CN.md` / `README_JA.md` 继续保留 Sub2API 文档债务；本轮不做品牌重写。
- 发布后需确认 GitHub Actions 备份、`rollback-latest`、`last-rollback-image.txt`、容器健康和线上版本。

## 结论

- 建议保留当前 merge 结果并进入双 reviewer 阶段；本地回归已覆盖后端 unit/integration、lint、前端 lint/typecheck/test、构建、compose config 与版本解析。
