import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import { buildKiroCredentials } from '@/components/account/credentialsBuilder'
import type { KiroTokenInfo } from '@/api/admin/kiro'

export function useKiroOAuth() {
  const appStore = useAppStore()
  const { t } = useI18n()

  const authUrl = ref('')
  const sessionId = ref('')
  const state = ref('')
  const loading = ref(false)
  const error = ref('')

  const resetState = () => {
    authUrl.value = ''
    sessionId.value = ''
    state.value = ''
    loading.value = false
    error.value = ''
  }

  const generateAuthUrl = async (
    proxyId: number | null | undefined,
    provider: 'google' | 'github',
    authRegion?: string,
    apiRegion?: string
  ): Promise<boolean> => {
    loading.value = true
    authUrl.value = ''
    sessionId.value = ''
    state.value = ''
    error.value = ''

    try {
      const payload: Record<string, unknown> = { provider }
      if (proxyId) payload.proxy_id = proxyId
      if (authRegion?.trim()) payload.auth_region = authRegion.trim()
      if (apiRegion?.trim()) payload.api_region = apiRegion.trim()

      const response = await adminAPI.kiro.generateAuthUrl(payload as any)
      authUrl.value = response.auth_url
      sessionId.value = response.session_id
      state.value = response.state
      return true
    } catch (err: any) {
      error.value = err.response?.data?.detail || t('admin.accounts.oauth.kiro.failedToGenerateUrl')
      appStore.showError(error.value)
      return false
    } finally {
      loading.value = false
    }
  }

  const exchangeAuthCode = async (params: {
    code: string
    sessionId: string
    state: string
    proxyId?: number | null
  }): Promise<KiroTokenInfo | null> => {
    const code = params.code?.trim()
    if (!code || !params.sessionId || !params.state) {
      error.value = t('admin.accounts.oauth.kiro.missingExchangeParams')
      return null
    }

    loading.value = true
    error.value = ''

    try {
      const payload: Record<string, unknown> = {
        session_id: params.sessionId,
        state: params.state,
        code
      }
      if (params.proxyId) payload.proxy_id = params.proxyId

      return await adminAPI.kiro.exchangeCode(payload as any)
    } catch (err: any) {
      error.value = err.response?.data?.detail || t('admin.accounts.oauth.kiro.failedToExchangeCode')
      appStore.showError(error.value)
      return null
    } finally {
      loading.value = false
    }
  }

  const buildCredentials = (tokenInfo: KiroTokenInfo): Record<string, unknown> => {
    return buildKiroCredentials({
      mode: 'create',
      authMethod: 'social',
      refreshToken: String(tokenInfo.refresh_token || ''),
      authRegion: String(tokenInfo.auth_region || ''),
      apiRegion: String(tokenInfo.api_region || ''),
      machineId: typeof tokenInfo.machine_id === 'string' ? tokenInfo.machine_id : '',
      profileArn: typeof tokenInfo.profile_arn === 'string' ? tokenInfo.profile_arn : '',
      accessToken: typeof tokenInfo.access_token === 'string' ? tokenInfo.access_token : '',
      expiresAt: tokenInfo.expires_at
    })
  }

  return {
    authUrl,
    sessionId,
    state,
    loading,
    error,
    resetState,
    generateAuthUrl,
    exchangeAuthCode,
    buildCredentials
  }
}
