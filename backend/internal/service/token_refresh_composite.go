package service

import (
	"context"
	"fmt"
	"time"
)

// compositeOAuthRefreshExecutor 让共享同一平台的不同账号类型保留各自的
// OAuth 刷新实现，同时让后台调度器继续按平台维护一套并发与限速状态。
type compositeOAuthRefreshExecutor struct {
	executors []OAuthRefreshExecutor
}

func newCompositeOAuthRefreshExecutor(executors ...OAuthRefreshExecutor) *compositeOAuthRefreshExecutor {
	filtered := make([]OAuthRefreshExecutor, 0, len(executors))
	for _, executor := range executors {
		if executor != nil {
			filtered = append(filtered, executor)
		}
	}
	return &compositeOAuthRefreshExecutor{executors: filtered}
}

func (e *compositeOAuthRefreshExecutor) executorFor(account *Account) OAuthRefreshExecutor {
	if e == nil {
		return nil
	}
	for _, executor := range e.executors {
		if executor != nil && executor.CanRefresh(account) {
			return executor
		}
	}
	return nil
}

func (e *compositeOAuthRefreshExecutor) CanRefresh(account *Account) bool {
	return e.executorFor(account) != nil
}

func (e *compositeOAuthRefreshExecutor) NeedsRefresh(account *Account, refreshWindow time.Duration) bool {
	executor := e.executorFor(account)
	return executor != nil && executor.NeedsRefresh(account, refreshWindow)
}

func (e *compositeOAuthRefreshExecutor) Refresh(ctx context.Context, account *Account) (map[string]any, error) {
	executor := e.executorFor(account)
	if executor == nil {
		return nil, fmt.Errorf("未找到适用于该账号的 OAuth 刷新执行器")
	}
	return executor.Refresh(ctx, account)
}

func (e *compositeOAuthRefreshExecutor) CacheKey(account *Account) string {
	executor := e.executorFor(account)
	if executor == nil {
		return ""
	}
	return executor.CacheKey(account)
}
