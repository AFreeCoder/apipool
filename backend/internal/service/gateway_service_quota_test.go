//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGatewayService_isAccountSchedulableForQuota_Kiro(t *testing.T) {
	t.Parallel()

	service := &GatewayService{}

	account := &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeKiro,
		Extra: map[string]any{
			"quota_limit": 10.0,
			"quota_used":  20.0,
		},
	}

	require.False(t, service.isAccountSchedulableForQuota(account))
}

func TestPostUsageBillingParams_shouldUpdateAccountQuota_Kiro(t *testing.T) {
	t.Parallel()

	params := &postUsageBillingParams{
		Account: &Account{
			Platform: PlatformAnthropic,
			Type:     AccountTypeKiro,
			Extra: map[string]any{
				"quota_limit": 100.0,
			},
		},
		Cost: &CostBreakdown{
			TotalCost: 0.3,
		},
	}

	require.True(t, params.shouldUpdateAccountQuota())
}
