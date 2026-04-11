//go:build unit

package service

import (
	"context"
	"strings"
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

func TestKiroTokenProvider_GetAccessToken_RateLimitedRefreshMarksTempUnschedAndUsesExistingToken(t *testing.T) {
	t.Parallel()

	cache := newClaudeTokenCacheStub()
	account := &Account{
		ID:       10,
		Platform: PlatformAnthropic,
		Type:     AccountTypeKiro,
		Credentials: map[string]any{
			"access_token":  "at-1",
			"refresh_token": "rt-1",
			"expires_at":    time.Now().Add(30 * time.Second).Unix(),
			"auth_method":   "social",
		},
	}
	accountRepo := &kiroProviderAccountRepo{
		refreshAPIAccountRepo: refreshAPIAccountRepo{account: account},
	}
	tempCache := &kiroProviderTempUnschedCache{}
	provider := NewKiroTokenProvider(accountRepo, cache, &KiroAuthService{})
	provider.SetTempUnschedCache(tempCache)
	provider.SetRefreshAPI(NewOAuthRefreshAPI(accountRepo, cache), &fakeKiroOAuthRefreshExecutor{
		cacheKey: "kiro:account:10",
		err: &KiroRefreshError{
			StatusCode: 429,
			Kind:       kiroRefreshErrorRateLimited,
			Body:       `{"message":"slow down"}`,
		},
	})

	token, err := provider.GetAccessToken(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, "at-1", token)
	require.Equal(t, 1, accountRepo.setTempUnschedCalls)
	require.Equal(t, int64(10), accountRepo.lastTempUnschedID)
	require.Contains(t, strings.ToLower(accountRepo.lastTempUnschedReason), "429")
	require.Equal(t, 1, tempCache.setCalls)
}

type fakeKiroOAuthRefreshExecutor struct {
	cacheKey       string
	newCredentials map[string]any
	err            error
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
	if f.err != nil {
		return nil, f.err
	}
	return f.newCredentials, nil
}

type kiroProviderAccountRepo struct {
	refreshAPIAccountRepo
	setTempUnschedCalls   int
	lastTempUnschedID     int64
	lastTempUnschedReason string
}

func (r *kiroProviderAccountRepo) SetTempUnschedulable(_ context.Context, id int64, until time.Time, reason string) error {
	r.setTempUnschedCalls++
	r.lastTempUnschedID = id
	r.lastTempUnschedReason = reason
	return nil
}

type kiroProviderTempUnschedCache struct {
	setCalls int
	lastID   int64
	state    *TempUnschedState
}

func (s *kiroProviderTempUnschedCache) SetTempUnsched(_ context.Context, accountID int64, state *TempUnschedState) error {
	s.setCalls++
	s.lastID = accountID
	s.state = state
	return nil
}

func (s *kiroProviderTempUnschedCache) GetTempUnsched(_ context.Context, accountID int64) (*TempUnschedState, error) {
	return nil, nil
}

func (s *kiroProviderTempUnschedCache) DeleteTempUnsched(_ context.Context, accountID int64) error {
	return nil
}
