package service

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const (
	kiroDefaultChatTriggerType = "MANUAL"
	kiroToolNameMaxLen         = 63
)

type anthropicKiroRequest struct {
	Model        string             `json:"model"`
	Metadata     anthropicMetadata  `json:"metadata"`
	System       json.RawMessage    `json:"system"`
	Messages     []anthropicMessage `json:"messages"`
	Tools        []anthropicTool    `json:"tools"`
	Thinking     map[string]any     `json:"thinking"`
	OutputConfig map[string]any     `json:"output_config"`
}

type anthropicMetadata struct {
	UserID string `json:"user_id"`
}

type anthropicMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

type anthropicContentBlock struct {
	Type      string                `json:"type"`
	Text      string                `json:"text"`
	Thinking  string                `json:"thinking"`
	ID        string                `json:"id"`
	Name      string                `json:"name"`
	Input     map[string]any        `json:"input"`
	ToolUseID string                `json:"tool_use_id"`
	IsError   bool                  `json:"is_error"`
	Content   any                   `json:"content"`
	Source    *anthropicImageSource `json:"source"`
}

type anthropicImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type kiroGenerateRequest struct {
	ConversationState kiroConversationState `json:"conversationState"`
	ProfileARN        string                `json:"profileArn,omitempty"`
}

type kiroConversationState struct {
	AgentContinuationID string               `json:"agentContinuationId,omitempty"`
	AgentTaskType       string               `json:"agentTaskType,omitempty"`
	ChatTriggerType     string               `json:"chatTriggerType,omitempty"`
	CurrentMessage      kiroCurrentMessage   `json:"currentMessage"`
	ConversationID      string               `json:"conversationId"`
	History             []kiroHistoryMessage `json:"history,omitempty"`
}

type kiroCurrentMessage struct {
	UserInputMessage kiroUserInputMessage `json:"userInputMessage"`
}

type kiroUserInputMessage struct {
	UserInputMessageContext kiroUserInputMessageContext `json:"userInputMessageContext"`
	Content                 string                      `json:"content"`
	ModelID                 string                      `json:"modelId"`
	Images                  []kiroImage                 `json:"images,omitempty"`
	Origin                  string                      `json:"origin,omitempty"`
}

type kiroUserInputMessageContext struct {
	ToolResults []kiroToolResult `json:"toolResults,omitempty"`
	Tools       []kiroTool       `json:"tools,omitempty"`
}

type kiroTool struct {
	ToolSpecification kiroToolSpecification `json:"toolSpecification"`
}

type kiroToolSpecification struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	InputSchema kiroInputSchemaWrap `json:"inputSchema"`
}

type kiroInputSchemaWrap struct {
	JSON map[string]any `json:"json"`
}

type kiroToolResult struct {
	ToolUseID string           `json:"toolUseId"`
	Content   []map[string]any `json:"content"`
	Status    string           `json:"status,omitempty"`
	IsError   bool             `json:"isError,omitempty"`
}

type kiroImage struct {
	Format string          `json:"format"`
	Source kiroImageSource `json:"source"`
}

type kiroImageSource struct {
	Bytes string `json:"bytes"`
}

type kiroHistoryMessage struct {
	UserInputMessage         *kiroHistoryUserMessage      `json:"userInputMessage,omitempty"`
	AssistantResponseMessage *kiroHistoryAssistantMessage `json:"assistantResponseMessage,omitempty"`
}

type kiroHistoryUserMessage struct {
	Content                 string                      `json:"content"`
	ModelID                 string                      `json:"modelId"`
	Origin                  string                      `json:"origin,omitempty"`
	Images                  []kiroImage                 `json:"images,omitempty"`
	UserInputMessageContext kiroUserInputMessageContext `json:"userInputMessageContext,omitempty"`
}

type kiroHistoryAssistantMessage struct {
	Content  string        `json:"content"`
	ToolUses []kiroToolUse `json:"toolUses,omitempty"`
}

type kiroToolUse struct {
	ToolUseID string         `json:"toolUseId"`
	Name      string         `json:"name"`
	Input     map[string]any `json:"input"`
}

type kiroGeneratePayload struct {
	Request     kiroGenerateRequest
	Raw         []byte
	MappedModel string
	ToolNameMap map[string]string
}

func BuildKiroGenerateRequest(body []byte, account *Account) ([]byte, string, error) {
	payload, err := buildKiroGeneratePayload(body, account)
	if err != nil {
		return nil, "", err
	}
	return payload.Raw, payload.MappedModel, nil
}

func buildKiroGeneratePayload(body []byte, account *Account) (*kiroGeneratePayload, error) {
	if account == nil || !account.IsKiro() {
		return nil, fmt.Errorf("not a kiro account")
	}

	var req anthropicKiroRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}
	if len(req.Messages) == 0 {
		return nil, fmt.Errorf("kiro messages are required")
	}

	mappedModel := strings.TrimSpace(account.GetMappedModel(req.Model))
	if mappedModel == "" {
		mappedModel = strings.TrimSpace(req.Model)
	}
	if mappedModel == "" {
		return nil, fmt.Errorf("kiro model is required")
	}

	messages := trimKiroPrefillMessages(req.Messages)
	if len(messages) == 0 {
		return nil, fmt.Errorf("kiro current message must be user role")
	}

	conversationID := uuid.NewString()
	if parsed := ParseMetadataUserID(strings.TrimSpace(req.Metadata.UserID)); parsed != nil && strings.TrimSpace(parsed.SessionID) != "" {
		conversationID = strings.TrimSpace(parsed.SessionID)
	}

	toolNameMap := make(map[string]string)
	thinkingPrefix := buildKiroThinkingPrefix(req.Thinking, req.OutputConfig)
	history := buildKiroHistory(req.System, messages[:len(messages)-1], mappedModel, toolNameMap, thinkingPrefix)

	currentText, currentImages, currentToolResults := parseKiroUserMessageContent(messages[len(messages)-1].Content)
	currentToolResults, orphanedToolUseIDs := validateKiroToolPairing(history, currentToolResults)
	removeOrphanedKiroToolUses(history, orphanedToolUseIDs)

	tools := convertKiroTools(req.Tools, toolNameMap)
	tools = appendMissingHistoryPlaceholderTools(tools, history)

	currentContent := strings.TrimSpace(currentText)
	if currentContent == "" && len(currentToolResults) > 0 {
		currentContent = " "
	}

	request := kiroGenerateRequest{
		ConversationState: kiroConversationState{
			AgentContinuationID: stableKiroContinuationID(conversationID),
			AgentTaskType:       "vibe",
			ChatTriggerType:     kiroDefaultChatTriggerType,
			CurrentMessage: kiroCurrentMessage{
				UserInputMessage: kiroUserInputMessage{
					UserInputMessageContext: kiroUserInputMessageContext{
						ToolResults: currentToolResults,
						Tools:       tools,
					},
					Content: currentContent,
					ModelID: mappedModel,
					Images:  currentImages,
					Origin:  "AI_EDITOR",
				},
			},
			ConversationID: conversationID,
			History:        history,
		},
		ProfileARN: strings.TrimSpace(account.GetCredential("profile_arn")),
	}

	raw, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	return &kiroGeneratePayload{
		Request:     request,
		Raw:         raw,
		MappedModel: mappedModel,
		ToolNameMap: toolNameMap,
	}, nil
}

func trimKiroPrefillMessages(messages []anthropicMessage) []anthropicMessage {
	trimmed := messages
	for len(trimmed) > 0 && !strings.EqualFold(strings.TrimSpace(trimmed[len(trimmed)-1].Role), "user") {
		trimmed = trimmed[:len(trimmed)-1]
	}
	return trimmed
}

func buildKiroHistory(
	systemRaw json.RawMessage,
	messages []anthropicMessage,
	modelID string,
	toolNameMap map[string]string,
	thinkingPrefix string,
) []kiroHistoryMessage {
	history := make([]kiroHistoryMessage, 0, len(messages)+2)

	systemContent := extractKiroSystemText(systemRaw)
	if systemContent != "" || thinkingPrefix != "" {
		if thinkingPrefix != "" && !hasKiroThinkingTags(systemContent) {
			if systemContent != "" {
				systemContent = thinkingPrefix + "\n" + systemContent
			} else {
				systemContent = thinkingPrefix
			}
		}
		if systemContent != "" {
			history = append(history,
				kiroHistoryMessage{
					UserInputMessage: &kiroHistoryUserMessage{
						Content: systemContent,
						ModelID: modelID,
						Origin:  "AI_EDITOR",
					},
				},
				kiroHistoryMessage{
					AssistantResponseMessage: &kiroHistoryAssistantMessage{
						Content: "I will follow these instructions.",
					},
				},
			)
		}
	}

	for _, message := range messages {
		switch strings.ToLower(strings.TrimSpace(message.Role)) {
		case "assistant":
			history = append(history, kiroHistoryMessage{
				AssistantResponseMessage: buildKiroAssistantHistoryMessage(message.Content, toolNameMap),
			})
		default:
			text, images, toolResults := parseKiroUserMessageContent(message.Content)
			text = strings.TrimSpace(text)
			if text == "" && len(toolResults) > 0 {
				text = " "
			}
			history = append(history, kiroHistoryMessage{
				UserInputMessage: &kiroHistoryUserMessage{
					Content: text,
					ModelID: modelID,
					Origin:  "AI_EDITOR",
					Images:  images,
					UserInputMessageContext: kiroUserInputMessageContext{
						ToolResults: toolResults,
					},
				},
			})
		}
	}

	return history
}

func buildKiroThinkingPrefix(thinking map[string]any, outputConfig map[string]any) string {
	if len(thinking) == 0 {
		return ""
	}

	thinkingType, _ := thinking["type"].(string)
	thinkingType = strings.TrimSpace(thinkingType)
	switch thinkingType {
	case "enabled":
		budget := intFromAny(thinking["budget_tokens"])
		if budget <= 0 {
			budget = 20000
		}
		return fmt.Sprintf("<thinking_mode>enabled</thinking_mode><max_thinking_length>%d</max_thinking_length>", budget)
	case "adaptive":
		effort := "high"
		if len(outputConfig) > 0 {
			if rawEffort, ok := outputConfig["effort"].(string); ok && strings.TrimSpace(rawEffort) != "" {
				effort = strings.TrimSpace(rawEffort)
			}
		}
		return fmt.Sprintf("<thinking_mode>adaptive</thinking_mode><thinking_effort>%s</thinking_effort>", effort)
	default:
		return ""
	}
}

func hasKiroThinkingTags(content string) bool {
	content = strings.TrimSpace(content)
	return strings.Contains(content, "<thinking_mode>") || strings.Contains(content, "<max_thinking_length>")
}

func extractKiroSystemText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return strings.TrimSpace(asString)
	}

	var asBlocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &asBlocks); err != nil {
		return ""
	}

	parts := make([]string, 0, len(asBlocks))
	for _, block := range asBlocks {
		if strings.EqualFold(block.Type, "text") && strings.TrimSpace(block.Text) != "" {
			parts = append(parts, strings.TrimSpace(block.Text))
		}
	}
	return strings.Join(parts, "\n")
}

func buildKiroAssistantHistoryMessage(contentRaw json.RawMessage, toolNameMap map[string]string) *kiroHistoryAssistantMessage {
	content, toolUses := parseKiroAssistantMessageContent(contentRaw, toolNameMap)
	if content == "" && len(toolUses) > 0 {
		content = " "
	}
	return &kiroHistoryAssistantMessage{
		Content:  content,
		ToolUses: toolUses,
	}
}

func parseKiroUserMessageContent(raw json.RawMessage) (string, []kiroImage, []kiroToolResult) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil, nil
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return strings.TrimSpace(asString), nil, nil
	}

	var blocks []anthropicContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return "", nil, nil
	}

	textParts := make([]string, 0, len(blocks))
	images := make([]kiroImage, 0)
	toolResults := make([]kiroToolResult, 0)

	for _, block := range blocks {
		switch strings.ToLower(strings.TrimSpace(block.Type)) {
		case "text":
			if strings.TrimSpace(block.Text) != "" {
				textParts = append(textParts, strings.TrimSpace(block.Text))
			}
		case "image":
			if img := convertAnthropicImageBlock(block.Source); img != nil {
				images = append(images, *img)
			}
		case "tool_result":
			if strings.TrimSpace(block.ToolUseID) == "" {
				continue
			}
			text := extractKiroToolResultText(block.Content)
			result := kiroToolResult{
				ToolUseID: strings.TrimSpace(block.ToolUseID),
				Content:   []map[string]any{{"text": text}},
			}
			if block.IsError {
				result.Status = "error"
				result.IsError = true
			} else {
				result.Status = "success"
			}
			toolResults = append(toolResults, result)
		}
	}

	return strings.Join(textParts, "\n"), images, toolResults
}

func parseKiroAssistantMessageContent(raw json.RawMessage, toolNameMap map[string]string) (string, []kiroToolUse) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return strings.TrimSpace(asString), nil
	}

	var blocks []anthropicContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return "", nil
	}

	textParts := make([]string, 0, len(blocks))
	thinkingParts := make([]string, 0)
	toolUses := make([]kiroToolUse, 0)

	for _, block := range blocks {
		switch strings.ToLower(strings.TrimSpace(block.Type)) {
		case "thinking":
			if strings.TrimSpace(block.Thinking) != "" {
				thinkingParts = append(thinkingParts, strings.TrimSpace(block.Thinking))
			}
		case "text":
			if strings.TrimSpace(block.Text) != "" {
				textParts = append(textParts, strings.TrimSpace(block.Text))
			}
		case "tool_use":
			if strings.TrimSpace(block.ID) == "" || strings.TrimSpace(block.Name) == "" {
				continue
			}
			input := block.Input
			if input == nil {
				input = map[string]any{}
			}
			toolUses = append(toolUses, kiroToolUse{
				ToolUseID: strings.TrimSpace(block.ID),
				Name:      mapKiroToolName(strings.TrimSpace(block.Name), toolNameMap),
				Input:     input,
			})
		}
	}

	content := strings.Join(textParts, "\n")
	if len(thinkingParts) > 0 {
		thinking := strings.Join(thinkingParts, "\n")
		if content != "" {
			content = "<thinking>" + thinking + "</thinking>\n\n" + content
		} else {
			content = "<thinking>" + thinking + "</thinking>"
		}
	}

	return content, toolUses
}

func convertAnthropicImageBlock(source *anthropicImageSource) *kiroImage {
	if source == nil {
		return nil
	}
	if !strings.EqualFold(strings.TrimSpace(source.Type), "base64") || strings.TrimSpace(source.Data) == "" {
		return nil
	}
	if _, err := base64.StdEncoding.DecodeString(source.Data); err != nil {
		return nil
	}

	format := ""
	switch strings.ToLower(strings.TrimSpace(source.MediaType)) {
	case "image/jpeg":
		format = "jpeg"
	case "image/png":
		format = "png"
	case "image/gif":
		format = "gif"
	case "image/webp":
		format = "webp"
	default:
		return nil
	}

	return &kiroImage{
		Format: format,
		Source: kiroImageSource{Bytes: source.Data},
	}
}

func extractKiroToolResultText(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if itemMap, ok := item.(map[string]any); ok {
				if text, ok := itemMap["text"].(string); ok && strings.TrimSpace(text) != "" {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, "\n")
	case map[string]any:
		if text, ok := v["text"].(string); ok {
			return text
		}
		raw, _ := json.Marshal(v)
		return string(raw)
	default:
		raw, _ := json.Marshal(v)
		return string(raw)
	}
}

func convertKiroTools(tools []anthropicTool, toolNameMap map[string]string) []kiroTool {
	if len(tools) == 0 {
		return nil
	}

	result := make([]kiroTool, 0, len(tools))
	for _, tool := range tools {
		name := strings.TrimSpace(tool.Name)
		if name == "" {
			continue
		}
		result = append(result, kiroTool{
			ToolSpecification: kiroToolSpecification{
				Name:        mapKiroToolName(name, toolNameMap),
				Description: strings.TrimSpace(tool.Description),
				InputSchema: kiroInputSchemaWrap{
					JSON: normalizeKiroInputSchema(tool.InputSchema),
				},
			},
		})
	}
	return result
}

func normalizeKiroInputSchema(schema map[string]any) map[string]any {
	if len(schema) == 0 {
		return map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"required":             []string{},
			"additionalProperties": true,
		}
	}

	out := make(map[string]any, len(schema)+2)
	for key, value := range schema {
		out[key] = value
	}
	if strings.TrimSpace(stringValue(out["type"])) == "" {
		out["type"] = "object"
	}
	if _, ok := out["properties"].(map[string]any); !ok {
		out["properties"] = map[string]any{}
	}
	switch required := out["required"].(type) {
	case []any:
		values := make([]string, 0, len(required))
		for _, item := range required {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				values = append(values, s)
			}
		}
		out["required"] = values
	case []string:
	default:
		out["required"] = []string{}
	}
	if _, exists := out["additionalProperties"]; !exists {
		out["additionalProperties"] = true
	}
	return out
}

func appendMissingHistoryPlaceholderTools(tools []kiroTool, history []kiroHistoryMessage) []kiroTool {
	existing := make(map[string]struct{}, len(tools))
	for _, tool := range tools {
		existing[strings.ToLower(strings.TrimSpace(tool.ToolSpecification.Name))] = struct{}{}
	}

	for _, name := range collectKiroHistoryToolNames(history) {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" {
			continue
		}
		if _, ok := existing[key]; ok {
			continue
		}
		existing[key] = struct{}{}
		tools = append(tools, createPlaceholderKiroTool(name))
	}
	return tools
}

func collectKiroHistoryToolNames(history []kiroHistoryMessage) []string {
	names := make([]string, 0)
	seen := make(map[string]struct{})
	for _, message := range history {
		if message.AssistantResponseMessage == nil {
			continue
		}
		for _, toolUse := range message.AssistantResponseMessage.ToolUses {
			name := strings.TrimSpace(toolUse.Name)
			if name == "" {
				continue
			}
			key := strings.ToLower(name)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			names = append(names, name)
		}
	}
	return names
}

func createPlaceholderKiroTool(name string) kiroTool {
	return kiroTool{
		ToolSpecification: kiroToolSpecification{
			Name:        name,
			Description: "Tool used in conversation history",
			InputSchema: kiroInputSchemaWrap{
				JSON: map[string]any{
					"type":                 "object",
					"properties":           map[string]any{},
					"required":             []string{},
					"additionalProperties": true,
				},
			},
		},
	}
}

func validateKiroToolPairing(history []kiroHistoryMessage, currentResults []kiroToolResult) ([]kiroToolResult, map[string]struct{}) {
	historyToolUseIDs := make(map[string]struct{})
	historyToolResultIDs := make(map[string]struct{})
	for _, message := range history {
		if message.AssistantResponseMessage != nil {
			for _, toolUse := range message.AssistantResponseMessage.ToolUses {
				if strings.TrimSpace(toolUse.ToolUseID) != "" {
					historyToolUseIDs[toolUse.ToolUseID] = struct{}{}
				}
			}
		}
		if message.UserInputMessage != nil {
			for _, result := range message.UserInputMessage.UserInputMessageContext.ToolResults {
				if strings.TrimSpace(result.ToolUseID) != "" {
					historyToolResultIDs[result.ToolUseID] = struct{}{}
				}
			}
		}
	}

	unpaired := make(map[string]struct{})
	for toolUseID := range historyToolUseIDs {
		if _, ok := historyToolResultIDs[toolUseID]; !ok {
			unpaired[toolUseID] = struct{}{}
		}
	}

	filtered := make([]kiroToolResult, 0, len(currentResults))
	for _, result := range currentResults {
		if _, ok := unpaired[result.ToolUseID]; ok {
			filtered = append(filtered, result)
			delete(unpaired, result.ToolUseID)
		}
	}
	return filtered, unpaired
}

func removeOrphanedKiroToolUses(history []kiroHistoryMessage, orphanedToolUseIDs map[string]struct{}) {
	if len(orphanedToolUseIDs) == 0 {
		return
	}

	for idx := range history {
		if history[idx].AssistantResponseMessage == nil {
			continue
		}
		toolUses := history[idx].AssistantResponseMessage.ToolUses
		filtered := toolUses[:0]
		for _, toolUse := range toolUses {
			if _, ok := orphanedToolUseIDs[toolUse.ToolUseID]; ok {
				continue
			}
			filtered = append(filtered, toolUse)
		}
		if len(filtered) == 0 {
			history[idx].AssistantResponseMessage.ToolUses = nil
		} else {
			history[idx].AssistantResponseMessage.ToolUses = filtered
		}
	}
}

func mapKiroToolName(name string, toolNameMap map[string]string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if len(name) <= kiroToolNameMaxLen {
		return name
	}
	short := shortenKiroToolName(name)
	if toolNameMap != nil {
		toolNameMap[short] = name
	}
	return short
}

func restoreKiroToolName(name string, toolNameMap map[string]string) string {
	if toolNameMap == nil {
		return name
	}
	if original, ok := toolNameMap[name]; ok && strings.TrimSpace(original) != "" {
		return original
	}
	return name
}

func shortenKiroToolName(name string) string {
	sum := sha256.Sum256([]byte(name))
	hash := hex.EncodeToString(sum[:])
	suffix := hash[:8]
	prefixMax := kiroToolNameMaxLen - 1 - len(suffix)

	runes := []rune(name)
	if len(runes) > prefixMax {
		runes = runes[:prefixMax]
	}
	return string(runes) + "_" + suffix
}

func stableKiroContinuationID(conversationID string) string {
	sum := sha256.Sum256([]byte(conversationID))
	return hex.EncodeToString(sum[:16])
}

func stringValue(v any) string {
	s, _ := v.(string)
	return s
}

func intFromAny(v any) int {
	switch value := v.(type) {
	case float64:
		return int(value)
	case float32:
		return int(value)
	case int:
		return value
	case int64:
		return int(value)
	case json.Number:
		if parsed, err := value.Int64(); err == nil {
			return int(parsed)
		}
	}
	return 0
}
