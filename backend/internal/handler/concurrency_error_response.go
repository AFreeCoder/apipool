package handler

import (
	"context"
	"errors"
	"net/http"
)

const statusClientClosedRequest = 499

func concurrencyErrorResponse(err error, slotType string) (int, string, string) {
	var concurrencyErr *ConcurrencyError
	if errors.As(err, &concurrencyErr) {
		if concurrencyErr.SlotType != "" {
			slotType = concurrencyErr.SlotType
		}
		return http.StatusTooManyRequests, "rate_limit_error", concurrencyLimitMessage(slotType)
	}

	if errors.Is(err, context.Canceled) {
		return statusClientClosedRequest, "api_error", "context canceled"
	}

	return http.StatusServiceUnavailable, "api_error", concurrencyServiceUnavailableMessage()
}
