//go:build unit

package service

import (
	"bytes"
	"encoding/binary"
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

func TestForwardKiro_NonStreamingEventStreamSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()

	upstream := &queuedHTTPUpstream{
		responses: []*http.Response{
			newKiroEventStreamResponse(
				kiroTestEventFrame{
					messageType: "event",
					eventType:   "assistantResponseEvent",
					payload:     `{"content":"hello from eventstream"}`,
				},
				kiroTestEventFrame{
					messageType: "event",
					eventType:   "contextUsageEvent",
					payload:     `{"contextUsagePercentage":50}`,
				},
			),
		},
	}
	svc := &GatewayService{
		httpUpstream:        upstream,
		kiroTokenProvider:   NewKiroTokenProvider(nil, nil, nil),
		tlsFPProfileService: &TLSFingerprintProfileService{},
	}
	account := &Account{
		ID:          302,
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
	require.NotZero(t, result.Usage.InputTokens)
	require.Contains(t, recorder.Body.String(), `"hello from eventstream"`)
}

func TestForwardKiro_StreamingEventStreamSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()

	upstream := &queuedHTTPUpstream{
		responses: []*http.Response{
			newKiroEventStreamResponse(
				kiroTestEventFrame{
					messageType: "event",
					eventType:   "assistantResponseEvent",
					payload:     `{"content":"hello"}`,
				},
				kiroTestEventFrame{
					messageType: "event",
					eventType:   "contextUsageEvent",
					payload:     `{"contextUsagePercentage":50}`,
				},
			),
		},
	}
	svc := &GatewayService{
		httpUpstream:        upstream,
		kiroTokenProvider:   NewKiroTokenProvider(nil, nil, nil),
		tlsFPProfileService: &TLSFingerprintProfileService{},
	}
	account := &Account{
		ID:          303,
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
	require.Contains(t, recorder.Body.String(), "event: content_block_delta")
}

type kiroTestEventFrame struct {
	messageType   string
	eventType     string
	exceptionType string
	errorCode     string
	payload       string
}

func newKiroEventStreamResponse(frames ...kiroTestEventFrame) *http.Response {
	resp := newJSONResponse(http.StatusOK, "")
	resp.Header.Set("Content-Type", "application/vnd.amazon.eventstream")
	resp.Body = io.NopCloser(bytes.NewReader(buildKiroEventStream(frames...)))
	return resp
}

func buildKiroEventStream(frames ...kiroTestEventFrame) []byte {
	var buf bytes.Buffer
	for _, frame := range frames {
		headers := make([]byte, 0)
		headers = append(headers, encodeKiroEventHeader(":message-type", frame.messageType)...)
		if frame.eventType != "" {
			headers = append(headers, encodeKiroEventHeader(":event-type", frame.eventType)...)
		}
		if frame.exceptionType != "" {
			headers = append(headers, encodeKiroEventHeader(":exception-type", frame.exceptionType)...)
		}
		if frame.errorCode != "" {
			headers = append(headers, encodeKiroEventHeader(":error-code", frame.errorCode)...)
		}

		payload := []byte(frame.payload)
		totalLength := uint32(12 + len(headers) + len(payload) + 4)
		_ = binary.Write(&buf, binary.BigEndian, totalLength)
		_ = binary.Write(&buf, binary.BigEndian, uint32(len(headers)))
		_ = binary.Write(&buf, binary.BigEndian, uint32(0))
		_, _ = buf.Write(headers)
		_, _ = buf.Write(payload)
		_ = binary.Write(&buf, binary.BigEndian, uint32(0))
	}
	return buf.Bytes()
}

func encodeKiroEventHeader(name string, value string) []byte {
	out := make([]byte, 0, 1+len(name)+1+2+len(value))
	out = append(out, byte(len(name)))
	out = append(out, []byte(name)...)
	out = append(out, byte(7))
	length := make([]byte, 2)
	binary.BigEndian.PutUint16(length, uint16(len(value)))
	out = append(out, length...)
	out = append(out, []byte(value)...)
	return out
}
