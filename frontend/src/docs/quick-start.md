# 快速开始

> 有任何问题，请联系微信：**AFreeCoder01**
>
> ![客服微信二维码](https://tjjsjwhj-blog.oss-cn-beijing.aliyuncs.com/2026/03/05/17726209788829.jpg)

APIPool 是一个 AI API 网关平台，专注于提供御三家 OpenAI、Claude、Gemini 的大模型 API 服务，只做最真的 API，做最靠谱的 API 站。

![74cbec8450496d2477ed1b464a9ceb36](https://tjjsjwhj-blog.oss-cn-beijing.aliyuncs.com/2026/04/03/74cbec8450496d2477ed1b464a9ceb36.jpg)


---

## 注册账号

1. 访问平台首页，点击右上角登录，点击下方**注册**。
2. 填写邮箱、密码，点击下一步，获取邮件验证码。
3. 输入验证码
4. 完成注册。

---

## 创建 API Key

1. 登录后自动进入控制台页面。
2. 左侧导航栏，点击菜单**API 密钥**
3. 点击**创建密钥**，填写以下信息：
	a. **名称**：为 Key 起一个便于识别的名称，如"开发测试"。
	b. **分组**：选择要使用的服务分组（决定可用的模型和计费方式）。
4. 点击 **创建**，生成密钥。
5. **重要**：额度限制建议不填。

---

## API 域名说明

本平台提供两个 API 端点：

| 域名 | 说明 | 适用场景 |
|------|------|---------|
| `api.apipool.dev`（推荐） | 直连线路，**无需代理** | 国内用户首选 |
| `apipool.dev` | Cloudflare CDN | 海外用户 / 备用线路 |

> **推荐**：国内用户请使用 `api.apipool.dev`，无需开启代理即可直接使用。

---

## 开始使用

API Key 创建完成后，点击右侧的**使用密钥**，平台会展示不同环境（windos、Max/Linux）下不同工具（Codex、Claude Code、OpenCode 等）的详细配置方法。

另外，强烈建议使用 [CC Switch](https://github.com/farion1231/cc-switch) 工具，可以方便管理 Codex、Claude Code 等多个工具的不同供应商，一键切换，本平台支持一键导入 CC Switch 配置。

### OpenAI（Codex） 渠道

下面是 Codex 等客户端的配置方法：

![](https://tjjsjwhj-blog.oss-cn-beijing.aliyuncs.com/2026/03/05/17726388322434.jpg)

### Anthropic（cc-max） 渠道

下面是 Claude Code 等客户端的配置方法：

![](https://tjjsjwhj-blog.oss-cn-beijing.aliyuncs.com/2026/03/05/17726388882147.jpg)
### 一键导入 cc switch 配置

点击密钥右侧**导入到 CCS** 即可：

![](https://tjjsjwhj-blog.oss-cn-beijing.aliyuncs.com/2026/03/05/17726736229817.jpg)

