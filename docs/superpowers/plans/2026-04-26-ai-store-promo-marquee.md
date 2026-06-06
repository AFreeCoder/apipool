# AI Store Promo Marquee Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an admin-configurable desktop-only AppHeader marquee that promotes the AI membership store without adding schema migrations or extra public settings requests.

**Architecture:** Store marquee settings in existing `system_settings` keys. Keep service-layer settings as raw JSON strings, expose structured DTOs in admin/public handlers, and filter disabled messages for public GET and SSR injection. Render the marquee in `AppHeader` through a new CSS-only component while editing stays in `SettingsView.vue` local form.

**Tech Stack:** Go 1.25.7, Gin, existing `SettingService`, Vue 3, Pinia, Vite/Vitest, Tailwind CSS.

---

## File Map

- Modify `backend/internal/service/domain_constants.go`: add `SettingMarqueeEnabled` and `SettingMarqueeMessages`.
- Modify `backend/internal/handler/dto/settings.go`: add `MarqueeMessage`, DTO fields, parse/filter/sort helpers.
- Modify `backend/internal/service/settings_view.go`: add raw string fields to service `SystemSettings` and `PublicSettings`.
- Modify `backend/internal/service/setting_service.go`: include new keys in get/default/update paths; add SSR injection fields and helper that filters disabled messages before injection.
- Modify `backend/internal/handler/admin/setting_handler.go`: admin GET/PUT marshalling, validation, diff logging.
- Modify `backend/internal/handler/setting_handler.go`: public GET structured marquee output.
- Modify `backend/internal/server/api_contract_test.go`: contract shape updates.
- Add or modify backend unit tests under `backend/internal/handler/dto/`, `backend/internal/service/`, `backend/internal/handler/`, and `backend/internal/handler/admin/`.
- Modify `frontend/src/types/index.ts`: add `MarqueeMessage`, public settings fields.
- Modify `frontend/src/api/admin/settings.ts`: add admin settings request/response fields.
- Modify `frontend/src/stores/app.ts`: add fallback defaults.
- Create `frontend/src/components/layout/AppHeaderMarquee.vue`: CSS-only marquee.
- Modify `frontend/src/components/layout/AppHeader.vue`: insert `<AppHeaderMarquee />` between left and right groups.
- Modify `frontend/src/views/admin/SettingsView.vue`: local form fields, UI section, add/remove/move functions, save payload.
- Modify `frontend/src/i18n/locales/zh.ts` and `frontend/src/i18n/locales/en.ts`: admin UI labels.
- Add or modify frontend tests in `frontend/src/stores/__tests__/app.spec.ts`, `frontend/src/components/layout/__tests__/AppHeaderMarquee.spec.ts`, and `frontend/src/views/admin/__tests__/SettingsView.spec.ts`.

---

## Task 1: Backend DTO And Parsing Helpers

**Files:**
- Modify: `backend/internal/handler/dto/settings.go`
- Test: `backend/internal/handler/dto/settings_marquee_test.go`

- [ ] **Step 1: Write failing DTO helper tests**

Create `backend/internal/handler/dto/settings_marquee_test.go`:

```go
//go:build unit

package dto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseMarqueeMessages_ReturnsAllValidMessages(t *testing.T) {
	raw := `[
		{"id":"b","text":" second ","enabled":false,"sort_order":1},
		{"id":"a","text":" first ","enabled":true,"sort_order":0}
	]`

	got := ParseMarqueeMessages(raw)

	require.Equal(t, []MarqueeMessage{
		{ID: "b", Text: " second ", Enabled: false, SortOrder: 1},
		{ID: "a", Text: " first ", Enabled: true, SortOrder: 0},
	}, got)
}

func TestParsePublicMarqueeMessages_FiltersAndSorts(t *testing.T) {
	raw := `[
		{"id":"draft","text":"draft","enabled":false,"sort_order":0},
		{"id":"later","text":"Later","enabled":true,"sort_order":2},
		{"id":"first","text":"First","enabled":true,"sort_order":1},
		{"id":"blank","text":"   ","enabled":true,"sort_order":3}
	]`

	got := ParsePublicMarqueeMessages(raw)

	require.Equal(t, []MarqueeMessage{
		{ID: "first", Text: "First", Enabled: true, SortOrder: 1},
		{ID: "later", Text: "Later", Enabled: true, SortOrder: 2},
	}, got)
}

func TestParseMarqueeMessages_InvalidInputReturnsEmptySlice(t *testing.T) {
	require.Empty(t, ParseMarqueeMessages(""))
	require.Empty(t, ParseMarqueeMessages("not-json"))
	require.Empty(t, ParsePublicMarqueeMessages("not-json"))
}
```

- [ ] **Step 2: Run DTO tests and verify they fail**

Run:

```bash
cd backend && go test -tags=unit ./internal/handler/dto -run 'TestParse.*Marquee' -count=1
```

Expected: FAIL with undefined `MarqueeMessage`, `ParseMarqueeMessages`, or `ParsePublicMarqueeMessages`.

- [ ] **Step 3: Implement DTO fields and helpers**

In `backend/internal/handler/dto/settings.go`, update imports:

```go
import (
	"encoding/json"
	"sort"
	"strings"
)
```

Add after `CustomEndpoint`:

```go
// MarqueeMessage represents a single AppHeader marquee message.
type MarqueeMessage struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	Enabled   bool   `json:"enabled"`
	SortOrder int    `json:"sort_order"`
}
```

Add to `SystemSettings` near `CustomMenuItems`:

```go
	MarqueeEnabled  bool             `json:"marquee_enabled"`
	MarqueeMessages []MarqueeMessage `json:"marquee_messages"`
```

Add to `PublicSettings` near `CustomMenuItems`:

```go
	MarqueeEnabled  bool             `json:"marquee_enabled"`
	MarqueeMessages []MarqueeMessage `json:"marquee_messages"`
```

Add helpers near `ParseCustomMenuItems`:

```go
// ParseMarqueeMessages parses a JSON string into a slice of MarqueeMessage.
// Returns an empty slice on empty/invalid input.
func ParseMarqueeMessages(raw string) []MarqueeMessage {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return []MarqueeMessage{}
	}
	var items []MarqueeMessage
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return []MarqueeMessage{}
	}
	return items
}

// ParsePublicMarqueeMessages returns enabled, non-empty marquee messages sorted
// by sort_order. This is the only shape exposed to public settings consumers.
func ParsePublicMarqueeMessages(raw string) []MarqueeMessage {
	items := ParseMarqueeMessages(raw)
	filtered := make([]MarqueeMessage, 0, len(items))
	for _, item := range items {
		if !item.Enabled || strings.TrimSpace(item.Text) == "" {
			continue
		}
		filtered = append(filtered, item)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].SortOrder < filtered[j].SortOrder
	})
	return filtered
}
```

- [ ] **Step 4: Run DTO tests and verify they pass**

Run:

```bash
cd backend && go test -tags=unit ./internal/handler/dto -run 'TestParse.*Marquee' -count=1
```

Expected: PASS.

---

## Task 2: Backend Service Settings And SSR Injection

**Files:**
- Modify: `backend/internal/service/domain_constants.go`
- Modify: `backend/internal/service/settings_view.go`
- Modify: `backend/internal/service/setting_service.go`
- Test: `backend/internal/service/setting_service_public_test.go`
- Test: `backend/internal/service/setting_service_update_test.go`
- Test: `backend/internal/handler/dto/public_settings_injection_schema_test.go`

- [ ] **Step 1: Write failing service tests**

Append to `backend/internal/service/setting_service_public_test.go`:

```go
func TestSettingService_GetPublicSettings_IncludesMarqueeRawSettings(t *testing.T) {
	repo := &settingPublicRepoStub{
		values: map[string]string{
			SettingMarqueeEnabled:  "true",
			SettingMarqueeMessages: `[{"id":"m1","text":"hello","enabled":true,"sort_order":0}]`,
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())

	require.NoError(t, err)
	require.True(t, settings.MarqueeEnabled)
	require.Equal(t, `[{"id":"m1","text":"hello","enabled":true,"sort_order":0}]`, settings.MarqueeMessages)
}
```

Append to `backend/internal/service/setting_service_update_test.go`:

```go
func TestSettingService_UpdateSettings_MarqueeSettingsPersisted(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		MarqueeEnabled:  true,
		MarqueeMessages: `[{"id":"m1","text":"hello","enabled":true,"sort_order":0}]`,
	})

	require.NoError(t, err)
	require.Equal(t, "true", repo.updates[SettingMarqueeEnabled])
	require.Equal(t, `[{"id":"m1","text":"hello","enabled":true,"sort_order":0}]`, repo.updates[SettingMarqueeMessages])
}
```

Add a focused SSR injection test to `backend/internal/service/setting_service_public_test.go`:

```go
func TestSettingService_GetPublicSettingsForInjection_FiltersMarqueeMessages(t *testing.T) {
	repo := &settingPublicRepoStub{
		values: map[string]string{
			SettingMarqueeEnabled: "true",
			SettingMarqueeMessages: `[
				{"id":"draft","text":"draft","enabled":false,"sort_order":0},
				{"id":"second","text":"Second","enabled":true,"sort_order":2},
				{"id":"first","text":"First","enabled":true,"sort_order":1}
			]`,
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	payload, err := svc.GetPublicSettingsForInjection(context.Background())

	require.NoError(t, err)
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	var got struct {
		MarqueeEnabled  bool `json:"marquee_enabled"`
		MarqueeMessages []struct {
			ID        string `json:"id"`
			Text      string `json:"text"`
			Enabled   bool   `json:"enabled"`
			SortOrder int    `json:"sort_order"`
		} `json:"marquee_messages"`
	}
	require.NoError(t, json.Unmarshal(raw, &got))
	require.True(t, got.MarqueeEnabled)
	require.Len(t, got.MarqueeMessages, 2)
	require.Equal(t, "first", got.MarqueeMessages[0].ID)
	require.Equal(t, "second", got.MarqueeMessages[1].ID)
}
```

Add `encoding/json` to the test file imports if it is not present.

- [ ] **Step 2: Run service tests and verify they fail**

Run:

```bash
cd backend && go test -tags=unit ./internal/service -run 'TestSettingService_.*Marquee|TestPublicSettingsInjectionPayload_SchemaDoesNotDrift' -count=1
```

Expected: FAIL because constants and fields are missing.

- [ ] **Step 3: Add constants and raw service fields**

In `backend/internal/service/domain_constants.go`, add near custom menu constants:

```go
	SettingMarqueeEnabled              = "marquee_enabled"              // 顶部跑马灯全局开关
	SettingMarqueeMessages             = "marquee_messages"             // 顶部跑马灯字幕（JSON 数组）
```

In `backend/internal/service/settings_view.go`, add to `SystemSettings` and `PublicSettings` near `CustomMenuItems`:

```go
	MarqueeEnabled  bool
	MarqueeMessages string // JSON array of marquee messages
```

- [ ] **Step 4: Wire service get/update/default paths**

In `backend/internal/service/setting_service.go`:

Add keys in `GetPublicSettings` key list near custom menu keys:

```go
		SettingMarqueeEnabled,
		SettingMarqueeMessages,
```

Map them in `GetPublicSettings` return:

```go
		MarqueeEnabled:                   settings[SettingMarqueeEnabled] == "true",
		MarqueeMessages:                  settings[SettingMarqueeMessages],
```

Map them in `parseSettings` near `CustomMenuItems`:

```go
		MarqueeEnabled:                   settings[SettingMarqueeEnabled] == "true",
		MarqueeMessages:                  settings[SettingMarqueeMessages],
```

Persist them in `buildSystemSettingsUpdates` near `CustomMenuItems`:

```go
	updates[SettingMarqueeEnabled] = strconv.FormatBool(settings.MarqueeEnabled)
	updates[SettingMarqueeMessages] = settings.MarqueeMessages
```

Add defaults in `InitializeDefaultSettings`:

```go
		SettingMarqueeEnabled:                           "false",
		SettingMarqueeMessages:                          "[]",
```

- [ ] **Step 5: Implement SSR injection fields with public filtering**

In `backend/internal/service/setting_service.go`, add to `PublicSettingsInjectionPayload` near `CustomMenuItems`:

```go
	MarqueeEnabled  bool            `json:"marquee_enabled"`
	MarqueeMessages json.RawMessage `json:"marquee_messages"`
```

Add a service-local helper near `safeRawJSONArray`:

```go
func publicMarqueeMessagesRaw(raw string) json.RawMessage {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return json.RawMessage("[]")
	}
	var items []struct {
		ID        string `json:"id"`
		Text      string `json:"text"`
		Enabled   bool   `json:"enabled"`
		SortOrder int    `json:"sort_order"`
	}
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return json.RawMessage("[]")
	}
	filtered := make([]struct {
		ID        string `json:"id"`
		Text      string `json:"text"`
		Enabled   bool   `json:"enabled"`
		SortOrder int    `json:"sort_order"`
	}, 0, len(items))
	for _, item := range items {
		if !item.Enabled || strings.TrimSpace(item.Text) == "" {
			continue
		}
		filtered = append(filtered, item)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].SortOrder < filtered[j].SortOrder
	})
	if len(filtered) == 0 {
		return json.RawMessage("[]")
	}
	result, err := json.Marshal(filtered)
	if err != nil {
		return json.RawMessage("[]")
	}
	return result
}
```

Add `sort` to the service imports if it is not already imported.

Map fields in `GetPublicSettingsForInjection` near `CustomMenuItems`:

```go
		MarqueeEnabled:                   settings.MarqueeEnabled,
		MarqueeMessages:                  publicMarqueeMessagesRaw(settings.MarqueeMessages),
```

- [ ] **Step 6: Run service and injection tests**

Run:

```bash
cd backend && go test -tags=unit ./internal/service ./internal/handler/dto -run 'TestSettingService_.*Marquee|TestPublicSettingsInjectionPayload_SchemaDoesNotDrift' -count=1
```

Expected: PASS.

---

## Task 3: Admin Handler Validation And Response

**Files:**
- Modify: `backend/internal/handler/admin/setting_handler.go`
- Test: `backend/internal/handler/admin/setting_handler_marquee_test.go`

- [ ] **Step 1: Write admin validation tests**

Create `backend/internal/handler/admin/setting_handler_marquee_test.go` with tests that exercise a pure helper. If no helper exists yet, the tests will fail until Step 3 creates it:

```go
//go:build unit

package admin

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/stretchr/testify/require"
)

func TestNormalizeMarqueeMessages_FillsIDAndSortOrder(t *testing.T) {
	raw, err := normalizeMarqueeMessages([]dto.MarqueeMessage{
		{Text: " First ", Enabled: true, SortOrder: 99},
		{ID: "existing-id", Text: "Second", Enabled: false, SortOrder: 42},
	})

	require.NoError(t, err)
	var got []dto.MarqueeMessage
	require.NoError(t, json.Unmarshal([]byte(raw), &got))
	require.Len(t, got, 2)
	require.NotEmpty(t, got[0].ID)
	require.Equal(t, " First ", got[0].Text)
	require.True(t, got[0].Enabled)
	require.Equal(t, 0, got[0].SortOrder)
	require.Equal(t, "existing-id", got[1].ID)
	require.Equal(t, 1, got[1].SortOrder)
}

func TestNormalizeMarqueeMessages_RejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name     string
		messages []dto.MarqueeMessage
	}{
		{name: "empty text", messages: []dto.MarqueeMessage{{Text: "   ", Enabled: true}}},
		{name: "too long text", messages: []dto.MarqueeMessage{{Text: string(make([]byte, 501)), Enabled: true}}},
		{name: "invalid id", messages: []dto.MarqueeMessage{{ID: "bad id", Text: "ok", Enabled: true}}},
		{name: "id too long", messages: []dto.MarqueeMessage{{ID: "abcdefghijklmnopqrstuvwxyz1234567", Text: "ok", Enabled: true}}},
		{name: "duplicate id", messages: []dto.MarqueeMessage{
			{ID: "same", Text: "one", Enabled: true},
			{ID: "same", Text: "two", Enabled: true},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := normalizeMarqueeMessages(tt.messages)
			require.Error(t, err)
		})
	}
}

func TestNormalizeMarqueeMessages_RejectsMoreThanTwentyMessages(t *testing.T) {
	messages := make([]dto.MarqueeMessage, 21)
	for i := range messages {
		messages[i] = dto.MarqueeMessage{Text: "ok", Enabled: true}
	}

	_, err := normalizeMarqueeMessages(messages)

	require.Error(t, err)
}
```

- [ ] **Step 2: Run admin tests and verify they fail**

Run:

```bash
cd backend && go test -tags=unit ./internal/handler/admin -run 'TestNormalizeMarqueeMessages' -count=1
```

Expected: FAIL with undefined `normalizeMarqueeMessages`.

- [ ] **Step 3: Implement admin helper and request fields**

In `backend/internal/handler/admin/setting_handler.go`, add request fields near `CustomMenuItems`:

```go
	MarqueeEnabled  *bool                 `json:"marquee_enabled"`
	MarqueeMessages *[]dto.MarqueeMessage `json:"marquee_messages"`
```

Add helper near `generateMenuItemID` or near the custom menu validation block:

```go
func normalizeMarqueeMessages(items []dto.MarqueeMessage) (string, error) {
	const (
		maxMarqueeMessages = 20
		maxMarqueeTextLen  = 500
		maxMarqueeIDLen    = 32
	)
	if len(items) > maxMarqueeMessages {
		return "", fmt.Errorf("too many marquee messages (max 20)")
	}
	seen := make(map[string]struct{}, len(items))
	for i, item := range items {
		if strings.TrimSpace(item.Text) == "" {
			return "", fmt.Errorf("marquee message text is required")
		}
		if len(item.Text) > maxMarqueeTextLen {
			return "", fmt.Errorf("marquee message text is too long (max 500 characters)")
		}
		if strings.TrimSpace(item.ID) == "" {
			id, err := generateMenuItemID()
			if err != nil {
				return "", fmt.Errorf("generate marquee message ID: %w", err)
			}
			items[i].ID = id
		} else if len(item.ID) > maxMarqueeIDLen {
			return "", fmt.Errorf("marquee message ID is too long (max 32 characters)")
		} else if !menuItemIDPattern.MatchString(item.ID) {
			return "", fmt.Errorf("marquee message ID contains invalid characters")
		}
		items[i].SortOrder = i
		if _, ok := seen[items[i].ID]; ok {
			return "", fmt.Errorf("duplicate marquee message ID: %s", items[i].ID)
		}
		seen[items[i].ID] = struct{}{}
	}
	data, err := json.Marshal(items)
	if err != nil {
		return "", fmt.Errorf("marshal marquee messages: %w", err)
	}
	return string(data), nil
}
```

If `fmt` is not imported in `setting_handler.go`, add it to imports.

- [ ] **Step 4: Wire admin GET/PUT and diff logging**

In `GetSettings` payload near `CustomMenuItems`:

```go
		MarqueeEnabled:                       settings.MarqueeEnabled,
		MarqueeMessages:                      dto.ParseMarqueeMessages(settings.MarqueeMessages),
```

In `UpdateSettings`, after custom menu validation:

```go
	marqueeEnabled := previousSettings.MarqueeEnabled
	if req.MarqueeEnabled != nil {
		marqueeEnabled = *req.MarqueeEnabled
	}
	marqueeMessagesJSON := previousSettings.MarqueeMessages
	if req.MarqueeMessages != nil {
		normalized, err := normalizeMarqueeMessages(*req.MarqueeMessages)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		marqueeMessagesJSON = normalized
	}
```

In the `service.SystemSettings` update payload near `CustomMenuItems`:

```go
		MarqueeEnabled:                   marqueeEnabled,
		MarqueeMessages:                  marqueeMessagesJSON,
```

In updated response payload near `CustomMenuItems`:

```go
		MarqueeEnabled:                       updatedSettings.MarqueeEnabled,
		MarqueeMessages:                      dto.ParseMarqueeMessages(updatedSettings.MarqueeMessages),
```

In changed fields detection near `custom_menu_items`:

```go
	if before.MarqueeEnabled != after.MarqueeEnabled {
		changed = append(changed, "marquee_enabled")
	}
	if before.MarqueeMessages != after.MarqueeMessages {
		changed = append(changed, "marquee_messages")
	}
```

- [ ] **Step 5: Run admin tests**

Run:

```bash
cd backend && go test -tags=unit ./internal/handler/admin -run 'TestNormalizeMarqueeMessages' -count=1
```

Expected: PASS.

---

## Task 4: Public Handler And Backend Contract

**Files:**
- Modify: `backend/internal/handler/setting_handler.go`
- Modify: `backend/internal/server/api_contract_test.go`
- Test: `backend/internal/handler/setting_handler_public_test.go`

- [ ] **Step 1: Write failing public handler test**

Append to `backend/internal/handler/setting_handler_public_test.go`:

```go
func TestSettingHandler_GetPublicSettings_FiltersMarqueeMessages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewSettingHandler(service.NewSettingService(&settingHandlerPublicRepoStub{
		values: map[string]string{
			service.SettingMarqueeEnabled: "true",
			service.SettingMarqueeMessages: `[
				{"id":"draft","text":"Draft","enabled":false,"sort_order":0},
				{"id":"second","text":"Second","enabled":true,"sort_order":2},
				{"id":"first","text":"First","enabled":true,"sort_order":1}
			]`,
		},
	}, &config.Config{}), "test-version")

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/settings/public", nil)

	h.GetPublicSettings(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Code int `json:"code"`
		Data struct {
			MarqueeEnabled  bool `json:"marquee_enabled"`
			MarqueeMessages []struct {
				ID        string `json:"id"`
				Text      string `json:"text"`
				Enabled   bool   `json:"enabled"`
				SortOrder int    `json:"sort_order"`
			} `json:"marquee_messages"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.True(t, resp.Data.MarqueeEnabled)
	require.Len(t, resp.Data.MarqueeMessages, 2)
	require.Equal(t, "first", resp.Data.MarqueeMessages[0].ID)
	require.Equal(t, "second", resp.Data.MarqueeMessages[1].ID)
}
```

- [ ] **Step 2: Run public handler test and verify it fails**

Run:

```bash
cd backend && go test -tags=unit ./internal/handler -run 'TestSettingHandler_GetPublicSettings_FiltersMarqueeMessages' -count=1
```

Expected: FAIL because public response lacks marquee fields.

- [ ] **Step 3: Wire public handler response**

In `backend/internal/handler/setting_handler.go`, add near `CustomMenuItems` in `dto.PublicSettings` construction:

```go
			MarqueeEnabled:                   settings.MarqueeEnabled,
			MarqueeMessages:                  dto.ParsePublicMarqueeMessages(settings.MarqueeMessages),
```

- [ ] **Step 4: Update API contract test fixtures**

In `backend/internal/server/api_contract_test.go`, find the expected public/admin settings JSON objects and add:

```json
"marquee_enabled": false,
"marquee_messages": []
```

For public contract cases that assert filtering, use:

```json
"marquee_enabled": true,
"marquee_messages": [
  {"id":"enabled","text":"Enabled message","enabled":true,"sort_order":0}
]
```

- [ ] **Step 5: Run public and contract tests**

Run:

```bash
cd backend && go test -tags=unit ./internal/handler ./internal/server -run 'TestSettingHandler_GetPublicSettings_FiltersMarqueeMessages|Test.*Settings.*Contract' -count=1
```

Expected: PASS. If the exact contract test name differs, run `go test -tags=unit ./internal/server -run Contract -count=1`.

---

## Task 5: Frontend Types, API Shapes, And Store Defaults

**Files:**
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/api/admin/settings.ts`
- Modify: `frontend/src/stores/app.ts`
- Test: `frontend/src/stores/__tests__/app.spec.ts`

- [ ] **Step 1: Add failing app store fallback test**

Append to `frontend/src/stores/__tests__/app.spec.ts` in the public settings section:

```ts
it('fetchPublicSettings fallback includes marquee defaults', async () => {
  vi.mocked(getPublicSettings).mockRejectedValueOnce(new Error('network'))
  const store = useAppStore()

  const settings = await store.fetchPublicSettings(true)

  expect(settings?.marquee_enabled).toBe(false)
  expect(settings?.marquee_messages).toEqual([])
  expect(store.cachedPublicSettings?.marquee_enabled).toBe(false)
  expect(store.cachedPublicSettings?.marquee_messages).toEqual([])
})
```

- [ ] **Step 2: Run app store test and verify it fails**

Run:

```bash
cd frontend && pnpm test:run src/stores/__tests__/app.spec.ts -t 'marquee defaults'
```

Expected: FAIL because `marquee_enabled` and `marquee_messages` are missing.

- [ ] **Step 3: Add frontend shared types**

In `frontend/src/types/index.ts`, add near `CustomMenuItem`:

```ts
export interface MarqueeMessage {
  id: string
  text: string
  enabled: boolean
  sort_order: number
}
```

Add to `PublicSettings`:

```ts
  marquee_enabled: boolean
  marquee_messages: MarqueeMessage[]
```

If this file defines a separate admin settings interface, add the same fields there.

- [ ] **Step 4: Add admin API request/response fields**

In `frontend/src/api/admin/settings.ts`, import `MarqueeMessage` with existing types:

```ts
import type { CustomMenuItem, CustomEndpoint, MarqueeMessage, NotifyEmailEntry } from "@/types";
```

Add to the admin settings response interface near `custom_menu_items`:

```ts
  marquee_enabled: boolean;
  marquee_messages: MarqueeMessage[];
```

Add to `UpdateSettingsRequest` near `custom_menu_items`:

```ts
  marquee_enabled?: boolean;
  marquee_messages?: MarqueeMessage[];
```

- [ ] **Step 5: Add fallback defaults**

In `frontend/src/stores/app.ts`, add to the fallback object returned in `fetchPublicSettings`:

```ts
        marquee_enabled: false,
        marquee_messages: [],
```

- [ ] **Step 6: Run frontend type/store tests**

Run:

```bash
cd frontend && pnpm test:run src/stores/__tests__/app.spec.ts -t 'marquee defaults'
cd frontend && pnpm typecheck
```

Expected: PASS.

---

## Task 6: AppHeader Marquee Component

**Files:**
- Create: `frontend/src/components/layout/AppHeaderMarquee.vue`
- Modify: `frontend/src/components/layout/AppHeader.vue`
- Test: `frontend/src/components/layout/__tests__/AppHeaderMarquee.spec.ts`

- [ ] **Step 1: Write failing component tests**

Create `frontend/src/components/layout/__tests__/AppHeaderMarquee.spec.ts`:

```ts
import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import AppHeaderMarquee from '../AppHeaderMarquee.vue'

const cachedPublicSettings = vi.hoisted(() => ({ value: null as any }))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    get cachedPublicSettings() {
      return cachedPublicSettings.value
    },
  }),
}))

describe('AppHeaderMarquee', () => {
  it('renders enabled messages as one separated marquee string', () => {
    cachedPublicSettings.value = {
      marquee_enabled: true,
      marquee_messages: [
        { id: 'a', text: ' First ', enabled: true, sort_order: 0 },
        { id: 'b', text: 'Second', enabled: true, sort_order: 1 },
      ],
    }

    const wrapper = mount(AppHeaderMarquee)

    expect(wrapper.find('.marquee-shell').exists()).toBe(true)
    expect(wrapper.find('.marquee-shell').attributes('aria-label')).toBe('First  ·  Second')
    expect(wrapper.findAll('.marquee-segment')).toHaveLength(2)
    expect(wrapper.find('.marquee-slot').classes()).toEqual(expect.arrayContaining(['hidden', 'lg:block', 'lg:flex-1']))
  })

  it('keeps the desktop slot but hides content when disabled', () => {
    cachedPublicSettings.value = {
      marquee_enabled: false,
      marquee_messages: [{ id: 'a', text: 'Hidden', enabled: true, sort_order: 0 }],
    }

    const wrapper = mount(AppHeaderMarquee)

    expect(wrapper.find('.marquee-slot').exists()).toBe(true)
    expect(wrapper.find('.marquee-shell').exists()).toBe(false)
  })

  it('pauses both marquee segments on hover', async () => {
    cachedPublicSettings.value = {
      marquee_enabled: true,
      marquee_messages: [{ id: 'a', text: 'Promo', enabled: true, sort_order: 0 }],
    }
    const wrapper = mount(AppHeaderMarquee)

    await wrapper.find('.marquee-track').trigger('mouseenter')

    expect(wrapper.findAll('.marquee-segment').every((node) => node.classes().includes('paused'))).toBe(true)
  })
})
```

- [ ] **Step 2: Run component test and verify it fails**

Run:

```bash
cd frontend && pnpm test:run src/components/layout/__tests__/AppHeaderMarquee.spec.ts
```

Expected: FAIL because component does not exist.

- [ ] **Step 3: Create `AppHeaderMarquee.vue`**

Create `frontend/src/components/layout/AppHeaderMarquee.vue`:

```vue
<template>
  <div class="marquee-slot hidden min-w-0 mx-4 lg:block lg:flex-1">
    <div
      v-if="text"
      class="marquee-shell"
      :aria-label="text"
      role="marquee"
    >
      <div class="marquee-track" @mouseenter="paused = true" @mouseleave="paused = false">
        <span class="marquee-segment" :class="{ paused }">{{ text }}</span>
        <span class="marquee-segment" :class="{ paused }" aria-hidden="true">{{ text }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useAppStore } from '@/stores'

const appStore = useAppStore()
const paused = ref(false)

const text = computed(() => {
  const settings = appStore.cachedPublicSettings
  if (!settings?.marquee_enabled) return ''
  const messages = (settings.marquee_messages ?? [])
    .filter((message) => message.enabled && message.text?.trim())
    .map((message) => message.text.trim())
  return messages.join('  ·  ')
})
</script>

<style scoped>
.marquee-shell {
  @apply relative h-full overflow-hidden flex items-center;
  mask-image: linear-gradient(90deg, transparent 0, black 24px, black calc(100% - 24px), transparent 100%);
}

.marquee-track {
  @apply flex w-max whitespace-nowrap;
}

.marquee-segment {
  @apply pr-12 text-sm text-gray-600 dark:text-dark-300;
  animation: marquee 30s linear infinite;
}

.marquee-segment.paused {
  animation-play-state: paused;
}

@keyframes marquee {
  from { transform: translateX(0); }
  to { transform: translateX(-100%); }
}

@media (prefers-reduced-motion: reduce) {
  .marquee-segment {
    animation: none;
  }
}
</style>
```

- [ ] **Step 4: Insert component into AppHeader**

In `frontend/src/components/layout/AppHeader.vue`, add import:

```ts
import AppHeaderMarquee from '@/components/layout/AppHeaderMarquee.vue'
```

In the template, insert between the left group and the right toolbar group:

```vue
      <AppHeaderMarquee />
```

Keep the wrapper class unchanged:

```vue
    <div class="flex h-16 items-center justify-between px-4 md:px-6">
```

- [ ] **Step 5: Run component tests and typecheck**

Run:

```bash
cd frontend && pnpm test:run src/components/layout/__tests__/AppHeaderMarquee.spec.ts
cd frontend && pnpm typecheck
```

Expected: PASS.

---

## Task 7: Admin Settings UI And Local Form

**Files:**
- Modify: `frontend/src/views/admin/SettingsView.vue`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`
- Test: `frontend/src/views/admin/__tests__/SettingsView.spec.ts`

- [ ] **Step 1: Add failing SettingsView tests**

In `frontend/src/views/admin/__tests__/SettingsView.spec.ts`, extend the base mocked settings object with:

```ts
marquee_enabled: false,
marquee_messages: [],
```

Add tests:

```ts
it('renders marquee settings and saves local form payload', async () => {
  getSettings.mockResolvedValueOnce({
    ...baseSettings,
    marquee_enabled: true,
    marquee_messages: [{ id: 'm1', text: 'Old message', enabled: true, sort_order: 0 }],
  })
  updateSettings.mockResolvedValueOnce({
    ...baseSettings,
    marquee_enabled: true,
    marquee_messages: [{ id: 'm1', text: 'New message', enabled: true, sort_order: 0 }],
  })

  const wrapper = mount(SettingsView, {
    global: {
      stubs: {
        AppLayout: AppLayoutStub,
        Toggle: ToggleStub,
        Select: SelectStub,
      },
    },
  })
  await flushPromises()

  expect(wrapper.text()).toContain('admin.settings.marquee.title')
  const textarea = wrapper.find('[data-testid="marquee-message-text-0"]')
  expect((textarea.element as HTMLTextAreaElement).value).toBe('Old message')
  await textarea.setValue('New message')
  await wrapper.find('form').trigger('submit.prevent')
  await flushPromises()

  expect(updateSettings).toHaveBeenCalledWith(expect.objectContaining({
    marquee_enabled: true,
    marquee_messages: [{ id: 'm1', text: 'New message', enabled: true, sort_order: 0 }],
  }))
})

it('adds removes and reorders marquee messages locally', async () => {
  getSettings.mockResolvedValueOnce({
    ...baseSettings,
    marquee_enabled: true,
    marquee_messages: [
      { id: 'a', text: 'A', enabled: true, sort_order: 0 },
      { id: 'b', text: 'B', enabled: true, sort_order: 1 },
    ],
  })

  const wrapper = mount(SettingsView, {
    global: {
      stubs: {
        AppLayout: AppLayoutStub,
        Toggle: ToggleStub,
        Select: SelectStub,
      },
    },
  })
  await flushPromises()

  await wrapper.find('[data-testid="move-marquee-down-0"]').trigger('click')
  expect((wrapper.find('[data-testid="marquee-message-text-0"]').element as HTMLTextAreaElement).value).toBe('B')

  await wrapper.find('[data-testid="add-marquee-message"]').trigger('click')
  expect(wrapper.findAll('[data-testid^="marquee-message-text-"]')).toHaveLength(3)

  await wrapper.find('[data-testid="remove-marquee-message-1"]').trigger('click')
  expect(wrapper.findAll('[data-testid^="marquee-message-text-"]')).toHaveLength(2)
})
```

If the file does not already define `baseSettings`, introduce a local helper:

```ts
const baseSettings = {
  registration_enabled: true,
  email_verify_enabled: false,
  registration_email_suffix_whitelist: [],
  promo_code_enabled: true,
  password_reset_enabled: false,
  invitation_code_enabled: false,
  totp_enabled: false,
  force_email_on_third_party_signup: false,
  site_name: 'APIPool',
  site_logo: '',
  site_subtitle: '',
  api_base_url: '',
  contact_info: '',
  doc_url: '',
  home_content: '',
  backend_mode_enabled: false,
  hide_ccs_import_button: false,
  purchase_subscription_enabled: false,
  purchase_subscription_url: '',
  table_default_page_size: 20,
  table_page_size_options: [10, 20, 50, 100],
  custom_menu_items: [],
  custom_endpoints: [],
  marquee_enabled: false,
  marquee_messages: [],
}
```

- [ ] **Step 2: Run SettingsView test and verify it fails**

Run:

```bash
cd frontend && pnpm test:run src/views/admin/__tests__/SettingsView.spec.ts -t 'marquee'
```

Expected: FAIL because UI and form fields are missing.

- [ ] **Step 3: Add i18n keys**

In `frontend/src/i18n/locales/zh.ts`, under `admin.settings`, add:

```ts
marquee: {
  title: '顶部跑马灯字幕',
  description: '在桌面端 Header 中间展示循环滚动字幕，用于轻量推广或提示。',
  enabled: '启用字幕',
  add: '新增字幕',
  textPlaceholder: '例如：AI 会员店上新，点击左侧 AI 会员店菜单查看',
  draftBadge: '未启用',
  tipMultipleSeparator: '字幕文本会在 Header 中间循环滚动；多条之间用“·”自动分隔。',
  moveUp: '上移',
  moveDown: '下移',
  remove: '删除',
}
```

In `frontend/src/i18n/locales/en.ts`, under `admin.settings`, add:

```ts
marquee: {
  title: 'Header Marquee',
  description: 'Show rotating text in the desktop header for lightweight promotion or notices.',
  enabled: 'Enable marquee',
  add: 'Add message',
  textPlaceholder: 'Example: AI membership store is live. Open the AI Store menu on the left.',
  draftBadge: 'Disabled',
  tipMultipleSeparator: 'Messages scroll in the header; multiple messages are separated with “·”.',
  moveUp: 'Move up',
  moveDown: 'Move down',
  remove: 'Remove',
}
```

- [ ] **Step 4: Add form fields and local functions**

In `frontend/src/views/admin/SettingsView.vue`, add to the reactive `form` near `custom_menu_items`:

```ts
marquee_enabled: false,
marquee_messages: [] as Array<{
  id: string;
  text: string;
  enabled: boolean;
  sort_order: number;
}>,
```

Add local functions near custom menu item management:

```ts
function addMarqueeMessage() {
  form.marquee_messages.push({
    id: "",
    text: "",
    enabled: true,
    sort_order: form.marquee_messages.length,
  });
}

function removeMarqueeMessage(index: number) {
  form.marquee_messages.splice(index, 1);
  form.marquee_messages.forEach((item, i) => {
    item.sort_order = i;
  });
}

function moveMarqueeMessage(index: number, direction: -1 | 1) {
  const targetIndex = index + direction;
  if (targetIndex < 0 || targetIndex >= form.marquee_messages.length) return;
  const items = form.marquee_messages;
  const current = items[index];
  items[index] = items[targetIndex];
  items[targetIndex] = current;
  items.forEach((item, i) => {
    item.sort_order = i;
  });
}
```

In `submitSettings` payload near `custom_menu_items`:

```ts
      marquee_enabled: form.marquee_enabled,
      marquee_messages: form.marquee_messages,
```

- [ ] **Step 5: Add UI section after custom menu**

In `frontend/src/views/admin/SettingsView.vue`, after the custom menu card, add:

```vue
          <div class="card">
            <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t("admin.settings.marquee.title") }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t("admin.settings.marquee.description") }}
              </p>
            </div>
            <div class="space-y-4 p-6">
              <label class="flex items-center justify-between gap-4">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t("admin.settings.marquee.enabled") }}
                </span>
                <Toggle v-model="form.marquee_enabled" />
              </label>

              <div
                v-for="(message, index) in form.marquee_messages"
                :key="message.id || index"
                class="rounded-lg border border-gray-200 p-4 dark:border-dark-600"
              >
                <div class="mb-3 flex items-center justify-between gap-3">
                  <label class="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-400">
                    <input v-model="message.enabled" type="checkbox" class="rounded border-gray-300">
                    {{ t("admin.settings.marquee.enabled") }}
                    <span
                      v-if="!message.enabled"
                      class="rounded bg-gray-100 px-2 py-0.5 text-xs text-gray-500 dark:bg-dark-700 dark:text-dark-300"
                    >
                      {{ t("admin.settings.marquee.draftBadge") }}
                    </span>
                  </label>
                  <div class="flex items-center gap-2">
                    <button
                      v-if="index > 0"
                      type="button"
                      class="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-700"
                      :title="t('admin.settings.marquee.moveUp')"
                      :data-testid="`move-marquee-up-${index}`"
                      @click="moveMarqueeMessage(index, -1)"
                    >
                      ↑
                    </button>
                    <button
                      v-if="index < form.marquee_messages.length - 1"
                      type="button"
                      class="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-700"
                      :title="t('admin.settings.marquee.moveDown')"
                      :data-testid="`move-marquee-down-${index}`"
                      @click="moveMarqueeMessage(index, 1)"
                    >
                      ↓
                    </button>
                    <button
                      type="button"
                      class="rounded p-1 text-red-400 hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20"
                      :title="t('admin.settings.marquee.remove')"
                      :data-testid="`remove-marquee-message-${index}`"
                      @click="removeMarqueeMessage(index)"
                    >
                      ×
                    </button>
                  </div>
                </div>
                <textarea
                  v-model="message.text"
                  rows="2"
                  maxlength="500"
                  class="input text-sm"
                  :placeholder="t('admin.settings.marquee.textPlaceholder')"
                  :data-testid="`marquee-message-text-${index}`"
                />
              </div>

              <button
                type="button"
                class="flex w-full items-center justify-center gap-2 rounded-lg border-2 border-dashed border-gray-300 py-3 text-sm text-gray-500 transition-colors hover:border-primary-400 hover:text-primary-600 dark:border-dark-600 dark:text-gray-400 dark:hover:border-primary-500 dark:hover:text-primary-400"
                data-testid="add-marquee-message"
                @click="addMarqueeMessage"
              >
                + {{ t("admin.settings.marquee.add") }}
              </button>

              <p class="text-sm text-gray-500 dark:text-gray-400">
                {{ t("admin.settings.marquee.tipMultipleSeparator") }}
              </p>
            </div>
          </div>
```

- [ ] **Step 6: Run SettingsView tests and typecheck**

Run:

```bash
cd frontend && pnpm test:run src/views/admin/__tests__/SettingsView.spec.ts -t 'marquee'
cd frontend && pnpm typecheck
```

Expected: PASS.

---

## Task 8: Full Verification And Documentation Cleanup

**Files:**
- Modify if needed: `docs/superpowers/specs/2026-04-26-ai-store-promo-marquee-design.md`
- Verify: backend and frontend test suites

- [ ] **Step 1: Clean spec residual contradictions**

Update the spec so the main sections no longer contain these stale statements:

```text
adminSettingsStore.* (Pinia, admin only)
getDefaultSystemSettings
adminSettings.spec.ts add/update/delete/reorder marquee message
拖拽排序组件是否已有可复用
```

Replace with:

```text
SettingsView 本地 form
InitializeDefaultSettings
SettingsView.spec.ts 覆盖本地 add/remove/move/save payload
本期排序使用上下箭头按钮
```

- [ ] **Step 2: Run focused backend tests**

Run:

```bash
cd backend && go test -tags=unit ./internal/handler/dto ./internal/service ./internal/handler ./internal/handler/admin ./internal/server -run 'Marquee|PublicSettingsInjectionPayload|Contract' -count=1
```

Expected: PASS.

- [ ] **Step 3: Run focused frontend tests**

Run:

```bash
cd frontend && pnpm test:run src/stores/__tests__/app.spec.ts src/components/layout/__tests__/AppHeaderMarquee.spec.ts src/views/admin/__tests__/SettingsView.spec.ts
```

Expected: PASS.

- [ ] **Step 4: Run project-level checks required by AGENTS.md**

Run:

```bash
cd backend && go test -tags=unit ./...
cd backend && go test -tags=integration ./...
cd backend && golangci-lint run ./...
cd frontend && pnpm test:run
cd frontend && pnpm typecheck
```

Expected: all commands PASS. If integration tests require local services and fail due missing environment, capture the exact error and run the narrow unit/type checks above as the verified fallback.

- [ ] **Step 5: Browser layout verification**

Start frontend/backend the same way this repo README documents for local development, then verify:

```text
Desktop >= 1024px:
- Header left title remains left.
- Header right toolbar remains right.
- Marquee text appears in the center when enabled.
- Hover pauses scrolling.

Mobile < 1024px:
- Marquee is hidden by CSS.
- Left menu button and right toolbar match pre-change layout.
- No overlap or horizontal scroll.

Dark mode:
- Marquee text is readable.
- Mask fade does not obscure the whole message.
```

- [ ] **Step 6: Final implementation commit if requested**

If the user asks for a commit, run:

```bash
git status --short
git add backend/internal/service/domain_constants.go \
  backend/internal/handler/dto/settings.go \
  backend/internal/service/settings_view.go \
  backend/internal/service/setting_service.go \
  backend/internal/handler/admin/setting_handler.go \
  backend/internal/handler/setting_handler.go \
  backend/internal/server/api_contract_test.go \
  backend/internal/handler/dto/settings_marquee_test.go \
  backend/internal/service/setting_service_public_test.go \
  backend/internal/service/setting_service_update_test.go \
  backend/internal/handler/admin/setting_handler_marquee_test.go \
  backend/internal/handler/setting_handler_public_test.go \
  frontend/src/types/index.ts \
  frontend/src/api/admin/settings.ts \
  frontend/src/stores/app.ts \
  frontend/src/components/layout/AppHeaderMarquee.vue \
  frontend/src/components/layout/AppHeader.vue \
  frontend/src/views/admin/SettingsView.vue \
  frontend/src/i18n/locales/zh.ts \
  frontend/src/i18n/locales/en.ts \
  frontend/src/stores/__tests__/app.spec.ts \
  frontend/src/components/layout/__tests__/AppHeaderMarquee.spec.ts \
  frontend/src/views/admin/__tests__/SettingsView.spec.ts \
  docs/superpowers/specs/2026-04-26-ai-store-promo-marquee-design.md
git commit -m "feat: add configurable header marquee"
```

Expected: commit succeeds and `git status --short` shows only unrelated pre-existing files.

---

## Self-Review

- Spec coverage: backend storage, admin configuration, public filtering, SSR injection, AppHeader rendering, desktop-only behavior, SettingsView UI, i18n, and verification are covered by Tasks 1-8.
- Residual design-review corrections: SSR injection filtering, `SettingsView` local form, `InitializeDefaultSettings`, `GetAllSettings`/`parseSettings`, and no `adminSettingsStore` editing are explicitly included.
- Placeholder scan: no implementation step depends on unspecified functions; helpers introduced in earlier tasks are named before later use.
- Type consistency: `MarqueeMessage`, `marquee_enabled`, and `marquee_messages` are used consistently across Go DTOs, frontend types, public settings, admin API, and component code.
