# Sound RAM 位址模型與 `$F001F0` 的實作契約

更新日期：2026-09-01。本文記錄兩個曾經未定案、現在改以「跟隨 Bcan 0.0.8b」定案的
實作契約，以及它們的證據邊界。

## 決定

| 項目 | 契約 | 證據等級 |
|---|---|---|
| sound RAM 位址空間 | **64 KiB 平面，不遮罩 A15**；`$0400–$04FF` 以完整 65C02 位址解碼為 I/O，其餘全部是 RAM | `confirmed-Bcan` |
| `$F001F0` gfx mode（bit 0–2） | 與 MAME 相同的三張圖層 region 表換算色深 | `confirmed-Bcan` + MAME 一致 |
| `$F001F0` pixel mode（bit 3–4） | 進入 renderer，但**只在 ROZ 層生效**：`pixel_mode == $08` 且 ROZ 為 8bpp region 時走替代路徑 | `confirmed-Bcan`（MAME 完全不消費） |

兩項都不是 `confirmed-hardware`。它們的意思是「與目前唯一能跑完整軟體庫的第三方實作
一致」，不是「實機如此」。硬體側的未決狀態記錄在唯讀知識庫
`../acan/docs/memory-map.md` §5.1 與 `../acan/docs/f003-video-mode.md` §7。

## Bcan 的實際做法（IDA Pro 9.4 反編譯）

**sound RAM**：65C02 側的讀寫（`sub_1400A59C0`／`sub_1400A5220`）以完整 16-bit 位址
`switch`，`$0407`、`$040A`、`$0420`、`$0422` 等逐一列為 I/O case，其餘走 `default:`：
`*(BYTE *)(*(QWORD *)(obj + 16) + a2)`，用未遮罩的 16-bit 位址索引同一塊緩衝區。68k 側
`SystemBus` 判斷 `(addr & $FF0000) == $E80000` 後轉發完整位址，位址分類器保留的 offset
是 `addr − $E80000`。即時存檔的機器區段連續序列化 `0x10000`／`0x10000`／`0x8000` 三塊。

**`$F001F0`**：寫入後在 `sub_1400A9200` 拆成 `value & 0x18`（pixel mode）與 `value & 7`
（gfx mode）兩個 byte。每幀由 `sub_140082130` 建 snapshot 給 renderer `sub_14009D6E0`：
`mov rax,[video+588]` → `shr rax,48` → `mov [snapshot+0BEh],r8w`，即 **pixel mode 進
snapshot+190、gfx mode 進 snapshot+191**。全 `.text` 掃描顯示兩者各只有一個讀取點，
都在 renderer 內：`14009F422`（gfx mode）與 `14009FA8D`（pixel mode）。

- gfx mode 查三張表得 tile region（layer0 `{2,1,0,1,0,0,0,0}`、layer1 `{2,1,1,1,2,2,2,2}`、
  layer2 恆為 2），再由 `{8,4,2,1,1,0,0,0}` 換成 bpp——與 MAME `get_tilemap_region()` 一致。
- pixel mode 只在 ROZ 區塊被讀：`v263 = (ROZ region == 0) && (pixel_mode == 8)`，其中
  ROZ region 由 `(roz_mode & 3)` 經 `{4,2,1,0}` 取得（即 `{1bpp,2bpp,4bpp,8bpp}`）。
  該旗標為真時，ROZ 整層改成**線性 bitmap**：跳過 tilemap 與 tile 圖形，直接以
  `addr = ((8 × (x + width × y)) >> 3 + 4 × $F00196) & 0x1FFFF` 逐像素讀 VRAM，
  palette bank 取自 ROZ tile mode（raw `$182`）低 4 bit，像素值 0 為透明；
  逐行參數則改走 24-bit 取值且**不加**全域 ROZ scroll 基底。

完整指令佐證見 `../acan/docs/f003-video-mode.md` §7。

## 本專案的現況

- `machine.Bus` 預設 `soundRAMMask = 0xffff`，符合上表契約。
  `SetSoundRAMAlias(true)` 與 headless `--sound-ram-alias` 只作為 32 KiB 假說的診斷開關
  保留，附帶上下半區對撞偵測；預設關閉，不影響任何驗證路徑。
- `chip/umc6618` 保存 `pixelMode = value & 0x1f` 並在 register `$1F0` 讀回；
  `tilemapRegion()` 以 `pixelMode & 7` 查表，`rozBitmapPixel()` 實作 bit 3 的 bitmap 路徑。

## ROZ 的 pixel-mode bit 3 路徑：已實作並逐像素對過

條件為 `(reg$1F0 & 0x18) == 0x08` 且 `(roz_mode & 3) == 3`（ROZ 為 8bpp）。成立時
`rozPixel()` 轉呼叫 `rozBitmapPixel()`：

```text
bit   = 8 × (x + 8 × 版面 tile 欄數 × y)
addr  = ((bit >> 3) + 4 × registers[0xcb]) & (VRAMSize - 1)
pixel = VRAM[addr] + (registers[0xc1] & 0x0f) << 8
```

流通 ROM 走不到這條路徑——八款各 1200 幀（The Son of Evil 另延長到 6000 幀）中三個條件
從未同時成立，bit 3 只出現在共用的開機 logo 段落而該段 ROZ 是 1bpp。驗證因此改用知識庫
的自製卡帶 `acan/homebrew/bit3probe/`：它讓 ROZ 停在 8bpp region 並每 300 幀切換
`$F001F0`，兩個相位的畫面與 Bcan 0.0.8b 的 F8 截圖**逐像素相同**（兩張 SHA-256 皆一致，
相異像素 0／76800）。

原理、指令位址與基底倍率的對照實驗見 `../acan/docs/f003-video-mode.md` §7.3、§7.6。
