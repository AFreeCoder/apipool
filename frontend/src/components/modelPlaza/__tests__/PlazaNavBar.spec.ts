import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import PlazaNavBar from '../PlazaNavBar.vue'
import { useAppStore } from '@/stores/app'
import type { PublicSettings } from '@/types'

let pinia: ReturnType<typeof createPinia>

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

describe('PlazaNavBar', () => {
  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
  })

  it('uses the APIPool brand when public site name is empty', () => {
    const appStore = useAppStore()
    appStore.cachedPublicSettings = { site_name: '' } as PublicSettings

    const wrapper = mount(PlazaNavBar, {
      global: {
        plugins: [pinia],
        stubs: { RouterLink: true },
      },
    })

    expect(wrapper.text()).toContain('APIPool')
    expect(wrapper.text()).not.toContain('Sub2API')
  })
})
