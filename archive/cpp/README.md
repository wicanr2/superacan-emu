# Deprecated C++ reference implementation

本目錄保存 Super A'Can 模擬器最後一版 C++／SDL2 實作。它在 commit `d923486` 曾是
production，2026-08-31 起停止功能開發，由純 Go 主線取代。

用途僅限：

- 作為 Go 核心的行為與效能差分 oracle；
- 回查已知相容性問題、trace、畫面與音訊輸出；
- 保存 Moira、CLK、SDL2 與 MAME-derived 實作的歷史脈絡。

限制：

- C++ 輸出不是實機硬體真相；每項結論仍依根目錄 `CONTEXT.md` 與
  `docs/chip-emulation-principles.md` 分級。
- 禁止在此新增產品功能或用 C++／cgo 繞過純 Go 重寫。
- ROM、BIOS 與遊戲素材不在 repo；驗證時由使用者合法提供並唯讀掛載。
- 原驗證紀錄保留在根目錄 `docs/verify-*.md`，截圖保留在 `docs/screenshots/`。

## 歷史建置

須在 Docker 內從本目錄執行 CMake。首次 configure 需要取得固定 commit 的 Moira 與
CLK；離線驗證可用已確認雜湊的 source directory 覆寫 FetchContent。

```sh
cmake -S archive/cpp -B /tmp/superacan-cpp-build -DCMAKE_BUILD_TYPE=Release \
  -DFETCHCONTENT_SOURCE_DIR_MOIRA=/path/to/moira-a4c273b \
  -DFETCHCONTENT_SOURCE_DIR_CLK=/path/to/clk-096de57
cmake --build /tmp/superacan-cpp-build -j2
```

此命令只重建 deprecated oracle，不是目前 Go 版的建置入口。

## 窄範圍動態探針

設定 `ACAN_WATCH=1` 時，16-bit 寫入 `$F001F0` 會輸出：

```text
[watchpix] f=<frame> $F001F0 <- $<value> (pc=$<pc>)
```

此探針只記錄 frame、完整 word value 與原始 PC，不改變 register 副作用；用途是驗證
`../acan/docs/f003-video-mode.md` 的 ROM producer。它不是 production 除錯 API，也不能把
`pixel_mode` 名稱升格為硬體語意。
