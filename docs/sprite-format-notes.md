# sprite 縮放與 mosaic 的實作契約

更新日期：2026-09-02。欄位語意、公式與量測方法在唯讀知識庫
`../acan/docs/sprite-format.md`；本檔只記本專案這一側的實作決定。

## 契約

| 項目 | 契約 | 證據等級 |
|---|---|---|
| sprite 致能 | **沒有致能位元**，`word3 == 0` 才不畫 | `confirmed-Bcan` |
| 水平縮放 | `word2` bits 15–11，`width = (hscale + 6×nw)/(hscale + 1)`，1:1 為 5 | `confirmed-Bcan` |
| 垂直縮放 | `word0` bits 15–13，`height = vscale ? (vscale + 2×nh − 1)/vscale : 3×nh`，1:1 為 2 | `confirmed-Bcan` |
| mosaic | `word1` bits 5–3，塊大小 = 值 + 1，塊原點 `floor(d/m)×m` | `confirmed-Bcan` |

`m` 不保證是 2 的冪次（3 與 6 都量到），所以塊原點只能用整數除法算，不能用位元遮罩。

## 實作

`drawSprites()` 改成單一路徑：先算 `spriteGeometry()` 得到繪製尺寸，再逐目的像素
以 mosaic 量化、線性對應回來源，最後由 `spriteSource()` 解出該來源像素（直接 tile 或
子 tile 表、整體翻轉與 tile entry 翻轉）。原本「原生尺寸走 `drawSpriteTile`、多 tile
走另一條」的雙路徑已移除，因為 mosaic 在 1:1 時也會生效，兩條路徑無法各自成立。

## 驗證

- 知識庫的 `homebrew/spriteprobe/` 兩頁共 48 個案例，與 Bcan 0.0.8b 逐像素相同
  （相異 0／76800）。
- 回歸：本地五款 ROM 各 1500 幀、每 150 幀取樣共 30 個檢查點，改動前後完全相同。
  已發行軟體在這些段落只用 1:1 且 mosaic 0。
- 單元測試 `TestSpriteScaleAndMosaicFields`、`TestSpriteMosaicSamplesBlockOrigin`
  釘住尺寸公式與塊原點的算法。
