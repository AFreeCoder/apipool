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

func TestBuildCodexAgentIdentityKeysUseChatGPTAccountOnly(t *testing.T) {
	keys := buildCodexAgentIdentityKeys("team-a")
	require.Equal(t, []string{"account:team-a"}, keys)
}

func TestCodexAgentIdentityIndexSeparatesTeamsForSameUser(t *testing.T) {
	existing := service.Account{
		ID: 1,
		Credentials: map[string]any{
			"auth_mode":          service.OpenAIAuthModeAgentIdentity,
			"chatgpt_account_id": "team-a",
			"chatgpt_user_id":    "same-user",
			"agent_runtime_id":   "runtime-a",
		},
	}
	index := buildCodexAccountIndex([]service.Account{existing})

	teamBKeys := buildCodexAgentIdentityKeys("team-b")
	matched, _ := index.Find(teamBKeys, "same-user")
	require.Nil(t, matched)

	teamAKeys := buildCodexAgentIdentityKeys("team-a")
	matched, matchedKey := index.Find(teamAKeys, "same-user")
	require.NotNil(t, matched)
	require.Equal(t, int64(1), matched.ID)
	require.Equal(t, "account:team-a", matchedKey)
}

func TestImportCodexSessionsKeepsAgentIdentityTeamsSeparate(t *testing.T) {
	first := buildAgentIdentityImportValue(t, "runtime-a", "team-a", "same-user", "task-a")
	second := buildAgentIdentityImportValue(t, "runtime-b", "team-b", "same-user", "task-b")
	svc := newCodexImportMemoryAdminService(nil)
	handler := NewAccountHandler(svc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	result, err := handler.importCodexSessions(context.Background(), CodexSessionImportRequest{
		SkipDefaultGroupBind: boolPtr(true),
	}, []codexImportEntry{{Index: 1, Value: first}, {Index: 2, Value: second}})
	require.NoError(t, err)
	require.Equal(t, 2, result.Created)
	require.Zero(t, result.Updated)
	require.Zero(t, result.Skipped)
	require.Len(t, svc.createdAccounts, 2)
}

func TestImportCodexSessionsMergesAgentIdentityRuntimesForSameTeam(t *testing.T) {
	first := buildAgentIdentityImportValue(t, "runtime-a", "team-a", "same-user", "task-a")
	second := buildAgentIdentityImportValue(t, "runtime-b", "team-a", "same-user", "task-b")
	firstIdentity, ok := first["agent_identity"].(map[string]any)
	require.True(t, ok)
	existing := service.Account{
		ID:       41,
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeOAuth,
		Credentials: map[string]any{
			"auth_mode":          service.OpenAIAuthModeAgentIdentity,
			"agent_runtime_id":   firstIdentity["agent_runtime_id"],
			"agent_private_key":  firstIdentity["agent_private_key"],
			"task_id":            firstIdentity["task_id"],
			"chatgpt_account_id": firstIdentity["account_id"],
			"chatgpt_user_id":    firstIdentity["chatgpt_user_id"],
		},
	}
	svc := newCodexImportMemoryAdminService([]service.Account{existing})
	handler := NewAccountHandler(svc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	result, err := handler.importCodexSessions(context.Background(), CodexSessionImportRequest{
		SkipDefaultGroupBind: boolPtr(true),
	}, []codexImportEntry{{Index: 1, Value: second}})
	require.NoError(t, err)
	require.Zero(t, result.Created)
	require.Equal(t, 1, result.Updated)
	require.Len(t, svc.updatedAccounts, 1)
	require.Equal(t, "runtime-b", svc.updatedAccounts[0].input.Credentials["agent_runtime_id"])
	require.Equal(t, "task-b", svc.updatedAccounts[0].input.Credentials["task_id"])
}

func buildAgentIdentityImportValue(t *testing.T, runtimeID, accountID, userID, taskID string) map[string]any {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	return map[string]any{
		"auth_mode": "agentIdentity",
		"agent_identity": map[string]any{
			"agent_runtime_id":  runtimeID,
			"agent_private_key": base64.StdEncoding.EncodeToString(der),
			"task_id":           taskID,
			"account_id":        accountID,
			"chatgpt_user_id":   userID,
		},
	}
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
