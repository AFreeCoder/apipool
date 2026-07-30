#!/usr/bin/env bash
set -Eeuo pipefail

# 只能由 owner 经受信任的 SSH/控制台显式调用。Runner 不允许更新 root 工具链。

[ "$(id -u)" -eq 0 ] || {
  echo "install-production-tooling.sh: 必须以 root 运行" >&2
  exit 77
}

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
SOURCE_ROOT="$(realpath -e -- "${1:-$SCRIPT_DIR/..}")"
APP_DIR="/opt/sub2api"
DEPLOY_DIR="$APP_DIR/deploy"

required_files=(
  deploy/backup-postgres.sh
  deploy/docker-compose.deploy.yml
  deploy/target-deploy.sh
  deploy/configure-caddy.sh
  deploy/runner-deploy.sh
  deploy/rollback.sh
  deploy/sub2api-backup.service
  deploy/sub2api-backup.timer
  deploy/version_resolver.sh
)
for required in "${required_files[@]}"; do
  [ -f "$SOURCE_ROOT/$required" ] || {
    echo "install-production-tooling.sh: 缺少源文件 $required" >&2
    exit 66
  }
  [ ! -L "$SOURCE_ROOT/$required" ] || {
    echo "install-production-tooling.sh: 部署文件不允许是符号链接 $required" >&2
    exit 77
  }
done

install -d -o root -g root -m 0755 "$APP_DIR" "$DEPLOY_DIR"
install -d -o root -g root -m 0700 "$APP_DIR/backups"

if [ -f "$DEPLOY_DIR/docker-compose.deploy.yml" ]; then
  backup="$APP_DIR/backups/tooling-$(date -u +%Y%m%dT%H%M%SZ).tar.gz"
  tar -C "$APP_DIR" -czf "$backup" deploy
  chmod 0600 "$backup"
  echo "[tooling] 旧工具链备份: $backup"
fi

install -o root -g root -m 0644 \
  "$SOURCE_ROOT/deploy/docker-compose.deploy.yml" \
  "$DEPLOY_DIR/docker-compose.deploy.yml"
for unit in sub2api-backup.service sub2api-backup.timer; do
  install -o root -g root -m 0644 \
    "$SOURCE_ROOT/deploy/$unit" "$DEPLOY_DIR/$unit"
done
for script in backup-postgres.sh target-deploy.sh configure-caddy.sh rollback.sh version_resolver.sh; do
  mode=0755
  if [ "$script" = "version_resolver.sh" ]; then
    mode=0644
  fi
  install -o root -g root -m "$mode" \
    "$SOURCE_ROOT/deploy/$script" "$DEPLOY_DIR/$script"
done
install -o root -g root -m 0755 \
  "$SOURCE_ROOT/deploy/runner-deploy.sh" \
  /usr/local/sbin/sub2api-runner-deploy

install -o root -g root -m 0644 \
  "$DEPLOY_DIR/sub2api-backup.service" \
  /etc/systemd/system/sub2api-backup.service
install -o root -g root -m 0644 \
  "$DEPLOY_DIR/sub2api-backup.timer" \
  /etc/systemd/system/sub2api-backup.timer
systemctl daemon-reload
systemctl enable --now sub2api-backup.timer

if [ -e "$DEPLOY_DIR/.env.biz" ] || [ -e "$DEPLOY_DIR/docker-compose.biz.yml" ]; then
  echo "install-production-tooling.sh: 目标活动目录不得包含 biz 配置" >&2
  exit 77
fi

if [ -f "$DEPLOY_DIR/.env" ]; then
  chown root:root "$DEPLOY_DIR/.env"
  chmod 0600 "$DEPLOY_DIR/.env"
else
  echo "[tooling] 尚未安装生产 .env；首次发布前必须由 owner 写入并 chmod 600"
fi

echo "[tooling] 已安装 root-owned APIPool 生产工具链"
