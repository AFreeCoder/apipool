package service

import (
	"context"
	"log/slog"
	"time"
)

func (s *TokenRefreshService) maybeRunOpenAIPlanSync(ctx context.Context, accounts []Account) {
	if s == nil || s.cfg == nil || s.accountRepo == nil || s.privacyClientFactory == nil {
		return
	}
	if s.cfg.PlanSyncIntervalMinutes <= 0 {
		return
	}

	interval := time.Duration(s.cfg.PlanSyncIntervalMinutes) * time.Minute
	now := time.Now()
	if !s.lastOpenAIPlanSyncAt.IsZero() && now.Sub(s.lastOpenAIPlanSyncAt) < interval {
		return
	}
	s.lastOpenAIPlanSyncAt = now

	s.runOpenAIPlanSyncSweep(ctx, accounts, now.UTC())
}

func (s *TokenRefreshService) runOpenAIPlanSyncSweep(ctx context.Context, accounts []Account, checkedAt time.Time) {
	total := 0
	verified := 0
	changed := 0
	free := 0
	autoUnsched := 0

	for i := range accounts {
		account := &accounts[i]
		if account.Platform != PlatformOpenAI || account.Type != AccountTypeOAuth {
			continue
		}

		total++
		previous := normalizeOpenAIPlanType(account.GetCredential("plan_type"))
		proxyURL := resolveProxyURLSilently(ctx, s.proxyRepo, account.ProxyID)
		result := syncOpenAIPlanTypeDetailed(ctx, s.privacyClientFactory, s.accountRepo, account, proxyURL)
		if !result.Verified {
			continue
		}

		verified++
		if result.PlanType != previous {
			changed++
		}
		if result.PlanType == "free" {
			free++
		}
		if s.persistOpenAIPlanSweepState(ctx, account, result.PlanType, checkedAt) {
			autoUnsched++
		}
	}

	if total == 0 {
		return
	}

	logLevel := slog.LevelInfo
	if verified == 0 && autoUnsched == 0 {
		logLevel = slog.LevelDebug
	}
	slog.Log(ctx, logLevel, "openai_plan_type_sweep_completed",
		"total", total,
		"verified", verified,
		"changed", changed,
		"free", free,
		"auto_unscheduled", autoUnsched,
	)
}

func (s *TokenRefreshService) persistOpenAIPlanSweepState(ctx context.Context, account *Account, planType string, checkedAt time.Time) bool {
	updates := map[string]any{
		openAIPlanLastCheckedAtExtraKey:   checkedAt.Format(time.RFC3339),
		openAIPlanLastCheckSourceExtraKey: openAIPlanCheckSource,
	}

	autoUnsched := false
	if planType == "free" {
		consecutiveFree := accountExtraInt(account, openAIPlanConsecutiveFreeExtraKey) + 1
		updates[openAIPlanConsecutiveFreeExtraKey] = consecutiveFree

		threshold := s.cfg.FreeUnscheduleThreshold
		if threshold <= 0 {
			threshold = 2
		}
		if s.cfg.AutoUnscheduleFree && consecutiveFree >= threshold && account.Schedulable {
			if err := s.accountRepo.SetSchedulable(ctx, account.ID, false); err != nil {
				slog.Warn("openai_plan_type_auto_unschedule_failed", "account_id", account.ID, "error", err.Error())
			} else {
				autoUnsched = true
				account.Schedulable = false
				updates[openAIPlanAutoUnschedAtExtraKey] = checkedAt.Format(time.RFC3339)
				slog.Warn("openai_plan_type_auto_unscheduled",
					"account_id", account.ID,
					"plan_type", planType,
					"consecutive_free", consecutiveFree,
				)
			}
		}
	} else {
		updates[openAIPlanConsecutiveFreeExtraKey] = 0
	}

	if err := s.accountRepo.UpdateExtra(ctx, account.ID, updates); err != nil {
		slog.Warn("openai_plan_type_sweep_state_update_failed", "account_id", account.ID, "error", err.Error())
		return autoUnsched
	}

	applyAccountExtraUpdates(account, updates)
	return autoUnsched
}
