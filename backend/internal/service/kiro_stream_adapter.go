package service

import (
	"encoding/json"
	"fmt"
	"strings"
)

const kiroDefaultContextWindow = 200_000

type kiroToolBlockState struct {
	Index int
	Open  bool
}

type KiroStreamAdapter struct {
	model          string
	messageStarted bool
	textBlockOpen  bool
	textBlockIndex int
	nextBlockIndex int
	stopReason     string
	inputTokens    int
	outputTokens   int
	toolBlocks     map[string]*kiroToolBlockState
	toolNameMap    map[string]string
}

func NewKiroStreamAdapter(model string) *KiroStreamAdapter {
	return NewKiroStreamAdapterWithToolNameMap(model, nil)
}

func NewKiroStreamAdapterWithToolNameMap(model string, toolNameMap map[string]string) *KiroStreamAdapter {
	return &KiroStreamAdapter{
		model:       strings.TrimSpace(model),
		toolBlocks:  make(map[string]*kiroToolBlockState),
		toolNameMap: toolNameMap,
	}
}

func (a *KiroStreamAdapter) ProcessEvent(event map[string]any) ([]string, *ClaudeUsage, error) {
	eventType := strings.TrimSpace(stringFromAny(event["type"]))
	switch eventType {
	case "assistantResponseEvent":
		return a.processAssistantResponse(event)
	case "toolUseEvent":
		return a.processToolUse(event)
	case "contextUsageEvent":
		a.processContextUsage(event)
		return nil, nil, nil
	case "exception":
		exceptionType := strings.TrimSpace(stringFromAny(event["exceptionType"]))
		if exceptionType == "" {
			exceptionType = strings.TrimSpace(stringFromAny(event["exception_type"]))
		}
		message := strings.TrimSpace(stringFromAny(event["message"]))
		if exceptionType == "ContentLengthExceededException" {
			a.stopReason = "max_tokens"
			return nil, nil, nil
		}
		if message == "" {
			message = "unknown exception"
		}
		return nil, nil, fmt.Errorf("kiro exception: %s", message)
	default:
		return nil, nil, nil
	}
}

func (a *KiroStreamAdapter) Finalize() ([]string, *ClaudeUsage, error) {
	events := make([]string, 0, 4)
	events = append(events, a.ensureMessageStart()...)
	if a.textBlockOpen {
		events = append(events, formatKiroSSE("content_block_stop", map[string]any{
			"type":  "content_block_stop",
			"index": a.textBlockIndex,
		}))
		a.textBlockOpen = false
	}
	for _, state := range a.toolBlocks {
		if state != nil && state.Open {
			events = append(events, formatKiroSSE("content_block_stop", map[string]any{
				"type":  "content_block_stop",
				"index": state.Index,
			}))
			state.Open = false
		}
	}
	if a.stopReason == "" {
		a.stopReason = "end_turn"
	}
	usage := &ClaudeUsage{
		InputTokens:  a.inputTokens,
		OutputTokens: a.outputTokens,
	}
	events = append(events, formatKiroSSE("message_delta", map[string]any{
		"type": "message_delta",
		"delta": map[string]any{
			"stop_reason": a.stopReason,
		},
		"usage": map[string]any{
			"input_tokens":  usage.InputTokens,
			"output_tokens": usage.OutputTokens,
		},
	}))
	events = append(events, formatKiroSSE("message_stop", map[string]any{
		"type": "message_stop",
	}))
	return events, usage, nil
}

func (a *KiroStreamAdapter) processAssistantResponse(event map[string]any) ([]string, *ClaudeUsage, error) {
	content := stringFromAny(event["content"])
	if content == "" {
		return nil, nil, nil
	}

	events := make([]string, 0, 3)
	events = append(events, a.ensureMessageStart()...)
	if !a.textBlockOpen {
		a.textBlockIndex = a.nextBlockIndex
		a.nextBlockIndex++
		a.textBlockOpen = true
		events = append(events, formatKiroSSE("content_block_start", map[string]any{
			"type":  "content_block_start",
			"index": a.textBlockIndex,
			"content_block": map[string]any{
				"type": "text",
				"text": "",
			},
		}))
	}
	events = append(events, formatKiroSSE("content_block_delta", map[string]any{
		"type":  "content_block_delta",
		"index": a.textBlockIndex,
		"delta": map[string]any{
			"type": "text_delta",
			"text": content,
		},
	}))
	a.outputTokens += estimateKiroOutputTokens(content)
	return events, nil, nil
}

func (a *KiroStreamAdapter) processToolUse(event map[string]any) ([]string, *ClaudeUsage, error) {
	toolUseID := strings.TrimSpace(stringFromAny(firstNonNil(event["toolUseId"], event["tool_use_id"])))
	name := strings.TrimSpace(stringFromAny(event["name"]))
	name = restoreKiroToolName(name, a.toolNameMap)
	if toolUseID == "" || name == "" {
		return nil, nil, nil
	}

	events := make([]string, 0, 4)
	events = append(events, a.ensureMessageStart()...)
	if a.textBlockOpen {
		events = append(events, formatKiroSSE("content_block_stop", map[string]any{
			"type":  "content_block_stop",
			"index": a.textBlockIndex,
		}))
		a.textBlockOpen = false
	}

	state, exists := a.toolBlocks[toolUseID]
	if !exists {
		startInput := map[string]any{}
		if inputMap, ok := event["input"].(map[string]any); ok {
			startInput = inputMap
		}
		state = &kiroToolBlockState{Index: a.nextBlockIndex, Open: true}
		a.toolBlocks[toolUseID] = state
		a.nextBlockIndex++
		events = append(events, formatKiroSSE("content_block_start", map[string]any{
			"type":  "content_block_start",
			"index": state.Index,
			"content_block": map[string]any{
				"type":  "tool_use",
				"id":    toolUseID,
				"name":  name,
				"input": startInput,
			},
		}))
	}

	partialJSON := ""
	if !isStructuredJSON(event["input"]) {
		partialJSON = toolInputPartialJSON(event["input"])
	}
	if partialJSON != "" {
		events = append(events, formatKiroSSE("content_block_delta", map[string]any{
			"type":  "content_block_delta",
			"index": state.Index,
			"delta": map[string]any{
				"type":         "input_json_delta",
				"partial_json": partialJSON,
			},
		}))
		a.outputTokens += estimateKiroOutputTokens(partialJSON)
	}

	stop, _ := event["stop"].(bool)
	if stop {
		events = append(events, formatKiroSSE("content_block_stop", map[string]any{
			"type":  "content_block_stop",
			"index": state.Index,
		}))
		state.Open = false
	}

	a.stopReason = "tool_use"
	return events, nil, nil
}

func (a *KiroStreamAdapter) processContextUsage(event map[string]any) {
	percent, ok := floatFromAny(event["contextUsagePercentage"])
	if !ok {
		return
	}
	a.inputTokens = int(percent * float64(contextWindowForKiroModel(a.model)) / 100.0)
	if percent >= 100 {
		a.stopReason = "model_context_window_exceeded"
	}
}

func (a *KiroStreamAdapter) ensureMessageStart() []string {
	if a.messageStarted {
		return nil
	}
	a.messageStarted = true
	return []string{formatKiroSSE("message_start", map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":            "msg_kiro",
			"type":          "message",
			"role":          "assistant",
			"model":         a.model,
			"content":       []any{},
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage": map[string]any{
				"input_tokens":  a.inputTokens,
				"output_tokens": 0,
			},
		},
	})}
}

func formatKiroSSE(eventName string, payload any) string {
	body, _ := json.Marshal(payload)
	return fmt.Sprintf("event: %s\ndata: %s\n\n", eventName, body)
}

func estimateKiroOutputTokens(text string) int {
	if strings.TrimSpace(text) == "" {
		return 0
	}
	runes := len([]rune(text))
	return (runes + 3) / 4
}

func contextWindowForKiroModel(model string) int {
	lower := strings.ToLower(strings.TrimSpace(model))
	if (strings.Contains(lower, "sonnet") || strings.Contains(lower, "opus")) &&
		(strings.Contains(lower, "4-6") || strings.Contains(lower, "4.6")) {
		return 1_000_000
	}
	return kiroDefaultContextWindow
}

func toolInputPartialJSON(v any) string {
	switch value := v.(type) {
	case string:
		return value
	case map[string]any, []any:
		raw, _ := json.Marshal(value)
		return string(raw)
	default:
		return ""
	}
}

func isStructuredJSON(v any) bool {
	switch v.(type) {
	case map[string]any, []any:
		return true
	default:
		return false
	}
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func stringFromAny(v any) string {
	s, _ := v.(string)
	return s
}

func floatFromAny(v any) (float64, bool) {
	switch value := v.(type) {
	case float64:
		return value, true
	case float32:
		return float64(value), true
	case int:
		return float64(value), true
	case int64:
		return float64(value), true
	default:
		return 0, false
	}
}
