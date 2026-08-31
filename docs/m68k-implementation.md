# 純 Go Motorola 68000 實作紀錄

更新日期：2026-08-31

## 來源與證據邊界

主要 ISA 與 timing 來源：

- NXP／Motorola, *M68000 Family Programmer's Reference Manual*：
  <https://www.nxp.com/docs/en/reference-manual/M68000PRM.pdf>
- NXP／Motorola, *MC68000 8-/16-/32-bit Microprocessors User's Manual*：
  <https://www.nxp.com/docs/en/reference-manual/MC68000UM.pdf>

官方 Programmer's Reference Manual 定義 opcode、operation、condition code 與 branch
displacement；User's Manual 表 8-9 定義 MC68000 Bcc／BRA timing 與 bus read/write 次數。
Moira `a4c273b08e07d82c73289ac032867c845969a0f2` 與 `archive/cpp/` 只作 sample／差分
oracle，不是本 Go 實作的程式來源，也不自動等同硬體。

## 目前 vertical slice

| 功能 | 狀態 | 證據等級／限制 |
|---|---|---|
| decoder | 啟動路徑所需 opcode 採明確 encoding 分流，未知值失敗即關閉 | ISA-spec；尚非完整 ISA matrix |
| condition code | 16 種條件 exhaustive boolean tests | ISA-spec |
| MOVEQ | sign extension、Dn、N/Z/V/C、X 保留 | ISA-spec；4-cycle prefetch phase |
| BRA.b／BRA.w | PC+2 base、signed displacement、queue refill | ISA-spec；User's Manual 10 cycles／2 reads |
| Bcc.b | taken 10 cycles；not taken 8 cycles | User's Manual 表 8-9 |
| Bcc.w | taken 10 cycles；not taken 12 cycles | User's Manual 表 8-9 |
| extension-word stream | 逐字消耗 IRC 並以 instruction-fetch phase 補入 queue | 內部設計契約 |
| MOVEA.L #imm,An | 32-bit immediate、An、CCR 不變、三次 fetch／12 cycles | ISA-spec／User's Manual |
| JSR (xxx).W | sign-extended target、PC+4 push、queue refill、18 cycles（2R/2W） | ISA-spec／User's Manual；Moira 僅核對 phase 次序 |
| MOVE.W (xxx).L,Dn | 絕對長位址 word read、Dn 低 word、N/Z/V/C | ISA-spec；16 cycles |
| MOVE.W Dn,(xxx).L | 絕對長位址 word write、CCR 不變 | ISA-spec；16 cycles；Moira 核對 write-before-final-prefetch |
| ANDI.W #imm,Dn | Dn 低 word、上半 word 與 X 保留、N/Z/V/C | ISA-spec；8 cycles |
| IPL 合成路徑 | `$400` 至第一個 poll branch target `$430` | BIOS bytes-derived opcode 序列；不嵌入 BIOS 映像 |
| MOVE.W #imm,Dn／Dn,Dn | 低 word 寫入、N/Z/V/C、X 與未覆蓋高 word 保留 | ISA-spec；8／4 cycles |
| MOVE.B Dn,(An) | byte data write 與 MOVE flags | ISA-spec；8 cycles |
| MOVE.B (An),(An)+ | byte read/write、A7 byte alignment 特例、post-increment | ISA-spec；12 cycles |
| MOVE.W #imm,(xxx).L | 三個 extension words、data write、final prefetch | ISA-spec；20 cycles |
| CMPI.W #imm,Dn | subtraction N/Z/V/C、X 與 operand 保留 | ISA-spec；8 cycles |
| DBcc | condition true 12、branch 10、counter expired 14 cycles | User's Manual execution-time table |
| IPL RAM backup loop | 32 次 `$5F…$40` address-port/data-port/Work RAM transaction | BIOS bytes-derived 合成回歸 |
| CMP.B (An),Dn | byte subtraction flags、operands 不變 | ISA-spec；8 cycles |
| MOVE.B -(An),(An) | predecrement、byte read/write、MOVE flags | ISA-spec；14 cycles |
| 真實 IPL | fixed SHA-256，自 `$400` 完成至 `$620` 並進入卡帶 | 87,204 指令；雙 overlay 關閉 |
| checksum ISA | CLR、ADD/SUB/ADDX/ADDQ/SUBQ、CMPM、MULS、BTST、NEG、SWAP | 真實 UMC6650 與卡帶授權兩階段通過 |
| control transfer | JMP/JSR absolute long、JMP (An)、MOVEA absolute word | 預取跨 high-overlay regression |
| stack／subroutine | BSR、RTS、MOVEM.L predecrement／postincrement、PEA、立即數／An long push | ISA-spec；合成堆疊 round-trip 回歸 |
| indexed／PC-relative | 68000 brief extension 的 Dn／An、word／long index；JSR／JMP／LEA／MOVE／MOVEA | ISA-spec；拒絕混入 68020 full extension／scale |
| 真實卡帶路徑 | Boom Zoo 由 IPL `$400` 無錯執行 200,000 條，PC 到 `$FF80A0` | software-observed；不代表完整 ISA 或遊戲可玩 |
| phase trace | `StepResult.Phases` | 只含目前已建模 phase，尚無 exception trace |

在 MC68000 上，opcode low byte `$FF` 仍是 8-bit displacement `-1`；32-bit branch
displacement 是後續 CPU 型號能力，本核心目前不得套用。

## 尚未完成

- 未被目前路徑涵蓋的 JSR／effective-address 模式、exception、interrupt acknowledge 與
  bus/address error。
- 一般化 effective-address decoder 與統一的 byte／word／long operand helpers；目前只為
  已觀察路徑組合定址模式，但所有 long read／write 都維持兩次有序 word transaction。
- user／supervisor function code 動態選擇。
- 真實 BIOS／ROM 只由外部路徑載入，版控內仍只保留合成 fixture；headless 探測已驗證
  IPL SHA-256 `2e4d88bec69b5e7e4803368c233ce0d20f6dd107c5af0cfcc0089d310c695d7c`。
- Motorola reset phase 的更細 bus timing 審查；目前 40-cycle reset 是 sample-derived
  起始契約，文件中不得標成硬體已證實。
- 與獨立公開 opcode vectors 及 archived oracle 的自動差分 harness。
- DIVU register 成功路徑目前採 140-cycle worst-case；需依 MC68000 iterative timing
  演算法補齊 data-dependent cycle，不能把目前值標成精確 timing。
- 卡帶入口 `$2B22 MOVEM.L` 已通過；目前 200,000 指令上限內沒有未知 opcode，完整
  ISA、exception 與 IRQ 仍未完成。
