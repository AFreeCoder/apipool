package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// P5：DropItem 不得复活一个已不存在的 session hash（避免产生无 TTL 的 key）。
func TestReqLogStoreDropItemDoesNotResurrectMissingHash(t *testing.T) {
	_, rdb, store := newReqLogMiniredis(t)
	ctx := context.Background()

	state := newReqLogStoreTestState("rl_drop_missing")
	require.NoError(t, store.CreateSession(ctx, state, 10*time.Minute, time.Hour, false))

	key := sessionKey(state.UserID, state.SessionID)
	// 模拟 session hash 已过期/被逐出。
	require.NoError(t, rdb.Del(ctx, key).Err())
	require.Equal(t, int64(0), rdb.Exists(ctx, key).Val())

	// DropItem 不应重建该 key。
	require.NoError(t, store.DropItem(ctx, state))
	require.Equal(t, int64(0), rdb.Exists(ctx, key).Val(), "DropItem 不应复活已消失的 session hash")
}

// P5 对照：session hash 存在时 DropItem 正常累加 dropped_count 且不破坏 TTL。
func TestReqLogStoreDropItemIncrementsWhenHashExists(t *testing.T) {
	_, rdb, store := newReqLogMiniredis(t)
	ctx := context.Background()

	state := newReqLogStoreTestState("rl_drop_exists")
	require.NoError(t, store.CreateSession(ctx, state, 10*time.Minute, time.Hour, false))
	key := sessionKey(state.UserID, state.SessionID)

	require.NoError(t, store.DropItem(ctx, state))
	require.NoError(t, store.DropItem(ctx, state))

	require.Equal(t, "2", rdb.HGet(ctx, key, "dropped_count").Val())
	require.Greater(t, rdb.PTTL(ctx, key).Val(), time.Duration(0), "session hash 仍应保留绝对 TTL")
}

// P4：CreateSession 必须在 Lua 内原子设置 sessions ZSet 的绝对 TTL。
func TestReqLogStoreEnableSetsSessionsZSetTTL(t *testing.T) {
	_, rdb, store := newReqLogMiniredis(t)
	ctx := context.Background()

	state := newReqLogStoreTestState("rl_sessions_ttl")
	require.NoError(t, store.CreateSession(ctx, state, 10*time.Minute, time.Hour, false))

	ttl := rdb.PTTL(ctx, sessionsKey(state.UserID)).Val()
	require.Greater(t, ttl, time.Duration(0), "sessions ZSet 应有绝对 TTL，避免残留无 TTL key")
}

// P1（store 兜底）：entry 归属会话与传入 state 不一致时，WriteItem 必须拒绝写入。
func TestReqLogStoreWriteItemRejectsSessionMismatch(t *testing.T) {
	_, _, store := newReqLogMiniredis(t)
	ctx := context.Background()

	state := newReqLogStoreTestState("rl_session_match")
	require.NoError(t, store.CreateSession(ctx, state, 10*time.Minute, time.Hour, false))

	entry := newReqLogStoreTestEntry(state, "/x", "payload")
	entry.SessionID = "rl_some_other_session" // 旧会话
	_, err := store.WriteItem(ctx, entry, state, time.Hour)
	require.Error(t, err, "归属会话不一致的 entry 不应被写入")
}
