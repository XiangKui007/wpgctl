#!/usr/bin/env bash
# wpgctl 构建脚本（Linux / macOS / Git Bash）
# 用法：
#   ./scripts/build.sh                  # 默认：先编前端再打 Linux amd64
#   SKIP_UI=1 ./scripts/build.sh        # 只编 Go，沿用已有 dist
#   ./scripts/build.sh release          # 前端 + linux amd64 + arm64
#   ./scripts/build.sh build            # 本机平台
#   ./scripts/build.sh ui-build

set -euo pipefail
cd "$(dirname "$0")/.."

APP=wpgctl
MODULE=github.com/wpg/wpgctl
TARGET="${1:-build-linux}"

VERSION="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
BUILT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
LDFLAGS="-s -w -X ${MODULE}/internal/version.Version=${VERSION} -X ${MODULE}/internal/version.GitCommit=${COMMIT} -X ${MODULE}/internal/version.BuildTime=${BUILT}"

mkdir -p bin

ui_build() {
  echo ">> 构建前端 (web/)..."
  (cd web && npm install && npm run build)
}

go_build() {
  local goos="$1" goarch="$2" out="$3"
  echo ">> go build GOOS=${goos} GOARCH=${goarch} -> bin/${out}"
  GOOS="${goos}" GOARCH="${goarch}" CGO_ENABLED=0 \
    GOPROXY="${GOPROXY:-https://goproxy.cn,direct}" \
    go build -ldflags "${LDFLAGS}" -o "bin/${out}" ./cmd/wpgctl
  ls -lh "bin/${out}"
}

case "${TARGET}" in
  ui-build)
    ui_build
    ;;
  build)
    go_build "$(go env GOOS)" "$(go env GOARCH)" "${APP}"
    ;;
  build-linux)
    if [[ "${SKIP_UI:-}" != "1" ]]; then ui_build; fi
    go_build linux amd64 "${APP}-linux-amd64"
    ;;
  build-arm64)
    if [[ "${SKIP_UI:-}" != "1" ]]; then ui_build; fi
    go_build linux arm64 "${APP}-linux-arm64"
    ;;
  release)
    ui_build
    go_build linux amd64 "${APP}-linux-amd64"
    go_build linux arm64 "${APP}-linux-arm64"
    echo
    echo "发布产物："
    ls -lh bin/${APP}-linux-*
    echo
    echo "现场：chmod +x bin/${APP}-linux-amd64 && ./bin/${APP}-linux-amd64 ui --site site.yaml"
    ;;
  test)
    go test ./...
    ;;
  *)
    echo "未知 target: ${TARGET}" >&2
    echo "可选: build | build-linux | build-arm64 | ui-build | release | test" >&2
    exit 1
    ;;
esac

echo OK
