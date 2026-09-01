# Ebitengine 前端實作紀錄

更新日期：2026-09-01

## 契約

- module 固定 `github.com/hajimehoshi/ebiten/v2 v2.9.9`，依賴雜湊保存於 `go.sum`。
- CPU、machine 與 chip 套件不 import Ebitengine；Ebitengine 只存在於 `frontend` 與
  `cmd/acan` 的主機呈現邊界。
- `machine.System.RunFrame` 依 CPU phase 推進裝置，直到 UM6618 完成下一個 frame；
  instruction bound 只負責失敗即關閉，不會人工注入 vblank。
- `Update` 讀取 P1 鍵盤、設定 machine pad，再要求下一個 machine frame；`Draw` 只把
  完成的 ARGB framebuffer 轉成 RGBA 並上傳。
- UMC6619 原生樣本以整數時間戳線性重取樣為 48 kHz、16-bit stereo PCM。主機佇列具
  200 ms 上限，溢位丟棄最舊樣本、缺料輸出靜音；這些呈現行為不回饋 machine state。
- `--audio=false` 只停用主機播放器，供沒有音效裝置的 Docker／CI 使用；UMC6619 仍按
  machine 時序執行。`--frames N` 提供有界終止，`--screenshot` 輸出同一 framebuffer。

## 已驗證

- Linux 工具鏈由 `docker/ebitengine.Dockerfile` 重建，固定 Go 1.26.7，包含 X11、
  OpenGL、ALSA 與 Xvfb 開發套件。
- 三款 ROM 均在 Xvfb 由 Ebitengine 正常執行 1200 frames，指令數與 framebuffer
  SHA-256 和 headless machine core 基準完全一致：

  | ROM | 68000 指令 | framebuffer SHA-256 |
  |---|---:|---|
  | Speedy Dragon | 18,515,145 | `c49af07407d6de2f32894ac6fc6f646e9baf6bc0f560e7f61da19d6c42c07794` |
  | Formosa Duel | 19,272,069 | `5e5e2f585abfa42790a9b36302ba729319cf469e5a2e01e1f02079aec7363477` |
  | Boom Zoo | 17,370,088 | `fe17ad3c89b2ad8c3a6dbde1821b119d5b2a02677366132f6a67a09170fb7aa5` |

- 三張 PNG 分別可辨識道路角色、標題／START 與房間場景，不再是早期 88-frame 的
  錯誤重複圖樣。
- P1 方向、A/B/X/Y/L/R、Start、Select 已接入 machine controller；PCM byte order、
  缺料靜音、容量上限與最舊樣本丟棄具有不依賴 ROM 的單元測試。
- `presentation`、`frontend` 與 `cmd/acan` 在專用 Docker/Xvfb 工具鏈內測試通過。

## 平台邊界與剩餘驗證

- cgo 政策已定案（2026-09-01）：整個發行 binary 禁止 cgo，前端不例外。現行
  `cmd/acan` 不符合此政策，只能作開發用 GUI，不得列入發行包。
- 依賴實測：Ebitengine v2.9.9 的 `internal/glfw` 只在 darwin 與 windows 使用 purego，
  linbsd 路徑是 cgo；`oto/v3@v3.4.0` 的 `driver_unix.go` 亦為 cgo。因此 Linux
  桌面在 `CGO_ENABLED=0` 無法編譯，`CGO_ENABLED=1` 可編譯。落地禁 cgo 政策需要
  另建純 Go 的視窗／輸入與音訊輸出層，machine／CPU／chip 不受影響。
- Xvfb smoke 使用 `--audio=false`，證明 GUI 與 headless 共用同一 machine 結果；實體 ALSA
  裝置的人耳播放、延遲與 underrun 仍需 Linux 實機驗收。
- `SetTPS(60)` 是主機更新請求；硬體 frame 邊界仍由 cycle scheduler 決定。不得用
  動態改變核心 cycle 數來補償主機音訊緩衝。
