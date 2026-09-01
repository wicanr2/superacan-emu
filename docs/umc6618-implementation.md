# 純 Go UM6618 實作紀錄

更新日期：2026-09-01

## 已建立的裝置契約

- 68000 register window `$F00000–$F001FF`、xBGR-555 palette
  `$F00200–$F003FF`、CPU-visible 128 KiB VRAM `$F40000–$F5FFFF` 分別由獨立
  `chip/umc6618.Device` 保存。
- register／palette／VRAM 的 16-bit access 是單一 device transaction；不能由 system bus
  拆成兩次 byte write。`$F0001E` sprite DMA control 有 trigger counter 回歸，證明一次 word
  write 只觸發一次。
- register 0 read 回報 vblank 區與 frame parity，並 ack vblank pending；register 1 回報
  scanline，register 4 與 `$1F0` 分別讀回 video flags 與遮罩後的 pixel／gfx mode。
- video clock 由 machine Timeline 的 68000 cycle 推進，不依賴 Ebitengine frame callback。
  目前採 software-observed 256-wide 684 cycles／line、320-wide 728 cycles／line、262 lines／frame。
  這兩個值繼承固定版 MAME 的 `htotal 455／divider 8`（320）與 `342／10`（256），因此 320 模式
  的幀率是 56.3 Hz、256 模式是 59.96 Hz。真實硬體不太可能在切換水平模式時改變行掃描率，
  但目前沒有實機量測可據以修正，故保留 MAME-derived 值並記錄此疑點。

## 真實 Boom Zoo 證據

- VRAM `$F44400` 起寫入後，裝置內有 5,587 個非零 byte，固定 SHA-256
  `53bf5ec61770b36b55e67c3f5a4eebdacde099d2218f1da890728c5272ee81d2`。
- palette 自第 398,713 條 68000 指令開始寫入；UM6618 register 隨後設定 sprite、ROZ、
  window、pixel mode `$0009` 與 video flags `$120E`。
- 接入 scanline 後，原本在 `$FFDBB0` 輪詢 `$F00000` 的迴圈會自然離開；沒有依 PC 或
  read count 注入 vblank。1,300,000 條 68000 指令的有界執行已完成 88 個 vblank，
  VRAM 有 7,268 個非零 byte，SHA-256 `b0b2d6…0f255`。

## 第一版 framebuffer

- `RenderFrame` 由 UM6618 狀態直接合成 320×240 ARGB framebuffer，已涵蓋三層 tilemap、
  sprite 與 mask、window、ROZ、優先度及 256／320 顯示寬度；vblank 起點觸發合成，
  不由 Ebitengine callback 推進時間。
- 合成測試以 window 邊界確認 xBGR-555 轉換、256 像素寬之外的 blanking，以及固定非黑
  像素數；另測 8／4／2bpp packed tile 解碼。
- 固定 IPL `2e4d88…c695d7c` 與 Boom Zoo `090827…370077` 執行 1,300,000 條 68000
  指令後，frame 88 的 framebuffer 有 61,437 個非黑像素，SHA-256
  `89ce08232bcfc61c396b514a981057b69ae7cf19733a4c3a247a051fc64684ee`。
- 此 hash 只作 Go 路徑的決定性回歸。尚未取得相同硬體狀態的 archived C++／實機畫面
  對照，因此不能標為像素正確或硬體已證實。
- 2026-09-01 勘誤：接入三張 ROZ 逐行表後，同一 frame 的非黑像素仍為 61,437，hash
  改為 `14449f1ba85c25a01b0466fa2b8b735b4dcef571c44a808faf75ac37f894a232`。
  這證明舊 hash 略過了會影響該狀態的表資料；保留舊值只作變更來源追溯。

### ROZ 逐行表（MAME-derived）

- 當 mode bit 9 為 0 且 priority nibble 非 0，每條輸出線讀 `$198` 的 incxx delta；值為
  0 時整行不畫，非 0 時以 signed 16-bit 加到 coefficient A。
- `$19A`／`$19E` 各提供每行 32-bit scrollx／scrolly delta，與全域 24.8 scroll 相加；
  表位址 register 依 `<<2` byte address 契約換成 VRAM word index。
- 合成測試覆蓋 zero-line suppression、三表位址、signed/wrapping 加法與 mode bit 9 bypass。
  此分支源於 MAME 自標 HACK 的行為，尚未經實機證實。

## Sprite DMA

- register `$08–$0F` 保存 count、32-bit destination／source、word stride 與 control；
  control bit 15 觸發同步 `count+1` 筆 16-bit bus transaction。
- control bit 8 執行零填充，bit 13／14 將目的位址置入 `$F40000` VRAM window。每筆
  read／write 都走 machine bus callback，因此 observer 可按實際順序看見 DMA 交易。
- 合成測試涵蓋兩 word copy、來源／目的 stride 與零填充。Boom Zoo 1,300,000 指令 smoke
  的 VRAM／framebuffer hash 不變，表示此有界路徑沒有產生改變既有狀態的 DMA；不能據此
  宣稱所有 control mode 已由遊戲驗證。

## 掃描線中斷

- `$E90010` bit 7／4 分別允許 vblank IRQ7 與可視線 raster IRQ4；`$F0000A／0C`
  bit 15 加低 8-bit 線號控制 IRQ5 line-on／line-off。
- 68000 在指令邊界選擇最高 level，執行 autovector acknowledge 後以 HOLD_LINE 語意清除
  對應來源。IRQ7 保留 rising-edge latch，即使 SR mask=7 仍可受理。
- 固定 Boom Zoo 1,300,000 指令回歸實際 acknowledge IRQ7 58 次；IRQ4／5 為 0，因此
  兩者目前只由合成掃描線測試證明，不能升格為該遊戲路徑的動態證據。

## 尚未完成

- ROZ 逐行模式的實機／同狀態 oracle 驗證、mid-frame register write、partial update 與
  VRAM 上半部來源。
- save state 與相同 frame renderer 差分；目前的 framebuffer hash 只證明固定 Go 路徑。
