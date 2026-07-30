#!/usr/bin/env bash
set -Eeuo pipefail

# 本文件由 owner 安装到 /opt/sub2api/deploy 后才允许生产 Runner 调用。
# Actions workspace 里的同名文件永远不以 root 身份执行。
umask 077

APP_DIR="${SUB2API_DEPLOY_ROOT:-/opt/sub2api}"
DEPLOY_DIR="$APP_DIR/deploy"
COMPOSE_FILE="${SUB2API_COMPOSE_FILE:-docker-compose.deploy.yml}"
ENV_FILE="${SUB2API_ENV_FILE:-.env}"
RELEASE_FILE="${SUB2API_RELEASE_FILE:-release.env}"
BACKUP_DIR="${SUB2API_BACKUP_DIR:-$APP_DIR/backups}"
LOCK_FILE="${SUB2API_DEPLOY_LOCK:-/run/sub2api-deploy.lock}"
IMAGE_REPO="${SUB2API_GHCR_IMAGE:-ghcr.io/afreecoder/apipool}"
LOCAL_IMAGE="deploy-sub2api"
IMAGE_TAG="${1:-}"

log() {
  printf '[deploy] %s\n' "$*"
}

fail() {
  printf '[deploy] ERROR: %s\n' "$*" >&2
  exit 1
}

if [[ ! "$IMAGE_TAG" =~ ^sha-[0-9a-f]{40}$ ]]; then
  fail "用法: $0 sha-<40位commit>"
fi

exec 9>"$LOCK_FILE"
flock -n 9 || {
  log "已有生产发布正在运行"
  exit 75
}

for required in \
  "$DEPLOY_DIR/$COMPOSE_FILE" \
  "$DEPLOY_DIR/$ENV_FILE" \
  "$DEPLOY_DIR/configure-caddy.sh"; do
  [ -f "$required" ] || fail "缺少 root-owned 部署文件: $required"
done

if [ -e "$DEPLOY_DIR/.env.biz" ] || [ -e "$DEPLOY_DIR/docker-compose.biz.yml" ]; then
  fail "目标机禁止部署 biz；请移除活动部署目录中的 biz 配置"
fi

env_owner="$(stat -c '%u' "$DEPLOY_DIR/$ENV_FILE")"
env_mode="$(stat -c '%a' "$DEPLOY_DIR/$ENV_FILE")"
if [ "$env_owner" != "0" ] || (( (8#$env_mode & 8#077) != 0 )); then
  fail "$ENV_FILE 必须为 root 所有且仅 owner 可读写"
fi

mkdir -p "$BACKUP_DIR"
chmod 700 "$BACKUP_DIR"
cd "$DEPLOY_DIR"

compose() {
  docker compose \
    --project-name sub2api \
    --env-file "$ENV_FILE" \
    -f "$COMPOSE_FILE" \
    "$@"
}

container_healthy() {
  local name="$1"
  [ "$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$name" 2>/dev/null || true)" = "healthy" ]
}

wait_for_health() {
  local name="$1"
  local timeout="${2:-300}"
  local elapsed=0
  local state=""

  while [ "$elapsed" -lt "$timeout" ]; do
    state="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$name" 2>/dev/null || true)"
    case "$state" in
      healthy)
        log "$name 状态正常"
        return 0
        ;;
      unhealthy|exited|dead)
        docker logs --tail 200 "$name" || true
        fail "$name 状态异常: $state"
        ;;
    esac
    sleep 5
    elapsed=$((elapsed + 5))
  done

  docker logs --tail 200 "$name" || true
  fail "等待 $name 健康检查超时"
}

current_release_tag=""
if [ -f "$RELEASE_FILE" ]; then
  current_release_tag="$(sed -n 's/^IMAGE_TAG=//p' "$RELEASE_FILE" | tail -1)"
fi

expected_image="${IMAGE_REPO}:${IMAGE_TAG}"

# 即使镜像相同也要收敛 Caddy fragment；该操作只 validate + 原子替换 +
# reload，不会重建应用容器。
log "校验并安装 APIPool Caddy 分片"
"$DEPLOY_DIR/configure-caddy.sh"

if [ "$current_release_tag" = "$IMAGE_TAG" ] \
  && container_healthy sub2api-postgres \
  && container_healthy sub2api-redis \
  && container_healthy sub2api; then
  expected_id="$(docker image inspect --format '{{.Id}}' "$expected_image" 2>/dev/null || true)"
  running_id="$(docker inspect --format '{{.Image}}' sub2api 2>/dev/null || true)"
  if [ -n "$expected_id" ] && [ "$expected_id" = "$running_id" ]; then
    log "$IMAGE_TAG 已健康运行；本次同 SHA 发布幂等跳过"
    exit 0
  fi
fi

compose config -q
if ! compose config --format json \
  | grep -Eq '"host_ip"[[:space:]]*:[[:space:]]*"127\.0\.0\.1"'; then
  fail "应用端口没有绑定到 127.0.0.1"
fi

free_kb="$(df -Pk "$APP_DIR" | awk 'NR==2 {print $4}')"
if [ -z "$free_kb" ] || [ "$free_kb" -lt $((10 * 1024 * 1024)) ]; then
  fail "$APP_DIR 所在文件系统可用空间不足 10GiB"
fi

if docker ps --filter 'name=^sub2api-postgres$' --format '{{.Names}}' | grep -q .; then
  db_user="$(docker exec sub2api-postgres printenv POSTGRES_USER)"
  db_name="$(docker exec sub2api-postgres printenv POSTGRES_DB)"
  [ -n "$db_user" ] && [ -n "$db_name" ] || fail "无法读取数据库名称或用户"

  backup_tmp="$(mktemp "$BACKUP_DIR/.pre-deploy.XXXXXX.sql.gz")"
  backup_file="$BACKUP_DIR/pre-deploy-$(date -u +%Y%m%dT%H%M%SZ).sql.gz"
  log "创建发布前 PostgreSQL 全量备份"
  if docker exec sub2api-postgres \
      pg_dump -U "$db_user" -d "$db_name" --clean --if-exists \
      | gzip >"$backup_tmp" \
    && [ -s "$backup_tmp" ] \
    && gzip -t "$backup_tmp"; then
    chmod 600 "$backup_tmp"
    mv "$backup_tmp" "$backup_file"
    log "备份完成: $backup_file"
  else
    rm -f "$backup_tmp"
    fail "发布前数据库备份失败"
  fi
else
  log "首次启动尚无 PostgreSQL 容器；数据库恢复/复制必须由迁移门禁提前完成"
fi

old_image_id="$(docker image inspect --format '{{.Id}}' "${LOCAL_IMAGE}:latest" 2>/dev/null || true)"
old_tag="$current_release_tag"
if [ -n "$old_image_id" ]; then
  rollback_stamp="$(date -u +%Y%m%dT%H%M%SZ)"
  rollback_tag="${LOCAL_IMAGE}:rollback-${rollback_stamp}"
  docker tag "${LOCAL_IMAGE}:latest" "$rollback_tag"
  docker tag "${LOCAL_IMAGE}:latest" "${LOCAL_IMAGE}:rollback-latest"
  {
    echo "created_at=$rollback_stamp"
    echo "source_image_tag=${current_release_tag:-unknown}"
    echo "rollback_tag=$rollback_tag"
    echo "rollback_alias=${LOCAL_IMAGE}:rollback-latest"
  } >"$BACKUP_DIR/last-rollback-image.txt"
fi

log "拉取不可变镜像 $expected_image"
docker pull "$expected_image"
pulled_id="$(docker image inspect --format '{{.Id}}' "$expected_image")"
docker tag "$expected_image" "${LOCAL_IMAGE}:latest"

rollback_app() {
  if [ -z "$old_image_id" ]; then
    return
  fi
  log "新应用健康检查失败，恢复上一镜像"
  docker tag "${LOCAL_IMAGE}:rollback-latest" "${LOCAL_IMAGE}:latest"
  compose up -d --no-deps --force-recreate sub2api || true
}

trap rollback_app ERR
compose up -d
wait_for_health sub2api-postgres 180
wait_for_health sub2api-redis 180
wait_for_health sub2api 300

case "$(docker port sub2api 8080/tcp 2>/dev/null || true)" in
  127.0.0.1:*) ;;
  *) fail "应用端口运行时绑定异常" ;;
esac

running_id="$(docker inspect --format '{{.Image}}' sub2api)"
[ "$running_id" = "$pulled_id" ] || fail "运行镜像与精确发布镜像不一致"

release_tmp="$(mktemp "$DEPLOY_DIR/.release.XXXXXX")"
{
  echo "IMAGE_TAG=$IMAGE_TAG"
  echo "IMAGE_ID=$pulled_id"
  echo "DEPLOYED_AT=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  if [ -n "$old_tag" ]; then
    echo "PREVIOUS_IMAGE_TAG=$old_tag"
  fi
} >"$release_tmp"
chmod 600 "$release_tmp"
mv "$release_tmp" "$RELEASE_FILE"
trap - ERR

find "$BACKUP_DIR" -maxdepth 1 -type f -name 'pre-deploy-*.sql.gz' -mtime +7 -delete
docker image prune -f >/dev/null
log "已发布 $IMAGE_TAG"
