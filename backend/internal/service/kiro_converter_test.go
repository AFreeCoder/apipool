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

func TestBuildKiroGeneratePayload_AppliesKiroCompatibilityRules(t *testing.T) {
	t.Parallel()

	longToolName := "very_long_tool_name_that_should_be_shortened_for_kiro_because_it_exceeds_sixty_three_characters"
	body := []byte(`{
	  "model":"claude-sonnet-4-5",
	  "system":[{"type":"text","text":"system rule"}],
	  "thinking":{"type":"enabled","budget_tokens":2048},
	  "messages":[
	    {"role":"assistant","content":[{"type":"tool_use","id":"tool_1","name":"` + longToolName + `","input":{"query":"a"}},{"type":"tool_use","id":"tool_orphan","name":"orphan_tool","input":{"query":"b"}}]},
	    {"role":"user","content":[{"type":"tool_result","tool_use_id":"tool_1","content":[{"text":"tool ok"}]},{"type":"text","text":"continue"}]}
	  ],
	  "stream":false
	}`)
	account := &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeKiro,
	}

	payload, err := buildKiroGeneratePayload(body, account)
	require.NoError(t, err)
	require.Equal(t, "claude-sonnet-4-5", payload.MappedModel)
	require.Len(t, payload.ToolNameMap, 1)

	var shortened string
	for short, original := range payload.ToolNameMap {
		shortened = short
		require.Equal(t, longToolName, original)
	}
	require.NotEmpty(t, shortened)

	history := payload.Request.ConversationState.History
	require.Len(t, history, 3)
	require.Contains(t, history[0].UserInputMessage.Content, "<thinking_mode>enabled</thinking_mode>")
	require.Contains(t, history[0].UserInputMessage.Content, "system rule")
	require.Len(t, history[2].AssistantResponseMessage.ToolUses, 1)
	require.Equal(t, "tool_1", history[2].AssistantResponseMessage.ToolUses[0].ToolUseID)
	require.Equal(t, shortened, history[2].AssistantResponseMessage.ToolUses[0].Name)

	current := payload.Request.ConversationState.CurrentMessage.UserInputMessage
	require.Equal(t, "continue", current.Content)
	require.Len(t, current.UserInputMessageContext.ToolResults, 1)
	require.Equal(t, "tool_1", current.UserInputMessageContext.ToolResults[0].ToolUseID)
	require.Len(t, current.UserInputMessageContext.Tools, 1)
	require.Equal(t, shortened, current.UserInputMessageContext.Tools[0].ToolSpecification.Name)
}
