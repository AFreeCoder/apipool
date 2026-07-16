package admin

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestNormalizeCodexImportEntryAcceptsAgentIdentityAuthJSON(t *testing.T) {
	item, err := normalizeCodexImportEntry(codexImportEntry{
		Index: 1,
		Value: buildCodexAgentIdentityImportValue(t, "account-import", "user-import"),
	})
	require.NoError(t, err)
	require.NotNil(t, item)
	require.True(t, item.IsAgentIdentity)
	require.Equal(t, service.OpenAIAuthModeAgentIdentity, item.Credentials["auth_mode"])
	require.Equal(t, "runtime-import", item.Credentials["agent_runtime_id"])
	require.NotEmpty(t, item.Credentials["agent_private_key"])
	require.Equal(t, "account-import", item.Credentials["chatgpt_account_id"])
	require.Equal(t, "user-import", item.Credentials["chatgpt_user_id"])
	require.NotContains(t, item.Credentials, "access_token")
	require.NotContains(t, item.Credentials, "refresh_token")
	require.Empty(t, item.WarningTexts)
}

func TestImportCodexSessionsCreatesAgentIdentityWithoutExpiry(t *testing.T) {
	svc := newCodexImportMemoryAdminService(nil)
	handler := NewAccountHandler(svc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	req := CodexSessionImportRequest{SkipDefaultGroupBind: boolPtr(true)}

	result, err := handler.importCodexSessions(context.Background(), req, []codexImportEntry{{
		Index: 1,
		Value: buildCodexAgentIdentityImportValue(t, "account-import", "user-import"),
	}})
	require.NoError(t, err)
	require.Equal(t, 1, result.Created)
	require.Zero(t, result.Failed)
	require.Len(t, svc.createdAccounts, 1)
	require.Nil(t, svc.createdAccounts[0].ExpiresAt)
	require.Nil(t, svc.createdAccounts[0].AutoPauseOnExpired)
	require.Equal(t, service.OpenAIAuthModeAgentIdentity, svc.createdAccounts[0].Credentials["auth_mode"])
	require.NotContains(t, svc.createdAccounts[0].Credentials, "access_token")
	require.NotContains(t, svc.createdAccounts[0].Credentials, "refresh_token")
}

func TestResolveCodexImportExpiryForAgentIdentityKeepsExplicitAccountExpiry(t *testing.T) {
	item, err := normalizeCodexImportEntry(codexImportEntry{
		Index: 1,
		Value: buildCodexAgentIdentityImportValue(t, "account-import", "user-import"),
	})
	require.NoError(t, err)

	expiresAt := time.Now().Add(time.Hour).UTC().Unix()
	autoPause := true
	accountExpiresAt, credentialExpiresAt, gotAutoPause, warnings, err := resolveCodexImportExpiry(
		CodexSessionImportRequest{ExpiresAt: &expiresAt, AutoPauseOnExpired: &autoPause},
		item,
	)
	require.NoError(t, err)
	require.NotNil(t, accountExpiresAt)
	require.Equal(t, expiresAt, *accountExpiresAt)
	require.Nil(t, credentialExpiresAt)
	require.NotNil(t, gotAutoPause)
	require.True(t, *gotAutoPause)
	require.Empty(t, warnings)
}

func TestImportCodexSessionsReplacesOAuthCredentialsWhenUpgradingToAgentIdentity(t *testing.T) {
	svc := newCodexImportMemoryAdminService([]service.Account{{
		ID:       42,
		Name:     "existing-oauth",
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":       "stale-access",
			"refresh_token":      "stale-refresh",
			"id_token":           "stale-id",
			"client_id":          "stale-client",
			"token_type":         "Bearer",
			"expires_at":         time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			"chatgpt_account_id": "account-import",
			"chatgpt_user_id":    "user-import",
			"model_mapping":      map[string]any{"gpt-old": "gpt-new"},
		},
	}})
	handler := NewAccountHandler(svc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	req := CodexSessionImportRequest{SkipDefaultGroupBind: boolPtr(true)}

	result, err := handler.importCodexSessions(context.Background(), req, []codexImportEntry{{
		Index: 1,
		Value: buildCodexAgentIdentityImportValue(t, "account-import", "user-import"),
	}})
	require.NoError(t, err)
	require.Equal(t, 1, result.Updated)
	require.Zero(t, result.Failed)
	require.Len(t, svc.updatedAccounts, 1)

	credentials := svc.updatedAccounts[0].input.Credentials
	require.Equal(t, service.OpenAIAuthModeAgentIdentity, credentials["auth_mode"])
	for _, key := range []string{"access_token", "refresh_token", "id_token", "client_id", "token_type", "expires_at"} {
		require.NotContains(t, credentials, key)
	}
	require.Contains(t, credentials, "model_mapping")
}

func buildCodexAgentIdentityImportValue(t *testing.T, accountID, userID string) map[string]any {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)

	return map[string]any{
		"auth_mode": "agentIdentity",
		"agent_identity": map[string]any{
			"agent_runtime_id":           "runtime-import",
			"agent_private_key":          base64.StdEncoding.EncodeToString(der),
			"task_id":                    "task-import",
			"account_id":                 accountID,
			"chatgpt_user_id":            userID,
			"email":                      "agent@example.invalid",
			"plan_type":                  "pro",
			"chatgpt_account_is_fedramp": false,
		},
	}
}
