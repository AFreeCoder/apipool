<template>
  <div class="flex h-screen flex-col bg-gray-50 dark:bg-dark-900">
    <!-- 顶栏 -->
    <header
      class="flex h-16 flex-shrink-0 items-center justify-between border-b border-gray-200 bg-white px-4 dark:border-dark-700 dark:bg-dark-800 md:px-6"
    >
      <div class="flex items-center gap-4">
        <!-- 移动端汉堡按钮 -->
        <button
          @click="sidebarOpen = !sidebarOpen"
          class="rounded-lg p-2 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-700 dark:hover:text-white lg:hidden"
          aria-label="Toggle sidebar"
        >
          <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25h16.5" />
          </svg>
        </button>

        <!-- 返回主站 -->
        <router-link
          to="/"
          class="flex items-center gap-2 rounded-lg px-2.5 py-1.5 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-900 dark:text-dark-400 dark:hover:bg-dark-700 dark:hover:text-white"
        >
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M10.5 19.5L3 12m0 0l7.5-7.5M3 12h18" />
          </svg>
          <span class="hidden sm:inline">返回主站</span>
        </router-link>

        <!-- 标题 -->
        <h1 class="text-lg font-semibold text-gray-900 dark:text-white">文档</h1>
      </div>

      <!-- 主题切换 -->
      <button
        @click="toggleTheme"
        class="rounded-lg p-2 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-700 dark:hover:text-white"
        :title="isDark ? '切换到亮色模式' : '切换到暗色模式'"
      >
        <svg v-if="isDark" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
          <path stroke-linecap="round" stroke-linejoin="round" d="M12 3v2.25m6.364.386l-1.591 1.591M21 12h-2.25m-.386 6.364l-1.591-1.591M12 18.75V21m-4.773-4.227l-1.591 1.591M5.25 12H3m4.227-4.773L5.636 5.636M15.75 12a3.75 3.75 0 11-7.5 0 3.75 3.75 0 017.5 0z" />
        </svg>
        <svg v-else class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
          <path stroke-linecap="round" stroke-linejoin="round" d="M21.752 15.002A9.718 9.718 0 0118 15.75c-5.385 0-9.75-4.365-9.75-9.75 0-1.33.266-2.597.748-3.752A9.753 9.753 0 003 11.25C3 16.635 7.365 21 12.75 21a9.753 9.753 0 009.002-5.998z" />
        </svg>
      </button>
    </header>

    <div class="flex flex-1 overflow-hidden">
      <!-- 移动端遮罩 -->
      <div
        v-if="sidebarOpen"
        class="fixed inset-0 z-20 bg-black/50 lg:hidden"
        @click="sidebarOpen = false"
      />

      <!-- 左侧导航 -->
      <aside
        :class="[
          'fixed inset-y-0 left-0 z-30 mt-16 w-64 flex-shrink-0 transform border-r border-gray-200 bg-white transition-transform duration-200 ease-in-out dark:border-dark-700 dark:bg-dark-800 lg:relative lg:z-0 lg:mt-0 lg:translate-x-0',
          sidebarOpen ? 'translate-x-0' : '-translate-x-full'
        ]"
      >
        <nav class="flex h-full flex-col overflow-y-auto p-4">
          <div class="mb-3 px-3 text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-dark-500">
            目录
          </div>
          <ul class="space-y-1">
            <li v-for="doc in docsList" :key="doc.slug">
              <router-link
                :to="`/docs/${doc.slug}`"
                :class="[
                  'flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium transition-colors',
                  currentSlug === doc.slug
                    ? 'bg-primary-50 text-primary-600 dark:bg-primary-900/20 dark:text-primary-400'
                    : 'text-gray-600 hover:bg-gray-100 hover:text-gray-900 dark:text-dark-400 dark:hover:bg-dark-700 dark:hover:text-white'
                ]"
                @click="sidebarOpen = false"
              >
                <svg class="h-4 w-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 00-3.375-3.375h-1.5A1.125 1.125 0 0113.5 7.125v-1.5a3.375 3.375 0 00-3.375-3.375H8.25m0 12.75h7.5m-7.5 3H12M10.5 2.25H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 00-9-9z" />
                </svg>
                {{ doc.title }}
              </router-link>
            </li>
          </ul>
        </nav>
      </aside>

      <!-- 右侧内容区 -->
      <main ref="mainRef" class="flex-1 overflow-y-auto">
        <div class="mx-auto max-w-4xl px-6 py-8 md:px-10 lg:px-16">
          <div v-if="currentDoc" class="docs-content" v-html="renderedHtml" />
          <div v-else class="flex flex-col items-center justify-center py-20 text-gray-400 dark:text-dark-500">
            <svg class="mb-4 h-16 w-16" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1">
              <path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 00-3.375-3.375h-1.5A1.125 1.125 0 0113.5 7.125v-1.5a3.375 3.375 0 00-3.375-3.375H8.25m0 12.75h7.5m-7.5 3H12M10.5 2.25H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 00-9-9z" />
            </svg>
            <p class="text-lg font-medium">文档未找到</p>
            <router-link to="/docs/quick-start" class="mt-2 text-sm text-primary-600 hover:underline dark:text-primary-400">
              前往快速开始
            </router-link>
          </div>
        </div>
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useRoute } from 'vue-router'
import { marked } from 'marked'
import DOMPurify from 'dompurify'

import quickStartMd from '@/docs/quick-start.md?raw'
import faqMd from '@/docs/faq.md?raw'
import rechargeAndPricingMd from '@/docs/recharge-and-pricing.md?raw'

// Markdown 渲染配置
marked.setOptions({
  breaks: true,
  gfm: true,
})

function renderMarkdown(content: string): string {
  if (!content) return ''
  const processed = content.replace(/your-api-domain/g, window.location.host)
  const html = marked.parse(processed) as string
  return DOMPurify.sanitize(html)
}

// 文档列表配置
const docsList = [
  { slug: 'quick-start', title: '快速开始', content: quickStartMd },
  { slug: 'recharge-and-pricing', title: '充值及费率说明', content: rechargeAndPricingMd },
  { slug: 'faq', title: '常见问题', content: faqMd },
]

// 路由和状态
const route = useRoute()
const sidebarOpen = ref(false)
const isDark = ref(document.documentElement.classList.contains('dark'))
const mainRef = ref<HTMLElement | null>(null)

// 当前文档 slug
const currentSlug = computed(() => route.params.slug as string)

// 当前文档对象
const currentDoc = computed(() => docsList.find(doc => doc.slug === currentSlug.value))

// 渲染后的 HTML
const renderedHtml = computed(() => {
  if (!currentDoc.value) return ''
  return renderMarkdown(currentDoc.value.content)
})

// 切换文档时滚动到顶部
watch(currentSlug, async () => {
  await nextTick()
  mainRef.value?.scrollTo({ top: 0 })
})

// 主题切换
function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

// 初始化主题
onMounted(() => {
  const savedTheme = localStorage.getItem('theme')
  if (
    savedTheme === 'dark' ||
    (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)
  ) {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }
})
</script>

<style scoped>
/* ============================
   Markdown 内容区样式（prose）
   ============================ */
.docs-content :deep(h1) {
  @apply mb-6 text-3xl font-bold text-gray-900 dark:text-white;
}

.docs-content :deep(h2) {
  @apply mb-4 mt-8 border-b border-gray-200 pb-2 text-2xl font-bold text-gray-900 dark:border-dark-700 dark:text-white;
}

.docs-content :deep(h3) {
  @apply mb-3 mt-6 text-xl font-semibold text-gray-900 dark:text-white;
}

.docs-content :deep(h4) {
  @apply mb-2 mt-4 text-lg font-semibold text-gray-900 dark:text-white;
}

.docs-content :deep(p) {
  @apply mb-4 leading-relaxed text-gray-700 dark:text-dark-300;
}

.docs-content :deep(a) {
  @apply text-primary-600 hover:underline dark:text-primary-400;
}

.docs-content :deep(strong) {
  @apply font-semibold text-gray-900 dark:text-white;
}

.docs-content :deep(em) {
  @apply italic;
}

/* 行内代码 */
.docs-content :deep(code) {
  @apply rounded bg-gray-100 px-1.5 py-0.5 text-sm text-primary-600 dark:bg-gray-800 dark:text-primary-400;
}

/* 代码块 */
.docs-content :deep(pre) {
  @apply mb-4 overflow-x-auto rounded-xl bg-gray-900 p-4 dark:bg-gray-950;
}

.docs-content :deep(pre code) {
  @apply bg-transparent p-0 text-sm text-gray-100;
}

/* 表格 */
.docs-content :deep(table) {
  @apply mb-4 w-full border-collapse overflow-hidden rounded-lg text-sm;
}

.docs-content :deep(thead) {
  @apply bg-gray-100 dark:bg-dark-700;
}

.docs-content :deep(th) {
  @apply border border-gray-200 px-4 py-2 text-left font-semibold text-gray-900 dark:border-dark-600 dark:text-white;
}

.docs-content :deep(td) {
  @apply border border-gray-200 px-4 py-2 text-gray-700 dark:border-dark-600 dark:text-dark-300;
}

.docs-content :deep(tbody tr:nth-child(even)) {
  @apply bg-gray-50 dark:bg-dark-800/50;
}

/* 列表 */
.docs-content :deep(ul) {
  @apply mb-4 list-disc pl-6 text-gray-700 dark:text-dark-300;
}

.docs-content :deep(ol) {
  @apply mb-4 list-decimal pl-6 text-gray-700 dark:text-dark-300;
}

.docs-content :deep(li) {
  @apply mb-1;
}

/* 引用块 */
.docs-content :deep(blockquote) {
  @apply mb-4 border-l-4 border-primary-500 bg-primary-50/50 p-4 italic text-gray-700 dark:bg-primary-900/10 dark:text-dark-300;
}

.docs-content :deep(blockquote p) {
  @apply mb-0;
}

/* 分割线 */
.docs-content :deep(hr) {
  @apply my-8 border-gray-200 dark:border-dark-700;
}

/* 图片 */
.docs-content :deep(img) {
  @apply my-4 max-w-full rounded-lg;
}

/* 引用块内图片（客服二维码等）缩小显示 */
.docs-content :deep(blockquote img) {
  @apply my-2 max-w-48 rounded-lg;
}
</style>
