package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/reqlog"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type countTokensEligibilityStub struct {
	err   error
	calls int
}

func (s *countTokensEligibilityStub) CheckBillingEligibility(
	context.Context,
	*service.User,
	*service.APIKey,
	*service.Group,
	*service.UserSubscription,
	string,
) error {
	s.calls++
	return s.err
}

func newGrokCountTokensTestContext(t *testing.T, body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	groupID := int64(17)
	user := &service.User{ID: 19}
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		ID:      23,
		GroupID: &groupID,
		Group:   &service.Group{ID: groupID, Platform: service.PlatformGrok},
		User:    user,
	})
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: user.ID, Concurrency: 1})
	return c, recorder
}

func TestGrokCountTokensFailsClosedWithoutEligibilityChecker(t *testing.T) {
	body := []byte(`{"model":"grok","messages":[{"role":"user","content":"hi"}]}`)
	c, recorder := newGrokCountTokensTestContext(t, body)
	h := &OpenAIGatewayHandler{cfg: &config.Config{
		RunMode: config.RunModeStandard,
		Gateway: config.GatewayConfig{MaxBodySize: 1024},
	}}

	h.GrokCountTokens(c)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Contains(t, recorder.Body.String(), "Service temporarily unavailable")
}

func TestGrokCountTokensUsesSharedRPMEligibilityGate(t *testing.T) {
	body := []byte(`{"model":"grok","messages":[{"role":"user","content":"hi"}]}`)
	c, recorder := newGrokCountTokensTestContext(t, body)
	checker := &countTokensEligibilityStub{err: service.ErrUserRPMExceeded}
	cache := &concurrencyCacheMock{
		acquireUserSlotFn: func(context.Context, int64, int, string) (bool, error) {
			return true, nil
		},
	}
	h := &OpenAIGatewayHandler{
		billingCacheService: checker,
		concurrencyHelper:   NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatNone, time.Second),
		cfg: &config.Config{
			RunMode: config.RunModeStandard,
			Gateway: config.GatewayConfig{MaxBodySize: 1024},
		},
	}

	h.GrokCountTokens(c)

	require.Equal(t, http.StatusTooManyRequests, recorder.Code)
	require.Equal(t, 1, checker.calls)
	require.NotEmpty(t, recorder.Header().Get("Retry-After"))
	require.Contains(t, recorder.Body.String(), "rate_limit_exceeded")
	require.Equal(t, int32(1), cache.releaseUserCalled)
}

func TestGrokCountTokensCapturesBodyAfterEligibility(t *testing.T) {
	body := []byte(`{"model":"grok","messages":[{"role":"user","content":"hi"}]}`)
	c, recorder := newGrokCountTokensTestContext(t, body)
	reqlog.SetCaptureState(c, &reqlog.CaptureState{
		UserID:           19,
		SessionID:        "rl_grok_count_tokens",
		ExpiresAt:        time.Now().Add(time.Minute),
		SingleRequestCap: 1024,
	})
	checker := &countTokensEligibilityStub{}
	cache := &concurrencyCacheMock{
		acquireUserSlotFn: func(context.Context, int64, int, string) (bool, error) {
			return true, nil
		},
	}
	h := &OpenAIGatewayHandler{
		billingCacheService: checker,
		concurrencyHelper:   NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatNone, time.Second),
		cfg: &config.Config{
			RunMode: config.RunModeStandard,
			Gateway: config.GatewayConfig{MaxBodySize: 1024},
		},
	}

	h.GrokCountTokens(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, 1, checker.calls)
	snapshot, ok := reqlog.RequestBodySnapshot(c)
	require.True(t, ok)
	require.Equal(t, body, snapshot.Body)
	require.False(t, snapshot.Truncated)
	require.Equal(t, int32(1), cache.releaseUserCalled)
}

func TestGrokCountTokensRejectsWhenUserConcurrencyIsFull(t *testing.T) {
	body := []byte(`{"model":"grok","messages":[{"role":"user","content":"hi"}]}`)
	c, recorder := newGrokCountTokensTestContext(t, body)
	checker := &countTokensEligibilityStub{}
	cache := &concurrencyCacheMock{
		acquireUserSlotFn: func(context.Context, int64, int, string) (bool, error) {
			return false, nil
		},
	}
	h := &OpenAIGatewayHandler{
		billingCacheService: checker,
		concurrencyHelper:   NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatNone, time.Second),
		cfg: &config.Config{
			RunMode: config.RunModeStandard,
			Gateway: config.GatewayConfig{MaxBodySize: 1024},
		},
	}

	h.GrokCountTokens(c)

	require.Equal(t, http.StatusTooManyRequests, recorder.Code)
	require.Equal(t, 0, checker.calls)
	require.Contains(t, recorder.Body.String(), "rate_limit_error")
	require.Equal(t, int32(0), cache.releaseUserCalled)
}
