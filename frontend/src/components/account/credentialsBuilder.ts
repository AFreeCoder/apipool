export function applyInterceptWarmup(
  credentials: Record<string, unknown>,
  enabled: boolean,
  mode: 'create' | 'edit'
): void {
  if (enabled) {
    credentials.intercept_warmup_requests = true
  } else if (mode === 'edit') {
    delete credentials.intercept_warmup_requests
  }
}

export interface KiroCredentialInput {
  mode: 'create' | 'edit'
  authMethod: 'social' | 'idc'
  refreshToken: string
  authRegion: string
  apiRegion: string
  machineId?: string
  clientId?: string
  clientSecret?: string
  profileArn?: string
  currentCredentials?: Record<string, unknown>
}

export function buildKiroCredentials(input: KiroCredentialInput): Record<string, unknown> {
  const next: Record<string, unknown> = {
    ...(input.currentCredentials || {}),
    auth_method: input.authMethod,
    refresh_token: input.refreshToken.trim(),
    auth_region: input.authRegion.trim() || 'us-east-1',
    api_region: input.apiRegion.trim() || 'us-east-1'
  }

  const machineId = input.machineId?.trim()
  if (machineId) {
    next.machine_id = machineId
  } else if (input.mode === 'edit') {
    delete next.machine_id
  }

  const profileArn = input.profileArn?.trim()
  if (profileArn) {
    next.profile_arn = profileArn
  } else if (input.mode === 'edit') {
    delete next.profile_arn
  }

  if (input.authMethod === 'idc') {
    next.client_id = input.clientId?.trim() || ''
    next.client_secret = input.clientSecret?.trim() || ''
  } else if (input.mode === 'edit') {
    delete next.client_id
    delete next.client_secret
  }

  return next
}
