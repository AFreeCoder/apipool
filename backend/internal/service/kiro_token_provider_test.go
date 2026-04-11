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
	accountRepo := &refreshAPIAccountRepo{
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

	refresher := &fakeKiroOAuthRefreshExecutor{
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

type fakeKiroOAuthRefreshExecutor struct {
	cacheKey       string
	newCredentials map[string]any
}

func (f *fakeKiroOAuthRefreshExecutor) CacheKey(account *Account) string {
	return f.cacheKey
}

func (f *fakeKiroOAuthRefreshExecutor) CanRefresh(account *Account) bool {
	return true
}

func (f *fakeKiroOAuthRefreshExecutor) NeedsRefresh(account *Account, refreshWindow time.Duration) bool {
	return true
}

func (f *fakeKiroOAuthRefreshExecutor) Refresh(ctx context.Context, account *Account) (map[string]any, error) {
	return f.newCredentials, nil
}
