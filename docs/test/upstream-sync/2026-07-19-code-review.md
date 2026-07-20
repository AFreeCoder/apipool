# v0.1.161 上游同步双重代码评审报告

## 基本信息

- 日期：2026-07-19
- 评审基线：`67399dddec717c448ac1488d0e4b5c31b5d3d932`
- 同步评审终点：`1aef5053be3a0f66716f659951516ead13eec989`
- 评审范围：`git diff 67399dddec717c448ac1488d0e4b5c31b5d3d932..1aef5053be3a0f66716f659951516ead13eec989`
- 评审方式：一个 subagent 使用 `requesting-code-review`，另一个 subagent 使用 gstack code review；两者只读、相互独立
- 接受方式：主 agent 使用 `receiving-code-review` 逐条核对调用链、同步前基线、生产代理边界和回归测试

## 结论

评审输入版本存在两个发布阻塞回归：安全客户端 IP 会直接信任可伪造的原始转发头，固定高风险路由会被默认关闭的 `step_up_enabled` 开关整体绕过。两项均已先补 RED 测试再修复，定向回归已转绿。gstack 提出的鉴权缓存跨实例失效覆盖不足和 Prompt Audit 明文保留策略缺口属于有效 P2，但在当前单实例部署、Prompt Audit 保持关闭的前提下不阻塞本次同步，已转入 `issues.md`。

## 发现与接收结论

| 发现 | 评审来源 | 核验结论 | 处置 |
| --- | --- | --- | --- |
| 兼容迁移会把旧数据库中的 `false` 自动改为 `true`，随后安全路径直接读取 `CF-Connecting-IP`、`X-Real-IP`、`X-Forwarded-For` 或自定义头；生产 Compose 默认把应用端口发布到 `0.0.0.0` | requesting | 有效，Critical；可伪造 IP 会进入 API Key ACL、会话绑定和安全审计，公网直连还会绕过 Caddy | 保留已存 `false`；开启时仅使用 Gin `server.trusted_proxies`，关闭时只使用 TCP 直连对端；原始头仅保留非安全元数据用途；生产端口固定绑定 `127.0.0.1` 并在部署前后断言 |
| `step_up_enabled` 缺失、读取失败或默认关闭时，账号/代理导出、备份创建/下载及存储配置等既有固定高风险路由全部放行 | requesting | 有效，Critical；同步前这些路由始终门控 | 固定路由中间件改回无条件 fail-closed；开关只控制新增的管理员创建/提权附加门控 |
| 鉴权缓存 outbox 触发器未覆盖配额、余额、并发、组费率、模型路由和媒体权限等快照字段，服务层失效失败又被忽略 | gstack | 有效，P2；当前单实例仍有进程内失效，跨实例/Redis 故障窗口需独立设计 | 不扩大本次同步修复范围，转入 `issues.md` |
| Prompt Audit 把完整提示词明文写入 PostgreSQL，未发现自动 TTL、字段加密或保留策略 | gstack | 有效，P2；功能默认关闭时不影响当前生产 | 转入 `issues.md`；完成隐私与保留策略前，部署门禁要求生产保持关闭 |

## RED 到 GREEN 证据

修复前新增或收紧的测试稳定失败：

- 原始 `X-Real-IP` 和自定义 CDN 头会覆盖安全客户端 IP。
- 开关关闭且 Gin 已配置可信代理时，安全路径仍会读取代理转发值，而不是直连对端。
- 旧数据库中的 `false` 会在首次迁移时被写成 `true`。
- 固定高风险路由中间件在 `step_up_enabled=false` 时放行 admin API key。

修复后以下定向验证全部通过：

- `go test -tags=unit ./internal/pkg/ip`
- `go test -tags=unit ./internal/service -run '^TestSettingService_LoadForwardedClientIPSettings'`
- `go test -tags=unit ./internal/server/middleware -run 'Test(APIKeyAuthIPRestriction|SessionBindingContext|SecurityClientIP|RequestSessionBinding|ProtectedRouteStepUp|EnforceStepUp)'`
- `pnpm exec vitest run src/views/admin/__tests__/SettingsView.spec.ts`：28 项通过

## 对同步验证基线的调整

已冻结的 `2026-07-19-sync-validation.md` 记录了评审前的上游兼容语义。接收评审后，以本文为后续基线：

- 不接受“兼容开关开启时原始转发头进入安全路径”的上游行为。
- 不接受旧 `false` 自动迁移为 `true`。
- 不接受固定高风险路由受默认关闭开关控制。

## 完整回归

- `GOTOOLCHAIN=go1.26.4 go test -tags=unit ./...`：通过。
- `GOTOOLCHAIN=go1.26.4 go test -tags=integration ./...`：通过。
- `GOTOOLCHAIN=go1.26.4 golangci-lint run ./...`：通过，`0 issues`。
- `pnpm lint:check`：通过。
- `pnpm typecheck`：通过。
- `pnpm test:run`：181 个测试文件、1260 项测试全部通过。
- `GOTOOLCHAIN=go1.26.4 make build`：通过，后端版本 `0.1.161` 与前端生产构建成功。
- `bash deploy/tests/apple-container-test.sh`：通过，覆盖创建、启动、停止、重建与持久卷清理。
- 主站和企业版生产 Compose 均通过 `docker compose config --quiet`。
- 主站生产 Compose 渲染结果的 `host_ip` 为 `127.0.0.1`，部署工作流同时校验渲染结果与运行时端口绑定。
- `bash deploy/tests/install-github-token-test.sh`：本机 Bash 3.2 按预期明确跳过；Linux Bash 4+ CI 保留完整检查。
- `git diff --check`：通过。
- `bash deploy/version_resolver.sh resolve .`、最新上游 tag 和 `backend/cmd/server/VERSION` 均为 `0.1.161`。

## 评审边界

- 两个评审 subagent 均未修改、暂存、提交或推送文件，也未执行外部评审写入。
- 本轮接收修复没有修改 Ent schema 或新增数据库 migration。
- 生产备份、回滚镜像、配置门禁和真实请求冒烟由后续 push-deploy 阶段单独留证，不回填本文。
