# 里程碑 5 驗證：收尾 TODO（DMA 觸發修正、ROZ、save state、P2、FRC 等）

> 驗證日期：2026-08-31。對照參考：MAME 0.264（Ubuntu 套件解出，headless + Lua
> 腳本 dump 對照；BSD-3-Clause）。Boom Zoo 標題的 MAME 對照截圖見本文分析。
>
> **版權聲明**：`docs/screenshots/` 內為商業遊戲執行畫面截圖，僅供開發驗證用途。

## 1. Boom Zoo 標題背景雜訊 — **已修復（root cause 確定）**

**根因**：`SystemBus::write16` 把 16-bit 寫入拆成兩次 `write8`，經 video 暫存器
的 byte RMW 路徑各觸發一次 `UM6618::writeReg`。對觸發型暫存器
（sprite DMA `$F0001E`）等於**每次 word 寫入觸發兩次 DMA**：第一次用正確的
src/dst 複製，第二次用第一次已推進的 src/dst 再複製一遍。遊戲的 tilemap 上傳
常式 `$25B0` 因此每次實際複製兩份，第二份覆寫到相鄰區域（tilemap1 的
`$F5E800` 區被 tilemap0 的第二份半 chunk 蓋掉）→ 標題背景雜訊。

**證據鏈**：
- MAME 對照（0.264，Lua dump）：標題選單相位（video_flags=$9ACC 連續 60 幀）
  的 tilemap1 前 32 word 全為 `$3000`（乾淨）；我們修正前為雜訊 entry。
- 兩邊 video 暫存器、調色盤、WRAM staging buffer（`$FC0800+`）逐 byte 比對
  一致；VRAM 僅 tilemap 上傳區不同。
- sprite DMA 觸發 log：同一 caller 連續兩次觸發、第二次 dst/src 各 +0x780
  （= 第一次的位元組數）→ 雙觸發實錘。
- 修正（bus.cpp：$F00000-$F003FF 的 write16 不再拆分，直接單次
  `writeReg`/`writePalette`）後，Boom Zoo 標題渲染為：大 BOO ZOO logo +
  爆爆動物園 + 金球 + 色輪 + 選單，無雜訊，結構與 MAME 一致且色彩正確
  （MAME 0.264 的 Boom Zoo 調色/圖層本身有問題，畫面偏灰階，僅作結構對照）。

**同修正的附带改善**：Monopoly 標題現在顯示大「非洲探險」logo tilemap
（之前只有花紋底 + 選單字）。

## 2. 逐行 partial update — **維持整幀渲染，文件化理由**

- 里程碑 1 的雜訊並非缺逐行更新，而是雙觸發 bug（§1）。
- Sango Fighter 開頭（f2400）：紅天 + 文本 + 武將圖可讀，與里程碑 2 的
  「sangofgt intro 需要 mid-frame 更新」對照——目前整幀渲染下無明顯破圖。
  與 MAME 0.264 同點對照見 §6。若日後發現具體畫面錯誤再實作逐行渲染
  （成本：sprite 合成需改逐行評估，暫緩）。

## 3. ROZ 層 — **已實作（affine + 逐行表模式）**

- 依 MAME `draw_roz_layer`/`get_roz_tilemap_info` (b) 重新實作：
  `$180` mode（優先度 bit15-13、尺寸 bit11-8、wrap bit5、flip bit0/1、
  bit9=逐行表模式關）、`$184-$18A` 32-bit scroll（24.8 固定小數點）、
  `$18C-$192` 係數 A/B/C/D（8.8）、`$194` base、`$196` tile bank、
  `$198/$19A/$19E` 逐行參數表（incxx/scrollx/scrolly）。
- tile region 依 `roz_mode & 3` → {1bpp-alt, 2bpp, 4bpp, 8bpp}。
- 逐行表模式（`!(mode & $0200) && (mode & $F000)`，MAME 的 HACK 分支）：
  每行 `incxx = coeffA + 表0[y]`（表值 0 → 該行不畫）、scroll 加表1/表2 的
  32-bit 值。
- 1bpp-alt（mode 0，A'Can 開機 logo）實作：位址重排 VRAM 副本
  （低 7 bit `[b2b1b0b6b5b4b3]`，MAME `write_swapped_byte` (b)）+ 固定
  tile 解碼（`tile = 0x880 + (count&7)*2` 等），32x32、palette 0/1。
- 驗證：Boom Zoo 標題的色輪（ROZ 層，ACAN_LAYERMASK 隔離確認）；Monopoly
  開機 A'Can logo（1bpp-alt 路徑）正常；Boom Zoo/Speedy 開頭過場無回歸。

## 4. window 1 — **保守實作**

`$1D8-$1DE` 對稱實作（control/start addr/scrollx/scrolly），僅在 control 非 0
時啟用。MAME 未接（「尚無遊戲使用」）；無遊戲可驗證 → **行為待查證**，
文件化為推測實作。

## 5. FRC IRQ3 — **已實作（MAME case 表）**

`$E90014` control / `$E90016` frequency；`(control & $FF00) == $A200` 時依
`control & $F` 排程：`case 0` → 1 Hz（MAME HACK）、`case 1` → 1024×period
68k cycles、`case $F` → 8192×period；觸發 68k IRQ3（HOLD_LINE）。MAME 的
實作本身就是 case-by-case HACK（註解明寫公式未解），真實 FRC 公式
**待查證**；本次三款驗證遊戲不設 FRC（FRC3=0），僅語法/回歸驗證。

## 6. UM6619 envelope（$A0-$D0）與混音增益 — **維持 stub，文件化**

- MAME `umc6619_sound.cpp` 對 envelope regs 同樣只存不算（註解 "Envelope
  Parameters? (not yet known)"），無可對照的演算法 → 維持存取 stub，
  **待查證**（需硬體或更完整驅動分析佐證）。
- 混音增益：里程碑 3 已加 >>1 headroom（clip 0%）；本次回歸三款遊戲 WAV
  RMS 與前次一致（Boom Zoo 2972、Monopoly 5272；Speedy 787→4741 是因為
  第二驅動修復後音樂實際播放了）。

## 7. P2 輸入 — **已實作**

- SDL 鍵盤第二組：I/J/K/L 方向、U/O/N/M = A/B/X/Y、逗號/句號 = L/R、
  右 Ctrl=Start、左 Shift=Select。
- headless `--press2 <spec>`（語法同 `--press`）。
- 驗證：Boom Zoo `--press2` 注入後 65C02 shift register P2 路徑
  （`$0402`）與 `$E80202` direct mode 皆反映（經 ACAN_DUMP sram65/wram
  與既有 P1 路徑相同機制；P2 實機雙人畫面未深入驗證，待查證）。

## 8. Save state — **已實作**

- 自訂格式（`src/state.hpp` 標頭註明）：magic `ACANEST1`、96-byte 標頭
  （version/headerSize/ROM FNV-1a 64 hash/payload 大小）+ payload。
  參考知識庫 ACANRTS 的標頭設計但**不相容 Bcan**（Bcan 的 payload 欄位
  版面本來就待查證，相容無低成本路徑）。
- 序列化範圍：Work RAM / sound RAM / 卡帶 SRAM / UM6618（含 VRAM、
  palette、全部圖層暫存器、IRQ 狀態；framebuffer 等衍生狀態除外）、
  UM6619（暫存器+16 通道+timer）、65C02（CLK 全內部狀態，含 resume
  point——子類逐 POD 成員複製）、68k（Moira 全內部狀態，protected 成員）、
  UMC6650、主機 DMA、控制/overlay latch、runner 時序狀態。
- 操作：CLI `--save-state <f>`（結束時寫出）、`--load-state <f>`（啟動載入，
  跳過 IPL；`--frames N` 在載入後表示「再多跑 N 幀」）；SDL 熱鍵 F5 存 /
  F7 讀 / F6 切槽 0-9（與 Bcan 預設熱鍵一致，檔名 `<rom>.st<N>`）。
- **驗證**：Boom Zoo 連續跑 3060 幀 vs 3000 幀存檔→新行程載入→再跑 60 幀，
  截圖逐 byte 比對 **0.00% 差異**（完全確定性還原）。

## 9. latch 3-byte 封包語意 — **結構釐清，語意維持待查證**

第二驅動反組譯（Speedy，Capstone 65C02）：
- 封包解碼 `$EE47`：byte0 bit0-1（符號擴充）<<8 + byte1 → 累加進
  `$76/$77,y`；byte0 bit2-3 <<8 + byte2 → 累加進 `$7A/$7B,y`；
  byte0 bit4-5 → `$5E,y`；y=0/2 對應 `$0404`/`$0405` 兩路。
- 消費者：命令表 `$FE1C` 的 cmd `$20`（`$FBA0`：初始化/清零該路累加器）與
  cmd `$21`（`$FBBF`：取 `$5E,y`、把累加值經 `$EFAF`（有號 16-bit 除法）
  處理後經 `$FC53` 推入 `$0340` 佇列——65C02→68k 的回應佇列）。
- 形狀像「68k 經 latch 送參數 → 65C02 運算（除法）→ 結果回 68k」的
  協處理器通道，但確切用途（音高計算？亂數？）**待查證**。

## 10. 其他驗證

- 三款遊戲完整回歸（畫面 + 音樂 WAV + 按鍵）通過；Speedy Dragon 音樂
  現在實際播放（RMS 4741，第二驅動工作）。
- ACAN_LAYERMASK 新增 bit4 = ROZ 層隔離。

## TODO / 已知缺陷（更新後）

- 逐行 partial update：暫緩（§2）。
- FRC 真實計時公式、UM6619 envelope 語意、latch 封包用途：待查證。
- window 1 行為為推測實作（無遊戲可驗證）。
- ROZ 逐行表模式僅 Boom Zoo 用過簡單路徑驗證；Speedy intro/bonus 的
  複雜逐行模式未逐一對照。
- P2 實機雙人流程未深入驗證。
