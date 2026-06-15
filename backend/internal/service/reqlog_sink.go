package service

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/reqlog"
)

type ReqLogSinkHealth struct {
	QueueDepth    int64  `json:"queue_depth"`
	QueueCapacity int64  `json:"queue_capacity"`
	InflightBytes int64  `json:"inflight_bytes"`
	DroppedCount  uint64 `json:"dropped_count"`
	DroppedBytes  uint64 `json:"dropped_bytes"`
	WriteFailed   uint64 `json:"write_failed_count"`
	WrittenCount  uint64 `json:"written_count"`
	LastError     string `json:"last_error"`
}

type reqLogQueuedEntry struct {
	entry *reqlog.ReqLogEntry
	bytes int64
}

type ReqLogSink struct {
	store ReqLogStore
	cfg   *config.Config

	queue chan reqLogQueuedEntry
	ctx   context.Context
	stop  context.CancelFunc
	wg    sync.WaitGroup

	startOnce sync.Once
	stopOnce  sync.Once

	inflightBytes atomic.Int64
	droppedCount  atomic.Uint64
	droppedBytes  atomic.Uint64
	writeFailed   atomic.Uint64
	writtenCount  atomic.Uint64
	lastError     atomic.Value
}

func NewReqLogSink(store ReqLogStore, cfg *config.Config) *ReqLogSink {
	capacity := 2000
	if cfg != nil && cfg.Ops.RequestLog.QueueCapacity > 0 {
		capacity = cfg.Ops.RequestLog.QueueCapacity
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &ReqLogSink{
		store: store,
		cfg:   cfg,
		queue: make(chan reqLogQueuedEntry, capacity),
		ctx:   ctx,
		stop:  cancel,
	}
	s.lastError.Store("")
	return s
}

func ProvideReqLogSink(store ReqLogStore, cfg *config.Config) *ReqLogSink {
	s := NewReqLogSink(store, cfg)
	s.Start()
	return s
}

func (s *ReqLogSink) Start() {
	if s == nil || s.store == nil {
		return
	}
	s.startOnce.Do(func() {
		workers := 2
		for i := 0; i < workers; i++ {
			s.wg.Add(1)
			go s.run()
		}
	})
}

func (s *ReqLogSink) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		s.stop()
		s.wg.Wait()
	})
}

func (s *ReqLogSink) Submit(entry *reqlog.ReqLogEntry) bool {
	if s == nil || entry == nil {
		return false
	}
	select {
	case <-s.ctx.Done():
		return false
	default:
	}
	cp := entry.DeepCopy()
	n := cp.EstimateBytes()
	budget := int64(64 * 1024 * 1024)
	if s.cfg != nil && s.cfg.Ops.RequestLog.QueueByteBudget > 0 {
		budget = s.cfg.Ops.RequestLog.QueueByteBudget
	}
	for {
		cur := s.inflightBytes.Load()
		if cur+n > budget {
			s.droppedCount.Add(1)
			s.droppedBytes.Add(uint64(n))
			return false
		}
		if s.inflightBytes.CompareAndSwap(cur, cur+n) {
			break
		}
	}
	select {
	case s.queue <- reqLogQueuedEntry{entry: cp, bytes: n}:
		return true
	default:
		s.inflightBytes.Add(-n)
		s.droppedCount.Add(1)
		s.droppedBytes.Add(uint64(n))
		return false
	}
}

func (s *ReqLogSink) run() {
	defer s.wg.Done()
	for {
		select {
		case <-s.ctx.Done():
			s.drain()
			return
		case item := <-s.queue:
			s.process(item)
		}
	}
}

func (s *ReqLogSink) drain() {
	deadline := time.After(2 * time.Second)
	for {
		select {
		case item := <-s.queue:
			s.process(item)
		case <-deadline:
			return
		default:
			return
		}
	}
}

func (s *ReqLogSink) process(item reqLogQueuedEntry) {
	defer s.inflightBytes.Add(-item.bytes)
	if item.entry == nil {
		return
	}
	fallback := &reqlog.CaptureState{
		UserID:            item.entry.UserID,
		SessionID:         item.entry.SessionID,
		ExpiresAt:         item.entry.Timestamp.Add(time.Second),
		MaxBytes:          defaultReqLogMaxBytes(s.cfg),
		MaxItems:          defaultReqLogMaxItems(s.cfg),
		OverflowStrategy:  defaultReqLogOverflow(s.cfg),
		SingleRequestCap:  defaultReqLogRequestCap(s.cfg),
		SingleResponseCap: defaultReqLogResponseCap(s.cfg),
	}
	enabled, err := s.store.GetEnabled(context.Background(), item.entry.UserID)
	if err != nil || enabled == nil {
		_ = s.store.DropItem(context.Background(), fallback)
		s.droppedCount.Add(1)
		return
	}
	state := enabled
	state.NormalizeTimes()
	if !state.ExpiresAt.IsZero() && item.entry.Timestamp.After(state.ExpiresAt) {
		_ = s.store.DropItem(context.Background(), state)
		s.droppedCount.Add(1)
		return
	}
	retention := 6 * time.Hour
	if s.cfg != nil && s.cfg.Ops.RequestLog.RetentionAfterWindow > 0 {
		retention = s.cfg.Ops.RequestLog.RetentionAfterWindow
	}
	if _, err := s.store.WriteItem(context.Background(), item.entry, state, retention); err != nil {
		s.writeFailed.Add(1)
		s.lastError.Store(err.Error())
		return
	}
	s.writtenCount.Add(1)
	s.lastError.Store("")
}

func defaultReqLogMaxBytes(cfg *config.Config) int64 {
	if cfg != nil && cfg.Ops.RequestLog.MaxBytesPerSession > 0 {
		return cfg.Ops.RequestLog.MaxBytesPerSession
	}
	return 32 * 1024 * 1024
}

func defaultReqLogMaxItems(cfg *config.Config) int {
	if cfg != nil && cfg.Ops.RequestLog.MaxItemsPerSession > 0 {
		return cfg.Ops.RequestLog.MaxItemsPerSession
	}
	return 1000
}

func defaultReqLogOverflow(cfg *config.Config) string {
	if cfg != nil && cfg.Ops.RequestLog.OverflowStrategy != "" {
		return cfg.Ops.RequestLog.OverflowStrategy
	}
	return reqlog.OverflowDropOldest
}

func defaultReqLogRequestCap(cfg *config.Config) int {
	if cfg != nil && cfg.Ops.RequestLog.SingleRequestCap > 0 {
		return cfg.Ops.RequestLog.SingleRequestCap
	}
	return 256 * 1024
}

func defaultReqLogResponseCap(cfg *config.Config) int {
	if cfg != nil && cfg.Ops.RequestLog.SingleResponseCap > 0 {
		return cfg.Ops.RequestLog.SingleResponseCap
	}
	return 256 * 1024
}

func (s *ReqLogSink) Health() ReqLogSinkHealth {
	if s == nil {
		return ReqLogSinkHealth{}
	}
	last, _ := s.lastError.Load().(string)
	return ReqLogSinkHealth{
		QueueDepth:    int64(len(s.queue)),
		QueueCapacity: int64(cap(s.queue)),
		InflightBytes: s.inflightBytes.Load(),
		DroppedCount:  s.droppedCount.Load(),
		DroppedBytes:  s.droppedBytes.Load(),
		WriteFailed:   s.writeFailed.Load(),
		WrittenCount:  s.writtenCount.Load(),
		LastError:     last,
	}
}
