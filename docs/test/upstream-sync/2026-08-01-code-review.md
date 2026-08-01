# v0.1.169 上游同步双重代码评审报告

## 基本信息

- 日期：2026-08-01
- 本地基线：`7a103700255b28e1e1de9025568cb38caca3bbb7`
- 上游同步提交：`796c8ffcb559fb49d0d7ca13060d744033bbb56b`
- 评审修复提交：`7fb4f44b8cec7fad31d5846a49fadc182f7ad224`
- 第 1 轮范围：`7a103700255b28e1e1de9025568cb38caca3bbb7..796c8ffcb559fb49d0d7ca13060d744033bbb56b`
- 第 2 轮范围：`796c8ffcb559fb49d0d7ca13060d744033bbb56b..7fb4f44b8cec7fad31d5846a49fadc182f7ad224`
- 评审方式：两个只读 subagent 相互独立评审；一个侧重行为、品牌和集成语义，另一个侧重安全、推理元数据和发布契约
- 轮次：2 轮，未达到 5 轮上限；第 2 轮双方均明确“通过，可发布”后提前结束

## 结论

第 1 轮确认四项真实缺陷：Caddy 双站点压缩契约与企业站 SSE 风险、GPT-5.6 `max` 推理强度的请求与用量元数据不一致、生产 Compose 未吸收上游 `no-new-privileges` 加固，以及新 Passkey/模型广场泄漏上游品牌。

四项均由主 agent 独立复现并实施最小修复。第 2 轮两位评审者复核修复提交后均未发现新的真实、可复现缺陷；无缓存 Go unit/integration、前端完整测试、静态检查、生产构建和部署契约全部通过，可以进入 `apipool-push-deploy` 阶段。

## 发现与接收结论

| 发现 | 严重度 | 核验结论 | 处置 |
| --- | --- | --- | --- |
| Caddy 测试硬编码全文件只有一个 `encode`，但本地有主站和企业站两个；企业站仍用 `text/*` | P1 | 有效；CI 必然失败，且 `text/event-stream` 可能被压缩和缓冲 | 两个站点统一使用显式非 SSE MIME 白名单；测试改为逐块验证两个 canonical `encode` 块 |
| Anthropic → Chat Completions fallback 将 GPT-5.6 原生 `max` 记录为 `xhigh` | P1 | 有效；实际转发请求与 `OpenAIForwardResult.ReasoningEffort`/usage 元数据不一致，专项单测稳定失败 | 改用已有 model-aware effort extractor；验证 GPT-5.6 `max`、旧模型 `max → xhigh`、其他强度和默认值 |
| 自定义生产 Compose 未继承上游 `no-new-privileges` | P2 | 有效；真实 `docker-compose.deploy.yml` 没有获得上游容器权限加固，原测试也未覆盖 | deploy、biz、test 三份本地 Compose 补齐该项，并把所有本地 Compose 纳入安全契约测试 |
| WebAuthn 默认/示例与模型广场 fallback 显示 `Sub2API` | P2 | 有效；启用 Passkey 或公共设置站点名为空时会显示错误品牌 | 默认、示例域名和前端 fallback 改为 `APIPool`，新增后端默认值与前端回退测试 |

## 第 2 轮复核

- GPT-5.6 `max`、旧模型归一化、其他 effort 与省略值专项测试通过。
- model-aware 候选模型推导测试通过。
- 生产、企业版和测试 Compose 均可解析，渲染配置包含 `no-new-privileges:true`。
- Caddy 两个站点的压缩块均为相同的非 SSE 策略，契约测试通过。
- WebAuthn 品牌默认值与模型广场 fallback 测试通过。
- 两位评审者均未发现新的真实、可复现缺陷。

## 完整回归

- `go test -count=1 -tags=unit ./...`：通过。
- 显式 Colima socket 的 `go test -count=1 -tags=integration ./...`：通过。
- `golangci-lint run ./...`：通过，`0 issues`。
- `pnpm run lint:check`、`pnpm run typecheck`：通过。
- `pnpm run test:run`：通过，206 个测试文件、1467 个用例全绿。
- `make build`：通过，版本 `0.1.169`。
- 目标部署、Compose 安全与资源、Caddy 压缩/SSE 契约及 Compose `config -q`：全部通过。
- `git diff --check`：通过。

## 评审边界

- 两个 subagent 均未修改、暂存、提交或推送文件，也未执行外部写入。
- 生产固定工具链、GitHub Actions、容器、备份、回滚镜像和真实入口由后续 `apipool-push-deploy` 阶段单独留证。
- `docs/test/upstream-sync/issues.md` 中既有遗留仍有效，本轮没有新增未闭环问题。
