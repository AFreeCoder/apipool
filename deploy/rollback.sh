#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/version_resolver.sh"

APP_DIR="${APP_DIR:-/opt/sub2api}"
DEPLOY_DIR="${DEPLOY_DIR:-$APP_DIR/deploy}"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.deploy.yml}"
BACKUP_DIR="${BACKUP_DIR:-$APP_DIR/backups}"
IMAGE_REPO="${IMAGE_REPO:-deploy-sub2api}"
APP_SERVICE="${APP_SERVICE:-sub2api}"
POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-sub2api-postgres}"
REDIS_CONTAINER="${REDIS_CONTAINER:-sub2api-redis}"
PREDEPLOY_BACKUP_RETENTION_HOURS="${PREDEPLOY_BACKUP_RETENTION_HOURS:-168}"

log() {
  printf '[rollback] %s\n' "$*"
}

fail() {
  printf '[rollback] ERROR: %s\n' "$*" >&2
  exit 1
}

wait_for_health() {
  name="$1"
  timeout="${2:-300}"
  elapsed=0

  while [ "$elapsed" -lt "$timeout" ]; do
    state="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$name" 2>/dev/null || true)"
    case "$state" in
      healthy|running)
        log "$name 状态正常: $state"
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

ensure_paths() {
  [ -d "$APP_DIR" ] || fail "APP_DIR 不存在: $APP_DIR"
  [ -d "$DEPLOY_DIR" ] || fail "DEPLOY_DIR 不存在: $DEPLOY_DIR"
  mkdir -p "$BACKUP_DIR"
}

current_commit() {
  if [ -d "$APP_DIR/.git" ]; then
    (
      cd "$APP_DIR"
      git rev-parse --short=12 HEAD 2>/dev/null || echo "unknown"
    )
  else
    echo "unknown"
  fi
}

current_version() {
  read_version_file "$APP_DIR" || true
}

create_rollback_tag() {
  ensure_paths

  strict="${1:-1}"

  if ! docker image inspect "${IMAGE_REPO}:latest" >/dev/null 2>&1; then
    if [ "$strict" = "0" ]; then
      log "未找到 ${IMAGE_REPO}:latest，跳过回退镜像打标"
      return 0
    fi
    fail "未找到 ${IMAGE_REPO}:latest，无法创建回退镜像 tag"
  fi

  ts="$(date +%Y%m%d_%H%M%S)"
  commit="$(current_commit)"
  rollback_tag="${IMAGE_REPO}:rollback-${ts}-${commit}"

  docker tag "${IMAGE_REPO}:latest" "$rollback_tag"
  docker tag "${IMAGE_REPO}:latest" "${IMAGE_REPO}:rollback-latest"

  {
    echo "created_at=$ts"
    echo "source_commit=$commit"
    echo "rollback_tag=$rollback_tag"
    echo "rollback_alias=${IMAGE_REPO}:rollback-latest"
  } > "$BACKUP_DIR/last-rollback-image.txt"

  log "已创建回退镜像标签: $rollback_tag"
  log "已更新回退镜像别名: ${IMAGE_REPO}:rollback-latest"
}

backup_db() {
  ensure_paths

  if ! docker ps --filter "name=^${POSTGRES_CONTAINER}$" --format '{{.Names}}' | grep -q .; then
    log "未发现运行中的 ${POSTGRES_CONTAINER}，跳过数据库备份"
    return 0
  fi

  db_user="$(docker exec "$POSTGRES_CONTAINER" printenv POSTGRES_USER)"
  db_name="$(docker exec "$POSTGRES_CONTAINER" printenv POSTGRES_DB)"
  [ -n "$db_user" ] || fail "无法读取 POSTGRES_USER"
  [ -n "$db_name" ] || fail "无法读取 POSTGRES_DB"

  backup_file="${BACKUP_DIR}/pre-deploy-$(date +%Y%m%d_%H%M%S).sql.gz"
  log "开始数据库备份: $backup_file"

  docker exec "$POSTGRES_CONTAINER" pg_dump -U "$db_user" -d "$db_name" --clean --if-exists | gzip > "$backup_file"
  gzip -t "$backup_file"
  find "$BACKUP_DIR" -maxdepth 1 -type f -name 'pre-deploy-*.sql.gz' -mmin "+$((PREDEPLOY_BACKUP_RETENTION_HOURS * 60))" -delete

  log "数据库备份完成: $backup_file"
}

prep() {
  create_rollback_tag 0
  backup_db
}

build_app_image_from_current_source() {
  ensure_paths
  [ -f "$DEPLOY_DIR/.env" ] || fail "缺少 $DEPLOY_DIR/.env"

  file_version="$(current_version)"
  tag_version="$(latest_release_tag_version "$APP_DIR" || true)"
  app_version="$(resolve_app_version "$APP_DIR" || true)"
  [ -n "$app_version" ] || fail "无法读取版本号"

  if [ -n "$file_version" ] && [ -n "$tag_version" ] && [ "$file_version" != "$tag_version" ]; then
    log "VERSION 文件($file_version)与最新发布 tag($tag_version)不一致，构建版本以 VERSION 文件为准"
  fi

  app_commit="$(
    cd "$APP_DIR"
    git rev-parse --short=12 HEAD
  )"
  app_build_date="$(
    cd "$APP_DIR"
    git log -1 --format=%cI HEAD
  )"

  (
    cd "$APP_DIR"
    docker build \
      -t "${IMAGE_REPO}:latest" \
      --build-arg VERSION="$app_version" \
      --build-arg COMMIT="$app_commit" \
      --build-arg DATE="$app_build_date" \
      -f "$APP_DIR/Dockerfile" \
      "$APP_DIR"
  )
}

recreate_app_container() {
  ensure_paths
  (
    cd "$DEPLOY_DIR"
    docker compose -f "$COMPOSE_FILE" up -d --no-deps --force-recreate "$APP_SERVICE"
  )
  wait_for_health "$APP_SERVICE" 300
}

rollback_image() {
  ensure_paths

  rollback_tag="${1:-${IMAGE_REPO}:rollback-latest}"
  docker image inspect "$rollback_tag" >/dev/null 2>&1 || fail "未找到回退镜像: $rollback_tag"

  docker tag "$rollback_tag" "${IMAGE_REPO}:latest"
  log "已将 $rollback_tag 重新标记为 ${IMAGE_REPO}:latest"

  recreate_app_container
}

rollback_source() {
  ensure_paths

  good_commit="${1:-}"
  [ -n "$good_commit" ] || fail "用法: rollback.sh source <commit>"
  [ -d "$APP_DIR/.git" ] || fail "$APP_DIR 不是 git 仓库"

  (
    cd "$APP_DIR"
    git fetch --tags origin main
    git reset --hard "$good_commit"
  )

  build_app_image_from_current_source
  recreate_app_container
}

restore_db() {
  ensure_paths

  backup_file=""
  restore_mode=""
  image_tag="${IMAGE_REPO}:rollback-latest"
  source_commit=""

  set_restore_mode() {
    mode="$1"
    if [ -n "$restore_mode" ] && [ "$restore_mode" != "$mode" ]; then
      fail "db-restore 只能选择一种恢复后应用策略"
    fi
    restore_mode="$mode"
  }

  while [ "$#" -gt 0 ]; do
    case "$1" in
      --with-image)
        set_restore_mode "image"
        shift
        ;;
      --image-tag)
        [ -n "${2:-}" ] || fail "用法: rollback.sh db-restore [backup-file] --with-image [--image-tag <rollback-tag>]"
        image_tag="$2"
        shift 2
        ;;
      --with-source)
        [ -n "${2:-}" ] || fail "用法: rollback.sh db-restore [backup-file] --with-source <commit>"
        set_restore_mode "source"
        source_commit="$2"
        shift 2
        ;;
      --allow-current-app)
        set_restore_mode "current"
        shift
        ;;
      --help|-h)
        fail "用法: rollback.sh db-restore [backup-file] (--with-image [--image-tag <rollback-tag>] | --with-source <commit> | --allow-current-app)"
        ;;
      --*)
        fail "未知参数: $1"
        ;;
      *)
        [ -z "$backup_file" ] || fail "db-restore 只接受一个备份文件参数"
        backup_file="$1"
        shift
        ;;
    esac
  done

  [ -n "$restore_mode" ] || fail "db-restore 为高风险操作，请显式指定 --with-image、--with-source <commit>，或在确认当前应用版本安全时使用 --allow-current-app"
  if [ "$restore_mode" != "image" ] && [ "$image_tag" != "${IMAGE_REPO}:rollback-latest" ]; then
    fail "--image-tag 只能与 --with-image 一起使用"
  fi

  if [ -z "$backup_file" ]; then
    backup_file="$(
      ls -1t "$BACKUP_DIR"/pre-deploy-*.sql.gz "$BACKUP_DIR"/scheduled-*.sql.gz 2>/dev/null | head -1 || true
    )"
  fi
  [ -n "$backup_file" ] || fail "未找到可恢复的数据库备份文件"
  [ -f "$backup_file" ] || fail "备份文件不存在: $backup_file"

  db_user="$(docker exec "$POSTGRES_CONTAINER" printenv POSTGRES_USER)"
  db_name="$(docker exec "$POSTGRES_CONTAINER" printenv POSTGRES_DB)"
  [ -n "$db_user" ] || fail "无法读取 POSTGRES_USER"
  [ -n "$db_name" ] || fail "无法读取 POSTGRES_DB"

  (
    cd "$DEPLOY_DIR"
    docker compose -f "$COMPOSE_FILE" stop "$APP_SERVICE"
  )

  case "$restore_mode" in
    image)
      log "数据库恢复完成后将回退到镜像: $image_tag"
      ;;
    source)
      log "数据库恢复完成后将回退到源码提交: $source_commit"
      ;;
    current)
      log "数据库恢复完成后将重启当前应用版本；仅在确认当前代码与备份库兼容时使用"
      ;;
  esac

  log "开始恢复数据库: $backup_file"
  gunzip -c "$backup_file" | docker exec -i "$POSTGRES_CONTAINER" psql -U "$db_user" -d "$db_name"
  log "数据库恢复完成"

  case "$restore_mode" in
    image)
      rollback_image "$image_tag"
      ;;
    source)
      rollback_source "$source_commit"
      ;;
    current)
      recreate_app_container
      ;;
  esac
}

usage() {
  cat <<'EOF'
用法:
  rollback.sh prep
  rollback.sh tag-current
  rollback.sh backup-db
  rollback.sh image [rollback-tag]
  rollback.sh source <commit>
  rollback.sh db-restore [backup-file] --with-image [--image-tag <rollback-tag>]
  rollback.sh db-restore [backup-file] --with-source <commit>
  rollback.sh db-restore [backup-file] --allow-current-app

说明:
  prep         部署前准备：给当前镜像打回退 tag，并执行 pre-deploy 数据库备份
  tag-current  只给当前镜像打回退 tag
  backup-db    只执行数据库备份
  image        用回退镜像快速回滚，默认使用 deploy-sub2api:rollback-latest
  source       回退到指定 git commit，重新构建并重启 sub2api
  db-restore   恢复数据库，默认取最新 pre-deploy 或 scheduled 备份
               需要显式指定恢复后的应用策略，避免数据库恢复后误起当前坏版本
EOF
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  cmd="${1:-}"
  case "$cmd" in
    prep)
      prep
      ;;
    tag-current)
      create_rollback_tag 1
      ;;
    backup-db)
      backup_db
      ;;
    image)
      shift
      rollback_image "${1:-}"
      ;;
    source)
      shift
      rollback_source "${1:-}"
      ;;
    db-restore)
      shift
      restore_db "$@"
      ;;
    -h|--help|help|"")
      usage
      ;;
    *)
      usage
      fail "未知命令: $cmd"
      ;;
  esac
fi
