# 里程碑 2 驗證：UM6618 繪圖 + SDL2 視窗 + 65C02 實跑

> 驗證日期：2026-08-31。規格出處：知識庫 `acan/docs/memory-map.md` §1/§3/§4/§6、
> `sound-driver.md`；渲染/IRQ 行為對照 MAME `src/mame/umc/supracan.cpp`（BSD-3-Clause，
> 重新實作未複製程式碼）。
>
> **版權聲明**：`docs/screenshots/` 內為商業遊戲（Funtech/熊貓 等）的執行畫面截圖，
> 僅供開發驗證用途，不作其他散布。

## 環境

- `build/superacan-emu`（Release），SDL2 2.30（系統 runtime + `third_party/sdl2-local`
  headers）、Moira `a4c273b`、CLK `096de57`
- BIOS：`/tmp/acan_bios/`（版權檔，不入庫）

## 驗證結果摘要

| 遊戲 | 幀數 | 結果 |
|---|---|---|
| Boom Zoo | 600 | 開頭劇情（怪獸+月亮+台詞字）正確渲染（tilemap+sprite） |
| Boom Zoo | 6000 | 標題「爆爆動物園」+ 金色球 sprite + 選單 + ©1996 FUNTECH（video_flags=$9ACC 與 MAME 註解一致）；**背景 tilemap1 有雜訊條（已知缺陷）** |
| Monopoly | 120 | A'Can 開機 logo（綠氣泡+A'can 字樣）正確 |
| Monopoly | 3600 | 標題畫面：花紋背景 + "START GAME / LOAD GAME" 選單（vblank/raster IRQ 驅動淡入） |
| Speedy Dragon | 1200（`ACAN_65NAIVE=1`） | 開頭飛龍場景（天/雲/草地/龍 sprite）正確渲染 |

指令範例：

```sh
./build/superacan-emu --bios /tmp/acan_bios \
    --rom "Boom Zoo (Taiwan).bin" --headless --frames 6000 --screenshot out.bmp
```

## 過程中修正的關鍵問題

1. **IRQ HOLD_LINE 語意**：Moira 的 `setIPL` 是 level-triggered；遊戲用
   `STOP #$2700` 等 vblank，若 IRQ 線受理後不解除會把 68k 鎖死在中斷進入循環
   （clock 凍結、PC 停在 handler 入口）。以 Moira `willInterrupt` delegate 模擬
   MAME `HOLD_LINE`（CPU ack 即解除）。知識庫可補記此為實機推論依據。
2. **IPL overlay 是單向 latch**：遊戲上傳音效驅動時會把 `$E9001C` 整個清 0
   （sound-driver.md §1.1）；若 overlay 隨 bit1/bit3 清除而恢復，卡帶中斷向量會被
   IPL 蓋回（IPL 向量全部指向 `rte`），遊戲主迴圈永遠等不到 vblank 任務。
   改為 bit 設置後永久關閉。**這是對 (a) 級 `$E9001C` bit1/bit3 語意的修正**，
   建議回填知識庫。
3. **65C02 reset 模式**：遊戲會多次 HALT→釋放副 CPU 重新上傳驅動。
   CLK 6502Mk2 的 Reset 是 level-triggered 且需給 cycle 才跑 reset 序列；現做法：
   首次釋放走 deferred PowerOn、再次釋放時手動補跑 7-cycle reset 序列。
   ※ Speedy Dragon 第二套音樂驅動（`$E800` 起，DMA word 模式上傳）會跑起來，
   但後續命令通道卡住（見 TODO）。
4. **65C02 必須實跑**：遊戲開機等 `$0300=$FF` boot ack，不跑副 CPU 會卡死。

## 已實作（對照 MAME 行為）

- UM6618 暫存器窗口 `$F00000-$F001FF`、palette（xBGR-555）、VRAM 128KB
- 3 tilemap 層：8/4/2bpp、5 種尺寸、scroll（12-bit 帶符號）、全層 flip、
  mosaic、linescroll、lineselect、優先度混色（priomap 高 nibble）
- sprite：表驅動 + direct sprite、X/Y flip、bank、mask/masked 模式、
  優先度（priomap 低 nibble）、9-bit 座標環繞、Y 尺寸表
- window 0（pen 填充、reverse clip、逐行表）
- sprite DMA（`$F00010-$1E`，含 VRAM 強制位址、0 填充模式）
- 主機 DMA ch0/ch1（byte/word/0xA800 填充/間接回捲）
- IRQ：vblank IRQ7（mask `$E90010` bit7）、raster IRQ4（bit4 每可視線脈衝）、
  line on/off IRQ5（`$F0000A/0C` 目標線）、65C02 mailbox IRQ6（脈衝）
- 65C02（3.579545 MHz = 68k/3）：I/O `$0400-$04FF`（手把 shift register、
  IRQ enable/來源、NMI ack、UM6619 位址/資料埠 stub）、`$E9000A`→IRQ bit5、
  vblank NMI 脈衝、取樣 DMA 完成 IRQ（bit6）固定 1ms 近似
- 手把 `$E80200/$E80202` direct-mode 讀取（無輸入裝置，恆無鍵）
- 時序：262 線/幀，256 模式 684 cycle/線、320 模式 728 cycle/線（U13/10、/8）
- SDL2 視窗（960×720、vsync）；`--headless`、`--frames N`、`--screenshot bmp`
- 除錯：`ACAN_DEBUG`（每幀 PC）、`ACAN_DMA`、`ACAN_WATCH`、`ACAN_TRACE65`、
  `ACAN_LAYERMASK`、`ACAN_DUMP=<prefix>`（vram/pal/regs/wram/sram65 dump）、
  68k clock 凍結偵測（[fault] + PC 軌跡，exit 4）

## TODO / 已知缺陷

- **ROZ 層未實作**（video flags bit2）：Bcan 自述也沒接完；A'Can logo 開機畫面
  在部分遊戲（Boom Zoo 開頭紅幕）依賴它。
- **Speedy Dragon 第二套音樂驅動**：上傳後能把 IRQ enable 切成 `$0C`
  （latch 通道），之後命令不再 ack，68k 停在 `$28DE` 等待迴圈，畫面停在黑屏。
  用 `ACAN_65NAIVE=1`（不重新 reset 65C02）可繞過並看到開頭場景，但音樂系統
  不完整。第二驅動的完整協定知識庫本來就標「待查證」。
- **Boom Zoo 標題背景 tilemap1 雜訊**：sprite 層/標題文字正確，背景層
  （flags=$6400、base=$F400）資料本身含大量重複 entry，輸出與獨立 Python
  重渲染一致——渲染器忠實呈現 VRAM 內容，疑為需要逐行/中途更新效果或
  尚有未明的 DMA 細節。
- **window 1**（`$1D8-$1DE`）未實作（MAME：尚無遊戲使用）。
- **1bpp tilemap / ROZ 專用 1bpp（addr-swap）**未實作（僅開機 logo 用）。
- **逐行 partial update**：目前整幀渲染，mid-frame 改 scroll/暫存器不會即時生效
  （sangofgt intro 需要）。
- **FRC 計時器 IRQ3**只有暫存器 stub。
- **UM6619 無聲**；timer IRQ（bit7，音樂 tempo）未實作。
- **手把輸入未接**（SDL 鍵盤/手把 → shift register / direct mode）。
- DMA 觸發條件 `$2648`（知識庫舊記述）實測未出現；Speedy 實際用 `$B800`
  （word 模式，bit15 觸發，已支援）。知識庫 sound-driver.md §1.2 的 control
  值應修正為 `$B800`。

## 授權標註狀況

- UM6618/DMA/IRQ 行為依 MAME `supracan.cpp`（BSD-3-Clause，Angelo Salese /
  Ryan Holtz 等）**重新實作**，未複製程式碼；標頭註明出處。
- SDL2（zlib）、Moira（MIT）、CLK（MIT）— README 第三方表已更新。
- `docs/screenshots/` 為遊戲版權畫面，僅開發驗證用（README 加註）。
