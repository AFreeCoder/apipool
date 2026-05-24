package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPublicGatewayAccountSelectionError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		requestedModel string
		wantStatus     int
		wantType       string
		wantCode       string
		wantMessage    string
	}{
		{
			name:           "claude code only restriction",
			err:            fmt.Errorf("select account failed: %w", service.ErrClaudeCodeOnly),
			requestedModel: "claude-sonnet-4-6",
			wantStatus:     http.StatusForbidden,
			wantType:       "permission_error",
			wantCode:       "claude_code_client_required",
			wantMessage:    "This group is restricted to Claude Code clients (/v1/messages only)",
		},
		{
			name:           "group does not support requested model",
			err:            fmt.Errorf("%w supporting model: gpt-5.4-mini (total=2 eligible=0 model_unsupported=2)", service.ErrNoAvailableAccounts),
			requestedModel: "gpt-5.4-mini",
			wantStatus:     http.StatusServiceUnavailable,
			wantType:       "api_error",
			wantCode:       "model_not_supported_in_group",
			wantMessage:    "Model gpt-5.4-mini is not supported in this group",
		},
		{
			name:           "no available accounts stays generic even with requested model",
			err:            service.ErrNoAvailableAccounts,
			requestedModel: "gpt-5.4-mini",
			wantStatus:     http.StatusServiceUnavailable,
			wantType:       "api_error",
			wantCode:       "no_available_accounts",
			wantMessage:    "No available accounts",
		},
		{
			name:        "no available accounts without model",
			err:         service.ErrNoAvailableAccounts,
			wantStatus:  http.StatusServiceUnavailable,
			wantType:    "api_error",
			wantCode:    "no_available_accounts",
			wantMessage: "No available accounts",
		},
		{
			name:           "generic infrastructure error",
			err:            errors.New("query accounts failed: redis unavailable"),
			requestedModel: "gpt-5.4",
			wantStatus:     http.StatusServiceUnavailable,
			wantType:       "api_error",
			wantCode:       "service_unavailable",
			wantMessage:    "Service temporarily unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := publicGatewayAccountSelectionError(tt.err, tt.requestedModel)
			require.Equal(t, tt.wantStatus, got.Status)
			require.Equal(t, tt.wantType, got.Type)
			require.Equal(t, tt.wantCode, got.Code)
			require.Equal(t, tt.wantMessage, got.Message)
		})
	}
}

func TestGatewayHandleStreamingAwareErrorWithCode_NonStreaming(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	h := &GatewayHandler{}
	h.handleStreamingAwareErrorWithCode(c, http.StatusForbidden, "permission_error", "claude_code_client_required", "This group is restricted to Claude Code clients (/v1/messages only)", false)

	require.Equal(t, http.StatusForbidden, w.Code)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &parsed))
	require.Equal(t, "error", parsed["type"])
	errorObj, ok := parsed["error"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "permission_error", errorObj["type"])
	require.Equal(t, "claude_code_client_required", errorObj["code"])
	require.Equal(t, "This group is restricted to Claude Code clients (/v1/messages only)", errorObj["message"])
}

func TestGatewayHandleStreamingAwareErrorWithCode_SSE(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	h := &GatewayHandler{}
	h.handleStreamingAwareErrorWithCode(c, http.StatusForbidden, "permission_error", "claude_code_client_required", "This group is restricted to Claude Code clients (/v1/messages only)", true)

	body := w.Body.String()
	require.True(t, strings.HasPrefix(body, "data: "))
	require.True(t, strings.HasSuffix(body, "\n\n"))

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(strings.TrimSuffix(body, "\n\n"), "data: ")), &parsed))
	errorObj, ok := parsed["error"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "permission_error", errorObj["type"])
	require.Equal(t, "claude_code_client_required", errorObj["code"])
	require.Equal(t, "This group is restricted to Claude Code clients (/v1/messages only)", errorObj["message"])
}

func TestGatewayChatCompletionsClaudeCodeOnlyReturnsStableCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-5.4"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	setGatewayClaudeCodeOnlyContext(c)

	h := &GatewayHandler{}
	require.NotPanics(t, func() {
		h.ChatCompletions(c)
	})

	require.Equal(t, http.StatusForbidden, w.Code)
	errorObj := decodeGatewayErrorObject(t, w.Body.Bytes())
	require.Equal(t, "permission_error", errorObj["type"])
	require.Equal(t, gatewayErrorCodeClaudeCodeClientRequired, errorObj["code"])
	require.Equal(t, "This group is restricted to Claude Code clients (/v1/messages only)", errorObj["message"])
}

func TestGatewayResponsesClaudeCodeOnlyReturnsStableCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5.4"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	setGatewayClaudeCodeOnlyContext(c)

	h := &GatewayHandler{}
	require.NotPanics(t, func() {
		h.Responses(c)
	})

	require.Equal(t, http.StatusForbidden, w.Code)
	errorObj := decodeGatewayErrorObject(t, w.Body.Bytes())
	require.Equal(t, gatewayErrorCodeClaudeCodeClientRequired, errorObj["code"])
	require.Equal(t, "This group is restricted to Claude Code clients (/v1/messages only)", errorObj["message"])
}

func TestGatewayNoAvailableAccountsErrorHelpersEmitStableCode(t *testing.T) {
	t.Run("anthropic messages", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

		h := &GatewayHandler{}
		h.handleNoAvailableAccountsError(c, false)

		require.Equal(t, http.StatusServiceUnavailable, w.Code)
		errorObj := decodeGatewayErrorObject(t, w.Body.Bytes())
		require.Equal(t, "api_error", errorObj["type"])
		require.Equal(t, gatewayErrorCodeNoAvailableAccounts, errorObj["code"])
		require.Equal(t, "No available accounts", errorObj["message"])
	})

	t.Run("chat completions", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

		h := &GatewayHandler{}
		h.chatCompletionsNoAvailableAccountsError(c)

		require.Equal(t, http.StatusServiceUnavailable, w.Code)
		errorObj := decodeGatewayErrorObject(t, w.Body.Bytes())
		require.Equal(t, "api_error", errorObj["type"])
		require.Equal(t, gatewayErrorCodeNoAvailableAccounts, errorObj["code"])
		require.Equal(t, "No available accounts", errorObj["message"])
	})

	t.Run("responses", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

		h := &GatewayHandler{}
		h.responsesNoAvailableAccountsError(c)

		require.Equal(t, http.StatusServiceUnavailable, w.Code)
		errorObj := decodeGatewayErrorObject(t, w.Body.Bytes())
		require.Equal(t, gatewayErrorCodeNoAvailableAccounts, errorObj["code"])
		require.Equal(t, "No available accounts", errorObj["message"])
	})
}

func setGatewayClaudeCodeOnlyContext(c *gin.Context) {
	groupID := int64(7)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		ID:      11,
		GroupID: &groupID,
		Group: &service.Group{
			ID:             groupID,
			ClaudeCodeOnly: true,
		},
	})
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{
		UserID:      1,
		Concurrency: 1,
	})
}

func decodeGatewayErrorObject(t *testing.T, body []byte) map[string]any {
	t.Helper()

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))
	errorObj, ok := parsed["error"].(map[string]any)
	require.True(t, ok)
	return errorObj
}
