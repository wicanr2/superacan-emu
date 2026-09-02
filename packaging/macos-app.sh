#!/bin/sh
# 組出 macOS 的 .app bundle。在容器內執行；整條路上沒有 Apple 的工具鏈，
# 因為本專案的 macOS 執行檔是 CGO_ENABLED=0 的純 Go，Go 自己就產 Mach-O。
#
#   packaging/macos-app.sh <輸出目錄>
#
# 產物是 <輸出目錄>/Super A'Can.app 與同名的 .zip。
#
# bundle 沒有簽章：`codesign` 只在 macOS 上有，Linux 這端做不出
# _CodeSignature/CodeResources。未簽勝過壞簽——壞簽會被 Gatekeeper 直接拒絕，
# 未簽只是第一次開啟要「右鍵 → 打開」。要對外散布必須在 Mac 上重簽並公證。
set -eu

REPO=$(cd "$(dirname "$0")/.." && pwd)
OUT=${1:?用法：macos-app.sh <輸出目錄>}
APP="$OUT/Super A'Can.app"

for arch in arm64 amd64; do
  [ -f "$REPO/build/acan-macos-$arch" ] || {
    echo "macos-app.sh: 缺 build/acan-macos-$arch" >&2; exit 1; }
done

rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"

# 每個架構各編一次再合併。單次雙架構在很多建置系統下會出事，而且這樣才驗得到
# 每一弧各自的載入命令。
go run "$REPO/packaging/macho" fat "$APP/Contents/MacOS/acan-macos" \
    "$REPO/build/acan-macos-arm64" "$REPO/build/acan-macos-amd64"
chmod 755 "$APP/Contents/MacOS/acan-macos"

cp "$REPO/packaging/Info.plist" "$APP/Contents/Info.plist"
cp "$REPO/packaging/superacan-emu.icns" "$APP/Contents/Resources/AppIcon.icns"
cp "$REPO/LICENSE" "$APP/Contents/Resources/LICENSE"
cp "$REPO/packaging/THIRD-PARTY-LICENSES" "$APP/Contents/Resources/THIRD-PARTY-LICENSES"
printf 'APPL????' > "$APP/Contents/PkgInfo"

# Linux 上執行不了 macOS 執行檔，所以「編得出來」與「跑得起來」之間只能靠靜態
# 檢查補：雙弧、arm64 有簽章、最低系統版本、相依只在系統路徑。
go run "$REPO/packaging/macho" check "$APP/Contents/MacOS/acan-macos"

go run "$REPO/packaging/zipdir" "$APP" "$OUT/Super A'Can.app.zip"
ls -ld "$APP"; ls -l "$OUT/Super A'Can.app.zip"
