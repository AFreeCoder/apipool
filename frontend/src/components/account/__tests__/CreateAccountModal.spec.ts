import { describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent, h } from 'vue'

const { createMock, kiroGenerateAuthUrlMock, kiroExchangeCodeMock } = vi.hoisted(() => ({
  createMock: vi.fn(),
  kiroGenerateAuthUrlMock: vi.fn(),
  kiroExchangeCodeMock: vi.fn()
}))

const oauthStubState = {
  authCode: '',
  oauthState: '',
  inputMethod: 'manual' as const
}

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
    settings: {
      getSettings: vi.fn().mockResolvedValue({
        account_quota_notify_enabled: false
      }),
      getWebSearchEmulationConfig: vi.fn().mockResolvedValue({
        enabled: false,
        providers: []
      })
    },
    kiro: {
      generateAuthUrl: kiroGenerateAuthUrlMock,
      exchangeCode: kiroExchangeCodeMock
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

const OAuthAuthorizationFlowStub = defineComponent({
  name: 'OAuthAuthorizationFlow',
  emits: [
    'generate-url',
    'cookie-auth',
    'validate-refresh-token',
    'validate-mobile-refresh-token',
    'validate-session-token'
  ],
  setup(_props, { expose }) {
    expose({
      get authCode() {
        return oauthStubState.authCode
      },
      get oauthState() {
        return oauthStubState.oauthState
      },
      projectId: '',
      sessionKey: '',
      refreshToken: '',
      sessionToken: '',
      get inputMethod() {
        return oauthStubState.inputMethod
      },
      reset: () => {
        oauthStubState.authCode = ''
        oauthStubState.oauthState = ''
        oauthStubState.inputMethod = 'manual'
      }
    })

    return () => h('div', { 'data-testid': 'oauth-flow-stub' })
  }
})

describe('CreateAccountModal', () => {
  it('submits anthropic kiro idc credentials', async () => {
    oauthStubState.authCode = ''
    oauthStubState.oauthState = ''
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
          OAuthAuthorizationFlow: OAuthAuthorizationFlowStub
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

  it('creates anthropic kiro social account through browser oauth flow', async () => {
    oauthStubState.authCode = 'auth-code-1'
    oauthStubState.oauthState = 'state-1'
    createMock.mockResolvedValue({ id: 2 })
    kiroGenerateAuthUrlMock.mockResolvedValue({
      auth_url: 'https://prod.us-east-1.auth.desktop.kiro.dev/login?idp=google',
      session_id: 'session-1',
      state: 'state-1',
      machine_id: 'abcd'.repeat(16),
      redirect_uri: 'http://localhost:49153/callback',
      provider: 'google',
      auth_region: 'us-east-1',
      api_region: 'us-west-2'
    })
    kiroExchangeCodeMock.mockResolvedValue({
      access_token: 'at-1',
      refresh_token: 'rt-1',
      expires_at: 1735689600,
      expires_in: 3600,
      profile_arn: 'arn:aws:kiro:::profile/default',
      auth_method: 'social',
      provider: 'google',
      auth_region: 'us-east-1',
      api_region: 'us-west-2',
      machine_id: 'abcd'.repeat(16)
    })

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
          OAuthAuthorizationFlow: OAuthAuthorizationFlowStub
        }
      }
    })

    await wrapper.get('[data-testid="account-category-kiro"]').trigger('click')
    await wrapper.get('#account-name').setValue('Kiro Social OAuth')
    await wrapper.get('[data-testid="kiro-social-input-mode-oauth"]').setValue(true)
    await wrapper.get('#kiro-social-provider').setValue('google')
    await wrapper.get('#kiro-auth-region').setValue('us-east-1')
    await wrapper.get('#kiro-api-region').setValue('us-west-2')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    const oauthFlow = wrapper.findComponent(OAuthAuthorizationFlowStub)
    expect(oauthFlow.exists()).toBe(true)
    oauthFlow.vm.$emit('generate-url')
    await flushPromises()

    const footerButtons = wrapper.findAll('button')
    const completeButton = footerButtons.find((button) =>
      button.text().includes('admin.accounts.oauth.completeAuth')
    )
    expect(completeButton).toBeTruthy()
    await completeButton!.trigger('click')
    await flushPromises()

    expect(kiroGenerateAuthUrlMock).toHaveBeenCalledWith(expect.objectContaining({
      provider: 'google',
      auth_region: 'us-east-1',
      api_region: 'us-west-2'
    }))
    expect(kiroExchangeCodeMock).toHaveBeenCalledWith(expect.objectContaining({
      session_id: 'session-1',
      state: 'state-1',
      code: 'auth-code-1'
    }))
    expect(createMock).toHaveBeenCalledWith(expect.objectContaining({
      platform: 'anthropic',
      type: 'kiro',
      credentials: expect.objectContaining({
        auth_method: 'social',
        access_token: 'at-1',
        refresh_token: 'rt-1',
        machine_id: 'abcd'.repeat(16),
        auth_region: 'us-east-1',
        api_region: 'us-west-2',
        profile_arn: 'arn:aws:kiro:::profile/default'
      })
    }))
  })
})
