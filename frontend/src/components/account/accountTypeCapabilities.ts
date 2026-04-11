import type { AccountType } from '@/types'

export function supportsQuotaLimit(type: AccountType): boolean {
  return type === 'apikey' || type === 'bedrock' || type === 'kiro'
}

export function supportsPoolMode(type: AccountType): boolean {
  return type === 'apikey' || type === 'bedrock' || type === 'kiro'
}

export function supportsCustomErrorCodes(type: AccountType): boolean {
  return type === 'apikey'
}
