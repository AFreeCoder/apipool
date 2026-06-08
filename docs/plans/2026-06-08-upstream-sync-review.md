# 上游同步评审记录（2026-06-08）

## 基线

- 当前分支：`main`
- 合入提交：`02733b28485357807b27fed5dc01d8b14ac87db2`
- 合入后 lint 修复提交：`0b5c6eea`
- 合入前本地基线：`02ef691874599de254038bf2c294d22ab9c5c4fa`
- 上游引用：`upstream/main`
- 上游 SHA：`0aad6030130c02cadb8b70e6cc90c9ed04bb1a7a`
- merge-base：`635ad81cdcad5fced96afd70afa6c1483dc0f118`
- 同步方式：`git merge upstream/main`
- upstream 最新 release tag：`v0.1.135`
- upstream/main 版本文件：`0.1.135`
- 本地最终 VERSION：`0.1.135`

## 上游更新摘要

- 合入 `upstream/main` 到 `0aad6030130c02cadb8b70e6cc90c9ed04bb1a7a`，最新 upstream release tag 为 `v0.1.135`。
- 吸收代理有效期、过期代理失败回退、代理列表有效期展示、账号代理回退提示与恢复入口。
- 吸收 OpenAI / Responses 侧的 sticky group 校验、跨组 `previous_response_id` 剥离、传输层错误 failover 与持久故障临时摘除账号逻辑。
- 吸收 API key 专属分组访问校验、5h usage reset 时间修正、cache create / cache hit token 统计拆分、`account_temp_unscheduled_count` 告警指标。
- 吸收 `skills/sub2api-admin/` 上游管理 skill 资产。
- 命中的高风险模块：`backend/cmd/server/VERSION`、`backend/internal/handler/openai_gateway_handler.go`、`backend/internal/service/openai_gateway_service.go`、`backend/internal/service/openai_gateway_service_test.go`、`frontend/src/views/admin/AccountsView.vue`、`frontend/src/views/admin/ProxiesView.vue`。

## 本地定制保护点

- 品牌与文案：本轮 upstream delta 未覆盖 APIPool 首页、帮助文档、logo、购买入口和长期 APIPool 文案定制。
- 部署/回滚/版本链路：本轮 upstream delta 未改 `.github/workflows/deploy.yml`、`deploy/rollback.sh`、`deploy/docker-compose.deploy.yml`、`deploy/version_resolver.sh`；`backend/cmd/server/VERSION` 已随 upstream 对齐到 `0.1.135`。
- OpenAI OAuth / Codex 兼容：保留 APIPool 既有 `output:null -> output:[]` normalization、Codex/OpenAI OAuth passthrough、Kiro/OpenClaw 相关服务和前端导入逻辑；同时吸收 upstream sticky group 与 transport failover 修复。
- 后台入口与默认配置：保留 APIPool 既有 `/purchase` iframe 充值入口、备份/设置/账号管理定制和表格分页默认值逻辑；吸收 upstream 代理有效期与回退 UI。

## 冲突与取舍

- Git 冲突：无，`git merge upstream/main -m "chore(sync): merge upstream v0.1.135"` 直接完成。
- 语义复核：合并后确认 `upstream/main` 已是当前 HEAD 祖先；`VERSION=0.1.135`，与 upstream tag `v0.1.135` 和 upstream/main 版本文件一致。
- 语义复核：精确扫描 `^(<<<<<<<|=======$|>>>>>>>)` 无 merge marker。
- 语义复核：`ent/schema` 有变更，已执行 `cd backend && go generate ./ent`；本地修复将 `backup_proxy_id` 从 Ent one-to-one 自引用改为多对一自引用，生成代码同步去掉错误的 `Unique: true`。
- 语义复核：`golangci-lint` 暴露 `internal/server/router.go` 在非 embed 构建下的死分支；已将 embedded frontend 中间件构造收敛到 `backend/internal/web/embed_on.go` / `embed_off.go`，router 不再调用固定失败的 stub。

## 双评审反馈与接收处理

- `requesting-code-review` subagent 结论：不建议部署，阻断点是 `UpdateProxy` 对 `ExpiresAt/FallbackMode/BackupProxyID/ExpiryWarnDays` 的部分更新会零值覆盖现有配置；同时指出代理 fallback 可能落到 inactive proxy、Ent schema 与手写迁移对 `backup_proxy_id` 唯一性的定义不一致。
- `gstack review` subagent 结论：不建议部署，阻断点是 Google/Gemini 鉴权路径缺少专属分组撤权校验；同时指出代理过期 sweep 可能用旧快照覆盖管理员刚续期/改状态的代理，以及 `UpdateProxy` PATCH 语义问题。
- 已按 `receiving-code-review` 逐条核验并接受：以上 Critical/Important 反馈均成立。
- 修复：`UpdateProxyInput` 改为 nullable/指针 PATCH 语义，导入流程只传需要变更的 status，避免零值清空代理有效期和 fallback 配置。
- 修复：Google/Gemini API key auth 补齐 `validateAPIKeyGroupAllowed`，专属分组撤权后返回 Google-style 403，并标记 Ops business-limited 原因。
- 修复：代理 fallback 只选择 `active` 且未过期的备用代理；创建/编辑 fallback=proxy 时校验备用代理存在、不能自指、必须 active 且未过期。
- 修复：过期代理 sweep 对 `proxies` 增加条件 UPDATE，仅当代理仍为 active 且 `expires_at <= now` 时才标记 expired；若管理员已续期/改状态，则不再改投账号。
- 修复：Ent 自引用 edge 改为 `fallback_sources -> backup_proxy` 多对一模型，生成 schema 中 `backup_proxy_id` 不再是唯一列。
- 修复：补齐 admin usage 前端响应类型中的 cache creation/read token 字段；修正 `ErrAccountNotInFallback` 注释。

## 测试记录

- 通过：`cd backend && go generate ./ent`。
- 通过：`cd backend && go test -tags=unit ./internal/service -run 'TestResolveFallbackTarget|TestAdminService_UpdateProxy|TestAdminService_CreateProxy'`。
- 通过：`cd backend && go test -tags=unit ./internal/server/middleware -run 'Google.*Exclusive|GoogleRejectsExclusive|ExclusiveGroup'`。
- 通过：`cd backend && go test -tags=unit ./internal/repository -run 'Proxy|Sweep'`。
- 通过：`cd backend && go test -tags=unit ./internal/handler/admin`。
- 通过：`cd backend && go test -tags=unit ./...`。
- 通过：`cd backend && make test-unit`（同步初期执行）。
- 环境阻塞：`cd backend && go test -tags=integration ./...` 失败于本地 Docker daemon 不可用，testcontainers 无法连接 `unix:///Users/afreecoder/.docker/run/docker.sock`；失败包包括 `internal/middleware` 和 `internal/server/routes` 的 Redis rate-limit integration 用例。
- 环境阻塞：`cd backend && make test-integration` 同样失败于本地 Docker daemon 不可用。
- 通过：`cd backend && golangci-lint run ./...`，修复后为 `0 issues`。
- 通过：`pnpm --dir frontend run lint:check`。
- 通过：`pnpm --dir frontend run typecheck`。
- 通过：`pnpm --dir frontend run test:run`，120 个测试文件、732 个用例通过；有预期错误场景 stderr、Element Plus stub warning、browserslist 过期提示和 i18n compiler warning。
- 通过：`pnpm --dir frontend run build`，存在既有 chunk size / dynamic import warning。
- 通过：`git tag --merged upstream/main --sort=-version:refname | head -1` 输出 `v0.1.135`。
- 通过：`cat backend/cmd/server/VERSION` 输出 `0.1.135`。
- 通过：`cd backend && make build`，Go build 使用 `main.Version=0.1.135`。
- 通过：`POSTGRES_PASSWORD=dummy docker compose -f deploy/docker-compose.deploy.yml config -q`。
- 通过：`POSTGRES_PASSWORD=dummy docker compose -f deploy/docker-compose.local.yml config -q`。
- 通过：`bash deploy/version_resolver.sh resolve .` 输出 `0.1.135`。
- 通过：`bash scripts/collect_upstream_sync_context.sh --no-fetch`，确认 upstream tag/version/local version 均为 `0.1.135`。
- 待部署后核对：线上 `docker exec sub2api /app/sub2api --version` 或等价版本输出、页面左上角版本、容器健康和备份/回滚元数据。

## 剩余风险与观察点

- 本地 integration 未完成，原因是本机 Docker daemon 不可用；需要以后在 Docker 可用环境或 CI/部署环境继续以 integration 结果补强。
- 本轮触及代理调度、OpenAI failover、API key group authorization、usage 统计和后台代理 UI，部署后重点观察账号临时不可调度数量、代理过期回退、OpenAI 请求 failover 频率、API key 403 `GROUP_NOT_ALLOWED` 反馈。
- 快速回滚建议仍是先走镜像回滚：`cd /opt/sub2api/deploy && ./rollback.sh image`。仅当确认数据迁移/持久状态异常时才考虑 `db-restore --with-image`。

## 结论

- 已完成上游同步、双 subagent review 和 receiving-code-review 修复。代码层面已通过 unit、lint、前端 lint/typecheck/Vitest/build、后端 build、compose config 和版本解析；未完成项限定在本机 Docker 环境阻塞的 integration。建议进入 `apipool-push-deploy`。
