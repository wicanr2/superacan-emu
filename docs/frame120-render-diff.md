# UM6618 frame 120 分層差分

更新日期：2026-09-01

## 固定輸入與執行點

- IPL SHA-256：`2e4d88bec69b5e7e4803368c233ce0d20f6dd107c5af0cfcc0089d310c695d7c`
- Monopoly ROM SHA-256：`b90b8dbfd15f1bcdd3e8f70910fa2f69effe07f2c781d04b123b738813fecb2f`
- 純 Go machine 執行至 UM6618 frame 120／scanline 240，共 1,715,920 條 68000 指令。
- 對照圖：`docs/screenshots/monopoly-logo-f120.png`。它是 deprecated C++ oracle 輸出，
  不是實機硬體真相。

Monopoly 原先在第 95,188 條、PC `$002416` 因尚未實作 opcode `$0C79` 停止。補上一般
`CMPI.W #imm,(xxx).L` 與 20-cycle 合成測試後，才得以到達同 frame；未用遊戲特判。

## 分層結果

| mask | 層 | 非黑像素 | framebuffer SHA-256 | 觀察 |
|---:|---|---:|---|---|
| 8 | sprite | 4,727 | `81f4cc51e1bec1bdeffffeb8680c40a3817a0c5dafc5feae4d27fa6273f24c02` | A'Can 彩色標誌形狀正確 |
| 16 | ROZ | 23,733 | `7e3a99256637ebc9d57a2230f7419ad45d13da43dfe443ea02104175075fe0c3` | 產生重複灰色中文字／弧形圖樣 |
| 32 | window | 61,440 | `2f4e334116e701928372f98503370f324e0a296680d428c935b43df3a9c2b167` | 產生預期純紫背景，右側 64 px blank |
| 63 | 全層 | 61,195 | `81a5381a6a412f9a5c86fadd089d20c479fce7a79e73567156547ec996f2697e` | ROZ 污染純紫背景 |

當時 `video_flags=$120E`、ROZ mode `$4020`、window control `$6802`。診斷性略過 ROZ
逐行表仍會產生圖樣，證明問題不只在三張 scaling table，而在 1bpp ROZ 基本 HACK／
混色或優先度契約。

本節的 framebuffer SHA-256 產生於 5 位元調色盤展開修正之前，現行程式不會重現這些
值；分層像素數與圖層觀察結論不受影響。展開規則與新基準見
[`docs/bcan-oracle-diff.md`](bcan-oracle-diff.md)。

## 勘誤與證據界線

- deprecated C++ renderer 的 `ACAN_LAYERMASK` 預設值是 `0xF`，後來新增 ROZ bit `0x10`
  時沒有同步更新；因此既有「正常」截圖其實預設沒有畫 ROZ，不能證明 1bpp 路徑正確。
- 固定 MAME commit `6ae579a` 的 driver 自己把 1bpp ROZ 標為 improperly-emulated，並把
  scaling-table 條件標為 `HACK - Not trusted`；該來源只能列 MAME-derived。
- sprite＋window 已足以重建玩家預期構圖，但目前沒有硬體證據允許直接關閉 ROZ、翻轉
  透明 bit 或改寫全域 priority。這些方案維持 hypothesis，不進 production renderer。

固定來源：

- MAME Super A'Can driver：<https://github.com/mamedev/mame/blob/6ae579aed3107c0b42c1c1c5cb05c02df4456eff/src/mame/umc/supracan.cpp>
- 專案分層輸出由 `acan-headless --frames 120 --layer-mask` 產生；商業 ROM／暫存 PNG 不入版控。
