#!/bin/sh
# 打出 Linux 的 AppImage。在容器內執行（superacan-package image），不在主機上跑。
#
#   ACAN_APPIMAGE_RUNTIME=<type-2 runtime 檔> packaging/appimage.sh <輸出.AppImage>
#
# runtime 是 AppImage 的前半段（一個 static-pie ELF），本 repo 不收錄：它是
# AppImage 專案的產物，取得方式是從任何一個既有的 type-2 AppImage 前面切下來——
#   off=$(./某個.AppImage --appimage-offset)
#   dd if=某個.AppImage of=runtime bs=$off count=1
# 或直接從 https://github.com/AppImage/type2-runtime 下載。
set -eu

REPO=$(cd "$(dirname "$0")/.." && pwd)
OUT=${1:?用法：appimage.sh <輸出.AppImage>}
RUNTIME=${ACAN_APPIMAGE_RUNTIME:?請用 ACAN_APPIMAGE_RUNTIME 指定 type-2 runtime}
BUILD=${ACAN_APPIMAGE_BUILD:-$REPO/build/appimage}

command -v mksquashfs >/dev/null || { echo "appimage.sh: 缺 mksquashfs" >&2; exit 1; }
[ -x "$REPO/build/acan-x11" ] || { echo "appimage.sh: 先建置 build/acan-x11" >&2; exit 1; }

rm -rf "$BUILD"
mkdir -p "$BUILD/AppDir/usr/bin"
cp "$REPO/build/acan-x11" "$BUILD/AppDir/usr/bin/acan-x11"
cp "$REPO/packaging/AppRun" "$BUILD/AppDir/AppRun"
chmod +x "$BUILD/AppDir/AppRun" "$BUILD/AppDir/usr/bin/acan-x11"
cp "$REPO/packaging/superacan-emu.desktop" "$BUILD/AppDir/superacan-emu.desktop"
cp "$REPO/packaging/superacan-emu.png" "$BUILD/AppDir/superacan-emu.png"
# .DirIcon 是 AppImage 的圖示慣例；桌面環境找的是這個名字。
cp "$REPO/packaging/superacan-emu.png" "$BUILD/AppDir/.DirIcon"
cp "$REPO/LICENSE" "$BUILD/AppDir/LICENSE"
cp "$REPO/packaging/THIRD-PARTY-LICENSES" "$BUILD/AppDir/THIRD-PARTY-LICENSES"

# gzip 而不是 zstd：舊的 runtime 不一定支援 zstd，而這個包不大，
# 壓縮率換相容性划算。
mksquashfs "$BUILD/AppDir" "$BUILD/payload.squashfs" \
    -root-owned -noappend -no-progress -comp gzip -b 128K >/dev/null

cat "$RUNTIME" "$BUILD/payload.squashfs" > "$OUT"
chmod +x "$OUT"
ls -l "$OUT"
