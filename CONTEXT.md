# Super A'Can 模擬器目前脈絡

更新日期：2026-08-31

## 專案定位

本專案是 Super A'Can 晶片與整機行為的跨平台模擬器，不是遊戲 remake。可執行遊戲
是驗證硬體模型的方法，不代表可以用遊戲專屬特判取代晶片契約。`../acan/` 是唯讀的
硬體／Bcan 逆向知識庫，目前由另一工作階段 review；本專案只引用固定證據，不回寫。

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

## 2026-08-31 目前狀態表

基準為 `master` 的 `18110af` 加上目前尚未提交的里程碑 5 工作樹。舊里程碑文件若與
`docs/verify-misc.md` 衝突，以後者及目前程式為準。

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
- window 1 行為與 ROZ 複雜逐行模式。
- P2 完整雙人流程，以及 save state 在所有裝置事件邊界的決定性。
- 逐行 partial update 是否為現有遊戲所需；目前未見能證明必須實作的畫面缺陷。

## 下一個交付閘門

先把目前里程碑 5 工作樹在隔離 Docker 工具鏈完成乾淨建置與回歸，修正 save state 的
ROM 身分與失敗即關閉政策，並建立至少兩款遊戲的存讀檔決定性測試。通過後才進入
macOS 可重現編譯規劃；macOS 移植不得改變核心時序或晶片語意。
