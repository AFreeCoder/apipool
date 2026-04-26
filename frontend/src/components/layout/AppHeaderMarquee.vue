<template>
  <div class="marquee-slot hidden min-w-0 mx-4 lg:block lg:flex-1">
    <div
      v-if="marqueeText"
      class="marquee-shell bg-primary-50 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300"
      role="marquee"
      :aria-label="marqueeText"
    >
      <div
        class="marquee-track"
        @mouseenter="paused = true"
        @mouseleave="paused = false"
      >
        <span class="marquee-segment" :class="{ paused }">{{ marqueeText }}</span>
        <span class="marquee-segment" :class="{ paused }" aria-hidden="true">{{ marqueeText }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useAppStore } from '@/stores'

const appStore = useAppStore()
const paused = ref(false)

const marqueeText = computed(() => {
  const settings = appStore.cachedPublicSettings
  if (!settings?.marquee_enabled) return ''

  return settings.marquee_messages
    .filter((message) => message.enabled)
    .map((message) => message.text.trim())
    .filter(Boolean)
    .join('  ·  ')
})
</script>

<style scoped>
.marquee-shell {
  overflow: hidden;
  border-radius: 9999px;
}

.marquee-track {
  display: flex;
  min-width: 100%;
  width: max-content;
}

.marquee-segment {
  flex: 0 0 auto;
  min-width: 100%;
  padding: 0.375rem 2rem;
  white-space: nowrap;
  font-size: 0.875rem;
  font-weight: 500;
  line-height: 1.25rem;
  animation: app-header-marquee 32s linear infinite;
}

.marquee-segment.paused {
  animation-play-state: paused;
}

@keyframes app-header-marquee {
  from {
    transform: translateX(0);
  }
  to {
    transform: translateX(-100%);
  }
}

@media (prefers-reduced-motion: reduce) {
  .marquee-segment {
    animation: none;
  }
}

</style>
