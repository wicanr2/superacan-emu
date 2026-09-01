# 商業 ROM 相容性矩陣

更新日期：2026-09-01

固定輸入：IPL SHA-256 `2e4d88bec69b5e7e4803368c233ce0d20f6dd107c5af0cfcc0089d310c695d7c`，
UMC6650 key SHA-256 `f158d83be6e73389967c6dadfd5160bb742e09212a1b218fb829bae3b4961b28`，
以及 `internal_6502_1.bin`／`internal_6502_2.bin` 兩個音效 BIOS bank。ROM、BIOS 與
Bcan 二進位都不入版控，執行時由外部唯讀掛載。

## 3600 frame 有界執行

命令形如：

```sh
go run ./cmd/acan-headless --ipl … --key … --sound-bios1 … --sound-bios2 … \
    --rom "…/<ROM>.bin" --frames 3600
```

| ROM | 68000 指令 | 完成 frame | 非黑像素 | 非零音訊樣本 |
|---|---:|---:|---:|---:|
| Boom Zoo | 50,505,560 | 3600 | 61,440 | 1,781,630 |
| Formosa Duel | 56,747,720 | 3600 | 75,809 | 2,257,007 |
| Journey to the Laugh | 53,531,761 | 3600 | 72,298 | 2,618,516 |
| Monopoly – Adventure in Africa | 32,974,618 | 3600 | 76,800 | 2,557,330 |
| Sango Fighter | 30,846,962 | 3600 | 18,173 | 2,607,495 |
| Speedy Dragon | 55,885,830 | 3600 | 76,374 | 2,372,355 |
| Super Taiwanese Baseball League | 51,678,959 | 3600 | 62,674 | 2,546,863 |
| The Son of Evil | 49,999,382 | 3600 | 3,907 | 2,279,978 |

八款全部沒有觸發未實作 opcode、未知硬體操作或有界執行上限。這是本專案第一次讓
`Bcan008b/ROMS` 下的每一款 raw ROM 都連續執行到指定 frame 數。

## 5400 frame 正常玩家路徑

同樣的命令加上 `--press 1200:START,1500:START,…,4800:START` 與
`--screenshot-dir/--screenshot-every 600`，用來確認畫面會隨輸入前進，而不是停在
開機畫面。八款都完成 5400 frame，人工檢查取樣畫面的結果：

| ROM | 觀察到的畫面 |
|---|---|
| Boom Zoo | 開場動畫（月夜、狼影、角色）→ 標題與選單 |
| Formosa Duel | 對戰選角畫面：兩張人物照、姓名框與下方棋盤 |
| Journey to the Laugh | 實際遊戲場景：油桶、管線、平台與 HUD |
| Monopoly | 標題畫面：立體字標與 START GAME／LOAD GAME |
| Sango Fighter | 停在只有兩張相同人物立繪的畫面，缺背景與 UI（見下） |
| Speedy Dragon | 實際遊戲場景：角色、草地、磚牆與天空 |
| Super Taiwanese Baseball League | 實際比賽畫面：球場、守備球員與打者 |
| The Son of Evil | 開場圖版 → 遊戲中對話：人物立繪、地圖與文字框 |

## 已知落差

- **Sango Fighter 停在選角畫面**：畫面上只有兩張相同立繪，背景與 UI 沒有出現，且
  frame 3600 與 5400 的內容相同。待與 Bcan 同畫面差分後才判定是輸入路徑、圖層
  還是遊戲邏輯問題。
- **The Son of Evil 有單張雜訊畫面**：frame 3600 取樣到整片隨機像素，前後的 frame 都
  正常。尚未定位是換場中的暫態還是特定 pixel mode 的解碼問題；該遊戲會使用
  `$F001F0` 的 bit 3，而 MAME 只保存該位元、渲染路徑並未讀取。
- 畫面正確性只到「可辨識、可操作」這一級。像素級對帳見
  [`docs/bcan-oracle-diff.md`](bcan-oracle-diff.md)；Boom Zoo 標題畫面與 Bcan 的差異
  目前是 43.48%（`--width 256`），調色盤數值兩邊相同，差異在像素落點。
- Super Dragon Force 是雙部分卡帶的 ZIP，本專案的 media 層尚未支援，不在上表。
