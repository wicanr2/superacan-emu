# 發行包與展示影片

更新日期：2026-09-02。

Linux 的發行形式是 AppImage：單一檔案、不需要安裝、不依賴發行版的套件庫。裡面裝
的是 `cmd/acan-x11`，那是 `CGO_ENABLED=0` 的純 Go 執行檔，所以這個包沒有任何動態
連結的第三方相依。

ROM、BIOS 與遊戲畫面都不進發行包。韌體與卡帶由使用者自己放到
`~/.local/share/superacan-emu/` 底下，程式不隨附也不代為下載。

## AppImage

### 為什麼可以不用 appimagetool

type-2 AppImage 的結構就是「一個 static-pie 的 runtime ELF」接上「一個 squashfs」。
`appimagetool` 做的是同一件事再加上一些檢查，所以在沒有網路的容器裡也能自己組：

```sh
mksquashfs AppDir payload.squashfs -root-owned -noappend -comp gzip -b 128K
cat runtime payload.squashfs > SuperACan-x86_64.AppImage
chmod +x SuperACan-x86_64.AppImage
```

runtime 本身不在本 repo（它是 AppImage 專案的產物）。取得方式有兩個：從
[type2-runtime](https://github.com/AppImage/type2-runtime) 下載，或從任何一個既有的
type-2 AppImage 前面切下來——

```sh
off=$(./某個.AppImage --appimage-offset)
dd if=某個.AppImage of=runtime bs=$off count=1
```

`--appimage-offset` 印的就是 runtime 的長度。**不要用 grep 找 squashfs 的 `hsqs`
魔數當作切點**：那四個位元組會在 runtime 內部出現，切出來的檔案看起來像 ELF，
`file` 卻會抱怨 section header 落在檔案之外。

### 建置

```sh
# 1. 純 Go 執行檔
ACAN_CGO=0 docker/go.sh build -o /src/build/acan-x11 ./cmd/acan-x11/

# 2. 圖示（建置產物，可從原始碼重現）
docker/go.sh run ./packaging/icon /src/packaging/superacan-emu.png

# 3. 組出 AppImage（在 superacan-package image 內）
ACAN_APPIMAGE_RUNTIME=<runtime> packaging/appimage.sh build/SuperACan-x86_64.AppImage
```

`docker/package.Dockerfile` 是打包與編碼用的工具鏈，從專案的 Go image 延伸，
只多了 `squashfs-tools`、`ffmpeg`、`xvfb` 與 `file`（多出約 264 MB）。

### 驗證

AppImage 跑出來的結果要與模擬核心逐位元相同。在沒有 FUSE 的容器裡用
`APPIMAGE_EXTRACT_AND_RUN=1` 執行：

```sh
DISPLAY=:99 ./SuperACan-x86_64.AppImage --ipl … --key … --sound-bios1 … --sound-bios2 … \
    --rom "…/Boom Zoo (Taiwan).bin" --scale 3 --pace=false --frames 300 --config none
→ frames=300 instructions=4364786 framebuffer_sha256=defbd19a…885c6
```

這組數字與 `verify-ui.md` 記錄的 headless 基準相同，所以包起來的執行檔沒有被打包
流程改變行為。

### 預設路徑

發行包要能直接點兩下就開，所以沒有給旗標時每個路徑都有預設值（`$XDG_DATA_HOME`
沒設時用 `~/.local/share`）：

```
~/.local/share/superacan-emu/firmware/internal_68k.bin  等四份韌體
~/.local/share/superacan-emu/cartridges/                卡帶
```

缺韌體或缺卡帶不是啟動失敗：啟動畫面（S0）會列出四份韌體各自的狀態與雜湊，
韌體畫面會說明檔案該放到哪裡。這比「跳出一行錯誤然後結束」有用。

### 第三方授權

`packaging/THIRD-PARTY-LICENSES` 隨包散布，內容是 bitmapfont/v4 的 Apache-2.0
原文、Baekmuk 授權原文與它要求的商標標示、M+ Bitmap Fonts 授權原文，以及六份
字型來源的出處與授權名稱。

**目前不符合散布條件**：其中四份來源採用 OFL-1.1，而該授權要求原文隨字型散布；
OFL-1.1 的原文不在 bitmapfont 模組內，本專案也還沒把它取進來。在補上之前，
這個 AppImage 只能自用與內部驗證，不可對外散布。

## 展示影片

影片是用**發行的 AppImage** 錄的，不是用開發中的執行檔：跑的就是那個檔案，
輸入是腳本，所以同一份腳本在同一份 AppImage 上會得到同一段影片。

### 錄的是合成後的視窗

`--record <檔案.avi>` 錄的是合成之後的視窗——遊戲畫面加上覆蓋層。這條路與截圖、
一般錄影**不共用**：後者取的是 UM6618 的顯示孔徑，不含覆蓋層，那是給畫面證據用的。
展示影片要看得到選單與診斷畫面，所以走的是另一條，而且它**不能當畫面證據**。

取樣節奏是主機迴圈而不是模擬 frame。覆蓋層開著時模擬時間停住，但使用者看到的
畫面仍然在動，不補這些幀的話走選單那一段在影片裡會是靜止的。

補幀帶出音訊的問題：那些幀沒有樣本。AVI 的音訊是一條連續的串流，少掉的部分會讓
**後面的聲音整體提前**，不是「那一段沒聲音」。所以每錄一幀就把音訊補到
`48000/60 × 4` 位元組的整數倍，缺的部分寫靜音。

### 為什麼用讀檔而不是從頭跑

這些遊戲從開機跑到標題畫面要數千幀：Boom Zoo 約 6000 幀、Monopoly 約 3600 幀。
以每 tick 一個影片幀計算，那是一百秒的開機畫面。所以影片先用同一支程式預先做好
三個存檔（走的是與正式錄影同一條「啟動畫面 → 瀏覽器 → 載入」的路，存檔目錄才會
落在同一個地方），正式錄影時用讀檔跳到有東西看的畫面——那既是這個模擬器的功能，
也讓影片不必停在黑畫面上。

### 錄影前先重建

影片的內容來自 AppImage 裡的執行檔，不是工作區裡的原始碼。改過 `session` 或
`cmd/acan-x11` 之後直接重錄，錄到的是舊行為——而影片看起來完全正常，只有仔細量
才會發現（例如音訊長度對不上畫面）。所以錄影的第一步永遠是重建：

```sh
ACAN_CGO=0 docker/go.sh build -o /src/build/acan-x11 ./cmd/acan-x11/
ACAN_APPIMAGE_RUNTIME=<runtime> packaging/appimage.sh build/SuperACan-x86_64.AppImage
```

`packaging/promo.sh` 會印出它用的 AppImage 的 SHA-256，錄出來的影片因此可以回溯
到某一份執行檔。

### 重現

```sh
docker run --rm --network none --memory 6g --cpus 4 --pids-limit 256 \
    --log-opt max-size=10m --log-opt max-file=3 \
    -u "$(id -u):$(id -g)" -v "$PWD:/src" -v "$PWD/build:/build" \
    -v <bios>:/bios:ro -v <roms>:/media:ro \
    -e HOME=/tmp -e APPIMAGE_EXTRACT_AND_RUN=1 -w /src \
    superacan-package:v1 sh -c 'packaging/promo.sh /build/promo'
```

產物是 `promo.avi`（MJPEG＋PCM，純 Go 錄出來的）與 `superacan-emu-promo.mp4`
（H.264＋AAC）。**影片含遊戲執行畫面，版權仍屬原廠商**，與 `docs/screenshots/`
同一個限制。
