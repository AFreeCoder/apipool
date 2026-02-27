# 快速开始

APIPool 是一个 AI API 网关平台，将 Claude、GPT、Gemini 等 AI 服务统一封装为标准 API 格式，让你通过一个 API Key 即可访问多种 AI 模型。

---

## 注册账号

1. 访问平台首页，点击 **注册**。
2. 填写邮箱、用户名和密码。如果平台要求邀请码，请一并填写。
3. 点击 **注册** 完成账号创建。
4. 如果平台开启了邮箱验证，前往邮箱点击验证链接完成激活。

---

## 创建 API Key

1. 登录后进入 **API Keys** 页面（`/keys`）。
2. 点击 **创建 API Key**。
3. 填写以下信息：
   - **名称**：为 Key 起一个便于识别的名称，如"开发测试"。
   - **分组**：选择要使用的服务分组（决定可用的模型和计费方式）。
4. 点击 **创建**。
5. **重要**：创建成功后会显示完整的 API Key，请立即复制保存。关闭对话框后将无法再次查看完整 Key。

---

## 第一次 API 调用

拿到 API Key 后，使用以下 curl 命令发送第一个请求（OpenAI 兼容格式）：

```bash
curl https://your-api-domain/v1/chat/completions \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-5-20250514",
    "messages": [
      {"role": "user", "content": "你好，请用一句话介绍你自己。"}
    ],
    "max_tokens": 256
  }'
```

预期返回（JSON 格式）：

```json
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "model": "claude-sonnet-4-5-20250514",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "你好！我是一个 AI 助手，可以帮你回答问题、编写代码和处理各种任务。"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 18,
    "completion_tokens": 30,
    "total_tokens": 48
  }
}
```

看到类似的返回结果，说明你已经成功接入 APIPool。接下来可以查看 [API 接入指南](/docs/api-guide) 了解更多 API 格式，或查看 [客户端配置](/docs/client-setup) 将 APIPool 接入你常用的开发工具。
