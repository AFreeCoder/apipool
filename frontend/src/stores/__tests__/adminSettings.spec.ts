import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAdminSettingsStore } from '@/stores/adminSettings'

const mocks = vi.hoisted(() => ({
  getSettings: vi.fn(),
  getConfig: vi.fn(),
}))

vi.mock('@/api', () => ({
  adminAPI: {
    settings: {
      getSettings: mocks.getSettings,
    },
    payment: {
      getConfig: mocks.getConfig,
    },
  },
}))

describe('useAdminSettingsStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    mocks.getSettings.mockReset()
    mocks.getConfig.mockReset()
    mocks.getConfig.mockResolvedValue({ data: { enabled: false } })
  })

  it('提供跑马灯设置默认值', () => {
    const store = useAdminSettingsStore()

    expect(store.marqueeEnabled).toBe(false)
    expect(store.marqueeMessages).toEqual([])
  })

  it('fetch 后映射跑马灯设置', async () => {
    const message = {
      id: 'promo-1',
      text: '充值优惠',
      enabled: true,
      sort_order: 0,
    }
    mocks.getSettings.mockResolvedValue({
      marquee_enabled: true,
      marquee_messages: [message],
      custom_menu_items: [],
    })

    const store = useAdminSettingsStore()
    await store.fetch(true)

    expect(store.marqueeEnabled).toBe(true)
    expect(store.marqueeMessages).toEqual([message])
  })
})
