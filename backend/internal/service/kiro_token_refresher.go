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

	creds, err := ParseKiroCredentials(account)
	if err != nil {
		return nil, err
	}

	next := map[string]any{
		"access_token":  info.AccessToken,
		"refresh_token": info.RefreshToken,
		"auth_method":   creds.AuthMethod,
		"machine_id":    creds.MachineID,
	}
	if info.ProfileARN != "" {
		next["profile_arn"] = info.ProfileARN
	}
	if !info.ExpiresAt.IsZero() {
		next["expires_at"] = info.ExpiresAt.Unix()
	}

	return MergeCredentials(account.Credentials, next), nil
}
