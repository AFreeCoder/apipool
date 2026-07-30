#!/usr/bin/env bash
set -Eeuo pipefail

umask 077

APP_DIR="${SUB2API_DEPLOY_ROOT:-/opt/sub2api}"
ENV_FILE="${SUB2API_ENV_FILE:-$APP_DIR/deploy/.env}"
BACKUP_DIR="${SUB2API_BACKUP_DIR:-$APP_DIR/backups}"
CONTAINER="${SUB2API_POSTGRES_CONTAINER:-sub2api-postgres}"
LOCK_FILE="${SUB2API_DEPLOY_LOCK:-/run/sub2api-deploy.lock}"
DEFAULT_EXCLUDE_TABLE_DATA="public.ops_system_logs,public.ops_error_logs,public.ops_system_metrics,public.ops_metrics_hourly,public.ops_metrics_daily,public.ops_retry_attempts,public.ops_alert_events,public.ops_system_log_cleanup_audits"

log() {
  printf '%s [backup] %s\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" "$*"
}

fail() {
  log "ERROR: $*" >&2
  exit 1
}

if [ -f "$ENV_FILE" ]; then
  set -a
  # shellcheck disable=SC1090
  . "$ENV_FILE"
  set +a
fi

RETENTION_HOURS="${SCHEDULED_BACKUP_RETENTION_HOURS:-72}"
MAX_FILES="${SCHEDULED_BACKUP_MAX_FILES:-18}"
EXCLUDE_TABLE_DATA="${SCHEDULED_BACKUP_EXCLUDE_TABLE_DATA:-$DEFAULT_EXCLUDE_TABLE_DATA}"
[[ "$RETENTION_HOURS" =~ ^[0-9]+$ ]] \
  || fail "SCHEDULED_BACKUP_RETENTION_HOURS 必须是非负整数"
[[ "$MAX_FILES" =~ ^[0-9]+$ ]] \
  || fail "SCHEDULED_BACKUP_MAX_FILES 必须是非负整数"

exec 9>"$LOCK_FILE"
flock -n 9 || fail "发布或另一备份正在运行"

mkdir -p "$BACKUP_DIR"
chmod 700 "$BACKUP_DIR"
docker ps --filter "name=^${CONTAINER}$" --format '{{.Names}}' | grep -q . \
  || fail "$CONTAINER 未运行"

db_user="$(docker exec "$CONTAINER" printenv POSTGRES_USER)"
db_name="$(docker exec "$CONTAINER" printenv POSTGRES_DB)"
[ -n "$db_user" ] && [ -n "$db_name" ] || fail "无法读取数据库名称或用户"

dump_args=(pg_dump -U "$db_user" -d "$db_name" --clean --if-exists)
IFS=',' read -r -a exclusions <<<"$EXCLUDE_TABLE_DATA"
for entry in "${exclusions[@]}"; do
  table="$(printf '%s' "$entry" | xargs)"
  [ -z "$table" ] || dump_args+=(--exclude-table-data="$table")
done

backup_tmp="$(mktemp "$BACKUP_DIR/.scheduled.XXXXXX.sql.gz")"
backup_file="$BACKUP_DIR/scheduled-$(date -u +%Y%m%dT%H%M%SZ).sql.gz"
cleanup() {
  [ -z "${backup_tmp:-}" ] || rm -f -- "$backup_tmp"
}
trap cleanup EXIT

log "开始周期数据库备份"
docker exec "$CONTAINER" "${dump_args[@]}" | gzip >"$backup_tmp"
[ -s "$backup_tmp" ] && gzip -t "$backup_tmp" || fail "备份校验失败"
chmod 600 "$backup_tmp"
mv "$backup_tmp" "$backup_file"
backup_tmp=""

find "$BACKUP_DIR" -maxdepth 1 -type f -name 'scheduled-*.sql.gz' \
  -mmin "+$((RETENTION_HOURS * 60))" -delete
if [ "$MAX_FILES" -gt 0 ]; then
  mapfile -t backups < <(
    find "$BACKUP_DIR" -maxdepth 1 -type f -name 'scheduled-*.sql.gz' \
      -printf '%f\n' | sort -r
  )
  if [ "${#backups[@]}" -gt "$MAX_FILES" ]; then
    for old_backup in "${backups[@]:$MAX_FILES}"; do
      rm -f "$BACKUP_DIR/$old_backup"
    done
  fi
fi

log "备份完成: $backup_file"
