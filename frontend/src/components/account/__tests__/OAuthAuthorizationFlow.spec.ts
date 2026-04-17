import { unref } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copied: false,
    copyToClipboard: vi.fn()
  })
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

import OAuthAuthorizationFlow from '../OAuthAuthorizationFlow.vue'

describe('OAuthAuthorizationFlow', () => {
  it('extracts code and state from kiro callback url', async () => {
    const wrapper = mount(OAuthAuthorizationFlow, {
      props: {
        addMethod: 'oauth',
        authUrl: 'https://example.com',
        sessionId: 'session-1',
        platform: 'kiro' as any,
        showCookieOption: false
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    const textareas = wrapper.findAll('textarea')
    expect(textareas.length).toBeGreaterThan(0)
    await textareas[textareas.length - 1].setValue('http://localhost:49153/callback?code=auth-code-1&state=state-1')

    const exposed = (wrapper.vm as any).$
      ?.exposed as { authCode: string; oauthState: string } | undefined

    expect(unref(exposed?.authCode as any)).toBe('auth-code-1')
    expect(unref(exposed?.oauthState as any)).toBe('state-1')
  })
})
