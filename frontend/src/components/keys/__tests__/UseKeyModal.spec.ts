import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, nextTick } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import UseKeyModal from '../UseKeyModal.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const copyToClipboard = vi.fn().mockResolvedValue(true)

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard
  })
}))

const BaseDialogStub = defineComponent({
  name: 'BaseDialog',
  props: {
    show: { type: Boolean, default: false },
    title: { type: String, default: '' },
    width: { type: String, default: '' }
  },
  template: `
    <div v-if="show" class="base-dialog-stub">
      <slot />
      <slot name="footer" />
    </div>
  `
})

const IconStub = defineComponent({
  name: 'Icon',
  template: '<span class="icon-stub" />'
})

const mountUseKeyModal = (
  platform: 'anthropic' | 'openai',
  baseUrl = 'https://apipool.dev'
) => mount(UseKeyModal, {
  props: {
    show: true,
    apiKey: 'sk-test',
    baseUrl,
    platform
  },
  global: {
    stubs: {
      BaseDialog: BaseDialogStub,
      Icon: IconStub
    }
  }
})

const findButtonByText = (wrapper: ReturnType<typeof mount>, text: string) =>
  wrapper.findAll('button').find((button) => button.text().includes(text))

describe('UseKeyModal', () => {
  beforeEach(() => {
    copyToClipboard.mockReset()
    copyToClipboard.mockResolvedValue(true)
  })

  it('renders Grok Build and OpenCode setup for Grok groups', async () => {
    const wrapper = mount(UseKeyModal, {
      props: {
        show: true,
        apiKey: 'sk-grok-test',
        baseUrl: 'https://example.com/v1',
        platform: 'grok'
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /><slot name="footer" /></div>'
          },
          Icon: {
            template: '<span />'
          }
        }
      }
    })

    const grokTab = wrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.cliTabs.grokCli')
    )
    expect(grokTab).toBeDefined()

    const grokConfig = wrapper.findAll('pre code')
      .map((code) => code.text())
      .find((content) => content.includes('[model."sub2api-grok"]'))
    expect(grokConfig).toBeDefined()
    expect(grokConfig).toContain('model = "grok-4.5"')
    expect(grokConfig).toContain('base_url = "https://example.com/v1"')
    expect(grokConfig).toContain('api_key = "sk-grok-test"')
    expect(grokConfig).toContain('api_backend = "responses"')

    const windowsTab = wrapper.findAll('button').find(
      (button) => button.text().trim() === 'Windows'
    )
    expect(windowsTab).toBeDefined()
    await windowsTab!.trigger('click')
    await nextTick()
    expect(wrapper.text()).toContain('%userprofile%\\.grok/config.toml')

    const opencodeTab = wrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.cliTabs.opencode')
    )
    expect(opencodeTab).toBeDefined()
    await opencodeTab!.trigger('click')
    await nextTick()

    const parsed = JSON.parse(wrapper.find('pre code').text())
    expect(parsed.provider.grok.npm).toBe('@ai-sdk/openai')
    expect(parsed.provider.grok.options).toEqual({
      baseURL: 'https://example.com/v1',
      apiKey: 'sk-grok-test'
    })
    expect(parsed.provider.grok.models['grok-4.5']).toBeDefined()
    expect(parsed.provider.grok.models['grok-build-0.1']).toBeDefined()
    expect(parsed.provider.grok.models['grok-composer-2.5-fast']).toBeDefined()
    expect(parsed.provider.grok.models['gpt-5.6']).toBeUndefined()
  })

  it('anthropic 分组会显示 OpenClaw tab，并展示对应的 OpenClaw provider 信息', async () => {
    const wrapper = mountUseKeyModal('anthropic')

    expect(wrapper.text()).toContain('keys.useKeyModal.cliTabs.claudeCode')
    expect(wrapper.text()).toContain('keys.useKeyModal.cliTabs.opencode')
    expect(wrapper.text()).toContain('keys.useKeyModal.cliTabs.openclaw')

    const openClawTab = findButtonByText(wrapper, 'keys.useKeyModal.cliTabs.openclaw')
    expect(openClawTab).toBeTruthy()

    await openClawTab!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('keys.useKeyModal.openclaw.importDescription')
    expect(wrapper.text()).toContain('apipool-anthropic')
    expect(wrapper.text()).toContain('apipool-anthropic/claude-opus-4-6')

    const fileInput = wrapper.get('input[type="file"]')
    const configFile = new File(['{}'], 'openclaw.json', { type: 'application/json' })
    Object.defineProperty(configFile, 'text', {
      value: vi.fn().mockResolvedValue('{}'),
      configurable: true
    })
    Object.defineProperty(fileInput.element, 'files', {
      value: [configFile],
      configurable: true
    })
    await fileInput.trigger('change')
    await flushPromises()

    const generatedConfig = wrapper.get('pre code').text()
    expect(generatedConfig).toContain('"thinking": "high"')
    expect(generatedConfig).toContain('"primary": "apipool-anthropic/claude-opus-4-6"')
  })

  it('OpenCode 配置会展示 GPT-5.4 Mini，且不再展示 Nano', async () => {
    const wrapper = mountUseKeyModal('openai', 'https://example.com/v1')

    const opencodeTab = findButtonByText(wrapper, 'keys.useKeyModal.cliTabs.opencode')
    expect(opencodeTab).toBeTruthy()

    await opencodeTab!.trigger('click')
    await nextTick()

    const codeBlock = wrapper.find('pre code')
    expect(codeBlock.exists()).toBe(true)
    expect(codeBlock.text()).toContain('"name": "GPT-5.4 Mini"')
    expect(codeBlock.text()).not.toContain('"name": "GPT-5.4 Nano"')
  })

  it('OpenAI Codex 默认配置使用 GPT-5.5', () => {
    const wrapper = mountUseKeyModal('openai', 'https://example.com/v1')

    const codeBlocks = wrapper.findAll('pre code')
    expect(codeBlocks[0].text()).toContain('model = "gpt-5.5"')
    expect(codeBlocks[0].text()).toContain('review_model = "gpt-5.5"')
    expect(codeBlocks[0].text()).toContain('model_provider = "apipool"')
    expect(codeBlocks[0].text()).toContain('name = "apipool"')
    expect(codeBlocks[0].text()).not.toContain('model = "gpt-5.4"')
    expect(codeBlocks[0].text()).not.toContain('model_context_window')
    expect(codeBlocks[0].text()).not.toContain('model_auto_compact_token_limit')
    expect(codeBlocks[0].text()).toContain('[features]\ngoals = true')
  })

  it('OpenAI Codex WebSocket 默认配置使用 GPT-5.5', async () => {
    const wrapper = mountUseKeyModal('openai', 'https://example.com/v1')

    const codexWsTab = findButtonByText(wrapper, 'keys.useKeyModal.cliTabs.codexCliWs')
    expect(codexWsTab).toBeTruthy()

    await codexWsTab!.trigger('click')
    await nextTick()

    const codeBlocks = wrapper.findAll('pre code')
    expect(codeBlocks[0].text()).toContain('model = "gpt-5.5"')
    expect(codeBlocks[0].text()).toContain('review_model = "gpt-5.5"')
    expect(codeBlocks[0].text()).toContain('supports_websockets = true')
    expect(codeBlocks[0].text()).not.toContain('model = "gpt-5.4"')
    expect(codeBlocks[0].text()).not.toContain('model_context_window')
    expect(codeBlocks[0].text()).not.toContain('model_auto_compact_token_limit')
    expect(codeBlocks[0].text()).toContain('[features]\nresponses_websockets_v2 = true\ngoals = true')
  })

  it('renders GPT-5.6 alias and max variants in OpenCode config', async () => {
    const wrapper = mount(UseKeyModal, {
      props: {
        show: true,
        apiKey: 'sk-test',
        baseUrl: 'https://example.com/v1',
        platform: 'openai'
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /><slot name="footer" /></div>'
          },
          Icon: {
            template: '<span />'
          }
        }
      }
    })

    const opencodeTab = wrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.cliTabs.opencode')
    )
    expect(opencodeTab).toBeDefined()
    await opencodeTab!.trigger('click')
    await nextTick()

    const parsed = JSON.parse(wrapper.find('pre code').text())
    const models = parsed.provider.openai.models
    for (const model of ['gpt-5.6', 'gpt-5.6-sol', 'gpt-5.6-terra', 'gpt-5.6-luna']) {
      expect(models[model]).toBeDefined()
      expect(models[model].variants).toHaveProperty('max')
      expect(models[model].variants).toHaveProperty('xhigh')
    }
    expect(models['gpt-5.6'].name).toBe('GPT-5.6 (Sol)')
  })

  it('renders Claude Fable 5 OpenCode config with adaptive thinking', async () => {
    const wrapper = mount(UseKeyModal, {
      props: {
        show: true,
        apiKey: 'sk-test',
        baseUrl: 'https://example.com/v1',
        platform: 'antigravity'
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /><slot name="footer" /></div>'
          },
          Icon: {
            template: '<span />'
          }
        }
      }
    })

    const opencodeTab = wrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.cliTabs.opencode')
    )

    expect(opencodeTab).toBeDefined()
    await opencodeTab!.trigger('click')
    await nextTick()

    const claudeConfig = wrapper.findAll('pre code')
      .map((code) => code.text())
      .find((content) => content.includes('"antigravity-claude"'))

    expect(claudeConfig).toBeDefined()
    const parsed = JSON.parse(claudeConfig!)
    const fable = parsed.provider['antigravity-claude'].models['claude-fable-5']

    expect(fable.name).toBe('Claude Fable 5')
    expect(fable.limit).toEqual({ context: 1048576, output: 128000 })
    expect(fable.options.thinking).toEqual({ type: 'adaptive' })
    expect(fable.options.thinking).not.toHaveProperty('budgetTokens')
  })
})
