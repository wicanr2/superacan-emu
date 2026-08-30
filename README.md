# superacan-emu

Super A'Can（敦煌科技 Funtech，1995，台灣自製 16 位元遊戲機）模擬器的
**Linux 重製**。Bcan 0.0.8b 為閉源 Windows 程式且無公開移植；本專案以知識庫
[taiwan_history/acan](https://github.com/)（`docs/` 內的 (a) 級逆向結論）＋
MAME driver（BSD-3-Clause，僅作規格參考、未複製程式碼）為基礎重新實作。

長期目標：在 Linux 上重製 Bcan 的模擬能力（影像/音效/視窗後續以 SDL2 加入）。

## 目前進度（里程碑 1）

- [x] 68k（Moira）+ 匯流排記憶體映射（依知識庫 `docs/memory-map.md` §2 (a) 級定案）
- [x] UMC6650 lockout 晶片完整實作（埠角色以 IPL 反組譯為準，見
      `docs/bios-68k.md` §3；**MAME `umc6650.cpp` 的埠角色寫反，本實作不沿用**）
- [x] 68k IPL overlay（`$E9001C` bit1/bit3 分別關閉低區/高區 overlay）
- [x] UM6618 / UM6619 / DMA / palette / VRAM stub（讀寫不當機）
- [x] 65C02（CLK `6502Mk2`，WDC65C02）整合進 build＋Bus 介面 stub（不執行；
      IPL 全程不解除副 CPU HALT）
- [x] headless runner 驗證 IPL 開機流程 → 跳卡帶入口（結果見 `docs/verify-ipl.md`）
- [ ] 影像（UM6618）、音效（UM6619）、65C02 實際執行、SDL2 視窗

## 建置

需求：CMake ≥ 3.20、C++20 編譯器（GCC 13 驗證過）、git、網路
（首次 configure 以 FetchContent 下載第三方 CPU 核心）。

```sh
cmake -B build -DCMAKE_BUILD_TYPE=Release
cmake --build build -j
```

## 執行（headless）

ROM 與 BIOS 為受版權保護檔案，**不包含**在本 repo；請自備 Bcan 的
`bios/supracan.zip`、`bios/umc6650.zip` 解壓後的檔案，以及卡帶 ROM。

```sh
# 例：BIOS 解壓在 /tmp/acan_bios/
./build/superacan-emu --bios /tmp/acan_bios \
    --rom "/path/to/Boom Zoo (Taiwan).bin" --trace 20
```

- `--bios <dir>`：內含 `internal_68k.bin`（4KB IPL）、`umc6650.bin`（16B 金鑰）；
  可選 `internal_6502_1/2.bin`（音訊取樣，開機複製進 sound RAM）
- `--rom <file>`：卡帶 ROM，raw binary 無標頭、16-bit word-swap 格式
  （載入時自動還原，見知識庫 `docs/bios-rom-format.md`）
- `--trace N`：以 Moira 反組譯器 log 前 N 條指令
- `--instructions N`：到達卡帶入口後再多執行的指令數（預設 5000）

預期輸出：UMC6650 交握通過 → 卡帶授權比對通過 → 關 overlay →
進入卡帶入口（Boom Zoo 應為 `$00000412`）。詳見 `docs/verify-ipl.md`。

## 第三方元件與版本（固定）

| 元件 | 用途 | 版本（commit） | 授權 |
|---|---|---|---|
| [Moira](https://github.com/dirkwhoffmann/Moira) | 68k CPU 核心 | `a4c273b` | MIT |
| [CLK](https://github.com/TomHarte/CLK)（`Processors/6502Mk2`） | WDC 65C02 核心 | `096de57` | MIT |

MAME `supracan.cpp` / `umc6650.cpp`（BSD-3-Clause）僅作為硬體行為的
參考文獻（經知識庫文件轉述），本專案未複製其程式碼。

## 版權注意

- ROM、BIOS（`internal_68k.bin`、`internal_6502_*.bin`、`umc6650.bin`）皆為
  UMC/Funtech 受版權保護內容，本 repo 不收錄；`.gitignore` 已排除
  `*.bin`/`*.zip`/`ROMS/`/`bios/`。
- 硬體規格結論引自知識庫 `acan/docs/`（對 Bcan 0.0.8b 的逆向分析）。

## 授權

本專案自有程式碼以 MIT 授權釋出（見 `LICENSE`）。第三方元件依其各自授權
（Moira、CLK 皆為 MIT）。
