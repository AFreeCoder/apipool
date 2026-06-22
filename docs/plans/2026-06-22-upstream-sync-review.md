# 上游同步评审记录（2026-06-22）

## 基线

- 当前分支：`main`
- 合入后评审基线 HEAD：`cc966a3a878e9832fdbff988a2f3adc874b7be41`
- 评审修复提交：`34d918865437e6ef611df367cf9f1ab775e9ee8c`
- 合入前本地基线：`f6bb9d6550b3b26a71e1a9de6cc33acdbc410759`
- 上游引用：`upstream/main`
- 上游 SHA：`d430343f513aa811be4ef949a945d3d69e3dd0df`
- merge-base：`4a5665da5b2c6b83c4597844ea6e573746c821b1`
- 同步方式：`git merge upstream/main`
- upstream 最新 release tag：`v0.1.138`
- upstream/main 版本文件：`0.1.138`
- 本地最终 VERSION：`0.1.138`

## 上游更新摘要

- 吸收 `v0.1.138` 上游更新，共 37 个 upstream-only commit。
- 主要功能/修复：
  - OpenAI 图片 Responses 识别 `response.incomplete` 并记录软失败上游响应。
  - Vertex Anthropic 路径过滤 `anthropic-beta`，补充对应测试。
  - Claude Code 识别从 `cc_entrypoint=cli` 放宽为存在 `cc_entrypoint=`，兼容新的 IDE / SDK 入口。
  - OpenAI 调度增加 opt-in 的 `prefer_soonest_reset` / `scheduler_score_weights.reset`，默认关闭，不改变现有行为。
  - email bind 后缀白名单强制校验，补充 pending auth flow 测试。
  - 订阅支付加入 affiliate rebate，promo code 到期时间允许清空。
  - 用量统计展示缓存 Token 明细，前端自定义页面标题统一到 `resolveRouteDocumentTitle`。
  - CI/Actions 更新 pnpm action、Node 24、CLA workflow 相关配置。
- 高风险命中：
  - `.github/workflows/backend-ci.yml`
  - `backend/internal/handler/openai_gateway_handler.go`
  - `deploy/config.example.yaml`
  - `deploy/docker-compose.local.yml`
  - `backend/cmd/server/VERSION`

## 本地定制保护点

- 品牌与文案：
  - `frontend/src/router/title.ts` 的默认站点名仍为 `APIPool`。
  - `README.md` 仍保留 APIPool 定制说明和本地 iframe 充值方案说明。
- 部署/回滚/版本链路：
  - `.github/workflows/deploy.yml` 未被上游覆盖，仍保留 `pre-deploy-*.sql.gz`、`pre-deploy-biz-*.sql.gz`、`deploy-sub2api:rollback-latest` 和 `last-rollback-image.txt` 链路。
  - `deploy/rollback.sh`、`deploy/docker-compose.deploy.yml`、`deploy/version_resolver.sh` 仍保留 APIPool 生产约定。
  - `backend/cmd/server/VERSION` 已随 upstream/main 对齐到 `0.1.138`，与最新 tag `v0.1.138` 一致。
- OpenAI OAuth / Codex / 网关兼容：
  - APIPool 的 reqlog 捕获、ops 记录、Cyber policy 记录和自定义 OpenAI 网关处理仍存在。
  - 本地 Codex/OpenAI 兼容逻辑在吸收上游 `cc_entrypoint=` 放宽识别后仍保留。
  - `resolveOpenAIUpstreamEndpoint(c, account)` 已吸收上游 chat-only API-key upstream endpoint 记录修复。
- Kiro / OpenClaw：
  - Kiro OAuth handler、Kiro token/provider/service、前端 Kiro API、账号表单、OpenClaw 配置导入仍存在。
  - `frontend/src/utils/openclawConfig.ts` 和相关测试仍通过。
- 后台入口与默认配置：
  - `/purchase` 仍指向 APIPool 现有 iframe 购买入口。
  - `purchase_subscription_enabled` / `purchase_subscription_url` 仍贯通公开设置、路由和用户页。
  - 请求明细日志总开关仍在 `deploy/docker-compose.deploy.yml` 转发配置中。

## 冲突与取舍

- Git 冲突文件：
  - `deploy/docker-compose.local.yml`：接受上游 SELinux `:Z` bind mount 标记，同时保留 APIPool 本地 Redis `activedefrag` 参数和数组式 `command` 写法，避免 Compose 过早展开 Redis 密码变量。
  - `frontend/src/router/index.ts`：采用上游统一的 `resolveRouteDocumentTitle`，并传入 public/admin custom menu items；该 helper 已保留 APIPool 默认站点名和自定义页面标题逻辑。
- 无冲突但复核过的热点：
  - `backend/internal/handler/openai_gateway_handler.go`：确认 usage/cyber policy 记录改为 `resolveOpenAIUpstreamEndpoint`，不影响 APIPool reqlog 捕获。
  - `deploy/config.example.yaml`：新增 reset 权重和 `prefer_soonest_reset` 均默认关闭。
  - Kiro/OpenClaw、购买入口、部署 workflow、rollback 脚本和 APIPool 默认标题均已 grep 复核仍存在。

## 测试记录

- 通过：`go test -tags=unit ./...`
- 通过：`make test-unit`
- 初次失败后通过：`go test -tags=integration ./...`
  - 初次失败原因：testcontainers 未从 Docker context 读取 Colima socket，报 `Cannot connect to the Docker daemon`。
  - 设置 `DOCKER_HOST=unix:///Users/afreecoder/.colima/default/docker.sock` 后又遇到本地 testcontainers reaper 并发容器名冲突。
  - 最终通过命令：`DOCKER_HOST=unix:///Users/afreecoder/.colima/default/docker.sock TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock go test -p 1 -tags=integration ./...`。
- 通过：`DOCKER_HOST=unix:///Users/afreecoder/.colima/default/docker.sock TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock GOFLAGS=-p=1 make test-integration`
- 通过：`golangci-lint run ./...`，输出 `0 issues.`
- 通过：`pnpm --dir frontend run lint:check`
- 通过：`pnpm --dir frontend run typecheck`
- 通过：`pnpm --dir frontend run test:run`，122 个测试文件 / 744 个测试通过。
- 通过：`git tag --merged upstream/main --sort=-version:refname | head -1`，输出 `v0.1.138`。
- 通过：`cat backend/cmd/server/VERSION`，输出 `0.1.138`。
- 通过：`make build`，构建版本 `0.1.138`；仅有 Vite 既有 chunk/dynamic import 警告。
- 通过：`POSTGRES_PASSWORD=dummy docker compose -f deploy/docker-compose.deploy.yml config -q`
- 通过：`POSTGRES_PASSWORD=dummy docker compose -f deploy/docker-compose.local.yml config -q`
- 通过：`bash deploy/version_resolver.sh resolve .`，输出 `0.1.138`。
- 通过：`bash scripts/collect_upstream_sync_context.sh --no-fetch`
- 待部署后核对：GitHub Actions run、服务器 `backend/cmd/server/VERSION`、容器健康和 live version。

## 剩余风险与观察点

- 本次上游新增 `prefer_soonest_reset` 和 `scheduler_score_weights.reset`，默认关闭；如生产开启，需要额外观察 OpenAI 账号选择是否符合「优先用尽即将重置窗口」预期。
- 本地 integration 需要显式 Colima socket 和串行 `-p 1` 才稳定通过；这是本机测试环境约束，不是代码失败。
- 部署后仍需确认自动备份、回退镜像、服务器代码版本和 `sub2api` 健康状态。
- 快速回滚建议仍优先使用镜像回滚：`cd /opt/sub2api/deploy && ./rollback.sh image`。

## 代码评审处理

- Descartes 评审发现的 `scheduler_score_weights.reset` 校验/求和漏项已在 `34d918865437e6ef611df367cf9f1ab775e9ee8c` 修复，并补充 reset 单独启用与 reset 负数用例。
- Ampere 评审发现的流式 OpenAI 图片 `response.incomplete` 未触发 failover 已在 `34d918865437e6ef611df367cf9f1ab775e9ee8c` 修复，并补充未 flush 前 failover 回归测试。
- Descartes 评审发现的文档 HEAD 字段已改为“合入后评审基线 HEAD”，避免与后续修复提交混淆。

## 结论

- 建议保留当前合入结果。上游 `v0.1.138` 的功能和修复已吸收，APIPool 的部署、Kiro/OpenClaw、reqlog、购买入口、品牌默认值和版本链路均已复核，完整本地回归在上述环境参数下通过。
