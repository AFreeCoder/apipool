# 上游同步与评审修复验证记录（2026-07-16）

## 验证范围

- 合入前本地基线：`1c032621fa222f7acdbc181a5bb3c14ec634d3bc`
- 上游引用：`upstream/main`
- 上游 SHA：`bc2244c83fd8e92769d89ca01eb980513a720486`
- 上游 release tag：`v0.1.158`
- 上游合并提交：`0250f7aa00ff1efd9fb92f28e6e8de353dcd6bea`
- 同步记录提交：`0d1d7534a`
- 本地最终版本：`0.1.158`
- 本轮额外范围：双路代码评审发现的 SSRF、审计清空、step-up、审计截断/批量写入与异步任务超时止损

## 评审修复验证

- 异步图片 URL：
  - 私网 IP 初始 URL 被策略拒绝。
  - 跳转到 `169.254.169.254` 云元数据地址被拒绝。
  - socket 层使用安全 dialer 复核解析后 IP，覆盖 DNS rebinding。
  - 文本/元数据等非真实图片字节不再按 `image/png` 上传。
- 审计日志：
  - `COUNT + TRUNCATE + clear trace` 在单个数据库事务内完成；留痕写入失败会回滚。
  - 清空前已进入异步 batch 的旧记录不会在清空后写回。
  - 清空事务失败时不会丢弃排队记录。
  - COPY 批量写入失败会逐条降级，避免一条坏记录拖掉整批。
  - 超长中文请求体截断后保持合法 UTF-8、合法 JSON 且不超过 16 KiB。
- step-up：
  - 无 `sid` 的旧 token 不再使用用户级授权键，返回 `STEP_UP_SESSION_REQUIRED`。
  - TOTP service 在验证码校验前防御性拒绝空 session key。
  - 前端将旧会话错误识别为不可重试门控并提示重新登录。
- 异步任务：
  - 超过执行超时和宽限期仍处于 `processing` 的任务，在轮询时转为 `failed`。
  - 完整持久队列仍作为独立遗留；生产部署门禁要求 `image_storage.enabled=false`。

## 完整回归

- `GOMAXPROCS=2 GOTOOLCHAIN=auto GOFLAGS='-p=1' go test -tags=unit ./...`：通过。
- `GOMAXPROCS=2 GOTOOLCHAIN=auto GOFLAGS='-p=1' go test -tags=integration ./...`：通过。
- `GOTOOLCHAIN=auto golangci-lint run ./...`：通过，`0 issues`。
- `pnpm run lint:check`：通过。
- `pnpm run typecheck`：通过。
- `pnpm run test:run`：172 个测试文件、1200 个测试全部通过。
- `make build`：后端 `Version=0.1.158` 与前端 Vite 生产构建通过；仅有既有动态/静态导入和大 chunk 警告。
- `git diff --check`：通过。
- `cat backend/cmd/server/VERSION`：`0.1.158`。
- `bash deploy/version_resolver.sh resolve .`：`0.1.158`。
- `git tag --merged upstream/main --sort=-version:refname | head -1`：`v0.1.158`。

## receiving review 结论

- SSRF、审计清空竞态、旧会话 step-up 串授权、UTF-8/批量丢审计均已完整修复并通过回归。
- 异步任务重启不可续跑属于有效架构缺口；本轮已消除长期假处理中状态，但未声称具备持久执行能力。
- 在生产确认 `image_storage.enabled=false`、部署备份与上一镜像回滚链路正常的前提下，可以进入 `apipool-push-deploy` 阶段。
