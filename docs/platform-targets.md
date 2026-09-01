# 發行平台與 cgo 邊界

更新日期：2026-09-01

發行平台限定為 **Linux 桌面、macOS、Android** 三個。Windows 與瀏覽器不在範圍內。

## 實測建置矩陣

`CGO_ENABLED=0`，在固定的 Go 1.26.7 image 內交叉編譯：

| 目標 | `cmd/acan-headless` | `cmd/acan-x11` | `cmd/acan`（Ebitengine）|
|---|---|---|---|
| linux/amd64 | 成功 | 成功 | 失敗 |
| linux/arm64 | 成功 | 成功 | 失敗 |
| darwin/amd64 | 成功 | 成功 | 失敗 |
| darwin/arm64 | 成功 | 成功 | 失敗 |
| android/arm64 | 成功 | 成功 | 失敗 |

`cmd/acan` 三個平台各自失敗在不同地方：

- **linux**：`oto/v3` 的 ALSA driver 與 `internal/graphicsdriver/opengl` 都需要 cgo。
- **darwin**：`internal/graphicsdriver/opengl/graphics_macos.go` 用到 `glfw.Window`，
  而 glfw 的 darwin 實作在 `CGO_ENABLED=0` 下沒有編進來。
- **android**：`internal/vibrate` 的 build constraints 排除全部檔案——Android 走的是
  gomobile 路徑，不是一般的 `go build`。

**「建置成功」不等於「跑得起來」。** `cmd/acan-x11` 在 darwin 與 android 都能編譯，
因為 `jezek/xgb` 只是 X11 協定的純 Go 實作，跟目標平台無關；但 macOS 要另外裝
XQuartz 才有 X server，Android 根本沒有。這兩個目標的「成功」只代表沒有編譯錯誤。

## 政策衝突

2026-09-01 稍早定案「整個發行 binary 禁止 cgo，前端不例外」。當時的量測顯示
`js/wasm` 與 `windows/amd64` 可以做到，缺口只在 Linux 桌面，而 Linux 已由
`cmd/acan-x11` 補上。發行平台改成 Linux／macOS／Android 之後，那兩個原本成立的目標
都不在範圍內，於是：

| 平台 | 目前有沒有可行的無 cgo 路徑 |
|---|---|
| Linux 桌面 | **有**。`cmd/acan-x11` 已完成並驗證，九款卡帶與 headless 逐位元相同 |
| macOS | **沒有現成的**。需要自己用 purego 呼叫 Objective-C runtime 做視窗與音訊 |
| Android | **沒有**。app 必須是 Android 元件，Go 端要以共享函式庫被 JNI 呼叫，那條路必然有 cgo |

## 三個選項與代價

1. **維持全 binary 禁 cgo。** macOS 要自己寫 purego 的 Cocoa 視窗與 CoreAudio 輸出
   （`oto/v3` 的 `api_darwin.go` 證明 purego 走得通，但那是音訊，視窗還得自己來）；
   Android 則要放棄，因為 JNI 沒有無 cgo 的替代。等於發行平台從三個縮成兩個。
2. **把禁令縮到模擬核心。** `machine`／`cpu`／`chip`／`media`／`presentation`／`ui`
   維持 `CGO_ENABLED=0` 可建置並有測試守著；平台層在該平台沒有無 cgo 路徑時允許 cgo。
   Linux 仍然出無 cgo 的 `acan-x11`，macOS 與 Android 走各自平台的既有路徑。
3. **全平台都用 cgo。** 放棄已經做好的 Linux 無 cgo 成果，沒有換到任何東西。

選項 2 保住禁令原本要保護的東西——核心的可攜性與可測試性——而且不必砍掉一個平台。
選項 1 的代價是砍掉 Android，選項 3 的代價是白做。**這一項需要使用者拍板**，
在拍板之前不動現有程式。

## `oto/v3` 在 darwin 是無 cgo 的

單獨建置 `github.com/ebitengine/oto/v3`（`CGO_ENABLED=0`）：

| 目標 | 結果 |
|---|---|
| darwin/arm64 | 成功 |
| darwin/amd64 | 成功 |
| linux/amd64 | 失敗 |
| android/arm64 | 失敗 |

darwin 的 oto 走 purego 呼叫 CoreAudio，不需要 cgo。這把 macOS 的缺口縮小了：
**macOS 缺的只是視窗與繪圖層，音訊已經有無 cgo 的路徑。** Ebitengine 在 darwin
失敗的位置是 `internal/graphicsdriver/opengl/graphics_macos.go` 用到 `glfw.Window`，
而 glfw 的 darwin 實作在禁 cgo 下沒有編進來。

因此選項 1（維持全禁）在 macOS 上要做的是「以 purego 建立 Cocoa 視窗並把 RGBA
緩衝貼上去」，不必連音訊一起自己寫。這比原先估的小，但仍是一個新的平台層。

## 不受影響的部分

不論選哪一個，這些都成立：

- `machine`／`cpu`／`chip`／`media`／`presentation` 目前在五個目標上都能以
  `CGO_ENABLED=0` 建置，這個性質要有 CI 守著，不能只靠人記得。
- `cmd/acan-headless` 在五個目標上都是無 cgo 的，核心回歸不依賴任何平台層。
- 規劃中的 `ui` 套件是純 Go 自繪，不 import 任何前端套件，因此不受平台層決定影響。
