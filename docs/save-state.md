# 存檔格式與載入契約

更新日期：2026-09-01

## 檔案版面

存檔是本專案自有的格式，與 Bcan 的 `ACANRTS` 不相容，也不宣稱相容。

| Offset | 大小 | 內容 |
|---:|---:|---|
| 0x00 | 8 | magic `ACANGOS1` |
| 0x08 | 2 | 版本（目前 1，讀端強制相符）|
| 0x0A | 2 | 標頭長度（96，讀端強制相符）|
| 0x0C | 4 | flags，保留 |
| 0x10 | 32 | IPL 的 SHA-256 |
| 0x30 | 32 | 卡帶的 SHA-256 |
| 0x50 | 8 | payload 長度 |
| 0x58 | 8 | 保留 |
| 0x60 | 32 | payload 的 SHA-256 |
| 0x80 | n | payload |

雜湊取的是建構 `machine.System` 時餵進來的位元組，也就是 word-swap 之前的原始檔案；
使用者拿手上的檔案算 SHA-256 就能對上。Boom Zoo 的存檔約 590 KiB。

## 載入是交易式的

`LoadState` 先把整份檔案讀進記憶體並依序驗證 magic、版本、標頭長度、IPL 身分、
卡帶身分、payload 長度與 payload 雜湊，全部通過才呼叫 `Restore` 一次套用。任何一項
失敗都在套用之前回傳錯誤，現行狀態一個位元組都不會改變——這一點有測試守著，
四種壞檔（payload 損壞、截斷、magic 不符、版本不符）逐一驗證拒絕後狀態不變。

跨卡帶載入預設拒絕，沒有放寬的選項。

## payload 涵蓋什麼

`machine.SystemState` 的欄位順序就是序列化順序。新增欄位一定要同步提高版本號，
否則讀端會把舊檔誤讀成新版。目前涵蓋：

- 兩顆 CPU 的完整架構狀態與 prefetch queue，加上 68000 的中斷輸入與 level 7 latch、
  65C02 的 IRQ／NMI 取樣與 WAI 停等旗標。
- 兩條時間線的累計週期、相位計數與最後一次相位／週期記錄。
- Work RAM、sound RAM、卡帶 SRAM、overlay latch、`$E90B3C`、control。
- sound bus 的 IRQ、I/O 區、兩組手把、shift register 與 latch。
- UM6618 的暫存器、調色盤、VRAM、掃描線／frame 計數、三個 IRQ 旗標與 framebuffer。
- UM6619 的暫存器、十六個通道（含小數相位）、timer 餘數與兩個 IRQ 旗標。
- UMC6650、FRC（含尚未走完的餘數）、主機 DMA 兩個通道。

framebuffer 雖然是衍生資料仍然保存：載入後到下一個 vblank 之間畫面不會重新合成，
不存就會顯示上一份內容。`soundClashes` 是診斷用的統計 map，不影響輸出，不保存。

## 已驗證

- 單元測試：round-trip 後 snapshot 完全相同；由存檔續跑 128 條指令的結果與連續執行
  相同；四種壞檔都被拒絕且不改變現行狀態。
- 真實 ROM：Boom Zoo 在 frame 600 存檔，另一個行程載入後再跑 600 frame，指令數
  17,369,003 與 framebuffer SHA-256 `3784f866…94155562` 與連續跑 1200 frame 完全相同。

  音訊雜湊在兩邊不同，這是取樣視窗不同造成的：連續執行累積的是第 0 到 1200 frame
  的樣本，載入後只累積第 600 到 1200 frame。這不是分歧。

## 使用方式

```sh
# headless：跑到指定 frame 存檔，之後從存檔續跑
go run ./cmd/acan-headless … --frames 600 --save-state run.state
go run ./cmd/acan-headless … --load-state run.state --frames 600

# 兩個 GUI 前端：--state 指定檔案，F5 存、F7 讀
go run ./cmd/acan-x11 … --state run.state
```

`--save` 則是另一件事：卡帶電池記憶體的 32768 bytes 完整映像，開機載入、結束寫回，
兩者都採先寫暫存檔再改名，避免中途失敗留下半套檔案。
