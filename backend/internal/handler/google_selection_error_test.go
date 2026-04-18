//go:build unit

package handler

import (
	"errors"
	"fmt"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestPublicGoogleAccountSelectionMessage(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		requestedModel string
		want           string
	}{
		{
			name:           "wrapped no available accounts with model unsupported details",
			err:            fmt.Errorf("%w supporting model: gemini-2.5-pro (total=2 eligible=0 model_unsupported=2)", service.ErrNoAvailableAccounts),
			requestedModel: "gemini-2.5-pro",
			want:           "Model gemini-2.5-pro is not supported in this group",
		},
		{
			name:           "gemini scheduler no available accounts",
			err:            errors.New("no available Gemini accounts"),
			requestedModel: "gemini-2.5-pro",
			want:           "No available Gemini accounts",
		},
		{
			name:           "generic infrastructure error keeps generic message",
			err:            errors.New("query accounts failed: redis unavailable"),
			requestedModel: "gemini-2.5-pro",
			want:           "Service temporarily unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, publicGoogleAccountSelectionMessage(tt.err, tt.requestedModel))
		})
	}
}
