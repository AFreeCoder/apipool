# Anthropic AWS Kiro Account Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 Anthropic 平台新增 `kiro` 账号类型，完成账号录入、测试连接、实际请求转发和 access token 自动刷新整条链路，并让 Kiro 复用现有配额、池模式、模型映射、代理和默认分组能力。

**Architecture:** 以 `platform = anthropic, type = kiro` 作为独立账号类型落地，不复用 Claude OAuth 或 Bedrock 的协议分支。后端新增 Kiro 专属 credentials 解析、刷新服务、token provider、Anthropic→Kiro 请求转换器和 Kiro→Anthropic 响应/流式适配器；前端在现有账号管理界面补一条 Anthropic 下的 Kiro 分支，并把“配额 / 池模式 / 自定义错误码”从类型硬编码切到能力门控。

**Tech Stack:** Go service layer + Gin + Wire + existing `HTTPUpstream` / `OAuthRefreshAPI` / `TokenRefreshService`, Vue 3 + TypeScript + Vitest, existing account admin UI and i18n system.

---

## File Map

### Backend: create

- `backend/internal/service/kiro_credentials.go`
  - 解析 / 标准化 `accounts.credentials` 中的 Kiro 字段，处理 `auth_method`、region 优先级、`machine_id` 标准化与生成。
- `backend/internal/service/kiro_credentials_test.go`
  - 覆盖 machine id、region、auth method、能力门控的单元测试。
- `backend/internal/service/kiro_auth_service.go`
  - 负责 `social` / `idc` 刷新协议，封装 refresh URL、headers、请求体和响应映射。
- `backend/internal/service/kiro_auth_service_test.go`
  - 覆盖 social/idc 刷新成功、`invalid_grant`、header/body 构造。
- `backend/internal/service/kiro_token_refresher.go`
  - 实现 `TokenRefresher` / `OAuthRefreshExecutor`，把 Kiro 接进后台刷新体系。
- `backend/internal/service/kiro_token_provider.go`
  - 热路径 token 获取，接入缓存、锁、短 TTL、统一刷新 API。
- `backend/internal/service/kiro_token_provider_test.go`
  - 覆盖缓存命中、预刷新、锁占用等待、刷新失败短 TTL。
- `backend/internal/service/account_test_service_kiro.go`
  - 负责 Kiro 测试连接专用分支。
- `backend/internal/service/account_test_service_kiro_test.go`
  - 覆盖测试连接成功、上游错误、token 刷新失败。
- `backend/internal/service/kiro_converter.go`
  - Anthropic `/v1/messages` 请求体 → Kiro `generateAssistantResponse` 请求体。
- `backend/internal/service/kiro_converter_test.go`
  - 覆盖消息历史、session id、模型映射、`profileArn` 注入。
- `backend/internal/service/kiro_stream_adapter.go`
  - Kiro event stream / 非流式响应 → Anthropic 兼容响应与 SSE。
- `backend/internal/service/kiro_stream_adapter_test.go`
  - 覆盖 `assistantResponseEvent`、`toolUseEvent`、`contextUsageEvent`、异常事件。
- `backend/internal/service/gateway_service_kiro.go`
  - Kiro 转发主逻辑，隔离出 `forwardKiro`、构造请求、处理流式/非流式响应。
- `backend/internal/service/gateway_service_kiro_test.go`
  - 覆盖 `GetAccessToken`、`forwardKiro` 非流式、流式、错误路径。
- `backend/internal/handler/admin/account_handler_kiro_test.go`
  - 验证 `type = kiro` 可通过管理端创建 / 更新接口。
- `backend/internal/handler/dto/account_kiro_test.go`
  - 验证 Kiro 账号 DTO 可暴露 quota 字段。

### Backend: modify

- `backend/internal/domain/constants.go`
- `backend/internal/service/domain_constants.go`
- `backend/internal/service/account.go`
- `backend/internal/service/token_cache_key.go`
- `backend/internal/service/token_cache_key_test.go`
- `backend/internal/service/token_cache_invalidator.go`
- `backend/internal/service/token_refresher.go`
- `backend/internal/service/token_refresh_service.go`
- `backend/internal/service/refresh_policy.go`
- `backend/internal/service/gateway_service.go`
- `backend/internal/service/account_test_service.go`
- `backend/internal/service/wire.go`
- `backend/internal/handler/admin/account_handler.go`
- `backend/internal/handler/dto/mappers.go`
- `backend/cmd/server/wire_gen.go`

### Frontend: create

- `frontend/src/components/account/accountTypeCapabilities.ts`
  - 统一前端“支持 quota / pool mode / custom error codes”的能力判断。
- `frontend/src/components/account/__tests__/CreateAccountModal.spec.ts`
  - 验证 Kiro 卡片、字段切换和创建 payload。
- `frontend/src/components/account/__tests__/accountTypeCapabilities.spec.ts`
  - 验证 Kiro 的能力边界。

### Frontend: modify

- `frontend/src/types/index.ts`
- `frontend/src/components/account/credentialsBuilder.ts`
- `frontend/src/components/account/__tests__/credentialsBuilder.spec.ts`
- `frontend/src/components/account/CreateAccountModal.vue`
- `frontend/src/components/account/EditAccountModal.vue`
- `frontend/src/components/account/BulkEditAccountModal.vue`
- `frontend/src/components/account/AccountCapacityCell.vue`
- `frontend/src/components/account/AccountUsageCell.vue`
- `frontend/src/components/admin/account/AccountActionMenu.vue`
- `frontend/src/components/common/PlatformTypeBadge.vue`
- `frontend/src/components/account/__tests__/EditAccountModal.spec.ts`
- `frontend/src/components/account/__tests__/BulkEditAccountModal.spec.ts`
- `frontend/src/components/account/__tests__/AccountUsageCell.spec.ts`
- `frontend/src/i18n/locales/en.ts`
- `frontend/src/i18n/locales/zh.ts`

### Generated

- `backend/cmd/server/wire_gen.go`
  - 由 `cd backend && go generate ./cmd/server` 生成，不手改。

---

### Task 1: 建立 Kiro 共享领域模型和能力门控

**Files:**
- Create: `backend/internal/service/kiro_credentials.go`
- Create: `backend/internal/service/kiro_credentials_test.go`
- Create: `backend/internal/handler/dto/account_kiro_test.go`
- Modify: `backend/internal/domain/constants.go`
- Modify: `backend/internal/service/domain_constants.go`
- Modify: `backend/internal/service/account.go`
- Modify: `backend/internal/service/token_cache_key.go`
- Modify: `backend/internal/service/token_cache_key_test.go`
- Modify: `backend/internal/service/token_cache_invalidator.go`
- Modify: `backend/internal/handler/dto/mappers.go`

- [ ] **Step 1: 先写会失败的共享能力测试**

在 `backend/internal/service/kiro_credentials_test.go` 里加入：

```go
//go:build unit

package service

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseKiroCredentials_NormalizesIDCAndMachineID(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       42,
		Platform: PlatformAnthropic,
		Type:     AccountTypeKiro,
		Credentials: map[string]any{
			"auth_method":   "iam",
			"refresh_token": "rt-123",
			"region":        "us-west-2",
			"machine_id":    "550e8400-e29b-41d4-a716-446655440000",
		},
	}

	creds, err := ParseKiroCredentials(account)
	require.NoError(t, err)
	require.Equal(t, "idc", creds.AuthMethod)
	require.Equal(t, "us-west-2", creds.EffectiveAuthRegion())
	require.Equal(t, "us-east-1", creds.EffectiveAPIRegion())
	require.Len(t, creds.MachineID, 64)
}

func TestParseKiroCredentials_GeneratesMachineIDFromRefreshToken(t *testing.T) {
	t.Parallel()

	account := &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeKiro,
		Credentials: map[string]any{
			"auth_method":   "social",
			"refresh_token": "rt-generated",
		},
	}

	creds, err := ParseKiroCredentials(account)
	require.NoError(t, err)

	sum := sha256.Sum256([]byte("KotlinNativeAPI/" + "rt-generated"))
	require.Equal(t, hex.EncodeToString(sum[:]), creds.MachineID)
}

func TestAccount_KiroCapabilities(t *testing.T) {
	t.Parallel()

	account := &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeKiro,
		Credentials: map[string]any{
			"pool_mode":                true,
			"custom_error_codes":       []int{429},
			"custom_error_codes_enabled": true,
		},
	}

	require.True(t, account.IsKiro())
	require.True(t, account.SupportsQuotaLimit())
	require.True(t, account.SupportsPoolMode())
	require.False(t, account.SupportsCustomErrorCodes())
	require.True(t, account.IsPoolMode())
	require.False(t, account.IsCustomErrorCodesEnabled())
}
```

在 `backend/internal/handler/dto/account_kiro_test.go` 里加入：

```go
package dto

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAccountFromServiceShallow_ExposesQuotaForKiro(t *testing.T) {
	t.Parallel()

	account := &service.Account{
		ID:       7,
		Platform: service.PlatformAnthropic,
		Type:     service.AccountTypeKiro,
		Credentials: map[string]any{
			"pool_mode": true,
		},
		Extra: map[string]any{
			"quota_limit":        100.0,
			"quota_daily_limit":  20.0,
			"quota_weekly_limit": 70.0,
		},
	}

	out := AccountFromServiceShallow(account)
	require.NotNil(t, out.QuotaLimit)
	require.Equal(t, 100.0, *out.QuotaLimit)
	require.NotNil(t, out.QuotaDailyLimit)
	require.NotNil(t, out.QuotaWeeklyLimit)
}
```

并在 `backend/internal/service/token_cache_key_test.go` 里追加：

```go
func TestKiroTokenCacheKey(t *testing.T) {
	t.Parallel()

	account := &Account{ID: 555}
	require.Equal(t, "kiro:account:555", KiroTokenCacheKey(account))
}
```

- [ ] **Step 2: 跑最小测试，确认现在确实是红灯**

Run:

```bash
cd backend && go test -tags=unit ./internal/service ./internal/handler/dto -run 'Test(ParseKiroCredentials|Account_KiroCapabilities|KiroTokenCacheKey|AccountFromServiceShallow_ExposesQuotaForKiro)' -count=1
```

Expected:

```text
FAIL
undefined: AccountTypeKiro
undefined: ParseKiroCredentials
undefined: KiroTokenCacheKey
```

- [ ] **Step 3: 实现 Kiro 常量、credentials 解析、能力门控和 DTO 暴露**

在 `backend/internal/domain/constants.go` 和 `backend/internal/service/domain_constants.go` 加入：

```go
const (
	AccountTypeOAuth      = "oauth"
	AccountTypeSetupToken = "setup-token"
	AccountTypeAPIKey     = "apikey"
	AccountTypeUpstream   = "upstream"
	AccountTypeBedrock    = "bedrock"
	AccountTypeKiro       = "kiro"
)
```

新增 `backend/internal/service/kiro_credentials.go`：

```go
package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	kiroAuthMethodSocial = "social"
	kiroAuthMethodIDC    = "idc"
	defaultKiroRegion    = "us-east-1"
)

type KiroCredentials struct {
	AccessToken  string
	RefreshToken string
	AuthMethod   string
	ClientID     string
	ClientSecret string
	ProfileARN   string
	Region       string
	AuthRegion   string
	APIRegion    string
	MachineID    string
	BaseURL      string
}

func ParseKiroCredentials(account *Account) (*KiroCredentials, error) {
	if account == nil || !account.IsKiro() {
		return nil, fmt.Errorf("not a kiro account")
	}

	authMethod := NormalizeKiroAuthMethod(account.GetCredential("auth_method"))
	if authMethod == "" {
		return nil, fmt.Errorf("kiro auth_method is required")
	}

	refreshToken := strings.TrimSpace(account.GetCredential("refresh_token"))
	if refreshToken == "" {
		return nil, fmt.Errorf("kiro refresh_token is required")
	}

	machineID, err := NormalizeKiroMachineID(account.GetCredential("machine_id"), refreshToken)
	if err != nil {
		return nil, err
	}

	creds := &KiroCredentials{
		AccessToken:  strings.TrimSpace(account.GetCredential("access_token")),
		RefreshToken: refreshToken,
		AuthMethod:   authMethod,
		ClientID:     strings.TrimSpace(account.GetCredential("client_id")),
		ClientSecret: strings.TrimSpace(account.GetCredential("client_secret")),
		ProfileARN:   strings.TrimSpace(account.GetCredential("profile_arn")),
		Region:       strings.TrimSpace(account.GetCredential("region")),
		AuthRegion:   strings.TrimSpace(account.GetCredential("auth_region")),
		APIRegion:    strings.TrimSpace(account.GetCredential("api_region")),
		MachineID:    machineID,
		BaseURL:      strings.TrimSpace(account.GetCredential("base_url")),
	}

	if creds.AuthMethod == kiroAuthMethodIDC && (creds.ClientID == "" || creds.ClientSecret == "") {
		return nil, fmt.Errorf("kiro idc requires client_id and client_secret")
	}

	return creds, nil
}

func NormalizeKiroAuthMethod(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case kiroAuthMethodSocial:
		return kiroAuthMethodSocial
	case kiroAuthMethodIDC, "builder-id", "iam":
		return kiroAuthMethodIDC
	default:
		return ""
	}
}

func NormalizeKiroMachineID(raw string, refreshToken string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		sum := sha256.Sum256([]byte("KotlinNativeAPI/" + refreshToken))
		return hex.EncodeToString(sum[:]), nil
	}

	normalized := strings.ReplaceAll(trimmed, "-", "")
	switch len(normalized) {
	case 32:
		return normalized + normalized, nil
	case 64:
		return normalized, nil
	default:
		return "", fmt.Errorf("invalid kiro machine_id")
	}
}

func (k *KiroCredentials) EffectiveAuthRegion() string {
	if strings.TrimSpace(k.AuthRegion) != "" {
		return strings.TrimSpace(k.AuthRegion)
	}
	if strings.TrimSpace(k.Region) != "" {
		return strings.TrimSpace(k.Region)
	}
	return defaultKiroRegion
}

func (k *KiroCredentials) EffectiveAPIRegion() string {
	if strings.TrimSpace(k.APIRegion) != "" {
		return strings.TrimSpace(k.APIRegion)
	}
	return defaultKiroRegion
}
```

在 `backend/internal/service/account.go` 改成能力方法，不再复用 `IsAPIKeyOrBedrock()`：

```go
func (a *Account) IsKiro() bool {
	return a.Platform == PlatformAnthropic && a.Type == AccountTypeKiro
}

func (a *Account) SupportsQuotaLimit() bool {
	return a.Type == AccountTypeAPIKey || a.Type == AccountTypeBedrock || a.Type == AccountTypeKiro
}

func (a *Account) SupportsPoolMode() bool {
	return a.Type == AccountTypeAPIKey || a.Type == AccountTypeBedrock || a.Type == AccountTypeKiro
}

func (a *Account) SupportsCustomErrorCodes() bool {
	return a.Type == AccountTypeAPIKey
}

func (a *Account) IsPoolMode() bool {
	if !a.SupportsPoolMode() || a.Credentials == nil {
		return false
	}
	v, ok := a.Credentials["pool_mode"]
	if !ok {
		return false
	}
	enabled, _ := v.(bool)
	return enabled
}

func (a *Account) IsCustomErrorCodesEnabled() bool {
	if !a.SupportsCustomErrorCodes() || a.Credentials == nil {
		return false
	}
	v, ok := a.Credentials["custom_error_codes_enabled"]
	if !ok {
		return false
	}
	enabled, _ := v.(bool)
	return enabled
}
```

在 `backend/internal/service/token_cache_key.go` 与 `backend/internal/service/token_cache_invalidator.go` 中加入：

```go
func KiroTokenCacheKey(account *Account) string {
	return "kiro:account:" + strconv.FormatInt(account.ID, 10)
}
```

```go
func (c *CompositeTokenCacheInvalidator) InvalidateToken(ctx context.Context, account *Account) error {
	if c == nil || c.cache == nil || account == nil {
		return nil
	}
	if account.Type != AccountTypeOAuth && !account.IsKiro() {
		return nil
	}

	var keysToDelete []string
	accountIDKey := "account:" + strconv.FormatInt(account.ID, 10)

	switch account.Platform {
	case PlatformAnthropic:
		if account.IsKiro() {
			keysToDelete = append(keysToDelete, KiroTokenCacheKey(account))
		} else {
			keysToDelete = append(keysToDelete, ClaudeTokenCacheKey(account))
		}
	case PlatformGemini:
		keysToDelete = append(keysToDelete, GeminiTokenCacheKey(account), "gemini:"+accountIDKey)
	case PlatformAntigravity:
		keysToDelete = append(keysToDelete, AntigravityTokenCacheKey(account), "ag:"+accountIDKey)
	case PlatformOpenAI:
		keysToDelete = append(keysToDelete, OpenAITokenCacheKey(account))
	}

	for _, key := range keysToDelete {
		_ = c.cache.DeleteAccessToken(ctx, key)
	}
	return nil
}
```

把 `backend/internal/handler/dto/mappers.go` 和所有 quota 判断从：

```go
if a.IsAPIKeyOrBedrock() {
```

改成：

```go
if a.SupportsQuotaLimit() {
```

- [ ] **Step 4: 再跑同一组测试，确认共享基础变绿**

Run:

```bash
cd backend && go test -tags=unit ./internal/service ./internal/handler/dto -run 'Test(ParseKiroCredentials|Account_KiroCapabilities|KiroTokenCacheKey|AccountFromServiceShallow_ExposesQuotaForKiro)' -count=1
```

Expected:

```text
ok  	github.com/Wei-Shaw/sub2api/internal/service
ok  	github.com/Wei-Shaw/sub2api/internal/handler/dto
```

- [ ] **Step 5: 提交共享基础**

```bash
git add backend/internal/domain/constants.go \
  backend/internal/service/domain_constants.go \
  backend/internal/service/account.go \
  backend/internal/service/kiro_credentials.go \
  backend/internal/service/kiro_credentials_test.go \
  backend/internal/service/token_cache_key.go \
  backend/internal/service/token_cache_key_test.go \
  backend/internal/service/token_cache_invalidator.go \
  backend/internal/handler/dto/mappers.go \
  backend/internal/handler/dto/account_kiro_test.go
git commit -m "feat: add kiro account foundation"
```

### Task 2: 打通管理端校验和前端账号录入/编辑入口

**Files:**
- Create: `frontend/src/components/account/accountTypeCapabilities.ts`
- Create: `frontend/src/components/account/__tests__/CreateAccountModal.spec.ts`
- Create: `frontend/src/components/account/__tests__/accountTypeCapabilities.spec.ts`
- Create: `backend/internal/handler/admin/account_handler_kiro_test.go`
- Modify: `backend/internal/handler/admin/account_handler.go`
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/components/account/credentialsBuilder.ts`
- Modify: `frontend/src/components/account/__tests__/credentialsBuilder.spec.ts`
- Modify: `frontend/src/components/account/CreateAccountModal.vue`
- Modify: `frontend/src/components/account/EditAccountModal.vue`
- Modify: `frontend/src/components/account/BulkEditAccountModal.vue`
- Modify: `frontend/src/components/account/AccountCapacityCell.vue`
- Modify: `frontend/src/components/account/AccountUsageCell.vue`
- Modify: `frontend/src/components/admin/account/AccountActionMenu.vue`
- Modify: `frontend/src/components/common/PlatformTypeBadge.vue`
- Modify: `frontend/src/components/account/__tests__/EditAccountModal.spec.ts`
- Modify: `frontend/src/components/account/__tests__/BulkEditAccountModal.spec.ts`
- Modify: `frontend/src/components/account/__tests__/AccountUsageCell.spec.ts`
- Modify: `frontend/src/i18n/locales/en.ts`
- Modify: `frontend/src/i18n/locales/zh.ts`

- [ ] **Step 1: 先写管理端和前端会失败的测试**

新增 `backend/internal/handler/admin/account_handler_kiro_test.go`：

```go
package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAccountHandler_Create_KiroAcceptsType(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adminSvc := newStubAdminService()
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	router := gin.New()
	router.POST("/api/v1/admin/accounts", handler.Create)

	body := map[string]any{
		"name":     "kiro-social-1",
		"platform": "anthropic",
		"type":     "kiro",
		"credentials": map[string]any{
			"auth_method":   "social",
			"refresh_token": "rt-1",
			"auth_region":   "us-east-1",
			"api_region":    "us-east-1",
		},
		"concurrency": 1,
		"priority":    1,
	}

	raw, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, adminSvc.createdAccounts, 1)
	require.Equal(t, "kiro", adminSvc.createdAccounts[0].Type)
}
```

在 `frontend/src/components/account/__tests__/accountTypeCapabilities.spec.ts` 里加入：

```ts
import { describe, expect, it } from 'vitest'
import { supportsQuotaLimit, supportsPoolMode, supportsCustomErrorCodes } from '../accountTypeCapabilities'

describe('accountTypeCapabilities', () => {
  it('kiro supports quota and pool mode but not custom error codes', () => {
    expect(supportsQuotaLimit('kiro')).toBe(true)
    expect(supportsPoolMode('kiro')).toBe(true)
    expect(supportsCustomErrorCodes('kiro')).toBe(false)
  })
})
```

在 `frontend/src/components/account/__tests__/credentialsBuilder.spec.ts` 里追加：

```ts
import { buildKiroCredentials } from '../credentialsBuilder'

it('buildKiroCredentials creates idc payload and strips empty optional fields', () => {
  expect(
    buildKiroCredentials({
      mode: 'create',
      authMethod: 'idc',
      refreshToken: 'rt-1',
      authRegion: 'us-east-1',
      apiRegion: 'us-west-2',
      machineId: '',
      clientId: 'client-1',
      clientSecret: 'secret-1',
      profileArn: ''
    })
  ).toEqual({
    auth_method: 'idc',
    refresh_token: 'rt-1',
    auth_region: 'us-east-1',
    api_region: 'us-west-2',
    client_id: 'client-1',
    client_secret: 'secret-1'
  })
})
```

新增 `frontend/src/components/account/__tests__/CreateAccountModal.spec.ts`：

```ts
import { describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent } from 'vue'

const { createMock } = vi.hoisted(() => ({
  createMock: vi.fn()
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showInfo: vi.fn()
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    isSimpleMode: true
  })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      create: createMock,
      checkMixedChannelRisk: vi.fn().mockResolvedValue({ has_risk: false })
    },
    tlsFingerprintProfiles: {
      list: vi.fn().mockResolvedValue([])
    }
  }
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

import CreateAccountModal from '../CreateAccountModal.vue'

const BaseDialogStub = defineComponent({
  name: 'BaseDialog',
  props: { show: { type: Boolean, default: false } },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>'
})

describe('CreateAccountModal', () => {
  it('submits anthropic kiro idc credentials', async () => {
    createMock.mockResolvedValue({ id: 1 })

    const wrapper = mount(CreateAccountModal, {
      props: { show: true, proxies: [], groups: [] },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          Icon: true,
          Select: true,
          ProxySelector: true,
          GroupSelector: true,
          ModelWhitelistSelector: true,
          OAuthFlowSection: true
        }
      }
    })

    await wrapper.get('[data-testid="account-category-kiro"]').trigger('click')
    await wrapper.get('#account-name').setValue('Kiro IDC')
    await wrapper.get('[data-testid="kiro-auth-method-idc"]').setValue(true)
    await wrapper.get('#kiro-refresh-token').setValue('rt-123')
    await wrapper.get('#kiro-client-id').setValue('client-1')
    await wrapper.get('#kiro-client-secret').setValue('secret-1')
    await wrapper.get('#kiro-auth-region').setValue('us-east-1')
    await wrapper.get('#kiro-api-region').setValue('us-west-2')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(createMock).toHaveBeenCalledWith(expect.objectContaining({
      platform: 'anthropic',
      type: 'kiro',
      credentials: expect.objectContaining({
        auth_method: 'idc',
        refresh_token: 'rt-123',
        client_id: 'client-1',
        client_secret: 'secret-1',
        auth_region: 'us-east-1',
        api_region: 'us-west-2'
      })
    }))
  })
})
```

- [ ] **Step 2: 跑管理端和前端最小测试，确认现在是红灯**

Run:

```bash
cd backend && go test ./internal/handler/admin -run TestAccountHandler_Create_KiroAcceptsType -count=1
cd frontend && pnpm test:run src/components/account/__tests__/accountTypeCapabilities.spec.ts src/components/account/__tests__/credentialsBuilder.spec.ts src/components/account/__tests__/CreateAccountModal.spec.ts
```

Expected:

```text
FAIL backend/internal/handler/admin: "kiro" fails oneof validation
FAIL frontend: unknown AccountType "kiro" / missing buildKiroCredentials / missing data-testid
```

- [ ] **Step 3: 实现前后端账号管理入口**

在 `backend/internal/handler/admin/account_handler.go` 里把 oneof 扩成：

```go
Type string `json:"type" binding:"required,oneof=oauth setup-token apikey upstream bedrock kiro"`
```

```go
Type string `json:"type" binding:"omitempty,oneof=oauth setup-token apikey upstream bedrock kiro"`
```

在 `frontend/src/types/index.ts` 里把联合类型改成：

```ts
export type AccountType = 'oauth' | 'setup-token' | 'apikey' | 'upstream' | 'bedrock' | 'kiro'
```

新增 `frontend/src/components/account/accountTypeCapabilities.ts`：

```ts
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
```

在 `frontend/src/components/account/credentialsBuilder.ts` 里追加：

```ts
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
```

在 `frontend/src/components/account/CreateAccountModal.vue` 中：

```ts
const accountCategory = ref<'oauth-based' | 'apikey' | 'bedrock' | 'kiro'>('oauth-based')
const kiroAuthMethod = ref<'social' | 'idc'>('social')
const kiroRefreshToken = ref('')
const kiroAuthRegion = ref('us-east-1')
const kiroApiRegion = ref('us-east-1')
const kiroMachineId = ref('')
const kiroClientId = ref('')
const kiroClientSecret = ref('')
const kiroProfileArn = ref('')
```

```ts
if (form.platform === 'anthropic' && category === 'kiro') {
  form.type = 'kiro' as AccountType
  return
}
```

```ts
if (form.platform === 'anthropic' && accountCategory.value === 'kiro') {
  if (!form.name.trim()) {
    appStore.showError(t('admin.accounts.pleaseEnterAccountName'))
    return
  }
  if (!kiroRefreshToken.value.trim()) {
    appStore.showError(t('admin.accounts.kiroRefreshTokenRequired'))
    return
  }
  if (kiroAuthMethod.value === 'idc' && (!kiroClientId.value.trim() || !kiroClientSecret.value.trim())) {
    appStore.showError(t('admin.accounts.kiroIDCClientRequired'))
    return
  }

  const credentials = buildKiroCredentials({
    mode: 'create',
    authMethod: kiroAuthMethod.value,
    refreshToken: kiroRefreshToken.value,
    authRegion: kiroAuthRegion.value,
    apiRegion: kiroApiRegion.value,
    machineId: kiroMachineId.value,
    clientId: kiroClientId.value,
    clientSecret: kiroClientSecret.value,
    profileArn: kiroProfileArn.value
  })

  const modelMapping = buildModelMappingObject(modelRestrictionMode.value, allowedModels.value, modelMappings.value)
  if (modelMapping) {
    credentials.model_mapping = modelMapping
  }
  if (poolModeEnabled.value) {
    credentials.pool_mode = true
    credentials.pool_mode_retry_count = normalizePoolModeRetryCount(poolModeRetryCount.value)
  }
  applyInterceptWarmup(credentials, interceptWarmupRequests.value, 'create')

  await createAccountAndFinish('anthropic', 'kiro' as AccountType, credentials)
  return
}
```

把创建 / 编辑 / 批量编辑 / 容量 / 用量 / 动作菜单里的类型判断统一改成能力判断，例如：

```ts
if (supportsQuotaLimit(type)) {
  quotaExtra.quota_limit = editQuotaLimit.value
}
```

```ts
const isQuotaEligible = computed(() => supportsQuotaLimit(props.account.type))
```

```ts
const hasQuotaLimit = computed(() => supportsQuotaLimit(props.account?.type ?? 'oauth') && (
  (props.account?.quota_limit ?? 0) > 0 ||
  (props.account?.quota_daily_limit ?? 0) > 0 ||
  (props.account?.quota_weekly_limit ?? 0) > 0
))
```

`custom error codes` 仍只在 `supportsCustomErrorCodes(type)` 为 `true` 时显示或提交。

在 `frontend/src/components/common/PlatformTypeBadge.vue` 加：

```ts
case 'kiro':
  return 'Kiro'
```

并为 Kiro 卡片和输入框加测试钩子：

```vue
<button data-testid="account-category-kiro" @click="accountCategory = 'kiro'">
```

```vue
<input id="kiro-refresh-token" v-model="kiroRefreshToken" class="input" />
```

```vue
<input id="kiro-client-id" v-model="kiroClientId" class="input" />
<input id="kiro-client-secret" v-model="kiroClientSecret" class="input" />
```

在 `frontend/src/i18n/locales/en.ts` / `zh.ts` 增加：

```ts
kiroLabel: 'AWS Kiro',
kiroDesc: 'refresh_token / social / idc',
kiroRefreshTokenRequired: 'Kiro refresh token is required',
kiroIDCClientRequired: 'Kiro IdC requires client ID and client secret',
```

- [ ] **Step 4: 跑 UI 与管理端测试，再做一次 TS 类型检查**

Run:

```bash
cd backend && go test ./internal/handler/admin -run TestAccountHandler_Create_KiroAcceptsType -count=1
cd frontend && pnpm test:run src/components/account/__tests__/accountTypeCapabilities.spec.ts src/components/account/__tests__/credentialsBuilder.spec.ts src/components/account/__tests__/CreateAccountModal.spec.ts src/components/account/__tests__/EditAccountModal.spec.ts src/components/account/__tests__/BulkEditAccountModal.spec.ts src/components/account/__tests__/AccountUsageCell.spec.ts
cd frontend && pnpm typecheck
```

Expected:

```text
PASS TestAccountHandler_Create_KiroAcceptsType
PASS frontend account tests
Done in frontend typecheck with 0 errors
```

- [ ] **Step 5: 提交管理端与前端入口**

```bash
git add backend/internal/handler/admin/account_handler.go \
  backend/internal/handler/admin/account_handler_kiro_test.go \
  frontend/src/types/index.ts \
  frontend/src/components/account/accountTypeCapabilities.ts \
  frontend/src/components/account/credentialsBuilder.ts \
  frontend/src/components/account/CreateAccountModal.vue \
  frontend/src/components/account/EditAccountModal.vue \
  frontend/src/components/account/BulkEditAccountModal.vue \
  frontend/src/components/account/AccountCapacityCell.vue \
  frontend/src/components/account/AccountUsageCell.vue \
  frontend/src/components/admin/account/AccountActionMenu.vue \
  frontend/src/components/common/PlatformTypeBadge.vue \
  frontend/src/components/account/__tests__/accountTypeCapabilities.spec.ts \
  frontend/src/components/account/__tests__/credentialsBuilder.spec.ts \
  frontend/src/components/account/__tests__/CreateAccountModal.spec.ts \
  frontend/src/components/account/__tests__/EditAccountModal.spec.ts \
  frontend/src/components/account/__tests__/BulkEditAccountModal.spec.ts \
  frontend/src/components/account/__tests__/AccountUsageCell.spec.ts \
  frontend/src/i18n/locales/en.ts \
  frontend/src/i18n/locales/zh.ts
git commit -m "feat: add kiro account admin ui"
```

### Task 3: 实现 Kiro 刷新协议

**Files:**
- Create: `backend/internal/service/kiro_auth_service.go`
- Create: `backend/internal/service/kiro_auth_service_test.go`

- [ ] **Step 1: 先写 social / idc 刷新协议测试**

在 `backend/internal/service/kiro_auth_service_test.go` 里加入：

```go
//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

type kiroAuthHTTPUpstreamStub struct {
	requests  []*http.Request
	bodies    []string
	responses []*http.Response
}

func (s *kiroAuthHTTPUpstreamStub) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	body, _ := io.ReadAll(req.Body)
	req.Body = io.NopCloser(strings.NewReader(string(body)))
	s.requests = append(s.requests, req)
	s.bodies = append(s.bodies, string(body))
	resp := s.responses[0]
	s.responses = s.responses[1:]
	return resp, nil
}

func (s *kiroAuthHTTPUpstreamStub) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
	panic("unexpected DoWithTLS")
}

func TestKiroAuthService_RefreshSocial(t *testing.T) {
	t.Parallel()

	upstream := &kiroAuthHTTPUpstreamStub{
		responses: []*http.Response{
			newJSONResponse(http.StatusOK, `{"accessToken":"at-2","refreshToken":"rt-2","expiresIn":3600,"profileArn":"arn:aws:kiro:::profile/default"}`),
		},
	}
	svc := NewKiroAuthService(nil, upstream)
	account := &Account{
		ID:       1,
		Platform: PlatformAnthropic,
		Type:     AccountTypeKiro,
		Credentials: map[string]any{
			"auth_method":   "social",
			"refresh_token": "rt-1",
			"auth_region":   "us-east-1",
		},
	}

	info, err := svc.RefreshAccountToken(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, "at-2", info.AccessToken)
	require.Equal(t, "rt-2", info.RefreshToken)
	require.Equal(t, "arn:aws:kiro:::profile/default", info.ProfileARN)
	require.Equal(t, "application/json", upstream.requests[0].Header.Get("Content-Type"))
	require.Contains(t, upstream.bodies[0], `"refreshToken":"rt-1"`)
}

func TestKiroAuthService_RefreshIDC_InvalidGrant(t *testing.T) {
	t.Parallel()

	upstream := &kiroAuthHTTPUpstreamStub{
		responses: []*http.Response{
			newJSONResponse(http.StatusBadRequest, `{"error":"invalid_grant","error_description":"Invalid refresh token provided"}`),
		},
	}
	svc := NewKiroAuthService(nil, upstream)
	account := &Account{
		ID:       2,
		Platform: PlatformAnthropic,
		Type:     AccountTypeKiro,
		Credentials: map[string]any{
			"auth_method":   "idc",
			"refresh_token": "rt-bad",
			"client_id":     "client-1",
			"client_secret": "secret-1",
			"auth_region":   "us-east-1",
		},
	}

	_, err := svc.RefreshAccountToken(context.Background(), account)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid_grant")
	require.Equal(t, "application/json", upstream.requests[0].Header.Get("content-type"))
}
```

- [ ] **Step 2: 跑刷新协议测试，先确认红灯**

Run:

```bash
cd backend && go test -tags=unit ./internal/service -run 'TestKiroAuthService_' -count=1
```

Expected:

```text
FAIL
undefined: NewKiroAuthService
undefined: AccountTypeKiro
```

- [ ] **Step 3: 实现 KiroAuthService，默认按 kiro.rs 的 JSON 协议发起刷新**

新增 `backend/internal/service/kiro_auth_service.go`：

```go
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type KiroTokenInfo struct {
	AccessToken string
	RefreshToken string
	ExpiresAt time.Time
	ExpiresIn int64
	ProfileARN string
}

type KiroAuthService struct {
	proxyRepo ProxyRepository
	httpUpstream HTTPUpstream
}

func NewKiroAuthService(proxyRepo ProxyRepository, httpUpstream HTTPUpstream) *KiroAuthService {
	return &KiroAuthService{
		proxyRepo: proxyRepo,
		httpUpstream: httpUpstream,
	}
}

func (s *KiroAuthService) RefreshAccountToken(ctx context.Context, account *Account) (*KiroTokenInfo, error) {
	creds, err := ParseKiroCredentials(account)
	if err != nil {
		return nil, err
	}

	proxyURL := ""
	if s.proxyRepo != nil && account.ProxyID != nil {
		if proxy, proxyErr := s.proxyRepo.GetByID(ctx, *account.ProxyID); proxyErr == nil && proxy != nil {
			proxyURL = proxy.URL()
		}
	}

	if creds.AuthMethod == kiroAuthMethodSocial {
		return s.refreshSocial(ctx, account, creds, proxyURL)
	}
	return s.refreshIDC(ctx, account, creds, proxyURL)
}

func (s *KiroAuthService) refreshSocial(ctx context.Context, account *Account, creds *KiroCredentials, proxyURL string) (*KiroTokenInfo, error) {
	body := []byte(fmt.Sprintf(`{"refreshToken":%q}`, creds.RefreshToken))
	url := fmt.Sprintf("https://prod.%s.auth.desktop.kiro.dev/refreshToken", creds.EffectiveAuthRegion())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", fmt.Sprintf("KiroIDE-dev-%s", creds.MachineID))
	req.Header.Set("Host", req.URL.Host)

	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return parseKiroRefreshResponse(resp)
}

func (s *KiroAuthService) refreshIDC(ctx context.Context, account *Account, creds *KiroCredentials, proxyURL string) (*KiroTokenInfo, error) {
	bodyMap := map[string]string{
		"clientId":     creds.ClientID,
		"clientSecret": creds.ClientSecret,
		"refreshToken": creds.RefreshToken,
		"grantType":    "refresh_token",
	}
	body, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://oidc.%s.amazonaws.com/token", creds.EffectiveAuthRegion())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-amz-user-agent", "aws-sdk-js/3.980.0 KiroIDE")
	req.Header.Set("user-agent", "aws-sdk-js/3.980.0 ua/2.1 api/sso-oidc#3.980.0 KiroIDE")
	req.Header.Set("host", req.URL.Host)
	req.Header.Set("amz-sdk-invocation-id", uuid.NewString())
	req.Header.Set("amz-sdk-request", "attempt=1; max=4")
	req.Header.Set("Connection", "close")

	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return parseKiroRefreshResponse(resp)
}

func parseKiroRefreshResponse(resp *http.Response) (*KiroTokenInfo, error) {
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		msg := string(body)
		if resp.StatusCode == http.StatusBadRequest && strings.Contains(msg, "invalid_grant") {
			return nil, fmt.Errorf("invalid_grant: %s", msg)
		}
		return nil, fmt.Errorf("kiro refresh failed: %s", msg)
	}

	var raw struct {
		AccessToken string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ExpiresIn int64 `json:"expiresIn"`
		ProfileARN string `json:"profileArn"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	return &KiroTokenInfo{
		AccessToken: raw.AccessToken,
		RefreshToken: raw.RefreshToken,
		ExpiresIn: raw.ExpiresIn,
		ExpiresAt: time.Now().Add(time.Duration(raw.ExpiresIn) * time.Second),
		ProfileARN: raw.ProfileARN,
	}, nil
}
```

- [ ] **Step 4: 跑刷新协议测试，确认协议层变绿**

Run:

```bash
cd backend && go test -tags=unit ./internal/service -run 'TestKiroAuthService_' -count=1
```

Expected:

```text
ok  	github.com/Wei-Shaw/sub2api/internal/service
```

- [ ] **Step 5: 提交 KiroAuthService**

```bash
git add backend/internal/service/kiro_auth_service.go \
  backend/internal/service/kiro_auth_service_test.go
git commit -m "feat: add kiro refresh protocol service"
```

### Task 4: 接入后台刷新、热路径 token provider 和依赖注入

**Files:**
- Create: `backend/internal/service/kiro_token_refresher.go`
- Create: `backend/internal/service/kiro_token_provider.go`
- Create: `backend/internal/service/kiro_token_provider_test.go`
- Modify: `backend/internal/service/refresh_policy.go`
- Modify: `backend/internal/service/token_refresher.go`
- Modify: `backend/internal/service/token_refresh_service.go`
- Modify: `backend/internal/service/account_test_service.go`
- Modify: `backend/internal/service/gateway_service.go`
- Modify: `backend/internal/service/wire.go`
- Modify: `backend/cmd/server/wire_gen.go`

- [ ] **Step 1: 先写 provider / refresher 行为测试**

新增 `backend/internal/service/kiro_token_provider_test.go`：

```go
//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestKiroTokenRefresher_CanRefreshAndNeedsRefresh(t *testing.T) {
	t.Parallel()

	refresher := NewKiroTokenRefresher(nil)
	account := &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeKiro,
		Credentials: map[string]any{
			"expires_at": time.Now().Add(5 * time.Minute).Unix(),
		},
	}

	require.True(t, refresher.CanRefresh(account))
	require.True(t, refresher.NeedsRefresh(account, 10*time.Minute))
}

func TestKiroTokenProvider_GetAccessToken_UsesRefreshAPI(t *testing.T) {
	t.Parallel()

	cache := newClaudeTokenCacheStub()
	accountRepo := &claudeAccountRepoStub{
		account: &Account{
			ID:       9,
			Platform: PlatformAnthropic,
			Type:     AccountTypeKiro,
			Credentials: map[string]any{
				"access_token":  "at-1",
				"refresh_token": "rt-1",
				"expires_at":    time.Now().Add(1 * time.Minute).Unix(),
				"auth_method":   "social",
			},
		},
	}
	authService := &KiroAuthService{}
	provider := NewKiroTokenProvider(accountRepo, cache, authService)

	refresher := &fakeOAuthRefreshExecutor{
		cacheKey: "kiro:account:9",
		newCredentials: map[string]any{
			"access_token":  "at-2",
			"refresh_token": "rt-2",
			"expires_at":    time.Now().Add(55 * time.Minute).Unix(),
			"auth_method":   "social",
		},
	}

	provider.SetRefreshAPI(NewOAuthRefreshAPI(accountRepo, cache), refresher)
	provider.SetRefreshPolicy(KiroProviderRefreshPolicy())

	token, err := provider.GetAccessToken(context.Background(), &Account{
		ID:       9,
		Platform: PlatformAnthropic,
		Type:     AccountTypeKiro,
		Credentials: map[string]any{
			"access_token": "at-1",
			"expires_at":   time.Now().Add(1 * time.Minute).Unix(),
		},
	})

	require.NoError(t, err)
	require.Equal(t, "at-2", token)
}

type fakeOAuthRefreshExecutor struct {
	cacheKey       string
	newCredentials map[string]any
}

func (f *fakeOAuthRefreshExecutor) CacheKey(account *Account) string {
	return f.cacheKey
}

func (f *fakeOAuthRefreshExecutor) CanRefresh(account *Account) bool {
	return true
}

func (f *fakeOAuthRefreshExecutor) NeedsRefresh(account *Account, refreshWindow time.Duration) bool {
	return true
}

func (f *fakeOAuthRefreshExecutor) Refresh(ctx context.Context, account *Account) (map[string]any, error) {
	return f.newCredentials, nil
}
```

- [ ] **Step 2: 跑 provider / refresher 测试，确认红灯**

Run:

```bash
cd backend && go test -tags=unit ./internal/service -run 'TestKiroToken(Refresher|Provider)_' -count=1
```

Expected:

```text
FAIL
undefined: NewKiroTokenRefresher
undefined: NewKiroTokenProvider
undefined: KiroProviderRefreshPolicy
```

- [ ] **Step 3: 实现 refresher/provider，并注入到 GatewayService、AccountTestService、Wire**

新增 `backend/internal/service/kiro_token_refresher.go`：

```go
package service

import (
	"context"
	"time"
)

type KiroTokenRefresher struct {
	authService *KiroAuthService
}

func NewKiroTokenRefresher(authService *KiroAuthService) *KiroTokenRefresher {
	return &KiroTokenRefresher{authService: authService}
}

func (r *KiroTokenRefresher) CacheKey(account *Account) string {
	return KiroTokenCacheKey(account)
}

func (r *KiroTokenRefresher) CanRefresh(account *Account) bool {
	return account != nil && account.IsKiro()
}

func (r *KiroTokenRefresher) NeedsRefresh(account *Account, refreshWindow time.Duration) bool {
	expiresAt := account.GetCredentialAsTime("expires_at")
	if expiresAt == nil {
		return true
	}
	return time.Until(*expiresAt) < refreshWindow
}

func (r *KiroTokenRefresher) Refresh(ctx context.Context, account *Account) (map[string]any, error) {
	info, err := r.authService.RefreshAccountToken(ctx, account)
	if err != nil {
		return nil, err
	}
	creds, parseErr := ParseKiroCredentials(account)
	if parseErr != nil {
		return nil, parseErr
	}
	next := MergeCredentials(account.Credentials, map[string]any{
		"access_token":  info.AccessToken,
		"refresh_token": info.RefreshToken,
		"expires_at":    info.ExpiresAt.Unix(),
		"profile_arn":   info.ProfileARN,
		"auth_method":   creds.AuthMethod,
		"machine_id":    creds.MachineID,
	})
	return next, nil
}
```

新增 `backend/internal/service/kiro_token_provider.go`：

```go
package service

import (
	"context"
	"errors"
	"strings"
	"time"
)

const (
	kiroTokenRefreshSkew = 3 * time.Minute
	kiroTokenCacheSkew   = 5 * time.Minute
	kiroLockWaitTime     = 200 * time.Millisecond
)

type KiroTokenProvider struct {
	accountRepo   AccountRepository
	tokenCache    GeminiTokenCache
	authService   *KiroAuthService
	refreshAPI    *OAuthRefreshAPI
	executor      OAuthRefreshExecutor
	refreshPolicy ProviderRefreshPolicy
}

func NewKiroTokenProvider(accountRepo AccountRepository, tokenCache GeminiTokenCache, authService *KiroAuthService) *KiroTokenProvider {
	return &KiroTokenProvider{
		accountRepo:   accountRepo,
		tokenCache:    tokenCache,
		authService:   authService,
		refreshPolicy: KiroProviderRefreshPolicy(),
	}
}

func (p *KiroTokenProvider) SetRefreshAPI(api *OAuthRefreshAPI, executor OAuthRefreshExecutor) {
	p.refreshAPI = api
	p.executor = executor
}

func (p *KiroTokenProvider) SetRefreshPolicy(policy ProviderRefreshPolicy) {
	p.refreshPolicy = policy
}

func (p *KiroTokenProvider) GetAccessToken(ctx context.Context, account *Account) (string, error) {
	if account == nil || !account.IsKiro() {
		return "", errors.New("not a kiro account")
	}

	cacheKey := KiroTokenCacheKey(account)
	if p.tokenCache != nil {
		if token, err := p.tokenCache.GetAccessToken(ctx, cacheKey); err == nil && strings.TrimSpace(token) != "" {
			return token, nil
		}
	}

	expiresAt := account.GetCredentialAsTime("expires_at")
	needsRefresh := expiresAt == nil || time.Until(*expiresAt) <= kiroTokenRefreshSkew
	refreshFailed := false

	if needsRefresh && p.refreshAPI != nil && p.executor != nil {
		result, err := p.refreshAPI.RefreshIfNeeded(ctx, account, p.executor, kiroTokenRefreshSkew)
		if err != nil {
			if p.refreshPolicy.OnRefreshError == ProviderRefreshErrorReturn {
				return "", err
			}
			refreshFailed = true
		} else if result.LockHeld && p.tokenCache != nil {
			time.Sleep(kiroLockWaitTime)
			if token, cacheErr := p.tokenCache.GetAccessToken(ctx, cacheKey); cacheErr == nil && strings.TrimSpace(token) != "" {
				return token, nil
			}
		} else if result.Account != nil {
			account = result.Account
			expiresAt = account.GetCredentialAsTime("expires_at")
		}
	}

	token := strings.TrimSpace(account.GetCredential("access_token"))
	if token == "" {
		return "", errors.New("access_token not found in credentials")
	}

	if p.tokenCache != nil {
		ttl := 30 * time.Minute
		if refreshFailed {
			ttl = p.refreshPolicy.FailureTTL
		} else if expiresAt != nil {
			until := time.Until(*expiresAt)
			if until > kiroTokenCacheSkew {
				ttl = until - kiroTokenCacheSkew
			} else if until > 0 {
				ttl = until
			}
		}
		_ = p.tokenCache.SetAccessToken(ctx, cacheKey, token, ttl)
	}

	return token, nil
}
```

在 `backend/internal/service/refresh_policy.go` 里新增：

```go
func KiroProviderRefreshPolicy() ProviderRefreshPolicy {
	return ProviderRefreshPolicy{
		OnRefreshError: ProviderRefreshErrorUseExistingToken,
		OnLockHeld:     ProviderLockHeldWaitForCache,
		FailureTTL:     time.Minute,
	}
}
```

在 `backend/internal/service/token_refresh_service.go` 注册：

```go
kiroRefresher := NewKiroTokenRefresher(kiroAuthService)

s.refreshers = []TokenRefresher{
	claudeRefresher,
	openAIRefresher,
	geminiRefresher,
	agRefresher,
	kiroRefresher,
}

s.executors = []OAuthRefreshExecutor{
	claudeRefresher,
	openAIRefresher,
	geminiRefresher,
	agRefresher,
	kiroRefresher,
}
```

把 `backend/internal/service/gateway_service.go` 的构造器和 token 获取扩成：

```go
type GatewayService struct {
	claudeTokenProvider *ClaudeTokenProvider
	kiroTokenProvider   *KiroTokenProvider
}
```

```go
case AccountTypeKiro:
	if s.kiroTokenProvider == nil {
		return "", "", errors.New("kiro token provider is not configured")
	}
	token, err := s.kiroTokenProvider.GetAccessToken(ctx, account)
	return token, "kiro", err
```

把 `backend/internal/service/account_test_service.go` 构造器扩成：

```go
type AccountTestService struct {
	accountRepo         AccountRepository
	geminiTokenProvider *GeminiTokenProvider
	kiroTokenProvider   *KiroTokenProvider
}
```

在 `backend/internal/service/wire.go` 加 provider：

```go
func ProvideKiroTokenProvider(
	accountRepo AccountRepository,
	tokenCache GeminiTokenCache,
	kiroAuthService *KiroAuthService,
	refreshAPI *OAuthRefreshAPI,
) *KiroTokenProvider {
	p := NewKiroTokenProvider(accountRepo, tokenCache, kiroAuthService)
	executor := NewKiroTokenRefresher(kiroAuthService)
	p.SetRefreshAPI(refreshAPI, executor)
	p.SetRefreshPolicy(KiroProviderRefreshPolicy())
	return p
}
```

并把 `NewKiroAuthService` / `ProvideKiroTokenProvider` 加入 `ProviderSet`，最后生成：

```bash
cd backend && go generate ./cmd/server
```

- [ ] **Step 4: 跑 provider 测试并验证 Wire 代码可生成**

Run:

```bash
cd backend && go test -tags=unit ./internal/service -run 'TestKiroToken(Refresher|Provider)_' -count=1
cd backend && go generate ./cmd/server
cd backend && go test -tags=unit ./cmd/server -run TestDummy -count=1
```

Expected:

```text
ok  	github.com/Wei-Shaw/sub2api/internal/service
go generate finished with no diff except wire_gen.go
ok  	github.com/Wei-Shaw/sub2api/cmd/server [no tests to run]
```

- [ ] **Step 5: 提交刷新/热路径接入**

```bash
git add backend/internal/service/kiro_token_refresher.go \
  backend/internal/service/kiro_token_provider.go \
  backend/internal/service/kiro_token_provider_test.go \
  backend/internal/service/refresh_policy.go \
  backend/internal/service/token_refresher.go \
  backend/internal/service/token_refresh_service.go \
  backend/internal/service/account_test_service.go \
  backend/internal/service/gateway_service.go \
  backend/internal/service/wire.go \
  backend/cmd/server/wire_gen.go
git commit -m "feat: wire kiro token refresh pipeline"
```

### Task 5: 实现 Kiro 测试连接

**Files:**
- Create: `backend/internal/service/account_test_service_kiro.go`
- Create: `backend/internal/service/account_test_service_kiro_test.go`
- Modify: `backend/internal/service/account_test_service.go`

- [ ] **Step 1: 先写会失败的 Kiro 测试连接用例**

新增 `backend/internal/service/account_test_service_kiro_test.go`：

```go
//go:build unit

package service

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAccountTestService_KiroSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()

	resp := newJSONResponse(http.StatusOK, `{"limits":[{"resourceType":"AGENTIC_REQUEST"}]}`)
	upstream := &queuedHTTPUpstream{responses: []*http.Response{resp}}
	svc := &AccountTestService{
		httpUpstream: upstream,
		kiroTokenProvider: &KiroTokenProvider{},
	}
	account := &Account{
		ID:          101,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeKiro,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":  "at-1",
			"refresh_token": "rt-1",
			"auth_method":   "social",
			"auth_region":   "us-east-1",
			"api_region":    "us-west-2",
		},
	}

	err := svc.testKiroAccountConnection(ctx, account)
	require.NoError(t, err)
	require.Contains(t, upstream.requests[0].URL.String(), "/getUsageLimits")
	require.Equal(t, "Bearer at-1", upstream.requests[0].Header.Get("Authorization"))
	require.Contains(t, recorder.Body.String(), "test_complete")
}

func TestAccountTestService_KiroUpstreamError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := newTestContext()

	resp := newJSONResponse(http.StatusForbidden, `{"message":"denied"}`)
	upstream := &queuedHTTPUpstream{responses: []*http.Response{resp}}
	svc := &AccountTestService{
		httpUpstream: upstream,
		kiroTokenProvider: &KiroTokenProvider{},
	}
	account := &Account{
		ID:          102,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeKiro,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":  "at-1",
			"refresh_token": "rt-1",
			"auth_method":   "social",
		},
	}

	err := svc.testKiroAccountConnection(ctx, account)
	require.Error(t, err)
}
```

- [ ] **Step 2: 跑会失败的测试**

Run:

```bash
cd backend && go test -tags=unit ./internal/service -run 'TestAccountTestService_Kiro' -count=1
```

Expected:

```text
FAIL
svc.testKiroAccountConnection undefined
```

- [ ] **Step 3: 实现 Kiro 测试连接分支**

新增 `backend/internal/service/account_test_service_kiro.go`：

```go
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (s *AccountTestService) testKiroAccountConnection(c *gin.Context, account *Account) error {
	ctx := c.Request.Context()
	token, err := s.kiroTokenProvider.GetAccessToken(ctx, account)
	if err != nil {
		return s.sendErrorAndEnd(c, err.Error())
	}

	creds, err := ParseKiroCredentials(account)
	if err != nil {
		return s.sendErrorAndEnd(c, err.Error())
	}

	reqURL := fmt.Sprintf("https://q.%s.amazonaws.com/getUsageLimits?origin=AI_EDITOR&resourceType=AGENTIC_REQUEST", creds.EffectiveAPIRegion())
	if creds.ProfileARN != "" {
		u, _ := url.Parse(reqURL)
		q := u.Query()
		q.Set("profileArn", creds.ProfileARN)
		u.RawQuery = q.Encode()
		reqURL = u.String()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return s.sendErrorAndEnd(c, err.Error())
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("x-amz-user-agent", "aws-sdk-js/3.980.0 KiroIDE")
	req.Header.Set("user-agent", fmt.Sprintf("KiroIDE-dev-%s", creds.MachineID))
	req.Header.Set("host", req.URL.Host)
	req.Header.Set("amz-sdk-invocation-id", uuid.NewString())
	req.Header.Set("amz-sdk-request", "attempt=1; max=4")

	resp, err := s.httpUpstream.DoWithTLS(req, "", account.ID, account.Concurrency, nil)
	if err != nil {
		return s.sendErrorAndEnd(c, err.Error())
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return s.sendErrorAndEnd(c, fmt.Sprintf("kiro upstream error: %d", resp.StatusCode))
	}

	var payload map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&payload)
	s.sendEvent(c, TestEvent{Type: "status", Status: "kiro_ok", Success: true, Data: payload})
	s.sendEvent(c, TestEvent{Type: "test_complete", Success: true})
	return nil
}
```

在 `backend/internal/service/account_test_service.go` 的路由里，在普通 Claude 分支前加：

```go
if account.IsKiro() {
	return s.testKiroAccountConnection(c, account)
}
```

- [ ] **Step 4: 跑测试连接测试，确认通过**

Run:

```bash
cd backend && go test -tags=unit ./internal/service -run 'TestAccountTestService_Kiro' -count=1
```

Expected:

```text
ok  	github.com/Wei-Shaw/sub2api/internal/service
```

- [ ] **Step 5: 提交 Kiro 测试连接**

```bash
git add backend/internal/service/account_test_service.go \
  backend/internal/service/account_test_service_kiro.go \
  backend/internal/service/account_test_service_kiro_test.go
git commit -m "feat: add kiro account test connection"
```

### Task 6: 实现 Anthropic → Kiro 请求转换和非流式转发

**Files:**
- Create: `backend/internal/service/kiro_converter.go`
- Create: `backend/internal/service/kiro_converter_test.go`
- Create: `backend/internal/service/gateway_service_kiro.go`
- Create: `backend/internal/service/gateway_service_kiro_test.go`
- Modify: `backend/internal/service/gateway_service.go`

- [ ] **Step 1: 先写请求转换和非流式转发测试**

新增 `backend/internal/service/kiro_converter_test.go`：

```go
//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildKiroGenerateRequest_UsesMetadataSessionID(t *testing.T) {
	t.Parallel()

	body := []byte(`{
	  "model":"claude-sonnet-4-5",
	  "metadata":{"user_id":"user_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa_account__session_550e8400-e29b-41d4-a716-446655440000"},
	  "messages":[
	    {"role":"user","content":[{"type":"text","text":"hello"}]}
	  ],
	  "stream":false
	}`)
	account := &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeKiro,
		Credentials: map[string]any{
			"profile_arn": "arn:aws:kiro:::profile/default",
			"model_mapping": map[string]any{
				"claude-sonnet-4-5": "kiro-sonnet",
			},
		},
	}

	raw, mappedModel, err := BuildKiroGenerateRequest(body, account)
	require.NoError(t, err)
	require.Equal(t, "kiro-sonnet", mappedModel)
	require.Contains(t, string(raw), `"conversationId":"550e8400-e29b-41d4-a716-446655440000"`)
	require.Contains(t, string(raw), `"profileArn":"arn:aws:kiro:::profile/default"`)
	require.Contains(t, string(raw), `"agentTaskType":"vibe"`)
	require.Contains(t, string(raw), `"origin":"AI_EDITOR"`)
}
```

在 `backend/internal/service/gateway_service_kiro_test.go` 里加入：

```go
//go:build unit

package service

import (
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGatewayService_GetAccessToken_Kiro(t *testing.T) {
	t.Parallel()

	provider := NewKiroTokenProvider(nil, nil, nil)
	svc := &GatewayService{kiroTokenProvider: provider}
	account := &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeKiro,
		Credentials: map[string]any{
			"access_token": "at-1",
		},
	}

	token, tokenType, err := svc.GetAccessToken(t.Context(), account)
	require.NoError(t, err)
	require.Equal(t, "at-1", token)
	require.Equal(t, "kiro", tokenType)
}

func TestForwardKiro_NonStreamingSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()

	upstream := &queuedHTTPUpstream{
		responses: []*http.Response{
			newJSONResponse(http.StatusOK, `{"content":"hello from kiro","usage":{"inputTokens":11,"outputTokens":7}}`),
		},
	}
	svc := &GatewayService{
		httpUpstream: upstream,
		kiroTokenProvider: NewKiroTokenProvider(nil, nil, nil),
	}
	account := &Account{
		ID:          300,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeKiro,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "at-1",
			"refresh_token": "rt-1",
			"auth_method": "social",
			"api_region": "us-east-1",
		},
	}
	parsed := &ParsedRequest{
		Body: []byte(`{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"stream":false}`),
		Model: "claude-sonnet-4-5",
		Stream: false,
	}

	result, err := svc.forwardKiro(ctx.Request.Context(), ctx, account, parsed, time.Now())
	require.NoError(t, err)
	require.Equal(t, 11, result.Usage.InputTokens)
	require.Equal(t, 7, result.Usage.OutputTokens)
	require.Contains(t, recorder.Body.String(), `"type":"message"`)
}
```

- [ ] **Step 2: 跑会失败的转换/非流式测试**

Run:

```bash
cd backend && go test -tags=unit ./internal/service -run 'Test(BuildKiroGenerateRequest|GatewayService_GetAccessToken_Kiro|ForwardKiro_NonStreamingSuccess)' -count=1
```

Expected:

```text
FAIL
undefined: BuildKiroGenerateRequest
undefined: (*GatewayService).forwardKiro
```

- [ ] **Step 3: 实现 converter 和非流式 forwardKiro**

新增 `backend/internal/service/kiro_converter.go`：

```go
package service

import (
	"encoding/json"
	"strings"
)

type kiroGenerateRequest struct {
	Model             string                 `json:"model,omitempty"`
	ConversationState map[string]any         `json:"conversationState"`
	ProfileARN        string                 `json:"profileArn,omitempty"`
}

func BuildKiroGenerateRequest(body []byte, account *Account) ([]byte, string, error) {
	var in struct {
		Model    string `json:"model"`
		Metadata struct {
			UserID string `json:"user_id"`
		} `json:"metadata"`
		Messages []map[string]any `json:"messages"`
	}
	if err := json.Unmarshal(body, &in); err != nil {
		return nil, "", err
	}

	mappedModel := account.GetMappedModel(in.Model)
	conversationID := ""
	if parsed := ParseMetadataUserID(strings.TrimSpace(in.Metadata.UserID)); parsed != nil {
		conversationID = parsed.SessionID
	}

	history := make([]map[string]any, 0, len(in.Messages))
	for idx, message := range in.Messages {
		content, _ := message["content"].([]any)
		text := ""
		if len(content) > 0 {
			if item, ok := content[0].(map[string]any); ok {
				text, _ = item["text"].(string)
			}
		}
		entry := map[string]any{
			"role":          message["role"],
			"text":          text,
			"origin":        "AI_EDITOR",
			"agentTaskType": "vibe",
		}
		if idx == len(in.Messages)-1 {
			raw, err := json.Marshal(kiroGenerateRequest{
				Model: mappedModel,
				ConversationState: map[string]any{
					"conversationId": conversationID,
					"history":        history,
					"currentMessage": entry,
				},
				ProfileARN: strings.TrimSpace(account.GetCredential("profile_arn")),
			})
			return raw, mappedModel, err
		}
		history = append(history, entry)
	}

	raw, err := json.Marshal(kiroGenerateRequest{
		Model: mappedModel,
		ConversationState: map[string]any{
			"conversationId": conversationID,
			"history":        history,
		},
		ProfileARN: strings.TrimSpace(account.GetCredential("profile_arn")),
	})
	return raw, mappedModel, err
}
```

新增 `backend/internal/service/gateway_service_kiro.go`：

```go
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/google/uuid"
)

func (s *GatewayService) forwardKiro(ctx context.Context, c *gin.Context, account *Account, parsed *ParsedRequest, startTime time.Time) (*ForwardResult, error) {
	token, tokenType, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, err
	}
	if tokenType != "kiro" {
		return nil, fmt.Errorf("expected kiro token, got %s", tokenType)
	}

	creds, err := ParseKiroCredentials(account)
	if err != nil {
		return nil, err
	}
	requestBody, mappedModel, err := BuildKiroGenerateRequest(parsed.Body, account)
	if err != nil {
		return nil, err
	}

	targetURL := fmt.Sprintf("https://q.%s.amazonaws.com/generateAssistantResponse", creds.EffectiveAPIRegion())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(requestBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("x-amzn-codewhisperer-optout", "true")
	req.Header.Set("x-amzn-kiro-agent-mode", "vibe")
	req.Header.Set("x-amz-user-agent", "aws-sdk-js/3.980.0 KiroIDE")
	req.Header.Set("user-agent", fmt.Sprintf("KiroIDE-dev-%s", creds.MachineID))
	req.Header.Set("host", req.URL.Host)
	req.Header.Set("amz-sdk-invocation-id", uuid.NewString())
	req.Header.Set("amz-sdk-request", "attempt=1; max=4")
	req.Header.Set("Connection", "close")

	var tlsProfile *tlsfingerprint.Profile
	if s.tlsFPProfileService != nil {
		tlsProfile = s.tlsFPProfileService.ResolveTLSProfile(account)
	}

	resp, err := s.httpUpstream.DoWithTLS(req, "", account.ID, account.Concurrency, tlsProfile)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("kiro upstream error: %d", resp.StatusCode)
	}

	if parsed.Stream {
		return s.handleKiroStreamingResponse(ctx, c, account, resp, parsed.Model, mappedModel, startTime)
	}
	return s.handleKiroNonStreamingResponse(c, resp, parsed.Model, mappedModel, startTime)
}

func (s *GatewayService) handleKiroNonStreamingResponse(c *gin.Context, resp *http.Response, requestModel string, upstreamModel string, startTime time.Time) (*ForwardResult, error) {
	var raw struct {
		Content string `json:"content"`
		Usage struct {
			InputTokens  int `json:"inputTokens"`
			OutputTokens int `json:"outputTokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	if c != nil {
		c.JSON(http.StatusOK, gin.H{
			"type":  "message",
			"model": requestModel,
			"content": []gin.H{
				{"type": "text", "text": raw.Content},
			},
			"usage": gin.H{
				"input_tokens":  raw.Usage.InputTokens,
				"output_tokens": raw.Usage.OutputTokens,
			},
		})
	}

	return &ForwardResult{
		RequestID:     resp.Header.Get("x-amzn-requestid"),
		Usage:         ClaudeUsage{InputTokens: raw.Usage.InputTokens, OutputTokens: raw.Usage.OutputTokens},
		Model:         requestModel,
		UpstreamModel: upstreamModel,
		Stream:        false,
		Duration:      time.Since(startTime),
	}, nil
}
```

在 `backend/internal/service/gateway_service.go` 选路处增加：

```go
if account != nil && account.IsKiro() {
	return s.forwardKiro(ctx, c, account, parsed, startTime)
}
```

- [ ] **Step 4: 跑转换和非流式转发测试**

Run:

```bash
cd backend && go test -tags=unit ./internal/service -run 'Test(BuildKiroGenerateRequest|GatewayService_GetAccessToken_Kiro|ForwardKiro_NonStreamingSuccess)' -count=1
```

Expected:

```text
ok  	github.com/Wei-Shaw/sub2api/internal/service
```

- [ ] **Step 5: 提交 Kiro converter 和非流式转发**

```bash
git add backend/internal/service/kiro_converter.go \
  backend/internal/service/kiro_converter_test.go \
  backend/internal/service/gateway_service_kiro.go \
  backend/internal/service/gateway_service_kiro_test.go \
  backend/internal/service/gateway_service.go
git commit -m "feat: add kiro gateway forwarding"
```

### Task 7: 实现 Kiro 流式响应适配

**Files:**
- Create: `backend/internal/service/kiro_stream_adapter.go`
- Create: `backend/internal/service/kiro_stream_adapter_test.go`
- Modify: `backend/internal/service/gateway_service_kiro.go`
- Modify: `backend/internal/service/gateway_service_kiro_test.go`

- [ ] **Step 1: 先写流式适配的失败测试**

新增 `backend/internal/service/kiro_stream_adapter_test.go`：

```go
//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKiroStreamAdapter_AssistantResponseToAnthropicSSE(t *testing.T) {
	t.Parallel()

	adapter := NewKiroStreamAdapter("claude-sonnet-4-5")
	events, usage, err := adapter.ProcessEvent(map[string]any{
		"type": "assistantResponseEvent",
		"content": "hello",
	})
	require.NoError(t, err)
	require.Nil(t, usage)
	require.Len(t, events, 3)
	require.Contains(t, events[0], "event: message_start")
	require.Contains(t, events[1], "event: content_block_delta")
	require.Contains(t, events[2], "hello")
}

func TestKiroStreamAdapter_ToolUseSetsStopReason(t *testing.T) {
	t.Parallel()

	adapter := NewKiroStreamAdapter("claude-sonnet-4-5")
	_, _, _ = adapter.ProcessEvent(map[string]any{
		"type": "assistantResponseEvent",
		"content": "before tool",
	})

	events, usage, err := adapter.ProcessEvent(map[string]any{
		"type": "toolUseEvent",
		"name": "search",
		"toolUseId": "tool_1",
		"input": map[string]any{"query": "hello"},
	})
	require.NoError(t, err)
	require.Nil(t, usage)
	require.Contains(t, events[len(events)-1], `"tool_use"`)
}

func TestKiroStreamAdapter_ContextUsageAccumulatesUsage(t *testing.T) {
	t.Parallel()

	adapter := NewKiroStreamAdapter("claude-sonnet-4-5")
	_, _, err := adapter.ProcessEvent(map[string]any{
		"type": "contextUsageEvent",
		"contextUsagePercentage": 50.0,
	})
	require.NoError(t, err)

	finalEvents, usage, err := adapter.Finalize()
	require.NoError(t, err)
	require.NotNil(t, usage)
	require.NotZero(t, usage.InputTokens)
	require.Contains(t, finalEvents[len(finalEvents)-1], "event: message_stop")
}
```

- [ ] **Step 2: 跑流式测试，确认红灯**

Run:

```bash
cd backend && go test -tags=unit ./internal/service -run 'TestKiroStreamAdapter_' -count=1
```

Expected:

```text
FAIL
undefined: NewKiroStreamAdapter
```

- [ ] **Step 3: 实现 stream adapter，并接到 forwardKiro 的 streaming 分支**

新增 `backend/internal/service/kiro_stream_adapter.go`：

```go
package service

import (
	"encoding/json"
	"fmt"
)

type KiroStreamAdapter struct {
	model         string
	messageStarted bool
	textIndex     int
	stopReason    string
	inputTokens   int
	outputTokens  int
}

func NewKiroStreamAdapter(model string) *KiroStreamAdapter {
	return &KiroStreamAdapter{
		model: model,
	}
}

func (a *KiroStreamAdapter) ProcessEvent(event map[string]any) ([]string, *ClaudeUsage, error) {
	eventType, _ := event["type"].(string)
	switch eventType {
	case "assistantResponseEvent":
		content, _ := event["content"].(string)
		return []string{
			fmt.Sprintf("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"model\":%q}}\n", a.model),
			"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n",
			fmt.Sprintf("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":%q}}\n", content),
		}, nil, nil
	case "toolUseEvent":
		a.stopReason = "tool_use"
		rawInput, _ := json.Marshal(event["input"])
		return []string{
			fmt.Sprintf("event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":1,\"content_block\":{\"type\":\"tool_use\",\"id\":%q,\"name\":%q,\"input\":%s}}\n",
				event["toolUseId"], event["name"], string(rawInput)),
		}, nil, nil
	case "contextUsageEvent":
		if percent, ok := event["contextUsagePercentage"].(float64); ok {
			a.inputTokens = int(percent * 200000 / 100)
		}
		return nil, nil, nil
	case "exception":
		return nil, nil, fmt.Errorf("kiro exception: %v", event["message"])
	default:
		return nil, nil, nil
	}
}

func (a *KiroStreamAdapter) Finalize() ([]string, *ClaudeUsage, error) {
	if a.stopReason == "" {
		a.stopReason = "end_turn"
	}
	return []string{
		fmt.Sprintf("event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":%q}}\n", a.stopReason),
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n",
	}, &ClaudeUsage{
		InputTokens:  a.inputTokens,
		OutputTokens: a.outputTokens,
	}, nil
}
```

在 `backend/internal/service/gateway_service_kiro.go` 里追加：

```go
func (s *GatewayService) handleKiroStreamingResponse(ctx context.Context, c *gin.Context, account *Account, resp *http.Response, requestModel string, upstreamModel string, startTime time.Time) (*ForwardResult, error) {
	adapter := NewKiroStreamAdapter(requestModel)
	decoder := json.NewDecoder(resp.Body)
	usage := &ClaudeUsage{}

	for decoder.More() {
		var event map[string]any
		if err := decoder.Decode(&event); err != nil {
			return nil, err
		}
		chunks, partialUsage, err := adapter.ProcessEvent(event)
		if err != nil {
			return nil, err
		}
		for _, chunk := range chunks {
			_, _ = c.Writer.Write([]byte(chunk + "\n"))
		}
		if partialUsage != nil {
			usage = partialUsage
		}
	}

	finalChunks, finalUsage, err := adapter.Finalize()
	if err != nil {
		return nil, err
	}
	for _, chunk := range finalChunks {
		_, _ = c.Writer.Write([]byte(chunk + "\n"))
	}
	if finalUsage != nil {
		usage = finalUsage
	}

	return &ForwardResult{
		RequestID:     resp.Header.Get("x-amzn-requestid"),
		Usage:         *usage,
		Model:         requestModel,
		UpstreamModel: upstreamModel,
		Stream:        true,
		Duration:      time.Since(startTime),
	}, nil
}
```

- [ ] **Step 4: 跑流式适配测试**

Run:

```bash
cd backend && go test -tags=unit ./internal/service -run 'TestKiroStreamAdapter_' -count=1
```

Expected:

```text
ok  	github.com/Wei-Shaw/sub2api/internal/service
```

- [ ] **Step 5: 提交流式适配**

```bash
git add backend/internal/service/kiro_stream_adapter.go \
  backend/internal/service/kiro_stream_adapter_test.go \
  backend/internal/service/gateway_service_kiro.go \
  backend/internal/service/gateway_service_kiro_test.go
git commit -m "feat: add kiro stream adapter"
```

### Task 8: 全量回归、生成代码和交付前验证

**Files:**
- Modify: `backend/cmd/server/wire_gen.go`
- Verify: `backend/internal/service`
- Verify: `frontend/src/components/account`

- [ ] **Step 1: 运行后端单元测试**

Run:

```bash
cd backend && go test -tags=unit ./...
```

Expected:

```text
ok  	github.com/Wei-Shaw/sub2api/internal/service
ok  	github.com/Wei-Shaw/sub2api/internal/handler/admin
ok  	github.com/Wei-Shaw/sub2api/internal/handler/dto
```

- [ ] **Step 2: 运行前端测试与类型检查**

Run:

```bash
cd frontend && pnpm test:run
cd frontend && pnpm typecheck
```

Expected:

```text
All frontend tests passed
Type checking completed with 0 errors
```

- [ ] **Step 3: 运行后端集成测试和 lint**

Run:

```bash
cd backend && go test -tags=integration ./...
cd backend && golangci-lint run ./...
```

Expected:

```text
ok  	github.com/Wei-Shaw/sub2api/internal/repository
ok  	github.com/Wei-Shaw/sub2api/internal/integration
golangci-lint: no issues found
```

- [ ] **Step 4: 最后一次生成 Wire 并检查工作树**

Run:

```bash
cd backend && go generate ./cmd/server
git status --short
```

Expected:

```text
仅剩本次实现相关文件，无意外脏文件
```

- [ ] **Step 5: 提交完整功能**

```bash
git add frontend/src backend/internal backend/cmd/server/wire_gen.go
git commit -m "feat: add anthropic kiro account support"
```

## Manual Smoke Checklist

1. 启动本地依赖：`docker start apipool-postgres apipool-redis`
2. 启动后端：`cd backend && DATABASE_HOST=127.0.0.1 DATABASE_PORT=5432 DATABASE_USER=sub2api DATABASE_PASSWORD=sub2api DATABASE_DBNAME=sub2api REDIS_HOST=127.0.0.1 REDIS_PORT=6379 SERVER_MODE=debug SERVER_PORT=8080 go run ./cmd/server/`
3. 启动前端：`cd frontend && pnpm install && pnpm dev`
4. 用 `admin@apipool.local / admin123` 登录后台，在 Anthropic 平台创建一条 `social` Kiro 账号并执行“测试连接”。
5. 创建一条 `idc` Kiro 账号并执行“测试连接”，确认 `client_id` / `client_secret` 分支也通。
6. 使用真实 `/v1/messages` 请求打到本地网关，确认：
   - 非流式返回可以拿到 `type = message`
   - 流式返回能收到 `message_start -> content_block_delta -> message_stop`
   - 账号 token 过期后会自动刷新
7. 在数据库或后台手动把 `expires_at` 改到临近过期，复测热路径刷新和后台刷新不会互相打架。
8. 用失效的 refresh token 复测，确认系统能暴露错误，且不会无限重试。

## Notes

- IdC 刷新首版按 `kiro.rs` 当前 JSON 请求实现，不默认切到 `application/x-www-form-urlencoded`；若联调时真实上游返回 `415 Unsupported Media Type`，再在 `KiroAuthService.refreshIDC` 增加 fallback。
- 流式映射表以 `assistantResponseEvent`、`toolUseEvent`、`contextUsageEvent` 为主，不把 `codeEvent` 这种当前参考实现没有的名字写死进实现。
- Kiro 首版不扩展自定义错误码能力；前后端都必须用能力判断，而不是把 `kiro` 混进 “apikey 特例”。
