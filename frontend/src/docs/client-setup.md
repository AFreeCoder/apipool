# 客户端配置

本文介绍如何将 APIPool 接入常见的 AI 客户端和开发工具。核心配置思路都是一样的：将原始 API 地址替换为 APIPool 的地址，填入你的 API Key 即可。

---

## Claude Code

Claude Code 通过环境变量配置 API 地址和密钥。在终端中设置以下环境变量：

```bash
export ANTHROPIC_BASE_URL=https://your-api-domain
export ANTHROPIC_API_KEY=YOUR_API_KEY
```

建议将以上内容添加到 `~/.bashrc` 或 `~/.zshrc` 中，避免每次打开终端都需要重新设置。

配置完成后，直接运行 `claude` 命令即可使用。

---

## Cursor

1. 打开 Cursor，进入 **Settings** > **Models**。
2. 找到 **Override OpenAI Base URL** 设置项，填入：
   ```
   https://your-api-domain
   ```
3. 在 **API Keys** 区域填入你的 API Key。
4. 保存设置后即可使用。

---

## Cherry Studio

1. 打开 Cherry Studio，进入 **设置** > **模型服务商**。
2. 选择或添加一个服务商（如 OpenAI 或 Anthropic）。
3. 将 **API 地址** 修改为：
   ```
   https://your-api-domain
   ```
4. 填入你的 **API Key**。
5. 保存配置，在对话中选择对应模型即可使用。

---

## ChatBox

1. 打开 ChatBox，进入 **设置** > **AI 模型提供方**。
2. 选择 **OpenAI API** 或其他兼容的提供方。
3. 将 **API Host** 修改为：
   ```
   https://your-api-domain
   ```
4. 填入你的 **API Key**。
5. 保存配置后即可使用。

---

## 其他 OpenAI 兼容客户端

大多数支持 OpenAI API 格式的客户端（如 Open WebUI、LobeChat、NextChat 等）都可以通过以下方式接入 APIPool：

1. 在客户端的设置中找到 **API Base URL**（或 API Host、API Endpoint 等类似名称）。
2. 将地址修改为：
   ```
   https://your-api-domain
   ```
3. 填入你的 **API Key**。
4. 保存配置，选择可用的模型即可开始使用。

> 部分客户端可能需要在地址末尾添加 `/v1`，即 `https://your-api-domain/v1`，请根据实际情况调整。

---

## Python SDK 示例

使用 OpenAI 官方 Python SDK 接入 APIPool：

```python
from openai import OpenAI

client = OpenAI(
    api_key="YOUR_API_KEY",
    base_url="https://your-api-domain/v1",
)

response = client.chat.completions.create(
    model="claude-sonnet-4-5-20250514",
    messages=[
        {"role": "user", "content": "你好，请用一句话介绍你自己。"}
    ],
    max_tokens=256,
)

print(response.choices[0].message.content)
```

如果需要使用流式响应：

```python
stream = client.chat.completions.create(
    model="claude-sonnet-4-5-20250514",
    messages=[
        {"role": "user", "content": "写一首关于春天的短诗"}
    ],
    max_tokens=1024,
    stream=True,
)

for chunk in stream:
    if chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="")
```

---

## Node.js SDK 示例

使用 OpenAI 官方 Node.js SDK 接入 APIPool：

```javascript
import OpenAI from "openai";

const client = new OpenAI({
  apiKey: "YOUR_API_KEY",
  baseURL: "https://your-api-domain/v1",
});

async function main() {
  const response = await client.chat.completions.create({
    model: "claude-sonnet-4-5-20250514",
    messages: [
      { role: "user", content: "你好，请用一句话介绍你自己。" },
    ],
    max_tokens: 256,
  });

  console.log(response.choices[0].message.content);
}

main();
```

如果需要使用流式响应：

```javascript
import OpenAI from "openai";

const client = new OpenAI({
  apiKey: "YOUR_API_KEY",
  baseURL: "https://your-api-domain/v1",
});

async function main() {
  const stream = await client.chat.completions.create({
    model: "claude-sonnet-4-5-20250514",
    messages: [
      { role: "user", content: "写一首关于春天的短诗" },
    ],
    max_tokens: 1024,
    stream: true,
  });

  for await (const chunk of stream) {
    const content = chunk.choices[0]?.delta?.content;
    if (content) {
      process.stdout.write(content);
    }
  }
}

main();
```
