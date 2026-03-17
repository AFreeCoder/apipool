package service

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

type openAIErrorPayload struct {
	Error struct {
		Code    string `json:"code"`
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

type openAIErrorFields struct {
	Code    string
	Type    string
	Message string
}

func extractOpenAIErrorFields(responseBody []byte) openAIErrorFields {
	fields := openAIErrorFields{
		Message: sanitizeUpstreamErrorMessage(strings.TrimSpace(extractUpstreamErrorMessage(responseBody))),
	}
	if len(responseBody) == 0 {
		return fields
	}

	var payload openAIErrorPayload
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return fields
	}

	fields.Code = strings.TrimSpace(payload.Error.Code)
	fields.Type = strings.TrimSpace(payload.Error.Type)
	if message := sanitizeUpstreamErrorMessage(strings.TrimSpace(payload.Error.Message)); message != "" {
		fields.Message = message
	}
	return fields
}

// persistOpenAIOAuthStatusFromHTTPError 将 OpenAI OAuth 探测/测试里的终态认证错误同步为账号 error 状态。
// 正式转发链路已经会处理大部分情况；这里补的是管理员手动检查场景，避免账号被暂停后永远不再落库。
func persistOpenAIOAuthStatusFromHTTPError(ctx context.Context, repo AccountRepository, account *Account, statusCode int, responseBody []byte) {
	fields := extractOpenAIErrorFields(responseBody)
	persistOpenAIOAuthStatusFromRaw(ctx, repo, account, statusCode, fields.Code, fields.Type, fields.Message)
}

func persistOpenAIOAuthStatusFromRaw(ctx context.Context, repo AccountRepository, account *Account, statusCode int, codeRaw, errTypeRaw, msgRaw string) {
	if repo == nil || account == nil || !account.IsOpenAIOAuth() {
		return
	}

	msg := buildOpenAIOAuthErrorMessage(statusCode, codeRaw, errTypeRaw, msgRaw)
	if msg == "" {
		return
	}

	if err := repo.SetError(ctx, account.ID, msg); err != nil {
		return
	}

	account.Status = StatusError
	account.ErrorMessage = msg
}

func buildOpenAIOAuthHTTPErrorMessage(statusCode int, responseBody []byte) string {
	fields := extractOpenAIErrorFields(responseBody)
	return buildOpenAIOAuthErrorMessage(statusCode, fields.Code, fields.Type, fields.Message)
}

func buildOpenAIOAuthErrorMessage(statusCode int, codeRaw, errTypeRaw, msgRaw string) string {
	upstreamMsg := sanitizeUpstreamErrorMessage(strings.TrimSpace(msgRaw))

	if isOpenAIOAuthAccountDeactivated(codeRaw, errTypeRaw, upstreamMsg) {
		if upstreamMsg != "" {
			return "Account deactivated (401): " + upstreamMsg
		}
		return "Account deactivated (401): upstream account is deactivated"
	}

	if isOpenAIOAuthTerminalForbidden(statusCode, codeRaw, errTypeRaw, upstreamMsg) {
		if upstreamMsg != "" {
			return "Access forbidden (403): " + upstreamMsg
		}
		return "Access forbidden (403): account may be suspended or lack permissions"
	}

	return ""
}

func isOpenAIOAuthTerminalForbidden(statusCode int, codeRaw, errTypeRaw, upstreamMsg string) bool {
	code := strings.ToLower(strings.TrimSpace(codeRaw))
	errType := strings.ToLower(strings.TrimSpace(errTypeRaw))
	msg := strings.ToLower(strings.TrimSpace(upstreamMsg))

	if statusCode != http.StatusForbidden &&
		!strings.Contains(code, "forbidden") &&
		!strings.Contains(errType, "permission") {
		return false
	}

	terminalMarkers := []string{
		"account has been deactivated",
		"openai account has been deactivated",
		"account deactivated",
		"deactivated due to policy violation",
		"policy violation",
		"terms of service violation",
		"terms-of-service violation",
		"account suspended",
		"has been suspended",
		"suspended for",
		"suspended due to",
		"account banned",
		"has been banned",
	}
	for _, marker := range terminalMarkers {
		if strings.Contains(msg, marker) {
			return true
		}
	}

	terminalCodeMarkers := []string{
		"account_deactivated",
		"suspend",
		"suspended",
		"violation",
		"banned",
		"disabled",
	}
	for _, marker := range terminalCodeMarkers {
		if strings.Contains(code, marker) {
			return true
		}
	}
	return false
}

func isOpenAIOAuthAccountDeactivated(codeRaw, errTypeRaw, upstreamMsg string) bool {
	_ = errTypeRaw

	code := strings.ToLower(strings.TrimSpace(codeRaw))
	if code == "account_deactivated" {
		return true
	}

	msg := strings.ToLower(strings.TrimSpace(upstreamMsg))
	return strings.Contains(msg, "account has been deactivated") ||
		(strings.Contains(msg, "openai account") && strings.Contains(msg, "has been deactivated")) ||
		strings.Contains(msg, "account deactivated")
}
