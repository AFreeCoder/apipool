import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
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

const copyToClipboard = vi.fn()

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

const mountUseKeyModal = (platform: 'anthropic' | 'sora') => mount(UseKeyModal, {
  props: {
    show: true,
    apiKey: 'sk-test',
    baseUrl: 'https://apipool.dev',
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
    expect(wrapper.text()).toContain('apipool-anthropic/claude-sonnet-4-6')
  })

  it('sora 分组不会显示 OpenClaw tab', () => {
    const wrapper = mountUseKeyModal('sora')

    expect(wrapper.text()).not.toContain('keys.useKeyModal.cliTabs.openclaw')
  })
})
