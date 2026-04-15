#!/usr/bin/env bash
# 本脚本承载 MetricsNode 中与 `build_aar` 相关的构建/验证流程。

set -euo pipefail

TARGET="${1:-android/arm64,android/arm,android/amd64,android/386}"
JAVA_PKG="${2:-com.myflowhub.gomobile}"
OUT_FILE="${3:-android/app/libs/myflowhub.aar}"
ANDROID_API="${ANDROID_API:-26}"

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODULE_DIR="${REPO_ROOT}/nodemobile"
OUT_PATH="${REPO_ROOT}/${OUT_FILE}"

echo "Build AAR via gomobile"
echo "  RepoRoot  : ${REPO_ROOT}"
echo "  ModuleDir : ${MODULE_DIR}"
echo "  Target    : ${TARGET}"
echo "  AndroidApi: ${ANDROID_API}"
echo "  JavaPkg   : ${JAVA_PKG}"
echo "  OutFile   : ${OUT_PATH}"

mkdir -p "$(dirname "${OUT_PATH}")"

if ! command -v gomobile >/dev/null 2>&1; then
  echo "gomobile not found, installing..."
  go install golang.org/x/mobile/cmd/gomobile@latest
fi

export GOWORK=off

pushd "${MODULE_DIR}" >/dev/null
gomobile init
gomobile bind -target "${TARGET}" -androidapi "${ANDROID_API}" -javapkg "${JAVA_PKG}" -o "${OUT_PATH}" .
popd >/dev/null

echo "OK: ${OUT_PATH}"

