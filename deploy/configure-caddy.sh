#!/usr/bin/env bash
set -Eeuo pipefail

umask 077

APP_DIR="${SUB2API_DEPLOY_ROOT:-/opt/sub2api}"
ENV_FILE="${SUB2API_ENV_FILE:-$APP_DIR/deploy/.env}"
CADDY_ROOT="${SUB2API_CADDY_ROOT:-/etc/caddy/Caddyfile}"
SITES_DIR="${SUB2API_CADDY_SITES_DIR:-/etc/caddy/sites-enabled}"
FRAGMENT="${SUB2API_CADDY_FRAGMENT:-$SITES_DIR/apipool-legacy.caddy}"
LOCK_FILE="${SUB2API_CADDY_LOCK:-/run/apipool-caddy.lock}"

[ "$(id -u)" -eq 0 ] || {
  echo "configure-caddy.sh: 必须以 root 运行" >&2
  exit 77
}

for command_name in caddy cp flock grep install mktemp realpath rm sed stat systemctl; do
  command -v "$command_name" >/dev/null 2>&1 || {
    echo "configure-caddy.sh: 缺少命令 $command_name" >&2
    exit 69
  }
done

[ -f "$ENV_FILE" ] || {
  echo "configure-caddy.sh: 缺少 $ENV_FILE" >&2
  exit 66
}
[ -f "$CADDY_ROOT" ] || {
  echo "configure-caddy.sh: 缺少 $CADDY_ROOT" >&2
  exit 66
}

set -a
# shellcheck disable=SC1090
. "$ENV_FILE"
set +a

cert_file="${APIPOOL_CADDY_CERT_FILE:-/etc/caddy/certs/apipool.dev.crt}"
key_file="${APIPOOL_CADDY_KEY_FILE:-/etc/caddy/certs/apipool.dev.key}"
upstream="${APIPOOL_CADDY_UPSTREAM:-127.0.0.1:8080}"

for secret_file in "$cert_file" "$key_file"; do
  [ -f "$secret_file" ] || {
    echo "configure-caddy.sh: 缺少 TLS 文件 $secret_file" >&2
    exit 66
  }
  [ "$(stat -c '%u' "$secret_file")" = "0" ] || {
    echo "configure-caddy.sh: TLS 文件必须为 root 所有: $secret_file" >&2
    exit 77
  }
  [ "$(realpath -e "$secret_file")" = "$secret_file" ] || {
    echo "configure-caddy.sh: TLS 文件不允许是符号链接: $secret_file" >&2
    exit 77
  }
done
key_mode="$(stat -c '%a' "$key_file")"
if (( (8#$key_mode & 8#077) != 0 )); then
  echo "configure-caddy.sh: TLS 私钥必须仅允许 root 访问" >&2
  exit 77
fi

if ! grep -Fq 'import /etc/caddy/sites-enabled/*.caddy' "$CADDY_ROOT"; then
  echo "configure-caddy.sh: 根 Caddyfile 未启用 sites-enabled 分片" >&2
  exit 78
fi

exec 9>"$LOCK_FILE"
flock -w 60 9 || {
  echo "configure-caddy.sh: 等待共享 Caddy 配置锁超时" >&2
  exit 75
}

install -d -o root -g root -m 0755 "$SITES_DIR"
candidate_dir="$(mktemp -d)"
candidate_root="$(mktemp)"
fragment_tmp="$(mktemp)"
previous_fragment=""
cleanup() {
  rm -rf "$candidate_dir" "$candidate_root" "$fragment_tmp"
  if [ -n "$previous_fragment" ]; then
    rm -f "$previous_fragment"
  fi
}
trap cleanup EXIT

if find "$SITES_DIR" -maxdepth 1 -type f -name '*.caddy' -print -quit | grep -q .; then
  cp -a "$SITES_DIR"/. "$candidate_dir"/
fi

cat >"$fragment_tmp" <<EOF
# 由 /opt/sub2api/deploy/configure-caddy.sh 管理。
# api.apipool.dev 当前由 qingyun 转发到本机并使用 apipool.dev 作为上游 SNI；
# 同时保留该 hostname，便于入口故障时直接验证目标机。
apipool.dev, api.apipool.dev {
	tls $cert_file $key_file {
		protocols tls1.2 tls1.3
	}

	@static {
		path /assets/*
		path /logo.png
		path /favicon.ico
	}
	header @static {
		Cache-Control "public, max-age=31536000, immutable"
		-Pragma
		-Expires
	}

	reverse_proxy $upstream {
		health_uri /health
		health_interval 30s
		health_timeout 10s
		health_status 200
		header_up X-Real-IP {remote_host}
		header_up CF-Connecting-IP {http.request.header.CF-Connecting-IP}
		transport http {
			keepalive 120s
			keepalive_idle_conns 256
		}
		flush_interval -1
	}

	encode gzip zstd
	header {
		X-Frame-Options "SAMEORIGIN"
		X-XSS-Protection "1; mode=block"
		X-Content-Type-Options "nosniff"
		Referrer-Policy "strict-origin-when-cross-origin"
		Strict-Transport-Security "max-age=31536000; includeSubDomains; preload"
		Permissions-Policy "accelerometer=(), camera=(), geolocation=(), gyroscope=(), magnetometer=(), microphone=(), payment=(), usb=()"
		-Server
		-X-Powered-By
	}
	request_body {
		max_size 100MB
	}
	log {
		output file /var/log/caddy/apipool.dev.log {
			roll_size 50mb
			roll_keep 10
			roll_keep_for 720h
		}
		format json
		level INFO
	}
	handle_errors {
		respond "{err.status_code} {err.status_text}"
	}
}
EOF
install -o root -g root -m 0644 "$fragment_tmp" "$candidate_dir/$(basename "$FRAGMENT")"

# 用候选分片目录验证整套配置；验证成功前不修改线上 fragment。
sed "s|import /etc/caddy/sites-enabled/\\*.caddy|import $candidate_dir/*.caddy|" \
  "$CADDY_ROOT" >"$candidate_root"
caddy validate --config "$candidate_root" --adapter caddyfile >/dev/null

previous_fragment="$(mktemp)"
had_previous=0
if [ -f "$FRAGMENT" ]; then
  cp -a "$FRAGMENT" "$previous_fragment"
  had_previous=1
fi
install -o root -g root -m 0644 "$fragment_tmp" "$FRAGMENT"
if ! caddy validate --config "$CADDY_ROOT" --adapter caddyfile >/dev/null; then
  if [ "$had_previous" -eq 1 ]; then
    install -o root -g root -m 0644 "$previous_fragment" "$FRAGMENT"
  else
    rm -f "$FRAGMENT"
  fi
  echo "configure-caddy.sh: 安装后的整套 Caddy 配置校验失败" >&2
  exit 78
fi
systemctl reload caddy
echo "configure-caddy.sh: 已安装 $FRAGMENT"
