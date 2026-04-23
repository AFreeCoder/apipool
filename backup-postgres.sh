#!/usr/bin/env bash

set -euo pipefail

APP_DIR="${APP_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"
DEPLOY_ENV_FILE="${DEPLOY_ENV_FILE:-$APP_DIR/deploy/.env}"
BACKUP_DIR="${BACKUP_DIR:-$APP_DIR/backups}"
CONTAINER="${CONTAINER:-sub2api-postgres}"
RETENTION_HOURS="${SCHEDULED_BACKUP_RETENTION_HOURS:-72}"
MAX_FILES="${SCHEDULED_BACKUP_MAX_FILES:-18}"
DEFAULT_EXCLUDE_TABLE_DATA="public.ops_system_logs,public.ops_error_logs,public.ops_system_metrics,public.ops_metrics_hourly,public.ops_metrics_daily,public.ops_retry_attempts,public.ops_alert_events,public.ops_system_log_cleanup_audits"
EXCLUDE_TABLE_DATA="${SCHEDULED_BACKUP_EXCLUDE_TABLE_DATA:-$DEFAULT_EXCLUDE_TABLE_DATA}"

log() {
  printf '%s [backup] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"
}

fail() {
  log "ERROR: $*" >&2
  exit 1
}

load_env_file() {
  local env_file="$1"
  if [ -f "$env_file" ]; then
    set -a
    # shellcheck disable=SC1090
    . "$env_file"
    set +a
  fi
}

append_exclude_args() {
  local raw="$1"
  local entry table
  IFS=',' read -ra entries <<< "$raw"
  for entry in "${entries[@]}"; do
    table="$(printf '%s' "$entry" | xargs)"
    if [ -z "$table" ]; then
      continue
    fi
    DUMP_ARGS+=(--exclude-table-data="$table")
  done
}

prune_scheduled_backups() {
  local retention_minutes old_backup
  retention_minutes=$((RETENTION_HOURS * 60))
  find "$BACKUP_DIR" -maxdepth 1 -type f -name 'scheduled-*.sql.gz' -mmin "+$retention_minutes" -delete

  if [ "$MAX_FILES" -gt 0 ]; then
    mapfile -t SCHEDULED_BACKUPS < <(find "$BACKUP_DIR" -maxdepth 1 -type f -name 'scheduled-*.sql.gz' -printf '%f\n' | sort -r)
    if [ "${#SCHEDULED_BACKUPS[@]}" -gt "$MAX_FILES" ]; then
      for old_backup in "${SCHEDULED_BACKUPS[@]:$MAX_FILES}"; do
        rm -f "$BACKUP_DIR/$old_backup"
      done
    fi
  fi
}

load_env_file "$DEPLOY_ENV_FILE"
mkdir -p "$BACKUP_DIR"

if [ -z "$(docker ps --filter "name=^${CONTAINER}$" --format '{{.Names}}')" ]; then
  fail "${CONTAINER} not running"
fi

DB_USER="$(docker exec "$CONTAINER" printenv POSTGRES_USER)"
DB_NAME="$(docker exec "$CONTAINER" printenv POSTGRES_DB)"
[ -n "$DB_USER" ] || fail "POSTGRES_USER missing in ${CONTAINER}"
[ -n "$DB_NAME" ] || fail "POSTGRES_DB missing in ${CONTAINER}"

BACKUP_FILE="${BACKUP_DIR}/scheduled-$(date +%Y%m%d_%H%M%S).sql.gz"
DUMP_ARGS=(pg_dump -U "$DB_USER" -d "$DB_NAME" --clean --if-exists)
append_exclude_args "$EXCLUDE_TABLE_DATA"

log "starting scheduled backup: $BACKUP_FILE"
if [ "${#DUMP_ARGS[@]}" -gt 4 ]; then
  log "excluding table data: ${EXCLUDE_TABLE_DATA}"
fi

docker exec "$CONTAINER" "${DUMP_ARGS[@]}" | gzip > "$BACKUP_FILE"

if [ -s "$BACKUP_FILE" ] && gzip -t "$BACKUP_FILE"; then
  prune_scheduled_backups
  log "backup complete: $BACKUP_FILE ($(du -h "$BACKUP_FILE" | cut -f1))"
else
  rm -f "$BACKUP_FILE"
  fail "backup failed"
fi
