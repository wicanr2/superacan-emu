# 使用者介面規劃

更新日期：2026-09-01

## 現況

模擬器目前只有命令列旗標。啟動必須先知道 IPL、key、兩個音效 BIOS 與卡帶的路徑，
按鍵是寫死的，存檔槽只有單一檔案，診斷功能（`--layer-mask`、`--video-registers`、
`--trace-instructions`）也只能在重跑時指定。兩個 GUI 前端各自處理視窗與輸入，
共用的只有 machine core。

## 設計約束

這些不是偏好，是已經定案的契約：

1. **UI 不得改寫晶片狀態。** 暫停只停止呼叫端推進時間線，不動 scheduler 的餘數；
   任何「讓遊戲過關」的介面功能都不做。
2. **整個發行 binary 禁止 cgo。** 因此不能用 GTK、Qt 或任何系統原生 widget，
   也不能用需要 cgo 的字型 rasterizer。
3. **兩個前端要共用同一份 UI。** Ebitengine（`js/wasm`、`windows`）與純 Go X11
   （Linux 桌面）不能各寫一套。
4. **headless 必須維持可跑核心回歸。** UI 是可選層，不進 `cmd/acan-headless`。

## 架構決定：自繪的 `ui` 套件

新增純 Go 的 `ui` 套件，把介面畫進一塊 RGBA 緩衝，並消費抽象的輸入事件。
前端只負責兩件事：把緩衝貼上視窗、把實體輸入翻成抽象事件。

```
machine core ──► UM6618 framebuffer ─┐
                                     ├─► ui.Compose ──► 前端貼圖
ui 狀態機 ──► overlay RGBA ──────────┘
```

這樣切的理由：

- **可在 headless 測試。** 把介面畫進緩衝再比對 SHA-256，就跟現在驗證 framebuffer
  的方式一樣，不需要開視窗。
- **兩個前端零重複。** 新增 wasm 前端時 UI 直接可用。
- **UI 不碰 machine。** `ui` 套件只讀 framebuffer 與一組唯讀的狀態查詢介面，
  要求動作時回傳意圖（載入這個 ROM、寫入這個存檔槽），由入口執行。

`ui` 套件不 import Ebitengine，也不 import X11；這一條要有測試守著，
和現在 machine core 不 import Ebitengine 的規則一樣。

## 畫面

| 畫面 | 用途 | 進入方式 |
|---|---|---|
| 無卡帶啟動畫面 | 沒有 `--rom` 時的落點，列出最近開過的卡帶 | 啟動 |
| 卡帶瀏覽器 | 從目錄挑檔，顯示檔名、大小、SHA-256 前八碼 | 啟動畫面或選單 |
| 覆蓋選單 | 繼續／存檔／讀檔／重設／設定／離開 | 遊戲中按 Esc |
| 存檔槽 | 十個槽，每格顯示縮圖與時間 | 覆蓋選單 |
| 按鍵設定 | P1／P2 逐鍵重新綁定，含衝突提示 | 設定 |
| 影像設定 | 縮放倍率、整數縮放、4:3、掃描線 | 設定 |
| 音訊設定 | 音量、輸出方式 | 設定 |
| 診斷面板 | frame／指令數／圖層遮罩／視訊暫存器 | 設定或熱鍵 |

存檔槽的縮圖直接用存檔裡已經保存的 framebuffer，不另外存 PNG——這是把
`docs/save-state.md` 裡「framebuffer 雖然是衍生資料仍然保存」那個決定用起來。

## 輸入模型

`ui` 消費抽象事件（`Up`／`Down`／`Left`／`Right`／`Confirm`／`Cancel`／`Menu`），
不認識 keysym 或 `ebiten.Key`。因此手把也能操作選單，而且兩個前端各自的鍵盤對照表
不會滲進 UI 邏輯。

## 設定檔

JSON（stdlib，純 Go，不需要新依賴），路徑依 XDG：
`$XDG_CONFIG_HOME/superacan-emu/config.json`，未設時退回 `~/.config`。

內容：BIOS 與 key 的路徑、最近使用的卡帶目錄與清單、P1／P2 按鍵配置、影像與音訊選項、
存檔目錄。**未知欄位一律忽略**，讓舊版讀得懂新檔；這一點沿用 `Bcan.ini` 的做法
（「Unknown keys are ignored for forward and backward compatibility」）。

版權輸入不進設定檔以外的任何地方，設定檔本身也只存路徑，不存內容。

## 字型

先用 `golang.org/x/image/font/basicfont`：純 Go、ASCII、體積小、授權明確
（BSD-3，Go 作者）。UI 字串因此第一階段是英數。

繁體中文另排一階段。候選是 `github.com/hajimehoshi/bitmapfont/v4`（含 CJK）或自製
子集。**動工前要先確認授權能否隨發行包散布**，這是硬性前置條件，不是實作細節；
`rulebook/85` 的授權規則對字型同樣適用。

## 分階段交付

| 階段 | 內容 | 驗收條件 |
|---|---|---|
| 一 | `ui` 套件骨架、覆蓋選單、存讀檔槽 | 兩個前端都能叫出選單並完成存讀檔；`ui` 的畫面在 headless 測試中有可重現的 SHA-256；`ui` 不 import 任何前端套件 |
| 二 | 無卡帶啟動畫面、卡帶瀏覽器 | 不帶 `--rom` 啟動可以選到九款卡帶並開始執行；ZIP 與 raw 都能列出 |
| 三 | 按鍵設定與設定檔 | 重新綁定後重啟仍生效；未知欄位被忽略；衝突有提示 |
| 四 | 影音設定、診斷面板 | 縮放與整數縮放即時生效；圖層遮罩與視訊暫存器可在遊戲中查看，且不改變 framebuffer 雜湊 |
| 五 | 繁體中文與語言切換 | 字型授權已確認可散布；中英文切換不需重啟 |

每一階段都要維持既有的驗證：九款卡帶的 1200-frame 指令數與 framebuffer SHA-256
不得改變。UI 是覆蓋層，動到這些數字就表示它滲進了模擬路徑。

## 不做什麼

- 不做 shader 濾鏡。可用的 GPU 路徑在禁 cgo 之下受限，而且畫面正確性還沒收斂，
  先做濾鏡是把精力放錯地方。
- 不做金手指與即時記憶體編輯。診斷面板只讀不寫；要寫入的需求等硬體行為定案再談。
- 不在 UI 裡內建任何 ROM、BIOS 或遊戲畫面。
- 不做網路對戰。

## 與現有工作的關係

診斷面板是把 `--layer-mask`、`--video-registers`、`--trace-instructions` 這些既有的
有界除錯介面搬到遊戲中查看，不是新增能力。`WORKLIST.md` 裡「將一次性環境變數 trace
整理成有界、可篩選的 device／address-space 除錯介面」這一項，會在階段四收尾。
