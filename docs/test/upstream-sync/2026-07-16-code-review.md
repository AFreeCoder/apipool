# v0.1.158 上游同步双重代码评审报告

## 基本信息

- 日期：2026-07-16
- 评审基线：`1c032621fa222f7acdbc181a5bb3c14ec634d3bc`
- 同步评审终点：`0250f7aa00ff1efd9fb92f28e6e8de353dcd6bea`
- 评审范围：`git diff 1c032621f..0250f7aa0`
- 评审方式：一个 subagent 使用 `requesting-code-review`，另一个 subagent 使用 gstack code review；两者均只读、相互独立
- 接受方式：主 agent 使用 `receiving-code-review` 逐条验证、补 RED 用例、实现修复并执行回归

## 结论

两路评审共同确认异步图片 URL 下载 SSRF 和审计清空竞态为发布阻塞项；另外确认旧 JWT step-up 授权串会话、审计 UTF-8 截断导致批量丢记录、异步任务进程重启后长期停留 `processing`。前四类问题已完成修复，异步任务已增加超时失败止损；完整持久任务队列作为独立遗留记录，在完成前生产必须保持该默认关闭功能未启用。

## 发现与处置

| 发现 | 评审来源 | 核验结论 | 处置 |
| --- | --- | --- | --- |
| 异步图片结果下载任意上游 URL，可读取私网/元数据并经 S3 回显 | requesting、gstack | 有效，Critical/P1 | 仅允许公网 HTTPS；初始 URL 和重定向都校验；socket 层拒绝私网、loopback、link-local、metadata 与 DNS rebinding；非真实图片字节直接失败 |
| 审计 `Count → TRUNCATE → Insert` 非原子，异步 batch 会在清空后写回旧记录 | requesting、gstack | 有效，Important/P1 | repository 改为单事务统计、清空、写留痕；service 增加写入锁与 generation cutover；失败时恢复代际，避免失败清空丢队列 |
| 无 `sid` 的旧 JWT 退化为用户级 step-up key，多个旧会话共享授权 | requesting | 有效，Important | 无会话 ID 时 fail closed，要求重新登录；service 增加防御检查，前端显示明确重新登录提示 |
| 审计请求体按字节截断可能产生非法 UTF-8；单条异常会让整批记录丢失 | requesting | 有效，Important | 截断结果改为有界、合法 UTF-8/JSON 包装；批量 COPY 失败时逐条降级写入，只丢真正失败的单条 |
| 异步任务只由进程内 goroutine 执行，重启后不能续跑 | gstack | 有效，但完整修复需要新架构 | 当前轮询发现超时 `processing` 会转为明确失败；功能保持默认关闭，生产启用列为部署阻断；持久队列与补偿机制转入 `issues.md` |

## RED 到 GREEN 证据

修复前新增定向测试稳定复现：

- 私网图片 URL 返回连接错误而不是策略拒绝；非图片文本被伪装为 `image/png` 并上传
- 无 `sid` 会话可以直接复用用户级 step-up grant
- 超长中文审计请求体产生非法 UTF-8/非法 JSON
- 审计 batch 写入失败后两条记录全部消失
- 超时异步任务轮询仍返回 `processing`

修复后：

- 私网初始 URL、云元数据重定向和非图片内容均被拒绝
- 无 `sid` 会话返回 `STEP_UP_SESSION_REQUIRED`
- 审计截断保持合法 UTF-8/JSON，batch 失败逐条降级成功
- 清空事务失败不会丢弃排队审计，成功清空不会回写旧 batch
- 超时异步任务轮询返回 `failed`

## 剩余边界

- 两个评审 subagent 均未修改、暂存、提交或推送任何文件。
- 完整持久异步任务队列不在本次上游同步修复内；当前部署只能在生产 `image_storage.enabled=false` 时继续。
- 生产为单实例 Compose，本次审计 generation 同步已覆盖当前运行模型；未来扩为多实例时需要数据库 advisory lock 或持久 epoch 协调清空边界。
