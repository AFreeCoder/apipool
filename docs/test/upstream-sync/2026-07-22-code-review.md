# v0.1.163 上游同步双重代码评审报告

## 基本信息

- 日期：2026-07-22
- 评审基线：`bd8371ae82c61ba0c5f759f13b21556b66d782fa`
- 同步评审终点：`1d1478a22a6218d19b5515184010cfd5c583bafc`
- 评审范围：`git diff bd8371ae82c61ba0c5f759f13b21556b66d782fa..1d1478a22a6218d19b5515184010cfd5c583bafc`
- 评审方式：一个 subagent 使用 `requesting-code-review`，另一个 subagent 使用 gstack code review；两者只读、相互独立
- 接受方式：主 agent 使用 `receiving-code-review` 逐条核验调用链、生产 Compose 语义和测试证据，再决定接受、调整或拒绝

## 结论

评审输入版本存在一个发布阻塞资源保护问题：Grok 本地 `count_tokens` 使用全局 256MB 请求体上限，且没有进入现有 RPM 和用户并发控制。另有请求明细遗漏、原始错误体日志泄露和推理强度上限 fail-open。上述有效问题已修复并完成完整后端回归。

两路评审都提到生产 Compose 未传递 `REDIS_USERNAME`。核验后不直接接受该改动：当前生产 Compose 固定连接同一 Compose 内的 Redis，Redis 服务只配置 `default` 用户和可选密码，并没有创建命名 ACL 用户。仅向应用传入用户名会造成认证失败，而不是补齐支持。示例环境文件已明确命名 ACL 用户只适用于已完成 ACL 配置的外部 Redis；生产固定内置 Redis 继续使用 `default` 用户。

当前没有未闭环的本次同步发布阻塞项，可以进入 `apipool-push-deploy` 阶段。
生产部署门禁必须确认实际 `RUN_MODE=standard`；`simple` 模式按其既有语义跳过计费、RPM 和用户并发检查，只保留文本请求体上限，不适合作为当前公网生产配置。

## 发现与接收结论

| 发现 | 评审来源 | 核验结论 | 处置 |
| --- | --- | --- | --- |
| Grok 本地 `count_tokens` 可读取 256MB 请求体，且绕过 RPM 与用户并发槽位 | gstack | 有效，P1，发布阻塞 | 两条兼容路径统一改用 `text_max_body_size`（默认 32MB）；standard 模式先抢占用户并发槽位并登记 API Key 活跃槽位，再执行既有计费/API Key/RPM 资格检查；缺少依赖时关闭失败 |
| `/messages/count_tokens` 未进入 ReqLog，中转后的 OpenAI/Grok handler 也未保存请求体快照 | requesting、gstack | 有效，Important/P2 | 别名路由补 `ReqLogCaptureMiddleware`；Grok 和 OpenAI 两个 handler 在读体后调用 `MaybeCaptureRequestBody`；两条路径均增加端到端捕获测试 |
| Grok encrypted reasoning 重试日志记录原始上游错误体预览；畸形 JSON 解析日志记录请求体头尾片段 | gstack | 有效，P2，可能泄露提示词、凭据或个人信息 | 移除原始片段，只记录请求体/响应体长度和截断 SHA-256；增加包含敏感标记的结构化日志回归测试 |
| reasoning effort 出现未知显式值时绕过分组上限 | gstack | 有效，P2，成本策略 fail-open | 配置上限存在时，未知字符串值保守钳制到上限；未配置上限时保持兼容，不修改未知值 |
| 生产 Compose 未传递 `REDIS_USERNAME` | requesting、gstack | 发现属实，但建议修复不适用于当前生产拓扑 | 不向固定内置 Redis 传入未创建的 ACL 用户；在 `.env.example` 明确只有外部 Redis 已创建 ACL 用户时才配置，当前生产使用 `default` |

## 定向验证

- Grok `count_tokens`：standard 模式缺少资格/并发依赖返回 503；RPM 超限返回 429 和 `Retry-After`；用户并发已满返回 429 且不会消耗 RPM；成功请求获取并释放用户槽位。
- 两条路由 `/v1/messages/count_tokens`、`/messages/count_tokens`：超过文本体积限制均返回 413；成功请求均生成一条包含模型和请求体的 ReqLog。
- 解析失败日志：只包含 `body_len`、`body_sha256` 和错误，不再出现 `body_head`、`body_tail` 或原始敏感片段。
- Grok encrypted reasoning 重试：第一次上游错误体含敏感标记，日志只出现长度与哈希；第二次重试成功。
- reasoning effort：未知显式值在配置上限时被钳制；没有上限时保持原值。

## 完整回归

- `GOTOOLCHAIN=go1.26.4 go test -count=1 -tags=unit ./...`：通过。
- `DOCKER_HOST=unix:///Users/afreecoder/.docker/run/docker.sock GOTOOLCHAIN=go1.26.4 go test -count=1 -tags=integration ./...`：通过。
- `GOTOOLCHAIN=go1.26.4 golangci-lint run ./...`：通过，`0 issues`。
- `GOTOOLCHAIN=go1.26.4 make build`：通过，后端版本 `0.1.163` 与前端 Vite 生产构建成功。
- 前端源码未在接收评审阶段修改；同步评审终点已经通过 lint、typecheck 和 188 个测试文件、1347 个用例的完整测试，接收修复后的 `make build` 再次完成类型检查和生产构建。
- 主站、企业版生产 Compose 均通过 `docker compose config -q`。
- `bash deploy/version_resolver.sh resolve .`：输出 `0.1.163`。
- `git diff --check`：通过。

前端构建仍有既有 Browserslist 数据陈旧、动静态混合导入和大 chunk 提示，不影响构建退出码，也不是本次接收修复引入。

## 评审边界

- 两个评审 subagent 均未修改、暂存、提交或推送文件，也未执行外部评审写入。
- 本轮接收修复没有修改 Ent schema 或新增数据库 migration。
- `docs/test/upstream-sync/issues.md` 中既有遗留继续有效，本次没有扩大其生产启用范围。
- 生产备份、回滚镜像、GitHub Actions 与真实域名健康检查由后续 `apipool-push-deploy` 阶段单独留证。
