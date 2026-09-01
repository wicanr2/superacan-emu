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

| ROM | 68000 指令 | framebuffer SHA-256 |
|---|---:|---|
| Boom Zoo | 17,369,003 | `3784f8663b1c3a869498d2e14c0b948c598d50d15cf54b6f5380c9b294155562` |
| Formosa Duel | 19,270,779 | `0856269e7b402158e953de03d0553128d720ef64f29afc97403f93471404d587` |
| Journey to the Laugh | 17,778,132 | `42285d489bd74a5c5fd0d66700ed7e7c8b2b83f4855612d7dec4db07c30b146e` |
| Monopoly | 11,827,355 | `c254c50d5f85dd6ede60b82c8b2a07ca2ca8ccd41e9bfbe65a1c45299083d582` |
| Sango Fighter | 11,634,924 | `412213dac64ec07ef8db6ee69f4a90a351880f11c3229b378647d05f559bd505` |
| Speedy Dragon | 18,513,698 | `d3e5336af35b4c5bdac93dca6e1f3686be861564f16d69a97ef8fa947a5b7d67` |
| Super Taiwanese Baseball League | 17,572,195 | `e28f1c411a389ecd46206d8006e1e9b54f62a75047bcb2e64b7f12763f094023` |
| The Son of Evil | 16,727,440 | `bbd3a45fb5d27acf8e6caef06f5c9f7d00f8743d2ad42b0bbb8baea2d23bca73` |

## 尚未完成

- 只支援 X11。Wayland 需要另一條路徑，或依賴 XWayland。
- 沒有 MIT-SHM，整張影像每 frame 透過 socket 送出。320×240 放大三倍是每 frame 約
  2.7 MB，在本機 socket 上可行，但不是最省的做法。
- 視窗尺寸固定為 `--scale` 的整數倍，不處理視窗被使用者拉伸。
- 音訊需要外部播放程序；純 Go 直接操作 `/dev/snd` 或 PulseAudio 原生協定尚未實作。
- 實體音效裝置上的人耳播放、延遲與 underrun 仍需 Linux 實機驗收。
