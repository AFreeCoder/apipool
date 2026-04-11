package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (s *GatewayService) forwardKiro(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	parsed *ParsedRequest,
	startTime time.Time,
) (*ForwardResult, error) {
	if parsed == nil {
		return nil, fmt.Errorf("parse request: empty request")
	}

	token, tokenType, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, err
	}
	if tokenType != "kiro" {
		return nil, fmt.Errorf("expected kiro token, got %s", tokenType)
	}

	creds, err := ParseKiroCredentials(account)
	if err != nil {
		return nil, err
	}

	requestBody, mappedModel, err := BuildKiroGenerateRequest(parsed.Body, account)
	if err != nil {
		return nil, err
	}

	targetURL := fmt.Sprintf("https://q.%s.amazonaws.com/generateAssistantResponse", creds.EffectiveAPIRegion())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(requestBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("x-amzn-codewhisperer-optout", "true")
	req.Header.Set("x-amzn-kiro-agent-mode", "vibe")
	req.Header.Set("x-amz-user-agent", kiroIDCSDKUserAgent)
	req.Header.Set("user-agent", fmt.Sprintf("KiroIDE-dev-%s", creds.MachineID))
	req.Header.Set("host", req.URL.Host)
	req.Header.Set("amz-sdk-invocation-id", uuid.NewString())
	req.Header.Set("amz-sdk-request", kiroIDCSDKRequestHeader)
	req.Header.Set("Connection", kiroRefreshConnectionHeader)

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	var tlsProfile *tlsfingerprint.Profile
	if s.tlsFPProfileService != nil {
		tlsProfile = s.tlsFPProfileService.ResolveTLSProfile(account)
	}

	resp, err := s.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, tlsProfile)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		return nil, fmt.Errorf("kiro upstream error: %d %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if parsed.OnUpstreamAccepted != nil {
		parsed.OnUpstreamAccepted()
	}

	if parsed.Stream {
		return s.handleKiroStreamingResponse(ctx, c, account, resp, parsed.Model, mappedModel, startTime)
	}
	return s.handleKiroNonStreamingResponse(c, resp, parsed.Model, mappedModel, startTime)
}

func (s *GatewayService) handleKiroNonStreamingResponse(
	c *gin.Context,
	resp *http.Response,
	requestModel string,
	upstreamModel string,
	startTime time.Time,
) (*ForwardResult, error) {
	var raw struct {
		Content string `json:"content"`
		Usage   struct {
			InputTokens  int `json:"inputTokens"`
			OutputTokens int `json:"outputTokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	if c != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
		c.JSON(http.StatusOK, gin.H{
			"type":  "message",
			"role":  "assistant",
			"model": requestModel,
			"content": []gin.H{
				{"type": "text", "text": raw.Content},
			},
			"stop_reason":   "end_turn",
			"stop_sequence": nil,
			"usage": gin.H{
				"input_tokens":  raw.Usage.InputTokens,
				"output_tokens": raw.Usage.OutputTokens,
			},
		})
	}

	result := &ForwardResult{
		RequestID: resp.Header.Get("x-amzn-requestid"),
		Usage: ClaudeUsage{
			InputTokens:  raw.Usage.InputTokens,
			OutputTokens: raw.Usage.OutputTokens,
		},
		Model:    requestModel,
		Stream:   false,
		Duration: time.Since(startTime),
	}
	if upstreamModel != requestModel {
		result.UpstreamModel = upstreamModel
	}
	return result, nil
}

func (s *GatewayService) handleKiroStreamingResponse(
	_ context.Context,
	c *gin.Context,
	_ *Account,
	resp *http.Response,
	requestModel string,
	upstreamModel string,
	startTime time.Time,
) (*ForwardResult, error) {
	if c == nil {
		return nil, fmt.Errorf("kiro streaming requires gin context")
	}
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported")
	}

	responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	if requestID := resp.Header.Get("x-amzn-requestid"); requestID != "" {
		c.Header("x-request-id", requestID)
	}

	decoder := json.NewDecoder(resp.Body)
	adapter := NewKiroStreamAdapter(requestModel)
	var firstTokenMs *int

	for {
		var event map[string]any
		if err := decoder.Decode(&event); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		chunks, _, err := adapter.ProcessEvent(event)
		if err != nil {
			return nil, err
		}
		if len(chunks) > 0 && firstTokenMs == nil {
			ms := int(time.Since(startTime).Milliseconds())
			firstTokenMs = &ms
		}
		for _, chunk := range chunks {
			if _, err := io.WriteString(c.Writer, chunk); err != nil {
				return nil, err
			}
			flusher.Flush()
		}
	}

	finalChunks, usage, err := adapter.Finalize()
	if err != nil {
		return nil, err
	}
	for _, chunk := range finalChunks {
		if _, err := io.WriteString(c.Writer, chunk); err != nil {
			return nil, err
		}
		flusher.Flush()
	}

	result := &ForwardResult{
		RequestID:    resp.Header.Get("x-amzn-requestid"),
		Usage:        *usage,
		Model:        requestModel,
		Stream:       true,
		Duration:     time.Since(startTime),
		FirstTokenMs: firstTokenMs,
	}
	if upstreamModel != requestModel {
		result.UpstreamModel = upstreamModel
	}
	return result, nil
}
