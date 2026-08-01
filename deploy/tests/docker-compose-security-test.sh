#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$repo_root"

check_application_security_opt() {
  file=$1
  service=$2
  count=$(
    awk -v application="  $service:" '
      $0 == application {
        in_application = 1
        next
      }
      in_application && $0 ~ /^  [A-Za-z0-9_-]+:$/ {
        in_application = 0
      }
      in_application && $0 == "    security_opt:" {
        in_security_opt = 1
        next
      }
      in_application && in_security_opt && $0 == "      - no-new-privileges:true" {
        count++
      }
      END { print count + 0 }
    ' "$file"
  )

  if [ "$count" -ne 1 ]; then
    printf '%s must enable no-new-privileges exactly once for the %s service\n' "$file" "$service" >&2
    exit 1
  fi
}

for compose_file in \
  deploy/docker-compose.yml \
  deploy/docker-compose.local.yml \
  deploy/docker-compose.standalone.yml \
  deploy/docker-compose.dev.yml \
  deploy/docker-compose.deploy.yml \
  deploy/docker-compose-test.yml
do
  check_application_security_opt "$compose_file" sub2api
done

check_application_security_opt deploy/docker-compose.biz.yml sub2api-biz

printf 'docker compose security test passed\n'
