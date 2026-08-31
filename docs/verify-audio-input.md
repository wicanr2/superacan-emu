# 里程碑 3+4 驗證：UM6619 音效 + 手把輸入（含 Speedy Dragon 第二驅動修復）

> 驗證日期：2026-08-31。規格出處：知識庫 `acan/docs/sound-driver.md`（§3-§5 (a)）、
> `memory-map.md` §5/§7；UM6619 合成模型依 MAME `src/mame/umc/umc6619_sound.cpp`
> （BSD-3-Clause，(c) Ryan Holtz / superctr）的行為描述**重新實作**，未複製程式碼。
>
> **版權聲明**：`docs/screenshots/` 內為商業遊戲執行畫面截圖，僅供開發驗證用途。

## 環境

- `build/superacan-emu`（Release），SDL2 2.30（系統 runtime）、Moira `a4c273b`、
  CLK `096de57`；BIOS 在 `/tmp/acan_bios/`（版權檔，不入庫）
- WAV 分析：Python + numpy（RMS / 峰值 / 每段 rFFT 主峰）

## 已實作

### 音效（`src/audio/um6619.{hpp,cpp}`）

- 16 通道 PCM 取樣合成：period（reg `$20/$30`，addr_increment = period<<6，
  16.16 固定小數點）、音量（reg `$E0-$EF`，高/低 nibble 左右聲道 ×17）、
  key on/off（reg `$17`）、長度/one-shot（reg `$50-$5F`：0x40<<n）、
  取樣起始位址（reg `$60/$70`，單位 0x40 bytes）、DMA 雙緩衝旗標
  （reg `$90-$9F`：播完觸發 65C02 IRQ bit6 並自動重新 key-on）
- 內建 timer：reg `$11/$12` 設 period（= 10×(0x10000−n) clocks），reg `$14`
  bit7 啟動、bit6 致能到期觸發 65C02 IRQ bit7（音樂 tempo；驅動初始值
  `$11=$02/$12=$F9` → 約 200 Hz）；讀 reg `$14`/`$16` 分別 ack timer/DMA IRQ
- 取樣資料 = 共享 sound RAM（8-bit 無號 → ±16-bit）；原生抽樣率
  = 3.579545 MHz / 80 = **44744.3125 Hz**，線性插值重取樣到 48000 Hz
- 混音 clamp 前 >>1 留 headroom（16 通道滿幅會超過 int16）
- 輸出：SDL2 audio（48000 Hz S16 stereo，callback 從佇列消費，underflow 補
  靜音；佇列爆滿則丟棄防延遲累積）；headless 用 `--wav <file>` 全程錄 WAV
- 兩塊 BIOS 取樣（`internal_6502_1/2.bin` → sound RAM `$0000-$3FFF`）載入
  路徑沿用里程碑 1 既有實作，開機 jingle（約 788 Hz 主峰，BIOS 取樣
  `$2E00+`）三款遊戲皆在 0-3 秒出現，確認路徑正確

### 手把輸入

- 位元序依知識庫 `memory-map.md` §7（16-bit active low：bit15=A、bit14=B、
  bit13=Start、bit12=Select、bit11-8=方向、bit7=X、bit6=Y、bit5=L、bit4=R；
  與 MAME INPUT_PORTS 完全一致）
- 兩條路徑都通：65C02 shift register 掃描（`$0407` 控制/`$0402/03` 讀出 →
  驅動存 sound RAM `$0200/$0202`）與 68k direct mode（`$E80200/$E80202`，
  回 `pad ^ 0xFFFF`）
- SDL 鍵盤：方向鍵 + Z/X/A/S/Q/W = A/B/X/Y/L/R（Bcan.ini 預設風格）、
  Enter=Start、右 Shift=Select；單人（P1）
- headless 注入：`--press frame:BTN+BTN,...`（到幀按下、10 幀後放開）

### Speedy Dragon 第二套音樂驅動：**已修復**

里程碑 2 的已知缺陷（上傳後 IRQ enable 卡 `$0C`、命令不 ack、68k 停等
黑屏）根因有三層，全部修好：

1. **65C02 reset 時序**（`cpu65c02.hpp::setReset`）：CLK 6502Mk2 的 Reset 是
   level-triggered 且只在給 cycle 時捕捉。里程碑 2 的手動 7-cycle pump 會在
   CPU 停在中途指令時把 reset 序列截斷（之後雖靠 restore point 跑完，但
   Speedy 第二驅動的 init probe 因此異常）；而純 level 模式（HALT 期間不給
   cycle）會讓 reset 請求「設了又清」整個消失（Boom Zoo 音樂上傳後無聲）。
   修正：釋放時**繼續拉住 Reset 線**，讓 CPU 跑完當前指令並進入 reset 序列
   （以讀 `$FFFC` 向量為準）後才放開，64 cycle 上限保底。reset 時一併清
   65C02 I/O 的 IRQ enable/source（真實硬體行為）。
2. **65C02 IRQ 來源是 level-held、各有專屬 ack**（不是 MAME 的「`$0411` 讀取
   即清全部」）：bit2←讀 `$0405`、bit3←讀 `$0404`、bit4←讀 `$0409`、
   bit5←讀 `$040A`、bit6←讀 UM6619 reg `$16`、bit7←讀 reg `$14`。
   證據 (a)：兩套驅動的 IRQ dispatcher 都只分派**一個**來源就 rti，依賴
   level 重觸發補跑其餘來源；各 handler 都讀自己的暫存器 ack
   （第一驅動 `$F595` 讀 `$040A`、`$F64A` 讀 reg `$16`、`$F620` 讀 reg `$14`；
   第二驅動 `$EE96` 讀 `$040A`）。「讀取即清」會丟掉同時發生的來源
   （實測：probe 期間 latch bit2/bit3 同時置位，清全部會丟一個 → probe
   逾時 → boot 太慢 → 68k 逾時重試迴圈）。
3. **latch `$0404/$0405` 空值 `$CD` + probe 觸發**：latch 讀取在無資料時回
   `$CD`（MAME 註解的 magic value）；68k 經 `$E80404/$E80405` 窗口寫入即
   置位並觸發對應 IRQ。開機 probe（寫 `$0406=3`、`$0407=$30` 清除脈衝）
   會觸發 latch IRQ 讓 handler 讀到 `$CD` 而即時結束等待——**此觸發條件為
   功能推測**（讓 probe 走快速路徑；真實硬體必須有等效機制，否則 68k 端
   0.2 秒逾時來不及）。latch 3-byte 封包語意仍待查證（知識庫 §4.3）。

第二驅動結構（ROM `$3D6A` 起 0x1800 bytes → 65C02 `$E800-$FFFF`，
Capstone 65C02 反組譯 (a)）：

- 向量：NMI=`$ED61`（→`jmp ($FD50)`=`$ED64`，做手把掃描 `$F90C`）、
  RESET=`$E802`、IRQ=`$ED75`
- IRQ 分派：讀 `$0411` 後依位移到間接向量表 `$FD52`：
  bit2→`$EDB3`（latch `$0405`）、bit3→`$EDFC`（latch `$0404`）、
  bit4→`$EE90`（讀 `$0408`）、bit5→`$EE96`（**命令分派**：串流指標
  `$0300`，命令表 `$FE1C`，`<$70` 查表、`≥$70` 走 `jmp ($0008)`）、
  bit6→`$EF38`（取樣 DMA）、bit7→`$EF02`（timer）
- init `$F093`：清 zp/`$0200` 頁 → 16 通道 key-off → UM6619 init
  （`$07=$80,$13=$90,$14=$CF`）→ probe `$F0D3`（enable=`$0C`、寫
  `$0406=3`/`$0407=$30`、等 `$023D==0`）→ boot ack `$F083`（`$0300=$FF`，
  若 `$FE76`≠0 寫 `~$FE76` 到 `$040A` 通知 68k）
- 上傳常式是 **`$34E4`**（68k 端）：DMA control `$B800`、src `$3D6A`、
  dest `$E8E800`、count `$BFF` words；釋放後**立即** `jsr $28DE` 送空命令
  等 ack（逾時 `$C000` 次 → `$34C6` 整組重試）。靜態碼 `$954`（control
  `$2648`）無任何呼叫者，是未使用的舊碼

## 驗證結果

| 遊戲 | 項目 | 結果 |
|---|---|---|
| Boom Zoo | 3000 幀標題 + WAV 50.1s | 音樂播放中：分段 RMS 260→4681 隨段落變化、peak 16193、clip 0%（見下表） |
| Boom Zoo | `--press 2500:START,2600:START,2700:A` | 標題 → **角色選擇**畫面（像素差異 77.9%） |
| Monopoly | 3600 幀標題 + WAV 63.6s | 音樂全程：分段 RMS 2593-6378、clip 0% |
| Monopoly | `--press 3700:START` | 標題（START GAME/LOAD GAME）→ **玩家人數/角色選擇**（96.4% 差異） |
| Speedy Dragon | 1200 幀（**預設模式**，不再需 ACAN_65NAIVE） | 開頭飛龍場景 + 音樂（9s 起 441/661/498/659 Hz 旋律段落）、68k PC 正常推進、命令 ack 正常（enable=$FE） |
| Boom Zoo | SDL dummy driver 煙測 | 視窗（software renderer fallback）+ 音訊裝置開啟成功（48000 Hz stereo） |

截圖：`docs/screenshots/`（`*-m3.png` 標題、`*-after-start.png` 按鍵後、
`speedydragon-intro-f1200.png`）。

WAV 分段 RMS（每 6 秒取 3 秒窗，左聲道）：

```
Boom Zoo  50.1s rms=2972 peak=16193 clip=0.00%
  0s:2335 6s:1046 12s:331 18s:260 24s:619 30s:4681 36s:4537 42s:1538
Monopoly  63.6s rms=5272 peak=16193 clip=0.00%
  0s:2335 6s:2593 12s:5801 18s:5262 24s:5926 30s:6193 36s:4708 ...
Speedy    21.1s rms=4090 peak=16065 clip=0.00%
  0s:2334(開機 jingle) 3-6s 靜音 9s:3748(441Hz) 12s:5747(661Hz) 15s:5435 18s:5864
```

三款遊戲 0-3 秒皆有相同的 788 Hz 開機 jingle（BIOS 內建取樣
`$2E00-$2EFF`，bios-65c02.md (b) 註解一致），之後進入各自的遊戲音樂——
非靜音、非爆音、頻譜為合理樂音結構。

## 指令範例

```sh
# 音效驗證（headless + WAV）
./build/superacan-emu --bios /tmp/acan_bios --rom "Boom Zoo (Taiwan).bin" \
    --headless --frames 3000 --wav out.wav --screenshot out.bmp

# 按鍵注入驗證
./build/superacan-emu --bios /tmp/acan_bios --rom "Monopoly - Adventure in Africa (Taiwan).bin" \
    --headless --frames 4300 --press 3700:START --screenshot after.bmp
```

## 過程中修正的關鍵問題（對知識庫的回填點）

1. 65C02 IRQ 來源模型：level-held + 專屬 ack，`$0411` 純狀態（上文 §Speedy
   修復 2）——修正 memory-map.md §5「讀取即清除」與 MAME 行為的記述。
2. latch `$0404/$0405`：空=`$CD`、68k 寫入置位觸發 IRQ、讀取 ack；
   probe 清除脈衝觸發 latch IRQ（推測）。
3. Speedy 第二驅動上傳常式為 `$34E4`（非 `$954`）；命令協定與第一驅動
   同通道（`$0300` + IRQ bit5），命令表在 `$FE1C`。
4. 65C02 reset 的正確模擬方式（拉住直到向量讀取），CLK level-triggered
   語意下「設了又清」會整個丟失 reset。

## TODO / 已知缺陷

- latch 3-byte 封包語意（知識庫 §4.3）仍待查證；probe 的 latch IRQ 觸發
  條件為功能推測。
- UM6619 envelope regs（`$A0-$D0`）未實作（MAME 同）；reg `$07/$11/$12/$13`
  細節、`$16` bit6 busy 以外的位元待查證。
- 混音 >>1 headroom 為實用取捨；真實硬體的混音增益/削波特性待查證。
- ROZ、window 1、逐行 partial update、FRC IRQ3 等里程碑 2 遺留項目維持不變。
- Boom Zoo 標題背景 tilemap1 雜訊（里程碑 2 已記）維持未解。
- 雙人輸入（P2）未接（`--press` 與鍵盤都只餵 P1）。
