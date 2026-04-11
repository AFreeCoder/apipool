package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (s *AccountTestService) testKiroAccountConnection(c *gin.Context, account *Account) error {
	ctx := c.Request.Context()

	token, err := s.kiroTokenProvider.GetAccessToken(ctx, account)
	if err != nil {
		return s.sendErrorAndEnd(c, err.Error())
	}

	creds, err := ParseKiroCredentials(account)
	if err != nil {
		return s.sendErrorAndEnd(c, err.Error())
	}

	reqURL := fmt.Sprintf(
		"https://q.%s.amazonaws.com/getUsageLimits?origin=AI_EDITOR&resourceType=AGENTIC_REQUEST",
		creds.EffectiveAPIRegion(),
	)
	if creds.ProfileARN != "" {
		u, _ := url.Parse(reqURL)
		q := u.Query()
		q.Set("profileArn", creds.ProfileARN)
		u.RawQuery = q.Encode()
		reqURL = u.String()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return s.sendErrorAndEnd(c, err.Error())
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("x-amz-user-agent", kiroIDCSDKUserAgent)
	req.Header.Set("user-agent", fmt.Sprintf("KiroIDE-dev-%s", creds.MachineID))
	req.Header.Set("host", req.URL.Host)
	req.Header.Set("amz-sdk-invocation-id", uuid.NewString())
	req.Header.Set("amz-sdk-request", "attempt=1; max=4")
	req.Header.Set("Connection", kiroRefreshConnectionHeader)

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	resp, err := s.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, nil)
	if err != nil {
		return s.sendErrorAndEnd(c, err.Error())
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusBadRequest {
		return s.sendErrorAndEnd(c, fmt.Sprintf("kiro upstream error: %d", resp.StatusCode))
	}

	var payload map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&payload)
	s.sendEvent(c, TestEvent{Type: "status", Status: "kiro_ok", Success: true, Data: payload})
	s.sendEvent(c, TestEvent{Type: "test_complete", Success: true})
	return nil
}
