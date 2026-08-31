# 純 Go W65C02 實作紀錄

更新日期：2026-08-31

## 契約與來源

- 型號為 WDC 65C02，時脈 3.579545 MHz；Super A'Can 68000 為 10.738635 MHz，固定
  比例 3:1。硬體證據見唯讀知識庫 `../acan/docs/memory-map.md`。
- reset 在 `$E9001C` bit0 為 0 時保持 asserted；0→1 才讀 little-endian
  `$FFFC/$FFFD`。卡帶上傳的 sound RAM 與 65C02 完整 64 KiB 空間同體。
- ISA 依 W65C02S 契約獨立實作；deprecated CLK wrapper 只作差分 oracle，不移植程式碼。

## 目前已驗證

- 7-cycle reset、scheduler-before-bus、PC／A／X／Y／SP／P state。
- Boom Zoo boot 路徑所需的 immediate／zero-page／absolute／indexed load/store、transfer、
  stack、JSR／RTS／JMP、relative branch、compare、INC／DEC、flag 與 NOP 子集。
- machine timeline 每累積 3 個 68000 cycle 提供 1 個 65C02 cycle credit；reset release、
  vector、第一條 `SEI` 與共享 RAM 有整合測試。
- 真實 driver 從 `$F000` 起跑，完成 UM6619 register 初始化並寫 sound RAM
  `$0300=$FF`；68000 第 395,493 條指令的 `CMPI.B` 首次讀取即看到 ack。

## 尚未完成

- 完整 256 opcode matrix、decimal ADC／SBC、bit branch、indirect modes、BRK／RTI、
  IRQ／NMI／WAI／STP 與所有 page-cross timing。
- 現在以完整指令為單位消耗 3:1 credit，確定性已建立，但 instruction boundary 可能有
  最多一條 65C02 指令的相位誤差；DMA／IRQ 前須收斂到逐 cycle 可驗證排程。
- I/O page 目前只建模 boot 路徑需要的 `$0404/$0405/$0410/$0411/$0420/$0422`；
  mailbox、手把 shift、IRQ source／ack 仍須逐項接入。
