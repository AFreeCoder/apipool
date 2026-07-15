# v0.1.156 上游同步双重代码评审报告

## 基本信息

- 日期：2026-07-15
- 评审基线：`579166b874c6672f5429c68cf217c8dab998669d`
- 同步评审终点：`ff1711f75521dac423088868497a75def4721422`
- 评审范围：`git diff 579166b8..ff1711f7`
- 评审方式：一个 subagent 使用 `requesting-code-review`，另一个 subagent 使用 gstack code review；两者均只读、相互独立
- 接受方式：主 agent 使用 `receiving-code-review` 逐条复现、核对基线归因并补充 RED 测试

## 结论

评审前结论为阻断发布。接受评审后，5 项本次发布范围内的有效问题已修复并完成完整回归，当前可以进入推送部署阶段。Kiro 后台主动刷新候选缺口经基线对比确认不是本次同步引入，已单独记录到 `issues.md`，不作为本次发布回归阻断。

## 发现与处置

| 发现 | 评审来源 | 核验结论 | 处置 |
| --- | --- | --- | --- |
| Agent Identity 使用默认空过期时间时导入失败 | requesting | 有效，本次新增路径缺陷 | 单独处理 Agent Identity 到期语义；默认允许不过期，显式账号到期仍生效 |
| OAuth 账号升级为 Agent Identity 后残留 OAuth 凭证并可能被后台刷新器置错 | requesting | 有效，本次新增路径缺陷 | 更新时替换 OAuth 凭证；OpenAI 刷新器增加 Agent Identity 防御门禁 |
| 并行工具参数交错会在 Anthropic block stop 后继续发送 delta | requesting、gstack | 有效，本次新增协议错误 | 缓存各工具参数分片，在 finish 或流结束时按 index 串行输出完整 block，并保留原始分片粒度 |
| 企业版 Caddy 段仍强制 immutable，导致新增自检脚本失败 | requesting | 有效，合并遗漏 | 移除企业域名的重复 immutable 规则，由后端统一决定缓存策略 |
| Anthropic `output_config.effort` 转换后未进入用量元数据 | gstack | 有效，本次新增观测错误 | 从最终发送给 Chat Completions 上游的请求提取实际 effort，覆盖默认 `medium` 与 `max→xhigh` |
| Kiro 未进入后台主动刷新候选 | gstack | 有效，但基线已存在，不是本次回归 | 本次不扩大同步修复范围，转入 `issues.md` |

## RED 到 GREEN 证据

修复前新增的定向测试分别稳定失败：

- Agent Identity 默认导入、显式到期及 OAuth→Agent Identity 更新：失败
- Agent Identity 刷新门禁：失败
- 两个并行工具参数跨 chunk 交错：失败，复现已关闭 block 继续收到 delta
- 默认 effort 与 `max` effort 的结果元数据：失败，结果为 `nil`
- `sh deploy/test-caddyfile-cache.sh`：退出码 1

修复后上述测试全部通过，Caddy 自检退出码为 0。

## 完整回归

- `GOTOOLCHAIN=go1.26.4 go test -tags=unit ./...`：通过
- `GOTOOLCHAIN=go1.26.4 go test -tags=integration ./...`：通过
- `GOTOOLCHAIN=go1.26.4 golangci-lint run ./...`：通过，`0 issues`
- `pnpm lint:check`：通过
- `pnpm typecheck`：通过
- `pnpm test:run`：通过，160 个测试文件、1114 个测试
- `GOTOOLCHAIN=go1.26.4 make build`：通过
- `sh deploy/test-caddyfile-cache.sh`：通过
- `git diff --check`：通过

前端构建仍有既有的动态/静态混合导入、chunk 大小与 Browserslist 数据提示，不影响构建退出码，且不属于本次修复引入。

## 评审边界

- 两个评审 subagent 均未修改、暂存、提交或推送文件，也未执行 GitHub/Greptile 外部写入。
- 本轮没有 Ent schema 或 migration 变更。
- 用户原有未跟踪目录 `.agents/` 未被读取、修改或纳入提交。
