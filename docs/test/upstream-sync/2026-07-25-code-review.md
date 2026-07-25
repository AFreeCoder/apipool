# v0.1.165 上游同步双重代码评审报告

## 基本信息

- 日期：2026-07-25
- 评审基线：`fda30d430d8ac4282691178ea6b7381769fbca91`
- 同步评审终点：`1a0d54a59389003e59fe61f2e3a2f25cc3ab49f1`
- 评审范围：`git diff fda30d430d8ac4282691178ea6b7381769fbca91..1a0d54a59389003e59fe61f2e3a2f25cc3ab49f1`
- 评审方式：一个 subagent 使用 `requesting-code-review`，另一个 subagent 使用 gstack code review；两者只读、相互独立
- 接受方式：主 agent 使用 `receiving-code-review` 对调用链、迁移重试、缓存失效和 ReqLog 生命周期逐项复核，再补失败用例、实施最小修复并交回原评审者终审

## 结论

初审确认三个发布阻断项：`allow_live` 没有进入持久化鉴权缓存失效触发器；Live POST 虽进入 ReqLog 中间件但没有请求体快照；迁移 190 在并发索引构建中断后无法清理同名无效索引。另有一个本轮新引入的可观测性缺口：Composite resolver 和请求体超限错误发生在 ReqLog 之前。

四项均经独立调用链与失败测试确认有效，已接受并修复。两位原评审者终审均给出 `SHIP`；主 agent 使用显式 Colima Docker socket 补跑评审者环境中未能启动的 PostgreSQL integration，完整通过。

当前没有未闭环的本次同步发布阻塞项，可以进入 `apipool-push-deploy` 阶段。

## 发现与接收结论

| 发现 | 评审来源 | 核验结论 | 处置 |
| --- | --- | --- | --- |
| `allow_live` 未纳入 group 鉴权缓存 outbox 触发器 | requesting、gstack | 有效，Important/P1，授权策略可能短期 fail-open | 新增追加式迁移 191，重定义触发器函数并比较 `allow_live`；集成测试验证仅切换该字段也会入队 |
| Live POST 没有 ReqLog 请求体快照 | requesting | 有效，Important | 解析前单次读体并调用 `MaybeCaptureRequestBody`；JSON 从同一字节解析，multipart 重建 Body 后解析；分别验证文本和二进制元数据快照 |
| 迁移 190 无法从无效并发索引恢复 | gstack | 有效，P1，迁移重试可信度不足 | 将迁移 190 接入 `dropInvalidIndexIfPresent`；单测验证先删无效索引再重建，集成测试验证 `indisvalid` 与 `indisready` |
| Composite resolver/413 错误绕过 ReqLog | gstack | 有效，P2；属于本轮上游新增路径 | 保持鉴权与分组校验在前，把 ReqLog 调整到 Composite resolver 前；两条端到端测试验证 500 与 413 均提交一条正确状态日志 |
| 测试中 `t.Fatal` 后的静态空指针告警 | 完整 lint | 有效，CI 阻断 | 增加显式 `return`，不改变测试语义 |

## 定向验证

- `allow_live`：直接 SQL 切换权限会向 `auth_cache_invalidation_outbox` 写入绑定 API Key 的哈希缓存键。
- 迁移 190：检测到 `idx_users_email_dot_stripped` 无效时，先以 `DROP INDEX CONCURRENTLY` 清理，再执行原并发创建并记录 migration。
- Schema：邮箱别名索引同时满足 `indisvalid=true` 与 `indisready=true`。
- Live JSON：ReqLog 保存文本快照，解析结果保持原 session。
- Live multipart：ReqLog 按二进制策略只保存元数据和摘要，重建 Body 后仍正确解析 SDP 与 session。
- Composite：resolver 返回错误时记录状态 500；请求体超过上限时记录状态 413。

## 完整回归

- `make test-unit`：通过。
- 显式 Colima socket 的 `make test-integration`：通过。
- `golangci-lint run ./...`：通过，`0 issues`。
- `make build`：通过，后端版本 `0.1.165`。
- 前端源码未在接收评审阶段修改；同步评审终点已经通过 lint、typecheck、194 个测试文件、1392 个用例及生产构建。
- 主站与本地 Compose 均通过 `docker compose config -q`。
- `bash deploy/version_resolver.sh resolve .`：输出 `0.1.165`。
- `git diff --check`：通过。

## 评审边界

- 两个评审 subagent 均未修改、暂存、提交或推送文件，也未执行外部写入。
- 本轮新增数据库 migration 191，不回改已经可能应用的迁移 186 或 189。
- `docs/test/upstream-sync/issues.md` 中鉴权缓存完整失效遗留仍未全部关闭；本轮只闭环 Live 权限字段。
- 生产备份、回滚镜像、GitHub Actions 和真实域名健康检查由后续 `apipool-push-deploy` 阶段单独留证。
