import JSON5 from 'json5'
import {
  OPENAI_DEFAULT_MODEL_CONTEXT_WINDOW,
  OPENAI_DEFAULT_MODEL_ID,
  OPENAI_DEFAULT_MODEL_MAX_TOKENS,
  OPENAI_DEFAULT_MODEL_NAME,
} from '@/constants/openai'
import type { GroupPlatform } from '@/types'

const DEFAULT_COST = {
  input: 0,
  output: 0,
  cacheRead: 0,
  cacheWrite: 0,
}

export const OPENCLAW_CONFIG_PATH = '~/.openclaw/openclaw.json'

type OpenClawProviderAPI = 'openai-responses' | 'anthropic-messages' | 'google-generative-ai'

interface OpenClawModelCatalogEntry {
  id: string
  name: string
  reasoning: boolean
  input: string[]
  cost: typeof DEFAULT_COST
  contextWindow: number
  maxTokens: number
}

interface OpenClawProviderConfig {
  api: OpenClawProviderAPI
  apiKey: string
  baseUrl: string
  headers?: Record<string, string>
  models: OpenClawModelCatalogEntry[]
}

export interface OpenClawImportSpec {
  providerId: string
  modelId: string
  modelName: string
  modelRef: string
  alias: string
  provider: OpenClawProviderConfig
}

export type OpenClawConfigRoot = Record<string, unknown>

const OPENCLAW_DEFAULT_THINKING = 'high' as const

const OPENCLAW_COMPAT_HEADERS = {
  'User-Agent': 'OpenClaw/1.0',
} as const

const isPlainObject = (value: unknown): value is Record<string, unknown> =>
  Object.prototype.toString.call(value) === '[object Object]'

const ensureV1 = (value: string) => {
  const trimmed = value.replace(/\/+$/, '')
  return trimmed.endsWith('/v1') ? trimmed : `${trimmed}/v1`
}

const normalizeBaseUrl = (value: string) => {
  const trimmed = value.trim()
  const withoutV1 = trimmed.replace(/\/v1(?:beta)?\/?$/, '')
  return withoutV1.replace(/\/+$/, '')
}

const buildCatalogModel = (
  id: string,
  name: string,
  options: {
    reasoning: boolean
    input: string[]
    contextWindow: number
    maxTokens: number
  }
): OpenClawModelCatalogEntry => ({
  id,
  name,
  reasoning: options.reasoning,
  input: [...options.input],
  cost: { ...DEFAULT_COST },
  contextWindow: options.contextWindow,
  maxTokens: options.maxTokens,
})

export const supportsOpenClawPlatform = (platform: GroupPlatform | null): boolean =>
  platform === 'anthropic' ||
  platform === 'openai' ||
  platform === 'gemini' ||
  platform === 'antigravity'

export const buildOpenClawImportSpec = (
  platform: GroupPlatform | null,
  baseUrl: string,
  apiKey: string
): OpenClawImportSpec | null => {
  if (!supportsOpenClawPlatform(platform)) {
    return null
  }

  const baseRoot = normalizeBaseUrl(baseUrl)
  const openAIBase = ensureV1(baseRoot)

  switch (platform) {
    case 'openai': {
      const modelId = OPENAI_DEFAULT_MODEL_ID
      const modelName = OPENAI_DEFAULT_MODEL_NAME
      const providerId = 'apipool-openai'
      return {
        providerId,
        modelId,
        modelName,
        modelRef: `${providerId}/${modelId}`,
        alias: `APIPool ${modelName}`,
        provider: {
          api: 'openai-responses',
          apiKey,
          baseUrl: openAIBase,
          headers: { ...OPENCLAW_COMPAT_HEADERS },
          models: [
            buildCatalogModel(modelId, modelName, {
              reasoning: true,
              input: ['text'],
              contextWindow: OPENAI_DEFAULT_MODEL_CONTEXT_WINDOW,
              maxTokens: OPENAI_DEFAULT_MODEL_MAX_TOKENS,
            }),
          ],
        },
      }
    }
    case 'gemini': {
      const modelId = 'gemini-2.0-flash'
      const modelName = 'Gemini 2.0 Flash'
      const providerId = 'apipool-gemini'
      return {
        providerId,
        modelId,
        modelName,
        modelRef: `${providerId}/${modelId}`,
        alias: 'APIPool Gemini 2.0 Flash',
        provider: {
          api: 'google-generative-ai',
          apiKey,
          baseUrl: `${baseRoot}/v1beta`,
          models: [
            buildCatalogModel(modelId, modelName, {
              reasoning: false,
              input: ['text', 'image'],
              contextWindow: 1048576,
              maxTokens: 65536,
            }),
          ],
        },
      }
    }
    case 'anthropic': {
      const modelId = 'claude-opus-4-6'
      const modelName = 'Claude Opus 4.6'
      const providerId = 'apipool-anthropic'
      return {
        providerId,
        modelId,
        modelName,
        modelRef: `${providerId}/${modelId}`,
        alias: 'APIPool Claude Opus 4.6',
        provider: {
          api: 'anthropic-messages',
          apiKey,
          baseUrl: baseRoot,
          headers: { ...OPENCLAW_COMPAT_HEADERS },
          models: [
            buildCatalogModel(modelId, modelName, {
              reasoning: true,
              input: ['text', 'image'],
              contextWindow: 200000,
              maxTokens: 128000,
            }),
          ],
        },
      }
    }
    case 'antigravity': {
      const modelId = 'claude-sonnet-4-6'
      const modelName = 'Claude Sonnet 4.6'
      const providerId = 'apipool-antigravity'
      return {
        providerId,
        modelId,
        modelName,
        modelRef: `${providerId}/${modelId}`,
        alias: 'APIPool Antigravity Claude Sonnet 4.6',
        provider: {
          api: 'anthropic-messages',
          apiKey,
          baseUrl: `${baseRoot}/antigravity`,
          headers: { ...OPENCLAW_COMPAT_HEADERS },
          models: [
            buildCatalogModel(modelId, modelName, {
              reasoning: true,
              input: ['text', 'image'],
              contextWindow: 200000,
              maxTokens: 64000,
            }),
          ],
        },
      }
    }
    default:
      return null
  }
}

export const parseOpenClawConfig = (source: string): OpenClawConfigRoot => {
  const parsed = JSON5.parse(source)
  if (!isPlainObject(parsed)) {
    throw new Error('OpenClaw 配置文件根节点必须是对象')
  }
  return parsed
}

export const mergeOpenClawConfig = (
  source: OpenClawConfigRoot,
  spec: OpenClawImportSpec
): OpenClawConfigRoot => {
  const next: OpenClawConfigRoot = { ...source }

  const sourceModels = isPlainObject(source.models) ? source.models : {}
  const sourceProviders = isPlainObject(sourceModels.providers) ? sourceModels.providers : {}
  next.models = {
    ...sourceModels,
    mode: typeof sourceModels.mode === 'string' && sourceModels.mode ? sourceModels.mode : 'merge',
    providers: {
      ...sourceProviders,
      [spec.providerId]: spec.provider,
    },
  }

  const sourceAgents = isPlainObject(source.agents) ? source.agents : {}
  const sourceDefaults = isPlainObject(sourceAgents.defaults) ? sourceAgents.defaults : {}
  const sourceDefaultModels = isPlainObject(sourceDefaults.models) ? sourceDefaults.models : {}
  const rawExistingModelEntry = sourceDefaultModels[spec.modelRef]
  const existingModelEntry: Record<string, unknown> = isPlainObject(rawExistingModelEntry)
    ? { ...rawExistingModelEntry }
    : {}
  const existingParams = isPlainObject(existingModelEntry.params)
    ? { ...existingModelEntry.params }
    : {}
  const importedModel = spec.provider.models.find((model) => model.id === spec.modelId)
  const mergedParams = importedModel?.reasoning
    ? {
        thinking: OPENCLAW_DEFAULT_THINKING,
        ...existingParams,
      }
    : existingParams

  const sourceDefaultModel = sourceDefaults.model
  const normalizedDefaultModel = isPlainObject(sourceDefaultModel)
    ? { ...sourceDefaultModel, primary: spec.modelRef }
    : { primary: spec.modelRef }

  next.agents = {
    ...sourceAgents,
    defaults: {
      ...sourceDefaults,
      model: normalizedDefaultModel,
      models: {
        ...sourceDefaultModels,
        [spec.modelRef]: {
          alias:
            typeof existingModelEntry.alias === 'string' && existingModelEntry.alias
              ? existingModelEntry.alias
              : spec.alias,
          ...existingModelEntry,
          ...(Object.keys(mergedParams).length > 0 ? { params: mergedParams } : {}),
        },
      },
    },
  }

  return next
}

export const buildMergedOpenClawConfigText = (
  source: string,
  spec: OpenClawImportSpec
): string => {
  const parsed = parseOpenClawConfig(source)
  const merged = mergeOpenClawConfig(parsed, spec)
  return `${JSON.stringify(merged, null, 2)}\n`
}
