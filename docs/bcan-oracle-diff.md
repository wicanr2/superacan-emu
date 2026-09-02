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

要先還原的是幾何：**孔徑固定 320×240，實際顯示區比它小，Bcan 用最近鄰把兩個軸都
撐滿孔徑**。量法是數「完全相同的相鄰欄／列」——單色的列要先排除，否則平坦區域會
混進來：

| 軸 | 量到的複製樣式 | 推回的原生尺寸 |
|---|---|---|
| 水平（256 模式，Boom Zoo `video_flags=$9ACC`）| 每個 *x* ≡ 0 (mod 5) 與 *x*+1 相同 | 256 欄，`dst = floor(src × 320 / 256)` |
| 水平（320 模式，Sango `$03C8`）| 一對都沒有 | 320 欄，不縮放 |
| 垂直（兩款都是）| *y* = 30, 45, 60, …, 210 與 *y*+1 相同 | 224 列，`dst = floor(src × 240 / 224)` |

還原取 `src = ceil(dst × 孔徑 / 原生)`，也就是每一組重複裡丟掉多出來的那一份。
本專案的 framebuffer 是 320×240，內容落在**第 8 條起的 224 條**，所以完整的還原
規格是 `--reference-unstretch 256x224+0+8`（320 模式是 `320x224+0+8`）：
把原生畫面擺回本專案的座標系，並以那個矩形當比較範圍。

只還原一個軸不夠。Boom Zoo 標題只還原水平時差異 25.48%，兩軸都還原後降到 6.47%，
而那 6.47% 全部落在會旋轉的球上；`--reference-out` 可以輸出還原後的參考圖做並排。

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
    --reference-unstretch 256x224+0+8 \
    --candidate-dir out/boomzoo --top 3 --diff diff.png \
    --reference-out unstretched.png
```

`--reference-unstretch` 的幾何同時就是比較範圍，所以不必再加 `--width`；
320 模式的畫面用 `320x224+0+8`。`--reference-out` 會寫出還原後的參考圖做並排。

顯示模式會在同一次執行內變，所以逐張掃描時兩種幾何都跑、取較好的那個：

```sh
for geo in 256x224+0+8 320x224+0+8; do
    go run ./cmd/acan-imgdiff --reference snap/shot-07.png \
        --reference-unstretch $geo --candidate-dir out/<rom> --top 1
done
```

判讀時要扣掉單色畫面：全黑或全一色的畫面就算 0 差異也證明不了 renderer。

### 執行環境的實測限制

這些是實際踩過並確認的，不是推測：

- Xvfb 下沒有視窗管理員時 Wine 收不到 xdotool 的鍵盤事件，必須先啟動 openbox。
- Ctrl+O 在此環境無效；開檔要用滑鼠點「檔案(F)」→「開啟 ROM(O)...」。
- Bcan 沒有以 argv 載入 ROM 的路徑，只能走檔案對話框。
- 容器內以 Mesa 軟體 OpenGL 執行 wined3d，Bcan 的實際幀率遠低於 60 Hz，因此
  「截圖時刻」與硬體 frame 編號沒有固定換算；frame 對齊只能靠差分搜尋。

## 兩端可比與不可比的部分

比較範圍由 `--reference-unstretch` 的幾何決定，範圍外不列入統計：256 模式下
本專案右側 64 欄輸出為黑而 Bcan 撐滿，垂直方向本專案上下各有 8 條在 Bcan 的孔徑
之外。這兩處的硬體真相都是 `unknown`，不得依 Bcan 截圖改寫本專案 renderer。

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
27.83% 降到 25.48%（當時只還原水平軸；兩軸都還原後見下一節）。

第二款：Sango Fighter 開場旁白（320 模式，不需還原）。把畫面下緣 320×44 的文字帶
單獨比對，同一頁旁白的差異由 16.78% 降到 8.68%，其中**第 206–225 條（20 條掃描線
乘 320 欄）逐位元組相同**——那是已經打完字的一整行。剩下的差異只有兩處：最上面一行
被捲動切掉一半，最下面一行兩邊打字進度不同。

證據等級 `confirmed-Bcan`。兩款遊戲一款走 256 模式的 tilemap、一款走 320 模式，
結論一致。`chip/umc6618` 的 `TestTilePixelPackedModes` 釘住這個次序。

region 2（2bpp，一個位元組四個像素）沒有同級證據。目前維持低位優先，
列在 `WORKLIST` 待驗。

### 以發行的 AppImage 複驗

修正之後重打 AppImage（SHA-256 `29350d20…f16aae`），**用那個 AppImage 自己**產生
對拍畫面，得到與原始碼樹完全相同的結果：

| 檢查 | 結果 |
|---|---|
| Boom Zoo 1200 frame | `instructions=17369003`、`f720c9d1…b92301`，與 C10 基準相同 |
| Sango Fighter 1200 frame | `instructions=11634924`、`f5bfffa1…4f9f06`，與 C10 基準相同 |
| AppImage 與 headless 的 `--screenshot` PNG | 兩款都逐位元組相同 |

## 第三輪結果：整張標題與 oracle 完全相同

幾何還原正確之後，Boom Zoo 標題不再有任何差異：

| 對照 | 範圍 | 差異 |
|---|---|---|
| Bcan `bz-10` vs 本專案 frame 2336 | 256×224 全畫面（57,344 像素）| **0** |
| Bcan `sango-06` 的旁白文字帶 vs 本專案 frame 2104 | 320×40（12,800 像素）| **0** |

Boom Zoo 那一張是整個顯示區逐像素相同，包含 logo、中文副標、選單文字、版權行與
舷窗。要對上的只有球的旋轉相位：拿同一段執行的其他 frame 去搜，第 2336、2340、
2456、2460、2576 個 frame 都是 0 差異，其他 frame 的差異全部集中在球上。

並排圖：`docs/screenshots/appimage/boomzoo-title-bcan-vs-appimage.png`，左為 Bcan 的
原始截圖（320×240，兩軸都被撐開），右為本專案的 frame 2336（320×240，內容是
256×224 擺在第 8 條起）。看起來的尺寸差就是孔徑縮放；還原之後兩張逐像素相同。

## 第四輪：八款卡帶的逐張對拍

幾何還原正確之後，把 `Bcan008b/ROMS` 下八款卡帶各跑一次 oracle（不送輸入，
12–14 張、間隔 4–5 秒），每一張都拿本專案同一組輸入的候選幀去搜最接近的一張。
兩種孔徑幾何（`256x224+0+8`、`320x224+0+8`）都試，取較好的那個——**顯示模式會在
同一次執行內變**，例如 Speedy Dragon 十二張裡有一張是 256 模式、其餘是 320。

| 卡帶 | oracle 張數 | 0 差異 | 其中是有內容的畫面 | 對不上的那些是什麼 |
|---|---:|---:|---:|---|
| Boom Zoo | 14 | 11 | 9 | 1 張動畫相位；2 張是 sprite 越過 letterbox（見下） |
| Formosa Duel | 12 | 5 | 4 | 選角輪播停在不同人物 |
| Speedy Dragon | 12 | 5 | 4 | 遊戲中動畫相位 |
| Super Taiwanese Baseball League | 12 | 3 | 2 | Bcan 已走到本專案這一輪沒走到的畫面 |
| The Son of Evil | 12 | 2 | 0 | 動畫相位；兩張 0 都是單色畫面 |
| Journey to the Laugh | 12 | 1 | 0 | attract 是實際遊玩，沒有靜止畫面 |
| Sango Fighter | 12 | 0 | 0 | 開場淡入相位；旁白文字帶 320×40 是 0 差異 |
| Monopoly | 10 | 0 | 0 | 標題背景持續斜向捲動 |
| **合計** | **96** | **27** | **19** | |

「有內容的畫面」是排除單色畫面之後的數字：全黑或全一色的畫面就算 0 差異也證明
不了 renderer，統計時要扣掉。留下的 19 張都是實際畫面，例如：

- Boom Zoo 標題（36 色、整屏 76,800 像素非黑）
- Formosa Duel 標題「福爾摩沙對決」（44 色）與其選角框架
- Speedy Dragon 標題「音速飛龍」（26–28 色，人物、山景與版權行）
- Super Taiwanese Baseball League 主選單「中華職棒聯盟」（44 色）

這四款各自走不同的圖層組合與顯示模式，都是整個顯示區逐像素相同。

## 唯一定位到的真差異：sprite 沒有被 letterbox 切掉

Boom Zoo 開場的兩張（`bz-02`、`bz-03`）差 88 與 352 個像素，全部落在第 152–159 條。
開場是上下加黑條的信箱式構圖，黑條上緣在兩邊都是第 152 條；差的是**越過黑條的
sprite**：Bcan 把它切齊黑條，本專案讓它畫在黑條上。

這不是相位。第 0–2400 幀逐幀搜過，只比第 144–167 條那一帶，最好的一幀仍差 88 個
像素；本專案自己的黑條上緣在多數幀都落在第 151 條，與 Bcan 一致，超出去的只有
sprite。

試過並排除的假設：window 的優先度比較改成同級也蓋掉 sprite（`>=` 改 `>`）——
差異一個像素都沒變，代表該 sprite 的優先度嚴格高於 window 0（`$F001D0 = $29AF`，
優先度 1），不是同級平手的問題。真正的機制還沒定位。

## 仍未解釋的差異

- **顯示區為什麼是 224 條、又為什麼從第 8 條起**：Bcan 的孔徑只涵蓋本專案第 8–231
  條，上下各 8 條在 oracle 這一側看不到。這是垂直方向的「右側 64 欄」問題——同樣是
  `unknown`，不得依 oracle 的取景改寫 renderer。要定案需要實機或掃描線層級的證據。
- **Sango Fighter 開場全畫面**：旁白文字帶已經是 0 差異，但整張差 73%，差異集中在
  天空的淡入配色。第 2000–2200 個 frame 逐幀搜過，沒有一個 frame 的淡入階段對得上
  Bcan 的取樣時刻（Bcan 在容器內遠低於 60 Hz）。這是相位不是缺陷，但也因此無法用
  這個畫面定案整張。
- **Super Taiwanese Baseball League 的後段畫面**：oracle 第 4 張之後 Bcan 已經走到
  本專案這一輪（4000 frame、無輸入）沒有走到的畫面，差 50–88%。要先確認兩邊在同一
  個流程位置才有比較意義，不能直接當 renderer 差異。
- **Monopoly 的標題背景持續斜向捲動**，沒有任何一個 frame 對得上取樣時刻，
  這款不適合當逐像素 oracle。

## 已被推翻的斷言

| 原斷言 | 現況 |
|---|---|
| Bcan 的 320×240 截圖是原生像素，兩邊可直接逐像素比 | 只有 320 模式成立；256 模式是最近鄰撐滿孔徑，要先 `--reference-unstretch 256` |
| 256 模式兩邊的差異全部落在 `x ≥ 256` | 只在畫面內容近乎平坦時成立；有細節的畫面，撐滿造成的差異遍佈整行 |
| 只還原水平就可以比 | 垂直也被撐開（224 → 240），只還原一個軸會留下隨畫面高度變化的假偏移 |
| Boom Zoo 標題各圖層的垂直落點不同，原因不明 | 那是垂直撐滿造成的假象；兩軸都還原後整張 0 差異 |
| 68000 走不到 Boom Zoo 標題畫面 | `$D06A` 之後的路徑已補上，標題可達 |
