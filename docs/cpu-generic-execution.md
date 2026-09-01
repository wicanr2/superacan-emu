# CPU 一般化執行層

更新日期：2026-09-01

## 為什麼要換掉逐一 opcode 的結構

原本的 68000 decoder 為每一組「操作 × 大小 × 定址模式」寫一個 `Instruction` 常數與
一個處理函式，累積到 233 條。這個結構在垂直切片階段有效——每加一條就有一段可回查的
證據——但它沒有終點：真正的 MC68000 是 12 種定址模式乘上三種大小乘上數十個指令族。

八款商業 ROM 同時量測的結果讓這件事變得明確：每一款都停在不同的未實作編碼，而且橫跨
七個不同的指令族。

| ROM | 停止 opcode | 指令 |
|---|---|---|
| Boom Zoo | `$D06A` | `ADD.W (d16,A2),D0` |
| Formosa Duel | `$2233` | `MOVE.L (d8,A3,Xn),D1` |
| Journey to the Laugh | `$5278` | `ADDQ.W #1,(xxx).W` |
| Monopoly、Sango Fighter | `$40E7` | `MOVE SR,-(A7)` |
| Speedy Dragon | `$23D8` | `MOVE.L (A0)+,(xxx).L` |
| Super Taiwanese Baseball League | `$4E56` | `LINK A6,#d16` |
| The Son of Evil | `$4264` | `CLR.W -(A4)` |

七個族各補一次只會換來下一批七個。因此改成先解析 effective address，再由指令族共用
讀寫路徑。

## 結構

- `cpu/m68k/ea.go`：把 mode／register 欄位解析成 operand（資料暫存器、位址暫存器、
  記憶體、立即值），涵蓋全部 12 種定址模式含 PC 相對與立即值。`(An)+`／`-(An)` 的副作用
  在解析時套用一次，之後的讀寫不再改動位址暫存器。
- `cpu/m68k/generic.go`、`generic_misc.go`、`generic_shift.go`、`generic_bcd.go`：以此實作
  MOVE／MOVEA、立即值運算、位元操作、ADDQ／SUBQ／Scc、ADD／SUB／AND／OR／EOR／CMP
  與其位址暫存器形式、MULU／MULS／DIVU／DIVS、EXG、單運算元運算、TST／TAS、
  MOVE SR／CCR、SWAP／EXT、LEA／PEA、LINK／UNLK、JMP／JSR／RTR、MOVEM、
  ABCD／SBCD／NBCD、ADDX／SUBX 與全部位移旋轉。
- `cpu/m65c02/generic.go`、`generic_exec.go`：65C02 有同樣的結構問題，改以 256 項指令表
  加定址模式解析器覆蓋完整指令集，含 `(zp)`、`STZ`、`TSB`／`TRB`、
  `RMB`／`SMB`／`BBR`／`BBS`、`BRA` 與 `WAI`。

兩邊都只在既有逐一 case 判定為未知編碼時才進入，因此既有已釘住的行為與時序完全不變。
一般化層仍不認識的編碼照樣 fail-closed。

## 時間模型

沿用既有約定：每次 bus transaction 由讀寫函式自動計 4 個 cycle（65C02 為 1 個），
額外的內部 cycle 由指令族明確補上。這讓「PRM 表上的總時間」可以直接對帳：

```
內部 cycle = PRM 指令時間 − 4 × 實際發生的 bus transaction 數
```

例如 `MOVE.L (d8,An,Xn),Dn` 在 PRM 是 18(4/0)：4 次讀取（1 個延伸字、2 個資料字、
1 次 prefetch 補位）等於 16，加上 brief-indexed 的 2 個內部 cycle 剛好 18。

定址模式本身只負責兩個地方的內部時間：預減 2 個、brief-indexed 2 個。MOVE 的目的端
預減是例外，PRM 的 MOVE 表對 `-(An)` 目的沒有這 2 個 cycle，因為位址計算與寫入重疊。

證據等級：這些時間出自 M68000 Programmer's Reference Manual 的指令時間表，屬
`strong-inference`；尚未與 Moira 做逐指令差分，也沒有實機量測。

## 已定案的兩個勘誤

**位址暫存器是 32 位元。** 68000 的 `An` 保留完整 32 位元，24 位元遮罩只發生在位址
匯流排。先前 `(An)+`／`-(An)`、`ADDA`／`SUBA`、`ADDQ`／`SUBQ`、`LINK`／`UNLK`、`MOVEM`
與 brief-indexed 計算都在寫回暫存器時就截斷。Work RAM 位於 `$FC0000–$FFFFFF` 且四頁
互為鏡像，資料存取看不出差別，但與高位址比較的程式碼會永遠不相等：The Son of Evil 在
`$08729C` 的複製迴圈是 `MOVE.W (A4)+,(A5)+` 配 `CMPA.L #$FFFFA122,A5`，A5 被截成
`$00FFA122` 之後整台機器停在該迴圈，VRAM 一個位元組都沒寫入，畫面全黑卻不報錯。

**暫存器位移的長字時間是 8 + 2n。** PRM 的位移／旋轉表對長字是 8 + 2n，位元組與字才是
6 + 2n；先前長字沿用了後者。

## W65C02S 的未指派編碼

那顆晶片的未指派編碼不是非法指令，而是有明確長度與週期的 NOP，依分組為 1–3 個位元組、
1–8 個週期。把它們實作出來不是「把未知當 NOP 帶過」，而是照資料手冊的行為；`$DB`（STP）
仍然 fail-closed，因為停機需要外部 reset。
