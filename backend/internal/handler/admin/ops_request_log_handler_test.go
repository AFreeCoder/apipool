package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/reqlog"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpsRequestLogHandlerListActiveRequestLogs(t *testing.T) {
	now := time.Now().UTC()
	active := newHandlerReqLogState(42, "rl_handler_active", now.Add(time.Minute))
	expired := newHandlerReqLogState(44, "rl_handler_expired", now.Add(-time.Minute))
	store := &handlerReqLogStore{active: []*reqlog.CaptureState{active, expired}}

	reqLogSvc := service.NewReqLogService(&config.Config{Ops: config.OpsConfig{
		RequestLog: config.OpsRequestLogConfig{Enabled: true},
	}}, store, nil)
	handler := NewOpsHandler(nil, reqLogSvc, nil)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/active", handler.ListActiveRequestLogs)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/active", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Code int `json:"code"`
		Data struct {
			Items []service.ReqLogActiveSession `json:"items"`
			Count int                           `json:"count"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, 0, body.Code)
	require.Equal(t, 1, body.Data.Count)
	require.Len(t, body.Data.Items, 1)
	require.Equal(t, int64(42), body.Data.Items[0].UserID)
	require.Equal(t, "rl_handler_active", body.Data.Items[0].SessionID)
	require.Equal(t, active.ExpiresAt.Unix(), body.Data.Items[0].ExpiresAt)
}

type handlerReqLogStore struct {
	active []*reqlog.CaptureState
}

func (s *handlerReqLogStore) GetEnabled(context.Context, int64) (*reqlog.CaptureState, error) {
	return nil, nil
}

func (s *handlerReqLogStore) EnableSession(context.Context, *reqlog.CaptureState, time.Duration, time.Duration, bool, int) (*reqlog.CaptureState, bool, error) {
	return nil, false, nil
}

func (s *handlerReqLogStore) DisableSession(context.Context, int64) error {
	return nil
}

func (s *handlerReqLogStore) CountEnabled(context.Context) (int, error) {
	return len(s.active), nil
}

func (s *handlerReqLogStore) ListActive(context.Context) ([]*reqlog.CaptureState, error) {
	return s.active, nil
}

func (s *handlerReqLogStore) WriteItem(context.Context, *reqlog.ReqLogEntry, *reqlog.CaptureState, time.Duration) (int64, error) {
	return 0, nil
}

func (s *handlerReqLogStore) DropItem(context.Context, *reqlog.CaptureState) error {
	return nil
}

func (s *handlerReqLogStore) GetStats(context.Context, int64, string) (*service.ReqLogSessionStats, error) {
	return nil, nil
}

func (s *handlerReqLogStore) ResolveSessionUser(context.Context, string) (int64, error) {
	return 0, nil
}

func (s *handlerReqLogStore) ListSessions(context.Context, int64, int) ([]service.ReqLogSession, error) {
	return nil, nil
}

func (s *handlerReqLogStore) ListItems(context.Context, string, int, int) ([]*reqlog.ReqLogEntry, int64, error) {
	return nil, 0, nil
}

func (s *handlerReqLogStore) GetItem(context.Context, string, int64) (*reqlog.ReqLogEntry, error) {
	return nil, nil
}

func (s *handlerReqLogStore) CreateDownloadToken(context.Context, string, int64, time.Duration) (string, time.Time, error) {
	return "", time.Time{}, nil
}

func (s *handlerReqLogStore) ConsumeDownloadToken(context.Context, string) (*service.ReqLogDownloadToken, error) {
	return nil, nil
}

func (s *handlerReqLogStore) MemoryStats(context.Context) (*service.ReqLogRedisMemoryStats, error) {
	return nil, nil
}

func (s *handlerReqLogStore) Close() error {
	return nil
}

func newHandlerReqLogState(userID int64, sessionID string, expiresAt time.Time) *reqlog.CaptureState {
	return &reqlog.CaptureState{
		UserID:            userID,
		SessionID:         sessionID,
		StartedAt:         time.Now().UTC().Add(-time.Minute),
		ExpiresAt:         expiresAt,
		MaxBytes:          4096,
		MaxItems:          10,
		OverflowStrategy:  reqlog.OverflowDropOldest,
		SingleRequestCap:  1024,
		SingleResponseCap: 1024,
		Reason:            "debug",
	}
}
