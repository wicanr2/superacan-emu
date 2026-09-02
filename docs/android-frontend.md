# Android 前端

更新日期：2026-09-02。

Android 版分成兩層：與平台無關的判斷放在 `frontend/mobile`，真正接上視窗、觸控與
音訊的那一層放在 `mobile/acan`。這樣分不是為了整齊——前者在
`GOOS=android CGO_ENABLED=0` 下就能建置與測試，後者需要 Android NDK，而這台
開發機沒有。把會被實機打臉的東西與現在就能釘住的東西分開，才不會因為缺一套
工具鏈就整包沒有任何驗證。

## cgo 是必要的，這是量出來的

`-buildmode=c-shared` 在任何平台都要求 cgo，而 Android 應用程式的原生碼必須是被
Java runtime 載入的共享程式庫。兩條路都試過：

```
$ GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build github.com/hajimehoshi/ebiten/v2/mobile
imports github.com/ebitengine/gomobile/internal/mobileinit:
    build constraints exclude all Go files in .../gomobile/internal/mobileinit
imports github.com/hajimehoshi/ebiten/v2/internal/vibrate:
    build constraints exclude all Go files in .../ebiten/v2/internal/vibrate

$ GOOS=android GOARCH=arm64 CGO_ENABLED=1 go build ...   # 沒有 NDK
# runtime/cgo
gcc_arm64.S: Assembler messages:
gcc_arm64.S:30: Error: no such instruction: `stp x29,x30,[sp,'
```

第一個是「禁 cgo 時整批檔案被 build constraint 排除」，第二個是「開了 cgo 但沒有
交叉編譯器，於是拿主機的 gcc 去編 arm64 組語」。所以 Android 的 cgo 例外不是偏好
問題，是這條路上唯一的走法；禁令因此限定在模擬核心與 Linux、macOS 的發行 binary，
見 [`platform-targets.md`](platform-targets.md)。

`ebiten/v2/mobile` 在 **linux + cgo** 下可以建置，所以 `mobile/acan` 的型別檢查與
`vet` 在這台機器上跑得動——**建置成功不等於跑得起來**，實機行為仍未驗證。

## 表面尺寸與縮放

`mobile.Surface(widthPx, heightPx, density)` 決定介面表面。Scale 取顯示密度：
介面的最小觸控目標是 44 設計單位，Scale 等於密度時那 44 單位剛好是 44 dp，也就是
Android 建議的最小觸控尺寸。密度高的螢幕因此得到「一樣大的按鍵、更清楚的字」，
而不是「一樣多的像素、更小的按鍵」。

低密度小螢幕上照密度取值會讓設計單位不夠放控制項（橫向要放得下置中的 4:3 畫面
加上兩側的方向鍵與面鍵），這時降低 Scale：按鍵小一點還能按，控制項疊在一起就
不能用了。下限是 640×360 設計單位，上限 Scale 4。

| 裝置 | 像素 | 密度 | Scale | 設計單位 |
|---|---|---:|---:|---|
| 1080p 手機橫向 | 2400×1080 | 3 | 3 | 800×360 |
| 1080×1920 | 1920×1080 | 2.625 | 3 | 640×360 |
| 720p | 1280×720 | 2 | 2 | 640×360 |
| 低密度小螢幕 | 854×480 | 1.5 | 1 | 854×480 |
| 平板 | 2560×1600 | 2 | 2 | 1280×800 |

`LayoutF` 回傳實體像素而不是 dp：介面在原生解析度上作畫，文字才不會被放大兩次。
方向改變時 `LayoutF` 送出新的 `ui.Surface`，觸控版面（直式／橫式）跟著重算。

## 生命週期與返回鍵

行動平台沒有「正常結束」：切走之後程式可能直接被系統回收。所以離開前景的那一刻
就是最後一次能寫檔的機會。

| 事件 | 來源 | 處置 |
|---|---|---|
| `LifeSuspend` | `onPause` → `Suspend()` | 寫回卡帶電池記憶體，然後叫出覆蓋選單 |
| `LifeResume` | `onResume` → `Resume()` | 不做事；選單留在畫面上 |
| `LifeBack` | `onBackPressed` → `Back()` | 交給 `ui.handleBack` |

叫出選單而不是只設一個暫停旗標，是因為回到前景時使用者要看得出來為什麼畫面不動，
而且要有一個明顯的「繼續遊戲」。凍住的畫面配上沒有任何說明的介面，看起來像當掉。
回到前景不自動恢復執行，理由同上：不要在人還沒看清畫面時就把他丟回操作中的遊戲。

`ui.handleBack` 一律回傳 true，所以**返回鍵不會把應用程式關掉**：在遊戲中它開啟
選單，在選單中往回退一層，退到根畫面就停住。要離開請用選單裡的「離開模擬器」。
這一點與桌面的 Esc 目前不一致，見 WORKLIST 的 A1。

設定、金手指在每次變更時就已經寫檔，所以離開前景只需要處理隨時在變的電池記憶體。
桌面前端不送生命週期事件——視窗失去焦點就跳出選單在桌面上是干擾，而桌面有正常的
結束流程可以寫檔。

## 檔案位置

`mobile.PathsUnder(filesDir)` 期望拿到 `getExternalFilesDir(null)`：那個位置不需要
任何權限，使用者又能用檔案管理程式或 USB 把東西放進去。

```
<files>/firmware/     internal_68k.bin、umc6650.bin、internal_6502_1.bin、internal_6502_2.bin
<files>/cartridges/   卡帶（raw 或 ZIP）
<files>/states/       每片卡帶一個子目錄，十個存檔槽
<files>/saves/        卡帶電池記憶體
<files>/cheats/       每片卡帶一個金手指檔
<files>/captures/     截圖與錄影
<files>/settings.json 設定
```

韌體與卡帶都由使用者自己放進去：受版權保護的內容不隨程式散布，程式也不代為下載。
四份韌體的檔名與桌面旗標一致，這樣同一份檔案在兩邊可以直接共用。缺檔不是啟動
失敗——介面的啟動畫面與韌體畫面會列出缺哪一份，那比啟動失敗更有用。

## 建置

需要的工具鏈**不在**本專案的 Go image 裡：

| 元件 | 用途 | 概略大小 |
|---|---|---:|
| JDK 17 | gomobile bind 與 Gradle | 180 MB |
| Android cmdline-tools | sdkmanager | 130 MB |
| platforms;android-34、build-tools;34.0.0 | 編譯 Java 端 | 120 MB |
| NDK（r26 以上） | 交叉編譯 Go 端 | 下載約 700 MB，展開約 3.5 GB |

加起來的 image 約 5–6 GB。**這台機器同時放著其他專案的 image，而清理映像不是本
專案可以自行決定的事**，所以這一步要先取得同意再做。

工具鏈就位之後：

```sh
go install github.com/hajimehoshi/ebiten/v2/cmd/ebitenmobile@v2.9.9
ebitenmobile bind -target android -androidapi 21 \
    -javapkg tw.wicanr2.superacan -o acan.aar \
    github.com/wicanr2/superacan-emu/mobile/acan
```

產出的 `acan.aar` 含 `EbitenView`（Ebitengine 產生的）與 `Acan`（gomobile 由本套件
匯出的函式產生的）。Activity 要做的事：

```java
public class MainActivity extends Activity {
    private EbitenView view;

    @Override protected void onCreate(Bundle state) {
        super.onCreate(state);
        try {
            Acan.start(getExternalFilesDir(null).getAbsolutePath());
        } catch (Exception e) { finish(); return; }
        view = new EbitenView(this);
        setContentView(view);
    }
    @Override protected void onPause()  { Acan.suspend(); view.suspendGame(); super.onPause(); }
    @Override protected void onResume() { super.onResume(); view.resumeGame(); Acan.resume(); }
    @Override public void onBackPressed() { if (!Acan.back()) super.onBackPressed(); }
}
```

`AndroidManifest.xml` 不需要任何權限：所有檔案都在 app 自己的外部檔案目錄裡。

## 目前驗證到哪裡

| 項目 | 狀態 |
|---|---|
| `frontend/mobile`（表面政策、檔案位置） | `GOOS=android CGO_ENABLED=0` 建置通過，有單元測試 |
| 生命週期（暫停／恢復／落地／返回鍵） | `session` 的單元測試涵蓋，與平台無關 |
| `mobile/acan`（ebiten.Game、觸控、音訊） | 只有 linux + cgo 的建置與 vet；**沒有在 Android 上跑過** |
| `ebitenmobile bind` | **沒有跑過**，缺 NDK |
| APK、實機觸控、音訊延遲、旋轉、返回鍵 | **全部未驗證** |

實機 smoke 的最小清單（工具鏈就位之後）：載入韌體與卡帶各一次、虛擬手把六個按鍵
各按一次、開選單存讀檔各一次、切到背景再回來確認電池記憶體有寫出去、轉一次螢幕
確認版面重算、按返回鍵確認不會離開應用程式。
