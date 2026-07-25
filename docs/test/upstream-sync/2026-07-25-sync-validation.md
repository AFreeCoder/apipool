# 上游同步验证记录（2026-07-25）

## 基线

- 同步分支：`codex/upstream-sync-20260725`
- 合入前本地基线：`fda30d430d8ac4282691178ea6b7381769fbca91`
- 上游引用：`upstream/main`
- 上游 SHA：`2730c1c43b29be003925b033f3f9e645e726bb8c`
- merge-base：`63cef605940c9acf0ef6f1827065877f18952c5d`
- 同步方式：`git merge --no-ff upstream/main`
- 同步 merge commit：`1a0d54a59389003e59fe61f2e3a2f25cc3ab49f1`
- 上游独有提交：96 个
- 本地独有提交：243 个
- upstream 最新 tag：`v0.1.165`
- 最终 `backend/cmd/server/VERSION`：`0.1.165`
- 最终 Go 版本：`1.26.4`

## 上游更新摘要

- 分组与路由：新增 Composite 分组、显式模型路由、模型改写及 OpenAI、Anthropic、Gemini、Grok 多平台调度。
- OpenAI：新增 Live 创建与 sideband 通道、macOS attestation、OAuth 原生 HTTP/WSv2 透传及 Codex 输入兼容。
- 账号与配额：新增 Kiro 接线、Ollama Cloud 用量同步、Grok 兼容更新及相关后台能力。
- 注册与身份：增加邮箱点号/加号别名去重查询及表达式索引，降低重复注册和验证码滥用风险。
- 计费与日志：Usage Log 和批量图片增加 `session_id`，扩展请求类型、套餐与支付展示字段。
- 数据库：新增 187—190 迁移，覆盖会话字段、请求类型、Live 权限和邮箱别名索引。
- 前端：增加 Composite 分组与 Live capability 管理界面，并更新相应类型、API 与测试。

## 本地定制保护点

- 保留 APIPool 主 README、日文 README、本地品牌说明和生产访问地址，同时吸收 Composite/OpenAI Live 能力摘要。
- 保留 Kiro token provider、ReqLog 服务和 Composite resolver 的完整 Wire 接线。
- 网关中间件最终顺序为鉴权、分组校验、ReqLog、Composite 解析；既过滤未分组入口拒绝，又记录 Composite 解析阶段错误。
- 保留 `/messages/count_tokens` 的本地文本请求体上限，以及 OpenAI/Grok/Anthropic 的既有分流语义。
- OpenAI OAuth 继续使用本地更完整的输入规范化；OAuth 原生 WSv2 明确保留 namespace 与原生 input shape。
- `.github/workflows/deploy.yml`、`deploy/docker-compose.deploy.yml`、`deploy/docker-compose.biz.yml`、`deploy/rollback.sh` 与 `deploy/version_resolver.sh` 相对本地基线无差异。
- Go 版本继续保持项目 CI 要求的 `1.26.4`。

## 冲突与取舍

显式冲突文件共 7 个：

- `README.md`、`README_JA.md`：保留 APIPool 品牌和本地说明，补入本轮上游能力。
- `backend/cmd/server/wire_gen.go`：同时保留 Kiro、ReqLog 与 Composite resolver 生成接线。
- `backend/internal/server/http.go`：合并 ReqLog 生命周期与上游新增服务生命周期。
- `backend/internal/server/router.go`：补齐 Composite resolver 路由依赖。
- `backend/internal/server/routes/gateway.go`：保留本地鉴权、分组校验与 ReqLog，加入 Composite 解析和 Live 路由。
- `backend/internal/server/routes/gateway_test.go`：保留本地测试构造参数并吸收上游新增路由覆盖。

无冲突但经语义核验后调整：

- 移除上游较简化的重复 OpenAI passthrough input 规范化，保留本地会把文本内容规范为 `input_text` 的完整契约。
- 空白输入继续规范为 `[]`；OAuth 原生 WSv2 通过 `PreserveNativeInputShape` 保留客户端原始结构。
- 补齐五处构造函数参数，使 Kiro token provider 与 Composite resolver 同时存在。
- 补齐两个 `GroupsView` 测试的 `getLiveCapability` mock。

## 生成与静态核验

- `go generate ./ent`：通过；第二次执行无差异。
- Wire 重新生成：通过；第二次执行无差异。
- `git diff --check`：通过。
- 生产 workflow、主站/企业版 Compose、回滚和版本解析脚本相对本地基线无差异。
- `bash scripts/collect_upstream_sync_context.sh --no-fetch`：通过；上游提交数、tag 与版本文件一致。
- `bash deploy/version_resolver.sh resolve .`：输出 `0.1.165`。
- 主站与本地 Compose 均通过 `docker compose config -q`。

## 测试记录

- `make test-unit`：通过。
- `DOCKER_HOST=unix:///Users/afreecoder/.colima/default/docker.sock TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock make test-integration`：通过。
- `golangci-lint run ./...`：通过，`0 issues`。
- `pnpm install --frozen-lockfile`：通过。
- `pnpm run lint:check`：通过。
- `pnpm run typecheck`：通过。
- `pnpm run test:run`：通过，194 个测试文件、1392 个用例全绿。
- `pnpm run build`：通过。
- `make build`：通过，后端版本为 `0.1.165`。

## 失败—修复—复验记录

- 初次集成测试未识别 Colima，Testcontainers 报 `rootless Docker not found`；显式设置 Colima Docker socket 与容器内 socket 后，定向及完整 integration 全绿。该失败归因于环境发现，不是代码或迁移失败。
- 初次前端完整测试的 1392 个断言均通过，但两个旧测试文件缺少新增 Live capability mock，产生 10 个 unhandled rejection；补齐 mock 后 lint、typecheck、完整测试和 build 全绿。
- 接收评审阶段新增四组失败用例，分别稳定复现 Live 请求体日志遗漏、`allow_live` 缓存失效遗漏、迁移 190 无效索引不可恢复、Composite 早期错误不记录 ReqLog；最小修复后定向与完整门禁全绿。
- 完整 lint 暴露既有测试中 `t.Fatal` 后缺少显式 `return` 的静态空指针告警；补一行控制流后定向测试与 lint 全绿。

## 剩余风险与发布边界

- `docs/test/upstream-sync/issues.md` 中 Kiro 主动刷新、异步图片持久队列、鉴权快照完整失效和 Prompt Audit 隐私治理遗留仍然有效。
- 本轮只补齐 `allow_live` 的持久缓存失效；其他尚未被 outbox 完整覆盖的鉴权快照字段仍需独立 feature 处理。
- 生产部署前必须再次确认主站与企业版 `RUN_MODE=standard`，并验证双数据库新备份、回滚镜像和实际部署提交。

## 结论

同步、冲突解决、生成、前后端回归及双路评审修复均已完成，可以进入 `apipool-push-deploy` 发布门禁。
