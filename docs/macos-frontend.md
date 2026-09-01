# macOS 前端

`cmd/acan-macos` 是 macOS 桌面前端。視窗與輸入透過 [purego](https://github.com/ebitengine/purego)
直接呼叫 Objective-C runtime，所以整個 binary 在 `CGO_ENABLED=0` 下建置得出來，
符合「Linux 與 macOS 的發行 binary 不含 cgo」這條政策。

## 為什麼不用 Ebitengine

Ebitengine 在 darwin 的 cgo 依賴幾乎只剩 GLFW，但那個「幾乎」擋住整條路：
`CGO_ENABLED=0 GOOS=darwin go build` 會在 `internal/graphicsdriver/metal` 的
`view_macos.go`、`displaylink_macos.go` 與 `internal/glfw` 停下來。實測（2026-09-01）：

```
g.view.finishDrawableUsage undefined
g.view.nextDrawable undefined
v.initDisplayLink undefined
```

需要的其實只有「開一個視窗、貼一張圖、收鍵盤事件」，所以自己走 purego 比補上
Ebitengine 缺的那幾塊便宜。

## 做法

| 元件 | 做法 |
|---|---|
| 視窗 | `NSWindow -initWithContentRect:styleMask:backing:defer:`，樣式為 titled＋closable＋miniaturizable |
| 畫面 | content view 設 `wantsLayer`，每幀把畫面包成 `NSBitmapImageRep`、取其 `CGImage`、設成 `layer.contents` |
| 放大 | `layer.magnificationFilter = "nearest"`。硬體輸出是點陣，內插只會製造原本不存在的中間色 |
| 事件 | `nextEventMatchingMask:untilDate:inMode:dequeue:` 迴圈，`keyDown`／`keyUp` 更新按鍵狀態後再 `sendEvent:` |
| 關窗 | 以 `objc.RegisterClass` 註冊 `AcanWindowDelegate`，實作 `windowShouldClose:` 與 `windowWillClose:` |
| 音訊 | 外部播放程序，與 X11 前端共用 `frontend/hostio` |

兩個實作上的取捨值得記：

- **像素緩衝自己留一份。** `NSBitmapImageRep` 不複製 `planes`，直接指向來源緩衝的話，
  下一幀覆寫時畫面會撕裂。
- **`hasAlpha:false` 配 `bitsPerPixel:32`。** 第四個位元組當成填充，因此不必回答
  「alpha 是不是預乘」這個問題——顯示孔徑的輸出本來就不透明。

鍵碼是 macOS 的虛擬鍵碼（`kVK_*`），描述鍵的**實體位置**而不是鍵面上印的字，
所以同一個鍵碼在 QWERTY 與 AZERTY 上是不同的字母。設定檔裡這一組綁定的前端字串
是 `cocoa`，與 X11 的 `x11` 不同：兩者是不同的數值空間，跨前端讀入會得到錯誤的按鍵。

## 驗證狀態

已驗證（可在 Linux 容器內重跑）：

| 項目 | 結果 |
|---|---|
| `CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build` | 通過（含 `frontend/cocoa` 與 `cmd/acan-macos`） |
| `CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build` | 通過 |
| `go vet`（darwin 兩個架構） | 無輸出 |
| 鍵碼表 | `frontend/cocoa` 的單元測試比對 `kVK_*` 常數並檢查鍵名唯一 |
| 模擬核心不受影響 | X11 前端重跑 Boom Zoo 與 Sango Fighter 1200 frame，指令數與 framebuffer SHA-256 與基準相同 |

**尚未驗證**：這個前端沒有在真實 macOS 上跑過。交叉編譯只證明它組得起來，
證明不了 Objective-C 呼叫慣例、視窗生命週期或事件迴圈在實機上的行為。
在有 Mac 之前，不應把它視為可用。

## 實機 smoke 步驟

拿到 Mac 之後依序確認，每一項都失敗即停：

```sh
CGO_ENABLED=0 go build ./cmd/acan-macos
./acan-macos --ipl … --key … --sound-bios1 … --sound-bios2 … \
    --rom "…/Boom Zoo (Taiwan).bin" --frames 1200 --pace=false --scale 3 --config none
```

1. 視窗開得起來且畫面會動。
2. 上面那條命令印出的 `instructions` 與 `framebuffer_sha256` 必須等於
   [`verify-ui.md`](verify-ui.md) 的卡帶基準——不同就表示平台層改到了模擬語意。
3. F1 叫出覆蓋選單，方向鍵與 Return 可以操作，Esc 返回。
4. 存檔與讀檔（選單裡走一次），`--state-root` 下出現 `slot0.acanstate`。
5. `--ui-script` 走一次 P3 的改綁定流程，設定檔的 `frontend` 欄位是 `cocoa`。
6. 關窗按鈕能結束程式（`windowShouldClose:` 有接上）。
7. `otool -L ./acan-macos` 不含任何非系統函式庫。
