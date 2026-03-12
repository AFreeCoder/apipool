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

详细说明见 [deploy/ROLLBACK_CN.md](/Users/afreecoder/project/apipool/deploy/ROLLBACK_CN.md)。

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

- **后端默认端口是 3000**：本地开发必须加 `SERVER_PORT=8080`，否则会和前端 Vite 的 3000 端口冲突
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
