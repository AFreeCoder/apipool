import { describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { mount } from '@vue/test-utils'

const { updateAccountMock, checkMixedChannelRiskMock } = vi.hoisted(() => ({
  updateAccountMock: vi.fn(),
  checkMixedChannelRiskMock: vi.fn()
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
      update: updateAccountMock,
      checkMixedChannelRisk: checkMixedChannelRiskMock
    },
    settings: {
      getSettings: vi.fn().mockResolvedValue({
        account_quota_notify_enabled: false
      }),
      getWebSearchEmulationConfig: vi.fn().mockResolvedValue({
        enabled: false,
        providers: []
      })
    }
  }
}))

vi.mock('@/api/admin/accounts', () => ({
  getAntigravityDefaultModelMapping: vi.fn()
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

import EditAccountModal from '../EditAccountModal.vue'

const BaseDialogStub = defineComponent({
  name: 'BaseDialog',
  props: {
    show: {
      type: Boolean,
      default: false
    }
  },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>'
})

const ModelWhitelistSelectorStub = defineComponent({
  name: 'ModelWhitelistSelector',
  props: {
    modelValue: {
      type: Array,
      default: () => []
    }
  },
  emits: ['update:modelValue'],
  template: `
    <div>
      <button
        type="button"
        data-testid="rewrite-to-snapshot"
        @click="$emit('update:modelValue', ['gpt-5.2-2025-12-11'])"
      >
        rewrite
      </button>
      <span data-testid="model-whitelist-value">
        {{ Array.isArray(modelValue) ? modelValue.join(',') : '' }}
      </span>
    </div>
  `
})

function buildAccount() {
  return {
    id: 1,
    name: 'OpenAI Key',
    notes: '',
    platform: 'openai',
    type: 'apikey',
    credentials: {
      api_key: 'sk-test',
      base_url: 'https://api.openai.com',
      model_mapping: {
        'gpt-5.2': 'gpt-5.2'
      }
    },
    extra: {},
    proxy_id: null,
    concurrency: 1,
    priority: 1,
    rate_multiplier: 1,
    status: 'active',
    group_ids: [],
    expires_at: null,
    auto_pause_on_expired: false
  } as any
}

function mountModal(account = buildAccount()) {
  return mount(EditAccountModal, {
    props: {
      show: true,
      account,
      proxies: [],
      groups: []
    },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        Select: true,
        Icon: true,
        ProxySelector: true,
        GroupSelector: true,
        ModelWhitelistSelector: ModelWhitelistSelectorStub
      }
    }
  })
}

describe('EditAccountModal', () => {
  it('reopening the same account rehydrates the OpenAI whitelist from props', async () => {
    const account = buildAccount()
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    expect(wrapper.get('[data-testid="model-whitelist-value"]').text()).toBe('gpt-5.2')

    await wrapper.get('[data-testid="rewrite-to-snapshot"]').trigger('click')
    expect(wrapper.get('[data-testid="model-whitelist-value"]').text()).toBe('gpt-5.2-2025-12-11')

    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true })

    expect(wrapper.get('[data-testid="model-whitelist-value"]').text()).toBe('gpt-5.2')

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.model_mapping).toEqual({
      'gpt-5.2': 'gpt-5.2'
    })
  })

  it('submits kiro credentials, mapping, and pool mode updates', async () => {
    const account = {
      id: 2,
      name: 'Kiro',
      notes: '',
      platform: 'anthropic',
      type: 'kiro',
      credentials: {
        auth_method: 'social',
        refresh_token: 'rt-old',
        auth_region: 'us-east-1',
        api_region: 'us-east-1',
        machine_id: 'abcd'.repeat(16),
        model_mapping: {
          'claude-sonnet-4': 'claude-sonnet-4'
        }
      },
      extra: {
        quota_limit: 25
      },
      proxy_id: null,
      concurrency: 1,
      priority: 1,
      rate_multiplier: 1,
      status: 'active',
      group_ids: [],
      expires_at: null,
      auto_pause_on_expired: false
    } as any

    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    await wrapper.get('#edit-kiro-auth-method-idc').setValue(true)
    await wrapper.get('#edit-kiro-refresh-token').setValue('rt-new')
    await wrapper.get('#edit-kiro-client-id').setValue('client-id')
    await wrapper.get('#edit-kiro-client-secret').setValue('client-secret')
    await wrapper.get('#edit-kiro-auth-region').setValue('us-west-2')
    await wrapper.get('#edit-kiro-api-region').setValue('eu-west-1')
    await wrapper.get('#edit-kiro-machine-id').setValue('1234abcd1234abcd1234abcd1234abcd')
    await wrapper.get('#edit-kiro-profile-arn').setValue('arn:aws:iam::123456789012:role/Kiro')
    await wrapper.get('[data-testid="rewrite-to-snapshot"]').trigger('click')

    await wrapper.get('#edit-kiro-pool-mode-toggle').trigger('click')

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]).toMatchObject({
      credentials: {
        auth_method: 'idc',
        refresh_token: 'rt-new',
        client_id: 'client-id',
        client_secret: 'client-secret',
        auth_region: 'us-west-2',
        api_region: 'eu-west-1',
        machine_id: '1234abcd1234abcd1234abcd1234abcd',
        profile_arn: 'arn:aws:iam::123456789012:role/Kiro',
        model_mapping: {
          'gpt-5.2-2025-12-11': 'gpt-5.2-2025-12-11'
        },
        pool_mode: true
      },
      extra: {
        quota_limit: 25
      }
    })
  })
})
