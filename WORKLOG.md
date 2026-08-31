# 工作歷程

## 2026-08-31：接手與文件基線

- 目標：接手 Super A'Can 模擬器，讀取 `../acan` 與對應 Kimi session，建立模擬器專用
  規範、目前真相及唯一工作清單，訂正 README。
- 來源：唯讀檢查 `../acan/AGENTS.md`、`docs/`、Kimi session 索引與主 agent 紀錄；
  以目前 `18110af` 加未提交里程碑 5 程式／`docs/verify-misc.md` 為最新基準。
- 文件變更：新增 `AGENTS.md`、`CONTEXT.md`、`WORKLIST.md`、`WORKLOG.md`；README 改為
  模擬器定位、目前限制、穩定文件入口與 Docker 建置政策。
- 驗證：`openbor-linux-build:local`（CMake 3.28.3、GCC 13.3.0、SDL2 2.30.0）
  在無網路、2 GiB／2 CPU 容器內使用唯讀 Moira／CLK source 完成 Release 建置。
  首次建置發現 GCC 對 `pad_` state 載入的越界警告，改為兩欄明確讀取後增量重建
  無警告；`git diff --check` 通過。
- 動態 smoke：在無網路一次性容器內以唯讀 BIOS／ROM 跑 Boom Zoo 300 幀，成功通過
  IPL／UMC6650、輸出截圖與 save state；新行程載入後續跑 10 幀成功。這只證明單一路徑，
  不取代 `WORKLIST.md` 的跨遊戲決定性矩陣。
- Docker 清理：本輪 `docker run --rm` 容器均已移除；另有既存的
  `openbor-linux-build:local` 容器 `upbeat_rosalind` 正在執行，判定不屬本專案，未碰觸。
- Git：里程碑 5 與文件基線已提交為 `8037a33`，並推送至 `origin/master`。

## 2026-08-31：確認純 Go＋Ebitengine 轉向

- 決策：全面以純 Go 重寫模擬核心，Ebitengine 作跨平台前端；排除 cgo、C ABI 與只換
  前端。現有 C++ 留在同 repo 的 `archive/cpp/`，標為 deprecated oracle。
- 68000：Moira `a4c273b` 只作 sample／差分 oracle，新 Go 核心獨立實作，不直接移植。
- 時序：`Step` 對外一次一條指令，內部按 fetch／prefetch／read／write／internal／IRQ
  poll phase 推進 scheduler；排除純 instruction-total timing 與 pin-level 模擬。
- 調查：Moira 同時具 simple timing 與 precise timing；後者在 memory access 前後同步，
  並於指定 prefetch／poll point 取樣 IPL。據此建立 `docs/chip-emulation-principles.md`，
  但不把 Moira 行為升格為 Super A'Can 硬體證據。
- 驗證：文件相對連結與 `git diff --check` 通過；本輪唯讀調查／文件檢查容器皆以
  `docker run --rm` 結束，沒有留下本專案容器。

## 2026-08-31：純 Go 68000 第一個 vertical slice

- 範圍確認：所有其他晶片也採純 Go 獨立實作與 phase scheduler 通則；舊 C++、MAME、
  Bcan 與實機只作分級 oracle。
- 歸檔：將 CMake 與 `src/` 移至 `archive/cpp/`，每個檔案及 archive README 都標明
  deprecated；根目錄改由 Go module 接管。
- Go 68000：新增 bus／scheduler／phase 契約、register 與兩級 prefetch state、reset
  vector vertical slice、NOP 與 unknown-opcode fail-closed 行為。
- 測試：新增 reset vector、40-cycle 起始契約、scheduler-before-bus 順序、NOP prefetch
  與 unknown opcode 不改狀態測試。
- 驗證：`golang:1.26.7-bookworm` 無網路容器內 `go test ./...` 與
  `go test -race ./...` 均通過；第一次使用 login shell 清掉 image PATH，分類為驗證
  命令問題，改用非 login shell 後以同一 image 乾淨重跑。
- Archive 驗證：`openbor-linux-build:local` 無網路容器以 GCC 13.3、CMake 3.28.3、
  SDL2 2.30.0，從 `archive/cpp/` 新 source root 與唯讀固定 Moira／CLK source 完成
  Release 重建。
- Docker 清理：上述工作均使用 `docker run --rm`；沒有留下本專案容器，所有可寫
  Go 檔案仍由目前 UID/GID 擁有。
- Git：C++ 歸檔與純 Go 68000 第一個 vertical slice 已提交為 `977b2eb`，並推送至
  `origin/master`。

## 2026-08-31：68000 decoder、MOVEQ 與 branch

- 來源：NXP／Motorola Programmer's Reference Manual 的 opcode／condition／branch
  契約，以及 MC68000 User's Manual 表 8-9 的 Bcc／BRA cycle 與 read-count。
- 實作：可稽核 decoder、16 condition、MOVEQ、BRA.b／BRA.w、Bcc.b／Bcc.w，以及每條
  `Step` 的結構化 phase trace；BSR 只辨識並明確回報未實作。
- 測試：condition exhaustive truth table、MOVEQ sign／flags／X preserve、正反向 byte／
  word branch、taken／not-taken timing 與 prefetch refill。
- 修正：首次測試的向後 branch fixture 讓 target prefetch 與原 opcode 位址重疊，造成
  map 值覆蓋；移到不重疊 target 後以同一容器命令乾淨重跑。這是測試資料問題，不是
  CPU branch 計算缺陷。
- 驗證：Go 1.26.7 無網路容器內 `go test ./...`、`go test -race ./...`、`go vet ./...`
  均通過；本輪容器使用 `--rm`，未留下專案容器。
- Git：decoder、MOVEQ、BRA／Bcc 與證據文件已提交為 `8a67cae`，並推送至
  `origin/master`。
