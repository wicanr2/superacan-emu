# 工作歷程

## 2026-09-03：README 收 Bcan 選單列對照，順帶推翻自己的盤點

- 使用者要 README 有「Windows 工具列」的畫面，選的是**放 Bcan 的截圖做對照**
  而不是實作選單列。收在 `docs/screenshots/bcan/`，未載入卡帶所以不含遊戲畫面。
- **實測推翻我們自己的盤點**：`bcan-ui-inventory.md` 原本寫選單列有六個選單，
  含「金手指 (&C)」。實際在 Wine 內逐一開過，**只有五個**（檔案／顯示／輸入／
  語言／說明），而且**載入卡帶前後都一樣**。字串表裡確實有完整的金手指介面文字、
  `Bcan.ini` 也有 `hotkey_lock_all_cheats`，所以功能存在——但它不在選單列上。
  從哪裡開啟還沒查證，文件改成寫明「未查證」而不是留著錯的選單項。
  這是「字串表有」被寫成「選單列有」的典型誤推。
- 另外補上顯示選單有三個子選單（整數縮放、視訊濾鏡、錄影格式）。
- **也更正我自己前一輪的說法**：擋住選單列的不是 C3。C3 禁的是系統原生 widget，
  自繪的選單列不違反它。真正的理由是兩個取捨——Android 沒有選單列這個概念，
  以及 320×240 的畫面上緣讓給選單列就得縮小遊戲畫面。寫成 `ui-design.md` §4.0，
  並註明取捨改變就可以重新評估。
- 取截圖踩到的坑：Wine 容器沒有 CJK 字型，中文介面全是方框；掛主機字型與把字型
  複製進 wineprefix 都沒有用（字型替換要動登錄檔）。改用 Bcan 自己的英文介面
  （`Bcan.ini` 的 `language=en`，拍完還原），比繼續修字型便宜得多。

## 2026-09-03：覆蓋層接上指標，桌面可以用滑鼠點

- 起點是一個判斷錯誤：先前說「`ui` 端已經是現成的，兩個前端各補一段事件訂閱即可」。
  實際查過才發現 **`Pointer` 只被虛擬手把消化**，而且只在覆蓋層關著時；
  覆蓋層本身從來沒有解讀過座標，連 Android 開了選單之後也點不動。
- 機制用**命中區**：畫面在自己的 `draw` 裡登記可點區域，`ui` 收到指標事件時查表。
  命中區每一次 `Draw` 重建——版面只在畫的當下才算得出來，另外維護一份資料結構
  會有兩份真相，而且一定會有一份先過期。
- 每一塊帶 `hover`（把焦點移過來）與 `action`（等同鍵盤確認）。確認的邏輯抽成
  各畫面的 `activate`，鍵盤與指標呼叫同一份，避免「鍵盤能做、滑鼠不能做」的分岔。
  查表由後往前，modal 才會蓋過底下的列；按下與放開要落在同一塊才算數。
- 覆蓋層的每一個畫面都接上了：單欄選單（S3、S5 根、S0、S9）、確認對話 D1、
  卡帶瀏覽器 S1、存檔槽 S4（含頁籤）、綁定 S5.1、熱鍵 S5.2、影像 S5.3、音訊 S5.4、
  觸控版面 S5.6、金手指搜尋 S6.1（含按鈕與候選）與清單 S6.2（含總開關）。
  **Android 一併受惠**：`handleTouch` 在覆蓋層開著時本來就讓路，選單因此變成可點。
- 兩個地方值得記：**返回登記在共用的 `page` 標題列上**，一改就讓所有走 page 的
  畫面都能點；**金手指搜尋不能用預設的確認行為**（第 3 列開的是數值輸入、最後三項
  是按鈕），所以 `drawOptionRows` 拆出一個讓呼叫端決定確認行為的版本。
- 掃描式測試 `TestEveryListScreenAcceptsPointer` 逐一檢查十個畫面在內容區有沒有
  登記命中區。這條擋的是「新畫面忘了接指標」，那種缺漏用畫面雜湊看不出來。
- X11 視窗層訂閱三個滑鼠事件，只收第 1 號按鈕（第 4／5 號是滾輪，收進來會變成
  「在滾輪位置點了一下」），移動事件在一幀內合併成最後一筆。
- 在 Xvfb 內用真的 xdotool 指標驗過：滑到「讀檔」那一列高亮跟著移動，按下去進到
  存檔槽畫面。C10 抽驗不變（`steps=17369003`、`f720c9d1…b92301`）。
- **macOS 沒做，而且是刻意不做。** `NSEvent.locationInWindow` 回傳 `NSPoint`
  （兩個 CGFloat 的結構），purego 的 `objc.ID.Send` 只取得到整數回傳值，
  arm64 上結構走浮點暫存器，直接讀會拿到垃圾。這種錯誤靜態檢查抓不到，症狀是
  「值看起來合理但其實是垃圾」，而這台機器上沒有 Mac 可驗。寫進 WORKLIST 並註明
  必須在實體 Mac 上驗過才算完成，勝過把沒驗過的 FFI 塞進發行版。

## 2026-09-03：補齊 OFL-1.1，發行 AppImage 與 macOS，另做本機完整版

- **公開散布的阻擋條件解除**。`packaging/THIRD-PARTY-LICENSES` 原本自己寫著
  「補上 OFL-1.1 原文之前這個包不可對外散布」。bitmapfont v4.1.0 模組內只有
  Apache-2.0 的 `LICENSE`，四份 OFL 來源只列在 README，所以原文要從各字型專案的
  上游取：Ark Pixel 的 `LICENSE-OFL`、Cubic 11 的 `OFL.txt`、Galmuri 的 `ofl.md`。
- 四份的授權原文相同，包裡只收一份。**用兩份獨立上游（Ark Pixel 與 Galmuri）
  正規化後逐字比對確認一致**才收，不是抄一份就算。版權聲明四份各自列出。
- 留著一個缺口並寫明：阿拉伯字符（Eternal Dream Arabization）上游沒有可引用的
  版權聲明檔，bitmapfont 的 README 也只標作者與授權名稱，只能照上游標示轉載。
- 兩個包都重建並確認帶到新的授權檔（387 行、含 OFL 原文）；AppImage 的 300-frame
  smoke 與基準相同（`instructions=4364786`、`122922cb…c71198`）。
- 公開 Release `v0.1.0-preview`（prerelease）：AppImage、macOS universal `.app.zip`
  與 `SHA256SUMS.txt`。**上傳前逐項確認過包內沒有韌體或卡帶**——列了 AppImage 的
  squashfs 內容（只有執行檔、圖示、`.desktop` 與兩份授權）與 `.app` 的 zip 清單。
- **Android 改用發行金鑰**。除錯金鑰是公開的固定金鑰，不適合對外散布。
  `packaging/android-apk.sh` 改成吃 `ACAN_ANDROID_KEYSTORE`／`_KEY_ALIAS`／
  `_KEYSTORE_PASS_FILE`，有給就產 `superacan-emu.apk`，沒給維持除錯金鑰。
  **密碼走檔案不走環境變數**：環境變數會出現在 `ps` 與 `docker inspect` 裡。
- 踩到一個坑：`apksigner` 的 `file:` 是「一行一次密碼」，而 `--ks-pass` 與
  `--key-pass` 共用同一個讀取器，指到同一個單行檔案時第二次讀會撞 EOF
  （`Failed to read Key ... end of file reached`）。腳本改成產一份兩行的暫存檔，
  簽完 `trap` 刪掉；事後確認 `build/` 內沒有殘留。
- 金鑰放 `~/.local/share/superacan-emu-keys/`（0700，檔案 0600），RSA 4096、
  10000 天。**不在 repo 內也不在 `build/` 內**——前者會進版控，後者會被建置腳本清掉。
  憑證 SHA-256 `df0a3988…96ba5a`，與 APK 上的簽章者指紋一致；v1／v2／v3 都通過。
  這把金鑰弄丟就無法再發同一個應用程式的更新，要另外備份到這台機器以外。
- **本機完整版** `build/full/SuperACan-full/`（另有 `.tar.gz`，共 57 MB／50 MB）：
  三個平台的執行檔＋四份韌體＋九款卡帶＋`run.sh`。`run.sh` 把韌體、卡帶、存檔、
  截圖與設定全指到包內的相對路徑，不碰 `$HOME` 的 XDG 目錄，整包搬走也能跑；
  1200-frame 實跑結果與 C10 基準相同。包內附「請先讀我.txt」寫明不可對外散布，
  並指向只含執行檔的公開 Release。`build/` 已在 `.gitignore` 內，`git check-ignore`
  對韌體、卡帶與 tarball 逐一確認過。

## 2026-09-02（續）：推廣影片重錄並進版控

- 用發行的 AppImage（`29350d20…f16aae`）重錄一次，70 秒、960×720、60 fps、含聲音；
  預建存檔與錄影都在容器內，輸入是腳本，可重現。
- **進版控的版本另外壓一次**：`-preset veryslow -tune animation -crf 26`，
  12.4 MB → 5.0 MB。逐幀比過介面文字（卡帶瀏覽器那一段最吃細節），crf 26 與 crf 20
  肉眼無異；crf 30（3.3 MB）的文字開始變軟，所以停在 26。
- **影片走 GitHub Release 附件，不進版控**（使用者 2026-09-02 決定）。這個 repo 的
  pack 只有 791 KB，而二進位檔不做差分——每重錄一次就在歷程多留一份完整副本。
  影片固定掛在 tag `promo` 上，網址是
  `releases/download/promo/superacan-emu-promo.mp4`，重錄只要 `--clobber` 覆蓋，
  網址不變。`packaging/promo.sh` 多產一份 `-repo` 檔供上傳。
- 踩到兩個會讓網址失效的坑：`gh` 的 `檔案#名稱` 設的是顯示標籤不是檔名（附件會叫
  `repo-crf26.mp4`），以及 `releases/latest/download/…` 對 prerelease 回 404。
  兩件都已寫進 `docs/release-packaging.md`。

## 2026-09-02（續）：八款卡帶逐張對拍，19 張實際畫面與 Bcan 逐像素相同

- 八款各跑一次 oracle（不送輸入，12–14 張、間隔 4–5 秒）共 96 張，每張都用兩種孔徑
  幾何（`256x224+0+8`、`320x224+0+8`）去搜本專案的候選幀，取較好的。
  **顯示模式會在同一次執行內變**——Speedy Dragon 十二張裡有一張是 256 模式、
  其餘 320，所以不能一款只試一種幾何。
- 結果：27 張 0 差異，**扣掉單色畫面之後還有 19 張是有內容的實際畫面**，橫跨
  Boom Zoo、Formosa Duel、Speedy Dragon、Super Taiwanese Baseball League 四款
  （36–44 色、多數是整屏非黑）。
- **單色畫面要從統計裡扣掉**：全黑或全一色的畫面就算 0 差異也證明不了 renderer。
  八張 0 差異是這種，寫進報告會虛報。
- 對不上的多半是相位（選角輪播、開場淡入、attract 實際遊玩、斜向捲動背景）。
  唯一定位到的真差異是 Boom Zoo 開場的 sprite 沒有被 letterbox 黑條切掉：
  `bz-02` 差 88 像素、`bz-03` 差 352 像素，全部在第 152–159 條，黑條上緣兩邊都在
  第 152 條。第 0–2400 幀逐幀搜過，只比那一帶最好仍差 88 像素，不是相位。
- 試過並排除一個假設：把 window 的優先度比較從 `>=` 改成 `>`（同級時 window 蓋掉
  sprite），差異一個像素都沒變 → 該 sprite 的優先度嚴格高於 window 0
  （`$F001D0 = $29AF`，優先度 1）。實驗已還原，沒有留在程式裡。
- Super Taiwanese Baseball League 第 4 張之後 Bcan 走到本專案這一輪沒走到的畫面
  （差 50–88%）。那是流程位置不同，不能當 renderer 差異，另列待辦。

## 2026-09-02（續）：Bcan 截圖垂直也被撐開，還原後 Boom Zoo 標題 0 差異

- 上一輪只還原了水平軸。逐掃描線比對顯示「偏移量會隨畫面高度單調變化」：第 27 條
  是 +6，第 215 條是 −7，每約 15 條變一次——那是縮放不是平移。直接量重複掃描線就
  確認了：Bcan 的截圖在 *y* = 30, 45, 60, …, 210 都與下一條完全相同，
  即 `dst = floor(src × 240 / 224)`；本專案的輸出一條重複都沒有。
  **兩款卡帶都是 224 條**，與水平的 256／320 之分無關。
- 本專案 framebuffer 是 320×240，內容落在第 8 條起的 224 條。完整還原規格因此是
  `--reference-unstretch 256x224+0+8`（320 模式 `320x224+0+8`），`acan-imgdiff` 的該
  旗標改成吃 `WxH+X+Y`，幾何同時就是比較範圍，`--width` 不必再另外給。
- 還原之後：**Boom Zoo 標題整個顯示區 0 / 57,344 逐像素相同**（frame 2336、2340、
  2456、2460、2576 都是 0；其餘 frame 的差異全部集中在會旋轉的球上）。
  Sango Fighter 開場旁白的文字帶 320×40 也是 0 / 12,800（frame 2104）。
- 因此推翻上一輪「各圖層垂直落點不同、原因不明」的斷言：那是垂直撐滿造成的假象。
  **只還原一個軸比完全不還原更危險**——它留下的假偏移隨畫面高度變化，看起來很像
  「不同圖層各差幾條」這種真缺陷。
- 新的 `unknown`：顯示區為何是第 8 條起的 224 條。Bcan 的孔徑看不到本專案上下各
  8 條，這是垂直版的「右側 64 欄」，同樣不得依 oracle 的取景改 renderer。
- Sango 開場整張仍差 73%，差異在天空淡入的配色；第 2000–2200 個 frame 逐幀搜過都
  對不上 Bcan 的取樣時刻。相位問題，不是缺陷，但也因此無法用它定案整張。

## 2026-09-02：Bcan 對拍找到 4bpp 半位元組次序反了

- **Bcan 的截圖在 256 模式不是原生像素。** 孔徑固定 320 欄，UM6618 在 bit 8 = 0 時
  只輸出 256 欄，Bcan 用最近鄰撐滿。量法：數相鄰欄完全相同的對數——Boom Zoo 標題
  （`video_flags=$9ACC`）在每個 *x* ≡ 0 (mod 5) 都成立，Sango Fighter 開場
  （`$03C8`，320 模式）一對都沒有。`acan-imgdiff` 新增 `--reference-unstretch`
  還原它，另加 `--reference-out` 輸出還原後的參考圖。
  沒還原之前是在比兩張幾何不同的圖，量到的差異幾乎全是縮放。
- **`tilePixel` 的 region 1（4bpp packed）偶數 *x* 取錯半位元組。** 症狀是字形位置
  正確但每一對相鄰欄互換，字母裂成直條——README 截圖看起來會抖就是這個。
  改成偶數 *x* 取高半位元組後，Boom Zoo 標題版權文字列（Bcan 第 215–222 條 vs
  本專案第 208–215 條，56 欄）逐位元組相同；Sango Fighter 開場旁白的文字帶
  第 206–225 條（20 條掃描線 × 320 欄）也逐位元組相同，差異 16.78% → 8.68%。
  兩款一款 256 模式、一款 320 模式。`TestTilePixelPackedModes` 釘住次序。
- **診斷順序的教訓**：一開始把「圖形爛掉」歸給最近改過的 sprite 縮放，
  但把兩邊同一列印成字元圖之後，錯誤形狀是「相鄰欄兩兩互換」，與縮放無關。
  形狀先看清楚再猜成因，比先猜再驗便宜。
- 受影響基準：九款 1200-frame framebuffer SHA-256 除 Super Dragon Force 外全部改變，
  68000 指令數一個位元都沒動。3600-frame 的非黑像素只有兩款變
  （Journey to the Laugh 72,298 → 66,897、Super Taiwanese Baseball League
  62,674 → 62,129）。
- **雜湊原本抄在五個檔案裡**（`verify-ui.md`、`verify-rom-matrix.md`、
  `x11-frontend.md`、`ebitengine-frontend.md`、`bcan-oracle-diff.md`），其中三份
  在這次之前就已經過期（`x11-frontend.md` 與 `verify-rom-matrix.md` 對同一次
  1200-frame 執行給出不同的 Speedy Dragon 雜湊）。現在只有
  `verify-ui.md` 的卡帶基準（C10）保存值，其他四份改成指標。
- 前端等價性以新 renderer 重測：Boom Zoo 與 Sango Fighter 在 Xvfb 下由 `acan-x11`
  跑 1200 frame，指令數與 framebuffer SHA-256 與 headless 相同，`--screenshot` 的
  PNG 逐位元組相同。
- 未解：Boom Zoo 標題各圖層的垂直落點（logo 低 6 條、版權文字高 7 條，整張平移的
  最佳位移是 0，兩邊都已靜止）。region 2（2bpp）的位元次序沒有同級證據，維持現況。
  兩項都進 `WORKLIST` D。
- 重建：AppImage、macOS universal `.app`、Android AAR／APK 都以修正後的 renderer
  重出；README 的九張畫面（五張遊戲、四張介面）重新產生，遊戲畫面的重現命令寫進
  `docs/verify-rom-matrix.md`。
- 以發行的 AppImage（`29350d20…f16aae`）複驗一次：它自己產生的對拍畫面與原始碼樹
  的數字完全相同（Boom Zoo 標題 25.48%、Sango 文字帶 8.68%，兩段逐位元組相同的
  區塊都還在），1200-frame 的兩款雜湊與 C10 基準相同，`--screenshot` 的 PNG 與
  headless 逐位元組相同。並排圖收在
  `docs/screenshots/appimage/boomzoo-title-bcan-vs-appimage.png`。

## 2026-09-02：FRC 週期公式改為 Bcan 版，window 1 致能位元修正

- `chip/frc` 依 Bcan 反編譯改寫（知識庫 `../acan/docs/memory-map.md` §2.2）：
  改記主時脈 tick（master = 68k × 10）、週期算 `(n+1)`、倍率 12×1024 與 12×9040。
  原本是 MAME 衍生的 `1024×n`／`8192×n` 且以 68k cycle 計。
- `chip/umc6618` 的 window 1 致能位元由 `videoFlags&2` 改為 `&1`。Bcan 的 snapshot
  builder 是 `+160←v2&2`（window 0）、`+170←v2&1`（window 1）。
- 兩項對本地九款都是 no-op：沒有遊戲用 FRC 模式 `$1`／`$F`，也沒有遊戲設 window 1 的
  致能位元。純正確性修正。

## 2026-09-02：sprite 表的縮放與 mosaic 補上

- `drawSprites()` 原本把 `word0` bit14 當致能位元、忽略 `word2` 高 5 位與 `word1`
  bits 5–3。實測那三處分別是垂直縮放、水平縮放與 mosaic，而且 sprite 沒有致能位元
  （`word3 == 0` 才不畫）。公式與量測見 `docs/sprite-format-notes.md` 與知識庫
  `../acan/docs/sprite-format.md`。
- 實作改成單一路徑（幾何 → mosaic 量化 → 線性對應回來源 → 解出來源像素），
  原本的「原生尺寸／多 tile」雙路徑移除：mosaic 在 1:1 時也會生效，雙路徑無法各自成立。
- 踩到的坑：塊原點一開始寫成 `d &^ (m-1)`，在 m = 3、6 時錯。**mosaic 不保證是 2 的
  冪次**，只能用 `d / m * m`。這個錯誤讓 48 個案例中恰好只有 3 個不符，其餘全對——
  幾乎全綠的結果最容易讓人停在錯的實作上。
- 驗證：知識庫探針 48 個案例與 Bcan 逐像素相同；五款 ROM × 30 個檢查點無回歸。

## 2026-09-01：ROZ bit 3 改為實作 bitmap 路徑，並移除多餘的整層翻轉

- 知識庫的自製卡帶 `acan/homebrew/bit3probe/` 讓 bit 3 分支變成可達路徑，因此推翻同日
  稍早「不實作」的決定。`rozPixel()` 在 `(reg$1F0 & 0x18) == 0x08` 且 ROZ 為 8bpp region
  時改呼叫新的 `rozBitmapPixel()`：VRAM 當線性點陣圖，基底 `4 × $F00196`、遮罩
  `VRAMSize-1`、palette bank 取 `$F00182` 低 4 bit、像素值 0 透明。
- 驗證：同一顆卡帶在本專案與 Bcan 0.0.8b 的兩個相位畫面**逐像素相同**（相異 0／76800，
  SHA-256 一致）。單元測試 `TestROZBitmapModeFollowsPixelModeBit3` 另外釘住基底倍率。
- 一併移除 `rozPixel()` 的整層 X/Y flip。ROZ 的 mode bit 1/0 是 region 選擇，Bcan 的 ROZ
  迴圈只用 `& 3`、`& 0x20`、`& 0xF00`、`& 0x40`，沒有翻轉；原本那兩行是初版從 tilemap
  路徑帶過來的。1bpp（region 4）路徑提前返回，不受影響。
- 待辦：The Son of Evil 有長時間的 ROZ 8bpp 畫面，翻轉移除後應與 Bcan 做一次同畫面差分，
  確認改善而非只是換一種錯法。

## 2026-09-01：ROZ bit 3 分支確認不實作

- 以純記錄探針量測 Bcan ROZ bit 3 分支的條件（`(reg$1F0 & 0x18) == 0x08` 且 ROZ 8bpp）：
  八款 ROM 各 1200 幀、The Son of Evil 另跑 6000 幀，同時成立的幀數皆為 0。
- bit 3 只出現在八款共用的開機 logo 段落（各約 191 幀），該段 ROZ 為 1bpp，與分支要求的
  8bpp 互斥。因此 `docs/sound-ram-model.md` 由「待實作」改為「不實作」，並附量測表。
- 探針未進版控；`chip/umc6618` 與 `cmd/acan-headless` 沒有因此變更。

## 2026-09-01：`$F001F0` 契約修正（bit 3 在 ROZ 層有作用）

- 由 `../acan` 稽核工作階段以 IDA 逐指令追出 Bcan 的資料流後，前一則「pixel mode 不進入
  renderer」的記述作廢：pixel mode 與 gfx mode 都會進 renderer snapshot（`+190`／`+191`），
  各有唯一讀取點。gfx mode 用與 MAME 相同的三張 region 表（本專案 `tilemapRegion()` 已一致）；
  bit 3 只在 ROZ 層生效，條件是 `pixel_mode == $08` 且 ROZ 為 8bpp region。
- `docs/sound-ram-model.md` 已改寫該節，並把「ROZ bit 3 路徑」列為待實作項，附三步驟
  （先量測命中率 → F003 同畫面差分 → 實作並記錄 hash）。sound RAM 的 64 KiB 契約不受影響。

## 2026-09-01：FRC 的 ROM 用法與 IRQ3 消費者

- 來源：`../acan` 稽核工作階段以 Capstone 反組譯 Speedy Dragon、Formosa Duel、
  Journey to the Laugh 的 `$E90014/16/18` 寫入點、卡帶 autovector 表與 `$E90018` consumer。
- 訂正 `docs/frc-timer.md`：舊敘述「不能證明任何遊戲實際使用 FRC」在 1200 幀回歸下不成立，
  Speedy Dragon 的 IRQ3 acknowledge 為 17，且其 IRQ3 handler `$3454` 累加 `$FCE00E`、
  `$30DE` 是等待該 tick 的迴圈。
- 補記三款遊戲的 producer／consumer，以及 `$E90018` 必須回報持續變動的計數值
  （Formosa Duel 把它加到 tilemap 1 的 scroll，並用兩次讀值拼亂數種子）。
- 真實週期公式仍未知；校準入口是 Speedy 的「設週期→等 N 個 tick」。

## 2026-09-01：sound RAM 32 KiB alias 假說的 A/B 實驗

- 問題：`APU.sch` 的 U11 只接 `SNDRAM_A0..A14`（32 KiB），但 65C02 位址空間、68k
  `$E80000` 視窗與 MAME／Bcan／本核心都用 64 KiB。上半區是否只是下半區的 alias 未定案。
- 新增診斷開關：`Bus.SetSoundRAMAlias` 與 headless `--sound-ram-alias`（預設關閉），
  只在 RAM 存取丟掉 A15，`$0400-$04FF` 的 I/O 解碼仍用完整 65C02 位址；另加一個對撞
  偵測器，記錄同一實體 cell 先後被上下半區寫入的次數。
- 結果（各 1200 幀，完整 BIOS）：Boom Zoo、Monopoly、Speedy Dragon、Formosa Duel 的
  68000／65C02 指令數、`vram_sha256`、`framebuffer_sha256` 與 IRQ ack 計數在兩種模式下
  完全相同。唯一差異是 Boom Zoo 的音訊：`audio_nonzero` 453046 → 453000、
  `audio_sha256` 不同（約 0.01% 樣本）。
- 對撞偵測：Monopoly、Speedy、Formosa 為 0；Boom Zoo 恰好兩個 cell
  `$040A`／`$040B`（各一次），也就是 65C02→68k mailbox 旗標與該遊戲複製到 `$8400`
  起的歌曲位址表在 alias 模型下會共用同一塊儲存。
- 判讀：現有可驗證路徑幾乎無法區分 32 KiB alias 與 64 KiB 兩種模型；唯一可量測的
  分歧點就是 Boom Zoo 的 `$840A/$840B`。要定案需以 Bcan 或實機在同一狀態下比對
  「寫 `$8400+n` 之後 `$E9000C`／`$E8040A` 讀到什麼」，本輪不下結論。
- 驗證：本輪 `go test` 在同一 base commit 的乾淨 clone 內執行（`chip`、`machine`、
  `cpu`、`cmd` 全綠）；本工作區當時有另一工作階段未完成的 `cpu/m68k/*.go`，
  整包編譯不過，故未在此工作區跑測試，也未碰觸那些檔案。

## 2026-09-01：FRC period 對齊固定版 MAME 的實際行為

- 來源：`../acan` 知識庫稽核期間逐行比對固定 commit `6ae579a` 的 `update_frc_state`。
- 訂正：MAME 的 period 運算式 `((m_frc_control & 0xff << 16) | m_frc_frequency)` 依 C++
  運算子優先序等於 `control & 0x00ff0000`，對 16 位元的 control 恆為 0，因此該 oracle 的
  實際 period 只有 frequency，其逐 case 時間感也是照這個值校出來的。`chip/frc` 原本實作
  字面上的 24 位組合，在 mode 1／mode `$F` 會比 oracle 慢兩個數量級
  （magipool `$a201`／`$0104`：0x104 → 0x10104）。
- 變更：`chip/frc/device.go` 改用 `period = frequency`，並在程式旁註明來源與理由；
  `chip/frc` 與 `machine` 的三個相關測試期望值同步更新；`docs/frc-timer.md` 改寫該條契約。
- 另記：`docs/umc6618-implementation.md` 補上 320／256 模式的行時序繼承自 MAME 的
  `455/8` 與 `342/10`，因此兩模式幀率分別是 56.3 Hz 與 59.96 Hz。此疑點缺實機量測，
  維持 MAME-derived 值不改。
- 驗證：`superacan-ebitengine:go1.26.7-v1` 容器內 `go test ./...` 全綠；frontend 與
  `cmd/acan` 首次編譯需下載 Ebitengine 模組，該次開放網路，`go.mod`／`go.sum` 未變動。
- 邊界：本輪由 `../acan` 稽核工作階段代改，只動 FRC 契約與兩份文件，未碰其他子系統。

## 2026-08-31：接手與文件基線

- 目標：接手 Super A'Can 模擬器，讀取 `../acan` 與對應 Kimi session，建立模擬器專用
  規範、目前真相及唯一工作清單，訂正 README。
- 來源：唯讀檢查 `../acan/AGENTS.md`、`docs/`、Kimi session 索引與主 agent 紀錄；
  以目前 `18110af` 加未提交里程碑 5 程式／`docs/verify-misc.md` 為最新基準。
- 文件變更：新增 `AGENTS.md`、`CONTEXT.md`、`WORKLIST.md`、`WORKLOG.md`；README 改為
  模擬器定位、目前限制、穩定文件入口與 Docker 建置政策。
- 驗證：`openbor-linux-build:local`（CMake 3.28.3、GCC 13.3.0、SDL2 2.30.0）
  在無網路、2 GiB／2 CPU 容器內使用唯讀 Moira／CLK source 完成 Release 建置。
  首次建置發現 GCC 對 `pad_` state 載入的越界警告，改為兩欄明確讀取後增量重建
  無警告；`git diff --check` 通過。
- 動態 smoke：在無網路一次性容器內以唯讀 BIOS／ROM 跑 Boom Zoo 300 幀，成功通過
  IPL／UMC6650、輸出截圖與 save state；新行程載入後續跑 10 幀成功。這只證明單一路徑，
  不取代 `WORKLIST.md` 的跨遊戲決定性矩陣。
- Docker 清理：本輪 `docker run --rm` 容器均已移除；另有既存的
  `openbor-linux-build:local` 容器 `upbeat_rosalind` 正在執行，判定不屬本專案，未碰觸。
- Git：里程碑 5 與文件基線已提交為 `8037a33`，並推送至 `origin/master`。

## 2026-08-31：確認純 Go＋Ebitengine 轉向

- 決策：全面以純 Go 重寫模擬核心，Ebitengine 作跨平台前端；排除 cgo、C ABI 與只換
  前端。現有 C++ 留在同 repo 的 `archive/cpp/`，標為 deprecated oracle。
- 68000：Moira `a4c273b` 只作 sample／差分 oracle，新 Go 核心獨立實作，不直接移植。
- 時序：`Step` 對外一次一條指令，內部按 fetch／prefetch／read／write／internal／IRQ
  poll phase 推進 scheduler；排除純 instruction-total timing 與 pin-level 模擬。
- 調查：Moira 同時具 simple timing 與 precise timing；後者在 memory access 前後同步，
  並於指定 prefetch／poll point 取樣 IPL。據此建立 `docs/chip-emulation-principles.md`，
  但不把 Moira 行為升格為 Super A'Can 硬體證據。
- 驗證：文件相對連結與 `git diff --check` 通過；本輪唯讀調查／文件檢查容器皆以
  `docker run --rm` 結束，沒有留下本專案容器。

## 2026-08-31：純 Go 68000 第一個 vertical slice

- 範圍確認：所有其他晶片也採純 Go 獨立實作與 phase scheduler 通則；舊 C++、MAME、
  Bcan 與實機只作分級 oracle。
- 歸檔：將 CMake 與 `src/` 移至 `archive/cpp/`，每個檔案及 archive README 都標明
  deprecated；根目錄改由 Go module 接管。
- Go 68000：新增 bus／scheduler／phase 契約、register 與兩級 prefetch state、reset
  vector vertical slice、NOP 與 unknown-opcode fail-closed 行為。
- 測試：新增 reset vector、40-cycle 起始契約、scheduler-before-bus 順序、NOP prefetch
  與 unknown opcode 不改狀態測試。
- 驗證：`golang:1.26.7-bookworm` 無網路容器內 `go test ./...` 與
  `go test -race ./...` 均通過；第一次使用 login shell 清掉 image PATH，分類為驗證
  命令問題，改用非 login shell 後以同一 image 乾淨重跑。
- Archive 驗證：`openbor-linux-build:local` 無網路容器以 GCC 13.3、CMake 3.28.3、
  SDL2 2.30.0，從 `archive/cpp/` 新 source root 與唯讀固定 Moira／CLK source 完成
  Release 重建。
- Docker 清理：上述工作均使用 `docker run --rm`；沒有留下本專案容器，所有可寫
  Go 檔案仍由目前 UID/GID 擁有。
- Git：C++ 歸檔與純 Go 68000 第一個 vertical slice 已提交為 `977b2eb`，並推送至
  `origin/master`。

## 2026-08-31：68000 decoder、MOVEQ 與 branch

- 來源：NXP／Motorola Programmer's Reference Manual 的 opcode／condition／branch
  契約，以及 MC68000 User's Manual 表 8-9 的 Bcc／BRA cycle 與 read-count。
- 實作：可稽核 decoder、16 condition、MOVEQ、BRA.b／BRA.w、Bcc.b／Bcc.w，以及每條
  `Step` 的結構化 phase trace；BSR 只辨識並明確回報未實作。
- 測試：condition exhaustive truth table、MOVEQ sign／flags／X preserve、正反向 byte／
  word branch、taken／not-taken timing 與 prefetch refill。
- 修正：首次測試的向後 branch fixture 讓 target prefetch 與原 opcode 位址重疊，造成
  map 值覆蓋；移到不重疊 target 後以同一容器命令乾淨重跑。這是測試資料問題，不是
  CPU branch 計算缺陷。
- 驗證：Go 1.26.7 無網路容器內 `go test ./...`、`go test -race ./...`、`go vet ./...`
  均通過；本輪容器使用 `--rm`，未留下專案容器。
- Git：decoder、MOVEQ、BRA／Bcc 與證據文件已提交為 `8a67cae`，並推送至
  `origin/master`。

## 2026-08-31：68000 IPL 起始 JSR 與 MOVEA

- IPL 證據：`../acan` 的 CPU 可見反組譯確認 `$400` 為 `NOP`、`$402` 為
  `JSR $040A.W`，其後以 `MOVEA.L #imm,An` 建立 UMC6650 相關基址；原始 BIOS dump
  為逐 word byte-swap 儲存，不能直接把檔案 byte order 當成 CPU opcode。
- 來源：NXP／Motorola Programmer's Reference Manual 的 JSR／MOVEA 語意，以及
  MC68000 User's Manual 的 execution-time 表；Moira master
  `a4c273b08e07d82c73289ac032867c845969a0f2` 只用來核對 68000 JSR 的 phase 次序。
- 實作：新增共用 instruction extension-word stream、`MOVEA.L #imm,An`、
  `JSR (xxx).W`、可觀測的 16-bit data-write phase 與兩次有序的堆疊 long write。
- 契約：JSR 將 PC+4 寫入 A7-4，然後重填目標 instruction queue；總計 18 cycles，
  phase 為 internal 2、兩次 data write、兩次 instruction fetch。未知 addressing mode
  仍失敗即關閉，不以泛化 stub 放行。

## 2026-08-31：IPL 第一個 UMC6650 poll 分支

- 範圍：沿已證實 BIOS 路徑補上 `$41C MOVE.W $E90B3C.L,D0`、
  `$422 ANDI.W #1,D0`、`$426 BEQ.W`，並實作未取分支需要的
  `MOVE.W Dn,$E90B3C.L`，不向後擴張成未使用的完整 MOVE matrix。
- 實作：instruction stream 新增 long extension helper；新增絕對長位址 word read／write、
  16-bit condition-code 更新，以及每次資料存取的 supervisor-data phase。
- 時序核對：NXP／Motorola User's Manual 給出兩種 MOVE 皆 16 cycles、ANDI.W register
  為 8 cycles；固定 Moira `a4c273b08e07d82c73289ac032867c845969a0f2` 只用來確認
  register-to-absolute-long MOVE 是「消耗兩個 extension → data write → final prefetch」。
- 測試：以最小合成 IPL fixture 執行八條指令，從 `$400` 抵達 `$430`，驗證 A0/A1/A2、
  zero branch、資料 phase 與包含 reset 的 132 cycles；fixture 不含完整 BIOS 或版權資料。

## 2026-08-31：UMC6650 RAM 備份迴圈

- 範圍：實作 BIOS `$430–$448` 所需的 `MOVE.W #imm,Dn`、`MOVE.W Dn,Dn`、
  `MOVE.B Dn,(An)`、`MOVE.B (An),(An)+`、`MOVE.W #imm,(xxx).L`、`CMPI.W` 與 `DBcc`。
- 匯流排：新增 byte read/write phase；仍維持 scheduler 先推進、再呼叫 bus side effect。
  `(A7)+` 的 byte increment 為 2，其餘 address register 為 1。
- 旗標：MOVE byte／word 與 CMPI word 各有獨立寬度契約；CMPI 保留 X 與 operands，
  明確測試 equal、positive、borrow-negative 與 signed-overflow。
- 迴圈回歸：不含 BIOS 映像的合成 fixture 執行 162 條指令，完成 32 次 `$5F…$40`
  倒數；驗證 32 次 address-port byte write、32 次 data-port read、Work RAM `$FC0000…1F`
  post-increment write、32 次 noise word write，並以 1910 reset-inclusive cycles 離開迴圈。
- DBcc：分別測試 condition true 12 cycles、decrement-and-branch 10 cycles、counter expired
  14 cycles；未將後續 CPU 型號的 loop mode 套入 MC68000。

## 2026-08-31：Go 整機 bus 與真實 IPL 探測

- media：新增逐 16-bit word byte-swap、嚴格大小檢查、原始輸入 SHA-256 與轉換 manifest；
  UMC6650 key 維持線性 16 bytes。ROM／BIOS 內容不加入版控。
- UMC6650：新增獨立 Go chip package，實作 7-bit 位址、`$20–$2F` 唯讀 key、
  `$40–$5F` RAM 及 `$09/$0C` output register 儲存。
- machine：新增 24-bit bus、低／高 IPL overlay 單向 latch、卡帶雙視圖、Work RAM mirror、
  sound RAM、SRAM odd lane、`$E90B3C` 與 shared phase timeline；未知晶片 window 尚未假造。
- runner：新增 `cmd/acan-headless`，要求外部 IPL/key/ROM，輸出輸入雜湊、PC、opcode、
  instruction count 與 cycles；未知 opcode 失敗即關閉。
- 真實驗證：IPL SHA-256 為 `2e4d88bec69b5e7e4803368c233ce0d20f6dd107c5af0cfcc0089d310c695d7c`。
  第一輪停 `$46C CMP.B (A1),D0`，補實作後停 `$47C MOVE.B -(A2),(A1)`；再補實作後
  已跨過 RAM restore 與 key 讀取迴圈；成功完成 772 條指令後，精確停於
  `$4C2 CLR.W D4`、8652 cycles。

## 2026-08-31：完整 IPL、卡帶授權與 overlay 轉交

- checksum ISA：新增 CLR byte/word/long、ADD/SUB byte/word/long、ADDX、ADDQ/SUBQ、
  ANDI/ORI、CMP/CMPI/CMPM、BTST、NEG、SWAP，以及 predecrement/postincrement 變體。
- MULS：依 MC68000 Booth transition 規則建模 `42 + 2n` 的 `(An)+` 總週期，乘數為
  16-bit signed、結果為 32-bit，並測試資料相依 timing。
- 卡帶授權：真實 Boom Zoo 已通過 `$570` 的 64-word CMPM loop 與 `$578–$5F0`
  巢狀 MULS checksum；沒有略過比較、硬編結果或遊戲特判。
- 轉交：新增 stack immediate MOVE、absolute／indirect JMP、absolute-long JSR、ORI、
  `(An)` word MOVE 與 absolute-word MOVEA。新增合成 regression，證明 `$61E` 關 high
  overlay 後，已預取的 `$620 JMP (A0)` 仍執行並從卡帶向量進入 `$400`。
- 真實結果：成功完成 87,204 條指令、797,418 cycles；low/high overlay 均為 off，
  執行卡帶 `$420 JSR $2B22` 後停於 `$2B22 MOVEM.L`（opcode `$48E7`）。

## 2026-08-31：卡帶啟動與高位址 IPL 服務路徑

- 從 Boom Zoo `$2B22` 開始，以未知 opcode 失敗即關閉的方式逐段擴充一般 68000
  語意；沒有依 ROM hash、PC 或遊戲名稱加入特判。
- 堆疊／呼叫：新增 MOVEM.L predecrement／postincrement、BSR、RTS、PEA、long push，
  以及 brief-indexed JSR；回傳位址、A7 更新與 long-word bus 順序可觀察。
- 定址：新增 MC68000 brief extension 的 Dn／An、word／long index，套用於卡帶路徑所需
  MOVE／MOVEA／JMP／JSR；另補 PC-relative LEA。未接受 68020 full extension 或 scale。
- 算術／搬移：補齊實際路徑要求的 byte／word／long MOVE 變體、quick／immediate
  ADD/SUB/CMP、shift／rotate、BSET 與 OR；各 encoding 是一般暫存器形式而非單一 opcode
  stub。
- 真實驗證：固定 IPL `2e4d88…c695d7c` 與 Boom Zoo ROM `090827…370077` 在 Go
  headless runner 無錯完成 200,000 條指令、1,935,470 cycles，PC `$FF80A0`、opcode
  `$6AF0`，low/high overlay 均為 off；結束原因是指定指令上限。
- 回歸：新增 MOVEM push／restore round-trip、PEA effective-address push、indexed JSR
  return／prefetch、ROL flags／timing，以及本批真實 opcode 的 decoder cases。
- 證據限制：這證明固定軟體路徑已前進，不代表完整 MC68000、exception／IRQ 或遊戲
  可玩；下一步須以裝置交易 checkpoint 判定 UM6618／UM6619／sound RAM 初始化進度。

## 2026-08-31：deprecated oracle `$F001F0` 動態探針

- 範圍只限 `archive/cpp` 的 `ACAN_WATCH`：補上原先被 `write16` 單次 transaction path
  繞過的 `$F001F0` word log，輸出 frame、value 與原始 PC；沒有更動 UM6618 state 或 renderer。
- 以 F003 `The Son of Evil` raw SHA-256 `791ab9…deb` 在 Docker headless 跑 6000 幀：
  frame 20=`$0009`、211=`$0001`、216/219=`$0009`、255/3155/3349=`$0001`、5914=`$0009`。
  動態結果確認 bit 3 會切換，並確認 `$27EE` shadow consumer；未證實其 direct-color 語意。
- 建置沿用既有 `cd-access:dev` SDL2 image、固定 `/tmp/moira` 與 `/tmp/clk` source；一次性
  容器皆使用 `--rm`，沒有留下專案容器。輸出僅存 `/tmp/superacan-emu-watch` 作本輪探針。
- 探針後續加入 PC 起八個 instruction words，並窄記錄 `$FCDA50–$FCDA6F`、
  `$FCDB80–$FCDBAF` 的生成寫入。兩段 code 分別在 frame 15／16 由 `$FFFF80B6` 生成；
  writer 簽章 `12C3:60E4:0028:002C` 可精確回查 word-swap 後 ROM `$00073A54`。
- `$FFFFDA5C` 片段只在前五個 words 與 ROM `$74C86` 相同，後續立即值不同；`$FFFFDB90`
  完整簽章不存在 ROM。故已證實 runtime code generation；當時尚未界定 source 與長度，
  後續 register probe 結果如下，仍不冒稱已完整解出格式。
- 後續 register probe 修正解碼器 RAM 基址為 `$FFFF8000`，並界定同一次 frame 5–16 呼叫：
  A0 `$73B44→$74BEC`、實際 bitstream `$73BE8–$74BEB`（`$1004` bytes），A1
  `$FFFFB800→$FFFFDC56`（輸出 `$2456` bytes）。兩段 mode producer 都屬同一次連續輸出，
  不是兩次解壓；完整格式欄位仍待離線解碼器逐 byte 驗證。

## 2026-08-31：雙 CPU sound boot 與第一筆 VRAM 初始化

- 新增 bus transaction observer 與 headless `--watch`／`--watch-limit`；byte／word access
  都以完整 CPU transaction 記錄，並附 68000 step、PC、opcode。範圍解析、保留上限與
  word 不重複計數皆有測試。
- 第一輪證據：第 178,789 條起將 driver 寫入 `$E8F000`，第 182,885 條寫
  `$E9001C=$0001`，第 395,493 條開始輪詢 `$E80300`；沒有 sound CPU 時永遠讀 `$00`。
- 新增獨立純 Go W65C02 core、sound bus、3:1 shared scheduling 與 reset/HALT gate；實作
  真實 boot 所需 ISA 子集。新增 UMC6619 indirect register port，未假造 PCM／timer。
- 真實結果：65C02 從 `$F000` reset vector 起跑並把 `$0300` 寫成 `$FF`；68000 第一次
  輪詢即取得 ack，離開原本等待迴圈。其後在第 397,684 條起觀察到 `$F44400` VRAM
  word writes，證明已進入視訊資料初始化。
- 目前前進至 462,153 條 68000 指令附近；W65C02 已執行約 311,000 條。這只證明 boot
  路徑，尚不代表完整 65C02、IRQ、UM6618 renderer 或 UMC6619 音訊完成。

## 2026-08-31：UM6618 儲存窗口與 scanline 時間線

- 新增獨立 `chip/umc6618`：256 word registers、256 色 palette、128 KiB VRAM；bus 的
  word read/write 直接呼叫 device 一次，byte access 才做明確 read-modify-write。
- 回歸：register／palette／VRAM readback、vblank status read-ack、pixel-mode mask、
  `$F0001E` 單次 word trigger，以及 684／728 cycles-per-line 與第 240 線 vblank。
- 真實資料：首次穩定 VRAM 狀態有 5,587 個非零 byte、SHA-256 `53bf5e…81d2`；palette
  與 sprite／ROZ／window／video flags registers 均由真實卡帶寫入。
- Timeline 接入 scanline 後，`$FFDBB0` 的 `$F00000` poll 依時間自然離開；後續 VRAM
  非零資料增加至 7,344 bytes。未用 PC、ROM hash 或讀取次數特判 vblank。
- 68000 同步補上初始化路徑需要的 TST、SUBI.L、EXT.L、MULU／DIVU、displacement／
  predecrement MOVE 與 long shift。DIVU 成功 timing 暫採 140-cycle worst-case，已在文件
  標為待收斂，不冒稱精確。
- 最終真實 smoke 無未知 opcode 完成 1,300,000 條 68000 指令與 1,524,044 條 65C02
  指令；video frame=88、scanline=69、video flags=`$120E`，VRAM SHA-256
  `b0b2d6d8a8a77e71928ef88c0980f57493b0e3279634a0956753b0887c80f255`。

## 2026-09-01：UM6618 第一版 framebuffer

- 新增純 Go 320×240 ARGB 合成器，涵蓋三層 tilemap、sprite／mask、window、ROZ、
  layer priority 與 256／320 顯示寬度；vblank 起點合成，不讓前端控制晶片時間。
- 合成回歸驗證 xBGR-555、window 邊界、blanking、非黑像素計數與 8／4／2bpp tile
  packing；完整 `go test ./...`、競態檢查及 `go vet ./...` 在固定 Go Docker image 通過。
- 固定 IPL／Boom Zoo 執行 1,300,000 條 68000 指令後，frame 88 有 61,437 個非黑
  像素，framebuffer SHA-256 為
  `89ce08232bcfc61c396b514a981057b69ae7cf19733a4c3a247a051fc64684ee`。
- 此結果只證明 Go 合成路徑可重現且非黑；sprite DMA、逐行 ROZ、IRQ 與相同 frame
  archived oracle 差分尚未完成，未宣稱像素正確。

## 2026-09-01：UM6618 sprite DMA bus master

- 將 `$F00010–$F0001E` 建模為同步 16-bit bus master，實作 `count+1`、來源／目的
  word stride、零填充及 VRAM 目的高位模式；所有 transaction 可由 machine observer 看見。
- 合成回歸驗證兩 word copy 與單 word zero-fill；真實 Boom Zoo 1,300,000 指令 smoke
  的 CPU、VRAM 與 framebuffer 指紋不變，表示既有啟動路徑沒有被新 DMA 模型破壞。

## 2026-09-01：UM6618 IRQ 與 68000 autovector

- UM6618 新增 vblank IRQ7、可視線 raster IRQ4、可程式 line-on／line-off IRQ5 與最高
  level 仲裁；acknowledge 採 HOLD_LINE 清除來源。
- 68000 新增 instruction-boundary IPL 採樣、level 7 rising-edge latch、44-cycle autovector、
  supervisor SR／PC stack frame、RTE，以及真實 handler 所需 `ADDQ.W #n,(xxx).L`。
- 第一輪真實 smoke 在第 96,156 條指令進入 ROM IRQ7 handler，因 `$5279` 明確停止；
  補齊 ADDQ.W／RTE 後可再次完成 1,300,000 條指令，實際 acknowledge IRQ7 58 次。
- IRQ 接入後 VRAM 與 framebuffer SHA-256 不變；IRQ4／5 acknowledge 為 0，故目前只
  標為合成驗證。user-mode USP／SSP 切換與一般 exception 仍未完成。

## 2026-09-01：ROZ 逐行參數表

- 依 MAME-derived HACK 契約加入 `$198／$19A／$19E` 三表，逐行調整 incxx、scrollx、
  scrolly，並實作 incxx table 值 0 時整行不畫及 mode bit 9 bypass。
- 合成測試驗證 register 到 word index 的 `<<2` byte-address 換算、16／32-bit wrapping
  加法與 line suppression。
- Boom Zoo 固定 frame 88 非黑像素仍為 61,437，但 framebuffer SHA-256 從 `89ce…`
  改為 `14449f1ba85c25a01b0466fa2b8b735b4dcef571c44a808faf75ac37f894a232`；這推翻
  「該固定狀態不受逐行表影響」的舊推測。硬體正確性仍待同狀態 oracle 差分。

## 2026-09-01：第一個 Ebitengine 產品入口

- 固定 Ebitengine v2.9.9 與 `go.sum`，新增 `cmd/acan`、`frontend.Game`、ARGB→RGBA
  上傳、`System.RunFrame` deadline、`--frames` 有界終止及 `--screenshot` PNG。
- Xvfb 真實 Boom Zoo 88-frame smoke 完成 1,294,949 條指令，framebuffer SHA-256
  `14449f…4a232` 與 headless core 基準一致，證明 GUI 沒有另走簡化 scheduler。
- 人工檢查 PNG 顯示重複藍灰圖樣，判定 renderer 仍錯、模擬器尚不可玩；保留此負面
  證據作下一輪同 frame oracle 差分入口，不以非黑畫面冒稱完成。

## 2026-09-01：Ebitengine 輸入、音訊與可辨識 GUI smoke

- P1 鍵盤已接入 machine controller；新增 UMC6619 原生樣本到 48 kHz stereo 的主機
  音訊橋接，以及 200 ms、有界、執行緒安全的 PCM 佇列。缺料補靜音、溢位丟棄最舊
  樣本，不讓主機播放狀態影響模擬器時間線。
- 新增 `--audio=false`，使無音效裝置的 Docker／CI 仍走相同 Ebitengine GUI 與 machine
  core；新增 PCM byte order、underrun 與容量上限單元測試。
- 新增 `docker/ebitengine.Dockerfile`，固定 Go 1.26.7 與 Linux X11／OpenGL／ALSA／
  Xvfb 建置依賴。容器內 `presentation`、`frontend`、`cmd/acan` 測試通過。
- 三款 ROM 均在 Xvfb 完成 1200 frames；Speedy Dragon 18,515,145、Formosa Duel
  19,272,069、Boom Zoo 17,370,088 條 68000 指令，且 framebuffer SHA-256 與各自
  headless 基準完全相同。人工檢查分別可辨識道路角色、標題／START 與房間場景，
  推翻早期「GUI 仍只有錯誤重複圖樣」現況。
- 建置實證顯示 Ebitengine v2.9.9 Linux 桌面在 `CGO_ENABLED=0` 失敗、啟用 cgo 成功；
  CPU／machine／chip 仍純 Go，前端 cgo 例外是否正式允許仍待使用者決策。

## 2026-09-01：交接、cgo 政策定案與 Bcan oracle 路線

- 接手前任未提交的 Ebitengine 前端成果。先在 `superacan-ebitengine:go1.26.7-v1` 容器內
  （`--network none`、唯讀 module cache）跑完 `go build ./...`、`go vet ./...`、
  `go test ./...` 全數通過，才把該批成果提交為 `fda2fe4`。
- cgo 政策定案：**整個發行 binary 禁止 cgo，前端不例外**。依賴實測支持這個決定的代價：
  Ebitengine v2.9.9 的 `internal/glfw` 只有 darwin／windows 走 purego，linbsd 是 cgo；
  `oto/v3@v3.4.0` 的 `driver_unix.go`（ALSA）同樣是 cgo。因此現行 `cmd/acan` 只能作
  開發用 GUI，發行前需另建純 Go 的視窗／輸入與音訊輸出層。machine／CPU／chip 不受影響。
- 畫面正確性的主要 oracle 由 archived C++ 改為 Bcan 0.0.8b。理由是證據優先序：Bcan 是
  `confirmed-Bcan` 等級的固定版本二進位，archived C++ 與 MAME driver 同屬更低一級。

## 2026-09-01：Bcan 畫面 oracle 管線與 5 位元調色盤展開

- 建立可重現的 oracle 管線：`docker/bcan-oracle.Dockerfile`（Ubuntu 24.04、wine64 9.0、
  Xvfb、openbox、xdotool、ImageMagick、Mesa）與 `docker/bcan-oracle.sh`。Bcan 0.0.8b 的
  F8 截圖直接取自 UM6618 顯示孔徑，輸出固定 320×240 PNG，與本專案的 framebuffer
  可逐像素比較。版權輸入全部外部掛載，不進映像也不進版控。
- 環境限制實測：Xvfb 無視窗管理員時 Wine 收不到 xdotool 鍵盤事件（需 openbox）；
  Ctrl+O 無效，開檔必須點選單；Bcan 沒有 argv 載入 ROM 的路徑。
- 工具面新增 `acan-headless --screenshot-dir/--screenshot-every`（單次執行輸出多張取樣
  幀）與 `cmd/acan-imgdiff`（逐像素比對、目錄搜尋最接近幀、差異遮罩、`--width` 限制
  比較欄數）。
- 第一個定案差異：5 位元調色盤分量展開。同一像素 Bcan 輸出 `21/10/73`，本專案輸出
  `20/10/70`，反推分量 R=4、G=2、B=14；Bcan 等於 `v<<3 | v>>2`，MAME 宣告的
  `palette_device::xBGR_555` 亦同。已改為 `expand5` 並補上不依賴商業 ROM 的回歸測試。
  Boom Zoo 開場同一張 oracle 截圖差異由 42.51%／平均 13.09 降到 15.03%／10.54。
- 256 模式的右側 64 欄兩邊語意不同（本專案輸出黑、Bcan 填滿 320 欄），實測 frame 600
  的 6,119 個差異像素落在 `x ≥ 256`。這是孔徑處理差異不是圖層錯誤，比較一律加
  `--width 256`；硬體真相仍列 unknown，不依 Bcan 截圖改寫 renderer。
- 1200-frame framebuffer SHA-256 三款全部更新，指令數不變：Speedy Dragon
  `d3e533…5b7d67`、Formosa Duel `085626…404d587`、Boom Zoo `3784f8…94155562`。
- 阻擋項：Boom Zoo 在第 1,695 個 frame（第 24,181,668 條指令、PC `$007D2E`）遇未實作
  opcode `$D06A`（`ADD.W (d16,A2),D0`）停止，走不到靜止的標題選單，因此還無法做
  沒有動畫相位干擾的定案差分。
- 同時盤點到 `cpu/m68k` 目前是 233 個「操作×大小×定址模式」個別 case 的結構，逐一
  補 case 沒有終點；已列入 worklist 要求先做一般化 EA 執行層的設計再決定重寫。

## 2026-09-01：CPU 一般化執行層與八款 ROM 全部可執行

- 先量完整缺口再動手：八款商業 ROM 同時執行，每一款停在不同的未實作 opcode，橫跨七個
  指令族。逐一補 case 沒有終點，因此改成先解析 effective address、再由指令族共用讀寫
  路徑。68000 新增 `ea.go` 與四個 generic 檔，65C02 改以 256 項指令表覆蓋完整指令集。
  兩邊都只在既有逐一 case 判定為未知時才進入，既有已釘住的行為與時序不變。
- 一般化層上線後暴露出三個獨立缺陷，都靠證據定位而不是猜：
  - `$E90004/05`、`$E9000C/0D` 原本落到 open-bus 回 `$FF`，Formosa Duel 的 IRQ handler
    因此永遠認為有取樣 DMA 請求，把 `$FFFF` 當請求位址寫進 `$FFE094`，主程式組出
    `A6=$00E8FFFF` 撞上奇數位址。依 acan 位址表接上 sound RAM `$040C/$040D` 與 `$040A`。
  - 位址暫存器被截成 24 位元。The Son of Evil 在 `$08729C` 的
    `MOVE.W (A4)+,(A5)+` 配 `CMPA.L #$FFFFA122,A5` 因此永遠不相等，整台機器停在該迴圈，
    VRAM 一個位元組都沒寫入，畫面全黑卻不報錯。遮罩改為只在 bus 存取與跳躍目標套用。
  - 暫存器位移的長字時間應為 8 + 2n，先前沿用位元組與字的 6 + 2n。
- 新增有界的 `machine.InstructionRing` 與 `--trace-instructions`：停止時印出最近 N 條指令
  的 PC、opcode 與週期。上面兩個缺陷都是靠它把停止點回推到真正的原因。
- 結果：八款 raw ROM 全部完成 3600-frame 有界執行；帶 START／A／B 輸入的 5400-frame
  路徑也全部完成，人工檢查看到實際遊戲畫面（Journey to the Laugh 的平台場景、
  Speedy Dragon 的關卡、Super Taiwanese Baseball League 的比賽畫面、The Son of Evil 的
  遊戲中對話）。八款的 Ebitengine GUI 與 headless 在 1200 frame 的指令數與 framebuffer
  SHA-256 完全一致。
- cgo 缺口盤點：`CGO_ENABLED=0` 下 headless 與 imgdiff 任何平台都能建置，`cmd/acan` 的
  `js/wasm` 與 `windows/amd64` 也能建置，只有 `linux/amd64` 失敗。禁 cgo 政策的缺口
  因此只剩 Linux 桌面的視窗／輸入與音訊輸出層。
- 仍未收斂：Sango Fighter 走到選角畫面但沒進對戰（Bcan 同輸入會進），The Son of Evil
  在 frame 3600 有單張雜訊畫面，Boom Zoo 標題與 Bcan 差 43.48%（調色盤值相同、落點不同）。

## 2026-09-01：純 Go X11 前端補上禁 cgo 政策的最後一塊

- 先量清楚缺口：`CGO_ENABLED=0` 下 headless 與 imgdiff 任何平台都能建置，`cmd/acan`
  的 `js/wasm` 與 `windows/amd64` 也能建置，只有 `linux/amd64` 失敗。Ebitengine 的
  `internal/glfw` 在 darwin 與 windows 走 purego，linbsd 才是 cgo；音訊的 `oto/v3`
  同樣只有 unix driver 用 cgo。因此缺口只在 Linux 桌面，不是整個前端。
- 新增 `frontend/x11` 與 `cmd/acan-x11`：以 `jezek/xgb` 建視窗、`GetKeyboardMapping`
  取 keysym（不寫死 keycode）、ARGB framebuffer 整數倍放大後依 `MaximumRequestLength`
  切條 `PutImage`。音訊重取樣成 48 kHz 16-bit stereo 後寫進外部播放程序的 stdin。
- 八款 ROM 在 Xvfb 內以 X11 前端跑 1200 frame，68000 指令數與 framebuffer SHA-256
  與 headless 及 Ebitengine 前端三者完全相同。
- 用 `--layer-mask` 加新的 `--video-registers` 把 Sango Fighter 選單缺文字定位到 ROZ
  圖層：字形正確但被畫到 `x≈300` 之後切掉。該 frame 的 ROZ 暫存器 scroll 全零、
  `incxx`／`incyy` 都是 1:1。在能取得 oracle 同一瞬間的暫存器之前不動 renderer。

## 2026-09-01：例外路徑、雙部分卡帶、卡帶存檔、P2 與 save state

- 68000 補上統一的例外進入點，接上 TRAP、TRAPV、CHK、除以零、特權違例、ILLEGAL 與
  line-A／line-F。SR 的寫入集中到 `setStatusRegister`，S 位元改變時交換 A7 與
  `InactiveSP`，MOVE USP 因此可以實作。「68000 定義為非法」與「我們還沒實作」分開：
  前者產生例外，後者維持 fail-closed。八款既有 ROM 的 1200-frame 指紋完全不變。
- `media.DecodeCartridge` 接受 raw 與 ZIP。雙部分卡帶依尺寸排序而不是檔名——流通版本
  的成員檔名被改過，尺寸則由 Bcan 的驗證規則固定。補上 CMPM 的一般化實作之後，
  `Super Dragon Force (Taiwan).zip` 成為第九款可執行的卡帶，標題畫面為
  「SUPER DRAGONFORCE ©1996 KINGFORMATION」。
- 卡帶電池記憶體可存讀（`--save`），兩個 GUI 前端補上 P2 鍵位（沿用 Bcan.ini 配置）。
- 新增 `ACANGOS1` 存檔格式：每個裝置有 Snapshot／Restore，載入是交易式的，
  四種壞檔都有測試守著。真實 ROM 驗證：Boom Zoo 在 frame 600 存檔、另一個行程載入後
  續跑 600 frame，指令數與 framebuffer SHA-256 與連續跑 1200 frame 完全相同。
- 九款卡帶重跑 3600 frame：八款既有的數字一個位元組都沒變，第九款新增。
- UI 規劃寫入 `docs/ui-plan.md`：自繪的 `ui` 套件把介面畫進 RGBA 緩衝，兩個前端只負責
  貼圖與翻譯輸入，因此可在 headless 比對畫面雜湊，也不會讓 UI 滲進模擬路徑。


## 2026-09-01（續）：UI 設計定案與 `ui` 套件 P1

- UX 設計產出 `docs/ui-design.md`：Bcan 功能逐項對照、十六張畫面線框、三平台差異化、
  互動模型、視覺規範、文案表、設定檔、P0–P8 分階段與可用畫面雜湊驗證的驗收條件。
- 六項決定全部定案（紀錄在該文件 §15）：cgo 禁令縮為「Linux 與 macOS 的發行 binary」
  而 **Android 開例外**；嵌入 `bitmapfont/v4` 並做五種介面語言；錄影預設
  MP4／MJPEG＋PCM、OpenH264 為選配；金手指進第一個發行版但啟用時畫面常駐標記且該
  工作階段的雜湊不作硬體證據。
- Android 的關鍵事實是量出來的，不是估的：`-buildmode=c-shared` 在 linux、darwin、
  android 三個目標上都回「requires external (cgo) linking, but cgo is not enabled」，
  而 Android 應用的原生碼一定要是共享程式庫，所以禁 cgo 之下沒有產出 Android 應用的
  路徑。對照組是同一份程式建成**執行檔**在 android/arm64 成功——核心跑得動，
  不能成立的是應用程式形式。
- macOS 反而比原估便宜：Ebitengine 的 `internal/cocoa`（367 行）與 Metal 驅動
  （3,252 行）已經走 `purego/objc`，darwin 的 cgo 幾乎只剩 GLFW 的 Cocoa 視窗。
- 新增 `ui` 套件（P1）：抽象事件與 Intent、`compact`／`touch` 兩套度量、十二色主題、
  點陣字繪製、S3 覆蓋選單、S4 存檔槽、toast 與錯誤列、D1 確認對話。
- 三個雜湊擋不住的版面錯誤是靠 `ACAN_UI_DUMP` 存出 PNG 用人眼抓到的：字型基線多加
  一次 ascent 讓整行下墜、面板高度沒把分隔線算進去讓最後一列被外框切掉、整頁畫面
  沿用帶 alpha 的面板色讓下層畫面透出來。**畫面雜湊只證明沒有意外變動，
  證明不了版面本來就對。**
- `text-off` 由 `#5A646E` 改為 `#68727C`：原值對面板的對比只有 2.5:1，達不到設計自己
  訂的 3:1。對比檢查已寫成測試。
- `machine` 抽出 `ParseSaveState`，`LoadState` 與新增的 `InspectSaveState` 共用，
  存檔槽畫面的拒絕理由與實際載入的錯誤字串同源，測試直接比對兩者。
- 新增 `docker/go.sh`：Go 工具鏈在容器內跑，模組來源是主機下載快取的唯讀
  `file://` proxy，解壓與建置快取寫在容器外的工作目錄，不動主機的 `~/go/pkg/mod`。

- 新增 `session` 套件把核心與介面接起來：`ui.Snapshot` 與 `ui.SlotSource` 的實作、
  Intent 執行、遊戲畫面與覆蓋層的合成。它相依 machine 與 ui 但不相依任何前端，
  所以三個前端共用同一條流程，而流程本身在沒有視窗的容器裡就能驗證。
- `--ui-script` 用抽象事件名（`menu`、`down`、`confirm`…）而不是按鍵餵事件，
  headless 與 X11 兩個入口共用同一份解析。Boom Zoo 實跑：frame 600 開選單存到
  槽 0、frame 904 讀回，結束時 `video_frame=896`（600＋296）而不是 1200——
  讀檔沒生效的話這個數字會是 1200。
- 腳本以主機迴圈次數計時而不是模擬 frame 數。第一版用 frame 數當索引，
  覆蓋層一開模擬時間就停住，腳本永遠等不到下一個事件，整個程式卡死到逾時。
- X11 前端接上覆蓋層：`PresentRGBA` 走不放大的送圖路徑，原本的放大路徑保留給
  沒有覆蓋層的一般情況。Xvfb 內跑完 900 frame 並產生存檔；覆蓋層沒開時三款卡帶
  各跑 1200 frame，指令數與 framebuffer SHA-256 與 headless 完全相同。
- 互動鍵位暫定 F1 開選單。Esc 改成「開啟選單」還沒拍板（WORKLIST A1），
  在那之前 Esc 維持「離開」。

- P2–P8 與 P4 全部完成，介面階段收尾。新增套件：`session`（核心與介面的接線與
  唯一的 Intent 執行點）、`cheat`（Work RAM 搜尋與檔案格式）、`capture`
  （AVI/MJPEG＋PCM 錄影）。
- 每一階段的驗收都是可重跑的測試，不是人工檢查；畫面雜湊記在
  `docs/verify-ui.md`，卡帶基準同一份文件。

### 這幾輪抓到而雜湊擋不住的問題

- 字型基線多加一次 ascent、面板高度沒算分隔線、整頁畫面沿用帶 alpha 的面板色。
  三個都要把畫面存成 PNG 用眼睛看才發現。**畫面雜湊只證明沒有意外變動。**
- `--frames` 只數真正跑掉的 frame，覆蓋層開著時它不前進，所以腳本用 frame 數當
  索引會卡死；改用主機迴圈次數，另加 `--max-ticks`。
- 載入卡帶會在同一次 `Advance` 裡讓暫停變成執行中，呼叫端不能用「呼叫前是否暫停」
  推斷有沒有跑掉一個 frame。X11 的 frame 計數就是這樣少算一個。
- AVI 的長度欄位在 `Close` 才回填。第一次真機錄影沒有收尾，得到 1.5 MB、資料完整、
  標頭全是 0 的檔案。手算的標頭位移也錯了兩個，改成寫入時記錄位移。
- 版面溢出測試第一次跑就抓到八處：法文與西班牙文在觸控版面把文字畫出畫面。
  中文短，這類問題在中文下完全看不出來。

### 本輪收尾

- HEAD：`716e9ad`。
- 驗證：`docker/go.sh test ./...` 全綠（含 `ui` 的三十餘組畫面雜湊、五種語言 ×
  十六個畫面的版面溢出檢查、`session` 的存讀檔與金手指界線、`cheat` 與 `capture`
  的格式測試）；`vet` 無輸出。九款卡帶 1200-frame 重跑，指令數與 framebuffer
  SHA-256 與 `docs/verify-ui.md` 記錄的完全相同——介面沒有滲進模擬路徑。
  X11 前端在 Xvfb 內走過：叫選單存讀檔、從啟動畫面經瀏覽器載入卡帶、改鍵位寫設定檔、
  從選單開始錄影。
- 未證實：觸控層只有離線渲染與事件測試，沒有在真實 Android 上跑過；macOS 與
  Android 平台層都還沒開始；音訊輸出仍靠外部播放程序。
- 下一個最小行動：C 節的平台層——先做 macOS 的 purego 視窗，再做 Android。
- Docker 清理：本輪全部 `docker run --rm`；一個逾時的 Xvfb 容器由 `docker stop`
  停掉（本輪自己建立的），沒有留下本專案容器。

## 2026-09-02：macOS purego 視窗層、歷史清理與 README 介面畫面

- macOS 前端不走 Ebitengine。量過才決定：`CGO_ENABLED=0 GOOS=darwin` 會停在 Metal
  的 `view_macos`／`displaylink_macos` 與 GLFW，而這一層真正需要的只有「開視窗、
  貼圖、收鍵盤」。`frontend/cocoa` 用 `purego/objc` 直接呼叫 Objective-C runtime：
  `NSWindow` 開窗、每幀把畫面包成 `NSBitmapImageRep` 取 `CGImage` 設進
  `layer.contents`、`nextEventMatchingMask:` 收鍵盤、`objc.RegisterClass` 註冊視窗代理。
- 兩個刻意的取捨寫在 `docs/macos-frontend.md`：`NSBitmapImageRep` 不複製 plane，
  所以前端自己留一份像素；`hasAlpha:false` 配 `bitsPerPixel:32`，第四個 byte 是填充。
- 桌面兩個入口共用的主機端 I/O 抽成 `frontend/hostio`（載入、截圖、卡帶存檔、
  save state、音訊與錄影 sink），全部回傳 error 不自行結束程式。`cmd/acan-x11`
  從 542 行降到 385 行，三款卡帶 1200-frame 重跑與基準逐位元相同。
- macOS 鍵碼用 `kVK_*` 常數並附單元測試：虛擬鍵碼描述的是實體位置不是印上去的字母，
  照字母猜會在非 QWERTY 版面錯位。**沒有實機 smoke**，交叉編譯只證明組得起來。
- 歷史清理：`9b0971d` 誤入的 9.3 MB Mach-O（`acan-macos`）以 `git filter-branch
  --index-filter` 從全部 107 個 commit 移除後 force push。改寫在獨立 clone 內做，
  主工作樹只用 `git reset --soft` 對齊，避免碰到另一個工作階段的未提交變更。
  驗證：107 筆 commit 的作者／日期／訊息逐筆相同、HEAD 根樹雜湊一致
  （`5d6f385c…`）、最大 blob 從 9,291,378 降到 102,410（`docs/ui-design.md`）、
  `.git` 15 MB → 1 MB。改寫前的完整歷史留了一份 bundle。
- README 新增介面畫面：五張 PNG 由 `cmd/acan-headless --ui-compose` 產生，
  與桌面視窗共用同一個 `session.Compose`，所以不是另外畫的示意圖。重現命令寫進
  `docs/verify-ui.md`。headless 的診斷畫面補上 `FrontendName = "headless"`——
  原本那格是空的，而診斷畫面的職責就是回報事實。

### 教訓：這個 repo 同時有別的工作階段在動

- **`git add -A` 在共用工作樹裡不安全**，`git commit -a` 同理。要提交什麼就逐一
  列出路徑；`git status --porcelain` 先看過，不是自己改的就不要碰。
  `9b0971d` 就是這樣同時掃進建置產物與別人未提交的 143 行 sprite 幾何重構。
- **別人的未提交變更被誤提交之後不要 revert。** revert 會把那份工作從樹上刪掉，
  比錯誤的歸屬更糟。正確處置是留著、驗證輸出等價（九款卡帶 1200-frame 重跑
  指令數與 framebuffer SHA-256 全數相同）、把事實寫下來。
- **建置輸出寫在 repo 根目錄時，`.gitignore` 要跟著新入口一起加。** 當時只列了
  `/acan`、`/acan-x11`、`/acan-headless`，新的 `/acan-macos` 沒人補。
- **commit message 說「寫進 WORKLOG」就要真的寫。** `f231f0a` 的訊息這樣寫，
  但那個 commit 只動了 `.gitignore`，紀錄到這一輪才補上。

### 本輪收尾

- HEAD：`5acfe63`（改寫後），與 `origin/master` 同步。
- 驗證：`docker/go.sh vet ./cmd/... ./session/... ./ui/...` 無輸出；
  `docker/go.sh test ./ui/ ./session/ -count=1` 全綠。介面畫面五張逐張目視檢查過。
- 未證實：macOS 實機 smoke（步驟在 `docs/macos-frontend.md`）；Android 平台層未開始；
  音訊輸出仍靠外部播放程序。
- 下一個最小行動：把十六個熱鍵動作接上入口（目前只有「開啟選單」有效），
  再進 Android 平台層。
- Docker 清理：本輪全部 `docker run --rm`；一次 Bash 逾時中斷的 headless 執行由
  `--rm` 自行清掉，`docker ps` 確認沒有殘留本專案容器。

## 2026-09-02（續）：熱鍵動作接線

- 十七個動作全部走 `ui.Hotkey(action)`：介面改自己的設定、送出 Intent、留下提示，
  前端只把「這個鍵剛按下／剛放開」翻譯成動作名稱。分派放在 `ui` 而不是每個入口，
  所以熱鍵不會出現一條繞過 Intent 邊界的捷徑，X11 與 macOS 也不必各寫一份。
- 生效條件寫死在同一個地方：覆蓋層開著時只有 `menu` 有作用，等待指定綁定時全部
  不生效。前者擋的是「方向鍵與 Enter 同時有兩種意思」，後者擋的是「已經被佔用的
  鍵永遠指定不到」。
- 暫停要與「選單開著所以暫停」分開記。原本 `Close()` 無條件把 paused 設成 false，
  用熱鍵暫停之後開一次選單再關掉，暫停就被吃掉了。
- 按住型全速放開時回到鎖定狀態決定的速度，不是無條件回到實時——否則按一下全速鍵
  會把鎖定解掉。
- 音量不再只是設定畫面上的數字：`Session.Volume()` 給出這一刻該送到主機音訊的
  百分比（含 `MuteOnFastFwd`），`hostio.AudioSink` 依它縮放。縮放做在每 10 ms 送出
  的那一段，不做在一秒四萬多次的取樣回呼裡；回呼裡讀設定等於把介面狀態拉進模擬
  迴圈。錄影拿到未縮放的樣本——靜音是監聽控制，不該讓檔案跟著變成無聲。
- FPS 是常駐指示不是 toast，畫在右上角並與金手指標記讓開。
- 設定畫面顯示的鍵位改成「實際生效的鍵」：入口用 `SetDefaultHotkeys` 把出廠鍵位
  交給介面。加了出廠鍵位卻讓 S5.2 繼續只顯示設定檔內容的話，F5 明明會存檔而畫面
  上是空的。
- headless 腳本加 `hk<動作>`／`hkup<動作>`，整條路因此在沒有視窗的容器裡驗得完。
  Boom Zoo 實跑：存檔 600／讀檔 950 得到 `video_frame=850`（600＋250），暫停 600
  停在 600，暫停 600／恢復 900 得到 900，換兩次槽存檔落在 `slot2.acanstate`。
  三條的 `ui_visible` 都是 false，量到的是熱鍵本身不是選單。

### 看 PNG 才看得到的：長清單根本沒有捲動

- 十七個熱鍵在觸控版面（`RowHeight` 44）一頁放不下，多出來的四列畫到畫面外，
  焦點移過去之後也看不見——**那四個動作沒辦法重新指定**。畫面雜湊完全穩定，
  因為畫出畫面外的列不在畫布上。
- 這不只是 S5.2：`ui` 裡除了 S1 卡帶瀏覽器之外，沒有任何清單會捲動，而金手指
  清單上限是一千多筆。把瀏覽器那段抽成 `listWindow`，S5.1、S5.2、S6.2 都改用它，
  清單沒到底時右側畫 ▲▼。
- 教訓與前幾輪同一條：**畫面雜湊只證明沒有意外變動，證不了版面本來就是對的**。
  這一輪是把 S5.2 的 PNG 存出來用眼睛看才發現。

### 順帶修正

- `docs/verify-ui.md` 的 C10 基準表，Super Taiwanese Baseball League 的非黑像素
  是 45,084 不是 45,080。`a834338` 更新 ROZ 那一輪的雜湊時漏了這一格，
  同一列的雜湊與像素數互相矛盾了一段時間。
- 全螢幕是唯一沒有實際效果的熱鍵：設定與熱鍵都會切換 `Video.Fullscreen`，
  但兩個視窗層都還沒有全螢幕。列成 WORKLIST 自己的一項，不假裝它會動。
  這一項不能只靠容器內建置就宣告完成——bare Xvfb 沒有視窗管理員，
  EWMH 的 client message 不會有人處理。

### 本輪收尾

- HEAD：`67177ba` 之後。
- 驗證：`docker/go.sh test ./...` 全綠；`vet ./...` 無輸出；darwin/arm64、
  darwin/amd64、linux/amd64 的 `CGO_ENABLED=0` 建置與 android/arm64 的核心建置
  都通過。九款卡帶 1200-frame 重跑，指令數與 framebuffer SHA-256 與基準相同——
  熱鍵沒有滲進模擬路徑。
- 未證實：全螢幕；macOS 實機 smoke；Android 平台層未開始；音訊仍靠外部播放程序。
- 下一個最小行動：Android 平台層（gomobile，cgo 例外）。
- Docker 清理：本輪全部 `docker run --rm`，`docker ps` 沒有殘留本專案容器。

## 2026-09-02（續）：Android 平台層——能驗證的部分

- **cgo 例外是量出來的不是選出來的。** `GOOS=android CGO_ENABLED=0` 之下
  `gomobile/internal/mobileinit` 與 `ebiten/internal/vibrate` 整批檔案被 build
  constraint 排除；開了 cgo 但沒有 NDK，則會拿主機 gcc 去編 arm64 組語而失敗。
  兩段錯誤訊息抄進 `docs/android-frontend.md`。
- **`ebiten/v2/mobile` 在 linux＋cgo 下可以建置**，所以 `mobile/acan` 的型別檢查與
  vet 在這台機器上跑得動。這只證明組得起來。
- 依「會被實機打臉的」與「現在就能釘住的」把 Android 前端切成兩層：
  - `frontend/mobile`（純 Go，`GOOS=android CGO_ENABLED=0` 建置通過並有單元測試）：
    表面尺寸政策與檔案位置。Scale 取顯示密度，因為介面的最小觸控目標是 44 設計
    單位，Scale＝密度時那 44 單位剛好是 44 dp；低密度小螢幕上照密度取值會讓設計
    單位不夠放控制項，這時降 Scale——按鍵小一點還能按，控制項疊在一起就不能用。
  - `mobile/acan`（需要 NDK）：`ebiten.Game`、觸控、音訊與 gomobile 匯出的四個
    Java 入口。
- 生命週期補在 `session` 而不是 `ui`：要落地的東西屬於主機端，`ui` 不碰檔案。
  `LifeSuspend`／`LifeResume`／`LifeFocusLost`／`LifeFocusGained` 四個事件本來就
  定義在 `ui.LifeKind` 裡，但**沒有任何地方處理**——只有 `LifeBack` 有。
- 離開前景寫回卡帶電池記憶體，然後叫出覆蓋選單；回到前景不自動恢復執行。
  叫選單不是只設暫停旗標，是因為凍住的畫面配上沒有說明的介面看起來像當掉，
  而且要有一個明顯的「繼續遊戲」。設定與金手指在每次變更時就寫過，所以離開前景
  只需要處理隨時在變的電池記憶體。桌面前端不送這些事件：視窗失去焦點就跳出選單
  在桌面上是干擾，而桌面有正常的結束流程可以寫檔。
- 返回鍵沿用 `ui.handleBack`（一律吃掉），所以**返回鍵不會關掉應用程式**：
  遊戲中開選單、選單中退一層、退到根畫面停住，要離開用選單的「離開模擬器」。
  這與桌面 Esc 目前不一致，WORKLIST A1 還沒拍板。

### 沒做的與為什麼

- `ebitenmobile bind` 沒有跑過，APK 與實機行為全部未驗證。缺 Android NDK，而
  補上工具鏈是一個約 5–6 GB 的 image（JDK 17＋cmdline-tools＋platform 34＋
  build-tools＋NDK）。**這台機器同時放著其他專案的 image，清理映像不是本專案能
  自行決定的事**，所以先報數字再問，不自己開下去。列成 WORKLIST 自己的一項。
- 沒有提交跑不起來的 Gradle 專案。Activity 的四個呼叫與 manifest 需求寫進
  `docs/android-frontend.md`，工具鏈就位時照著做即可。

### 本輪收尾

- HEAD：`8ff5cc8`（另一個工作階段的 FRC／window 1 修正）之後。
- 驗證：`docker/go.sh test ./...` 全綠；`vet ./...` 無輸出；`frontend/mobile`、`ui`、
  `session` 在 `GOOS=android CGO_ENABLED=0` 下建置通過；`mobile/acan` 在 linux＋cgo
  下建置與 vet 通過；darwin 兩個架構的 `CGO_ENABLED=0` 建置通過。九款卡帶
  1200-frame 重跑與基準相同——本輪沒有動到 `machine`／`cpu`／`chip` 任何一個檔。
- 未證實：Android 實機一切；macOS 實機 smoke；全螢幕；音訊仍靠外部播放程序（桌面）。
- 下一個最小行動：等 Android 工具鏈的決定；在那之前可做的是三平台發行包與第三方
  授權清單。
- Docker 清理：本輪全部 `docker run --rm`，`docker ps` 沒有殘留本專案容器。

## 2026-09-02（續）：Linux AppImage 與展示影片

- **AppImage 不需要 appimagetool。** type-2 的結構就是「static-pie 的 runtime ELF」
  接上「一個 squashfs」，所以在沒有網路的容器裡自己組就好：`mksquashfs` 之後
  `cat runtime payload > out.AppImage`。runtime 從既有的 AppImage 前面切下來——
  切點要用 `--appimage-offset` 印出來的長度，**不能 grep squashfs 的 `hsqs` 魔數**：
  那四個位元組會在 runtime 內部出現，切出來的檔案看起來像 ELF，`file` 卻會抱怨
  section header 落在檔案之外。
- 打包／編碼工具鏈另建一個 image（`docker/package.Dockerfile`，從 Go image 延伸），
  只多 `squashfs-tools`、`ffmpeg`、`xvfb`、`file`，實測多 264 MB。
- 發行包要能直接點兩下就開，所以 `--ipl`／`--key`／`--rom` 全部變成選用，
  預設走 XDG（`~/.local/share/superacan-emu/`）；讀不到韌體不再是啟動失敗，
  改由啟動畫面列出四份韌體各自的狀態與雜湊。
- 驗證：AppImage 在 Xvfb 內跑 300 frame 得到
  `instructions=4364786 framebuffer_sha256=defbd19a…885c6`，與 headless 基準相同，
  所以打包流程沒有改變執行檔的行為。
- 影片錄的是**合成後的視窗**（遊戲畫面加覆蓋層），與截圖那條路不共用：後者取的是
  UM6618 顯示孔徑、不含覆蓋層，是給畫面證據用的。合成這條**不能當畫面證據**。
  取樣節奏是主機迴圈而不是模擬 frame，否則走選單那一段在影片裡會是靜止的。

### 這一輪修掉的兩個真缺陷

- **AVI 音訊串流的時間單位用錯。** 原本 `dwScale=1`／`dwRate=每秒位元組`／
  `dwSampleSize=1`，等於「一個位元組是一個取樣」。那樣自洽、也讀得回原始資料，
  但不是 VfW 的慣例：解碼器算不出串流長度（ffprobe 的 duration 是 N/A），
  有視訊一起轉檔時音訊會被截掉——量到的是 68.3 秒的畫面配 17.1 秒的音訊。
  改成 `dwScale=dwSampleSize=nBlockAlign`、`dwLength` 以 block 計之後，
  轉檔後兩條串流都是 70.000 秒。
- **合成錄影的音訊只補不削。** 覆蓋層開著時沒有樣本要補靜音，這一半原本就有；
  但重取樣一幀不會剛好是 800 個取樣，多出來的不削掉會單向累積——量到四百幀多
  181 個 block，換算到一分鐘就是幾十毫秒的偏移。

### 教訓

- **錄影／驗證之前先重建。** 影片的內容來自 AppImage 裡的執行檔，不是工作區裡的
  原始碼。修好音訊補齊之後直接重錄，錄到的是舊行為，而影片看起來完全正常，
  只有量音訊長度才發現。`packaging/promo.sh` 現在會印出它用的 AppImage 雜湊。
- **腳本不要依賴會累積的狀態。** 存檔槽索引是設定檔裡的值，換卡帶不重設；腳本
  切過一次「下一個槽」之後，後面每一片卡帶都跟著偏，偏到不存在的槽就跳出錯誤列，
  而錯誤列會吃掉下一個確認鍵（那是正確行為），整條腳本從那裡開始失準。
  現在只用槽 0。
- **逐幀看過才算驗證。** 四次錄影裡，音畫長度、存檔槽偏移、按鍵時機三個問題都是
  把影格存成 PNG 用眼睛看才發現的；檔案大小、退出碼、幀數全部正常。

### 本輪收尾

- HEAD：`0adf532` 之後（另有另一個工作階段的 FRC／window 1 修正在同一分支）。
- 驗證：`test ./...` 與 `vet ./...` 全綠；九款卡帶 1200-frame 與基準完全相同；
  影片 4200 幀／70.000 秒，音訊 70.000 秒，AVI 的 PCM 恰為 4200×3200 位元組，
  音量 mean −16.9 dB（不是靜音）；十一個時間點的影格逐張目視檢查過。
- 未證實：Android 實機一切；macOS 實機 smoke；全螢幕。
- **不可對外散布**：OFL-1.1 的授權原文尚未進包（見 `packaging/THIRD-PARTY-LICENSES`）。
- 下一個最小行動：補 OFL-1.1 原文，或等 Android 工具鏈的決定。
- Docker 清理：本輪自建 `superacan-package:v1`（比基礎 image 多 264 MB），
  其餘全部 `docker run --rm`；`docker ps` 沒有殘留本專案容器。

## 2026-09-02（續）：macOS 與 Android 的發行包

- **macOS 不需要 osxcross。** 那套工具鏈補的是 SDK 標頭檔、Mach-O 連結器與旗標
  wrapper 三樣，而本專案的 macOS 執行檔是 `CGO_ENABLED=0` 的純 Go，Go 自己就產
  Mach-O，三樣都用不到。剩下的只有兩件檔案格式操作，用標準庫寫：
  `packaging/macho fat` 合成 universal binary、`packaging/macho check` 做靜態驗收、
  `packaging/zipdir` 壓縮並保留權限位元。
- fat binary 的對齊要照架構的頁大小（arm64 是 2^14、x86_64 是 2^12）。寫錯的話
  dyld 會拒絕載入，而檔案本身看起來完全正常。
- **arm64 的 `LC_CODE_SIGNATURE` 是硬條件**：Apple 從 arm64 開始強制簽章，沒有簽章
  的執行檔在 Apple Silicon 上會被核心直接殺掉（`Killed: 9`），在 Linux 這端一點
  異狀都看不到。Go 的連結器會自己加 ad-hoc 簽章——但要驗過才知道。實測有。
- Android 的工具鏈 image 5.69 GB（新增層約 2.9 GB）。AAR 28.9 MB，APK 28.5 MB，
  三個 ABI，簽章 v1/v2/v3 皆通過，min 21 / target 34。APK 不走 Gradle：只有一個
  Activity、一份 manifest 與一張圖示，用 build-tools 的 `aapt2`／`d8`／`zipalign`／
  `apksigner` 就組得完。

### 三個「錯誤指著別的地方」的坑

- `gomobile init` 要寫 `$GOPATH/pkg/gomobile`。容器以非 root 執行時 image 內的
  `/go` 不可寫，錯誤是 `mkdir /go/pkg/gomobile: permission denied`。
- **gomobile 會把 Go 的文件註解原樣搬進產生的 Java。** 本專案的註解是中文，而
  JDK 17 的預設來源編碼跟著 locale 走。沒設 locale 的容器裡 javac 對每個非 ASCII
  位元組報 `unmappable character`，825 個錯誤全指著 `Acan.java`——看起來像
  gomobile 產出壞檔，其實是 image 少了 `LANG=C.UTF-8`。
- `javac` 的 `-bootclasspath` 只在 `-source 8` 以下可用，錯誤是
  `option --boot-class-path not allowed with target 11`。Android 的平台類別要由
  `android.jar` 提供而不是 JDK 的，所以只能用 source 8 這條路。

### 本輪收尾

- HEAD：`8111de7`（macOS）之後再加這一次。
- 驗證：`test ./...`、`vet ./...` 全綠。macOS：`file` 認定雙弧 universal binary、
  arm64 有 `LC_CODE_SIGNATURE`、最低系統 12.0.0、相依只有 `/usr/lib` 底下兩個。
  Android：簽章 v1/v2/v3 通過、`native-code` 三個 ABI、`libgojni.so` 內找得到
  `superacan-emu/chip/umc6618.*` 與 `ACANGOS1`、五種語言的字串——證明這份 binary
  真的含這一份程式碼。
- 未證實：**兩個包都沒有在實機上跑過。** macOS 的 bundle 未簽（`codesign` 只在
  macOS 上有），Android 用除錯金鑰簽只夠側載。靜態驗收全過只代表不會因為結構
  問題開不起來，不代表功能正常。
- **三個平台的包都還缺 OFL-1.1 原文**，在補上之前不可對外散布。
- 下一個最小行動：補 OFL-1.1 原文；之後是 CI 守住三個平台的建置。
- Docker 清理：本輪自建 `superacan-android:v1`（5.69 GB），其餘全部
  `docker run --rm`；`docker ps` 沒有殘留本專案容器。

## 2026-09-03：發行、指標輸入與授權改為 RRSAL-1.0

### 發行

- OFL-1.1 原文補進 `packaging/THIRD-PARTY-LICENSES` 之後，公開散布的最後一個
  阻礙解除。`v0.1.0-preview` 發出 AppImage、macOS universal `.app.zip`、APK 與
  `SHA256SUMS`；展示影片另掛在固定 tag `promo` 的附件。
- **`gh release upload file#name` 設的是 label，不是下載檔名**，要換檔名得先改
  本機檔名。**`releases/latest/download/…` 對 prerelease 回 404**（`latest` 會跳過
  prerelease），README 的連結因此指向固定 tag。
- APK 改用發行金鑰簽章。`apksigner` 的 `--ks-pass` 與 `--key-pass` **共用同一個
  讀取器，一行讀一個密碼**：只寫一行時第二次讀取撞到檔尾，錯誤訊息卻是
  `Failed to read Key … password`，看起來像別名錯了。
- 本機另留一份含 BIOS 與九款卡帶的完整包，不進版控也不對外。

### 指標輸入

- 覆蓋層改成 immediate-mode 命中區：每次 `Draw` 重建，由後往前掃描讓對話框先拿到
  事件，按下與放開要落在同一區才算數。頁面共用的返回鍵在標題列註冊，所以每個
  page-based 畫面都自動有返回。X11 前端補上 `ButtonPress`／`ButtonRelease`／
  `PointerMotion`，只接受 button 1。
- 執行中畫面補了一個會自動消失的開選單提示，按鍵名從綁定表取，不寫死 F1。
  **計時器不能拿「時間為 0」當「還沒開始」的哨符**——單調時鐘在第一幀就是 0，
  提示因此永遠不會過期；改用一個明確的布林旗標。
- 選單列維持不做。理由與 Bcan 選單列的對照截圖寫在 `docs/ui-design.md` §4.0 與
  README。

### 授權改為 RRSAL-1.0

- 自有程式碼由 MIT 改為 RRSAL-1.0（SPDX `LicenseRef-RRSAL-1.0`）：非商業免費含
  修改與再散布，商業使用需事先書面授權，實況／錄影／教學／論文引用明列為非商業。
- 這個儲存庫不是遊戲 remake，所以條款裡的「原版素材」對應到 ROM、BIOS 與遊戲
  內容；第 2 條 (c) 的灰色地帶列的是 `docs/screenshots/` 的執行畫面截圖、
  `docs/screenshots/bcan/` 的 Bcan 介面截圖，以及量測所得的暫存器與調色盤數值表。
  條款本文只換占位符，未改動任何條件。
- 授權要跟著包走，四個位置：儲存庫的 `LICENSE`、README 的授權段、AppImage 的
  根目錄與 `usr/share/doc/superacan-emu/`、`.app` 的 `Contents/Resources/`。
  **APK 是例外**——使用者翻不到包裡的檔案，所以改在 S8 關於畫面顯示授權名稱、
  非商業條件與取得全文的位置（五種語言）。S8 的畫面雜湊因此改變，`verify-ui.md`
  的 C10 表同步更新。
- 對外文件一律寫 source-available，不寫開源：非商業限制不符合 OSI 的定義，寫成
  開源會讓人以為可以商用。README 第一段的「開源硬體模擬器」因此改掉。

### 作者身分改寫與重新發行

- 131 個 commit 裡有 58 個掛公司信箱 `cy.wang@coretronic.com`，全部改寫成
  `Chun-Yu Wang <wicanr2@gmail.com>`（`git filter-branch --env-filter`，author 與
  committer 都改）。改寫後逐一比對：131 個 commit 位置一一對應、每個位置的 tree
  雜湊相同、訊息與 author／committer 兩個時間戳完全一致，所以只有身分欄改變。
- **只 force push 分支不夠**：tag 會把舊 commit 釘住不放，GitHub 那邊的舊身分
  因此還在。`v0.1.0-preview` 與 `promo` 兩個 tag 連同 release 都要刪掉重建。
- 三個發行包全部重建，帶新的 `LICENSE` 與關於畫面的授權行；`SHA256SUMS.txt`
  重新產生。APK 用同一把發行金鑰簽（憑證 SHA-256 `df0a3988…ba5a` 不變，所以裝得
  上去覆蓋舊版）。
- **AppImage 的執行檔是 `-ldflags "-s -w"` 建的**，文件原本漏寫。少了這兩個旗標
  執行檔 7.9 MB → 10.2 MB，AppImage 跟著大 2 MB；發現的方式是重建後檔案莫名變大，
  不是讀文件讀出來的。文件已補。
- 重建後的 AppImage 跑 300 frame 得到 `instructions=4364786`
  `framebuffer_sha256=122922cb…c71198`，與 `verify-ui.md` 的 headless 基準相同，
  所以重建沒有改變行為。

### 展示影片重錄：介面走完整套

- 影片同時是介面的展示，所以覆蓋層底下每個畫面都走一遍（存檔槽存／讀、金手指、
  設定六個子畫面、診斷）加上啟動畫面的「關於」，共十六站，每站約 200 tick。
  長度從 70 秒變成 113 秒。
- **確認鍵是 A，而且要按住約三十幀**。`--press` 預設的十幀在這兩款遊戲的標題選單
  上沒有反應，`START` 完全沒有作用——舊腳本按的就是 `START`，所以舊影片其實從來
  沒有進到遊玩畫面，只是停在標題。這件事是抽格看出來的：把候選按鍵排在不同 tick
  上錄一小段，再看哪一格之後畫面變了。
- **載入或讀檔之後不能馬上按**：畫面淡入時的按鍵會被吃掉，實測要隔約 400 tick。
  第一次探測失敗就是因為讀檔後 100 tick 就按 A。
- 預建存檔改存在**遊玩中**而不是標題畫面。從標題按到可操作要一千多 tick，那段
  等待不必出現在影片裡。
- 長度定在 6800 tick：Monopoly 的標題約在 6850 tick 自己淡出進 attract mode，
  錄到 7000 會以一片純色收尾。
- 兩份 MP4 併成一份。兩份的分工來自「一份進版控、一份留全畫質」，影片改掛 Release
  附件之後就沒有進版控的那一份了。
- 驗收方式是**抽 18 格拼成聯絡表用眼睛看**，不是看總長度或檔案大小。腳本裡每個
  `down` 的數量都是從「上一次焦點停在哪」算的，改一段就要重算後面所有段落，
  只有逐格看才抓得到偏移。

### 本輪收尾

- HEAD：`72e4f34`（改寫後 `fd586fa`）之後再加這一次。
- 驗證：展示影片 113.333 秒、6800 幀、960×720、60 fps，音訊與視訊長度相符；
  十六個介面站點逐格確認。`test ./...`、`vet ./...` 全綠；S8 的畫面雜湊由
  `fb18139c…f39b81` 變為 `f99ad055…0f1842`，是本輪唯一改變的畫面。AppImage
  300-frame 與 headless 基準逐位元相同。三個包各自確認帶著 RRSAL-1.0 的 `LICENSE`，
  APK 的 `libgojni.so` 內找得到 `RRSAL-1.0` 字串。
- 未證實：macOS 與 Android 兩個包仍未在實機上跑過。
- 下一個最小行動：CI 守住三個平台的建置與 `CGO_ENABLED=0`。
- Docker 清理：本輪全部 `docker run --rm`（`docker/go.sh`、一次 `gofmt`、
  android／package 兩個 image 各數次），`docker ps` 沒有殘留本專案容器。

## 2026-09-03（續）：README 補上 Bcan 與 BillyJr 的來歷

「專案起源」原本只寫「手邊能用的模擬器是 Bcan 0.0.8b 與 MAME 的 driver」，
沒有交代 Bcan 是誰做的，也沒寫這個 repo 為什麼在 2026-08-31 開第一個 commit。
本輪把這條線補完，並新增「致謝」一節。

### 來源與時間軸

- Bcan 的作者是 BillyJr（T客邦報導寫作 Billy Jr）。2004 年第一版模擬器，
  2010 年併進 MESS，2026-08-30 釋出 0.0.8b。
- T客邦〈31年之後，聯電傳奇主機 Super A'Can 模擬器終於完美達成，重溫 1995 年
  國產 16 位元巔峰榮光〉，李文恩，2026-08-31 15:30，
  <https://www.techbang.com/posts/132758-super-acan-emulator>。
- 本 repo 第一個 commit 是 2026-08-31，與報導同日，時間軸相符。
- 上述年份與釋出日期都只有這一篇報導佐證，屬二手資料，README 內已標明。
  BillyJr 的 Facebook 粉絲團原文與 Mega 載點尚未查證。

### 本輪收尾

- HEAD：`ff08d90` 之後再加這一次；改動只有 `README.md`（+25／−3）與本檔。
- 驗證：無程式碼改動，未跑測試。README 兩處改動已逐段複讀；報導連結以 WebFetch 實抓，標題與日期取自該次回應。
- 未證實：BillyJr 的年份、MESS 併入時間與 0.0.8b 釋出日期均引自單一報導，
  未取得一手來源。
- 下一個最小行動：CI 守住三個平台的建置與 `CGO_ENABLED=0`（沿用上一輪）。
- Docker 清理：本輪未起容器。
