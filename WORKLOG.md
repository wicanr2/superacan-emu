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
