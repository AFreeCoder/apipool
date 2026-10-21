package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPassthroughModelNotFoundFailoverScope(t *testing.T) {
	managed := &OpenAIGatewayService{accountRepo: &modelNotFoundManagedAccountRepo{}}
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"pool_mode": true}}
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{"production error", 400, `{"error":{"code":"model_not_found","message":"unknown provider for model gpt-6-luna","param":"model","type":"invalid_request_error"}}`, true},
		{"message only", 400, `{"error":{"message":"unknown provider for model gpt-6-luna"}}`, true},
		{"invalid tool schema", 400, openAIInvalidFunctionParametersBody, false},
		{"explicit parameter error", 400, `{"error":{"code":"invalid_request_error","message":"unknown provider for model echoed in input"}}`, false},
		{"context limit", 400, `{"error":{"code":"context_length_exceeded","message":"maximum context length exceeded"}}`, false},
		{"unrelated 404", 404, `{"error":{"message":"Not Found"}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, managed.shouldFailoverOpenAIPassthroughResponse(account, tc.status, []byte(tc.body)))
		})
	}
	body := []byte(`{"error":{"code":"model_not_found"}}`)
	require.False(t, (&OpenAIGatewayService{}).shouldFailoverOpenAIPassthroughResponse(account, http.StatusBadRequest, body))
	require.False(t, managed.shouldFailoverOpenAIPassthroughResponse(nil, http.StatusBadRequest, body))
	require.False(t, managed.shouldFailoverOpenAIPassthroughResponse(&Account{Platform: PlatformAnthropic, Type: AccountTypeAPIKey}, http.StatusBadRequest, body))
}
