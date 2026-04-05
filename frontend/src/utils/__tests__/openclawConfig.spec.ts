import { describe, expect, it } from 'vitest'
import {
  buildMergedOpenClawConfigText,
  buildOpenClawImportSpec,
  mergeOpenClawConfig,
  parseOpenClawConfig,
  supportsOpenClawPlatform,
} from '@/utils/openclawConfig'

describe('openclawConfig utils', () => {
  it('supports the expected platforms', () => {
    expect(supportsOpenClawPlatform('openai')).toBe(true)
    expect(supportsOpenClawPlatform('gemini')).toBe(true)
    expect(supportsOpenClawPlatform('anthropic')).toBe(true)
    expect(supportsOpenClawPlatform('antigravity')).toBe(true)
    expect(supportsOpenClawPlatform(null)).toBe(false)
  })

  it('parses json5 and merges a new OpenAI provider while preserving fallbacks', () => {
    const spec = buildOpenClawImportSpec('openai', 'https://apipool.dev', 'sk-openai')
    expect(spec).toBeTruthy()

    const config = parseOpenClawConfig(`
      {
        // existing config
        agents: {
          defaults: {
            model: {
              primary: "moonshot/kimi-k2.5",
              fallbacks: ["moonshot/kimi-k1.5"],
            },
            models: {
              "moonshot/kimi-k2.5": { alias: "Kimi" },
            },
          },
        },
      }
    `)

    const merged = mergeOpenClawConfig(config, spec!)
    const defaults = (merged.agents as any).defaults

    expect((merged.models as any).mode).toBe('merge')
    expect((merged.models as any).providers['apipool-openai']).toMatchObject({
      api: 'openai-responses',
      apiKey: 'sk-openai',
      baseUrl: 'https://apipool.dev/v1',
      headers: {
        'User-Agent': 'OpenClaw/1.0',
      },
    })
    expect((merged.models as any).providers['apipool-openai'].models).toEqual([
      expect.objectContaining({
        id: 'gpt-5.4',
        contextWindow: 500000,
        maxTokens: 128000,
      }),
    ])
    expect(defaults.models['apipool-openai/gpt-5.4']).toEqual({
      alias: 'APIPool GPT-5.4',
      params: {
        thinking: 'high',
      },
    })
    expect(defaults.model).toEqual({
      primary: 'apipool-openai/gpt-5.4',
      fallbacks: ['moonshot/kimi-k1.5'],
    })
    expect(defaults.models['moonshot/kimi-k2.5']).toEqual({ alias: 'Kimi' })
  })

  it('re-importing the same provider updates credentials and keeps custom model settings', () => {
    const spec = buildOpenClawImportSpec('openai', 'https://new.apipool.dev/', 'sk-new')
    expect(spec).toBeTruthy()

    const merged = mergeOpenClawConfig(
      {
        models: {
          mode: 'replace',
          providers: {
            'apipool-openai': {
              api: 'openai-responses',
              apiKey: 'sk-old',
              baseUrl: 'https://old.apipool.dev/v1',
              models: [{ id: 'gpt-5.4', name: 'Old GPT-5.4' }],
            },
          },
        },
        agents: {
          defaults: {
            model: 'old-provider/old-model',
            models: {
              'apipool-openai/gpt-5.4': {
                alias: 'My GPT',
                params: {
                  temperature: 0.2,
                },
              },
            },
          },
        },
      },
      spec!
    )

    expect((merged.models as any).mode).toBe('replace')
    expect((merged.models as any).providers['apipool-openai']).toMatchObject({
      apiKey: 'sk-new',
      baseUrl: 'https://new.apipool.dev/v1',
      headers: {
        'User-Agent': 'OpenClaw/1.0',
      },
    })
    expect((merged.agents as any).defaults.models['apipool-openai/gpt-5.4']).toEqual({
      alias: 'My GPT',
      params: {
        thinking: 'high',
        temperature: 0.2,
      },
    })
    expect((merged.agents as any).defaults.model).toEqual({
      primary: 'apipool-openai/gpt-5.4',
    })
  })

  it('keeps an existing thinking override when re-importing the same provider', () => {
    const spec = buildOpenClawImportSpec('anthropic', 'https://apipool.dev', 'sk-anthropic')
    expect(spec).toBeTruthy()

    const merged = mergeOpenClawConfig(
      {
        agents: {
          defaults: {
            models: {
              'apipool-anthropic/claude-opus-4-6': {
                alias: 'My Anthropic',
                params: {
                  thinking: 'adaptive',
                  cacheRetention: 'long',
                },
              },
            },
          },
        },
      },
      spec!
    )

    expect((merged.agents as any).defaults.models['apipool-anthropic/claude-opus-4-6']).toEqual({
      alias: 'My Anthropic',
      params: {
        thinking: 'adaptive',
        cacheRetention: 'long',
      },
    })
  })

  it('generates valid JSON text after importing an antigravity provider', () => {
    const spec = buildOpenClawImportSpec('antigravity', 'https://apipool.dev/', 'sk-antigravity')
    expect(spec).toBeTruthy()

    const result = buildMergedOpenClawConfigText('{}', spec!)
    const parsed = JSON.parse(result)

    expect(parsed.models.providers['apipool-antigravity']).toMatchObject({
      api: 'anthropic-messages',
      apiKey: 'sk-antigravity',
      baseUrl: 'https://apipool.dev/antigravity',
      headers: {
        'User-Agent': 'OpenClaw/1.0',
      },
    })
    expect(parsed.agents.defaults.models['apipool-antigravity/claude-sonnet-4-6']).toEqual({
      alias: 'APIPool Antigravity Claude Sonnet 4.6',
      params: {
        thinking: 'high',
      },
    })
    expect(parsed.agents.defaults.model).toEqual({
      primary: 'apipool-antigravity/claude-sonnet-4-6',
    })
  })

  it('builds anthropic-compatible provider config for third-party gateways', () => {
    const spec = buildOpenClawImportSpec('anthropic', 'https://apipool.dev', 'sk-anthropic')
    expect(spec).toBeTruthy()

    expect(spec).toMatchObject({
      providerId: 'apipool-anthropic',
      modelRef: 'apipool-anthropic/claude-opus-4-6',
      alias: 'APIPool Claude Opus 4.6',
      provider: {
        api: 'anthropic-messages',
        apiKey: 'sk-anthropic',
        baseUrl: 'https://apipool.dev',
        headers: {
          'User-Agent': 'OpenClaw/1.0',
        },
        models: [
          expect.objectContaining({
            id: 'claude-opus-4-6',
            contextWindow: 200000,
            maxTokens: 128000,
          }),
        ],
      },
    })
  })
})
