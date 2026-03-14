package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"
)

const chatgptAccountsCheckURL = "https://chatgpt.com/backend-api/accounts/check/v4-2023-04-27"

const (
	openAIPlanLastCheckedAtExtraKey   = "openai_plan_last_checked_at"
	openAIPlanLastCheckSourceExtraKey = "openai_plan_last_check_source"
	openAIPlanConsecutiveFreeExtraKey = "openai_plan_consecutive_free"
	openAIPlanAutoUnschedAtExtraKey   = "openai_plan_auto_unscheduled_at"
	openAIPlanCheckSource             = "accounts_check"
)

// chatgptAccountsCheckEndpoint 是实际使用的端点 URL，测试时可覆盖。
var chatgptAccountsCheckEndpoint = chatgptAccountsCheckURL

type openAIPlanTypeSyncResult struct {
	PlanType string
	Verified bool
}

// syncOpenAIPlanTypeCore 是 plan_type 同步的唯一底层实现。
// 调用 ChatGPT accounts/check 端点，按 chatgpt_account_id 精确匹配提取 plan_type，
// 通过 MergeCredentials 原子更新到 DB，并同步更新传入的 account 内存对象。
// 失败不阻塞，返回获取到的 plan_type（空字符串表示跳过或失败）。
func syncOpenAIPlanTypeCore(
	ctx context.Context,
	clientFactory PrivacyClientFactory,
	accountRepo AccountRepository,
	account *Account,
	proxyURL string,
) string {
	return syncOpenAIPlanTypeDetailed(ctx, clientFactory, accountRepo, account, proxyURL).PlanType
}

func syncOpenAIPlanTypeDetailed(
	ctx context.Context,
	clientFactory PrivacyClientFactory,
	accountRepo AccountRepository,
	account *Account,
	proxyURL string,
) openAIPlanTypeSyncResult {
	// 前置检查：仅对 OpenAI OAuth 账号执行
	if account == nil || accountRepo == nil {
		return openAIPlanTypeSyncResult{}
	}
	if account.Platform != PlatformOpenAI || account.Type != AccountTypeOAuth {
		return openAIPlanTypeSyncResult{}
	}
	if clientFactory == nil {
		return openAIPlanTypeSyncResult{}
	}

	accessToken, _ := account.Credentials["access_token"].(string)
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return openAIPlanTypeSyncResult{}
	}

	// 需要 chatgpt_account_id 才能精确匹配
	chatgptAccountID, _ := account.Credentials["chatgpt_account_id"].(string)
	chatgptAccountID = strings.TrimSpace(chatgptAccountID)
	if chatgptAccountID == "" {
		slog.Debug("openai_plan_type_sync_skip_no_account_id", "account_id", account.ID)
		return openAIPlanTypeSyncResult{}
	}

	requestCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	client, err := clientFactory(proxyURL)
	if err != nil {
		slog.Warn("openai_plan_type_client_error", "account_id", account.ID, "error", err.Error())
		return openAIPlanTypeSyncResult{}
	}

	resp, err := client.R().
		SetContext(requestCtx).
		SetHeader("Authorization", "Bearer "+accessToken).
		SetHeader("Origin", "https://chatgpt.com").
		SetHeader("Referer", "https://chatgpt.com/").
		Get(chatgptAccountsCheckEndpoint)

	if err != nil {
		slog.Warn("openai_plan_type_request_error", "account_id", account.ID, "error", err.Error())
		return openAIPlanTypeSyncResult{}
	}

	if !resp.IsSuccessState() {
		slog.Warn("openai_plan_type_request_failed", "account_id", account.ID, "status", resp.StatusCode)
		return openAIPlanTypeSyncResult{}
	}

	// 解析响应
	var result struct {
		Accounts map[string]struct {
			Account struct {
				PlanType string `json:"plan_type"`
			} `json:"account"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal(resp.Bytes(), &result); err != nil {
		slog.Warn("openai_plan_type_parse_error", "account_id", account.ID, "error", err.Error())
		return openAIPlanTypeSyncResult{}
	}

	// 按 chatgpt_account_id 精确匹配
	entry, ok := result.Accounts[chatgptAccountID]
	if !ok {
		slog.Warn("openai_plan_type_account_not_found", "account_id", account.ID, "chatgpt_account_id", chatgptAccountID)
		return openAIPlanTypeSyncResult{}
	}

	planType := normalizeOpenAIPlanType(entry.Account.PlanType)
	if planType == "" {
		return openAIPlanTypeSyncResult{}
	}

	// 如果 plan_type 未变化，不写库
	existingPlanType := normalizeOpenAIPlanType(account.GetCredential("plan_type"))
	if planType == existingPlanType {
		return openAIPlanTypeSyncResult{PlanType: planType, Verified: true}
	}

	// 先 DB 成功，再同步更新内存对象
	if err := accountRepo.MergeCredentials(ctx, account.ID, map[string]any{"plan_type": planType}); err != nil {
		slog.Warn("openai_plan_type_merge_failed", "account_id", account.ID, "error", err.Error())
		return openAIPlanTypeSyncResult{}
	}

	// DB 写入成功，更新内存
	if account.Credentials == nil {
		account.Credentials = make(map[string]any)
	}
	account.Credentials["plan_type"] = planType

	slog.Info("openai_plan_type_synced",
		"account_id", account.ID,
		"plan_type", planType,
		"previous", existingPlanType,
	)
	return openAIPlanTypeSyncResult{PlanType: planType, Verified: true}
}

func resolveProxyURLSilently(ctx context.Context, proxyRepo ProxyRepository, proxyID *int64) string {
	if proxyRepo == nil || proxyID == nil {
		return ""
	}
	proxy, err := proxyRepo.GetByID(ctx, *proxyID)
	if err != nil || proxy == nil {
		return ""
	}
	return proxy.URL()
}

func normalizeOpenAIPlanType(planType string) string {
	return strings.ToLower(strings.TrimSpace(planType))
}

func accountExtraInt(account *Account, key string) int {
	if account == nil || account.Extra == nil {
		return 0
	}
	switch v := account.Extra[key].(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float32:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

func applyAccountExtraUpdates(account *Account, updates map[string]any) {
	if account == nil || len(updates) == 0 {
		return
	}
	if account.Extra == nil {
		account.Extra = make(map[string]any, len(updates))
	}
	for key, value := range updates {
		account.Extra[key] = value
	}
}
