# 上游同步验证记录（2026-07-22）

## 基线

- 同步分支：`codex/upstream-sync-20260722`
- 合入前本地基线：`bd8371ae82c61ba0c5f759f13b21556b66d782fa`
- 上游引用：`upstream/main`
- 上游 SHA：`63cef605940c9acf0ef6f1827065877f18952c5d`
- merge-base：`b8e844f4ee130ac069a7c5713c2413233186b83f`
- 同步方式：`git merge --no-ff upstream/main`
- 上游独有提交：109 个
- 本地独有提交：241 个
- upstream 最新 tag：`v0.1.163`
- 最终 `backend/cmd/server/VERSION`：`0.1.163`
- 最终 Go 版本：`1.26.4`

## 上游更新摘要

- OpenAI/Codex：新增按分组限制与映射 reasoning effort、补充 Responses 客户端工具协议、WS 请求头透传与模型/身份兼容修复。
- Grok：补充 Responses 工具调用往返、compact 适配、错误分类、内容策略 403 隔离、billing 临时错误单次重试。
- 调度与缓存：补充筛选原因诊断、账号 `LastUsedAt` 旁路缓存、配额元数据与 API Key 鉴权缓存版本更新。
- 计费：从 Responses `tool_usage.image_gen` 合并图片输入/输出 token，用现有 usage 计费路径结算。
- 运维与依赖：增加 Redis ACL username 配置，升级 Axios、`x/text` 等依赖，并更新前端移动端交互。
- 版本与文档：吸收 `v0.1.163` 版本文件、支付文档与上游截图资源。

## 本地定制保护点

- 保留 APIPool 主 README、日文 README、本地 SVG 品牌标识与兼容用 `frontend/public/logo.png`，未把本地默认镜像改为 `weishaw/sub2api:latest`。
- `.github/workflows/deploy.yml`、`deploy/docker-compose.deploy.yml`、`deploy/docker-compose.biz.yml`、`deploy/rollback.sh` 与 `deploy/version_resolver.sh` 相对本地基线无差异。
- Kiro OAuth 路由、前端账号配置、OpenClaw 配置导入和 ReqLog 中间件/下载路由仍在接线。
- Redis 连接继续复用 APIPool 的统一选项构造，只在该路径加入上游 `Username` 支持。
- Go 版本继续保持项目 CI 要求的 `1.26.4`，未接受上游文档中的旧版本回退。
- Grok billing 错误继续执行 APIPool 的错误体脱敏策略，响应体不得进入错误或日志。

## 冲突与取舍

显式冲突文件共 8 个：

- `README.md`、`README_JA.md`：保留本地 APIPool 说明与安全文档。
- `frontend/public/logo.svg`：保留本地品牌资源；同时恢复上游删除的兼容用 PNG。
- `backend/internal/handler/admin/account_codex_agent_identity_import_test.go`：保留本地凭据清理/过期测试，并吸收上游 Team 隔离测试。
- `backend/internal/repository/redis.go`：保留统一 Redis 选项构造，加入 ACL username。
- `backend/internal/server/routes/gateway_test.go`：保留本地 `settingService` 构造参数。
- `backend/internal/service/grok_quota_service.go`：吸收临时错误单次重试，保留错误体脱敏。
- `backend/internal/service/openai_account_scheduler.go`：吸收上游筛选原因统计，并保持仅“全部候选均不支持模型”时返回 `model_not_supported_in_group` 的本地语义。

无冲突但经语义核验后调整：

- 将 Grok compact 辅助函数重命名为 `grokCompactStringValue`，避免与本地 Kiro 辅助函数重名。
- 补齐本地 `noAvailableOpenAISelectionError` 的新增参数调用。
- 修正配额暂停、传输不兼容和空池被误报为“分组不支持模型”的分类回归，并增加/调整定向断言。
- 保留 `deploy/docker-compose.yml` 的本地 `sub2api:latest`，仅吸收 Redis ACL username。
- 将上游新增“重试后错误包含响应体”的测试改为验证错误体不外泄，保持安全基线一致。

## 生成与静态核验

- `GOTOOLCHAIN=go1.26.4 go generate ./ent ./cmd/server`：通过；生成结果与最终工作树一致。
- `git diff --check`：通过。
- 生产部署 workflow、compose、回滚与版本解析脚本相对本地基线：无差异。
- `bash scripts/collect_upstream_sync_context.sh --no-fetch`：通过；确认上游 109 个提交、最新 tag 与最终版本均为 `v0.1.163` / `0.1.163`。

## 测试记录

- `GOTOOLCHAIN=go1.26.4 go test -count=1 -tags=unit ./...`：通过。
- `DOCKER_HOST=unix:///Users/afreecoder/.docker/run/docker.sock GOTOOLCHAIN=go1.26.4 go test -count=1 -tags=integration ./...`：通过；使用 Docker Engine `20.10.12` 运行 Testcontainers。
- `GOTOOLCHAIN=go1.26.4 golangci-lint run ./...`：通过，`0 issues`。
- `pnpm --dir frontend run lint:check`：通过。
- `pnpm --dir frontend run typecheck`：通过。
- `pnpm --dir frontend run test:run`：通过，188 个测试文件、1347 个用例全绿。
- `GOTOOLCHAIN=go1.26.4 make build`：通过，后端与前端生产构建成功；仅有既有 Vite chunk size / 动静态导入提示。
- `POSTGRES_PASSWORD=sync-validation docker compose -f deploy/docker-compose.deploy.yml config -q`：通过。
- `POSTGRES_PASSWORD=sync-validation docker compose -f deploy/docker-compose.local.yml config -q`：通过。
- `bash deploy/version_resolver.sh resolve .`：通过，输出 `0.1.163`。

## 失败—修复—复验记录

- 初次完整单元测试在 `TestGrokQuotaServiceFetchBillingStopsAfterSingleTransientRetry` 失败：上游测试期望错误包含响应体，与本地脱敏红线冲突。保留安全实现、修正测试后，定向用例与最终完整单元套件均通过。
- 初次集成测试因 Docker daemon 未启动而在 Testcontainers 初始化阶段失败；启动本机 Docker Desktop、确认引擎可连接后，最终完整集成套件通过。
- 独立 worktree 初始缺少 `node_modules`；按 CI 使用 `pnpm install --frozen-lockfile` 恢复依赖后，前端三项检查及构建全部通过。

## 剩余风险与发布边界

- 本记录只覆盖本地同步与回归，不代表生产发布批准；必须继续完成两路独立代码评审及反馈接收。
- `docs/test/upstream-sync/issues.md` 中既有 Kiro 主动刷新、异步图片持久队列、鉴权缓存完整失效和 Prompt Audit 隐私治理遗留仍然有效，本次同步未扩大其生产启用范围。
- 上游中文 README 仍保留部分 Sub2API 品牌内容，属于既有文档债务；主 README、运行时默认品牌和本地静态资源已保持 APIPool。

## 结论

同步结果通过本地门禁，建议进入双路独立代码评审。评审与反馈接收完成前，不建议推送 `main` 或触发生产部署。
