# API 接入指南

APIPool 兼容多种主流 AI API 格式，你可以根据使用场景选择最合适的格式进行调用。

---

## 支持的 API 格式概览

| 格式 | 端点 | 说明 |
|------|------|------|
| OpenAI Chat Completions | `POST /v1/chat/completions` | 最通用的格式，兼容大多数客户端和工具 |
| Claude Messages | `POST /v1/messages` | Anthropic 原生格式，需要 `anthropic-version` Header |
| OpenAI Responses | `POST /v1/responses` | OpenAI 新版 Responses API 格式 |
| Gemini | `POST /v1beta/models/{model}:generateContent` | Google Gemini 原生格式 |

---

## 各格式端点和请求示例

### OpenAI Chat Completions

这是最通用的格式，绝大多数客户端和 SDK 都支持。

```bash
curl https://your-api-domain/v1/chat/completions \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-5-20250514",
    "messages": [
      {"role": "user", "content": "你好"}
    ],
    "max_tokens": 1024
  }'
```

### Claude Messages

Anthropic 原生格式，需要额外传递 `anthropic-version` Header。

```bash
curl https://your-api-domain/v1/messages \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-sonnet-4-5-20250514",
    "max_tokens": 1024,
    "messages": [
      {"role": "user", "content": "你好"}
    ]
  }'
```

### OpenAI Responses

OpenAI 新版 Responses API 格式，使用 `input` 字段代替 `messages`。

```bash
curl https://your-api-domain/v1/responses \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-5-20250514",
    "input": "你好"
  }'
```

### Gemini

Google Gemini 原生格式，端点路径中需要包含模型名称，认证使用 `x-goog-api-key` Header。

```bash
curl "https://your-api-domain/v1beta/models/gemini-2.5-pro:generateContent" \
  -H "x-goog-api-key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "contents": [
      {"parts": [{"text": "你好"}]}
    ]
  }'
```

---

## 认证方式

API 请求支持以下三种认证 Header，任选其一即可：

| Header | 格式 | 适用场景 |
|--------|------|----------|
| `Authorization` | `Bearer YOUR_API_KEY` | 标准方式，兼容所有格式 |
| `x-api-key` | `YOUR_API_KEY` | Anthropic 风格，适用于 Claude 客户端 |
| `x-goog-api-key` | `YOUR_API_KEY` | Google 风格，适用于 Gemini 客户端 |

---

## 查看可用模型

通过 `/v1/models` 接口查询当前 API Key 可用的模型列表：

```bash
curl https://your-api-domain/v1/models \
  -H "Authorization: Bearer YOUR_API_KEY"
```

返回结果包含所有可用模型的 ID、名称等信息，格式与 OpenAI 的 `/v1/models` 接口一致。

---

## Token 计数

在发送请求前，可以使用 Token 计数接口预估消耗：

```bash
curl https://your-api-domain/v1/messages/count_tokens \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-5-20250514",
    "messages": [
      {"role": "user", "content": "Hello, world!"}
    ]
  }'
```

---

## 流式响应

所有支持的 API 格式均支持流式响应，只需在请求体中添加 `"stream": true` 参数即可。流式响应会以 Server-Sent Events（SSE）格式逐步返回内容。

**OpenAI 格式示例：**

```bash
curl https://your-api-domain/v1/chat/completions \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-5-20250514",
    "messages": [
      {"role": "user", "content": "写一首短诗"}
    ],
    "max_tokens": 1024,
    "stream": true
  }'
```

**Claude 格式示例：**

```bash
curl https://your-api-domain/v1/messages \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-sonnet-4-5-20250514",
    "max_tokens": 1024,
    "messages": [
      {"role": "user", "content": "写一首短诗"}
    ],
    "stream": true
  }'
```
