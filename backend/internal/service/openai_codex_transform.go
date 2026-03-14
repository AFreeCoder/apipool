package service

import (
	"fmt"
	"strings"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

var codexModelMap = map[string]string{
	"gpt-5.4":                    "gpt-5.4",
	"gpt-5.4-none":               "gpt-5.4",
	"gpt-5.4-low":                "gpt-5.4",
	"gpt-5.4-medium":             "gpt-5.4",
	"gpt-5.4-high":               "gpt-5.4",
	"gpt-5.4-xhigh":              "gpt-5.4",
	"gpt-5.4-chat-latest":        "gpt-5.4",
	"gpt-5.3":                    "gpt-5.3-codex",
	"gpt-5.3-none":               "gpt-5.3-codex",
	"gpt-5.3-low":                "gpt-5.3-codex",
	"gpt-5.3-medium":             "gpt-5.3-codex",
	"gpt-5.3-high":               "gpt-5.3-codex",
	"gpt-5.3-xhigh":              "gpt-5.3-codex",
	"gpt-5.3-codex":              "gpt-5.3-codex",
	"gpt-5.3-codex-spark":        "gpt-5.3-codex",
	"gpt-5.3-codex-spark-low":    "gpt-5.3-codex",
	"gpt-5.3-codex-spark-medium": "gpt-5.3-codex",
	"gpt-5.3-codex-spark-high":   "gpt-5.3-codex",
	"gpt-5.3-codex-spark-xhigh":  "gpt-5.3-codex",
	"gpt-5.3-codex-low":          "gpt-5.3-codex",
	"gpt-5.3-codex-medium":       "gpt-5.3-codex",
	"gpt-5.3-codex-high":         "gpt-5.3-codex",
	"gpt-5.3-codex-xhigh":        "gpt-5.3-codex",
	"gpt-5.1-codex":              "gpt-5.1-codex",
	"gpt-5.1-codex-low":          "gpt-5.1-codex",
	"gpt-5.1-codex-medium":       "gpt-5.1-codex",
	"gpt-5.1-codex-high":         "gpt-5.1-codex",
	"gpt-5.1-codex-max":          "gpt-5.1-codex-max",
	"gpt-5.1-codex-max-low":      "gpt-5.1-codex-max",
	"gpt-5.1-codex-max-medium":   "gpt-5.1-codex-max",
	"gpt-5.1-codex-max-high":     "gpt-5.1-codex-max",
	"gpt-5.1-codex-max-xhigh":    "gpt-5.1-codex-max",
	"gpt-5.2":                    "gpt-5.2",
	"gpt-5.2-none":               "gpt-5.2",
	"gpt-5.2-low":                "gpt-5.2",
	"gpt-5.2-medium":             "gpt-5.2",
	"gpt-5.2-high":               "gpt-5.2",
	"gpt-5.2-xhigh":              "gpt-5.2",
	"gpt-5.2-codex":              "gpt-5.2-codex",
	"gpt-5.2-codex-low":          "gpt-5.2-codex",
	"gpt-5.2-codex-medium":       "gpt-5.2-codex",
	"gpt-5.2-codex-high":         "gpt-5.2-codex",
	"gpt-5.2-codex-xhigh":        "gpt-5.2-codex",
	"gpt-5.1-codex-mini":         "gpt-5.1-codex-mini",
	"gpt-5.1-codex-mini-medium":  "gpt-5.1-codex-mini",
	"gpt-5.1-codex-mini-high":    "gpt-5.1-codex-mini",
	"gpt-5.1":                    "gpt-5.1",
	"gpt-5.1-none":               "gpt-5.1",
	"gpt-5.1-low":                "gpt-5.1",
	"gpt-5.1-medium":             "gpt-5.1",
	"gpt-5.1-high":               "gpt-5.1",
	"gpt-5.1-chat-latest":        "gpt-5.1",
	"gpt-5-codex":                "gpt-5.1-codex",
	"codex-mini-latest":          "gpt-5.1-codex-mini",
	"gpt-5-codex-mini":           "gpt-5.1-codex-mini",
	"gpt-5-codex-mini-medium":    "gpt-5.1-codex-mini",
	"gpt-5-codex-mini-high":      "gpt-5.1-codex-mini",
	"gpt-5":                      "gpt-5.1",
	"gpt-5-mini":                 "gpt-5.1",
	"gpt-5-nano":                 "gpt-5.1",
}

type codexTransformResult struct {
	Modified        bool
	NormalizedModel string
	PromptCacheKey  string
}

var loggedUnknownCodexOAuthInputItemTypes sync.Map

func applyCodexOAuthTransform(reqBody map[string]any, isCodexCLI bool, isCompact bool) codexTransformResult {
	result := codexTransformResult{}
	// 工具续链需求会影响存储策略与 input 过滤逻辑。
	needsToolContinuation := NeedsToolContinuation(reqBody)

	model := ""
	if v, ok := reqBody["model"].(string); ok {
		model = v
	}
	normalizedModel := normalizeCodexModel(model)
	if normalizedModel != "" {
		if model != normalizedModel {
			reqBody["model"] = normalizedModel
			result.Modified = true
		}
		result.NormalizedModel = normalizedModel
	}

	if isCompact {
		if _, ok := reqBody["store"]; ok {
			delete(reqBody, "store")
			result.Modified = true
		}
		if _, ok := reqBody["stream"]; ok {
			delete(reqBody, "stream")
			result.Modified = true
		}
	} else {
		// OAuth 走 ChatGPT internal API 时，store 必须为 false；显式 true 也会强制覆盖。
		// 避免上游返回 "Store must be set to false"。
		if v, ok := reqBody["store"].(bool); !ok || v {
			reqBody["store"] = false
			result.Modified = true
		}
		if v, ok := reqBody["stream"].(bool); !ok || !v {
			reqBody["stream"] = true
			result.Modified = true
		}
	}

	// Strip parameters unsupported by codex models via the Responses API.
	for _, key := range []string{
		"max_output_tokens",
		"max_completion_tokens",
		"temperature",
		"top_p",
		"frequency_penalty",
		"presence_penalty",
	} {
		if _, ok := reqBody[key]; ok {
			delete(reqBody, key)
			result.Modified = true
		}
	}

	// 兼容遗留的 functions 和 function_call，转换为 tools 和 tool_choice
	if functionsRaw, ok := reqBody["functions"]; ok {
		if functions, k := functionsRaw.([]any); k {
			tools := make([]any, 0, len(functions))
			for _, f := range functions {
				tools = append(tools, map[string]any{
					"type":     "function",
					"function": f,
				})
			}
			reqBody["tools"] = tools
		}
		delete(reqBody, "functions")
		result.Modified = true
	}

	if fcRaw, ok := reqBody["function_call"]; ok {
		if fcStr, ok := fcRaw.(string); ok {
			// e.g. "auto", "none"
			reqBody["tool_choice"] = fcStr
		} else if fcObj, ok := fcRaw.(map[string]any); ok {
			// e.g. {"name": "my_func"}
			if name, ok := fcObj["name"].(string); ok && strings.TrimSpace(name) != "" {
				reqBody["tool_choice"] = map[string]any{
					"type": "function",
					"function": map[string]any{
						"name": name,
					},
				}
			}
		}
		delete(reqBody, "function_call")
		result.Modified = true
	}

	if normalizeCodexTools(reqBody) {
		result.Modified = true
	}

	if v, ok := reqBody["prompt_cache_key"].(string); ok {
		result.PromptCacheKey = strings.TrimSpace(v)
	}

	// instructions 处理逻辑：根据是否是 Codex CLI 分别调用不同方法
	if applyInstructions(reqBody, isCodexCLI) {
		result.Modified = true
	}

	if normalizeCodexOAuthInput(reqBody, needsToolContinuation) {
		result.Modified = true
	}

	// 续链场景保留 item_reference 与 id，避免 call_id 上下文丢失。
	if input, ok := reqBody["input"].([]any); ok {
		input = filterCodexInput(input, needsToolContinuation)
		reqBody["input"] = input
		result.Modified = true
	} else if inputStr, ok := reqBody["input"].(string); ok {
		// ChatGPT codex endpoint requires input to be a list, not a string.
		// Convert string input to the expected message array format.
		trimmed := strings.TrimSpace(inputStr)
		if trimmed != "" {
			reqBody["input"] = []any{
				map[string]any{
					"type":    "message",
					"role":    "user",
					"content": inputStr,
				},
			}
		} else {
			reqBody["input"] = []any{}
		}
		result.Modified = true
	}

	return result
}

func normalizeCodexOAuthInput(reqBody map[string]any, preserveReferences bool) bool {
	rawInput, ok := reqBody["input"]
	if !ok || rawInput == nil {
		return false
	}

	switch input := rawInput.(type) {
	case string:
		trimmed := strings.TrimSpace(input)
		if trimmed == "" {
			reqBody["input"] = []any{}
			return true
		}
		reqBody["input"] = []any{
			map[string]any{
				"type":    "message",
				"role":    "user",
				"content": input,
			},
		}
		return true
	case []any:
		if len(input) == 0 {
			return false
		}
		if isCodexOAuthMessageList(input) {
			normalized, changed := normalizeCodexOAuthMessageList(input, preserveReferences)
			if changed {
				reqBody["input"] = normalized
			}
			return changed
		}
		if shouldPreserveCodexOAuthInputItems(input) {
			normalized, changed := normalizeCodexOAuthMixedInput(input, preserveReferences)
			if changed {
				reqBody["input"] = normalized
			}
			return changed
		}
		normalizedContent, _ := normalizeCodexOAuthContentItems(input, preserveReferences)
		reqBody["input"] = []any{
			map[string]any{
				"role":    "user",
				"content": normalizedContent,
			},
		}
		return true
	case map[string]any:
		if _, ok := input["role"]; ok {
			normalizedMessage, _ := normalizeCodexOAuthMessage(input, preserveReferences)
			reqBody["input"] = []any{normalizedMessage}
			return true
		}
		if shouldPreserveCodexOAuthInputItem(input) {
			return false
		}
		normalizedItem, _ := normalizeCodexOAuthContentItem(input, preserveReferences)
		reqBody["input"] = []any{
			map[string]any{
				"role":    "user",
				"content": []any{normalizedItem},
			},
		}
		return true
	default:
		return false
	}
}

func isCodexOAuthMessageList(input []any) bool {
	if len(input) == 0 {
		return false
	}
	for _, item := range input {
		message, ok := item.(map[string]any)
		if !ok {
			return false
		}
		if _, ok := message["role"]; !ok {
			return false
		}
	}
	return true
}

func normalizeCodexOAuthMessageList(input []any, preserveReferences bool) ([]any, bool) {
	normalized := make([]any, 0, len(input))
	changed := false
	for _, item := range input {
		message, ok := item.(map[string]any)
		if !ok {
			normalized = append(normalized, item)
			continue
		}
		normalizedMessage, messageChanged := normalizeCodexOAuthMessage(message, preserveReferences)
		normalized = append(normalized, normalizedMessage)
		changed = changed || messageChanged
	}
	return normalized, changed
}

// normalizeCodexOAuthMixedInput 处理包含 message、top-level transcript item（如 reasoning）
// 与裸 content item 混合的 input 数组：
// 1. message 做 normalize
// 2. transcript item 原样保留
// 3. 裸 content item 按顺序聚合回 user message，避免被错误当作 transcript item 透传
func normalizeCodexOAuthMixedInput(input []any, preserveReferences bool) ([]any, bool) {
	normalized := make([]any, 0, len(input))
	changed := false
	pendingContent := make([]any, 0)

	flushPendingContent := func() {
		if len(pendingContent) == 0 {
			return
		}
		normalized = append(normalized, map[string]any{
			"role":    "user",
			"content": pendingContent,
		})
		pendingContent = nil
		changed = true
	}

	for _, item := range input {
		m, ok := item.(map[string]any)
		if !ok {
			pendingContent = append(pendingContent, item)
			continue
		}
		// 有 role 的是 message，做 normalize
		if _, hasRole := m["role"]; hasRole {
			flushPendingContent()
			normalizedMessage, messageChanged := normalizeCodexOAuthMessage(m, preserveReferences)
			normalized = append(normalized, normalizedMessage)
			changed = changed || messageChanged
			continue
		}
		if shouldPreserveCodexOAuthInputItem(m) {
			flushPendingContent()
			// 无 role 的 top-level transcript item（reasoning 等）原样保留
			normalized = append(normalized, m)
			continue
		}

		normalizedItem, itemChanged := normalizeCodexOAuthContentItem(m, preserveReferences)
		pendingContent = append(pendingContent, normalizedItem)
		changed = changed || itemChanged
	}
	flushPendingContent()
	return normalized, changed
}

func normalizeCodexOAuthMessage(message map[string]any, preserveReferences bool) (map[string]any, bool) {
	rawContent, ok := message["content"]
	if !ok || rawContent == nil {
		return message, false
	}

	switch content := rawContent.(type) {
	case string:
		next := cloneAnyMap(message)
		next["content"] = []any{
			map[string]any{
				"type": "input_text",
				"text": content,
			},
		}
		return next, true
	case []any:
		normalizedContent, contentChanged := normalizeCodexOAuthContentItems(content, preserveReferences)
		if !contentChanged {
			return message, false
		}
		next := cloneAnyMap(message)
		next["content"] = normalizedContent
		return next, true
	default:
		return message, false
	}
}

func shouldPreserveCodexOAuthInputItems(input []any) bool {
	for _, item := range input {
		message, ok := item.(map[string]any)
		if !ok {
			return true
		}
		if shouldPreserveCodexOAuthInputItem(message) {
			return true
		}
	}
	return false
}

func shouldPreserveCodexOAuthInputItem(item map[string]any) bool {
	itemType, _ := item["type"].(string)
	normalizedType := strings.TrimSpace(itemType)
	if isKnownCodexOAuthContentItemType(normalizedType) {
		return false
	}
	if !isKnownCodexOAuthTopLevelItemType(normalizedType) {
		logUnknownCodexOAuthInputItemType(normalizedType)
	}
	return true
}

func isKnownCodexOAuthContentItemType(itemType string) bool {
	// 已知可安全嵌入 message content 的类型走 content 路径。
	switch itemType {
	case "", "text", "message", "input_text", "input_image", "input_file",
		"output_text", "refusal", "computer_screenshot", "summary_text",
		"image_url", "image":
		return true
	default:
		return false
	}
}

func isKnownCodexOAuthTopLevelItemType(itemType string) bool {
	// 常见的 top-level transcript item：保留原状，不打印噪声日志。
	switch itemType {
	case "reasoning", "function_call_output", "item_reference", "tool_call", "function_call", "web_search_call":
		return true
	default:
		return false
	}
}

func logUnknownCodexOAuthInputItemType(itemType string) {
	if itemType == "" {
		return
	}
	if _, loaded := loggedUnknownCodexOAuthInputItemTypes.LoadOrStore(itemType, struct{}{}); loaded {
		return
	}
	logger.LegacyPrintf("service.openai_gateway",
		"[Codex OAuth] Unknown input item type preserved as top-level transcript item: %s",
		itemType,
	)
}

func normalizeCodexOAuthContentItems(input []any, preserveReferences bool) ([]any, bool) {
	normalized := make([]any, 0, len(input))
	changed := false
	for _, item := range input {
		contentItem, ok := item.(map[string]any)
		if !ok {
			normalized = append(normalized, item)
			continue
		}
		normalizedItem, itemChanged := normalizeCodexOAuthContentItem(contentItem, preserveReferences)
		normalized = append(normalized, normalizedItem)
		changed = changed || itemChanged
	}
	return normalized, changed
}

func normalizeCodexOAuthContentItem(item map[string]any, preserveReferences bool) (map[string]any, bool) {
	next := item
	changed := false

	if itemType, _ := next["type"].(string); isLegacyCodexTextContentType(itemType) {
		next = cloneAnyMap(next)
		next["type"] = "input_text"
		if text, ok := extractLegacyCodexText(next); ok {
			next["text"] = text
		}
		delete(next, "content")
		delete(next, "message")
		delete(next, "role")
		changed = true
	}

	if preserveReferences {
		return next, changed
	}

	if _, ok := next["id"]; ok {
		if !changed {
			next = cloneAnyMap(next)
			changed = true
		}
		delete(next, "id")
	}

	itemType, _ := next["type"].(string)
	if !isCodexToolCallItemType(itemType) {
		if _, ok := next["call_id"]; ok {
			if !changed {
				next = cloneAnyMap(next)
				changed = true
			}
			delete(next, "call_id")
		}
	}

	return next, changed
}

func isLegacyCodexTextContentType(itemType string) bool {
	switch strings.TrimSpace(itemType) {
	case "text", "message":
		return true
	default:
		return false
	}
}

func extractLegacyCodexText(item map[string]any) (string, bool) {
	if item == nil {
		return "", false
	}
	if text, ok := item["text"].(string); ok && strings.TrimSpace(text) != "" {
		return text, true
	}
	if text, ok := item["message"].(string); ok && strings.TrimSpace(text) != "" {
		return text, true
	}
	if text, ok := item["content"].(string); ok && strings.TrimSpace(text) != "" {
		return text, true
	}
	content, ok := item["content"].([]any)
	if !ok {
		return "", false
	}
	for _, rawPart := range content {
		part, ok := rawPart.(map[string]any)
		if !ok {
			continue
		}
		if text, ok := part["text"].(string); ok && strings.TrimSpace(text) != "" {
			return text, true
		}
		if text, ok := part["content"].(string); ok && strings.TrimSpace(text) != "" {
			return text, true
		}
	}
	return "", false
}

func cloneAnyMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func normalizeCodexModel(model string) string {
	if model == "" {
		return "gpt-5.1"
	}

	modelID := model
	if strings.Contains(modelID, "/") {
		parts := strings.Split(modelID, "/")
		modelID = parts[len(parts)-1]
	}

	if mapped := getNormalizedCodexModel(modelID); mapped != "" {
		return mapped
	}

	normalized := strings.ToLower(modelID)

	if strings.Contains(normalized, "gpt-5.4") || strings.Contains(normalized, "gpt 5.4") {
		return "gpt-5.4"
	}
	if strings.Contains(normalized, "gpt-5.2-codex") || strings.Contains(normalized, "gpt 5.2 codex") {
		return "gpt-5.2-codex"
	}
	if strings.Contains(normalized, "gpt-5.2") || strings.Contains(normalized, "gpt 5.2") {
		return "gpt-5.2"
	}
	if strings.Contains(normalized, "gpt-5.3-codex") || strings.Contains(normalized, "gpt 5.3 codex") {
		return "gpt-5.3-codex"
	}
	if strings.Contains(normalized, "gpt-5.3") || strings.Contains(normalized, "gpt 5.3") {
		return "gpt-5.3-codex"
	}
	if strings.Contains(normalized, "gpt-5.1-codex-max") || strings.Contains(normalized, "gpt 5.1 codex max") {
		return "gpt-5.1-codex-max"
	}
	if strings.Contains(normalized, "gpt-5.1-codex-mini") || strings.Contains(normalized, "gpt 5.1 codex mini") {
		return "gpt-5.1-codex-mini"
	}
	if strings.Contains(normalized, "codex-mini-latest") ||
		strings.Contains(normalized, "gpt-5-codex-mini") ||
		strings.Contains(normalized, "gpt 5 codex mini") {
		return "codex-mini-latest"
	}
	if strings.Contains(normalized, "gpt-5.1-codex") || strings.Contains(normalized, "gpt 5.1 codex") {
		return "gpt-5.1-codex"
	}
	if strings.Contains(normalized, "gpt-5.1") || strings.Contains(normalized, "gpt 5.1") {
		return "gpt-5.1"
	}
	if strings.Contains(normalized, "codex") {
		return "gpt-5.1-codex"
	}
	if strings.Contains(normalized, "gpt-5") || strings.Contains(normalized, "gpt 5") {
		return "gpt-5.1"
	}

	return "gpt-5.1"
}

func SupportsVerbosity(model string) bool {
	if !strings.HasPrefix(model, "gpt-") {
		return true
	}

	var major, minor int
	n, _ := fmt.Sscanf(model, "gpt-%d.%d", &major, &minor)

	if major > 5 {
		return true
	}
	if major < 5 {
		return false
	}

	// gpt-5
	if n == 1 {
		return true
	}

	return minor >= 3
}

func getNormalizedCodexModel(modelID string) string {
	if modelID == "" {
		return ""
	}
	if mapped, ok := codexModelMap[modelID]; ok {
		return mapped
	}
	lower := strings.ToLower(modelID)
	for key, value := range codexModelMap {
		if strings.ToLower(key) == lower {
			return value
		}
	}
	return ""
}

// applyInstructions 处理 instructions 字段：仅在 instructions 为空时填充默认值。
func applyInstructions(reqBody map[string]any, isCodexCLI bool) bool {
	if !isInstructionsEmpty(reqBody) {
		return false
	}
	reqBody["instructions"] = "You are a helpful coding assistant."
	return true
}

// isInstructionsEmpty 检查 instructions 字段是否为空
// 处理以下情况：字段不存在、nil、空字符串、纯空白字符串
func isInstructionsEmpty(reqBody map[string]any) bool {
	val, exists := reqBody["instructions"]
	if !exists {
		return true
	}
	if val == nil {
		return true
	}
	str, ok := val.(string)
	if !ok {
		return true
	}
	return strings.TrimSpace(str) == ""
}

// filterCodexInput 按需过滤 item_reference 与 id。
// preserveReferences 为 true 时保持引用与 id，以满足续链请求对上下文的依赖。
func filterCodexInput(input []any, preserveReferences bool) []any {
	filtered := make([]any, 0, len(input))
	for _, item := range input {
		m, ok := item.(map[string]any)
		if !ok {
			filtered = append(filtered, item)
			continue
		}
		typ, _ := m["type"].(string)

		// 仅修正真正的 tool/function call 标识，避免误改普通 message/reasoning id；
		// 若 item_reference 指向 legacy call_* 标识，则仅修正该引用本身。
		fixCallIDPrefix := func(id string) string {
			if id == "" || strings.HasPrefix(id, "fc") {
				return id
			}
			if strings.HasPrefix(id, "call_") {
				return "fc" + strings.TrimPrefix(id, "call_")
			}
			return "fc_" + id
		}

		if typ == "item_reference" {
			if !preserveReferences {
				continue
			}
			newItem := make(map[string]any, len(m))
			for key, value := range m {
				newItem[key] = value
			}
			if id, ok := newItem["id"].(string); ok && strings.HasPrefix(id, "call_") {
				newItem["id"] = fixCallIDPrefix(id)
			}
			filtered = append(filtered, newItem)
			continue
		}

		// reasoning 等非工具型 top-level transcript item 必须原样保留。
		// tool_call/function_call/function_call_output 仍需继续向下处理，
		// 以便补全/规范 call_id 前缀。
		if shouldPreserveCodexOAuthInputItem(m) && !isCodexToolCallItemType(typ) {
			filtered = append(filtered, m)
			continue
		}

		newItem := m
		copied := false
		// 仅在需要修改字段时创建副本，避免直接改写原始输入。
		ensureCopy := func() {
			if copied {
				return
			}
			newItem = make(map[string]any, len(m))
			for key, value := range m {
				newItem[key] = value
			}
			copied = true
		}

		if isCodexToolCallItemType(typ) {
			callID, ok := m["call_id"].(string)
			if !ok || strings.TrimSpace(callID) == "" {
				if id, ok := m["id"].(string); ok && strings.TrimSpace(id) != "" {
					callID = id
					ensureCopy()
					newItem["call_id"] = callID
				}
			}

			if callID != "" {
				fixedCallID := fixCallIDPrefix(callID)
				if fixedCallID != callID {
					ensureCopy()
					newItem["call_id"] = fixedCallID
				}
			}
		}

		if !preserveReferences {
			ensureCopy()
			delete(newItem, "id")
			if !isCodexToolCallItemType(typ) {
				delete(newItem, "call_id")
			}
		}

		filtered = append(filtered, newItem)
	}
	return filtered
}

func isCodexToolCallItemType(typ string) bool {
	if typ == "" {
		return false
	}
	return strings.HasSuffix(typ, "_call") || strings.HasSuffix(typ, "_call_output")
}

func normalizeCodexTools(reqBody map[string]any) bool {
	rawTools, ok := reqBody["tools"]
	if !ok || rawTools == nil {
		return false
	}
	tools, ok := rawTools.([]any)
	if !ok {
		return false
	}

	modified := false
	validTools := make([]any, 0, len(tools))

	for _, tool := range tools {
		toolMap, ok := tool.(map[string]any)
		if !ok {
			// Keep unknown structure as-is to avoid breaking upstream behavior.
			validTools = append(validTools, tool)
			continue
		}

		toolType, _ := toolMap["type"].(string)
		toolType = strings.TrimSpace(toolType)
		if toolType != "function" {
			validTools = append(validTools, toolMap)
			continue
		}

		// OpenAI Responses-style tools use top-level name/parameters.
		if name, ok := toolMap["name"].(string); ok && strings.TrimSpace(name) != "" {
			validTools = append(validTools, toolMap)
			continue
		}

		// ChatCompletions-style tools use {type:"function", function:{...}}.
		functionValue, hasFunction := toolMap["function"]
		function, ok := functionValue.(map[string]any)
		if !hasFunction || functionValue == nil || !ok || function == nil {
			// Drop invalid function tools.
			modified = true
			continue
		}

		if _, ok := toolMap["name"]; !ok {
			if name, ok := function["name"].(string); ok && strings.TrimSpace(name) != "" {
				toolMap["name"] = name
				modified = true
			}
		}
		if _, ok := toolMap["description"]; !ok {
			if desc, ok := function["description"].(string); ok && strings.TrimSpace(desc) != "" {
				toolMap["description"] = desc
				modified = true
			}
		}
		if _, ok := toolMap["parameters"]; !ok {
			if params, ok := function["parameters"]; ok {
				toolMap["parameters"] = params
				modified = true
			}
		}
		if _, ok := toolMap["strict"]; !ok {
			if strict, ok := function["strict"]; ok {
				toolMap["strict"] = strict
				modified = true
			}
		}

		validTools = append(validTools, toolMap)
	}

	if modified {
		reqBody["tools"] = validTools
	}

	return modified
}
