# 介面字型與語言涵蓋

更新日期：2026-09-01

`docs/ui-design.md` 的 G2 閘門要求在動介面語言之前，先把字型的涵蓋範圍與散布授權
查清楚。以下是實測與授權原文查核的結果。

## `golang.org/x/image/font/basicfont` 的實際涵蓋

`basicfont.Face7x13` 的 `Ranges` 只有兩段：`U+0020` 到 `U+007F`（可列印 ASCII 加
DEL），以及 `U+FFFD` 這個替換字元。

**沒有 Latin-1 補充區。** 這確認了一件事：法文與西班牙文的重音字元（e-acute、
n-tilde、u-diaeresis 等）與中文擋在同一個閘門後面，不是「中文另案、其他四種先做」。
第一版只有英文。

字面尺寸 7x13，字型資料 49 KB，授權為 Go 專案的 BSD-3-Clause，散布無疑義。

## CJK 候選：`github.com/hajimehoshi/bitmapfont/v4`

| 項目 | 值 |
|---|---|
| 版本 | v4.1.0 |
| 字型資料大小 | 2.5 MB |
| 字面尺寸 | 半形 6x13、全形 12x13 |
| 程式碼授權 | Apache-2.0 |

字型資料是多份來源的混合，各自的授權如下（取自該套件 `README.md` 與 `LICENSE`）：

| 來源 | 授權 | 散布條件 |
|---|---|---|
| Baekmuk Gulim | Baekmuk License | 可自由使用、複製、修改、散布，但**必須在支援文件中標示** Baekmuk 為 Kim Jeong-Hwan 的註冊商標 |
| Cubic 11 | OFL-1.1 | 可隨軟體散布，需附授權，不得單獨販售 |
| Galmuri | OFL-1.1 | 同上 |
| misc-fixed | Public Domain | 無條件 |
| M+ Bitmap Font | M+ Bitmap Fonts License | 可自由散布 |
| 阿拉伯字符（Eternal Dream Arabization）| OFL-1.1 | 同上 |

六份來源都允許隨軟體散布，沒有一份禁止商業使用或要求原始碼開放。**條件是發行包
必須完整附上這六份授權文字與 Baekmuk 的商標標示**，這一項要進第三方授權清單
（`docs/ui-design.md` 的 S8 關於畫面已經規劃了這個清單）。

半形 6x13 與全形 12x13 的尺寸與 `ui` 在原生表面解析度繪製的決定相容；若沿用
320x240 直接繪製，12 px 高的漢字會失去可讀性，這也是設計選擇在表面解析度繪製的
原因之一。

## 另一條路：讀系統字型

| 平台 | 是否保證有 CJK 字型 |
|---|---|
| Android | 有，系統內建 Noto Sans CJK |
| macOS | 有，系統內建蘋方（PingFang）|
| Linux 桌面 | **不保證**。最小安裝可能一套中文字型都沒有 |

讀系統字型可以省下 2.5 MB，但在 Linux 上會出現「同一版程式在某些機器上中文變成
豆腐」的情況，而且要處理三個平台各自的字型探索路徑（fontconfig、Core Text、
Android 的 `/system/fonts`）。以純 Go 解析 TrueType 需要
`golang.org/x/image/font/sfnt`，它已在模組快取內，但這條路的變數比嵌入位元圖字型多。

## 目前結論

- **英文介面沒有前置條件**，可以立刻做。
- **其餘四種語言（含法文與西班牙文）都擋在字型決策後面**，不是只有中文。
- **嵌入 `bitmapfont/v4` 在授權上可行**，代價是發行包多 2.5 MB 與六份授權文字。
- 讀系統字型省空間但在 Linux 上不可靠。

選哪一條是使用者的決定；上面是做決定所需的事實。在拍板之前，介面字串一律限定
ASCII，不預先寫入其他語言的字串表。
