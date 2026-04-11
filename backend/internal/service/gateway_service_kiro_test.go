//go:build unit

package service

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGatewayService_GetAccessToken_Kiro(t *testing.T) {
	t.Parallel()

	provider := NewKiroTokenProvider(nil, nil, nil)
	svc := &GatewayService{kiroTokenProvider: provider}
	account := &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeKiro,
		Credentials: map[string]any{
			"access_token": "at-1",
		},
	}

	token, tokenType, err := svc.GetAccessToken(t.Context(), account)
	require.NoError(t, err)
	require.Equal(t, "at-1", token)
	require.Equal(t, "kiro", tokenType)
}

func TestForwardKiro_NonStreamingSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()

	upstream := &queuedHTTPUpstream{
		responses: []*http.Response{
			newJSONResponse(http.StatusOK, `{"content":"hello from kiro","usage":{"inputTokens":11,"outputTokens":7}}`),
		},
	}
	svc := &GatewayService{
		httpUpstream:        upstream,
		kiroTokenProvider:   NewKiroTokenProvider(nil, nil, nil),
		tlsFPProfileService: &TLSFingerprintProfileService{},
	}
	account := &Account{
		ID:          300,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeKiro,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":  "at-1",
			"refresh_token": "rt-1",
			"auth_method":   "social",
			"api_region":    "us-east-1",
		},
	}
	parsed := &ParsedRequest{
		Body:   []byte(`{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"stream":false}`),
		Model:  "claude-sonnet-4-5",
		Stream: false,
	}

	result, err := svc.forwardKiro(ctx.Request.Context(), ctx, account, parsed, time.Now())
	require.NoError(t, err)
	require.Equal(t, 11, result.Usage.InputTokens)
	require.Equal(t, 7, result.Usage.OutputTokens)
	require.Contains(t, recorder.Body.String(), `"type":"message"`)
}

func TestForwardKiro_StreamingSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()

	upstream := &queuedHTTPUpstream{
		responses: []*http.Response{
			{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(
					"{\"type\":\"assistantResponseEvent\",\"content\":\"hello\"}\n" +
						"{\"type\":\"contextUsageEvent\",\"contextUsagePercentage\":50}\n",
				)),
			},
		},
	}
	svc := &GatewayService{
		httpUpstream:        upstream,
		kiroTokenProvider:   NewKiroTokenProvider(nil, nil, nil),
		tlsFPProfileService: &TLSFingerprintProfileService{},
	}
	account := &Account{
		ID:          301,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeKiro,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":  "at-1",
			"refresh_token": "rt-1",
			"auth_method":   "social",
			"api_region":    "us-east-1",
		},
	}
	parsed := &ParsedRequest{
		Body:   []byte(`{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"stream":true}`),
		Model:  "claude-sonnet-4-5",
		Stream: true,
	}

	result, err := svc.forwardKiro(ctx.Request.Context(), ctx, account, parsed, time.Now())
	require.NoError(t, err)
	require.True(t, result.Stream)
	require.NotZero(t, result.Usage.InputTokens)
	require.Contains(t, recorder.Body.String(), "event: message_start")
	require.Contains(t, recorder.Body.String(), "event: message_stop")
}
