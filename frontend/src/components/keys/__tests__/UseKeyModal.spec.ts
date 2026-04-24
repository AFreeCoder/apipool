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
  })
})
