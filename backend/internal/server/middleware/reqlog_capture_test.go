package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/reqlog"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type reqLogCaptureStub struct {
	state     *reqlog.CaptureState
	submitted *reqlog.ReqLogEntry
}

func (s *reqLogCaptureStub) GetCaptureState(ctx context.Context, userID int64, now time.Time) (*reqlog.CaptureState, bool) {
	if s.state == nil {
		return nil, false
	}
	cp := *s.state
	return &cp, true
}

func (s *reqLogCaptureStub) Submit(entry *reqlog.ReqLogEntry) bool {
	cp := entry.DeepCopy()
	s.submitted = cp
	return true
}

func TestReqLogCaptureMiddlewareTransparentAndDeepCopies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &reqLogCaptureStub{state: &reqlog.CaptureState{
		UserID:            12,
		SessionID:         "rl_mid",
		ExpiresAt:         time.Now().Add(time.Minute),
		SingleRequestCap:  1024,
		SingleResponseCap: 1024,
	}}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyUser), AuthSubject{UserID: 12})
		ctx := context.WithValue(c.Request.Context(), ctxkey.InboundEndpoint, "/v1/messages")
		ctx = context.WithValue(ctx, ctxkey.RequestID, "req-1")
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.Use(ReqLogCaptureMiddleware(stub))
	router.POST("/v1/messages", func(c *gin.Context) {
		reqlog.MaybeCaptureRequestBody(c, []byte(`{"model":"claude","stream":true}`), "application/json")
		c.Header("Content-Type", "text/event-stream")
		_, _ = c.Writer.WriteString("data: hello\n\n")
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/messages", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "data: hello\n\n", rec.Body.String())
	require.NotNil(t, stub.submitted)
	require.Equal(t, "req-1", stub.submitted.RequestID)
	require.Equal(t, "sse", stub.submitted.Transport)
	require.Equal(t, []byte("data: hello\n\n"), stub.submitted.RespBody)
	require.JSONEq(t, `{"model":"claude","stream":true}`, string(stub.submitted.ReqBody))
}

func TestReqLogCaptureMiddlewareSkipsResponseWriterForWebSocketUpgrade(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &reqLogCaptureStub{state: &reqlog.CaptureState{
		UserID:            12,
		SessionID:         "rl_ws",
		ExpiresAt:         time.Now().Add(time.Minute),
		SingleRequestCap:  1024,
		SingleResponseCap: 1024,
	}}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyUser), AuthSubject{UserID: 12})
		c.Next()
	})
	router.Use(ReqLogCaptureMiddleware(stub))
	router.GET("/responses", func(c *gin.Context) {
		_, wrapped := c.Writer.(*reqlog.CaptureWriter)
		require.False(t, wrapped)
		c.Status(http.StatusSwitchingProtocols)
	})

	req := httptest.NewRequest(http.MethodGet, "/responses", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusSwitchingProtocols, rec.Code)
	require.NotNil(t, stub.submitted)
	require.Equal(t, "ws", stub.submitted.Transport)
	require.Empty(t, stub.submitted.RespBody)
}

func TestShouldCaptureResponseUsesExactMetadataPaths(t *testing.T) {
	cases := []struct {
		name string
		path string
		want bool
	}{
		{name: "v1 models", path: "/v1/models", want: false},
		{name: "root usage", path: "/usage", want: false},
		{name: "gemini model detail", path: "/v1beta/models/gemini-pro", want: false},
		{name: "antigravity usage", path: "/antigravity/v1/usage", want: false},
		{name: "models substring is not metadata", path: "/v1/models-extra", want: true},
		{name: "business path containing models segment elsewhere", path: "/v1/foo/models-extra", want: true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			require.Equal(t, tt.want, shouldCaptureResponse(req))
		})
	}
}
