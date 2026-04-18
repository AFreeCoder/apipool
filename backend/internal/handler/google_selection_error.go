package handler

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func publicGoogleAccountSelectionMessage(err error, requestedModel string) string {
	model := strings.TrimSpace(requestedModel)
	if isGatewayModelUnsupportedError(err) && model != "" {
		return fmt.Sprintf("Model %s is not supported in this group", model)
	}

	if isGoogleAccountUnavailableError(err) {
		return "No available Gemini accounts"
	}

	return "Service temporarily unavailable"
}

func isGoogleAccountUnavailableError(err error) bool {
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

	return strings.Contains(msg, "no available gemini accounts") ||
		strings.Contains(msg, "no available accounts")
}
