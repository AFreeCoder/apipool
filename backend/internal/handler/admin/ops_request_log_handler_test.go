package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/reqlog"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestOpsRequestLogHandlerListActiveRequestLogs(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		if strings.Contains(err.Error(), "bind: operation not permitted") {
			t.Skipf("当前沙箱禁止本地 TCP 监听，跳过 miniredis 用例: %v", err)
		}
		t.Fatalf("could not start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	store := repository.NewReqLogStoreForClient(rdb)
	ctx := context.Background()
	now := time.Now().UTC()
	active := newHandlerReqLogState(42, "rl_handler_active", now.Add(time.Minute))
	require.NoError(t, store.CreateSession(ctx, active, time.Minute, time.Hour, false))

	disabled := newHandlerReqLogState(43, "rl_handler_disabled", now.Add(time.Minute))
	require.NoError(t, store.CreateSession(ctx, disabled, time.Minute, time.Hour, false))
	require.NoError(t, store.DisableSession(ctx, disabled.UserID))

	expired := newHandlerReqLogState(44, "rl_handler_expired", now.Add(-time.Minute))
	require.NoError(t, store.CreateSession(ctx, expired, time.Minute, time.Hour, false))

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
