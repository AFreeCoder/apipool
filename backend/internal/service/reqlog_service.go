package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/reqlog"
	"github.com/Wei-Shaw/sub2api/internal/util/logredact"
)

var (
	ErrReqLogDisabled        = errors.New("request log disabled")
	ErrReqLogReasonRequired  = errors.New("request log reason is required")
	ErrReqLogAlreadyEnabled  = errors.New("request log already enabled")
	ErrReqLogNotFound        = errors.New("request log not found")
	ErrReqLogUnauthorized    = errors.New("request log unauthorized")
	ErrReqLogConcurrentLimit = errors.New("request log concurrent session limit reached")
)

type ReqLogStore interface {
	GetEnabled(ctx context.Context, userID int64) (*reqlog.CaptureState, error)
	EnableSession(ctx context.Context, state *reqlog.CaptureState, window, retention time.Duration, force bool, maxConcurrent int) (*reqlog.CaptureState, bool, error)
	DisableSession(ctx context.Context, userID int64) error
	CountEnabled(ctx context.Context) (int, error)
	ListActive(ctx context.Context) ([]*reqlog.CaptureState, error)
	WriteItem(ctx context.Context, entry *reqlog.ReqLogEntry, state *reqlog.CaptureState, retention time.Duration) (int64, error)
	DropItem(ctx context.Context, state *reqlog.CaptureState) error
	GetStats(ctx context.Context, userID int64, sessionID string) (*ReqLogSessionStats, error)
	ResolveSessionUser(ctx context.Context, sessionID string) (int64, error)
	ListSessions(ctx context.Context, userID int64, limit int) ([]ReqLogSession, error)
	ListItems(ctx context.Context, sessionID string, page, pageSize int) ([]*reqlog.ReqLogEntry, int64, error)
	GetItem(ctx context.Context, sessionID string, seq int64) (*reqlog.ReqLogEntry, error)
	CreateDownloadToken(ctx context.Context, sessionID string, adminID int64, ttl time.Duration) (string, time.Time, error)
	ConsumeDownloadToken(ctx context.Context, token string) (*ReqLogDownloadToken, error)
	MemoryStats(ctx context.Context) (*ReqLogRedisMemoryStats, error)
	Close() error
}

type ReqLogSessionStats struct {
	BytesUsed    int64     `json:"bytes_used"`
	ItemCount    int64     `json:"item_count"`
	Truncated    bool      `json:"truncated"`
	DroppedCount int64     `json:"dropped_count"`
	StartedAt    time.Time `json:"started_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	Status       string    `json:"status"`
}

type ReqLogSession struct {
	UserID       int64     `json:"user_id"`
	SessionID    string    `json:"session_id"`
	StartedAt    time.Time `json:"started_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	CutoffAt     time.Time `json:"cutoff_at"`
	BytesUsed    int64     `json:"bytes_used"`
	ItemCount    int64     `json:"item_count"`
	Truncated    bool      `json:"truncated"`
	DroppedCount int64     `json:"dropped_count"`
	Status       string    `json:"status"`
	Reason       string    `json:"reason,omitempty"`
}

type ReqLogDownloadToken struct {
	SessionID string    `json:"session_id"`
	AdminID   int64     `json:"admin_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

type ReqLogRedisMemoryStats struct {
	UsedMemory int64 `json:"used_memory"`
	MaxMemory  int64 `json:"maxmemory"`
	Percent    int   `json:"percent"`
	Guarded    bool  `json:"guarded"`
}

type ReqLogEnableInput struct {
	UserID   int64         `json:"user_id"`
	AdminID  int64         `json:"admin_id"`
	TTL      time.Duration `json:"ttl"`
	Reason   string        `json:"reason"`
	Force    bool          `json:"force"`
	MaxBytes int64         `json:"max_bytes,omitempty"`
	MaxItems int           `json:"max_items,omitempty"`
	ReqCap   int           `json:"single_request_cap,omitempty"`
	RespCap  int           `json:"single_response_cap,omitempty"`
	Now      time.Time     `json:"-"`
}

type ReqLogStatus struct {
	Enabled          bool                    `json:"enabled"`
	Session          *reqlog.CaptureState    `json:"session,omitempty"`
	RemainingSeconds int64                   `json:"remaining_seconds"`
	Stats            *ReqLogSessionStats     `json:"stats,omitempty"`
	Memory           *ReqLogRedisMemoryStats `json:"memory,omitempty"`
}

type ReqLogActiveSession struct {
	UserID           int64  `json:"user_id"`
	SessionID        string `json:"session_id"`
	StartedAt        int64  `json:"started_at"`
	ExpiresAt        int64  `json:"expires_at"`
	RemainingSeconds int64  `json:"remaining_seconds"`
	Reason           string `json:"reason,omitempty"`
}

type reqLogLocalState struct {
	state      *reqlog.CaptureState
	expiresAt  time.Time
	generation uint64
}

type ReqLogService struct {
	cfg   *config.Config
	store ReqLogStore
	sink  *ReqLogSink

	mu    sync.RWMutex
	cache map[int64]reqLogLocalState

	generation atomic.Uint64

	memMu      sync.Mutex
	memStats   *ReqLogRedisMemoryStats
	memExpires time.Time
}

func NewReqLogService(cfg *config.Config, store ReqLogStore, sink *ReqLogSink) *ReqLogService {
	return &ReqLogService{
		cfg:   cfg,
		store: store,
		sink:  sink,
		cache: make(map[int64]reqLogLocalState),
	}
}

func ProvideReqLogService(cfg *config.Config, store ReqLogStore, sink *ReqLogSink) *ReqLogService {
	return NewReqLogService(cfg, store, sink)
}

func (s *ReqLogService) ConfigEnabled() bool {
	return s != nil && s.cfg != nil && s.cfg.Ops.RequestLog.Enabled && s.store != nil
}

func (s *ReqLogService) IncrementGeneration() {
	if s == nil {
		return
	}
	s.generation.Add(1)
}

func (s *ReqLogService) currentGeneration() uint64 {
	if s == nil {
		return 0
	}
	return s.generation.Load()
}

func (s *ReqLogService) GetCaptureState(ctx context.Context, userID int64, now time.Time) (*reqlog.CaptureState, bool) {
	if !s.ConfigEnabled() || userID <= 0 {
		return nil, false
	}
	if now.IsZero() {
		now = time.Now()
	}
	gen := s.currentGeneration()
	ttl := s.cfg.Ops.RequestLog.LocalCacheTTL
	if ttl <= 0 {
		ttl = 5 * time.Second
	}

	s.mu.RLock()
	if cached, ok := s.cache[userID]; ok && cached.generation == gen && now.Before(cached.expiresAt) {
		s.mu.RUnlock()
		if cached.state == nil {
			return nil, false
		}
		st := cached.state.Clone()
		if !st.ExpiresAt.IsZero() && now.After(st.ExpiresAt) {
			return nil, false
		}
		return st, true
	}
	s.mu.RUnlock()

	state, err := s.store.GetEnabled(ctx, userID)
	if err != nil {
		s.writeLocalCache(userID, nil, now.Add(ttl), gen)
		return nil, false
	}
	if state != nil {
		state.NormalizeTimes()
		if !state.ExpiresAt.IsZero() && now.After(state.ExpiresAt) {
			state = nil
		}
	}
	s.writeLocalCache(userID, state, now.Add(ttl), gen)
	if state == nil {
		return nil, false
	}
	return state.Clone(), true
}

func (s *ReqLogService) writeLocalCache(userID int64, state *reqlog.CaptureState, expiresAt time.Time, generation uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache[userID] = reqLogLocalState{state: state.Clone(), expiresAt: expiresAt, generation: generation}
}

func (s *ReqLogService) Enable(ctx context.Context, input ReqLogEnableInput) (*reqlog.CaptureState, *ReqLogRedisMemoryStats, error) {
	if !s.ConfigEnabled() {
		return nil, nil, ErrReqLogDisabled
	}
	if input.UserID <= 0 {
		return nil, nil, fmt.Errorf("invalid user id")
	}
	reason := strings.TrimSpace(input.Reason)
	if reason == "" {
		return nil, nil, ErrReqLogReasonRequired
	}
	now := input.Now
	if now.IsZero() {
		now = time.Now()
	}
	cfg := s.cfg.Ops.RequestLog
	window := input.TTL
	if window <= 0 {
		window = cfg.MaxWindow
	}
	if window > cfg.MaxWindow {
		window = cfg.MaxWindow
	}
	if window > 30*time.Minute {
		window = 30 * time.Minute
	}

	mem, _ := s.CheckMemory(ctx)
	if mem != nil && mem.Guarded {
		return nil, mem, fmt.Errorf("request log redis memory guard active")
	}

	state := &reqlog.CaptureState{
		UserID:             input.UserID,
		SessionID:          newReqLogSessionID(now),
		StartedAt:          now.UTC(),
		ExpiresAt:          now.Add(window).UTC(),
		StartedByAdminID:   input.AdminID,
		MaxBytes:           firstPositiveInt64(input.MaxBytes, cfg.MaxBytesPerSession),
		MaxItems:           firstPositiveInt(input.MaxItems, cfg.MaxItemsPerSession),
		SingleRequestCap:   firstPositiveInt(input.ReqCap, cfg.SingleRequestCap),
		SingleResponseCap:  firstPositiveInt(input.RespCap, cfg.SingleResponseCap),
		OverflowStrategy:   cfg.OverflowStrategy,
		Reason:             reason,
		RetentionAfterStop: cfg.RetentionAfterWindow,
	}
	state.NormalizeTimes()
	maxConcurrent := cfg.MaxConcurrentSessions
	if maxConcurrent <= 0 {
		maxConcurrent = 3
	}
	created, existed, err := s.store.EnableSession(ctx, state, window, cfg.RetentionAfterWindow, input.Force, maxConcurrent)
	if err != nil {
		return nil, mem, err
	}
	if existed && !input.Force {
		return created, mem, ErrReqLogAlreadyEnabled
	}
	s.IncrementGeneration()
	s.audit(input.AdminID, input.UserID, state.SessionID, "enable", map[string]any{"reason": reason})
	return created, mem, nil
}

func (s *ReqLogService) Disable(ctx context.Context, userID, adminID int64) error {
	if !s.ConfigEnabled() {
		return ErrReqLogDisabled
	}
	var sessionID string
	if st, _ := s.GetCaptureState(ctx, userID, time.Now()); st != nil {
		sessionID = st.SessionID
	}
	if err := s.store.DisableSession(ctx, userID); err != nil {
		return err
	}
	s.IncrementGeneration()
	s.audit(adminID, userID, sessionID, "disable", nil)
	return nil
}

func (s *ReqLogService) Status(ctx context.Context, userID int64) (*ReqLogStatus, error) {
	if !s.ConfigEnabled() {
		return &ReqLogStatus{Enabled: false}, nil
	}
	state, err := s.store.GetEnabled(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := &ReqLogStatus{Enabled: state != nil}
	if state == nil {
		return out, nil
	}
	state.NormalizeTimes()
	out.Session = state.Clone()
	if !state.ExpiresAt.IsZero() {
		out.RemainingSeconds = int64(time.Until(state.ExpiresAt).Seconds())
		if out.RemainingSeconds < 0 {
			out.RemainingSeconds = 0
		}
	}
	out.Stats, _ = s.store.GetStats(ctx, userID, state.SessionID)
	out.Memory, _ = s.CheckMemory(ctx)
	return out, nil
}

func (s *ReqLogService) ListActive(ctx context.Context) ([]ReqLogActiveSession, error) {
	if !s.ConfigEnabled() {
		return []ReqLogActiveSession{}, nil
	}
	states, err := s.store.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	out := make([]ReqLogActiveSession, 0, len(states))
	for _, state := range states {
		if state == nil {
			continue
		}
		state.NormalizeTimes()
		if state.UserID <= 0 || state.SessionID == "" || state.ExpiresAt.IsZero() || !now.Before(state.ExpiresAt) {
			continue
		}
		remaining := int64(time.Until(state.ExpiresAt).Seconds())
		if remaining < 0 {
			remaining = 0
		}
		out = append(out, ReqLogActiveSession{
			UserID:           state.UserID,
			SessionID:        state.SessionID,
			StartedAt:        state.StartedAtUnix,
			ExpiresAt:        state.ExpiresAtUnix,
			RemainingSeconds: remaining,
			Reason:           state.Reason,
		})
	}
	return out, nil
}

func (s *ReqLogService) Submit(entry *reqlog.ReqLogEntry) bool {
	if s == nil || s.sink == nil || entry == nil {
		return false
	}
	return s.sink.Submit(entry)
}

func (s *ReqLogService) ListSessions(ctx context.Context, adminID, userID int64, limit int) ([]ReqLogSession, error) {
	if !s.ConfigEnabled() {
		return nil, ErrReqLogDisabled
	}
	items, err := s.store.ListSessions(ctx, userID, limit)
	if err == nil {
		s.audit(adminID, userID, "", "list_sessions", map[string]any{"limit": limit})
	}
	return items, err
}

func (s *ReqLogService) ListItems(ctx context.Context, adminID int64, sessionID string, page, pageSize int) ([]*reqlog.ReqLogEntry, int64, int64, error) {
	if !s.ConfigEnabled() {
		return nil, 0, 0, ErrReqLogDisabled
	}
	userID, err := s.store.ResolveSessionUser(ctx, sessionID)
	if err != nil {
		return nil, 0, 0, err
	}
	items, total, err := s.store.ListItems(ctx, sessionID, page, pageSize)
	if err == nil {
		s.audit(adminID, userID, sessionID, "list_items", map[string]any{"page": page, "page_size": pageSize})
	}
	return items, total, userID, err
}

func (s *ReqLogService) GetItem(ctx context.Context, adminID int64, sessionID string, seq int64) (*reqlog.ReqLogEntry, int64, error) {
	if !s.ConfigEnabled() {
		return nil, 0, ErrReqLogDisabled
	}
	userID, err := s.store.ResolveSessionUser(ctx, sessionID)
	if err != nil {
		return nil, 0, err
	}
	item, err := s.store.GetItem(ctx, sessionID, seq)
	if err == nil {
		s.audit(adminID, userID, sessionID, "view_item", map[string]any{"seq": seq})
	}
	return item, userID, err
}

func (s *ReqLogService) CreateDownloadToken(ctx context.Context, adminID int64, sessionID string) (string, time.Time, error) {
	if !s.ConfigEnabled() {
		return "", time.Time{}, ErrReqLogDisabled
	}
	// P6：按 sid 的接口必须先反查 uid，避免给不存在/已过期的 session 签发有效 token。
	if _, err := s.store.ResolveSessionUser(ctx, sessionID); err != nil {
		return "", time.Time{}, err
	}
	return s.store.CreateDownloadToken(ctx, sessionID, adminID, time.Minute)
}

func (s *ReqLogService) ConsumeDownloadToken(ctx context.Context, token, sessionID string) (int64, error) {
	if !s.ConfigEnabled() {
		return 0, ErrReqLogDisabled
	}
	tok, err := s.store.ConsumeDownloadToken(ctx, strings.TrimSpace(token))
	if err != nil {
		return 0, err
	}
	if tok == nil || tok.SessionID != sessionID {
		return 0, ErrReqLogUnauthorized
	}
	return tok.AdminID, nil
}

func (s *ReqLogService) ResolveSessionUser(ctx context.Context, sessionID string) (int64, error) {
	if !s.ConfigEnabled() {
		return 0, ErrReqLogDisabled
	}
	return s.store.ResolveSessionUser(ctx, sessionID)
}

// GetSessionStats 反查 uid 后读取该会话的统计信息（P7：供下载首行 metadata 使用）。
func (s *ReqLogService) GetSessionStats(ctx context.Context, sessionID string) (*ReqLogSessionStats, int64, error) {
	if !s.ConfigEnabled() {
		return nil, 0, ErrReqLogDisabled
	}
	userID, err := s.store.ResolveSessionUser(ctx, sessionID)
	if err != nil {
		return nil, 0, err
	}
	stats, err := s.store.GetStats(ctx, userID, sessionID)
	if err != nil {
		return nil, userID, err
	}
	return stats, userID, nil
}

func (s *ReqLogService) DownloadItems(ctx context.Context, adminID int64, sessionID string, fn func(*reqlog.ReqLogEntry) error) (int64, error) {
	userID, err := s.ResolveSessionUser(ctx, sessionID)
	if err != nil {
		return 0, err
	}
	s.audit(adminID, userID, sessionID, "download", nil)
	page := 1
	for {
		items, _, err := s.store.ListItems(ctx, sessionID, page, 100)
		if err != nil {
			return userID, err
		}
		if len(items) == 0 {
			break
		}
		for _, item := range items {
			if err := fn(item); err != nil {
				return userID, err
			}
		}
		page++
	}
	return userID, nil
}

func (s *ReqLogService) CheckMemory(ctx context.Context) (*ReqLogRedisMemoryStats, error) {
	if s == nil || s.store == nil || s.cfg == nil {
		return nil, nil
	}
	now := time.Now()
	ttl := s.cfg.Ops.RequestLog.MemoryInfoCacheTTL
	if ttl <= 0 {
		ttl = 5 * time.Second
	}
	s.memMu.Lock()
	if s.memStats != nil && now.Before(s.memExpires) {
		out := *s.memStats
		s.memMu.Unlock()
		return &out, nil
	}
	s.memMu.Unlock()

	stats, err := s.store.MemoryStats(ctx)
	if err != nil {
		return nil, err
	}
	if stats != nil && stats.MaxMemory > 0 {
		stats.Percent = int((stats.UsedMemory * 100) / stats.MaxMemory)
		stats.Guarded = stats.Percent >= s.cfg.Ops.RequestLog.MemoryGuardPercent
	}
	s.memMu.Lock()
	if stats != nil {
		cp := *stats
		s.memStats = &cp
	} else {
		s.memStats = nil
	}
	s.memExpires = now.Add(ttl)
	s.memMu.Unlock()
	return stats, nil
}

func (s *ReqLogService) Close() error {
	if s == nil || s.store == nil {
		return nil
	}
	return s.store.Close()
}

func (s *ReqLogService) audit(adminID, targetUserID int64, sessionID, action string, extra map[string]any) {
	fields := map[string]any{
		"component":      "audit.reqlog",
		"admin_id":       adminID,
		"target_user_id": targetUserID,
		"session_id":     sessionID,
		"action":         action,
	}
	for k, v := range extra {
		fields[k] = v
	}
	logger.WriteSinkEvent("info", "audit.reqlog", "request log audit", fields)
}

func RedactHeaders(h http.Header, maxBytes int) map[string]string {
	raw := reqlog.HeaderMap(h, maxBytes)
	input := make(map[string]any, len(raw))
	for k, v := range raw {
		input[k] = v
	}
	redacted := logredact.RedactMap(input,
		"authorization",
		"proxy-authorization",
		"x-api-key",
		"x-goog-api-key",
		"api-key",
		"apikey",
		"cookie",
		"set-cookie",
	)
	out := make(map[string]string, len(redacted))
	for k, v := range redacted {
		if s, ok := v.(string); ok {
			out[k] = s
			continue
		}
		out[k] = fmt.Sprint(v)
	}
	return out
}

func TruncateBody(in []byte, capBytes int) ([]byte, bool) {
	return reqlog.TruncateUTF8Bytes(in, capBytes)
}

func newReqLogSessionID(now time.Time) string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return "rl_" + now.UTC().Format("20060102T150405Z") + "_" + hex.EncodeToString(b[:])
}

func firstPositiveInt(v, fallback int) int {
	if v > 0 {
		return v
	}
	return fallback
}

func firstPositiveInt64(v, fallback int64) int64 {
	if v > 0 {
		return v
	}
	return fallback
}
