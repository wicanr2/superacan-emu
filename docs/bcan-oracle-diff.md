# Bcan 畫面 oracle 差分

更新日期：2026-09-02

Bcan 0.0.8b 是目前唯一能直接輸出 UM6618 顯示孔徑原生像素的第三方實作，證據等級
`confirmed-Bcan`，高於 archived C++ 與 MAME-derived。本文記錄如何重現這條對照管線、
它能證明與不能證明什麼，以及兩輪的差分結果。

## 為什麼比截圖對拍更嚴格

Bcan 的截圖直接取自顯示孔徑（二進位字串：「The screenshot aperture is not a valid
UM6618 display mode」「The screenshot framebuffer is shorter than 320x240」），輸出固定
320×240 PNG，不含濾鏡與 4:3 修正。本專案 `acan-headless --screenshot-dir` 輸出的也是
同一顆晶片的合成結果，因此兩邊可以逐像素比較，不需要色彩空間轉換。

唯一要先還原的是水平方向：**孔徑固定 320 欄，但 UM6618 在 256 模式下只輸出 256 欄，
Bcan 用最近鄰把它撐滿孔徑**。實測 Boom Zoo 標題（`video_flags=$9ACC`，bit 8 = 0）的
截圖，第 *x* 欄與第 *x*+1 欄在每一個 *x* ≡ 0 (mod 5) 都完全相同，正好是
`dst = floor(src × 320 / 256)` 的複製樣式；同一批工具下 Sango Fighter 開場
（`video_flags=$03C8`，bit 8 = 1，320 模式）的截圖沒有任何一對相鄰欄相同。

`acan-imgdiff --reference-unstretch 256` 取 `src = ceil(dst × 320 / 256)` 把那一欄丟掉，
還原成 256 欄再比；右側 64 欄補黑，與本專案 256 模式的輸出同框。沒有還原就在比兩張
幾何不同的圖，量到的差異幾乎全是這個縮放，看不到 renderer 本身的問題。

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
    --rom "…/Boom Zoo (Taiwan).bin" --frames 4000 \
    --screenshot-dir out/boomzoo --screenshot-every 4
go run ./cmd/acan-imgdiff --reference snap/shot-10.png \
    --reference-unstretch 256 --width 256 \
    --candidate-dir out/boomzoo --top 3 --diff diff.png \
    --reference-out unstretched.png
```

`--reference-out` 會寫出還原後的參考圖，用來做並排圖；`--reference-unstretch`
在 320 模式的畫面不要加。

### 執行環境的實測限制

這些是實際踩過並確認的，不是推測：

- Xvfb 下沒有視窗管理員時 Wine 收不到 xdotool 的鍵盤事件，必須先啟動 openbox。
- Ctrl+O 在此環境無效；開檔要用滑鼠點「檔案(F)」→「開啟 ROM(O)...」。
- Bcan 沒有以 argv 載入 ROM 的路徑，只能走檔案對話框。
- 容器內以 Mesa 軟體 OpenGL 執行 wined3d，Bcan 的實際幀率遠低於 60 Hz，因此
  「截圖時刻」與硬體 frame 編號沒有固定換算；frame 對齊只能靠差分搜尋。

## 兩端可比與不可比的部分

256 模式的比較一律是 `--reference-unstretch 256 --width 256`：前者還原 Bcan 的
水平放大，後者把右側 64 欄排除在統計外。本專案在 256 模式把右側 64 欄輸出為黑，
Bcan 撐滿；右側 64 欄的硬體真相仍是 `unknown`，不得依 Bcan 截圖改寫本專案 renderer。

垂直方向沒有這個問題：兩邊都是 240 條，不需要還原。

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
15.03%／10.54，另一張降到 6.84%。這組數字量於只加 `--width 256`、還沒還原 Bcan
水平放大的時候，不能與下一節的百分比並列；它證明的是「同一條件下修正讓差異變小」，
不是絕對的相符程度。開場動畫的相位本來就對不上（Bcan 在容器內遠低於 60 Hz），
所以這一輪只能靠色彩定案，不靠落點。

### 受影響的既有基準

1200-frame framebuffer SHA-256 全部因此改變，指令數不變（機器行為未動）。現行值見
[`verify-ui.md` 的卡帶基準（C10）](verify-ui.md#卡帶基準c10)。

## 第二輪結果：4bpp 的半位元組次序

68000 走得到標題畫面之後，靜止的標題選單就能當逐像素基準——它停在原地等輸入，
沒有相位問題。Boom Zoo 標題的版權文字列在此暴露出一個系統性缺陷：字形位置正確，
但每一對相鄰欄互換，字母因此裂成一格一格的直條。

`tilePixel` 的 region 1（4bpp packed，一個位元組兩個像素）原本讓偶數 *x* 取低半
位元組。改成偶數 *x* 取高半位元組之後，Bcan 第 215–222 條與本專案第 208–215 條
（同一段 56 欄）**逐位元組相同**；改回去則每一對相鄰欄互換。整張標題的差異由
27.83% 降到 25.48%（`--reference-unstretch 256 --width 256`）。

第二款：Sango Fighter 開場旁白（320 模式，不需還原）。把畫面下緣 320×44 的文字帶
單獨比對，同一頁旁白的差異由 16.78% 降到 8.68%，其中**第 206–225 條（20 條掃描線
乘 320 欄）逐位元組相同**——那是已經打完字的一整行。剩下的差異只有兩處：最上面一行
被捲動切掉一半，最下面一行兩邊打字進度不同。

證據等級 `confirmed-Bcan`。兩款遊戲一款走 256 模式的 tilemap、一款走 320 模式，
結論一致。`chip/umc6618` 的 `TestTilePixelPackedModes` 釘住這個次序。

region 2（2bpp，一個位元組四個像素）沒有同級證據。目前維持低位優先，
列在 `WORKLIST` 待驗。

## 仍未解釋的差異

- **Boom Zoo 標題的元素垂直落點**：logo 那一塊在 Bcan 從第 27 條開始，本專案從第 33
  條；版權文字列反過來，Bcan 在第 215 條、本專案在第 208 條。整張圖做垂直平移掃描
  時最佳位移是 0，所以不是整體偏移，而是不同圖層各自差幾條。Bcan 六張間隔 4 秒的
  標題截圖彼此只差 1–6%，本專案第 3200–3800 個 frame 的落點也不動，兩邊都已靜止，
  因此不是動畫相位。
- **Monopoly 的標題背景持續斜向捲動**，沒有任何一個 frame 對得上取樣時刻，
  這款不適合當逐像素 oracle。

## 已被推翻的斷言

| 原斷言 | 現況 |
|---|---|
| Bcan 的 320×240 截圖是原生像素，兩邊可直接逐像素比 | 只有 320 模式成立；256 模式是最近鄰撐滿孔徑，要先 `--reference-unstretch 256` |
| 256 模式兩邊的差異全部落在 `x ≥ 256` | 只在畫面內容近乎平坦時成立；有細節的畫面，撐滿造成的差異遍佈整行 |
| 68000 走不到 Boom Zoo 標題畫面 | `$D06A` 之後的路徑已補上，標題可達 |
