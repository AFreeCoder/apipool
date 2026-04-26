# GHCR Deploy Pull Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build APIPool Docker images in GitHub Actions, push them to GHCR, and make the production server deploy by pulling the exact commit image instead of building locally.

**Architecture:** The deploy workflow becomes a build-then-deploy pipeline: GitHub runner builds `ghcr.io/afreecoder/apipool:sha-<commit>`, then the server pulls that immutable tag and retags it to `deploy-sub2api:latest`. Compose keeps using the local stable image name so current rollback tags and the enterprise instance remain compatible.

**Tech Stack:** GitHub Actions, GHCR, Docker Buildx, Docker Compose v2, shell deploy script embedded in `.github/workflows/deploy.yml`.

---

### Task 1: Switch Production Compose to Pulled Image

**Files:**
- Modify: `deploy/docker-compose.deploy.yml:1-13`

- [ ] **Step 1: Verify current compose still declares a build**

Run:

```bash
rg -n "build:|context:|dockerfile:" deploy/docker-compose.deploy.yml
```

Expected: output includes `build:`, `context: ..`, and `dockerfile: Dockerfile`.

- [ ] **Step 2: Replace the build block with an image reference**

Edit `deploy/docker-compose.deploy.yml` so the header says the file deploys a prebuilt image, and the `sub2api` service starts with:

```yaml
services:
  sub2api:
    image: ${SUB2API_IMAGE:-deploy-sub2api:latest}
    container_name: sub2api
```

- [ ] **Step 3: Verify compose no longer contains an app build block**

Run:

```bash
rg -n "build:|context:|dockerfile:" deploy/docker-compose.deploy.yml
```

Expected: no output and exit code `1`.

### Task 2: Build and Push GHCR Image Before SSH Deploy

**Files:**
- Modify: `.github/workflows/deploy.yml:1-257`

- [ ] **Step 1: Add workflow permissions, concurrency, and build metadata steps**

Add top-level permissions:

```yaml
permissions:
  contents: read
  packages: write
```

Add deploy concurrency so an older immutable-image deploy cannot finish after a newer one:

```yaml
concurrency:
  group: deploy-production
  cancel-in-progress: true
```

Before `Deploy via SSH`, add checkout, Buildx setup, GHCR login, and metadata calculation:

```yaml
      - name: Checkout
        uses: actions/checkout@v6
        with:
          fetch-depth: 0

      - name: Set image metadata
        id: image
        run: |
          APP_COMMIT="${GITHUB_SHA::12}"
          APP_VERSION="$(tr -d '\r\n' < backend/cmd/server/VERSION)"
          APP_BUILD_DATE="$(git log -1 --format=%cI HEAD)"
          IMAGE_REPO="ghcr.io/afreecoder/apipool"
          IMAGE_TAG="sha-${APP_COMMIT}"
          {
            echo "app_commit=${APP_COMMIT}"
            echo "app_version=${APP_VERSION}"
            echo "app_build_date=${APP_BUILD_DATE}"
            echo "image_repo=${IMAGE_REPO}"
            echo "image_tag=${IMAGE_TAG}"
            echo "deploy_image=${IMAGE_REPO}:${IMAGE_TAG}"
          } >> "$GITHUB_OUTPUT"

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Login to GitHub Container Registry
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.repository_owner }}
          password: ${{ secrets.GITHUB_TOKEN }}
```

- [ ] **Step 2: Add Docker build-push step**

Add this step before SSH deploy:

```yaml
      - name: Build and push image
        uses: docker/build-push-action@v6
        with:
          context: .
          file: Dockerfile
          platforms: linux/amd64
          push: true
          build-args: |
            VERSION=${{ steps.image.outputs.app_version }}
            COMMIT=${{ steps.image.outputs.app_commit }}
            DATE=${{ steps.image.outputs.app_build_date }}
          tags: |
            ${{ steps.image.outputs.deploy_image }}
            ${{ steps.image.outputs.image_repo }}:main
            ${{ steps.image.outputs.image_repo }}:latest
          cache-from: type=gha
          cache-to: type=gha,mode=max
```

- [ ] **Step 3: Pass immutable image values into SSH**

In the `Deploy via SSH` step, add:

```yaml
          envs: DEPLOY_IMAGE,DEPLOY_COMMIT
        env:
          DEPLOY_IMAGE: ${{ steps.image.outputs.deploy_image }}
          DEPLOY_COMMIT: ${{ steps.image.outputs.app_commit }}
```

- [ ] **Step 4: Remove server-side build cache pruning**

Remove the `DOCKER_BUILDX_CACHE_MAX_USED_SPACE` variable, the `prune_build_cache` function, and its final invocation because the server no longer builds images.

- [ ] **Step 5: Replace server-side build with pull and retag**

Replace the block that resolves `APP_VERSION`, `APP_COMMIT`, `APP_BUILD_DATE`, and runs `docker compose ... build` with:

```bash
            if [ -z "${DEPLOY_IMAGE:-}" ] || [ -z "${DEPLOY_COMMIT:-}" ]; then
              echo "缺少 DEPLOY_IMAGE / DEPLOY_COMMIT，停止部署" >&2
              exit 1
            fi

            echo "拉取部署镜像: ${DEPLOY_IMAGE}"
            docker pull "$DEPLOY_IMAGE"
            docker tag "$DEPLOY_IMAGE" deploy-sub2api:latest
            echo "已更新本地部署镜像别名: deploy-sub2api:latest (${DEPLOY_COMMIT})"
            docker compose -f docker-compose.deploy.yml config -q
```

Leave stale container cleanup, `docker compose ... up -d --remove-orphans`, health checks, enterprise sync, rollback tag pruning, and `docker image prune -f` in place.

Also change the server repository reset from `origin/main` to `"$DEPLOY_COMMIT"` after `git fetch --tags origin main`, so the checked-out compose file matches the immutable image built by this workflow run.

### Task 3: Update Rollback Documentation

**Files:**
- Modify: `deploy/rollback.sh:132-164`
- Modify: `README.md:81-86`
- Modify: `deploy/.env.example:145-147`
- Modify: `deploy/ROLLBACK_CN.md:1-232`

- [ ] **Step 1: Keep emergency source rollback usable**

Because `deploy/docker-compose.deploy.yml` no longer has a `build:` section, update `build_app_image_from_current_source()` in `deploy/rollback.sh` to build directly from the repository Dockerfile:

```bash
  (
    cd "$APP_DIR"
    docker build \
      -t "${IMAGE_REPO}:latest" \
      --build-arg VERSION="$app_version" \
      --build-arg COMMIT="$app_commit" \
      --build-arg DATE="$app_build_date" \
      -f "$APP_DIR/Dockerfile" \
      "$APP_DIR"
  )
```

- [ ] **Step 2: Update deployment mode summary**

Change the deployment mode line to:

```markdown
- 部署方式：GitHub Actions 构建并推送 GHCR 镜像，服务器 `docker pull` 后用 `docker compose -f docker-compose.deploy.yml up -d`
```

- [ ] **Step 3: Update automatic deploy description**

Change section `1.1` so it says the workflow runs backup and rollback tagging before replacing `deploy-sub2api:latest`, not before building on the server.

- [ ] **Step 4: Demote source rollback**

Rename section `3` to “紧急路径：源码回退后重建” and add a warning that this path rebuilds on the server and should only be used when no rollback image is available.

- [ ] **Step 5: Update README and env example**

Remove the stale production note that auto-deploy controls Docker build cache on the server. In `deploy/.env.example`, remove `DOCKER_BUILDX_CACHE_MAX_USED_SPACE` because the normal deploy workflow no longer reads it.

- [ ] **Step 6: Update failure classification**

Change section `5.1` from server-side `docker compose build` failure to GitHub Actions GHCR build or push failure. Change section `5.3` so the first recommendation is checking older rollback image tags, with source rebuild as the last fallback.

### Task 4: Verify and Commit

**Files:**
- Modify: `.github/workflows/deploy.yml`
- Modify: `deploy/docker-compose.deploy.yml`
- Modify: `deploy/ROLLBACK_CN.md`
- Create: `docs/superpowers/plans/2026-04-27-ghcr-deploy-pull.md`

- [ ] **Step 1: Validate compose config**

Run:

```bash
POSTGRES_PASSWORD=test docker compose -f deploy/docker-compose.deploy.yml config -q
```

Expected: exit code `0`.

- [ ] **Step 2: Verify compose output uses the local image alias**

Run:

```bash
POSTGRES_PASSWORD=test docker compose -f deploy/docker-compose.deploy.yml config | rg -n "image: deploy-sub2api:latest|build:"
```

Expected: output includes `image: deploy-sub2api:latest`; output does not include `build:`.

- [ ] **Step 3: Verify workflow structure**

Run:

```bash
rg -n "permissions:|docker/build-push-action|DEPLOY_IMAGE|docker pull|docker compose -f docker-compose.deploy.yml build|DOCKER_BUILDX_CACHE_MAX_USED_SPACE|prune_build_cache" .github/workflows/deploy.yml
```

Expected: output includes `permissions:`, `docker/build-push-action`, `DEPLOY_IMAGE`, and `docker pull`; output does not include `docker compose -f docker-compose.deploy.yml build`, `DOCKER_BUILDX_CACHE_MAX_USED_SPACE`, or `prune_build_cache`.

- [ ] **Step 4: Verify docs no longer describe normal server builds**

Run:

```bash
rg -n "docker compose build|源码回退后重建|GHCR|docker pull" deploy/ROLLBACK_CN.md
```

Expected: GHCR and `docker pull` are documented, source rebuild is marked as emergency fallback, and no normal deploy path says the server builds the image.

- [ ] **Step 5: Review diff**

Run:

```bash
git diff -- .github/workflows/deploy.yml deploy/docker-compose.deploy.yml deploy/ROLLBACK_CN.md docs/superpowers/plans/2026-04-27-ghcr-deploy-pull.md
```

Expected: diff only includes the approved GHCR deploy pull changes.

- [ ] **Step 6: Commit related changes**

Run:

```bash
git add .github/workflows/deploy.yml deploy/docker-compose.deploy.yml deploy/ROLLBACK_CN.md docs/superpowers/plans/2026-04-27-ghcr-deploy-pull.md
git commit -m "ci: build deploy image in GitHub Actions"
```
