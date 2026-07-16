package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type auditLogMemoryRepo struct {
	mu             sync.Mutex
	logs           []*AuditLog
	batchErr       error
	individualErr  error
	clearErr       error
	truncateCalled chan struct{}
}

func (r *auditLogMemoryRepo) BatchInsert(_ context.Context, logs []*AuditLog) (int64, error) {
	if r.batchErr != nil {
		return 0, r.batchErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logs = append(r.logs, logs...)
	return int64(len(logs)), nil
}

func (r *auditLogMemoryRepo) Insert(_ context.Context, log *AuditLog) error {
	if r.individualErr != nil {
		return r.individualErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logs = append(r.logs, log)
	return nil
}

func (r *auditLogMemoryRepo) List(context.Context, *AuditLogFilter) (*AuditLogList, error) {
	return &AuditLogList{}, nil
}

func (r *auditLogMemoryRepo) GetByID(context.Context, int64) (*AuditLog, error) {
	return nil, ErrAuditLogNotFound
}

func (r *auditLogMemoryRepo) Count(context.Context) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return int64(len(r.logs)), nil
}

func (r *auditLogMemoryRepo) TruncateAll(context.Context) error {
	r.mu.Lock()
	r.logs = nil
	r.mu.Unlock()
	if r.truncateCalled != nil {
		close(r.truncateCalled)
	}
	return nil
}

func (r *auditLogMemoryRepo) ClearAllWithTrace(_ context.Context, trace *AuditLog) (int64, error) {
	if r.clearErr != nil {
		return 0, r.clearErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	deleted := int64(len(r.logs))
	r.logs = nil
	if trace != nil {
		if trace.Extra == nil {
			trace.Extra = map[string]any{}
		}
		trace.Extra["deleted_rows"] = deleted
		r.logs = append(r.logs, trace)
	}
	return deleted, nil
}

func (r *auditLogMemoryRepo) DeleteBefore(context.Context, time.Time, int) (int64, error) {
	return 0, nil
}

func (r *auditLogMemoryRepo) snapshot() []*AuditLog {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]*AuditLog(nil), r.logs...)
}

func TestAuditLogServiceClearDoesNotReinsertBufferedEntries(t *testing.T) {
	repo := &auditLogMemoryRepo{}
	svc := NewAuditLogService(repo, nil)
	svc.Start()
	defer svc.Stop()

	svc.Record(&AuditLog{Action: "before.clear"})
	require.Eventually(t, func() bool { return len(svc.queue) == 0 }, time.Second, 10*time.Millisecond)

	deleted, err := svc.ClearAll(context.Background(), &AuditLog{Action: AuditActionAuditLogClear})
	require.NoError(t, err)
	require.Zero(t, deleted)

	time.Sleep(auditLogFlushInterval + 100*time.Millisecond)
	logs := repo.snapshot()
	require.Len(t, logs, 1)
	require.Equal(t, AuditActionAuditLogClear, logs[0].Action)
}

func TestAuditLogServiceFallsBackToIndividualInserts(t *testing.T) {
	repo := &auditLogMemoryRepo{batchErr: errors.New("copy failed")}
	svc := NewAuditLogService(repo, nil)
	svc.Start()

	svc.Record(&AuditLog{Action: "first"})
	svc.Record(&AuditLog{Action: "second"})
	svc.Stop()

	logs := repo.snapshot()
	require.Len(t, logs, 2)
	require.Equal(t, "first", logs[0].Action)
	require.Equal(t, "second", logs[1].Action)
}

func TestAuditLogServiceFailedClearKeepsBufferedEntries(t *testing.T) {
	repo := &auditLogMemoryRepo{clearErr: errors.New("transaction rolled back")}
	svc := NewAuditLogService(repo, nil)
	svc.Start()
	defer svc.Stop()

	svc.Record(&AuditLog{Action: "before.failed.clear"})
	require.Eventually(t, func() bool { return len(svc.queue) == 0 }, time.Second, 10*time.Millisecond)

	_, err := svc.ClearAll(context.Background(), &AuditLog{Action: AuditActionAuditLogClear})
	require.Error(t, err)

	time.Sleep(auditLogFlushInterval + 100*time.Millisecond)
	logs := repo.snapshot()
	require.Len(t, logs, 1)
	require.Equal(t, "before.failed.clear", logs[0].Action)
}
