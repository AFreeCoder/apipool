import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const {
  createAccountMock,
  importCodexSessionMock,
  createOpenAICodexPATMock,
  kiroGenerateAuthUrlMock,
  kiroExchangeCodeMock,
} = vi.hoisted(() => ({
  createAccountMock: vi.fn(),
  importCodexSessionMock: vi.fn(),
  createOpenAICodexPATMock: vi.fn(),
  kiroGenerateAuthUrlMock: vi.fn(),
  kiroExchangeCodeMock: vi.fn(),
}))

const oauthStubState = {
  authCode: '',
  oauthState: '',
  inputMethod: 'manual' as const,
}

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showWarning: vi.fn(),
    showInfo: vi.fn(),
  }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ isSimpleMode: true }),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      create: createAccountMock,
      checkMixedChannelRisk: vi.fn().mockResolvedValue({ has_risk: false }),
      importCodexSession: importCodexSessionMock,
      createOpenAICodexPAT: createOpenAICodexPATMock,
    },
    settings: {
      getWebSearchEmulationConfig: vi.fn().mockResolvedValue({ enabled: false, providers: [] }),
      getSettings: vi.fn().mockResolvedValue({ account_quota_notify_enabled: false }),
    },
    kiro: {
      generateAuthUrl: kiroGenerateAuthUrlMock,
      exchangeCode: kiroExchangeCodeMock,
    },
    tlsFingerprintProfiles: {
      list: vi.fn().mockResolvedValue([]),
    },
  },
}))

vi.mock('@/api/admin/accounts', () => ({
  getAntigravityDefaultModelMapping: vi.fn().mockResolvedValue([]),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

import CreateAccountModal from '../CreateAccountModal.vue'

const BaseDialogStub = defineComponent({
  name: 'BaseDialog',
  props: { show: { type: Boolean, default: false } },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>',
})

const OAuthAuthorizationFlowStub = defineComponent({
  name: 'OAuthAuthorizationFlow',
  emits: [
    'generate-url',
    'cookie-auth',
    'validate-refresh-token',
    'validate-mobile-refresh-token',
    'validate-session-token',
    'import-codex-session',
    'import-codex-pat',
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
      codexSession: '',
      codexPAT: '',
      ssoCookie: '',
      get inputMethod() {
        return oauthStubState.inputMethod
      },
      reset: () => {
        oauthStubState.authCode = ''
        oauthStubState.oauthState = ''
        oauthStubState.inputMethod = 'manual'
      },
    })
  },
  template: `
    <div>
      <button data-testid="import-codex-session" @click="$emit('import-codex-session', 'session-json')">session</button>
      <button data-testid="import-codex-pat" @click="$emit('import-codex-pat', 'pat-token')">pat</button>
    </div>
  `,
})

function mountModal() {
  return mount(CreateAccountModal, {
    props: { show: true, proxies: [], groups: [] },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        OAuthAuthorizationFlow: OAuthAuthorizationFlowStub,
        ConfirmDialog: true,
        Select: true,
        Icon: true,
        PlatformIcon: true,
        ProxySelector: true,
        ProxyAdBanner: true,
        GroupSelector: true,
        ModelWhitelistSelector: true,
        QuotaLimitCard: true,
      },
    },
  })
}

async function selectButtonByText(wrapper: ReturnType<typeof mountModal>, text: string) {
  const button = wrapper.findAll('button').find((candidate) => candidate.text().includes(text))
  expect(button).toBeDefined()
  await button?.trigger('click')
}

async function submitApiKeyAccount(platform: 'openai' | 'anthropic', enableLongContextBilling = false) {
  const wrapper = mountModal()
  await selectButtonByText(wrapper, platform === 'openai' ? 'OpenAI' : 'admin.accounts.claudeConsole')
  if (platform === 'openai') {
    await selectButtonByText(wrapper, 'API Key')
  }
  await wrapper.get('form#create-account-form input[type="text"]').setValue(`${platform} account`)
  await wrapper.get('form#create-account-form input[type="password"]').setValue('test-api-key')
  if (enableLongContextBilling) {
    await wrapper.get('[data-testid="openai-long-context-billing-toggle"]').trigger('click')
  }
  await wrapper.get('form#create-account-form').trigger('submit.prevent')
  await flushPromises()
}

async function openCodexImportStep(toggleClicks = 0) {
  const wrapper = mountModal()
  await selectButtonByText(wrapper, 'OpenAI')
  for (let click = 0; click < toggleClicks; click += 1) {
    await wrapper.get('[data-testid="openai-long-context-billing-toggle"]').trigger('click')
  }
  await wrapper.get('form#create-account-form input[type="text"]').setValue('Codex import')
  await wrapper.get('form#create-account-form').trigger('submit.prevent')
  return wrapper
}

describe('CreateAccountModal OpenAI long-context billing', () => {
  beforeEach(() => {
    createAccountMock.mockReset().mockResolvedValue({})
    importCodexSessionMock.mockReset().mockResolvedValue({
      created: 1,
      updated: 0,
      skipped: 0,
      failed: 0,
      errors: [],
      warnings: [],
    })
    createOpenAICodexPATMock.mockReset().mockResolvedValue({})
  })

  it('sends false explicitly for normal OpenAI account creation by default', async () => {
    await submitApiKeyAccount('openai')

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock.mock.calls[0]?.[0]?.extra?.openai_long_context_billing_enabled).toBe(false)
  })

  it('sends true explicitly when OpenAI long-context billing is enabled', async () => {
    await submitApiKeyAccount('openai', true)

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock.mock.calls[0]?.[0]?.extra?.openai_long_context_billing_enabled).toBe(true)
  })

  it('omits the OpenAI setting for non-OpenAI account creation', async () => {
    await submitApiKeyAccount('anthropic')

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock.mock.calls[0]?.[0]?.extra?.openai_long_context_billing_enabled).toBeUndefined()
  })

  it('leaves Codex session import billing ownership to the backend', async () => {
    const wrapper = await openCodexImportStep()
    await wrapper.get('[data-testid="import-codex-session"]').trigger('click')
    await flushPromises()

    expect(importCodexSessionMock).toHaveBeenCalledTimes(1)
    expect(importCodexSessionMock.mock.calls[0]?.[0]?.extra?.openai_long_context_billing_enabled).toBeUndefined()
  })

  it('leaves Codex PAT import billing ownership to the backend', async () => {
    const wrapper = await openCodexImportStep()
    await wrapper.get('[data-testid="import-codex-pat"]').trigger('click')
    await flushPromises()

    expect(createOpenAICodexPATMock).toHaveBeenCalledTimes(1)
    expect(createOpenAICodexPATMock.mock.calls[0]?.[0]?.extra?.openai_long_context_billing_enabled).toBeUndefined()
  })

  it('sends explicit true for Codex session import after the toggle is enabled', async () => {
    const wrapper = await openCodexImportStep(1)
    await wrapper.get('[data-testid="import-codex-session"]').trigger('click')
    await flushPromises()

    expect(importCodexSessionMock.mock.calls[0]?.[0]?.extra?.openai_long_context_billing_enabled).toBe(true)
  })

  it('sends explicit false for Codex session import after the toggle is changed back', async () => {
    const wrapper = await openCodexImportStep(2)
    await wrapper.get('[data-testid="import-codex-session"]').trigger('click')
    await flushPromises()

    expect(importCodexSessionMock.mock.calls[0]?.[0]?.extra?.openai_long_context_billing_enabled).toBe(false)
  })

  it('sends explicit true for Codex PAT import after the toggle is enabled', async () => {
    const wrapper = await openCodexImportStep(1)
    await wrapper.get('[data-testid="import-codex-pat"]').trigger('click')
    await flushPromises()

    expect(createOpenAICodexPATMock.mock.calls[0]?.[0]?.extra?.openai_long_context_billing_enabled).toBe(true)
  })

  it('sends explicit false for Codex PAT import after the toggle is changed back', async () => {
    const wrapper = await openCodexImportStep(2)
    await wrapper.get('[data-testid="import-codex-pat"]').trigger('click')
    await flushPromises()

    expect(createOpenAICodexPATMock.mock.calls[0]?.[0]?.extra?.openai_long_context_billing_enabled).toBe(false)
  })
})

describe('CreateAccountModal Kiro', () => {
  beforeEach(() => {
    createAccountMock.mockReset().mockResolvedValue({})
    kiroGenerateAuthUrlMock.mockReset()
    kiroExchangeCodeMock.mockReset()
    oauthStubState.authCode = ''
    oauthStubState.oauthState = ''
    oauthStubState.inputMethod = 'manual'
  })

  it('submits anthropic kiro idc credentials', async () => {
    const wrapper = mountModal()

    await wrapper.get('[data-testid="account-category-kiro"]').trigger('click')
    await wrapper.get('#account-name').setValue('Kiro IDC')
    await wrapper.get('[data-testid="kiro-auth-method-idc"]').setValue(true)
    await wrapper.get('#kiro-refresh-token').setValue('rt-123')
    await wrapper.get('#kiro-client-id').setValue('client-1')
    await wrapper.get('#kiro-client-secret').setValue('secret-1')
    await wrapper.get('#kiro-auth-region').setValue('us-east-1')
    await wrapper.get('#kiro-api-region').setValue('us-west-2')
    await wrapper.get('#kiro-pool-mode-toggle').trigger('click')
    await wrapper.get('#kiro-pool-mode-retry-status-codes').setValue('502, 503 529')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(createAccountMock).toHaveBeenCalledWith(expect.objectContaining({
      platform: 'anthropic',
      type: 'kiro',
      credentials: expect.objectContaining({
        auth_method: 'idc',
        refresh_token: 'rt-123',
        client_id: 'client-1',
        client_secret: 'secret-1',
        auth_region: 'us-east-1',
        api_region: 'us-west-2',
        pool_mode: true,
        pool_mode_retry_count: 3,
        pool_mode_retry_status_codes: [502, 503, 529],
      }),
    }))
  })

  it('creates anthropic kiro social account through browser oauth flow', async () => {
    oauthStubState.authCode = 'auth-code-1'
    oauthStubState.oauthState = 'state-1'
    kiroGenerateAuthUrlMock.mockResolvedValue({
      auth_url: 'https://prod.us-east-1.auth.desktop.kiro.dev/login?idp=google',
      session_id: 'session-1',
      state: 'state-1',
      machine_id: 'abcd'.repeat(16),
      redirect_uri: 'http://localhost:49153/callback',
      provider: 'google',
      auth_region: 'us-east-1',
      api_region: 'us-west-2',
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
      machine_id: 'abcd'.repeat(16),
    })

    const wrapper = mountModal()
    await wrapper.get('[data-testid="account-category-kiro"]').trigger('click')
    await wrapper.get('#account-name').setValue('Kiro Social OAuth')
    await wrapper.get('[data-testid="kiro-social-input-mode-oauth"]').setValue(true)
    await wrapper.get('#kiro-social-provider').setValue('google')
    await wrapper.get('#kiro-auth-region').setValue('us-east-1')
    await wrapper.get('#kiro-api-region').setValue('us-west-2')
    await wrapper.get('#kiro-pool-mode-toggle').trigger('click')
    await wrapper.get('#kiro-pool-mode-retry-status-codes').setValue('429, 503')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    const oauthFlow = wrapper.findComponent(OAuthAuthorizationFlowStub)
    expect(oauthFlow.exists()).toBe(true)
    oauthFlow.vm.$emit('generate-url')
    await flushPromises()

    const completeButton = wrapper.findAll('button').find((button) =>
      button.text().includes('admin.accounts.oauth.completeAuth')
    )
    expect(completeButton).toBeTruthy()
    await completeButton!.trigger('click')
    await flushPromises()

    expect(kiroGenerateAuthUrlMock).toHaveBeenCalledWith(expect.objectContaining({
      provider: 'google',
      auth_region: 'us-east-1',
      api_region: 'us-west-2',
    }))
    expect(kiroExchangeCodeMock).toHaveBeenCalledWith(expect.objectContaining({
      session_id: 'session-1',
      state: 'state-1',
      code: 'auth-code-1',
    }))
    expect(createAccountMock).toHaveBeenCalledWith(expect.objectContaining({
      platform: 'anthropic',
      type: 'kiro',
      credentials: expect.objectContaining({
        auth_method: 'social',
        access_token: 'at-1',
        refresh_token: 'rt-1',
        machine_id: 'abcd'.repeat(16),
        auth_region: 'us-east-1',
        api_region: 'us-west-2',
        profile_arn: 'arn:aws:kiro:::profile/default',
        pool_mode: true,
        pool_mode_retry_count: 3,
        pool_mode_retry_status_codes: [429, 503],
      }),
    }))
  })
})
