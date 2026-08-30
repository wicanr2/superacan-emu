# 里程碑 1 驗證：68k IPL 開機流程

> 驗證日期：2026-08-30。規格出處：知識庫 `acan/docs/bios-68k.md`、
> `acan/docs/memory-map.md`、`acan/docs/bios-rom-format.md`。

## 環境

- `superacan-emu`（本 repo，`build/superacan-emu`），Moira `a4c273b`、CLK `096de57`
- BIOS：Bcan `bios/supracan.zip` + `umc6650.zip` 解壓至 `/tmp/acan_bios/`（版權檔，不入庫）
- 測試 ROM：Boom Zoo（512 KB，最小流通 ROM）＋ Monopoly（1 MB）交叉驗證

## Boom Zoo 執行 log（`--trace 12 --instructions 5000`）

```
[boot] reset：SSP=$00FD000A PC=$00000400（IPL 向量表）
[info] 卡帶向量表入口 PC=$00000412 SSP=$00FCFFFE
[trace] $00000400  nop
[trace] $00000402  jsr     $40a.w
[trace] $0000040A  movea.l #$eb0d03, A0
[trace] $00000410  movea.l #$eb0d01, A1
[trace] $00000416  movea.l #$fc0000, A2
[trace] $0000041C  move.w  $e90b3c.l, D0
[trace] $00000422  andi.w  #$1, D0
[trace] $00000426  beq     $430
[trace] $00000430  move.w  #$5f, D0
[trace] $00000434  move.w  D0, D1
[trace] $00000436  move.b  D0, (A0)
[trace] $00000438  move.b  (A1), (A2)+
[event] UMC6650 交握通過（IPL $55A：lockout 結果 $09/$0C 已寫出）
[event] 卡帶授權比對通過（IPL $5F4）
[event] IPL 轉交控制權（IPL $F80604：關 overlay → JMP (A0)）
[event] $E9001C: $0000 -> $0002  [bit1：關閉低區 IPL overlay]
[event] $E9001C: $0002 -> $000A  [bit3：關閉高區 IPL overlay]
[event] *** 進入卡帶入口 PC=$00000412 ***
[done] 到達卡帶入口後再執行 5000 條指令，無 bus fault
[done] 最終 PC=$00002BF8
```

對應 IPL 流程（`bios-68k.md` §2）逐步命中：

1. reset 向量 SSP=`$FD000A`、PC=`$400`（取自 IPL 向量表，overlay 生效）
2. `$40A` UMC6650 交握（trace 可見 `$EB0D03`/`$EB0D01` 埠操作）
   → RAM 區讀寫測試、金鑰反向讀回與四種校驗全部通過（走到 `$55A`）
3. 卡帶 `$2000` 起 128 byte 授權比對＋MULS 雜湊兩階段通過（走到 `$5F4`）
4. `$F80604`：`$E9001C` OR `$0002`（關低區 overlay）、重讀卡帶向量
   `$0/$4`、OR `$0008`（關高區 overlay）、`JMP (A0)`
5. **進入卡帶入口 PC = `$00000412`**，之後再跑 5000 條指令無例外。

## 交叉驗證

- 知識庫 `tools/rominfo.py`：Boom Zoo entry PC = `$00000412`、
  SSP = `$00FCFFFE` → 與模擬器實際跳入位址**一致**。
- Monopoly（`--instructions 5000`）：同樣流程通過，進入入口
  PC = `$000024C6`，與 `bios-rom-format.md` §2.1 表一致；之後再跑
  5000 條指令無例外（卡帶程式自行重寫 `$E9001C=$0001`，即釋放 65C02
  reset（bit0），屬卡帶行為，與 `bios-68k.md` 所述「IPL 不動 bit0、
  由卡帶自行釋放」吻合）。

## 實作備註（與知識庫/MAME 的出入）

- UMC6650 埠角色以 IPL 實際用法實作（`$EB0D03`=位址埠、`$EB0D01`=
  資料埠，金鑰區 `$20-$2F` 唯讀）；MAME `umc6650.cpp` 寫反，未沿用。
- `$E9001C` 實作為 16-bit 暫存器（IPL 以 `move.w`+`ori.w` 操作）。
- 手排修正：轉交點判定位址是 `$F80604`（高區視圖）而非 `$604`——
  IPL 在 `$5FE` 即 `JMP $F80604`。
- `$E90B3C` 雜訊區給予真實 word 儲存（MAME 標 nopr；IPL 會讀寫）。
