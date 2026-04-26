# 上游同步评审记录（2026-04-26）

## 基线

- 当前分支：`main`
- 文档生成时 HEAD：`0d594a29c486942b91e6f58363dc3a00be10f56f`
- upstream merge commit：`b54b512fdaa6470ca10fe6a1af53130d603cefe5`
- 合入前本地基线：`8a4d1ca2de306be49bffbd3636113c07a1be4fc4`
- 上游引用：`upstream/main`
- 上游 SHA：`c056db740d56ce008292a7b414c804cc6f308208`
- 合入前 merge-base：`496469ac4e22a90f417ce1f1b48ff8868f938183`
- 同步方式：`git merge upstream/main`
- upstream 最新 release tag：`v0.1.119`
- upstream/main 版本文件：`0.1.119`
- 本地最终 VERSION：`0.1.119`
- 版本对齐：未拆独立 `chore(version)` 提交；上游本身已包含 `backend/cmd/server/VERSION=0.1.119`

## 上游更新摘要

- 合入 `upstream/main` 从 `496469ac4e22a90f417ce1f1b48ff8868f938183` 到 `c056db740d56ce008292a7b414c804cc6f308208`，包含 6 个上游提交（其中 4 个非 merge 提交）。
- 主要吸收内容：
  - `feat(affiliate): 完善邀请返利系统`：新增返利冻结期、返利有效期、单个被邀请人返利上限、冻结额度展示、被邀请人返利明细；OAuth 注册路径补充 `aff_code` 传递；新增迁移 `backend/migrations/133_affiliate_rebate_freeze.sql`。
  - `Fix Zpay refund endpoint handling`：增强 EasyPay/Zpay 退款请求，规范 `apiBase`，按订单号/交易号回退尝试，处理非 JSON/HTML 错误响应，并新增退款测试。
  - `fix(anthropic): 修正缓存 token 的 Anthropic 用量语义`：将 cached input tokens 归入 `cache_read_input_tokens`，并从普通 input tokens 中扣除。
  - `chore: sync VERSION to 0.1.119 [skip ci]`：同步运行时版本文件。
- 命中的高风险模块：
  - `backend/cmd/server/VERSION`
  - `backend/internal/handler/auth_oidc_oauth.go`
  - `backend/internal/handler/admin/setting_handler.go`
  - `backend/internal/service/setting_service.go`
  - `backend/internal/service/settings_view.go`
  - `frontend/src/views/admin/SettingsView.vue`
  - `frontend/src/api/admin/settings.ts`
  - `frontend/src/i18n/locales/en.ts`
  - `frontend/src/i18n/locales/zh.ts`
  - `frontend/src/utils/oauthAffiliate.ts`

## 本地定制保护点

- 品牌与文案：`APIPool` 站点默认名、首页/帮助文档、本地 Logo 和文案定制未被上游回退；本轮未触碰 `README.md`、`frontend/public/logo.svg`、`frontend/src/views/docs/`、`frontend/src/docs/`。
- 部署/回滚/版本链路：`.github/workflows/deploy.yml`、`deploy/rollback.sh`、`deploy/version_resolver.sh`、`deploy/docker-compose.deploy.yml`、`deploy/docker-compose.local.yml` 均未被上游覆盖；版本文件、upstream tag、version resolver 输出均为 `0.1.119`。
- OpenAI OAuth / Codex 兼容：本轮上游未触碰 APIPool 的 OpenAI gateway / Codex 兼容核心文件；既有停用账号、403、reasoning/transcript 归一化、Codex fallback 逻辑仍保留。
- Kiro / OpenClaw：Kiro 服务、路由、前端 OAuth/composable、OpenClaw 配置导入未被覆盖；`frontend/src/types/index.ts` 仍保留 `AccountType` 中的 `kiro`。
- 后台入口与默认配置：APIPool 的 `purchase_subscription_enabled` / `purchase_subscription_url` iframe 充值入口继续保留；本地 AppHeader 跑马灯 `marquee_enabled` / `marquee_messages` 与上游新增 affiliate 设置并存。

## 冲突与取舍

- Git 冲突文件：
  - `.gitignore`：上游新增 `.codex`，本地已有 `.worktrees/`；最终同时保留两条规则。
- 无 Git 冲突但做过语义复核的热点：
  - `frontend/src/views/admin/SettingsView.vue`：确认 APIPool 购买 iframe 设置、跑马灯设置和上游 affiliate 冻结期/有效期/单人上限字段全部保留。
  - `backend/internal/handler/admin/setting_handler.go`、`backend/internal/handler/dto/settings.go`、`backend/internal/service/setting_service.go`、`backend/internal/service/settings_view.go`：确认 purchase、marquee、本轮 affiliate 设置字段均贯通到 API、公开设置和默认值。
  - `backend/internal/handler/auth_oidc_oauth.go`：保留 APIPool 既有 ECDSA JWK 校验改动，同时吸收上游 `AffCode` 参数传递。
  - `frontend/src/views/auth/EmailVerifyView.vue` 与 OAuth callback 相关文件：吸收上游 referral code 持久化和 OAuth pending flow `aff_code` 传递。
  - `frontend/src/views/auth/__tests__/EmailVerifyView.spec.ts`：首次完整前端测试暴露上游新增断言缺少 referral code 测试种子；已在独立提交 `0d594a29` 补齐 `storeAffiliateReferralCode('AFF123')`，未改生产代码。

## 测试记录

- `cd backend && go test -tags=unit ./...`：通过。
- `cd backend && make test-unit`：通过。
- `cd backend && go test -tags=integration ./...`：通过。
- `cd backend && make test-integration`：通过。
- `cd backend && golangci-lint run ./...`：通过，`0 issues`。
- `pnpm --dir frontend run lint:check`：通过。
- `pnpm --dir frontend run typecheck`：通过。
- `pnpm --dir frontend run test:run`：首次运行失败 1 个测试：`EmailVerifyView.spec.ts` 期望 `aff_code: "AFF123"`，但用例没有在 `register_data` 或 `localStorage` 中提供该来源。补齐测试种子后重跑通过，最终结果 `96 passed / 562 tests`。
- `pnpm --dir frontend run test:run -- src/views/auth/__tests__/EmailVerifyView.spec.ts`：通过；该命令在当前 Vitest 配置下实际运行全量测试，结果同为 `96 passed / 562 tests`。
- `git tag --merged upstream/main --sort=-version:refname | head -1`：`v0.1.119`。
- `cat backend/cmd/server/VERSION`：`0.1.119`。
- `bash deploy/version_resolver.sh resolve .`：`0.1.119`。
- `make build`：通过；后端构建使用 `-X main.Version=0.1.119`，前端 Vite 仅输出既有 dynamic import / chunk size warning。
- `POSTGRES_PASSWORD=dummy docker compose -f deploy/docker-compose.deploy.yml config -q`：通过。
- `POSTGRES_PASSWORD=dummy docker compose -f deploy/docker-compose.local.yml config -q`：通过。
- `bash scripts/collect_upstream_sync_context.sh --no-fetch`：通过；最终 `upstream_only_commits=0`，`latest_upstream_tag=v0.1.119`，`local_version=0.1.119`，`upstream_version=0.1.119`。
- 部署后版本输出 / 页面版本人工核对：本轮未部署，未执行线上 `docker exec sub2api /app/sub2api --version` 或页面左上角版本人工核对。

## 剩余风险与观察点

- 上游新增 affiliate 冻结期/有效期/单人上限与迁移 `133` 已通过本地回归；上线后仍需观察迁移执行、返利冻结解冻、返利转余额、被邀请人返利明细。
- OAuth referral code 现在贯通普通注册、LinuxDo/OIDC/WeChat OAuth 和 pending create-account；测试已覆盖，但上线后建议观察带 `aff` / `aff_code` 链接进入第三方 OAuth 后的绑定结果。
- EasyPay/Zpay 退款处理更健壮，但实际支付平台错误响应形态可能和测试样例不同；上线后建议重点观察退款失败日志和 provider 返回摘要。
- Anthropic cached token usage 语义改变会影响展示/计费拆分；上线后建议观察 Responses-to-Anthropic 兼容路径的 input/cache_read 统计。
- 本轮未执行真实部署，因此运行中容器版本、页面左上角版本、数据库备份和镜像回滚 tag 仍需在部署阶段复核。
- 技能资产复核：`references/local-customizations.md`、`references/testing-matrix.md`、`scripts/collect_upstream_sync_context.sh`、`scripts/scaffold_sync_review_doc.sh` 当前仍覆盖本轮暴露的高风险模式，无需修改。

## 结论

- 建议保留当前结果。上游 `v0.1.119` 已合入，APIPool 的品牌、购买 iframe、跑马灯、部署/回滚、OpenAI OAuth/Codex、Kiro/OpenClaw 定制均已保留；本地完整回归、静态检查、构建、Compose 配置和版本链路校验均已通过。
