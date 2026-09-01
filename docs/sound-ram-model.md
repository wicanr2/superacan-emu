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
  該旗標為真時，ROZ 改走 24-bit 逐行取值、**不加**全域 ROZ scroll 基底，並在每個像素多做
  一次以 ROZ tile bank（raw `$196`）為基底、搭配 ROZ tile mode（raw `$182`）低 4 bit
  palette bank 的 VRAM 遮罩查表，未通過就跳過該像素。

完整指令佐證見 `../acan/docs/f003-video-mode.md` §7。

## 本專案的現況

- `machine.Bus` 預設 `soundRAMMask = 0xffff`，符合上表契約。
  `SetSoundRAMAlias(true)` 與 headless `--sound-ram-alias` 只作為 32 KiB 假說的診斷開關
  保留，附帶上下半區對撞偵測；預設關閉，不影響任何驗證路徑。
- `chip/umc6618` 保存 `pixelMode = value & 0x1f` 並在 register `$1F0` 讀回，renderer
  不使用 bit 3／bit 4，與上表一致。

## 待實作：ROZ 的 pixel-mode bit 3 路徑

`chip/umc6618` 目前保存 `pixelMode = value & 0x1f` 並在 `$1F0` 讀回，
`tilemapRegion()` 用 `pixelMode & 7` 查與 Bcan／MAME 相同的三張表——這部分已符合契約。
**尚未實作的是 bit 3 的 ROZ 分支**。要補的話依此順序：

1. 先在 headless 加一個純記錄探針，統計八款 ROM 執行期間 `(pixel_mode == 8) && ROZ 8bpp`
   同時成立的幀數。若沒有任何遊戲命中，這條路徑就不影響現有驗證集，可延後。
2. F003（The Son of Evil）寫 `$0009` 且使用 ROZ，是最可能命中的候選；先取它的
   Bcan 同畫面截圖與本專案輸出做差分，確認差異確實落在 ROZ 層。
3. 實作後標為 `confirmed-Bcan`，並記錄改前改後的 framebuffer hash。硬體是否如此仍未知，
   不得標成 `confirmed-hardware`。
