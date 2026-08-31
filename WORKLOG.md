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

## 2026-08-31：68000 IPL 起始 JSR 與 MOVEA

- IPL 證據：`../acan` 的 CPU 可見反組譯確認 `$400` 為 `NOP`、`$402` 為
  `JSR $040A.W`，其後以 `MOVEA.L #imm,An` 建立 UMC6650 相關基址；原始 BIOS dump
  為逐 word byte-swap 儲存，不能直接把檔案 byte order 當成 CPU opcode。
- 來源：NXP／Motorola Programmer's Reference Manual 的 JSR／MOVEA 語意，以及
  MC68000 User's Manual 的 execution-time 表；Moira master
  `a4c273b08e07d82c73289ac032867c845969a0f2` 只用來核對 68000 JSR 的 phase 次序。
- 實作：新增共用 instruction extension-word stream、`MOVEA.L #imm,An`、
  `JSR (xxx).W`、可觀測的 16-bit data-write phase 與兩次有序的堆疊 long write。
- 契約：JSR 將 PC+4 寫入 A7-4，然後重填目標 instruction queue；總計 18 cycles，
  phase 為 internal 2、兩次 data write、兩次 instruction fetch。未知 addressing mode
  仍失敗即關閉，不以泛化 stub 放行。

## 2026-08-31：IPL 第一個 UMC6650 poll 分支

- 範圍：沿已證實 BIOS 路徑補上 `$41C MOVE.W $E90B3C.L,D0`、
  `$422 ANDI.W #1,D0`、`$426 BEQ.W`，並實作未取分支需要的
  `MOVE.W Dn,$E90B3C.L`，不向後擴張成未使用的完整 MOVE matrix。
- 實作：instruction stream 新增 long extension helper；新增絕對長位址 word read／write、
  16-bit condition-code 更新，以及每次資料存取的 supervisor-data phase。
- 時序核對：NXP／Motorola User's Manual 給出兩種 MOVE 皆 16 cycles、ANDI.W register
  為 8 cycles；固定 Moira `a4c273b08e07d82c73289ac032867c845969a0f2` 只用來確認
  register-to-absolute-long MOVE 是「消耗兩個 extension → data write → final prefetch」。
- 測試：以最小合成 IPL fixture 執行八條指令，從 `$400` 抵達 `$430`，驗證 A0/A1/A2、
  zero branch、資料 phase 與包含 reset 的 132 cycles；fixture 不含完整 BIOS 或版權資料。

## 2026-08-31：UMC6650 RAM 備份迴圈

- 範圍：實作 BIOS `$430–$448` 所需的 `MOVE.W #imm,Dn`、`MOVE.W Dn,Dn`、
  `MOVE.B Dn,(An)`、`MOVE.B (An),(An)+`、`MOVE.W #imm,(xxx).L`、`CMPI.W` 與 `DBcc`。
- 匯流排：新增 byte read/write phase；仍維持 scheduler 先推進、再呼叫 bus side effect。
  `(A7)+` 的 byte increment 為 2，其餘 address register 為 1。
- 旗標：MOVE byte／word 與 CMPI word 各有獨立寬度契約；CMPI 保留 X 與 operands，
  明確測試 equal、positive、borrow-negative 與 signed-overflow。
- 迴圈回歸：不含 BIOS 映像的合成 fixture 執行 162 條指令，完成 32 次 `$5F…$40`
  倒數；驗證 32 次 address-port byte write、32 次 data-port read、Work RAM `$FC0000…1F`
  post-increment write、32 次 noise word write，並以 1910 reset-inclusive cycles 離開迴圈。
- DBcc：分別測試 condition true 12 cycles、decrement-and-branch 10 cycles、counter expired
  14 cycles；未將後續 CPU 型號的 loop mode 套入 MC68000。

## 2026-08-31：Go 整機 bus 與真實 IPL 探測

- media：新增逐 16-bit word byte-swap、嚴格大小檢查、原始輸入 SHA-256 與轉換 manifest；
  UMC6650 key 維持線性 16 bytes。ROM／BIOS 內容不加入版控。
- UMC6650：新增獨立 Go chip package，實作 7-bit 位址、`$20–$2F` 唯讀 key、
  `$40–$5F` RAM 及 `$09/$0C` output register 儲存。
- machine：新增 24-bit bus、低／高 IPL overlay 單向 latch、卡帶雙視圖、Work RAM mirror、
  sound RAM、SRAM odd lane、`$E90B3C` 與 shared phase timeline；未知晶片 window 尚未假造。
- runner：新增 `cmd/acan-headless`，要求外部 IPL/key/ROM，輸出輸入雜湊、PC、opcode、
  instruction count 與 cycles；未知 opcode 失敗即關閉。
- 真實驗證：IPL SHA-256 為 `2e4d88bec69b5e7e4803368c233ce0d20f6dd107c5af0cfcc0089d310c695d7c`。
  第一輪停 `$46C CMP.B (A1),D0`，補實作後停 `$47C MOVE.B -(A2),(A1)`；再補實作後
  已跨過 RAM restore 與 key 讀取迴圈；成功完成 772 條指令後，精確停於
  `$4C2 CLR.W D4`、8652 cycles。

## 2026-08-31：完整 IPL、卡帶授權與 overlay 轉交

- checksum ISA：新增 CLR byte/word/long、ADD/SUB byte/word/long、ADDX、ADDQ/SUBQ、
  ANDI/ORI、CMP/CMPI/CMPM、BTST、NEG、SWAP，以及 predecrement/postincrement 變體。
- MULS：依 MC68000 Booth transition 規則建模 `42 + 2n` 的 `(An)+` 總週期，乘數為
  16-bit signed、結果為 32-bit，並測試資料相依 timing。
- 卡帶授權：真實 Boom Zoo 已通過 `$570` 的 64-word CMPM loop 與 `$578–$5F0`
  巢狀 MULS checksum；沒有略過比較、硬編結果或遊戲特判。
- 轉交：新增 stack immediate MOVE、absolute／indirect JMP、absolute-long JSR、ORI、
  `(An)` word MOVE 與 absolute-word MOVEA。新增合成 regression，證明 `$61E` 關 high
  overlay 後，已預取的 `$620 JMP (A0)` 仍執行並從卡帶向量進入 `$400`。
- 真實結果：成功完成 87,204 條指令、797,418 cycles；low/high overlay 均為 off，
  執行卡帶 `$420 JSR $2B22` 後停於 `$2B22 MOVEM.L`（opcode `$48E7`）。

## 2026-08-31：卡帶啟動與高位址 IPL 服務路徑

- 從 Boom Zoo `$2B22` 開始，以未知 opcode 失敗即關閉的方式逐段擴充一般 68000
  語意；沒有依 ROM hash、PC 或遊戲名稱加入特判。
- 堆疊／呼叫：新增 MOVEM.L predecrement／postincrement、BSR、RTS、PEA、long push，
  以及 brief-indexed JSR；回傳位址、A7 更新與 long-word bus 順序可觀察。
- 定址：新增 MC68000 brief extension 的 Dn／An、word／long index，套用於卡帶路徑所需
  MOVE／MOVEA／JMP／JSR；另補 PC-relative LEA。未接受 68020 full extension 或 scale。
- 算術／搬移：補齊實際路徑要求的 byte／word／long MOVE 變體、quick／immediate
  ADD/SUB/CMP、shift／rotate、BSET 與 OR；各 encoding 是一般暫存器形式而非單一 opcode
  stub。
- 真實驗證：固定 IPL `2e4d88…c695d7c` 與 Boom Zoo ROM `090827…370077` 在 Go
  headless runner 無錯完成 200,000 條指令、1,935,470 cycles，PC `$FF80A0`、opcode
  `$6AF0`，low/high overlay 均為 off；結束原因是指定指令上限。
- 回歸：新增 MOVEM push／restore round-trip、PEA effective-address push、indexed JSR
  return／prefetch、ROL flags／timing，以及本批真實 opcode 的 decoder cases。
- 證據限制：這證明固定軟體路徑已前進，不代表完整 MC68000、exception／IRQ 或遊戲
  可玩；下一步須以裝置交易 checkpoint 判定 UM6618／UM6619／sound RAM 初始化進度。

## 2026-08-31：雙 CPU sound boot 與第一筆 VRAM 初始化

- 新增 bus transaction observer 與 headless `--watch`／`--watch-limit`；byte／word access
  都以完整 CPU transaction 記錄，並附 68000 step、PC、opcode。範圍解析、保留上限與
  word 不重複計數皆有測試。
- 第一輪證據：第 178,789 條起將 driver 寫入 `$E8F000`，第 182,885 條寫
  `$E9001C=$0001`，第 395,493 條開始輪詢 `$E80300`；沒有 sound CPU 時永遠讀 `$00`。
- 新增獨立純 Go W65C02 core、sound bus、3:1 shared scheduling 與 reset/HALT gate；實作
  真實 boot 所需 ISA 子集。新增 UMC6619 indirect register port，未假造 PCM／timer。
- 真實結果：65C02 從 `$F000` reset vector 起跑並把 `$0300` 寫成 `$FF`；68000 第一次
  輪詢即取得 ack，離開原本等待迴圈。其後在第 397,684 條起觀察到 `$F44400` VRAM
  word writes，證明已進入視訊資料初始化。
- 目前前進至 462,153 條 68000 指令附近；W65C02 已執行約 311,000 條。這只證明 boot
  路徑，尚不代表完整 65C02、IRQ、UM6618 renderer 或 UMC6619 音訊完成。
