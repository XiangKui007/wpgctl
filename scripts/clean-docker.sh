#!/usr/bin/env bash
# Stop and remove Docker containers on this Linux host (field teardown / redeploy).
#
# Usage:
#   bash clean-docker.sh -y
#   bash clean-docker.sh -y --all
#   bash clean-docker.sh -y --compose /path/to/middleware /path/to/platform

set -euo pipefail

YES=0
REMOVE_IMAGES=0
REMOVE_VOLUMES=0
PRUNE_NETWORKS=0
COMPOSE_DIRS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    -y|--yes) YES=1; shift ;;
    --images) REMOVE_IMAGES=1; shift ;;
    --volumes) REMOVE_VOLUMES=1; shift ;;
    --all) REMOVE_IMAGES=1; REMOVE_VOLUMES=1; PRUNE_NETWORKS=1; shift ;;
    --compose)
      shift
      [[ $# -gt 0 ]] || { echo "error: --compose requires a directory" >&2; exit 1; }
      COMPOSE_DIRS+=("$1")
      shift
      ;;
    -h|--help)
      echo "Usage: bash clean-docker.sh [-y] [--all] [--compose DIR ...]"
      exit 0
      ;;
    *) COMPOSE_DIRS+=("$1"); shift ;;
  esac
done

if [[ -n "${COMPOSE_ROOTS:-}" ]]; then
  # shellcheck disable=SC2206
  extra=(${COMPOSE_ROOTS})
  COMPOSE_DIRS+=("${extra[@]}")
fi

command -v docker >/dev/null 2>&1 || { echo "error: docker not found" >&2; exit 1; }

compose_bin() {
  if docker compose version >/dev/null 2>&1; then
    echo "docker compose"
  elif command -v docker-compose >/dev/null 2>&1; then
    echo "docker-compose"
  else
    echo ""
  fi
}

compose_down_dir() {
  local dir="$1" bin file name
  bin="$(compose_bin)"
  [[ -n "$bin" ]] || return 0
  [[ -d "$dir" ]] || return 0
  for name in docker-compose.yml docker-compose.yaml compose.yml; do
    if [[ -f "${dir}/${name}" ]]; then
      echo "compose down: ${dir}/${name}"
      (cd "$dir" && $bin -f "$name" down --remove-orphans) || true
      return 0
    fi
  done
}

echo "=== Docker cleanup ==="
echo "containers: $(docker ps -aq 2>/dev/null | wc -l)"
echo "images:     $(docker images -q 2>/dev/null | wc -l)"

if [[ "$YES" -ne 1 ]]; then
  read -r -p "Continue? [y/N] " ans
  case "$ans" in y|Y|yes|YES) ;; *) echo "cancelled"; exit 0 ;; esac
fi

for dir in "${COMPOSE_DIRS[@]}"; do
  compose_down_dir "$dir"
  [[ -d "$dir" ]] || continue
  while IFS= read -r -d '' sub; do
    compose_down_dir "$sub"
  done < <(find "$dir" -mindepth 1 -maxdepth 2 -type d -print0 2>/dev/null || true)
done

ids="$(docker ps -aq 2>/dev/null || true)"
if [[ -n "$ids" ]]; then
  echo "stopping/removing containers..."
  docker stop $ids >/dev/null 2>&1 || true
  docker rm -f $ids >/dev/null 2>&1 || true
fi

if [[ "$REMOVE_IMAGES" -eq 1 ]]; then
  imgs="$(docker images -aq 2>/dev/null || true)"
  [[ -n "$imgs" ]] && docker rmi -f $imgs >/dev/null 2>&1 || true
fi

if [[ "$REMOVE_VOLUMES" -eq 1 ]]; then
  vols="$(docker volume ls -q 2>/dev/null || true)"
  [[ -n "$vols" ]] && docker volume rm -f $vols >/dev/null 2>&1 || true
fi

[[ "$PRUNE_NETWORKS" -eq 1 ]] && docker network prune -f >/dev/null 2>&1 || true

echo "done. remaining containers: $(docker ps -aq 2>/dev/null | wc -l)"
