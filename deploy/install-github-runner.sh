#!/usr/bin/env bash
set -Eeuo pipefail

[ "$(id -u)" -eq 0 ] || {
  echo "install-github-runner.sh: 必须以 root 运行" >&2
  exit 77
}

for required_command in curl sha256sum tar runuser visudo nft systemctl getent; do
  command -v "$required_command" >/dev/null 2>&1 || {
    echo "install-github-runner.sh: 缺少命令 $required_command" >&2
    exit 69
  }
done

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
RUNNER_USER="sub2api-runner"
RUNNER_HOME="/opt/actions-runner-sub2api"
RUNNER_NAME="sub2api-prod-deploy"
RUNNER_LABEL="sub2api-prod-deploy"
REPOSITORY_URL="https://github.com/AFreeCoder/apipool"
RUNNER_VERSION="${SUB2API_RUNNER_VERSION:-2.336.0}"
RUNNER_SHA256="${SUB2API_RUNNER_SHA256:-04cf0be1aff4c3ec3554466c39124ca250e3effd8873bb7e8d68535aa9505d5d}"
RUNNER_ARCHIVE="actions-runner-linux-x64-${RUNNER_VERSION}.tar.gz"
RUNNER_URL="https://github.com/actions/runner/releases/download/v${RUNNER_VERSION}/${RUNNER_ARCHIVE}"
EGRESS_CONFIG="/etc/nftables.d/sub2api-runner-egress.nft"
EGRESS_SERVICE="/etc/systemd/system/sub2api-runner-egress.service"

[ -f "$SCRIPT_DIR/runner-deploy.sh" ] || {
  echo "install-github-runner.sh: 缺少 runner-deploy.sh" >&2
  exit 66
}

registration_token=""
IFS= read -r registration_token || true
[ -n "$registration_token" ] || {
  echo "install-github-runner.sh: stdin 缺少注册 token" >&2
  exit 78
}

if ! id "$RUNNER_USER" >/dev/null 2>&1; then
  useradd --system --home-dir "$RUNNER_HOME" --create-home \
    --shell /usr/sbin/nologin "$RUNNER_USER"
fi

runner_passwd="$(getent passwd "$RUNNER_USER")"
[ "$(printf '%s\n' "$runner_passwd" | cut -d: -f6)" = "$RUNNER_HOME" ] \
  && [ "$(printf '%s\n' "$runner_passwd" | cut -d: -f7)" = /usr/sbin/nologin ] || {
    echo "install-github-runner.sh: 现有 runner 用户 home 或 shell 不符合预期" >&2
    exit 77
  }

install -d -o "$RUNNER_USER" -g "$RUNNER_USER" -m 0750 "$RUNNER_HOME"
if [ ! -f "$RUNNER_HOME/.runner" ]; then
  archive="$(mktemp)"
  trap 'rm -f "$archive"' EXIT
  curl --fail --location --proto '=https' --tlsv1.2 "$RUNNER_URL" --output "$archive"
  printf '%s  %s\n' "$RUNNER_SHA256" "$archive" | sha256sum --check --status
  tar -xzf "$archive" -C "$RUNNER_HOME"
  chown -R "$RUNNER_USER:$RUNNER_USER" "$RUNNER_HOME"
  (
    cd "$RUNNER_HOME"
    ./bin/installdependencies.sh
    runuser -u "$RUNNER_USER" -- env HOME="$RUNNER_HOME" \
      ./config.sh \
        --url "$REPOSITORY_URL" \
        --token "$registration_token" \
        --name "$RUNNER_NAME" \
        --labels "$RUNNER_LABEL" \
        --work _work \
        --unattended \
        --replace
  )
fi
registration_token=""

install -o root -g root -m 0755 \
  "$SCRIPT_DIR/runner-deploy.sh" /usr/local/sbin/sub2api-runner-deploy

sudoers_tmp="$(mktemp)"
printf '%s ALL=(root) NOPASSWD: /usr/local/sbin/sub2api-runner-deploy *\n' \
  "$RUNNER_USER" >"$sudoers_tmp"
chmod 0440 "$sudoers_tmp"
visudo -cf "$sudoers_tmp" >/dev/null
install -o root -g root -m 0440 \
  "$sudoers_tmp" /etc/sudoers.d/sub2api-runner-deploy
rm -f "$sudoers_tmp"

if [ ! -f "$RUNNER_HOME/.service" ]; then
  (
    cd "$RUNNER_HOME"
    ./svc.sh install "$RUNNER_USER"
  )
fi

runner_uid="$(id -u "$RUNNER_USER")"
egress_tmp="$(mktemp)"
cat >"$egress_tmp" <<EOF
table inet sub2api_runner_egress {
  chain output {
    type filter hook output priority 10; policy accept;
    meta skuid $runner_uid ip daddr 169.254.0.0/16 reject
    meta skuid $runner_uid ip6 daddr fe80::/10 reject
    meta skuid $runner_uid udp dport 53 accept
    meta skuid $runner_uid tcp dport { 53, 443 } accept
    meta skuid $runner_uid reject
  }
}
EOF
egress_check="$(mktemp)"
sed 's/sub2api_runner_egress/sub2api_runner_egress_validate/' \
  "$egress_tmp" >"$egress_check"
nft --check --file "$egress_check"
install -d -o root -g root -m 0755 "$(dirname "$EGRESS_CONFIG")"
install -o root -g root -m 0600 "$egress_tmp" "$EGRESS_CONFIG"
rm -f "$egress_tmp" "$egress_check"

service_tmp="$(mktemp)"
cat >"$service_tmp" <<EOF
[Unit]
Description=APIPool legacy GitHub Runner egress boundary
Before=network.target
After=network-pre.target

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStartPre=-/usr/sbin/nft delete table inet sub2api_runner_egress
ExecStart=/usr/sbin/nft --file $EGRESS_CONFIG
ExecStop=-/usr/sbin/nft delete table inet sub2api_runner_egress

[Install]
WantedBy=multi-user.target
EOF
install -o root -g root -m 0644 "$service_tmp" "$EGRESS_SERVICE"
rm -f "$service_tmp"
systemctl daemon-reload
systemctl enable --now sub2api-runner-egress.service

(
  cd "$RUNNER_HOME"
  ./svc.sh start
  ./svc.sh status
)
