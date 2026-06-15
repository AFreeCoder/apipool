//go:build integration

package repository

import (
	"context"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestReqLogStoreIntegrationLuaConcurrentDoubleConstraints(t *testing.T) {
	rdb, store := newReqLogIntegrationStore(t)
	ctx := context.Background()
	state := newReqLogStoreTestState("rl_it_concurrent")
	state.UserID = 7701
	state.MaxBytes = 4096
	state.MaxItems = 5
	require.NoError(t, store.CreateSession(ctx, state, time.Minute, time.Hour, false))

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := store.WriteItem(ctx, newReqLogStoreTestEntry(state, "/it/"+strconv.Itoa(i), strings.Repeat("x", 128)), state, time.Hour)
			require.NoError(t, err)
		}(i)
	}
	wg.Wait()

	stats, err := store.GetStats(ctx, state.UserID, state.SessionID)
	require.NoError(t, err)
	require.LessOrEqual(t, stats.ItemCount, int64(state.MaxItems))
	require.LessOrEqual(t, stats.BytesUsed, state.MaxBytes)
	llen, err := rdb.LLen(ctx, "ops:reqlog:idx:7701:rl_it_concurrent").Result()
	require.NoError(t, err)
	require.Equal(t, llen, stats.ItemCount)
}

func TestReqLogStoreIntegrationMemoryStats(t *testing.T) {
	_, store := newReqLogIntegrationStore(t)
	stats, err := store.MemoryStats(context.Background())
	require.NoError(t, err)
	require.NotNil(t, stats)
	require.GreaterOrEqual(t, stats.UsedMemory, int64(0))
	require.GreaterOrEqual(t, stats.MaxMemory, int64(0))
}

func TestReqLogStoreIntegrationAllKeysLRUSelfHeal(t *testing.T) {
	rdb, store := newReqLogIntegrationStore(t)
	if strings.TrimSpace(os.Getenv("REQLOG_REDIS_LRU")) != "1" {
		t.Skip("设置 REQLOG_REDIS_LRU=1 后才执行会修改 Redis maxmemory-policy 的 LRU 集成测试")
	}
	ctx := context.Background()
	previousPolicy, err := rdb.ConfigGet(ctx, "maxmemory-policy").Result()
	require.NoError(t, err)
	if policy := strings.TrimSpace(previousPolicy["maxmemory-policy"]); policy != "" {
		t.Cleanup(func() { _ = rdb.ConfigSet(context.Background(), "maxmemory-policy", policy).Err() })
	}
	require.NoError(t, rdb.ConfigSet(ctx, "maxmemory-policy", "allkeys-lru").Err())

	state := newReqLogStoreTestState("rl_it_lru")
	state.UserID = 7702
	next, expectedBytes := prepareReqLogMissingItemNearByteBudget(t, ctx, rdb, store, state)
	seq, err := store.WriteItem(ctx, next, state, time.Hour)
	require.NoError(t, err)
	require.Equal(t, int64(4), seq)
	requireReqLogIndexSeqs(t, ctx, rdb, state, []int64{1, 3, 4})

	stats, err := store.GetStats(ctx, state.UserID, state.SessionID)
	require.NoError(t, err)
	require.Equal(t, int64(3), stats.ItemCount)
	require.Equal(t, expectedBytes, stats.BytesUsed)
	require.LessOrEqual(t, stats.BytesUsed, state.MaxBytes)
}

func newReqLogIntegrationStore(t *testing.T) (*redis.Client, *ReqLogStore) {
	t.Helper()
	addr := strings.TrimSpace(os.Getenv("REQLOG_REDIS_ADDR"))
	if addr == "" {
		t.Skip("未设置 REQLOG_REDIS_ADDR，跳过真实 Redis 集成测试")
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr, DB: 15})
	t.Cleanup(func() { _ = rdb.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, rdb.Ping(ctx).Err())
	cleanupReqLogKeys(t, rdb)
	t.Cleanup(func() { cleanupReqLogKeys(t, rdb) })
	return rdb, NewReqLogStoreForClient(rdb)
}

func cleanupReqLogKeys(t *testing.T, rdb *redis.Client) {
	t.Helper()
	ctx := context.Background()
	var cursor uint64
	for {
		keys, next, err := rdb.Scan(ctx, cursor, reqLogPrefix+"*", 100).Result()
		require.NoError(t, err)
		if len(keys) > 0 {
			require.NoError(t, rdb.Del(ctx, keys...).Err())
		}
		if next == 0 {
			return
		}
		cursor = next
	}
}
