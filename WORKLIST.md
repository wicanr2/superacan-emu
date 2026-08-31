# Super A'Can 模擬器工作清單

更新日期：2026-08-31。狀態只反映目前程式與最近證據；歷史見 `WORKLOG.md`。

## 進行中：純 Go 轉向

- [x] 把目前 C++／CMake／SDL2 實作移至 `archive/cpp/`，在 archive README 標明
  deprecated、最後 production commit `d923486`、證據限制與唯讀 oracle 用途。
  完成條件：Docker 內仍可重建 archived binary，原有驗證文件與第三方 notice 可回查。
- [x] 建立純 Go module、package 邊界與 headless test runner；production path 禁止 cgo。
  完成條件：`go test ./...` 在無網路固定 image 內通過，machine core 不 import Ebitengine。
- [x] 建立獨立 Go Motorola 68000 核心骨架。
  完成條件：公開 bus／phase／IRQ API、register 與 prefetch state、`Step` result、reset
  vector vertical slice；設計符合 `docs/chip-emulation-principles.md`。
- [ ] 建立 opcode／addressing-mode／exception 測試階梯與 Moira 差分 harness。
  完成條件：每個差異可分類為 Go bug、sample 差異或硬體 unknown，不以 Moira 自動定案。
- [x] 建立第一組 opcode decoder、16 種 condition code、MOVEQ、BRA.b／BRA.w 與
  Bcc.b／Bcc.w phase trace。完成證據見 `docs/m68k-implementation.md`。
- [ ] 實作 extension-word cursor、operand size 與 68000 effective-address 基礎，再完成
  BSR／JSR／MOVEA，讓 IPL 能從 `$400` 的 NOP 前進到第一段 UMC6650 初始化。
- [ ] Go 68000 跑通 Super A'Can IPL 第一段。
  完成條件：固定 BIOS hash、reset SSP／PC、phase trace 與 C++ oracle 對照，逐步抵達
  UMC6650 交握；缺 opcode 時明確停止，不用 stub NOP。

## Deprecated C++ 收尾紀錄

- [ ] 在專案專用、可重現的 Docker image 內從乾淨 archive build 目錄完成 Release 建置。
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

## Go 核心工程

- [ ] 建立純 Go phase scheduler，讓 Linux、headless、macOS 與 Ebitengine 前端共用
  同一硬體時間線；不沿用 C++ `main.cpp` runner 架構。
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
