# 上游同步评审记录（2026-04-08）

## 基线

- 当前分支：`main`
- 合入前基线：`8f141560e7b1d432a6e75f293ff9238a55ed9ab2`
- 合入前 SHA：`8f141560e7b1d432a6e75f293ff9238a55ed9ab2`
- 上游引用：`06e2756ee4322d5ea08f2d258e078b902a6cc127`
- 上游 SHA：`06e2756ee4322d5ea08f2d258e078b902a6cc127`
- merge-base：`339d906e547e34fd7536586275205cb355817729`
- 同步方式：`git merge 06e2756ee4322d5ea08f2d258e078b902a6cc127`
- 当前运行时版本：`0.1.108`
- upstream/main 版本文件：`0.1.109`
- upstream 最新 release tag：`v0.1.109`

## 上游更新摘要

- 上游引入了 OpenAI / OAuth 链路的一组修复：API Key 账号不再被错误做 Codex 模型归一化、passthrough 在 429/529 时支持 failover、非流式路径会把 SSE 异常返回转成 JSON，并在终态 `output` 为空时从 delta 事件补回响应内容。
- 后台设置新增了 Beta Policy 的模型白名单、fallback action / fallback error message 字段，并同步补齐 DTO、后端校验与前端表单。
- 版本链路同步到 `v0.1.109`，`backend/cmd/server/VERSION` 随 upstream 一起升级到 `0.1.109`；由于 upstream tag 与 VERSION 一致，本轮没有额外拆分 `chore(version)` 提交。
- 命中的高风险模块：
  - `backend/internal/service/openai_codex_transform.go`
  - `backend/internal/service/openai_gateway_service.go`
  - `backend/internal/service/openai_oauth_passthrough_test.go`
  - `backend/internal/service/openai_ws_forwarder.go`
  - `backend/internal/service/setting_service.go`
  - `frontend/src/i18n/locales/en.ts`
  - `frontend/src/i18n/locales/zh.ts`
  - `frontend/src/views/admin/SettingsView.vue`
- 本轮额外吸收但不能覆盖本地逻辑的实现：
  - 保留 APIPool 对 OAuth/Codex 输入归一化与 top-level transcript item 兼容，同时吸收 upstream 对 API Key 账号模型名保留策略
  - 保留 APIPool 设置页默认值、品牌文案和支付集成文档入口，同时吸收 upstream 的 Beta Policy 新配置能力

## 本地定制保护点

- 品牌/APIPool 文案与入口：
  - 保留 `APIPool` 站点默认名、多语言欢迎文案，以及设置页支付集成文档链接 `AFreeCoder/apipool`
- 部署、回滚、版本链路：
  - 保留本地 deploy/rollback 体系不变；本轮仅吸收 `.github/audit-exceptions.yml` 与版本号同步
  - 最终版本链路对齐为：upstream 最新 release tag `v0.1.109` / upstream VERSION `0.1.109` / 本地最终 VERSION `0.1.109`
- OpenAI OAuth / Codex 兼容逻辑：
  - 保留 OAuth passthrough 输入归一化、reasoning/top-level transcript item 兼容与 Codex 输入修正
  - 吸收 upstream 对 API Key 账号模型名保留、429/529 passthrough failover、SSE->JSON 兼容和 output 补全
- 管理后台行为与默认配置：
  - 保留 APIPool 的 `site_name: APIPool`、设置页品牌入口与现有默认值
  - 吸收 Beta Policy 模型白名单/fallback 配置，并补齐前后端类型与保存逻辑

## 冲突与取舍

- 显式 Git 冲突文件：
  - `backend/internal/repository/account_repo.go`
  - `backend/internal/repository/account_repo_integration_test.go`
  - `backend/internal/service/openai_oauth_passthrough_test.go`
- 无冲突但做了语义审查的文件：
  - `backend/internal/service/openai_codex_transform.go`
  - `backend/internal/service/openai_gateway_service.go`
  - `backend/internal/service/openai_gateway_service_test.go`
  - `backend/internal/service/openai_ws_forwarder.go`
  - `backend/internal/service/setting_service.go`
  - `backend/internal/service/settings_view.go`
  - `frontend/src/api/admin/settings.ts`
  - `frontend/src/i18n/locales/en.ts`
  - `frontend/src/i18n/locales/zh.ts`
  - `frontend/src/views/admin/SettingsView.vue`
- 冲突处理取舍：
  - `account_repo.go` 选择保留 APIPool 现有“active 仅返回运行时可调度账号”的更强语义：同时排除 `schedulable=false`、rate limited、overload、temp unschedulable，并继续保留 `inactive` -> `disabled` 别名
  - `account_repo_integration_test.go` 保留本地更强覆盖用例，而不是退回为仅校验 rate-limited 的窄测试
  - `openai_oauth_passthrough_test.go` 同时保留本地 OAuth passthrough 输入归一化测试，并补入 upstream 的 429/529 failover 测试
- 最终保留的 APIPool 行为：
  - 品牌与入口仍是 APIPool
  - 设置页默认站点名和支付集成文档入口仍指向本仓库
  - OpenAI OAuth / Codex 兼容逻辑未回退
- 本轮没有额外的版本对齐提交；`VERSION` 直接随 upstream merge 一起更新到 `0.1.109`

## 测试记录

- [x] `cd backend && go test ./...`
- [x] `cd backend && make test-unit`
- [ ] `cd backend && make test-integration`
  - 未通过，阻塞原因是当前机器 Docker daemon 不可连接，`testcontainers` 在 `internal/middleware/rate_limiter_integration_test.go` 和 `internal/server/routes/auth_rate_limit_integration_test.go` 启动 Redis 容器时 panic；不是业务断言失败
- [x] `cd backend && golangci-lint run ./...`
- [x] `pnpm --dir frontend run lint:check`
- [x] `pnpm --dir frontend run typecheck`
- [x] `pnpm --dir frontend run test:run`
- [x] `git tag --merged upstream/main --sort=-version:refname | head -1`
  - 输出：`v0.1.109`
- [x] `cat backend/cmd/server/VERSION`
  - 输出：`0.1.109`
- [x] `make build`
  - 后端二进制与前端构建均成功；Vite 仅给出 chunk size warning，无构建失败
- [ ] `docker compose -f deploy/docker-compose.deploy.yml config -q`
  - 直接执行时因当前 shell 未注入 `POSTGRES_PASSWORD` 等必填环境变量失败
- [x] `docker compose --env-file deploy/.env.example -f deploy/docker-compose.deploy.yml config -q`
- [x] `docker compose --env-file deploy/.env.example -f deploy/docker-compose.local.yml config -q`
- [x] `bash deploy/version_resolver.sh resolve .`
  - 输出：`0.1.109`
- [ ] 部署后版本输出 / 页面版本人工核对：
  - 本轮未部署，未做线上容器/页面版本核对
- [x] 其他专项验证：
  - `git diff --check`
  - 冲突文件与高风险自动合并文件的语义复查

## 剩余风险与观察点

- 当前最大剩余风险不是代码回归，而是 Docker 不可用导致本机无法完成 integration 环境验证；后续在有 Docker daemon 的环境中应补跑 `make test-integration`
- 本轮未部署，因此还缺少 2 个生产侧确认：
  - 部署后 `docker exec sub2api /app/sub2api --version` 或等价方式确认运行中二进制版本为 `0.1.109`
  - 页面左上角/公开设置返回的运行时版本确认展示为 `0.1.109`
- 观察点：
  - OpenAI passthrough 的 429/529 failover 是否在真实上游容量波动下符合预期
  - Beta Policy 模型白名单在设置页保存、读取和实际 Anthropic `/v1/messages` 转发时是否一致

## 结论

- [x] 建议保留当前 merge 结果
- [x] 已提交 merge commit：`1e09509ad2633b42f76d6e6b180bfd2bbc714475`（`merge: sync upstream/main to v0.1.109`）
- [x] 版本结论：
  - upstream 最新 release tag：`v0.1.109`
  - upstream/main `backend/cmd/server/VERSION`：`0.1.109`
  - 本地最终 `backend/cmd/server/VERSION`：`0.1.109`
  - 版本对齐未拆分独立提交，原因是 upstream tag 与 VERSION 已一致
