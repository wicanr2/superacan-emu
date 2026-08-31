# 純 Go UM6618 實作紀錄

更新日期：2026-08-31

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

## 真實 Boom Zoo 證據

- VRAM `$F44400` 起寫入後，裝置內有 5,587 個非零 byte，固定 SHA-256
  `53bf5ec61770b36b55e67c3f5a4eebdacde099d2218f1da890728c5272ee81d2`。
- palette 自第 398,713 條 68000 指令開始寫入；UM6618 register 隨後設定 sprite、ROZ、
  window、pixel mode `$0009` 與 video flags `$120E`。
- 接入 scanline 後，原本在 `$FFDBB0` 輪詢 `$F00000` 的迴圈會自然離開；沒有依 PC 或
  read count 注入 vblank。1,300,000 條 68000 指令的有界執行已完成 88 個 vblank，
  VRAM 有 7,268 個非零 byte，SHA-256 `b0b2d6…0f255`。

## 尚未完成

- tilemap、sprite、window、ROZ renderer 與 RGB framebuffer。
- sprite DMA transaction state machine；目前只辨識控制 register 的 start edge，尚未複製。
- raster／line IRQ4／5、vblank IRQ7 到 68000 的受理與 acknowledge。
- mid-frame register write、partial update、VRAM 上半部來源與 1bpp swapped view。
- save state 與 renderer 差分；目前的 VRAM hash 只證明固定軟體路徑與儲存契約。
