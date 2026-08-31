# 主機 DMA 實作紀錄

更新日期：2026-09-01

## 軟體可見契約

Super A'Can 主機 DMA 位於 `$E90020-$E9003F`，分成兩個通道；每個通道有
source、destination、count、control 四類 16-bit 暫存器。Go 實作將 DMA 保持為獨立
晶片元件，由 SystemBus 只負責位址分派及 8/16-bit 原子交易。

- count 採 `count+1` 次交易。
- control 的 `$8800` 位元會觸發傳輸；16-bit 寫入只能觸發一次，byte 寫入則等低位元組
  抵達後才判斷，避免把同一個 CPU word transaction 錯拆成兩次 DMA。
- bit 12 選 word 傳輸；bit 10／9 分別控制 destination／source 遞減；bit 8 是目的位址
  16-byte 間接回捲模式。
- control `$A800` 的特殊 byte／fill 路徑依固定 MAME `supracan_state::dma_w` 行為建模。
- 所有位址在交易時限制為 24-bit；來源與目的暫存器會隨實際傳輸更新。

## 證據等級與限制

暫存器配置及模式目前是成熟模擬器來源（MAME-derived）契約，不宣稱為晶片實測。
合成測試覆蓋 word `count+1`、control byte 原子觸發、間接目的位址回捲，以及經真實
SystemBus 從卡帶 ROM 搬至 Work RAM。

Speedy Dragon（ROM SHA-256
`dfba00a46e7d71b9d78688bd902ec05e2c353f2ff119273d47b0a02602f3c9a2`）的實際開機路徑
已在約 306 萬條 68000 指令處觸發 ch0 一次，並越過聲音 driver 上傳常式。這只證明
該軟體路徑消費目前契約，不把未知模式或時序升格為已證實。

## 尚未完成

- 現行觸發會立即完成整批 bus burst；DMA bus ownership、CPU wait state、完成事件及逐週期
  仲裁仍未知，之後須改成可排程的分段狀態機。
- Super Taiwanese Baseball League 所用的額外 DMA type、所有 control 組合及 VRAM 第 17
  位來源尚未覆蓋。
- save state 尚未保存 DMA 中途進度；在可排程模型完成前，不得宣稱 cycle-exact。
