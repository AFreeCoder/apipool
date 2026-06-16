package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/reqlog"
	"github.com/stretchr/testify/require"
)

func newReqLogSinkTestCfg() *config.Config {
	return &config.Config{Ops: config.OpsConfig{RequestLog: config.OpsRequestLogConfig{
		Enabled:              true,
		RetentionAfterWindow: time.Hour,
		MemoryGuardPercent:   80,
		MemoryInfoCacheTTL:   time.Second,
		MaxBytesPerSession:   32 * 1024 * 1024,
		MaxItemsPerSession:   1000,
		SingleRequestCap:     256 * 1024,
		SingleResponseCap:    256 * 1024,
	}}}
}

// P1：force 重开 / disable+重开 后，旧会话在途 entry 必须被丢弃，绝不写进新会话。
func TestReqLogSinkProcessDropsCrossSessionEntry(t *testing.T) {
	store := &reqLogMemoryStore{resolveUserID: 42}
	store.enabled.Store(&reqlog.CaptureState{
		UserID:    42,
		SessionID: "rl_new_B",
		ExpiresAt: time.Now().Add(time.Minute),
	})
	sink := NewReqLogSink(store, newReqLogSinkTestCfg())

	entry := &reqlog.ReqLogEntry{
		UserID:    42,
		SessionID: "rl_old_A", // 旧会话
		Timestamp: time.Now(),
	}
	sink.process(reqLogQueuedEntry{entry: entry, bytes: entry.EstimateBytes()})

	require.Equal(t, int64(0), store.writes.Load(), "旧会话 entry 不应写入")
	require.Equal(t, int64(1), store.drops.Load(), "旧会话 entry 应被丢弃")
	require.Equal(t, uint64(1), sink.droppedCount.Load())
	require.Equal(t, uint64(0), sink.writtenCount.Load())
}

// P9：Lua 因二次校验/预算返回 seq==0（无 err）时应计入 dropped，而非 written。
func TestReqLogSinkProcessCountsLuaDropAsDropped(t *testing.T) {
	store := &reqLogMemoryStore{resolveUserID: 7, writeReturn: -1} // 强制 WriteItem 返回 (0,nil)
	store.enabled.Store(&reqlog.CaptureState{
		UserID:    7,
		SessionID: "rl_sess",
		ExpiresAt: time.Now().Add(time.Minute),
	})
	sink := NewReqLogSink(store, newReqLogSinkTestCfg())

	entry := &reqlog.ReqLogEntry{UserID: 7, SessionID: "rl_sess", Timestamp: time.Now()}
	sink.process(reqLogQueuedEntry{entry: entry, bytes: entry.EstimateBytes()})

	require.Equal(t, uint64(0), sink.writtenCount.Load(), "Lua 丢弃不应计为 written")
	require.Equal(t, uint64(1), sink.droppedCount.Load())
}

// P2：worker 写入前的 Redis 内存护栏；越线时丢弃不写。
func TestReqLogSinkProcessMemoryGuardDrops(t *testing.T) {
	store := &reqLogMemoryStore{
		resolveUserID: 7,
		memStats:      &ReqLogRedisMemoryStats{UsedMemory: 95, MaxMemory: 100}, // 95% > 80%
	}
	store.enabled.Store(&reqlog.CaptureState{
		UserID:    7,
		SessionID: "rl_sess",
		ExpiresAt: time.Now().Add(time.Minute),
	})
	sink := NewReqLogSink(store, newReqLogSinkTestCfg())

	entry := &reqlog.ReqLogEntry{UserID: 7, SessionID: "rl_sess", Timestamp: time.Now()}
	sink.process(reqLogQueuedEntry{entry: entry, bytes: entry.EstimateBytes()})

	require.Equal(t, int64(0), store.writes.Load(), "内存越线时不应写入")
	require.Equal(t, uint64(1), sink.droppedCount.Load())
}

// 对照：内存水位正常时正常写入。
func TestReqLogSinkProcessMemoryGuardAllowsBelowThreshold(t *testing.T) {
	store := &reqLogMemoryStore{
		resolveUserID: 7,
		memStats:      &ReqLogRedisMemoryStats{UsedMemory: 10, MaxMemory: 100}, // 10% < 80%
	}
	store.enabled.Store(&reqlog.CaptureState{
		UserID:    7,
		SessionID: "rl_sess",
		ExpiresAt: time.Now().Add(time.Minute),
	})
	sink := NewReqLogSink(store, newReqLogSinkTestCfg())

	entry := &reqlog.ReqLogEntry{UserID: 7, SessionID: "rl_sess", Timestamp: time.Now()}
	sink.process(reqLogQueuedEntry{entry: entry, bytes: entry.EstimateBytes()})

	require.Equal(t, int64(1), store.writes.Load())
	require.Equal(t, uint64(1), sink.writtenCount.Load())
}

func TestReqLogServiceCreateDownloadTokenResolvesSession(t *testing.T) {
	cfg := &config.Config{Ops: config.OpsConfig{RequestLog: config.OpsRequestLogConfig{Enabled: true}}}

	t.Run("unknown session returns not found and issues no token", func(t *testing.T) {
		store := &reqLogMemoryStore{resolveUserID: 0} // ResolveSessionUser -> ErrReqLogNotFound
		svc := NewReqLogService(cfg, store, nil)
		_, _, err := svc.CreateDownloadToken(context.Background(), 9, "rl_missing")
		require.ErrorIs(t, err, ErrReqLogNotFound)
		require.Equal(t, int64(0), store.createToken.Load(), "不存在的 session 不应签发 token")
	})

	t.Run("existing session issues token", func(t *testing.T) {
		store := &reqLogMemoryStore{resolveUserID: 42}
		svc := NewReqLogService(cfg, store, nil)
		tok, _, err := svc.CreateDownloadToken(context.Background(), 9, "rl_ok")
		require.NoError(t, err)
		require.Equal(t, "tok", tok)
		require.Equal(t, int64(1), store.createToken.Load())
	})
}
