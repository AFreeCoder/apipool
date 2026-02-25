# APIPool 项目开发指南

> 本文档记录项目环境配置、常见坑点和注意事项，供 Claude Code 和团队成员参考。

## 一、项目基本信息

| 项目 | 说明 |
|------|------|
| **品牌名** | APIPool（上游原名 Sub2API） |
| **上游仓库** | [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api) |
| **自有仓库** | [AFreeCoder/apipool](https://github.com/AFreeCoder/apipool)（private） |
| **技术栈** | Go 后端 (Ent ORM + Gin) + Vue 3 前端 (TypeScript + Vite + Pinia) |
| **数据库** | PostgreSQL 16 + Redis 7 |
| **包管理** | 后端: go modules / 前端: npm（上游 CI 用 pnpm） |

## 二、本地开发环境（macOS）

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

### 本地测试环境凭据

| 服务 | 配置项 | 值 |
|------|--------|-----|
| **PostgreSQL** | 容器名 | `apipool-postgres` |
| | 端口 | `5432` |
| | 用户 | `sub2api` |
| | 密码 | `sub2api` |
| | 数据库 | `sub2api` |
| **Redis** | 容器名 | `apipool-redis` |
| | 端口 | `6379` |
| | 密码 | 无 |
| **后端** | 端口 | `8080` |
| | 模式 | `debug` |
| **前端** | 端口 | `3000`（Vite 开发服务器） |
| | 代理 | `/api`、`/setup` → `localhost:8080` |
| **管理员** | 邮箱 | `admin@apipool.local` |
| | 密码 | 首次安装向导时设置 |

### 启动开发

```bash
# 1. 启动 Docker 基础服务
docker start apipool-postgres apipool-redis

# 2. 启动后端（需要设置环境变量）
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

> **首次启动说明**：后端首次启动会进入安装向导（Setup Wizard），通过前端页面 `http://localhost:3000` 完成数据库配置和管理员账号设置。完成后后端自动重启进入正常模式。

### 开发工具

```bash
# golangci-lint v2.7
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.7
```

## 三、CI/CD 流水线

### GitHub Actions Workflows

| Workflow | 触发条件 | 检查内容 |
|----------|----------|----------|
| **deploy.yml** | push to main | SSH 部署到 DigitalOcean |
| **backend-ci.yml** | push, pull_request | 单元测试 + 集成测试 + golangci-lint v2.7 |
| **security-scan.yml** | push, pull_request, 每周一 | govulncheck + gosec + pnpm audit |
| **release.yml** | tag `v*` | 构建发布 |

### CI 要求

- Go 版本必须是 **1.25.7**
- 前端 CI 使用 `pnpm install --frozen-lockfile`，需提交 `pnpm-lock.yaml`

### 本地测试命令

```bash
# 后端单元测试
cd backend && go test -tags=unit ./...

# 后端集成测试
cd backend && go test -tags=integration ./...

# 代码质量检查
cd backend && golangci-lint run ./...
```

## 四、常见坑点 & 解决方案

### 坑 1：pnpm-lock.yaml 必须同步提交

**问题**：`package.json` 新增依赖后，CI 的 `pnpm install --frozen-lockfile` 失败。

**原因**：上游 CI 使用 pnpm，lock 文件不同步会报错。

**解决**：
```bash
cd frontend
pnpm install  # 更新 pnpm-lock.yaml
git add pnpm-lock.yaml
git commit -m "chore: update pnpm-lock.yaml"
```

---

### 坑 2：npm 和 pnpm 的 node_modules 冲突

**问题**：之前用 npm 装过 `node_modules`，pnpm install 报 `EPERM` 错误。

**解决**：
```bash
cd frontend
rm -rf node_modules
pnpm install
```

---

### 坑 3：Go interface 新增方法后 test stub 必须补全

**问题**：给 interface 新增方法后，编译报错 `does not implement interface (missing method XXX)`。

**原因**：所有测试文件中实现该 interface 的 stub/mock 都必须补上新方法。

**解决**：
```bash
# 搜索所有实现该 interface 的 struct
cd backend
grep -r "type.*Stub.*struct" internal/
grep -r "type.*Mock.*struct" internal/

# 逐一补全新方法
```

---

### 坑 4：Ent Schema 修改后必须重新生成

**问题**：修改 `ent/schema/*.go` 后，代码不生效。

**解决**：
```bash
cd backend
go generate ./ent  # 重新生成 ent 代码
git add ent/       # 生成的文件也要提交
```

---

### 坑 5：VERSION 文件必须与上游 tag 同步

**问题**：合并上游代码后，网站持续提示有新版本。

**原因**：`backend/cmd/server/VERSION` 通过 `//go:embed` 嵌入二进制，前端比较 `currentVersion` 与 GitHub 最新 release 的 `latestVersion`。

**解决**：合并上游后检查最新 tag 并更新 VERSION 文件：
```bash
git tag --sort=-v:refname | head -1
echo “0.1.83” > backend/cmd/server/VERSION
```

---

### 坑 10：前端测试看似正常，但后端调用失败（模型映射被批量误改）

**典型现象**：
- 前端按钮点测看起来正常；
- 实际通过 API/客户端调用时返回 `Service temporarily unavailable` 或提示无可用账号；
- 常见于 OpenAI 账号（例如 Codex 模型）在批量修改后突然不可用。

**根因**：
- OpenAI 账号编辑页默认不显式展示映射规则，容易让人误以为”没映射也没关系”；
- 但在**批量修改同时选中不同平台账号**（OpenAI + Antigravity/Gemini）时，模型白名单/映射可能被跨平台策略覆盖；
- 结果是 OpenAI 账号的关键模型映射丢失或被改坏，后端选不到可用账号。

**修复方案（按优先级）**：
1. **快速修复（推荐）**：在批量修改中补回正确的透传映射（例如 `gpt-5.3-codex -> gpt-5.3-codex-spark`）。
2. **彻底重建**：删除并重新添加全部相关账号（最稳但成本高）。

**关键经验**：
- 如果某模型已被软件内置默认映射覆盖，通常不需要额外再加透传；
- 但当上游模型更新快于本仓库默认映射时，**手动批量添加透传映射**是最简单、最低风险的临时兜底方案；
- 批量操作前尽量按平台分组，不要混选不同平台账号。

---

### 坑 11：PR 提交前检查清单

提交 PR 前务必本地验证：

- [ ] `go test -tags=unit ./...` 通过
- [ ] `go test -tags=integration ./...` 通过
- [ ] `golangci-lint run ./...` 无新增问题
- [ ] `pnpm-lock.yaml` 已同步（如果改了 package.json）
- [ ] 所有 test stub 补全新接口方法（如果改了 interface）
- [ ] Ent 生成的代码已提交（如果改了 schema）

## 五、常用命令速查

### Docker 服务管理

```bash
# 启动基础服务
docker start apipool-postgres apipool-redis

# 停止基础服务
docker stop apipool-postgres apipool-redis

# 查看容器状态
docker ps --filter "name=apipool"

# 连接 PostgreSQL
docker exec -it apipool-postgres psql -U sub2api -d sub2api

# 连接 Redis
docker exec -it apipool-redis redis-cli
```

### Git 操作

```bash
# 同步上游
git fetch upstream
git checkout main
git merge upstream/main

# 更新 VERSION 后提交
echo "x.y.z" > backend/cmd/server/VERSION
git add backend/cmd/server/VERSION
git commit -m "chore: update version to x.y.z"
git push origin main
```

### 前端操作

```bash
cd frontend

# 安装依赖
npm install

# 开发服务器
npm run dev

# 构建（输出到 backend/internal/web/dist）
npm run build
```

### 后端操作

```bash
cd backend

# 运行服务器
go run ./cmd/server/

# 嵌入前端构建
go build -tags embed -o apipool ./cmd/server

# 生成 Ent 代码
go generate ./ent

# 运行测试
go test -tags=unit ./...
go test -tags=integration ./...

# Lint 检查
golangci-lint run ./...
```

## 六、项目结构速览

```
sub2api/
├── backend/
│   ├── cmd/server/          # 主程序入口 + VERSION 文件
│   ├── ent/                 # Ent ORM 生成代码
│   │   └── schema/          # 数据库 Schema 定义
│   ├── internal/
│   │   ├── handler/         # HTTP 处理器
│   │   ├── service/         # 业务逻辑
│   │   ├── repository/      # 数据访问层
│   │   ├── config/          # 配置管理
│   │   ├── setup/           # 安装向导
│   │   └── server/          # 服务器 & 中间件
│   └── migrations/          # 数据库迁移脚本
├── frontend/
│   ├── src/
│   │   ├── api/             # API 调用
│   │   ├── components/      # Vue 组件
│   │   ├── views/           # 页面视图
│   │   ├── stores/          # Pinia 状态管理
│   │   ├── i18n/            # 国际化（中/英）
│   │   └── router/          # 路由配置
│   └── package.json
├── deploy/                  # 部署配置（Docker Compose / systemd / Caddy）
├── .github/workflows/       # CI/CD
└── CLAUDE.md                # Claude Code 项目指令
```

## 七、参考资源

- [上游仓库](https://github.com/Wei-Shaw/sub2api)
- [Ent 文档](https://entgo.io/docs/getting-started)
- [Vue 3 文档](https://vuejs.org/)
- [Vite 文档](https://vite.dev/)
