# Bcan 0.0.8b 畫面 oracle 執行環境。
# 用途：在無顯示器的容器內以 Wine 執行 Bcan（Windows x64、Direct3D 11），
# 取得同一畫面的 320x240 截圖，供純 Go renderer 做逐像素差分。
# Bcan.exe、BIOS 與 ROM 皆為版權輸入，只在執行時由外部唯讀掛載，不進映像。
FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        wine64 \
        xvfb \
        x11-utils \
        xdotool \
        openbox \
        imagemagick \
        libgl1-mesa-dri \
        libglx-mesa0 \
        ca-certificates \
    && rm -rf /var/lib/apt/lists/*

ENV WINEDEBUG=-all
