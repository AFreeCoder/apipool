# 上游 v0.1.161 同步验证记录（2026-07-19）

## 验证范围

- 合入前本地基线：`67399dddec717c448ac1488d0e4b5c31b5d3d932`
- 上游引用：`upstream/main`
- 上游 SHA：`b8e844f4ee130ac069a7c5713c2413233186b83f`
- 上游 release tag：`v0.1.161`
- 上游合并提交：`57d979ed9762ebefbedb4a8e70ad375c2bc00db1`
- 合并适配提交：`bff6970e4`
- 本地最终版本：`0.1.161`
- 增量：173 个上游提交，主要覆盖提示词审计、客户端 IP 安全模型、Grok 媒体代理、OpenAI 流错误分类、在线更新超时和部署配置修复。

## 冲突处理结论

- 保留 APIPool 的 Kiro/OpenClaw、请求明细日志、调度与计费、购买页 iframe、品牌默认值，以及主站和企业版双数据库备份/回滚链路。
- 管理台数据库恢复入口继续保持禁用；在线二进制版本回退接口仍可用。
- OpenAI 网关合并上游显式生图意图、流读取 SLA 分类与安全审计，同时保留 APIPool 稳定错误码和 OAuth input 归一化。
- API Key ACL 采用上游统一安全模型：兼容开关开启时使用原始转发头，关闭时严格使用 Gin `trusted_proxies` 链；示例配置保持关闭。
- step-up 在缺少 session ID 时继续 fail closed，不退化为用户级授权键。
- 网关路由继续启用请求明细日志，并吸收 `/embeddings`、`/alpha/search` 文本体积限制和 Grok 视频内容代理路由。
- 备份页吸收异步图片存储配置，但不恢复数据库导入/恢复操作。

## 合并适配修复

- 将非事务迁移辅助函数统一到 `migrationConnection`，恢复迁移测试桩兼容。
- 补齐审计、运维 handler 与 Wire 构造函数的新接口参数。
- 对齐 OpenAI 流错误 helper 的 `countTowardsSLA` 参数，并保留 APIPool 稳定错误响应格式。
- 更新 APIPool updater User-Agent、在线回退 15 分钟超时和 OAuth JSON mode content 数组的测试断言。
- 安装器 GitHub token 安全测试改用 BSD/GNU 均支持的 `sed`；本机 Bash 3.2 时明确跳过，Linux Bash 4+ CI 仍执行完整断言。

## 完整回归

- `GOTOOLCHAIN=go1.26.4 go test -tags=unit ./...`：通过。
- `GOTOOLCHAIN=go1.26.4 go test -tags=integration ./...`：通过。
- `golangci-lint run ./...`：通过，`0 issues`。
- `pnpm install --frozen-lockfile`：通过。
- `pnpm lint:check`：通过。
- `pnpm typecheck`：通过。
- `pnpm test:run`：181 个测试文件、1260 个测试全部通过。
- `make build`：后端 `Version=0.1.161` 与前端 Vite 生产构建通过；仅有既有动态/静态导入和大 chunk 警告。
- `bash deploy/tests/apple-container-test.sh`：通过，覆盖创建、启动、停止、重建和清理。
- `docker compose -f deploy/docker-compose.local.yml config --quiet`：通过（使用非敏感占位密码）。
- `docker compose -f deploy/docker-compose.yml config --quiet`：通过（使用非敏感占位密码）。
- `git diff --check`：业务文件通过；上游 source-freeze patch 自带的空白告警不改写。
- `bash deploy/version_resolver.sh resolve .`：`0.1.161`。

## 当前结论

- 同步分支已达到可评审状态，尚未推送或部署。
- 下一道门禁是两路独立代码评审及 receiving review；评审发现未闭环前不得进入生产发布。
