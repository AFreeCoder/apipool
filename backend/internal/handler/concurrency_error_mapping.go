package handler

import (
	"errors"
	"fmt"
	"net/http"
)

func concurrencyLimitMessage(slotType string) string {
	if slotType == "" {
		slotType = "request"
	}
	return fmt.Sprintf("Concurrency limit exceeded for %s, please retry later", slotType)
}

func concurrencyServiceUnavailableMessage() string {
	return "Concurrency system unavailable (cache or network issue), please retry later"
}

func mapConcurrencyAcquireError(err error, slotType string) (status int, errType, message string) {
	var concurrencyErr *ConcurrencyError
	if errors.As(err, &concurrencyErr) {
		if slotType == "" {
			slotType = concurrencyErr.SlotType
		}
		return http.StatusTooManyRequests, "rate_limit_error", concurrencyLimitMessage(slotType)
	}
	return http.StatusServiceUnavailable, "api_error", concurrencyServiceUnavailableMessage()
}

func mapGeminiConcurrencyAcquireError(err error, slotType string) (status int, message string) {
	status, _, message = mapConcurrencyAcquireError(err, slotType)
	return status, message
}
