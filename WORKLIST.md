# Super A'Can 模擬器工作清單

更新日期：2026-09-01。狀態只反映目前程式與最近證據；歷程見 `WORKLOG.md`。

## 目前位置

模擬核心已可執行：`Bcan008b/ROMS` 下九個卡帶檔案（八個 raw、一個雙部分 ZIP）全部
完成 3600-frame 有界執行，帶輸入的 5400-frame 路徑也全部完成並看得到實際遊戲畫面。
三條呈現路徑（headless、Ebitengine、純 Go X11）的指令數與 framebuffer SHA-256 逐位元
相同。存檔、卡帶電池記憶體、P1／P2 輸入、68000 主要例外路徑都已完成。

剩下的工作分成四類：**要拍板的決定**、**使用者介面**、**平台層與發行**、
**模擬正確性**。硬體證據缺口另列，不排期。

相關文件：[`docs/verify-rom-matrix.md`](docs/verify-rom-matrix.md)、
[`docs/platform-targets.md`](docs/platform-targets.md)、
[`docs/bcan-ui-inventory.md`](docs/bcan-ui-inventory.md)、
[`docs/ui-design.md`](docs/ui-design.md)、[`docs/save-state.md`](docs/save-state.md)。

## A. 要拍板才能往下走的決定

- [ ] **cgo 邊界對三個發行平台的適用範圍。** 發行平台限定 Linux／macOS／Android 之後，
  原本支持「全 binary 禁 cgo」的兩個目標（js/wasm、windows）都不在範圍內。實測與
  三個選項的代價見 `docs/platform-targets.md`；建議把禁令縮到模擬核心，平台層按平台
  決定。**拍板前不動現有程式。**
- [ ] **CJK 字型的散布授權。** 介面要對齊 Bcan 的五種語言就需要 CJK 字型。候選與授權
  必須先確認可隨發行包散布，這是介面語言階段的前置條件，不是實作細節。
- [ ] **錄影功能的做法。** Bcan 用 Windows Media Foundation 產生 H.264/AAC；純 Go 沒有
  對等的編碼器。可行方案與取捨由 `docs/ui-design.md` 提出，需要選一個。

## B. 使用者介面：對齊 Bcan 的功能

架構決定與畫面設計見 [`docs/ui-design.md`](docs/ui-design.md)，對照基準見
[`docs/bcan-ui-inventory.md`](docs/bcan-ui-inventory.md)。階段順序依設計文件；
下列是必須達成的功能清單，每一項都以 Bcan 的同名功能為驗收對照。

- [ ] `ui` 套件骨架：純 Go 自繪，畫進 RGBA 緩衝，消費抽象輸入事件，不 import 任何
  前端套件（要有測試守著），畫面可在 headless 以 SHA-256 驗證。
- [ ] 卡帶載入：無卡帶啟動畫面、卡帶瀏覽器、最近開啟清單；raw 與 ZIP 都能列出。
- [ ] 存讀檔：十個槽、槽位循環、縮圖（直接用存檔 payload 內的 framebuffer）。
- [ ] 遊戲控制：重新啟動遊戲、暫停、全速切換（含全速靜音選項）。
- [ ] 影像：整數縮放、全螢幕、4:3 顯示、視訊濾鏡（Bcan 有七種）、動態平滑。
- [ ] 音訊：音量與輸出方式。
- [ ] 輸入設定：P1／P2 逐鍵重新綁定，鍵盤與手把可並存，重複偵測，回復預設值。
- [ ] 熱鍵設定：存檔、讀檔、循環槽位、截圖、開始／停止錄影、FPS 顯示、全速、
  鎖定全部金手指；重複偵測。
- [ ] 金手指：限 Work RAM `$FC0000–$FCFFFF` 的記憶體搜尋（Exact／Fuzzy、New Search／
  Refine）、清單管理（新增並鎖定、鎖定／解鎖、刪除、更新值與名稱）、每遊戲一個檔案。
  這是除錯／作弊工具，不是硬體行為，要在文件與程式碼標明。
- [ ] 截圖：PNG，直接取自 UM6618 顯示孔徑，不套濾鏡、不含疊加層。
- [ ] 錄影：依 A 節的決定實作。
- [ ] 狀態列與 toast 訊息，含可隱藏操作訊息但錯誤仍顯示。
- [ ] 診斷面板：frame／指令數／圖層遮罩／視訊暫存器；只讀不寫。同時收尾
  「把一次性環境變數 trace 整理成有界、可篩選的除錯介面」。
- [ ] 安全停止文案：啟動失敗與執行中止都要說明發生什麼，不得假裝成功。
- [ ] 設定檔：JSON，路徑依 XDG，未知欄位忽略。
- [ ] 介面語言：英／法／西／繁中／簡中，切換不需重啟。前置條件見 A 節字型授權。

## C. 平台層與發行

- [x] Linux 桌面：純 Go X11 前端 `cmd/acan-x11`，`CGO_ENABLED=0` 可建置，九款卡帶與
  headless 逐位元相同。見 `docs/x11-frontend.md`。
- [ ] 純 Go 音訊輸出，取代目前的外部播放程序（候選：直接操作 `/dev/snd`、
  PulseAudio 原生協定）。
- [ ] 在有實體音效裝置的 Linux 驗收 48 kHz 播放、鍵盤操作、延遲與 underrun。
- [ ] macOS 平台層。等 A 節 cgo 決定；無 cgo 路徑需要以 purego 自建 Cocoa 視窗與
  CoreAudio 輸出。完成條件：實機啟動、輸入、音訊、存讀檔 smoke 全過，且不修改
  模擬核心來遷就平台。
- [ ] Android 平台層：觸控輸入、虛擬手把、生命週期（暫停／恢復）與存檔落地。
  完成條件同上，另加螢幕旋轉與返回鍵行為。
- [ ] 三個平台的可重現發行包與第三方授權清單；只含程式，不含 ROM／BIOS／遊戲畫面。
- [ ] CI 守住「模擬核心在五個目標上 `CGO_ENABLED=0` 可建置」，不靠人記得。

## D. 模擬正確性

- [ ] 在靜止畫面上完成與 Bcan 的逐像素差分並分類每一處差異。Boom Zoo 標題目前差
  43.48%（`--width 256`），調色盤數值兩邊相同、差異在落點。完成條件：至少一款卡帶的
  靜止畫面差異能逐項標成 renderer 缺陷、oracle 侷限或硬體 unknown。
- [ ] Sango Fighter 的 ROZ 文字位置：目前只有「取樣到的狀態文字在畫面外」這個觀察，
  沒有證據說算錯。要定案需要 oracle 同一瞬間的 ROZ 暫存器，也就是先解出 Bcan
  `ACANRTS` 的 payload 版面。
- [ ] The Son of Evil 的 F003 pixel mode（`$F001F0` bit 3）：frame 3600 取樣到單張雜訊
  畫面，前後正常。MAME 只保存該位元、渲染路徑未讀取。
- [ ] 把既有 233 條逐一 case 遷移到一般化執行層並刪除，每步維持測試綠燈。
  完成條件：`Decode` 只保留無法一般化的編碼，兩套路徑不再並存。
- [ ] 補上剩餘的 68000 例外：位址錯誤、匯流排錯誤、RESET、STOP。
  TRAP／TRAPV／CHK／除以零／特權違例／MOVE USP 已完成。
- [ ] 將 65C02 的 3:1 排程收斂成可驗證的 cycle 邊界。
- [ ] P2 完成一條實際雙人選單或遊戲流程，不只讀值 dump。
- [ ] 把 save-state 決定性回歸擴到第二套音效驅動的遊戲，並比對同一取樣視窗的音訊雜湊。

## E. 測試與工具

- [ ] 為 register transaction、IRQ edge／level／ack、DMA 邊界、reset 與 open-bus 建立
  不依賴商業 ROM 的單元／整合測試。
- [ ] 建立 ROM／BIOS manifest：檔名、大小、雜湊、word-swap 規則與錯誤訊息；
  版權輸入繼續排除於 Git。

## F. 硬體證據缺口

這些不排期，等到有證據才動；在此之前維持現有標示，不以聽感或畫面感覺定案。

- [ ] FRC：取得實機、Bcan 動態 trace 或更強證據，取代目前的 MAME HACK case 表。
- [ ] UM6619：envelope、混音增益、削波與未知暫存器。已量到九款卡帶中五款會撞到滿刻度，
  最高 1.80%（見 `docs/verify-rom-matrix.md`），但這只說明現況，不足以定出正確公式。
- [ ] latch 3-byte 封包：從 68k producer、65C02 consumer 到玩家可見效果的完整鏈。
- [ ] window 1 與複雜 ROZ line table：找到實際使用的軟體與同狀態 oracle 後再升格。
- [ ] partial update：只有出現可重現的 mid-frame 差異才實作。

## G. Deprecated C++ 收尾

- [ ] 在專案專用、可重現的 Docker image 內從乾淨 archive build 目錄完成 Release 建置。
- [ ] 審查 archive 內未提交的程式變更，移除或隔離 `ACAN_STAGING` 等一次性探針。

## 已完成且不重新開啟

歷程與證據見 `WORKLOG.md`，各項的驗證方式見對應的 `docs/` 文件。

- 純 Go module、package 邊界、headless runner，machine core 不 import 任何前端。
- 68000 與 65C02 的一般化執行層：全部 12 種定址模式、主要指令族、W65C02S 未指派編碼的
  NOP 行為；八款卡帶由各自停在不同編碼變成全部跑完（`docs/cpu-generic-execution.md`）。
- 位址暫存器保留完整 32 位元、長字位移時間、5 位元調色盤展開三項勘誤。
- IPL、UMC6650 lockout、UM6618（tilemap／sprite／window／ROZ／DMA／IRQ）、UM6619
  （PCM／timer／DMA／IRQ）、主機 DMA、FRC、bus observer。
- UM6619 主機端讀取埠 `$E90004/05`、`$E9000C/0D`、`$E90018/19`。
- media：raw 與 ZIP 卡帶（含雙部分依尺寸接合）、word-swap 與 SHA-256 manifest。
- 存檔 `ACANGOS1`：交易式載入、綁定 IPL 與卡帶身分、決定性已用真實卡帶驗證。
- 卡帶電池記憶體存讀、兩個前端的 P1／P2 輸入。
- Bcan 畫面 oracle 管線與 `cmd/acan-imgdiff`；有界指令回溯與視訊暫存器 dump。
- Ebitengine 前端與純 Go X11 前端，兩者與 headless 逐位元相同。
