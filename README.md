# APIPool

English | [中文](README_CN.md) | [日本語](README_JA.md)

基于 [Sub2API](https://github.com/Wei-Shaw/sub2api) 的 AI API 网关平台，用于订阅配额的分发与管理。

在线访问：**https://apipool.dev**

API 端点：**https://api.apipool.dev**（推荐，国内直连无需代理）

## 重要提示

- **服务条款风险**：使用本项目可能违反 Anthropic、OpenAI 或其他上游服务商的服务条款。请在使用前自行阅读相关服务协议，使用风险由使用者自行承担。
- **合规使用**：请仅在所在国家或地区法律法规允许的范围内使用本项目，禁止用于任何违法用途。
- **免责声明**：本项目仅供技术学习和研究使用。因使用本项目造成的账号封禁、服务中断、数据丢失或其他直接/间接损失，项目作者不承担责任。

## 功能特性

- **多账号管理** - 支持多种上游账号类型（OAuth、API Key）
- **API Key 分发** - 为用户生成和管理 API Key
- **精确计费** - Token 级别的用量追踪与费用计算
- **智能调度** - 智能账号选择与会话粘滞
- **并发控制** - 用户级与账号级并发限制
- **限速策略** - 可配置的请求与 Token 限速
- **支付能力** - 已合入上游内建支付相关能力，同时保留当前项目通过 iframe 接入外部充值页的方式
- **管理后台** - Web 界面进行监控与管理
- **外部系统集成** - 可通过 iframe 嵌入支付、工单等外部系统扩展后台能力
- **Grok / xAI OAuth** - 支持 Grok 订阅账号接入和 OpenAI-compatible / Anthropic-compatible 转发
- **Antigravity** - 支持专用 Claude / Gemini 网关入口

## 技术栈

| 组件 | 技术 |
|------|------|
| 后端 | Go 1.26.4, Gin, Ent ORM |
| 前端 | Vue 3 + TypeScript, Vite, Pinia, TailwindCSS |
| 数据库 | PostgreSQL 18 |
| 缓存 | Redis 8 |
| 包管理 | 后端: go modules / 前端: npm（CI 用 pnpm） |

## 部署

部署在 **DigitalOcean** 服务器上，通过 GitHub Actions 自动部署（push to `main` 触发）。

当前生产部署特征：

- 单实例 Docker Compose 部署
- 发布前自动做数据库备份
- 发布前自动给当前线上镜像打回退 tag
- 应用进程支持优雅退出，容器设置了 `stop_grace_period`
- 但 **不是零中断滚动发布**；发布或重启时会重建单个 `sub2api` 容器，通常只有短暂中断窗口

### Nginx 反向代理注意事项

如果使用 Nginx 反向代理 Codex CLI 或其他依赖 `session_id` / `conversation_id` 头的请求，需要在 `http` 配置块中开启：

```nginx
underscores_in_headers on;
```

否则 Nginx 会默认丢弃带下划线的请求头，导致粘性会话和多账号路由异常。

```bash
ssh digitalocean               # 登录服务器
cd /opt/sub2api/deploy

# 常用运维命令
docker compose -f docker-compose.deploy.yml up -d       # 启动服务
docker compose -f docker-compose.deploy.yml down         # 停止服务
docker compose -f docker-compose.deploy.yml logs -f      # 查看日志
docker compose -f docker-compose.deploy.yml restart      # 重启服务
```

### 备份与回滚

生产环境已经验证可正常执行以下动作：

- `cd /opt/sub2api/deploy && ./rollback.sh prep`
  部署前生成 `pre-deploy-*.sql.gz` 数据库备份，并刷新 `deploy-sub2api:rollback-latest`
- `cd /opt/sub2api/deploy && ./rollback.sh image`
  出现问题时快速回到上一个线上镜像
- `cd /opt/sub2api/deploy && ./rollback.sh db-restore --with-image`
  必要时恢复数据库，并显式指定恢复后启动的应用版本

详细说明见 [deploy/ROLLBACK_CN.md](deploy/ROLLBACK_CN.md)。

生产环境当前额外约束：

- 发布前 `pre-deploy*.sql.gz` 仍保留全量备份，用于高可靠回滚
- 周期备份脚本 `backup-postgres.sh` 默认只保留最近 `72` 小时 / `18` 份，并排除 `ops_system_logs`、`ops_error_logs`、`ops_system_metrics`、`ops_metrics_*` 等纯运维日志/指标表的数据
- `ops.cleanup.error_log_retention_days` 建议设为 `7`，它会一并清理 `ops_system_logs` / `ops_error_logs` / `ops_retry_attempts` / `ops_alert_events`
- `dashboard_aggregation.retention.usage_logs_days` 建议设为 `30`
- 自动部署会额外收敛旧 `rollback-*` 镜像标签；应用镜像由 GitHub Actions 构建并推送到 GHCR，服务器只拉取本次 commit 对应镜像

上游内建支付功能的配置文档见 [docs/PAYMENT.md](docs/PAYMENT.md) 与 [docs/PAYMENT_CN.md](docs/PAYMENT_CN.md)。当前 APIPool 仍保留通过系统设置配置 iframe 充值页的本地方案，两种能力并存，合入上游时不要默认互相替换。

## Simple Mode

- 开启方式：设置环境变量 `RUN_MODE=simple`
- 行为差异：隐藏 SaaS 相关功能并跳过计费流程
- 生产安全要求：生产环境还必须设置 `SIMPLE_MODE_CONFIRM=true`

## Grok / xAI OAuth 支持

APIPool 合入了上游 Grok 订阅账号支持。Grok 账号通过 xAI OAuth 接入，OpenAI-compatible Responses 请求会转发到 `${XAI_BASE_URL:-https://api.x.ai/v1}/responses`。

### 支持范围

- 平台名：`grok`
- 账号类型：OAuth 订阅账号
- OpenAI-compatible Responses：`/v1/responses`、`/responses`、`/backend-api/codex/responses`
- Anthropic-compatible Messages：`/v1/messages`
- OpenAI-compatible Chat Completions：`/v1/chat/completions`、`/chat/completions`
- Codex CLI 风格 Responses WebSocket：`/v1/responses`、`/responses`、`/backend-api/codex/responses`
- 初始模型：`grok-4.3`、`grok-build-0.1`、`grok-4.20-0309-reasoning`、`grok-4.20-0309-non-reasoning`、`grok-4.20-multi-agent-0309`
- 暂不支持：图片、视频、TTS、转录、浏览器自动化、cookie 和 Grok 网页抓取

### OAuth 配置

Grok OAuth 使用 PKCE，不需要提交私有密钥。默认配置可通过环境变量覆盖：

| 变量 | 默认值 |
|------|--------|
| `XAI_OAUTH_CLIENT_ID` | public xAI OAuth client ID |
| `XAI_OAUTH_SCOPE` | `openid profile email offline_access grok-cli:access api:access` |
| `XAI_OAUTH_REDIRECT_URI` | `http://127.0.0.1:56121/callback` |
| `XAI_OAUTH_AUTHORIZE_URL` | `https://auth.x.ai/oauth2/authorize` |
| `XAI_OAUTH_TOKEN_URL` | `https://auth.x.ai/oauth2/token` |
| `XAI_BASE_URL` | `https://api.x.ai/v1` |

管理员可以在后台创建或重新授权 Grok 账号，也可以调用管理 API：

| Endpoint | 用途 |
|----------|------|
| `POST /api/v1/admin/grok/oauth/auth-url` | 生成 xAI OAuth 授权 URL |
| `POST /api/v1/admin/grok/oauth/exchange-code` | 将 callback URL、query string 或 code 换成 OAuth 凭据 |
| `POST /api/v1/admin/grok/oauth/refresh-token` | 校验或刷新 Grok refresh token |
| `POST /api/v1/admin/grok/accounts/:id/refresh` | 刷新已有 Grok 账号 |

xAI quota 是被动展示：系统不会自造订阅额度，只会在 xAI 返回 rate-limit headers 时记录并展示。`401` 会标记账号需要重新授权；`403` 作为权益或订阅层级失败处理；`429` 根据 `Retry-After` 或短 cooldown 临时移出调度。

## Antigravity 支持

APIPool 支持 [Antigravity](https://antigravity.so/) 账号。授权后可使用专用 Claude 和 Gemini 入口。

### 专用端点

| Endpoint | Model |
|----------|-------|
| `/antigravity/v1/messages` | Claude models |
| `/antigravity/v1beta/` | Gemini models |

### Claude Code 配置

```bash
export ANTHROPIC_BASE_URL="http://localhost:8080/antigravity"
export ANTHROPIC_AUTH_TOKEN="sk-xxx"
```

## 本地开发

### Docker 基础服务

```bash
# PostgreSQL 18
docker run -d --name apipool-postgres \
  -e POSTGRES_USER=sub2api \
  -e POSTGRES_PASSWORD=sub2api \
  -e POSTGRES_DB=sub2api \
  -p 5432:5432 \
  postgres:18-alpine

# Redis 8
docker run -d --name apipool-redis \
  -p 6379:6379 \
  redis:8-alpine
```

### 本地环境凭据

| 服务 | 配置项 | 值 |
|------|--------|-----|
| PostgreSQL | 用户/密码/数据库 | `sub2api` / `sub2api` / `sub2api` |
| PostgreSQL | 端口 | `5432` |
| Redis | 端口 | `6379`（无密码） |
| 后端 | 端口/模式 | `8080` / `debug` |
| 前端 | 端口 | `3000`（Vite，代理 `/api`、`/setup` → `localhost:8080`） |
| 管理员账号 | 邮箱 | `admin@apipool.local` |
| 管理员账号 | 密码 | `admin123`（已在本地数据库重置为此值） |

### 启动开发

```bash
# 1. 启动 Docker 基础服务
docker start apipool-postgres apipool-redis

# 2. 启动后端
cd backend
DATABASE_HOST=127.0.0.1 DATABASE_PORT=5432 \
DATABASE_USER=sub2api DATABASE_PASSWORD=sub2api DATABASE_DBNAME=sub2api \
REDIS_HOST=127.0.0.1 REDIS_PORT=6379 \
SERVER_MODE=debug SERVER_PORT=8080 \
go run ./cmd/server/

# 3. 启动前端（另开终端）
cd frontend
npm install
npm run dev
```

> 首次启动会进入安装向导（Setup Wizard），通过 `http://localhost:3000` 完成数据库配置和管理员账号设置。

## CI/CD

| Workflow | 触发条件 | 内容 |
|----------|----------|------|
| deploy.yml | push to main | SSH 部署到 DigitalOcean |
| backend-ci.yml | push, PR | 单元测试 + 集成测试 + golangci-lint v2.10.1 |
| security-scan.yml | push, PR, 每周一 | govulncheck + gosec + pnpm audit |
| release.yml | tag `v*` | 构建发布 |

## 常用命令

```bash
# Docker 服务
docker start apipool-postgres apipool-redis
docker stop apipool-postgres apipool-redis
docker exec -it apipool-postgres psql -U sub2api -d sub2api
docker exec -it apipool-redis redis-cli

# 前端
cd frontend && npm run dev        # 开发
cd frontend && npm run build      # 构建（输出到 backend/internal/web/dist）

# 后端
cd backend && go run ./cmd/server/                         # 运行
cd backend && go build -tags embed -o apipool ./cmd/server # 嵌入前端构建
cd backend && go generate ./ent                            # 生成 Ent 代码
cd backend && go test -tags=unit ./...                     # 单元测试
cd backend && go test -tags=integration ./...              # 集成测试
cd backend && golangci-lint run ./...                      # Lint 检查

# 同步上游
git fetch upstream && git merge upstream/main
```

## 常见坑点

- **后端默认端口是 3000**：本地开发必须加 `SERVER_PORT=8080`，否则会和前端 Vite 的 3000 端口冲突
- **pnpm-lock.yaml 必须同步提交**：改了 `package.json` 后执行 `cd frontend && pnpm install && git add pnpm-lock.yaml`
- **npm 与 pnpm 的 node_modules 冲突**：`rm -rf node_modules && pnpm install`
- **Go interface 新增方法**：必须补全所有 Stub/Mock struct 的实现
- **Ent Schema 修改**：必须 `go generate ./ent`，生成的文件也要提交
- **VERSION 文件**：合并上游后必须更新 `backend/cmd/server/VERSION`，否则网站持续提示有新版本
- **批量修改账号**：按平台分组，不要混选不同平台账号，避免模型映射被覆盖

## 项目结构

```text
apipool/
├── backend/
│   ├── cmd/server/          # 主程序入口 + VERSION 文件
│   ├── ent/schema/          # 数据库 Schema 定义（Ent ORM）
│   ├── internal/
│   │   ├── handler/         # HTTP 处理器
│   │   ├── service/         # 业务逻辑
│   │   ├── repository/      # 数据访问层
│   │   ├── config/          # 配置管理
│   │   ├── setup/           # 安装向导
│   │   └── server/          # 服务器 & 中间件
│   └── migrations/          # 数据库迁移脚本
├── frontend/src/
│   ├── api/                 # API 调用
│   ├── components/          # Vue 组件
│   ├── views/               # 页面视图
│   ├── stores/              # Pinia 状态管理
│   ├── i18n/                # 国际化
│   └── router/              # 路由配置
├── deploy/                  # 部署配置
└── .github/workflows/       # CI/CD
```

## 许可证

本项目遵循 [GNU Lesser General Public License v3.0](LICENSE)（或更高版本）。

Copyright (c) 2026 Wesley Liddick
