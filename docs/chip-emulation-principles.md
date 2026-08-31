# 晶片模擬通則

狀態：已採用

更新日期：2026-08-31

適用範圍：Super A'Can 純 Go 模擬核心，包括 Motorola 68000、WDC 65C02、
UMC6650、UM6618、UM6619、主機 DMA、輸入介面與整機 scheduler。

## 1. 目標與停止線

模擬器要重建晶片對軟體可觀察的數位契約：指令結果、bus transaction、暫存器副作用、
中斷、DMA、時間順序及輸出資料。遊戲能執行是驗證契約的方法，不得以 ROM 專屬特判
取代晶片行為。

第一階段不模擬封裝內電路、類比波形、訊號完整性或每根 pin 的電氣特性。只有當玩家
軟體的可重現差異能證明現有數位 phase 不足時，才提高精度；不能因「可能更準」無限
擴大範圍。

## 2. 證據契約

每項晶片行為必須標示下列其中一級，並附來源版本與定位：

| 等級 | 意義 |
|---|---|
| `confirmed-hardware` | 實機量測或正式晶片／介面規格直接證實 |
| `confirmed-Bcan` | 固定雜湊 Bcan 的反組譯與動態行為證實 |
| `MAME-derived` | 固定 MAME commit 的 device／driver 行為；不是自動等於實機 |
| `software-observed` | 固定 ROM／BIOS 的 producer 與 consumer 行為證實 |
| `strong-inference` | 多條證據一致，但仍缺直接硬體證明 |
| `hypothesis` | 可測試的暫時解釋，不得寫成正式相容性契約 |
| `unknown` | 尚無足夠證據 |

Moira `a4c273b08e07d82c73289ac032867c845969a0f2` 只作設計 sample 與差分 oracle。
新的 Go 68000 核心是獨立實作，不翻譯 Moira 的 handler、table generator 或類別結構；
測試與文件可以記錄「與 Moira 相同／不同」，但不能把一致性冒稱為硬體證明。

## 3. 裝置邊界

- 每顆晶片是一個有明確輸入、輸出、clock domain、暫存器與可序列化狀態的 device。
- CPU 只能經 bus interface 存取外部世界；前端、測試和遊戲 adapter 不得直接改 CPU
  register 或晶片私有狀態來通過相容性測試。
- bus 負責 address decode、資料寬度、端序、遮罩、mirror、bank、overlay、open bus、
  wait state 與 fault；device 負責其 register 的讀寫副作用。
- ROM、RAM、memory-mapped device 與動態 view 分開建模。word transaction 不能任意
  拆成兩個 byte transaction；若實體 bus 本來會拆分，拆分位置與順序必須由 CPU／bus
  契約明確產生。
- device API 不暴露主機時間。所有硬體事件只讀取單調遞增的模擬時間。

## 4. 整機時間與 phase scheduler

整機使用單一單調時間軸；每個 clock domain 以整數比例或保留餘數的 rational accumulator
推進，不使用浮點秒數累積。SDL／Ebitengine 的 frame callback、audio callback 與主機
wall-clock 只負責節流或輸出，不能決定晶片事件先後。

CPU 對外提供一次執行完整指令的 `Step`，但指令內部必須產生可觀察 phase：

```text
instruction fetch → extension/prefetch → operand read/write
                  → internal cycles → IRQ poll/exception boundary
```

每個 phase 至少攜帶：開始 cycle、耗用 cycle、address space、位址、資料寬度、讀／寫、
function code（適用時）與 fault 結果。scheduler 在 transaction 發生前先把其他 device
推進到同一時間點，再執行讀寫副作用；不能在整條指令結束後才一次補總 cycle。

這個模型刻意介於兩端：

- 比純 instruction-total timing 精確，能處理 DMA、IRQ、雙 CPU 與觸發型 register；
- 不承諾逐 pin、逐半週期或類比波形；尚未證明需要時不實作。

## 5. CPU 通則

### 指令解碼與狀態

- opcode table 可以生成，但生成來源、覆蓋率與 illegal encoding 必須可稽核；未知 opcode
  不得當 NOP。
- register、status flag、prefetch queue、STOP／HALT、trace、pending exception、IPL
  sample 與 cycle counter 都是正式狀態，不得藏在不可序列化 closure。
- byte、word、long 的符號延伸、截斷、旗標與 address-register 特例分開測試。
- 68000 的 long access 是有順序的多個 16-bit transaction；odd word／long access 的
  address error、stack frame 與 fault 時機必須有測試後才標成完成。

### Prefetch 與中斷

- 68000 的 instruction prefetch queue 是可觀察 CPU 狀態，不能把所有取指簡化成直接
  `Read16(PC)` 而忽略順序。
- 外部 IPL 與 SR mask 分開保存。IRQ 只在定義的 poll point 取樣，並在合法 exception
  boundary 受理；level、edge、pulse、ack 與 vector source 不得混為一個 bool。
- STOP 必須繼續讓模擬時間與外部 device 前進，直到可受理事件喚醒 CPU；HALT 與 STOP
  是不同狀態。
- RESET 的 asserted duration、vector fetch、prefetch refill 與外部 reset side effect
  分開建模。

### Cycle accounting

- 每條指令的總 cycle 必須等於全部 bus phase 與 internal phase 的總和。
- branch、page／alignment、MUL／DIV data-dependent timing、exception 與 interrupt
  acknowledge 分別建立表格或演算法測試。
- `Step` 結果至少回報 elapsed cycles、exception／interrupt、PC before／after 與可選
  trace events，供 scheduler 和差分測試使用。

## 6. DMA、IRQ 與並行裝置

- DMA 不是函式內立即複製整塊記憶體的同義詞。要記錄啟動條件、每次 transaction、
  source／destination 更新、bus ownership、完成時間與 IRQ；若初版採批次近似，必須
  標示等級並保留可升級的 state machine。
- 同一時間點多事件的排序必須固定且文件化，例如 CPU bus phase、DMA、video scanline、
  timer 與 IRQ line 更新；不能依 Go map iteration 或 goroutine 排程決定。
- 模擬 device 預設不使用 goroutine 並行改狀態。可以用 goroutine 做不影響結果的輸出、
  壓縮或檔案工作，但需傳送不可變 snapshot，核心結果不得依主機排程。
- IRQ source 各自保存 pending／enable／ack 狀態，再由 interrupt controller 或明確優先權
  合成 CPU line；讀取狀態 register 不得順便清除沒有證據支持的其他 source。

## 7. 視訊、音訊與 Ebitengine 邊界

- UM6618 依 scanline／dot 或已證實的較粗 phase 推進；rendered framebuffer 是 device
  狀態的輸出，不是 scheduler 的時間來源。
- Ebitengine `Update` 預設 tick 與 `Draw` frame 都不保證等於 A'Can 晶片頻率。
  `Update` 只要求核心推進到下一個呈現 deadline；`Draw` 只讀已完成 framebuffer。
- UM6619 先產生固定硬體 sample domain，再用有狀態、可重現的 resampler 轉成 host
  audio rate。audio buffer 是否飢餓不能回頭改變 CPU／DMA 執行結果。
- headless runner 與 Ebitengine frontend 必須共用完全相同的 machine core；測試不可走
  另一套簡化 scheduler。

## 8. Reset、save state 與決定性

- 每個 device 明確實作 power-on、hard reset、soft reset；不能依 Go 零值偶然得到正確
  reset state。
- save state 只在定義的 phase boundary 擷取，涵蓋 CPU prefetch、scheduler queue、
  DMA 中途狀態、IRQ line、clock accumulator、RAM 與 device register。
- 載入先完整驗證 magic、schema version、ROM／BIOS 身分、payload 長度與 checksum，
  再交易性套用；失敗不得留下半套 state。
- 相同輸入 hash、初始 state 與 input timeline 必須產生相同的 bus trace、frame hash、
  audio hash 與終止 state。Go race-free 只是最低要求，不等於模擬決定性。

## 9. 測試階梯

1. 純 ALU／flag 與 addressing-mode table tests。
2. 每 opcode 的獨立向量：初始 register／memory → phase trace → 最終 state。
3. exception、IRQ、STOP、RESET、odd access 與 prefetch tests。
4. CPU 與 synthetic bus／DMA device 的事件排序測試。
5. 與固定 Moira sample 的差分測試；差異先分類，不自動以 Moira 為正解。
6. 以固定雜湊 IPL 跑到 UMC6650／授權／卡帶 entry 的 vertical slice。
7. C++ deprecated oracle 與 Go machine 的同輸入 bus／IRQ／frame／audio 差分。
8. 多款 ROM 的正常操作路徑與長時間決定性回歸。

每個測試必須說明它證明的是 ISA、sample 相容、Bcan 相容、MAME 相容或實機硬體；
綠色測試不可自動提升證據等級。

## 10. 效能與最佳化

- 先建立正確的 phase trace，再最佳化。fast path 與 slow path 必須共享相同語意測試。
- 可把沒有外部事件的 internal cycles 合併，但不能跨過 bus access、IRQ poll、DMA
  deadline 或 device event。
- opcode dispatch、memory fast path、framebuffer 更新與音訊批次化都需 benchmark；
  最佳化前後比較完整 state／trace hash，不能只看 FPS。
- 任何為單一 ROM 加入的快取或捷徑都要證明對所有合法輸入保持相同可觀察結果。

## 11. Moira 調查摘要

固定 sample 的 `MoiraConfig.h` 提供 simple 與 precise timing：simple mode 在指令末端
一次 `sync`；precise mode 在每次 memory access 前後同步，讓外部 device 先推進到
transaction 發生點。Moira 的 read／write、instruction fetch、prefetch、exception 與
interrupt handler 都顯式安排 cycle；IPL 在指定 poll point 取樣。

本專案採用其「指令 API＋內部 phase」概念，但不採用其 C++ template handler、macro、
class hierarchy 或程式生成結構。此摘要是 `sample-derived` 架構研究，不是 Super A'Can
硬體證據。
