# 使用者介面規劃

完整設計見 [`docs/ui-design.md`](ui-design.md)：功能對照表、資訊架構、十六張畫面
線框、三平台差異化、互動模型、視覺規範、文案、設定檔、分階段驗收條件與待決策。

對照基準見 [`docs/bcan-ui-inventory.md`](bcan-ui-inventory.md)，字型與語言涵蓋見
[`docs/ui-font.md`](ui-font.md)，平台與 cgo 邊界見
[`docs/platform-targets.md`](platform-targets.md)。

## 為什麼是自繪的 `ui` 套件

三個既有契約把選項收斂到只剩這一條：

1. 發行平台是 Linux 桌面、macOS、Android，模擬核心與呈現層要三個平台共用。
2. cgo 政策讓系統原生 widget（GTK／Qt／Cocoa／Android View）都不能用來畫模擬器介面。
3. `AGENTS.md` 要求前端不得直接改寫晶片內部狀態。

因此 `ui` 是純 Go 套件，把介面畫進 RGBA 緩衝並消費抽象輸入事件；前端只負責貼圖與
把實體輸入翻成抽象事件。介面因此可以在 headless 以畫面雜湊驗證，而且 `ui` 只回傳
意圖、不持有 `*machine.System`，「UI 不寫晶片」成為結構保證而不是自律。
