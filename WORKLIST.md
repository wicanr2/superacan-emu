# Super A'Can 模擬器工作清單

更新日期：2026-08-31。狀態只反映目前程式與最近證據；歷史見 `WORKLOG.md`。

## 進行中：里程碑 5 收斂

- [ ] 在專案專用、可重現的 Docker image 內從乾淨 build 目錄完成 Release 建置。
  完成條件：固定 compiler／CMake／SDL2 與依賴 commit，無主機 runtime 混入，輸出由
  目前 UID/GID 擁有。
- [ ] 審查全部未提交程式變更，移除或隔離 `ACAN_STAGING` 等一次性探針。
  完成條件：核心正常路徑沒有遊戲專屬特判，`git diff --check` 通過。
- [ ] 收緊 save state 格式與載入交易。
  完成條件：ROM 不符、截斷、版本錯誤、payload 損壞一律拒絕且不改變現行狀態；
  記錄 BIOS 身分或明確說明限制；衍生狀態載入後可確定重建。
- [ ] 建立 save-state 決定性回歸。
  完成條件：至少 Boom Zoo 與另一套音效驅動遊戲，各做連續執行對照「存檔→新行程
  載入→相同額外幀」，比較 frame、audio 與關鍵 CPU／bus 狀態 hash。
- [ ] 重跑最小相容性矩陣。
  完成條件：Boom Zoo、Monopoly、Speedy Dragon 的 IPL、畫面、音訊、P1 路徑無回歸；
  P2 至少完成一條實際雙人選單或遊戲流程，不只讀值 dump。

## 硬體證據缺口

- [ ] FRC：取得實機、Bcan 動態 trace 或更強證據，取代目前 MAME HACK case 表；在此之前
  保持 `MAME-derived/unknown-hardware` 標示。
- [ ] UM6619：確認 envelope、混音增益、削波與未知暫存器；禁止用聽感猜出演算法。
- [ ] latch 3-byte 封包：從 68k producer、65C02 consumer 到玩家可見效果建立完整鏈。
- [ ] window 1／複雜 ROZ line table：找到實際使用軟體與同狀態 oracle 後再升格。
- [ ] partial update：只有出現可重現的 mid-frame 差異才實作，不因舊 TODO 自動開工。

## 核心工程

- [ ] 將 scheduler／runner 狀態從 `main.cpp` 抽離成可測核心，讓 Linux、headless 與未來
  macOS 前端共用同一硬體時間線。
- [ ] 為 register transaction、IRQ edge／level／ack、DMA 邊界、reset 與 open-bus 建立
  不依賴商業 ROM 的單元／整合測試。
- [ ] 將一次性環境變數 trace 整理成有界、可篩選的 device／address-space 除錯介面。
- [ ] 建立 ROM／BIOS manifest：檔名、大小、雜湊、word-swap 規則與錯誤訊息；版權輸入
  繼續排除於 Git。

## 平台與發行

- [ ] 里程碑 5 收斂後規劃 macOS 編譯。
  完成條件：選定可重現工具鏈、SDL2 來源、支援架構與最低 macOS 版本，產物在 macOS
  實機完成啟動、輸入、音訊、headless 與 save-state smoke。這是平台工作，不得修改
  模擬核心來通過封包。
- [ ] 建立 Linux 可重現發行包與第三方授權清單；只含程式，不含 ROM／BIOS／遊戲畫面。

## 已完成且不重新開啟

- [x] IPL、UMC6650 lockout、68k／65C02 基本執行與主要 bus mapping。
- [x] UM6618 主要 tilemap／sprite／DMA／IRQ 與 SDL2/headless 畫面路徑。
- [x] UM6619 PCM、DMA、timer、WAV 與 P1 輸入；Speedy 第二驅動可播放。
- [x] sprite DMA word 雙觸發根因修正；Boom Zoo／Monopoly 標題結構恢復。
- [x] ROZ、P2、FRC、save state 初版程式已寫入工作樹；仍須完成上列收斂閘門。
