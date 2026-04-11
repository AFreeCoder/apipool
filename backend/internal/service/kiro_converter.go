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

const kiroDefaultChatTriggerType = "MANUAL"

type anthropicKiroRequest struct {
	Model        string             `json:"model"`
	Metadata     anthropicMetadata  `json:"metadata"`
	System       json.RawMessage    `json:"system"`
	Messages     []anthropicMessage `json:"messages"`
	Tools        []anthropicTool    `json:"tools"`
	Thinking     any                `json:"thinking"`
	OutputConfig any                `json:"output_config"`
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

func BuildKiroGenerateRequest(body []byte, account *Account) ([]byte, string, error) {
	if account == nil || !account.IsKiro() {
		return nil, "", fmt.Errorf("not a kiro account")
	}

	var req anthropicKiroRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, "", err
	}
	if len(req.Messages) == 0 {
		return nil, "", fmt.Errorf("kiro messages are required")
	}

	mappedModel := strings.TrimSpace(account.GetMappedModel(req.Model))
	if mappedModel == "" {
		mappedModel = strings.TrimSpace(req.Model)
	}
	if mappedModel == "" {
		return nil, "", fmt.Errorf("kiro model is required")
	}

	messages := req.Messages
	for len(messages) > 0 && !strings.EqualFold(strings.TrimSpace(messages[len(messages)-1].Role), "user") {
		messages = messages[:len(messages)-1]
	}
	if len(messages) == 0 {
		return nil, "", fmt.Errorf("kiro current message must be user role")
	}

	conversationID := uuid.NewString()
	if parsed := ParseMetadataUserID(strings.TrimSpace(req.Metadata.UserID)); parsed != nil && strings.TrimSpace(parsed.SessionID) != "" {
		conversationID = strings.TrimSpace(parsed.SessionID)
	}

	history := buildKiroHistory(req.System, messages[:len(messages)-1], mappedModel)

	currentText, currentImages, currentToolResults := parseKiroUserMessageContent(messages[len(messages)-1].Content)
	currentMessage := kiroCurrentMessage{
		UserInputMessage: kiroUserInputMessage{
			UserInputMessageContext: kiroUserInputMessageContext{
				Tools:       convertKiroTools(req.Tools),
				ToolResults: currentToolResults,
			},
			Content: strings.TrimSpace(currentText),
			ModelID: mappedModel,
			Images:  currentImages,
			Origin:  "AI_EDITOR",
		},
	}

	if currentMessage.UserInputMessage.Content == "" && len(currentToolResults) > 0 {
		currentMessage.UserInputMessage.Content = " "
	}

	payload := kiroGenerateRequest{
		ConversationState: kiroConversationState{
			AgentContinuationID: stableKiroContinuationID(conversationID),
			AgentTaskType:       "vibe",
			ChatTriggerType:     kiroDefaultChatTriggerType,
			CurrentMessage:      currentMessage,
			ConversationID:      conversationID,
			History:             history,
		},
		ProfileARN: strings.TrimSpace(account.GetCredential("profile_arn")),
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}
	return raw, mappedModel, nil
}

func buildKiroHistory(systemRaw json.RawMessage, messages []anthropicMessage, modelID string) []kiroHistoryMessage {
	history := make([]kiroHistoryMessage, 0, len(messages)+2)

	if systemText := extractKiroSystemText(systemRaw); systemText != "" {
		history = append(history,
			kiroHistoryMessage{
				UserInputMessage: &kiroHistoryUserMessage{
					Content: systemText,
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

	for _, message := range messages {
		role := strings.ToLower(strings.TrimSpace(message.Role))
		switch role {
		case "assistant":
			history = append(history, kiroHistoryMessage{
				AssistantResponseMessage: buildKiroAssistantHistoryMessage(message.Content),
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

func buildKiroAssistantHistoryMessage(contentRaw json.RawMessage) *kiroHistoryAssistantMessage {
	content, toolUses := parseKiroAssistantMessageContent(contentRaw)
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

func parseKiroAssistantMessageContent(raw json.RawMessage) (string, []kiroToolUse) {
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
				Name:      strings.TrimSpace(block.Name),
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
	if !strings.EqualFold(strings.TrimSpace(source.Type), "base64") {
		return nil
	}
	if strings.TrimSpace(source.Data) == "" {
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

func convertKiroTools(tools []anthropicTool) []kiroTool {
	if len(tools) == 0 {
		return nil
	}

	result := make([]kiroTool, 0, len(tools))
	for _, tool := range tools {
		name := strings.TrimSpace(tool.Name)
		if name == "" {
			continue
		}
		schema := normalizeKiroInputSchema(tool.InputSchema)
		result = append(result, kiroTool{
			ToolSpecification: kiroToolSpecification{
				Name:        name,
				Description: strings.TrimSpace(tool.Description),
				InputSchema: kiroInputSchemaWrap{JSON: schema},
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
	for k, v := range schema {
		out[k] = v
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

func stableKiroContinuationID(conversationID string) string {
	sum := sha256.Sum256([]byte(conversationID))
	return hex.EncodeToString(sum[:16])
}

func stringValue(v any) string {
	s, _ := v.(string)
	return s
}
