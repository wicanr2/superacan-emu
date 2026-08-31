# Super A'Can 模擬器目前脈絡

更新日期：2026-09-01

## 專案定位

本專案是 Super A'Can 晶片與整機行為的跨平台模擬器，不是遊戲 remake。2026-08-31
已決定停止 C++ 產品線，改以純 Go 重寫 machine core，Ebitengine 負責畫面、音訊與
輸入；禁止 cgo。現有 C++ 將移至同 repo 的 `archive/cpp/`，只作 deprecated 行為
oracle 與歷史紀錄。可執行遊戲是驗證硬體模型的方法，不代表可以用遊戲專屬特判
取代晶片契約。`../acan/` 是唯讀的硬體／Bcan 逆向知識庫，目前由另一工作階段
review；本專案只引用固定證據，不回寫。

CPU 路線也已定案：Go Motorola 68000 是獨立實作，Moira 固定版只作 sample 與差分
oracle，不直接移植。CPU 對外一次 `Step` 一條指令，但內部按 fetch、prefetch、bus
read／write、internal cycle 與 IRQ poll phase 推進整機 scheduler，確保 DMA、IRQ、
雙 CPU 與觸發型 register 的時間順序可觀察。完整通則見
[`docs/chip-emulation-principles.md`](docs/chip-emulation-principles.md)。

同一方法適用所有其他晶片：65C02、UMC6650、UM6618、UM6619、DMA 與輸入都須純 Go
獨立實作，以舊 C++、MAME、Bcan 與實機作分級 oracle，不直接翻譯來源程式。

### Go 主線目前實作

| 元件 | 狀態 | 證據邊界 |
|---|---|---|
| module | `github.com/wicanr2/superacan-emu`，Go 1.26 | 尚未加入 Ebitengine dependency |
| 68000 phase API | scheduler-before-bus、24-bit address、FC、byte／word transaction | API 已測；尚未有整機 scheduler consumer |
| 68000 reset | supervisor SR、SSP／PC vector、兩級 prefetch | 40-cycle 起始值目前是 sample-derived，待 Motorola 規格審查 |
| 68000 opcode | 真實 IPL＋Boom Zoo 已無錯執行 1,300,000 條指令，完成 sound boot、多輪 video 初始化與 58 次 IRQ7；已有 autovector、RTE | 官方 ISA／timing 表；未覆蓋完整 ISA、一般 exception、user/supervisor stack 切換 |
| W65C02 | 純 Go reset／堆疊／分支與 Boom Zoo boot 所需子集；已產生 `$0300=$FF` ack | 3:1 shared scheduler；尚未完整 ISA／IRQ |
| media | word-swap、大小驗證、原始 SHA-256 manifest | BIOS／ROM 不入版控 |
| machine bus | ROM 雙視圖、IPL 雙 overlay、Work/sound RAM、SRAM、`$E90B3C`、UMC6650 | 視訊／音訊／DMA window 尚未接入 Go |
| UMC6650 | 位址／資料埠、唯讀 key、32-byte RAM 與 output registers | IPL/Bcan (a) 級 port 契約 |
| UMC6619 | 65C02 間接位址／資料埠與暫存器檔 | PCM／timer／DMA 尚未執行 |
| UM6618 | register／palette／128 KiB VRAM、684／728-cycle scanline、IRQ4／5／7；sprite DMA bus master；tilemap／sprite／window／ROZ framebuffer 與逐行 ROZ 表 | Boom Zoo 已非黑且 hash 可重現；IRQ7 真實受理，IRQ4／5 僅合成驗證；逐行表為 MAME-derived，oracle 畫面差分尚未完成 |
| headless runner | 可載入外部 IPL/key/ROM 並有界執行雙 CPU 與裝置 | 1,300,000 條 68k／1,524,044 條 65C02；雙 overlay 關閉 |
| bus observer | 可依 24-bit 位址範圍有界保留 byte／word transaction | word access 恰為一筆；含 68k PC／opcode／step |
| archived C++ | `archive/cpp/` | 從新 source root 的 Docker Release 重建已通過 |

MAME 的核心觀念適用於本專案：模擬器原始碼同時是硬體文件，可執行性用來驗證文件
是否足夠準確。MAME 也把位址空間建模為具有資料寬度、位址寬度、端序與 address shift
的 bus，並將 share、bank、region 與 view 分開；除錯報告則要求固定版本、精確系統／
媒體設定、重現步驟與原始硬體參考。這些原則已轉成 `AGENTS.md` 的裝置邊界、排程、
證據分級、可重現 trace 與 save-state 規則。

官方參考：

- MAME README：<https://github.com/mamedev/mame/blob/master/README.md>
- MAME 貢獻與問題重現：<https://docs.mamedev.org/contributing/index.html>
- MAME 位址空間與記憶體：<https://docs.mamedev.org/techspecs/memory.html>
- MAME debugger：<https://docs.mamedev.org/debugger/index.html>
- MAME watchpoint：<https://docs.mamedev.org/debugger/watchpoint.html>

## 2026-08-31 deprecated C++ 狀態表

此表描述即將移入 `archive/cpp/` 的 `d923486` C++ oracle，不再代表 production 主線。
舊里程碑文件若與 `docs/verify-misc.md` 衝突，以後者及該 commit 程式為準。

| 類別 | 目前狀態 | 證據邊界 |
|---|---|---|
| 68k／65C02 | 已執行兩顆 CPU、reset／HALT、主要 IRQ | Moira／CLK API 與三款遊戲路徑已驗證；非所有 cycle edge 均有實機證據 |
| bus／UMC6650 | IPL、lockout、overlay、主要映射已接通 | IPL 與 Bcan RE 證據強；open-bus 與未知區仍需逐項分級 |
| UM6618 | 3 tilemap、sprite、window 0、ROZ、DMA、主要 IRQ 已實作 | ROZ 多數來自 MAME；window 1 是未經遊戲驗證的推測實作 |
| UM6619 | 16-channel PCM、DMA、timer、48 kHz 輸出 | MAME-derived；envelope 與實機混音／削波仍未知 |
| 輸入 | P1、P2 鍵盤與 headless 注入已接 | P1 正常路徑較完整；P2 只驗證資料路徑，未驗證完整雙人遊戲 |
| Save state | 自訂 `ACANEST1` 格式與 CLI／熱鍵已寫入 | Boom Zoo 3000→存檔→載入→60 幀截圖相同；格式不相容 Bcan |
| FRC IRQ3 | 依 MAME case 表實作 | MAME 自身標為 HACK，真實硬體公式未知 |
| 平台 | Linux SDL2／headless | macOS 尚未建立可重現編譯與實機 smoke |

## 已確認的重要勘誤

- 16-bit video 暫存器寫入不能拆成兩次 byte read-modify-write；這會讓 sprite DMA
  觸發兩次並破壞相鄰 VRAM。里程碑 5 已改為單一 word transaction。
- IPL overlay bit1／bit3 是關閉後不恢復的單向 latch；IRQ 採受理後解除的
  HOLD_LINE 語意，否則遊戲主迴圈會鎖死。
- 65C02 reset 必須在 CLK 核心實際消耗 cycle 時保持有效，直到向量讀取序列開始；
  IRQ 來源是 level-held 且各有專屬 ack，`$0411` 只回報狀態。
- Speedy Dragon 實際第二音效驅動上傳路徑與舊靜態猜測不同；目前路徑已可播放。

## 尚未升格為硬體事實

- FRC 真實計時公式。
- UM6619 `$A0-$D0` envelope、混音增益與削波。
- latch 3-byte 封包的玩家可見用途。
- window 1 行為，以及 ROZ 複雜逐行模式的硬體正確性；目前只有 MAME-derived 實作。
- P2 完整雙人流程，以及 save state 在所有裝置事件邊界的決定性。
- 逐行 partial update 是否為現有遊戲所需；目前未見能證明必須實作的畫面缺陷。

## 下一個交付閘門

下一個 vertical slice 已把 UM6618 register／palette／VRAM、scanline 與第一版 framebuffer
接入，並讓卡帶自然離開 vblank poll。固定 Boom Zoo 路徑已有 61,437 個非黑像素；這是
合成器生命跡象，不是畫面正確性宣告。現在實作 sprite／主機 DMA、IRQ4／5／7、複雜
ROZ 逐行模式，並建立同 frame archived oracle 差分。掃描線 IRQ4／5／7 與 68000
autovector 已接通；固定 smoke 實際受理 58 次 IRQ7。
UM6619 仍須由 register file 擴充為 PCM／timer／DMA，W65C02 仍須完成
完整 ISA 與 IRQ。各晶片都以 archived C++ 與 MAME 作分級 oracle，而非直接翻譯。
Ebitengine 前端不能先於 headless machine core 決定 scheduler。
