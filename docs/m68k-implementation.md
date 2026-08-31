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
| decoder | NOP、MOVEQ、BRA、BSR 分流、Bcc | ISA-spec；BSR 尚未執行 |
| condition code | 16 種條件 exhaustive boolean tests | ISA-spec |
| MOVEQ | sign extension、Dn、N/Z/V/C、X 保留 | ISA-spec；4-cycle prefetch phase |
| BRA.b／BRA.w | PC+2 base、signed displacement、queue refill | ISA-spec；User's Manual 10 cycles／2 reads |
| Bcc.b | taken 10 cycles；not taken 8 cycles | User's Manual 表 8-9 |
| Bcc.w | taken 10 cycles；not taken 12 cycles | User's Manual 表 8-9 |
| phase trace | `StepResult.Phases` | 只含目前已建模 phase，尚無 exception trace |

在 MC68000 上，opcode low byte `$FF` 仍是 8-bit displacement `-1`；32-bit branch
displacement 是後續 CPU 型號能力，本核心目前不得套用。

## 尚未完成

- BSR stack write、exception、interrupt acknowledge 與 bus/address error。
- effective-address decoder、extension-word cursor、byte／word／long operand helpers。
- user／supervisor function code 動態選擇。
- Motorola reset phase 的更細 bus timing 審查；目前 40-cycle reset 是 sample-derived
  起始契約，文件中不得標成硬體已證實。
- 與獨立公開 opcode vectors 及 archived oracle 的自動差分 harness。
