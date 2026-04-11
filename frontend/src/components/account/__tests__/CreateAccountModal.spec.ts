import { describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent } from 'vue'

const { createMock } = vi.hoisted(() => ({
  createMock: vi.fn()
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showInfo: vi.fn()
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    isSimpleMode: true
  })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      create: createMock,
      checkMixedChannelRisk: vi.fn().mockResolvedValue({ has_risk: false })
    },
    tlsFingerprintProfiles: {
      list: vi.fn().mockResolvedValue([])
    }
  }
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

import CreateAccountModal from '../CreateAccountModal.vue'

const BaseDialogStub = defineComponent({
  name: 'BaseDialog',
  props: { show: { type: Boolean, default: false } },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>'
})

describe('CreateAccountModal', () => {
  it('submits anthropic kiro idc credentials', async () => {
    createMock.mockResolvedValue({ id: 1 })

    const wrapper = mount(CreateAccountModal, {
      props: { show: true, proxies: [], groups: [] },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          ConfirmDialog: true,
          Icon: true,
          Select: true,
          ProxySelector: true,
          GroupSelector: true,
          ModelWhitelistSelector: true,
          QuotaLimitCard: true,
          OAuthAuthorizationFlow: true
        }
      }
    })

    await wrapper.get('[data-testid="account-category-kiro"]').trigger('click')
    await wrapper.get('#account-name').setValue('Kiro IDC')
    await wrapper.get('[data-testid="kiro-auth-method-idc"]').setValue(true)
    await wrapper.get('#kiro-refresh-token').setValue('rt-123')
    await wrapper.get('#kiro-client-id').setValue('client-1')
    await wrapper.get('#kiro-client-secret').setValue('secret-1')
    await wrapper.get('#kiro-auth-region').setValue('us-east-1')
    await wrapper.get('#kiro-api-region').setValue('us-west-2')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(createMock).toHaveBeenCalledWith(expect.objectContaining({
      platform: 'anthropic',
      type: 'kiro',
      credentials: expect.objectContaining({
        auth_method: 'idc',
        refresh_token: 'rt-123',
        client_id: 'client-1',
        client_secret: 'secret-1',
        auth_region: 'us-east-1',
        api_region: 'us-west-2'
      })
    }))
  })
})
