//go:build unit

package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKiroStreamAdapter_AssistantResponseToAnthropicSSE(t *testing.T) {
	t.Parallel()

	adapter := NewKiroStreamAdapter("claude-sonnet-4-5")
	events, usage, err := adapter.ProcessEvent(map[string]any{
		"type":    "assistantResponseEvent",
		"content": "hello",
	})
	require.NoError(t, err)
	require.Nil(t, usage)
	require.Len(t, events, 3)
	require.Contains(t, events[0], "event: message_start")
	require.Contains(t, events[1], "event: content_block_start")
	require.Contains(t, events[2], "hello")
}

func TestKiroStreamAdapter_ToolUseSetsStopReason(t *testing.T) {
	t.Parallel()

	adapter := NewKiroStreamAdapter("claude-sonnet-4-5")
	_, _, _ = adapter.ProcessEvent(map[string]any{
		"type":    "assistantResponseEvent",
		"content": "before tool",
	})

	events, usage, err := adapter.ProcessEvent(map[string]any{
		"type":      "toolUseEvent",
		"name":      "search",
		"toolUseId": "tool_1",
		"input":     map[string]any{"query": "hello"},
	})
	require.NoError(t, err)
	require.Nil(t, usage)
	require.Contains(t, events[len(events)-1], `"tool_use"`)
}

func TestKiroStreamAdapter_ContextUsageAccumulatesUsage(t *testing.T) {
	t.Parallel()

	adapter := NewKiroStreamAdapter("claude-sonnet-4-5")
	_, _, err := adapter.ProcessEvent(map[string]any{
		"type":                   "contextUsageEvent",
		"contextUsagePercentage": 50.0,
	})
	require.NoError(t, err)

	finalEvents, usage, err := adapter.Finalize()
	require.NoError(t, err)
	require.NotNil(t, usage)
	require.NotZero(t, usage.InputTokens)
	require.Contains(t, finalEvents[len(finalEvents)-1], "event: message_stop")
}

func TestKiroStreamAdapter_RestoresOriginalToolName(t *testing.T) {
	t.Parallel()

	longToolName := "very_long_tool_name_that_should_be_restored_when_kiro_returns_the_shortened_alias_name"
	adapter := NewKiroStreamAdapterWithToolNameMap("claude-sonnet-4-5", map[string]string{
		"short_tool_123": longToolName,
	})

	events, _, err := adapter.ProcessEvent(map[string]any{
		"type":      "toolUseEvent",
		"name":      "short_tool_123",
		"toolUseId": "tool_1",
		"input":     map[string]any{"query": "hello"},
	})
	require.NoError(t, err)
	require.Contains(t, strings.Join(events, "\n"), longToolName)
}
