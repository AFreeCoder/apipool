import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import PurchaseSubscriptionView from '../PurchaseSubscriptionView.vue'

const { settings } = vi.hoisted(() => ({
  settings: {
    purchase_subscription_enabled: true,
    purchase_subscription_url: 'https://payments.example.test/purchase?campaign=existing',
    payment_enabled: false,
    subscription_enabled: true,
  },
}))
vi.mock('@/stores', () => ({
  useAppStore: () => ({ publicSettingsLoaded: true, cachedPublicSettings: settings }),
}))
vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ user: { id: 42 }, token: 'test-session-token' }),
}))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key, locale: { value: 'zh' } }),
}))

const render = () => mount(PurchaseSubscriptionView, {
  global: { stubs: { AppLayout: { template: '<div><slot /></div>' }, Icon: true } },
})

describe('APIPool external purchase', () => {
  beforeEach(() => {
    settings.purchase_subscription_enabled = true
    settings.payment_enabled = false
  })

  it('keeps the external purchase available when built-in payment is disabled', async () => {
    const wrapper = render()
    await flushPromises()
    const url = new URL(wrapper.get('iframe').attributes('src'))
    expect(url.origin + url.pathname).toBe('https://payments.example.test/purchase')
    expect(url.searchParams.get('campaign')).toBe('existing')
    expect(url.searchParams.get('user_id')).toBe('42')
    expect(url.searchParams.get('token')).toBe('test-session-token')
    expect(url.searchParams.get('lang')).toBe('zh')
    expect(url.searchParams.get('ui_mode')).toBe('embedded')
    expect(wrapper.get('a').attributes('href')).toBe(url.toString())
    wrapper.unmount()
  })

  it('honors the independent purchase switch even when built-in payment is enabled', async () => {
    settings.purchase_subscription_enabled = false
    settings.payment_enabled = true
    const wrapper = render()
    await flushPromises()
    expect(wrapper.find('iframe').exists()).toBe(false)
    expect(wrapper.text()).toContain('purchase.notEnabledTitle')
    wrapper.unmount()
  })
})
