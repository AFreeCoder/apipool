//go:build unit

package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type openAIPlanSyncAdminService struct {
	*stubAdminService
	getAccount  *service.Account
	syncCalls   int
	lastSynced  *service.Account
	lastCreated *service.Account
}

func (s *openAIPlanSyncAdminService) GetAccount(ctx context.Context, id int64) (*service.Account, error) {
	if s.getAccount != nil {
		account := *s.getAccount
		if s.getAccount.Credentials != nil {
			account.Credentials = cloneMap(s.getAccount.Credentials)
		}
		if s.getAccount.Extra != nil {
			account.Extra = cloneMap(s.getAccount.Extra)
		}
		account.ID = id
		return &account, nil
	}
	return s.stubAdminService.GetAccount(ctx, id)
}

func (s *openAIPlanSyncAdminService) CreateAccount(ctx context.Context, input *service.CreateAccountInput) (*service.Account, error) {
	account, err := s.stubAdminService.CreateAccount(ctx, input)
	if err != nil {
		return nil, err
	}
	account.Platform = input.Platform
	account.Type = input.Type
	account.Credentials = cloneMap(input.Credentials)
	s.lastCreated = account
	return account, nil
}

func (s *openAIPlanSyncAdminService) UpdateAccount(ctx context.Context, id int64, input *service.UpdateAccountInput) (*service.Account, error) {
	account, err := s.stubAdminService.UpdateAccount(ctx, id, input)
	if err != nil {
		return nil, err
	}
	account.Platform = service.PlatformOpenAI
	account.Type = service.AccountTypeOAuth
	account.Credentials = cloneMap(input.Credentials)
	account.Schedulable = true
	return account, nil
}

func (s *openAIPlanSyncAdminService) SyncOpenAIPlanType(ctx context.Context, account *service.Account) string {
	s.syncCalls++
	s.lastSynced = account
	if account.Credentials == nil {
		account.Credentials = make(map[string]any)
	}
	account.Credentials["plan_type"] = "plus"
	return "plus"
}

type openAITestOAuthClient struct {
	tokenResponse *openai.TokenResponse
}

func (c *openAITestOAuthClient) ExchangeCode(ctx context.Context, code, codeVerifier, redirectURI, proxyURL, clientID string) (*openai.TokenResponse, error) {
	return c.tokenResponse, nil
}

func (c *openAITestOAuthClient) RefreshToken(ctx context.Context, refreshToken, proxyURL string) (*openai.TokenResponse, error) {
	return c.tokenResponse, nil
}

func (c *openAITestOAuthClient) RefreshTokenWithClientID(ctx context.Context, refreshToken, proxyURL string, clientID string) (*openai.TokenResponse, error) {
	return c.tokenResponse, nil
}

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func TestAccountHandler_Refresh_OpenAISyncsPlanTypeIntoResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adminSvc := &openAIPlanSyncAdminService{
		stubAdminService: newStubAdminService(),
		getAccount: &service.Account{
			ID:          26,
			Name:        "openai-oauth",
			Platform:    service.PlatformOpenAI,
			Type:        service.AccountTypeOAuth,
			Status:      service.StatusActive,
			Schedulable: true,
			Credentials: map[string]any{
				"refresh_token": "refresh-token",
				"keep":          "existing",
			},
		},
	}
	openaiSvc := service.NewOpenAIOAuthService(nil, &openAITestOAuthClient{
		tokenResponse: &openai.TokenResponse{
			AccessToken: "new-access-token",
			ExpiresIn:   int64(time.Hour.Seconds()),
		},
	})
	handler := NewAccountHandler(adminSvc, nil, openaiSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	router := gin.New()
	router.POST("/api/v1/admin/accounts/:id/refresh", handler.Refresh)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/26/refresh", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 1, adminSvc.syncCalls)
	require.NotNil(t, adminSvc.lastSynced)
	require.Equal(t, "plus", adminSvc.lastSynced.Credentials["plan_type"])

	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	data := payload["data"].(map[string]any)
	credentials := data["credentials"].(map[string]any)
	require.Equal(t, "new-access-token", credentials["access_token"])
	require.Equal(t, "existing", credentials["keep"])
	require.Equal(t, "plus", credentials["plan_type"])
}

func TestOpenAIOAuthHandler_CreateAccountFromOAuth_SyncsPlanType(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adminSvc := &openAIPlanSyncAdminService{stubAdminService: newStubAdminService()}
	openaiSvc := service.NewOpenAIOAuthService(nil, &openAITestOAuthClient{
		tokenResponse: &openai.TokenResponse{
			AccessToken: "created-access-token",
			ExpiresIn:   int64(time.Hour.Seconds()),
		},
	})

	authURLResult, err := openaiSvc.GenerateAuthURL(context.Background(), nil, "", service.PlatformOpenAI)
	require.NoError(t, err)
	parsedAuthURL, err := url.Parse(authURLResult.AuthURL)
	require.NoError(t, err)
	state := parsedAuthURL.Query().Get("state")
	require.NotEmpty(t, state)

	handler := NewOpenAIOAuthHandler(openaiSvc, adminSvc)
	router := gin.New()
	router.POST("/api/v1/admin/openai/create-from-oauth", handler.CreateAccountFromOAuth)

	body, err := json.Marshal(map[string]any{
		"session_id": authURLResult.SessionID,
		"code":       "test-code",
		"state":      state,
		"name":       "created-openai-oauth",
	})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/openai/create-from-oauth", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 1, adminSvc.syncCalls)
	require.NotNil(t, adminSvc.lastCreated)
	require.NotNil(t, adminSvc.lastSynced)
	require.Equal(t, adminSvc.lastCreated.ID, adminSvc.lastSynced.ID)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	data := payload["data"].(map[string]any)
	credentials := data["credentials"].(map[string]any)
	require.Equal(t, "plus", credentials["plan_type"])
}
