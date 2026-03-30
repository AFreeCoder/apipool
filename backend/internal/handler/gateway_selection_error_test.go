package handler

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestPublicGatewayAccountSelectionError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		requestedModel string
		wantStatus     int
		wantType       string
		wantMessage    string
	}{
		{
			name:           "claude code only restriction",
			err:            fmt.Errorf("select account failed: %w", service.ErrClaudeCodeOnly),
			requestedModel: "claude-sonnet-4-6",
			wantStatus:     http.StatusForbidden,
			wantType:       "permission_error",
			wantMessage:    "This group is restricted to Claude Code clients (/v1/messages only)",
		},
		{
			name:           "group does not support requested model",
			err:            fmt.Errorf("%w supporting model: gpt-5.4-mini (total=2 eligible=0 model_unsupported=2)", service.ErrNoAvailableAccounts),
			requestedModel: "gpt-5.4-mini",
			wantStatus:     http.StatusServiceUnavailable,
			wantType:       "api_error",
			wantMessage:    "Model gpt-5.4-mini is not supported in this group",
		},
		{
			name:           "no available accounts stays generic even with requested model",
			err:            service.ErrNoAvailableAccounts,
			requestedModel: "gpt-5.4-mini",
			wantStatus:     http.StatusServiceUnavailable,
			wantType:       "api_error",
			wantMessage:    "No available accounts",
		},
		{
			name:        "no available accounts without model",
			err:         service.ErrNoAvailableAccounts,
			wantStatus:  http.StatusServiceUnavailable,
			wantType:    "api_error",
			wantMessage: "No available accounts",
		},
		{
			name:           "generic infrastructure error",
			err:            errors.New("query accounts failed: redis unavailable"),
			requestedModel: "gpt-5.4",
			wantStatus:     http.StatusServiceUnavailable,
			wantType:       "api_error",
			wantMessage:    "Service temporarily unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := publicGatewayAccountSelectionError(tt.err, tt.requestedModel)
			require.Equal(t, tt.wantStatus, got.Status)
			require.Equal(t, tt.wantType, got.Type)
			require.Equal(t, tt.wantMessage, got.Message)
		})
	}
}
