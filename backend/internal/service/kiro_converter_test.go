//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildKiroGenerateRequest_UsesMetadataSessionID(t *testing.T) {
	t.Parallel()

	body := []byte(`{
	  "model":"claude-sonnet-4-5",
	  "metadata":{"user_id":"user_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa_account__session_550e8400-e29b-41d4-a716-446655440000"},
	  "messages":[
	    {"role":"user","content":[{"type":"text","text":"hello"}]}
	  ],
	  "stream":false
	}`)
	account := &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeKiro,
		Credentials: map[string]any{
			"profile_arn": "arn:aws:kiro:::profile/default",
			"model_mapping": map[string]any{
				"claude-sonnet-4-5": "kiro-sonnet",
			},
		},
	}

	raw, mappedModel, err := BuildKiroGenerateRequest(body, account)
	require.NoError(t, err)
	require.Equal(t, "kiro-sonnet", mappedModel)
	require.Contains(t, string(raw), `"conversationId":"550e8400-e29b-41d4-a716-446655440000"`)
	require.Contains(t, string(raw), `"profileArn":"arn:aws:kiro:::profile/default"`)
	require.Contains(t, string(raw), `"agentTaskType":"vibe"`)
	require.Contains(t, string(raw), `"origin":"AI_EDITOR"`)
}
