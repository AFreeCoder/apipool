# 上游同步验证记录（2026-08-01）

## 基线

- 同步分支：`codex/upstream-sync-20260801`
- 合入前本地基线：`7a103700255b28e1e1de9025568cb38caca3bbb7`
- 上游引用：`upstream/main`
- 上游 SHA：`b74024c7868ee88a0bf921306cbc22a2f922872a`
- merge-base：`2730c1c43b29be003925b033f3f9e645e726bb8c`
- 同步方式：`git merge --no-ff --no-commit upstream/main`
- 同步 merge commit：`796c8ffcb559fb49d0d7ca13060d744033bbb56b`
- 评审修复 commit：`7fb4f44b8cec7fad31d5846a49fadc182f7ad224`
- 上游独有提交：187 个
- 本地独有提交：253 个
- upstream 最新 tag：`v0.1.169`
- 最终 `backend/cmd/server/VERSION`：`0.1.169`
- 最终 Go toolchain：`1.26.4`

## 上游更新摘要

- 认证与配置：新增 Passkey/WebAuthn 登录、部署就绪校验、配置项和管理开关。
- 模型目录：首页 compact preset 与按分组定价的模型广场，补充导航、筛选、定价表和权限控制。
- OpenAI/Anthropic/Grok：增加 GPT-5.6 effort、Chat/Responses/WS 兼容、流式重试、工具图片桥接、路径防护和 usage 记录修复。
- 仓储与账号：更新用户/API Key 声明列写入、lost-update 集成覆盖、账号批量删除和筛选结果全选。
- 计费与订阅：补充模型价格、套餐周期配额、支付方式保存和多币种统计修复。
- 安全与运维：面板限流、Prompt Audit 配置恢复、内容审核代理、容器 `no-new-privileges`、Caddy SSE 压缩保护。
- 数据库：新增 migration 191，用于 Passkey credential 持久化。

## 本地定制保护点

- 保留 APIPool README、站点品牌、生产入口及 `apipool_vps` 自托管 Runner 发布链，不接受上游 sponsor/默认品牌覆盖。
- Wire 与服务生命周期同时保留本地 ReqLog、Kiro 等定制，并接入上游 Passkey、Model Plaza、JWT denylist、panel rate limiter。
- 网关保持本地 ReqLog 可观测性，同时在分组校验前完成 Composite 解析；Responses 子路径继续使用上游严格 guard。
- 图片存储同时保留上游 data URL 支持与本地 HTTPS/重定向 SSRF 防护、大小上限和真实图片 MIME 校验。
- Anthropic → Chat Completions 同时保留 GPT-5.6 `max` 上游语义和本地实际 effort/usage 记录。
- Go 版本继续使用项目 CI 要求的 `1.26.4`。

## 冲突与取舍

显式 Git 冲突共 14 个：

- `.github/workflows/backend-ci.yml`：同时运行本地目标部署契约与上游 Compose/Caddy 安全测试。
- `README.md`：保留 APIPool 品牌、域名与部署说明，不引入 sponsor 块。
- `backend/cmd/server/wire_gen.go`：重新生成，保留本地服务并接入上游新增依赖。
- `backend/internal/handler/endpoint.go`：保留类型安全的 inbound endpoint key，同时吸收上游 endpoint 语义。
- `backend/internal/server/http.go`、`backend/internal/server/router.go`：合并本地生命周期/安全响应与上游 Passkey、Model Plaza、JWT 和限流接线。
- `backend/internal/server/routes/gateway.go`：保留 ReqLog，接入 Composite resolver 与 Responses 子路径 guard。
- `backend/internal/service/image_storage.go` 及测试：合并 data URL 与本地 SSRF/MIME 防线。
- `backend/internal/service/openai_gateway_messages_chat_fallback.go`：保留本地 usage effort，同时吸收上游 GPT-5.6 `max`。
- 中英文 settings i18n 与 `frontend/src/utils/featureFlags.ts`：保留本地 step-up/订阅开关，合入 Passkey 与 Model Plaza。

无冲突但经专项验证后调整：

- 公共 settings API contract 补齐 Passkey、compact home 和 Model Plaza 字段。
- ReqLog 移到 Composite resolver 前，确保 resolver 失败仍留下一条状态日志。
- 图片上传拒绝声明为 `image/*` 但实际不是图片的数据。
- 生产、本地扩展 Compose 全部吸收 `no-new-privileges`，并扩展契约测试覆盖。

## 生成、版本与部署契约

- Wire 使用 `go run -mod=mod github.com/google/wire/cmd/wire ./cmd/server` 重新生成并通过编译。
- `bash scripts/collect_upstream_sync_context.sh --no-fetch`：确认 187 个上游独有提交、tag 和版本文件一致。
- `git tag --merged upstream/main --sort=-version:refname | head -1`：`v0.1.169`。
- `bash deploy/version_resolver.sh resolve .`：`0.1.169`。
- deploy/local/biz/test Compose 均通过 `docker compose config -q`。
- 目标部署脚本语法、目标部署测试、Compose 安全与资源测试、Caddy 压缩/SSE 测试全部通过。

## 测试记录

- `go test -count=1 -tags=unit ./...`：通过。
- `DOCKER_HOST=unix:///Users/afreecoder/.colima/default/docker.sock TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock go test -count=1 -tags=integration ./...`：通过。
- `golangci-lint run ./...`：通过，`0 issues`。
- `pnpm install --frozen-lockfile`：通过，lockfile 无变化。
- `pnpm run lint:check`：通过。
- `pnpm run typecheck`：通过。
- `pnpm run test:run`：通过，206 个测试文件、1467 个用例全绿。
- `make build`：通过；后端嵌入版本 `0.1.169`，前端生产构建完成。
- `git diff --check`：通过。

## 失败—修复—复验记录

- 初次公共 settings contract 测试发现四个新字段未暴露；补齐 DTO 后定向与完整 unit 通过。
- 初次网关定向测试发现 Composite resolver 错误发生在 ReqLog 之前；调整顺序后 resolver 500 与分组校验语义均通过。
- 初次 Caddy 契约测试因本地双站点配置与上游单站点假设冲突而失败；企业站同时仍包含 SSE 压缩风险。统一两个站点策略并逐块验证后通过。
- 评审发现 GPT-5.6 `max` 实际转发正确但 usage 元数据被归一成 `xhigh`；改用 model-aware extractor 后专项及无缓存全量 unit 通过。
- 评审发现生产 Compose 未吸收上游容器权限加固；补齐三份本地 Compose 并扩展测试后，安全契约和 Compose 解析通过。
- 评审发现 Passkey/模型广场默认品牌回归；统一 APIPool 默认并补测试后，前端完整回归增加到 206 个文件、1467 个用例且全绿。
- 一次大范围 unit 运行曾观察到 GPT-5.6 effort 用例失败、隔离运行通过；完成实际修复后，以 `-count=1` 禁用缓存重跑完整 unit，稳定全绿。

## 剩余风险与发布边界

- `docs/test/upstream-sync/issues.md` 中 Kiro 主动刷新、异步图片持久队列、鉴权快照完整失效和 Prompt Audit 隐私治理遗留继续有效。
- 本轮改变 `deploy/docker-compose.deploy.yml`；推送前必须由 owner 从本候选安装 root-owned 生产工具链，否则自托管 Runner 会按设计 fail-closed。
- 生产发布必须验证 Actions 精确 SHA、`release.env`、容器健康/镜像 ID、发布前数据库备份、rollback metadata、Caddy 与真实入口。

## 结论

上游同步、冲突解决、完整回归及两轮双路独立评审均已完成；第 2 轮双方均确认无新缺陷，可以进入 `apipool-push-deploy` 发布门禁。
