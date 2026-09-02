# 介面畫面驗證

更新日期：2026-09-01。

介面的迴歸靠兩種雜湊：**畫面雜湊**守住版面不被意外改動，**卡帶基準**守住介面沒有
滲進模擬路徑。兩者都由 `go test ./ui/` 與 `go test ./machine/` 自動比對，
數值同時記錄在這裡供人查閱。

## 畫面雜湊

覆蓋層畫在表面的原生解析度，所以雜湊會隨表面大小改變。固定兩組表面，
其餘尺寸不入驗證：

| 設定檔 | 表面 | Scale |
|---|---|---:|
| `compact` | 960×720 | 1 |
| `touch` | 1280×720 | 1 |

取像方式：先把固定的測試 framebuffer 放大鋪滿整個表面，再畫覆蓋層，
對整張 RGBA 的 `Pix` 取 SHA-256。測試資料是固定的漸層與固定的存檔槽現況
（槽 0、3 可讀，槽 7 被拒絕，其餘為空），所以雜湊只受介面本身影響。

| 畫面 | 表面 | SHA-256 |
|---|---|---|
| S3 覆蓋選單 | 960×720 `compact` | `ff406b886d278a0c70600e20d8b53161713df90cdf58434635ab5ef9546e5ef0` |
| S3 覆蓋選單 | 1280×720 `touch` | `46eef64f92a9890101d108c61e033ab431b5b08fd9570c3a798bdfc98a84a924` |
| S3 經 Down×3、Confirm、Cancel | 960×720 `compact` | `73ad19293711c6a1b51c6d86f6f20f63d68fdcf8592f990d8c09ecb14ce243ca` |
| S4 存檔槽（存檔模式） | 960×720 `compact` | `8279ec7eade67dca4b70e3cca03deb3f0bd9cd8573a170119cb0a06f52d007cc` |
| S4 存檔槽（存檔模式） | 1280×720 `touch` | `ce19280911c7abca897d29aacabcd241ac646e01f5a713029a45721b6b8273e0` |
| D1 覆寫確認 | 960×720 `compact` | `e1bf95ba55c979941a854d0e07d8a964b630a26e51bf28c5e81b99a6eb4c72e6` |
| S0 啟動（韌體不齊） | 960×720 `compact` | `84e731b98b74525e2414a15c7df179c9b787ce415a11b8a12a1bc55e16a1f65a` |
| S0 啟動（韌體不齊） | 1280×720 `touch` | `fa17df07234c345ca9edeafef255dbe9c1ad6094d7b3588e2b18c3fe21794446` |
| S0 啟動（韌體齊備） | 960×720 `compact` | `8ca7a1e7fbfb1a66eaedae4c2d31498a08da1f6915e698f6e42048b74c092148` |
| S0.1 主機韌體 | 960×720 `compact` | `90adda11beebb2cb6fb7529b7177f8522a8b42b372ba37ca0fc94cc1deef90bf` |
| S1 卡帶瀏覽器 | 960×720 `compact` | `82cae5f1eceefee32895587a3972ea42c1e5b03a3d4ba14a5cb39fbda23bddca` |
| S1 卡帶瀏覽器 | 1280×720 `touch` | `64274509659f2a7e7a637d197f9b264efba212fedc72d26eed4816902e41d1f9` |
| S8 關於 | 960×720 `compact` | `fb18139c4ac701d5187e9110dd0e9b9e485382eba23a78775b03e2725ff39b81` |
| S9 停機 | 960×720 `compact` | `c06f549d0b21a9caba9422a89cef873cc66bb867a0eef0dc3899991d2533e1bf` |
| S5 設定 | 960×720 `compact` | `c48bf197a895e9b46313a21d23dae7e98577ebcd7231e6afb7799228d16056cc` |
| S5.1 輸入綁定 | 960×720 `compact` | `1ef20869b7fb4a4442f58beb90bd1b1f7717e0aec4555344c66ef5e5258fdb62` |
| S5.2 熱鍵 | 960×720 `compact` | `2cccd2b67daf8a5e36be3d14722c3037ab6ed2b87a55384c36350d8e1964d4f8` |
| S5.2 熱鍵（含衝突標示） | 960×720 `compact` | `107ecbc854d803c375197eec83f6c68097dd35778cc307b06767ad6b07f0ddbf` |
| S5.3 影像 | 960×720 `compact` | 見 `ui/render_test.go` |
| S5.4 音訊 | 960×720 `compact` | 見 `ui/render_test.go` |
| S7 診斷 | 960×720 `compact` | 見 `ui/render_test.go` |

畫面雜湊的權威來源是 `ui/render_test.go` 的 `wantHashes`；這張表列出主要畫面供
查閱，兩者不一致時以測試為準。

雜湊只能守住「沒有意外變動」，看不出版面本來就畫錯。要用人眼檢查時把畫面另存
PNG：

```sh
ACAN_UI_DUMP=/src/build/uidump docker/go.sh test ./ui/ -count=1
```

版面刻意改動時，先看 PNG 確認新版面正確，再把新雜湊填回 `ui/render_test.go`
與這張表——順序反過來就等於用雜湊掩蓋錯誤。

## 在 headless 驗證覆蓋層

「叫出選單、存檔、讀檔」這條流程不靠人在視窗前面按一次來證明。`session` 套件把
模擬核心與介面接在一起，`cmd/acan-headless` 用 `--ui-script` 餵抽象事件，因此整條
流程在沒有視窗的容器裡就能跑完並比對。

事件名稱是介面層的動作而不是按鍵（`menu`、`up`、`down`、`left`、`right`、
`confirm`、`cancel`、`delete`、`secondary`、`tabprev`、`tabnext`、`home`、`end`、
`back`），格式與 `--press` 一樣是 `frame:事件`。

```sh
docker/go.sh run ./cmd/acan-headless --ipl /bios/internal_68k.bin --key /bios/umc6650.bin \
    --sound-bios1 /bios/internal_6502_1.bin --sound-bios2 /bios/internal_6502_2.bin \
    --rom "/media/Boom Zoo (Taiwan).bin" --frames 1200 \
    --ui-state-dir /gowork/states \
    --ui-script "600:menu,601:down,602:confirm,603:confirm,604:cancel,\
900:menu,901:down,902:down,903:confirm,904:confirm" \
    --ui-compose /gowork/ui-boomzoo.png
```

2026-09-01 的結果：

```
ui_visible=false ui_halt=0 ui_slot=0 present=true rejected=false
steps=13048709 video_frame=896
```

`video_frame=896` 是這條驗證的關鍵：迴圈跑了 1200 次，但 frame 600 存檔、
frame 904 讀檔之後機器退回存檔當下，再往前 296 個 frame，600＋296＝896。
如果讀檔沒有真的生效，這個數字會是 1200。`ui_slot=0 present=true` 則說明存檔槽
畫面讀到的檔案通過了與實際載入同一份驗證。

不需要商業 ROM 的版本在 `session` 的單元測試裡：`TestMenuSaveAndLoadRoundTripHeadless`
走完同一條流程，`TestLoadStateResumesIdentically` 證明從存檔續跑與一路跑到底的
指令數、frame 與 framebuffer SHA-256 完全相同，`TestOverlayGatesPadInput` 證明選單
開著時 machine 收到的是「全部放開」。

## README 的介面畫面

README 的介面畫面不是手動截的，是 `--ui-compose` 的輸出，因此可以重現。
`cmd/acan-x11`、`cmd/acan-macos` 與 headless 共用同一個 `session.Compose`，
所以這些 PNG 與桌面視窗上的像素是同一份合成結果。

```sh
export ACAN_MEDIA_DIR=…/Bcan008b/ROMS ACAN_BIOS_DIR=…/bios
BIOS="--ipl /bios/internal_68k.bin --key /bios/umc6650.bin \
      --sound-bios1 /bios/internal_6502_1.bin --sound-bios2 /bios/internal_6502_2.bin"

# 存檔槽畫面要有東西可看，先在槽 0 存一次
docker/go.sh run ./cmd/acan-headless $BIOS \
    --rom "/media/Monopoly - Adventure in Africa (Taiwan).bin" --frames 3620 \
    --ui-state-dir /gowork/states-shot \
    --ui-script "3600:menu,3601:down,3602:confirm,3603:confirm"

# ui-menu-boomzoo.png
docker/go.sh run ./cmd/acan-headless $BIOS --rom "/media/Boom Zoo (Taiwan).bin" \
    --frames 6010 --ui-state-dir /gowork/states-shot --ui-script "6000:menu" \
    --ui-compose /src/build/uishots/ui-menu.png

# ui-slots-monopoly.png（選單第 2 項＝讀檔）
docker/go.sh run ./cmd/acan-headless $BIOS \
    --rom "/media/Monopoly - Adventure in Africa (Taiwan).bin" --frames 3620 \
    --ui-state-dir /gowork/states-shot \
    --ui-script "3600:menu,3601:down,3602:down,3603:confirm" \
    --ui-compose /src/build/uishots/ui-slots.png

# ui-settings.png（第 5 項）、ui-diagnostics-boomzoo.png（第 6 項，ACAN_CGO=0）
# ui-touch-monopoly.png：--ui-touch --ui-surface 1280x720，不開選單
```

兩個欄位在 headless 路徑是固定值，不是缺陷：存檔槽時間戳固定成 `01-01 00:00`
（時間戳是環境不是介面行為，固定它 `ui_sha256` 才可比），診斷畫面的前端固定
回報 `headless`。診斷的 cgo 欄位跟著建置走，README 的那張以 `ACAN_CGO=0` 產生，
對應 Linux 與 macOS 實際發行的建置。

## 熱鍵

十七個動作都走 `ui.Hotkey`：介面改自己的設定、送出 Intent、留下提示，前端只把
「這個鍵剛按下／剛放開」翻譯成動作名稱。因此熱鍵不會有一條繞過 Intent 邊界的
捷徑，也不必在每個前端各寫一份。

生效條件寫在同一個地方：覆蓋層開著時只有 `menu` 有作用（其餘動作在選單裡有自己
的入口，兩套同時生效會讓方向鍵與 Enter 有兩種意思），等待指定綁定時全部不生效
（否則已經被佔用的鍵永遠指定不到）。

### 出廠鍵位

X11 與 macOS 用同一組功能鍵。沒有列出的動作預設不綁鍵——它們要嘛只在診斷時用，
要嘛容易誤觸，而這一層還沒有修飾鍵的概念；使用者可以在 S5.2 自己指定。

| 鍵 | 動作 | 鍵 | 動作 |
|---|---|---|---|
| F1 | 開啟選單 | F8 | 截圖 |
| F2 | 暫停／繼續 | F9 | 開始／停止錄影 |
| F3 | 重設主機（軟） | F10 | 顯示／隱藏 FPS |
| F4 | 靜音 | F11 | 全螢幕 |
| F5 | 存檔到目前槽 | F12 | 載入卡帶 |
| F6 | 下一個存檔槽 | Tab | 全速（按住） |
| F7 | 從目前槽讀檔 | — | 上一個槽、全速鎖定、鎖定金手指、循環圖層遮罩 |

`cmd/acan-x11` 給了 `--state` 時，F5／F7 維持舊的單檔存讀路徑，兩個槽位熱鍵讓開，
同一個鍵不會有兩種存檔語意。

### 在 headless 驗證

腳本用 `hk<動作>` 送出熱鍵、`hkup<動作>` 送出按住型的放開，所以整條路在沒有視窗
的容器裡就能跑完。2026-09-02 對 Boom Zoo 的實跑：

| 腳本 | `--frames` | `video_frame` | 判讀 |
|---|---:|---:|---|
| （無） | 1200 | 1200 | 基準 |
| `600:hksave_state,950:hkload_state` | 1200 | 850 | 讀檔生效：600＋(1200−950) |
| `600:hkpause` | 1200 | 600 | 暫停停住模擬時間 |
| `600:hkpause,900:hkpause` | 1200 | 900 | 再按一次恢復 |
| `600:hknext_slot,601:hknext_slot,602:hksave_state` | 700 | 700 | 檔案落在 `slot2.acanstate` |

三條都不開選單（輸出的 `ui_visible=false`），所以量到的是熱鍵本身而不是選單。

不需要商業 ROM 的版本在測試裡：`ui` 的 `TestEveryHotkeyActionIsImplemented` 守著
「S5.2 列得出來、按下去卻沒作用」，`TestHotkeysAreInertWhileTheOverlayIsOpen` 與
`TestHotkeysAreInertWhileRebinding` 守著生效條件；`session` 的
`TestHotkeySaveAndLoadRoundTripHeadless`、`TestHotkeyPauseStopsEmulatedTime` 與
`TestHotkeyFastForwardDoesNotChangeEmulatedWork` 走完整條路，最後一條同時證明
全速改的是主機節奏而不是模擬器的時間線：同樣的 frame 數要得到同樣的指令數。

### 音量與 FPS

音量不是設定畫面上的裝飾：`Session.Volume()` 是這一刻該送到主機音訊的百分比，
`hostio.AudioSink` 依它縮放送出的樣本。縮放做在每 10 ms 送出的那一段，不做在
一秒四萬多次的取樣回呼裡。錄影拿到的是未縮放的樣本——靜音是監聽控制，不該讓
錄下來的檔案跟著變成無聲。全速時依 `MuteOnFastFwd` 靜音。

FPS 是常駐指示不是 toast：`ui.drawFPS` 在 `Video.ShowFPS` 為真時畫在右上角，
與金手指標記讓開。headless 沒有主機迴圈節奏，所以那裡顯示 `0.0 FPS`。

### 長清單

清單比畫面長是常態：熱鍵十七個動作在觸控版面（`RowHeight` 44）一頁放不下，
金手指清單上限一千多筆。`listWindow` 算出這一次要畫哪一段並把焦點留在可視範圍
內，S5.1、S5.2 與 S6.2 都走它，清單沒到底時右側畫 ▲▼。S1 卡帶瀏覽器本來就有
自己的捲動。

畫面雜湊看不出這件事——畫出畫面外的列根本不在畫布上，雜湊一樣穩定。這一項是
把 PNG 存出來用眼睛看才發現的。

## 沒有卡帶時的啟動流程

`cmd/acan-x11` 的 `--rom` 變成選用：給 `--rom-dir` 就從 S0 啟動畫面開始，
走瀏覽器選卡帶。這條路在 Xvfb 內驗過，而且**從介面載入的卡帶與從命令列載入的
是同一台機器**：

```sh
DISPLAY=:99 acan-x11 --ipl … --key … --sound-bios1 … --sound-bios2 … \
    --rom-dir /media --state-root …/states --frames 300 --pace=false --scale 3 \
    --ui-script "5:down,10:confirm,20:confirm"
→ frames=300 instructions=4364786 framebuffer_sha256=122922cb…c71198

acan-headless --rom "/media/Boom Zoo (Taiwan).bin" --frames 300
→ steps=4364786 framebuffer_sha256=122922cb…c71198
```

不需要商業 ROM 的版本在 `session` 的單元測試裡：`TestShellBrowsesAndLoadsHeadless`
用自製的 raw 檔與雙部分 ZIP 走完「啟動畫面 → 瀏覽器 → 載入 → 退出卡帶」，
`TestIncompleteFirmwareBlocksBrowserLoad` 確認韌體不齊時載不進去。

## 影像、音訊與診斷

四條規則各有測試（`session` 套件）：

- **濾鏡作用在放大後的畫面上，不改 framebuffer。** `TestFilterDoesNotReachScreenshots`
  比對套 scanline 前後的截圖位元組必須相同——截圖直接取自 UM6618 的顯示孔徑，
  所以它可以當畫面證據用。
- **三段 scanline 各自產生不同且可重現的畫面**（`TestScanlineFiltersAreDistinct
  AndReproducible`）。未知的濾鏡名稱視為不套濾鏡而不是猜一個：設定檔可能來自更新
  的版本。測試會先把 framebuffer 畫成漸層——全黑的畫面套 scanline 之後還是全黑，
  那證明不了濾鏡有作用。
- **圖層遮罩只影響 framebuffer 合成**（`TestLayerMaskDoesNotChangeMachine`）：
  套用前後指令數與 frame 相同。畫面上同時以 warn 提醒「套了遮罩的畫面雜湊不可拿來
  對帳」。
- **音量與緩衝不回饋核心**（`TestAudioSettingsDoNotFeedBackIntoTheCore`）：
  改音量與緩衝之後，UM6619 送進 sink 的樣本序列雜湊相同。

診斷畫面的數字直接讀 machine（`TestDiagnosticsReadMachineDirectly`），不另外累計，
所以它顯示的 68000 指令數就是 headless 報出來的那一個。

## 觸控層

虛擬手把與觸控版面在 Android 前端存在之前就先做完並驗證，理由是觸控度量會逼出
「這個面板在 44 單位列高下放不下」這類問題，晚發現要重畫每一個畫面。

- **命中區不小於 44 見方，而且每邊比繪製區大至少 4 單位**
  （`TestTouchTargetsAreLargeEnough`，兩個方向都驗）。手指會遮住按鍵，
  使用者看不到自己按在哪。
- **五個同時觸點全部生效**（`TestFiveSimultaneousTouches`）：方向＋兩鍵＋肩鍵是
  常見組合，只追一個觸點會讓斜向移動時按不出動作。放開其中一個不影響其餘。
- **方向鍵有死區、對角線是兩個位元**（`TestDPadDeadzoneAndDiagonals`）：
  死區太小會讓「按上」變成「上＋左」。
- **覆蓋層開著時虛擬手把隱藏且不吃觸點**（`TestVirtualPadHidesUnderTheOverlay`）：
  兩套控制同時存在會互相搶觸點。
- **橫式與直式各有記錄的畫面雜湊**（`touch/landscape/1280x720`、
  `touch/portrait/720x1280`）。橫式把按鍵疊在 4:3 畫面左右的黑邊上，
  直式把畫面貼齊上方、控制區獨占下半不與畫面重疊。

## 多語言

字串表是結構不是 map：少一個 key 會在編譯期就發現，不會變成畫面上的空白。
另有三條測試：`TestEveryLanguageHasEveryString` 斷言五種語言的每個欄位都有值、
`TestTranslationTableMatchesStruct` 斷言表與結構沒有多餘或缺少的 key、
`TestSwitchingLanguageChangesTheScreen` 斷言五種語言的同一畫面雜湊互不相同
——相同就表示字串沒有真的換掉。

**版面溢出有測試守著**：canvas 記錄每一幀文字畫到的最右邊，
`TestNoScreenOverflowsInAnyLanguage` 對十六個畫面 × 五種語言 × 兩組表面渲染，
斷言沒有文字掉出畫面。它第一次跑就抓到八處溢出（S5.3／S5.4／S7 在法文與西班牙文
的觸控版面），修法是加上 `textFit`：放不下就截斷並補省略號，而不是畫到容器之外
蓋掉別的欄位。

技術標識（IPL、SHA-256、tilemap0、CHEAT、48,000 Hz、frame）五種語言相同。
那不是沒翻譯，是不該翻。

## 擷取

- **截圖等同硬體輸出**（`TestScreenshotMatchesHardwareOutput`）：介面走的路徑與
  `--screenshot` 走的 `presentation.EncodePNG` 解出來的像素逐點相同。
- **外部接收端的位元組數 = 幀數 × 320 × 240 × 4**（`TestCaptureSinkReceivesRawFrames`）。
- **錄影不影響時序**（`TestCaptureDoesNotChangeTiming`）：開著錄影跑二十個 frame，
  指令數與 framebuffer 雜湊與沒錄影時相同。
- **沒按停止就結束也要是完整檔案**（`TestShutdownFinalisesAnOpenClip`）：AVI 的長度
  欄位在收尾才回填，少了這一步會得到資料完整但標頭全是 0 的檔案。真機第一次錄影
  就踩到這個，現在入口 defer `Session.Shutdown()`。

真機驗證：X11 前端跑 120 frame 並從選單開始錄影，得到 1.58 MB 的 AVI，
`RIFF` 大小欄位與檔案長度相符、`dwTotalFrames` 115、第一幀的 JPEG SOI 是 `FFD8`。

## 金手指的界線

金手指是 C5「UI 不改寫晶片狀態」的例外，所以界線要由測試守著，不是由文件宣告：

- **越界拒絕**（`TestPokeRejectsOutOfRangeAddresses`）：`$F40000`、`$FBFFFF`、
  `$FD0000` 三個位址各發一次 `PokeWorkRAM`，三次都被**入口**拒絕、記憶體不變、
  計數器不動。拒絕發生在 session 而不是 UI：UI 沒有寫入能力這件事要在那一層成立。
- **關閉時無寫入**（`TestNoPokesWhenCheatsAreDisabled`）：清單裡有鎖定項但金手指
  關閉時，跑三十個 frame 之後 `PokeCount()` 仍是 0。這是 C10 的前提——回歸基準
  必須在金手指關閉下取得。
- **只在 frame 邊界寫入**（`TestLockedCheatsWriteOncePerFrame`）：五個 frame 寫五次。
- **搜尋可重現**（`cheat.TestSearchNarrowsReproducibly`）：同一組快照序列跑兩次得到
  同一組候選；快照一律在 frame 邊界取，兩次快照之間的比較才有定義。
- **上限行為**（`cheat.TestSearchReportsTruncation`）：候選超過 4096 時只回 4096 筆
  並回報截斷，畫面上也說明「請再縮小一次範圍」——使用者要知道自己看到的不是全部。
- **標記可見**（`TestCheatMarkerIsVisibleWithoutOverlay`）：啟用之後畫面雜湊與未啟用
  時不同，而且覆蓋層關著也看得到。

`BCAN_CHT_1` 匯入的欄位順序是 **hypothesis**：從 `Bcan.exe` 只能確定檔案開頭是
`; Bcan per-game cheat file` 與 `BCAN_CHT_1`，以及欄位清單（Name／Address／Width／
Value／Format），逐欄順序沒有出現在字串表裡。實作採「名稱優先」，第一欄看起來像
位址時自動改用「位址優先」，兩者都不成立的行以警告回報並跳過，不猜。

## 設定檔與重新綁定

設定檔的四條規則各自對應一種失敗，四條都有測試：型別不符只讓那個欄位回到預設
（`TestTypeMismatchOnlyResetsThatField`）；未知的頂層鍵原樣保留
（`TestUnknownKeysSurviveARoundTrip`）——只忽略不保留的話，舊版寫一次設定就會把
新版的欄位刪光；整份無法解析時改名成 `config.json.bad` 再用預設值繼續
（`TestBrokenConfigIsRenamedNotOverwritten`）；寫入先寫暫存檔再改名。

綁定帶前端識別：同一個實體鍵在 X11 keysym 與 `ebiten.Key` 底下是不同數值，
所以每組綁定都記錄寫入它的前端字串，讀入時前端不符就不套用而回到預設
（`cmd/acan-x11` 的 `TestForeignFrontendBindingFallsBackToDefault`）。

整條「改綁定 → 寫檔」在真實 X11 視窗裡跑過。腳本的 `raw<十六進位鍵碼>` 事件會送出
一個原始按鍵，所以指定綁定這條路也能腳本化：

```sh
DISPLAY=:99 acan-x11 … --max-ticks 80 --config /tmp/settings.json \
    --ui-script "5:menu,10:down,12:down,14:down,16:down,18:down,20:confirm,\
25:confirm,30:down,32:down,34:down,36:down,38:confirm,42:raw71"
```

結果：`settings.json` 的 `input.players[0].keyboard.a` 變成
`{"frontend": "x11", "code": 113}`。`--max-ticks` 是必要的——`--frames` 只數真正
跑掉的 frame，而腳本會停在選單裡，沒有這個上限 smoke run 不會結束。

## X11 前端的覆蓋層

`cmd/acan-x11` 同樣吃 `--ui-script`，所以覆蓋層在**真實 X 伺服器**上的路徑
（`PresentRGBA` 的 RGBA→BGRX 轉換與切條 `PutImage`）也能在容器裡驗證，
不必有人坐在螢幕前。腳本以主機迴圈次數計時而不是模擬 frame 數——覆蓋層開著時
模擬時間停住，用 frame 數當索引的腳本會永遠等不到下一個事件。

```sh
Xvfb :99 -screen 0 1280x1024x24 &
DISPLAY=:99 acan-x11 --ipl … --key … --rom "…/Boom Zoo (Taiwan).bin" \
    --frames 900 --pace=false --scale 3 --state-dir …/states \
    --ui-script "600:menu,610:down,620:confirm,630:confirm,640:cancel"
```

2026-09-01 的結果：跑完 900 個模擬 frame，`slot0.acanstate` 產生，
`instructions=13117468`。互動鍵位：**F1 開選單**（Esc 改成開選單還沒拍板，
見 WORKLIST A1，在那之前 Esc 維持「離開」）、方向鍵導覽、Enter 確認、
Esc 或 Backspace 取消、Del 刪除、Tab 換頁籤。

覆蓋層沒開時走原本的放大路徑，畫面結果不受影響。三款卡帶各跑 1200 frame，
68000 指令數與 framebuffer SHA-256 與 headless 完全相同：

| ROM | 68000 指令 | framebuffer SHA-256 |
|---|---:|---|
| Boom Zoo | 17,369,003 | `f720c9d1…b92301` |
| Formosa Duel | 19,270,779 | `499d876d…9e7b0d` |
| Sango Fighter | 11,634,924 | `f5bfffa1…4f9f06` |

## 卡帶基準（C10）

介面不得滲進模擬路徑。每一個介面階段完成後，九款卡帶的 1200-frame 執行結果
必須與下表相同。

固定輸入與 3600-frame 結果見 [`verify-rom-matrix.md`](verify-rom-matrix.md)。

| ROM | 68000 指令 | 非黑像素 | framebuffer SHA-256 |
|---|---:|---:|---|
| Boom Zoo | 17,369,003 | 22,533 | `f720c9d12517beb1f2575a0d83675e2fca834d5df6018fddeeeab025f9b92301` |
| Formosa Duel | 19,270,779 | 76,800 | `499d876de1b6db15fac1efd1958f6c649624612045fd9f09276afdf0f39e7b0d` |
| Journey to the Laugh | 17,778,132 | 7,645 | `a4bba957604591f3ebf22ffd120e8680986c5adac80878259bf635cb139ef3c6` |
| Monopoly – Adventure in Africa | 11,827,355 | 76,800 | `71b754e4f0157bd29a6597755a9177685e93329a6b962f0c73515ce921dff892` |
| Sango Fighter | 11,634,924 | 50,390 | `f5bfffa115d32a45c6d2aacc7412829ace9ff5bbd97b23b6fdaf63d14f9f0606` |
| Speedy Dragon | 18,513,698 | 33,125 | `e03f86b9f076a8b003039f646e7a8334e5522457e70624c3051734ed6758bee0` |
| Super Taiwanese Baseball League | 17,572,195 | 45,071 | `6eecdc748f3d1440e8f2bf457b6e56d2d99509523745248c84cf47baebb64a33` |
| The Son of Evil | 16,727,440 | 32,274 | `8059d2e75a0e9526cb96dafa7b0f07cf207b519effe68803ad0ded95128616c3` |
| Super Dragon Force（雙部分 ZIP） | 23,988,611 | 5,066 | `b9bba41025f239508306cd4a8e6d58c632681e05b9e24c997f8695bb40e8eedf` |

這些數字綁在 renderer 的現況上。渲染路徑一改，指令數不動而 framebuffer 雜湊會動，
那不是迴歸而是預期——例如 `2d71080` 移除 ROZ 的整層翻轉之後，Monopoly、Speedy Dragon
與 Super Taiwanese Baseball League 三款的雜湊隨之改變，指令數一個位元都沒變。
介面階段要看的是「指令數與雜湊在**沒有改 renderer 的情況下**保持不變」。

重跑方式（ROM 與 BIOS 唯讀掛載，不入版控）：

```sh
export ACAN_MEDIA_DIR=…/Bcan008b/ROMS ACAN_BIOS_DIR=…/bios
docker/go.sh run ./cmd/acan-headless --ipl /bios/internal_68k.bin --key /bios/umc6650.bin \
    --sound-bios1 /bios/internal_6502_1.bin --sound-bios2 /bios/internal_6502_2.bin \
    --rom "/media/Boom Zoo (Taiwan).bin" --frames 1200
```
