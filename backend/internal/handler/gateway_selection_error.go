package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type publicGatewayError struct {
	Status  int
	Type    string
	Message string
}

func publicGatewayAccountSelectionError(err error, requestedModel string) publicGatewayError {
	if isClaudeCodeOnlySelectionError(err) {
		return publicGatewayError{
			Status:  http.StatusForbidden,
			Type:    "permission_error",
			Message: "This group is restricted to Claude Code clients (/v1/messages only)",
		}
	}

	model := strings.TrimSpace(requestedModel)
	if isGatewayModelUnsupportedError(err) && model != "" {
		return publicGatewayError{
			Status:  http.StatusServiceUnavailable,
			Type:    "api_error",
			Message: fmt.Sprintf("Model %s is not supported in this group", model),
		}
	}

	if isGatewayAccountUnavailableError(err) {
		return publicGatewayError{
			Status:  http.StatusServiceUnavailable,
			Type:    "api_error",
			Message: "No available accounts",
		}
	}

	return publicGatewayError{
		Status:  http.StatusServiceUnavailable,
		Type:    "api_error",
		Message: "Service temporarily unavailable",
	}
}

func isClaudeCodeOnlySelectionError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, service.ErrClaudeCodeOnly) {
		return true
	}

	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return msg != "" && strings.Contains(msg, strings.ToLower(service.ErrClaudeCodeOnly.Error()))
}

func isGatewayModelUnsupportedError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return msg != "" && strings.Contains(msg, "supporting model:")
}

func isGatewayAccountUnavailableError(err error) bool {
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

	return strings.Contains(msg, "no available accounts")
}
