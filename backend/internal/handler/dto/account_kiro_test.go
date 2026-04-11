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
