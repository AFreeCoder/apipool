# APIPool

基于 [Sub2API](https://github.com/Wei-Shaw/sub2api) 的 AI API 网关平台，用于订阅配额的分发与管理。

在线访问：**https://apipool.dev**

## 功能特性

- **多账号管理** - 支持多种上游账号类型（OAuth、API Key）
- **API Key 分发** - 为用户生成和管理 API Key
- **精确计费** - Token 级别的用量追踪与费用计算
- **智能调度** - 智能账号选择与会话粘滞
- **并发控制** - 用户级与账号级并发限制
- **限速策略** - 可配置的请求与 Token 限速
- **管理后台** - Web 界面进行监控与管理

## 技术栈

| 组件 | 技术 |
|------|------|
| 后端 | Go 1.25.7, Gin, Ent ORM |
| 前端 | Vue 3 + TypeScript, Vite, Pinia, TailwindCSS |
| 数据库 | PostgreSQL 16 |
| 缓存 | Redis 7 |
| 包管理 | 后端: go modules / 前端: npm（CI 用 pnpm） |

## 部署

部署在 **silicon** 服务器上，使用 Docker Compose 方式运行。

```bash
ssh silicon                   # 登录服务器
docker compose up -d          # 启动服务
docker compose down           # 停止服务
docker compose logs -f        # 查看日志
docker compose pull && docker compose up -d  # 更新镜像
```

## 本地开发

### Docker 基础服务

```bash
# PostgreSQL 16
docker run -d --name apipool-postgres \
  -e POSTGRES_USER=sub2api \
  -e POSTGRES_PASSWORD=sub2api \
  -e POSTGRES_DB=sub2api \
  -p 5432:5432 \
  postgres:16-alpine

# Redis 7
docker run -d --name apipool-redis \
  -p 6379:6379 \
  redis:7-alpine
```

### 本地环境凭据

| 服务 | 配置项 | 值 |
|------|--------|-----|
| PostgreSQL | 用户/密码/数据库 | `sub2api` / `sub2api` / `sub2api` |
| PostgreSQL | 端口 | `5432` |
| Redis | 端口 | `6379`（无密码） |
| 后端 | 端口/模式 | `8080` / `debug` |
| 前端 | 端口 | `3000`（Vite，代理 `/api`、`/setup` → `localhost:8080`） |

### 启动开发

```bash
# 1. 启动 Docker 基础服务
docker start apipool-postgres apipool-redis

# 2. 启动后端
cd backend
DATABASE_HOST=127.0.0.1 DATABASE_PORT=5432 \
DATABASE_USER=sub2api DATABASE_PASSWORD=sub2api DATABASE_DBNAME=sub2api \
REDIS_HOST=127.0.0.1 REDIS_PORT=6379 \
SERVER_MODE=debug \
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
| deploy.yml | push to main | SSH 部署到服务器 |
| backend-ci.yml | push, PR | 单元测试 + 集成测试 + golangci-lint v2.7 |
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

- **pnpm-lock.yaml 必须同步提交**：改了 `package.json` 后执行 `cd frontend && pnpm install && git add pnpm-lock.yaml`
- **npm 与 pnpm 的 node_modules 冲突**：`rm -rf node_modules && pnpm install`
- **Go interface 新增方法**：必须补全所有 Stub/Mock struct 的实现
- **Ent Schema 修改**：必须 `go generate ./ent`，生成的文件也要提交
- **VERSION 文件**：合并上游后必须更新 `backend/cmd/server/VERSION`，否则网站持续提示有新版本
- **批量修改账号**：按平台分组，不要混选不同平台账号，避免模型映射被覆盖

## 项目结构

```
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

MIT License
