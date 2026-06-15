package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/reqlog"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestReqLogStoreWriteItemDropOldestUsesRecordedSize(t *testing.T) {
	// 覆盖 Redis Lua 的 drop_oldest 对称扣账路径。miniredis 无法覆盖 INFO memory
	// 的真实 Redis 输出格式，ReqLogStore.MemoryStats 需要在集成测试环境补充验证。
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
	store := NewReqLogStoreForClient(rdb)

	state := &reqlog.CaptureState{
		UserID:             7,
		SessionID:          "rl_store",
		StartedAt:          time.Now().Add(-time.Minute),
		ExpiresAt:          time.Now().Add(time.Minute),
		MaxBytes:           520,
		MaxItems:           2,
		OverflowStrategy:   reqlog.OverflowDropOldest,
		SingleRequestCap:   1024,
		SingleResponseCap:  1024,
		RetentionAfterStop: time.Hour,
	}
	require.NoError(t, store.CreateSession(context.Background(), state, 10*time.Minute, time.Hour, false))

	entry := func(path string, body string) *reqlog.ReqLogEntry {
		return &reqlog.ReqLogEntry{
			UserID:      state.UserID,
			SessionID:   state.SessionID,
			Timestamp:   time.Now(),
			Method:      "POST",
			Path:        path,
			StatusCode:  200,
			ReqBody:     []byte(body),
			ReqBodyKind: reqlog.BodyKindText,
		}
	}
	_, err = store.WriteItem(context.Background(), entry("/one", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), state, time.Hour)
	require.NoError(t, err)
	_, err = store.WriteItem(context.Background(), entry("/two", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"), state, time.Hour)
	require.NoError(t, err)

	items, total, err := store.ListItems(context.Background(), state.SessionID, 1, 10)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	require.Equal(t, "/two", items[0].Path)

	stats, err := store.GetStats(context.Background(), state.UserID, state.SessionID)
	require.NoError(t, err)
	require.Equal(t, int64(1), stats.ItemCount)
	require.Greater(t, stats.BytesUsed, int64(0))

	raw := rdb.Get(context.Background(), "ops:reqlog:item:7:rl_store:2").Val()
	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.Equal(t, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", decoded["req_body"])
}

func TestReqLogStoreWriteItemSkipsReconcileWhenIndexCountMatches(t *testing.T) {
	_, rdb, store := newReqLogMiniredis(t)
	ctx := context.Background()

	state := newReqLogStoreTestState("rl_reconcile_skip")
	state.MaxBytes = 4096
	state.MaxItems = 10
	require.NoError(t, store.CreateSession(ctx, state, 10*time.Minute, time.Hour, false))

	seq, err := store.WriteItem(ctx, newReqLogStoreTestEntry(state, "/one", "first"), state, time.Hour)
	require.NoError(t, err)
	require.Equal(t, int64(1), seq)

	// 模拟历史裸 seq 成员，但保持 LLEN(idx) == item_count。若写入路径误跑
	// full reconcile，这个成员会被重写为 "1:size" 并增加 calibration_count。
	require.NoError(t, rdb.LSet(ctx, idxKey(state.UserID, state.SessionID), 0, "1").Err())

	seq, err = store.WriteItem(ctx, newReqLogStoreTestEntry(state, "/two", "second"), state, time.Hour)
	require.NoError(t, err)
	require.Equal(t, int64(2), seq)

	members, err := rdb.LRange(ctx, idxKey(state.UserID, state.SessionID), 0, -1).Result()
	require.NoError(t, err)
	require.Len(t, members, 2)
	require.Equal(t, "1", members[0])
	require.Equal(t, int64(2), parseSeqMember(members[1]))

	calibrations, err := rdb.HGet(ctx, sessionKey(state.UserID, state.SessionID), "calibration_count").Result()
	require.NoError(t, err)
	require.Equal(t, "0", calibrations)

	var liveBytes int64
	for _, member := range members {
		seq := parseSeqMember(member)
		n, err := rdb.StrLen(ctx, itemKey(state.UserID, state.SessionID, seq)).Result()
		require.NoError(t, err)
		liveBytes += n
	}

	stats, err := store.GetStats(ctx, state.UserID, state.SessionID)
	require.NoError(t, err)
	require.Equal(t, int64(len(members)), stats.ItemCount)
	require.Equal(t, liveBytes, stats.BytesUsed)
	require.LessOrEqual(t, stats.BytesUsed, state.MaxBytes)
}

func TestReqLogStoreWriteItemReconcilesMissingItemsBeforeDropOldestBudget(t *testing.T) {
	_, rdb, store := newReqLogMiniredis(t)
	ctx := context.Background()

	state := newReqLogStoreTestState("rl_reconcile")
	next, expectedBytes := prepareReqLogMissingItemNearByteBudget(t, ctx, rdb, store, state)

	seq, err := store.WriteItem(ctx, next, state, time.Hour)
	require.NoError(t, err)
	require.Equal(t, int64(4), seq)

	requireReqLogIndexSeqs(t, ctx, rdb, state, []int64{1, 3, 4})

	calibrations, err := rdb.HGet(ctx, sessionKey(state.UserID, state.SessionID), "calibration_count").Int64()
	require.NoError(t, err)
	require.GreaterOrEqual(t, calibrations, int64(1))

	stats, err := store.GetStats(ctx, state.UserID, state.SessionID)
	require.NoError(t, err)
	require.Equal(t, int64(3), stats.ItemCount)
	require.Equal(t, expectedBytes, stats.BytesUsed)
	require.Equal(t, int64(0), stats.DroppedCount)
	require.False(t, stats.Truncated)
	require.LessOrEqual(t, stats.BytesUsed, state.MaxBytes)
}

func TestReqLogStoreWriteItemStopReconcilesMissingItemsBeforeTruncate(t *testing.T) {
	_, rdb, store := newReqLogMiniredis(t)
	ctx := context.Background()

	state := newReqLogStoreTestState("rl_reconcile_stop")
	state.OverflowStrategy = reqlog.OverflowStop
	next, expectedBytes := prepareReqLogMissingItemNearByteBudget(t, ctx, rdb, store, state)

	seq, err := store.WriteItem(ctx, next, state, time.Hour)
	require.NoError(t, err)
	require.Equal(t, int64(4), seq)

	requireReqLogIndexSeqs(t, ctx, rdb, state, []int64{1, 3, 4})

	stats, err := store.GetStats(ctx, state.UserID, state.SessionID)
	require.NoError(t, err)
	require.Equal(t, int64(3), stats.ItemCount)
	require.Equal(t, expectedBytes, stats.BytesUsed)
	require.Equal(t, int64(0), stats.DroppedCount)
	require.False(t, stats.Truncated)
	require.LessOrEqual(t, stats.BytesUsed, state.MaxBytes)
}

func TestReqLogStoreWriteItemStopMarksTruncatedAndDrops(t *testing.T) {
	_, _, store := newReqLogMiniredis(t)
	state := newReqLogStoreTestState("rl_stop")
	state.MaxBytes = 4096
	state.MaxItems = 1
	state.OverflowStrategy = reqlog.OverflowStop
	require.NoError(t, store.CreateSession(context.Background(), state, 10*time.Minute, time.Hour, false))

	seq, err := store.WriteItem(context.Background(), newReqLogStoreTestEntry(state, "/one", "first"), state, time.Hour)
	require.NoError(t, err)
	require.Equal(t, int64(1), seq)
	seq, err = store.WriteItem(context.Background(), newReqLogStoreTestEntry(state, "/two", "second"), state, time.Hour)
	require.NoError(t, err)
	require.Zero(t, seq)

	items, total, err := store.ListItems(context.Background(), state.SessionID, 1, 10)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	require.Equal(t, "/one", items[0].Path)

	stats, err := store.GetStats(context.Background(), state.UserID, state.SessionID)
	require.NoError(t, err)
	require.True(t, stats.Truncated)
	require.Equal(t, int64(1), stats.DroppedCount)
	require.Equal(t, int64(1), stats.ItemCount)
}

func TestReqLogStoreWriteItemMaxItemsEvictsOldestIndependently(t *testing.T) {
	_, _, store := newReqLogMiniredis(t)
	state := newReqLogStoreTestState("rl_max_items")
	state.MaxBytes = 1 << 20
	state.MaxItems = 2
	require.NoError(t, store.CreateSession(context.Background(), state, 10*time.Minute, time.Hour, false))

	for _, path := range []string{"/one", "/two", "/three"} {
		_, err := store.WriteItem(context.Background(), newReqLogStoreTestEntry(state, path, "body"), state, time.Hour)
		require.NoError(t, err)
	}

	items, total, err := store.ListItems(context.Background(), state.SessionID, 1, 10)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, items, 2)
	require.Equal(t, "/two", items[0].Path)
	require.Equal(t, "/three", items[1].Path)
}

func TestReqLogStoreEnableSessionConcurrentLimitAndForceClosesOldSession(t *testing.T) {
	_, _, store := newReqLogMiniredis(t)
	ctx := context.Background()
	now := time.Now().UTC()

	first := newReqLogStoreTestState("rl_limit_one")
	first.UserID = 101
	first.StartedAt = now
	first.ExpiresAt = now.Add(time.Minute)
	created, existed, err := store.EnableSession(ctx, first, time.Minute, time.Hour, false, 1)
	require.NoError(t, err)
	require.False(t, existed)
	require.Equal(t, first.SessionID, created.SessionID)

	second := newReqLogStoreTestState("rl_limit_two")
	second.UserID = 202
	second.StartedAt = now
	second.ExpiresAt = now.Add(time.Minute)
	_, _, err = store.EnableSession(ctx, second, time.Minute, time.Hour, false, 1)
	require.ErrorIs(t, err, service.ErrReqLogConcurrentLimit)

	reopened := newReqLogStoreTestState("rl_limit_reopen")
	reopened.UserID = first.UserID
	reopened.StartedAt = now
	reopened.ExpiresAt = now.Add(time.Minute)
	created, existed, err = store.EnableSession(ctx, reopened, time.Minute, time.Hour, true, 1)
	require.NoError(t, err)
	require.False(t, existed)
	require.Equal(t, reopened.SessionID, created.SessionID)

	oldStats, err := store.GetStats(ctx, first.UserID, first.SessionID)
	require.NoError(t, err)
	require.Equal(t, "closed", oldStats.Status)
	count, err := store.CountEnabled(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestReqLogStoreEnableSessionConcurrentRaceDoesNotExceedLimit(t *testing.T) {
	_, _, store := newReqLogMiniredis(t)
	ctx := context.Background()
	now := time.Now().UTC()
	const attempts = 8

	var wg sync.WaitGroup
	errs := make(chan error, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			state := newReqLogStoreTestState("rl_race_" + strconv.Itoa(i))
			state.UserID = int64(1000 + i)
			state.StartedAt = now
			state.ExpiresAt = now.Add(time.Minute)
			_, _, err := store.EnableSession(ctx, state, time.Minute, time.Hour, false, 1)
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)

	successes := 0
	limitErrors := 0
	for err := range errs {
		if err == nil {
			successes++
			continue
		}
		if errors.Is(err, service.ErrReqLogConcurrentLimit) {
			limitErrors++
			continue
		}
		require.NoError(t, err)
	}
	require.Equal(t, 1, successes)
	require.Equal(t, attempts-1, limitErrors)
	count, err := store.CountEnabled(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestReqLogStoreListActiveReflectsEnableDisableAndExpiredScores(t *testing.T) {
	_, _, store := newReqLogMiniredis(t)
	ctx := context.Background()
	now := time.Now().UTC()

	first := newReqLogStoreTestState("rl_active_one")
	first.UserID = 101
	first.StartedAt = now
	first.ExpiresAt = now.Add(time.Minute)
	require.NoError(t, store.CreateSession(ctx, first, time.Minute, time.Hour, false))

	second := newReqLogStoreTestState("rl_active_two")
	second.UserID = 202
	second.StartedAt = now
	second.ExpiresAt = now.Add(2 * time.Minute)
	require.NoError(t, store.CreateSession(ctx, second, 2*time.Minute, time.Hour, false))

	items, err := store.ListActive(ctx)
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, int64(101), items[0].UserID)
	require.Equal(t, "rl_active_one", items[0].SessionID)
	require.Equal(t, int64(202), items[1].UserID)
	require.Equal(t, "rl_active_two", items[1].SessionID)

	require.NoError(t, store.DisableSession(ctx, first.UserID))
	items, err = store.ListActive(ctx)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(202), items[0].UserID)

	expired := newReqLogStoreTestState("rl_active_expired")
	expired.UserID = 303
	expired.StartedAt = now.Add(-2 * time.Minute)
	expired.ExpiresAt = now.Add(-time.Minute)
	require.NoError(t, store.CreateSession(ctx, expired, time.Minute, time.Hour, false))

	items, err = store.ListActive(ctx)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(202), items[0].UserID)
}

func TestReqLogStoreDownloadTokenIsOneTimeAndSessionBound(t *testing.T) {
	_, _, store := newReqLogMiniredis(t)
	token, _, err := store.CreateDownloadToken(context.Background(), "rl_download", 88, time.Minute)
	require.NoError(t, err)

	consumed, err := store.ConsumeDownloadToken(context.Background(), token)
	require.NoError(t, err)
	require.Equal(t, "rl_download", consumed.SessionID)
	require.Equal(t, int64(88), consumed.AdminID)

	consumed, err = store.ConsumeDownloadToken(context.Background(), token)
	require.ErrorIs(t, err, service.ErrReqLogUnauthorized)
	require.Nil(t, consumed)
}

func TestReqLogStoreListSessionsSkipsEvictedSessionHash(t *testing.T) {
	_, rdb, store := newReqLogMiniredis(t)
	state := newReqLogStoreTestState("rl_evicted_hash")
	require.NoError(t, store.CreateSession(context.Background(), state, 10*time.Minute, time.Hour, false))
	require.NoError(t, rdb.Del(context.Background(), "ops:reqlog:sess:7:rl_evicted_hash").Err())

	items, err := store.ListSessions(context.Background(), state.UserID, 10)
	require.NoError(t, err)
	require.Empty(t, items)
}

func newReqLogMiniredis(t *testing.T) (*miniredis.Miniredis, *redis.Client, *ReqLogStore) {
	t.Helper()
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
	return mr, rdb, NewReqLogStoreForClient(rdb)
}

func newReqLogStoreTestState(sessionID string) *reqlog.CaptureState {
	now := time.Now().UTC()
	return &reqlog.CaptureState{
		UserID:             7,
		SessionID:          sessionID,
		StartedAt:          now.Add(-time.Minute),
		ExpiresAt:          now.Add(time.Minute),
		MaxBytes:           4096,
		MaxItems:           10,
		OverflowStrategy:   reqlog.OverflowDropOldest,
		SingleRequestCap:   1024,
		SingleResponseCap:  1024,
		RetentionAfterStop: time.Hour,
		Reason:             "test",
	}
}

func newReqLogStoreTestEntry(state *reqlog.CaptureState, path string, body string) *reqlog.ReqLogEntry {
	return &reqlog.ReqLogEntry{
		UserID:      state.UserID,
		SessionID:   state.SessionID,
		Timestamp:   time.Now().UTC(),
		Method:      "POST",
		Path:        path,
		StatusCode:  200,
		ReqBody:     []byte(body),
		ReqBodyKind: reqlog.BodyKindText,
	}
}

func prepareReqLogMissingItemNearByteBudget(t *testing.T, ctx context.Context, rdb *redis.Client, store *ReqLogStore, state *reqlog.CaptureState) (*reqlog.ReqLogEntry, int64) {
	t.Helper()
	state.MaxBytes = 1 << 20
	state.MaxItems = 10
	require.NoError(t, store.CreateSession(ctx, state, 10*time.Minute, time.Hour, false))

	for i, path := range []string{"/one", "/two", "/three"} {
		seq, err := store.WriteItem(ctx, newReqLogStoreTestEntry(state, path, strings.Repeat(string(rune('a'+i)), 64)), state, time.Hour)
		require.NoError(t, err)
		require.Equal(t, int64(i+1), seq)
	}

	staleBytes, err := rdb.HGet(ctx, sessionKey(state.UserID, state.SessionID), "bytes_used").Int64()
	require.NoError(t, err)
	staleCount, err := rdb.HGet(ctx, sessionKey(state.UserID, state.SessionID), "item_count").Int64()
	require.NoError(t, err)
	llen, err := rdb.LLen(ctx, idxKey(state.UserID, state.SessionID)).Result()
	require.NoError(t, err)
	require.Equal(t, staleCount, llen)

	require.NoError(t, rdb.Del(ctx, itemKey(state.UserID, state.SessionID, 2)).Err())
	next := newReqLogStoreTestEntry(state, "/four", strings.Repeat("d", 64))
	nextSize := reqLogEntryJSONSize(t, next)
	liveBytes := reqLogLiveIndexedBytes(t, ctx, rdb, state)
	state.MaxBytes = liveBytes + nextSize

	require.Greater(t, staleBytes+nextSize, state.MaxBytes)
	require.LessOrEqual(t, liveBytes+nextSize, state.MaxBytes)
	afterDeleteCount, err := rdb.HGet(ctx, sessionKey(state.UserID, state.SessionID), "item_count").Int64()
	require.NoError(t, err)
	afterDeleteLLen, err := rdb.LLen(ctx, idxKey(state.UserID, state.SessionID)).Result()
	require.NoError(t, err)
	require.Equal(t, afterDeleteCount, afterDeleteLLen)

	return next, liveBytes + nextSize
}

func reqLogEntryJSONSize(t *testing.T, entry *reqlog.ReqLogEntry) int64 {
	t.Helper()
	raw, err := json.Marshal(entry)
	require.NoError(t, err)
	return int64(len(raw))
}

func reqLogLiveIndexedBytes(t *testing.T, ctx context.Context, rdb *redis.Client, state *reqlog.CaptureState) int64 {
	t.Helper()
	members, err := rdb.LRange(ctx, idxKey(state.UserID, state.SessionID), 0, -1).Result()
	require.NoError(t, err)
	var liveBytes int64
	for _, member := range members {
		seq := parseSeqMember(member)
		n, err := rdb.StrLen(ctx, itemKey(state.UserID, state.SessionID, seq)).Result()
		require.NoError(t, err)
		liveBytes += n
	}
	return liveBytes
}

func requireReqLogIndexSeqs(t *testing.T, ctx context.Context, rdb *redis.Client, state *reqlog.CaptureState, want []int64) {
	t.Helper()
	members, err := rdb.LRange(ctx, idxKey(state.UserID, state.SessionID), 0, -1).Result()
	require.NoError(t, err)
	require.Len(t, members, len(want))
	got := make([]int64, 0, len(members))
	for _, member := range members {
		got = append(got, parseSeqMember(member))
	}
	require.Equal(t, want, got)
}
