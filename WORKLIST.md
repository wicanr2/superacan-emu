# Super A'Can 模擬器工作清單

更新日期：2026-09-01。狀態只反映目前程式與最近證據；歷史見 `WORKLOG.md`。

## 進行中：純 Go 轉向

- [x] 把目前 C++／CMake／SDL2 實作移至 `archive/cpp/`，在 archive README 標明
  deprecated、最後 production commit `d923486`、證據限制與唯讀 oracle 用途。
  完成條件：Docker 內仍可重建 archived binary，原有驗證文件與第三方 notice 可回查。
- [x] 建立純 Go module、package 邊界與 headless test runner；production path 禁止 cgo。
  完成條件：`go test ./...` 在無網路固定 image 內通過，machine core 不 import Ebitengine。
- [x] 建立獨立 Go Motorola 68000 核心骨架。
  完成條件：公開 bus／phase／IRQ API、register 與 prefetch state、`Step` result、reset
  vector vertical slice；設計符合 `docs/chip-emulation-principles.md`。
- [x] 建立 opcode／addressing-mode／exception 測試階梯與 Moira 差分 harness。
  完成條件：每個差異可分類為 Go bug、sample 差異或硬體 unknown，不以 Moira 自動定案。
- [x] 建立第一組 opcode decoder、16 種 condition code、MOVEQ、BRA.b／BRA.w 與
  Bcc.b／Bcc.w phase trace。完成證據見 `docs/m68k-implementation.md`。
- [x] 實作共用 extension-word cursor，以及 IPL 起始路徑所需的 `JSR (xxx).W` 與
  `MOVEA.L #imm,An`；包含 18-cycle JSR、監督者堆疊 long-word 寫入與目標 queue refill。
- [x] 建立目前啟動路徑所需的 operand size 與 68000 effective-address 基礎，完成 BSR、
  RTS、MOVEM、PC-relative／brief-indexed JSR／JMP／LEA／MOVEA 等模式；一般化完整 EA
  matrix 仍屬後續 ISA 工作。
- [x] 完成 IPL `$41C–$42A` 的絕對長位址 word 讀寫、`ANDI.W #imm,Dn` 與第一個
  `BEQ.W` 垂直切片；以不含商業 BIOS 的合成程式驗證 `$400 → $430`、暫存器基址、
  phase trace 與 132-cycle reset-inclusive 契約。
- [x] 完成 IPL `$430–$448` 的 UMC6650 RAM 備份迴圈：byte `(An)`／`(An)+`、
  immediate／register `MOVE.W`、`CMPI.W`、`DBLE` 與立即數到絕對長位址寫入。
  合成回歸執行 32 次迭代並驗證 `$5F…$40`、Work RAM 遞增及三種 DBcc timing。
- [x] Go 68000 跑通完整 Super A'Can IPL 並進入卡帶入口。
  完成條件：固定 BIOS hash、reset SSP／PC、phase trace 與 C++ oracle 對照，逐步抵達
  UMC6650 交握；缺 opcode 時明確停止，不用 stub NOP。
- [x] 建立 Go media／machine 第一層：word-swap 與 SHA-256 manifest、ROM／IPL overlay、
  Work/sound RAM、SRAM lane、`$E90B3C`、UMC6650、shared timeline 與 headless runner。
- [x] 以固定真實 IPL `2e4d88…c695d7c` 及 Boom Zoo 驗證 `$400 → $620 → cart`；
  UMC6650 checksum、64-word 授權比較、第二階段 MULS hash 與雙 overlay 關閉均通過。
- [x] 從 Boom Zoo `$2B22 MOVEM.L` 擴充卡帶啟動 ISA，直到第一個 UM6618／UM6619／
  sound RAM 初始化交易。已完成 sound driver 上傳、65C02 `$0300=$FF` boot ack，並觀察
  `$F44400` 起 VRAM 寫入；核心沒有遊戲專屬 opcode stub。
- [x] 將 UM6618 register／palette／VRAM 接入 Go bus，保證 word write 單次生效；以
  真實 VRAM hash、palette／register trace 與自然離開 vblank poll 驗證。
- [x] 建立 UM6618 tilemap／sprite／window／ROZ 第一版 framebuffer；固定 Boom Zoo
  1,300,000 指令可產生 61,437 個非黑像素與可重現 SHA-256。此項只證明第一張可合成
  frame，不代表 archived oracle 畫面一致。
- [x] 完成 UM6618 sprite DMA 同步 bus transaction：`count+1`、來源／目的 word stride、
  零填充與 VRAM 目的高位模式；合成測試驗證複製／填充，真實 smoke 無回歸。
- [x] 接通 UM6618 掃描線 IRQ4／5／7 與 68000 autovector／acknowledge；Boom Zoo 固定
  smoke 實際受理 58 次 IRQ7，IRQ4／5 目前只有合成線位測試。
- [x] 依 MAME-derived 契約實作 UM6618 ROZ 三張逐行表：incxx、scrollx、scrolly 與
  zero-line suppression；Boom Zoo frame 88 hash 確實改變。硬體正確性仍待 oracle。
- [x] 固定 Ebitengine v2.9.9，建立 `cmd/acan` 視窗入口、P1 鍵盤、RGBA framebuffer、
  48 kHz 音訊、`--frames` 有界 smoke 與 PNG 輸出；三款 ROM 1200-frame Xvfb 路徑
  均得到可辨識畫面，指令數與 framebuffer hash 完全吻合 headless 基準。
- [x] 建立 Bcan 0.0.8b 畫面 oracle 管線：`docker/bcan-oracle.Dockerfile`、
  `docker/bcan-oracle.sh`、`acan-headless --screenshot-dir/--screenshot-every` 與
  `cmd/acan-imgdiff`。第一輪已定位並修正 5 位元調色盤展開，見
  `docs/bcan-oracle-diff.md`。
- [ ] 在靜止畫面（標題選單）上完成逐像素差分並分類每一處差異。目前被 CPU 擋住：
  Boom Zoo 第 1,695 個 frame 遇 `$D06A` 停止，走不到標題。完成條件：至少一款 ROM
  的靜止畫面差異能逐項標成 renderer 缺陷、oracle 侷限或硬體 unknown。
- [x] 收斂 68000 與 65C02 的 ISA 結構：改為一般化 effective-address 執行層與 256 項
  指令表，既有逐一 case 仍優先且行為不變。八款 ROM 由各自停在不同編碼變成全部完成
  3600 frames。設計與時間模型見 `docs/cpu-generic-execution.md`。
- [ ] 把既有 233 條逐一 case 逐步遷移到一般化層並刪除，遷移過程每步都要維持測試綠燈。
  完成條件：`Decode` 只保留無法一般化的編碼，兩套路徑不再並存。
- [ ] 補上 68000 的例外路徑：TRAP、TRAPV、CHK、除以零、位址錯誤與匯流排錯誤、
  privilege violation、MOVE USP、RESET、STOP。目前這些一律 fail-closed。
- [ ] 將 65C02 的 3:1 排程收斂成可驗證的 cycle 邊界；ISA 覆蓋已由 256 項指令表完成。
- [x] 將 UMC6619 從間接 register port 擴充為 PCM、timer、DMA 與 IRQ 來源，並接上
  Ebitengine 主機音訊佇列；實機音訊播放與未知 envelope 仍分列驗收／證據缺口。

## Deprecated C++ 收尾紀錄

- [ ] 在專案專用、可重現的 Docker image 內從乾淨 archive build 目錄完成 Release 建置。
  完成條件：固定 compiler／CMake／SDL2 與依賴 commit，無主機 runtime 混入，輸出由
  目前 UID/GID 擁有。
- [ ] 審查全部未提交程式變更，移除或隔離 `ACAN_STAGING` 等一次性探針。
  完成條件：核心正常路徑沒有遊戲專屬特判，`git diff --check` 通過。
- [x] Go 主線的 save state 與交易式載入。格式 `ACANGOS1` 綁定 IPL 與卡帶 SHA-256，
  版本、標頭長度、payload 長度與 payload 雜湊逐項驗證，全部通過才一次套用；
  四種壞檔都有測試守著。見 `docs/save-state.md`。
- [x] save-state 決定性回歸。Boom Zoo 在 frame 600 存檔、另一個行程載入後續跑
  600 frame，指令數與 framebuffer SHA-256 與連續跑 1200 frame 完全相同。
- [ ] 把決定性回歸擴到第二套音效驅動的遊戲，並比對同一取樣視窗的音訊雜湊。
- [x] 重跑最小相容性矩陣。八款 raw ROM 全部完成 3600-frame 有界執行與 5400-frame
  帶輸入路徑，見 `docs/verify-rom-matrix.md`。
- [ ] P2 至少完成一條實際雙人選單或遊戲流程，不只讀值 dump。

## 硬體證據缺口

- [ ] FRC：取得實機、Bcan 動態 trace 或更強證據，取代目前 MAME HACK case 表；在此之前
  保持 `MAME-derived/unknown-hardware` 標示。
- [ ] UM6619：確認 envelope、混音增益、削波與未知暫存器；禁止用聽感猜出演算法。
- [ ] latch 3-byte 封包：從 68k producer、65C02 consumer 到玩家可見效果建立完整鏈。
- [ ] window 1／複雜 ROZ line table：找到實際使用軟體與同狀態 oracle 後再升格。
- [ ] partial update：只有出現可重現的 mid-frame 差異才實作，不因舊 TODO 自動開工。

## Go 核心工程

- [x] 建立純 Go phase scheduler，讓 Linux、headless、macOS 與 Ebitengine 前端共用
  同一硬體時間線；不沿用 C++ `main.cpp` runner 架構。
- [ ] 為 register transaction、IRQ edge／level／ack、DMA 邊界、reset 與 open-bus 建立
  不依賴商業 ROM 的單元／整合測試。
- [ ] 將一次性環境變數 trace 整理成有界、可篩選的 device／address-space 除錯介面。
- [ ] 建立 ROM／BIOS manifest：檔名、大小、雜湊、word-swap 規則與錯誤訊息；版權輸入
  繼續排除於 Git。

## 平台與發行

- [x] 決定 cgo 邊界。2026-09-01 定案：**整個發行 binary 禁止 cgo，前端不例外**。
- [x] 盤點禁 cgo 政策的實際缺口。`CGO_ENABLED=0` 下 headless 與 imgdiff 在任何平台
  都能建置，`cmd/acan` 的 `js/wasm` 與 `windows/amd64` 目標也能建置；只有
  `linux/amd64` 失敗。
- [x] 讓 Linux 桌面前端在 `CGO_ENABLED=0` 建置成功。新增 `frontend/x11` 與
  `cmd/acan-x11`：純 Go 的 X11 視窗、輸入與整數倍放大，音訊交給外部播放程序。
  八款 ROM 的 1200-frame 指令數與 framebuffer SHA-256 與 headless 完全相同。
  見 `docs/x11-frontend.md`。
- [ ] 純 Go 的音訊輸出（直接操作 `/dev/snd` 或 PulseAudio 原生協定），取代目前的
  外部播放程序。
- [ ] Linux 發行包改以 `cmd/acan-x11` 為桌面入口；`cmd/acan` 保留給 `js/wasm` 與
  `windows/amd64` 這兩個 `CGO_ENABLED=0` 可建置的目標。
- [ ] 在有實體音效裝置的 Linux 驗收 48 kHz 播放、鍵盤操作、延遲與 underrun。
- [x] 八款 ROM 各完成 1200-frame GUI 正常路徑，指令數與 framebuffer SHA-256 與
  headless 完全一致；結果見 `docs/verify-rom-matrix.md`。

- [ ] 里程碑 5 收斂後規劃 macOS 編譯。
  完成條件：選定可重現工具鏈、SDL2 來源、支援架構與最低 macOS 版本，產物在 macOS
  實機完成啟動、輸入、音訊、headless 與 save-state smoke。這是平台工作，不得修改
  模擬核心來通過封包。
- [ ] 建立 Linux 可重現發行包與第三方授權清單；只含程式，不含 ROM／BIOS／遊戲畫面。

## 已完成且不重新開啟

- [x] IPL、UMC6650 lockout、68k／65C02 基本執行與主要 bus mapping。
- [x] UM6618 主要 tilemap／sprite／DMA／IRQ 與 SDL2/headless 畫面路徑。
- [x] UM6619 PCM、DMA、timer、WAV 與 P1 輸入；Speedy 第二驅動可播放。
- [x] sprite DMA word 雙觸發根因修正；Boom Zoo／Monopoly 標題結構恢復。
- [x] ROZ、P2、FRC、save state 初版程式已寫入工作樹；仍須完成上列收斂閘門。
