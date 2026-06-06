# Ops Error Code Whitelist Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为运维监控新增按响应错误码精确白名单屏蔽错误日志的能力，并在高级设置中暴露该配置入口。

**Architecture:** 在后端 `OpsAdvancedSettings` 中新增 `ignored_error_codes` 数组，通过 `OpsErrorLoggerMiddleware` 写入前过滤；前端在运维高级设置中新增错误码白名单配置 UI，默认值为空数组并兼容旧配置。

**Tech Stack:** Go, Gin, Vue 3, TypeScript, Vitest-compatible frontend types, Go tests

---

### Task 1: 扩展后端高级设置模型

**Files:**
- Modify: `backend/internal/service/ops_settings_models.go`
- Modify: `backend/internal/service/ops_settings.go`
- Test: `backend/internal/service/ops_settings_advanced_test.go`

- [ ] **Step 1: 写失败测试，断言默认值和持久化包含 `ignored_error_codes`**
- [ ] **Step 2: 运行相关 Go 测试，确认因字段缺失而失败**
- [ ] **Step 3: 在高级设置模型、默认值、归一化逻辑中新增 `ignored_error_codes`**
- [ ] **Step 4: 再跑 Go 测试，确认通过**

### Task 2: 在错误日志过滤链路接入错误码白名单

**Files:**
- Modify: `backend/internal/handler/ops_error_logger.go`
- Test: `backend/internal/handler/ops_error_logger_test.go`

- [ ] **Step 1: 写失败测试，断言命中白名单错误码时跳过写入**
- [ ] **Step 2: 运行相关 Go 测试，确认失败原因正确**
- [ ] **Step 3: 调整过滤函数签名和调用链，按精确 `code` 匹配 `ignored_error_codes`**
- [ ] **Step 4: 再跑 handler 相关测试，确认通过**

### Task 3: 暴露前端配置字段与设置 UI

**Files:**
- Modify: `frontend/src/api/admin/ops.ts`
- Modify: `frontend/src/views/admin/ops/components/OpsSettingsDialog.vue`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`

- [ ] **Step 1: 更新前端类型，增加 `ignored_error_codes`**
- [ ] **Step 2: 在运维设置对话框中新增白名单配置 UI**
- [ ] **Step 3: 补齐中英文文案**
- [ ] **Step 4: 运行前端类型检查或构建验证**

### Task 4: 回归验证

**Files:**
- Modify: `docs/superpowers/specs/2026-04-18-ops-error-code-whitelist-design.md`
- Modify: `docs/superpowers/plans/2026-04-18-ops-error-code-whitelist.md`

- [ ] **Step 1: 运行本次涉及的 Go 测试集合**
- [ ] **Step 2: 运行前端构建或类型检查**
- [ ] **Step 3: 自查默认值为空数组、旧布尔开关兼容、UI 可提交空白名单**
- [ ] **Step 4: 总结改动与验证结果**
