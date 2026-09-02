# 打包與影片編碼用的工具鏈。從專案的 Go image 延伸，所以建置出來的執行檔與
# 這裡用的 runtime 是同一套。
#
# 只加三樣東西：
#   squashfs-tools   AppImage 的內容是一個 squashfs，接在 runtime ELF 後面
#   ffmpeg           把純 Go 錄出來的 AVI（MJPEG＋PCM）轉成 H.264 MP4
#   xvfb             沒有實體顯示器時給 X11 前端一個 X server
FROM superacan-ebitengine:go1.26.7-v1

USER root
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
        squashfs-tools \
        ffmpeg \
        xvfb \
        file \
 && rm -rf /var/lib/apt/lists/*
