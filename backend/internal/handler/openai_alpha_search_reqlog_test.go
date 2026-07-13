package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/reqlog"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAlphaSearchCapturesRequestBodyForReqLog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"gpt-5.6-sol",invalid}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/alpha/search", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	groupID := int64(11)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		ID:      101,
		GroupID: &groupID,
		Group:   &service.Group{ID: groupID, Platform: service.PlatformOpenAI},
	})
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 201, Concurrency: 1})
	reqlog.SetCaptureState(c, &reqlog.CaptureState{
		UserID:           201,
		SessionID:        "rl_alpha_search",
		ExpiresAt:        time.Now().Add(time.Minute),
		SingleRequestCap: 4096,
	})

	h := newOpenAIHandlerForPreviousResponseIDValidation(t, nil)
	h.AlphaSearch(c)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	snapshot, ok := reqlog.RequestBodySnapshot(c)
	require.True(t, ok)
	require.Equal(t, reqlog.BodyKindText, snapshot.Kind)
	require.Equal(t, body, snapshot.Body)
	require.False(t, snapshot.Truncated)
}
