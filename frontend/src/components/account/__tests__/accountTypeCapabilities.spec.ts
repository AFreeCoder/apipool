import { describe, expect, it } from 'vitest'
import { supportsCustomErrorCodes, supportsPoolMode, supportsQuotaLimit } from '../accountTypeCapabilities'

describe('accountTypeCapabilities', () => {
  it('kiro supports quota and pool mode but not custom error codes', () => {
    expect(supportsQuotaLimit('kiro')).toBe(true)
    expect(supportsPoolMode('kiro')).toBe(true)
    expect(supportsCustomErrorCodes('kiro')).toBe(false)
  })
})
