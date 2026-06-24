# APIPool Deployment

本文档记录当前 APIPool 生产发布流程。不要在本文档中写入密钥、token、密码、私钥或敏感客户数据。

## Release Target

- Release branch: `main`
- Release environment: production on the `digitalocean` host
- Production URL: `https://apipool.dev`
- API health endpoint: `https://api.apipool.dev/health`
- Primary runtime directory: `/opt/sub2api`

## Trigger

- Deployment trigger: push to `origin/main`, or manual `workflow_dispatch`
- CI/CD workflow: `.github/workflows/deploy.yml`
- Workflow name: `Deploy to DigitalOcean`
- Concurrency group: `deploy-production`; newer runs cancel in-progress runs
- Expected deployment behavior: GitHub Actions builds a GHCR image tagged with the pushed commit, then SSHes to `digitalocean` and recreates the Docker Compose services
- This is not a zero-downtime rollout. The single `sub2api` container is recreated, so a short restart window is expected.

## Runtime Architecture

- Runtime units:
  - `sub2api`
  - `sub2api-postgres`
  - `sub2api-redis`
  - optional `sub2api-biz` when the enterprise compose/env files are present on the server
- Runtime platform: Docker Compose on the `digitalocean` host
- Compose file: `/opt/sub2api/deploy/docker-compose.deploy.yml`
- Application image alias on the server: `deploy-sub2api:latest`
- Rollback image alias on the server: `deploy-sub2api:rollback-latest`
- Database: PostgreSQL container `sub2api-postgres`
- Cache: Redis container `sub2api-redis`
- Application healthcheck: container healthcheck hits `http://localhost:8080/health`

## Pre-Deploy Checks

Run these before pushing production code. Use broader checks when the release affects auth, billing, quota, model routing, deploy scripts, migrations, or public API contracts.

```bash
git status -sb
git branch --show-current
git remote -v
git log --oneline --decorate -n 5

cd backend && go test -tags=unit ./...
cd backend && go test -tags=integration ./...
cd backend && golangci-lint run ./...
pnpm --dir frontend run lint:check
pnpm --dir frontend run typecheck
POSTGRES_PASSWORD=dummy docker compose -f deploy/docker-compose.deploy.yml config -q
```

If a check cannot run locally, record the exact blocker and residual risk before pushing.

## Deployment-Critical Files

Read these before each production deployment:

- `README.md`
- `docs/deployment.md`
- `.github/workflows/deploy.yml`
- `deploy/rollback.sh`
- `deploy/docker-compose.deploy.yml`
- `deploy/version_resolver.sh`
- `deploy/ROLLBACK_CN.md`

Also inspect any changed deployment, compose, migration, rollback, config, or CI/CD files touched by the current release.

## Backup Requirements

- Backup trigger: the `Deploy via SSH` step in `.github/workflows/deploy.yml`, before the server resets code and recreates containers
- Main DB backup artifact: `/opt/sub2api/backups/pre-deploy-YYYYmmdd_HHMMSS.sql.gz`
- Enterprise DB backup artifact when applicable: `/opt/sub2api/backups/pre-deploy-biz-YYYYmmdd_HHMMSS.sql.gz`
- Backup sanity check: backup file must be non-empty and pass `gzip -t`
- Rollback metadata: `/opt/sub2api/backups/last-rollback-image.txt`
- Rollback image alias: `deploy-sub2api:rollback-latest`
- Failure meaning: deployment must stop if required DB backup, gzip validation, rollback image tagging, compose validation, or service health checks fail

## Rollback and Recovery

- Fastest service recovery path:

```bash
ssh digitalocean 'cd /opt/sub2api/deploy && ./rollback.sh image'
```

- Source rebuild fallback when rollback image is unavailable:

```bash
ssh digitalocean 'cd /opt/sub2api/deploy && ./rollback.sh source <commit>'
```

- Database restore plus image rollback, only when persisted state must be restored:

```bash
ssh digitalocean 'cd /opt/sub2api/deploy && ./rollback.sh db-restore --with-image'
```

- Actions requiring fresh explicit confirmation:
  - database restore
  - destructive data changes
  - credential rotation
  - environment rebuilds
  - deleting containers, volumes, images, or backups outside the documented rollback script

After any recovery action, verify container health, live commit/version, `/health`, and recent logs.

## Monitoring During Deployment

CI/CD:

```bash
gh run list -R AFreeCoder/apipool --workflow 'Deploy to DigitalOcean' --limit 3
gh run watch -R AFreeCoder/apipool <run-id> --exit-status
gh run view -R AFreeCoder/apipool <run-id> --json status,conclusion,displayTitle,headSha,jobs
```

Server baseline:

```bash
ssh -o BatchMode=yes digitalocean 'hostname && uptime && docker ps --format "table {{.Names}}\t{{.Status}}\t{{.RunningFor}}\t{{.Image}}"'
```

Runtime version, backup, and rollback metadata:

```bash
ssh digitalocean 'cd /opt/sub2api && git rev-parse --short=12 HEAD && cat backend/cmd/server/VERSION'
ssh digitalocean 'ls -lt /opt/sub2api/backups | head -6'
ssh digitalocean 'cat /opt/sub2api/backups/last-rollback-image.txt'
```

Container and image state:

```bash
ssh digitalocean 'docker ps --format "table {{.Names}}\t{{.Status}}\t{{.RunningFor}}\t{{.Image}}" | grep -E "^NAMES|^sub2api|^sub2api-postgres|^sub2api-redis"'
ssh digitalocean 'docker inspect --format "container={{.Id}} image={{.Image}} created={{.Created}} health={{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}" sub2api'
ssh digitalocean 'docker image inspect deploy-sub2api:latest --format "latest={{.Id}} created={{.Created}}"'
ssh digitalocean 'docker image inspect deploy-sub2api:rollback-latest --format "rollback_latest={{.Id}} created={{.Created}}"'
```

Logs:

```bash
ssh digitalocean 'docker logs --since 2m sub2api 2>&1 | tail -120'
```

External health:

```bash
curl -fsS https://api.apipool.dev/health
```

## Success Criteria

A deployment is complete only when all required criteria are true:

- The GitHub Actions run finishes with `conclusion=success`.
- The server repo `HEAD` matches the pushed commit.
- The server `backend/cmd/server/VERSION` matches the expected runtime version.
- A fresh `pre-deploy-*.sql.gz` exists for the current deployment window and passed workflow validation.
- `last-rollback-image.txt` points to the previous live commit or image.
- `deploy-sub2api:latest` points to the new image.
- `deploy-sub2api:rollback-latest` exists.
- `sub2api-postgres`, `sub2api-redis`, and `sub2api` are healthy.
- Recent `sub2api` logs do not show boot, migration, auth, permission, network, or resource failures relevant to the release.
- External health checks pass.

## Failure Handling

- Failure before live impact: collect the failing GitHub Actions step, build/deploy logs, and current server state; old live service should still be running.
- Failure during image build: do not rollback; fix the build failure and redeploy.
- Failure after the new app is live: prioritize `./rollback.sh image` to restore service quickly.
- Database restore is a last resort and requires explicit confirmation unless incident recovery was already delegated end to end.
- If the real process differs from this file, update this document after the service is stable.
