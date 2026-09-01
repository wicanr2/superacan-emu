# FRC／IRQ3 計時器

FRC 是 68000 主機域的獨立裝置，control 位於 `$E90014`、frequency 位於
`$E90016`，兩者皆為 16 位元 big-endian bus transaction。裝置以實際 68000 cycle
推進，IRQ3 採 HOLD_LINE：到期後維持 pending，直到 CPU interrupt acknowledge；
acknowledge 後才由該邊界重新開始完整週期。

目前週期規則固定標示為 **MAME-derived／unknown-hardware**：

- 只有 `(control & $ff00) == $a200` 啟用。
- period 取 `frequency` 暫存器本身。MAME 的來源寫成
  `((m_frc_control & 0xff << 16) | m_frc_frequency)`，但依 C++ 運算子優先序等於
  `control & 0x00ff0000`，對 16 位元的 control 恆為 0；因此該固定版 oracle 的實際
  period 只有 frequency，其逐 case 的時間感（magipool `$a201`／`$0104` 等）也是照這個
  值校出來的。不採用該式面上看起來的 24 位組合。
- `control & $f == 0`：固定 10,738,635 個 68000 cycles（MAME 的 1 Hz HACK）。
- mode 1：`1024 * period` cycles。
- mode `$f`：`8192 * period` cycles。
- 其他 mode 失敗即關閉，不排程 IRQ；不以插值猜測未知公式。

單元測試涵蓋三個已知 case、未知 mode、byte／word 暫存器交易、共享時間線推進、
IRQ3 優先權、level-held pending 與 acknowledge 後重排程。headless 會輸出 control、
frequency、pending、supported 及 IRQ3 acknowledge 計數，避免未使用 FRC 的遊戲
回歸被誤稱為計時器軟體相容性證據。

Boom Zoo、Monopoly 與 Speedy Dragon 搭配完整 BIOS 的 120 幀回歸皆完成，三者
均回報 `frc=$0000/$0000,pending:false,supported:false`、IRQ3 acknowledge 為 0。

跑到 1200 幀後 Speedy Dragon 會實際使用它：該次回歸的 `irq_ack` 的 IRQ3 欄為 17。
ROM 端的用法也已由 `../acan` 的反組譯固定（見該庫 `docs/memory-map.md` §2.1）：

- Speedy Dragon `$30CE` `move.w d0,$E90016` 後 `$30D4` `move.w #$A200,$E90014`，是
  「以 d0 設週期並啟動」的常式；卡帶 IRQ3 向量指向 `$3454`，內容是
  `addq.b #$1,$FCE00E` + `rte`，而 `$30DE` 是 `tst.b $FCE00E` / `beq` 的等待迴圈。
- Formosa Duel `$5EB4/$5EBC` 寫 `$A000`／`$FFFF`，並把 `$E90018` 的讀值加到
  `$F00124/$F00126`（tilemap 1 scroll），另在 `$6076/$607E` 連讀兩次拼成 32-bit 種子。
- Journey to the Laugh `$FC6AC/$FC6B4` 寫週期 `$8FC`＋control `$A0D6`。

因此 `$E90014/16/18` 是同一顆計數器（控制／週期／目前值），到期拉 IRQ3；`$E90018`
必須回報一個持續變動的計數值，遊戲會把它當亂數與捲動來源。真實週期公式仍未知，
Speedy 的「設週期→等 N 個 tick」是目前最好的校準入口。
