//go:build unit

package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestForwardAsAnthropicGrokEncryptedRetryDoesNotLogRawUpstreamBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{
		"model":"grok",
		"max_tokens":32,
		"stream":false,
		"messages":[
			{"role":"assistant","content":[{"type":"thinking","thinking":"plan","signature":"encrypted-reasoning"},{"type":"text","text":"previous"}]},
			{"role":"user","content":"hi"}
		]
	}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Set("api_key", &APIKey{ID: 6101})

	account := healthyGrokOAuthGatewayTestAccount(61, "access-token")
	repo := &grokQuotaAccountRepo{
		mockAccountRepoForPlatform: &mockAccountRepoForPlatform{
			accountsByID: map[int64]*Account{61: account},
		},
	}
	sensitiveError := `{"code":"invalid-argument","error":"Could not decrypt encrypted_content for customer-secret-prompt"}`
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		{
			StatusCode: http.StatusBadRequest,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(sensitiveError)),
		},
		grokMessagesSSECompletedResponse("resp_grok_retry", 0),
	}}
	svc := &OpenAIGatewayService{
		httpUpstream:      upstream,
		grokTokenProvider: NewGrokTokenProvider(repo, nil),
		accountRepo:       repo,
	}
	logSink, restore := captureStructuredLog(t)
	defer restore()

	result, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, "", "")

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.requests, 2)
	require.True(t, logSink.ContainsField("upstream_error_body_len"))
	require.True(t, logSink.ContainsFieldValue("upstream_error_body_sha256", hashSensitiveValueForLog(sensitiveError)))
	require.False(t, logSink.ContainsField("upstream_error_preview"))
	require.False(t, logSink.ContainsMessage("customer-secret-prompt"))
}
