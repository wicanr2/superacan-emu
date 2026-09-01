# Bcan 0.0.8b 介面功能盤點

更新日期：2026-09-01

本文是「Go 版介面要對齊什麼」的基準。內容取自 `Bcan008b/Bcan.exe` 的 UTF-16 字串表
（1,000 條去重後的介面字串）與 Wine 內實際開啟選單的截圖，屬第一手觀察；
不從 Bcan 的行為推論硬體語意。

## 選單列

| 選單 | 項目 |
|---|---|
| 檔案 (&F) | 開啟 ROM…（Ctrl+O）／重新啟動遊戲 (&R)／循環存檔槽（0–9）／離開 (&X) |
| 顯示 (&V) | 整數縮放 (&S)／全螢幕（Alt+Enter）／以 4:3 顯示（預設）／視訊濾鏡 (&V)／動態平滑（減少 LCD 拖影）／全速選項／隱藏熱鍵操作訊息（錯誤仍顯示）／錄影格式 |
| 輸入 (&I) | P1 鍵盤／手把…／P2 鍵盤／手把…／熱鍵設定… |
| 金手指 (&C) | 記憶體搜尋／金手指管理…／鎖定全部金手指 |
| 語言 (&L) | English／Français／Español／繁體中文／简体中文 |
| 說明 (&H) | 關於 Bcan／效能診斷：開始／停止並儲存 |

## 視訊濾鏡

Nearest（預設）、Bilinear smoothing、Scanline 25/50/75%、CRT Lite、CRT Full、
Composite，另有「CRT Full brightness」百分比。設定會存進 `Bcan.ini`；訊息文字
「Video filter applied and saved. F8/F9 remain unfiltered.」表示截圖與錄影不套濾鏡。

## 熱鍵（全部可重新指定）

存檔、讀檔、循環存檔槽、截圖、開始錄影、停止錄影、顯示／隱藏 FPS、切換全速、
鎖定全部金手指。指定時偵測重複：「Each action must use a different hotkey.」

## 輸入設定對話框

- 輸入裝置切換：鍵盤／手把，並顯示「Current controller:」。
- 鍵盤與手把兩組綁定可同時存在：「Click an item, then press a keyboard key or
  controller button; both may coexist. Delete clears both.」
- 按鈕：Save／Cancel／Close／Defaults。
- 版面說明：「The layout follows the physical X/A, Y/B, R/L button arrangement.」
- 變更立即生效並寫回 `Bcan.ini`。

## 金手指管理

- 搜尋範圍限 Work RAM `$FC0000–$FCFFFF`（64 KiB）。
- 搜尋模式：Exact／Fuzzy；操作：New Search／Refine。
- 欄位：Search:／Value:／Width:／Name:；顯示格式含 Normal decimal 與 BCD。
- 清單操作：Add + Lock／Lock / Unlock／Remove／Update Value / Name。
- 顯示候選數量「Candidate addresses: N」，上限 4096 筆。
- 每遊戲一個 `.cht` 檔（tab 分隔純文字，標頭 `BCAN_CHT_1`），上限 1024 筆。

## 存檔

十個槽（0–9），熱鍵循環切換。檔案 magic `ACANRTS`，綁定 ROM 的 SHA-256，
只在 CPU 指令邊界存檔，讀檔採交易式還原。

## 錄影與截圖

- 錄影兩種格式：MP4（H.264/AAC，檔案較小）與 AVI（MJPEG，結束快、檔案較大）。
- 截圖為 PNG，直接取自 UM6618 顯示孔徑，固定 320×240，FPS 顯示不會進截圖。

## 狀態列與訊息

- 未載入：「Ready. Place your BIOS in bios\ and choose File / Load ROM.」
- 執行中：「ROM running | native 320x240 video | live P1/P2 input」
- 操作訊息採 toast 形式（可隱藏，錯誤仍顯示），例如「Full speed enabled.」、
  「Full-speed audio: muted.」、「FPS display enabled.」。

## 安全停止與診斷

- 啟動失敗有明確文案，例如「ROM not started: the 68000 cold reset stopped on a
  checked fault.」與「The checked BIOS boot did not reach this cartridge's reset entry
  within the safety limit. No gameplay session was started.」
- 效能診斷：從選單開始／停止並寫出 `save\Bcan-performance-diagnostic.txt`。
- 另有自動故障報告 `.fault.txt` 與 `presentation-trace.csv`。

## 設定檔

`Bcan.ini`，純文字鍵值。註解明寫「Unknown keys are ignored for forward and backward
compatibility.」——未知鍵一律忽略，這個相容策略值得沿用。
