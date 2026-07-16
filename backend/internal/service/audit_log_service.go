package service

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

const (
	auditLogQueueCapacity = 4096
	auditLogBatchSize     = 100
	auditLogFlushInterval = time.Second

	auditRetentionCheckInterval = 24 * time.Hour
	auditRetentionStartupDelay  = 5 * time.Minute
	auditRetentionBatchSize     = 5000
)

// AuditLogService 管理面操作审计日志服务。
// 写入端为非阻塞异步批量落库（不拖慢管理请求）；
// 读取端提供分页查询；清空端点由 handler 层做 TOTP 强校验后调用 ClearAll。
type AuditLogService struct {
	repo           AuditLogRepository
	settingService *SettingService

	queue chan *AuditLog

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	clearMu    sync.Mutex
	writeMu    sync.Mutex
	generation uint64

	droppedCount uint64
	writeFailed  uint64
	writtenCount uint64
}

func NewAuditLogService(repo AuditLogRepository, settingService *SettingService) *AuditLogService {
	ctx, cancel := context.WithCancel(context.Background())
	return &AuditLogService{
		repo:           repo,
		settingService: settingService,
		queue:          make(chan *AuditLog, auditLogQueueCapacity),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start 启动异步写入与保留期清理协程。
func (s *AuditLogService) Start() {
	if s == nil || s.repo == nil {
		return
	}
	s.wg.Add(2)
	go s.runWriter()
	go s.runRetentionLoop()
}

// Stop 停止服务并尽量落盘队列中剩余记录。
func (s *AuditLogService) Stop() {
	if s == nil {
		return
	}
	s.cancel()
	s.wg.Wait()
}

// Record 非阻塞入队一条审计记录；队列打满时丢弃并计数（管理面流量下几乎不可能发生）。
func (s *AuditLogService) Record(entry *AuditLog) {
	if s == nil || entry == nil {
		return
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}
	entry.generation = atomic.LoadUint64(&s.generation)
	select {
	case <-s.ctx.Done():
		return
	default:
	}
	select {
	case s.queue <- entry:
	default:
		atomic.AddUint64(&s.droppedCount, 1)
	}
}

// List 分页查询审计日志。
func (s *AuditLogService) List(ctx context.Context, filter *AuditLogFilter) (*AuditLogList, error) {
	return s.repo.List(ctx, filter)
}

// GetByID 查询单条详情。
func (s *AuditLogService) GetByID(ctx context.Context, id int64) (*AuditLog, error) {
	return s.repo.GetByID(ctx, id)
}

// ClearAll 全量清空审计日志并写入留痕记录。
// 调用方（handler）必须先完成 TOTP 验证；本方法负责：
//  1. 统计并清空全表
//  2. 同步写入一条 "audit_log.clear" 留痕记录（绕过异步队列，保证落库）
func (s *AuditLogService) ClearAll(ctx context.Context, trace *AuditLog) (int64, error) {
	if s == nil || s.repo == nil {
		return 0, fmt.Errorf("audit log service is unavailable")
	}
	if trace != nil {
		trace.Action = AuditActionAuditLogClear
		if trace.CreatedAt.IsZero() {
			trace.CreatedAt = time.Now().UTC()
		}
		if trace.Extra == nil {
			trace.Extra = map[string]any{}
		}
	}

	s.clearMu.Lock()
	defer s.clearMu.Unlock()

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	// 持有 writer 锁后推进代际：此前已排队/已进入 batch 的记录会在下次 flush 时丢弃；
	// 此后到达的记录属于新代际，应在清空事务完成后继续落库。
	atomic.AddUint64(&s.generation, 1)

	deleted, err := s.repo.ClearAllWithTrace(ctx, trace)
	if err != nil {
		// 数据库事务未发生清空时恢复旧代际，避免一次失败的清空请求丢弃排队审计。
		atomic.AddUint64(&s.generation, ^uint64(0))
		return 0, err
	}
	return deleted, nil
}

func (s *AuditLogService) runWriter() {
	defer s.wg.Done()

	ticker := time.NewTicker(auditLogFlushInterval)
	defer ticker.Stop()

	batch := make([]*AuditLog, 0, auditLogBatchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		s.writeMu.Lock()
		defer s.writeMu.Unlock()

		currentGeneration := atomic.LoadUint64(&s.generation)
		pending := make([]*AuditLog, 0, len(batch))
		for _, item := range batch {
			if item != nil && item.generation >= currentGeneration {
				pending = append(pending, item)
			}
		}
		batch = batch[:0]
		if len(pending) == 0 {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		inserted, err := s.repo.BatchInsert(ctx, pending)
		cancel()
		if err != nil {
			var recovered uint64
			var failed uint64
			for _, item := range pending {
				itemCtx, itemCancel := context.WithTimeout(context.Background(), 5*time.Second)
				itemErr := s.repo.Insert(itemCtx, item)
				itemCancel()
				if itemErr != nil {
					failed++
					_, _ = fmt.Fprintf(os.Stderr, "time=%s level=WARN msg=\"audit log individual insert failed\" err=%v action=%s\n",
						time.Now().Format(time.RFC3339Nano), itemErr, item.Action)
					continue
				}
				recovered++
			}
			atomic.AddUint64(&s.writtenCount, recovered)
			atomic.AddUint64(&s.writeFailed, failed)
			_, _ = fmt.Fprintf(os.Stderr, "time=%s level=WARN msg=\"audit log batch flush failed; individual fallback completed\" err=%v batch=%d recovered=%d failed=%d\n",
				time.Now().Format(time.RFC3339Nano), err, len(pending), recovered, failed)
		} else {
			atomic.AddUint64(&s.writtenCount, uint64(inserted))
		}
	}

	for {
		select {
		case <-s.ctx.Done():
			// 停机前排空队列。
			for {
				select {
				case item := <-s.queue:
					if item == nil {
						continue
					}
					batch = append(batch, item)
					if len(batch) >= auditLogBatchSize {
						flush()
					}
				default:
					flush()
					return
				}
			}
		case item := <-s.queue:
			if item == nil {
				continue
			}
			batch = append(batch, item)
			if len(batch) >= auditLogBatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

// runRetentionLoop 按保留期定期删除过期审计日志。
// 删除操作幂等，多实例并发执行无害，因此无需选主。
func (s *AuditLogService) runRetentionLoop() {
	defer s.wg.Done()

	startupTimer := time.NewTimer(auditRetentionStartupDelay)
	defer startupTimer.Stop()
	select {
	case <-s.ctx.Done():
		return
	case <-startupTimer.C:
	}

	ticker := time.NewTicker(auditRetentionCheckInterval)
	defer ticker.Stop()

	s.runRetentionOnce()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.runRetentionOnce()
		}
	}
}

func (s *AuditLogService) runRetentionOnce() {
	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Minute)
	defer cancel()

	days := 0
	if s.settingService != nil {
		days = s.settingService.GetAuditLogRetentionDays(ctx)
	}
	if days <= 0 {
		return // 0 或负值表示永久保留，仅支持手动清空
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -days)
	for {
		deleted, err := s.repo.DeleteBefore(ctx, cutoff, auditRetentionBatchSize)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "time=%s level=WARN msg=\"audit log retention cleanup failed\" err=%v\n",
				time.Now().Format(time.RFC3339Nano), err)
			return
		}
		if deleted == 0 {
			return
		}
	}
}
