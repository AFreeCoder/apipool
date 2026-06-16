# 用户请求明细日志（reqlog）实现验证报告

> 日期：2026-06-16
> 验证对象：commit `1f4f7c17`（feat(reqlog): 新增用户请求明细日志功能）/ 分支 `feat/user-request-detail-logging`
> 权威设计：[docs/design/user-request-detail-logging.md](./design/user-request-detail-logging.md)（草案 v4）
> 验证方：Claude Code 与 Codex CLI 各自独立对照设计文档做静态审查 + reqlog 相关包级单测，再交叉合并结论。

---

## 0. 验证方法

- **Claude**：逐文件审查核心硬规则落地情况（NB1 深拷贝 / NB2 录制判定 / NM3 计数 / B2 截断 / B3 绝对 TTL / 10 个埋点 / WS / Flush 透传 / 下载鉴权 / 配置默认与校验）。
- **Codex**（full access，`codex exec --dangerously-bypass-approvals-and-sandbox`）：独立对照设计逐项核对，输出带 file:line 的问题清单，并运行包级单测。
- 两份结论交叉比对、就根因与严重度达成一致后形成本报告。

### 单测结果（双方一致）

```bash
cd backend
go test -tags=unit ./internal/pkg/reqlog ./internal/server/middleware ./internal/service ./internal/repository ./internal/handler/admin
```

结果：通过。

### 一处需澄清的背景（已达成一致）

Codex 中途从本机 Chronicle 屏幕录制 OCR 数据里读到过一条 `panic: init reqlog redis: redis: invalid URL scheme`，疑指 `reqlog_redis.go` 调用 `redis.ParseURL("")`。Claude 用代码证据核实：

- 全代码库无 `init reqlog redis` 字符串、无 `invalid URL scheme` 来源；
- reqlog redis 初始化路径 `ProvideReqLogRedis → buildRedisOptionsFromRedisConfig → redis.NewClient(&redis.Options{Addr:...})` **不调用 `redis.ParseURL`**（`ParseURL` 仅出现在一个 benchmark 测试里）；
- `redis.NewClient` 不解析 URL scheme、不返回 error、空 Addr 也不 panic。

**结论：该 panic 不成立**，系外部 OCR 数据污染，不作为本次问题。Codex 在最终报告中已撤回，仅作背景核对。

---

## 1. 总体结论（双方一致）

实现质量高、主干能力完整并紧贴设计：

- ✅ NB1 深拷贝：`CaptureWriter.CapturedCopy()=cloneBytes`、`RequestBodySnapshot` 克隆、`Submit` 再 `DeepCopy`，无零拷贝投递 pooled buffer。
- ✅ NB2 录制判定：generation 原子计数 + 热路径 gen 失效；middleware `started.After(expires)` 跳过；worker `process` 二次校验；Lua `writeScript` 校验 `EXISTS enabled` + `entryTs ≤ expiresAt`，丢弃不补落。
- ✅ NM3 计数：idx 成员 `"seq:size"`、reconcile 对称扣账 + clamp、LPOP nil 校准、末尾 LLEN 校准。
- ✅ B2：入队前截断、body 存原始字符串（非 base64）、sink `inflightBytes` 字节预算。
- ✅ B3：`absTTL = expires_at + retention - now`，每次写入 PEXPIRE 幂等重设（item/sess/seq/idx）。
- ✅ 10 个请求体埋点齐全；middleware 注册于全部网关路由（含 `/v1beta` Google、WS、antigravity）。
- ✅ 下载执行端点移出 admin 组（`registerReqLogDownloadRoutes` 挂 v1），handler 内 `authenticateReqLogDownload` 支持一次性 token / admin 凭据。
- ✅ Flush 透传、嵌套 OpsErrorLogger 内层回收约定，有 nesting 透明性单测。
- ✅ 配置有 viper 默认值 + `validateOpsRequestLog` 校验，旧部署升级不会因缺省启动失败。

**但仍存在若干与设计不符 / 真实缺陷，需修复后再上线。** 以下问题编号沿用合并后的统一编号（标注来源 C=Claude / X=Codex，重叠的合并）。

---

## 2. 问题清单（已就根因与严重度达成一致）

### P1 [High]（X#1）旧会话在途 entry 会被写进新会话（force 重开 / disable+重开场景）

- 位置：`server/middleware/reqlog_capture.go:172`、`service/reqlog_sink.go:174`、`repository/reqlog_store.go:209`
- 现象：中间件入队的 entry 带请求开始时的 `state.SessionID`；worker `process` 仅按 `entry.UserID` 重新读"当前 enabled 会话"，只校验 `EXISTS enabled` + `timestamp ≤ expiresAt`，**未校验 `entry.SessionID == enabled.SessionID`**；随后 `WriteItem` 又把 `entry.SessionID` 覆盖为当前会话 id。
- 触发：会话 A 录制中、请求 R 入队 → admin `force=true` 重开会话 B（或 disable 后快速重开）→ worker 处理 R 时 enabled 已指向 B → R 被写进 B。
- 与设计冲突：违反 §3.1.1「一次排查=一个会话」、§3.1.2「force 结束旧会话、旧数据进保留期」、NB2 不补落语义。纯 disable（无重开）因 `enabled==nil` 会走 DropItem 不受影响；本问题仅在新会话存在时发生。
- 根因：worker / store 未把 `entry.SessionID` 当作不可变归属参与二次校验，且 `WriteItem` 无条件覆盖 SessionID。
- 修复：worker 读到 enabled 后要求 `enabled.SessionID == entry.SessionID`，否则丢弃（计 dropped）；`WriteItem` 不覆盖、改为防御性断言一致；Lua 增加 expected-session 参数校验。

### P2 [High]（X#2，含 C#2）worker 无 Redis 内存护栏，仅 Enable 时检查一次

- 位置：`service/reqlog_sink.go:174`(process)、`service/reqlog_service.go:458`(CheckMemory)
- 现象：设计 §3.6.2 防线4 要求 enable 时与 **worker 周期性**（非每条，配合 `MemoryInfoCacheTTL` 缓存）调用 `INFO memory`，越线则拒绝写入并标记 dropped。当前 worker 写入前完全不检查内存。
- 附带（C#2）：`CheckMemory` 仅当 `maxmemory>0` 才算 Guarded；独立实例若未设 maxmemory，护栏静默失效且无任何提示。
- 与设计冲突：§3.6.2 防线4 未落地。后果按 §3.6.1：写入接近打满的（尤其共享）Redis 可能逐出计费/配额 key。
- 缓解：防线1（每会话 max_bytes × max_concurrent 硬上界，默认 ≈96MB）已实现，故非致命，但 fail-safe 缺失。
- 修复：给 worker 注入内存 guard（按 `MemoryInfoCacheTTL` 读缓存的 INFO memory）；guarded 时 DropItem 不写；`maxmemory=0` 保持跳过但记一次告警。

### P3 [Medium]（X#3）共享档默认并发未按设计降为 1，生产独立实例未强约束

- 位置：`config/config.go:1779,1785`(默认)、`repository/reqlog_redis.go:16`、`deploy/config.example.yaml`
- 现象：设计 §3.5.1 / §3.6.3 规定共享 Redis 档默认 `max_concurrent_sessions=1`、独立档默认 3，且生产强制独立。当前默认 `use_dedicated=false` + `max_concurrent_sessions=3`。
- 与设计冲突：共享档默认并发偏高，逻辑上界 3×32MB 落在主 Redis 上，风险高于设计意图。
- 修复：当 `!use_dedicated` 时把 `max_concurrent_sessions` 默认/钳制为 1 并告警；`deploy/config.example.yaml` 与注释明确生产应 `use_dedicated=true`。（不采用 Codex「enabled+!dedicated 直接拒绝启动」的强约束——设计允许开发/测试共享档。）

### P4 [Medium]（X#4）`sessions:{uid}` 绝对 TTL 非原子，崩溃会留下无 TTL ZSet

- 位置：`repository/reqlog_store.go:138,455,624`(Lua ZADD 后 Go 侧 refreshSessionsTTL)
- 现象：`reqLogEnableSessionScript` 内 `ZADD sessionsKey` 但不设 TTL，TTL 由 Lua 外 `refreshSessionsTTL()` 设置。两步非原子，进程在中间崩溃或 refresh 出错会留下无 TTL 的 `sessions:{uid}`，违反 B3。
- 修复：把 TTL 维护移入 Lua（ZADD 后取最大 score 对 sessionsKey `PEXPIREAT`）。

### P5 [Medium]（X#5 = C#1）`DropItem` 可创建无 TTL 的 session hash

- 位置：`repository/reqlog_store.go:242`(DropItem)、`service/reqlog_sink.go:164,174`
- 现象：worker 发现 `enabled==nil` 时构造 fallback state 调 `DropItem`，其直接 `HINCRBY sess dropped_count`，不检查 key 是否存在、不设 TTL。若 sess hash 已过期/被逐出，HINCRBY 会重建一个无 TTL 的 hash，永久驻留，违反 B3。
- 修复：DropItem 改 Lua：仅 `EXISTS sess` 才 HINCRBY，并按绝对 TTL 重设；无法安全计算 TTL 时只增进程内 dropped 指标、不建 Redis key。

### P6 [Medium]（X#6）签发下载 token 未先反查 uid

- 位置：`handler/admin/ops_request_log_handler.go:155`、`service/reqlog_service.go:405`、`repository/reqlog_store.go:385`
- 现象：设计 §3.8 line 575 要求按 sid 的接口（含 `download-token`）必须先反查 uid。当前 `CreateDownloadToken` 直接写 token，可为不存在/过期/拼错的 sid 签发有效 token，下载时才 404。
- 修复：`CreateDownloadToken` 开头先 `ResolveSessionUser`，不存在返回 `ErrReqLogNotFound`；可把 uid 写进 token payload。

### P7 [Low-Medium]（X#7）下载 NDJSON 首行 metadata 缺字段

- 位置：`handler/admin/ops_request_log_handler.go:200`
- 现象：设计 §3.8 line 578 指定首行 metadata 含 `schema_version/session_id/user_id/started_at/expires_at/item_count/truncated`；当前仅 `schema_version/session_id/user_id`。
- 修复：下载前读 `GetStats`，补齐 started_at/expires_at/item_count/truncated（及 dropped_count）。

### P8 [Low-Medium]（X#8）entry 缺 `response_captured` 字段

- 位置：`pkg/reqlog/types.go:212`、`server/middleware/reqlog_capture.go:139`
- 现象：设计 §3.2.3 要求 WS / 无 body 元数据 GET 标记 `response_captured=false`。当前 entry 无该字段，`responseCaptured=false` 时只是把 respBody 置 nil，无法区分"设计上没录 / 录了但空 / 捕获失败"。
- 修复：`ReqLogEntry` + JSON DTO 新增 `ResponseCaptured bool`；HTTP/SSE 正常捕获置 true，WS / 元数据 GET 置 false；前端据此展示"响应未捕获"。

### P9 [Low]（C#3）worker 把被 Lua 丢弃的 entry 计为 writtenCount

- 位置：`service/reqlog_sink.go:191-196`、`repository/reqlog_store.go:233-236`
- 现象：Lua `writeScript` 因 disabled/expired/oversize/full 返回 `{0,...}` 时 `WriteItem` 返回 `(0,nil)` 无 err，process 仍 `writtenCount++`，健康指标 written/dropped 失真。
- 修复：`WriteItem` 返回是否写入；written==0 时 sink 计 dropped 而非 written。

### P10 [Low]（X#9）Redis 中 item 原始 JSON 的 `seq` 恒为 0 —— 暂不修

- 位置：`repository/reqlog_store.go:211,821,823`
- 现象：`WriteItem` 在 Lua `INCR` 分配 seq 前就 marshal entry，Lua 原样 SET，故 Redis 原始 JSON 内 `seq=0`。但读取/下载路径均在 `getItemByUID` 用 list member 的 seq 覆盖 `entry.Seq`，对外暴露的 seq 正确。
- 结论：纯属 Redis 原始字符串的内部不一致，**当前任何对外输出都正确**。修复需在 Lua 内 `cjson` 解码改写再编码（每条写入增加开销）。**决定暂不修**，记为已知次要项；若将来改为直接流式输出 Redis 原文，再处理。

### P11 [Low]（X#10）热路径未实现全局"无录制快速放行"标志 —— 暂不修

- 位置：`server/middleware/reqlog_capture.go:35`、`service/reqlog_service.go:168`
- 现象：设计 §3.2.2 把全局 any-recording 标志列为"推荐优化"。当前有 per-user 正/负缓存，但无全局原子标志，未录制用户在负缓存过期后仍按 user 维度回源 Redis。
- 结论：已有正/负缓存把开销降得很低，且属"推荐"非"必须"。**本轮决定暂不修**（避免冷启动初始化/计数维护引入风险），记为后续优化项。

---

## 3. 修复决定（达成一致）

- **本轮修复**：P1、P2、P3、P4、P5、P6、P7、P8、P9。
- **暂不修（记为已知次要/后续优化）**：P10（cosmetic，对外无影响）、P11（推荐优化，现有缓存已够）。

修复后由 Claude 与 Codex 各自复验；发现新问题回到分析→修复流程，直到双方一致通过。

---

## 4. 复验轮次（Round 2）

修复实施后，Claude 与 Codex 各自复验工作区相对 `1f4f7c17` 的改动。

### Claude 复验（全绿）
- `go build ./...` 通过；`go test -tags=unit ./...` 全量通过（含新增回归测试：sink P1/P2/P9、service P6、store P1/P4/P5）。
- golangci-lint 无新增问题（4 个既有告警确认为基线 `1f4f7c17` 自带，未触碰）。
- 集成测试需真实 Redis，本环境无 docker/redis 跳过；**miniredis 已覆盖新增 Lua 路径**（P4 的 `PEXPIREAT`/`ZREVRANGE WITHSCORES`、P5 的 `EXISTS`+`HINCRBY`）。

### Codex 复验结论
- P1/P2/P3/P5/P6/P7/P9：确认已修复，未破坏 NB1/NB2/NM3/B2/B3 原有硬规则。
- P10/P11 暂不修的决定：认同。
- **新发现 N1 [High]**：P8 后端与 Vue 模板已改，但前端 API 类型 `ReqLogEntry` 漏加 `response_captured`，导致 `pnpm typecheck`/`vue-tsc -b` 失败，阻断前端构建。
- N2 [Low]：P4 回归测试因 `createSession` 仍调用 Go 侧 `refreshSessionsTTL`，无法单独证明 Lua 原子设置 TTL。
- N3 [Low]：新增测试文件未纳入版本控制，提交时需 `git add`。

### 针对复验新问题的处理（达成一致）
- **N1（阻塞）→ 已修复**：在 `frontend/src/api/admin/ops.ts` 的 `ReqLogEntry` 增加 `response_captured?: boolean`；`pnpm typecheck` 现退出码 0，Codex 报告的 `TS2339` 报错消失。P8 至此完整修复。
- **N2 → 已修复**：移除 `createSession` 末尾冗余的 Go 侧 `refreshSessionsTTL` 调用，使 sessions ZSet 的绝对 TTL **完全由 enable Lua 内 `PEXPIREAT` 设置**（P4 的本意）。这既消除了"Lua 成功但 Go 侧 TTL 未设"的崩溃窗口，也让 `TestReqLogStoreEnableSetsSessionsZSetTTL` 真正只验证 Lua 路径。`refreshSessionsTTL` 仍由 `ListSessions` 的惰性清理使用。
- **N3 → 提交时处理**：`reqlog_store_fixes_test.go`、`reqlog_sink_test.go` 等新增文件在提交修复时一并 `git add`。

### 复验后最终状态
- 后端：`go build ./...` + `go test -tags=unit ./...` 全绿。
- 前端：`pnpm typecheck` 退出码 0。
- P1~P9 全部修复完成；P10/P11 记为已知次要/后续优化；N1/N2 已闭环；N3 为提交注意事项。

