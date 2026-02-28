<template>
  <AppLayout>
    <div class="purchase-page-layout">
      <div class="flex items-start justify-between gap-4">
        <div>
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('purchase.title') }}
          </h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            {{ t('purchase.description') }}
          </p>
        </div>

        <div class="flex items-center gap-2">
          <a
            v-if="isValidUrl"
            :href="purchaseUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="btn btn-secondary btn-sm"
          >
            <Icon name="externalLink" size="sm" class="mr-1.5" :stroke-width="2" />
            {{ t('purchase.openInNewTab') }}
          </a>
        </div>
      </div>

      <div class="card overflow-hidden">
        <div v-if="loading" class="flex items-center justify-center py-12">
          <div
            class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"
          ></div>
        </div>

        <div
          class="flex items-center justify-center p-10 text-center"
          v-else-if="!purchaseEnabled"
        >
          <div class="max-w-md">
            <div
              class="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-gray-100 dark:bg-dark-700"
            >
              <Icon name="creditCard" size="lg" class="text-gray-400" />
            </div>
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('purchase.notEnabledTitle') }}
            </h3>
            <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">
              {{ t('purchase.notEnabledDesc') }}
            </p>
          </div>
        </div>

        <div
          v-else-if="!isValidUrl"
          class="flex h-full items-center justify-center p-10 text-center"
        >
          <div class="max-w-md">
            <div
              class="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-gray-100 dark:bg-dark-700"
            >
              <Icon name="link" size="lg" class="text-gray-400" />
            </div>
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('purchase.notConfiguredTitle') }}
            </h3>
            <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">
              {{ t('purchase.notConfiguredDesc') }}
            </p>
          </div>
        </div>

        <iframe v-else ref="iframeRef" :src="purchaseUrl" class="block w-full border-0" :style="{ height: iframeHeight }" allowfullscreen referrerpolicy="no-referrer"></iframe>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const iframeRef = ref<HTMLIFrameElement | null>(null)
const iframeHeight = ref('600px')

const purchaseEnabled = computed(() => {
  return appStore.cachedPublicSettings?.purchase_subscription_enabled ?? false
})

const purchaseUrl = computed(() => {
  return (appStore.cachedPublicSettings?.purchase_subscription_url || '').trim()
})

const isValidUrl = computed(() => {
  const url = purchaseUrl.value
  return url.startsWith('http://') || url.startsWith('https://')
})

function updateIframeHeight() {
  const header = document.querySelector('header') as HTMLElement | null
  const headerH = header?.offsetHeight || 64
  const main = document.querySelector('main') as HTMLElement | null
  const mainPaddingTop = main ? parseInt(getComputedStyle(main).paddingTop) : 32
  const mainPaddingBottom = main ? parseInt(getComputedStyle(main).paddingBottom) : 32
  // 减去标题区域高度（约 80px）和 gap-6（24px）
  const titleSectionH = 104
  const available = window.innerHeight - headerH - mainPaddingTop - mainPaddingBottom - titleSectionH
  iframeHeight.value = `${Math.max(available, 600)}px`
}

onMounted(async () => {
  if (!appStore.publicSettingsLoaded) {
    loading.value = true
    try {
      await appStore.fetchPublicSettings()
    } finally {
      loading.value = false
    }
  }
  updateIframeHeight()
  window.addEventListener('resize', updateIframeHeight)
})

onUnmounted(() => {
  window.removeEventListener('resize', updateIframeHeight)
})
</script>

<style scoped>
.purchase-page-layout {
  @apply flex flex-col gap-6;
}
</style>

