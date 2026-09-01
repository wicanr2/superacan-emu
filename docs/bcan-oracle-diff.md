# Bcan 畫面 oracle 差分

更新日期：2026-09-01

Bcan 0.0.8b 是目前唯一能直接輸出 UM6618 顯示孔徑原生像素的第三方實作，證據等級
`confirmed-Bcan`，高於 archived C++ 與 MAME-derived。本文記錄如何重現這條對照管線、
它能證明與不能證明什麼，以及第一輪的差分結果。

## 為什麼比截圖對拍更嚴格

Bcan 的截圖直接取自顯示孔徑（二進位字串：「The screenshot aperture is not a valid
UM6618 display mode」「The screenshot framebuffer is shorter than 320x240」），輸出固定
320×240 PNG，不含濾鏡、整數縮放與 4:3 修正。本專案 `acan-headless --screenshot-dir`
輸出的也是同一顆晶片的合成結果，因此兩邊可以逐像素比較，不需要縮放或色彩空間轉換。

## 重現方式

環境由 `docker/bcan-oracle.Dockerfile` 固定（Ubuntu 24.04、wine64 9.0、Xvfb、openbox、
xdotool、ImageMagick、Mesa）。Bcan.exe、BIOS 與 ROM 都是版權輸入，只在執行時從外部
掛載，不進映像也不進版控。

```sh
docker build -f docker/bcan-oracle.Dockerfile -t superacan-bcan-wine:noble-v1 .

# /work 底下需備妥可寫的 bcan/（Bcan.exe、Bcan.ini、bios/、ROMS/、snap/）
docker run --rm --network none --memory 6g --cpus 4 --pids-limit 1024 \
    -u "$(id -u):$(id -g)" -v "$WORK:/work" \
    -v "$PWD/docker/bcan-oracle.sh:/oracle.sh:ro" \
    -e HOME=/work -e WINEPREFIX=/work/wineprefix \
    superacan-bcan-wine:noble-v1 bash /oracle.sh "Boom Zoo (Taiwan).bin" 24 5 run1
```

本專案側以同一組 BIOS／ROM 產生候選幀，再用 `acan-imgdiff` 找出最接近的一張：

```sh
go run ./cmd/acan-headless --ipl … --key … --sound-bios1 … --sound-bios2 … \
    --rom "…/Boom Zoo (Taiwan).bin" --frames 1690 \
    --screenshot-dir out/boomzoo --screenshot-every 2
go run ./cmd/acan-imgdiff --reference snap/shot-02.png \
    --candidate-dir out/boomzoo --width 256 --top 3 --diff diff.png
```

### 執行環境的實測限制

這些是實際踩過並確認的，不是推測：

- Xvfb 下沒有視窗管理員時 Wine 收不到 xdotool 的鍵盤事件，必須先啟動 openbox。
- Ctrl+O 在此環境無效；開檔要用滑鼠點「檔案(F)」→「開啟 ROM(O)...」。
- Bcan 沒有以 argv 載入 ROM 的路徑，只能走檔案對話框。
- 容器內以 Mesa 軟體 OpenGL 執行 wined3d，Bcan 的實際幀率遠低於 60 Hz，因此
  「截圖時刻」與硬體 frame 編號沒有固定換算；frame 對齊只能靠差分搜尋。

## 兩端可比與不可比的部分

`--width` 存在的理由：UM6618 在 256 模式（video flags bit 8 = 0）下顯示孔徑只有 256
欄，本專案把右側 64 欄輸出為黑；Bcan 的截圖仍填滿 320 欄。實測 Boom Zoo frame 600
（`video_flags=$9A8A`，bit 8 = 0）時，兩邊差異的 6,119 個像素落在 `x ≥ 256`。這是兩邊
孔徑處理不同，不是任一方的圖層錯誤，因此 256 模式的比較一律加 `--width 256`。
右側 64 欄的硬體真相仍是 `unknown`，不得依 Bcan 截圖改寫本專案 renderer。

## 第一輪結果：5 位元調色盤展開

差分把 Boom Zoo 開場的背景色定位成單一系統性偏移：同一像素 Bcan 輸出
`21/10/73`，本專案輸出 `20/10/70`。反推分量即 R=4、G=2、B=14：

| 分量 | 5 位元值 | 本專案（修正前）`v<<3` | Bcan |
|---|---:|---:|---:|
| R | 4 | `$20` | `$21` |
| G | 2 | `$10` | `$10` |
| B | 14 | `$70` | `$73` |

Bcan 的值等於 `v<<3 | v>>2`，也就是把高 3 位複製到低位，使 `$1F` 對到 `$FF` 而非
`$F8`。MAME supracan driver 宣告 `palette_device::xBGR_555`（`pal5bit`）也是同一個展開。
兩個獨立 oracle 一致，因此 `chip/umc6618/render.go` 已改為 `expand5`，並加上不依賴商業
ROM 的回歸測試。證據等級 `confirmed-Bcan` + `MAME-derived`，尚未有實機量測。

修正後 Boom Zoo 開場同一張 oracle 截圖的差異由 42.51%／平均誤差 13.09 降到
15.03%／10.54（`--width 256`）；另一張降到 6.84%。剩餘差異主要是動畫相位不同
（Bcan 在容器內遠低於 60 Hz，取樣時刻對不上硬體 frame），不是已證實的 renderer 缺陷。

### 受影響的既有基準

1200-frame framebuffer SHA-256 全部因此改變，指令數不變（機器行為未動）：

| ROM | 68000 指令 | framebuffer SHA-256 |
|---|---:|---|
| Speedy Dragon | 18,515,145 | `d3e5336af35b4c5bdac93dca6e1f3686be861564f16d69a97ef8fa947a5b7d67` |
| Formosa Duel | 19,272,069 | `0856269e7b402158e953de03d0553128d720ef64f29afc97403f93471404d587` |
| Boom Zoo | 17,370,088 | `3784f8663b1c3a869498d2e14c0b948c598d50d15cf54b6f5380c9b294155562` |

## 目前擋住嚴格差分的是 CPU 不是 renderer

最適合逐像素定案的畫面是靜止不動的標題選單，它會停在原地等輸入，沒有相位問題。
本專案的 68000 在 Boom Zoo 第 1,695 個 frame（第 24,181,668 條指令、PC `$007D2E`）
遇到未實作的 opcode `$D06A`（`ADD.W (d16,A2),D0`）依約定停止，因此還走不到標題畫面。
在補上該路徑之前，只能拿開場動畫做對照，而開場的相位差會蓋掉小的 renderer 差異。
