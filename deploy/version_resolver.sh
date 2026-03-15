#!/usr/bin/env bash

release_version_regex='^v?[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$'

normalize_version() {
  local raw="${1-}"
  raw="${raw//$'\r'/}"
  raw="${raw//$'\n'/}"
  printf '%s' "$raw"
}

is_release_version() {
  local value
  value="$(normalize_version "${1-}")"
  [[ -n "$value" && "$value" =~ $release_version_regex ]]
}

read_version_file() {
  local repo_dir="${1:-.}"
  local version_file="${repo_dir%/}/backend/cmd/server/VERSION"
  local version

  if [ ! -f "$version_file" ]; then
    return 1
  fi

  version="$(tr -d '\r\n' < "$version_file")"
  if ! is_release_version "$version"; then
    return 1
  fi

  printf '%s\n' "${version#v}"
}

latest_release_tag_version() {
  local repo_dir="${1:-.}"
  local tag

  if [ ! -d "${repo_dir%/}/.git" ]; then
    return 1
  fi

  while IFS= read -r tag; do
    tag="$(normalize_version "$tag")"
    if is_release_version "$tag"; then
      printf '%s\n' "${tag#v}"
      return 0
    fi
  done < <(cd "$repo_dir" && git tag --sort=-version:refname)

  return 1
}

resolve_app_version() {
  local repo_dir="${1:-.}"
  local version

  version="$(read_version_file "$repo_dir" || true)"
  if [ -n "$version" ]; then
    printf '%s\n' "$version"
    return 0
  fi

  version="$(latest_release_tag_version "$repo_dir" || true)"
  if [ -n "$version" ]; then
    printf '%s\n' "$version"
    return 0
  fi

  return 1
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  set -euo pipefail

  command="${1:-resolve}"
  repo_dir="${2:-.}"

  case "$command" in
    resolve)
      resolve_app_version "$repo_dir"
      ;;
    file)
      read_version_file "$repo_dir"
      ;;
    tag)
      latest_release_tag_version "$repo_dir"
      ;;
    *)
      echo "usage: $0 [resolve|file|tag] [repo_dir]" >&2
      exit 1
      ;;
  esac
fi
