package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"
)

const (
	kiroTokenRefreshSkew = 3 * time.Minute
	kiroTokenCacheSkew   = 5 * time.Minute
	kiroLockWaitTime     = 200 * time.Millisecond
)

type KiroTokenCache = GeminiTokenCache

type KiroTokenProvider struct {
	accountRepo   AccountRepository
	tokenCache    KiroTokenCache
	authService   *KiroAuthService
	refreshAPI    *OAuthRefreshAPI
	executor      OAuthRefreshExecutor
	refreshPolicy ProviderRefreshPolicy
}

func NewKiroTokenProvider(accountRepo AccountRepository, tokenCache KiroTokenCache, authService *KiroAuthService) *KiroTokenProvider {
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
	if account == nil {
		return "", errors.New("account is nil")
	}
	if !account.IsKiro() {
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
			slog.Warn("kiro_token_refresh_failed", "account_id", account.ID, "error", err)
			refreshFailed = true
		} else if result.LockHeld {
			if p.refreshPolicy.OnLockHeld == ProviderLockHeldWaitForCache && p.tokenCache != nil {
				time.Sleep(kiroLockWaitTime)
				if token, cacheErr := p.tokenCache.GetAccessToken(ctx, cacheKey); cacheErr == nil && strings.TrimSpace(token) != "" {
					return token, nil
				}
			}
		} else if result.Account != nil {
			account = result.Account
			expiresAt = account.GetCredentialAsTime("expires_at")
		}
	}

	accessToken := strings.TrimSpace(account.GetCredential("access_token"))
	if accessToken == "" {
		return "", errors.New("access_token not found in credentials")
	}

	if p.tokenCache != nil {
		latestAccount, isStale := CheckTokenVersion(ctx, account, p.accountRepo)
		if isStale && latestAccount != nil {
			accessToken = latestAccount.GetCredential("access_token")
			if strings.TrimSpace(accessToken) == "" {
				return "", errors.New("access_token not found after version check")
			}
		} else {
			ttl := 30 * time.Minute
			if refreshFailed {
				if p.refreshPolicy.FailureTTL > 0 {
					ttl = p.refreshPolicy.FailureTTL
				} else {
					ttl = time.Minute
				}
			} else if expiresAt != nil {
				until := time.Until(*expiresAt)
				switch {
				case until > kiroTokenCacheSkew:
					ttl = until - kiroTokenCacheSkew
				case until > 0:
					ttl = until
				default:
					ttl = time.Minute
				}
			}
			_ = p.tokenCache.SetAccessToken(ctx, cacheKey, accessToken, ttl)
		}
	}

	return accessToken, nil
}
