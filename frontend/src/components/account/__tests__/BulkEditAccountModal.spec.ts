import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import BulkEditAccountModal from '../BulkEditAccountModal.vue'
import ModelWhitelistSelector from '../ModelWhitelistSelector.vue'

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showInfo: vi.fn()
  })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      bulkUpdate: vi.fn(),
      checkMixedChannelRisk: vi.fn()
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

function mountModal() {
  return mount(BulkEditAccountModal, {
    props: {
      show: true,
      accountIds: [1, 2],
      selectedPlatforms: ['antigravity'],
      selectedTypes: ['apikey'],
      proxies: [],
      groups: []
    } as any,
    global: {
      stubs: {
        BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
        Select: true,
        ProxySelector: true,
        GroupSelector: true,
        Icon: true
      }
    }
  })
}

function mountOpenAIModal() {
  return mount(BulkEditAccountModal, {
    props: {
      show: true,
      accountIds: [1],
      selectedPlatforms: ['openai'],
      selectedTypes: ['oauth'],
      proxies: [],
      groups: []
    } as any,
    global: {
      stubs: {
        BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
        Select: true,
        ProxySelector: true,
        GroupSelector: true,
        Icon: true
      }
    }
  })
}

describe('BulkEditAccountModal', () => {
  it('antigravity 白名单仅保留官方支持模型，并过滤普通 GPT 模型', async () => {
    const wrapper = mountModal()
    const selector = wrapper.findComponent(ModelWhitelistSelector)

    await selector.find('.cursor-pointer').trigger('click')

    expect(selector.text()).toContain('gemini-3.1-flash-image')
    expect(selector.text()).not.toContain('gemini-3-pro-image')
    expect(selector.text()).not.toContain('gpt-5.3-codex')
  })

  it('antigravity 映射预设保留 legacy 图片模型映射并过滤 OpenAI 预设', async () => {
    const wrapper = mountModal()

    const mappingTab = wrapper.findAll('button').find((btn) => btn.text().includes('admin.accounts.modelMapping'))
    expect(mappingTab).toBeTruthy()
    await mappingTab!.trigger('click')

    expect(wrapper.text()).toContain('3.1-Flash-Image透传')
    expect(wrapper.text()).toContain('3-Pro-Image→3.1')
    expect(wrapper.text()).not.toContain('GPT-5.3 Codex')
  })

  it('openai 白名单使用 GPT-5.2 别名而不是日期版快照 ID', async () => {
    const wrapper = mountOpenAIModal()
    const selector = wrapper.findComponent(ModelWhitelistSelector)

    await selector.find('.cursor-pointer').trigger('click')

    expect(selector.text()).toContain('gpt-5.2')
    expect(selector.text()).not.toContain('gpt-5.2-2025-12-11')
  })
})
