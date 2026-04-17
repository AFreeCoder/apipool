# Anthropic 平台新增 AWS Kiro 账号类型设计

## 背景

当前 APIPool 的 Anthropic 平台账号类型主要分为三类：

- `oauth` / `setup-token`：Claude OAuth 体系
- `apikey`：Anthropic API Key 体系
- `bedrock`：AWS Bedrock 体系

本次需求是在账号管理中为 Anthropic 平台新增 `AWS Kiro` 类型，并且首版直接覆盖完整链路：

- 账号录入
- 测试连接
- 实际请求转发
- access token 自动刷新

参考实现来自 `kiro.rs`。其核心特征不是 Bedrock SigV4，也不是普通 Anthropic API Key，而是基于：

- `refresh_token`
- `auth_method = social | idc`
- `auth_region`
- `api_region`
- `machine_id`
- 可选 `profile_arn`

因此不能简单复用现有 `bedrock` 或 `apikey` 语义。

## 目标

- 在 Anthropic 平台下新增独立账号类型 `kiro`
- 后台可以录入并保存 Kiro 凭证
- 后台可以对 Kiro 账号执行测试连接
- 网关可使用 Kiro 账号处理 Anthropic `/v1/messages` 主路径请求
- Kiro 账号可接入现有自动刷新体系，在 token 过期前后台刷新
- Kiro token 在热路径上也具备按需刷新能力，避免因后台刷新周期导致瞬时失效

## 非目标

- 不复刻 `kiro.rs` 的多凭据负载均衡与自动故障转移
- 不在首版支持一个 APIPool 账号挂载多组 Kiro refresh token
- 不承诺首版完全兼容所有 Kiro 特有协议能力
- 不在本次引入新的数据库列，优先复用现有 `credentials` / `extra` JSON 结构

## 备选方案

### 方案 1：新增独立账号类型 `kiro`

- `platform` 仍为 `anthropic`
- `type` 新增 `kiro`
- 前后端分别为 `kiro` 建立专用录入、测试、转发、刷新逻辑

优点：

- 语义清晰，和现有 `oauth` / `apikey` / `bedrock` 不混淆
- 后续新增 Kiro 专属 region、machine id、错误处理、请求头时不污染其他分支
- 可维护性最好

缺点：

- 需要改动账号类型枚举、表单、handler、service、测试

### 方案 2：复用 `bedrock` 类型，额外加 provider 区分

优点：

- 少一个类型字段值

缺点：

- Kiro 与 Bedrock 的认证、刷新、转发协议都不同
- 会在大量 Bedrock 代码里引入 `if provider == "kiro"` 分支
- 长期维护成本高

### 方案 3：复用 `apikey` 类型，作为自定义上游

优点：

- 表面改动较少

缺点：

- 无法自然承载 `refresh_token + social/idc` 自动刷新体系
- 与“测试连接 + 实际请求转发 + 自动刷新整条链路”目标冲突

## 结论

采用方案 1：在 Anthropic 平台下新增独立账号类型 `kiro`。

## 当前实现现状

### 前端

Anthropic 平台新增账号入口位于：

- `frontend/src/components/account/CreateAccountModal.vue`

当前 Anthropic 下已有三类卡片：

- Claude Code
- Claude Console
- AWS Bedrock

账号类型定义位于：

- `frontend/src/types/index.ts`

### 后端

账号类型常量位于：

- `backend/internal/domain/constants.go`
- `backend/internal/service/domain_constants.go`

管理端创建账号请求约束位于：

- `backend/internal/handler/admin/account_handler.go`

Anthropic 测试连接逻辑位于：

- `backend/internal/service/account_test_service.go`

Anthropic 网关选路与凭证获取逻辑位于：

- `backend/internal/service/gateway_service.go`

Claude OAuth 刷新与热路径 token 获取位于：

- `backend/internal/service/token_refresher.go`
- `backend/internal/service/token_refresh_service.go`
- `backend/internal/service/claude_token_provider.go`

现状结论：

- 现有 Anthropic 三类账号在“测试连接”和“网关转发”都已经是分类型处理
- 系统已经具备“后台刷新 + 热路径 token provider + OAuthRefreshAPI”的基础设施
- 因此新增 `kiro` 类型属于现有架构可承载的扩展，不需要另起一套账号系统

## 数据模型设计

### 账号类型

前后端统一新增：

- `AccountTypeKiro = "kiro"`

影响范围：

- 前端 `frontend/src/types/index.ts` 中的 `AccountType`
- 后端 `backend/internal/domain/constants.go` 新增 `AccountTypeKiro`
- 后端 `backend/internal/service/domain_constants.go` 同步 re-export `AccountTypeKiro`
- 后端 `backend/internal/handler/admin/account_handler.go` 中 `CreateAccountRequest.Type` 的 `oneof` binding tag 需添加 `kiro`
- 后端 `backend/internal/handler/admin/account_handler.go` 中 `UpdateAccountRequest.Type` 的 `oneof` binding tag 需添加 `kiro`
- 账号列表筛选选项

### credentials 字段

Kiro 首版凭证全部存储在 `accounts.credentials` 中，字段定义如下：

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `access_token` | 否 | 当前可用 access token，运行时优先使用 |
| `refresh_token` | 是 | Kiro 刷新 token |
| `expires_at` | 否 | access token 过期时间，使用现有 RFC3339/时间戳兼容解析 |
| `auth_method` | 是 | `social` 或 `idc` |
| `client_id` | `idc` 时必填 | IdC 登录需要 |
| `client_secret` | `idc` 时必填 | IdC 登录需要 |
| `profile_arn` | 否 | Kiro/AWS 返回的 profile ARN，刷新后可回填 |
| `region` | 否 | 兼容字段，仅作为 auth region 的回退值 |
| `auth_region` | 否 | 用于 refresh token |
| `api_region` | 否 | 用于实际上游 API 请求 |
| `machine_id` | 否 | 机器标识；不填时由后端派生或生成 |
| `base_url` | 否 | 预留扩展字段，首版默认不要求用户填写 |
| `model_mapping` | 否 | 复用现有模型白名单/映射存储结构 |

兼容规则：

- `auth_method` 只允许 `social` 或 `idc`
- 为与参考实现一致，后端读取时可兼容 `builder-id` / `iam`，但保存时统一标准化为 `idc`
- `auth_region` 的优先级为：`auth_region > region > us-east-1`
- `api_region` 的优先级为：`api_region > us-east-1`
- `machine_id` 缺失时按 Kiro 规则由后端生成，避免前端承担算法细节
- `machine_id` 如传入 UUID 格式，后端会标准化为 64 位十六进制字符串

### machine_id 规则

首版按 `kiro.rs` 的规则兼容处理：

- 允许用户传入 64 位十六进制 machine id
- 允许用户传入 UUID 格式 machine id，后端标准化时去掉连字符并重复一次，转成 64 位十六进制
- 如果创建时未传 `machine_id`，后端使用 `sha256("KotlinNativeAPI/" + refresh_token)` 生成稳定 machine id
- 生成后的 machine id 应在创建时直接回写进 `credentials.machine_id`，保证后续测试、刷新、转发使用同一值
- 编辑时允许修改，但需通过格式校验；未传则保留原值

### 通用能力复用

首版不新增 Kiro 专属数据库列，但需要明确不同能力的存储位置与范围：

- `credentials`
  - `model_mapping`
  - `pool_mode`
  - `pool_mode_retry_count`
  - `temp_unschedulable_enabled`
  - `temp_unschedulable_rules`
  - `intercept_warmup_requests`
- `extra`
  - `quota_limit`
  - `quota_daily_limit`
  - `quota_weekly_limit`
  - 固定时间重置相关字段
- 账号实体字段
  - `proxy_id`
  - `group_ids`
  - `expires_at`
  - `rate_multiplier`

首版不把 Kiro 纳入“自定义错误码”能力范围，原因是：

- 现有实现将该能力明确收敛到 `apikey`
- Kiro 的错误分类和同账号重试语义需要先基于真实链路验证
- 不在首版同时扩展该能力可以减少类型门控扩散

结论：

- 认证与上游连接信息全部进入 `credentials`
- 调度/计费/限制策略按现有存储模型复用
- Kiro 首版复用配额限制、池模式、临时不可调度、模型映射、代理、分组、过期自动暂停、计费倍率
- Kiro 首版不复用自定义错误码

## 前端设计

### 创建账号弹窗

Anthropic 平台新增第 4 张账号类型卡片：

- `AWS Kiro`

位置建议与 Bedrock 并列，避免与 Claude OAuth/API Key 混淆。

### 交互流程

Kiro 不走现有 OAuth 两步授权流程，而是直接在表单内录入凭证。

流程如下：

1. 选择平台 `anthropic`
2. 选择账号类别 `AWS Kiro`
3. 选择认证方式：
   - `Social`
   - `IdC / Builder-ID / IAM`
4. 填写凭证与区域信息
5. 可选配置模型限制、配额、池模式、临时不可调度、代理、分组等通用能力
6. 提交创建

### 表单字段

#### Social

- `refresh_token`
- `auth_region`
- `api_region`
- `machine_id`

#### IdC / Builder-ID / IAM

在 Social 基础上新增：

- `client_id`
- `client_secret`

### 模型限制

复用 Anthropic 现有模型限制区：

- 白名单模式
- 映射模式

默认推荐映射模式，并可为常用模型提供预设映射按钮。

### 列表与筛选

需要同步支持：

- 账号列表类型展示 `kiro`
- 类型筛选新增 `AWS Kiro`
- 测试连接弹窗展示类型信息
- 编辑账号弹窗支持回显与修改 Kiro 凭证

### TypeScript 联合类型影响面

新增 `'kiro'` 后，需要检查所有依赖 `AccountType` 的分支逻辑，重点包括：

- badge / 类型展示组件
- 列表筛选
- 创建与编辑弹窗
- 配额显示组件
- 批量编辑弹窗
- 动作菜单与测试弹窗

原则：

- 不依赖“漏掉分支时默认落到某个旧类型”
- 对需要穷举 `AccountType` 的组件做显式补全

## 后端设计

### 1. 创建账号

管理端创建账号接口接受：

- `platform = anthropic`
- `type = kiro`

后端创建阶段需要做最小校验：

- `refresh_token` 必填
- `auth_method` 必须为 `social` 或 `idc`
- `idc` 时 `client_id` / `client_secret` 必填
- `auth_region` / `api_region` 若填写则需非空字符串

### 2. Account 能力方法

在 `backend/internal/service/account.go` 中新增 Kiro 相关能力判断：

- `IsKiro()`
- `IsKiroSocial()`
- `IsKiroIDC()`
- `GetKiroAuthRegion()`
- `GetKiroAPIRegion()`
- `GetKiroMachineID()`

这些方法用于避免在 handler/service 中直接散落解析 JSON 字段。

### 3. 能力门控

现有代码中部分通用能力通过类型名硬编码门控，例如：

- `IsAPIKeyOrBedrock()`
- `IsPoolMode()`
- `isAccountSchedulableForQuota()`

如果直接新增 `kiro` 而不调整门控，Kiro 将无法正确复用首版设计中承诺的通用能力。

因此本次实现不应简单沿用 `IsAPIKeyOrBedrock()`，而应改成显式能力判断方法，例如：

- `SupportsQuotaLimit()`
  - `apikey`
  - `bedrock`
  - `kiro`
- `SupportsPoolMode()`
  - `apikey`
  - `bedrock`
  - `kiro`

对应影响点至少包括：

- 配额限制与配额调度判断
- 池模式开关与同账号重试
- 账号容量/配额 UI 展示

Kiro 首版不纳入 `SupportsCustomErrorCodes()`，避免额外扩大能力面。

### 4. 测试连接

在 `backend/internal/service/account_test_service.go` 中新增 Kiro 专用分支：

- Anthropic 测试入口识别 `type == kiro`
- 不复用普通 Claude OAuth 测试逻辑

测试步骤：

1. 读取或刷新 Kiro `access_token`
2. 解析 `api_region`、`profile_arn`、`machine_id`
3. 生成最小 Kiro 请求
4. 调用 Kiro 上游接口
5. 将响应转换为现有 SSE 测试事件

目标不是验证 Anthropic API 是否可达，而是验证：

- 刷新链路正确
- 凭证正确
- Kiro 上游接口可访问

建议测试连接优先调用 Kiro 的轻量能力接口，而不是直接走完整对话流。参考 `kiro.rs`，首选验证接口可使用：

- `GET https://q.{api_region}.amazonaws.com/getUsageLimits?origin=AI_EDITOR&resourceType=AGENTIC_REQUEST`

请求头至少包括：

- `Authorization: Bearer <access_token>`
- `x-amz-user-agent`
- `user-agent`
- `host`
- `amz-sdk-invocation-id`
- `amz-sdk-request`

如存在 `profile_arn`，需要一并带入请求参数。

### 5. 默认分组策略

首版 Kiro 账号沿用现有平台默认分组策略：

- 未显式指定分组时，自动绑定 `anthropic-default`

理由：

- Kiro 在平台层面仍属于 `anthropic`
- 现有默认分组逻辑按 `platform` 工作，无需首版额外引入 `anthropic-kiro-default`
- 如果运营上需要隔离，可通过手动分组完成

后续若 Kiro 账号规模增长，且与普通 Claude Anthropic 账号在调度或配额策略上需要长期隔离，再评估独立默认分组。

### 6. 网关实际请求转发

在 `backend/internal/service/gateway_service.go` 中新增 Kiro 分支。

#### 账号选择

Kiro 账号仍归属 `platform = anthropic`，因此：

- 仍参加 Anthropic 分组与调度
- 模型支持判断要考虑 Kiro 映射

#### 获取凭证

`GetAccessToken` 需要新增：

- `AccountTypeKiro` -> 通过 `KiroTokenProvider` 获取 token，返回类型标识 `"kiro"`

#### 实际转发

新增 `forwardKiro(...)`，职责如下：

1. 获取可用 token
2. 将 Anthropic `/v1/messages` 请求转换为 Kiro 请求
3. 使用 `api_region`、`profile_arn`、`machine_id` 调用上游
4. 将 Kiro 响应转换回 Anthropic 兼容响应

#### Kiro 上游 URL

首版主请求使用：

- `POST https://q.{api_region}.amazonaws.com/generateAssistantResponse`

后续如果实现 Kiro MCP/WebSearch 能力，可扩展：

- `POST https://q.{api_region}.amazonaws.com/mcp`

#### Kiro 主请求头

首版 `/v1/messages` 主路径至少需要以下请求头：

- `Content-Type: application/json`
- `Authorization: Bearer <access_token>`
- `x-amzn-codewhisperer-optout: true`
- `x-amzn-kiro-agent-mode: vibe`
- `x-amz-user-agent`
- `user-agent`
- `host`
- `amz-sdk-invocation-id`
- `amz-sdk-request`
- `Connection: close`

其中：

- `x-amz-user-agent` 和 `user-agent` 需要包含 `kiro_version` 与 `machine_id`
- `host` 需要与 `q.{api_region}.amazonaws.com` 一致
- `profile_arn` 在主请求路径中应写入请求体 `profileArn` 字段，而不是 header

#### 请求体转换

这里不是“Anthropic Messages 原样透传，仅补几个 header”，而是必须有独立转换层。

Anthropic `/v1/messages` 请求需要转换为 Kiro 专用结构：

- 根对象：`KiroRequest`
- 核心字段：`conversationState`
- 可选字段：`profileArn`

转换层至少负责：

- 模型名转换
- `messages` 转换为 `conversationState.currentMessage + history`
- 从 `metadata.user_id` 中提取会话 ID，作为 `conversationId` 的优先来源
- 为当前消息设置 `agentTaskType = "vibe"`
- 为当前消息填充 `origin = "AI_EDITOR"`
- 必要时将工具、图片、历史上下文适配为 Kiro 结构

#### 响应转换

Kiro 流式响应也不是标准 Anthropic SSE 原样返回，因此需要独立响应适配层：

- 非流式响应：从 Kiro 返回结构提取正文，重组为 Anthropic 兼容响应
- 流式响应：将 Kiro event stream 转换成 Anthropic SSE 事件序列

首版必须明确：

- 存在独立的 `Kiro -> Anthropic` 响应适配器
- 不是通过“替换 URL + 透传 body”完成兼容

### 7. 自动刷新

新增一条完整的 Kiro 刷新链路：

- `KiroOAuthService` 或 `KiroAuthService`
- `KiroTokenRefresher`
- `KiroTokenProvider`

#### 刷新协议

`social` 模式：

- 请求 `https://prod.{auth_region}.auth.desktop.kiro.dev/refreshToken`

`idc` 模式：

- 请求 `https://oidc.{auth_region}.amazonaws.com/token`

#### social 刷新细节

- 方法：`POST`
- Content-Type：`application/json`
- 请求体：

```json
{
  "refreshToken": "<refresh_token>"
}
```

- 关键请求头：
  - `Accept: application/json, text/plain, */*`
  - `Content-Type: application/json`
  - `User-Agent: KiroIDE-<kiro_version>-<machine_id>`
  - `Host: prod.{auth_region}.auth.desktop.kiro.dev`

- 关键响应字段：
  - `accessToken`
  - `refreshToken`
  - `expiresIn`
  - `profileArn`

#### idc 刷新细节

- 方法：`POST`
- Content-Type：`application/json`
- 请求体：

```json
{
  "clientId": "<client_id>",
  "clientSecret": "<client_secret>",
  "refreshToken": "<refresh_token>",
  "grantType": "refresh_token"
}
```

- 关键请求头：
  - `content-type: application/json`
  - `x-amz-user-agent`
  - `user-agent`
  - `host: oidc.{auth_region}.amazonaws.com`
  - `amz-sdk-invocation-id`
  - `amz-sdk-request`

- 关键响应字段：
  - `accessToken`
  - `refreshToken`
  - `expiresIn`
  - `profileArn`

#### 刷新结果回写

刷新成功后更新：

- `access_token`
- `refresh_token`（若返回新值）
- `expires_at`
- `profile_arn`（若返回）

#### 集成方式

Kiro 刷新器接入现有：

- `TokenRefreshService`
- `OAuthRefreshAPI`

Kiro provider 参照现有 Claude token provider：

- 先查缓存
- 即将过期时尝试刷新
- 刷新锁被占用时短暂等待缓存
- 失败时按短 TTL 写缓存，避免热路径放大

实现上建议拆成两个阶段：

- `3a`：实现 `KiroAuthService`
  - 只负责 HTTP 刷新协议与字段映射
  - 用 mock HTTP 覆盖 `social` / `idc`
- `3b`：实现 `KiroTokenRefresher` + `KiroTokenProvider`
  - 接入 `TokenRefreshService`
  - 接入 `OAuthRefreshAPI`
  - 接入网关热路径

这样可以先验证协议正确性，再验证与现有刷新基础设施的集成。

### 8. 错误处理

需要单独区分以下错误：

- `invalid_grant`
  视为 refresh token 永久失效，应将账号标记错误或不可调度
- `401/403`
  可能是 access token 失效、profile 权限异常或凭证错误
- `429`
  视为限流，可走临时不可调度
- `5xx` / 网络错误
  视为临时故障，进入重试或短期不可调度

原则：

- 永久失效与临时故障不能混淆
- 不能静默吞掉刷新失败

## 兼容边界

首版兼容目标是“主路径可用”，不是“完整复刻 kiro.rs 全量能力”。

### 首版必须支持

- Anthropic `/v1/messages`
- 流式响应
- 非流式响应
- 文本消息
- 模型映射
- 账号测试连接
- 后台自动刷新
- 热路径 token 刷新

### 首版保守处理

- 复杂工具调用
- 图片/多模态
- Kiro 特有事件的全量映射
- 多凭据负载均衡
- 凭据自动文件回写

实现原则：

- 可以安全透传的能力继续透传
- 不稳定的能力明确返回不支持，避免假兼容

## 测试策略

### 单元测试

- `kiro` 账号类型判定
- `social` / `idc` 凭证校验
- 区域优先级解析
- machine id 生成与读取
- Kiro 刷新请求构造
- `NeedsRefresh` / `CanRefresh` 判断
- Kiro 模型映射

### 服务层测试

- `AccountTestService` Kiro 分支成功/失败路径
- `GatewayService` 将 `kiro` 账号路由到 `forwardKiro`
- Kiro 请求体转换正确生成 `KiroRequest`
- Kiro SSE / 非流式响应成功转换回 Anthropic 格式
- token 过期时 provider 热路径刷新
- `invalid_grant` 时错误态处理
- 模型不支持时拒绝路径

### 前端测试

- CreateAccountModal 出现 `AWS Kiro`
- `social` / `idc` 字段切换
- 表单必填校验
- 创建请求序列化为 `type = kiro`
- 类型筛选与展示文案

### 集成验证

- 创建一条 `social` Kiro 账号并测试连接成功
- 创建一条 `idc` Kiro 账号并测试连接成功
- 使用真实 Anthropic `/v1/messages` 请求打通流式返回
- 验证 token 过期后自动刷新
- 验证 refresh token 失效时可观测错误

## 风险与缓解

### 风险 1：Kiro 上游协议与标准 Anthropic 不完全一致

缓解：

- 转换层独立实现，不污染普通 Anthropic 分支
- 首版明确只承诺主路径

### 风险 2：区域、profile、machine id 组合错误导致不稳定

缓解：

- 将字段解析收敛到 `Account` 方法或 Kiro helper
- 关键失败点输出结构化日志

### 风险 3：将 Kiro 混入 Claude OAuth 逻辑导致后续维护困难

缓解：

- 类型独立
- provider/refresher 独立
- 测试连接与转发独立

### 风险 4：刷新失败处理不当导致无意义重试

缓解：

- 明确区分永久失效与临时失败
- 与现有临时不可调度策略联动

### 风险 5：Kiro 上游端点缺乏公开稳定性保证

Kiro 使用的上游端点来自社区逆向与兼容实现，虽然当前可用，但存在以下风险：

- endpoint 结构可能变化
- header 要求可能变化
- token 生命周期与刷新行为可能变化
- 无公开 SLA 保证

缓解：

- 将请求 URL、刷新 URL、header 构造逻辑集中封装，不把格式散落在多个 service 中
- 首版保留 `base_url` 作为推理请求覆盖的扩展点
- 刷新 URL 先不向前台暴露，但实现层需保留集中配置入口，避免未来改动时大面积重构

## 验收标准

- 管理后台可新增、编辑、查看 `Anthropic / Kiro` 账号
- 后台可对 `Kiro` 账号执行测试连接并得到明确结果
- 网关可用 `Kiro` 账号完成至少一条真实 Anthropic `/v1/messages` 请求
- `social` 与 `idc` 两种账号都可自动刷新 token
- refresh token 永久失效时，系统能给出可观测错误并避免无意义重复刷新

## 后续实现建议

实现阶段建议按以下顺序推进：

1. 扩展前后端账号类型与表单
2. 完成 Kiro credentials 校验与保存
3. 实现 `KiroAuthService` 并补齐刷新协议单元测试
4. 实现 `KiroTokenRefresher` / `KiroTokenProvider` 并接入刷新体系
5. 接入 Kiro 测试连接
6. 接入 Kiro 网关转发与响应转换
7. 补齐单元测试、服务测试与前端测试

以上顺序能保证每一步都具备明确的验证点，并且降低请求转发阶段的联调风险。

---

## 二次评审意见（2026-04-11）

首轮评审的 10 项意见已全部合入正文。以下为二次评审发现的遗留问题：

### 1. IdC 刷新的 Content-Type 需确认

第 7 节 idc 刷新细节指定 `Content-Type: application/json`，请求体也是 JSON。但 AWS OIDC token endpoint（`oidc.{region}.amazonaws.com/token`）标准协议通常使用 `application/x-www-form-urlencoded`。kiro.rs 的实际实现中可能做了适配，但这个点在实现时需要用真实 endpoint 验证。

**建议**：在 idc 刷新细节中标注"以 kiro.rs 源码为准；如果上游返回 415 Unsupported Media Type 则切换为 `application/x-www-form-urlencoded`"。

### 2. 响应转换缺具体 event 映射

第 6 节响应转换部分正确指出"Kiro 流式响应不是标准 Anthropic SSE"，需要独立适配层。但没有列出 Kiro 的 event 类型（如 kiro.rs 中的 `assistantResponseEvent`、`codeEvent` 等），以及如何映射到 Anthropic 的 `message_start` / `content_block_delta` / `message_stop` 事件。

**建议**：在响应转换小节补充一句"具体 Kiro event 到 Anthropic SSE 的映射表在实现阶段参照 kiro.rs 的 `converter.rs` 补充"，明确这部分设计将在实现阶段细化而非遗漏。

### 3. 二次评审结论

设计方案已具备进入实现的条件。上述两项为实现阶段注意事项，不阻塞开工。

---

## 实现评审（2026-04-11）

Codex 按 spec 分 9 个 commit 完成开发（`feat/anthropic-kiro-account` 分支）。本次针对代码实现的评审。

### 一、总体评价

代码结构与现有模式高度一致：

- 类型常量、`oneof` binding tag、前端 `AccountType` 联合类型、`domain_constants.go` re-export 全部同步修改
- 能力门控按 spec 要求抽成 `SupportsQuotaLimit()` / `SupportsPoolMode()` / `SupportsCustomErrorCodes()` 语义方法，hot path 调用点全部切换（`account.go:843`、`mappers.go:277`、`gateway_service.go:2252/7341`）
- 刷新、provider、converter、stream adapter、测试连接、网关转发全部按 spec 的 URL / header / 字段要求实现，DI 完整接入 `OAuthRefreshAPI` + `TokenRefreshService`
- 单元测试覆盖 credentials、machine_id 规范化、刷新协议、token provider、stream adapter、converter、quota gating，共约 1200 行新增测试

### 二、验证结果

| 项目 | 结果 |
|------|------|
| `go build ./...` | 通过 |
| `go test -tags=unit ./...` | 通过 |
| `go test -tags=integration ./...` | 通过 |
| `golangci-lint run --new-from-rev=main ./...` | 0 issues |
| `pnpm test:run`（前端） | 308/308 通过 |

### 三、Must Fix（会造成运行时 bug）

#### 1. `usage_billing_repo.go:135` 漏改导致 Kiro 账号配额写入被静默吞掉

`backend/internal/repository/usage_billing_repo.go:135` 仍然硬编码：

```go
if cmd.AccountQuotaCost > 0 && (strings.EqualFold(cmd.AccountType, service.AccountTypeAPIKey) || strings.EqualFold(cmd.AccountType, service.AccountTypeBedrock)) {
    if err := incrementUsageBillingAccountQuota(ctx, tx, cmd.AccountID, cmd.AccountQuotaCost); err != nil {
```

这与 `gateway_service.go:7341` 的 `shouldUpdateAccountQuota()` 新语义不一致。完整链路：

1. 网关计算完本次用量，`shouldUpdateAccountQuota()` 因 `SupportsQuotaLimit()` 返回 true，把 `cmd.AccountQuotaCost` 填好，`AccountType` 设为 `"kiro"`
2. 进入 `repo.Apply → applyUsageBillingEffects`
3. 这里的硬编码 type 检查把 Kiro 排除，**数据库里 Kiro 账号的 `quota_used` 永远不会递增**

后果：Kiro 账号的配额限制完全失效，但前端还是会显示可配置配额，调度侧 `isAccountSchedulableForQuota` 也确实在读这个值，一旦运营团队依赖 Kiro 账号配额控制就会翻车。

`gateway_service.go:7387` 的 `postUsageBilling` fallback 路径在 Kiro 能正确写入，但生产走的是 `applyUsageBilling → repo.Apply`，不会走 fallback。

**修复**：

```go
if cmd.AccountQuotaCost > 0 &&
    (strings.EqualFold(cmd.AccountType, service.AccountTypeAPIKey) ||
     strings.EqualFold(cmd.AccountType, service.AccountTypeBedrock) ||
     strings.EqualFold(cmd.AccountType, service.AccountTypeKiro)) {
```

或者更根本的修复：把这个 type 判断抽成 `service.AccountTypeSupportsQuota(cmd.AccountType)` 辅助函数，和 Account 方法的 `SupportsQuotaLimit()` 共用一份枚举。

同时建议在 `usage_billing_repo_integration_test.go` 加一个 Kiro 用例，防止回归。

### 四、Should Fix（spec 要求未完全满足）

#### 2. 刷新错误分类不完整

spec 第 8 节"错误处理"要求区分 `invalid_grant` / `401/403` / `429` / `5xx`，但 `kiro_auth_service.go:153-158` 只在 `400 + invalid_grant` 子字符串命中时做特殊处理，其余错误一律 `"kiro refresh failed: ..."` 折叠：

```go
if resp.StatusCode >= http.StatusBadRequest {
    if resp.StatusCode == http.StatusBadRequest && strings.Contains(strings.ToLower(bodyText), "invalid_grant") {
        return nil, fmt.Errorf("invalid_grant: %s", bodyText)
    }
    return nil, fmt.Errorf("kiro refresh failed: %s", bodyText)
}
```

配合 `refresh_policy.go:48` 的 `KiroProviderRefreshPolicy` 全部走 `ProviderRefreshErrorUseExistingToken`，结果是 429 / 5xx / 403 都会被当成"临时失败继续用旧 token"。这在 refresh token 已被 IdC 撤销 (403) 或 Kiro 主站 5xx 持续宕机时没有大问题，但不能联动临时不可调度策略——也就是 spec 明确要求的"429 视为限流，可走临时不可调度"。

**建议**：

- 在 `parseKiroRefreshResponse` 返回 typed error（`ErrKiroInvalidGrant` / `ErrKiroRateLimited` / `ErrKiroUpstream`），或者至少把 status code 放到错误字符串里
- `kiro_token_refresher.go` 或 `kiro_token_provider.go` 对 429 响应调用现有的 temp unschedulable 机制

#### 3. IdC 刷新 Content-Type 的二次评审建议未落地

二次评审第 1 条写了："以 kiro.rs 源码为准，如果上游返回 415 则切换为 `application/x-www-form-urlencoded`"。

`kiro_auth_service.go:119` 硬编码 `application/json`：

```go
req.Header.Set("content-type", "application/json")
```

没有 415 自动降级，也没有注释标注这是已知兼容风险。如果未来 AWS OIDC endpoint 要求回归到标准的 form-urlencoded，IdC 模式会整条挂掉。

**建议**：至少加一行注释 `// NOTE: 若上游返回 415 Unsupported Media Type，需切换为 application/x-www-form-urlencoded（参考 kiro.rs auth.rs）`，便于运维定位。如果愿意做得更稳，可以在 401/415 时重试一次 form 格式。

#### 4. Kiro 模型支持检查调用了 `claude.NormalizeModelID`

`gateway_service.go:3437`：

```go
if account.Platform == PlatformAnthropic && account.Type != AccountTypeAPIKey {
    requestedModel = claude.NormalizeModelID(requestedModel)
}
return account.IsModelSupported(requestedModel)
```

这个分支现在也会命中 Kiro 账号。虽然 `IsModelSupported` 内部本身已经会做一次 normalize（`account.go:567`），所以重复 normalize 不会破坏功能，但意图不清晰。参考 Bedrock 的做法——它有自己的 `IsBedrock()` 早退分支。

**建议**：加一段 `if account.IsKiro() { return account.IsModelSupported(requestedModel) }`，或者直接让 `Platform==Anthropic && Type==Kiro` 跳过 `claude.NormalizeModelID`。这不是 bug，但容易在未来增加短 ID → Kiro 长 ID 映射时埋坑。

### 五、Nice to Have（可延后）

#### 5. 测试连接直接 `sendErrorAndEnd(err.Error())` 泄露内部信息

`account_test_service_kiro.go:17-19` 等位置把底层错误直接回显到 SSE：

```go
if err != nil {
    return s.sendErrorAndEnd(c, err.Error())
}
```

包含完整的 `url.Parse` 错误栈或 HTTP 错误细节。参照 `testClaudeAccountConnection` 的常见处理，建议对外提示脱敏，内部用 slog 记录完整错误。

#### 6. 错误状态码 `kiro upstream error: %d` 缺少上下文

`gateway_service_kiro.go:84` 直接把 body 拼进错误，没有上下文分类，不便于后续错误分类器匹配。建议和第 2 点一起做成 typed error。

#### 7. Converter 文件过大

`kiro_converter.go` 单文件 804 行，包含结构体定义、请求体构建、工具名处理、system 提取、图片转换、tool pairing 修复、历史 placeholder、stream utils 等多层职责。建议后续拆成 `kiro_types.go` / `kiro_request_builder.go` / `kiro_tool_utils.go`，便于后续维护。**非本次必须**。

#### 8. `kiro_converter_test.go` 只有 87 行，覆盖不足

相对 804 行实现，测试明显偏薄。工具名 SHA256 截断、thinking prefix、image base64 decode、tool pairing validation 这些分支都没有显式用例。若日后 kiro.rs 更新协议，回归风险较高。

#### 9. Spec 文档 vs 实现

spec 第 4 节"兼容边界"里"首版保守处理 / 图片多模态 / 复杂工具调用"被实现覆盖了一遍——这个超出 spec 是加分，但需要同步更新 spec 或至少在 commit message 里标注，让运营团队知道首版实际支持这些能力。

### 六、评审结论

这版实现可以进入下一阶段（真实 Kiro 账号联调），但有一个必须在合并前修掉的 bug：

- **Must Fix**：`usage_billing_repo.go:135` 的硬编码 type 检查漏掉了 Kiro，会导致 Kiro 账号的配额扣减在生产链路上被静默吞掉。建议一并补一条集成测试。

- **Should Fix（强烈建议合并前修）**：
  - 刷新错误分类没有 typed error，429 / 5xx / 403 无法联动临时不可调度
  - IdC 刷新 Content-Type 的兼容风险未在代码中留痕

- **Nice to Have**：测试连接错误脱敏、`claude.NormalizeModelID` 分支收敛、converter 拆分、converter 测试补强。

整体是一次高质量的实现，修完上面第 1 项之后可以开始真实 Kiro 账号的端到端联调。
