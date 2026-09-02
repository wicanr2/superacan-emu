# Android 的建置工具鏈。從專案的 Go image 延伸，所以 Go 版本與其他平台一致。
#
# Android 是唯一開 cgo 例外的目標：應用程式的原生碼必須是被 Java runtime 載入的
# 共享程式庫，而 -buildmode=c-shared 在任何平台都要求 cgo（量測見
# docs/android-frontend.md）。所以這個 image 才需要存在。
#
# 三樣東西缺一不可：
#   NDK          交叉編譯 Go 的 cgo 部分（clang + sysroot）
#   SDK platform gomobile 產生 AAR 時要 android.jar 才編得了 Java 端
#   JDK          javac 與 jar
FROM superacan-ebitengine:go1.26.7-v1

USER root
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
        openjdk-17-jdk-headless unzip \
 && rm -rf /var/lib/apt/lists/*

# 版本鎖死：工具鏈版本會影響產出的 .so，不能跟著上游浮動。
ENV ANDROID_HOME=/opt/android-sdk
ENV ANDROID_NDK_VERSION=27.2.12479018
ENV ANDROID_NDK_HOME=${ANDROID_HOME}/ndk/${ANDROID_NDK_VERSION}
ENV ANDROID_PLATFORM=android-34
ENV ANDROID_BUILD_TOOLS=34.0.0

RUN mkdir -p ${ANDROID_HOME}/cmdline-tools \
 && curl -fsSL -o /tmp/cmdline-tools.zip \
      https://dl.google.com/android/repository/commandlinetools-linux-11076708_latest.zip \
 && unzip -q /tmp/cmdline-tools.zip -d ${ANDROID_HOME}/cmdline-tools \
 && mv ${ANDROID_HOME}/cmdline-tools/cmdline-tools ${ANDROID_HOME}/cmdline-tools/latest \
 && rm /tmp/cmdline-tools.zip

RUN yes | ${ANDROID_HOME}/cmdline-tools/latest/bin/sdkmanager --licenses >/dev/null \
 && ${ANDROID_HOME}/cmdline-tools/latest/bin/sdkmanager --install \
      "platforms;${ANDROID_PLATFORM}" \
      "build-tools;${ANDROID_BUILD_TOOLS}" \
      "ndk;${ANDROID_NDK_VERSION}" >/dev/null \
 && rm -rf ${ANDROID_HOME}/.temp ${ANDROID_HOME}/emulator

# ebitenmobile 包住 gomobile；版本跟著 go.mod 裡的 ebiten 走。
ENV GOPATH=/go
ENV PATH=/go/bin:${ANDROID_HOME}/cmdline-tools/latest/bin:${PATH}
RUN go install github.com/hajimehoshi/ebiten/v2/cmd/ebitenmobile@v2.9.9 \
 && chmod -R a+rX /go/bin /go/pkg

# gomobile 會把 Go 的文件註解原樣搬進產生的 Java，而本專案的註解是中文。
# JDK 17 的預設來源編碼跟著 locale 走（JEP 400 是 18 才預設 UTF-8），沒設 locale
# 的話 javac 會對每一個非 ASCII 位元組報 unmappable character——錯誤指著產生出來
# 的檔案，看起來像 gomobile 壞了。
ENV LANG=C.UTF-8
ENV LC_ALL=C.UTF-8

# 組 APK 時要往 aapt2 產出的包裡加 dex 與 .so。
RUN apt-get update \
 && apt-get install -y --no-install-recommends zip \
 && rm -rf /var/lib/apt/lists/*
