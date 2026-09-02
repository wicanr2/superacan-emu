# Super A'Can 模擬器目前脈絡

更新日期：2026-09-02

## 專案定位

本專案是 Super A'Can 晶片與整機行為的跨平台模擬器，不是遊戲 remake。2026-08-31
已決定停止 C++ 產品線，改以純 Go 重寫 machine core，Ebitengine 負責畫面、音訊與
輸入。cgo 政策已於 2026-09-01 定案：**整個發行 binary 禁止 cgo，前端不例外**。
Ebitengine v2.9.9 只有 Linux／BSD 桌面目標需要 cgo，`js/wasm` 與 `windows/amd64`
在 `CGO_ENABLED=0` 下可建置；Linux 桌面另由純 Go 的 `cmd/acan-x11` 提供，兩個前端
共用同一個 machine core。現有 C++ 已移至同 repo 的 `archive/cpp/`，只作 deprecated 行為
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
| module | `github.com/wicanr2/superacan-emu`，Go 1.26、Ebitengine v2.9.9 | `go.sum` 已固定；machine core 不 import Ebitengine |
| 68000 phase API | scheduler-before-bus、24-bit address、FC、byte／word transaction | API 已測；尚未有整機 scheduler consumer |
| 68000 reset | supervisor SR、SSP／PC vector、兩級 prefetch | 40-cycle 起始值目前是 sample-derived，待 Motorola 規格審查 |
| 68000 opcode | 一般化 effective-address 執行層覆蓋全部 12 種定址模式與主要指令族；八款 ROM 各完成 3600 frames，最高 56,747,720 條指令 | 時間取自 PRM 指令時間表（`strong-inference`）；未與 Moira 做逐指令差分 |
| W65C02 | 256 項指令表覆蓋完整 65C02 指令集與 W65C02S 未指派編碼的 NOP 行為；八款 ROM 皆產生非零音訊 | 3:1 shared scheduler；`$DB`（STP）維持 fail-closed |
| media | word-swap、大小驗證、原始 SHA-256 manifest、raw 與 ZIP 卡帶（含雙部分）| BIOS／ROM 不入版控；雙部分依尺寸接合，來源為 Bcan 的驗證規則 |
| machine bus | ROM 雙視圖、IPL 雙 overlay、Work/sound RAM、SRAM、`$E90B3C`、UMC6650、UM6619 主機端讀取埠 | `$E90004/05`、`$E9000C/0D`、`$E90018/19` 為 MAME-derived，與 Bcan 讀取閂位置一致 |
| UMC6650 | 位址／資料埠、唯讀 key、32-byte RAM 與 output registers | IPL/Bcan (a) 級 port 契約 |
| UMC6619 | 16-channel PCM、timer、DMA、IRQ、原生樣本與 48 kHz 呈現重取樣 | 三款 ROM 有非零音訊；envelope／實機混音與削波仍未知 |
| UM6618 | register／palette／128 KiB VRAM、684／728-cycle scanline、IRQ4／5／7；sprite DMA bus master；tilemap／sprite／window／ROZ framebuffer 與逐行 ROZ 表 | 四款卡帶共 19 張實際畫面與 Bcan 逐像素相同（4bpp 半位元組次序因此定案）；IRQ7 真實受理，IRQ4／5 僅合成驗證；逐行表為 MAME-derived；顯示區為何是第 8 條起的 224 條仍是 unknown；已知一處真差異是 sprite 沒有被 letterbox 切掉 |
| headless runner | 可載入外部 IPL/key/ROM 並有界執行雙 CPU 與裝置；有界指令回溯、視訊暫存器 dump、存讀檔 | 九款卡帶各完成 3600 frames |
| save state | `ACANGOS1` 格式、交易式載入、綁定 IPL 與卡帶 SHA-256 | 決定性已用 Boom Zoo 驗證；與 Bcan 的 ACANRTS 不相容 |
| Ebitengine frontend | P1 鍵盤、320×240 framebuffer、48 kHz audio、frame-bound runner、PNG smoke | 八款 ROM 各 1200 frames，指令數與 framebuffer SHA-256 均吻合 headless；`CGO_ENABLED=0` 可建 js/wasm 與 windows |
| X11 frontend | 純 Go 視窗／鍵盤／整數倍放大，音訊走外部播放程序 | `CGO_ENABLED=0` 可建置；八款 ROM 各 1200 frames 與 headless 完全相同 |
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
| 平台 | Linux：Ebitengine／純 Go X11／headless；macOS：purego Cocoa 視窗層 | macOS 只有 `CGO_ENABLED=0` 交叉編譯與 vet，沒有實機 smoke；Android 尚未開始 |

## 已確認的重要勘誤

- 16-bit video 暫存器寫入不能拆成兩次 byte read-modify-write；這會讓 sprite DMA
  觸發兩次並破壞相鄰 VRAM。里程碑 5 已改為單一 word transaction。
- IPL overlay bit1／bit3 是關閉後不恢復的單向 latch；IRQ 採受理後解除的
  HOLD_LINE 語意，否則遊戲主迴圈會鎖死。
- 65C02 reset 必須在 CLK 核心實際消耗 cycle 時保持有效，直到向量讀取序列開始；
  IRQ 來源是 level-held 且各有專屬 ack，`$0411` 只回報狀態。
- Speedy Dragon 實際第二音效驅動上傳路徑與舊靜態猜測不同；目前路徑已可播放。
- 調色盤 5 位元分量展開為 `v<<3 | v>>2`，`$1F` 對到 `$FF`。Bcan 截圖像素與 MAME 的
  `palette_device::xBGR_555` 兩個 oracle 一致；先前的 `v<<3` 讓每個非黑顏色都偏暗最多 7。
  1200-frame framebuffer SHA-256 基準因此全部更新，指令數不變。

## 尚未升格為硬體事實

- FRC 真實計時公式。
- UM6619 `$A0-$D0` envelope、混音增益與削波。
- latch 3-byte 封包的玩家可見用途。
- window 1 行為，以及 ROZ 複雜逐行模式的硬體正確性；目前只有 MAME-derived 實作。
- P2 完整雙人流程，以及 save state 在所有裝置事件邊界的決定性。
- 逐行 partial update 是否為現有遊戲所需；目前未見能證明必須實作的畫面缺陷。

## 下一個交付閘門

目前 machine core 的最低相容性閘門已由三款 ROM 各 1200 frames 通過：Speedy Dragon
18,515,145、Formosa Duel 19,272,069、Boom Zoo 17,370,088 條 68000 指令，均有可辨識
framebuffer；三款亦有非零音訊資料。

2026-09-01 後續：CPU 改為一般化 effective-address 執行層之後，`Bcan008b/ROMS` 下的
八款 raw ROM 全部完成 3600-frame 有界執行，帶輸入的 5400-frame 路徑也全部完成，人工
檢查可看到實際遊戲畫面而不只是標題。八款的 Ebitengine GUI 與 headless 在 1200 frame
的指令數與 framebuffer SHA-256 完全一致。完整矩陣見
[`docs/verify-rom-matrix.md`](docs/verify-rom-matrix.md)。

像素層級的正確性已經有一批定案：八款卡帶各跑一次 Bcan 0.0.8b oracle 共 96 張截圖，
其中 **19 張有實際內容的畫面整個顯示區逐像素相同**（橫跨 Boom Zoo、Formosa Duel、
Speedy Dragon 與 Super Taiwanese Baseball League 四款，含各自的標題與選單）。
前提是先還原 Bcan 截圖的孔徑縮放——它把實際顯示區以最近鄰撐滿 320×240，兩個軸都要
還原。對不上的多半是動畫相位；目前只定位到一處真差異（Boom Zoo 開場的 sprite 沒有
被 letterbox 切掉，88–352 個像素）。管線見
[`docs/bcan-oracle-diff.md`](docs/bcan-oracle-diff.md)。禁 cgo 政策已落地：`cmd/acan-x11` 與 `cmd/acan-macos` 都在
`CGO_ENABLED=0` 下建置，覆蓋層介面（P0–P8）與 `session` 的 Intent 邊界已完成。
平行未完成的是實機音訊（目前交給外部播放程序）、macOS 實機 smoke、Android 平台層
與發行包。尚未被遊戲覆蓋的 ISA／硬體模式保留明確證據限制。
