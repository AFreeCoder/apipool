# 上游同步评审记录（2026-03-20）

## 基线

- 当前分支：`main`
- 当前 merge commit：`60e2c6fc`
- 合入前本地基线：`e17a36d5`
- 上游引用：`upstream/main`
- 上游 SHA：`94bba415`
- merge-base：`9f6ab6b8`
- 同步方式：`git merge upstream/main`
- upstream 最新 release tag：`v0.1.104`
- upstream/main 版本文件：`0.1.104`
- 本地最终 VERSION：`0.1.104`

## 上游更新摘要

- 本轮主要吸收了 19 个 upstream 提交，集中在 6 类：
- Claude Code 版本控制：新增 `max_claude_code_version` 设置项，并补了 API contract 预期。
- Anthropic / Antigravity：新增被动用量采样展示、代理不可用 fast-fail、credits exhausted 429 修复。
- OpenAI 兼容：chat completions -> responses 路径补了稳定 compat prompt cache key。
- 管理后台：用户列表新增分组列、分组筛选和专属分组一键替换；账号列表支持 ungrouped filter。
- 前端体验：DataTable 虚拟滚动、分页 pageSize 持久化、批量编辑允许清空模型限制。
- 文档/发布：README / deploy 文档切到 `docker compose` 语法，release / VERSION 同步前移到 `v0.1.104`。

## 命中的高风险模块

- `README.md`
- `deploy/README.md`
- `backend/cmd/server/VERSION`
- `backend/internal/service/ratelimit_service.go`
- `backend/internal/service/account_usage_service.go`
- `backend/internal/service/openai_gateway_chat_completions.go`
- `backend/internal/service/openai_gateway_service.go`
- `backend/internal/service/admin_service.go`
- `frontend/src/views/admin/SettingsView.vue`
- `frontend/src/components/account/BulkEditAccountModal.vue`
- `frontend/src/views/admin/UsersView.vue`

## 本地定制保护点

- 品牌与文案：
- 保留 `APIPool` 默认站点名，没有把 `SettingsView` 和 README 拉回 `Sub2API`
- 保留后台 GitHub 链接到 `AFreeCoder/apipool`
- 保留支付集成文档下载链接到本仓库 `docs/ADMIN_PAYMENT_INTEGRATION_API.md`

- 部署、回滚与版本链路：
- 保留 APIPool 现有 DigitalOcean + Docker Compose 部署说明
- 保留回滚手册和运行时版本链路
- `backend/cmd/server/VERSION` 已跟上 upstream `0.1.104`

- OpenAI OAuth / Codex 兼容：
- 保留 passthrough input 归一化
- 保留本地 OpenAI OAuth 401/403 / Cloudflare challenge 处理语义
- 吸收 upstream compat prompt cache key 注入

- 管理后台与默认配置：
- 吸收 upstream 的 `max_claude_code_version`
- 吸收用户分组替换和账号 ungrouped filter
- 保留 APIPool 的后台默认值和品牌相关入口

## 冲突与取舍

- 显式 Git 冲突文件共 4 个：
- `README.md`
- `backend/cmd/server/VERSION`
- `backend/internal/handler/admin/admin_service_stub_test.go`
- `frontend/src/components/account/__tests__/BulkEditAccountModal.spec.ts`

- 冲突处理结果：
- `README.md` 保留 APIPool 定制版，不回退到 upstream 通用安装文档
- `VERSION` 对齐到 upstream `0.1.104`
- `admin_service_stub_test.go` 同时保留 `SyncOpenAIPlanType` 并补上 upstream 新增的 `ReplaceUserGroup`
- `BulkEditAccountModal.spec.ts` 合并为“本地 OpenAI/Antigravity 断言 + upstream 新增空 model_mapping 用例”的并集

- 无冲突但做了语义复核的文件：
- `backend/internal/service/ratelimit_service.go`
- `backend/internal/service/account_usage_service.go`
- `backend/internal/service/openai_gateway_chat_completions.go`
- `frontend/src/views/admin/SettingsView.vue`
- `frontend/src/components/layout/AppHeader.vue`

## 测试记录

- 后端：
- `cd backend && go test -tags=unit ./...` 通过
- `cd backend && go test -tags=integration ./...` 通过
- `cd backend && golangci-lint run ./...` 通过

- 前端：
- `cd frontend && pnpm install --frozen-lockfile` 通过
- `cd frontend && pnpm lint:check` 通过
- `cd frontend && pnpm typecheck` 通过
- `cd frontend && pnpm test:run` 通过

- 测试过程中额外同步的基线修复：
- `frontend/src/components/account/__tests__/AccountUsageCell.spec.ts` 跟新签名 `getUsage(id, source?)` 对齐
- `backend/.golangci.yml` 增加 `gosec` 的 `G704` 排除，避免网关类仓库对受控上游请求产生系统性误报

## 版本链路核对

- upstream 最新 release tag：`v0.1.104`
- `upstream/main:backend/cmd/server/VERSION`：`0.1.104`
- 本地 `backend/cmd/server/VERSION`：`0.1.104`
- 版本对齐是否拆独立提交：否；本轮 upstream 自身已经把 `VERSION` 同步到 `0.1.104`

## 剩余风险与观察点

- 尚未执行真实部署，因此还没有核对：
- `deploy/version_resolver.sh` 的部署时解析结果
- 线上容器内二进制版本
- 页面实际展示版本

- 当前工作区仍保留用户自己的未提交改动，不在本轮 merge commit 中：
- `backend/internal/handler/concurrency_error_mapping_test.go`
- `frontend/src/components/keys/__tests__/UseKeyModal.spec.ts`
- `frontend/src/utils/__tests__/openclawConfig.spec.ts`
- `frontend/src/utils/openclawConfig.ts`

- 文档/规范漂移：
- 当前 GitHub Actions 已使用 Go `1.26.1`
- 仓库说明和 AGENTS 项目规范仍写 `1.25.7`
- 本轮已在测试矩阵和定制点文档中记录，建议后续统一修正文档

## 维护资产

- 本轮新增了以下 upstream sync 复用资产：
- `references/local-customizations.md`
- `references/testing-matrix.md`
- `scripts/collect_upstream_sync_context.sh`
- `scripts/scaffold_sync_review_doc.sh`

## 结论

- 建议保留当前 merge 结果
- merge commit：`60e2c6fc`
- 本轮已保住 APIPool 品牌/部署/OpenAI OAuth 定制，同时吸收了 upstream `v0.1.104` 相关功能与修复
