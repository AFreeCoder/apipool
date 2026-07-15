# 上游同步验证记录（2026-07-15）

## 验证范围

- 当前分支：`main`
- 合入前本地基线：`579166b874c6672f5429c68cf217c8dab998669d`
- 上游引用：`upstream/main`
- 上游 SHA：`d515c3045ce838976ebedab87846aaaf893dbbf6`
- 上游 release tag：`v0.1.156`
- 同步方式：`git merge upstream/main`
- 合并提交：`77a91dbfd22ba5d208538ef76a0322b8dd40b18e`
- 合并提交父节点：本地基线 `579166b874`、上游 `d515c3045`
- 变更规模：254 个文件，31585 行新增、1716 行删除
- 本地最终 `backend/cmd/server/VERSION`：`0.1.156`

本记录只冻结“同步上游并完成本地回归”阶段的结论。按本次任务约定，后续仍需完成两路独立代码评审、统一接收评审结论以及生产部署验收。

## 上游更新摘要

本轮从上游吸收的主要变化如下：

- OpenAI / Codex：新增 Agent Identity 认证与任务恢复，支持无令牌 Agent Identity WebSocket；完善 Responses 首输出超时、SSE 事件边界、拼接 JSON、图片工具、Lite tools、Messages 错误事件和 API Key 5xx failover。
- Grok / xAI：增强 OAuth 池主动刷新与状态和解，补充 Free 账号探测、视觉输入、函数工具缓存、图片模型路由保护和 base URL 安全校验。
- 调度与刷新：增加调度快照生命周期、退役桶清理、降级 outbox 重建以及 token refresh provider 并发、限速、超时和健康门禁。
- 账号与管理后台：增加账号重复创建幂等保护、账号复制接口、分组和 Key 的可选 ID 列，以及 Codex 认证模式选择。
- 前端与静态资源：修复 DataTable 旧行缓存，优化静态资源 immutable cache 范围，并补齐对应 API、组件和回归测试。
- 版本与发布：上游版本推进到 `v0.1.156`；本地继续使用 DigitalOcean、GHCR、部署前数据库备份和回退镜像链路。

## 本地定制保护结果

- 保留 APIPool 的 README、站点入口、DigitalOcean 自动部署、数据库预备份、镜像回退标签和线上健康检查流程。
- 保留 Kiro / OpenClaw 扩展；Anthropic OAuth 与 Kiro 共用 `anthropic` 平台时，使用组合刷新执行器按账号类型分流，避免上游单平台注册覆盖 Kiro。
- 保留 OpenAI `plan_type` 独立巡检、free 账号自动停止调度、本地 Codex 模型别名和既有 OpenClaw 配置生成入口。
- 保留 OpenAI Responses 输出规范化、终态事件已出站标记、上游 User-Agent 规范化和错误透传保护，同时吸收上游首输出超时与事件边界刷新。
- 保留 Grok 已配置代理的 fail-closed 语义、敏感错误脱敏和凭据并发保护，同时吸收上游 OAuth reconciler 与主动刷新池。
- 前端 `UseKeyModal` 同时保留 OpenClaw 导入能力，并吸收上游 Codex legacy / API Key 认证模式切换。

## 显式冲突与取舍

本轮共处理 19 个显式冲突文件：

- `README.md`
- `backend/cmd/server/wire_gen.go`
- `backend/internal/config/config.go`
- `backend/internal/config/config_test.go`
- `backend/internal/handler/admin/grok_oauth_handler_test.go`
- `backend/internal/handler/openai_chat_completions.go`
- `backend/internal/handler/openai_gateway_handler.go`
- `backend/internal/handler/openai_gateway_handler_test.go`
- `backend/internal/service/grok_oauth_service.go`
- `backend/internal/service/grok_quota_service.go`
- `backend/internal/service/image_generation_intent_test.go`
- `backend/internal/service/openai_codex_models_service.go`
- `backend/internal/service/openai_gateway_passthrough.go`
- `backend/internal/service/openai_oauth_passthrough_test.go`
- `backend/internal/service/token_refresh_service.go`
- `backend/internal/service/token_refresh_service_test.go`
- `deploy/config.example.yaml`
- `frontend/src/components/keys/UseKeyModal.vue`
- `frontend/src/components/keys/__tests__/UseKeyModal.spec.ts`

主要取舍：

- README 以 APIPool 生产说明为主，不回退为上游通用部署文档。
- 配置层合并本地 OpenAI plan 巡检字段与上游 token refresh 池的并发、QPS、attempt/cycle timeout 字段及默认值。
- OpenAI 流式路径按“完整 SSE 事件边界 flush”，并只在真实 flush 成功后记录 `response.failed` 已出站，避免重复终态事件。
- Grok 配额探测先补载并验证代理，再获取可能需要刷新令牌的 access token；代理仓储故障返回 503，代理不存在返回 400，均禁止静默直连。
- Agent Identity 可以无令牌进入专用握手；普通 OpenAI 账号缺令牌仍维持不可用语义。
- Wire 重新生成，并显式绑定 `GrokOAuthTokenService`；`KiroTokenProvider` 同时注入账户测试服务。

## 合并后发现并修复的语义衔接点

- 新增组合 OAuth 刷新执行器，避免 Claude OAuth 与 Kiro 在同一平台下互相覆盖。
- 修复 OpenAI Responses 普通流与 passthrough 流的 failed 终态标记作用域和事件边界刷新。
- 修复 Grok 配额探测在账号代理边未预加载时的安全补载顺序。
- 补齐 `AccountTestService`、`AccountHandler`、`GatewayService`、`GrokOAuthHandler` 等新增构造参数及测试桩。
- 保留本地 `response.output=[]` 规范化，并调整上游新增的 flush 测试，使其只验证边界而不误判既有兼容语义。
- 将上游首输出临时文件测试限制到 `t.TempDir()` 生成并记录的路径，消除 gosec G703 告警。

## 测试记录

以下命令均使用 Go `1.26.4`，最终结果全部通过：

- `cd backend && GOTOOLCHAIN=go1.26.4 go test -tags=unit ./...`
- `cd backend && GOTOOLCHAIN=go1.26.4 make test-unit`
- `cd backend && GOTOOLCHAIN=go1.26.4 go test -tags=integration ./...`
- `cd backend && GOTOOLCHAIN=go1.26.4 make test-integration`
- `cd backend && GOTOOLCHAIN=go1.26.4 golangci-lint run ./...`：`0 issues`
- `pnpm --dir frontend run lint:check`
- `pnpm --dir frontend run typecheck`
- `pnpm --dir frontend run test:run`：160 个测试文件、1114 项测试通过
- `cd backend && GOTOOLCHAIN=go1.26.4 go test -tags=unit ./internal/service -run 'Kiro|GatewayServiceKiro|AccountTestService'`
- `cd backend && GOTOOLCHAIN=go1.26.4 go test -tags=unit ./internal/handler/admin -run Kiro`
- `make build`：后端二进制和前端 Vite 生产构建通过
- `POSTGRES_PASSWORD=dummy docker compose -f deploy/docker-compose.deploy.yml config -q`
- `POSTGRES_PASSWORD=dummy docker compose -f deploy/docker-compose.local.yml config -q`
- `bash deploy/version_resolver.sh resolve .`：输出 `0.1.156`
- `bash /Users/afreecoder/project/my-skills/skills/apipool-sync-upstream/scripts/collect_upstream_sync_context.sh --no-fetch`

前端测试输出包含既有的模拟异常日志、Vue 组件 stub 警告、Browserslist 数据陈旧提示和 Vite chunk size 警告；它们未导致测试、类型检查或构建失败。

## 版本链路

- `git tag --merged upstream/main --sort=-version:refname | head -1`：`v0.1.156`
- `upstream/main:backend/cmd/server/VERSION`：`0.1.156`
- 本地 `backend/cmd/server/VERSION`：`0.1.156`
- `deploy/version_resolver.sh`：`0.1.156`
- 上游已自行完成 VERSION 推进，因此没有另做版本对齐提交。

## 剩余风险与观察点

- 本轮改动规模大，且命中 OpenAI、Grok、调度、账号幂等、前端和部署链路；必须完成用户指定的两路独立代码评审后再推送。
- Agent Identity、首输出 timeout、Grok OAuth reconciler 与调度快照生命周期是新增高风险路径，部署后应重点观察 failover、账号 schedulable 状态、刷新错误率和首输出超时日志。
- 生产是单实例 Compose 重建，不是零中断滚动发布；部署期间仍可能出现短暂连接中断。
- 当前工作区仅剩用户已有的未跟踪 `.agents/` 目录，未纳入本次提交。

## 结论

同步阶段验证通过，建议将 `77a91dbfd22ba5d208538ef76a0322b8dd40b18e` 作为双路代码评审的共同基线；在评审结论统一接收并处理前，不推送、不部署。
