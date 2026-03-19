package handler

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const (
	openAIErrorCodeServiceUnavailable          = "service_unavailable"
	openAIErrorCodeNoAvailableAccounts         = "no_available_accounts"
	openAIErrorCodeNoAvailableAccountsForModel = "no_available_accounts_for_model"
)

type publicOpenAIError struct {
	Code    string
	Message string
}

func publicOpenAIAccountSelectionError(err error, requestedModel string) publicOpenAIError {
	if isOpenAIAccountUnavailableError(err) {
		model := strings.TrimSpace(requestedModel)
		if model != "" {
			return publicOpenAIError{
				Code:    openAIErrorCodeNoAvailableAccountsForModel,
				Message: fmt.Sprintf("No available accounts supporting model: %s", model),
			}
		}
		return publicOpenAIError{
			Code:    openAIErrorCodeNoAvailableAccounts,
			Message: "No available accounts",
		}
	}
	return publicOpenAIError{
		Code:    openAIErrorCodeServiceUnavailable,
		Message: "Service temporarily unavailable",
	}
}

// publicOpenAIAccountSelectionMessage converts account selection failures into a
// user-facing message without exposing internal account diagnostics.
func publicOpenAIAccountSelectionMessage(err error, requestedModel string) string {
	return publicOpenAIAccountSelectionError(err, requestedModel).Message
}

func isOpenAIAccountUnavailableError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, service.ErrNoAvailableAccounts) {
		return true
	}

	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	if msg == "" {
		return false
	}

	return strings.Contains(msg, "no available openai accounts") ||
		strings.Contains(msg, "no available accounts supporting model") ||
		strings.Contains(msg, "supporting model:")
}
