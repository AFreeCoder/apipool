#!/usr/bin/env bash
set -euo pipefail

workflow="${1:-.github/workflows/deploy.yml}"

if [ ! -f "$workflow" ]; then
  echo "workflow not found: $workflow" >&2
  exit 1
fi

line_no() {
  local pattern="$1"
  awk -v pattern="$pattern" 'index($0, pattern) { print NR; exit }' "$workflow"
}

count_lines() {
  local pattern="$1"
  awk -v pattern="$pattern" 'index($0, pattern) { count++ } END { print count + 0 }' "$workflow"
}

biz_backup_pattern='BIZ_BACKUP_FILE="${BACKUP_DIR}/pre-deploy-biz-'
main_compose_pattern='docker compose -f docker-compose.deploy.yml up -d --remove-orphans'
biz_remove_pattern='docker rm -f sub2api-biz'
pipefail_pattern='set -o pipefail'
first_pg_dump_pattern='pg_dump'

pipefail_line="$(line_no "$pipefail_pattern")"
first_pg_dump_line="$(line_no "$first_pg_dump_pattern")"

if [ -z "$pipefail_line" ] || [ -z "$first_pg_dump_line" ]; then
  echo "required workflow pipefail/pg_dump markers are missing" >&2
  exit 1
fi

if [ "$pipefail_line" -ge "$first_pg_dump_line" ]; then
  echo "deploy workflow must enable pipefail before pg_dump backup pipelines" >&2
  echo "pipefail line: $pipefail_line; first pg_dump line: $first_pg_dump_line" >&2
  exit 1
fi

biz_backup_count="$(count_lines "$biz_backup_pattern")"
if [ "$biz_backup_count" -ne 1 ]; then
  echo "expected exactly one biz pre-deploy backup block, found $biz_backup_count" >&2
  exit 1
fi

biz_backup_line="$(line_no "$biz_backup_pattern")"
main_compose_line="$(line_no "$main_compose_pattern")"
biz_remove_line="$(line_no "$biz_remove_pattern")"

if [ -z "$biz_backup_line" ] || [ -z "$main_compose_line" ] || [ -z "$biz_remove_line" ]; then
  echo "required workflow markers are missing" >&2
  exit 1
fi

if [ "$biz_backup_line" -ge "$main_compose_line" ]; then
  echo "biz backup must run before main compose --remove-orphans" >&2
  echo "biz backup line: $biz_backup_line; main compose line: $main_compose_line" >&2
  exit 1
fi

if [ "$biz_backup_line" -ge "$biz_remove_line" ]; then
  echo "biz backup must run before explicit sub2api-biz removal" >&2
  echo "biz backup line: $biz_backup_line; biz removal line: $biz_remove_line" >&2
  exit 1
fi

echo "deploy workflow biz backup order OK"
