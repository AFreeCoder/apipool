# 上游同步评审记录（2026-04-08）

## 基线

- 当前分支：`main`
- 当前 HEAD：`5d3ccd6e52da77d2351855eb5a68b56d039aabcf`
- 合入前本地基线：`30e92a00dc0af2a9e067883c37358a99a9b47866`
- 上游引用：`upstream/main`
- 上游 SHA：`0d69c0cd643bab82bd011682407030955f2389a7`
- merge-base：`06e2756ee4322d5ea08f2d258e078b902a6cc127`
- 同步方式：`git merge upstream/main`
- upstream 最新 release tag：`v0.1.110`
- upstream/main 版本文件：`0.1.110`
- 本地最终 VERSION：`0.1.110`

## 上游更新摘要

- 本轮合入 `06e2756e..0d69c0cd`，核心吸收内容包括：Go 从 `1.26.1` 升到 `1.26.2`、`backend/cmd/server/VERSION` 对齐到 `0.1.110`、网关转发新增 `enable_cch_signing` 与 billing header `cc_version` 同步、OpenAI 非 Codex 客户端新增基于内容的会话哈希 fallback、空 base64 图片输入清洗，以及 Gemini / Antigravity 的工具归一化修复。
- 额外吸收了 upstream 的 CI / Docker 基线更新：workflow 校验版本切到 `go1.26.2`，根 `Dockerfile` 与 `deploy/Dockerfile` 的 Go builder 镜像同步升级。
- 命中的高风险模块：
  - `README.md`
  - `backend/cmd/server/VERSION`
  - `backend/go.mod`
  - `backend/internal/service/openai_gateway_service.go`
  - `backend/internal/service/openai_gateway_service_test.go`
  - `backend/internal/service/setting_service.go`
  - `backend/internal/service/settings_view.go`
  - `backend/internal/handler/admin/setting_handler.go`
  - `backend/internal/handler/dto/settings.go`
  - `deploy/Dockerfile`
  - `frontend/src/api/admin/settings.ts`
  - `frontend/src/i18n/locales/en.ts`
  - `frontend/src/i18n/locales/zh.ts`
  - `frontend/src/views/admin/SettingsView.vue`

## 本地定制保护点

- 品牌与文案：
  - 保留 APIPool 本地 README 结构，不回退为 upstream 的 sponsors / ecosystem 文档布局。
  - 保留 `site_name: APIPool`、多语言 APIPool 文案，以及设置页支付集成文档链接 `AFreeCoder/apipool`。
- 部署 / 回滚 / 版本链路：
  - 保留 DigitalOcean 单机 Compose、数据库备份和镜像回退这套本地部署约定。
  - 吸收 `deploy/Dockerfile` 的 Go 1.26.2 builder 升级，同时保留本地 `VERSION` 回退与 `main.Version` 注入逻辑。
  - 最终版本链路对齐为：upstream tag `v0.1.110` / upstream VERSION `0.1.110` / 本地最终 VERSION `0.1.110`，未额外拆分 `chore(version)`。
- OpenAI OAuth / Codex 兼容：
  - 保留 OAuth passthrough 输入归一化、WS fallback 状态持久化和 APIPool 的 Codex/OpenAI 兼容修复。
  - 吸收 upstream 的内容哈希 fallback 与空 base64 图片清洗，不覆盖现有兼容逻辑。
- 后台入口与默认配置：
  - 保留 APIPool 默认站点名与设置页品牌入口。
  - 吸收 `enable_cch_signing` 的后端 DTO、服务层、API contract、前端设置页和多语言文案串联。

## 冲突与取舍

- 显式 Git 冲突文件：
  - `README.md`
- 冲突处理方式：
  - `README.md` 选择保留 APIPool 本地文档结构，只手工吸收与本轮同步相关的版本信息，将 Go 版本说明改为 `1.26.2`，不回退到 upstream 的赞助商 / ecosystem 展示。
- 无冲突但做过语义复核的热点文件：
  - `backend/internal/service/openai_gateway_service.go`
  - `backend/internal/service/openai_gateway_service_test.go`
  - `backend/internal/service/setting_service.go`
  - `backend/internal/service/settings_view.go`
  - `backend/internal/handler/admin/setting_handler.go`
  - `backend/internal/handler/dto/settings.go`
  - `frontend/src/api/admin/settings.ts`
  - `frontend/src/i18n/locales/en.ts`
  - `frontend/src/i18n/locales/zh.ts`
  - `frontend/src/views/admin/SettingsView.vue`
  - `deploy/Dockerfile`
  - `.github/workflows/backend-ci.yml`
  - `.github/workflows/security-scan.yml`
  - `.github/workflows/release.yml`
- 语义复核结论：
  - `openai_gateway_service.go` 中，本地的 OAuth passthrough 输入归一化、WS fallback 状态持久化仍在；upstream 的 `deriveOpenAIContentSessionSeed` 和 `sanitizeEmptyBase64InputImages*` 也已合入。
  - `setting_service.go` / `SettingsView.vue` 中，本地 APIPool 默认值与文档入口仍在，同时 `enable_cch_signing` 已从保存逻辑贯通到前后端视图。

## 测试记录

- [x] `cd backend && go test ./...`
- [x] `cd backend && make test-unit`
- [ ] `cd backend && make test-integration`
  - 启动 Docker Desktop 后，原先因 `testcontainers` 找不到 daemon 而失败的 `internal/middleware` 与 `internal/server/routes` 已恢复通过。
  - 当前仅剩 `backend/internal/pkg/tlsfingerprint` 失败，表现为对外部探针 `https://tls.peet.ws/api/all` 和 `https://tls.sub2api.org:8090` 的 TLS handshake EOF。
  - 已在 merge 前临时 worktree（`HEAD=30e92a00`）复跑 `go test -tags=integration ./internal/pkg/tlsfingerprint`，同样失败，确认不是本轮 upstream sync 引入的回归。
- [x] `cd backend && golangci-lint run ./...`
- [x] `pnpm --dir frontend run lint:check`
- [x] `pnpm --dir frontend run typecheck`
- [x] `pnpm --dir frontend run test:run`
  - `51` 个文件、`303` 个测试全部通过。
- [x] `git tag --merged upstream/main --sort=-version:refname | head -1`
  - 输出：`v0.1.110`
- [x] `cat backend/cmd/server/VERSION`
  - 输出：`0.1.110`
- [x] `make build`
  - 后端构建成功，前端 Vite 构建成功；仅有 chunk size warning，无构建失败。
- [x] `POSTGRES_PASSWORD=dummy docker compose -f deploy/docker-compose.deploy.yml config -q`
- [x] `POSTGRES_PASSWORD=dummy docker compose -f deploy/docker-compose.local.yml config -q`
- [x] `bash deploy/version_resolver.sh resolve .`
  - 输出：`0.1.110`
- [ ] 部署后运行时版本 / 页面版本核对
  - 本轮未部署，未执行线上容器或页面版本确认。

## 剩余风险与观察点

- 当前剩余风险主要在外部依赖，而不是 merge 结果本身：
  - `backend/internal/pkg/tlsfingerprint` 的 integration 依赖公网探针服务，当前环境与 merge 前基线都存在 TLS handshake EOF。
- 本轮未部署，仍缺少两项生产侧确认：
  - 部署后用 `docker exec sub2api /app/sub2api --version` 或等价方式确认运行中二进制版本为 `0.1.110`
  - 页面左上角 / 公开设置返回的运行时版本确认为 `0.1.110`
- 观察点：
  - `enable_cch_signing` 在真实 OpenAI OAuth / Codex 流量下的 billing header 行为
  - 非 Codex 客户端内容哈希 fallback 是否符合当前账号粘性预期
  - Go `1.26.2` 升级后 CI / release 构建环境是否继续稳定

## 结论

- [x] 建议保留当前 merge 结果
- [x] merge commit：`5d3ccd6e52da77d2351855eb5a68b56d039aabcf`（`merge: sync upstream/main to v0.1.110`）
- [x] 版本结论：
  - upstream 最新 release tag：`v0.1.110`
  - upstream/main `backend/cmd/server/VERSION`：`0.1.110`
  - 本地最终 `backend/cmd/server/VERSION`：`0.1.110`
  - 版本对齐未拆分独立提交，原因是 upstream tag 与 VERSION 已一致
