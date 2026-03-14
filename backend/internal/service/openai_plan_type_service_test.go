package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/imroc/req/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 脱敏测试 fixture（基于真实响应）
const accountsCheckFixture = `{
  "accounts": {
    "dc284052-xxxx-xxxx-xxxx-9544ffbb46e6": {
      "account": {
        "account_user_role": "account-owner",
        "account_id": "dc284052-xxxx-xxxx-xxxx-9544ffbb46e6",
        "plan_type": "plus",
        "is_deactivated": false
      },
      "features": ["browsing_available", "canvas"]
    }
  }
}`

// planTypeSyncRepo 是 syncOpenAIPlanTypeCore 测试用的 AccountRepository stub
type planTypeSyncRepo struct {
	AccountRepository // 嵌入接口，未实现的方法会 panic
	mergedID          int64
	mergedUpdates     map[string]any
	mergeErr          error
}

func (r *planTypeSyncRepo) MergeCredentials(_ context.Context, id int64, updates map[string]any) error {
	r.mergedID = id
	r.mergedUpdates = updates
	return r.mergeErr
}

func planTypeTestClientFactory() PrivacyClientFactory {
	return func(proxyURL string) (*req.Client, error) {
		return req.C(), nil
	}
}

// withTestEndpoint 临时覆盖 accounts/check 端点 URL 为测试服务器地址
func withTestEndpoint(t *testing.T, serverURL string) {
	t.Helper()
	old := chatgptAccountsCheckEndpoint
	chatgptAccountsCheckEndpoint = serverURL
	t.Cleanup(func() { chatgptAccountsCheckEndpoint = old })
}

func TestSyncOpenAIPlanTypeCore_PreciseMatch(t *testing.T) {
	// 精确匹配命中：chatgpt_account_id 与响应 map key 匹配
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.Header.Get("Authorization"), "Bearer test-token")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, accountsCheckFixture)
	}))
	defer server.Close()
	withTestEndpoint(t, server.URL)

	repo := &planTypeSyncRepo{}
	account := &Account{
		ID:       42,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":       "test-token",
			"chatgpt_account_id": "dc284052-xxxx-xxxx-xxxx-9544ffbb46e6",
		},
	}

	result := syncOpenAIPlanTypeCore(context.Background(), planTypeTestClientFactory(), repo, account, "")

	assert.Equal(t, "plus", result)
	assert.Equal(t, int64(42), repo.mergedID)
	assert.Equal(t, "plus", repo.mergedUpdates["plan_type"])
	// 内存对象也应被更新
	assert.Equal(t, "plus", account.Credentials["plan_type"])
}

func TestSyncOpenAIPlanTypeCore_PreciseMatchMiss(t *testing.T) {
	// 精确匹配未命中：chatgpt_account_id 与响应中的 key 不匹配
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, accountsCheckFixture)
	}))
	defer server.Close()
	withTestEndpoint(t, server.URL)

	repo := &planTypeSyncRepo{}
	account := &Account{
		ID:       42,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":       "test-token",
			"chatgpt_account_id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		},
	}

	result := syncOpenAIPlanTypeCore(context.Background(), planTypeTestClientFactory(), repo, account, "")

	assert.Equal(t, "", result)
	assert.Nil(t, repo.mergedUpdates) // 不应写库
}

func TestSyncOpenAIPlanTypeCore_NoChatGPTAccountID(t *testing.T) {
	// 无 chatgpt_account_id：直接跳过，不发请求
	repo := &planTypeSyncRepo{}
	account := &Account{
		ID:       42,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "test-token",
		},
	}

	result := syncOpenAIPlanTypeCore(context.Background(), func(proxyURL string) (*req.Client, error) {
		t.Fatal("should not create HTTP client when chatgpt_account_id is missing")
		return nil, nil
	}, repo, account, "")

	assert.Equal(t, "", result)
}

func TestSyncOpenAIPlanTypeCore_PlanTypeUnchanged(t *testing.T) {
	// plan_type 未变化：不写库
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, accountsCheckFixture)
	}))
	defer server.Close()
	withTestEndpoint(t, server.URL)

	repo := &planTypeSyncRepo{}
	account := &Account{
		ID:       42,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":       "test-token",
			"chatgpt_account_id": "dc284052-xxxx-xxxx-xxxx-9544ffbb46e6",
			"plan_type":          "plus", // 已经是 plus
		},
	}

	result := syncOpenAIPlanTypeCore(context.Background(), planTypeTestClientFactory(), repo, account, "")

	assert.Equal(t, "plus", result)
	assert.Nil(t, repo.mergedUpdates) // 未变化，不写库
}

func TestSyncOpenAIPlanTypeCore_MergeFailsNoMemoryPollution(t *testing.T) {
	// MergeCredentials 失败时不污染内存对象
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, accountsCheckFixture)
	}))
	defer server.Close()
	withTestEndpoint(t, server.URL)

	repo := &planTypeSyncRepo{mergeErr: fmt.Errorf("db connection lost")}
	account := &Account{
		ID:       42,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":       "test-token",
			"chatgpt_account_id": "dc284052-xxxx-xxxx-xxxx-9544ffbb46e6",
			"plan_type":          "free", // 现有值
		},
	}

	result := syncOpenAIPlanTypeCore(context.Background(), planTypeTestClientFactory(), repo, account, "")

	assert.Equal(t, "", result) // 失败返回空
	// 关键：内存对象不应被修改
	assert.Equal(t, "free", account.Credentials["plan_type"])
}

func TestSyncOpenAIPlanTypeCore_NonOpenAISkipped(t *testing.T) {
	// 非 OpenAI 平台跳过
	repo := &planTypeSyncRepo{}
	account := &Account{
		ID:       42,
		Platform: "claude",
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":       "test-token",
			"chatgpt_account_id": "dc284052-xxxx-xxxx-xxxx-9544ffbb46e6",
		},
	}

	result := syncOpenAIPlanTypeCore(context.Background(), func(proxyURL string) (*req.Client, error) {
		t.Fatal("should not be called for non-openai platform")
		return nil, nil
	}, repo, account, "")

	assert.Equal(t, "", result)
}

func TestSyncOpenAIPlanTypeCore_EmptyPlanTypeInResponse(t *testing.T) {
	// 响应中 plan_type 为空
	emptyPlanResp := `{
		"accounts": {
			"dc284052-xxxx-xxxx-xxxx-9544ffbb46e6": {
				"account": {
					"plan_type": ""
				}
			}
		}
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, emptyPlanResp)
	}))
	defer server.Close()
	withTestEndpoint(t, server.URL)

	repo := &planTypeSyncRepo{}
	account := &Account{
		ID:       42,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":       "test-token",
			"chatgpt_account_id": "dc284052-xxxx-xxxx-xxxx-9544ffbb46e6",
		},
	}

	result := syncOpenAIPlanTypeCore(context.Background(), planTypeTestClientFactory(), repo, account, "")

	assert.Equal(t, "", result)
	assert.Nil(t, repo.mergedUpdates)
}

func TestSyncOpenAIPlanTypeCore_ResponseParseStructure(t *testing.T) {
	// 验证解析结构与真实响应兼容
	var result struct {
		Accounts map[string]struct {
			Account struct {
				PlanType string `json:"plan_type"`
			} `json:"account"`
		} `json:"accounts"`
	}
	err := json.Unmarshal([]byte(accountsCheckFixture), &result)
	require.NoError(t, err)

	entry, ok := result.Accounts["dc284052-xxxx-xxxx-xxxx-9544ffbb46e6"]
	require.True(t, ok)
	assert.Equal(t, "plus", entry.Account.PlanType)
}
