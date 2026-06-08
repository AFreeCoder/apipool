//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type proxyRepoStubForAdminProxy struct {
	proxyRepoStub

	nextID  int64
	proxies map[int64]Proxy
	created []Proxy
	updated []Proxy
}

func (s *proxyRepoStubForAdminProxy) Create(_ context.Context, proxy *Proxy) error {
	if s.proxies == nil {
		s.proxies = make(map[int64]Proxy)
	}
	if proxy.ID == 0 && s.nextID != 0 {
		proxy.ID = s.nextID
	}
	s.proxies[proxy.ID] = *proxy
	s.created = append(s.created, *proxy)
	return nil
}

func (s *proxyRepoStubForAdminProxy) GetByID(_ context.Context, id int64) (*Proxy, error) {
	if s.proxies == nil {
		return nil, ErrProxyNotFound
	}
	proxy, ok := s.proxies[id]
	if !ok {
		return nil, ErrProxyNotFound
	}
	return &proxy, nil
}

func (s *proxyRepoStubForAdminProxy) Update(_ context.Context, proxy *Proxy) error {
	if s.proxies == nil {
		s.proxies = make(map[int64]Proxy)
	}
	s.proxies[proxy.ID] = *proxy
	s.updated = append(s.updated, *proxy)
	return nil
}

func TestAdminService_UpdateProxy_StatusPatchPreservesFallbackFields(t *testing.T) {
	expiresAt := time.Now().Add(24 * time.Hour).Truncate(time.Second)
	backupID := int64(2)
	repo := &proxyRepoStubForAdminProxy{
		proxies: map[int64]Proxy{
			1: {
				ID:             1,
				Name:           "primary",
				Protocol:       "http",
				Host:           "127.0.0.1",
				Port:           8080,
				Status:         StatusActive,
				ExpiresAt:      &expiresAt,
				FallbackMode:   FallbackModeProxy,
				BackupProxyID:  &backupID,
				ExpiryWarnDays: 14,
			},
			2: {
				ID:       2,
				Name:     "backup",
				Protocol: "http",
				Host:     "127.0.0.2",
				Port:     8080,
				Status:   StatusActive,
			},
		},
	}
	svc := &adminServiceImpl{proxyRepo: repo}

	updated, err := svc.UpdateProxy(context.Background(), 1, &UpdateProxyInput{Status: StatusDisabled})

	require.NoError(t, err)
	require.Equal(t, StatusDisabled, updated.Status)
	require.NotNil(t, updated.ExpiresAt)
	require.True(t, updated.ExpiresAt.Equal(expiresAt))
	require.Equal(t, FallbackModeProxy, updated.FallbackMode)
	require.NotNil(t, updated.BackupProxyID)
	require.Equal(t, backupID, *updated.BackupProxyID)
	require.Equal(t, 14, updated.ExpiryWarnDays)
	require.Len(t, repo.updated, 1)
}

func TestAdminService_UpdateProxy_RejectsUnavailableBackup(t *testing.T) {
	mode := FallbackModeProxy
	backupID := int64(2)
	repo := &proxyRepoStubForAdminProxy{
		proxies: map[int64]Proxy{
			1: {
				ID:           1,
				Name:         "primary",
				Protocol:     "http",
				Host:         "127.0.0.1",
				Port:         8080,
				Status:       StatusActive,
				FallbackMode: FallbackModeNone,
			},
			2: {
				ID:       2,
				Name:     "backup",
				Protocol: "http",
				Host:     "127.0.0.2",
				Port:     8080,
				Status:   StatusDisabled,
			},
		},
	}
	svc := &adminServiceImpl{proxyRepo: repo}

	_, err := svc.UpdateProxy(context.Background(), 1, &UpdateProxyInput{
		FallbackMode:  &mode,
		BackupProxyID: NullableInt64Update{Set: true, Value: &backupID},
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "PROXY_BACKUP_UNAVAILABLE")
	require.Empty(t, repo.updated)
}

func TestAdminService_CreateProxy_DefaultsExpiryWarnDays(t *testing.T) {
	repo := &proxyRepoStubForAdminProxy{nextID: 10}
	svc := &adminServiceImpl{proxyRepo: repo}

	created, err := svc.CreateProxy(context.Background(), &CreateProxyInput{
		Name:     "proxy",
		Protocol: "http",
		Host:     "127.0.0.1",
		Port:     8080,
	})

	require.NoError(t, err)
	require.Equal(t, 10, int(created.ID))
	require.Equal(t, 7, created.ExpiryWarnDays)
	require.Len(t, repo.created, 1)
}
