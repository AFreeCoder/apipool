import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import en from '@/i18n/locales/en'
import zh from '@/i18n/locales/zh'

function flattenKeys(obj: Record<string, any>, prefix = ''): string[] {
  const keys: string[] = []
  for (const [k, v] of Object.entries(obj)) {
    const fullKey = prefix ? `${prefix}.${k}` : k
    if (typeof v === 'object' && v !== null && !Array.isArray(v)) {
      keys.push(...flattenKeys(v, fullKey))
    } else {
      keys.push(fullKey)
    }
  }
  return keys
}

describe('ops locale key completeness', () => {
  const requiredKeys = [
    'admin.ops.result',
    'admin.ops.timeRange.custom',
    'admin.ops.customTimeRange.startTime',
    'admin.ops.customTimeRange.endTime',
  ]

  for (const key of requiredKeys) {
    it(`en locale has ${key}`, () => {
      const enKeys = flattenKeys(en)
      expect(enKeys).toContain(key)
    })
  }
})

describe('request log locale key completeness', () => {
  const sources = [
    resolve(process.cwd(), 'src/components/admin/user/UserRequestLogDialog.vue'),
    resolve(process.cwd(), 'src/views/admin/UsersView.vue'),
  ]
  const requestLogKeys = new Set(
    sources.flatMap((source) => (
      readFileSync(source, 'utf8').match(/admin\.ops\.requestLog(?:\.[A-Za-z0-9_]+)+/g) ?? []
    ))
  )

  it('finds request log translation keys in their call sites', () => {
    expect(requestLogKeys.size).toBeGreaterThan(0)
  })

  for (const key of requestLogKeys) {
    it(`en and zh locales both have ${key}`, () => {
      expect(flattenKeys(en)).toContain(key)
      expect(flattenKeys(zh)).toContain(key)
    })
  }
})

describe('groups locale key completeness', () => {
  it('en locale has admin.groups.failedToSave', () => {
    const enKeys = flattenKeys(en)
    expect(enKeys).toContain('admin.groups.failedToSave')
  })

  const webSearchPricingKeys = [
    'admin.groups.webSearchPricing.title',
    'admin.groups.webSearchPricing.pricePerCall',
    'admin.groups.webSearchPricing.pricePerCallHint',
    'admin.groups.webSearchPricing.finalPricePreview',
  ]

  for (const key of webSearchPricingKeys) {
    it(`en and zh locales both have ${key}`, () => {
      expect(flattenKeys(en)).toContain(key)
      expect(flattenKeys(zh)).toContain(key)
    })
  }
})
