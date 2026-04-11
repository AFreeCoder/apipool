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
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	contentBlocks, usage, stopReason, err := parseKiroNonStreamingResponseBody(body, resp.Header, requestModel)
	if err != nil {
		return nil, err
	}

	if stopReason == "" {
		stopReason = "end_turn"
	}

	if c != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
		c.JSON(http.StatusOK, gin.H{
			"type":          "message",
			"role":          "assistant",
			"model":         requestModel,
			"content":       contentBlocks,
			"stop_reason":   stopReason,
			"stop_sequence": nil,
			"usage": gin.H{
				"input_tokens":  usage.InputTokens,
				"output_tokens": usage.OutputTokens,
			},
		})
	}

	result := &ForwardResult{
		RequestID: resp.Header.Get("x-amzn-requestid"),
		Usage:     *usage,
		Model:     requestModel,
		Stream:    false,
		Duration:  time.Since(startTime),
	}
	if upstreamModel != requestModel {
		result.UpstreamModel = upstreamModel
	}
	return result, nil
}

func parseKiroNonStreamingResponseBody(body []byte, header http.Header, requestModel string) ([]map[string]any, *ClaudeUsage, string, error) {
	var raw struct {
		Content string `json:"content"`
		Usage   struct {
			InputTokens  int `json:"inputTokens"`
			OutputTokens int `json:"outputTokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &raw); err == nil && (strings.TrimSpace(raw.Content) != "" || raw.Usage.InputTokens != 0 || raw.Usage.OutputTokens != 0) {
		contentBlocks := []map[string]any{
			{"type": "text", "text": raw.Content},
		}
		return contentBlocks, &ClaudeUsage{
			InputTokens:  raw.Usage.InputTokens,
			OutputTokens: raw.Usage.OutputTokens,
		}, "end_turn", nil
	}

	events, err := decodeKiroResponseEvents(body, header)
	if err != nil {
		return nil, nil, "", err
	}
	return aggregateKiroEvents(events, requestModel)
}

func decodeKiroResponseEvents(body []byte, header http.Header) ([]map[string]any, error) {
	if isKiroEventStreamContentType(header) {
		decoder := newKiroEventStreamDecoder(bytes.NewReader(body))
		events := make([]map[string]any, 0, 8)
		for {
			frame, err := decoder.Decode()
			if err != nil {
				if err == io.EOF {
					return events, nil
				}
				return nil, err
			}
			event, err := frameToKiroEventMap(frame)
			if err != nil {
				return nil, err
			}
			if event != nil {
				events = append(events, event)
			}
		}
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	events := make([]map[string]any, 0, 8)
	for {
		var event map[string]any
		if err := decoder.Decode(&event); err != nil {
			if err == io.EOF {
				return events, nil
			}
			return nil, err
		}
		events = append(events, event)
	}
}

func aggregateKiroEvents(events []map[string]any, requestModel string) ([]map[string]any, *ClaudeUsage, string, error) {
	textBuilder := strings.Builder{}
	toolBuffers := make(map[string]string)
	toolUses := make([]map[string]any, 0)
	stopReason := "end_turn"
	usage := &ClaudeUsage{}

	for _, event := range events {
		switch strings.TrimSpace(stringFromAny(event["type"])) {
		case "assistantResponseEvent":
			content := stringFromAny(event["content"])
			textBuilder.WriteString(content)
			usage.OutputTokens += estimateKiroOutputTokens(content)
		case "toolUseEvent":
			toolUseID := strings.TrimSpace(stringFromAny(firstNonNil(event["toolUseId"], event["tool_use_id"])))
			name := strings.TrimSpace(stringFromAny(event["name"]))
			if toolUseID == "" || name == "" {
				continue
			}

			stop, _ := event["stop"].(bool)
			if partial := toolInputPartialJSON(event["input"]); partial != "" && !isStructuredJSON(event["input"]) {
				toolBuffers[toolUseID] += partial
			}

			if structured, ok := event["input"].(map[string]any); ok {
				toolUses = append(toolUses, map[string]any{
					"type":  "tool_use",
					"id":    toolUseID,
					"name":  name,
					"input": structured,
				})
				stopReason = "tool_use"
				continue
			}

			if stop {
				input := map[string]any{}
				if raw := strings.TrimSpace(toolBuffers[toolUseID]); raw != "" {
					_ = json.Unmarshal([]byte(raw), &input)
				}
				toolUses = append(toolUses, map[string]any{
					"type":  "tool_use",
					"id":    toolUseID,
					"name":  name,
					"input": input,
				})
				stopReason = "tool_use"
			}
		case "contextUsageEvent":
			if percent, ok := floatFromAny(event["contextUsagePercentage"]); ok {
				usage.InputTokens = int(percent * float64(contextWindowForKiroModel(requestModel)) / 100.0)
				if percent >= 100 {
					stopReason = "model_context_window_exceeded"
				}
			}
		case "exception":
			exceptionType := strings.TrimSpace(stringFromAny(firstNonNil(event["exceptionType"], event["exception_type"])))
			message := strings.TrimSpace(stringFromAny(event["message"]))
			if exceptionType == "ContentLengthExceededException" {
				stopReason = "max_tokens"
				continue
			}
			if message == "" {
				message = "unknown exception"
			}
			return nil, nil, "", fmt.Errorf("kiro exception: %s", message)
		}
	}

	contentBlocks := make([]map[string]any, 0, 1+len(toolUses))
	if text := textBuilder.String(); text != "" {
		contentBlocks = append(contentBlocks, map[string]any{
			"type": "text",
			"text": text,
		})
	}
	contentBlocks = append(contentBlocks, toolUses...)
	return contentBlocks, usage, stopReason, nil
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

	adapter := NewKiroStreamAdapter(requestModel)
	var firstTokenMs *int
	processEvent := func(event map[string]any) error {
		chunks, _, err := adapter.ProcessEvent(event)
		if err != nil {
			return err
		}
		if len(chunks) > 0 && firstTokenMs == nil {
			ms := int(time.Since(startTime).Milliseconds())
			firstTokenMs = &ms
		}
		for _, chunk := range chunks {
			if _, err := io.WriteString(c.Writer, chunk); err != nil {
				return err
			}
			flusher.Flush()
		}
		return nil
	}

	if isKiroEventStreamContentType(resp.Header) {
		decoder := newKiroEventStreamDecoder(resp.Body)
		for {
			frame, err := decoder.Decode()
			if err != nil {
				if err == io.EOF {
					break
				}
				return nil, err
			}
			event, err := frameToKiroEventMap(frame)
			if err != nil {
				return nil, err
			}
			if event == nil {
				continue
			}
			if err := processEvent(event); err != nil {
				return nil, err
			}
		}
	} else {
		decoder := json.NewDecoder(resp.Body)
		for {
			var event map[string]any
			if err := decoder.Decode(&event); err != nil {
				if err == io.EOF {
					break
				}
				return nil, err
			}
			if err := processEvent(event); err != nil {
				return nil, err
			}
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
