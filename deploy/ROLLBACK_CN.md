# APIPool / Sub2API 生产回退手册

本文档基于当前实际部署环境整理：

- 服务器：`digitalocean`
- 部署目录：`/opt/sub2api`
- 部署方式：`docker compose -f docker-compose.deploy.yml build && up -d`
- 应用容器：`sub2api`
- 数据库容器：`sub2api-postgres`
- Redis 容器：`sub2api-redis`

截至 2026-03-14，近期上游同步新增的迁移包括：

- `068_add_announcement_notify_mode.sql`
- `069_add_group_messages_dispatch.sql`
- `070_add_scheduled_test_auto_recover.sql`
- `070_add_usage_log_service_tier.sql`
- `071_add_gemini25_flash_image_to_model_mapping.sql`
- `071_add_usage_billing_dedup.sql`
- `072_add_usage_billing_dedup_created_at_brin_notx.sql`
- `073_add_usage_billing_dedup_archive.sql`

这些迁移整体仍偏向“新增列 / 新映射 / 新配置”，但已经不适合再简单假设“任意旧代码都能无风险运行在新 schema 上”。当前推荐回退原则是：

- 第一优先级：先回退应用版本
- 第二优先级：确认问题是否真的指向数据库状态
- 最后手段：才恢复数据库
- 如果要恢复数据库，必须同时明确“恢复后要启动哪个应用版本”

## 1. 部署前固定动作

推荐统一使用 `deploy/rollback.sh`。

### 1.1 通过 GitHub Actions 部署

当前 [deploy.yml](../.github/workflows/deploy.yml) 已经在真正构建新镜像前自动执行两件事：

- 生成 `pre-deploy-YYYYmmdd_HHMMSS.sql.gz` 数据库备份
- 给当前线上应用镜像打回退 tag

自动生成的镜像 tag 包括：

- `deploy-sub2api:rollback-latest`
- `deploy-sub2api:rollback-YYYYmmdd_HHMMSS-<当前线上commit>`

同时会写一份元数据文件：

- `/opt/sub2api/backups/last-rollback-image.txt`

因此，**如果是走 CI 正式部署，不需要额外手工做 prep**。

### 1.2 手工 SSH 部署

如果本次不是走 GitHub Actions，而是手工 SSH 到服务器执行部署，必须先做一次 prep：

```bash
ssh digitalocean 'cd /opt/sub2api/deploy && ./rollback.sh prep'
```

这条命令会同时完成：

- 给当前运行中的 `deploy-sub2api:latest` 打回退 tag
- 生成一份 `pre-deploy-*.sql.gz` 数据库备份

如果线上启用了本地 Sora 存储，或者依赖 `/app/data` 中的本地配置 / 媒体 / 日志，还应额外备份 `sub2api_data` 卷：

```bash
ssh digitalocean '
  mkdir -p /opt/sub2api/backups &&
  docker run --rm \
    -v sub2api_data:/data \
    -v /opt/sub2api/backups:/backup \
    alpine sh -c "cd /data && tar czf /backup/sub2api-data-$(date +%Y%m%d_%H%M%S).tar.gz ."
'
```

如果只想单独做其中一步：

```bash
ssh digitalocean 'cd /opt/sub2api/deploy && ./rollback.sh tag-current'
ssh digitalocean 'cd /opt/sub2api/deploy && ./rollback.sh backup-db'
```

查看当前回退镜像 tag：

```bash
ssh digitalocean 'docker images --format "table {{.Repository}}\t{{.Tag}}\t{{.ID}}" | grep "^deploy-sub2api"'
```

查看最近一次自动记录的回退镜像信息：

```bash
ssh digitalocean 'cat /opt/sub2api/backups/last-rollback-image.txt'
```

## 2. 最快回退：镜像热回退

适用场景：

- 新版本已部署完成
- `sub2api` 容器启动后接口异常、健康检查异常、业务行为异常
- PostgreSQL / Redis 正常

这是最快方案，通常只需要几十秒。

默认直接回退到 `deploy-sub2api:rollback-latest`：

```bash
ssh digitalocean 'cd /opt/sub2api/deploy && ./rollback.sh image'
```

如果要回退到某个特定历史镜像 tag：

```bash
ssh digitalocean 'cd /opt/sub2api/deploy && ./rollback.sh image deploy-sub2api:rollback-YYYYmmdd_HHMMSS-<commit>'
```

注意：

- 不要执行 `docker compose down`
- 不要先停 `postgres` 和 `redis`
- `--no-deps --force-recreate` 比单纯 `up -d sub2api` 更稳

## 3. 二级回退：源码回退后重建

适用场景：

- 部署前没有打镜像回退 tag
- 回退 tag 被清理或找不到
- 需要明确回到某个 git commit

```bash
ssh digitalocean 'cd /opt/sub2api/deploy && ./rollback.sh source <commit>'
```

如果本次需要回到“部署前的稳定点”，先从服务器记录里拿到部署前 commit，再执行源码回退：

```bash
ssh digitalocean 'cat /opt/sub2api/backups/last-rollback-image.txt'
ssh digitalocean 'cd /opt/sub2api && git reflog --date=iso --max-count=20'
ssh digitalocean 'cd /opt/sub2api/deploy && ./rollback.sh source <部署前稳定commit>'
```

## 4. 数据库恢复：最后手段

只有下面情况才考虑恢复数据库：

- 确认 migration 本身执行错误
- 新版本写入了错误数据，旧代码无法自恢复
- 应用热回退后问题依然存在，并且证据指向数据库状态
- 需要把数据状态回退到部署前快照

通过 GitHub Actions 正式部署时，部署工作流会在构建前自动生成：

- `/opt/sub2api/backups/pre-deploy-YYYYmmdd_HHMMSS.sql.gz`

服务器上还有定时备份：

- `/opt/sub2api/backups/scheduled-YYYYmmdd_HHMMSS.sql.gz`

恢复步骤：

1. 先确定恢复后要启动哪个应用版本。
   优先使用镜像回退：

```bash
ssh digitalocean 'cd /opt/sub2api/deploy && ./rollback.sh db-restore --with-image'
```

如果要恢复后直接切到某个指定回退镜像：

```bash
ssh digitalocean "cd /opt/sub2api/deploy && ./rollback.sh db-restore --with-image --image-tag deploy-sub2api:rollback-YYYYmmdd_HHMMSS-<commit>"
```

如果没有可用回退镜像，而是要回到某个 git commit：

```bash
ssh digitalocean 'cd /opt/sub2api/deploy && ./rollback.sh db-restore --with-source <commit>'
```

如果你已经确认“当前应用版本本身没问题，只需要恢复数据库”，才允许显式使用：

```bash
ssh digitalocean 'cd /opt/sub2api/deploy && ./rollback.sh db-restore --allow-current-app'
```

如果要恢复指定备份文件，把备份路径放在命令里即可：

```bash
ssh digitalocean 'cd /opt/sub2api/deploy && ./rollback.sh db-restore /opt/sub2api/backups/pre-deploy-YYYYmmdd_HHMMSS.sql.gz --with-image'
```

注意：

- `db-restore` 现在要求显式声明恢复后的应用策略，防止数据库恢复完又把当前坏版本重新拉起。
- 如果问题来自代码或配置，不要只做 `db-restore`；应优先用 `image` 或 `source` 回退应用。
- 数据库恢复过程中必须保持应用停止状态，避免继续写入。
- 如果线上依赖 `/app/data` 本地数据，数据库回退前后要一并考虑卷级快照和恢复。
- 恢复完成后优先检查 `sub2api` 健康状态、登录、网关转发、管理后台查询。

## 5. 故障分类与动作建议

### 5.1 build 失败

表现：

- GitHub Actions 在 `docker compose build` 阶段失败
- 线上旧容器没有被替换

动作：

- 不需要回退
- 直接修代码后重新部署

### 5.2 新容器启动失败 / 健康检查失败

表现：

- `sub2api` 容器 `unhealthy` / `exited`
- 核心接口异常

动作：

- 先走第 2 节“镜像热回退”

### 5.3 热回退后仍异常

动作：

- 再走第 3 节“源码回退重建”
- 同时查看最近 migration、配置变更、环境变量变更

### 5.4 明确是数据库问题

动作：

- 先决定恢复后要启动的应用版本
- 再走第 4 节“数据库恢复”
- 不要只恢复数据库后直接拉起当前坏版本

## 6. 回退后必须做的事

服务恢复只是第一步，后面还必须处理源头，否则下次部署会再次把坏版本打上去。

最少要做：

1. 在本地对问题提交执行 `git revert`，或提交修复代码
2. 推送到 `origin/main`
3. 确认下一次部署不会重新发布坏版本

## 7. 常用检查命令

查看当前运行状态：

```bash
ssh digitalocean '
  docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Image}}"
'
```

查看应用日志：

```bash
ssh digitalocean 'docker logs --tail 200 -f sub2api'
```

查看当前线上代码版本：

```bash
ssh digitalocean '
  cd /opt/sub2api
  git rev-parse HEAD
  git log --oneline -1
'
```

查看已执行迁移：

```bash
ssh digitalocean "docker exec sub2api-postgres psql -U sub2api -d sub2api -Atc 'select filename from schema_migrations order by filename desc limit 10;'"
```
