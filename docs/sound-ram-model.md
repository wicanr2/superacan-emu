# Sound RAM 位址模型與 `$F001F0` 的實作契約

更新日期：2026-09-01。本文記錄兩個曾經未定案、現在改以「跟隨 Bcan 0.0.8b」定案的
實作契約，以及它們的證據邊界。

## 決定

| 項目 | 契約 | 證據等級 |
|---|---|---|
| sound RAM 位址空間 | **64 KiB 平面，不遮罩 A15**；`$0400–$04FF` 以完整 65C02 位址解碼為 I/O，其餘全部是 RAM | `confirmed-Bcan` |
| `$F001F0` pixel mode（bit 3／bit 4） | 解碼後保存並供讀回與存檔，**不進入 renderer** | `confirmed-Bcan` + MAME 一致 |

兩項都不是 `confirmed-hardware`。它們的意思是「與目前唯一能跑完整軟體庫的第三方實作
一致」，不是「實機如此」。硬體側的未決狀態記錄在唯讀知識庫
`../acan/docs/memory-map.md` §5.1 與 `../acan/docs/f003-video-mode.md` §7。

## Bcan 的實際做法（IDA Pro 9.4 反編譯）

**sound RAM**：65C02 側的讀寫（`sub_1400A59C0`／`sub_1400A5220`）以完整 16-bit 位址
`switch`，`$0407`、`$040A`、`$0420`、`$0422` 等逐一列為 I/O case，其餘走 `default:`：
`*(BYTE *)(*(QWORD *)(obj + 16) + a2)`，用未遮罩的 16-bit 位址索引同一塊緩衝區。68k 側
`SystemBus` 判斷 `(addr & $FF0000) == $E80000` 後轉發完整位址，位址分類器保留的 offset
是 `addr − $E80000`。即時存檔的機器區段連續序列化 `0x10000`／`0x10000`／`0x8000` 三塊。

**pixel mode**：`$F001F0` 寫入後在 `sub_1400A9200` 拆成 `value & 0x18`（pixel mode）與
`value & 7`（gfx mode）兩個 byte。每幀由 `sub_140082130` 建立 snapshot 給 renderer
`sub_14009D6E0`；該建構器以一次 8-byte 讀取取得含這兩個 byte 的欄位群，但只取用其中的
video flags 與圖層致能位元，兩個 mode byte 未進入 snapshot。

## 本專案的現況

- `machine.Bus` 預設 `soundRAMMask = 0xffff`，符合上表契約。
  `SetSoundRAMAlias(true)` 與 headless `--sound-ram-alias` 只作為 32 KiB 假說的診斷開關
  保留，附帶上下半區對撞偵測；預設關閉，不影響任何驗證路徑。
- `chip/umc6618` 保存 `pixelMode = value & 0x1f` 並在 register `$1F0` 讀回，renderer
  不使用 bit 3／bit 4，與上表一致。

## 仍待解決的分歧：全域 gfx mode

`tilemapRegion()` 目前依 MAME 的 `get_tilemap_region()` 用 `pixelMode & 7` 選 tile
region（`{2,1,0,1,0,0,0,0}`／`{2,1,1,1,2,2,2,2}`）。但 Bcan 的 renderer snapshot **不含**
全域 gfx mode，代表它改由各圖層自己的 mode 暫存器決定色深與 region。

兩邊規則不同，就會在畫面差分時顯示為圖層色深或 tile 來源錯誤。**在釐清 Bcan 的逐層規則
之前不要盲目移除現行 MAME-derived 路徑**——它同時決定 bpp，拿掉會直接壞掉。處理順序：

1. 反編譯 `sub_140082130` 的逐層欄位與 `sub_14009D6E0` 的 tile 取樣，寫出 Bcan 由哪個
   暫存器位元決定 bpp／region；
2. 找一款 gfx mode 非 1 的遊戲（Sango Fighter 寫 `$0003`）做同畫面差分；
3. 差分確認後才改規則，並在此文件記錄改前改後的 framebuffer hash。
