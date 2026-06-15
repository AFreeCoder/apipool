package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/reqlog"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type reqLogCaptureHandlerStub struct {
	state     *reqlog.CaptureState
	submitted *reqlog.ReqLogEntry
}

func (s *reqLogCaptureHandlerStub) GetCaptureState(context.Context, int64, time.Time) (*reqlog.CaptureState, bool) {
	if s.state == nil {
		return nil, false
	}
	return s.state.Clone(), true
}

func (s *reqLogCaptureHandlerStub) Submit(entry *reqlog.ReqLogEntry) bool {
	s.submitted = entry.DeepCopy()
	return true
}

func TestReqLogCaptureNestedInsideOpsErrorLoggerIsTransparent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &reqLogCaptureHandlerStub{state: &reqlog.CaptureState{
		UserID:            9,
		SessionID:         "rl_nested",
		ExpiresAt:         time.Now().Add(time.Minute),
		SingleRequestCap:  1024,
		SingleResponseCap: 1024,
	}}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 9})
		c.Next()
	})
	router.Use(OpsErrorLoggerMiddleware(nil))
	router.Use(middleware.ReqLogCaptureMiddleware(stub))
	router.GET("/v1/messages", func(c *gin.Context) {
		c.Header("X-Test-Header", "kept")
		c.Status(http.StatusTeapot)
		_, _ = c.Writer.WriteString("short and stout")
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/messages", nil))

	require.Equal(t, http.StatusTeapot, rec.Code)
	require.Equal(t, "kept", rec.Header().Get("X-Test-Header"))
	require.Equal(t, "short and stout", rec.Body.String())
	require.NotNil(t, stub.submitted)
	require.Equal(t, http.StatusTeapot, stub.submitted.StatusCode)
	require.Equal(t, []byte("short and stout"), stub.submitted.RespBody)
}
