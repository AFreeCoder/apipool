package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/reqlog"
	"github.com/stretchr/testify/require"
)

type reqLogMemoryStore struct {
	enabled       atomic.Value
	gets          atomic.Int64
	drops         atomic.Int64
	writes        atomic.Int64
	resolveUserID int64
	active        []*reqlog.CaptureState
	listItems     []*reqlog.ReqLogEntry
	getItem       *reqlog.ReqLogEntry
	consumeToken  *ReqLogDownloadToken
	createToken   atomic.Int64
	writeReturn   int64 // >0: 用作 WriteItem 返回 seq；<0: 返回 0（模拟 Lua 丢弃）；0: 默认用 writes 计数
	memStats      *ReqLogRedisMemoryStats
}

func (s *reqLogMemoryStore) GetEnabled(ctx context.Context, userID int64) (*reqlog.CaptureState, error) {
	s.gets.Add(1)
	v := s.enabled.Load()
	if v == nil {
		return nil, nil
	}
	st := v.(*reqlog.CaptureState)
	if st == nil {
		return nil, nil
	}
	cp := *st
	return &cp, nil
}

func (s *reqLogMemoryStore) EnableSession(context.Context, *reqlog.CaptureState, time.Duration, time.Duration, bool, int) (*reqlog.CaptureState, bool, error) {
	return nil, false, nil
}
func (s *reqLogMemoryStore) DisableSession(context.Context, int64) error { return nil }
func (s *reqLogMemoryStore) CountEnabled(context.Context) (int, error)   { return 0, nil }
func (s *reqLogMemoryStore) ListActive(context.Context) ([]*reqlog.CaptureState, error) {
	out := make([]*reqlog.CaptureState, 0, len(s.active))
	for _, st := range s.active {
		out = append(out, st.Clone())
	}
	return out, nil
}
func (s *reqLogMemoryStore) WriteItem(context.Context, *reqlog.ReqLogEntry, *reqlog.CaptureState, time.Duration) (int64, error) {
	s.writes.Add(1)
	if s.writeReturn < 0 {
		return 0, nil
	}
	if s.writeReturn > 0 {
		return s.writeReturn, nil
	}
	return s.writes.Load(), nil
}
func (s *reqLogMemoryStore) DropItem(context.Context, *reqlog.CaptureState) error {
	s.drops.Add(1)
	return nil
}
func (s *reqLogMemoryStore) GetStats(context.Context, int64, string) (*ReqLogSessionStats, error) {
	return nil, nil
}
func (s *reqLogMemoryStore) ResolveSessionUser(context.Context, string) (int64, error) {
	if s.resolveUserID <= 0 {
		return 0, ErrReqLogNotFound
	}
	return s.resolveUserID, nil
}
func (s *reqLogMemoryStore) ListSessions(context.Context, int64, int) ([]ReqLogSession, error) {
	return nil, nil
}
func (s *reqLogMemoryStore) ListItems(context.Context, string, int, int) ([]*reqlog.ReqLogEntry, int64, error) {
	if len(s.listItems) == 0 {
		return nil, 0, nil
	}
	out := make([]*reqlog.ReqLogEntry, len(s.listItems))
	for i, item := range s.listItems {
		out[i] = item.DeepCopy()
	}
	s.listItems = nil
	return out, int64(len(out)), nil
}
func (s *reqLogMemoryStore) GetItem(context.Context, string, int64) (*reqlog.ReqLogEntry, error) {
	if s.getItem == nil {
		return nil, ErrReqLogNotFound
	}
	return s.getItem.DeepCopy(), nil
}
func (s *reqLogMemoryStore) CreateDownloadToken(context.Context, string, int64, time.Duration) (string, time.Time, error) {
	s.createToken.Add(1)
	return "tok", time.Now().Add(time.Minute), nil
}
func (s *reqLogMemoryStore) ConsumeDownloadToken(context.Context, string) (*ReqLogDownloadToken, error) {
	if s.consumeToken == nil {
		return nil, ErrReqLogUnauthorized
	}
	cp := *s.consumeToken
	return &cp, nil
}
func (s *reqLogMemoryStore) MemoryStats(context.Context) (*ReqLogRedisMemoryStats, error) {
	if s.memStats == nil {
		return nil, nil
	}
	cp := *s.memStats
	return &cp, nil
}
func (s *reqLogMemoryStore) Close() error { return nil }

func TestReqLogServiceGenerationInvalidatesPositiveCache(t *testing.T) {
	store := &reqLogMemoryStore{}
	store.enabled.Store(&reqlog.CaptureState{
		UserID:           42,
		SessionID:        "rl_generation",
		ExpiresAt:        time.Now().Add(time.Minute),
		SingleRequestCap: 1024,
	})
	svc := NewReqLogService(&config.Config{Ops: config.OpsConfig{RequestLog: config.OpsRequestLogConfig{
		Enabled:       true,
		LocalCacheTTL: time.Minute,
	}}}, store, nil)

	st, ok := svc.GetCaptureState(context.Background(), 42, time.Now())
	require.True(t, ok)
	require.Equal(t, "rl_generation", st.SessionID)
	require.Equal(t, int64(1), store.gets.Load())

	store.enabled.Store((*reqlog.CaptureState)(nil))
	svc.IncrementGeneration()

	st, ok = svc.GetCaptureState(context.Background(), 42, time.Now())
	require.False(t, ok)
	require.Nil(t, st)
	require.Equal(t, int64(2), store.gets.Load())
}

func TestTruncateUTF8KeepsValidString(t *testing.T) {
	body, truncated := TruncateBody([]byte("你好世界"), 5)
	require.True(t, truncated)
	require.Equal(t, "你", string(body))
}

func TestRedactHeadersRedactsSensitiveKeysAndPreservesNormalValues(t *testing.T) {
	headers := http.Header{}
	headers.Set("Authorization", "Bearer sk-secret")
	headers.Set("X-API-Key", "sk-test")
	headers.Set("User-Agent", "client access_token=literal-should-stay")
	headers.Set("X-Trace", "AIzaSyA12345678901234567890123456789012345")

	out := RedactHeaders(headers, 4096)

	require.Equal(t, "***", out["Authorization"])
	require.Equal(t, "***", out["X-Api-Key"])
	require.Equal(t, "client access_token=literal-should-stay", out["User-Agent"])
	require.Equal(t, "AIzaSyA12345678901234567890123456789012345", out["X-Trace"])
}

func TestReqLogSinkProcessAlwaysRevalidatesEnabledEvenWithoutConfig(t *testing.T) {
	store := &reqLogMemoryStore{}
	sink := NewReqLogSink(store, nil)

	sink.process(reqLogQueuedEntry{
		entry: &reqlog.ReqLogEntry{
			UserID:    42,
			SessionID: "rl_disabled",
			Timestamp: time.Now().UTC(),
			Method:    "POST",
			Path:      "/v1/messages",
		},
		bytes: 128,
	})

	require.Equal(t, int64(1), store.gets.Load())
	require.Equal(t, int64(1), store.drops.Load())
	require.Equal(t, int64(0), store.writes.Load())
	require.Equal(t, uint64(1), sink.Health().DroppedCount)
}

func TestReqLogSinkProcessDropsEntryAfterEnabledWindow(t *testing.T) {
	store := &reqLogMemoryStore{}
	store.enabled.Store(&reqlog.CaptureState{
		UserID:           42,
		SessionID:        "rl_expired_entry",
		ExpiresAt:        time.Now().Add(-time.Second),
		MaxBytes:         1024,
		MaxItems:         10,
		OverflowStrategy: reqlog.OverflowDropOldest,
	})
	sink := NewReqLogSink(store, &config.Config{})

	sink.process(reqLogQueuedEntry{
		entry: &reqlog.ReqLogEntry{
			UserID:    42,
			SessionID: "rl_expired_entry",
			Timestamp: time.Now().UTC(),
			Method:    "POST",
			Path:      "/v1/messages",
		},
		bytes: 128,
	})

	require.Equal(t, int64(1), store.gets.Load())
	require.Equal(t, int64(1), store.drops.Load())
	require.Equal(t, int64(0), store.writes.Load())
}

func TestReqLogSinkSubmitDropsWhenInflightBytesExceedsBudget(t *testing.T) {
	store := &reqLogMemoryStore{}
	sink := NewReqLogSink(store, &config.Config{Ops: config.OpsConfig{RequestLog: config.OpsRequestLogConfig{
		QueueCapacity:   1,
		QueueByteBudget: 64,
	}}})

	ok := sink.Submit(&reqlog.ReqLogEntry{
		UserID:    42,
		SessionID: "rl_budget",
		Timestamp: time.Now().UTC(),
		Method:    "POST",
		Path:      "/v1/messages",
		ReqBody:   []byte(strings.Repeat("x", 512)),
	})

	require.False(t, ok)
	health := sink.Health()
	require.Equal(t, uint64(1), health.DroppedCount)
	require.Equal(t, int64(0), health.InflightBytes)
}

func TestReqLogServiceAuditsListViewAndDownloadBeforeStreaming(t *testing.T) {
	store := &reqLogMemoryStore{
		resolveUserID: 42,
		listItems: []*reqlog.ReqLogEntry{{
			UserID:    42,
			SessionID: "rl_audit",
			Seq:       1,
			Timestamp: time.Now().UTC(),
			Method:    "POST",
			Path:      "/v1/messages",
		}},
		getItem: &reqlog.ReqLogEntry{
			UserID:    42,
			SessionID: "rl_audit",
			Seq:       1,
			Timestamp: time.Now().UTC(),
			Method:    "POST",
			Path:      "/v1/messages",
		},
	}
	svc := NewReqLogService(&config.Config{Ops: config.OpsConfig{RequestLog: config.OpsRequestLogConfig{Enabled: true}}}, store, nil)
	sink := &captureLoggerSink{}
	logger.SetSink(sink)
	t.Cleanup(func() { logger.SetSink(nil) })

	_, _, _, err := svc.ListItems(context.Background(), 7, "rl_audit", 1, 20)
	require.NoError(t, err)
	_, _, err = svc.GetItem(context.Background(), 7, "rl_audit", 1)
	require.NoError(t, err)
	store.listItems = []*reqlog.ReqLogEntry{store.getItem.DeepCopy()}
	_, err = svc.DownloadItems(context.Background(), 7, "rl_audit", func(*reqlog.ReqLogEntry) error {
		return io.ErrClosedPipe
	})
	require.True(t, errors.Is(err, io.ErrClosedPipe))

	require.True(t, sink.hasReqLogAction("list_items"), fmt.Sprintf("events=%v", sink.events))
	require.True(t, sink.hasReqLogAction("view_item"), fmt.Sprintf("events=%v", sink.events))
	require.True(t, sink.hasReqLogAction("download"), fmt.Sprintf("events=%v", sink.events))
}

func TestReqLogServiceConsumeDownloadTokenRejectsSessionMismatch(t *testing.T) {
	store := &reqLogMemoryStore{consumeToken: &ReqLogDownloadToken{
		SessionID: "rl_other",
		AdminID:   7,
		ExpiresAt: time.Now().Add(time.Minute),
	}}
	svc := NewReqLogService(&config.Config{Ops: config.OpsConfig{RequestLog: config.OpsRequestLogConfig{Enabled: true}}}, store, nil)

	adminID, err := svc.ConsumeDownloadToken(context.Background(), "token", "rl_expected")

	require.ErrorIs(t, err, ErrReqLogUnauthorized)
	require.Zero(t, adminID)
}

func TestReqLogServiceListActiveReturnsCurrentSessions(t *testing.T) {
	now := time.Now().UTC()
	store := &reqLogMemoryStore{active: []*reqlog.CaptureState{
		{
			UserID:    42,
			SessionID: "rl_active",
			StartedAt: now.Add(-time.Minute),
			ExpiresAt: now.Add(time.Minute),
			Reason:    "debug",
		},
		{
			UserID:    43,
			SessionID: "rl_expired",
			StartedAt: now.Add(-time.Minute),
			ExpiresAt: now.Add(-time.Second),
		},
	}}
	svc := NewReqLogService(&config.Config{Ops: config.OpsConfig{RequestLog: config.OpsRequestLogConfig{Enabled: true}}}, store, nil)

	items, err := svc.ListActive(context.Background())

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(42), items[0].UserID)
	require.Equal(t, "rl_active", items[0].SessionID)
	require.Equal(t, now.Add(time.Minute).Unix(), items[0].ExpiresAt)
	require.Greater(t, items[0].RemainingSeconds, int64(0))
	require.Equal(t, "debug", items[0].Reason)
}

type captureLoggerSink struct {
	events []*logger.LogEvent
}

func (s *captureLoggerSink) WriteLogEvent(event *logger.LogEvent) {
	if event == nil {
		return
	}
	cp := *event
	if event.Fields != nil {
		cp.Fields = make(map[string]any, len(event.Fields))
		for k, v := range event.Fields {
			cp.Fields[k] = v
		}
	}
	s.events = append(s.events, &cp)
}

func (s *captureLoggerSink) hasReqLogAction(action string) bool {
	for _, event := range s.events {
		if event == nil || event.Fields == nil {
			continue
		}
		if event.Component == "audit.reqlog" && event.Fields["action"] == action {
			return true
		}
	}
	return false
}
