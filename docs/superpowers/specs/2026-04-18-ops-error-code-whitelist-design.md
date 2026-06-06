# 运维错误日志错误码白名单设计

**背景**

线上近 48 小时错误日志中，`API_KEY_QUOTA_EXHAUSTED`、`INVALID_API_KEY`、`API_KEY_DISABLED` 等用户侧或业务侧限制类错误占比高，但当前日志分类会把它们混入 `api_error`，导致无法按精确错误码做屏蔽，也会干扰错误率判断和运维排障。

**目标**

为运维监控增加“按响应错误码精确白名单屏蔽错误日志”的能力，避免通过粗粒度 `error_type` 过滤造成误伤；默认不启用任何新屏蔽值，只先上线能力和设置入口。

## 方案

### 1. 数据模型

在 `OpsAdvancedSettings` 中新增：

- `ignored_error_codes: []string`

该字段保存需要在写入 `ops_error_logs` 前被忽略的精确错误码列表，例如：

- `API_KEY_QUOTA_EXHAUSTED`
- `API_KEY_EXPIRED`
- `INVALID_API_KEY`
- `API_KEY_DISABLED`
- `API_KEY_REQUIRED`
- `USAGE_LIMIT_EXCEEDED`

### 2. 过滤口径

`OpsErrorLoggerMiddleware` 在解析响应体时已经能抽取：

- `error.type`
- `error.message`
- 顶层 `code`

新增白名单过滤时采用以下规则：

1. 先解析响应中的精确 `code`
2. 若 `code` 命中 `ignored_error_codes`，则跳过写入 `ops_error_logs`
3. 未命中时，继续沿用现有布尔过滤项：
   - `ignore_count_tokens_errors`
   - `ignore_context_canceled`
   - `ignore_no_available_accounts`
   - `ignore_invalid_api_key_errors`
   - `ignore_insufficient_balance_errors`

只做精确错误码匹配，不按 `error_type` 或 message 模糊匹配。

### 3. 前端设置

在运维监控高级设置的“错误过滤”区域新增错误码白名单配置：

- 预置常见错误码复选项
- 允许补充自定义错误码

本次先上线功能和设置 UI，默认值为空数组，不主动屏蔽任何新错误。

### 4. 兼容性

- 历史配置缺少 `ignored_error_codes` 时，读取默认值为空数组
- 旧布尔设置继续保留，避免破坏已有行为
- 该能力只影响后续错误日志写入，不修改历史数据

### 5. 测试

需要覆盖：

- 默认配置回填 `ignored_error_codes`
- 更新高级设置时持久化该字段
- `shouldSkipOpsErrorLog` 在命中白名单错误码时返回 `true`
- 未命中时保持现有布尔过滤逻辑不变

## 范围外

- 不在本次同时重构 `error_type` 分类体系
- 不对历史 `ops_error_logs` 做回填或清理
- 不调整当前 Dashboard 错误率计算公式
