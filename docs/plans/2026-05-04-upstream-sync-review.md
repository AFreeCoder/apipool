# 上游同步评审记录（2026-05-04）

## 基线

- 当前分支：`main`
- 合入后 HEAD（merge commit）：`f8ba14e46addc48feeab9a4d0c159a912ca9da65`
- 合入前本地基线：`d7a77ae2c2dc8f36bedc05ed4c39e8404e2d2073`
- 上游引用：`upstream/main`
- 上游 SHA：`4de28fec8c061ee5f0bad93e885c07fced41c864`
- merge-base：`df722c9a6e97312491232c11bf305d5f93b45e04`
- 同步方式：`git merge upstream/main`
- merge commit：`f8ba14e46addc48feeab9a4d0c159a912ca9da65`
- upstream 最新 release tag：`v0.1.123`
- upstream/main 版本文件：`0.1.123`
- 本地最终 VERSION：`0.1.123`
- 工作区说明：合入前已有 4 个未跟踪 `docs/superpowers/...` 文档；本轮未处理、未覆盖这些文件。

## 上游更新摘要

- 本轮 `HEAD..upstream/main` 只有 1 个 upstream-only 提交：`4de28fec chore: sync VERSION to 0.1.123 [skip ci]`。
- 实际吸收的代码变更只有 `backend/cmd/server/VERSION`：`0.1.122` -> `0.1.123`。
- 命中的高风险模块仅为版本链路：`backend/cmd/server/VERSION`。
- `scripts/collect_upstream_sync_context.sh --no-fetch` 复查结果：`upstream_only_commits: 1`、`File Overlap: <none>`、`High-risk Overlap: <none>`。

## 本地定制保护点

- 品牌与文案：本轮 upstream 未触碰 `README.md`、`frontend/public/logo.svg`、`frontend/src/i18n/locales/`、`frontend/src/views/docs/`、`frontend/src/docs/`，APIPool 品牌与帮助文档入口无变化。
- 部署/回滚/版本链路：`.github/workflows/deploy.yml`、`deploy/rollback.sh`、`deploy/version_resolver.sh`、`deploy/docker-compose.deploy.yml`、`deploy/docker-compose.local.yml` 均未被 upstream 覆盖；版本已与 upstream tag 对齐为 `0.1.123`，不需要额外 `chore(version)` 提交。
- OpenAI OAuth / Codex 兼容：本轮 upstream 未触碰 `backend/internal/service/openai_*`、`backend/internal/handler/openai_*`、`ratelimit_service.go` 或 token refresh 相关实现，既有 APIPool 语义未变。
- Kiro / OpenClaw：本轮 upstream 未触碰 `kiro_*` service、Kiro OAuth handler/routes、前端 Kiro OAuth 与 OpenClaw 配置导入文件。
- 后台入口与默认配置：本轮 upstream 未触碰 `SettingsView.vue`、`AppSidebar.vue`、`/purchase`、备份页或表格分页默认值相关实现。

## 冲突与取舍

- Git 冲突：无。`git merge-tree $(git merge-base HEAD upstream/main) HEAD upstream/main` 预检显示只有 `backend/cmd/server/VERSION` 一行版本更新。
- 语义取舍：接受 upstream 的版本号更新；不需要手动吸收其他 upstream 行为，也没有需要拒绝的 upstream 覆盖。
- 版本处理：版本文件变更来自 upstream commit 本身，因此没有拆分独立 `chore(version)` 提交。
- 技能资产维护：本轮发现仓库内 `scripts/scaffold_sync_review_doc.sh` 会覆盖同日已存在评审文档，已同步增加默认不覆盖保护和 `--force` 显式覆盖开关。

## 测试记录

- 通过：`cd backend && make test-unit`，包装入口实际执行 `go test -tags=unit ./...`。
- 环境失败：`cd backend && make test-integration`，包装入口实际执行 `go test -tags=integration ./...`；失败点为 `internal/server/routes.TestAuthRegisterRateLimitThresholdHitReturns429`，原因是 `testcontainers` 无法连接 Docker daemon。`docker info` 同样确认 `Cannot connect to the Docker daemon at unix:///var/run/docker.sock`，属于本机环境限制，未见代码断言失败。
- 通过：`cd backend && golangci-lint run ./...`，输出 `0 issues.`。
- 通过：`pnpm --dir frontend run lint:check`。
- 通过：`pnpm --dir frontend run typecheck`。
- 未运行：`pnpm --dir frontend run test:run`；本轮没有前端逻辑或界面文件变更。
- 通过：`git tag --merged upstream/main --sort=-version:refname | head -1` 输出 `v0.1.123`。
- 通过：`cat backend/cmd/server/VERSION` 输出 `0.1.123`。
- 通过：`make build`，后端构建参数为 `main.Version=0.1.123`；前端 Vite 构建通过，仅输出既有 dynamic import / 大 chunk 警告。
- 通过：`POSTGRES_PASSWORD=dummy docker compose -f deploy/docker-compose.deploy.yml config -q`。
- 通过：`POSTGRES_PASSWORD=dummy docker compose -f deploy/docker-compose.local.yml config -q`。
- 通过：`bash deploy/version_resolver.sh resolve .` 输出 `0.1.123`。
- 通过：`bash scripts/collect_upstream_sync_context.sh --no-fetch`。
- 通过：`bash scripts/scaffold_sync_review_doc.sh --output /tmp/apipool-sync-review-scaffold-test.md`。
- 通过：`bash scripts/scaffold_sync_review_doc.sh --output /tmp/apipool-sync-review-scaffold/nested-test.md --force`，验证自定义输出目录会自动创建。
- 通过：`bash -n scripts/scaffold_sync_review_doc.sh`。
- 预期拒绝覆盖：`bash scripts/scaffold_sync_review_doc.sh` 在同日文档已存在时退出并提示使用 `--force` 或 `--output`。
- 未执行：部署后线上版本输出 / 页面版本人工核对；本轮仅完成本地合入与验证，尚未 push/deploy。

## 剩余风险与观察点

- Docker 依赖的 integration 测试未能在本机完成；push 或部署前需要在 Docker daemon 可用的环境重跑 `cd backend && make test-integration`，至少确认 `internal/server/routes` 通过。
- 本轮没有 API contract、配置默认值、OpenAI/Kiro 网关、前端入口或部署脚本行为变化，业务风险集中在“运行时版本展示从 `0.1.122` 变为 `0.1.123`”。
- 若继续发布，应切换到 `apipool-push-deploy` 流程，确认 Actions、GHCR 镜像 tag、生产容器健康、数据库/镜像备份和 rollback metadata。

## 结论

- 建议保留当前合入结果。本轮同步仅吸收 upstream `v0.1.123` 版本文件更新，APIPool 的品牌、部署/回滚、OpenAI/Codex、Kiro/OpenClaw、purchase 入口和后台默认值定制均未被触碰。
- 当前本地 `main` 需要保留的同步结果包括：`4de28fec` upstream 版本提交、`f8ba14e4` merge commit、本评审文档，以及脚手架防覆盖保护。
