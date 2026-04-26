import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'
import AppHeaderMarquee from '../AppHeaderMarquee.vue'
import { useAppStore } from '@/stores'
import type { MarqueeMessage, PublicSettings } from '@/types'

let pinia: ReturnType<typeof createPinia>

function seedMarqueeSettings(
  enabled: boolean,
  messages: MarqueeMessage[],
): void {
  const appStore = useAppStore()
  appStore.cachedPublicSettings = {
    marquee_enabled: enabled,
    marquee_messages: messages,
  } as PublicSettings
}

function mountMarquee() {
  return mount(AppHeaderMarquee, {
    global: {
      plugins: [pinia],
    },
  })
}

describe('AppHeaderMarquee', () => {
  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
  })

  it('renders enabled messages as one separated marquee string', () => {
    seedMarqueeSettings(true, [
      { id: 'a', text: '  First promo  ', enabled: true, sort_order: 0 },
      { id: 'b', text: 'Hidden promo', enabled: false, sort_order: 1 },
      { id: 'c', text: '   ', enabled: true, sort_order: 2 },
      { id: 'd', text: 'Second promo', enabled: true, sort_order: 3 },
    ])

    const wrapper = mountMarquee()

    const slot = wrapper.get('.marquee-slot')
    expect(slot.classes()).toEqual(expect.arrayContaining(['hidden', 'min-w-0', 'mx-4', 'lg:block', 'lg:flex-1']))

    const shell = wrapper.get('.marquee-shell')
    expect(shell.attributes('role')).toBe('marquee')
    expect(shell.attributes('aria-label')).toBe('First promo  ·  Second promo')

    const segments = wrapper.findAll('.marquee-segment')
    expect(segments).toHaveLength(2)
    expect(segments.map((segment) => segment.text())).toEqual([
      'First promo  ·  Second promo',
      'First promo  ·  Second promo',
    ])
    expect(segments[1].attributes('aria-hidden')).toBe('true')
  })

  it('keeps the hidden desktop slot but hides content when disabled', () => {
    seedMarqueeSettings(false, [
      { id: 'a', text: 'Hidden promo', enabled: true, sort_order: 0 },
    ])

    const wrapper = mountMarquee()

    expect(wrapper.find('.marquee-slot').exists()).toBe(true)
    expect(wrapper.find('.marquee-slot').classes()).toEqual(expect.arrayContaining(['hidden', 'lg:block']))
    expect(wrapper.find('.marquee-shell').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('Hidden promo')
  })

  it('pauses both marquee segments on hover', async () => {
    seedMarqueeSettings(true, [
      { id: 'a', text: 'Promo', enabled: true, sort_order: 0 },
    ])
    const wrapper = mountMarquee()

    await wrapper.get('.marquee-track').trigger('mouseenter')

    expect(wrapper.findAll('.marquee-segment').every((segment) => segment.classes().includes('paused'))).toBe(true)

    await wrapper.get('.marquee-track').trigger('mouseleave')

    expect(wrapper.findAll('.marquee-segment').some((segment) => segment.classes().includes('paused'))).toBe(false)
  })
})
