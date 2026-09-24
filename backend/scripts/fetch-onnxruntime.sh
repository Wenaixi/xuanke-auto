#!/usr/bin/env bash
# 构建期下载 onnxruntime 共享库到 backend/internal/zhidao/assets/（不入 git）。
# 用法: fetch-onnxruntime.sh <goos> <goarch>
# 幂等: 目标已存在且大小合理则跳过；失败 exit 1。
# 版本锁定: 微软官方 onnxruntime v1.25.0（与 go.sum 的 onnxruntime_go v1.25.0 对齐）。
set -euo pipefail

GOOS="${1:?usage: fetch-onnxruntime.sh <goos> <goarch>}"
GOARCH="${2:?usage: fetch-onnxruntime.sh <goos> <goarch>}"
ASSETS_DIR="$(cd "$(dirname "$0")/../internal/zhidao/assets" && pwd)"

# 微软官方 onnxruntime v1.25.0 各平台包名与目标文件名
case "${GOOS}-${GOARCH}" in
  windows-amd64) PKG="onnxruntime-win-x64-1.25.0.zip";   INNER="lib/onnxruntime.dll";      OUT="onnxruntime_win_amd64.dll";;
  windows-arm64) PKG="onnxruntime-win-arm64-1.25.0.zip"; INNER="lib/onnxruntime.dll";      OUT="onnxruntime_win_arm64.dll";;
  linux-amd64)   PKG="onnxruntime-linux-x64-1.25.0.tgz";      INNER="lib/libonnxruntime.so";     OUT="libonnxruntime_linux_amd64.so";;
  linux-arm64)   PKG="onnxruntime-linux-aarch64-1.25.0.tgz";  INNER="lib/libonnxruntime.so";     OUT="libonnxruntime_linux_arm64.so";;
  darwin-arm64)  PKG="onnxruntime-osx-arm64-1.25.0.tgz";      INNER="lib/libonnxruntime.dylib";  OUT="libonnxruntime_darwin_arm64.dylib";;
  *) echo "不支持的平台: ${GOOS}-${GOARCH}" >&2; exit 1;;
esac

URL="https://github.com/microsoft/onnxruntime/releases/download/v1.25.0/${PKG}"

# 幂等：目标已存在且大小合理（>10MB）则跳过
if [ -f "$ASSETS_DIR/$OUT" ] && [ "$(stat -c%s "$ASSETS_DIR/$OUT" 2>/dev/null || stat -f%z "$ASSETS_DIR/$OUT")" -gt 10000000 ]; then
  echo "[fetch-onnxruntime] 已存在: $ASSETS_DIR/$OUT"
  exit 0
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "[fetch-onnxruntime] 下载 $URL"
curl -fsSL "$URL" -o "$TMP/pkg"
if [[ "$PKG" == *.zip ]]; then
  unzip -o -q "$TMP/pkg" -d "$TMP/x"
else
  tar -xzf "$TMP/pkg" -C "$TMP/x"
fi
cp "$TMP/x/$INNER" "$ASSETS_DIR/$OUT"

# 体积粗校验（>10MB 视为非空库；精确 sha256 由 CI/后续加固）
if [ "$(stat -c%s "$ASSETS_DIR/$OUT" 2>/dev/null || stat -f%z "$ASSETS_DIR/$OUT")" -lt 10000000 ]; then
  echo "[fetch-onnxruntime] 下载文件异常（过小）" >&2
  exit 1
fi
echo "[fetch-onnxruntime] 完成: $ASSETS_DIR/$OUT"
