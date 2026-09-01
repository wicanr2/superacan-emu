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
| Sango Fighter | 海景開場 → 龍紋選單 → 五名武將的選角畫面 |
| Speedy Dragon | 實際遊戲場景：角色、草地、磚牆與天空 |
| Super Taiwanese Baseball League | 實際比賽畫面：球場、守備球員與打者 |
| The Son of Evil | 開場圖版 → 遊戲中對話：人物立繪、地圖與文字框 |

## Ebitengine 前端與 headless 的一致性

同一組輸入以 `cmd/acan` 在 Xvfb 執行 1200 frame（`--audio=false`），八款的 68000
指令數與 framebuffer SHA-256 與 headless 完全相同，證明 GUI 沒有另走簡化排程：

| ROM | 68000 指令 | framebuffer SHA-256 |
|---|---:|---|
| Boom Zoo | 17,369,003 | `3784f8663b1c3a869498d2e14c0b948c598d50d15cf54b6f5380c9b294155562` |
| Formosa Duel | 19,270,779 | `0856269e7b402158e953de03d0553128d720ef64f29afc97403f93471404d587` |
| Journey to the Laugh | 17,778,132 | `42285d489bd74a5c5fd0d66700ed7e7c8b2b83f4855612d7dec4db07c30b146e` |
| Monopoly | 11,827,355 | `c254c50d5f85dd6ede60b82c8b2a07ca2ca8ccd41e9bfbe65a1c45299083d582` |
| Sango Fighter | 11,634,924 | `412213dac64ec07ef8db6ee69f4a90a351880f11c3229b378647d05f559bd505` |
| Speedy Dragon | 18,513,698 | `d3e5336af35b4c5bdac93dca6e1f3686be861564f16d69a97ef8fa947a5b7d67` |
| Super Taiwanese Baseball League | 17,572,195 | `e28f1c411a389ecd46206d8006e1e9b54f62a75047bcb2e64b7f12763f094023` |
| The Son of Evil | 16,727,440 | `bbd3a45fb5d27acf8e6caef06f5c9f7d00f8743d2ad42b0bbb8baea2d23bca73` |

## 音訊

以 `--frames 1800 --wav` 匯出 48 kHz 16-bit stereo，八款都有實際內容，不是只有非零值：

| ROM | 樣本數 | 峰值 | RMS | 觸頂樣本比例 |
|---|---:|---:|---:|---:|
| Boom Zoo | 1,427,305 | 20,128 | 2,108 | 0.00% |
| Formosa Duel | 1,508,407 | 32,768 | 11,874 | 1.80% |
| Journey to the Laugh | 1,497,757 | 32,768 | 10,983 | 0.70% |
| Monopoly | 1,511,950 | 32,768 | 11,624 | 0.42% |
| Sango Fighter | 1,515,024 | 32,768 | 9,914 | 0.08% |
| Speedy Dragon | 1,510,581 | 32,768 | 11,515 | 1.44% |
| Super Taiwanese Baseball League | 1,517,209 | 32,768 | 5,534 | 0.00% |
| The Son of Evil | 1,430,002 | 23,599 | 2,540 | 0.00% |

五款會撞到滿刻度，最高的 Formosa Duel 有 1.80% 的樣本觸頂。UM6619 的混音增益與削波
行為本來就列在「尚未升格為硬體事實」，這組數字把它從「未知」變成「已量到多少」：
在取得實機或 Bcan 的同狀態音訊之前，不以聽感調整增益。

## 已知落差

- **Sango Fighter 的文字與 UI 靠在畫面右緣被切掉**：龍紋選單的三個選項與選角畫面
  右上角的資訊框都出現在 `x≈300` 之後。以 `--layer-mask 16` 單獨輸出可確認它們在
  ROZ 圖層且字形正確，只是位置靠右。該 frame 的 ROZ 暫存器是 mode `$0622`
  （64×32、4bpp、wrap）、scroll 全零、`incxx`／`incyy` 都是 `$0100`（1:1），逐行表因
  mode bit 9 而略過；維度與圖層選擇都與 MAME 的 `get_tilemap_dimensions` 和
  `get_tilemap_region` 一致。因此目前沒有證據說 renderer 算錯，只有「取樣到的狀態
  文字在畫面外」這個觀察。要定案需要 oracle 在同一瞬間的 ROZ 暫存器，這要等
  ACANRTS save state 的 payload 版面解出來。
- **Sango Fighter 尚未進到對戰**：以 START／A／B 交替輸入可以走到五名武將的選角
  畫面，但沒有進到實際對戰；Bcan 在只按 START 的情況下會進到對戰場景。可能是輸入
  序列本身需要方向鍵選人再確認，也可能與上一項同源。
- **The Son of Evil 有單張雜訊畫面**：frame 3600 取樣到整片隨機像素，frame 1200 的
  開場圖版與 frame 5400 的遊戲中對話都正常。尚未定位是換場中的暫態還是特定 pixel
  mode 的解碼問題；該遊戲會使用 `$F001F0` 的 bit 3，而 MAME 只保存該位元、渲染路徑
  並未讀取。
- 畫面正確性只到「可辨識、可操作」這一級。像素級對帳見
  [`docs/bcan-oracle-diff.md`](bcan-oracle-diff.md)；Boom Zoo 標題畫面與 Bcan 的差異
  目前是 43.48%（`--width 256`），調色盤數值兩邊相同，差異在像素落點。
- Super Dragon Force 是雙部分卡帶的 ZIP，本專案的 media 層尚未支援，不在上表。
