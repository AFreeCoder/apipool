package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/reqlog"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const reqLogPrefix = "ops:reqlog:"

type ReqLogStore struct {
	rdb    *redis.Client
	client *ReqLogRedisClient
}

func NewReqLogStore(client *ReqLogRedisClient) service.ReqLogStore {
	if client == nil {
		return &ReqLogStore{}
	}
	return &ReqLogStore{rdb: client.Client, client: client}
}

func NewReqLogStoreForClient(rdb *redis.Client) *ReqLogStore {
	return &ReqLogStore{rdb: rdb}
}

func (s *ReqLogStore) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}

func (s *ReqLogStore) GetEnabled(ctx context.Context, userID int64) (*reqlog.CaptureState, error) {
	if s == nil || s.rdb == nil {
		return nil, nil
	}
	raw, err := s.rdb.Get(ctx, enabledKey(userID)).Bytes()
	if errors.Is(err, redis.Nil) {
		_ = s.cleanupEnabledUser(ctx, userID)
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var state reqlog.CaptureState
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, err
	}
	state.NormalizeTimes()
	return &state, nil
}

func (s *ReqLogStore) cleanupEnabledUser(ctx context.Context, userID int64) error {
	if s == nil || s.rdb == nil || userID <= 0 {
		return nil
	}
	return reqLogCleanupEnabledUserScript.Run(ctx, s.rdb, []string{enabledCountKey(), enabledActiveKey()}, userID).Err()
}

func (s *ReqLogStore) EnableSession(ctx context.Context, state *reqlog.CaptureState, window, retention time.Duration, force bool, maxConcurrent int) (*reqlog.CaptureState, bool, error) {
	if err := s.createSession(ctx, state, window, retention, force, maxConcurrent); err != nil {
		if errors.Is(err, errReqLogExists) {
			existing, getErr := s.GetEnabled(ctx, state.UserID)
			return existing, true, getErr
		}
		return nil, false, err
	}
	return state.Clone(), false, nil
}

var errReqLogExists = errors.New("request log session already exists")

func (s *ReqLogStore) CreateSession(ctx context.Context, state *reqlog.CaptureState, window, retention time.Duration, force bool) error {
	return s.createSession(ctx, state, window, retention, force, 0)
}

func (s *ReqLogStore) createSession(ctx context.Context, state *reqlog.CaptureState, window, retention time.Duration, force bool, maxConcurrent int) error {
	if s == nil || s.rdb == nil || state == nil {
		return fmt.Errorf("request log store unavailable")
	}
	state.NormalizeTimes()
	if state.UserID <= 0 || strings.TrimSpace(state.SessionID) == "" {
		return fmt.Errorf("invalid request log session")
	}
	encoded, err := json.Marshal(state)
	if err != nil {
		return err
	}
	absTTL := time.Until(state.ExpiresAt.Add(retention))
	if absTTL <= 0 {
		absTTL = time.Second
	}
	cutoff := state.ExpiresAt.Add(retention)
	forceArg := "0"
	if force {
		forceArg = "1"
	}
	res, err := reqLogEnableSessionScript.Run(ctx, s.rdb, []string{
		enabledKey(state.UserID),
		sessByIDKey(state.SessionID),
		sessionKey(state.UserID, state.SessionID),
		seqKey(state.UserID, state.SessionID),
		idxKey(state.UserID, state.SessionID),
		sessionsKey(state.UserID),
		currentSessKey(state.UserID),
		sessionPrefix(state.UserID),
		enabledCountKey(),
		enabledActiveKey(),
	}, string(encoded), int64(window/time.Millisecond), int64(absTTL/time.Millisecond), cutoff.Unix(), state.UserID, state.SessionID, time.Now().Unix(), state.ExpiresAt.Unix(), forceArg, maxConcurrent, state.StartedAt.Unix(), state.Reason).Result()
	if err != nil {
		return err
	}
	vals, ok := res.([]any)
	if !ok || len(vals) == 0 {
		return fmt.Errorf("unexpected reqlog enable lua result")
	}
	code, _ := vals[0].(string)
	switch code {
	case "ok":
	case "exists":
		return errReqLogExists
	case "limit":
		return service.ErrReqLogConcurrentLimit
	default:
		return fmt.Errorf("unexpected reqlog enable lua code: %v", vals[0])
	}
	return s.refreshSessionsTTL(ctx, state.UserID)
}

func (s *ReqLogStore) DisableSession(ctx context.Context, userID int64) error {
	if s == nil || s.rdb == nil {
		return nil
	}
	_, err := reqLogDisableSessionScript.Run(ctx, s.rdb, []string{
		enabledKey(userID),
		currentSessKey(userID),
		sessionPrefix(userID),
		enabledCountKey(),
		enabledActiveKey(),
	}, userID, time.Now().Unix()).Result()
	return err
}

func (s *ReqLogStore) CountEnabled(ctx context.Context) (int, error) {
	if s == nil || s.rdb == nil {
		return 0, nil
	}
	now := time.Now().Unix()
	res, err := reqLogSyncEnabledCountScript.Run(ctx, s.rdb, []string{enabledCountKey(), enabledActiveKey()}, now).Result()
	if err != nil {
		return 0, err
	}
	count, _ := toInt64(res)
	return int(count), nil
}

func (s *ReqLogStore) ListActive(ctx context.Context) ([]*reqlog.CaptureState, error) {
	if s == nil || s.rdb == nil {
		return nil, nil
	}
	now := time.Now().Unix()
	if err := s.rdb.ZRemRangeByScore(ctx, enabledActiveKey(), "-inf", strconv.FormatInt(now, 10)).Err(); err != nil {
		return nil, err
	}
	zs, err := s.rdb.ZRangeWithScores(ctx, enabledActiveKey(), 0, -1).Result()
	if err != nil {
		return nil, err
	}
	out := make([]*reqlog.CaptureState, 0, len(zs))
	for _, z := range zs {
		uid, err := strconv.ParseInt(fmt.Sprint(z.Member), 10, 64)
		if err != nil || uid <= 0 {
			continue
		}
		state, err := s.GetEnabled(ctx, uid)
		if err != nil {
			return nil, err
		}
		if state == nil {
			continue
		}
		state.NormalizeTimes()
		if state.ExpiresAt.IsZero() || state.ExpiresAt.Unix() <= now {
			_ = s.cleanupEnabledUser(ctx, uid)
			continue
		}
		out = append(out, state)
	}
	_ = s.rdb.Set(ctx, enabledCountKey(), len(out), 0).Err()
	return out, nil
}

func (s *ReqLogStore) WriteItem(ctx context.Context, entry *reqlog.ReqLogEntry, state *reqlog.CaptureState, retention time.Duration) (int64, error) {
	if s == nil || s.rdb == nil || entry == nil || state == nil {
		return 0, fmt.Errorf("request log store unavailable")
	}
	state.NormalizeTimes()
	entry.UserID = state.UserID
	entry.SessionID = state.SessionID
	raw, err := json.Marshal(entry)
	if err != nil {
		return 0, err
	}
	absTTL := time.Until(state.ExpiresAt.Add(retention))
	if absTTL <= 0 {
		return 0, fmt.Errorf("request log session expired")
	}
	res, err := reqLogWriteScript.Run(ctx, s.rdb, []string{
		enabledKey(state.UserID),
		sessionKey(state.UserID, state.SessionID),
		seqKey(state.UserID, state.SessionID),
		idxKey(state.UserID, state.SessionID),
		itemPrefix(state.UserID, state.SessionID),
	}, string(raw), len(raw), int64(absTTL/time.Millisecond), state.MaxBytes, state.MaxItems, state.OverflowStrategy, entry.Timestamp.Unix(), state.ExpiresAt.Unix()).Result()
	if err != nil {
		return 0, err
	}
	vals, ok := res.([]any)
	if !ok || len(vals) < 2 {
		return 0, fmt.Errorf("unexpected reqlog lua result")
	}
	written, _ := toInt64(vals[0])
	if written == 0 {
		return 0, nil
	}
	seq, _ := toInt64(vals[1])
	entry.Seq = seq
	return seq, nil
}

func (s *ReqLogStore) DropItem(ctx context.Context, state *reqlog.CaptureState) error {
	if s == nil || s.rdb == nil || state == nil {
		return nil
	}
	return s.rdb.HIncrBy(ctx, sessionKey(state.UserID, state.SessionID), "dropped_count", 1).Err()
}

func (s *ReqLogStore) GetStats(ctx context.Context, userID int64, sessionID string) (*service.ReqLogSessionStats, error) {
	if s == nil || s.rdb == nil {
		return nil, nil
	}
	m, err := s.rdb.HGetAll(ctx, sessionKey(userID, sessionID)).Result()
	if err != nil {
		return nil, err
	}
	if len(m) == 0 {
		return nil, service.ErrReqLogNotFound
	}
	return statsFromHash(m), nil
}

func (s *ReqLogStore) ResolveSessionUser(ctx context.Context, sessionID string) (int64, error) {
	if s == nil || s.rdb == nil {
		return 0, service.ErrReqLogNotFound
	}
	v, err := s.rdb.Get(ctx, sessByIDKey(sessionID)).Result()
	if errors.Is(err, redis.Nil) {
		return 0, service.ErrReqLogNotFound
	}
	if err != nil {
		return 0, err
	}
	uid, err := strconv.ParseInt(v, 10, 64)
	if err != nil || uid <= 0 {
		return 0, service.ErrReqLogNotFound
	}
	return uid, nil
}

func (s *ReqLogStore) ListSessions(ctx context.Context, userID int64, limit int) ([]service.ReqLogSession, error) {
	if s == nil || s.rdb == nil {
		return nil, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	now := time.Now().Unix()
	key := sessionsKey(userID)
	_ = s.rdb.ZRemRangeByScore(ctx, key, "-inf", strconv.FormatInt(now, 10)).Err()
	zs, err := s.rdb.ZRevRangeWithScores(ctx, key, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}
	out := make([]service.ReqLogSession, 0, len(zs))
	for _, z := range zs {
		sid, ok := z.Member.(string)
		if !ok || sid == "" {
			continue
		}
		m, _ := s.rdb.HGetAll(ctx, sessionKey(userID, sid)).Result()
		if len(m) == 0 {
			continue
		}
		stats := statsFromHash(m)
		item := service.ReqLogSession{
			UserID:       userID,
			SessionID:    sid,
			CutoffAt:     time.Unix(int64(z.Score), 0).UTC(),
			BytesUsed:    stats.BytesUsed,
			ItemCount:    stats.ItemCount,
			Truncated:    stats.Truncated,
			DroppedCount: stats.DroppedCount,
			Status:       stats.Status,
			StartedAt:    stats.StartedAt,
			ExpiresAt:    stats.ExpiresAt,
			Reason:       m["reason"],
		}
		out = append(out, item)
	}
	_ = s.refreshSessionsTTL(ctx, userID)
	return out, nil
}

func (s *ReqLogStore) ListItems(ctx context.Context, sessionID string, page, pageSize int) ([]*reqlog.ReqLogEntry, int64, error) {
	uid, err := s.ResolveSessionUser(ctx, sessionID)
	if err != nil {
		return nil, 0, err
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 500 {
		pageSize = 20
	}
	key := idxKey(uid, sessionID)
	total, err := s.rdb.LLen(ctx, key).Result()
	if err != nil {
		return nil, 0, err
	}
	start := int64((page - 1) * pageSize)
	stop := start + int64(pageSize) - 1
	members, err := s.rdb.LRange(ctx, key, start, stop).Result()
	if err != nil {
		return nil, 0, err
	}
	out := make([]*reqlog.ReqLogEntry, 0, len(members))
	for _, member := range members {
		seq := parseSeqMember(member)
		if seq <= 0 {
			continue
		}
		item, err := s.getItemByUID(ctx, uid, sessionID, seq)
		if err == nil && item != nil {
			out = append(out, item)
		}
	}
	return out, total, nil
}

func (s *ReqLogStore) GetItem(ctx context.Context, sessionID string, seq int64) (*reqlog.ReqLogEntry, error) {
	uid, err := s.ResolveSessionUser(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return s.getItemByUID(ctx, uid, sessionID, seq)
}

func (s *ReqLogStore) getItemByUID(ctx context.Context, uid int64, sessionID string, seq int64) (*reqlog.ReqLogEntry, error) {
	raw, err := s.rdb.Get(ctx, itemKey(uid, sessionID, seq)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, service.ErrReqLogNotFound
	}
	if err != nil {
		return nil, err
	}
	var entry reqlog.ReqLogEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		return nil, err
	}
	entry.Seq = seq
	return &entry, nil
}

func (s *ReqLogStore) CreateDownloadToken(ctx context.Context, sessionID string, adminID int64, ttl time.Duration) (string, time.Time, error) {
	if s == nil || s.rdb == nil {
		return "", time.Time{}, fmt.Errorf("request log store unavailable")
	}
	if ttl <= 0 {
		ttl = time.Minute
	}
	token := randomHex(24)
	expiresAt := time.Now().Add(ttl).UTC()
	payload, _ := json.Marshal(service.ReqLogDownloadToken{SessionID: sessionID, AdminID: adminID, ExpiresAt: expiresAt})
	if err := s.rdb.Set(ctx, downloadTokenKey(token), payload, ttl).Err(); err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

func (s *ReqLogStore) ConsumeDownloadToken(ctx context.Context, token string) (*service.ReqLogDownloadToken, error) {
	if s == nil || s.rdb == nil || strings.TrimSpace(token) == "" {
		return nil, service.ErrReqLogUnauthorized
	}
	res, err := reqLogConsumeTokenScript.Run(ctx, s.rdb, []string{downloadTokenKey(token)}).Result()
	if errors.Is(err, redis.Nil) {
		return nil, service.ErrReqLogUnauthorized
	}
	if err != nil {
		return nil, err
	}
	raw, ok := res.(string)
	if !ok || raw == "" {
		return nil, service.ErrReqLogUnauthorized
	}
	var out service.ReqLogDownloadToken
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *ReqLogStore) MemoryStats(ctx context.Context) (*service.ReqLogRedisMemoryStats, error) {
	if s == nil || s.rdb == nil {
		return nil, nil
	}
	info, err := s.rdb.Info(ctx, "memory").Result()
	if err != nil {
		return nil, err
	}
	stats := &service.ReqLogRedisMemoryStats{}
	for _, line := range strings.Split(info, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err != nil {
			continue
		}
		switch k {
		case "used_memory":
			stats.UsedMemory = n
		case "maxmemory":
			stats.MaxMemory = n
		}
	}
	return stats, nil
}

func (s *ReqLogStore) refreshSessionsTTL(ctx context.Context, userID int64) error {
	key := sessionsKey(userID)
	zs, err := s.rdb.ZRevRangeWithScores(ctx, key, 0, 0).Result()
	if err != nil || len(zs) == 0 {
		return err
	}
	cutoff := time.Unix(int64(zs[0].Score), 0)
	if time.Until(cutoff) <= 0 {
		_ = s.rdb.Del(ctx, key).Err()
		return nil
	}
	return s.rdb.ExpireAt(ctx, key, cutoff).Err()
}

func statsFromHash(m map[string]string) *service.ReqLogSessionStats {
	if len(m) == 0 {
		return &service.ReqLogSessionStats{}
	}
	return &service.ReqLogSessionStats{
		BytesUsed:    parseInt64(m["bytes_used"]),
		ItemCount:    parseInt64(m["item_count"]),
		Truncated:    parseInt64(m["truncated"]) == 1,
		DroppedCount: parseInt64(m["dropped_count"]),
		StartedAt:    time.Unix(parseInt64(m["started_at"]), 0).UTC(),
		ExpiresAt:    time.Unix(parseInt64(m["expires_at"]), 0).UTC(),
		Status:       m["status"],
	}
}

func parseSeqMember(member string) int64 {
	seqPart, _, _ := strings.Cut(member, ":")
	return parseInt64(seqPart)
}

func parseInt64(s string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}

func toInt64(v any) (int64, bool) {
	switch t := v.(type) {
	case int64:
		return t, true
	case int:
		return int64(t), true
	case string:
		n, err := strconv.ParseInt(t, 10, 64)
		return n, err == nil
	default:
		return 0, false
	}
}

func randomHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(buf)
}

func enabledKey(uid int64) string   { return reqLogPrefix + "enabled:" + strconv.FormatInt(uid, 10) }
func sessByIDKey(sid string) string { return reqLogPrefix + "sess_by_id:" + sid }
func sessionKey(uid int64, sid string) string {
	return reqLogPrefix + "sess:" + strconv.FormatInt(uid, 10) + ":" + sid
}
func sessionPrefix(uid int64) string {
	return reqLogPrefix + "sess:" + strconv.FormatInt(uid, 10) + ":"
}
func currentSessKey(uid int64) string {
	return reqLogPrefix + "current_sess:" + strconv.FormatInt(uid, 10)
}
func seqKey(uid int64, sid string) string {
	return reqLogPrefix + "seq:" + strconv.FormatInt(uid, 10) + ":" + sid
}
func idxKey(uid int64, sid string) string {
	return reqLogPrefix + "idx:" + strconv.FormatInt(uid, 10) + ":" + sid
}
func itemPrefix(uid int64, sid string) string {
	return reqLogPrefix + "item:" + strconv.FormatInt(uid, 10) + ":" + sid + ":"
}
func itemKey(uid int64, sid string, seq int64) string {
	return itemPrefix(uid, sid) + strconv.FormatInt(seq, 10)
}
func sessionsKey(uid int64) string         { return reqLogPrefix + "sessions:" + strconv.FormatInt(uid, 10) }
func downloadTokenKey(token string) string { return reqLogPrefix + "dltoken:" + token }
func enabledCountKey() string              { return reqLogPrefix + "enabled_count" }
func enabledActiveKey() string             { return reqLogPrefix + "enabled_active" }

var reqLogConsumeTokenScript = redis.NewScript(`
local v = redis.call("GET", KEYS[1])
if not v then
  return nil
end
redis.call("DEL", KEYS[1])
return v
`)

var reqLogEnableSessionScript = redis.NewScript(`
local enabledKey = KEYS[1]
local sessByIDKey = KEYS[2]
local sessKey = KEYS[3]
local seqKey = KEYS[4]
local idxKey = KEYS[5]
local sessionsKey = KEYS[6]
local currentSessKey = KEYS[7]
local sessionPrefix = KEYS[8]
local countKey = KEYS[9]
local activeKey = KEYS[10]

local stateJSON = ARGV[1]
local windowMs = tonumber(ARGV[2]) or 0
local absTTL = tonumber(ARGV[3]) or 0
local cutoffUnix = tonumber(ARGV[4]) or 0
local userID = tostring(ARGV[5])
local sessionID = tostring(ARGV[6])
local nowUnix = tonumber(ARGV[7]) or 0
local expiresUnix = tonumber(ARGV[8]) or 0
local force = ARGV[9] == "1"
local maxConcurrent = tonumber(ARGV[10]) or 0
local startedUnix = tonumber(ARGV[11]) or 0
local reason = ARGV[12] or ""

redis.call("ZREMRANGEBYSCORE", activeKey, "-inf", nowUnix)
local count = tonumber(redis.call("ZCARD", activeKey)) or 0
redis.call("SET", countKey, count)

local oldEnabled = redis.call("GET", enabledKey)
if oldEnabled and not force then
  return {"exists", count}
end

local activeScore = redis.call("ZSCORE", activeKey, userID)
local replacingExisting = oldEnabled and force
if force then
  local oldSid = redis.call("GET", currentSessKey)
  if oldSid and oldSid ~= "" and oldSid ~= sessionID then
    redis.call("HSET", sessionPrefix .. oldSid, "status", "closed")
  end
  redis.call("DEL", enabledKey)
end

if not activeScore then
  if maxConcurrent > 0 and count >= maxConcurrent and not replacingExisting then
    return {"limit", count}
  end
end

redis.call("SET", enabledKey, stateJSON, "PX", windowMs)
redis.call("ZADD", activeKey, expiresUnix, userID)
count = tonumber(redis.call("ZCARD", activeKey)) or count
redis.call("SET", countKey, count)

redis.call("SET", sessByIDKey, userID, "PX", absTTL)
redis.call("SET", currentSessKey, sessionID, "PX", absTTL)
redis.call("HSET", sessKey,
  "bytes_used", 0,
  "item_count", 0,
  "truncated", 0,
  "dropped_count", 0,
  "calibration_count", 0,
  "started_at", startedUnix,
  "expires_at", expiresUnix,
  "status", "recording",
  "reason", reason
)
redis.call("PEXPIRE", sessKey, absTTL)
redis.call("SET", seqKey, 0, "PX", absTTL)
redis.call("DEL", idxKey)
redis.call("ZADD", sessionsKey, cutoffUnix, sessionID)

return {"ok", count}
`)

var reqLogDisableSessionScript = redis.NewScript(`
local enabledKey = KEYS[1]
local currentSessKey = KEYS[2]
local sessionPrefix = KEYS[3]
local countKey = KEYS[4]
local activeKey = KEYS[5]
local userID = tostring(ARGV[1])
local nowUnix = tonumber(ARGV[2]) or 0

local oldSid = redis.call("GET", currentSessKey)
if oldSid and oldSid ~= "" then
  redis.call("HSET", sessionPrefix .. oldSid, "status", "closed")
end
redis.call("DEL", enabledKey)
redis.call("DEL", currentSessKey)
redis.call("ZREM", activeKey, userID)
redis.call("ZREMRANGEBYSCORE", activeKey, "-inf", nowUnix)
local count = tonumber(redis.call("ZCARD", activeKey)) or 0
redis.call("SET", countKey, count)
return count
`)

var reqLogSyncEnabledCountScript = redis.NewScript(`
local countKey = KEYS[1]
local activeKey = KEYS[2]
local nowUnix = tonumber(ARGV[1]) or 0
redis.call("ZREMRANGEBYSCORE", activeKey, "-inf", nowUnix)
local count = tonumber(redis.call("ZCARD", activeKey)) or 0
redis.call("SET", countKey, count)
return count
`)

var reqLogCleanupEnabledUserScript = redis.NewScript(`
local countKey = KEYS[1]
local activeKey = KEYS[2]
local userID = tostring(ARGV[1])
redis.call("ZREM", activeKey, userID)
local count = tonumber(redis.call("ZCARD", activeKey)) or 0
redis.call("SET", countKey, count)
return count
`)

var reqLogWriteScript = redis.NewScript(`
local enabledKey = KEYS[1]
local sessKey = KEYS[2]
local seqKey = KEYS[3]
local idxKey = KEYS[4]
local itemPrefix = KEYS[5]

local entryJSON = ARGV[1]
local entrySize = tonumber(ARGV[2]) or 0
local absTTL = tonumber(ARGV[3]) or 0
local maxBytes = tonumber(ARGV[4]) or 0
local maxItems = tonumber(ARGV[5]) or 0
local strategy = ARGV[6]
local entryTs = tonumber(ARGV[7]) or 0
local expiresAt = tonumber(ARGV[8]) or 0

local function drop()
  redis.call("HINCRBY", sessKey, "dropped_count", 1)
  if absTTL > 0 then
    redis.call("PEXPIRE", sessKey, absTTL)
  end
end

if redis.call("EXISTS", enabledKey) == 0 then
  drop()
  return {0, "disabled"}
end
if expiresAt > 0 and entryTs > expiresAt then
  drop()
  return {0, "expired"}
end
if absTTL <= 0 then
  drop()
  return {0, "ttl"}
end
if maxBytes <= 0 or maxItems <= 0 or entrySize <= 0 or entrySize > maxBytes then
  redis.call("HSET", sessKey, "truncated", 1)
  drop()
  return {0, "oversize"}
end

local bytesUsed = tonumber(redis.call("HGET", sessKey, "bytes_used") or "0") or 0
local itemCount = tonumber(redis.call("HGET", sessKey, "item_count") or "0") or 0

local function noteCalibration()
  redis.call("HINCRBY", sessKey, "calibration_count", 1)
end

local function parseMember(member)
  local sep = string.find(member, ":")
  if not sep then
    return member, nil, true
  end
  local seq = string.sub(member, 1, sep - 1)
  local size = tonumber(string.sub(member, sep + 1))
  if not size or size < 0 then
    return seq, nil, true
  end
  return seq, size, false
end

local function rebuildIndex(liveMembers)
  redis.call("DEL", idxKey)
  if #liveMembers > 0 then
    redis.call("RPUSH", idxKey, unpack(liveMembers))
  end
end

local function reconcile()
  local members = redis.call("LRANGE", idxKey, 0, -1)
  local liveMembers = {}
  local liveBytes = 0
  local liveCount = 0
  local changed = false
  for _, member in ipairs(members) do
    local seq, size, malformed = parseMember(member)
    if not seq or seq == "" then
      changed = true
      noteCalibration()
    else
      local key = itemPrefix .. seq
      if redis.call("EXISTS", key) == 1 then
        if malformed then
          size = tonumber(redis.call("STRLEN", key) or "0") or 0
          member = seq .. ":" .. tostring(size)
          changed = true
          noteCalibration()
        end
        liveBytes = liveBytes + size
        liveCount = liveCount + 1
        table.insert(liveMembers, member)
      else
        changed = true
        noteCalibration()
      end
    end
  end
  if changed then
    rebuildIndex(liveMembers)
  end
  if liveCount == 0 then
    liveBytes = 0
  end
  redis.call("HSET", sessKey, "bytes_used", liveBytes, "item_count", liveCount)
  return liveBytes, liveCount
end

local idxCount = redis.call("LLEN", idxKey)
local needReconcile = idxCount ~= itemCount or bytesUsed + entrySize > maxBytes or itemCount + 1 > maxItems
if needReconcile then
  bytesUsed, itemCount = reconcile()
end

if strategy == "stop" then
  if bytesUsed + entrySize > maxBytes or itemCount + 1 > maxItems then
    redis.call("HSET", sessKey, "truncated", 1)
    drop()
    return {0, "full"}
  end
else
  while bytesUsed + entrySize > maxBytes or itemCount + 1 > maxItems do
    local oldMember = redis.call("LPOP", idxKey)
    if not oldMember then
      bytesUsed = 0
      itemCount = 0
      break
    end
    local oldSeq, recordedSize, malformed = parseMember(oldMember)
    if not oldSeq or oldSeq == "" then
      recordedSize = 0
      noteCalibration()
    elseif malformed then
      local oldKey = itemPrefix .. oldSeq
      if redis.call("EXISTS", oldKey) == 1 then
        recordedSize = tonumber(redis.call("STRLEN", oldKey) or "0") or 0
      else
        recordedSize = 0
      end
      noteCalibration()
    end
    if oldSeq and oldSeq ~= "" then
      redis.call("DEL", itemPrefix .. oldSeq)
    end
    bytesUsed = bytesUsed - recordedSize
    if bytesUsed < 0 then bytesUsed = 0 end
    itemCount = itemCount - 1
    if itemCount < 0 then itemCount = 0 end
  end
end

local seq = redis.call("INCR", seqKey)
local itemKey = itemPrefix .. tostring(seq)
redis.call("SET", itemKey, entryJSON)
redis.call("PEXPIRE", itemKey, absTTL)
redis.call("RPUSH", idxKey, tostring(seq) .. ":" .. tostring(entrySize))
bytesUsed = bytesUsed + entrySize
itemCount = redis.call("LLEN", idxKey)

if itemCount == 0 then
  bytesUsed = 0
end
redis.call("HSET", sessKey, "bytes_used", bytesUsed, "item_count", itemCount)

redis.call("PEXPIRE", sessKey, absTTL)
redis.call("PEXPIRE", seqKey, absTTL)
redis.call("PEXPIRE", idxKey, absTTL)
return {1, seq}
`)
