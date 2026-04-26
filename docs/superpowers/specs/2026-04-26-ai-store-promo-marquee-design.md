# AppHeader 顶部跑马灯字幕（AI 会员店推广位）设计

- **创建日期**：2026-04-26
- **状态**：设计已确认，待编排实施计划
- **作者**：AFreeCoder（与 Claude 协作）
- **关联背景**：在 APIPool 中转站内嵌一个长期可用的"轻量营销位"，用于推广 AFreeCoder 自营的 AI 会员店（ChatGPT Plus / Grok / Gemini Pro 会员代充等）。

---

## 1. 背景与目标

### 1.1 业务背景

APIPool 目前是 API 中转站，已有付费用户群体。AFreeCoder 同时运营独立的 AI 会员店，希望把 APIPool 内的现有用户转化为会员店的客户，且未来能持续投放不同的促销活动文案，无需每次都改代码。

### 1.2 范围澄清

经核查，本次需求中**"自定义菜单页"功能项目里已经存在**，无需开发：

- 路由 `/custom/:id` → [CustomPageView.vue](../../../frontend/src/views/user/CustomPageView.vue)
- 系统设置中的 `custom_menu_items` 字段（支持「全员可见」与「仅管理员」两类，自动出现在 Sidebar）
- iframe 嵌入外链时自动透传 `userId / token / theme / locale`

接入 AI 会员店为菜单项是**纯配置工作**：登录管理后台 → 系统设置 → 自定义菜单 → 新增一条指向会员店 URL。本设计文档**不包含**这部分。

### 1.3 本设计的范围

设计并实现一个**顶部 AppHeader 中间区域的跑马灯字幕条**，作为常驻营销位：

- 仅在登录后的内部页面（`AppLayout` 包裹的页面）显示
- 纯文本展示，不可点击，文案中引导用户点击 Sidebar 中的"AI 会员店"菜单项
- Admin 可在系统设置中配置：全局开关 + 多条字幕 + 单条启停 + 排序

### 1.4 业务约束

- 字幕条**不要**占用主内容区竖向空间——挂在 AppHeader 中间天然空白处
- 文案可能较长，不能截断（→ 跑马灯横向滚动）
- 仅桌面端显示，移动端不渲染（AppHeader 在 `<lg` 时已经很挤）
- 不需要多语言（同一段文案对中英用户都展示）
- 不需要"用户级关闭/记忆"、不需要起止时间、不需要点击跳转

---

## 2. 高层架构

```text
┌─────────────────────────────────────────────────────────────────┐
│                        Admin 系统设置页                           │
│  开关 + 字幕列表 (新增/编辑/启停/排序/删除)                          │
└──────────┬───────────────────────────────────┬──────────────────┘
           │ PUT /api/admin/settings           │
           ▼                                   │
┌─────────────────────────────────────┐        │
│  system_settings 表                  │        │
│  + marquee_enabled (bool, JSON)     │        │
│  + marquee_messages (JSON array)    │        │
└──────────────┬──────────────────────┘        │
               │ GET /api/settings/public      │ GET /api/admin/settings
               ▼                                ▼
┌─────────────────────────────────────────────────────────────────┐
│             appStore.cachedPublicSettings (Pinia)                │
│             adminSettingsStore.* (Pinia, admin only)             │
└──────────────┬──────────────────────────────────────────────────┘
               │ computed: marqueeText = enabled msgs joined by " · "
               ▼
┌─────────────────────────────────────────────────────────────────┐
│  AppHeader.vue (中间槽位)                                        │
│   └─ <AppHeaderMarquee v-if="lg && enabled && marqueeText" />   │
│         CSS keyframe 横向匀速滚动 + hover 暂停 + 两端淡出渐变       │
└─────────────────────────────────────────────────────────────────┘
```

**架构关键决策**：

| 决策 | 选择 | 理由 |
|---|---|---|
| 数据存储 | 复用 `system_settings` 表的 JSON 字段 | 跟现有 `custom_menu_items` 同款范式；无新表、无 schema 改动；字幕数量小（≤20） |
| 数据下发 | 复用 `/api/settings/public` | 前端 `appStore.cachedPublicSettings` 已加载，零额外请求 |
| 动画实现 | 纯 CSS keyframe `marquee` | 无 JS 定时器；性能零成本；天然支持 `prefers-reduced-motion` |
| 多条拼接 | 后端按 `sort_order` 升序、前端 `join(' · ')` 渲染为单条循环 | 视觉简单连续，比"按条切换"更适合长文案 |
| 公告系统复用 | **不复用** | 公告有"已读/用户绑定/标题正文"等概念，跟营销字幕语义不同；强行复用会污染 |

---

## 3. 数据模型

### 3.1 新增 setting key

`backend/internal/service/domain_constants.go`：

```go
SettingMarqueeEnabled  = "marquee_enabled"   // bool, 全局开关
SettingMarqueeMessages = "marquee_messages"  // JSON array of MarqueeMessage
```

### 3.2 DTO 结构

`backend/internal/handler/dto/settings.go`：

```go
// MarqueeMessage represents a single marquee strip text.
type MarqueeMessage struct {
    ID        string `json:"id"`         // UUID, 后端生成
    Text      string `json:"text"`       // 字幕内容，1–500 字
    Enabled   bool   `json:"enabled"`    // 单条启停
    SortOrder int    `json:"sort_order"` // 排序，0..N-1
}

// SystemSettings (admin payload) 新增 ──
MarqueeEnabled  bool             `json:"marquee_enabled"`
MarqueeMessages []MarqueeMessage `json:"marquee_messages"`

// PublicSettings (公开 payload) 新增 ──
MarqueeEnabled  bool             `json:"marquee_enabled"`
MarqueeMessages []MarqueeMessage `json:"marquee_messages"` // 仅下发 enabled=true，按 sort_order 升序
```

### 3.3 校验规则

校验全部发生在 admin handler 层（与 `custom_menu_items` 同款做法，service 层只看 raw JSON）。

- `marquee_messages` 数组长度上限 **20**
- 每条 `text`：trim 后长度 **1–500**；为空则拒绝
- `sort_order`：保存时按数组顺序自动重新分配为 `0..N-1`，避免冲突
- `id`：
  - 为空时后端用项目已有的 `generateMenuItemID()`（或新写一个对称的 `generateMarqueeMessageID()`）生成短随机 hex
  - 非空时必须匹配 `^[a-zA-Z0-9_-]+$`（复用现有 `menuItemIDPattern`）
  - 长度上限 **32 字符**（与 `maxMenuItemIDLen` 一致）
  - 同次保存中所有 ID 必须唯一，重复直接 400

---

## 4. 后端改动

无 schema 改动，无 `go generate`。

### 4.1 三层数据形态

跟随 `custom_menu_items` 的现有范式分层：

| 层 | 数据形态 | 职责 |
|---|---|---|
| **service**（`SystemSettings` / `PublicSettings` / `PublicSettingsInjectionPayload`） | `MarqueeMessages json.RawMessage`（保留 raw JSON 字符串） | 透传；不做结构化校验 |
| **admin handler**（PUT） | 接收 `[]dto.MarqueeMessage` | 全部校验（长度、trim、ID 格式/补齐/唯一性、sort_order 归一化）→ marshal 成 JSON 字符串 → 写入 service |
| **admin handler**（GET）/ **public handler** / **SSR injection** | 输出 `[]dto.MarqueeMessage` | parse raw JSON；公开侧过滤 `enabled=false`，按 `sort_order` 升序 |

### 4.2 文件改动清单

| 文件 | 改动内容 |
|---|---|
| `backend/internal/handler/dto/settings.go` | 增 `MarqueeMessage` 结构；`SystemSettings`/`PublicSettings` 各加 2 字段；增 `ParseMarqueeMessages` helper（仿 `ParseCustomMenuItems`） |
| `backend/internal/service/domain_constants.go` | 增 2 个 setting key 常量 |
| `backend/internal/service/setting_service.go` | (a) `SystemSettings` / `PublicSettings` 加 2 字段（`MarqueeMessages` 用 `json.RawMessage` 与 `CustomMenuItems` 同款）；(b) **`PublicSettingsInjectionPayload` 同步加 2 字段**——这是必须项，否则契约测试 `TestPublicSettingsInjectionPayload_SchemaDoesNotDrift` 会 fail；(c) `GetPublicSettingsForInjection` 映射这两个字段；(d) `getDefaultSystemSettings` 默认值加 `SettingMarqueeMessages: "[]"` |
| `backend/internal/service/settings_view.go` | 公开视图：parse JSON、过滤 `enabled=false`、按 `sort_order` 升序输出 |
| `backend/internal/handler/admin/setting_handler.go` | (a) admin GET 在 `BuildAdminSettingsResponse` 处把 raw JSON parse 成 `[]MarqueeMessage`；(b) admin PUT 增加完整校验段（参考第 892-967 行 `custom_menu_items` 实现）；(c) 变更日志 diff 比较加 `marquee_messages` |
| `backend/internal/handler/setting_handler.go` | 公开 GET 输出 parsed + filtered marquee 数组 |
| `backend/internal/server/api_contract_test.go` | 契约测试加新字段断言 |
| `backend/internal/service/setting_service_test.go` | 单元测试覆盖默认值、JSON 往返 |
| `backend/internal/handler/admin/setting_handler_test.go` | 校验单测（长度、trim、ID 格式/补齐/唯一性、sort_order 归一化） |

---

## 5. 前端改动

### 5.1 新增组件 `AppHeaderMarquee.vue`

路径：`frontend/src/components/layout/AppHeaderMarquee.vue`

```vue
<template>
  <!--
    桌面端 (>=lg): 根容器渲染并 flex-1 撑满 AppHeader 中间槽位，无字幕内容时仍保留 flex-1
    避免"启用/禁用"切换时左右两端抖动。
    移动端 (<lg): display:none，槽位完全消失；AppHeader 外层的 justify-between 继续生效，
    左右两段被推到两端，视觉与改动前一致。
  -->
  <div class="marquee-slot hidden min-w-0 mx-4 lg:block lg:flex-1">
    <div
      v-if="text"
      class="marquee-shell"
      :aria-label="text"
      role="marquee"
    >
      <div class="marquee-track" @mouseenter="paused = true" @mouseleave="paused = false">
        <span class="marquee-segment" :class="{ paused }">{{ text }}</span>
        <span class="marquee-segment" :class="{ paused }" aria-hidden="true">{{ text }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useAppStore } from '@/stores'

const appStore = useAppStore()
const paused = ref(false)

const text = computed(() => {
  const s = appStore.cachedPublicSettings
  if (!s?.marquee_enabled) return ''
  const msgs = (s.marquee_messages ?? [])
    .filter(m => m.enabled && m.text?.trim())
    .map(m => m.text.trim())
  if (msgs.length === 0) return ''
  return msgs.join('  ·  ')
})
</script>

<style scoped>
.marquee-shell {
  @apply relative h-full overflow-hidden flex items-center;
  mask-image: linear-gradient(90deg, transparent 0, black 24px, black calc(100% - 24px), transparent 100%);
}
.marquee-track {
  @apply flex w-max whitespace-nowrap;
}
.marquee-segment {
  @apply pr-12 text-sm text-gray-600 dark:text-dark-300;
  animation: marquee 30s linear infinite;
}
.marquee-segment.paused { animation-play-state: paused; }
@keyframes marquee {
  from { transform: translateX(0); }
  to   { transform: translateX(-100%); }
}
@media (prefers-reduced-motion: reduce) {
  .marquee-segment { animation: none; }
}
</style>
```

**特性**：

- 纯 CSS 动画，无 JS 定时器
- 渲染两份文本实现无缝循环
- hover 暂停滚动
- 两端 24px mask 渐变，文字"虚出虚入"
- `prefers-reduced-motion` 下停用动画
- 移动端 (`<lg`) 不渲染

### 5.2 改动 `AppHeader.vue`

[frontend/src/components/layout/AppHeader.vue](../../../frontend/src/components/layout/AppHeader.vue) 的外层 `flex justify-between` **保留不动**。直接在左右两个 div 之间插入 `<AppHeaderMarquee />`：

```html
<div class="flex h-16 items-center justify-between px-4 md:px-6">
  <div class="flex items-center gap-4">  <!-- 左：菜单按钮 + 标题（不动） --> </div>
  <AppHeaderMarquee />                   <!-- 中：自身 lg:flex-1 撑开 -->
  <div class="flex items-center gap-3">  <!-- 右：工具栏（不动） --> </div>
</div>
```

**关键**：

- 桌面端（≥lg）：`AppHeaderMarquee` 自身带 `lg:flex-1` 把中间撑满，左右两段被推到两端
- 移动端（<lg）：`AppHeaderMarquee` 设 `display:none`，三段 flex 退化为两段，**`justify-between` 继续把左右两段推到两端**（与改动前的视觉完全一致）
- 任何情况下都不需要给左右段加 `ml-auto` 之类补丁

这种"中间组件自身负责响应式"的写法把改动局限在新组件内，对 AppHeader 的回归风险最小。

### 5.3 类型定义

`frontend/src/types/index.ts`：

```typescript
export interface MarqueeMessage {
  id: string
  text: string
  enabled: boolean
  sort_order: number
}
// 在 PublicSettings 和 SystemSettings 接口里加上：
//   marquee_enabled: boolean
//   marquee_messages: MarqueeMessage[]
```

### 5.4 store 改动

- [stores/app.ts](../../../frontend/src/stores/app.ts)：
  - 类型层：`PublicSettings` 加 `marquee_enabled: boolean` 和 `marquee_messages: MarqueeMessage[]`
  - **fallback 默认值**：`fetchPublicSettings` 内部第 320-361 行的兜底对象需要追加 `marquee_enabled: false, marquee_messages: []`，否则 `cachedPublicSettings` 命中默认分支时会缺字段、组件读到 `undefined`
  - 不需要新加 action / computed
- [stores/adminSettings.ts](../../../frontend/src/stores/adminSettings.ts)：模仿现有 `customMenuItems` 的 add/update/delete/reorder 行为，新增一组 `marqueeMessages` 操作
- [api/admin/settings.ts](../../../frontend/src/api/admin/settings.ts)：DTO 同步加新字段

---

## 6. Admin 设置页 UI

在 [admin/SettingsView.vue](../../../frontend/src/views/admin/SettingsView.vue) 中新增一个 section，**位置紧挨"自定义菜单"之后**（业务语义相邻）。

### 6.1 视觉布局

```text
┌──────────────────────────────────────────────────────────────────┐
│ 顶部跑马灯字幕                                                     │
│ ────────────────────────────────────                             │
│ ☐  启用字幕                          [全局开关 toggle]            │
│                                                                  │
│  排序  启用   字幕文本                                  操作       │
│  ───────────────────────────────────────────────────────────────  │
│  ⋮⋮    [✓]   🚀 AI 会员店上新！ChatGPT Plus / Grok... | [✏️] [🗑️]  │
│  ⋮⋮    [✓]   🎁 黑五五折大促开抢                        | [✏️] [🗑️]  │
│  ⋮⋮    [ ]   💬 (草稿) 暂未启用                         | [✏️] [🗑️]  │
│                                                                  │
│  [ + 新增字幕 ]                                                   │
│                                                                  │
│  💡 提示：字幕文本会在 Header 中间循环滚动；多条之间用"·"自动分隔。 │
│     启用条数为 0 时，字幕区不显示。                                 │
└──────────────────────────────────────────────────────────────────┘
```

### 6.2 交互细节

- **拖拽排序**：尽量复用项目现有的拖拽组件；若无，回退为"上下箭头按钮"
- **编辑**：inline editing 或 modal，跟现有"自定义菜单"保持一致
- **保存**：跟整个 SettingsView 的"保存"按钮共用（项目现有约定）
- **行内删除**：与现有列表组件交互一致（确认弹窗可省略，因为可通过"启用"开关软删）

### 6.3 i18n key（仅 Admin UI 文案）

- `admin.settings.marquee.title`
- `admin.settings.marquee.description`
- `admin.settings.marquee.enabled`
- `admin.settings.marquee.add`
- `admin.settings.marquee.textPlaceholder`
- `admin.settings.marquee.draftBadge`
- `admin.settings.marquee.tipMultipleSeparator`

中英两份。**注意**：字幕本身的内容**不做 i18n**（用户已明确）。

---

## 7. 测试计划

| 类型 | 文件 | 用例 |
|---|---|---|
| 后端单测 | `setting_service_test.go` | `getDefaultSystemSettings` 含 `SettingMarqueeMessages: "[]"`；JSON 序列化往返 |
| 后端单测 | `setting_handler_test.go`（admin） | 校验：长度上限 20、空文本拒绝、ID 格式 `^[a-zA-Z0-9_-]+$`、ID 自动补齐、ID 唯一性冲突 400、sort_order 归一化、变更日志 diff |
| 后端契约 | `api_contract_test.go` | admin GET/PUT、public GET 都包含 marquee 字段；公开侧仅返回 `enabled=true` 且按 `sort_order` 升序 |
| 后端契约 | `public_settings_injection_schema_test.go`（**已存在，自动生效**） | `PublicSettingsInjectionPayload` 加字段后，schema-drift 测试自动通过；漏加则 fail |
| 前端组件 | `AppHeaderMarquee.spec.ts`（新建） | 启用/禁用、空文本、多条拼接 ` · `、hover 暂停 class 切换、`hidden lg:block` class 存在 |
| 前端 store | `adminSettings.spec.ts` | add/update/delete/reorder marquee message 行为正确；payload 形态符合 admin handler 期望 |
| 前端 view | `SettingsView.spec.ts` | 配置面板能正确渲染、新增/删除字幕生效、保存调用正确 API |
| 视觉/手测 | 手动 | (a) 桌面端 Header 三段布局正确；(b) **移动端 Header 与改动前视觉一致**：右侧工具栏靠右、左侧菜单按钮靠左、不重叠、不抖动；(c) 暗色模式跑马灯文字对比度 OK |

**说明**：组件测中的"移动端不渲染"实际是 CSS `display:none`（DOM 仍存在），单测只能断言 class，**真实布局正确性由手动视觉/截图验证**——这是 Codex 12.6 指出的精确化措辞。

PR 提交前按 [CLAUDE.md](../../../CLAUDE.md) checklist：

- `go test -tags=unit ./...` + `go test -tags=integration ./...`
- `golangci-lint run ./...`
- 前端 `npm test`
- `pnpm-lock.yaml` 不变（本设计无新依赖）

---

## 8. 安全 / 性能 / 可访问性

### 8.1 安全

- 字幕由管理员配置，**会展示给所有登录后的用户（普通用户 + 管理员）**——但 XSS 风险低，因为前端使用 Vue 文本插值 `{{ text }}` 自动转义，**未使用 `v-html`**
- 公开接口下发的 marquee 字段不含敏感信息（仅 id / text / enabled / sort_order）
- 文本长度上限 500 字 + 数组上限 20 条，确保超长文本不会破坏 Header 布局或撑大公开接口体积

### 8.2 性能

- 纯 CSS 动画，GPU 合成，无 JS 主线程占用
- 数据走现有 `/api/settings/public` 缓存，**零额外网络请求**
- JSON 字段长度上限严格（20 条 × 500 字 ≈ 10KB），对设置接口体积无明显影响

### 8.3 可访问性

- `role="marquee"` + `aria-label`（朗读全部文案）
- `prefers-reduced-motion` 下自动停用动画
- 第二份文本带 `aria-hidden="true"`，避免重复朗读
- 颜色对比度走 Tailwind `gray-600 / dark-300`，符合 WCAG AA

---

## 9. 上线后运维

- **临时下线**：admin 把"启用字幕"全局开关关掉即可，无需发版
- **更换文案**：直接改文本、保存即可，所有用户下次刷新页面（或现有的 settings 缓存失效后）自动看到新文案
- **A/B 文案试验**：手动改文案 → 观察 `umami` / 自带的 dashboard 用户活跃数据；本期不做内置点击统计

---

## 10. 设计假设与待确认项

写实施计划阶段需要确认：

1. **拖拽排序组件是否已有可复用的**——查阅 `frontend/src/components/` 现有 SortableList / draggable 类组件；如无现成的，admin UI 用上下箭头按钮代替拖拽，避免引入新依赖
2. **管理员视角是否也展示跑马灯**——本设计默认"是"（便于自查文案）。如果产品上希望"admin 不看广告"，加一个 `authStore.isAdmin` 判断隐藏即可

---

## 11. 不在本期范围（YAGNI）

下列功能在本期**不实现**，未来如有真实需求再迭代：

- 单条字幕的起止时间窗口（活动期）
- 用户级"X 关闭后 N 天不再显示"
- 点击字幕跳转（外链 / 站内 iframe）
- 字幕配色 / 图标 / 样式个性化
- 字幕本身的多语言（zh / en 分别配置）
- 内置点击/曝光统计
- A/B test 后端分流

---

## 12. 设计评审记录（Codex）

### 12.1 评审结论

整体方向可行，但进入实施计划前需要修正几个会导致返工或回归的问题。重点是：移动端 Header 布局、公开设置 SSR 注入、后端分层落点、ID 校验规则，以及测试计划的可验证性。

**处理状态（2026-04-26 更新）**：7 条评审意见全部经源码核对验证为正确，已在本文档对应章节修订：

| 评审项 | 修订位置 |
|---|---|
| 12.2 P1 移动端 Header 布局 | §5.2 改为保留 `justify-between`、组件自身用 `lg:flex-1` |
| 12.3 P1 SSR 注入 + 前端 fallback | §4.2 显式列出 `PublicSettingsInjectionPayload` 改动；§5.4 列出 app.ts fallback 默认值改动；§7 列出契约测试自动覆盖 |
| 12.4 P2 后端分层 | §4.1 新增三层数据形态表 |
| 12.5 P2 ID 校验 | §3.3 补 ID 格式 / 长度 / 唯一性 / 复用现有 helper |
| 12.6 P3 测试措辞 | §7 改为 "CSS hidden + 手动视觉验证" 双重保证 |
| 12.7 P3 安全措辞 | §8.1 改为 "登录后所有用户可见，XSS 低风险来自 Vue 文本插值" |
| 12.8 实施 checklist | 直接进入 writing-plans 阶段时纳入实施步骤 |

### 12.2 P1：移动端 Header 布局可能回归

当前设计把 `AppHeader` 外层从现有 `justify-between` 改成普通三段 flex：

```html
<div class="flex h-16 items-center px-4 md:px-6">
  <div class="flex items-center gap-4">...</div>
  <AppHeaderMarquee />
  <div class="flex items-center gap-3">...</div>
</div>
```

但 `AppHeaderMarquee` 在 `<lg` 下 `hidden`，移动端中间槽位消失后，右侧工具栏可能会挤到左侧菜单按钮旁边。现有实现依赖 `justify-between` 把右侧工具栏推到最右侧：

```html
<div class="flex h-16 items-center justify-between px-4 md:px-6">
```

**建议修正**：

- 保留移动端 `justify-between` 行为，或给右侧工具栏加 `ml-auto`
- `AppHeaderMarquee` 只在桌面端参与中间弹性布局
- 在测试计划中增加移动端 Header 截图或 e2e 校验，确认右侧工具栏仍靠右且不与左侧菜单按钮重叠

### 12.3 P1：遗漏 SSR 注入与前端 fallback 默认值

设计文档目前写到：

> `stores/app.ts`：`cachedPublicSettings` 已经在使用，加完类型字段就能用，**无功能改动**

这个判断不完整。项目当前存在公开设置的 SSR 注入结构 `PublicSettingsInjectionPayload`，注释明确说明：漏加公开字段会导致前端首屏读取到 `undefined`，直到 `/api/v1/settings/public` 异步返回后才恢复，容易出现首屏闪烁或行为不一致。

同时，`frontend/src/stores/app.ts` 的 fallback 默认值也需要同步新增：

```ts
marquee_enabled: false,
marquee_messages: [],
```

**建议修正**：

- `backend/internal/service/setting_service.go`：`PublicSettingsInjectionPayload` 增加 `marquee_enabled` / `marquee_messages`
- `GetPublicSettingsForInjection` 同步映射这两个字段
- `frontend/src/stores/app.ts` 的默认 `cachedPublicSettings` 增加对应 fallback
- 测试计划补充 `public_settings_injection_schema_test.go` 和 app store fallback 覆盖

### 12.4 P2：后端分层描述需要贴合现有实现

设计文档目前写：

- `setting_service.go`：模仿 `custom_menu_items` 的 get/set/校验逻辑
- `settings_view.go`：公开视图过滤 `enabled=false`，按 `sort_order` 升序输出

但现有 `custom_menu_items` 的实际形态是：

- service 内部 `SystemSettings` / `PublicSettings` 仍使用 JSON string 存储类似字段
- admin PUT 的校验、补 ID、marshal 主要发生在 `backend/internal/handler/admin/setting_handler.go`
- public handler / SSR injection 再把 raw JSON parse 成 DTO 结构并做可见性过滤

**建议修正**：

明确三层数据形态：

- service 层：`MarqueeMessages string`，保持 raw JSON，与 `CustomMenuItems` 同范式
- admin handler：接收 `[]dto.MarqueeMessage`，负责校验、补 ID、排序归一化、marshal
- public handler / injection：parse raw JSON，过滤 `enabled=false`，按 `sort_order` 升序输出结构化数组

这样能减少实现时在 service/handler 两层重复校验或类型不一致的问题。

### 12.5 P2：`id` 校验规则不完整

当前文档只写了：

> `id`：为空时后端用 `uuid.New()` 补齐

但没有定义非空 ID 的格式、长度和唯一性。由于前端列表渲染、编辑、删除会依赖 `id`，重复或非法 ID 可能导致 UI key 冲突、误删、排序异常。

**建议修正**：

- 非空 `id` 增加格式校验：只允许 `a-zA-Z0-9_-`
- 增加最大长度限制，可沿用 `custom_menu_items` 的 32 字符限制，或如使用完整 UUID 则明确上限为 36
- 保存前检查 ID 唯一性，发现重复直接 400
- 文档中说明后端生成方式使用项目已有依赖 `github.com/google/uuid` 的 `uuid.NewString()`

### 12.6 P3：测试计划里的“移动端不渲染”表述不准确

组件示例使用的是 Tailwind：

```html
<div class="marquee-slot hidden lg:block flex-1 min-w-0 mx-4">
```

这意味着 DOM 仍然存在，只是 CSS 隐藏。组件单测通常只能断言 class 或状态，不能证明真实移动端布局不占空间、不重叠。

**建议修正**：

- 将测试描述从“移动端不渲染”改成“移动端不显示且不占用布局空间”
- 使用浏览器 viewport 截图或 e2e 测试覆盖移动端 Header 布局
- 若坚持“移动端不渲染”，则组件需要基于 viewport/composable 条件渲染，而不是仅使用 CSS class

### 12.7 P3：安全章节表述需要更准确

当前安全章节写：

> 字幕内容由 admin 输入、admin 自己看到，**不存在 XSS 风险**

这不准确。字幕是登录后内部页面的常驻营销位，普通用户也会看到；同时 `/api/settings/public` 是公开设置接口，不应把安全边界描述成“admin 自己看到”。

**建议修正**：

- 改成“字幕由管理员配置，会展示给登录后的普通用户和管理员”
- XSS 风险低的核心理由是 Vue 文本插值 `{{ text }}` 会转义，不使用 `v-html`
- 保留文本长度限制，避免异常长文本影响布局和接口体积

### 12.8 建议更新到实施计划的检查项

进入实施前，建议把以下事项加入实施 checklist：

- `PublicSettingsInjectionPayload` 与 `dto.PublicSettings` 同步
- `GetPublicSettingsForInjection` 映射 marquee 字段
- `app.ts` fallback 默认值包含 marquee 字段
- admin PUT 校验包含：长度上限、trim、ID 补齐、ID 格式、ID 唯一性、排序归一化
- public GET 和 SSR 注入都过滤 disabled 字幕并排序
- Header 桌面端和移动端都做布局验证
- 安全文案修正为“文本插值转义”，不要写成“admin 自己看到”
