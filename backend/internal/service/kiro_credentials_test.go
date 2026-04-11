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
			"client_id":     "client-id",
			"client_secret": "client-secret",
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

func TestParseKiroCredentials_RejectsNonKiroAccount(t *testing.T) {
	t.Parallel()

	account := &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"auth_method":   "social",
			"refresh_token": "rt-non-kiro",
		},
	}

	_, err := ParseKiroCredentials(account)
	require.ErrorContains(t, err, "not a kiro account")
}

func TestParseKiroCredentials_RequiresAuthMethod(t *testing.T) {
	t.Parallel()

	account := &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeKiro,
		Credentials: map[string]any{
			"refresh_token": "rt-no-auth",
		},
	}

	_, err := ParseKiroCredentials(account)
	require.ErrorContains(t, err, "auth_method is required")
}

func TestParseKiroCredentials_RequiresRefreshToken(t *testing.T) {
	t.Parallel()

	account := &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeKiro,
		Credentials: map[string]any{
			"auth_method": "social",
		},
	}

	_, err := ParseKiroCredentials(account)
	require.ErrorContains(t, err, "refresh_token is required")
}

func TestParseKiroCredentials_IDCRequiresClientCredentials(t *testing.T) {
	t.Parallel()

	account := &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeKiro,
		Credentials: map[string]any{
			"auth_method":   "builder-id",
			"refresh_token": "rt-idc",
		},
	}

	_, err := ParseKiroCredentials(account)
	require.ErrorContains(t, err, "client_id and client_secret")
}

func TestAccount_KiroCapabilities(t *testing.T) {
	t.Parallel()

	account := &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeKiro,
		Credentials: map[string]any{
			"pool_mode":                  true,
			"custom_error_codes":         []int{429},
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
