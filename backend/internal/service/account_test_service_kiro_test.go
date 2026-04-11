//go:build unit

package service

import (
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAccountTestService_KiroSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()

	resp := newJSONResponse(http.StatusOK, `{"limits":[{"resourceType":"AGENTIC_REQUEST"}]}`)
	upstream := &queuedHTTPUpstream{responses: []*http.Response{resp}}
	svc := &AccountTestService{
		httpUpstream:        upstream,
		kiroTokenProvider:   &KiroTokenProvider{},
		tlsFPProfileService: &TLSFingerprintProfileService{},
	}
	account := &Account{
		ID:          101,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeKiro,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":  "at-1",
			"refresh_token": "rt-1",
			"auth_method":   "social",
			"auth_region":   "us-east-1",
			"api_region":    "us-west-2",
		},
	}

	err := svc.testKiroAccountConnection(ctx, account)
	require.NoError(t, err)
	require.Contains(t, upstream.requests[0].URL.String(), "/getUsageLimits")
	require.Equal(t, "Bearer at-1", upstream.requests[0].Header.Get("Authorization"))
	require.Contains(t, recorder.Body.String(), "test_complete")
}

func TestAccountTestService_KiroUpstreamError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := newTestContext()

	resp := newJSONResponse(http.StatusForbidden, `{"message":"denied"}`)
	upstream := &queuedHTTPUpstream{responses: []*http.Response{resp}}
	svc := &AccountTestService{
		httpUpstream:      upstream,
		kiroTokenProvider: &KiroTokenProvider{},
	}
	account := &Account{
		ID:          102,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeKiro,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":  "at-1",
			"refresh_token": "rt-1",
			"auth_method":   "social",
		},
	}

	err := svc.testKiroAccountConnection(ctx, account)
	require.Error(t, err)
}

func TestAccountTestService_KiroTransportErrorIsSanitized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, recorder := newTestContext()

	upstream := &queuedHTTPUpstream{err: errors.New("dial tcp 10.0.0.1:443: i/o timeout")}
	svc := &AccountTestService{
		httpUpstream:      upstream,
		kiroTokenProvider: &KiroTokenProvider{},
	}
	account := &Account{
		ID:          103,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeKiro,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":  "at-1",
			"refresh_token": "rt-1",
			"auth_method":   "social",
		},
	}

	err := svc.testKiroAccountConnection(ctx, account)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Kiro request failed")
	require.NotContains(t, err.Error(), "10.0.0.1")
	require.Contains(t, recorder.Body.String(), "\"error\":\"Kiro request failed\"")
}
