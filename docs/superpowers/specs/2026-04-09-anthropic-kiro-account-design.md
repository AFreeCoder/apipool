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

- 前端 `AccountType`
- 后端 `CreateAccountRequest` / `UpdateAccountRequest` 的 `oneof` 校验
- 后端 service 常量
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
| `auth_region` | 否 | 用于 refresh token |
| `api_region` | 否 | 用于实际上游 API 请求 |
| `machine_id` | 否 | 机器标识；不填时由后端派生或生成 |
| `base_url` | 否 | 预留扩展字段，首版默认不要求用户填写 |
| `model_mapping` | 否 | 复用现有模型白名单/映射存储结构 |

兼容规则：

- `auth_method` 只允许 `social` 或 `idc`
- 为与参考实现一致，后端读取时可兼容 `builder-id` / `iam`，但保存时统一标准化为 `idc`
- `auth_region` 缺失时可回退到 `api_region`，再回退到默认区域
- `machine_id` 缺失时按 Kiro 规则由后端生成，避免前端承担算法细节

### extra 字段

首版不新增 Kiro 专属 `extra` 字段，直接复用现有通用能力：

- 配额限制
- 池模式
- 自定义错误码
- 临时不可调度规则
- 代理
- 分组
- 过期自动暂停
- 计费倍率

结论：

- 认证与上游连接信息全部进入 `credentials`
- 调度/计费/限制策略继续沿用 `extra`

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
5. 可选配置模型限制、配额、池模式、代理、分组等通用能力
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

### 3. 测试连接

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

### 4. 网关实际请求转发

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

### 5. 自动刷新

新增一条完整的 Kiro 刷新链路：

- `KiroOAuthService` 或 `KiroAuthService`
- `KiroTokenRefresher`
- `KiroTokenProvider`

#### 刷新协议

`social` 模式：

- 请求 `https://prod.{auth_region}.auth.desktop.kiro.dev/refreshToken`

`idc` 模式：

- 请求 `https://oidc.{auth_region}.amazonaws.com/token`

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

### 6. 错误处理

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
3. 实现 Kiro 刷新服务与 token provider
4. 接入 Kiro 测试连接
5. 接入 Kiro 网关转发
6. 补齐单元测试、服务测试与前端测试

以上顺序能保证每一步都具备明确的验证点，并且降低请求转发阶段的联调风险。
