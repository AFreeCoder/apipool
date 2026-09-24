//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAdminProxyPartialUpdatePreservesOmittedSettings(t *testing.T) {
	for _, input := range []*UpdateProxyInput{{Status: "inactive"}, {Name: "renamed"}, {Host: "new.example"}} {
		expiry := time.Now().Add(24 * time.Hour)
		backup := int64(10)
		original := &Proxy{ID: 9, Name: "original", Host: "old.example", Status: StatusActive, ExpiresAt: &expiry, FallbackMode: FallbackModeProxy, BackupProxyID: &backup, ExpiryWarnDays: 7}
		repo := &updatingProxyRepoStub{proxyRepoStub: &proxyRepoStub{}, proxy: original}
		svc := &adminServiceImpl{proxyRepo: repo}
		got, err := svc.UpdateProxy(context.Background(), 9, input)
		require.NoError(t, err)
		require.Equal(t, original.ExpiresAt, got.ExpiresAt)
		require.Equal(t, original.FallbackMode, got.FallbackMode)
		require.Equal(t, original.BackupProxyID, got.BackupProxyID)
		require.Equal(t, original.ExpiryWarnDays, got.ExpiryWarnDays)
		require.Equal(t, 1, repo.updateCalls)
	}
}

func TestAdminProxyPartialUpdateClearsAndSetsSettings(t *testing.T) {
	expiry := time.Now().Add(time.Hour)
	backup := int64(10)
	zero := 0
	repo := &updatingProxyRepoStub{proxyRepoStub: &proxyRepoStub{}, proxy: &Proxy{ID: 9, ExpiresAt: &expiry, FallbackMode: FallbackModeProxy, BackupProxyID: &backup, ExpiryWarnDays: 7}}
	svc := &adminServiceImpl{proxyRepo: repo}
	got, err := svc.UpdateProxy(context.Background(), 9, &UpdateProxyInput{
		ExpiresAt: NullableTimeUpdate{Set: true}, FallbackMode: ptrString(FallbackModeNone), BackupProxyID: NullableInt64Update{Set: true}, ExpiryWarnDays: &zero,
	})
	require.NoError(t, err)
	require.Nil(t, got.ExpiresAt)
	require.Nil(t, got.BackupProxyID)
	require.Equal(t, FallbackModeNone, got.FallbackMode)
	require.Zero(t, got.ExpiryWarnDays)
	days := 3
	got, err = svc.UpdateProxy(context.Background(), 9, &UpdateProxyInput{
		ExpiresAt: NullableTimeUpdate{Set: true, Value: &expiry}, FallbackMode: ptrString(FallbackModeProxy), BackupProxyID: NullableInt64Update{Set: true, Value: &backup}, ExpiryWarnDays: &days,
	})
	require.NoError(t, err)
	require.Equal(t, &expiry, got.ExpiresAt)
	require.Equal(t, &backup, got.BackupProxyID)
	require.Equal(t, FallbackModeProxy, got.FallbackMode)
	require.Equal(t, days, got.ExpiryWarnDays)
}

func TestAdminProxyPartialUpdateValidatesMergedFallback(t *testing.T) {
	backup := int64(10)
	self := int64(9)
	negative := -1
	for _, tc := range []struct {
		name      string
		input     UpdateProxyInput
		wantError bool
	}{
		{name: "reuse existing backup", input: UpdateProxyInput{FallbackMode: ptrString(FallbackModeProxy)}},
		{name: "clear required backup", input: UpdateProxyInput{BackupProxyID: NullableInt64Update{Set: true}}, wantError: true},
		{name: "self backup", input: UpdateProxyInput{BackupProxyID: NullableInt64Update{Set: true, Value: &self}}, wantError: true},
		{name: "negative warning", input: UpdateProxyInput{ExpiryWarnDays: &negative}, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &updatingProxyRepoStub{proxyRepoStub: &proxyRepoStub{}, proxy: &Proxy{ID: 9, FallbackMode: FallbackModeProxy, BackupProxyID: &backup}}
			svc := &adminServiceImpl{proxyRepo: repo}
			_, err := svc.UpdateProxy(context.Background(), 9, &tc.input)
			if tc.wantError {
				require.Error(t, err)
				require.Zero(t, repo.updateCalls)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
