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
| CMPI.W #imm,(xxx).L | extension cursor、絕對長位址 word read、比較 flags | ISA-spec；20 cycles；Monopoly `$002416` 動態命中 |
| DBcc | condition true 12、branch 10、counter expired 14 cycles | User's Manual execution-time table |
| IPL RAM backup loop | 32 次 `$5F…$40` address-port/data-port/Work RAM transaction | BIOS bytes-derived 合成回歸 |
| CMP.B (An),Dn | byte subtraction flags、operands 不變 | ISA-spec；8 cycles |
| MOVE.B -(An),(An) | predecrement、byte read/write、MOVE flags | ISA-spec；14 cycles |
| 真實 IPL | fixed SHA-256，自 `$400` 完成至 `$620` 並進入卡帶 | 87,204 指令；雙 overlay 關閉 |
| checksum ISA | CLR、ADD/SUB/ADDX/ADDQ/SUBQ、CMPM、MULS、BTST、NEG、SWAP | 真實 UMC6650 與卡帶授權兩階段通過 |
| control transfer | JMP/JSR absolute long、JMP (An)、MOVEA absolute word | 預取跨 high-overlay regression |
| stack／subroutine | BSR、RTS、MOVEM.L predecrement／postincrement、PEA、立即數／An long push | ISA-spec；合成堆疊 round-trip 回歸 |
| interrupt／return | level 1–7 autovector、44-cycle supervisor frame、RTE、IRQ acknowledge callback | 合成 phase／stack round-trip；Boom Zoo 實際受理 58 次 IRQ7 |
| indexed／PC-relative | 68000 brief extension 的 Dn／An、word／long index；JSR／JMP／LEA／MOVE／MOVEA | ISA-spec；拒絕混入 68020 full extension／scale |
| 真實卡帶路徑 | Boom Zoo 由 IPL `$400` 無錯執行 200,000 條，PC 到 `$FF80A0` | software-observed；不代表完整 ISA 或遊戲可玩 |
| phase trace | `StepResult.Phases` | 含 interrupt acknowledge；一般 exception 尚未建模 |
| Speedy Dragon DMA 後續路徑 | `MOVE.W (An)+,Dn`、`MOVE #imm,SR`、`MOVE.L An/#imm,(xxx).L`、`ADDA.L #imm/Dn,An`、`ORI.W #imm,(xxx).L`、`MOVE.B Dn,(xxx).L`、`TST.B (An)` | ISA-spec 與 Moira phase sample；真實 ROM 動態命中，尚持續補齊後續指令 |
| Speedy Dragon IRQ7 後續路徑 | `TST.B (xxx).L`、`EOR.W Dn,Dn`、`NOT.W Dn`、`AND.W Dn,Dn`、`MOVE.B #imm,Dn` | ISA-spec；真實 ROM 已由 307 萬推進至 328 萬條指令並受理 13 次 IRQ7 |
| Speedy Dragon 圖形資料載入路徑 | `ANDI.L #imm,Dn`、`MOVE.L #imm,Dn`、`OR.L Dn,Dn`、`MULU.W #imm,Dn`、`ADDQ.L #n,An`、`MOVE.W (An)+,(Am)/(d16,Am)`、`MOVE.W #imm,(An)` | ISA-spec；真實 ROM 已推進至 336 萬條指令，受理 18 次 IRQ7 且 VRAM 非零達 8,961 bytes |
| Speedy Dragon 圖形暫存器設定 | `MOVE.W/L #imm,(d16,An)`、`SUBQ.B #n,(xxx).L`、`MOVEA.L (xxx).L,An`、`MOVE.B (d16,An),(xxx).L`、`CMPI.L #imm,(An)`、`MOVE.W (d16,An),Dn` | ISA-spec；真實 ROM `$05EAxx-$05EBxx` 動態命中，VRAM 非零維持 8,961 bytes，framebuffer 尚黑 |
| Speedy Dragon 圖形表格更新 | `MOVE.B (d16,An),Dn`、`ADD.W Dn,(xxx).L`、`ADDQ.L #n,(xxx).L` | ISA-spec；真實 ROM `$05EA7C-$05EA92` 動態命中，long RMW 保持 high-word→low-word bus 次序 |
| Speedy Dragon 狀態檢查路徑 | `BTST #imm,Dn`、`MOVE.B (xxx).L,Dn`、`CMP.B/W (xxx).L,Dn` | ISA-spec；真實 ROM 已回到 `$002Dxx` 主程式並繼續執行，framebuffer 尚黑 |
| Speedy Dragon 主程式資料路徑 | `ADD.W Dn,Dn`、`CMP.L Dn,Dn`、`MOVEA.L (d16,An),Am`、`MOVE.B (An),Dn` | ISA-spec；真實 ROM 已推進至 337 萬條指令並受理第 19 次 IRQ7 |
| Speedy Dragon 位元資料路徑 | `LSL.W Dn,Dn`、`EOR.B Dn,Dn` | ISA-spec；register count 採低 6 bits，count=0 保留 X 並清 C；真實 ROM `$0031AC-$0031CC` 動態命中 |
| Speedy Dragon 長時間執行路徑 | `LSR.B #n,Dn`、`OR.B Dn,Dn`、`ADDI.L #imm,(xxx).L`、`MOVE.L (xxx).L,(xxx).L` | ISA-spec；真實 ROM 已推進至 936 萬條指令、617 幀，DMA ch1 64 次、IRQ7 397 次 |
| Speedy Dragon 長整數表格路徑 | `SUBI.L #imm,(xxx).L` | ISA-spec；36-cycle absolute-long RMW，真實 ROM `$05D052` 動態命中 |
| Speedy Dragon 1,200 幀回歸 | 無未知 opcode 完成 18,515,145 條指令；DMA ch0/ch1 3/96 次、IRQ7/5/3 979/583/17 次 | software-observed；framebuffer 33,125 個非黑像素，SHA-256 `c49af07407d6de2f32894ac6fc6f646e9baf6bc0f560e7f61da19d6c42c07794`，視覺確認為可辨識的飛龍與道路場景 |
| 跨 ROM 長時間路徑 | `CLR.L (xxx).L`、`LSR.L #n,Dn`、`MOVEA.L (d8,An,Xn),Am`、`ANDI.W #imm,(xxx).L`、`TST.L (An)`、`MOVE.L (xxx).L,Dn`、`MOVE.L Dn,Dn`、`MOVE.L An,Dn` | Motorola ISA／Moira phase 模型；Boom Zoo 與 Formosa Duel 均推進逾 310 萬條指令、約 224／228 幀，下一個未知 opcode 分別為 `$200E`／`$2E01`，後兩者已補入並通過合成測試，待下一輪真實 ROM 回歸 |
| 跨 ROM 初始化／圖形載入 | `MOVE.L An,(Am)/(d16,Am)`、`ADDQ.L #n,Dn`、`DIVU.W #imm,Dn`、`CLR.L (d16,An)`、`JSR (An)`、`ADD.W Dn,(An)+`、`MOVEA.W (xxx).L,An`、`MOVE.L Dn,(An)+`、`MOVE.W An,Dn` | Motorola ISA／Moira phase 模型；Boom Zoo 推進至 3,687,106 條／254 幀且 VRAM 非零 17,582 bytes，Formosa Duel 推進至 3,588,237 條／250 幀且 VRAM 非零 8,975 bytes；最後兩條已通過合成測試，待下一輪真實 ROM 回歸 |
| 跨 ROM 指標／算術路徑 | `MOVEA.L (An),Am`、`MOVEA.W (An)+,Am`、`MOVEA.W (An),Am`、`ADDQ.W #n,(d16,An)`、`CMPI.W #imm,(d16,An)`、`ADD.W (xxx).L,Dn`、`OR.L (An),Dn`、`DIVU.W (xxx).L,Dn` | Motorola ISA／Moira phase 模型；Boom Zoo 已推進至 3,701,358 條／255 幀，Formosa Duel 至 3,601,242 條／250 幀並受理 57 次 IRQ5；最後兩條已通過合成測試，待下一輪真實 ROM 回歸 |
| 跨 ROM 中期初始化 | `OR.L (d16,An),Dn`、`MOVE.W (An),(d16,Am)`、`SUB.W (d16,An),Dn`、`MOVEM.W <list>,-(An)`、`CMP.W (d16,An),Dn`、`MOVEA.W Dn,An` | Motorola ISA／Moira phase 模型；Boom Zoo 推進至 4,219,269 條／291 幀，VRAM 非零 23,558 bytes；Formosa Duel 推進至 9,700,346 條／641 幀，DMA ch1 48 次、IRQ5 839 次、VRAM 非零 20,594 bytes；最後兩條已通過合成測試，待下一輪真實 ROM 回歸 |
| 跨 ROM 堆疊／批次恢復 | `DIVU.W (d16,An),Dn`、`ADD.W An,Dn`、`MOVE.W (xxx).L,(xxx).L`、`MOVEM.W (An)+,<list>`、`OR.W (xxx).L,Dn`、`MOVEA.L Dn,An` | Motorola ISA／Moira phase 模型；Formosa Duel 的 word MOVEM 返回路徑已推進至 9,878,357 條／651 幀，IRQ5 859 次、VRAM 非零 20,796 bytes；最後兩條已通過合成測試，待下一輪真實 ROM 回歸 |
| 跨 ROM 畫面啟動／狀態檢查 | `CMP.W (An),Dn`、`TST.B (d16,An)`、`CMP.B (d16,An),Dn`、`TST.W (d16,An)` | Motorola ISA／Moira phase 模型；Boom Zoo 推進至 5,194,705 條／350 幀，framebuffer 23,752 個非黑像素且音訊非零樣本 102,101；Formosa Duel 推進至 10,135,499 條／666 幀，VRAM 非零 41,358 bytes；最後兩條已通過合成測試，待下一輪真實 ROM 回歸 |
| 跨 ROM byte 狀態寫入 | `MOVE.B Dn,(d16,An)`、`MOVE.B #imm,(An)` | Motorola ISA／Moira phase 模型；兩條均通過合成 bus write 與旗標測試，待下一輪真實 ROM 回歸。Boom Zoo 320 幀 PNG 已人工確認為暗色天空、月體與右側前景剪影的可辨識過場構圖，不是隨機像素；調色忠實度仍待 oracle 比對 |
| 跨 ROM bit／結構搬移 | `BTST #imm,(d16,An)`、`MOVE.L (An)+,(d16,Am)`、`ADD.W Dn,(d16,An)` | Motorola ISA／Moira phase 模型；BTST 記憶體 bit 採 modulo 8 且僅改 Z，已由 Boom Zoo／Formosa Duel 同時動態命中；後兩條通過 long bus order 與 read-modify-write 合成測試，待下一輪真實 ROM 回歸 |
| 跨 ROM 記憶體讀改寫 | `OR.W Dn,(An)`、`CLR.B (d16,An)` | Motorola ISA／Moira phase 模型；word OR 與 byte CLR 均保留 read-before-write，CLR 的 phase 測試明確驗證 extension fetch、byte data read、byte data write、final prefetch 次序；待下一輪真實 ROM 回歸 |
| 跨 ROM displacement 圖形路徑 | `OR.W Dn,(d16,An)`、`SUBQ.W #n,(d16,An)`、`MOVE.W (d8,An,Xn),(d16,Am)`、`CLR.W (d16,An)` | Motorola ISA／Moira phase 模型；前兩條已讓 Formosa Duel 推進至 14,136,451 條／900 幀，DMA ch1 59 次、IRQ5 1,357 次、framebuffer 56,538 個非黑像素；後兩條通過 indexed 22-cycle 與 word RMW 合成測試，待下一輪真實 ROM 回歸 |
| Formosa Duel 1,200 幀回歸 | `EXT.W Dn`、`MOVE.B #imm,(d16,An)`；後續無未知 opcode | software-observed；完成 19,272,069 條指令，DMA ch0/ch1 5/59 次、IRQ7/5 978/1,955 次、VRAM 非零 41,974 bytes、framebuffer 76,800 個非黑像素，SHA-256 `5e5e2f585abfa42790a9b36302ba729319cf469e5a2e01e1f02079aec7363477`；PNG 人工確認為可辨識的遊戲標題／開始畫面 |
| Boom Zoo long immediate 結構寫入 | `MOVE.L #imm,(An)` | Motorola ISA／Moira phase 模型；20-cycle、high-word→low-word bus order 與 NZVC 測試通過，待下一輪真實 ROM 回歸 |
| Boom Zoo 後段結構搬移 | `SUBQ.W #n,(xxx).L`、`MOVE.L (d16,An),(d16,Am)` | Motorola ISA／Moira phase 模型；前者已讓 Boom Zoo 推進至 8,016,687 條／530 幀，DMA ch0 250 次、framebuffer 23,752 個非黑像素；後者通過雙 extension、long read/write word order 與 28-cycle 合成測試，待下一輪真實 ROM 回歸 |

在 MC68000 上，opcode low byte `$FF` 仍是 8-bit displacement `-1`；32-bit branch
displacement 是後續 CPU 型號能力，本核心目前不得套用。

## 尚未完成

- 未被目前路徑涵蓋的 JSR／effective-address 模式、一般 exception、bus/address error，
  以及 user-mode interrupt 的 USP／SSP 切換。
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
- 卡帶入口 `$2B22 MOVEM.L` 已通過；目前 1,300,000 指令上限內沒有未知 opcode，完整
  ISA 與一般 exception 仍未完成。IRQ4／5 尚無真實軟體 acknowledge 證據。
