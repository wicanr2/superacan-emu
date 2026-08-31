# 手把輸入契約

Super A'Can 手把以 16 位元 active-low 狀態保存。bit 15 到 bit 4 依序為
`A B Start Select Up Down Left Right X Y L R`；其餘位元維持 1。

目前兩條硬體路徑共用同一份 P1／P2 狀態：

- 68000 direct mode 從 `$E80200`／`$E80202` 讀取，回傳內部 active-low 狀態的
  反相值，因此沒有按鍵時為 `$0000`。
- W65C02 以 `$0407` 的下降沿控制 latch、shift 與 clear，從 `$0402`／`$0403`
  讀取移位結果，採 MSB first。clear probe 會依既有驅動證據觸發 bit 3／bit 2
  latch IRQ；這一觸發條件仍標為功能推論。
- `$E80404/$E80405` 是 68000 到 W65C02 的 byte latch，讀 `$0404/$0405` 分別
  確認 bit 3／bit 2；空 latch 回 `$CD`。
- 68000 寫 `$E9000A` 觸發聲音命令 IRQ bit 5，W65C02 讀 `$040A` 專屬確認。
- W65C02 寫 `$040A` 觸發 68000 IRQ6，CPU acknowledge 後解除。

headless 使用 `--press frame:BUTTON+BUTTON,...` 與 `--press2` 注入 P1／P2，每次
按住十幀。名稱不分大小寫，未知名稱或錯誤格式會失敗即關閉。Ebitengine 鍵盤
配接仍屬前端整合工作，不作為本文件所列核心契約的完成證據。
