//go:build unit

package admin

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/stretchr/testify/require"
)

func TestNormalizeMarqueeMessages_FillsIDAndSortOrder(t *testing.T) {
	raw, err := normalizeMarqueeMessages([]dto.MarqueeMessage{
		{Text: " First ", Enabled: true, SortOrder: 99},
		{ID: "existing-id", Text: "Second", Enabled: false, SortOrder: 42},
	})

	require.NoError(t, err)
	var got []dto.MarqueeMessage
	require.NoError(t, json.Unmarshal([]byte(raw), &got))
	require.Len(t, got, 2)
	require.NotEmpty(t, got[0].ID)
	require.Equal(t, " First ", got[0].Text)
	require.True(t, got[0].Enabled)
	require.Equal(t, 0, got[0].SortOrder)
	require.Equal(t, "existing-id", got[1].ID)
	require.Equal(t, 1, got[1].SortOrder)
}

func TestNormalizeMarqueeMessages_RejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name     string
		messages []dto.MarqueeMessage
	}{
		{name: "empty text", messages: []dto.MarqueeMessage{{Text: "   ", Enabled: true}}},
		{name: "too long text", messages: []dto.MarqueeMessage{{Text: strings.Repeat("x", 501), Enabled: true}}},
		{name: "invalid id", messages: []dto.MarqueeMessage{{ID: "bad id", Text: "ok", Enabled: true}}},
		{name: "id too long", messages: []dto.MarqueeMessage{{ID: strings.Repeat("a", 33), Text: "ok", Enabled: true}}},
		{name: "duplicate id", messages: []dto.MarqueeMessage{
			{ID: "same", Text: "one", Enabled: true},
			{ID: "same", Text: "two", Enabled: true},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := normalizeMarqueeMessages(tt.messages)
			require.Error(t, err)
		})
	}
}

func TestNormalizeMarqueeMessages_RejectsMoreThanTwentyMessages(t *testing.T) {
	messages := make([]dto.MarqueeMessage, 21)
	for i := range messages {
		messages[i] = dto.MarqueeMessage{Text: "ok", Enabled: true}
	}

	_, err := normalizeMarqueeMessages(messages)

	require.Error(t, err)
}
