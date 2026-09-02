#!/bin/sh
# 用 ebitenmobile 產出 Android 的 AAR。在 superacan-android image 內執行。
#
#   packaging/android-aar.sh <輸出目錄>
#
# AAR 裡是每個 ABI 一份 .so（Go 端）加上 gomobile 產生的 Java 綁定，以及
# Ebitengine 的 EbitenView。Activity 要做的四個呼叫見 docs/android-frontend.md。
#
# Android 是唯一開 cgo 例外的目標：應用程式的原生碼必須是被 Java runtime 載入的
# 共享程式庫，而 -buildmode=c-shared 在任何平台都要求 cgo。
set -eu

REPO=$(cd "$(dirname "$0")/.." && pwd)
OUT=${1:?用法：android-aar.sh <輸出目錄>}
PKG=github.com/wicanr2/superacan-emu/mobile/acan
ABIS=${ACAN_ANDROID_ABIS:-android/arm64,android/arm,android/amd64}

command -v ebitenmobile >/dev/null || { echo "android-aar.sh: 缺 ebitenmobile" >&2; exit 1; }
[ -d "${ANDROID_NDK_HOME:-}" ] || { echo "android-aar.sh: ANDROID_NDK_HOME 沒設或不存在" >&2; exit 1; }

mkdir -p "$OUT"
cd "$REPO"
ebitenmobile bind \
    -target "$ABIS" \
    -androidapi 21 \
    -javapkg tw.wicanr2.superacan \
    -o "$OUT/acan.aar" \
    "$PKG"

ls -l "$OUT/acan.aar"
echo "=== AAR 內容 ==="
unzip -l "$OUT/acan.aar" | awk '{print $4}' | grep -E '\.so$|\.jar$|AndroidManifest' | sort
