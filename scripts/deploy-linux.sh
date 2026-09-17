#!/usr/bin/env bash
# wpgctl 现场部署脚本（在 Linux 主控机上执行）
#
# 用法：
#   ./scripts/deploy-linux.sh ui
#       启动 Web 向导（推荐）
#
#   ./scripts/deploy-linux.sh all --site site.yaml --package /path/to/release --base /path/to/base
#       CLI 一条龙：precheck → init → deploy
#
#   ./scripts/deploy-linux.sh precheck|init|deploy|status|render
#       单步执行
#
# 环境变量：
#   WPGCTL_BIN   wpgctl 路径（默认 ./bin/wpgctl-linux-amd64 或 PATH 中的 wpgctl）

set -euo pipefail
cd "$(dirname "$0")/.."

SITE="${SITE:-site.yaml}"
PACKAGE=""
BASE=""
DOCKER_PKG=""
LOCAL_INIT="--local"
DRY_RUN=""
CONCURRENCY="3"

resolve_bin() {
  if [[ -n "${WPGCTL_BIN:-}" ]]; then
    echo "${WPGCTL_BIN}"
    return
  fi
  if [[ -x "./bin/wpgctl-linux-amd64" ]]; then
    echo "./bin/wpgctl-linux-amd64"
    return
  fi
  if [[ -x "./bin/wpgctl" ]]; then
    echo "./bin/wpgctl"
    return
  fi
  if command -v wpgctl >/dev/null 2>&1; then
    command -v wpgctl
    return
  fi
  echo "错误: 未找到 wpgctl，请先运行 ./scripts/build.sh build-linux" >&2
  exit 1
}

WPG="$(resolve_bin)"

usage() {
  sed -n '2,20p' "$0" | sed 's/^# \?//'
  echo
  echo "选项（all / precheck / init / deploy / render）："
  echo "  --site PATH        site.yaml（默认 site.yaml）"
  echo "  --package PATH     release 包目录（含 manifest.yaml）"
  echo "  --base PATH        base 包目录（可选）"
  echo "  --docker-package PATH  Docker 离线目录（含 offline_install_docker.sh）"
  echo "  --manifest PATH    manifest.yaml（默认可从 package 推断）"
  echo "  --dry-run          deploy 仅渲染，不启动"
  echo "  --remote-init      init 时 SSH 分发到其他节点（默认仅本机 --local）"
  echo "  -h, --help         帮助"
}

MANIFEST=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --site) SITE="$2"; shift 2 ;;
    --package) PACKAGE="$2"; shift 2 ;;
    --base) BASE="$2"; shift 2 ;;
    --docker-package) DOCKER_PKG="$2"; shift 2 ;;
    --manifest) MANIFEST="$2"; shift 2 ;;
    --dry-run) DRY_RUN="--dry-run"; shift ;;
    --remote-init) LOCAL_INIT=""; shift ;;
    -h|--help) usage; exit 0 ;;
    ui|all|precheck|init|deploy|render|status)
      CMD="$1"; shift ;;
    *)
      if [[ -z "${CMD:-}" ]]; then CMD="$1"; fi
      shift ;;
  esac
done

CMD="${CMD:-ui}"

manifest_path() {
  if [[ -n "${MANIFEST}" ]]; then
    echo "${MANIFEST}"
  elif [[ -n "${PACKAGE}" ]]; then
    echo "${PACKAGE}/manifest.yaml"
  fi
}

echo "wpgctl: ${WPG}"
echo "site:   ${SITE}"

case "${CMD}" in
  ui)
    exec "${WPG}" ui --site "${SITE}"
    ;;
  precheck)
    args=(precheck --site "${SITE}")
    mf="$(manifest_path || true)"
    [[ -n "${mf}" ]] && args+=(--manifest "${mf}")
    "${WPG}" "${args[@]}"
    ;;
  init)
    args=(init --site "${SITE}")
    [[ -n "${LOCAL_INIT}" ]] && args+=("${LOCAL_INIT}")
    [[ -n "${BASE}" ]] && args+=(--base "${BASE}")
    [[ -n "${DOCKER_PKG}" ]] && args+=(--docker-package "${DOCKER_PKG}")
    mf="$(manifest_path || true)"
    [[ -n "${mf}" ]] && args+=(--manifest "${mf}")
    "${WPG}" "${args[@]}"
    ;;
  render)
    [[ -z "${PACKAGE}" ]] && { echo "需要 --package"; exit 1; }
    "${WPG}" render --site "${SITE}" --package "${PACKAGE}"
    ;;
  deploy)
    [[ -z "${PACKAGE}" ]] && { echo "需要 --package"; exit 1; }
    "${WPG}" deploy --site "${SITE}" --package "${PACKAGE}" \
      --concurrency "${CONCURRENCY}" ${DRY_RUN}
    ;;
  status)
    "${WPG}" status --site "${SITE}"
    ;;
  all)
    [[ -z "${PACKAGE}" ]] && { echo "需要 --package"; exit 1; }
    echo "======== 1/3 precheck ========"
    args=(precheck --site "${SITE}")
    mf="$(manifest_path || true)"
    [[ -n "${mf}" ]] && args+=(--manifest "${mf}")
    "${WPG}" "${args[@]}"
    echo "======== 2/3 init ========"
    args=(init --site "${SITE}")
    [[ -n "${LOCAL_INIT}" ]] && args+=("${LOCAL_INIT}")
    [[ -n "${BASE}" ]] && args+=(--base "${BASE}")
    [[ -n "${DOCKER_PKG}" ]] && args+=(--docker-package "${DOCKER_PKG}")
    mf="$(manifest_path || true)"
    [[ -n "${mf}" ]] && args+=(--manifest "${mf}")
    "${WPG}" "${args[@]}"
    echo "======== 3/3 deploy ========"
    "${WPG}" deploy --site "${SITE}" --package "${PACKAGE}" \
      --concurrency "${CONCURRENCY}" ${DRY_RUN}
    echo
    echo "L1 完成后请按需执行："
    echo "  ${WPG} nacos import --site ${SITE} --config <rendered>/nacos"
    echo "  ${WPG} db apply --site ${SITE} --package ${PACKAGE}"
    echo "  ${WPG} status --site ${SITE}"
    ;;
  *)
    usage
    exit 1
    ;;
esac
