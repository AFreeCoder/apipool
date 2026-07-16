//go:build unit

package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAuditLogRepositoryClearAllWithTraceCommitsAtomically(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM audit_logs`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(7))
	mock.ExpectExec(`TRUNCATE TABLE audit_logs`).
		WillReturnResult(sqlmock.NewResult(0, 7))
	mock.ExpectExec(`INSERT INTO audit_logs`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	repo := NewAuditLogRepository(db)
	trace := &service.AuditLog{Action: service.AuditActionAuditLogClear}
	deleted, err := repo.ClearAllWithTrace(context.Background(), trace)

	require.NoError(t, err)
	require.EqualValues(t, 7, deleted)
	require.EqualValues(t, 7, trace.Extra["deleted_rows"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAuditLogRepositoryClearAllWithTraceRollsBackWhenTraceInsertFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM audit_logs`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(7))
	mock.ExpectExec(`TRUNCATE TABLE audit_logs`).
		WillReturnResult(sqlmock.NewResult(0, 7))
	mock.ExpectExec(`INSERT INTO audit_logs`).
		WillReturnError(errors.New("insert failed"))
	mock.ExpectRollback()

	repo := NewAuditLogRepository(db)
	_, err = repo.ClearAllWithTrace(context.Background(), &service.AuditLog{Action: service.AuditActionAuditLogClear})

	require.Error(t, err)
	require.Contains(t, err.Error(), "insert audit clear trace")
	require.NoError(t, mock.ExpectationsWereMet())
}
