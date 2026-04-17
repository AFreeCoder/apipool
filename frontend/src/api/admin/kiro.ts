import { apiClient } from '../client'

export interface KiroAuthUrlRequest {
  proxy_id?: number
  provider: 'google' | 'github'
  auth_region?: string
  api_region?: string
}

export interface KiroAuthUrlResponse {
  auth_url: string
  session_id: string
  state: string
  machine_id: string
  redirect_uri: string
  provider: string
  auth_region: string
  api_region: string
}

export interface KiroExchangeCodeRequest {
  session_id: string
  state: string
  code: string
  proxy_id?: number
}

export interface KiroTokenInfo {
  access_token?: string
  refresh_token?: string
  expires_at?: number | string
  expires_in?: number
  profile_arn?: string
  auth_method?: string
  provider?: string
  auth_region?: string
  api_region?: string
  machine_id?: string
  [key: string]: unknown
}

export async function generateAuthUrl(
  payload: KiroAuthUrlRequest
): Promise<KiroAuthUrlResponse> {
  const { data } = await apiClient.post<KiroAuthUrlResponse>('/admin/kiro/oauth/auth-url', payload)
  return data
}

export async function exchangeCode(
  payload: KiroExchangeCodeRequest
): Promise<KiroTokenInfo> {
  const { data } = await apiClient.post<KiroTokenInfo>('/admin/kiro/oauth/exchange-code', payload)
  return data
}

export default { generateAuthUrl, exchangeCode }
