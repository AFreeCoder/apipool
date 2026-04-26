//go:build unit

package dto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseMarqueeMessages_ReturnsAllValidMessages(t *testing.T) {
	raw := `[
		{"id":"b","text":" second ","enabled":false,"sort_order":1},
		{"id":"a","text":" first ","enabled":true,"sort_order":0}
	]`

	got := ParseMarqueeMessages(raw)

	require.Equal(t, []MarqueeMessage{
		{ID: "b", Text: " second ", Enabled: false, SortOrder: 1},
		{ID: "a", Text: " first ", Enabled: true, SortOrder: 0},
	}, got)
}

func TestParsePublicMarqueeMessages_FiltersAndSorts(t *testing.T) {
	raw := `[
		{"id":"draft","text":"draft","enabled":false,"sort_order":0},
		{"id":"later","text":"Later","enabled":true,"sort_order":2},
		{"id":"first","text":"First","enabled":true,"sort_order":1},
		{"id":"blank","text":"   ","enabled":true,"sort_order":3}
	]`

	got := ParsePublicMarqueeMessages(raw)

	require.Equal(t, []MarqueeMessage{
		{ID: "first", Text: "First", Enabled: true, SortOrder: 1},
		{ID: "later", Text: "Later", Enabled: true, SortOrder: 2},
	}, got)
}

func TestParseMarqueeMessages_InvalidInputReturnsEmptySlice(t *testing.T) {
	require.Empty(t, ParseMarqueeMessages(""))
	require.Empty(t, ParseMarqueeMessages("not-json"))
	require.Empty(t, ParsePublicMarqueeMessages("not-json"))
}
