#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

fail() {
  printf 'target-deploy-test: %s\n' "$*" >&2
  exit 1
}

assert_contains() {
  file="$1"
  pattern="$2"
  grep -Eq "$pattern" "$file" || fail "$file 缺少约束: $pattern"
}

assert_not_contains() {
  file="$1"
  pattern="$2"
  if grep -Eq "$pattern" "$file"; then
    fail "$file 不应包含: $pattern"
  fi
}

workflow=".github/workflows/deploy.yml"
compose="deploy/docker-compose.deploy.yml"
caddy="deploy/configure-caddy.sh"
runner="deploy/runner-deploy.sh"
deploy_script="deploy/target-deploy.sh"
backup_script="deploy/backup-postgres.sh"
tooling_installer="deploy/install-production-tooling.sh"

assert_contains "$workflow" 'name: Deploy to apipool_vps'
assert_contains "$workflow" 'sha-\$\{GITHUB_SHA\}'
assert_contains "$workflow" 'image_tags<<EOF'
assert_contains "$workflow" 'if \[ "\$GITHUB_REF" = "refs/heads/main" \]'
assert_contains "$workflow" 'tags: \$\{\{ steps\.image\.outputs\.image_tags \}\}'
assert_contains "$workflow" 'sub2api-prod-deploy'
assert_contains "$workflow" 'cancel-in-progress: false'
assert_contains "$workflow" "if: github.ref == 'refs/heads/main'"
assert_contains "$workflow" 'environment: production'
assert_not_contains "$workflow" 'appleboy/ssh-action'
assert_not_contains "$workflow" 'DO_HOST|DO_SSH_PRIVATE_KEY'

assert_contains "$compose" '^name: sub2api$'
assert_contains "$compose" 'mem_limit: 2g'
assert_contains "$compose" 'postgres:18-alpine@sha256:[0-9a-f]{64}'
assert_contains "$compose" 'redis:8-alpine@sha256:[0-9a-f]{64}'
assert_contains "$compose" '127\.0\.0\.1:\$\{SERVER_PORT:-8080\}:8080'

assert_contains "$deploy_script" 'sha-\[0-9a-f\]\{40\}'
assert_contains "$deploy_script" '目标机禁止部署 biz'
assert_contains "$deploy_script" '本次同 SHA 发布幂等跳过'
assert_contains "$deploy_script" 'pg_dump .*--clean --if-exists'
assert_contains "$deploy_script" 'gzip -t'
assert_contains "$deploy_script" 'docker port sub2api'

assert_contains "$caddy" '/run/apipool-caddy\.lock'
assert_contains "$caddy" 'import /etc/caddy/sites-enabled/\*\.caddy'
assert_contains "$caddy" 'candidate_dir='
assert_contains "$caddy" 'caddy validate --config "\$candidate_root"'
assert_contains "$caddy" '^apipool\.dev \{$'
assert_not_contains "$caddy" 'apipool\.dev, api\.apipool\.dev'
assert_not_contains "$caddy" '^api\.apipool\.dev'
assert_not_contains "$caddy" 'APIPOOL_CADDY_CERT_FILE|APIPOOL_CADDY_KEY_FILE'
assert_not_contains "$caddy" '根 Caddyfile 未启用 auto_https ignore_loaded_certs'
assert_contains "$caddy" '根 Caddyfile 禁止启用 auto_https ignore_loaded_certs'
assert_not_contains "$caddy" 'biz\.apipool\.dev'
assert_contains "$caddy" 'caddy_version=.*caddy version'
assert_contains "$caddy" '"\$caddy_version" = "2\.6\.2"'
assert_contains "$caddy" 'systemctl restart caddy'
assert_contains "$caddy" 'systemctl is-active --quiet caddy'

validate_line="$(grep -n 'caddy validate --config "\$candidate_root"' "$caddy" | head -1 | cut -d: -f1)"
install_line="$(grep -n 'install -o root -g root -m 0644 "\$fragment_tmp" "\$FRAGMENT"' "$caddy" | head -1 | cut -d: -f1)"
[ "$validate_line" -lt "$install_line" ] \
  || fail "Caddy 候选树必须在写入 live fragment 前验证"

assert_contains "$runner" 'EXPECTED_WORKSPACE="/opt/actions-runner-sub2api/_work/apipool/apipool"'
assert_contains "$runner" 'checkout HEAD does not match image tag'
assert_contains "$runner" 'workspace is not clean'
assert_contains "$runner" 'production tooling drift'
assert_contains "$runner" 'cmp -s'
assert_contains "$runner" 'deploy/sub2api-backup\.service\|/etc/systemd/system/sub2api-backup\.service'
assert_contains "$runner" 'deploy/sub2api-backup\.timer\|/etc/systemd/system/sub2api-backup\.timer'
assert_contains "$tooling_installer" '"\$DEPLOY_DIR/sub2api-backup\.service"'
assert_contains "$tooling_installer" '"\$DEPLOY_DIR/sub2api-backup\.timer"'
assert_contains "$tooling_installer" '/usr/local/sbin/sub2api-runner-deploy'
assert_contains "$backup_script" 'RETENTION_HOURS="\$\{SCHEDULED_BACKUP_RETENTION_HOURS:-72\}"'
assert_contains "$backup_script" 'MAX_FILES="\$\{SCHEDULED_BACKUP_MAX_FILES:-18\}"'
assert_contains "$backup_script" '\[ -z "\$\{backup_tmp:-\}" \] \|\| rm -f -- "\$backup_tmp"'
assert_contains "deploy/rollback.sh" 'ON_ERROR_STOP=1'

if "$deploy_script" latest >/dev/null 2>&1; then
  fail "target-deploy.sh 必须拒绝可移动 tag"
fi
if "deploy/runner-deploy.sh" /tmp latest invalid-user! >/dev/null 2>&1; then
  fail "runner-deploy.sh 必须拒绝无效参数"
fi

printf 'target-deploy-test: all checks passed\n'
