# superacan-emu

Super A'Can（敦煌科技 Funtech，1995，台灣自製 16 位元遊戲機）模擬器的
**Linux 重製**。Bcan 0.0.8b 為閉源 Windows 程式且無公開移植；本專案以知識庫
[taiwan_history/acan](https://github.com/wicanr2/superacan)（`docs/` 內的 (a) 級逆向結論）＋
MAME driver（BSD-3-Clause，僅作規格參考、未複製程式碼）為基礎重新實作。

長期目標：在 Linux 上重製 Bcan 的模擬能力（影像/音效/視窗後續以 SDL2 加入）。

## 目前進度（里程碑 4）

- [x] 68k（Moira）+ 匯流排記憶體映射（依知識庫 `docs/memory-map.md` §2 (a) 級定案）
- [x] UMC6650 lockout 晶片完整實作（埠角色以 IPL 反組譯為準，見
      `docs/bios-68k.md` §3；**MAME `umc6650.cpp` 的埠角色寫反，本實作不沿用**）
- [x] 68k IPL overlay（`$E9001C` bit1/bit3，**單向 latch**，見 `docs/verify-video.md`）
- [x] UM6618 繪圖：3 tilemap 層（8/4/2bpp、scroll/mosaic/linescroll/lineselect/
      優先度混色）+ sprite（含 mask 模式）+ window 0 + sprite DMA；ROZ 暫 stub
- [x] 68k IRQ：vblank（IRQ7）、raster（IRQ4）、line on/off（IRQ5）、
      65C02 mailbox（IRQ6），HOLD_LINE 語意以 Moira `willInterrupt` 模擬
- [x] 主機 DMA 2 通道（byte/word/0xA800 填充/間接模式）
- [x] 65C02（CLK `6502Mk2`）實際執行：I/O `$0400-$04FF`、命令 mailbox、
      boot ack、HALT/釋放（重新上傳驅動；reset 拉住至向量讀取完成）
- [x] **UM6619 音效合成**：16 通道 PCM 取樣（period/音量/key/長度/起始位址）、
      DMA 雙緩衝（IRQ bit6）、內建 timer（IRQ bit7，音樂 tempo）；
      原生 44744 Hz → 線性插值 48000 Hz；SDL2 音訊輸出 + `--wav` headless 錄音
      （`src/audio/um6619.*`，見 `docs/verify-audio-input.md`）
- [x] 65C02 IRQ 來源模型：level-held + 專屬 ack（`$0411` 純狀態）；latch
      `$0404/$0405`（空=`$CD`、68k 寫入觸發 IRQ）
- [x] 手把輸入：SDL 鍵盤（方向鍵 + Z/X/A/S/Q/W = A/B/X/Y/L/R、Enter=Start、
      右 Shift=Select；P1）+ headless `--press frame:BTN,...` 注入；
      65C02 shift register 掃描與 68k direct mode 兩路皆通
- [x] SDL2 視窗輸出 + headless 驗證模式（`--frames`/`--screenshot`/`--wav`/`--press`）
- [x] 遊戲驗證：Boom Zoo、Monopoly（畫面+音樂+按鍵反應）、Speedy Dragon
      （**第二套音樂驅動已修復**，預設模式可跑；見 `docs/verify-audio-input.md`）
- [ ] ROZ 層、雙人輸入、save state、latch 3-byte 封包語意

## 遊戲截圖（開發驗證用途）

> 截圖為各遊戲之版權畫面，僅供模擬器開發驗證，不作其他用途。

| Boom Zoo 標題 | Boom Zoo 角色選擇（按鍵驗證） |
|---|---|
| ![Boom Zoo 標題](docs/screenshots/boomzoo-title-f6000.png) | ![Boom Zoo 角色選擇](docs/screenshots/boomzoo-charselect-after-start.png) |

| Monopoly 標題 | Monopoly 玩家人數選擇（按鍵驗證） |
|---|---|
| ![Monopoly 標題](docs/screenshots/monopoly-title-f3600.png) | ![Monopoly 人數選擇](docs/screenshots/monopoly-playersel-after-start.png) |

| Speedy Dragon 開頭場景 | A'Can 開機 logo |
|---|---|
| ![Speedy Dragon](docs/screenshots/speedydragon-intro-f1200.png) | ![A'Can logo](docs/screenshots/monopoly-logo-f120.png) |

## 建置

需求：CMake ≥ 3.20、C++20 編譯器（GCC 13 驗證過）、git、網路
（首次 configure 以 FetchContent 下載第三方 CPU 核心）。

SDL2：優先用系統套件（`libsdl2-dev`）。本機無 sudo 時的替代：
`apt-get download libsdl2-dev` 後把 headers 解到
`third_party/sdl2-local/include/SDL2/`（含 `x86_64-linux-gnu/SDL2/_real_SDL_config.h`），
並放一個指到系統 runtime 的 `libSDL2.so` symlink（此目錄已 gitignore）；
再不行 CMake 會 FetchContent 自建 SDL 2.30.0（需 X11 dev headers）。

```sh
cmake -B build -DCMAKE_BUILD_TYPE=Release
cmake --build build -j
```

## 執行

ROM 與 BIOS 為受版權保護檔案，**不包含**在本 repo；請自備 Bcan 的
`bios/supracan.zip`、`bios/umc6650.zip` 解壓後的檔案，以及卡帶 ROM。

```sh
# SDL2 視窗（預設）
./build/superacan-emu --bios /tmp/acan_bios \
    --rom "/path/to/Boom Zoo (Taiwan).bin"

# headless 驗證：跑 6000 幀後輸出 BMP 截圖
./build/superacan-emu --bios /tmp/acan_bios \
    --rom "/path/to/Boom Zoo (Taiwan).bin" \
    --headless --frames 6000 --screenshot /tmp/out.bmp

# 錄音 + 按鍵注入（里程碑 3/4）
./build/superacan-emu --bios /tmp/acan_bios \
    --rom "/path/to/Monopoly - Adventure in Africa (Taiwan).bin" \
    --headless --frames 4300 --wav /tmp/out.wav \
    --press 3700:START --screenshot /tmp/after.bmp
```

- `--bios <dir>`：內含 `internal_68k.bin`（4KB IPL）、`umc6650.bin`（16B 金鑰）；
  可選 `internal_6502_1/2.bin`（音訊取樣，開機複製進 sound RAM）
- `--rom <file>`：卡帶 ROM，raw binary 無標頭、16-bit word-swap 格式
  （載入時自動還原，見知識庫 `docs/bios-rom-format.md`）
- `--trace N`：以 Moira 反組譯器 log 前 N 條指令
- `--instructions N`：headless 且無 `--frames` 時，到卡帶入口後再執行的指令數
- `--headless` / `--frames N` / `--screenshot <file.bmp>`
- `--wav <file.wav>`：全程音訊錄成 48 kHz 16-bit stereo WAV
- `--press <spec>`：headless 按鍵注入 `frame:BTN+BTN,...`（按住 10 幀）；
  BTN = A/B/X/Y/L/R/START/SELECT/UP/DOWN/LEFT/RIGHT
- 視窗模式鍵盤：方向鍵 + Z/X/A/S/Q/W = A/B/X/Y/L/R、Enter=Start、右 Shift=Select
- 除錯環境變數：`ACAN_DEBUG`、`ACAN_DMA`、`ACAN_WATCH`、`ACAN_TRACE65`、
  `ACAN_DBG65`（65C02 暫存器定期 dump）、`ACAN_LAYERMASK`、
  `ACAN_DUMP=<prefix>`（見 `docs/verify-video.md`）

預期：通過 UMC6650 交握與授權比對後進入卡帶，vblank IRQ 驅動遊戲主迴圈，
畫面經 SDL2 顯示（Boom Zoo 可見標題「爆爆動物園」）。

## 第三方元件與版本（固定）

| 元件 | 用途 | 版本（commit） | 授權 |
|---|---|---|---|
| [Moira](https://github.com/dirkwhoffmann/Moira) | 68k CPU 核心 | `a4c273b` | MIT |
| [CLK](https://github.com/TomHarte/CLK)（`Processors/6502Mk2`） | WDC 65C02 核心 | `096de57` | MIT |
| [SDL2](https://github.com/libsdl-org/SDL) | 視窗/畫面輸出 | 2.30.0（系統或 FetchContent） | zlib |

MAME `supracan.cpp`（BSD-3-Clause，Angelo Salese / Ryan Holtz 等）：UM6618
渲染、DMA、中斷行為以其為參考**重新實作**（`src/video/um6618.cpp`、
`src/bus.cpp` 標頭註明出處），未複製程式碼。
MAME `umc6619_sound.cpp`（BSD-3-Clause，Ryan Holtz / superctr）：UM6619
PCM 合成模型（通道/period/音量/key/DMA/timer 暫存器語意與混音流程）以其為
參考**重新實作**（`src/audio/um6619.*` 標頭註明出處），未複製程式碼。

## 版權注意

- ROM、BIOS（`internal_68k.bin`、`internal_6502_*.bin`、`umc6650.bin`）皆為
  UMC/Funtech 受版權保護內容，本 repo 不收錄；`.gitignore` 已排除
  `*.bin`/`*.zip`/`ROMS/`/`bios/`。
- `docs/screenshots/` 內的截圖為遊戲執行畫面（遊戲內容仍屬原廠商版權），
  **僅供開發驗證用途**，請勿另作散布。
- 硬體規格結論引自知識庫 `acan/docs/`（對 Bcan 0.0.8b 的逆向分析）。

## 授權

本專案自有程式碼以 MIT 授權釋出（見 `LICENSE`）。第三方元件依其各自授權
（Moira、CLK 皆為 MIT）。
