#!/usr/bin/env bash
# 构建期下载 onnxruntime 共享库到 backend/internal/zhidao/assets/（不入 git）。
# 用法: fetch-onnxruntime.sh <goos> <goarch>
# 幂等: 目标已存在且大小合理则跳过；失败 exit 1。
# 版本锁定: 微软官方 onnxruntime v1.25.0（与 go.sum 的 onnxruntime_go v1.25.0 对齐）。
set -euo pipefail

GOOS="${1:?usage: fetch-onnxruntime.sh <goos> <goarch>}"
GOARCH="${2:?usage: fetch-onnxruntime.sh <goos> <goarch>}"
ASSETS_DIR="$(cd "$(dirname "$0")/../internal/zhidao/assets" && pwd)"

# 微软官方 onnxruntime v1.25.0 各平台包名与目标文件名。
# Android 特例：微软不在 GitHub Releases 发 aar（桌面库才进 releases），
# Android 走 Maven Central（com.microsoft.onnxruntime:onnxruntime-android）。
case "${GOOS}-${GOARCH}" in
  windows-amd64) PKG="onnxruntime-win-x64-1.25.0.zip";   INNER="lib/onnxruntime.dll";      OUT="onnxruntime_win_amd64.dll";;
  windows-arm64) PKG="onnxruntime-win-arm64-1.25.0.zip"; INNER="lib/onnxruntime.dll";      OUT="onnxruntime_win_arm64.dll";;
  linux-amd64)   PKG="onnxruntime-linux-x64-1.25.0.tgz";      INNER="lib/libonnxruntime.so";     OUT="libonnxruntime_linux_amd64.so";;
  linux-arm64)   PKG="onnxruntime-linux-aarch64-1.25.0.tgz";  INNER="lib/libonnxruntime.so";     OUT="libonnxruntime_linux_arm64.so";;
  darwin-arm64)  PKG="onnxruntime-osx-arm64-1.25.0.tgz";      INNER="lib/libonnxruntime.dylib";  OUT="libonnxruntime_darwin_arm64.dylib";;
  android-arm64) PKG="onnxruntime-android-1.25.0.aar";        INNER="jni/arm64-v8a/libonnxruntime.so"; OUT="libonnxruntime_android_arm64.so";;
  *) echo "不支持的平台: ${GOOS}-${GOARCH}" >&2; exit 1;;
esac

if [ "${GOOS}-${GOARCH}" = "android-arm64" ]; then
  URL="https://repo1.maven.org/maven2/com/microsoft/onnxruntime/onnxruntime-android/1.25.0/onnxruntime-android-1.25.0.aar"
else
  URL="https://github.com/microsoft/onnxruntime/releases/download/v1.25.0/${PKG}"
fi

# 幂等：目标已存在且大小合理（>10MB）则跳过
if [ -f "$ASSETS_DIR/$OUT" ] && [ "$(stat -c%s "$ASSETS_DIR/$OUT" 2>/dev/null || stat -f%z "$ASSETS_DIR/$OUT")" -gt 10000000 ]; then
  echo "[fetch-onnxruntime] 已存在: $ASSETS_DIR/$OUT"
  exit 0
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "[fetch-onnxruntime] 下载 $URL"
curl -fsSL "$URL" -o "$TMP/pkg"
# 解包目录必须提前建好（tar -C 不自建，unzip -d 会自建）
mkdir -p "$TMP/x"
if [[ "$PKG" == *.zip || "$PKG" == *.aar ]]; then
  unzip -o -q "$TMP/pkg" -d "$TMP/x"
else
  tar -xzf "$TMP/pkg" -C "$TMP/x"
fi
# 各平台包顶层目录不一（tgz/zip 带 onnxruntime-<os>-<ver>/ 层，aar 直接是 jni/）。
#
# **Android aar 必须按 ABI 目录精确定位，绝不能用 find | head -1**：aar 内
# jni/ 下有 arm64-v8a/ armeabi-v7a/ x86/ x86_64/ 四个同名 libonnxruntime.so，
# find 的遍历顺序由文件系统目录项顺序决定（不同 runner/文件系统上不同，
# 本地实测取到 arm64、CI runner 上取到 x86_64），取错即装机报
# "dlopen failed: ... for EM_X86_64 (62) instead of EM_AARCH64 (183)"。
# 修法：GOARCH→ABI 目录映射后直接拼路径，取不到就报错退出（绝不 fallback 到
# 任意架构）；zip/tgz 包内单架构，按文件名 find 即可（保留原逻辑）。
if [ "$OUT" = "libonnxruntime_android_arm64.so" ]; then
  ABI_DIR="arm64-v8a"
  FOUND="$TMP/x/jni/$ABI_DIR/libonnxruntime.so"
  if [ ! -f "$FOUND" ]; then
    echo "[fetch-onnxruntime] aar 内未找到 $ABI_DIR/libonnxruntime.so（包结构异常）" >&2
    echo "  实际内容: $(find "$TMP/x" -name 'libonnxruntime.so' | tr '\n' ' ')" >&2
    exit 1
  fi
else
  FNAME="$(basename "$INNER")"
  FOUND="$(find "$TMP/x" \( -type f -o -type l \) -name "$FNAME" | head -1)"
  if [ -z "$FOUND" ]; then
    echo "[fetch-onnxruntime] 未在包内找到 $FNAME" >&2
    exit 1
  fi
fi
cp -L "$FOUND" "$ASSETS_DIR/$OUT"

# **架构自检（关键）**：ELF e_machine 在偏移 0x12（ELF64 little-endian）。
# AARCH64=0xB7(183) / x86_64=0x3E(62)。架构错配只在装机 dlopen 时才炸
# （Go 侧 dlopen 报 EM_X86_64 instead of EM_AARCH64），必须在构建期拦住。
case "$OUT" in
  *android_arm64*) WANT_MACHINE="b7 3e 00 00" ;; # 0xB7 LE = b7 3e
  *linux_amd64*)   WANT_MACHINE="3e 00 00 00" ;;
  *linux_arm64*)   WANT_MACHINE="b7 3e 00 00" ;;
  *) WANT_MACHINE="" ;;
esac
if [ -n "$WANT_MACHINE" ]; then
  GOT_MACHINE="$(od -An -tx1 -j 18 -N 2 "$ASSETS_DIR/$OUT" | tr -s ' ' | sed 's/^ //;s/ $//')"
  if [ "$GOT_MACHINE" != "$WANT_MACHINE" ]; then
    echo "[fetch-onnxruntime] 架构错配！期望 e_machine=$WANT_MACHINE 实际=$GOT_MACHINE（文件 $OUT）" >&2
    exit 1
  fi
  echo "[fetch-onnxruntime] 架构校验通过: $OUT e_machine=$GOT_MACHINE"
fi

# 体积粗校验（>10MB 视为非空库；精确 sha256 由 CI/后续加固）
if [ "$(stat -c%s "$ASSETS_DIR/$OUT" 2>/dev/null || stat -f%z "$ASSETS_DIR/$OUT")" -lt 10000000 ]; then
  echo "[fetch-onnxruntime] 下载文件异常（过小）" >&2
  exit 1
fi
echo "[fetch-onnxruntime] 完成: $ASSETS_DIR/$OUT"
