#!/usr/bin/env bash
set -euo pipefail

NO_FETCH=0
if [[ "${1:-}" == "--no-fetch" ]]; then
  NO_FETCH=1
fi

git rev-parse --show-toplevel >/dev/null 2>&1
ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"

if [[ $NO_FETCH -eq 0 ]]; then
  git fetch upstream --tags >/dev/null 2>&1
fi

BRANCH="$(git branch --show-current)"
HEAD_SHA="$(git rev-parse HEAD)"
UPSTREAM_SHA="$(git rev-parse upstream/main)"

if git rev-parse HEAD^2 >/dev/null 2>&1; then
  LOCAL_BASELINE="$(git rev-parse HEAD^1)"
  UPSTREAM_PARENT="$(git rev-parse HEAD^2)"
  MERGE_BASE="$(git merge-base "$LOCAL_BASELINE" "$UPSTREAM_PARENT")"
else
  LOCAL_BASELINE="$HEAD_SHA"
  UPSTREAM_PARENT="$UPSTREAM_SHA"
  MERGE_BASE="$(git merge-base HEAD upstream/main)"
fi

LOCAL_VERSION="$(cat backend/cmd/server/VERSION 2>/dev/null || echo 'N/A')"
UPSTREAM_VERSION="$(git show upstream/main:backend/cmd/server/VERSION 2>/dev/null || echo 'N/A')"
LATEST_TAG="$(git tag --merged upstream/main --sort=-version:refname | head -1)"

LOCAL_ONLY_COUNT="$(git rev-list --count "$MERGE_BASE..$LOCAL_BASELINE")"
UPSTREAM_ONLY_COUNT="$(git rev-list --count "$MERGE_BASE..$UPSTREAM_PARENT")"

OVERLAP_FILES="$(
  comm -12 \
    <(git diff --name-only "$MERGE_BASE..$LOCAL_BASELINE" | sort) \
    <(git diff --name-only "$MERGE_BASE..$UPSTREAM_PARENT" | sort) || true
)"

HIGH_RISK_PATTERN='(^README|^deploy/|AppHeader|SettingsView|ratelimit_service|openai_oauth|openai_gateway|codex|oauth|VERSION|release|workflow|logo)'
HIGH_RISK_OVERLAP="$(printf '%s\n' "$OVERLAP_FILES" | rg "$HIGH_RISK_PATTERN" || true)"

cat <<EOF
== Upstream Sync Context ==
branch: $BRANCH
head: $HEAD_SHA
local_baseline: $LOCAL_BASELINE
upstream/main: $UPSTREAM_SHA
upstream_parent: $UPSTREAM_PARENT
merge_base: $MERGE_BASE
latest_upstream_tag: $LATEST_TAG
local_version: $LOCAL_VERSION
upstream_version: $UPSTREAM_VERSION
local_only_commits: $LOCAL_ONLY_COUNT
upstream_only_commits: $UPSTREAM_ONLY_COUNT

== Working Tree ==
$(git status -sb)

== Upstream-only Commits ==
$(git log --oneline --no-merges "$LOCAL_BASELINE..$UPSTREAM_PARENT" || true)

== Local-only Commits ==
$(git log --oneline --no-merges "$MERGE_BASE..$LOCAL_BASELINE" || true)

== File Overlap ==
${OVERLAP_FILES:-<none>}

== High-risk Overlap ==
${HIGH_RISK_OVERLAP:-<none>}
EOF
