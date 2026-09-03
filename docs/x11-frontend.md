# 純 Go X11 前端

更新日期：2026-09-01

## 為什麼要有第二個前端

cgo 政策定案為「整個發行 binary 禁止 cgo」。實測顯示 Ebitengine v2.9.9 只有
Linux／BSD 桌面目標需要 cgo：`internal/glfw` 在 darwin 與 windows 走 purego，linbsd
路徑是 cgo；音訊的 `oto/v3@v3.4.0` 也是 `driver_unix.go` 使用 cgo。

因此 `cmd/acan`（Ebitengine）保留給 `js/wasm` 與 `windows/amd64`，Linux 桌面改由
`cmd/acan-x11` 提供，兩者共用同一個 machine core。

## 契約

- `frontend/x11` 只做三件事：把 UM6618 的 ARGB framebuffer 以整數倍放大後 `PutImage`
  到視窗、讀取鍵盤狀態、回報視窗關閉。它不持有任何模擬器狀態，也不回饋給核心。
- 滑鼠只在覆蓋層開著時有意義：視窗訂閱 `ButtonPress`／`ButtonRelease`／
  `PointerMotion`，只收第 1 號按鈕（第 4／5 號是滾輪，收進來會變成「在滾輪位置
  點了一下」），移動事件在一幀內合併成最後一筆，然後翻成 `ui.Pointer` 交給介面。
  介面沒開選單時會回報沒有處理。
- 鍵位與 Ebitengine 前端相同：方向鍵、Z=A、X=B、A=X、S=Y、Q=L、W=R、Enter=Start、
  右 Shift=Select；Esc 離開。keysym 直接取自 `GetKeyboardMapping`，不寫死 keycode。
- 主機以 60 Hz 請求下一個硬體 frame（`--pace`，預設開啟）。硬體 frame 邊界仍由 cycle
  scheduler 決定，不用改變核心 cycle 數配合主機。`--pace=false` 供有界 smoke 使用。
- 音訊交給外部播放程序：`--audio-sink "aplay -f cd -t raw"`。UMC6619 的原生樣本重取樣
  為 48 kHz、16-bit stereo 之後寫入該程序的 stdin，佇列滿了丟最舊的樣本。這樣可以在
  不引入 cgo 的前提下有真實聲音；播放端的狀態不影響模擬器時間線。
- `PutImage` 依 `MaximumRequestLength` 切成水平條送出，避免超過單一請求上限。

## 已驗證

`CGO_ENABLED=0` 下 `cmd/acan-x11`、`cmd/acan-headless` 與 `cmd/acan-imgdiff` 全部建置成功。
八款 ROM 在 Xvfb 內以 X11 前端執行 1200 frame，68000 指令數與 framebuffer SHA-256 與
headless 及 Ebitengine 前端三者完全相同：

| ROM | 68000 指令 |
|---|---:|
| Boom Zoo | 17,369,003 |
| Formosa Duel | 19,270,779 |
| Journey to the Laugh | 17,778,132 |
| Monopoly | 11,827,355 |
| Sango Fighter | 11,634,924 |
| Speedy Dragon | 18,513,698 |
| Super Taiwanese Baseball League | 17,572,195 |
| The Son of Evil | 16,727,440 |

framebuffer SHA-256 的值只記在
[`verify-ui.md` 的卡帶基準（C10）](verify-ui.md#卡帶基準c10)，這裡不再複製一份：
雜湊綁在 renderer 現況上，抄成多份就會有幾份過期。本前端要證明的是「與 headless
相同」，那個結論不隨 renderer 改動而失效。

2026-09-02 以帶著 4bpp 半位元組次序修正的 build 重測 Boom Zoo 與 Sango Fighter：
`instructions=17369003`／`f720c9d1…b92301` 與 `instructions=11634924`／`f5bfffa1…4f9f06`，
與同一版 headless 的 `--screenshot` PNG 逐位元組相同。

## 尚未完成

- 只支援 X11。Wayland 需要另一條路徑，或依賴 XWayland。
- 沒有 MIT-SHM，整張影像每 frame 透過 socket 送出。320×240 放大三倍是每 frame 約
  2.7 MB，在本機 socket 上可行，但不是最省的做法。
- 視窗尺寸固定為 `--scale` 的整數倍，不處理視窗被使用者拉伸。
- 音訊需要外部播放程序；純 Go 直接操作 `/dev/snd` 或 PulseAudio 原生協定尚未實作。
- 實體音效裝置上的人耳播放、延遲與 underrun 仍需 Linux 實機驗收。
