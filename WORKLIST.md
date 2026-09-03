# Super A'Can 模擬器工作清單

更新日期：2026-09-02。狀態只反映目前程式與最近證據；歷程見 `WORKLOG.md`。

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

## A. 決定

2026-09-01 全部拍板，紀錄見 [`docs/ui-design.md`](docs/ui-design.md) §15：

- **cgo**：Linux 與 macOS 的發行 binary 維持 `CGO_ENABLED=0`；**Android 開例外**，走
  Ebitengine 的 gomobile 路徑。禁 cgo 之下沒有產出 Android 應用的路徑（`-buildmode=
  c-shared` 在任何平台都要求 cgo，而 Android 應用的原生碼一定要是共享程式庫），
  這是工具鏈限制不是工作量。見 [`docs/platform-targets.md`](docs/platform-targets.md)。
- **字型與語言**：嵌入 `bitmapfont/v4`，介面做英／法／西／繁中／簡中五種語言。
  發行包附六份來源授權文字與 Baekmuk 商標標示。
- **錄影**：預設 MP4／MJPEG＋PCM（純 Go、無執行期相依），另開設定指向本機的
  OpenH264 換成 H.264。見 [`docs/capture-formats.md`](docs/capture-formats.md)。
- **金手指**：進第一個發行版，但啟用時畫面常駐標記，且該工作階段的 frame／audio
  雜湊不得作硬體證據。

僅剩一項小決定，不擋任何階段開工：

- [ ] **A1 桌面 Esc 由「離開」改為「開啟選單」。** 會改變現行 X11 前端行為，要在發行
  說明標出；不改則 Android 返回鍵語意與桌面不一致。

## B. 使用者介面：對齊 Bcan 的功能

完整設計、畫面線框與每階段的可驗證驗收條件見
[`docs/ui-design.md`](docs/ui-design.md)；對照基準見
[`docs/bcan-ui-inventory.md`](docs/bcan-ui-inventory.md)。

階段順序與理由由設計文件定，原則是先解掉會讓後面白做的閘門，再做已有底層支援因而
立刻可用的功能，最後才做需要新編碼或新資產的。每一階段都要維持九款卡帶的
1200-frame 指令數與 framebuffer SHA-256 不變——這些數字一動就表示 UI 滲進了模擬路徑。

- [x] **P0 閘門的事實蒐集**：平台建置矩陣（`docs/platform-targets.md`）與字型涵蓋
  及授權查核（`docs/ui-font.md`）都已完成。決策本身在 A 節。
- [x] **P1** `ui` 核心與覆蓋選單：套件骨架、事件與 Intent 型別、兩套度量、主題、
  S3 覆蓋選單、S4 存檔槽、toast 與錯誤列、D1 確認對話。畫面雜湊與卡帶基準見
  [`docs/verify-ui.md`](docs/verify-ui.md)。`session` 套件把核心與介面接起來，
  「叫出選單、存檔、讀檔」已在 headless 以真實卡帶驗證通過（`--ui-script`），
  X11 前端也已接上並在 Xvfb 內跑過同一條腳本；覆蓋層沒開時畫面結果與 headless
  逐位元相同。Ebitengine 前端的接線併入 P4（Android）一起做。
- [x] **P2** 啟動（S0）、主機韌體設定（S0.1）、卡帶瀏覽器（S1）、關於（S8），
  另補上 fail-closed 停機畫面（S9）。畫面雜湊與 headless 驗證見
  [`docs/verify-ui.md`](docs/verify-ui.md)。
- [x] **P3** 設定總表（S5）、輸入綁定（S5.1）、熱鍵設定（S5.2）、JSON 設定檔
  （未知鍵保留、型別不符只重置該欄位、整份壞掉改名成 `.bad`、原子寫入）。
- [x] **P4** 觸控層：虛擬手把（橫式與直式）、九宮格方向鍵與死區、五點多點觸控、
  左右手互換、S5.6 觸控版面設定、返回鍵行為。搖桿模式與自繪十六進位鍵盤尚未實作，
  兩者都在畫面上標明原因。實機驗證要等 Android 前端（C 節）。
- [x] **P5** 影像設定（S5.3）、音訊設定（S5.4）、診斷（S7）。圖層遮罩、視訊暫存器
  與指令數都在診斷畫面上，取代一次性的環境變數 trace。
- [x] **P6** 金手指：搜尋（S6.1）、清單（S6.2）、`ACANCHT1` 讀寫與 `BCAN_CHT_1`
  匯入。啟用時畫面常駐 CHEAT 標記，越界寫入由入口拒絕，關閉時 UI 通道零寫入。
- [x] **P7** 擷取：PNG 截圖與 AVI／MJPEG＋PCM 錄影（容器由 MP4 改為 AVI，理由見
  `docs/capture-formats.md`），另有 `--capture-sink` 外部接收端。錄影開著時指令數與
  framebuffer 雜湊不變。編碼目前是同步的，丟棄計數永遠是 0；改成非同步時才會有值。
  OpenH264 選配尚未實作。
- [x] **P8** 多語言：英／法／西／繁中／簡中（S5.5），嵌入 `bitmapfont/v4`。
  發行包要附六份來源授權文字與 Baekmuk 商標標示——這一項屬於發行包，見 C 節。

下列是必須達成的功能清單，每一項都以 Bcan 的同名功能為驗收對照；分佈在上面的階段裡。

- [x] `ui` 套件骨架：純 Go 自繪，畫進 RGBA 緩衝，消費抽象輸入事件，不 import 任何
  前端套件（`TestNoFrontendDependencies` 守著），畫面在 headless 以 SHA-256 驗證。
- [x] 卡帶載入：無卡帶啟動畫面、卡帶瀏覽器、最近開啟清單；raw 與 ZIP 都能列出。
  最近清單的持久化屬於設定檔，隨 P3 一起做。
- [x] 存讀檔：十個槽與縮圖（直接用存檔 payload 內的 framebuffer），拒絕理由與實際
  載入同源。槽位循環由熱鍵接上。
- [x] 熱鍵動作接線：十七個動作全部走 `ui.Hotkey`，前端只翻譯「這個鍵剛按下／
  剛放開」。出廠鍵位 F1–F12 與 Tab，設定畫面顯示的是實際生效的鍵（含出廠鍵位）。
  headless 用 `hk<動作>`／`hkup<動作>` 驗證，結果見 `docs/verify-ui.md`。
  唯一沒有實際效果的是全螢幕，見下一項。
- [ ] 視窗層套用 `Video.Fullscreen`：設定與熱鍵都會切換這個欄位，但 `frontend/x11`
  與 `frontend/cocoa` 還沒有全螢幕。X11 要送 EWMH 的 `_NET_WM_STATE_FULLSCREEN`
  client message 並跟著 `ConfigureNotify` 改變呈現尺寸，macOS 是
  `toggleFullScreen:`。兩者都需要有視窗管理員的環境才驗得到——bare Xvfb 沒有 WM，
  client message 不會有人處理，所以這一項不能只靠容器內的建置就宣告完成。
- [x] 影像：縮放、整數縮放、長寬比、Scanline 25/50/75 濾鏡、全螢幕、顯示 FPS。
  Bcan 的 CRT 與 Composite 濾鏡不做（本前端沒有 shader 管線），動態平滑尚未實作，
  兩者都在畫面上標明原因而不是藏起來。
- [x] 音訊：主音量、全速時靜音、輸出緩衝、輸出方式與緩衝狀態（唯讀）。
- [x] 輸入設定：P1／P2 逐鍵重新綁定，鍵盤與手把可並存，重複偵測。
  「還原預設」按鈕還沒做，目前清除綁定（Del）等同回到預設。
- [x] 熱鍵設定：十七個動作可重新指定，含重複偵測；欄位顯示實際生效的鍵，
  設定檔沒寫時顯示出廠鍵位。清單超過一頁時捲動（`listWindow`），S5.1 與 S6.2 同。
- [x] 金手指：限 Work RAM 的記憶體搜尋（等於／不等於／大於／小於／變動／未變、
  New Search 與 Refine）、清單管理（新增並鎖定、鎖定切換、刪除）、每遊戲一個檔案。
  「更新值與名稱」還沒做：需要清單頁的行內文字編輯，與 S6.1 的數值輸入共用。
- [x] 截圖：PNG，直接取自 UM6618 顯示孔徑，不套濾鏡、不含疊加層（有測試比對兩條路徑）。
- [x] 錄影：AVI 容器、MJPEG 視訊、PCM 音訊，全部用標準函式庫。
  OpenH264 選配與 `.mp4` 容器留待後續。
- [x] toast 訊息與錯誤列，含可隱藏操作訊息但錯誤仍顯示。狀態列隨 P2 的畫面補齊。
- [x] 診斷面板：frame／指令數／IRQ 受理數／sound clash／媒體身分／圖層遮罩；
  只讀不寫，遮罩以 intent 交給入口執行。
- [x] 安全停止文案：S9 顯示停機原因、frame、指令數與媒體身分，且不能用返回鍵略過。
- [x] 設定檔：JSON，路徑依 XDG（macOS 走 Application Support），未知欄位保留。
- [x] 介面語言：英／法／西／繁中／簡中，切換不需重啟（畫面每一幀都從字串表取字）。

- [ ] **把命中區補到其餘畫面**：存檔槽（S4）、熱鍵清單（S5.2）、金手指清單（S6.2）
  與設定裡的選項列目前只吃鍵盤。單欄選單、確認對話與卡帶瀏覽器已經可以用指標點，
  機制見 [`docs/ui-design.md`](docs/ui-design.md) §9.1.1，補其餘畫面是照著加
  `addHit` 並把確認邏輯抽成 `activate`。
- [ ] **macOS 前端的滑鼠事件**。X11 已經接上，Cocoa 這邊卡在
  `NSEvent.locationInWindow` 回傳的是 `NSPoint`（兩個 CGFloat 的結構），
  purego 的 `objc.ID.Send` 只取得到整數回傳值，在 arm64 上結構是走浮點暫存器的，
  直接讀會拿到垃圾。要嘛用 `purego.RegisterLibFunc` 宣告結構回傳，要嘛繞道
  `NSWindow.mouseLocationOutsideOfEventStream`。**這一項必須在實體 Mac 上驗過
  才算完成**：FFI 寫錯的症狀是回傳值看起來合理但其實是垃圾，靜態檢查抓不到。

## C. 平台層與發行

- [x] Linux 桌面：純 Go X11 前端 `cmd/acan-x11`，`CGO_ENABLED=0` 可建置，九款卡帶與
  headless 逐位元相同。見 `docs/x11-frontend.md`。主機側的媒體載入、電池記憶體、
  截圖與外部音訊已抽到 `frontend/hostio`，與 macOS 前端共用。
- [ ] 純 Go 音訊輸出，取代目前的外部播放程序（候選：直接操作 `/dev/snd`、
  PulseAudio 原生協定）。
- [ ] 在有實體音效裝置的 Linux 驗收 48 kHz 播放、鍵盤操作、延遲與 underrun。
- [~] macOS 平台層：`frontend/cocoa` 與 `cmd/acan-macos` 已完成並在 darwin 的兩個
  架構上 `CGO_ENABLED=0` 建置與 vet 通過，鍵碼表有單元測試。**還沒在真實 macOS 上
  跑過**——交叉編譯只證明組得起來，證明不了 Objective-C 呼叫慣例與事件迴圈的實機
  行為。實機 smoke 步驟見 [`docs/macos-frontend.md`](docs/macos-frontend.md)。
- [~] Android 平台層：`frontend/mobile` 在 `GOOS=android CGO_ENABLED=0` 下建置通過
  並有單元測試；生命週期（離開前景落地＋叫出選單、返回鍵）在 `session` 有測試；
  `mobile/acan` 已用 NDK 編成 `arm64-v8a`／`armeabi-v7a`／`x86_64` 三份
  `libgojni.so`，包成 AAR 與可側載的 APK（簽章 v1/v2/v3，min 21 / target 34）。
  **沒有在任何實機或模擬器上跑過**：觸控、音訊延遲、旋轉與返回鍵全部未驗證。
  契約、量測與實機 smoke 清單見
  [`docs/android-frontend.md`](docs/android-frontend.md)。
- [x] Android 工具鏈 image（`docker/android.Dockerfile`：JDK 17＋cmdline-tools＋
  platform 34＋build-tools 34＋NDK 27.2＋ebitenmobile）。整個 image 5.69 GB，
  新增層約 2.9 GB。
- [~] 三個平台的可重現發行包與第三方授權清單；只含程式，不含 ROM／BIOS／遊戲畫面。
  Linux 的 AppImage 已可重現產出（`packaging/appimage.sh`，不需要 appimagetool），
  跑出來的指令數與 framebuffer 雜湊與 headless 基準相同；預設路徑走 XDG，缺韌體
  改由啟動畫面說明而不是啟動失敗。第三方授權清單已進包
  （`packaging/THIRD-PARTY-LICENSES`），含 Baekmuk 授權原文與商標標示、M+ 授權
  原文、bitmapfont 的 Apache-2.0 原文，以及 OFL-1.1 原文與四份 OFL 來源的版權聲明
  （原文取自 Ark Pixel 與 Galmuri 兩份上游並逐字比對一致）。唯一剩下的缺口是阿拉伯
  字符那一份：上游沒有可引用的版權聲明檔，目前依 bitmapfont README 的標示轉載。
  macOS 出 universal `.app`（arm64＋x86_64，未簽），Android 出 AAR 與除錯金鑰簽的
  APK。見
  [`docs/release-packaging.md`](docs/release-packaging.md)。
- [ ] CI 守住「Linux 與 macOS 的發行 binary `CGO_ENABLED=0` 可建置」，不靠人記得。
  Android 不受此檢查；模擬核心則在五個目標上都要通過。

## D. 模擬正確性

- [x] Boom Zoo 標題各圖層的垂直落點：是 Bcan 截圖把 224 條撐成 240 條造成的假偏移，
  不是 renderer 缺陷。兩軸都還原後整張 0 差異，見
  [`docs/bcan-oracle-diff.md`](docs/bcan-oracle-diff.md)。
- [ ] **顯示區為什麼是第 8 條起的 224 條**。Bcan 的孔徑只涵蓋本專案 framebuffer 的
  第 8–231 條，上下各 8 條在 oracle 這一側看不到；這是垂直方向的「右側 64 欄」問題。
  完成條件：實機或掃描線層級的證據說明可見區的起點與行數，在那之前維持 `unknown`，
  不依 oracle 的取景改 renderer。
- [ ] **`tilePixel` region 2（2bpp）的位元次序沒有 oracle 證據**。region 1 已由 Bcan
  截圖定案為高半位元組先出；region 2 目前維持低位優先，只是沿用而非量到。完成條件：
  找到實際使用 2bpp tile 的畫面並與 Bcan 同狀態對照。
- [x] 在靜止畫面上完成與 Bcan 的逐像素差分：Boom Zoo 標題整個顯示區 0 / 57,344。
- [x] 把逐像素定案擴到多款卡帶：八款各跑一次 oracle 共 96 張，27 張 0 差異，
  扣掉單色畫面還有 19 張是實際畫面，橫跨 Boom Zoo、Formosa Duel、Speedy Dragon 與
  Super Taiwanese Baseball League 四款。見 [`docs/bcan-oracle-diff.md`](docs/bcan-oracle-diff.md)。
- [ ] **Boom Zoo 開場的 sprite 沒有被 letterbox 黑條切掉**（`bz-02` 差 88 像素、
  `bz-03` 差 352 像素，全部在第 152–159 條）。黑條上緣兩邊都在第 152 條，差的是越過
  黑條的 sprite：Bcan 切齊，本專案畫在上面。第 0–2400 幀逐幀搜過都對不上，不是相位。
  已排除「window 與 sprite 同優先度時誰蓋誰」——把比較從 `>=` 改成 `>` 一個像素都沒變，
  代表該 sprite 的優先度嚴格高於 window 0（`$F001D0 = $29AF`，優先度 1）。
  完成條件：定位到實際機制（sprite 的 mask 模式、window 對 sprite 的裁切、或 raster 分割）。
- [ ] **Super Taiwanese Baseball League 在 4000 frame 無輸入下走到的畫面與 Bcan 不同**
  （oracle 第 4 張之後差 50–88%）。要先確認兩邊在同一個流程位置，才知道是流程差異
  還是 renderer 差異。
- [ ] Sango Fighter 的 ROZ 文字位置：目前只有「取樣到的狀態文字在畫面外」這個觀察，
  沒有證據說算錯。要定案需要 oracle 同一瞬間的 ROZ 暫存器，也就是先解出 Bcan
  `ACANRTS` 的 payload 版面。
- [x] The Son of Evil 的 F003 pixel mode（`$F001F0` bit 3）：已實作 bitmap 路徑，
  以知識庫的自製卡帶 `acan/homebrew/bit3probe` 與 Bcan 截圖逐像素相同（commit
  `2d71080`，由另一個工作階段完成）。
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
