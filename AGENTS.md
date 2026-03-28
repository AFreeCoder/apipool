# 项目规范

> **重要**：本地开发环境凭据、项目结构、常用命令、常见坑点等信息请查阅 [README.md](./README.md)。

本项目 fork 自 [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api)。

## Git 远程配置

- `origin`: [AFreeCoder/apipool](https://github.com/AFreeCoder/apipool)（private）
- `upstream`: [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api)

## 同步上游代码流程

1. `git fetch upstream`
2. `git merge upstream/main`
3. 检查上游最新 tag（`git tag --sort=-v:refname | head -1`），将版本号写入 `backend/cmd/server/VERSION`
   - 该文件通过 `//go:embed VERSION` 嵌入二进制，前端据此判断是否有新版本
   - **必须**更新，否则网站会持续提示有新版本
4. 提交并推送到 `origin/main`

## CI 约束

- Go 版本必须是 **1.25.7**
- 前端 CI 使用 `pnpm install --frozen-lockfile`，改了 `package.json` 必须同步提交 `pnpm-lock.yaml`

## 开发规则

- 修改 `ent/schema/*.go` 后必须执行 `go generate ./ent`，生成的文件一并提交
- 给 interface 新增方法后，必须补全所有 test stub/mock 的实现
- 批量修改账号时按平台分组，不要混选不同平台账号，避免模型映射被覆盖

## PR 提交前检查清单

- `go test -tags=unit ./...` 通过
- `go test -tags=integration ./...` 通过
- `golangci-lint run ./...` 无新增问题
- `pnpm-lock.yaml` 已同步（如果改了 package.json）
- test stub 补全新接口方法（如果改了 interface）
- Ent 生成的代码已提交（如果改了 schema）
