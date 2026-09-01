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

## 沒有卡帶時的啟動流程

`cmd/acan-x11` 的 `--rom` 變成選用：給 `--rom-dir` 就從 S0 啟動畫面開始，
走瀏覽器選卡帶。這條路在 Xvfb 內驗過，而且**從介面載入的卡帶與從命令列載入的
是同一台機器**：

```sh
DISPLAY=:99 acan-x11 --ipl … --key … --sound-bios1 … --sound-bios2 … \
    --rom-dir /media --state-root …/states --frames 300 --pace=false --scale 3 \
    --ui-script "5:down,10:confirm,20:confirm"
→ frames=300 instructions=4364786 framebuffer_sha256=defbd19a…885c6

acan-headless --rom "/media/Boom Zoo (Taiwan).bin" --frames 300
→ steps=4364786 framebuffer_sha256=defbd19a…885c6
```

不需要商業 ROM 的版本在 `session` 的單元測試裡：`TestShellBrowsesAndLoadsHeadless`
用自製的 raw 檔與雙部分 ZIP 走完「啟動畫面 → 瀏覽器 → 載入 → 退出卡帶」，
`TestIncompleteFirmwareBlocksBrowserLoad` 確認韌體不齊時載不進去。

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
| Boom Zoo | 17,369,003 | `3784f866…155562` |
| Formosa Duel | 19,270,779 | `0856269e…04d587` |
| Sango Fighter | 11,634,924 | `412213da…9bd505` |

## 卡帶基準（C10）

介面不得滲進模擬路徑。每一個介面階段完成後，九款卡帶的 1200-frame 執行結果
必須與下表相同。

固定輸入與 3600-frame 結果見 [`verify-rom-matrix.md`](verify-rom-matrix.md)。

| ROM | 68000 指令 | 非黑像素 | framebuffer SHA-256 |
|---|---:|---:|---|
| Boom Zoo | 17,369,003 | 22,533 | `3784f8663b1c3a869498d2e14c0b948c598d50d15cf54b6f5380c9b294155562` |
| Formosa Duel | 19,270,779 | 76,800 | `0856269e7b402158e953de03d0553128d720ef64f29afc97403f93471404d587` |
| Journey to the Laugh | 17,778,132 | 7,645 | `42285d489bd74a5c5fd0d66700ed7e7c8b2b83f4855612d7dec4db07c30b146e` |
| Monopoly – Adventure in Africa | 11,827,355 | 76,800 | `39ae8de2ed83c23de1750e5535f347e2c2466a353f3c5c7f3a69cf595b248f86` |
| Sango Fighter | 11,634,924 | 50,390 | `412213dac64ec07ef8db6ee69f4a90a351880f11c3229b378647d05f559bd505` |
| Speedy Dragon | 18,513,698 | 33,125 | `b525266ca176f01613091f059391171fada19a1b3225c4b79fa55857583f2595` |
| Super Taiwanese Baseball League | 17,572,195 | 45,080 | `23bd031e2b487da63249d6de2b4da6c55a0ef7ce347f253f80b98309f0bb173b` |
| The Son of Evil | 16,727,440 | 32,274 | `bbd3a45fb5d27acf8e6caef06f5c9f7d00f8743d2ad42b0bbb8baea2d23bca73` |
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
