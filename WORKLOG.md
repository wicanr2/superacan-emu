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
