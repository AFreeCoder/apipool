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

- `marquee_messages` 数组长度上限 **20**
- 每条 `text`：trim 后长度 **1–500**；为空则拒绝
- `sort_order`：保存时按数组顺序自动重新分配为 `0..N-1`，避免冲突
- `id`：为空时后端用 `uuid.New()` 补齐

---

## 4. 后端改动

无 schema 改动，无 `go generate`。

| 文件 | 改动内容 |
|---|---|
| `backend/internal/handler/dto/settings.go` | 增 `MarqueeMessage` 结构；`SystemSettings`/`PublicSettings` 各加 2 字段 |
| `backend/internal/service/domain_constants.go` | 增 2 个 setting key 常量 |
| `backend/internal/service/setting_service.go` | 模仿 `custom_menu_items` 的 get/set/校验逻辑（限长、UUID 补齐、sort_order 归一化） |
| `backend/internal/service/settings_view.go` | 公开视图：过滤 `enabled=false`，按 `sort_order` 升序输出 |
| `backend/internal/handler/admin/setting_handler.go` | admin GET/PUT 携带新字段 |
| `backend/internal/handler/setting_handler.go` | 公开 GET 携带新字段 |
| `backend/internal/server/api_contract_test.go` | 契约测试加新字段断言 |

---

## 5. 前端改动

### 5.1 新增组件 `AppHeaderMarquee.vue`

路径：`frontend/src/components/layout/AppHeaderMarquee.vue`

```vue
<template>
  <!--
    根容器始终渲染并占据 flex-1，保证 AppHeader 中间留白固定，
    避免"启用 / 禁用"切换时左右两端布局抖动。
    实际跑马灯内容按条件渲染。
    `hidden lg:block` 让移动端整槽位都不占空间。
  -->
  <div class="marquee-slot hidden lg:block flex-1 min-w-0 mx-4">
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

[frontend/src/components/layout/AppHeader.vue](../../../frontend/src/components/layout/AppHeader.vue) 的外层布局当前是 `flex justify-between`（左侧标题 + 右侧工具栏）。

改为显式三段式：

```html
<div class="flex h-16 items-center px-4 md:px-6">
  <div class="flex items-center gap-4">  <!-- 左：菜单按钮 + 标题 --> </div>
  <AppHeaderMarquee />                   <!-- 中：flex-1，自动撑开 -->
  <div class="flex items-center gap-3">  <!-- 右：工具栏 --> </div>
</div>
```

`AppHeaderMarquee` 根容器始终带 `flex-1` 占据中间空白槽位（即使无字幕内容），避免"启用/禁用"切换时左右两端布局抖动。仅 `hidden lg:block`：移动端槽位完全消失。

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

- [stores/app.ts](../../../frontend/src/stores/app.ts)：`cachedPublicSettings` 已经在使用，加完类型字段就能用，**无功能改动**
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
| 后端单测 | `setting_service_test.go` | marquee_messages 校验：长度上限 20、空文本拒绝、id 自动补齐、sort_order 归一化、JSON 序列化往返 |
| 后端契约 | `api_contract_test.go` | admin GET/PUT、public GET 都包含 marquee 字段；公开侧仅返回 `enabled=true` |
| 前端组件 | `AppHeaderMarquee.spec.ts`（新建） | 启用/禁用渲染、空文本不渲染、多条拼接 ` · `、hover 暂停状态、移动端不渲染 |
| 前端 store | `adminSettings.spec.ts` | add/update/delete/reorder marquee message 行为正确 |
| 前端 view | `SettingsView.spec.ts` | 配置面板能正确渲染、新增/删除字幕生效、保存调用正确 API |

PR 提交前按 [CLAUDE.md](../../../CLAUDE.md) checklist：

- `go test -tags=unit ./...` + `go test -tags=integration ./...`
- `golangci-lint run ./...`
- 前端 `npm test`
- `pnpm-lock.yaml` 不变（本设计无新依赖）

---

## 8. 安全 / 性能 / 可访问性

### 8.1 安全

- 字幕内容由 admin 输入、admin 自己看到，**不存在 XSS 风险**（前端用 `{{ text }}` 文本插值，不是 `v-html`）
- 公开接口下发的 marquee 字段不含敏感信息

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
