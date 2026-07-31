#!/usr/bin/env bash
set -Eeuo pipefail

# 固定副本由 owner 安装到 /usr/local/sbin。Runner 只能传入严格校验后的
# checkout、不可变镜像 tag 和 GHCR 用户名，并通过 stdin 提供短期 token。
umask 077

unset DOCKER_HOST DOCKER_CONTEXT DOCKER_TLS_VERIFY DOCKER_CERT_PATH

APP_DIR="/opt/sub2api"
EXPECTED_WORKSPACE="/opt/actions-runner-sub2api/_work/apipool/apipool"
EXPECTED_ORIGIN="https://github.com/AFreeCoder/apipool.git"

usage() {
  echo "usage: $0 <github-workspace> <sha-image-tag> <ghcr-user>" >&2
  exit 64
}

[ "$#" -eq 3 ] || usage

workspace="$(realpath -e -- "$1")"
image_tag="$2"
ghcr_user="$3"

[ "$workspace" = "$EXPECTED_WORKSPACE" ] || {
  echo "runner-deploy: unexpected workspace: $workspace" >&2
  exit 77
}
[[ "$image_tag" =~ ^sha-[0-9a-f]{40}$ ]] || {
  echo "runner-deploy: invalid immutable image tag" >&2
  exit 64
}
[[ "$ghcr_user" =~ ^[A-Za-z0-9-]+$ ]] || {
  echo "runner-deploy: invalid GHCR username" >&2
  exit 64
}

git_in_workspace() {
  env -i PATH=/usr/bin:/bin HOME=/root GIT_OPTIONAL_LOCKS=0 \
    git -c safe.directory="$workspace" -C "$workspace" "$@"
}

origin="$(git_in_workspace config --get remote.origin.url || true)"
case "$origin" in
  "$EXPECTED_ORIGIN" | "${EXPECTED_ORIGIN%.git}") ;;
  *)
    echo "runner-deploy: unexpected git origin" >&2
    exit 77
    ;;
esac

expected_sha="${image_tag#sha-}"
actual_sha="$(git_in_workspace rev-parse HEAD)"
[ "$actual_sha" = "$expected_sha" ] || {
  echo "runner-deploy: checkout HEAD does not match image tag" >&2
  exit 77
}
[ -z "$(git_in_workspace status --porcelain --untracked-files=all)" ] || {
  echo "runner-deploy: workspace is not clean" >&2
  exit 77
}

verify_root_owned() {
  local path=""
  local owner=""
  local mode=""
  for path in \
    "$APP_DIR" \
    "$APP_DIR/deploy" \
    "$APP_DIR/deploy/docker-compose.deploy.yml" \
    "$APP_DIR/deploy/backup-postgres.sh" \
    "$APP_DIR/deploy/sub2api-backup.service" \
    "$APP_DIR/deploy/sub2api-backup.timer" \
    "$APP_DIR/deploy/target-deploy.sh" \
    "$APP_DIR/deploy/configure-caddy.sh" \
    "$APP_DIR/deploy/caddy-runtime-contract" \
    /etc/systemd/system/sub2api-backup.service \
    /etc/systemd/system/sub2api-backup.timer; do
    [ -e "$path" ] && [ "$(realpath -e "$path")" = "$path" ] || {
      echo "runner-deploy: missing or unsafe fixed path: $path" >&2
      exit 77
    }
    owner="$(stat -c '%u' "$path")"
    mode="$(stat -c '%a' "$path")"
    if [ "$owner" != "0" ] || (( (8#$mode & 8#022) != 0 )); then
      echo "runner-deploy: fixed tooling must be root-owned and immutable to runner: $path" >&2
      exit 77
    fi
  done

  path="$APP_DIR/deploy/.env"
  [ -f "$path" ] && [ "$(realpath -e "$path")" = "$path" ] || {
    echo "runner-deploy: missing production env" >&2
    exit 77
  }
  owner="$(stat -c '%u' "$path")"
  mode="$(stat -c '%a' "$path")"
  if [ "$owner" != "0" ] || (( (8#$mode & 8#077) != 0 )); then
    echo "runner-deploy: production env must be root-owned and owner-only" >&2
    exit 77
  fi
  [ ! -e "$APP_DIR/deploy/.env.biz" ] || {
    echo "runner-deploy: biz config is forbidden on target" >&2
    exit 77
  }
}

verify_root_owned

# 仓库是部署工具链的唯一事实源。为保留 Runner 的最小 root 权限，普通发布
# 不会自行覆盖 root-owned 脚本；若仓库版本与目标机固定副本不一致，则
# fail-closed，提示 owner 从当前精确 commit 运行安装器。
verify_tooling_matches_repository() {
  local source_path=""
  local installed_path=""
  while IFS='|' read -r source_path installed_path; do
    if ! cmp -s "$workspace/$source_path" "$installed_path"; then
      echo "runner-deploy: production tooling drift: $source_path" >&2
      echo "runner-deploy: owner must install tooling from this exact audited commit" >&2
      exit 78
    fi
  done <<EOF
deploy/docker-compose.deploy.yml|$APP_DIR/deploy/docker-compose.deploy.yml
deploy/backup-postgres.sh|$APP_DIR/deploy/backup-postgres.sh
deploy/target-deploy.sh|$APP_DIR/deploy/target-deploy.sh
deploy/configure-caddy.sh|$APP_DIR/deploy/configure-caddy.sh
deploy/caddy-runtime-contract|$APP_DIR/deploy/caddy-runtime-contract
deploy/rollback.sh|$APP_DIR/deploy/rollback.sh
deploy/version_resolver.sh|$APP_DIR/deploy/version_resolver.sh
deploy/sub2api-backup.service|$APP_DIR/deploy/sub2api-backup.service
deploy/sub2api-backup.timer|$APP_DIR/deploy/sub2api-backup.timer
deploy/sub2api-backup.service|/etc/systemd/system/sub2api-backup.service
deploy/sub2api-backup.timer|/etc/systemd/system/sub2api-backup.timer
deploy/runner-deploy.sh|/usr/local/sbin/sub2api-runner-deploy
EOF
}

verify_tooling_matches_repository

ghcr_token=""
IFS= read -r ghcr_token || true
[ -n "$ghcr_token" ] || {
  echo "runner-deploy: missing GHCR token on stdin" >&2
  exit 78
}

DOCKER_CONFIG="$(mktemp -d /run/sub2api-ghcr-auth.XXXXXX)"
export DOCKER_CONFIG
trap 'docker logout ghcr.io >/dev/null 2>&1 || true; rm -rf "$DOCKER_CONFIG"' EXIT

printf '%s' "$ghcr_token" \
  | docker login ghcr.io -u "$ghcr_user" --password-stdin >/dev/null
ghcr_token=""

env -i \
  PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
  HOME=/root \
  DOCKER_CONFIG="$DOCKER_CONFIG" \
  "$APP_DIR/deploy/target-deploy.sh" "$image_tag"
