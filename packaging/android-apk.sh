#!/bin/sh
# 從 AAR 組出可安裝的 APK。在 superacan-android image 內執行。
#
#   packaging/android-apk.sh <輸出目錄>
#
# 不走 Gradle：Gradle 會再拉一整套相依，而這個 app 只有一個 Activity、一份
# manifest 與一張圖示，用 build-tools 自己的 aapt2／d8／zipalign／apksigner 就組
# 得完，步驟也因此看得見。
#
# 簽的是**除錯金鑰**，只夠側載安裝；要上架或對外散布必須換成自己的發行金鑰。
set -eu

REPO=$(cd "$(dirname "$0")/.." && pwd)
OUT=${1:?用法：android-apk.sh <輸出目錄>}
AAR="$OUT/acan.aar"
[ -f "$AAR" ] || { echo "android-apk.sh: 先跑 android-aar.sh 產出 $AAR" >&2; exit 1; }

SDK=${ANDROID_HOME:?ANDROID_HOME 沒設}
PLATFORM=${ANDROID_PLATFORM:-android-34}
TOOLS="$SDK/build-tools/${ANDROID_BUILD_TOOLS:-34.0.0}"
JAR="$SDK/platforms/$PLATFORM/android.jar"
WORK="$OUT/apk-work"

rm -rf "$WORK"
mkdir -p "$WORK/aar" "$WORK/classes" "$WORK/dex" "$WORK/res"
unzip -q "$AAR" -d "$WORK/aar"

# 圖示：直接用發行包共用的那張 PNG，Android 會依密度縮放。
mkdir -p "$WORK/res/mipmap"
cp "$REPO/packaging/superacan-emu.png" "$WORK/res/mipmap/ic_launcher.png"
cp -r "$REPO/packaging/android/res/." "$WORK/res/"

"$TOOLS/aapt2" compile --dir "$WORK/res" -o "$WORK/res.zip"
"$TOOLS/aapt2" link -o "$WORK/base.apk" -I "$JAR" \
    --manifest "$REPO/packaging/android/AndroidManifest.xml" \
    --java "$WORK/gen" --min-sdk-version 21 --target-sdk-version 34 \
    "$WORK/res.zip"

# android.jar 當 bootclasspath：平台類別要由它提供，不是 JDK 的，否則會編到
# Android 上不存在的 API。JDK 只在 -source 8 以下才接受 -bootclasspath
# （更高的版本要用 --release，但那會把 JDK 自己的平台類別帶進來）。
find "$REPO/packaging/android/java" "$WORK/gen" -name '*.java' > "$WORK/sources.txt"
javac -nowarn -encoding UTF-8 -source 8 -target 8 -bootclasspath "$JAR" \
    -classpath "$WORK/aar/classes.jar" \
    -d "$WORK/classes" @"$WORK/sources.txt" 2>&1 | grep -v 'obsolete\|deprecat' || true

"$TOOLS/d8" --release --lib "$JAR" --output "$WORK/dex" \
    $(find "$WORK/classes" -name '*.class') "$WORK/aar/classes.jar"

# 把 dex 與每個 ABI 的 .so 塞進 aapt2 產出的 APK。
cd "$WORK"
mkdir -p payload/lib
cp dex/classes.dex payload/
cp -r aar/jni/. payload/lib/
cp base.apk unsigned.apk
( cd payload && zip -qr ../unsigned.apk . )

"$TOOLS/zipalign" -f -p 4 unsigned.apk aligned.apk

# 除錯金鑰：沒有的話現做一把。APK 一定要簽才裝得上去。
KEYSTORE="$OUT/debug.keystore"
[ -f "$KEYSTORE" ] || keytool -genkeypair -keystore "$KEYSTORE" -storepass android \
    -keypass android -alias androiddebugkey -keyalg RSA -keysize 2048 -validity 10000 \
    -dname "CN=Super A'Can Debug, OU=, O=, L=, S=, C=TW" >/dev/null 2>&1

"$TOOLS/apksigner" sign --ks "$KEYSTORE" --ks-pass pass:android \
    --key-pass pass:android --ks-key-alias androiddebugkey \
    --out "$OUT/superacan-emu-debug.apk" aligned.apk
"$TOOLS/apksigner" verify --print-certs "$OUT/superacan-emu-debug.apk" | head -3

ls -l "$OUT/superacan-emu-debug.apk"
echo "=== APK 內的原生程式庫 ==="
unzip -l "$OUT/superacan-emu-debug.apk" | awk '{print $4}' | grep -E '\.so$|\.dex$' | sort
